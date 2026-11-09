#!/usr/bin/env bash
# Manual deploy to the bw server's k3s cluster (192.168.1.91): pulls the
# given branch (or whatever's currently checked out there if none given),
# builds the vecna Docker image on the host, imports it straight into k3s's
# containerd (no registry — this node never needs internet-wide pulls), and
# applies/rolls out the k8s manifests.
#
# Usage: scripts/deploy-bw-server.sh [branch]
set -euo pipefail

HOST="charlie@192.168.1.91"
REPO_DIR="/home/charlie/monotheism"
# ssh doesn't preserve an empty positional argument across the wire (it
# space-joins argv for the remote shell to re-split, and an empty token
# doesn't survive that), so use a sentinel instead of "" to mean "no branch
# given, just pull whatever's checked out".
BRANCH="${1:-__CURRENT__}"

ssh "$HOST" bash -s -- "$REPO_DIR" "$BRANCH" <<'REMOTE'
set -euo pipefail
REPO_DIR="$1"
BRANCH="$2"

cd "$REPO_DIR"
git fetch origin
if [ "$BRANCH" != "__CURRENT__" ]; then
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
REMOTE
