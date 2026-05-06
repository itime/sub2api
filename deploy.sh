#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"
DATA_DIR="${DATA_DIR:-$ROOT_DIR/data}"
BUILD_BIN="$ROOT_DIR/sub2api"
INSTALL_BIN="/usr/local/bin/sub2api"
BACKUP_KEEP="${BACKUP_KEEP:-10}"
STAMP="$(date +%Y%m%d_%H%M%S)"
SUPERVISOR_CONF_SRC="$ROOT_DIR/deploy/supervisor/sub2api.conf"
SUPERVISOR_CONF_DST="/opt/homebrew/etc/supervisor.d/sub2api.conf"
SERVICE_NAME="sub2api"
API_BASE="${API_BASE:-http://127.0.0.1:18080}"
FRONTEND_BUILD="${FRONTEND_BUILD:-0}"
DIST_DIR="$BACKEND_DIR/internal/web/dist"

export HTTP_PROXY="${HTTP_PROXY:-http://127.0.0.1:6152}"
export HTTPS_PROXY="${HTTPS_PROXY:-http://127.0.0.1:6152}"
export ALL_PROXY="${ALL_PROXY:-http://127.0.0.1:6152}"
export http_proxy="${http_proxy:-$HTTP_PROXY}"
export https_proxy="${https_proxy:-$HTTPS_PROXY}"
export all_proxy="${all_proxy:-$ALL_PROXY}"

cleanup_backups() {
  local keep="$1"
  if ! [[ "$keep" =~ ^[0-9]+$ ]]; then
    echo "BACKUP_KEEP must be a non-negative integer, got: $keep" >&2
    exit 1
  fi

  local count
  count="$(sudo find /usr/local/bin -maxdepth 1 -type f -name 'sub2api.bak.*' -print | wc -l | tr -d ' ')"
  if [ "$count" -le "$keep" ]; then
    return 0
  fi

  local remove_count=$((count - keep))
  echo "Pruning ${remove_count} old backup(s), keeping latest ${keep}..."
  sudo find /usr/local/bin -maxdepth 1 -type f -name 'sub2api.bak.*' -print | sort | head -n "$remove_count" | while IFS= read -r backup; do
    sudo rm -f "$backup"
  done
}

if [ "$FRONTEND_BUILD" = "1" ]; then
  echo "Building embedded frontend..."
  if [ ! -d "$FRONTEND_DIR/node_modules" ]; then
    echo "Installing frontend dependencies..."
    pnpm --dir "$FRONTEND_DIR" install --frozen-lockfile
  fi
  pnpm --dir "$FRONTEND_DIR" build
elif [ ! -d "$DIST_DIR" ]; then
  echo "Embedded frontend dist not found at $DIST_DIR" >&2
  echo "Run with FRONTEND_BUILD=1 after frontend dependencies are available." >&2
  exit 1
else
  echo "Using existing embedded frontend dist at $DIST_DIR"
fi

echo "Building sub2api..."
(
  cd "$BACKEND_DIR"
  go build -tags embed -o "$BUILD_BIN" ./cmd/server
)

echo "Ensuring runtime directories..."
mkdir -p "$DATA_DIR"
mkdir -p /Users/lzw/log/supervisor/sub2api

echo "Installing supervisor config..."
sudo cp "$SUPERVISOR_CONF_SRC" "$SUPERVISOR_CONF_DST"

echo "Stopping service if running..."
sudo /usr/local/bin/supervisord ctl stop "$SERVICE_NAME" || true

echo "Deploying binary to $INSTALL_BIN..."
if [ -f "$INSTALL_BIN" ]; then
  sudo cp "$INSTALL_BIN" "/usr/local/bin/sub2api.bak.${STAMP}"
  sudo chmod +x "/usr/local/bin/sub2api.bak.${STAMP}"
fi
cleanup_backups "$BACKUP_KEEP"
sudo cp "$BUILD_BIN" "$INSTALL_BIN"
sudo chmod +x "$INSTALL_BIN"

echo "Reloading supervisor..."
sudo /usr/local/bin/supervisord ctl reload
sleep 2
sudo /usr/local/bin/supervisord ctl start "$SERVICE_NAME"

echo "Waiting for service to start..."
sleep 3

echo "Checking status..."
sudo /usr/local/bin/supervisord ctl status "$SERVICE_NAME"

echo "Health check..."
curl -fsS "$API_BASE/health"

echo
echo "Done."
