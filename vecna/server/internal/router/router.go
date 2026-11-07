package router

import (
	"io/fs"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/handlers"
	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/httpx"
)

func New(distFS fs.FS, db *gorm.DB, publisher handlers.DevicePublisher) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery(), httpx.SecurityHeaders())

	deviceHandlers := handlers.NewDeviceHandlers(db, publisher)

	api := engine.Group("/api")
	api.GET("/health", handlers.Health)
	api.GET("/devices", deviceHandlers.List)
	api.GET("/devices/:id", deviceHandlers.Get)
	api.POST("/devices/:id/power", deviceHandlers.SetPower)
	api.POST("/devices/:id/brightness", deviceHandlers.SetBrightness)

	engine.NoRoute(httpx.ServeSPA(distFS, "index.html"))

	return engine
}
