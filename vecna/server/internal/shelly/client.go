package shelly

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/cragone/lorena-teaches-brazilian/vecna/server/internal/models"
)

const requestTimeout = 5 * time.Second

// Client talks to Shelly Gen2+ devices over their local RPC-over-HTTP API —
// no cloud, no MQTT broker, just the plug on the LAN.
type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{httpClient: &http.Client{Timeout: requestTimeout}}
}

type DeviceInfo struct {
	MAC   string `json:"mac"`
	Model string `json:"model"`
}

func (c *Client) GetDeviceInfo(host string) (DeviceInfo, error) {
	var info DeviceInfo
	if err := c.getJSON(host, "/rpc/Shelly.GetDeviceInfo", &info); err != nil {
		return DeviceInfo{}, err
	}
	if info.MAC == "" {
		return DeviceInfo{}, fmt.Errorf("shelly %s: device info missing mac", host)
	}
	return info, nil
}

type switchStatus struct {
	Output bool `json:"output"`
}

// GetOn reports whether the given relay is currently on.
func (c *Client) GetOn(host string, switchID int) (bool, error) {
	var status switchStatus
	path := fmt.Sprintf("/rpc/Switch.GetStatus?id=%d", switchID)
	if err := c.getJSON(host, path, &status); err != nil {
		return false, err
	}
	return status.Output, nil
}

// SetPower implements handlers.DevicePublisher.
func (c *Client) SetPower(d models.Device, on bool) error {
	path := fmt.Sprintf("/rpc/Switch.Set?id=%d&on=%t", d.SwitchID, on)
	return c.getJSON(d.Host, path, nil)
}

// SetBrightness implements handlers.DevicePublisher. Shelly plugs don't
// support dimming, so this is only reachable if SupportsBrightness is ever
// mistakenly set true for one.
func (c *Client) SetBrightness(_ models.Device, _ int) error {
	return fmt.Errorf("shelly: device does not support brightness")
}

func (c *Client) getJSON(host, path string, out any) error {
	url := fmt.Sprintf("http://%s%s", host, path)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("shelly request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("shelly request to %s: status %d", url, resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("shelly request to %s: decode response: %w", url, err)
	}
	return nil
}
