package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Request signing (super-chat style, no sessions/passwords):
//   X-Spam-Key: hex ed25519 public key
//   X-Spam-Ts:  unix seconds
//   X-Spam-Nonce: random hex, unique per request
//   X-Spam-Sig: hex ed25519 signature over
//               METHOD "\n" PATH?QUERY "\n" TS "\n" NONCE "\n" hex(sha256(body))
const maxSkew = 5 * time.Minute

type ctxKey struct{}

// authedKey returns the verified pubkey on the request, or "".
func authedKey(r *http.Request) string {
	v, _ := r.Context().Value(ctxKey{}).(string)
	return v
}

// withAuth verifies a signature if one is present. Requests with a bad
// signature are rejected; unsigned requests pass through as anonymous.
func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		keyHex := r.Header.Get("X-Spam-Key")
		if keyHex == "" {
			next.ServeHTTP(w, r)
			return
		}
		pub, err := hex.DecodeString(keyHex)
		sig, err2 := hex.DecodeString(r.Header.Get("X-Spam-Sig"))
		ts, err3 := strconv.ParseInt(r.Header.Get("X-Spam-Ts"), 10, 64)
		if err != nil || err2 != nil || err3 != nil || len(pub) != ed25519.PublicKeySize || len(sig) != ed25519.SignatureSize {
			writeErr(w, 401, "bad_auth", "malformed auth headers")
			return
		}
		if d := time.Since(time.Unix(ts, 0)); d > maxSkew || d < -maxSkew {
			writeErr(w, 401, "stale", "request timestamp out of range")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
		if err != nil {
			writeErr(w, 413, "too_large", "body too large")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		h := sha256.Sum256(body)
		msg := r.Method + "\n" + r.URL.RequestURI() + "\n" + r.Header.Get("X-Spam-Ts") + "\n" + r.Header.Get("X-Spam-Nonce") + "\n" + hex.EncodeToString(h[:])
		if !ed25519.Verify(pub, []byte(msg), sig) {
			writeErr(w, 401, "bad_sig", "signature check failed")
			return
		}
		if !s.nonces.firstUse(r.Header.Get("X-Spam-Sig")) {
			writeErr(w, 401, "replay", "signature already used")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, keyHex)))
	})
}

type nonceCache struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

func newNonceCache() *nonceCache { return &nonceCache{seen: map[string]time.Time{}} }

func (n *nonceCache) firstUse(sig string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	now := time.Now()
	if len(n.seen) > 5000 {
		for k, t := range n.seen {
			if now.Sub(t) > 2*maxSkew {
				delete(n.seen, k)
			}
		}
	}
	if _, ok := n.seen[sig]; ok {
		return false
	}
	n.seen[sig] = now
	return true
}

// limiter: simple per-key sliding window for writes.
type limiter struct {
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newLimiter() *limiter { return &limiter{hits: map[string][]time.Time{}} }

func (l *limiter) allow(key string, n int, per time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	h := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < per {
			h = append(h, t)
		}
	}
	if len(h) >= n {
		l.hits[key] = h
		return false
	}
	l.hits[key] = append(h, now)
	return true
}
