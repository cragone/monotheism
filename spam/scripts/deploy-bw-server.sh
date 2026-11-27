#!/usr/bin/env bash
# Manual deploy from your own machine: SSH to the bw server and run deploy-local.sh.
# Usage: spam/scripts/deploy-bw-server.sh [branch]
set -euo pipefail
ssh "charlie@192.168.1.91" "/home/charlie/monotheism/spam/scripts/deploy-local.sh" "${1:-}"
