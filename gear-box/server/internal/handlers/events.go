package handlers

import (
	"io"

	"github.com/gin-gonic/gin"
)

// Subscriber is the subset of sse.Hub the handlers need, defined here so
// tests can supply a mock instead of a real hub.
type Subscriber interface {
	Subscribe() (ch chan []byte, unsubscribe func())
}

// Hub is the full set of sse.Hub methods the handlers package needs across
// both Hear (Broadcaster) and HearEvents (Subscriber). router.New wires a
// single *sse.Hub value in as this combined type, satisfying both narrower
// interfaces at once.
type Hub interface {
	Broadcaster
	Subscriber
}

// EventsHandlers streams heard-utterance events to browser tabs.
type EventsHandlers struct {
	hub Subscriber
}

func NewEventsHandlers(hub Subscriber) *EventsHandlers {
	return &EventsHandlers{hub: hub}
}

// HearEvents upgrades to text/event-stream and streams each subsequent
// heard event to this client until it disconnects. No replay of anything
// that happened before this connection opened -- see internal/sse's
// package doc for why that's deliberate, not a gap.
func (h *EventsHandlers) HearEvents(c *gin.Context) {
	ch, unsubscribe := h.hub.Subscribe()
	defer unsubscribe()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-ch:
			if !ok {
				return false
			}
			c.SSEvent("heard", msg)
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}
