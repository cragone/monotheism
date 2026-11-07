#!/usr/bin/env bash
# Runs cync-lan bare-metal (from the venv scripts/setup-cync-lan.sh created)
# instead of the cync-lan Docker service in docker-compose.yml.
set -euo pipefail
cd "$(dirname "$0")/.."

VENV_DIR="cync-lan-src/.venv"
if [ ! -x "$VENV_DIR/bin/cync-lan" ]; then
  echo "cync-lan isn't set up yet. Run scripts/setup-cync-lan.sh first." >&2
  exit 1
fi

if [ ! -f .env.cync-lan ]; then
  echo "Missing vecna/.env.cync-lan. Copy .env.cync-lan.example and fill it in." >&2
  exit 1
fi

set -a
source .env.cync-lan
set +a

exec "$VENV_DIR/bin/cync-lan"
