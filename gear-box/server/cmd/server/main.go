package main

import (
	"io/fs"
	"log"

	"github.com/joho/godotenv"

	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/config"
	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/esp32"
	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/piper"
	"github.com/cragone/lorena-teaches-brazilian/gear-box/server/internal/router"
	webassets "github.com/cragone/lorena-teaches-brazilian/gear-box/server/web"
)

func main() {
	_ = godotenv.Load() // best-effort; production has no .env and relies on real env vars

	cfg := config.Load()

	synth := piper.New(cfg.PiperBinary, cfg.PiperModelPath)
	esp32Client := esp32.New(cfg.ESP32Host, cfg.ESP32APIToken)

	distFS, err := fs.Sub(webassets.DistFS, "dist")
	if err != nil {
		log.Fatalf("fatal: embed fs: %v", err)
	}

	engine := router.New(distFS, synth, esp32Client, esp32Client)

	log.Printf("info: gear-box server listening on :%s (esp32 at %s)", cfg.Port, cfg.ESP32Host)
	if err := engine.Run(":" + cfg.Port); err != nil {
		log.Fatalf("fatal: server exited: %v", err)
	}
}
