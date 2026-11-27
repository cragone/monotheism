// spam — a Threads-style social app. Accounts follow the super-chat model:
// no signup/login; a 32-byte seed lives on the user's device (24-word
// recovery phrase), derives an Ed25519 key, and every write is signed.
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func main() {
	dataDir := env("DATA_DIR", "./data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatal(err)
	}
	db, err := openDB(filepath.Join(dataDir, "spam.sqlite"))
	if err != nil {
		log.Fatal(err)
	}
	s := &Server{db: db, limiter: newLimiter(), nonces: newNonceCache()}

	mux := http.NewServeMux()
	s.routes(mux)

	static := env("STATIC_DIR", "./web/dist")
	fs := http.FileServer(http.Dir(static))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Join(static, filepath.Clean("/"+r.URL.Path))
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fs.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(static, "index.html")) // SPA fallback
	})

	addr := ":" + env("PORT", "8080")
	log.Printf("spam listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
