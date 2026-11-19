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
		// The firmware streams the body into a small ring buffer paced to
		// real-time playback (not transferred as fast as the network
		// allows), so request duration scales with the clip's length, up
		// to MaxTextChars worth of speech (~12-15s) -- plus the ESP32's
		// WiFi signal is middling and occasionally flaky (observed RSSI
		// around -55 to -70dBm). 30s covers a near-cap-length clip with
		// margin; a plain retry covers a single dropped connection.
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// Speak POSTs raw PCM (mono, 16-bit signed little-endian, see piper.SampleRate)
// to the device, which plays it immediately in MODE_SPEAK. Retries once on
// failure since a single dropped WiFi packet shouldn't fail the whole request.
func (c *Client) Speak(ctx context.Context, pcm []byte) error {
	err := c.speakOnce(ctx, pcm)
	if err == nil {
		return nil
	}
	time.Sleep(1 * time.Second)
	return c.speakOnce(ctx, pcm)
}

func (c *Client) speakOnce(ctx context.Context, pcm []byte) error {
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
