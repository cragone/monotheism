# gear-box — local text-to-speech for the house sound machine

A Go+Gin backend and a React+TS+Tailwind+daisyUI frontend that synthesizes
speech with [Piper](https://github.com/rhasspy/piper) (local, offline,
neural TTS) and pushes the finished audio over the LAN to an ESP32-S3
("sound_machine") that plays it through a speaker. A second ESP32-S3
("mic_listener") captures audio with an I2S mic and pushes what it heard
back to gear-box, closing the loop — see "Hardware: the mic_listener
ESP32" below.

## Why this is simple

gear-box owns the TTS engine; the ESP32 only ever receives. There's no
device registry, no database, and no polling: the website posts text,
gear-box runs Piper, and the resulting PCM gets POSTed straight to the
ESP32's `/speak` endpoint. gear-box doesn't retain any record of what was
spoken — no history, in memory or otherwise — the text is synthesized,
pushed, and forgotten. The ESP32 is speech-only: it used to also run a set
of ambient noise-machine generators (white/pink/brown/rain/ocean/fan/tone),
but driving them at high volume was pulling enough current through the I2S
amp to brown out the chip mid-playback, so that whole feature — the
generators in firmware and the mode/volume/power controls in gear-box's
UI — was removed rather than chasing a safe volume ceiling for a feature
nobody needed.

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
`SPEAK_MAX_BYTES`/`SPEAK_RING_BYTES` in the firmware — it's the one seam
between two independently-deployed pieces of code.

The firmware **streams** the body into a small fixed-size ring buffer
(`speakRing`, 16KB, static — never resized) as it arrives, via `WebServer`'s
raw-body callback mechanism (`handleSpeakRaw`, registered as the upload
function in `server.on()`), rather than buffering a whole clip in one
`malloc`. Two things ruled out the more obvious approaches, both found by
testing against the real device:
- `server.arg("plain")` buffers the body through a C-string
  (`String(plainBuf)`), which truncates at the first zero byte — and raw
  16-bit PCM is full of zero bytes.
- A single `malloc`'d buffer per clip (even freeing the previous one before
  allocating the next) ran into real heap fragmentation on this chip: a
  handful of requests dropped `ESP.getMaxAllocHeap()` to well under 100KB
  regardless of allocation order, making more than a couple of seconds of
  speech unreliable. A small *static* ring sidesteps this for
  clips of any length, since nothing is ever allocated from the heap for
  this feature at all — confirmed by pushing clips up to 650KB with a flat,
  stable `free_heap`/`max_alloc_heap` across repeated varied-size requests
  (`/status` reports both fields for exactly this kind of check).
- Streaming also means request duration now scales with the clip's length
  (paced to real-time playback, not transferred as fast as the network
  allows) — `esp32.Client`'s 30s timeout accounts for a near-`MaxTextChars`
  clip.

## Hardware: the mic_listener ESP32

Firmware lives at `gear-box/firmware/mic_listener/` — a second, separate
ESP32-S3 (specifically an ESP32-S3-WROOM-2, octal flash/PSRAM) with an I2S
MEMS mic, built/flashed the same way as sound_machine:

```
arduino-cli compile --fqbn esp32:esp32:esp32s3:FlashMode=opi,PSRAM=opi,FlashSize=16M gear-box/firmware/mic_listener
arduino-cli upload  --fqbn esp32:esp32:esp32s3:FlashMode=opi,PSRAM=opi,FlashSize=16M -p <serial-port> gear-box/firmware/mic_listener
```

Verify those `FlashMode=opi,PSRAM=opi` option keys against your installed
core (`arduino-cli board details -b esp32:esp32:esp32s3`) — WROOM-2 is
always octal flash and needs octal PSRAM selected explicitly; the plain
ESP32S3 Dev Module board entry's defaults are for quad flash/PSRAM modules.

Pin assignments, sample rate, and the mic-capture internals (FreeRTOS task
+ ring buffer, in `mic.c`/`mic.h`) are documented in `mic_config.h`'s
comments. Like sound_machine, copy `secrets.example.h` to `secrets.h`
(gitignored) and fill in WiFi credentials, `GEARBOX_HOST` (gear-box's LAN
address), and optionally `GEARBOX_API_TOKEN`.

The firmware runs a crude RMS-threshold voice-activity gate (see
`MIC_VAD_*` in `mic_config.h`): once a window of audio crosses a loudness
threshold it starts buffering an "utterance" into a PSRAM-backed buffer
(allocated once at boot, reused forever — the same anti-fragmentation
reasoning as `speakRing`/`micRing`), and once RMS has stayed below
threshold for `MIC_VAD_SILENCE_MS` it POSTs the whole utterance to
gear-box and resets. There's no real VAD/noise-floor calibration — the
threshold is a hand-picked constant, tune it against your actual room.

### Reaching gear-box from the LAN: the NodePort

gear-box's production `Service` is `NodePort`, not `ClusterIP` (see
`kubernetes/gear-box-production.yaml`) — the bw server's LAN IP on port
**30080** reaches gear-box directly, with **no basic auth**, same trust
model `sound_machine`'s own `/speak` already has. This exists because the
*only* other way in is the `gear-box.charlesragone.com` Traefik
`IngressRoute`, gated by `vecna-basic-auth` for the whole host with no
path-scoped carve-out — fine for a browser, but mic_listener (or any
future LAN device pushing to gear-box) can't do an interactive login.
**The whole app is reachable on this NodePort**, not just `/hear` — a
NodePort can't do path-based routing — so anything hit at
`<bw-server-ip>:30080` (including `/api/speak`) skips the login the public
hostname requires. `MIC_API_TOKEN` is the only gate actually in front of
`/hear` itself.

### Wire contract: `POST /hear`

The mirror image of `/speak`: `mic_listener` is the only thing that ever
calls this, over the NodePort above. Body is **raw PCM: mono, 16-bit
signed little-endian, 22050Hz** (mic_listener samples straight at this
rate so no resampling is needed on either end) — same shape `/speak`
expects, since gear-box currently just forwards the body through
unchanged. Capped at `MaxHearBytes` (882000 bytes / 20s, mirroring
`piper.MaxOutputBytes`); a larger body gets 413. Authenticated via
`X-Token` if `MIC_API_TOKEN` is set (unset = open, mirroring
`ESP32_API_TOKEN`'s own default).

**This is intentionally just an echo today.** gear-box doesn't transcribe
or understand what it hears — `Hear` pushes the uploaded PCM straight to
`esp32.Client.Speak`, the same call `/speak` uses, so sound_machine just
plays back whatever mic_listener recorded. The planned next step is
swapping that direct forward for something that actually reasons about
the audio (likely: transcribe, run an LLM, synthesize a reply with Piper)
before talking to sound_machine — `Hear`'s body is where that goes.

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
  Piper and pushes the result to the ESP32. Nothing about the text is
  retained afterward.
- `GET /api/health`
- `POST /hear` — raw PCM from mic_listener (see its wire contract above).
  Not under `/api`: it's device-to-server traffic, not a browser-facing
  endpoint, same as `/speak` is for sound_machine.

## Where things live

- `gear-box/` — React+TS+Tailwind+daisyUI frontend (Vite).
- `gear-box/server/` — Go+Gin backend. No GORM/SQLite/migrations — this is
  the one deliberate scope-cut from vecna's stack, since there's no
  persistent state worth a database here.
- `gear-box/firmware/sound_machine/` — the speaker's ESP32-S3 Arduino sketch.
- `gear-box/firmware/mic_listener/` — the mic's ESP32-S3 Arduino sketch.
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
3. **mic_listener alone**: flash it, then
   `curl -X POST --data-binary @out.pcm -H "Content-Type: application/octet-stream" http://<bw-server-ip>:30080/hear`
   to check `/hear` and the forward-to-speaker path independent of the
   mic's own VAD; then talk near the mic and confirm sound_machine plays
   back what you said (and check the mic's Serial log for `speech
   start`/`speech end` lines matching your actual speech, to sanity-check
   `MIC_VAD_RMS_THRESHOLD`).
4. **gear-box server, local, against the real device**:
   `ESP32_HOST=<esp32-ip> run-server.sh`, then
   `curl -X POST localhost:8083/api/speak -d '{"text":"hello"}' -H 'Content-Type: application/json'`.
5. **Frontend, local**: `run-frontend.sh`, use the real page in a browser.
6. **Container build, local, still against the real device**:
   `docker build -t gear-box:dev -f gear-box/dockerfile gear-box/` then
   `docker run -p 8083:80 -e ESP32_HOST=<esp32-ip> gear-box:dev` — catches
   any Piper-fetch/packaging issue before it reaches the bw server.
7. Only after 1–6 pass, run `deploy-bw-server.sh` by hand once before
   trusting CI's webhook trigger.
