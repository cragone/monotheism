package models

import "slices"

// DeviceType controls only which icon/emoji the frontend shows for a
// device — on/off control logic is identical for every type.
type DeviceType string

const (
	DeviceTypeLight        DeviceType = "light"
	DeviceTypeSoundMachine DeviceType = "sound_machine"
	DeviceTypeGarage       DeviceType = "garage"
	DeviceTypeTemp         DeviceType = "temp"
)

var ValidDeviceTypes = []DeviceType{DeviceTypeLight, DeviceTypeSoundMachine, DeviceTypeGarage, DeviceTypeTemp}

func (t DeviceType) Valid() bool {
	return slices.Contains(ValidDeviceTypes, t)
}
