#!/usr/bin/env bash
# Runs the Mosquitto MQTT broker in the foreground, using vecna/mosquitto.conf.
# Equivalent to the mqtt-broker service in docker-compose.yml, without Docker.
set -euo pipefail
cd "$(dirname "$0")/.."

mkdir -p mqtt-data
exec mosquitto -c mosquitto.local.conf
