package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/models"
)

// DevicePublisher is the subset of shelly.Client the handlers need, defined
// here so tests can supply a mock instead of a real Shelly client.
type DevicePublisher interface {
	SetPower(d models.Device, on bool) error
	SetBrightness(d models.Device, level int) error
}

// ShellyRegistrar is the subset of shelly.Registrar the Create handler
// needs, defined here so tests can supply a mock instead of hitting the
// network.
type ShellyRegistrar interface {
	Register(host, name string, deviceType models.DeviceType) (models.Device, error)
}

type DeviceHandlers struct {
	db        *gorm.DB
	publisher DevicePublisher
	registrar ShellyRegistrar
}

func NewDeviceHandlers(db *gorm.DB, publisher DevicePublisher, registrar ShellyRegistrar) *DeviceHandlers {
	return &DeviceHandlers{db: db, publisher: publisher, registrar: registrar}
}

type createDeviceRequest struct {
	Host       string            `json:"host"`
	Name       string            `json:"name"`
	DeviceType models.DeviceType `json:"deviceType"`
}

func (h *DeviceHandlers) Create(c *gin.Context) {
	var req createDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	host := strings.TrimSpace(req.Host)
	if host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "host must not be empty"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name must not be empty"})
		return
	}
	if !req.DeviceType.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device type"})
		return
	}

	device, err := h.registrar.Register(host, name, req.DeviceType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, device)
}

func (h *DeviceHandlers) List(c *gin.Context) {
	var devices []models.Device
	if err := h.db.Order("name").Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list devices"})
		return
	}
	c.JSON(http.StatusOK, devices)
}

func (h *DeviceHandlers) Get(c *gin.Context) {
	device, ok := h.findDevice(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, device)
}

type renameRequest struct {
	Name string `json:"name"`
}

func (h *DeviceHandlers) Rename(c *gin.Context) {
	device, ok := h.findDevice(c)
	if !ok {
		return
	}

	var req renameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name must not be empty"})
		return
	}

	if err := h.db.Model(&models.Device{}).Where("id = ?", device.ID).Update("name", name).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to rename device"})
		return
	}
	device.Name = name

	c.JSON(http.StatusOK, device)
}

type setTypeRequest struct {
	DeviceType models.DeviceType `json:"deviceType"`
}

func (h *DeviceHandlers) SetType(c *gin.Context) {
	device, ok := h.findDevice(c)
	if !ok {
		return
	}

	var req setTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if !req.DeviceType.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device type"})
		return
	}

	if err := h.db.Model(&models.Device{}).Where("id = ?", device.ID).Update("device_type", req.DeviceType).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update device type"})
		return
	}
	device.DeviceType = req.DeviceType

	c.JSON(http.StatusOK, device)
}

func (h *DeviceHandlers) Delete(c *gin.Context) {
	device, ok := h.findDevice(c)
	if !ok {
		return
	}

	if err := h.db.Delete(&models.Device{}, device.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete device"})
		return
	}

	c.Status(http.StatusNoContent)
}

type powerRequest struct {
	On bool `json:"on"`
}

func (h *DeviceHandlers) SetPower(c *gin.Context) {
	device, ok := h.findDevice(c)
	if !ok {
		return
	}

	var req powerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.publisher.SetPower(device, req.On); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to publish command"})
		return
	}

	state := "OFF"
	if req.On {
		state = "ON"
	}
	device.LastPowerState = state
	h.db.Model(&models.Device{}).Where("id = ?", device.ID).Update("last_power_state", state)

	c.JSON(http.StatusOK, device)
}

type brightnessRequest struct {
	Level int `json:"level"`
}

func (h *DeviceHandlers) SetBrightness(c *gin.Context) {
	device, ok := h.findDevice(c)
	if !ok {
		return
	}

	if !device.SupportsBrightness {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device does not support brightness"})
		return
	}

	var req brightnessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Level < 0 || req.Level > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "level must be between 0 and 255"})
		return
	}

	if err := h.publisher.SetBrightness(device, req.Level); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to publish command"})
		return
	}

	device.LastPowerState = "ON"
	device.LastBrightness = req.Level
	h.db.Model(&models.Device{}).Where("id = ?", device.ID).
		Updates(map[string]any{"last_power_state": "ON", "last_brightness": req.Level})

	c.JSON(http.StatusOK, device)
}

func (h *DeviceHandlers) findDevice(c *gin.Context) (models.Device, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid device id"})
		return models.Device{}, false
	}

	var device models.Device
	if err := h.db.First(&device, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return models.Device{}, false
	}
	return device, true
}
