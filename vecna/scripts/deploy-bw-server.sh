#!/usr/bin/env bash
# Manual deploy to the bw server's k3s cluster (192.168.1.91), for triggering
# a deploy by hand from your own machine. CI instead hits the deploy webhook
# (see hooks.json.example + VECNA.md) so it never needs SSH access.
#
# Usage: scripts/deploy-bw-server.sh [branch]
set -euo pipefail

HOST="charlie@192.168.1.91"
REPO_DIR="/home/charlie/monotheism"
BRANCH="${1:-}"

ssh "$HOST" "$REPO_DIR/vecna/scripts/deploy-local.sh" "$BRANCH"
