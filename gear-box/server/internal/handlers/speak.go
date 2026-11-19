package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/piper"
)

// Synthesizer is the subset of piper.Synthesizer the handlers need, defined
// here so tests can supply a mock instead of shelling out to the real
// piper binary.
type Synthesizer interface {
	Synthesize(ctx context.Context, text string) ([]byte, error)
}

// Pusher is the subset of esp32.Client the handlers need, defined here so
// tests can supply a mock instead of hitting the real device over the LAN.
type Pusher interface {
	Speak(ctx context.Context, pcm []byte) error
}

type SpeakHandlers struct {
	piper Synthesizer
	esp32 Pusher
}

func NewSpeakHandlers(synth Synthesizer, pusher Pusher) *SpeakHandlers {
	return &SpeakHandlers{piper: synth, esp32: pusher}
}

type speakRequest struct {
	Text string `json:"text" binding:"required"`
}

func (h *SpeakHandlers) Speak(c *gin.Context) {
	var req speakRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if len(req.Text) > piper.MaxTextChars {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text too long"})
		return
	}

	pcm, err := h.piper.Synthesize(c.Request.Context(), req.Text)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "tts synthesis failed"})
		return
	}

	if err := h.esp32.Speak(c.Request.Context(), pcm); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach sound machine"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
