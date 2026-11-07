#!/usr/bin/env bash
# Runs the Vite dev server, proxying /api to the bare-metal Go backend
# from scripts/run-server.sh instead of the client's Docker-built target.
set -euo pipefail
cd "$(dirname "$0")/.."

export VITE_API_PROXY_TARGET="${VITE_API_PROXY_TARGET:-http://localhost:4000}"

exec npm run dev
