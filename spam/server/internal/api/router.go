// Package api wires the Gin router: JSON API under /api plus the exported web app.
package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/cragone/monotheism/spam/server/internal/config"
)

type Server struct {
	db  *gorm.DB
	cfg config.Config
	lim *limiter
}

func New(db *gorm.DB, cfg config.Config) *gin.Engine {
	s := &Server{db: db, cfg: cfg, lim: newLimiter()}
	if cfg.Prod {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery(), securityHeaders())
	// Traefik/Cloudflare sit in front; we read client IPs ourselves (see clientIP).
	_ = r.SetTrustedProxies(nil)

	api := r.Group("/api", noStoreAPI(), s.loadUser())
	api.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	api.GET("/me", s.me)
	api.POST("/register", s.register)
	api.POST("/login", s.login)
	api.POST("/logout", s.logout)

	api.GET("/threads", s.listThreads)
	api.GET("/threads/:id", s.getThread)

	authed := api.Group("", requireUser)
	authed.POST("/threads", s.createThread)
	authed.DELETE("/threads/:id", s.deleteThread)
	authed.POST("/threads/:id/replies", s.createReply)
	authed.POST("/threads/:id/like", s.likeThread)
	authed.DELETE("/replies/:id", s.deleteReply)
	authed.POST("/replies/:id/like", s.likeReply)

	r.NoRoute(s.static())
	return r
}

// static serves the exported React Native web build, falling back to index.html so deep links
// like /t/12 load the app shell.
func (s *Server) static() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || s.cfg.WebDir == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.AbortWithStatus(http.StatusMethodNotAllowed)
			return
		}
		root, _ := filepath.Abs(s.cfg.WebDir)
		clean := filepath.Join(root, filepath.FromSlash(filepath.Clean("/"+c.Request.URL.Path)))
		if st, err := os.Stat(clean); err == nil && !st.IsDir() {
			// Expo fingerprints bundles under /_expo/static, so those are safe to cache hard.
			if strings.HasPrefix(c.Request.URL.Path, "/_expo/static/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			}
			c.File(clean)
			return
		}
		if filepath.Ext(c.Request.URL.Path) != "" {
			c.String(http.StatusNotFound, "Not found")
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.File(filepath.Join(root, "index.html"))
	}
}
