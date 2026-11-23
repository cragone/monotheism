#pragma once

// Copy this file to secrets.h and fill in your own values.
// secrets.h is gitignored and must never be committed.

#define WIFI_SSID "your-2.4ghz-ssid"
#define WIFI_PASS "your-wifi-password"

// gear-box's hostname, no scheme/port (sendUtterance always connects over
// HTTPS -- see mic_listener.ino). In production this is
// gear-box.charlesragone.com, reached through the Cloudflare Tunnel via a
// path-scoped bypass of vecna-basic-auth for /hear specifically (see
// kubernetes/gear-box-ingress.yaml) -- the LAN NodePort
// (gear-box-production.yaml) is still there but turned out to be blocked
// for this device specifically by something that looked like router-level
// isolation, so this goes out over the internet and back instead of
// fighting that. In local dev, point this at wherever run-server.sh is
// running instead (e.g. "192.168.1.x:8083") -- that'll need a plain HTTP
// client, not WiFiClientSecure, so don't use this file's value verbatim
// for local dev without also adjusting sendUtteranceOnce.
#define GEARBOX_HOST "gear-box.charlesragone.com"

// Sent as X-Token on every /hear POST. No longer optional in practice:
// /hear is reachable from the public internet now, so gear-box's
// MIC_API_TOKEN must be set to a real value in production, and this must
// match it exactly.
#define GEARBOX_API_TOKEN "replace-with-the-real-token"
