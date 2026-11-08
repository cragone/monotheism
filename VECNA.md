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

### Manual bare-metal deploy (current setup: the "bw server")

`vecna/scripts/deploy-bw-server.sh [branch]` SSHes to the home-LAN host,
pulls, builds frontend+backend inside `nix develop`, and (re)starts the
server in a detached tmux session (`tmux attach -t vecna` to watch it) —
no CI, no systemd unit, just a manual script for now.

It serves on port 80, which on Linux needs either root or the
`CAP_NET_BIND_SERVICE` capability — the script re-applies that capability
to the freshly built binary on every deploy (`setcap` loses it on rebuild
since each build is a new file/inode). That `setcap` call needs root, so
the target host's NixOS config needs both the port opened and a sudoers
rule scoping passwordless `setcap` to exactly that one command:

```nix
networking.firewall.allowedTCPPorts = [ 80 ];

security.sudo.extraRules = [
  {
    users = [ "charlie" ];
    commands = [
      {
        command = "/run/current-system/sw/bin/setcap cap_net_bind_service=+ep /tmp/vecna-server";
        options = [ "NOPASSWD" ];
      }
    ];
  }
];
```

then `sudo nixos-rebuild switch` once on that host.

## Scope

Single-plug MVP: on/off for one device, REST API polled by the frontend
every ~2s (no WebSocket/SSE). No rooms, schedules, scenes, or dimming
(Shelly plugs are on/off only). If more Shelly devices get added later, the
existing schema already supports multiple rows — the UI would just need a
grouping concept added, which is explicitly deferred for now.
