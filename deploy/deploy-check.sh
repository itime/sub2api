#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=deploy/lib/deploy-common.sh
source "$ROOT_DIR/deploy/lib/deploy-common.sh"

INSTALL_BIN="${INSTALL_BIN:-/usr/local/bin/sub2api}"
BUILD_BIN="${BUILD_BIN:-$ROOT_DIR/sub2api}"
API_BASE="${API_BASE:-http://127.0.0.1:18080}"
DIST_DIR="$ROOT_DIR/backend/internal/web/dist"
FRONTEND_DIR="$ROOT_DIR/frontend"
MANIFEST="$ROOT_DIR/.deploy/last-build.json"

echo "=== Sub2API local deploy check ==="
echo "git:     $(deploy_git_short_sha) (dirty: $(deploy_git_dirty))"
echo "api:     $API_BASE"

if [ -f "$MANIFEST" ]; then
  echo "manifest: $MANIFEST"
  if command -v python3 >/dev/null 2>&1; then
    python3 -c "import json,sys; d=json.load(open(sys.argv[1])); print('  built_at:', d.get('built_at')); print('  git_sha:', d.get('git_sha'), '(dirty:', d.get('git_dirty'), ')'); print('  binary_sha256:', d.get('binary_sha256'))" "$MANIFEST" 2>/dev/null || cat "$MANIFEST"
  else
    cat "$MANIFEST"
  fi
else
  echo "manifest: (none — run make deploy to record last build)"
fi

deploy_check_binary() {
  local label="$1"
  local path="$2"
  if [ -f "$path" ]; then
    echo "${label}: ${path}"
    echo "  mtime: $(stat -f '%Sm' -t '%Y-%m-%d %H:%M:%S' "$path" 2>/dev/null || stat -c '%y' "$path" 2>/dev/null)"
    echo "  sha256: $(deploy_binary_sha256 "$path")"
    if [ -x "$path" ]; then
      echo "  version: $($path -version 2>/dev/null | head -1 || echo '(no -version)')"
    fi
  else
    echo "${label}: (missing) ${path}"
  fi
}

echo ""
echo "--- binaries ---"
deploy_check_binary "installed" "$INSTALL_BIN"
deploy_check_binary "workspace" "$BUILD_BIN"
deploy_check_binary "make-backend" "$ROOT_DIR/backend/bin/server"

echo ""
echo "--- frontend embed ---"
if [ -f "${DIST_DIR}/index.html" ]; then
  echo "dist: $DIST_DIR (index.html mtime: $(stat -f '%Sm' -t '%Y-%m-%d %H:%M:%S' "${DIST_DIR}/index.html" 2>/dev/null || stat -c '%y' "${DIST_DIR}/index.html"))"
  if deploy_frontend_is_stale; then
    echo "status: STALE — frontend/src is newer than dist (run make deploy)"
  else
    echo "status: ok"
  fi
else
  echo "dist: MISSING at $DIST_DIR"
fi

echo ""
echo "--- runtime ---"
if curl -fsS "${API_BASE}/health" >/dev/null 2>&1; then
  echo "health: ok ($(curl -fsS "${API_BASE}/health" 2>/dev/null))"
else
  echo "health: FAILED (is supervisor running?)"
fi

if pgrep -x sub2api >/dev/null 2>&1; then
  running="$(command -v sub2api 2>/dev/null || true)"
  pid="$(pgrep -x sub2api | head -1)"
  echo "process: pid=$pid"
  if [ -n "$pid" ] && [ -r "/proc/$pid/exe" ] 2>/dev/null; then
    :
  elif [ -n "$pid" ]; then
    proc_bin="$(ps -p "$pid" -o command= 2>/dev/null | awk '{print $1}')"
    echo "  command: $proc_bin"
  fi
fi

echo ""
echo "To release local changes to :18080: make deploy"
echo "To build only (no sudo):       make build-release"
