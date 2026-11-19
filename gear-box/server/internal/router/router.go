package router

import (
	"io/fs"

	"github.com/gin-gonic/gin"

	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/handlers"
	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/httpx"
)

func New(distFS fs.FS, synth handlers.Synthesizer, pusher handlers.Pusher, sound handlers.SoundController) *gin.Engine {
	engine := gin.New()
	engine.Use(gin.Recovery(), httpx.SecurityHeaders())

	speakHandlers := handlers.NewSpeakHandlers(synth, pusher)
	soundHandlers := handlers.NewSoundHandlers(sound)

	api := engine.Group("/api")
	api.GET("/health", handlers.Health)
	api.POST("/speak", speakHandlers.Speak)
	api.GET("/sound/status", soundHandlers.Status)
	api.GET("/sound/modes", soundHandlers.Modes)
	api.POST("/sound/mode", soundHandlers.SetMode)
	api.POST("/sound/volume", soundHandlers.SetVolume)
	api.POST("/sound/power", soundHandlers.SetPower)

	engine.NoRoute(httpx.ServeSPA(distFS, "index.html"))

	return engine
}
