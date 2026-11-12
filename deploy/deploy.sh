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

# Hand edits to tracked files on the server would block the update. Set them aside (nothing is
# lost: "git stash list" shows them, "git stash show -p" prints them) and deploy main as it is.
# Server settings belong in deploy/.env, which git ignores and this never touches.
if ! git diff --quiet || ! git diff --cached --quiet; then
  log "Setting aside hand edits on the server:"
  git status --short --untracked-files=no
  git stash push --quiet -m "deploy $(date -u +%Y-%m-%dT%H:%MZ): hand edits set aside"
  log "Saved in git stash. Put settings in deploy/.env instead (e.g. AZAMAN_MEM_LIMIT, LAYA_DISABLE)."
fi

prev=$(git rev-parse HEAD)
git fetch --quiet origin main
# TARGET is the commit CI tested. Deploying it, not whatever main is now, means a commit pushed
# while the tests ran is never deployed untested; its own run deploys it.
next=$(git rev-parse "${TARGET:-origin/main}^{commit}")
if ! git merge-base --is-ancestor "$next" origin/main; then
  log "${next:0:7} is not on main; refusing to deploy it."
  exit 1
fi
if [[ "$prev" == "$next" && "${FORCE:-}" != "1" ]]; then
  log "Already at ${next:0:7}; nothing to deploy (FORCE=1 rebuilds anyway)."
  exit 0
fi
if git merge-base --is-ancestor "$next" "$prev"; then
  log "Server is already ahead (${prev:0:7}); not moving back to ${next:0:7}."
  exit 0
fi
git merge --ff-only --quiet "$next"
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
