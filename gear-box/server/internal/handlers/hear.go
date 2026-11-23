package handlers

import (
	"context"
	"encoding/binary"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/sse"
)

// MaxHearBytes mirrors piper.MaxOutputBytes -- the same 20s-of-audio
// sanity cap on the speaker side, applied here to the mic's upload.
const MaxHearBytes = 882000

// transcribeTimeout bounds the background whisper.cpp subprocess call
// kicked off after each utterance. It's independent of mic_listener's own
// 15s HTTP timeout (see Hear) -- just a ceiling so a stuck subprocess
// can't accumulate indefinitely across repeated utterances.
const transcribeTimeout = 20 * time.Second

// waveformPoints is the fixed size of the downsampled amplitude envelope
// sent to the frontend for visualization -- small enough to keep the SSE
// payload light, the browser never needs the full PCM.
const waveformPoints = 150

// Transcriber is the subset of whisper.Transcriber the handlers need,
// defined here so tests can supply a mock instead of shelling out to the
// real whisper-cli binary.
type Transcriber interface {
	Transcribe(ctx context.Context, pcm []byte) (string, error)
}

// Broadcaster fans out a heard utterance's transcript + waveform to any
// currently-connected SSE clients. Broadcast must never block on a
// slow/absent client -- see sse.Hub, the real implementation.
type Broadcaster interface {
	Broadcast(event sse.HeardEvent)
}

// HearHandlers receives raw PCM audio pushed by the mic_listener ESP32,
// pushes it straight to the sound_machine ESP32 to play back, and -- once
// that echo has succeeded -- transcribes it in the background and
// broadcasts the result to any browser tabs listening on /api/hear-events.
type HearHandlers struct {
	esp32       Pusher
	transcriber Transcriber
	broadcaster Broadcaster
	token       string
}

func NewHearHandlers(pusher Pusher, transcriber Transcriber, broadcaster Broadcaster, token string) *HearHandlers {
	return &HearHandlers{esp32: pusher, transcriber: transcriber, broadcaster: broadcaster, token: token}
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

	// Fire-and-forget, and deliberately independent of whether the echo
	// below succeeds: the mic captured real speech either way, and
	// whether sound_machine happens to be reachable right now shouldn't
	// decide whether that speech gets transcribed. mic_listener's HTTP
	// client times out at 15s, and an 8s clip transcribed on CPU isn't
	// guaranteed to fit comfortably inside that (especially with Piper
	// synthesis possibly running concurrently on the same box) -- a
	// transcription failure here is logged, not surfaced to the mic.
	go h.transcribeAndBroadcast(pcm)

	if err := h.esp32.Speak(c.Request.Context(), pcm); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach sound machine"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// transcribeAndBroadcast is a method (not an inline goroutine closure) so
// tests can call it directly and synchronously instead of racing a `go`
// statement.
func (h *HearHandlers) transcribeAndBroadcast(pcm []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), transcribeTimeout)
	defer cancel()

	text, err := h.transcriber.Transcribe(ctx, pcm)
	if err != nil {
		log.Printf("warn: transcription failed: %v", err)
		return
	}

	h.broadcaster.Broadcast(sse.HeardEvent{
		Transcript: text,
		Waveform:   downsampleWaveform(pcm, waveformPoints),
		Timestamp:  time.Now(),
	})
}

// downsampleWaveform buckets mono 16-bit signed little-endian PCM into a
// fixed number of points, taking each bucket's peak absolute amplitude --
// a lightweight envelope suitable for a simple client-side plot, computed
// server-side so the browser never needs the full PCM.
func downsampleWaveform(pcm []byte, points int) []int16 {
	n := len(pcm) / 2
	if n == 0 {
		return []int16{}
	}
	if points > n {
		points = n
	}

	out := make([]int16, points)
	bucketSize := n / points
	for i := range points {
		start := i * bucketSize
		end := start + bucketSize
		if i == points-1 {
			end = n // last bucket absorbs any remainder
		}

		var peak int16
		var peakAbs int32
		for s := start; s < end; s++ {
			v := int16(binary.LittleEndian.Uint16(pcm[s*2 : s*2+2]))
			if vAbs := abs32(int32(v)); vAbs > peakAbs {
				peak, peakAbs = v, vAbs
			}
		}
		out[i] = peak
	}
	return out
}

func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
