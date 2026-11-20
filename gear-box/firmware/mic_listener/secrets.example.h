#pragma once

// Copy this file to secrets.h and fill in your own values.
// secrets.h is gitignored and must never be committed.

#define WIFI_SSID "your-2.4ghz-ssid"
#define WIFI_PASS "your-wifi-password"

// gear-box's LAN address, e.g. "192.168.1.91:8083" -- same host the
// sound_machine's ESP32_HOST points at, since gear-box is what bridges the
// two devices. Not a hostname you'd reach from outside the LAN.
#define GEARBOX_HOST "gear-box.local:8083"

// Sent as X-Token on every /hear POST if gear-box's MIC_API_TOKEN is set.
// Leave empty to match an unset MIC_API_TOKEN on the server.
#define GEARBOX_API_TOKEN ""
