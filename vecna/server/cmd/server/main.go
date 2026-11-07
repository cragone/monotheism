package main

import (
	"io/fs"
	"log"

	"github.com/joho/godotenv"

	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/config"
	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/db"
	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/mqtt"
	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/router"
	webassets "github.com/cragone/lorena-teaches-brazilian/vecna/server/web"
)

func main() {
	_ = godotenv.Load() // best-effort; production has no .env and relies on real env vars

	cfg := config.Load()

	gormDB, err := db.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("db init: %v", err)
	}

	publisher, err := mqtt.Connect(cfg, gormDB)
	if err != nil {
		log.Fatalf("mqtt connect: %v", err)
	}

	distFS, err := fs.Sub(webassets.DistFS, "dist")
	if err != nil {
		log.Fatalf("embed fs: %v", err)
	}

	engine := router.New(distFS, gormDB, publisher)

	log.Printf("vecna server listening on :%s", cfg.Port)
	if err := engine.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server exited: %v", err)
	}
}
