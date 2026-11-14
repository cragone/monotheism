package shelly

import (
	"gorm.io/gorm"

	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/models"
)

// Registrar adapts Register to a narrow interface the HTTP handlers can
// depend on, mirroring how DevicePublisher narrows *Client for SetPower.
type Registrar struct {
	db     *gorm.DB
	client *Client
}

func NewRegistrar(db *gorm.DB, client *Client) *Registrar {
	return &Registrar{db: db, client: client}
}

func (r *Registrar) Register(host, name string, deviceType models.DeviceType) (models.Device, error) {
	return Register(r.db, r.client, host, name, deviceType)
}
