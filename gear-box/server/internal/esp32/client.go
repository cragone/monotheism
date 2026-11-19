package esp32

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

// SoundStatus mirrors the subset of the ESP32's own /status JSON that the
// ambient-noise controls care about (on/off, active mode, volume). The
// device returns this same shape from /on, /off, /mode and /volume too, so
// every mutating call below can report the resulting state in one round
// trip instead of a follow-up /status fetch.
type SoundStatus struct {
	On     bool    `json:"on"`
	Mode   string  `json:"mode"`
	Volume float64 `json:"volume"`
}

func (c *Client) statusRequest(ctx context.Context, path string) (SoundStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("http://%s%s", c.Host, path), nil)
	if err != nil {
		return SoundStatus{}, err
	}
	if c.APIToken != "" {
		req.Header.Set("X-Token", c.APIToken)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return SoundStatus{}, fmt.Errorf("esp32 request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return SoundStatus{}, fmt.Errorf("esp32 returned status %d", resp.StatusCode)
	}
	var status SoundStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return SoundStatus{}, fmt.Errorf("decode esp32 status: %w", err)
	}
	return status, nil
}

// Status fetches the device's current on/off, mode, and volume.
func (c *Client) Status(ctx context.Context) (SoundStatus, error) {
	return c.statusRequest(ctx, "/status")
}

// SetPower turns the ambient sound on or off.
func (c *Client) SetPower(ctx context.Context, on bool) (SoundStatus, error) {
	if on {
		return c.statusRequest(ctx, "/on")
	}
	return c.statusRequest(ctx, "/off")
}

// SetMode switches which ambient noise generator is playing.
func (c *Client) SetMode(ctx context.Context, mode string) (SoundStatus, error) {
	return c.statusRequest(ctx, "/mode?name="+url.QueryEscape(mode))
}

// SetVolume sets playback volume, 0.0-1.0.
func (c *Client) SetVolume(ctx context.Context, level float64) (SoundStatus, error) {
	return c.statusRequest(ctx, fmt.Sprintf("/volume?level=%.2f", level))
}

// Modes lists the device's ambient noise generators. "speak" is excluded:
// it's the internal mode the device switches itself into when gear-box
// pushes TTS audio to /speak, not something a person should pick from a
// mode list.
func (c *Client) Modes(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s/modes", c.Host), nil)
	if err != nil {
		return nil, err
	}
	if c.APIToken != "" {
		req.Header.Set("X-Token", c.APIToken)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("esp32 request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("esp32 returned status %d", resp.StatusCode)
	}
	var body struct {
		Modes []string `json:"modes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode esp32 modes: %w", err)
	}

	modes := make([]string, 0, len(body.Modes))
	for _, m := range body.Modes {
		if m != "speak" {
			modes = append(modes, m)
		}
	}
	return modes, nil
}
