#!/usr/bin/env bash
# Runs the Laya sidecar and the Go engine in one container.
# The engine is what users need; Laya only sharpens low-confidence memos and the engine already
# falls back to its own rules without it. So if Laya dies (e.g. killed for memory while loading
# its models) the engine keeps serving. Only an engine exit stops the container.
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
  echo "[entrypoint] Laya sidecar exited (status $status); engine keeps running on fallback rules" >&2
  if [ "$status" -eq 137 ]; then
    echo "[entrypoint] Status 137 usually means Laya ran out of memory while loading its models" >&2
  fi
  set +e
  wait "$engine_pid"
  status=$?
  set -e
fi
echo "[entrypoint] Engine exited (status $status); stopping" >&2
shutdown
exit $(( status == 0 ? 1 : status ))
