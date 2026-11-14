#!/usr/bin/env bash
# Runs the Go backend directly, instead of the vecna Docker service. Talks
# straight to the Shelly plug's local RPC API — no broker/bridge in between.
set -euo pipefail
cd "$(dirname "$0")/../server"

export PORT="${PORT:-4000}"
export DATA_DIR="${DATA_DIR:-./data}"
# Optional: seeds one device at startup from SHELLY_HOST/SHELLY_NAME. Not
# required — devices can also be added at runtime via the /settings page.
export SHELLY_HOST="${SHELLY_HOST:-}"
export SHELLY_NAME="${SHELLY_NAME:-}"

exec go run ./cmd/server
