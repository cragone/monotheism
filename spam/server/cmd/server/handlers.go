package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Server struct {
	db      *sql.DB
	limiter *limiter
	nonces  *nonceCache
}

type User struct {
	Pubkey      string `json:"pubkey"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	CreatedAt   int64  `json:"created_at"`
}

type Post struct {
	ID        int64  `json:"id"`
	ParentID  *int64 `json:"parent_id"`
	Body      string `json:"body"`
	CreatedAt int64  `json:"created_at"`
	Author    struct {
		Pubkey      string `json:"pubkey"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
	} `json:"author"`
	Likes   int  `json:"likes"`
	Replies int  `json:"replies"`
	Liked   bool `json:"liked"`
}

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{2,20}$`)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, id, msg string) {
	writeJSON(w, code, map[string]string{"error": id, "message": msg})
}

func (s *Server) routes(mux *http.ServeMux) {
	api := http.NewServeMux()
	api.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, map[string]bool{"ok": true}) })
	api.HandleFunc("GET /api/me", s.getMe)
	api.HandleFunc("PUT /api/me", s.putMe)
	api.HandleFunc("GET /api/feed", s.feed)
	api.HandleFunc("POST /api/posts", s.createPost)
	api.HandleFunc("GET /api/posts/{id}", s.getPost)
	api.HandleFunc("DELETE /api/posts/{id}", s.deletePost)
	api.HandleFunc("PUT /api/posts/{id}/like", s.like(true))
	api.HandleFunc("DELETE /api/posts/{id}/like", s.like(false))
	api.HandleFunc("GET /api/users/{username}", s.getUser)
	api.HandleFunc("PUT /api/users/{username}/follow", s.follow(true))
	api.HandleFunc("DELETE /api/users/{username}/follow", s.follow(false))
	mux.Handle("/api/", s.withAuth(api))
}

func (s *Server) userByKey(pub string) (*User, error) {
	var u User
	err := s.db.QueryRow(`SELECT pubkey,username,display_name,bio,created_at FROM users WHERE pubkey=?`, pub).
		Scan(&u.Pubkey, &u.Username, &u.DisplayName, &u.Bio, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &u, err
}

// requireUser: signed request from an account that has claimed a username.
func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) *User {
	k := authedKey(r)
	if k == "" {
		writeErr(w, 401, "auth_required", "sign the request with your account key")
		return nil
	}
	u, err := s.userByKey(k)
	if err != nil {
		writeErr(w, 500, "db", err.Error())
		return nil
	}
	if u == nil {
		writeErr(w, 403, "no_profile", "pick a username first")
		return nil
	}
	return u
}

func (s *Server) getMe(w http.ResponseWriter, r *http.Request) {
	k := authedKey(r)
	if k == "" {
		writeErr(w, 401, "auth_required", "sign the request with your account key")
		return
	}
	u, err := s.userByKey(k)
	if err != nil {
		writeErr(w, 500, "db", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"pubkey": k, "user": u})
}

// putMe creates (first call) or updates the profile. Claiming a username *is*
// the signup: the account already exists as a keypair.
func (s *Server) putMe(w http.ResponseWriter, r *http.Request) {
	k := authedKey(r)
	if k == "" {
		writeErr(w, 401, "auth_required", "sign the request with your account key")
		return
	}
	var in struct{ Username, DisplayName, Bio string }
	var raw map[string]string
	if json.NewDecoder(r.Body).Decode(&raw) != nil {
		writeErr(w, 400, "bad_json", "invalid json")
		return
	}
	in.Username, in.DisplayName, in.Bio = strings.TrimSpace(raw["username"]), strings.TrimSpace(raw["display_name"]), strings.TrimSpace(raw["bio"])
	if utf8.RuneCountInString(in.DisplayName) > 40 || utf8.RuneCountInString(in.Bio) > 160 {
		writeErr(w, 400, "too_long", "display name max 40, bio max 160")
		return
	}
	existing, err := s.userByKey(k)
	if err != nil {
		writeErr(w, 500, "db", err.Error())
		return
	}
	if existing == nil {
		if !usernameRe.MatchString(in.Username) {
			writeErr(w, 400, "bad_username", "2-20 letters, numbers or underscores")
			return
		}
		if in.DisplayName == "" {
			in.DisplayName = in.Username
		}
		_, err = s.db.Exec(`INSERT INTO users(pubkey,username,display_name,bio,created_at) VALUES(?,?,?,?,?)`,
			k, in.Username, in.DisplayName, in.Bio, time.Now().Unix())
		if err != nil {
			writeErr(w, 409, "username_taken", "that username is taken")
			return
		}
	} else {
		if in.DisplayName == "" {
			in.DisplayName = existing.Username
		}
		_, err = s.db.Exec(`UPDATE users SET display_name=?, bio=? WHERE pubkey=?`, in.DisplayName, in.Bio, k)
		if err != nil {
			writeErr(w, 500, "db", err.Error())
			return
		}
	}
	u, _ := s.userByKey(k)
	writeJSON(w, 200, map[string]any{"pubkey": k, "user": u})
}

const postSelect = `
SELECT p.id, p.parent_id, p.body, p.created_at, u.pubkey, u.username, u.display_name,
  (SELECT COUNT(*) FROM likes l WHERE l.post_id=p.id),
  (SELECT COUNT(*) FROM posts c WHERE c.parent_id=p.id),
  EXISTS(SELECT 1 FROM likes l WHERE l.post_id=p.id AND l.pubkey=?)
FROM posts p JOIN users u ON u.pubkey=p.author `

func scanPosts(rows *sql.Rows) ([]Post, error) {
	defer rows.Close()
	out := []Post{}
	for rows.Next() {
		var p Post
		var parent sql.NullInt64
		if err := rows.Scan(&p.ID, &parent, &p.Body, &p.CreatedAt, &p.Author.Pubkey, &p.Author.Username,
			&p.Author.DisplayName, &p.Likes, &p.Replies, &p.Liked); err != nil {
			return nil, err
		}
		if parent.Valid {
			p.ParentID = &parent.Int64
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Server) postByID(id int64, viewer string) (*Post, error) {
	rows, err := s.db.Query(postSelect+`WHERE p.id=?`, viewer, id)
	if err != nil {
		return nil, err
	}
	ps, err := scanPosts(rows)
	if err != nil || len(ps) == 0 {
		return nil, err
	}
	return &ps[0], nil
}

func intParam(r *http.Request, name string, def int64) int64 {
	if v, err := strconv.ParseInt(r.URL.Query().Get(name), 10, 64); err == nil && v > 0 {
		return v
	}
	return def
}

// feed: top-level posts, newest first. tab=following limits to followed accounts (+ self).
func (s *Server) feed(w http.ResponseWriter, r *http.Request) {
	viewer := authedKey(r)
	before := intParam(r, "before", 1<<62)
	q := postSelect + `WHERE p.parent_id IS NULL AND p.id < ? `
	args := []any{viewer, before}
	if r.URL.Query().Get("tab") == "following" {
		if viewer == "" {
			writeJSON(w, 200, []Post{})
			return
		}
		q += `AND (p.author=? OR p.author IN (SELECT followee FROM follows WHERE follower=?)) `
		args = append(args, viewer, viewer)
	}
	q += `ORDER BY p.id DESC LIMIT 30`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		writeErr(w, 500, "db", err.Error())
		return
	}
	ps, err := scanPosts(rows)
	if err != nil {
		writeErr(w, 500, "db", err.Error())
		return
	}
	writeJSON(w, 200, ps)
}

func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	var in struct {
		Body     string `json:"body"`
		ParentID *int64 `json:"parent_id"`
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeErr(w, 400, "bad_json", "invalid json")
		return
	}
	in.Body = strings.TrimSpace(in.Body)
	if in.Body == "" || utf8.RuneCountInString(in.Body) > 500 {
		writeErr(w, 400, "bad_body", "posts are 1-500 characters")
		return
	}
	if !s.limiter.allow(u.Pubkey, 12, time.Minute) {
		writeErr(w, 429, "slow_down", "easy on the spam")
		return
	}
	if in.ParentID != nil {
		var x int64
		if s.db.QueryRow(`SELECT id FROM posts WHERE id=?`, *in.ParentID).Scan(&x) != nil {
			writeErr(w, 404, "no_parent", "post you're replying to doesn't exist")
			return
		}
	}
	res, err := s.db.Exec(`INSERT INTO posts(author,parent_id,body,created_at) VALUES(?,?,?,?)`,
		u.Pubkey, in.ParentID, in.Body, time.Now().Unix())
	if err != nil {
		writeErr(w, 500, "db", err.Error())
		return
	}
	id, _ := res.LastInsertId()
	p, _ := s.postByID(id, u.Pubkey)
	writeJSON(w, 201, p)
}

// getPost returns the post, its ancestor chain, and direct+nested replies (flat, oldest first).
func (s *Server) getPost(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	viewer := authedKey(r)
	if err != nil {
		writeErr(w, 400, "bad_id", "bad id")
		return
	}
	p, err := s.postByID(id, viewer)
	if err != nil {
		writeErr(w, 500, "db", err.Error())
		return
	}
	if p == nil {
		writeErr(w, 404, "not_found", "no such post")
		return
	}
	ancestors := []Post{}
	for cur := p; cur.ParentID != nil && len(ancestors) < 50; {
		parent, err := s.postByID(*cur.ParentID, viewer)
		if err != nil || parent == nil {
			break
		}
		ancestors = append([]Post{*parent}, ancestors...)
		cur = parent
	}
	rows, err := s.db.Query(postSelect+`WHERE p.parent_id=? ORDER BY p.id ASC LIMIT 200`, viewer, id)
	if err != nil {
		writeErr(w, 500, "db", err.Error())
		return
	}
	replies, err := scanPosts(rows)
	if err != nil {
		writeErr(w, 500, "db", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"post": p, "ancestors": ancestors, "replies": replies})
}

func (s *Server) deletePost(w http.ResponseWriter, r *http.Request) {
	u := s.requireUser(w, r)
	if u == nil {
		return
	}
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	res, err := s.db.Exec(`DELETE FROM posts WHERE id=? AND author=?`, id, u.Pubkey)
	if err != nil {
		writeErr(w, 500, "db", err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeErr(w, 404, "not_found", "no such post of yours")
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (s *Server) like(on bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := s.requireUser(w, r)
		if u == nil {
			return
		}
		id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
		var err error
		if on {
			_, err = s.db.Exec(`INSERT OR IGNORE INTO likes(post_id,pubkey) VALUES(?,?)`, id, u.Pubkey)
		} else {
			_, err = s.db.Exec(`DELETE FROM likes WHERE post_id=? AND pubkey=?`, id, u.Pubkey)
		}
		if err != nil {
			writeErr(w, 404, "not_found", "no such post")
			return
		}
		p, _ := s.postByID(id, u.Pubkey)
		writeJSON(w, 200, p)
	}
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request) {
	viewer := authedKey(r)
	var u User
	err := s.db.QueryRow(`SELECT pubkey,username,display_name,bio,created_at FROM users WHERE username=?`, r.PathValue("username")).
		Scan(&u.Pubkey, &u.Username, &u.DisplayName, &u.Bio, &u.CreatedAt)
	if err != nil {
		writeErr(w, 404, "not_found", "no such user")
		return
	}
	var followers, following, isFollowing int
	s.db.QueryRow(`SELECT COUNT(*) FROM follows WHERE followee=?`, u.Pubkey).Scan(&followers)
	s.db.QueryRow(`SELECT COUNT(*) FROM follows WHERE follower=?`, u.Pubkey).Scan(&following)
	s.db.QueryRow(`SELECT COUNT(*) FROM follows WHERE follower=? AND followee=?`, viewer, u.Pubkey).Scan(&isFollowing)
	before := intParam(r, "before", 1<<62)
	cond := `p.parent_id IS NULL`
	if r.URL.Query().Get("tab") == "replies" {
		cond = `p.parent_id IS NOT NULL`
	}
	rows, err := s.db.Query(postSelect+`WHERE p.author=? AND `+cond+` AND p.id < ? ORDER BY p.id DESC LIMIT 30`, viewer, u.Pubkey, before)
	if err != nil {
		writeErr(w, 500, "db", err.Error())
		return
	}
	ps, err := scanPosts(rows)
	if err != nil {
		writeErr(w, 500, "db", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{
		"user": u, "followers": followers, "following": following,
		"is_following": isFollowing > 0, "is_me": viewer == u.Pubkey, "posts": ps,
	})
}

func (s *Server) follow(on bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := s.requireUser(w, r)
		if u == nil {
			return
		}
		var target string
		if s.db.QueryRow(`SELECT pubkey FROM users WHERE username=?`, r.PathValue("username")).Scan(&target) != nil {
			writeErr(w, 404, "not_found", "no such user")
			return
		}
		if target == u.Pubkey {
			writeErr(w, 400, "self", "can't follow yourself")
			return
		}
		if on {
			s.db.Exec(`INSERT OR IGNORE INTO follows(follower,followee) VALUES(?,?)`, u.Pubkey, target)
		} else {
			s.db.Exec(`DELETE FROM follows WHERE follower=? AND followee=?`, u.Pubkey, target)
		}
		writeJSON(w, 200, map[string]bool{"is_following": on})
	}
}
