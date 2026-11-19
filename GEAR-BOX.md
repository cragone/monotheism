# gear-box — local text-to-speech for the house sound machine

A Go+Gin backend and a React+TS+Tailwind+daisyUI frontend that synthesizes
speech with [Piper](https://github.com/rhasspy/piper) (local, offline,
neural TTS) and pushes the finished audio over the LAN to an ESP32-S3
("sound_machine") that plays it through a speaker.

## Why this is simple

gear-box owns the TTS engine; the ESP32 only ever receives. There's no
device registry, no database, and no polling: the website posts text,
gear-box runs Piper, and the resulting PCM gets POSTed straight to the
ESP32's `/speak` endpoint. "Recently spoken" history is a small in-memory
ring buffer — intentionally not persisted, since losing it on a restart is
a fine trade for not needing a database or a PVC for this service. The
ambient noise-machine modes already on the ESP32 (white/pink/brown/rain/
ocean/fan/tone) and their power/volume controls are untouched and
deliberately **not** exposed in gear-box's UI — reach them directly on the
ESP32's own HTTP API on the LAN if you ever need to.

## Hardware: the sound_machine ESP32

Firmware lives at `gear-box/firmware/sound_machine/`. It's an Arduino
sketch for an ESP32-S3, built/flashed with `arduino-cli`:

```
arduino-cli compile --fqbn esp32:esp32:esp32s3 gear-box/firmware/sound_machine
arduino-cli upload  --fqbn esp32:esp32:esp32s3 -p <serial-port> gear-box/firmware/sound_machine
```

Before compiling, copy `secrets.example.h` to `secrets.h` in that same
directory (gitignored, never committed) and fill in your home WiFi
SSID/password and, optionally, an `API_TOKEN` the ESP32 will require on
every request (gear-box sends it as `X-Token` if `ESP32_API_TOKEN` is set).

The device joins your home WiFi and answers to `soundmachine.local` via
mDNS, but **production should point `ESP32_HOST` at a static LAN IP
instead** — relying on mDNS resolving from inside a k3s pod's network
namespace is an extra moving part not worth debugging. Find its IP via your
router's device list or `nmap -sn <subnet>`, then set a DHCP reservation so
it doesn't change.

### Wire contract: `POST /speak`

gear-box is the only thing that ever calls this. Body is **raw PCM: mono,
16-bit signed little-endian, 22050Hz — no WAV header, no length prefix
beyond the HTTP `Content-Length` header itself**. A GET to the same path
returns 405. This must stay in sync with `piper.SampleRate`/
`piper.MaxOutputBytes` in `gear-box/server/internal/piper/piper.go` and
`SPEAK_MAX_BYTES` in the firmware — it's the one seam between two
independently-deployed pieces of code.

The firmware's body-read path (`gear-box/firmware/sound_machine/sound_machine.ino`,
`handleSpeakPost`) reads the request directly off the socket rather than
through `WebServer`'s `server.arg("plain")`, to avoid buffering a ~260KB
body into an Arduino `String` on a device already tight on heap. **This has
not been verified against the installed `esp32:esp32` Arduino core's exact
`WebServer` body-handling behavior — confirm by flashing and testing with
`curl` (see Verification below) before trusting it in production.**

## Local dev (no Docker)

`gear-box/flake.nix` / `gear-box/shell.nix` (`nix develop` or `nix-shell`
from `gear-box/`) put `gear-box/scripts/` on `PATH` and include `piper-tts`
from nixpkgs:

- `run-server.sh` — the Go backend (`:8083`). Requires `ESP32_HOST` (the
  device's real LAN IP/hostname) — **the ESP32 can't be mocked**, it's a
  physical device, so local dev talks to the real one. `PIPER_MODEL_PATH`
  defaults to `./models/voice.onnx`; point it at a real voice model you've
  downloaded locally (see [Piper's voice samples](https://rhasspy.github.io/piper-samples/)
  — pick one, download its `.onnx`/`.onnx.json` pair into `gear-box/models/`,
  which is gitignored).
- `run-frontend.sh` — the Vite dev server, proxying `/api` to the backend.

There's no basic-auth gate in local dev — Traefik's BasicAuth Middleware
only exists in the production ingress (see Deployment below).

Or from the repo root's shared dev shell: `run-gearbox` (same two processes,
same ports).

## API

- `POST /api/speak` — `{"text": "..."}` (max 200 chars). Synthesizes with
  Piper, pushes the result to the ESP32, records it in history.
- `GET /api/history` — the last 20 spoken lines, newest first.
- `GET /api/health`

## Where things live

- `gear-box/` — React+TS+Tailwind+daisyUI frontend (Vite).
- `gear-box/server/` — Go+Gin backend. No GORM/SQLite/migrations — this is
  the one deliberate scope-cut from vecna's stack, since there's no
  persistent state worth a database here.
- `gear-box/firmware/sound_machine/` — the ESP32-S3 Arduino sketch.
- `docker-compose.yml` service: `gear-box` — reads `ESP32_HOST` from the
  environment.
- `kubernetes/gear-box-production.yaml` / `kubernetes/gear-box-ingress.yaml`
  — production manifests (see Deployment below).

## Deployment

Same situation as vecna: gear-box needs to run on hardware physically on
the same LAN as the ESP32, so it deploys to **the bw server**
(`192.168.1.91`), which already runs vecna on its own single-node k3s
cluster — not the main cloud cluster.

The actual deploy logic lives in `gear-box/scripts/deploy-local.sh`, which
runs *on* the bw server: pulls, `docker build`s the image from
`gear-box/dockerfile` (which also fetches the Piper binary and a voice
model at build time — see the Dockerfile's comments, and verify the
HuggingFace `piper-voices` URL shape still matches before relying on it in
CI), imports it directly into k3s's containerd with `k3s ctr images
import` (no registry), then applies the manifests and rolls out the new
image. Invoked two ways:

- **By hand**: `gear-box/scripts/deploy-bw-server.sh [branch]` — SSHes in
  and runs `deploy-local.sh` remotely.
- **By CI** (`.github/workflows/production_gear_box.yml`, on push to
  `main`): same reasoning as vecna — no SSH access for CI on a public repo
  controlling physical hardware. It POSTs a shared secret
  (`GEAR_BOX_DEPLOY_TOKEN`, in both the repo's GitHub Actions secrets and a
  local `.env`) to a token-gated webhook on the host (config in
  `gear-box/scripts/hooks.json`, never committed — see
  `hooks.json.example`), which runs `deploy-local.sh`.

`gear-box.charlesragone.com` is reached via the same Cloudflare Tunnel
already running on the host for vecna, pointed at k3s's built-in Traefik on
`localhost:80`. The deploy webhook needs its own tunnel route too
(`deploy-gear-box.charlesragone.com`), pointed at the webhook process.

### Auth: reuses vecna's exact login

`kubernetes/gear-box-ingress.yaml` points its `IngressRoute` at the
**existing** `vecna-basic-auth` Traefik Middleware/Secret from
`kubernetes/vecna-ingress.yaml` — no new Secret is created. The same
username/password that unlocks `vecna.charlesragone.com` unlocks
`gear-box.charlesragone.com`.

### Host setup (external, not in this repo)

On the bw server's own `nixos-config` flake, alongside vecna's existing
`vecna-deploy-webhook` unit: either add gear-box's hook object into that
same `webhook -hooks ... ` invocation's hooks file (simplest — one webhook
process can serve multiple hook ids), or stand up a second
`systemd.services.gear-box-deploy-webhook` unit on its own port. Either way,
add a Cloudflare Tunnel ingress rule for `gear-box.charlesragone.com` (→
`localhost:80`, shared with vecna) and `deploy-gear-box.charlesragone.com`
(→ the webhook port).

## Verification before trusting a deploy

1. **Piper alone**: `piper --model <model> --output-raw --sample_rate 22050 <<< "test" > out.pcm`.
2. **Firmware alone**: flash the rewritten sketch, then
   `curl -X POST --data-binary @out.pcm -H "Content-Type: application/octet-stream" http://<esp32-ip>/speak`
   — confirms the POST handler and playback work, and resolves the
   body-read question flagged above, before any Go code is involved.
3. **gear-box server, local, against the real device**:
   `ESP32_HOST=<esp32-ip> run-server.sh`, then
   `curl -X POST localhost:8083/api/speak -d '{"text":"hello"}' -H 'Content-Type: application/json'`.
4. **Frontend, local**: `run-frontend.sh`, use the real page in a browser.
5. **Container build, local, still against the real device**:
   `docker build -t gear-box:dev -f gear-box/dockerfile gear-box/` then
   `docker run -p 8083:80 -e ESP32_HOST=<esp32-ip> gear-box:dev` — catches
   any Piper-fetch/packaging issue before it reaches the bw server.
6. Only after 1–5 pass, run `deploy-bw-server.sh` by hand once before
   trusting CI's webhook trigger.
