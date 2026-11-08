package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/cragone/monotheism/spam/server/internal/config"
	"github.com/cragone/monotheism/spam/server/internal/db"
)

type env struct {
	t   *testing.T
	srv http.Handler
}

func newEnv(t *testing.T) *env {
	t.Helper()
	gin.SetMode(gin.TestMode)
	web := t.TempDir()
	os.WriteFile(filepath.Join(web, "index.html"), []byte("<title>spam</title>"), 0o644)
	os.WriteFile(filepath.Join(web, "asset.txt"), []byte("nope"), 0o644)
	g, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &env{t, New(g, config.Config{WebDir: filepath.Join(web)})}
}

func (e *env) call(method, path, token string, body any) (int, map[string]any) {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.srv.ServeHTTP(w, req)
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func (e *env) want(got, want int, ctx string) {
	e.t.Helper()
	if got != want {
		e.t.Fatalf("%s: status %d, want %d", ctx, got, want)
	}
}

func (e *env) register(name string) string {
	code, out := e.call("POST", "/api/register", "", map[string]string{"username": name, "password": "password123"})
	e.want(code, 201, "register "+name)
	return out["token"].(string)
}

func TestThreadFlow(t *testing.T) {
	e := newEnv(t)
	alice, bob := e.register("alice"), e.register("bob")

	code, _ := e.call("POST", "/api/register", "", map[string]string{"username": "ALICE", "password": "password123"})
	e.want(code, 409, "duplicate username (case-insensitive)")
	code, _ = e.call("POST", "/api/threads", "", map[string]string{"title": "x"})
	e.want(code, 401, "anonymous post")

	code, out := e.call("POST", "/api/threads", alice, map[string]string{"title": "Hello", "body": "first!"})
	e.want(code, 201, "create thread")
	id := int(out["id"].(float64))
	tp := "/api/threads/" + itoa(id)

	code, r1 := e.call("POST", tp+"/replies", bob, map[string]any{"body": "top"})
	e.want(code, 201, "reply")
	code, _ = e.call("POST", tp+"/replies", alice, map[string]any{"body": "nested", "parent_id": r1["id"]})
	e.want(code, 201, "nested reply")
	code, _ = e.call("POST", tp+"/replies", alice, map[string]any{"body": "x", "parent_id": 9999})
	e.want(code, 400, "bad parent")

	_, like := e.call("POST", tp+"/like", bob, nil)
	if like["liked"] != true || like["likes"] != float64(1) {
		t.Fatalf("like: %v", like)
	}
	_, like = e.call("POST", tp+"/like", bob, nil)
	if like["liked"] != false || like["likes"] != float64(0) {
		t.Fatalf("unlike: %v", like)
	}

	_, view := e.call("GET", tp, alice, nil)
	replies := view["replies"].([]any)
	if len(replies) != 2 || replies[1].(map[string]any)["parent_id"] != r1["id"] {
		t.Fatalf("replies: %v", replies)
	}
	if view["thread"].(map[string]any)["mine"] != true {
		t.Fatal("thread should be mine for author")
	}

	_, feed := e.call("GET", "/api/threads?sort=top", "", nil)
	if feed["threads"].([]any)[0].(map[string]any)["reply_count"] != float64(2) {
		t.Fatalf("feed: %v", feed)
	}
	code, _ = e.call("GET", "/api/threads?sort=bogus", "", nil)
	e.want(code, 400, "bad sort")

	rp := "/api/replies/" + itoa(int(r1["id"].(float64)))
	code, _ = e.call("DELETE", rp, alice, nil)
	e.want(code, 404, "delete other's reply")
	code, _ = e.call("DELETE", rp, bob, nil)
	e.want(code, 200, "delete own reply")
	_, view = e.call("GET", tp, "", nil)
	first := view["replies"].([]any)[0].(map[string]any)
	if first["deleted"] != true || first["author"] != nil || first["body"] != "" {
		t.Fatalf("deleted reply should be masked: %v", first)
	}

	code, _ = e.call("DELETE", tp, alice, nil)
	e.want(code, 200, "delete thread")
	_, feed = e.call("GET", "/api/threads", "", nil)
	if len(feed["threads"].([]any)) != 0 {
		t.Fatal("deleted thread still in feed")
	}
}

func TestAuth(t *testing.T) {
	e := newEnv(t)
	tok := e.register("alice")
	for _, bad := range []map[string]string{
		{"username": "a b", "password": "password123"},
		{"username": "shorty", "password": "short"},
	} {
		code, _ := e.call("POST", "/api/register", "", bad)
		e.want(code, 400, "validation")
	}
	code, _ := e.call("POST", "/api/login", "", map[string]string{"username": "alice", "password": "wrong-pass"})
	e.want(code, 401, "bad password")
	code, _ = e.call("POST", "/api/login", "", map[string]string{"username": "nobody", "password": "password123"})
	e.want(code, 401, "unknown user")
	code, out := e.call("POST", "/api/login", "", map[string]string{"username": "alice", "password": "password123"})
	e.want(code, 200, "login")
	tok2 := out["token"].(string)

	_, me := e.call("GET", "/api/me", tok2, nil)
	if me["user"].(map[string]any)["username"] != "alice" {
		t.Fatalf("me: %v", me)
	}
	e.call("POST", "/api/logout", tok2, nil)
	_, me = e.call("GET", "/api/me", tok2, nil)
	if me["user"] != nil {
		t.Fatal("token should be revoked after logout")
	}
	_, me = e.call("GET", "/api/me", tok, nil)
	if me["user"] == nil {
		t.Fatal("other session should survive")
	}
}

func TestStatic(t *testing.T) {
	e := newEnv(t)
	for path, want := range map[string]int{
		"/": 200, "/t/1": 200, "/missing.js": 404, "/asset.txt": 200, "/api/nope": 404,
	} {
		code, _ := e.call("GET", path, "", nil)
		if code != want {
			t.Errorf("GET %s = %d, want %d", path, code, want)
		}
	}
}

// Traversal attempts must never be served (they may be rejected or fall back, but not leak files).
func TestStaticTraversal(t *testing.T) {
	e := newEnv(t)
	for _, path := range []string{"/../../etc/passwd", "/%2e%2e/%2e%2e/etc/passwd", "/..%2f..%2fetc/passwd"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		e.srv.ServeHTTP(w, req)
		if bytes.Contains(w.Body.Bytes(), []byte("root:")) {
			t.Errorf("GET %s leaked /etc/passwd", path)
		}
	}
}

func itoa(i int) string { b, _ := json.Marshal(i); return string(b) }
