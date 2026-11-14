# control-panel — kiosk touchscreen front-end for vecna

A native Rust (Slint) kiosk display, not a browser/webview: talks directly
to [vecna](VECNA.md)'s HTTP API for device names, type, and state, and sends
power toggles back to it. vecna is the single source of truth for both
naming and actual control — there's no local MQTT broker, Go backend, or
ESP32 hardware bridge involved on this host at all. App code lives in this
repo, under `control-panel/kiosk/` (it used to live in a separate
`cragone/control-panel` repo — that's been folded in here so the whole stack
is one repo; the old repo's `kiosk/` code and CI workflow were removed).

The raspi5 host is a dedicated kiosk appliance: a small touchscreen attached
to the Pi boots straight into a fullscreen, chrome-less view of this app (no
window manager, no browser at all) — it has no other job.

## Where things live

- Code: `control-panel/kiosk/` (this repo) — the Rust/Slint kiosk app.
  (`client/`, `web/`, `hardware/` from the old separate repo were leftover
  MVP pieces from before this talked to vecna directly, already unbuilt and
  undeployed there, and were not carried over.)
- Deploy orchestration: `control-panel/scripts/deploy-local.sh`,
  `control-panel/scripts/deploy-raspi5.sh`.
- CI trigger: `.github/workflows/production_control_panel.yml` in this
  repo, on push to `main` touching `control-panel/**`, POSTing to the
  deploy webhook below.

## How it talks to vecna

The kiosk polls `GET https://vecna.charlesragone.com/api/devices` every ~2s
(matching vecna's own web frontend's polling — that API has no
WebSocket/SSE), rendering one tile per device returned with an emoji for its
`deviceType` (light 💡 / sound_machine 🔊 / garage 🚪 / temp 🌡️, mirroring
`vecna/src/deviceTypes.ts` exactly — on/off control is identical regardless
of type), and POSTs `{"on": bool}` to `/api/devices/:id/power` when a tile
is tapped.

Authentication is HTTP BasicAuth with a **dedicated** htpasswd user,
`raspi5-kiosk`, added to vecna's existing `vecna-basic-auth` k3s Secret on
bw-server alongside the human login (`ragoglynn`) — not shared with it. The
kiosk reads `VECNA_API_USER`/`VECNA_API_PASSWORD` from the environment; on
the raspi5 host these come from an `EnvironmentFile` at
`/home/charlie/.vecna-kiosk-env`, created once by hand on the host and never
committed (same pattern as the deploy webhook's `hooks.json`).

## Deployment (native, on the raspi5 — no containers)

Unlike vecna/bw-server, this host runs a single always-on kiosk display
rather than a cluster, so there's no k3s/Docker here — everything is a
plain systemd service declared in `modules/raspi5.nix` in the
[nixos-config](https://github.com/cragone/nixos-config) flake:

- `services.cage` — a minimal Wayland kiosk compositor that boots straight
  to tty1 and runs the kiosk binary fullscreen, GPU-accelerated via mainline
  Mesa's V3D/VC4 driver where available (this board only exposes a
  display/KMS-only DRM node with no separate render node, so
  `SLINT_BACKEND=winit-software` forces Slint's software rasterizer, which
  reliably presents through cage's own pixman-based compositing — the
  default GPU/femtovg path renders nothing at all on this hardware).

The actual deploy logic lives in `control-panel/scripts/deploy-local.sh`,
which runs *on* the raspi5: pulls this repo's checkout at
`/home/charlie/monotheism`, `cargo build --release`s `control-panel/kiosk`
directly on the host, and restarts the `cage-tty1` session. It's invoked
two ways:

- **By hand**, from your own machine: `control-panel/scripts/deploy-raspi5.sh [branch]`,
  which just SSHes in and runs `deploy-local.sh` remotely.
- **By CI** (`.github/workflows/production_control_panel.yml` in this
  repo, on push to `main`): since this repo is public and the raspi5 is a
  home box, CI deliberately does *not* get SSH access. Instead it POSTs a
  shared secret to a small token-gated webhook running on the host
  (`adnanh/webhook`, config in `control-panel/scripts/hooks.json` — real
  secret never committed, see `hooks.json.example`), which runs
  `deploy-local.sh` for it. The secret is `CONTROL_PANEL_DEPLOY_TOKEN` in
  this repo's GitHub Actions secrets, and in a local `hooks.json` on the
  host.

The deploy webhook is exposed at `deploy-control-panel.charlesragone.com`
via a Cloudflare Tunnel running on the host (not a port-forward). The kiosk
UI itself is **not** publicly exposed — it's reached only via the attached
touchscreen.

### Host setup (`modules/raspi5.nix` in nixos-config)

One-time prerequisite: a checkout of this repo (monotheism) at
`/home/charlie/monotheism` on the raspi5 (same convention as the bw-server
uses for vecna) — `deploy-local.sh` pulls and builds from there now instead
of the old separate `control-panel` checkout.

Declared in `modules/raspi5.nix`, not in this repo: `services.cage` (kiosk session, running
`control-panel/kiosk`'s binary with `SLINT_BACKEND=winit-software` and an
`EnvironmentFile` pointing at the uncommitted vecna credentials), a
`systemd.services.cloudflared-tunnel` unit running
`cloudflared tunnel --config ~/.cloudflared/config.yml run`, a
`systemd.services.control-panel-deploy-webhook` unit running
`webhook -hooks control-panel/scripts/hooks.json -port 8089`, and a scoped
`security.sudo.extraRules` entry letting `charlie` restart
`cage-tty1`/`cloudflared-tunnel`/`control-panel-deploy-webhook`
passwordlessly (needed since the webhook runs unattended, with no terminal
to type a sudo password into).

## Scope

MVP: whatever devices vecna reports, on/off only, one tile per device with
its type's emoji (vecna also supports brightness for some devices, not yet
exposed in this UI). No local persistence — vecna's own device records are
the only state that matters.
