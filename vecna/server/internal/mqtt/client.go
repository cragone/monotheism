package mqtt

import (
	"log"

	paho "github.com/eclipse/paho.mqtt.golang"
	"gorm.io/gorm"

	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/config"
)

// Connect dials the MQTT broker that cync-lan also publishes to, wiring up
// discovery/state subscriptions on every (re)connect, and returns a ready
// Publisher for the handlers to send commands through.
func Connect(cfg config.Config, db *gorm.DB) (*Publisher, error) {
	subscriber := NewSubscriber(db)

	opts := paho.NewClientOptions().
		AddBroker(cfg.MQTTBrokerURL).
		SetClientID("vecna-server").
		SetAutoReconnect(true).
		SetOnConnectHandler(func(c paho.Client) {
			log.Printf("mqtt: connected to %s", cfg.MQTTBrokerURL)
			subscriber.Subscribe(c)
		}).
		SetConnectionLostHandler(func(_ paho.Client, err error) {
			log.Printf("mqtt: connection lost: %v", err)
		})

	if cfg.MQTTUsername != "" {
		opts.SetUsername(cfg.MQTTUsername)
		opts.SetPassword(cfg.MQTTPassword)
	}

	client := paho.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return nil, err
	}

	return NewPublisher(client), nil
}
