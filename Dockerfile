# syntax=docker/dockerfile:1

# ---------- Stage 1: Go engine (static binary) ----------
FROM golang:1.26-bookworm AS engine
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---------- Stage 2: runtime (engine + Laya sidecar in one container) ----------
FROM python:3.12-slim-bookworm

# Laya model checkpoints are baked into the image at build time, so the container needs no
# network access or writable disk at runtime (zero-disk policy; works with --read-only).
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    HF_HOME=/opt/models/hf \
    GIN_MODE=release \
    PORT=8080 \
    LAYA_URL=http://127.0.0.1:8001

RUN apt-get update \
 && apt-get install -y --no-install-recommends tini \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# CPU-only torch first (the default wheel bundles CUDA and is ~5GB larger)
RUN pip install --index-url https://download.pytorch.org/whl/cpu torch
COPY sidecar/requirements.txt sidecar/requirements.txt
RUN pip install -r sidecar/requirements.txt

# Prefetch Laya checkpoints. Set --build-arg PRELOAD_LAYA=0 to skip (sidecar then runs on fallback rules).
ARG PRELOAD_LAYA=1
RUN if [ "$PRELOAD_LAYA" = "1" ]; then \
      python -c "from laya import Router; Router(preload=True); print('Laya checkpoints cached')"; \
    fi

COPY sidecar/laya_service.py sidecar/memo_model.py sidecar/categories.json sidecar/
# A trained memo model is personal data: mount it at runtime
# (-v ./sidecar/models:/app/sidecar/models:ro), never bake it into the image.
COPY --from=engine /out/server /app/server
COPY docker/entrypoint.sh /app/entrypoint.sh
RUN chmod 0555 /app/entrypoint.sh /app/server \
 && useradd --system --uid 10001 --no-create-home azaman

# Runtime is offline: never try to download or update models
ENV HF_HUB_OFFLINE=1 \
    TRANSFORMERS_OFFLINE=1 \
    HOME=/tmp

USER azaman

# Only the engine is published. The sidecar listens on 127.0.0.1:8001 inside the container.
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=60s --retries=3 \
  CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://127.0.0.1:8080/health', timeout=2).status == 200 else 1)"

ENTRYPOINT ["/usr/bin/tini", "--", "/app/entrypoint.sh"]
