#pragma once

// Copy this file to secrets.h and fill in your own values.
// secrets.h is gitignored and must never be committed.

#define WIFI_SSID "your-2.4ghz-ssid"
#define WIFI_PASS "your-wifi-password"

// gear-box's LAN address. In production this is the bw server's LAN IP
// on gear-box's NodePort (see kubernetes/gear-box-production.yaml) --
// gear-box.charlesragone.com is gated by basic auth, which a device can't
// log into, so this NodePort is the only LAN-reachable, unauthenticated
// path in. In local dev, point this at wherever run-server.sh is running
// instead (e.g. "192.168.1.x:8083").
#define GEARBOX_HOST "192.168.1.91:30080"

// Sent as X-Token on every /hear POST if gear-box's MIC_API_TOKEN is set.
// Leave empty to match an unset MIC_API_TOKEN on the server.
#define GEARBOX_API_TOKEN ""
