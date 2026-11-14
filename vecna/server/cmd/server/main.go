package main

import (
	"io/fs"
	"log"

	"github.com/joho/godotenv"

	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/config"
	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/db"
	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/router"
	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/shelly"
	webassets "github.com/cragone/lorena-teaches-brazilian/vecna/server/web"
)

func main() {
	_ = godotenv.Load() // best-effort; production has no .env and relies on real env vars

	cfg := config.Load()

	gormDB, err := db.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("fatal: db init: %v", err)
	}

	shellyClient := shelly.NewClient()

	if cfg.ShellyHost != "" {
		device, err := shelly.Bootstrap(gormDB, shellyClient, cfg.ShellyHost, cfg.ShellyName)
		if err != nil {
			log.Fatalf("fatal: shelly bootstrap: %v", err)
		}
		log.Printf("info: shelly: registered %q (%s) at %s", device.Name, device.UniqueID, device.Host)
	}

	go shelly.PollForever(gormDB, shellyClient)

	distFS, err := fs.Sub(webassets.DistFS, "dist")
	if err != nil {
		log.Fatalf("fatal: embed fs: %v", err)
	}

	registrar := shelly.NewRegistrar(gormDB, shellyClient)
	engine := router.New(distFS, gormDB, shellyClient, registrar)

	log.Printf("info: vecna server listening on :%s", cfg.Port)
	if err := engine.Run(":" + cfg.Port); err != nil {
		log.Fatalf("fatal: server exited: %v", err)
	}
}
