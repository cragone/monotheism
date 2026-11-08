package api

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cragone/monotheism/spam/server/internal/auth"
	"github.com/cragone/monotheism/spam/server/internal/models"
)

const userKey = "user"

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

func noStoreAPI() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}

// loadUser resolves the Bearer token (if any) to a user. It never rejects the request.
func (s *Server) loadUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		if tok, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer "); ok && tok != "" {
			var u models.User
			err := s.db.Joins("JOIN sessions s ON s.user_id = users.id").
				Where("s.token_hash = ? AND s.expires_at > ?", auth.HashToken(tok), time.Now()).
				First(&u).Error
			if err == nil {
				c.Set(userKey, &u)
			}
		}
		c.Next()
	}
}

func currentUser(c *gin.Context) *models.User {
	if v, ok := c.Get(userKey); ok {
		return v.(*models.User)
	}
	return nil
}

func requireUser(c *gin.Context) {
	if currentUser(c) == nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Log in first."})
		return
	}
	c.Next()
}

// limiter is a small in-memory fixed-window rate limiter. Traefik rate-limits the edge; this
// protects abusable actions (password hashing, posting) per IP or per user.
type limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}
type bucket struct {
	count int
	reset time.Time
}

func newLimiter() *limiter {
	l := &limiter{buckets: map[string]*bucket{}}
	go func() {
		for range time.Tick(time.Minute) {
			l.mu.Lock()
			now := time.Now()
			for k, b := range l.buckets {
				if b.reset.Before(now) {
					delete(l.buckets, k)
				}
			}
			l.mu.Unlock()
		}
	}()
	return l
}

func (l *limiter) allow(key string, max int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b := l.buckets[key]
	if b == nil || b.reset.Before(now) {
		l.buckets[key] = &bucket{count: 1, reset: now.Add(window)}
		return true
	}
	b.count++
	return b.count <= max
}

// limit returns false (after writing a 429) when key has exceeded max requests per window.
func (s *Server) limit(c *gin.Context, key string, max int, window time.Duration) bool {
	if s.lim.allow(key, max, window) {
		return true
	}
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests — slow down a bit."})
	return false
}

// clientIP prefers Cloudflare's header, then the first X-Forwarded-For hop.
func clientIP(c *gin.Context) string {
	if ip := c.GetHeader("CF-Connecting-IP"); ip != "" {
		return ip
	}
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	return c.ClientIP()
}
