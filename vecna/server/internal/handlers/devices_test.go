package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/db"
	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/models"
)

type mockPublisher struct {
	powerCalls      []bool
	brightnessCalls []int
	err             error
}

func (m *mockPublisher) SetPower(_ models.Device, on bool) error {
	m.powerCalls = append(m.powerCalls, on)
	return m.err
}

func (m *mockPublisher) SetBrightness(_ models.Device, level int) error {
	m.brightnessCalls = append(m.brightnessCalls, level)
	return m.err
}

type testFixture struct {
	engine     *gin.Engine
	publisher  *mockPublisher
	dimmableID uint
	onOffID    uint
}

func setup(t *testing.T) testFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)

	gormDB, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}

	dimmable := models.Device{UniqueID: "plug-dimmable", Name: "Dimmable", Host: "192.168.1.26", SupportsBrightness: true}
	onOff := models.Device{UniqueID: "plug-onoff", Name: "OnOff", Host: "192.168.1.27", SupportsBrightness: false}
	seed(t, gormDB, &dimmable)
	seed(t, gormDB, &onOff)

	publisher := &mockPublisher{}
	h := NewDeviceHandlers(gormDB, publisher)

	engine := gin.New()
	api := engine.Group("/api")
	api.GET("/devices", h.List)
	api.GET("/devices/:id", h.Get)
	api.PATCH("/devices/:id/name", h.Rename)
	api.POST("/devices/:id/power", h.SetPower)
	api.POST("/devices/:id/brightness", h.SetBrightness)

	return testFixture{engine: engine, publisher: publisher, dimmableID: dimmable.ID, onOffID: onOff.ID}
}

func seed(t *testing.T, gormDB *gorm.DB, d *models.Device) {
	t.Helper()
	if err := gormDB.Create(d).Error; err != nil {
		t.Fatalf("seed device: %v", err)
	}
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

func TestSetPower(t *testing.T) {
	fx := setup(t)

	rec := doRequest(fx.engine, http.MethodPost, fmt.Sprintf("/api/devices/%d/power", fx.onOffID), powerRequest{On: true})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(fx.publisher.powerCalls) != 1 || fx.publisher.powerCalls[0] != true {
		t.Fatalf("expected one SetPower(true) call, got %+v", fx.publisher.powerCalls)
	}

	var device models.Device
	if err := json.Unmarshal(rec.Body.Bytes(), &device); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if device.LastPowerState != "ON" {
		t.Fatalf("expected LastPowerState ON, got %q", device.LastPowerState)
	}
}

func TestSetBrightnessRejectedWhenUnsupported(t *testing.T) {
	fx := setup(t)

	rec := doRequest(fx.engine, http.MethodPost, fmt.Sprintf("/api/devices/%d/brightness", fx.onOffID), brightnessRequest{Level: 128})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported brightness, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(fx.publisher.brightnessCalls) != 0 {
		t.Fatalf("expected no SetBrightness calls, got %+v", fx.publisher.brightnessCalls)
	}
}

func TestSetBrightnessSucceedsWhenSupported(t *testing.T) {
	fx := setup(t)

	rec := doRequest(fx.engine, http.MethodPost, fmt.Sprintf("/api/devices/%d/brightness", fx.dimmableID), brightnessRequest{Level: 180})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(fx.publisher.brightnessCalls) != 1 || fx.publisher.brightnessCalls[0] != 180 {
		t.Fatalf("expected one SetBrightness(180) call, got %+v", fx.publisher.brightnessCalls)
	}
}

func TestSetBrightnessValidatesRange(t *testing.T) {
	fx := setup(t)

	rec := doRequest(fx.engine, http.MethodPost, fmt.Sprintf("/api/devices/%d/brightness", fx.dimmableID), brightnessRequest{Level: 500})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for out-of-range level, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRenameDevice(t *testing.T) {
	fx := setup(t)

	rec := doRequest(fx.engine, http.MethodPatch, fmt.Sprintf("/api/devices/%d/name", fx.onOffID), renameRequest{Name: "  Living Room Lamp  "})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var device models.Device
	if err := json.Unmarshal(rec.Body.Bytes(), &device); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if device.Name != "Living Room Lamp" {
		t.Fatalf("expected trimmed name %q, got %q", "Living Room Lamp", device.Name)
	}

	rec = doRequest(fx.engine, http.MethodGet, fmt.Sprintf("/api/devices/%d", fx.onOffID), nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &device); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if device.Name != "Living Room Lamp" {
		t.Fatalf("expected rename to persist, got %q", device.Name)
	}
}

func TestRenameDeviceRejectsEmptyName(t *testing.T) {
	fx := setup(t)

	rec := doRequest(fx.engine, http.MethodPatch, fmt.Sprintf("/api/devices/%d/name", fx.onOffID), renameRequest{Name: "   "})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for blank name, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetDeviceNotFound(t *testing.T) {
	fx := setup(t)

	rec := doRequest(fx.engine, http.MethodGet, "/api/devices/99999", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestListDevices(t *testing.T) {
	fx := setup(t)

	rec := doRequest(fx.engine, http.MethodGet, "/api/devices", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var devices []models.Device
	if err := json.Unmarshal(rec.Body.Bytes(), &devices); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
}
