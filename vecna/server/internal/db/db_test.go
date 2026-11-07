package db

import (
	"testing"

	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/models"
)

func TestOpenAndDeviceRoundTrip(t *testing.T) {
	gormDB, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	device := models.Device{
		UniqueID:           "plug-1",
		Name:               "Office Plug",
		StateTopic:         "cync_lan/plug-1/state",
		CommandTopic:       "cync_lan/plug-1/set",
		SupportsBrightness: true,
	}
	if err := gormDB.Create(&device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	if device.ID == 0 {
		t.Fatalf("expected device ID to be set after create")
	}

	var fetched models.Device
	if err := gormDB.First(&fetched, device.ID).Error; err != nil {
		t.Fatalf("fetch device: %v", err)
	}
	if fetched.UniqueID != device.UniqueID || fetched.Name != device.Name {
		t.Fatalf("round-tripped device mismatch: got %+v, want %+v", fetched, device)
	}
	if !fetched.SupportsBrightness {
		t.Fatalf("expected SupportsBrightness to round-trip as true")
	}

	dup := models.Device{UniqueID: device.UniqueID}
	if err := gormDB.Create(&dup).Error; err == nil {
		t.Fatalf("expected unique constraint violation on duplicate unique_id")
	}
}
