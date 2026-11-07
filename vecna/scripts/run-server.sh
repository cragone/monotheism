#!/usr/bin/env bash
# Runs the Go backend directly, instead of the vecna Docker service. Talks to
# the bare-metal mosquitto broker from scripts/run-mqtt.sh on localhost:1883.
set -euo pipefail
cd "$(dirname "$0")/../server"

export PORT="${PORT:-4000}"
export DATA_DIR="${DATA_DIR:-./data}"
export MQTT_BROKER_URL="${MQTT_BROKER_URL:-tcp://localhost:1883}"
export MQTT_BASE_TOPIC="${MQTT_BASE_TOPIC:-cync_lan}"

exec go run ./cmd/server
