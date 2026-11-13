#!/usr/bin/env bash
# Runs ON the raspi5 host itself: pulls the given branch (or whatever's
# currently checked out if none given) of the *separate* cragone/control-panel
# repo, builds its Docker image, imports it into k3s's containerd, and rolls
# it out. Called either by the deploy webhook (see hooks.json.example +
# CONTROL-PANEL.md) or by hand via deploy-raspi5.sh from another machine.
#
# Unlike vecna, control-panel's app code is NOT part of this repo — this
# repo (monotheism) only carries the deploy orchestration (this script, the
# webhook config, the kubernetes manifests). CODE_DIR below is a separate
# checkout of cragone/control-panel on the host.
#
# Usage: deploy-local.sh [branch]
set -euo pipefail

INFRA_DIR="/home/charlie/monotheism"
CODE_DIR="/home/charlie/control-panel"
BRANCH="${1:-}"

cd "$CODE_DIR"
git fetch origin
if [ -n "$BRANCH" ]; then
  git checkout "$BRANCH"
fi
git pull --ff-only

SHA="$(git rev-parse --short HEAD)"
IMAGE="control-panel:$SHA"

docker build -t "$IMAGE" -f Dockerfile .

# k3s uses containerd, not the host's docker daemon, so the freshly built
# image has to be handed over explicitly rather than just being "docker run"-able.
docker save "$IMAGE" | sudo k3s ctr images import -

cd "$INFRA_DIR"
sudo k3s kubectl apply -f kubernetes/control-panel-production.yaml
sudo k3s kubectl apply -f kubernetes/control-panel-ingress.yaml
sudo k3s kubectl -n default set image deployment/prod-control-panel-web prod-control-panel-web="$IMAGE"
sudo k3s kubectl -n default rollout status deployment/prod-control-panel-web

echo "Deployed $IMAGE to k3s on $(hostname)."
