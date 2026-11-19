package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/history"
)

type mockSynthesizer struct {
	pcm []byte
	err error
	got []string
}

func (m *mockSynthesizer) Synthesize(_ context.Context, text string) ([]byte, error) {
	m.got = append(m.got, text)
	return m.pcm, m.err
}

type mockPusher struct {
	pushed [][]byte
	err    error
}

func (m *mockPusher) Speak(_ context.Context, pcm []byte) error {
	m.pushed = append(m.pushed, pcm)
	return m.err
}

type testFixture struct {
	engine *gin.Engine
	synth  *mockSynthesizer
	pusher *mockPusher
}

func setup(t *testing.T) testFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	synth := &mockSynthesizer{pcm: []byte{0x01, 0x02}}
	pusher := &mockPusher{}
	hist := history.NewRing(20)
	h := NewSpeakHandlers(synth, pusher, hist)

	engine := gin.New()
	api := engine.Group("/api")
	api.POST("/speak", h.Speak)
	api.GET("/history", h.History)

	return testFixture{engine: engine, synth: synth, pusher: pusher}
}

func doRequest(engine *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestSpeakPushesToESP32AndRecordsHistory(t *testing.T) {
	fx := setup(t)

	rec := doRequest(fx.engine, http.MethodPost, "/api/speak", speakRequest{Text: "hello"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(fx.synth.got) != 1 || fx.synth.got[0] != "hello" {
		t.Fatalf("expected one Synthesize(\"hello\") call, got %+v", fx.synth.got)
	}
	if len(fx.pusher.pushed) != 1 {
		t.Fatalf("expected one Speak push, got %d", len(fx.pusher.pushed))
	}

	rec = doRequest(fx.engine, http.MethodGet, "/api/history", nil)
	var entries []history.Entry
	if err := json.Unmarshal(rec.Body.Bytes(), &entries); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(entries) != 1 || entries[0].Text != "hello" {
		t.Fatalf("expected history to contain \"hello\", got %+v", entries)
	}
}

func TestSpeakRejectsEmptyText(t *testing.T) {
	fx := setup(t)

	rec := doRequest(fx.engine, http.MethodPost, "/api/speak", speakRequest{Text: ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty text, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSpeakRejectsTooLongText(t *testing.T) {
	fx := setup(t)

	long := make([]byte, 201)
	for i := range long {
		long[i] = 'a'
	}
	rec := doRequest(fx.engine, http.MethodPost, "/api/speak", speakRequest{Text: string(long)})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for too-long text, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSpeakReturnsBadGatewayWhenPushFails(t *testing.T) {
	fx := setup(t)
	fx.pusher.err = context.DeadlineExceeded

	rec := doRequest(fx.engine, http.MethodPost, "/api/speak", speakRequest{Text: "hello"})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when esp32 push fails, got %d: %s", rec.Code, rec.Body.String())
	}
}
