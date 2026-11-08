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
	if cfg.ShellyHost == "" {
		log.Fatal("SHELLY_HOST is required")
	}

	gormDB, err := db.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("db init: %v", err)
	}

	shellyClient := shelly.NewClient()
	device, err := shelly.Bootstrap(gormDB, shellyClient, cfg.ShellyHost, cfg.ShellyName)
	if err != nil {
		log.Fatalf("shelly bootstrap: %v", err)
	}
	log.Printf("shelly: registered %q (%s) at %s", device.Name, device.UniqueID, device.Host)

	go shelly.PollForever(gormDB, shellyClient)

	distFS, err := fs.Sub(webassets.DistFS, "dist")
	if err != nil {
		log.Fatalf("embed fs: %v", err)
	}

	engine := router.New(distFS, gormDB, shellyClient)

	log.Printf("vecna server listening on :%s", cfg.Port)
	if err := engine.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
