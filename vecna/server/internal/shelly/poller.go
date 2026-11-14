package shelly

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/models"
)

const pollInterval = 3 * time.Second

// Bootstrap registers (or updates) the single env-var-configured Shelly plug
// as a Device row, keyed by its MAC address. There's no MQTT discovery to
// learn devices from anymore — the host is given directly via config, so
// this just confirms the plug answers and records it. New devices beyond
// this one are added via Register (the /api/devices endpoint) instead.
func Bootstrap(db *gorm.DB, client *Client, host, name string) (models.Device, error) {
	info, err := client.GetDeviceInfo(host)
	if err != nil {
		return models.Device{}, fmt.Errorf("shelly bootstrap: %w", err)
	}

	device := models.Device{
		UniqueID:   info.MAC,
		Name:       name,
		Host:       host,
		SwitchID:   0,
		DeviceType: models.DeviceTypeLight,
	}

	var existing models.Device
	if err := db.Where("unique_id = ?", info.MAC).First(&existing).Error; err == nil {
		device.ID = existing.ID
		device.CreatedAt = existing.CreatedAt
		device.LastPowerState = existing.LastPowerState
		device.LastBrightness = existing.LastBrightness
		device.LastSeenAt = existing.LastSeenAt
		// Once a device is known, its name/type are whatever the user set via
		// the UI/API, not SHELLY_NAME/the "light" default — otherwise every
		// restart would clobber a rename or retype back to env-var defaults.
		device.Name = existing.Name
		device.DeviceType = existing.DeviceType
	}

	if err := db.Save(&device).Error; err != nil {
		return models.Device{}, fmt.Errorf("shelly bootstrap: save device: %w", err)
	}
	return device, nil
}

// Register confirms the plug at host answers Shelly's RPC API, then inserts
// a new Device row for it. Unlike Bootstrap, it errors on a MAC that's
// already registered rather than upserting — an explicit "add a device" call
// should fail loudly on a duplicate rather than silently repoint an existing
// row at a different host.
func Register(db *gorm.DB, client *Client, host, name string, deviceType models.DeviceType) (models.Device, error) {
	info, err := client.GetDeviceInfo(host)
	if err != nil {
		return models.Device{}, fmt.Errorf("register device: %w", err)
	}

	var existing models.Device
	if err := db.Where("unique_id = ?", info.MAC).First(&existing).Error; err == nil {
		return models.Device{}, fmt.Errorf("register device: %q at %s is already registered as %q", info.MAC, host, existing.Name)
	}

	device := models.Device{
		UniqueID:   info.MAC,
		Name:       name,
		Host:       host,
		SwitchID:   0,
		DeviceType: deviceType,
	}
	if err := db.Create(&device).Error; err != nil {
		return models.Device{}, fmt.Errorf("register device: save: %w", err)
	}
	return device, nil
}

// PollForever periodically reads each known device's actual relay state
// (which can change from the physical button or the Shelly app, not just
// vecna's own commands) and records it, in place of the state pushes MQTT
// used to provide. It never returns; run it in its own goroutine.
func PollForever(db *gorm.DB, client *Client) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for range ticker.C {
		var devices []models.Device
		if err := db.Find(&devices).Error; err != nil {
			log.Printf("error: shelly: poll: list devices: %v", err)
			continue
		}

		for _, d := range devices {
			on, err := client.GetOn(d.Host, d.SwitchID)
			if err != nil {
				log.Printf("error: shelly: poll %s: %v", d.Host, err)
				continue
			}

			state := "OFF"
			if on {
				state = "ON"
			}
			updates := map[string]any{"last_power_state": state, "last_seen_at": time.Now()}
			if err := db.Model(&models.Device{}).Where("id = ?", d.ID).Updates(updates).Error; err != nil {
				log.Printf("error: shelly: poll %s: update device: %v", d.Host, err)
			}
		}
	}
}
