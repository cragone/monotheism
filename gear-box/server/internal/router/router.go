package router

import (
	"io/fs"

	"github.com/gin-gonic/gin"

	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/handlers"
	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/history"
	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/httpx"
)

func New(distFS fs.FS, synth handlers.Synthesizer, pusher handlers.Pusher, hist *history.Ring) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery(), httpx.SecurityHeaders())

	speakHandlers := handlers.NewSpeakHandlers(synth, pusher, hist)

	api := engine.Group("/api")
	api.GET("/health", handlers.Health)
	api.POST("/speak", speakHandlers.Speak)
	api.GET("/history", speakHandlers.History)

	engine.NoRoute(httpx.ServeSPA(distFS, "index.html"))

	return engine
}
