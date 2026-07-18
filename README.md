# Inventra — Bank Fixed Asset Management System

[![CI](https://github.com/ragbuaj/inventra/actions/workflows/ci.yml/badge.svg)](https://github.com/ragbuaj/inventra/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](backend/go.mod)
[![Nuxt](https://img.shields.io/badge/Nuxt-4-00DC82?logo=nuxt&logoColor=white)](frontend/package.json)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](docs/DATABASE.md)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Web application for managing a **bank's fixed assets & inventory** (manajemen aset tetap; reference
context: Bank BTN): the full asset lifecycle from acquisition, assignment (check-out/check-in),
inter-office transfers (mutasi), maintenance, physical stock-take (stock opname), dual-basis
depreciation (commercial PSAK 16 + fiscal PMK 72/2023), disposal with gain/loss, reporting, and
**value-tiered maker-checker approvals** — all governed by configurable role-based authorization
across a 4-level office hierarchy (Pusat → Wilayah → Cabang/Unit → Outlet).

> This is **fixed/physical asset** management (buildings, vehicles, IT/ATM hardware, furniture),
> **not** investment/wealth asset management.

## Table of Contents

- [Features](#features)
- [Tech Stack](#tech-stack)
- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
  - [Run everything in Docker](#run-everything-in-docker-full-stack)
  - [Local development (run on host)](#local-development-run-on-host)
- [Testing](#testing)
- [Common Commands](#common-commands)
- [Documentation](#documentation)
- [Deployment](#deployment)
- [Project Status](#project-status)
- [Contributing](#contributing)
- [License](#license)

## Features

- **Configurable 3-layer authorization** — action permissions (RBAC), per-row data scope over the
  office hierarchy (`global` / `office_subtree` / `office` / `own`), and per-field view/edit
  permissions. All data-driven and Redis-cached; no hardcoded role matrix.
- **Master data** — offices (with floors & rooms), employees, asset categories, and a generic
  reference engine covering 11 flat reference tables (provinces, cities, brands, models, vendors…),
  plus an office location map.
- **Asset lifecycle** — asset catalog, detail & maintainable fields, attachments (MinIO), barcode/QR
  generation with printable label PDFs, and CSV bulk import.
- **Bank-grade operations** — inter-office transfers with BAST documents, physical stock opname
  (session + item reconciliation), disposal with SQL-computed gain/loss, and asset assignment.
- **Maker-checker approvals** — value-tiered segregation of duties (`approval_thresholds`) per
  POJK 17/2023 & 18/POJK.03/2016; every mutating action flows through a request/approval inbox.
- **Dual-basis depreciation** — commercial (PSAK 16) + fiscal (PMK 72/2023) schedules.
- **Reporting & dashboard** — 7 report types incl. the disposal gain/loss GL recap, with xlsx/pdf
  export and a Redis-cached dashboard summary.
- **Auth** — local login with JWT access/refresh (Redis token store + denylist) and Argon2 password
  hashing; optional Google sign-in (off by default).
- **Audit trail** — every mutation recorded with a before→after diff.
- **Production-ready ops** — HTTPS reverse proxy with a WAF (Caddy + Coraza/OWASP CRS), IaC
  (Ansible), and a self-hosted monitoring stack (Prometheus + Grafana + Loki + Alertmanager).

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.25 · Gin (modular monolith) |
| Database | PostgreSQL 16 (via sqlc + golang-migrate) |
| Cache & state | Redis 7 |
| Object storage | MinIO (S3-compatible) |
| Frontend | Nuxt 4 (Vue 3 + Vite, SPA) · Nuxt UI · Pinia · i18n (id/en) |
| Frontend tests | Vitest + @nuxt/test-utils (unit + runtime) · Playwright (e2e) |
| API docs | OpenAPI 3.1 (hand-maintained) · Scalar UI · Spectral lint |
| DevOps | Docker Compose · GitHub Actions · Caddy + WAF · Ansible · Prometheus/Grafana |

## Architecture

The backend is a **modular monolith**. `cmd/api/main.go` wires config + the PostgreSQL pool + Redis
and starts the HTTP server; `internal/server/router.go` (`NewRouter`) is the single composition root
where every feature module registers its routes under `/api/v1`. Each module follows a four-file split
by responsibility (`service.go` / `dto.go` / `handler.go` / `routes.go`).

The frontend is a **Nuxt 4 SPA** (`ssr: false`) built on the Nuxt UI component library with design
tokens, i18n, and a reusable global component layer. Feature screens are built 1:1 against the
high-fidelity mockups in [`docs/design/`](docs/design) and talk to the backend through a typed service
layer in `composables/api/`.

## Project Structure

```
asset-management/
├── backend/
│   ├── cmd/
│   │   ├── api/            # Entry point (HTTP server, graceful shutdown)
│   │   └── createadmin/    # One-off superadmin seeder
│   ├── internal/
│   │   ├── config/         # Env configuration
│   │   ├── auth/           # JWT (access/refresh), Argon2, Redis token store
│   │   ├── authz/          # RBAC · data scope · field permission (Redis-cached)
│   │   ├── identity/       # /auth/* (login/refresh/logout/me/permissions/scope)
│   │   ├── user/           # User management (CRUD + field filtering)
│   │   ├── masterdata/     # Offices/floors/rooms/employees/categories + reference engine
│   │   ├── asset/          # Asset core, attachments, barcode/label
│   │   ├── approval/       # Maker-checker request/approval engine
│   │   ├── assignment/ · transfer/ · maintenance/ · stockopname/ · disposal/
│   │   ├── depreciation/ · report/ · importer/ · audit/ · search/
│   │   ├── middleware/     # RequireAuth · RequirePermission · scope
│   │   └── server/         # NewRouter composition root
│   ├── db/{migrations,queries,sqlc}/   # SQL source · sqlc queries · generated Go
│   ├── api/openapi.yaml    # Hand-maintained spec (served at /docs, Spectral-linted)
│   └── sqlc.yaml
├── frontend/               # Nuxt 4 SPA
│   └── app/
│       ├── components/     # Global component library (auto-imported)
│       ├── composables/    # incl. api/ — typed service layer
│       ├── layouts/        # default (shell) + auth
│       ├── pages/          # assets · master · settings · approval · reports …
│       ├── middleware/ · stores/ · utils/
│       └── ../i18n/locales/{id,en}.json · ../test/ (Vitest) · ../e2e/ (Playwright)
├── docs/                   # PRD.md · DATABASE.md · ERD.md · PROGRESS.md · DEPLOYMENT.md · adr/ · design/
├── ops/                    # Caddy+WAF · Ansible · monitoring config
├── docker-compose.yml      # Production-like full stack (compiled images; CI e2e)
├── docker-compose.dev.yml  # Dev: infra by default · full stack + live reload via `--profile app watch`
├── docker-compose.prod.yml # Production stack (Caddy + WAF)
└── .github/workflows/      # ci.yml · deploy.yml
```

## Getting Started

### Run everything in Docker (full stack)

```bash
docker compose up --build
```

Brings up PostgreSQL, Redis, MinIO, runs database migrations, then starts the API and the frontend:

- Frontend → http://localhost:3000
- API → http://localhost:8080 (docs at `/docs`)
- MinIO console → http://localhost:9001

Seed a superadmin (one-off, while the stack runs) — from the host, since the `backend` image ships
only the API binary:

```bash
cd backend
go run ./cmd/createadmin -email admin@inventra.local -password admin12345
```

Stop with `docker compose down` (add `-v` to also drop the data volumes).

#### Full stack with live reload (`docker compose watch`)

Bring up the dev stack and have source edits hot-reload inside the containers — the backend rebuilds
via [Air](https://github.com/air-verse/air), the frontend via Vite HMR:

```bash
docker compose -f docker-compose.dev.yml --profile app watch
```

Docker Compose **syncs** changed files into each container (rather than bind-mounting), so the
in-container watchers see changes via normal inotify — including on Windows/WSL2, where bind-mount
events aren't delivered. Editing `go.mod`/`go.sum` or `package.json`/`pnpm-lock.yaml` triggers an image
rebuild for that service; `node_modules` stays in a named volume and is never synced from the host.

### Local development (run on host)

**Prerequisites:** Docker Desktop · Go 1.25+ · Node.js 24+ · pnpm 11+

**1. Start infrastructure**

```bash
docker compose -f docker-compose.dev.yml up -d
```

- PostgreSQL → `localhost:5433`
- Redis → `localhost:6379`
- MinIO → `localhost:9000` (console `localhost:9001`, `minioadmin` / `minioadmin123`)

**2. Backend**

```bash
cd backend
cp .env.example .env
go run ./cmd/api
```

API at `http://localhost:8080`. Liveness: `GET /health`. Readiness (checks PostgreSQL + Redis):
`GET /health/ready`.

> **Google sign-in** is optional and **off by default** (empty `GOOGLE_CLIENT_ID`). To enable it, see
> [docs/google-oauth-setup.md](docs/google-oauth-setup.md) — Google Cloud Console setup, the
> Testing/Internal/Production consent-screen choice, and the env variables.

**3. Frontend**

```bash
cd frontend
cp .env.example .env
pnpm install
pnpm dev
```

App at `http://localhost:3000`.

## Testing

```bash
# Backend
cd backend
go test ./...                              # unit tests
go test -tags=integration ./...            # integration tests (needs infra up)

# Frontend
cd frontend
pnpm test                                  # Vitest unit + Nuxt-runtime tests
pnpm test:e2e                              # Playwright e2e (needs backend stack up + seeded admin)
```

CI (`.github/workflows/ci.yml`) runs the backend build/vet/test, the frontend
lint/typecheck/test/build, the Spectral OpenAPI lint, and a separate e2e job (docker-compose backend +
seeded admin → Playwright).

## Common Commands

```bash
# Backend
cd backend
go build ./...
go vet ./...
go test ./...
sqlc generate                              # regenerate db/sqlc from SQL

# Database migrations (golang-migrate)
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
export DATABASE_URL="postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable"
migrate -path db/migrations -database "$DATABASE_URL" up
migrate -path db/migrations -database "$DATABASE_URL" down 1

# OpenAPI lint
npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml

# Frontend
cd frontend
pnpm dev | pnpm build | pnpm lint | pnpm typecheck
```

## Documentation

| Doc | Contents |
|---|---|
| [docs/PRD.md](docs/PRD.md) | Product requirements, roles, functional requirements; bank-FAM regulatory citations (Lampiran A) |
| [docs/DATABASE.md](docs/DATABASE.md) | Schema, conventions, data dictionary |
| [docs/ERD.md](docs/ERD.md) | Consolidated entity-relationship diagrams |
| [docs/PROGRESS.md](docs/PROGRESS.md) | Living tracker of what's built vs. remaining |
| [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) | VPS deployment guide (Docker Compose + Caddy + WAF) |
| [docs/adr/](docs/adr/) | Architecture Decision Records (MADR) |
| [docs/design/](docs/design) | High-fidelity screen mockups (source of truth for the UI) |

The design docs are written in Indonesian. The live OpenAPI 3.1 spec is served at `/docs` (Scalar UI).

## Deployment

Inventra ships a production stack that runs on a single VPS: `docker-compose.prod.yml` brings up
PostgreSQL, Redis, MinIO, the Go API, the Nuxt frontend, and a **Caddy reverse proxy with automatic
HTTPS and a WAF** (Coraza + OWASP CRS). Infrastructure is provisioned with **Ansible** (`ops/ansible/`)
and an optional self-hosted monitoring overlay (`docker-compose.monitoring.yml`: Prometheus, Grafana,
Loki, Alertmanager) is available. See [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md) for the full guide.

## Project Status

**In active development**, built in phases per the PRD roadmap (section 10). The core system is
feature-complete: identity & 3-layer authorization, master data, the full asset lifecycle (catalog,
attachments, barcode, import), maker-checker approvals, assignment, transfers, maintenance, stock
opname, disposal, dual-basis depreciation, reporting/dashboard, and the audit trail — backend modules
and their matching frontend screens are wired to the real API. Production ops hardening (WAF, IaC,
monitoring) is in place.

Remaining work is polish and follow-ups (notifications feed, backend global search, analytics/OLAP
read layer). See [docs/PROGRESS.md](docs/PROGRESS.md) for the authoritative, up-to-date checklist.

## Contributing

- **Branch per feature**, named `feat/<short-topic>`; `main` is the integration branch.
- **Conventional Commits with a scope** matching the module/area
  (`feat(masterdata): …`, `fix(security): …`, `feat(authz): …`).
- **Verify before committing** — for the backend: `go build ./...`, `go vet ./...`, `go test ./...`,
  and the Spectral lint; for the frontend: `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`.
  These are exactly what CI enforces.
- Keep [docs/PROGRESS.md](docs/PROGRESS.md) and [backend/api/openapi.yaml](backend/api/openapi.yaml)
  in sync with your changes.

See [CLAUDE.md](CLAUDE.md) for the full architecture and conventions guide.

## License

Released under the [MIT License](LICENSE).
