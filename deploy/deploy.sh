#!/usr/bin/env bash
# Deploys the latest main to this server: pull, rebuild, wait until healthy, roll back if not.
#
# Run on the VPS from anywhere:   bash ~/azaman-engine/deploy/deploy.sh
# CI runs it over SSH after the tests pass (.github/workflows/deploy.yml).
#
# APP_DIR overrides where the repo lives (default ~/azaman-engine).
set -euo pipefail

APP_DIR="${APP_DIR:-$HOME/azaman-engine}"
COMPOSE=(docker compose -f deploy/docker-compose.yml)
cd "$APP_DIR"

log() { printf '[deploy] %s\n' "$*"; }

# Local edits to tracked files would be overwritten or block the pull: stop and say so rather
# than guess. Settings belong in deploy/.env, which git ignores.
if ! git diff --quiet || ! git diff --cached --quiet; then
  log "Local changes to tracked files on the server:"
  git status --short --untracked-files=no
  log "Move settings into deploy/.env (or run: git checkout -- <file>), then deploy again."
  exit 1
fi

prev=$(git rev-parse HEAD)
git fetch --quiet origin main
next=$(git rev-parse origin/main)
if [[ "$prev" == "$next" && "${FORCE:-}" != "1" ]]; then
  log "Already at ${next:0:7}; nothing to deploy (FORCE=1 rebuilds anyway)."
  exit 0
fi
git merge --ff-only --quiet origin/main
log "Updating ${prev:0:7} -> ${next:0:7}"

domain=$(grep -E '^DOMAIN=' deploy/.env | cut -d= -f2- | tr -d '"'"'"' ')
if [[ -z "$domain" ]]; then
  log "DOMAIN is not set in deploy/.env"
  exit 1
fi

healthy() {
  # The engine answers through Caddy; "status":"ok" means it is serving audits.
  for _ in $(seq 1 40); do
    if curl -fsS --max-time 5 "https://$domain/health" 2>/dev/null | grep -q '"status":"ok"'; then
      return 0
    fi
    sleep 3
  done
  return 1
}

"${COMPOSE[@]}" up -d --build
if healthy; then
  log "Healthy on ${next:0:7}"
  docker image prune -f >/dev/null # old build layers; keeps the disk from filling up
  exit 0
fi

log "Not healthy after 2 minutes. Last engine logs:"
"${COMPOSE[@]}" logs --tail 40 azaman || true
log "Rolling back to ${prev:0:7}"
git reset --hard --quiet "$prev"
"${COMPOSE[@]}" up -d --build
if healthy; then
  log "Rolled back; ${prev:0:7} is serving. Fix ${next:0:7} and push again."
else
  log "Rollback is not healthy either. Check the server by hand."
fi
exit 1
