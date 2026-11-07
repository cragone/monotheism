#!/usr/bin/env bash
# One-time setup: clones baudneo/cync-lan (the `python` branch — `main` is
# unused upstream), installs it into a venv, and generates the self-signed
# TLS certs the plug needs to accept this box as GE's cloud. Re-run is safe;
# it skips steps that already happened.
#
# This replaces the `cync-lan` Docker image from docker-compose.yml with a
# bare-metal checkout, for running everything from the nix dev shell instead.
set -euo pipefail
cd "$(dirname "$0")/.."

SRC_DIR="cync-lan-src"
VENV_DIR="$SRC_DIR/.venv"

if [ ! -d "$SRC_DIR" ]; then
  git clone --branch python --single-branch https://github.com/baudneo/cync-lan "$SRC_DIR"
else
  echo "cync-lan-src/ already exists, skipping clone"
fi

if [ ! -d "$VENV_DIR" ]; then
  python3 -m venv "$VENV_DIR"
fi
"$VENV_DIR/bin/pip" install --upgrade pip >/dev/null
"$VENV_DIR/bin/pip" install -e "$SRC_DIR"

mkdir -p cync-lan-config
if [ ! -f cync-lan-config/certs/cert.pem ]; then
  (cd cync-lan-config && bash "../$SRC_DIR/create_certs.sh")
else
  echo "cync-lan-config/certs/ already exists, skipping cert generation"
fi

echo
echo "Setup done. Next:"
echo "  1. Copy vecna/.env.cync-lan.example to vecna/.env.cync-lan and fill in"
echo "     CYNC_ACCOUNT_USERNAME/PASSWORD and a CYNC_SECRET_KEY."
echo "  2. Run scripts/run-mqtt.sh and scripts/run-cync-lan.sh (separate terminals)."
echo "  3. Visit http://127.0.0.1:23778 to export your Cync devices (needs an OTP"
echo "     emailed to you)."
echo "  4. Override DNS for cm.gelighting.com / cm-sec.gelighting.com /"
echo "     cm-ge.xlink.cn at your router to point at this box, then power-cycle"
echo "     the plug. See VECNA.md."
