package mqtt

import (
	"encoding/json"
	"fmt"
)

// DiscoveryPayload is cync-lan's retained Home Assistant MQTT discovery
// message, published to homeassistant/<component>/<object_id>/config.
// The presence of Brightness (cync-lan sets it true for dimmable devices)
// is how we decide whether a device supports brightness control.
type DiscoveryPayload struct {
	UniqueID     string `json:"unique_id"`
	Name         string `json:"name"`
	StateTopic   string `json:"state_topic"`
	CommandTopic string `json:"command_topic"`
	Brightness   bool   `json:"brightness"`
}

func DecodeDiscovery(raw []byte) (DiscoveryPayload, error) {
	var p DiscoveryPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return DiscoveryPayload{}, fmt.Errorf("decode discovery payload: %w", err)
	}
	if p.UniqueID == "" {
		return DiscoveryPayload{}, fmt.Errorf("decode discovery payload: missing unique_id")
	}
	if p.StateTopic == "" || p.CommandTopic == "" {
		return DiscoveryPayload{}, fmt.Errorf("decode discovery payload: missing state_topic or command_topic")
	}
	return p, nil
}

// StatePayload is the JSON cync-lan publishes to a device's state topic,
// e.g. {"state":"ON","brightness":180}. Brightness is only meaningful when
// the device's discovery payload indicated brightness support.
type StatePayload struct {
	State      string `json:"state"`
	Brightness *int   `json:"brightness,omitempty"`
}

func DecodeState(raw []byte) (StatePayload, error) {
	var p StatePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return StatePayload{}, fmt.Errorf("decode state payload: %w", err)
	}
	if p.State == "" {
		return StatePayload{}, fmt.Errorf("decode state payload: missing state")
	}
	return p, nil
}

// EncodePowerCommand builds the JSON command payload to turn a device on
// or off, per the Home Assistant MQTT JSON light schema.
func EncodePowerCommand(on bool) []byte {
	state := "OFF"
	if on {
		state = "ON"
	}
	raw, _ := json.Marshal(map[string]any{"state": state})
	return raw
}

// EncodeBrightnessCommand builds the JSON command payload to set
// brightness (0-255); setting a non-zero brightness implies turning on.
func EncodeBrightnessCommand(level int) []byte {
	raw, _ := json.Marshal(map[string]any{"state": "ON", "brightness": level})
	return raw
}
