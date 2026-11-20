package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupHear(t *testing.T, token string) (*gin.Engine, *mockPusher) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	pusher := &mockPusher{}
	h := NewHearHandlers(pusher, token)

	engine := gin.New()
	engine.POST("/hear", h.Hear)

	return engine, pusher
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
	engine, pusher := setupHear(t, "")

	rec := doRawRequest(engine, []byte{0x01, 0x02, 0x03}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(pusher.pushed) != 1 || !bytes.Equal(pusher.pushed[0], []byte{0x01, 0x02, 0x03}) {
		t.Fatalf("expected one Speak push with the uploaded bytes, got %+v", pusher.pushed)
	}
}

func TestHearRejectsEmptyBody(t *testing.T) {
	engine, _ := setupHear(t, "")

	rec := doRawRequest(engine, []byte{}, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHearRejectsOversizedBody(t *testing.T) {
	engine, _ := setupHear(t, "")

	rec := doRawRequest(engine, make([]byte, MaxHearBytes+1), "")
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for oversized body, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHearRequiresTokenWhenConfigured(t *testing.T) {
	engine, _ := setupHear(t, "secret")

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
	engine, pusher := setupHear(t, "")
	pusher.err = context.DeadlineExceeded

	rec := doRawRequest(engine, []byte{0x01}, "")
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when esp32 push fails, got %d: %s", rec.Code, rec.Body.String())
	}
}
