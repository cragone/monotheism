# Vecna — local house control

A Go+Gin+GORM+Goose+SQLite backend and a React+TS+Tailwind+daisyUI frontend
to turn a Shelly smart plug on/off over the local network only.

## Why this is simple

Unlike GE/Cync ("C by GE") plugs, which are cloud-only by default and need a
reverse-engineered bridge (`cync-lan`) plus an MQTT broker in between, Shelly
Gen2+ plugs run a real local web server and expose a JSON-RPC API directly
over HTTP — no cloud, no broker, no bridge. vecna's Go backend is simply an
HTTP client of the plug itself.

## One-time plug setup (not automatable)

This needs physical access to the plug and your home WiFi credentials — it
can't be scripted ahead of time:

1. Plug it in. It broadcasts its own setup WiFi AP (something like
   `ShellyPlugUSG4-XXXXXXXXXXXX`).
2. Join that AP from a phone or laptop, then open `http://192.168.33.1` and
   submit your home WiFi SSID/password through the Shelly's own config page.
3. It reboots and joins your home network, dropping its own AP. Find its new
   IP/hostname (e.g. via `nmap -sn <subnet>` or your router's device list —
   it also answers to `<device-id>.lan` via mDNS).
4. (Recommended) Set a DHCP reservation on your router so its IP doesn't
   change later.

## Running without Docker, from the nix dev shell

`vecna/flake.nix` / `vecna/shell.nix` (`nix develop` or `nix-shell` from
`vecna/`) put `vecna/scripts/` on `PATH`:

- `run-server.sh` — the Go backend (`:4000`). Requires `SHELLY_HOST` (the
  plug's LAN IP/hostname from setup above) to be set in the environment.
- `run-frontend.sh` — the Vite dev server, proxying `/api` to the backend.

## Local RPC contract vecna expects

Shelly's Gen2+ API, called directly over HTTP:

- `GET /rpc/Shelly.GetDeviceInfo` — used once at startup to read the plug's
  MAC address, which becomes the device's stable `unique_id`.
- `GET /rpc/Switch.GetStatus?id=0` — polled every few seconds to pick up
  state changes from the physical button or the Shelly app, not just
  vecna's own commands.
- `GET /rpc/Switch.Set?id=0&on=true|false` — the power command.

If a device model's actual RPC shape drifts from this,
`vecna/server/internal/shelly/client.go` is where to adjust it.

## Where things live

- `vecna/` — React+TS+Tailwind+daisyUI frontend (Vite).
- `vecna/server/` — Go+Gin+GORM+Goose+SQLite backend, local HTTP client of
  the Shelly plug.
- `docker-compose.yml` service: `vecna` (this app) — reads `SHELLY_HOST` /
  `SHELLY_NAME` from the environment.
- `kubernetes/vecna-production.yaml` — included for consistency, but see
  the deployment note below before using it.

## Deployment

Unlike this repo's other production services, vecna needs to run on
hardware physically on the same LAN as the plug (a home server, Raspberry
Pi, or NAS) — not the existing cloud k8s cluster, unless that cluster has
network reachability into your home LAN. There is no GitHub Actions
production-deploy workflow for this reason; deployment is:

```
SHELLY_HOST=192.168.1.26 docker compose up -d vecna
```

run by hand on that home-LAN host.

### k3s deploy (current setup: the "bw server")

The bw server (`192.168.1.91`) runs its own single-node k3s cluster (added
declaratively to its `/etc/nixos/configuration.nix` via `services.k3s`, not
the `curl | sh` installer), so vecna deploys the same way every other service
in this repo does: `kubernetes/vecna-production.yaml` (Deployment + PVC +
Service) and `kubernetes/vecna-ingress.yaml` (Traefik BasicAuth Middleware +
IngressRoute for `vecna.charlesragone.com`).

`vecna/scripts/deploy-bw-server.sh [branch]` SSHes to that host, pulls,
`docker build`s the image from `vecna/dockerfile`, imports it directly into
k3s's containerd with `k3s ctr images import` (no registry — this node never
needs to be pulled from over the internet), then applies the manifests and
rolls out the new image.

`vecna.charlesragone.com` is reached via a Cloudflare Tunnel running on the
host (not a port-forward — the host's public IP is IPv6-only anyway), pointed
at k3s's built-in Traefik on `localhost:80`. A BasicAuth Middleware sits in
front of the IngressRoute so the plug control UI isn't open to the whole
internet; the htpasswd Secret it reads from is created once by hand (see
`kubernetes/vecna-ingress.yaml`) and never committed.

## Scope

Single-plug MVP: on/off for one device, REST API polled by the frontend
every ~2s (no WebSocket/SSE). No rooms, schedules, scenes, or dimming
(Shelly plugs are on/off only). If more Shelly devices get added later, the
existing schema already supports multiple rows — the UI would just need a
grouping concept added, which is explicitly deferred for now.
