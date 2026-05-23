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
git add ... && git commit -m "..."   # recommended
make deploy                          # one-shot: frontend + backend + restart + verify
make deploy-check                    # optional sanity check
```

**Do not assume `make build` updates the live site.**

| Command | Effect |
|---------|--------|
| `make build` | Dev compile only (`backend/bin/server`, no embedded UI) |
| `make deploy` | **Default release** — always rebuilds `frontend` → `dist`, embeds into binary, installs, restarts |
| `make deploy-fast` | Same install path, but skips frontend build if sources hash unchanged |
| `make build-release` | Build `./sub2api` only (no sudo); uses `FRONTEND_BUILD=auto` |
| `make deploy-check` | Compare git / dist fingerprint / installed binary / `/health` |

## Why `make deploy` always rebuilds the frontend

mtime-based `auto` mode is **not reliable** (e.g. dist newer than a source file while content is still old).  
Full deploy therefore defaults to `FRONTEND_BUILD=1` and runs an embed sanity check (`today_active_accounts` in the admin dashboard bundle).

Use `make deploy-fast` only when you changed **backend-only** and want to save ~20s.

## Deploy manifests

- `.deploy/last-build.json` — last build (includes `frontend_sources_hash`)
- `.deploy/last-deploy.json` — last successful install

## Common mistakes (agents)

1. `make build` then expecting UI changes on :18080  
2. `FRONTEND_BUILD=0 make deploy` — skips UI rebuild on purpose; rarely what you want  
3. `backend/bin/server` — no embedded UI unless built with `-tags embed`
