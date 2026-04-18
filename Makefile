# RegiKeep Makefile
# Targets: dev, build, cli, test, docker

.PHONY: dev build cli test docker clean

# ─── Development ────────────────────────────────────────────────────────────

## Start both backend and frontend with hot reload
dev:
	@echo "Starting RegiKeep backend on :8080 and frontend on :5173"
	@trap 'kill %1 %2 2>/dev/null' INT; \
	  (cd backend && go run ./cmd/rgk serve) & \
	  (cd frontend && npm run dev) & \
	  wait

## Start only the backend server
dev-backend:
	cd backend && go run ./cmd/rgk serve

## Start only the frontend dev server
dev-frontend:
	cd frontend && npm run dev

# ─── Build ───────────────────────────────────────────────────────────────────

## Build Go binary + React static files
build: build-backend build-frontend

build-backend:
	cd backend && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ../cli/rgk ./cmd/rgk

build-frontend:
	cd frontend && npm run build

## Build rgk CLI binary to ./cli/rgk
cli:
	cd backend && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ../cli/rgk ./cmd/rgk
	@echo "Built cli/rgk"

# ─── Test ────────────────────────────────────────────────────────────────────

## Run Go tests + frontend type check
test: test-backend test-frontend

test-backend:
	cd backend && go test ./...

test-frontend:
	cd frontend && npx tsc --noEmit

# ─── Docker ──────────────────────────────────────────────────────────────────

## Build and start the full stack with Docker Compose
docker:
	docker compose up --build -d

## Build Docker images only (no start)
docker-build:
	docker compose build

## Stop Docker stack
docker-down:
	docker compose down

# ─── Utilities ───────────────────────────────────────────────────────────────

## Install frontend dependencies
install:
	cd frontend && npm install

## Tidy Go module
tidy:
	cd backend && go mod tidy

clean:
	rm -rf cli/rgk frontend/dist
