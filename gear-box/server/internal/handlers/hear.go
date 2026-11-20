package handlers

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

// MaxHearBytes mirrors piper.MaxOutputBytes -- the same 20s-of-audio
// sanity cap on the speaker side, applied here to the mic's upload.
const MaxHearBytes = 882000

// HearHandlers receives raw PCM audio pushed by the mic_listener ESP32
// and pushes it straight to the sound_machine ESP32 to play back. This is
// a temporary echo, not understanding: it proves the round trip (mic ->
// gear-box -> speaker) works, ahead of swapping this handler for
// something that actually responds to what it heard instead of just
// replaying it.
type HearHandlers struct {
	esp32 Pusher
	token string
}

func NewHearHandlers(pusher Pusher, token string) *HearHandlers {
	return &HearHandlers{esp32: pusher, token: token}
}

func (h *HearHandlers) Hear(c *gin.Context) {
	if h.token != "" && c.GetHeader("X-Token") != h.token {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxHearBytes)
	pcm, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "audio too large"})
		return
	}
	if len(pcm) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "empty body"})
		return
	}

	if err := h.esp32.Speak(c.Request.Context(), pcm); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach sound machine"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
