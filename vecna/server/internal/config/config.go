package config

import "os"

type Config struct {
	Port    string
	DataDir string

	ShellyHost string
	ShellyName string
}

func Load() Config {
	return Config{
		Port:    getenv("PORT", "80"),
		DataDir: getenv("DATA_DIR", "./data"),

		ShellyHost: getenv("SHELLY_HOST", ""),
		ShellyName: getenv("SHELLY_NAME", ""),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
