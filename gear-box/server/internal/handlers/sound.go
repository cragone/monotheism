package handlers

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/esp32"
)

// SoundController is the subset of esp32.Client the sound handlers need,
// defined here so tests can supply a mock instead of hitting the real
// device over the LAN.
type SoundController interface {
	Status(ctx context.Context) (esp32.SoundStatus, error)
	Modes(ctx context.Context) ([]string, error)
	SetMode(ctx context.Context, mode string) (esp32.SoundStatus, error)
	SetVolume(ctx context.Context, level float64) (esp32.SoundStatus, error)
	SetPower(ctx context.Context, on bool) (esp32.SoundStatus, error)
}

type SoundHandlers struct {
	esp32 SoundController
}

func NewSoundHandlers(controller SoundController) *SoundHandlers {
	return &SoundHandlers{esp32: controller}
}

func (h *SoundHandlers) Status(c *gin.Context) {
	status, err := h.esp32.Status(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach sound machine"})
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *SoundHandlers) Modes(c *gin.Context) {
	modes, err := h.esp32.Modes(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach sound machine"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"modes": modes})
}

type modeRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *SoundHandlers) SetMode(c *gin.Context) {
	var req modeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if req.Name == "speak" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "speak is not a selectable mode"})
		return
	}

	status, err := h.esp32.SetMode(c.Request.Context(), req.Name)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach sound machine"})
		return
	}
	c.JSON(http.StatusOK, status)
}

type volumeRequest struct {
	Level *float64 `json:"level" binding:"required"`
}

func (h *SoundHandlers) SetVolume(c *gin.Context) {
	var req volumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	level := *req.Level
	if level < 0 || level > 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "level must be between 0 and 1"})
		return
	}

	status, err := h.esp32.SetVolume(c.Request.Context(), level)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach sound machine"})
		return
	}
	c.JSON(http.StatusOK, status)
}

type powerRequest struct {
	On *bool `json:"on" binding:"required"`
}

func (h *SoundHandlers) SetPower(c *gin.Context) {
	var req powerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	status, err := h.esp32.SetPower(c.Request.Context(), *req.On)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach sound machine"})
		return
	}
	c.JSON(http.StatusOK, status)
}
