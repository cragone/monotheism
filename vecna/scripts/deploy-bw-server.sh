#!/usr/bin/env bash
# Manual deploy to the bw server (192.168.1.91): pulls the given branch (or
# whatever's currently checked out there if none given), builds the
# frontend+backend inside vecna's nix dev shell, and (re)starts the server
# in a detached tmux session so it survives after this script exits.
#
# Usage: scripts/deploy-bw-server.sh [branch]
set -euo pipefail

HOST="charlie@192.168.1.91"
REPO_DIR="/home/charlie/monotheism"
SESSION="vecna"
# ssh doesn't preserve an empty positional argument across the wire (it
# space-joins argv for the remote shell to re-split, and an empty token
# doesn't survive that), so use a sentinel instead of "" to mean "no branch
# given, just pull whatever's checked out".
BRANCH="${1:-__CURRENT__}"

ssh "$HOST" bash -s -- "$REPO_DIR" "$SESSION" "$BRANCH" <<'REMOTE'
set -euo pipefail
REPO_DIR="$1"
SESSION="$2"
BRANCH="$3"

cd "$REPO_DIR"
git fetch origin
if [ "$BRANCH" != "__CURRENT__" ]; then
  git checkout "$BRANCH"
fi
git pull --ff-only

cd "$REPO_DIR/vecna"
nix develop --command bash -c '
  set -euo pipefail
  npm ci
  npm run build
  rm -rf server/web/dist
  mkdir -p server/web/dist
  cp -r dist/. server/web/dist/
  cd server
  CGO_ENABLED=0 go build -o /tmp/vecna-server ./cmd/server
'

tmux kill-session -t "$SESSION" 2>/dev/null || true
tmux new-session -d -s "$SESSION" -c "$REPO_DIR/vecna/server" "/tmp/vecna-server"

sleep 1
if tmux has-session -t "$SESSION" 2>/dev/null; then
  echo "Deployed. tmux session '$SESSION' running on $(hostname)."
else
  echo "tmux session died immediately - check /tmp/vecna-server manually." >&2
  exit 1
fi
REMOTE

echo "Attach with: ssh $HOST -t tmux attach -t $SESSION"
