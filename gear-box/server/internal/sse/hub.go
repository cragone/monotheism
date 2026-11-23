// Package sse fans out heard-utterance events to connected browser tabs.
//
// Deliberately no history: a client that subscribes after an event was
// broadcast simply never sees it. This is not an oversight -- it mirrors
// gear-box's existing "retain nothing" design (see GEAR-BOX.md and the
// removal of the old "recently spoken" history panel, commit 4a12b3e).
// This package is not a revival of that removed internal/history/ring.go
// pattern: there is no ring buffer here, by design.
package sse

import (
	"encoding/json"
	"sync"
	"time"
)

// HeardEvent is what gets pushed to connected browsers each time gear-box
// transcribes a mic_listener utterance. Never retained after Broadcast
// returns -- see package doc.
type HeardEvent struct {
	Transcript string    `json:"transcript"`
	Waveform   []int16   `json:"waveform"`
	Timestamp  time.Time `json:"timestamp"`
}

// Hub fans out HeardEvents to any currently-subscribed channels.
type Hub struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{subs: make(map[chan []byte]struct{})}
}

// Subscribe registers a new client channel. Call the returned unsubscribe
// func (typically deferred) when the client disconnects.
func (h *Hub) Subscribe() (ch chan []byte, unsubscribe func()) {
	// Buffered by 1: a slow/absent reader never blocks Broadcast (see
	// Broadcast's non-blocking send) -- this buffer just absorbs the one
	// in-flight message rather than dropping it immediately.
	ch = make(chan []byte, 1)

	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()

	return ch, func() {
		h.mu.Lock()
		delete(h.subs, ch)
		h.mu.Unlock()
	}
}

// Broadcast marshals event to JSON and sends it to every currently
// subscribed channel, non-blocking: a slow/full client's channel is
// skipped for this message rather than buffered, retried, or allowed to
// stall the broadcast to everyone else.
func (h *Hub) Broadcast(event HeardEvent) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs {
		select {
		case ch <- data:
		default:
		}
	}
}
