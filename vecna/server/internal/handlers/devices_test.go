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

type registerCall struct {
	host       string
	name       string
	deviceType models.DeviceType
}

type mockRegistrar struct {
	device models.Device
	err    error
	calls  []registerCall
}

func (m *mockRegistrar) Register(host, name string, deviceType models.DeviceType) (models.Device, error) {
	m.calls = append(m.calls, registerCall{host: host, name: name, deviceType: deviceType})
	return m.device, m.err
}

type testFixture struct {
	engine     *gin.Engine
	publisher  *mockPublisher
	registrar  *mockRegistrar
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

	dimmable := models.Device{UniqueID: "plug-dimmable", Name: "Dimmable", Host: "192.168.1.26", DeviceType: models.DeviceTypeLight, SupportsBrightness: true}
	onOff := models.Device{UniqueID: "plug-onoff", Name: "OnOff", Host: "192.168.1.27", DeviceType: models.DeviceTypeLight, SupportsBrightness: false}
	seed(t, gormDB, &dimmable)
	seed(t, gormDB, &onOff)

	publisher := &mockPublisher{}
	registrar := &mockRegistrar{}
	h := NewDeviceHandlers(gormDB, publisher, registrar)

	engine := gin.New()
	api := engine.Group("/api")
	api.GET("/devices", h.List)
	api.POST("/devices", h.Create)
	api.GET("/devices/:id", h.Get)
	api.PATCH("/devices/:id/name", h.Rename)
	api.PATCH("/devices/:id/type", h.SetType)
	api.POST("/devices/:id/power", h.SetPower)
	api.POST("/devices/:id/brightness", h.SetBrightness)
	api.DELETE("/devices/:id", h.Delete)

	return testFixture{engine: engine, publisher: publisher, registrar: registrar, dimmableID: dimmable.ID, onOffID: onOff.ID}
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

func TestCreateDevice(t *testing.T) {
	fx := setup(t)
	fx.registrar.device = models.Device{ID: 99, UniqueID: "shellyplugusg4-e8f60a7ded4c", Name: "Sound Machine", Host: "192.168.1.210", DeviceType: models.DeviceTypeSoundMachine}

	rec := doRequest(fx.engine, http.MethodPost, "/api/devices", createDeviceRequest{Host: "192.168.1.210", Name: "Sound Machine", DeviceType: models.DeviceTypeSoundMachine})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(fx.registrar.calls) != 1 || fx.registrar.calls[0].host != "192.168.1.210" {
		t.Fatalf("expected one Register call for the given host, got %+v", fx.registrar.calls)
	}

	var device models.Device
	if err := json.Unmarshal(rec.Body.Bytes(), &device); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if device.Name != "Sound Machine" || device.DeviceType != models.DeviceTypeSoundMachine {
		t.Fatalf("unexpected device in response: %+v", device)
	}
}

func TestCreateDeviceUnreachableHost(t *testing.T) {
	fx := setup(t)
	fx.registrar.err = fmt.Errorf("register device: connect: connection refused")

	rec := doRequest(fx.engine, http.MethodPost, "/api/devices", createDeviceRequest{Host: "192.168.1.250", Name: "Ghost", DeviceType: models.DeviceTypeLight})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unreachable host, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateDeviceInvalidType(t *testing.T) {
	fx := setup(t)

	rec := doRequest(fx.engine, http.MethodPost, "/api/devices", map[string]string{"host": "192.168.1.210", "name": "Sound Machine", "deviceType": "toaster"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid device type, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(fx.registrar.calls) != 0 {
		t.Fatalf("expected no Register calls, got %+v", fx.registrar.calls)
	}
}

func TestSetDeviceType(t *testing.T) {
	fx := setup(t)

	rec := doRequest(fx.engine, http.MethodPatch, fmt.Sprintf("/api/devices/%d/type", fx.onOffID), setTypeRequest{DeviceType: models.DeviceTypeGarage})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var device models.Device
	if err := json.Unmarshal(rec.Body.Bytes(), &device); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if device.DeviceType != models.DeviceTypeGarage {
		t.Fatalf("expected device type %q, got %q", models.DeviceTypeGarage, device.DeviceType)
	}

	rec = doRequest(fx.engine, http.MethodGet, fmt.Sprintf("/api/devices/%d", fx.onOffID), nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &device); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if device.DeviceType != models.DeviceTypeGarage {
		t.Fatalf("expected type change to persist, got %q", device.DeviceType)
	}
}

func TestSetDeviceTypeInvalid(t *testing.T) {
	fx := setup(t)

	rec := doRequest(fx.engine, http.MethodPatch, fmt.Sprintf("/api/devices/%d/type", fx.onOffID), map[string]string{"deviceType": "toaster"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid device type, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteDevice(t *testing.T) {
	fx := setup(t)

	rec := doRequest(fx.engine, http.MethodDelete, fmt.Sprintf("/api/devices/%d", fx.onOffID), nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = doRequest(fx.engine, http.MethodGet, fmt.Sprintf("/api/devices/%d", fx.onOffID), nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", rec.Code)
	}

	rec = doRequest(fx.engine, http.MethodGet, "/api/devices", nil)
	var devices []models.Device
	if err := json.Unmarshal(rec.Body.Bytes(), &devices); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 device remaining, got %d", len(devices))
	}
}
