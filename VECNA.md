# Vecna — local house control

A Go+Gin+GORM+Goose+SQLite backend and a React+TS+Tailwind+daisyUI frontend
to turn a GE/Cync ("C by GE"/Enbrighten) smart plug on/off and dim it, over
the local network only.

## Why this isn't a simple "call the plug" app

An nmap sweep of the home LAN identified the plug (`GEMODULE2F76.lan`,
192.168.1.134) with **zero open TCP ports**. GE/Cync devices are cloud-only
by default — they dial out to GE's AWS cloud over MQTT-over-TLS and don't
run a local web server, unlike Tasmota/Shelly/Kasa plugs. Direct local HTTP
control doesn't work against this device as shipped.

Instead, vecna talks to [`cync-lan`](https://github.com/baudneo/cync-lan) (also
packaged as `Proxy-alt/cync-lan-mqtt`), a reverse-engineered bridge that
masquerades as GE's cloud so the plug phones home locally instead. cync-lan
then republishes the device over MQTT using the Home Assistant MQTT JSON
schema, and vecna is an MQTT **client** of that broker — it doesn't replace
cync-lan or talk to the plug directly.

## Running without Docker, from the nix dev shell

`vecna/flake.nix` / `vecna/shell.nix` (`nix develop` or `nix-shell` from
`vecna/`) now include Python, git, and openssl alongside Node/Go/mosquitto,
and put `vecna/scripts/` on `PATH`, so the whole stack — including cync-lan
— can run bare-metal instead of via `docker-compose.yml`:

1. `setup-cync-lan.sh` — one-time: clones baudneo/cync-lan's `python` branch
   into `vecna/cync-lan-src/`, installs it into a venv, and generates TLS
   certs into `vecna/cync-lan-config/certs/`.
2. Copy `vecna/.env.cync-lan.example` to `vecna/.env.cync-lan` and fill in
   `CYNC_ACCOUNT_USERNAME`/`CYNC_ACCOUNT_PASSWORD` and a `CYNC_SECRET_KEY`.
3. `run-mqtt.sh` — bare-metal mosquitto, in its own terminal.
4. `run-cync-lan.sh` — the bridge itself, in its own terminal.
5. `run-server.sh` / `run-frontend.sh` — the Go backend (`:4000`) and Vite
   dev server, each in their own terminal.

Then continue with the manual steps below, using `http://127.0.0.1:23778`
for the export step and this box's LAN IP for the DNS override.

## One-time manual setup (not automatable)

This cannot be done by vecna, by cync-lan's own automation, or by an
assistant — it needs your Cync account and your router's admin access:

1. **Export devices from your Cync cloud account.** With `cync-lan` running
   (`docker compose up -d cync-lan mqtt-broker`, or `run-mqtt.sh` +
   `run-cync-lan.sh` from the nix dev shell — see above), visit
   `http://<host>:23778` and sign in with your Cync account username/password
   + OTP. This pulls down your device list and access keys into
   `vecna/cync-lan-config/`.
2. **Generate TLS certs.** Run cync-lan's `create_certs.sh` (see its README)
   so it can present a cert chain the plug will accept — `setup-cync-lan.sh`
   does this for you in the nix-dev-shell path above.
3. **Override DNS** for `cm.gelighting.com`, `cm-sec.gelighting.com`, and
   `cm-ge.xlink.cn` at your router (or a local DNS server / Pi-hole) to point
   at the host running cync-lan, instead of the real internet.
4. **Power-cycle the plug** so it re-resolves DNS and connects to the local
   cync-lan server instead of GE's cloud.

Open unknown: cync-lan's README implies needing at least one always-on
Wi-Fi Cync device to bridge BLE-mesh devices. It's unclear whether that
requirement applies to this plug, since it may be a direct Wi-Fi device
rather than a mesh one — untested until the above setup is actually done
against the real hardware.

## MQTT contract vecna expects

- **Discovery** (retained): `homeassistant/light/+/config` and
  `homeassistant/switch/+/config`, JSON with `unique_id`, `name`,
  `state_topic`, `command_topic`, and `brightness` (bool) when dimmable.
- **State**: JSON on the device's `state_topic`, e.g.
  `{"state":"ON","brightness":180}`.
- **Commands**: JSON published to the device's `command_topic` —
  `{"state":"ON"|"OFF"}` for power, `{"state":"ON","brightness":<0-255>}`
  for dimming.
- `homeassistant/status` birth/will messages are logged but otherwise
  unused in this MVP.

If a deployed cync-lan version's actual topics/payloads drift from this,
`vecna/server/internal/mqtt/payload.go` is where to adjust decoding, and
`vecna/server/internal/mqtt/subscriber.go` for topic wiring.

## Where things live

- `vecna/` — React+TS+Tailwind+daisyUI frontend (Vite).
- `vecna/server/` — Go+Gin+GORM+Goose+SQLite backend, MQTT client of the
  broker cync-lan also publishes to.
- `docker-compose.yml` services: `vecna` (this app), `mqtt-broker`
  (Mosquitto), `cync-lan` (the bridge) — the latter two are manually
  configured infra per above, not CI'd by this repo.
- `kubernetes/vecna-production.yaml` — included for consistency, but see
  the deployment note below before using it.

## Deployment

Unlike this repo's other production services, vecna/mqtt-broker/cync-lan
need to run on hardware physically on the same LAN as the plug (a home
server, Raspberry Pi, or NAS) — not the existing cloud k8s cluster, unless
that cluster has network reachability into your home LAN. There is no
GitHub Actions production-deploy workflow for this reason; deployment is:

```
docker compose up -d mqtt-broker cync-lan   # complete the manual setup above
docker compose up -d vecna
```

run by hand on that home-LAN host.

## Scope

Single-plug MVP: on/off + dimming for one device, REST API polled by the
frontend every ~2s (no WebSocket/SSE). No rooms, schedules, or scenes. If
more Cync devices get bridged later, the existing schema already supports
multiple rows — the UI would just need a grouping concept added, which is
explicitly deferred for now.
