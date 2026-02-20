BLOB_DIR ?= .artifacts
BLOB_SIZE_MB ?= 64
DOCKER_COMPOSE ?= docker compose
SERVER_PORT_MAP ?= 127.0.0.1:8080:8080
PPROF_PORT_MAP ?= 127.0.0.1:6060:6060
DOCKER_HEALTHCHECK_URL ?= http://127.0.0.1:8080/healthz
PPROF_BASE_URL ?= http://127.0.0.1:6060/debug/pprof
PPROF_SECONDS ?= 30
PPROF_DIR ?= $(BLOB_DIR)/pprof
PPROF_UI_CPU_ADDR ?= 127.0.0.1:18080
PPROF_UI_HEAP_ADDR ?= 127.0.0.1:18081

DOCKER_ENV = SERVER_PORT_MAP="$(SERVER_PORT_MAP)" PPROF_PORT_MAP="$(PPROF_PORT_MAP)"
PPROF_CPU_FILE = $(PPROF_DIR)/cpu-$(PPROF_SECONDS)s.pb.gz
PPROF_HEAP_FILE = $(PPROF_DIR)/heap.pb.gz

.PHONY: help docker-server docker-e2e docker-e2e-profile dcoker-e2e-profile docker-down pprof-cpu pprof-heap pprof-ui-cpu pprof-ui-heap

help:
	@echo "Targets:"
	@echo "  make docker-server        # run only server in docker (for profiling)"
	@echo "  make docker-e2e           # blob + server + client inside docker compose"
	@echo "  make docker-e2e-profile   # isolated flow: server + transfer-aligned cpu+heap profile"
	@echo "  make docker-down          # stop compose and remove volumes"
	@echo "  make pprof-cpu            # download CPU profile from pprof endpoint"
	@echo "  make pprof-heap           # download heap profile from pprof endpoint"
	@echo "  make pprof-ui-cpu         # open latest CPU profile in pprof web UI"
	@echo "  make pprof-ui-heap        # open heap profile in pprof web UI"
	@echo ""
	@echo "Variables:"
	@echo "  BLOB_SIZE_MB=$(BLOB_SIZE_MB)"
	@echo "  DOCKER_COMPOSE=$(DOCKER_COMPOSE)"
	@echo "  SERVER_PORT_MAP=$(SERVER_PORT_MAP)"
	@echo "  PPROF_PORT_MAP=$(PPROF_PORT_MAP)"
	@echo "  DOCKER_HEALTHCHECK_URL=$(DOCKER_HEALTHCHECK_URL)"
	@echo "  PPROF_BASE_URL=$(PPROF_BASE_URL)"
	@echo "  PPROF_SECONDS=$(PPROF_SECONDS)"
	@echo "  PPROF_DIR=$(PPROF_DIR)"
	@echo "  PPROF_UI_CPU_ADDR=$(PPROF_UI_CPU_ADDR)"
	@echo "  PPROF_UI_HEAP_ADDR=$(PPROF_UI_HEAP_ADDR)"

docker-e2e:
	@set -e; \
	trap '$(DOCKER_COMPOSE) down -v --remove-orphans >/dev/null 2>&1 || true' EXIT INT TERM; \
	$(DOCKER_COMPOSE) down -v --remove-orphans >/dev/null 2>&1 || true; \
	$(DOCKER_ENV) BLOB_SIZE_MB="$(BLOB_SIZE_MB)" $(DOCKER_COMPOSE) up --build --abort-on-container-exit --exit-code-from client client

docker-server:
	@set -e; \
	$(DOCKER_ENV) $(DOCKER_COMPOSE) up -d --build server; \
	attempt=0; \
	until curl -fsS "$(DOCKER_HEALTHCHECK_URL)" >/dev/null 2>&1; do \
		attempt=$$((attempt + 1)); \
		if [ $$attempt -ge 100 ]; then \
			echo "docker server did not become ready in time"; \
			$(DOCKER_COMPOSE) logs server; \
			exit 1; \
		fi; \
		sleep 0.1; \
	done; \
	echo "docker server is ready at $(DOCKER_HEALTHCHECK_URL)"

docker-e2e-profile:
	@set -e; \
	trap 'if [ -n "$$load_pid" ]; then kill $$load_pid >/dev/null 2>&1 || true; wait $$load_pid >/dev/null 2>&1 || true; fi; $(DOCKER_COMPOSE) down -v --remove-orphans >/dev/null 2>&1 || true' EXIT INT TERM; \
	$(DOCKER_COMPOSE) down -v --remove-orphans >/dev/null 2>&1 || true; \
	$(DOCKER_ENV) $(DOCKER_COMPOSE) up -d --build server; \
	attempt=0; \
	until curl -fsS "$(DOCKER_HEALTHCHECK_URL)" >/dev/null 2>&1; do \
		attempt=$$((attempt + 1)); \
		if [ $$attempt -ge 100 ]; then \
			echo "docker server did not become ready in time"; \
			$(DOCKER_COMPOSE) logs server; \
			exit 1; \
		fi; \
		sleep 0.1; \
	done; \
	$(DOCKER_ENV) BLOB_SIZE_MB="$(BLOB_SIZE_MB)" $(DOCKER_COMPOSE) run --rm blobgen >/dev/null; \
	elapsed=$$( { time -p $(DOCKER_ENV) $(DOCKER_COMPOSE) run --rm client >/dev/null; } 2>&1 | awk '/^real /{print $$2}'); \
	if [ -z "$$elapsed" ]; then \
		echo "cannot measure upload duration"; \
		exit 1; \
	fi; \
	profile_seconds=$$(awk -v d="$$elapsed" 'BEGIN { s=int(d + 0.999); if (s < 1) s = 1; print s }'); \
	echo "measured upload duration: $${elapsed}s, profiling for $${profile_seconds}s"; \
	cpu_file="$(PPROF_DIR)/cpu-$${profile_seconds}s.pb.gz"; \
	heap_file="$(PPROF_HEAP_FILE)"; \
	($(DOCKER_ENV) $(DOCKER_COMPOSE) run --rm client >/dev/null 2>&1) & \
	load_pid=$$!; \
	mkdir -p "$(PPROF_DIR)"; \
	curl --max-time $$((profile_seconds + 15)) -fsS "$(PPROF_BASE_URL)/profile?seconds=$$profile_seconds" -o "$$cpu_file"; \
	wait $$load_pid >/dev/null 2>&1 || true; \
	load_pid=""; \
	curl -fsS "$(PPROF_BASE_URL)/heap" -o "$$heap_file"; \
	ls -lh "$$cpu_file" "$$heap_file"

pprof-cpu:
	@mkdir -p "$(PPROF_DIR)"
	curl -fsS "$(PPROF_BASE_URL)/profile?seconds=$(PPROF_SECONDS)" -o "$(PPROF_CPU_FILE)"
	@ls -lh "$(PPROF_CPU_FILE)"

pprof-heap:
	@mkdir -p "$(PPROF_DIR)"
	curl -fsS "$(PPROF_BASE_URL)/heap" -o "$(PPROF_HEAP_FILE)"
	@ls -lh "$(PPROF_HEAP_FILE)"

pprof-ui-cpu:
	@cpu_file=$$(ls -1t "$(PPROF_DIR)"/cpu-*.pb.gz 2>/dev/null | head -n1); \
	if [ -z "$$cpu_file" ]; then \
		echo "no CPU profile found in $(PPROF_DIR). Run make pprof-cpu or make docker-e2e-profile first."; \
		exit 1; \
	fi; \
	echo "opening $$cpu_file at http://$(PPROF_UI_CPU_ADDR)"; \
	go tool pprof -http="$(PPROF_UI_CPU_ADDR)" "$$cpu_file"

pprof-ui-heap:
	@if [ ! -f "$(PPROF_HEAP_FILE)" ]; then \
		echo "heap profile not found at $(PPROF_HEAP_FILE). Run make pprof-heap or make docker-e2e-profile first."; \
		exit 1; \
	fi
	@echo "opening $(PPROF_HEAP_FILE) at http://$(PPROF_UI_HEAP_ADDR)"
	go tool pprof -http="$(PPROF_UI_HEAP_ADDR)" "$(PPROF_HEAP_FILE)"

docker-down:
	$(DOCKER_COMPOSE) down -v --remove-orphans
