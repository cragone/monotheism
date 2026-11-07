package config

import "os"

type Config struct {
	Port    string
	DataDir string

	MQTTBrokerURL string
	MQTTUsername  string
	MQTTPassword  string
	MQTTBaseTopic string
}

func Load() Config {
	return Config{
		Port:    getenv("PORT", "80"),
		DataDir: getenv("DATA_DIR", "./data"),

		MQTTBrokerURL: getenv("MQTT_BROKER_URL", "tcp://localhost:1883"),
		MQTTUsername:  getenv("MQTT_USERNAME", ""),
		MQTTPassword:  getenv("MQTT_PASSWORD", ""),
		MQTTBaseTopic: getenv("MQTT_BASE_TOPIC", "cync_lan"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
