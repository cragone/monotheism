package models

import "time"

// Device mirrors one cync-lan-bridged light/plug as learned from MQTT
// discovery + state messages. UniqueID is cync-lan's stable device
// identifier from the discovery payload. StateTopic/CommandTopic are
// stored directly from that same payload rather than derived by naming
// convention, since cync-lan's actual topic layout may not exactly match
// the generic Home Assistant MQTT JSON schema.
type Device struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	UniqueID           string     `gorm:"uniqueIndex" json:"uniqueId"`
	Name               string     `json:"name"`
	StateTopic         string     `json:"-"`
	CommandTopic       string     `json:"-"`
	SupportsBrightness bool       `json:"supportsBrightness"`
	LastPowerState     string     `json:"lastPowerState"`
	LastBrightness     int        `json:"lastBrightness"`
	LastSeenAt         *time.Time `json:"lastSeenAt"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

func (Device) TableName() string { return "devices" }
