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

# Content hash of tracked frontend sources (mtime alone is unreliable).
deploy_frontend_sources_hash() {
  (
    cd "$ROOT_DIR"
    if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
      git ls-files frontend 2>/dev/null | LC_ALL=C sort | while IFS= read -r file; do
        [ -f "$file" ] && shasum -a 256 "$file"
      done
    else
      find frontend/src frontend/package.json frontend/pnpm-lock.yaml frontend/vite.config.ts \
        frontend/index.html frontend/tsconfig.json frontend/tsconfig.app.json \
        -type f 2>/dev/null | LC_ALL=C sort | while IFS= read -r file; do
        shasum -a 256 "$file"
      done
    fi
  ) | shasum -a 256 | awk '{print $1}'
}

deploy_last_frontend_sources_hash() {
  local manifest="${ROOT_DIR}/.deploy/last-build.json"
  if [ ! -f "$manifest" ] || ! command -v python3 >/dev/null 2>&1; then
    echo ""
    return 0
  fi
  python3 -c "import json,sys; d=json.load(open(sys.argv[1])); print(d.get('frontend_sources_hash') or '')" "$manifest" 2>/dev/null || true
}

# Returns 0 when frontend dist should be rebuilt.
deploy_frontend_is_stale() {
  local dist_index="${DIST_DIR}/index.html"
  if [ ! -f "$dist_index" ]; then
    return 0
  fi

  local current_hash last_hash
  current_hash="$(deploy_frontend_sources_hash)"
  last_hash="$(deploy_last_frontend_sources_hash)"
  if [ -n "$last_hash" ] && [ "$current_hash" != "$last_hash" ]; then
    return 0
  fi

  if find "$FRONTEND_DIR/src" -type f -newer "$dist_index" -print -quit 2>/dev/null | grep -q .; then
    return 0
  fi
  for config_file in \
    "$FRONTEND_DIR/package.json" \
    "$FRONTEND_DIR/pnpm-lock.yaml" \
    "$FRONTEND_DIR/vite.config.ts" \
    "$FRONTEND_DIR/index.html"; do
    if [ -f "$config_file" ] && [ "$config_file" -nt "$dist_index" ]; then
      return 0
    fi
  done

  return 1
}

deploy_resolve_frontend_build() {
  local mode="${1:-${FRONTEND_BUILD:-auto}}"
  case "$mode" in
    0|false|no)
      FRONTEND_BUILD=0
      ;;
    1|true|yes)
      FRONTEND_BUILD=1
      ;;
    auto)
      if deploy_frontend_is_stale; then
        deploy_info "frontend sources changed or dist missing — will rebuild (FRONTEND_BUILD=auto)"
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

# Sanity-check that the admin dashboard bundle was embedded (catches stale dist).
deploy_verify_embedded_frontend() {
  local dashboard_chunk
  dashboard_chunk="$(find "${DIST_DIR}/assets" -name 'DashboardView-*.js' -print 2>/dev/null | head -1)"
  if [ -z "$dashboard_chunk" ]; then
    deploy_die "embed verify failed: no DashboardView chunk under ${DIST_DIR}/assets"
  fi
  if ! grep -q 'today_active_accounts' "$dashboard_chunk" 2>/dev/null; then
    deploy_die "embed verify failed: dashboard bundle missing today_active_accounts (dist is stale — use: make deploy)"
  fi
  deploy_info "embed verify ok ($(basename "$dashboard_chunk"))"
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
  deploy_resolve_frontend_build "${FRONTEND_BUILD:-auto}"
  if [ "$FRONTEND_BUILD" = "1" ]; then
    deploy_build_frontend
    if [ "${DEPLOY_VERIFY_EMBED:-1}" = "1" ]; then
      deploy_verify_embedded_frontend
    fi
  elif [ ! -d "$DIST_DIR" ] || [ ! -f "${DIST_DIR}/index.html" ]; then
    deploy_die "embedded frontend dist not found at $DIST_DIR — run: make deploy"
  else
    deploy_info "using embedded frontend dist at $DIST_DIR"
    if [ "${DEPLOY_VERIFY_EMBED:-0}" = "1" ]; then
      deploy_verify_embedded_frontend
    fi
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
  "frontend_rebuilt": $([ "$FRONTEND_BUILD" = "1" ] && echo true || echo false),
  "frontend_sources_hash": "$(deploy_frontend_sources_hash)"
}
EOF
  deploy_info "wrote ${manifest_dir}/last-build.json"
}

deploy_health_check() {
  local url="$1"
  local retries="${2:-60}"
  local i=1
  while [ "$i" -le "$retries" ]; do
    # Always bypass proxy for localhost health probes — shell HTTP_PROXY can break 127.0.0.1 checks.
    if curl -fsS --noproxy '*' "${url}/health" >/dev/null 2>&1; then
      curl -fsS --noproxy '*' "${url}/health"
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
