#!/usr/bin/env bash
# Runs the Go backend directly, instead of the vecna Docker service. Talks
# straight to the Shelly plug's local RPC API — no broker/bridge in between.
set -euo pipefail
cd "$(dirname "$0")/../server"

export PORT="${PORT:-4000}"
export DATA_DIR="${DATA_DIR:-./data}"
: "${SHELLY_HOST:?Set SHELLY_HOST to the plug's LAN IP/hostname, e.g. 192.168.1.26}"
export SHELLY_HOST
export SHELLY_NAME="${SHELLY_NAME:-}"

exec go run ./cmd/server
