#!/usr/bin/env bash
# Runs the Laya sidecar and the Go engine in one container.
# If either process exits, the other is stopped and the container exits non-zero,
# so the platform restarts the whole unit instead of running half-alive.
set -euo pipefail

cd /app

python -m uvicorn sidecar.laya_service:app \
  --host 127.0.0.1 --port 8001 --workers 1 --no-access-log &
sidecar_pid=$!

/app/server &
engine_pid=$!

shutdown() {
  kill -TERM "$engine_pid" "$sidecar_pid" 2>/dev/null || true
  wait "$engine_pid" "$sidecar_pid" 2>/dev/null || true
}
trap shutdown TERM INT

# Wait for whichever process exits first
set +e
wait -n "$sidecar_pid" "$engine_pid"
status=$?
set -e

if kill -0 "$engine_pid" 2>/dev/null; then
  echo "[entrypoint] Laya sidecar exited (status $status); stopping engine" >&2
else
  echo "[entrypoint] Engine exited (status $status); stopping sidecar" >&2
fi
shutdown
exit $(( status == 0 ? 1 : status ))
