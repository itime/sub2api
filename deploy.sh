#!/usr/bin/env bash
# Local release: build embed binary + install to supervisor-managed /usr/local/bin/sub2api.
#
# Usage:
#   ./deploy.sh              # full deploy (default FRONTEND_BUILD=auto)
#   ./deploy.sh --build-only # build ./sub2api only, no sudo
#   make deploy              # recommended entry point
#
# Environment:
#   FRONTEND_BUILD=auto|0|1  — auto rebuild dist when frontend/src is newer (default: auto)
#   API_BASE                 — health check URL (default: http://127.0.0.1:18080)
#   DATA_DIR                 — runtime data (default: ./data)
#   INSTALL_BIN              — target binary (default: /usr/local/bin/sub2api)
#   SKIP_HEALTH_CHECK=1      — skip post-deploy health check

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=deploy/lib/deploy-common.sh
source "$ROOT_DIR/deploy/lib/deploy-common.sh"

BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"
DATA_DIR="${DATA_DIR:-$ROOT_DIR/data}"
BUILD_BIN="${BUILD_BIN:-$ROOT_DIR/sub2api}"
INSTALL_BIN="${INSTALL_BIN:-/usr/local/bin/sub2api}"
BACKUP_KEEP="${BACKUP_KEEP:-10}"
STAMP="$(date +%Y%m%d_%H%M%S)"
SUPERVISOR_CONF_TEMPLATE="$ROOT_DIR/deploy/supervisor/sub2api.conf.template"
SUPERVISOR_CONF_LEGACY="$ROOT_DIR/deploy/supervisor/sub2api.conf"
SUPERVISOR_CONF_DST="${SUPERVISOR_CONF_DST:-/opt/homebrew/etc/supervisor.d/sub2api.conf}"
SUPERVISOR_CONF_RENDERED="${ROOT_DIR}/.deploy/sub2api.supervisor.conf"
SERVICE_NAME="${SERVICE_NAME:-sub2api}"
API_BASE="${API_BASE:-http://127.0.0.1:18080}"
DIST_DIR="$BACKEND_DIR/internal/web/dist"
DEPLOY_USER="${DEPLOY_USER:-$(whoami)}"
SUPERVISOR_LOG_DIR="${SUPERVISOR_LOG_DIR:-/Users/${DEPLOY_USER}/log/supervisor/sub2api}"
SUPERVISORD_CTL="${SUPERVISORD_CTL:-/usr/local/bin/supervisord}"
BUILD_ONLY=0

export HTTP_PROXY="${HTTP_PROXY:-http://127.0.0.1:6152}"
export HTTPS_PROXY="${HTTPS_PROXY:-http://127.0.0.1:6152}"
export ALL_PROXY="${ALL_PROXY:-http://127.0.0.1:6152}"
export http_proxy="${http_proxy:-$HTTP_PROXY}"
export https_proxy="${https_proxy:-$HTTPS_PROXY}"
export all_proxy="${all_proxy:-$ALL_PROXY}"

VERSION="$(tr -d '\r\n' <"$BACKEND_DIR/cmd/server/VERSION" 2>/dev/null || echo "0.0.0-dev")"
DEPLOY_LDFLAGS="-s -w -X main.Version=${VERSION}"

usage() {
  cat <<EOF
Local deploy for Sub2API (supervisor + ${INSTALL_BIN})

  ./deploy.sh                 Build and install release binary, restart service
  ./deploy.sh --build-only    Build ${BUILD_BIN} only (no sudo / no restart)
  ./deploy.sh --help

Environment:
  FRONTEND_BUILD=auto|0|1     auto: rebuild dist when frontend/src changed (default)
  API_BASE                    Health check base URL (default: ${API_BASE})
  DATA_DIR                    Runtime data directory (default: ${DATA_DIR})

Makefile shortcuts:
  make deploy                 Full local release
  make build-release          Same as ./deploy.sh --build-only
  make deploy-check           Compare git / dist / installed binary / health

Note: 'make build' compiles backend/bin/server WITHOUT embed tag and does NOT
update the process on port 18080. Always use 'make deploy' for local releases.
EOF
}

while [ $# -gt 0 ]; do
  case "$1" in
    --help|-h)
      usage
      exit 0
      ;;
    --build-only)
      BUILD_ONLY=1
      shift
      ;;
    *)
      deploy_die "unknown argument: $1 (try --help)"
      ;;
  esac
done

deploy_require_cmd go
deploy_require_cmd curl
if [ "$BUILD_ONLY" = "0" ]; then
  deploy_require_cmd sudo
  deploy_require_cmd pnpm
fi

deploy_info "git $(deploy_git_short_sha) (dirty: $(deploy_git_dirty))"
if [ "$(deploy_git_dirty)" = "yes" ] && [ "$BUILD_ONLY" = "0" ]; then
  deploy_info "warning: working tree has uncommitted changes — deployed binary may not match any commit"
fi

deploy_ensure_frontend_dist
deploy_build_binary "$BUILD_BIN"
deploy_write_manifest "$BUILD_BIN"

deploy_info "release binary version: $($BUILD_BIN -version 2>/dev/null | head -1 || echo unknown)"

if [ "$BUILD_ONLY" = "1" ]; then
  deploy_info "build-only complete: $BUILD_BIN"
  deploy_info "run 'make deploy' to install and restart ${API_BASE}"
  exit 0
fi

mkdir -p "$DATA_DIR"
mkdir -p "$SUPERVISOR_LOG_DIR"
mkdir -p "$(dirname "$SUPERVISOR_CONF_RENDERED")"

if [ -f "$SUPERVISOR_CONF_TEMPLATE" ]; then
  deploy_supervisor_render_conf "$SUPERVISOR_CONF_TEMPLATE" "$SUPERVISOR_CONF_RENDERED"
else
  deploy_info "template missing, falling back to legacy supervisor config"
  cp "$SUPERVISOR_CONF_LEGACY" "$SUPERVISOR_CONF_RENDERED"
fi

deploy_info "installing supervisor config -> $SUPERVISOR_CONF_DST"
sudo cp "$SUPERVISOR_CONF_RENDERED" "$SUPERVISOR_CONF_DST"

deploy_info "stopping $SERVICE_NAME (if running)..."
sudo "$SUPERVISORD_CTL" ctl stop "$SERVICE_NAME" 2>/dev/null || true

BACKUP_PATH=""
if [ -f "$INSTALL_BIN" ]; then
  BACKUP_PATH="/usr/local/bin/sub2api.bak.${STAMP}"
  deploy_info "backing up $INSTALL_BIN -> $BACKUP_PATH"
  sudo cp "$INSTALL_BIN" "$BACKUP_PATH"
  sudo chmod +x "$BACKUP_PATH"
fi
deploy_cleanup_backups "$BACKUP_KEEP"

deploy_info "installing $BUILD_BIN -> $INSTALL_BIN"
sudo cp "$BUILD_BIN" "$INSTALL_BIN"
sudo chmod +x "$INSTALL_BIN"

deploy_info "reloading supervisor..."
sudo "$SUPERVISORD_CTL" ctl reload
sleep 2
sudo "$SUPERVISORD_CTL" ctl start "$SERVICE_NAME" 2>/dev/null || true

deploy_info "supervisor status:"
sudo "$SUPERVISORD_CTL" ctl status "$SERVICE_NAME" || true

if [ "${SKIP_HEALTH_CHECK:-0}" = "1" ]; then
  deploy_info "SKIP_HEALTH_CHECK=1 — skipping health check"
  exit 0
fi

deploy_info "health check $API_BASE (up to 15s)..."
if deploy_health_check "$API_BASE" 15; then
  cat >"${ROOT_DIR}/.deploy/last-deploy.json" <<EOF
{
  "deployed_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "git_sha": "$(deploy_git_short_sha)",
  "git_dirty": "$(deploy_git_dirty)",
  "install_bin": "$INSTALL_BIN",
  "binary_sha256": "$(deploy_binary_sha256 "$INSTALL_BIN")",
  "backup": "${BACKUP_PATH:-}",
  "api_base": "$API_BASE"
}
EOF
  deploy_info "deploy complete"
  exit 0
fi

deploy_info "health check failed" >&2
if [ -n "$BACKUP_PATH" ] && [ -f "$BACKUP_PATH" ]; then
  deploy_info "rolling back to $BACKUP_PATH"
  sudo cp "$BACKUP_PATH" "$INSTALL_BIN"
  sudo chmod +x "$INSTALL_BIN"
  sudo "$SUPERVISORD_CTL" ctl start "$SERVICE_NAME" 2>/dev/null || true
  if deploy_health_check "$API_BASE" 10; then
    deploy_info "rollback succeeded — previous binary restored"
  else
    deploy_info "rollback completed but health still failing — check supervisor logs" >&2
  fi
fi
exit 1
