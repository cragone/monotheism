package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/cragone/monotheism/spam/server/internal/models"
)

const (
	pageSize     = 20
	maxTitle     = 200
	maxThreadBod = 10000
	maxReply     = 5000
	previewLen   = 280
)

type threadDTO struct {
	ID           uint    `json:"id"`
	Title        string  `json:"title"`
	Body         string  `json:"body"`
	Truncated    bool    `json:"truncated"`
	Author       *string `json:"author"`
	Mine         bool    `json:"mine"`
	CreatedAt    int64   `json:"created_at"`
	LastActivity int64   `json:"last_activity"`
	Likes        int     `json:"likes"`
	ReplyCount   int     `json:"reply_count"`
	Liked        bool    `json:"liked"`
	Deleted      bool    `json:"deleted"`
}

type replyDTO struct {
	ID        uint    `json:"id"`
	ParentID  *uint   `json:"parent_id"`
	Body      string  `json:"body"`
	Author    *string `json:"author"`
	Mine      bool    `json:"mine"`
	CreatedAt int64   `json:"created_at"`
	Likes     int     `json:"likes"`
	Liked     bool    `json:"liked"`
	Deleted   bool    `json:"deleted"`
}

type threadRow struct {
	ID           uint
	Title        string
	Body         string
	CreatedAt    time.Time
	LastActivity time.Time
	Deleted      bool
	Author       string
	AuthorID     uint
	Likes        int
	ReplyCount   int
	Liked        bool
}

type replyRow struct {
	ID        uint
	ParentID  *uint
	Body      string
	CreatedAt time.Time
	Deleted   bool
	Author    string
	AuthorID  uint
	Likes     int
	Liked     bool
}

func uid(c *gin.Context) uint {
	if u := currentUser(c); u != nil {
		return u.ID
	}
	return 0
}

func (s *Server) threadQuery(me uint) *gorm.DB {
	return s.db.Table("threads t").
		Select(`t.id, t.title, t.body, t.created_at, t.last_activity, t.deleted,
			u.username AS author, t.user_id AS author_id,
			(SELECT COUNT(*) FROM likes l WHERE l.kind = 'thread' AND l.target_id = t.id) AS likes,
			(SELECT COUNT(*) FROM replies r WHERE r.thread_id = t.id AND r.deleted = 0) AS reply_count,
			EXISTS (SELECT 1 FROM likes l WHERE l.kind = 'thread' AND l.target_id = t.id AND l.user_id = ?) AS liked`, me).
		Joins("JOIN users u ON u.id = t.user_id")
}

func toThreadDTO(r threadRow, me uint, full bool) threadDTO {
	d := threadDTO{
		ID: r.ID, CreatedAt: r.CreatedAt.UnixMilli(), LastActivity: r.LastActivity.UnixMilli(),
		Likes: r.Likes, ReplyCount: r.ReplyCount, Liked: r.Liked, Deleted: r.Deleted,
	}
	if r.Deleted {
		d.Title = "[deleted]"
		return d
	}
	d.Title, d.Author, d.Mine = r.Title, &r.Author, me != 0 && r.AuthorID == me
	d.Body = r.Body
	if !full {
		if runes := []rune(r.Body); len(runes) > previewLen {
			d.Body, d.Truncated = string(runes[:previewLen]), true
		}
	}
	return d
}

func cleanText(v, name string, max int, required bool) (string, string) {
	v = strings.TrimSpace(strings.ReplaceAll(v, "\r\n", "\n"))
	if required && v == "" {
		return "", name + " can't be empty"
	}
	if utf8.RuneCountInString(v) > max {
		return "", name + " is too long (max " + strconv.Itoa(max) + " characters)"
	}
	return v, ""
}

func (s *Server) listThreads(c *gin.Context) {
	order, ok := map[string]string{
		"":       "t.last_activity DESC",
		"active": "t.last_activity DESC",
		"new":    "t.created_at DESC",
		"top":    "likes DESC, reply_count DESC, t.created_at DESC",
	}[c.Query("sort")]
	if !ok {
		fail(c, http.StatusBadRequest, "sort must be new, active or top")
		return
	}
	page, _ := strconv.Atoi(c.Query("page"))
	if page < 0 {
		page = 0
	}
	me := uid(c)
	var rows []threadRow
	if err := s.threadQuery(me).Where("t.deleted = 0").Order(order).
		Limit(pageSize + 1).Offset(page * pageSize).Scan(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, "Internal error")
		return
	}
	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}
	out := make([]threadDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, toThreadDTO(r, me, false))
	}
	c.JSON(http.StatusOK, gin.H{"threads": out, "has_more": hasMore})
}

type threadInput struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (s *Server) createThread(c *gin.Context) {
	u := currentUser(c)
	if !s.limit(c, "post:"+strconv.Itoa(int(u.ID)), 10, 10*time.Minute) {
		return
	}
	var in threadInput
	if c.ShouldBindJSON(&in) != nil {
		fail(c, http.StatusBadRequest, "Invalid JSON")
		return
	}
	title, msg := cleanText(in.Title, "Title", maxTitle, true)
	if msg != "" {
		fail(c, http.StatusBadRequest, msg)
		return
	}
	body, msg := cleanText(in.Body, "Body", maxThreadBod, false)
	if msg != "" {
		fail(c, http.StatusBadRequest, msg)
		return
	}
	now := time.Now()
	t := models.Thread{UserID: u.ID, Title: title, Body: body, CreatedAt: now, LastActivity: now}
	if err := s.db.Create(&t).Error; err != nil {
		fail(c, http.StatusInternalServerError, "Internal error")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": t.ID})
}

func (s *Server) getThread(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusNotFound, "Thread not found")
		return
	}
	me := uid(c)
	var rows []threadRow
	if err := s.threadQuery(me).Where("t.id = ?", id).Scan(&rows).Error; err != nil {
		fail(c, http.StatusInternalServerError, "Internal error")
		return
	}
	if len(rows) == 0 {
		fail(c, http.StatusNotFound, "Thread not found")
		return
	}
	var rr []replyRow
	if err := s.db.Table("replies r").
		Select(`r.id, r.parent_id, r.body, r.created_at, r.deleted, u.username AS author, r.user_id AS author_id,
			(SELECT COUNT(*) FROM likes l WHERE l.kind = 'reply' AND l.target_id = r.id) AS likes,
			EXISTS (SELECT 1 FROM likes l WHERE l.kind = 'reply' AND l.target_id = r.id AND l.user_id = ?) AS liked`, me).
		Joins("JOIN users u ON u.id = r.user_id").
		Where("r.thread_id = ?", id).Order("r.created_at ASC, r.id ASC").Scan(&rr).Error; err != nil {
		fail(c, http.StatusInternalServerError, "Internal error")
		return
	}
	replies := make([]replyDTO, 0, len(rr))
	for _, r := range rr {
		d := replyDTO{ID: r.ID, ParentID: r.ParentID, CreatedAt: r.CreatedAt.UnixMilli(), Likes: r.Likes, Liked: r.Liked, Deleted: r.Deleted}
		if !r.Deleted {
			d.Body, d.Author, d.Mine = r.Body, &r.Author, me != 0 && r.AuthorID == me
		}
		replies = append(replies, d)
	}
	c.JSON(http.StatusOK, gin.H{"thread": toThreadDTO(rows[0], me, true), "replies": replies})
}

type replyInput struct {
	Body     string `json:"body"`
	ParentID *uint  `json:"parent_id"`
}

func (s *Server) createReply(c *gin.Context) {
	u := currentUser(c)
	if !s.limit(c, "reply:"+strconv.Itoa(int(u.ID)), 30, 10*time.Minute) {
		return
	}
	threadID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	var in replyInput
	if err != nil || c.ShouldBindJSON(&in) != nil {
		fail(c, http.StatusBadRequest, "Invalid request")
		return
	}
	body, msg := cleanText(in.Body, "Reply", maxReply, true)
	if msg != "" {
		fail(c, http.StatusBadRequest, msg)
		return
	}
	var th models.Thread
	if err := s.db.Where("id = ? AND deleted = 0", threadID).First(&th).Error; err != nil {
		fail(c, http.StatusNotFound, "Thread not found")
		return
	}
	if in.ParentID != nil {
		var p models.Reply
		if err := s.db.Where("id = ? AND thread_id = ?", *in.ParentID, threadID).First(&p).Error; err != nil {
			fail(c, http.StatusBadRequest, "Parent reply not found in this thread")
			return
		}
	}
	now := time.Now()
	rep := models.Reply{ThreadID: uint(threadID), ParentID: in.ParentID, UserID: u.ID, Body: body, CreatedAt: now}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&rep).Error; err != nil {
			return err
		}
		return tx.Model(&models.Thread{}).Where("id = ?", threadID).Update("last_activity", now).Error
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "Internal error")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": rep.ID})
}

func (s *Server) toggleLike(c *gin.Context, kind, table string) {
	u := currentUser(c)
	if !s.limit(c, "like:"+strconv.Itoa(int(u.ID)), 120, 10*time.Minute) {
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusNotFound, "Not found")
		return
	}
	var n int64
	s.db.Table(table).Where("id = ? AND deleted = 0", id).Count(&n)
	if n == 0 {
		fail(c, http.StatusNotFound, "Not found")
		return
	}
	like := models.Like{UserID: u.ID, Kind: kind, TargetID: uint(id)}
	liked := false
	err = s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Delete(&like)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			liked = true
			return tx.Create(&like).Error
		}
		return nil
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "Internal error")
		return
	}
	var total int64
	s.db.Model(&models.Like{}).Where("kind = ? AND target_id = ?", kind, id).Count(&total)
	c.JSON(http.StatusOK, gin.H{"liked": liked, "likes": total})
}

func (s *Server) likeThread(c *gin.Context) { s.toggleLike(c, models.KindThread, "threads") }
func (s *Server) likeReply(c *gin.Context)  { s.toggleLike(c, models.KindReply, "replies") }

func (s *Server) softDelete(c *gin.Context, table string) {
	u := currentUser(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		fail(c, http.StatusNotFound, "Not found, or not yours")
		return
	}
	res := s.db.Table(table).Where("id = ? AND user_id = ?", id, u.ID).Update("deleted", true)
	if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
		fail(c, http.StatusInternalServerError, "Internal error")
		return
	}
	if res.RowsAffected == 0 {
		fail(c, http.StatusNotFound, "Not found, or not yours")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) deleteThread(c *gin.Context) { s.softDelete(c, "threads") }
func (s *Server) deleteReply(c *gin.Context)  { s.softDelete(c, "replies") }
