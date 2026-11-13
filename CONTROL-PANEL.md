# control-panel — home lighting control

A Go+MQTT backend (WebSocket bridge to a Mosquitto broker) and a
Bun+React+Tailwind frontend to control home lighting zones, plus an
ESP32/platformio firmware component for the physical hardware side. App
code lives in its own repo, [cragone/control-panel](https://github.com/cragone/control-panel)
— this repo (monotheism) only carries the deploy orchestration, following
the same pattern as [VECNA.md](VECNA.md) but for a separate repo's code
rather than a monotheism subdirectory.

## Where things live

- Code: [cragone/control-panel](https://github.com/cragone/control-panel)
  (`client/` — Bun+React frontend, `web/` — Go backend, `hardware/` —
  ESP32/platformio firmware, root `Dockerfile` — multi-stage build bundling
  both frontend and backend).
- Deploy orchestration (this repo): `control-panel/scripts/deploy-local.sh`,
  `control-panel/scripts/deploy-raspi5.sh`, `control-panel/mosquitto.conf`,
  `kubernetes/control-panel-production.yaml`,
  `kubernetes/control-panel-ingress.yaml`.
- CI trigger: `.github/workflows/production.yml` in the control-panel repo
  itself (since that's where the code changes land), POSTing to the deploy
  webhook below.

## Deployment (k3s on the raspi5)

The raspi5 host runs its own single-node k3s cluster (added declaratively
to `modules/raspi5.nix` in the
[nixos-config](https://github.com/cragone/nixos-config) flake, via
`services.k3s`, not the `curl | sh` installer), the same way the bw server
hosts vecna. control-panel deploys the same way every other service in
this repo does: `kubernetes/control-panel-production.yaml` (a Mosquitto
broker Deployment+ConfigMap+Service, plus the web Deployment+Service) and
`kubernetes/control-panel-ingress.yaml` (Traefik BasicAuth Middleware +
IngressRoute for `control-panel.charlesragone.com`).

The actual deploy logic lives in `control-panel/scripts/deploy-local.sh`,
which runs *on* the raspi5: pulls the separate `cragone/control-panel`
checkout at `/home/charlie/control-panel`, `docker build`s its image,
imports it directly into k3s's containerd with `k3s ctr images import` (no
registry — this node never needs to be pulled from over the internet), then
applies the manifests and rolls out the new image. It's invoked two ways:

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

`control-panel.charlesragone.com` is reached via a Cloudflare Tunnel
running on the host (not a port-forward), pointed at k3s's built-in Traefik
on `localhost:80`. The same tunnel exposes the deploy webhook at
`deploy-control-panel.charlesragone.com`, pointed directly at the webhook
process rather than through k3s. A BasicAuth Middleware sits in front of
the control-panel IngressRoute (not the webhook, which has its own token
check) so the lighting control UI isn't open to the whole internet; the
htpasswd Secret it reads from is created once by hand (see
`kubernetes/control-panel-ingress.yaml`) and never committed.

### Host setup (`modules/raspi5.nix` in nixos-config)

Declared there, not in this repo: `services.k3s.enable = true;`,
`virtualisation.docker.enable = true;` (needed for the `docker build` step
before handing the image to k3s's containerd), a
`systemd.services.cloudflared-tunnel` unit running
`cloudflared tunnel --config ~/.cloudflared/config.yml run`, a
`systemd.services.control-panel-deploy-webhook` unit running
`webhook -hooks control-panel/scripts/hooks.json -port 8089`, and a scoped
`security.sudo.extraRules` entry letting `charlie` run `k3s` passwordlessly
(needed since the webhook runs unattended, with no terminal to type a sudo
password into).

## Scope

MVP: lighting zones only, on/off via the ESP32 hardware bridge over MQTT, no
persistence beyond the broker's own retained messages (no PVC — unlike
vecna's SQLite history, losing in-flight state on a pod restart isn't a big
deal here).
