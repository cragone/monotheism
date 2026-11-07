package mqtt

import (
	"log"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"gorm.io/gorm"

	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/models"
)

type Subscriber struct {
	db *gorm.DB
}

func NewSubscriber(db *gorm.DB) *Subscriber {
	return &Subscriber{db: db}
}

// Subscribe wires up discovery/state/status handlers on the given client.
// It's intended to be called from the client's OnConnect handler so
// subscriptions are re-established correctly after a reconnect.
func (s *Subscriber) Subscribe(client paho.Client) {
	client.Subscribe(discoveryLightTopic, 1, s.onDiscovery)
	client.Subscribe(discoverySwitchTopic, 1, s.onDiscovery)
	client.Subscribe(statusTopic, 1, s.onStatus)

	// Re-subscribe to state topics for devices we already know about,
	// since a dropped connection doesn't keep prior subscriptions alive
	// and a retained discovery message may not get redelivered.
	var devices []models.Device
	if err := s.db.Find(&devices).Error; err != nil {
		log.Printf("mqtt: resubscribe lookup failed: %v", err)
		return
	}
	for _, d := range devices {
		s.subscribeState(client, d.StateTopic)
	}
}

func (s *Subscriber) subscribeState(client paho.Client, stateTopic string) {
	client.Subscribe(stateTopic, 1, s.onState)
}

func (s *Subscriber) onDiscovery(client paho.Client, msg paho.Message) {
	payload, err := DecodeDiscovery(msg.Payload())
	if err != nil {
		log.Printf("mqtt: ignoring discovery message on %s: %v", msg.Topic(), err)
		return
	}

	device := models.Device{
		UniqueID:           payload.UniqueID,
		Name:               payload.Name,
		StateTopic:         payload.StateTopic,
		CommandTopic:       payload.CommandTopic,
		SupportsBrightness: payload.Brightness,
	}

	var existing models.Device
	if err := s.db.Where("unique_id = ?", payload.UniqueID).First(&existing).Error; err == nil {
		device.ID = existing.ID
		device.CreatedAt = existing.CreatedAt
		device.LastPowerState = existing.LastPowerState
		device.LastBrightness = existing.LastBrightness
		device.LastSeenAt = existing.LastSeenAt
	}

	if err := s.db.Save(&device).Error; err != nil {
		log.Printf("mqtt: failed to upsert device %s: %v", payload.UniqueID, err)
		return
	}

	s.subscribeState(client, payload.StateTopic)
}

func (s *Subscriber) onState(_ paho.Client, msg paho.Message) {
	payload, err := DecodeState(msg.Payload())
	if err != nil {
		log.Printf("mqtt: ignoring state message on %s: %v", msg.Topic(), err)
		return
	}

	updates := map[string]any{
		"last_power_state": payload.State,
		"last_seen_at":     time.Now(),
	}
	if payload.Brightness != nil {
		updates["last_brightness"] = *payload.Brightness
	}

	if err := s.db.Model(&models.Device{}).Where("state_topic = ?", msg.Topic()).Updates(updates).Error; err != nil {
		log.Printf("mqtt: failed to update state for topic %s: %v", msg.Topic(), err)
	}
}

func (s *Subscriber) onStatus(_ paho.Client, msg paho.Message) {
	log.Printf("mqtt: cync-lan status: %s", string(msg.Payload()))
}
