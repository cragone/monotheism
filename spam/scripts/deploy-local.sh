#!/usr/bin/env bash
# Runs ON the bw server (192.168.1.91): pulls, builds the spam image, imports
# it into k3s's containerd, applies manifests and rolls out. Invoked by the
# deploy webhook (hook id "deploy-spam", see hooks.json.example) or by hand.
#
# Usage: deploy-local.sh [branch]
set -euo pipefail

REPO_DIR="/home/charlie/monotheism"
BRANCH="${1:-}"

cd "$REPO_DIR"
git fetch origin
if [ -n "$BRANCH" ]; then
  git checkout "$BRANCH"
fi
git pull --ff-only

SHA="$(git rev-parse --short HEAD)"
IMAGE="spam:$SHA"

cd "$REPO_DIR/spam"
docker build -t "$IMAGE" -f dockerfile .
docker save "$IMAGE" | sudo k3s ctr images import -

cd "$REPO_DIR"
sudo k3s kubectl apply -f kubernetes/spam-production.yaml
sudo k3s kubectl apply -f kubernetes/spam-ingress.yaml
sudo k3s kubectl -n default set image deployment/prod-spam prod-spam="$IMAGE"
sudo k3s kubectl -n default rollout status deployment/prod-spam

echo "Deployed $IMAGE to k3s on $(hostname)."
