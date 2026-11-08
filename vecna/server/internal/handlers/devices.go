package handlers

import (
	"net/http"
	"strconv"

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

type DeviceHandlers struct {
	db        *gorm.DB
	publisher DevicePublisher
}

func NewDeviceHandlers(db *gorm.DB, publisher DevicePublisher) *DeviceHandlers {
	return &DeviceHandlers{db: db, publisher: publisher}
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
