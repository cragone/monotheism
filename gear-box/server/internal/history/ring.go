package history

import (
	"sync"
	"time"
)

// Entry is one previously-spoken line.
type Entry struct {
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

// Ring is a small, fixed-size, in-memory history of recently spoken lines.
// It is intentionally not persisted: a restart losing "what did I just say
// to the sound machine" is an acceptable trade for not needing a database
// or a PVC for this service.
type Ring struct {
	mu      sync.Mutex
	entries []Entry
	cap     int
}

func NewRing(capacity int) *Ring {
	return &Ring{entries: make([]Entry, 0, capacity), cap: capacity}
}

// Add records a new entry, evicting the oldest if the ring is full.
func (r *Ring) Add(text string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = append(r.entries, Entry{Text: text, At: time.Now()})
	if len(r.entries) > r.cap {
		r.entries = r.entries[len(r.entries)-r.cap:]
	}
}

// List returns the entries newest-first.
func (r *Ring) List() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]Entry, len(r.entries))
	for i, e := range r.entries {
		out[len(r.entries)-1-i] = e
	}
	return out
}
