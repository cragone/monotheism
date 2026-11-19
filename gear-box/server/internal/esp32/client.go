package esp32

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"
)

// Client pushes synthesized audio to the sound_machine ESP32's /speak
// endpoint. It is a client of the ESP32, not a server for it — the ESP32
// never initiates a connection back to gear-box.
type Client struct {
	Host     string
	APIToken string
	http     *http.Client
}

func New(host, apiToken string) *Client {
	return &Client{
		Host:     host,
		APIToken: apiToken,
		http:     &http.Client{Timeout: 5 * time.Second},
	}
}

// Speak POSTs raw PCM (mono, 16-bit signed little-endian, see piper.SampleRate)
// to the device, which plays it immediately in MODE_SPEAK.
func (c *Client) Speak(ctx context.Context, pcm []byte) error {
	url := fmt.Sprintf("http://%s/speak", c.Host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(pcm))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if c.APIToken != "" {
		req.Header.Set("X-Token", c.APIToken)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("push to esp32: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("esp32 returned status %d", resp.StatusCode)
	}
	return nil
}
