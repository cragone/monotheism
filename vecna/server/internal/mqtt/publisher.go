package mqtt

import (
	"fmt"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/models"
)

const publishTimeout = 5 * time.Second

type Publisher struct {
	client paho.Client
}

func NewPublisher(client paho.Client) *Publisher {
	return &Publisher{client: client}
}

func (p *Publisher) SetPower(d models.Device, on bool) error {
	return p.publish(d.CommandTopic, EncodePowerCommand(on))
}

func (p *Publisher) SetBrightness(d models.Device, level int) error {
	return p.publish(d.CommandTopic, EncodeBrightnessCommand(level))
}

func (p *Publisher) publish(topic string, payload []byte) error {
	token := p.client.Publish(topic, 1, false, payload)
	if !token.WaitTimeout(publishTimeout) {
		return fmt.Errorf("publish to %s: timed out", topic)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("publish to %s: %w", topic, err)
	}
	return nil
}
