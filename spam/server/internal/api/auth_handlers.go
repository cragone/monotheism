package api

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/cragone/monotheism/spam/server/internal/auth"
	"github.com/cragone/monotheism/spam/server/internal/models"
)

const sessionTTL = 30 * 24 * time.Hour

var usernameRE = regexp.MustCompile(`^[A-Za-z0-9_]{3,24}$`)

type userDTO struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func fail(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}

func (s *Server) startSession(c *gin.Context, u *models.User, status int) {
	token, hash, err := auth.NewToken()
	if err == nil {
		err = s.db.Create(&models.Session{TokenHash: hash, UserID: u.ID, ExpiresAt: time.Now().Add(sessionTTL)}).Error
	}
	if err != nil {
		fail(c, http.StatusInternalServerError, "Internal error")
		return
	}
	// Opportunistic cleanup keeps the table small without a background job.
	s.db.Where("expires_at < ?", time.Now()).Delete(&models.Session{})
	c.JSON(status, gin.H{"token": token, "user": userDTO{u.ID, u.Username}})
}

func (s *Server) me(c *gin.Context) {
	if u := currentUser(c); u != nil {
		c.JSON(http.StatusOK, gin.H{"user": userDTO{u.ID, u.Username}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": nil})
}

func (s *Server) register(c *gin.Context) {
	if !s.limit(c, "reg:"+clientIP(c), 5, time.Hour) {
		return
	}
	var in credentials
	if c.ShouldBindJSON(&in) != nil {
		fail(c, http.StatusBadRequest, "Invalid JSON")
		return
	}
	if !usernameRE.MatchString(in.Username) {
		fail(c, http.StatusBadRequest, "Username must be 3–24 letters, numbers or underscores")
		return
	}
	// bcrypt only uses the first 72 bytes, so reject longer passwords instead of truncating.
	if len(in.Password) < 8 || len(in.Password) > 72 {
		fail(c, http.StatusBadRequest, "Password must be 8–72 characters")
		return
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		fail(c, http.StatusInternalServerError, "Internal error")
		return
	}
	u := models.User{Username: in.Username, PasswordHash: hash}
	if err := s.db.Create(&u).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			fail(c, http.StatusConflict, "That username is taken")
			return
		}
		fail(c, http.StatusInternalServerError, "Internal error")
		return
	}
	s.startSession(c, &u, http.StatusCreated)
}

// dummyHash lets login do equal work for unknown usernames so timing doesn't reveal them.
var dummyHash, _ = auth.HashPassword("not-a-real-password")

func (s *Server) login(c *gin.Context) {
	if !s.limit(c, "login:"+clientIP(c), 20, 10*time.Minute) {
		return
	}
	var in credentials
	if c.ShouldBindJSON(&in) != nil || in.Username == "" || in.Password == "" {
		fail(c, http.StatusBadRequest, "Username and password required")
		return
	}
	if !s.limit(c, "login-user:"+strings.ToLower(in.Username), 10, 10*time.Minute) {
		return
	}
	var u models.User
	err := s.db.Where("username = ?", in.Username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		auth.CheckPassword(dummyHash, in.Password)
	} else if err != nil {
		fail(c, http.StatusInternalServerError, "Internal error")
		return
	}
	if err != nil || !auth.CheckPassword(u.PasswordHash, in.Password) {
		fail(c, http.StatusUnauthorized, "Wrong username or password")
		return
	}
	s.startSession(c, &u, http.StatusOK)
}

func (s *Server) logout(c *gin.Context) {
	if tok, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer "); ok {
		s.db.Where("token_hash = ?", auth.HashToken(tok)).Delete(&models.Session{})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
