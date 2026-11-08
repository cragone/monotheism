package models

import "time"

// Device is one Shelly plug/switch vecna controls directly over its local
// Gen2+ RPC API (no cloud, no broker in between). Host is the plug's LAN
// hostname/IP and SwitchID the relay index on it (always 0 for a single-relay
// plug, but Shelly's API numbers relays per-device for multi-channel models).
type Device struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	UniqueID           string     `gorm:"uniqueIndex" json:"uniqueId"`
	Name               string     `json:"name"`
	Host               string     `json:"-"`
	SwitchID           int        `json:"-"`
	SupportsBrightness bool       `json:"supportsBrightness"`
	LastPowerState     string     `json:"lastPowerState"`
	LastBrightness     int        `json:"lastBrightness"`
	LastSeenAt         *time.Time `json:"lastSeenAt"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

func (Device) TableName() string { return "devices" }
