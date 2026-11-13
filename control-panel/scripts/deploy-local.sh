#!/usr/bin/env bash
# Runs ON the raspi5 host itself: pulls the given branch (or whatever's
# currently checked out if none given) of the *separate* cragone/control-panel
# repo, builds its kiosk/ Rust binary natively (no Docker/k3s -- this host is
# a single always-on kiosk display, not a cluster), and restarts the cage
# session. Called either by the deploy webhook (see hooks.json.example +
# CONTROL-PANEL.md) or by hand via deploy-raspi5.sh from another machine.
#
# The kiosk talks directly to vecna's API for device names/state and
# control (see CONTROL-PANEL.md) -- there's no local MQTT broker, Go
# backend, or ESP32 hardware bridge involved on this host at all, so
# client/ and web/ in the control-panel repo aren't built here.
#
# This repo (monotheism) only carries the deploy orchestration; control-panel's
# app code lives in its own repo, checked out separately on the host.
#
# Usage: deploy-local.sh [branch]
set -euo pipefail

CODE_DIR="/home/charlie/control-panel"
BRANCH="${1:-}"

cd "$CODE_DIR"
git fetch origin
if [ -n "$BRANCH" ]; then
  git checkout "$BRANCH"
fi
git pull --ff-only

echo "==> Building kiosk..."
cd "$CODE_DIR/kiosk"
cargo build --release

echo "==> Restarting kiosk session..."
sudo systemctl restart cage-tty1

echo "Deployed $(git -C "$CODE_DIR" rev-parse --short HEAD) to $(hostname)."
