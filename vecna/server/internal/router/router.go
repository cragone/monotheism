package router

import (
	"io/fs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/handlers"
	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/httpx"
)

func New(distFS fs.FS, db *gorm.DB, publisher handlers.DevicePublisher, registrar handlers.ShellyRegistrar) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery(), httpx.SecurityHeaders())

	deviceHandlers := handlers.NewDeviceHandlers(db, publisher, registrar)

	api := engine.Group("/api")
	api.GET("/health", handlers.Health)
	api.GET("/devices", deviceHandlers.List)
	api.POST("/devices", deviceHandlers.Create)
	api.GET("/devices/:id", deviceHandlers.Get)
	api.PATCH("/devices/:id/name", deviceHandlers.Rename)
	api.PATCH("/devices/:id/type", deviceHandlers.SetType)
	api.POST("/devices/:id/power", deviceHandlers.SetPower)
	api.POST("/devices/:id/brightness", deviceHandlers.SetBrightness)
	api.DELETE("/devices/:id", deviceHandlers.Delete)

	engine.NoRoute(httpx.ServeSPA(distFS, "index.html"))

	return engine
}
