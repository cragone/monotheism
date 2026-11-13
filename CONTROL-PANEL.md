# control-panel — home lighting control + kiosk touchscreen

A Go+MQTT backend (WebSocket bridge to a Mosquitto broker) and a
Bun+React+Tailwind frontend to control home lighting zones, plus an
ESP32/platformio firmware component for the physical hardware side. App
code lives in its own repo, [cragone/control-panel](https://github.com/cragone/control-panel)
— this repo (monotheism) only carries the deploy orchestration.

The raspi5 host is a dedicated kiosk appliance: a small touchscreen
attached to the Pi boots straight into a fullscreen, chrome-less view of
this app (no window manager, no visible browser UI) — it has no other job.

## Where things live

- Code: [cragone/control-panel](https://github.com/cragone/control-panel)
  (`client/` — Bun+React frontend, `web/` — Go backend, `hardware/` —
  ESP32/platformio firmware).
- Deploy orchestration (this repo): `control-panel/scripts/deploy-local.sh`,
  `control-panel/scripts/deploy-raspi5.sh`.
- CI trigger: `.github/workflows/production.yml` in the control-panel repo
  itself (since that's where the code changes land), POSTing to the deploy
  webhook below.

## Deployment (native, on the raspi5 — no containers)

Unlike vecna/bw-server, this host runs a single always-on kiosk display
rather than a cluster, so there's no k3s/Docker here — everything is a
plain systemd service declared in `modules/raspi5.nix` in the
[nixos-config](https://github.com/cragone/nixos-config) flake:

- `services.mosquitto` — the MQTT broker (nixpkgs' built-in module, no
  container).
- `systemd.services.control-panel-web` — runs the built Go binary directly
  (`/home/charlie/control-panel/web/main`), listening on `PORT=8080`.
- `services.cage` — a minimal Wayland kiosk compositor that boots straight
  to tty1 and runs Chromium in `--kiosk` mode (no tabs, address bar, or any
  other browser UI — visually and functionally indistinguishable from a
  native app) pointed at `http://localhost:8080`, GPU-accelerated via
  mainline Mesa's V3D/VC4 driver for the Pi 5's GPU.

The actual deploy logic lives in `control-panel/scripts/deploy-local.sh`,
which runs *on* the raspi5: pulls the separate `cragone/control-panel`
checkout at `/home/charlie/control-panel`, builds the frontend (`bun`) and
backend (`go build`) directly on the host, and restarts
`control-panel-web`. It's invoked two ways:

- **By hand**, from your own machine: `control-panel/scripts/deploy-raspi5.sh [branch]`,
  which just SSHes in and runs `deploy-local.sh` remotely.
- **By CI** (`.github/workflows/production.yml` in the control-panel repo,
  on push to `main`): since both repos are public and the raspi5 is a home
  box, CI deliberately does *not* get SSH access. Instead it POSTs a shared
  secret to a small token-gated webhook running on the host
  (`adnanh/webhook`, config in `control-panel/scripts/hooks.json` — real
  secret never committed, see `hooks.json.example`), which runs
  `deploy-local.sh` for it. The secret is `CONTROL_PANEL_DEPLOY_TOKEN` in
  the control-panel repo's GitHub Actions secrets, and in a local
  `hooks.json` on the host.

The deploy webhook is exposed at `deploy-control-panel.charlesragone.com`
via a Cloudflare Tunnel running on the host (not a port-forward). The
kiosk UI itself is **not** publicly exposed (it was, briefly, behind a
Traefik BasicAuth middleware when this host ran k3s — that went away with
k3s; the UI is now reached only via the attached touchscreen, or on the
home LAN at `http://<raspi5-ip>:8080`).

### Host setup (`modules/raspi5.nix` in nixos-config)

Declared there, not in this repo: `services.mosquitto.enable = true;`,
`systemd.services.control-panel-web`, `services.cage` (kiosk session), a
`systemd.services.cloudflared-tunnel` unit running
`cloudflared tunnel --config ~/.cloudflared/config.yml run`, a
`systemd.services.control-panel-deploy-webhook` unit running
`webhook -hooks control-panel/scripts/hooks.json -port 8089`, and a scoped
`security.sudo.extraRules` entry letting `charlie` restart
`control-panel-web`/`cloudflared-tunnel`/`control-panel-deploy-webhook`
passwordlessly (needed since the webhook runs unattended, with no terminal
to type a sudo password into).

## Scope

MVP: lighting zones only, on/off via the ESP32 hardware bridge over MQTT, no
persistence beyond Mosquitto's own retained messages.
