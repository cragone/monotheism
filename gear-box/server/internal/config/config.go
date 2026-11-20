package config

import "os"

type Config struct {
	Port string

	ESP32Host     string
	ESP32APIToken string

	MicAPIToken string

	PiperBinary    string
	PiperModelPath string
}

func Load() Config {
	return Config{
		Port: getenv("PORT", "80"),

		ESP32Host:     getenv("ESP32_HOST", "soundmachine.local"),
		ESP32APIToken: getenv("ESP32_API_TOKEN", ""),

		// Checked against the X-Token header on /hear, the mic device's
		// upload endpoint. Leave unset to disable token auth there (mirrors
		// ESP32_API_TOKEN's own default-open behavior).
		MicAPIToken: getenv("MIC_API_TOKEN", ""),

		PiperBinary:    getenv("PIPER_BINARY", "piper"),
		PiperModelPath: getenv("PIPER_MODEL_PATH", "/models/voice.onnx"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
