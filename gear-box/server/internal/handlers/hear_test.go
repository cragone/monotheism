package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/sse"
)

type mockTranscriber struct {
	text string
	err  error
	got  [][]byte
}

func (m *mockTranscriber) Transcribe(_ context.Context, pcm []byte) (string, error) {
	m.got = append(m.got, pcm)
	return m.text, m.err
}

// mockBroadcaster is safe for concurrent use: Hear's transcribeAndBroadcast
// runs in its own goroutine (see hear.go), so tests that exercise it
// through Hear (rather than calling it directly) read these fields from a
// different goroutine than the one writing them.
type mockBroadcaster struct {
	mu     sync.Mutex
	events []sse.HeardEvent
}

func (m *mockBroadcaster) Broadcast(event sse.HeardEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
}

func (m *mockBroadcaster) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

func setupHear(t *testing.T, token string) (*gin.Engine, *mockPusher, *mockTranscriber, *mockBroadcaster, *HearHandlers) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	pusher := &mockPusher{}
	transcriber := &mockTranscriber{text: "hello"}
	broadcaster := &mockBroadcaster{}
	h := NewHearHandlers(pusher, transcriber, broadcaster, token)

	engine := gin.New()
	engine.POST("/hear", h.Hear)

	return engine, pusher, transcriber, broadcaster, h
}

func doRawRequest(engine *gin.Engine, body []byte, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/hear", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	if token != "" {
		req.Header.Set("X-Token", token)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestHearPushesToESP32(t *testing.T) {
	engine, pusher, _, _, _ := setupHear(t, "")

	rec := doRawRequest(engine, []byte{0x01, 0x02, 0x03}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(pusher.pushed) != 1 || !bytes.Equal(pusher.pushed[0], []byte{0x01, 0x02, 0x03}) {
		t.Fatalf("expected one Speak push with the uploaded bytes, got %+v", pusher.pushed)
	}
}

func TestHearRejectsEmptyBody(t *testing.T) {
	engine, _, _, _, _ := setupHear(t, "")

	rec := doRawRequest(engine, []byte{}, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHearRejectsOversizedBody(t *testing.T) {
	engine, _, _, _, _ := setupHear(t, "")

	rec := doRawRequest(engine, make([]byte, MaxHearBytes+1), "")
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for oversized body, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHearRequiresTokenWhenConfigured(t *testing.T) {
	engine, _, _, _, _ := setupHear(t, "secret")

	rec := doRawRequest(engine, []byte{0x01}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with no token, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doRawRequest(engine, []byte{0x01}, "wrong")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong token, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doRawRequest(engine, []byte{0x01}, "secret")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with correct token, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHearReturnsBadGatewayWhenPushFails(t *testing.T) {
	engine, pusher, _, _, _ := setupHear(t, "")
	pusher.err = context.DeadlineExceeded

	rec := doRawRequest(engine, []byte{0x01}, "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when esp32 push fails, got %d: %s", rec.Code, rec.Body.String())
	}
}

// Transcription must not depend on sound_machine being reachable: the mic
// captured real speech either way, so a speaker outage shouldn't also
// silence the live transcript feed.
func TestHearStillTranscribesWhenPushFails(t *testing.T) {
	engine, pusher, _, broadcaster, _ := setupHear(t, "")
	pusher.err = context.DeadlineExceeded

	rec := doRawRequest(engine, []byte{0x01, 0x02}, "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when esp32 push fails, got %d: %s", rec.Code, rec.Body.String())
	}

	if !waitFor(func() bool { return broadcaster.count() == 1 }) {
		t.Fatalf("expected a broadcast event despite the failed push, got %d", broadcaster.count())
	}
}

func waitFor(condition func() bool) bool {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return condition()
}

// The following tests call transcribeAndBroadcast directly rather than
// going through Hear's `go` statement, since that method exists
// specifically so tests can exercise it synchronously (see hear.go).

func TestTranscribeAndBroadcastSendsEvent(t *testing.T) {
	_, _, transcriber, broadcaster, h := setupHear(t, "")
	transcriber.text = "turn on the lights"

	pcm := make([]byte, 100)
	h.transcribeAndBroadcast(pcm)

	if len(transcriber.got) != 1 || !bytes.Equal(transcriber.got[0], pcm) {
		t.Fatalf("expected Transcribe to be called with the heard pcm, got %+v", transcriber.got)
	}
	if len(broadcaster.events) != 1 {
		t.Fatalf("expected one broadcast event, got %d", len(broadcaster.events))
	}
	if broadcaster.events[0].Transcript != "turn on the lights" {
		t.Fatalf("expected transcript %q, got %q", "turn on the lights", broadcaster.events[0].Transcript)
	}
	if len(broadcaster.events[0].Waveform) == 0 {
		t.Fatal("expected a non-empty waveform envelope")
	}
}

func TestTranscribeAndBroadcastSkipsBroadcastOnFailure(t *testing.T) {
	_, _, transcriber, broadcaster, h := setupHear(t, "")
	transcriber.err = context.DeadlineExceeded

	h.transcribeAndBroadcast(make([]byte, 100))

	if len(broadcaster.events) != 0 {
		t.Fatalf("expected no broadcast when transcription fails, got %d", len(broadcaster.events))
	}
}
