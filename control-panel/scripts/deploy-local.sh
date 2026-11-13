#!/usr/bin/env bash
# Runs ON the raspi5 host itself: pulls the given branch (or whatever's
# currently checked out if none given) of the *separate* cragone/control-panel
# repo, builds its frontend+backend natively (no Docker/k3s -- this host is
# a single always-on kiosk display, not a cluster), and restarts the
# systemd service. Called either by the deploy webhook (see
# hooks.json.example + CONTROL-PANEL.md) or by hand via deploy-raspi5.sh
# from another machine.
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

echo "==> Building frontend..."
cd "$CODE_DIR/client"
bun install --frozen-lockfile
bun run build.ts

echo "==> Replacing dist..."
rm -rf "$CODE_DIR/web/dist"
mv "$CODE_DIR/client/dist" "$CODE_DIR/web/dist"

echo "==> Building backend..."
cd "$CODE_DIR/web"
go build -o main

echo "==> Restarting service..."
sudo systemctl restart control-panel-web

echo "Deployed $(git -C "$CODE_DIR" rev-parse --short HEAD) to $(hostname)."
