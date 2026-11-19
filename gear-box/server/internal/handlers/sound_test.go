package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/esp32"
)

type mockSoundController struct {
	status esp32.SoundStatus
	modes  []string
	err    error

	gotMode   string
	gotVolume float64
	gotPower  *bool
}

func (m *mockSoundController) Status(_ context.Context) (esp32.SoundStatus, error) {
	return m.status, m.err
}

func (m *mockSoundController) Modes(_ context.Context) ([]string, error) {
	return m.modes, m.err
}

func (m *mockSoundController) SetMode(_ context.Context, mode string) (esp32.SoundStatus, error) {
	m.gotMode = mode
	return m.status, m.err
}

func (m *mockSoundController) SetVolume(_ context.Context, level float64) (esp32.SoundStatus, error) {
	m.gotVolume = level
	return m.status, m.err
}

func (m *mockSoundController) SetPower(_ context.Context, on bool) (esp32.SoundStatus, error) {
	m.gotPower = &on
	return m.status, m.err
}

func setupSound(t *testing.T) (*gin.Engine, *mockSoundController) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	controller := &mockSoundController{status: esp32.SoundStatus{On: true, Mode: "white", Volume: 0.3}}
	h := NewSoundHandlers(controller)

	engine := gin.New()
	api := engine.Group("/api")
	api.GET("/sound/status", h.Status)
	api.GET("/sound/modes", h.Modes)
	api.POST("/sound/mode", h.SetMode)
	api.POST("/sound/volume", h.SetVolume)
	api.POST("/sound/power", h.SetPower)

	return engine, controller
}

func TestSoundModeRejectsSpeak(t *testing.T) {
	engine, controller := setupSound(t)

	rec := doRequest(engine, http.MethodPost, "/api/sound/mode", modeRequest{Name: "speak"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for mode \"speak\", got %d: %s", rec.Code, rec.Body.String())
	}
	if controller.gotMode != "" {
		t.Fatalf("expected SetMode not to be called, got %q", controller.gotMode)
	}
}

func TestSoundModeSetsRequestedMode(t *testing.T) {
	engine, controller := setupSound(t)

	rec := doRequest(engine, http.MethodPost, "/api/sound/mode", modeRequest{Name: "ocean"})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if controller.gotMode != "ocean" {
		t.Fatalf("expected SetMode(\"ocean\"), got %q", controller.gotMode)
	}
}

func TestSoundVolumeRejectsOutOfRange(t *testing.T) {
	engine, controller := setupSound(t)

	level := 1.5
	rec := doRequest(engine, http.MethodPost, "/api/sound/volume", volumeRequest{Level: &level})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for out-of-range volume, got %d: %s", rec.Code, rec.Body.String())
	}
	if controller.gotVolume != 0 {
		t.Fatalf("expected SetVolume not to be called, got %v", controller.gotVolume)
	}
}

func TestSoundVolumeSetsRequestedLevel(t *testing.T) {
	engine, controller := setupSound(t)

	level := 0.75
	rec := doRequest(engine, http.MethodPost, "/api/sound/volume", volumeRequest{Level: &level})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if controller.gotVolume != 0.75 {
		t.Fatalf("expected SetVolume(0.75), got %v", controller.gotVolume)
	}
}

func TestSoundPowerTogglesOnAndOff(t *testing.T) {
	engine, controller := setupSound(t)

	off := false
	rec := doRequest(engine, http.MethodPost, "/api/sound/power", powerRequest{On: &off})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if controller.gotPower == nil || *controller.gotPower != false {
		t.Fatalf("expected SetPower(false), got %+v", controller.gotPower)
	}
}

func TestSoundHandlersReturnBadGatewayWhenUnreachable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	controller := &mockSoundController{err: context.DeadlineExceeded}
	h := NewSoundHandlers(controller)
	engine := gin.New()
	api := engine.Group("/api")
	api.GET("/sound/status", h.Status)

	rec := doRequest(engine, http.MethodGet, "/api/sound/status", nil)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 when esp32 is unreachable, got %d: %s", rec.Code, rec.Body.String())
	}
}
