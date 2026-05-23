#!/usr/bin/env bash
# Shared helpers for local deploy (sourced by deploy.sh / deploy-check.sh).

deploy_die() {
  echo "deploy: $*" >&2
  exit 1
}

deploy_info() {
  echo "deploy: $*"
}

deploy_require_cmd() {
  local cmd="$1"
  command -v "$cmd" >/dev/null 2>&1 || deploy_die "missing required command: $cmd"
}

deploy_git_short_sha() {
  if command -v git >/dev/null 2>&1 && git -C "$ROOT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo "unknown"
  else
    echo "unknown"
  fi
}

deploy_git_dirty() {
  if command -v git >/dev/null 2>&1 && git -C "$ROOT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    [ -n "$(git -C "$ROOT_DIR" status --porcelain 2>/dev/null)" ] && echo "yes" || echo "no"
  else
    echo "unknown"
  fi
}

# Returns 0 when frontend dist should be rebuilt (missing, empty, or older than src).
deploy_frontend_is_stale() {
  local dist_index="${DIST_DIR}/index.html"
  if [ ! -f "$dist_index" ]; then
    return 0
  fi
  if ! find "$FRONTEND_DIR/src" -type f -newer "$dist_index" -print -quit 2>/dev/null | grep -q .; then
    return 1
  fi
  return 0
}

deploy_resolve_frontend_build() {
  local mode="${FRONTEND_BUILD:-auto}"
  case "$mode" in
    0|false|no)
      FRONTEND_BUILD=0
      ;;
    1|true|yes)
      FRONTEND_BUILD=1
      ;;
    auto)
      if deploy_frontend_is_stale; then
        deploy_info "frontend dist is missing or older than frontend/src — will rebuild (FRONTEND_BUILD=auto)"
        FRONTEND_BUILD=1
      else
        FRONTEND_BUILD=0
      fi
      ;;
    *)
      deploy_die "invalid FRONTEND_BUILD=$mode (use 0, 1, or auto)"
      ;;
  esac
}

deploy_build_frontend() {
  deploy_info "building embedded frontend -> $DIST_DIR"
  if [ ! -d "$FRONTEND_DIR/node_modules" ]; then
    deploy_info "installing frontend dependencies..."
    pnpm --dir "$FRONTEND_DIR" install --frozen-lockfile
  fi
  pnpm --dir "$FRONTEND_DIR" build
  [ -f "${DIST_DIR}/index.html" ] || deploy_die "frontend build did not produce ${DIST_DIR}/index.html"
}

deploy_ensure_frontend_dist() {
  deploy_resolve_frontend_build
  if [ "$FRONTEND_BUILD" = "1" ]; then
    deploy_build_frontend
  elif [ ! -d "$DIST_DIR" ] || [ ! -f "${DIST_DIR}/index.html" ]; then
    deploy_die "embedded frontend dist not found at $DIST_DIR — run: FRONTEND_BUILD=1 make deploy"
  else
    deploy_info "using embedded frontend dist at $DIST_DIR"
  fi
}

deploy_build_binary() {
  local out="$1"
  deploy_info "building sub2api (tags=embed) -> $out"
  (
    cd "$BACKEND_DIR"
    CGO_ENABLED=0 go build -tags embed -ldflags="$DEPLOY_LDFLAGS" -trimpath -o "$out" ./cmd/server
  )
  [ -x "$out" ] || deploy_die "build failed: $out"
}

deploy_binary_sha256() {
  shasum -a 256 "$1" | awk '{print $1}'
}

deploy_write_manifest() {
  local bin="$1"
  local manifest_dir="${ROOT_DIR}/.deploy"
  mkdir -p "$manifest_dir"
  cat >"${manifest_dir}/last-build.json" <<EOF
{
  "built_at": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "git_sha": "$(deploy_git_short_sha)",
  "git_dirty": "$(deploy_git_dirty)",
  "binary": "$bin",
  "binary_sha256": "$(deploy_binary_sha256 "$bin")",
  "frontend_dist": "$DIST_DIR",
  "frontend_rebuilt": $([ "$FRONTEND_BUILD" = "1" ] && echo true || echo false)
}
EOF
  deploy_info "wrote ${manifest_dir}/last-build.json"
}

deploy_health_check() {
  local url="$1"
  local retries="${2:-15}"
  local i=1
  while [ "$i" -le "$retries" ]; do
    if curl -fsS "${url}/health" >/dev/null 2>&1; then
      curl -fsS "${url}/health"
      echo
      return 0
    fi
    sleep 1
    i=$((i + 1))
  done
  return 1
}

deploy_supervisor_render_conf() {
  local template="$1"
  local dest="$2"
  sed \
    -e "s|@ROOT_DIR@|${ROOT_DIR}|g" \
    -e "s|@INSTALL_BIN@|${INSTALL_BIN}|g" \
    -e "s|@DATA_DIR@|${DATA_DIR}|g" \
    -e "s|@LOG_DIR@|${SUPERVISOR_LOG_DIR}|g" \
    -e "s|@RUN_USER@|${DEPLOY_USER}|g" \
    "$template" >"$dest"
}

deploy_cleanup_backups() {
  local keep="$1"
  if ! [[ "$keep" =~ ^[0-9]+$ ]]; then
    deploy_die "BACKUP_KEEP must be a non-negative integer, got: $keep"
  fi

  local count
  count="$(sudo find /usr/local/bin -maxdepth 1 -type f -name 'sub2api.bak.*' -print 2>/dev/null | wc -l | tr -d ' ')"
  if [ "$count" -le "$keep" ]; then
    return 0
  fi

  local remove_count=$((count - keep))
  deploy_info "pruning ${remove_count} old backup(s), keeping latest ${keep}..."
  sudo find /usr/local/bin -maxdepth 1 -type f -name 'sub2api.bak.*' -print 2>/dev/null | sort | head -n "$remove_count" | while IFS= read -r backup; do
    sudo rm -f "$backup"
  done
}
