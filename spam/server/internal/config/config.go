package config

import "os"

type Config struct {
	Addr    string // listen address
	DataDir string // where the SQLite file lives (mounted volume in k8s)
	WebDir  string // exported React Native web build; empty disables static serving
	Prod    bool
}

func Load() Config {
	return Config{
		Addr:    ":" + getenv("PORT", "8080"),
		DataDir: getenv("DATA_DIR", "./data"),
		WebDir:  getenv("WEB_DIR", "./web"),
		Prod:    os.Getenv("APP_ENV") == "production",
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
