# RegiKeep

**Container Registry Retention Manager** — Prevent production images from being deleted by registry retention policies.

Container registries (OCIR, ECR, Docker Hub) run retention policies that delete images based on age or inactivity. RegiKeep keeps your critical images alive by automatically pulling manifests, re-tagging, or applying native protection rules — so a stable production image isn't garbage-collected just because it hasn't been pulled recently.

---

## The Problem

| Scenario | What happens | Impact |
|----------|-------------|--------|
| Retention deletes a prod image that hasn't been pulled recently | Rollback or scale-out fails | **High** |
| A shared base image expires across multiple services | Cascading build failures | **High** |
| Compliance-required image versions are purged | Audit violations | **Medium** |
| Archive images are lost with no recovery path | Permanent data loss | **Critical** |

## How RegiKeep Solves It

1. **Track** — Register images you care about into named groups
2. **Keep alive** — The daemon runs on a schedule and keeps images alive using the best strategy for each registry
3. **Monitor** — Dashboard shows status, expiry, and alerts for at-risk images
4. **Archive** — Cold-archive images to object storage before they expire

---

## Quick Start

```bash
git clone https://github.com/your-org/regikeep.git
cd regikeep
cp .env.example .env    # fill in your registry credentials

docker compose up -d    # backend :8080, frontend :3000
```

Open [http://localhost:3000](http://localhost:3000) for the dashboard.

### Configure a Registry

```bash
# Docker Hub
docker exec regikeep-backend rgk config add-registry docker.io \
  --username myuser --token 'dckr_pat_xxx'

# Oracle OCIR
docker exec regikeep-backend rgk config add-registry fra.ocir.io \
  --region eu-frankfurt-1 --tenancy mytenancy \
  --username 'tenancy/user@example.com' --token 'authtoken' \
  --extra 'ocid1.compartment...'

# AWS ECR
docker exec regikeep-backend rgk config add-registry 123456789.dkr.ecr.us-east-1.amazonaws.com \
  --region us-east-1 --username 'AKIAXXXXXXXX' --token 'secretkey' --extra '123456789'

# Verify
docker exec regikeep-backend rgk config test
```

### Track Images

```bash
# Create a group
rgk group create production --interval 7d --strategy pull

# Add images
rgk add docker.io/library/nginx:1.27 --group production
rgk add fra.ocir.io/mytenancy/myapp:v2.1.0 --group production

# Check status
rgk status --group production
```

---

## Architecture

```
regikeep/
├── backend/              # Go API + CLI (cmd/rgk, internal/)
│   ├── cmd/rgk/          # CLI commands (cobra)
│   ├── internal/
│   │   ├── api/          # HTTP server + handlers (chi v5)
│   │   ├── config/       # Environment + DB config loading
│   │   ├── core/         # Business logic (keepalive, audit, archive)
│   │   ├── daemon/       # Background scheduler
│   │   ├── registry/     # Registry adapters (OCIR, ECR, DockerHub)
│   │   └── store/        # SQLite data layer (WAL mode)
│   ├── Dockerfile        # Multi-stage Go build
│   └── go.mod
├── frontend/             # React dashboard (TypeScript, Vite, shadcn/ui)
│   ├── src/
│   ├── Dockerfile        # Multi-stage Node build + nginx
│   └── nginx.conf        # Reverse proxy to backend
├── docker-compose.yml    # Full stack: backend + frontend
├── Makefile              # dev / build / test / docker
└── .env.example          # All configuration variables
```

### Tech Stack

| Layer | Technology |
|-------|-----------|
| **Backend** | Go 1.22, chi v5, cobra CLI, zerolog |
| **Database** | SQLite (WAL mode, pure Go via modernc.org/sqlite) |
| **Frontend** | React 18, TypeScript, Vite, Tailwind CSS, shadcn/ui |
| **Metrics** | Prometheus (`/metrics` endpoint) |
| **Container** | Multi-stage Docker builds, Docker Compose |

---

## CLI Reference

```
rgk serve [--addr :8080] [--db /data/regikeep.db]    Start the API server

rgk config add-registry <endpoint> [--username] [--token] [--extra]
rgk config delete-registry <endpoint>
rgk config set-credential <endpoint> --source env|docker|k8s
rgk config list [--json]
rgk config test [endpoint]

rgk group create <name> --interval 7d --strategy pull|retag|native
rgk group list
rgk group enable|disable <name>
rgk group delete <name> [--force]

rgk add <registry/repo:tag> --group <name>
rgk remove <repo:tag> [--id <uuid>]
rgk status [--group <name>] [--watch]

rgk keepalive [--group <name>] [--strategy pull|retag|native]
rgk audit [--registry <name>]
rgk history <image>
rgk export --format oracle-json|ecr-json|csv [--output file]

rgk archive run [--group <name>] [--image <ref>] --to s3|oci-os
rgk archive list
rgk restore <repo:tag> --from <archive-id> --to <registry>

rgk daemon start|stop|status|restart
```

### Keepalive Strategies

| Strategy | How it works | Best for |
|----------|-------------|----------|
| `pull` | Pulls the image manifest (resets "last pulled" timestamp) | Registries that delete based on inactivity |
| `retag` | Re-tags the image with the same tag (resets age timers) | Registries that delete based on age since push |
| `native` | Applies registry-native protection (lock, pin, exemption) | Registries that support retention exemptions (OCIR) |

---

## API

### Supabase-Compatible Routes (`/functions/v1/*`)

The UI connects via these routes, matching the Supabase Edge Function URL pattern:

| Method | Path | Action |
|--------|------|--------|
| `GET` | `/functions/v1/registry-images` | List images (supports `?registry=&status=&search=`) |
| `POST` | `/functions/v1/registry-images` | Pin, unpin, export, set-group, remove-group |
| `POST` | `/functions/v1/audit` | Run retention audit |
| `POST` | `/functions/v1/keepalive` | Trigger keepalive |
| `GET` | `/functions/v1/archive` | List archives |
| `POST` | `/functions/v1/archive` | Archive or restore |
| `POST` | `/functions/v1/trivy-scan` | Vulnerability scan |

### REST API (`/api/v1/*`)

Structured API for CLI and programmatic use:

```
GET/POST        /api/v1/groups
PATCH/DELETE    /api/v1/groups/:id
POST            /api/v1/groups/:id/enable
POST            /api/v1/groups/:id/disable

GET/POST        /api/v1/images
DELETE          /api/v1/images/:id
POST            /api/v1/images/:id/pin|unpin|keepalive
GET             /api/v1/images/:id/history

POST            /api/v1/audit
GET/POST        /api/v1/archive
GET             /api/v1/archive/stats
POST            /api/v1/archive/:id/restore
GET             /api/v1/export?format=oracle-json|ecr-json|csv

GET             /api/v1/daemon/status
POST            /api/v1/daemon/start|stop

GET/POST        /api/v1/registries
POST            /api/v1/registries/test
DELETE          /api/v1/registries/:id

GET             /api/v1/dockerhub/search?q=nginx
GET             /metrics                          (Prometheus)
GET             /healthz                          (Health check)
```

---

## Registry Support

| Registry | Auth | Keepalive | Digest Resolve | List Images | Test |
|----------|------|-----------|---------------|-------------|------|
| **Oracle OCIR** | Basic auth | pull, retag, native | Yes | Yes | Yes |
| **Docker Hub** | Token auth (auth.docker.io) | pull | Yes | Yes | Yes |
| **AWS ECR** | Endpoint connectivity | Planned | Planned | Planned | Yes |
| **Azure ACR** | Planned | Planned | Planned | Planned | Planned |
| **Harbor** | Planned | Planned | Planned | Planned | Planned |

---

## Dashboard

The web UI at `http://localhost:3000` provides:

- **Dashboard** — Image overview with status badges, registry tabs, stats cards
- **Groups** — Create/enable/disable/delete groups, assign/unassign images
- **Audit** — Run retention audits, see at-risk images
- **Archive** — View archived images, trigger archive/restore
- **Export** — Export image lists as Oracle JSON, ECR JSON, or CSV
- **Settings** — Configure registries with credentials, test connectivity
- **Docker Hub Search** — Search and discover images (proxied through backend)

---

## Configuration

All configuration is via `rgk config` CLI commands or the Settings UI. Registry credentials are stored in the SQLite database (not environment variables).

### Environment Variables

Only server-level config uses environment variables (see [.env.example](.env.example)):

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8080` | Server listen address |
| `DB_PATH` | `/data/regikeep.db` | SQLite database path |
| `DAEMON_WORKERS` | `4` | Concurrent keepalive workers |
| `DAEMON_AUTO_START` | `false` | Start scheduler on boot |
| `ALLOWED_ORIGINS` | `*` | CORS allowed origins |
| `TRIVY_SERVER_URL` | — | Trivy server for vulnerability scanning |

---

## Observability

### Prometheus Metrics (`/metrics`)

| Metric | Description |
|--------|-------------|
| `regikeep_keepalive_success_total` | Successful keepalive attempts |
| `regikeep_keepalive_failure_total` | Failed keepalive attempts |
| `regikeep_keepalive_last_success_timestamp` | Last successful keepalive per image |
| `regikeep_archive_bytes_total` | Total bytes archived |

### Structured Logging

Every keepalive attempt logs JSON with: `image_id`, `repo`, `tag`, `registry`, `strategy`, `duration_ms`, `success`, `error`.

3 consecutive failures for any image produce a `WARN` log entry.

---

## Development

```bash
# Start both services with hot reload
make dev

# Build binaries
make build

# Build CLI only
make cli

# Run tests
make test

# Docker build + start
make docker
```

---

## License

MIT
