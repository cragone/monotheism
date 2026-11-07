package mqtt

// Home Assistant MQTT discovery topics that cync-lan publishes retained
// config payloads to. The plug could be advertised as either a dimmable
// "light" or a plain "switch" depending on its capabilities, so we
// subscribe to both.
const (
	discoveryLightTopic  = "homeassistant/light/+/config"
	discoverySwitchTopic = "homeassistant/switch/+/config"
	statusTopic          = "homeassistant/status"
)
