#!/usr/bin/env bash
# Manual deploy to the raspi5's k3s cluster, for triggering a deploy by hand
# from your own machine. CI instead hits the deploy webhook (see
# hooks.json.example + CONTROL-PANEL.md) so it never needs SSH access.
#
# Usage: scripts/deploy-raspi5.sh [branch]
set -euo pipefail

HOST="charlie@192.168.1.150"
INFRA_DIR="/home/charlie/monotheism"
BRANCH="${1:-}"

ssh "$HOST" "$INFRA_DIR/control-panel/scripts/deploy-local.sh" "$BRANCH"
