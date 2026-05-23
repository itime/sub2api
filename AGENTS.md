# Local development (this machine)

## What runs on port 18080

The admin UI at `http://localhost:18080` is served by **supervisor**, not `go run` or `backend/bin/server`:

| Item | Path |
|------|------|
| Process | `sub2api` via supervisord |
| Binary | `/usr/local/bin/sub2api` |
| Working dir | Repository root |
| Data | `./data` (`DATA_DIR`) |

## Release workflow (required after code changes)

```bash
# 1. Commit (or accept dirty-tree warning on deploy)
git add ... && git commit -m "..."

# 2. Build + install + restart + health check
make deploy

# 3. Verify
make deploy-check
```

**Do not assume `make build` updates the live site.**  
`make build` produces `backend/bin/server` **without** the `embed` tag and does not touch `/usr/local/bin/sub2api`.

| Command | Effect |
|---------|--------|
| `make build` | Dev compile only (`backend/bin/server`, no embedded UI) |
| `make build-release` | Production binary at `./sub2api` (embed UI), no sudo |
| `make deploy` | `build-release` + install to `/usr/local/bin/sub2api` + supervisor restart |
| `make deploy-check` | Compare git / dist staleness / installed binary / `/health` |

## Frontend rebuild policy

`deploy.sh` defaults to `FRONTEND_BUILD=auto`:

- Rebuilds `backend/internal/web/dist` when `frontend/src` is newer than `dist/index.html`
- Skips frontend build when dist is already fresh

Force rebuild: `FRONTEND_BUILD=1 make deploy`

## Deploy manifests

After build/deploy:

- `.deploy/last-build.json` — last `build-release` / deploy build
- `.deploy/last-deploy.json` — last successful `make deploy`

## Proxy

Surge HTTP proxy for builds that fetch modules: `127.0.0.1:6152` (see workspace rules in openai-free if needed). Runtime proxy is set in generated supervisor config.

## Common mistakes (agents)

1. Editing Vue/Go then only running `make build` → UI/API unchanged on :18080  
2. Leaving dashboard fixes uncommitted → hard to match running binary to git  
3. Using `backend/bin/server` for manual tests → no embedded frontend unless built with `-tags embed`
