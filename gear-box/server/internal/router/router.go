package router

import (
	"io/fs"

	"github.com/gin-gonic/gin"

	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/handlers"
	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/httpx"
)

func New(distFS fs.FS, synth handlers.Synthesizer, pusher handlers.Pusher, micAPIToken string) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery(), httpx.SecurityHeaders())

	speakHandlers := handlers.NewSpeakHandlers(synth, pusher)
	hearHandlers := handlers.NewHearHandlers(pusher, micAPIToken)

	api := engine.Group("/api")
	api.GET("/health", handlers.Health)
	api.POST("/speak", speakHandlers.Speak)

	// Not under /api: this is the mic_listener ESP32 pushing audio to us,
	// the same device-to-server relationship (and LAN-only reach) as
	// gear-box pushing to sound_machine's own /speak, just in the other
	// direction.
	engine.POST("/hear", hearHandlers.Hear)

	engine.NoRoute(httpx.ServeSPA(distFS, "index.html"))

	return engine
}
