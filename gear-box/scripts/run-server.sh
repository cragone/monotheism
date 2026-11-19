#!/usr/bin/env bash
# Runs the Go backend directly, instead of the gear-box Docker service.
# Pushes synthesized audio straight to the ESP32 over the LAN — the ESP32
# cannot be mocked, so ESP32_HOST must point at the real device.
set -euo pipefail
cd "$(dirname "$0")/../server"

export PORT="${PORT:-8083}"
export ESP32_HOST="${ESP32_HOST:-soundmachine.local}"
export PIPER_BINARY="${PIPER_BINARY:-piper}"
export PIPER_MODEL_PATH="${PIPER_MODEL_PATH:-./models/voice.onnx}"

exec go run ./cmd/server
