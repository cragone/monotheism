#!/usr/bin/env bash
# Runs ON the bw server itself (192.168.1.91): pulls the given branch (or
# whatever's currently checked out if none given), builds the vecna Docker
# image, imports it into k3s's containerd, and rolls it out. Called either
# by the deploy webhook (see hooks.json.example + VECNA.md) or by hand via
# deploy-bw-server.sh from another machine.
#
# Usage: deploy-local.sh [branch]
set -euo pipefail

REPO_DIR="/home/charlie/monotheism"
BRANCH="${1:-}"

# Idempotent cleanup of the old pre-k3s deploy (a /tmp/vecna-server binary
# run directly in tmux, bound straight to host port 80): if it's still
# running, it wins the race for port 80 over k3s's Traefik, which makes the
# BasicAuth middleware and everything else in the IngressRoute a no-op since
# traffic never reaches Traefik at all. Match by exact process name, not -f,
# so this can't ever match its own invocation's command line.
pkill -x vecna-server 2>/dev/null || true

cd "$REPO_DIR"
git fetch origin
if [ -n "$BRANCH" ]; then
  git checkout "$BRANCH"
fi
git pull --ff-only

SHA="$(git rev-parse --short HEAD)"
IMAGE="vecna:$SHA"

cd "$REPO_DIR/vecna"
docker build -t "$IMAGE" -f dockerfile .

# k3s uses containerd, not the host's docker daemon, so the freshly built
# image has to be handed over explicitly rather than just being "docker run"-able.
docker save "$IMAGE" | sudo k3s ctr images import -

cd "$REPO_DIR"
sudo k3s kubectl apply -f kubernetes/vecna-production.yaml
sudo k3s kubectl apply -f kubernetes/vecna-ingress.yaml
sudo k3s kubectl -n default set image deployment/prod-vecna prod-vecna="$IMAGE"
sudo k3s kubectl -n default rollout status deployment/prod-vecna

echo "Deployed $IMAGE to k3s on $(hostname)."
