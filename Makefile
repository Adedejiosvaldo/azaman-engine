PORT       ?= 8080
LAYA_PORT  ?= 8001
VENV       := sidecar/.venv
PY         := $(VENV)/bin/python

.PHONY: dev api sidecar setup test build clean

# Run the Go API and the Laya sidecar together; Ctrl-C stops both.
dev: $(VENV)
	@trap 'kill 0' INT TERM EXIT; \
	$(MAKE) --no-print-directory sidecar & \
	$(MAKE) --no-print-directory api & \
	wait

api:
	LAYA_URL=http://127.0.0.1:$(LAYA_PORT) PORT=$(PORT) go run ./cmd/server

sidecar: $(VENV)
	$(PY) -m uvicorn laya_service:app --app-dir sidecar --host 127.0.0.1 --port $(LAYA_PORT)

setup: $(VENV)

# The laya package is optional: the sidecar falls back to heuristics without it.
$(VENV): sidecar/requirements.txt
	python3 -m venv $(VENV)
	$(PY) -m pip install -q --upgrade pip
	$(PY) -m pip install -q -r sidecar/requirements.txt || \
		(echo "laya unavailable; installing core sidecar deps only" && \
		 $(PY) -m pip install -q fastapi uvicorn pydantic)
	@touch $(VENV)

test:
	go test ./...

build:
	go build -o bin/azaman ./cmd/server

clean:
	rm -rf bin tmp $(VENV)
