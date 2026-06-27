# Inventra — Bank Fixed Asset Management System

Web application for managing a **bank's fixed assets & inventory** (manajemen aset tetap; reference
context: Bank BTN): asset lifecycle from acquisition, check-out/check-in, inter-office transfers
(mutasi), maintenance, physical stock-take (stock opname), dual-basis depreciation (commercial +
fiscal), disposal, reporting, and value-tiered approvals — with configurable role-based authorization
across a 4-level office hierarchy (Pusat → Wilayah → Cabang/Unit → Outlet). This is **fixed/physical
asset** management, **not** investment/wealth asset management.

Full requirements: [docs/PRD.md](docs/PRD.md) · database design: [docs/DATABASE.md](docs/DATABASE.md) ·
entity-relationship diagrams: [docs/ERD.md](docs/ERD.md) · architecture decisions:
[docs/adr/](docs/adr/) · live progress tracker: [docs/PROGRESS.md](docs/PROGRESS.md).

> Status: **in active development**, built in phases per the PRD roadmap (§10).
>
> - **Backend:** identity & 3-layer authorization (RBAC + data scope + field permission), user
>   management, and master data (offices · floors · rooms · employees · categories · 11 reference
>   resources) — all data-scoped and access-controlled. OpenAPI 3.1 spec served at `/docs`.
> - **Frontend:** SPA foundation (app shell, design system, real auth, reusable component library,
>   Vitest + Playwright), plus the **Master Data** feature screens (Kantor · Pegawai · Referensi)
>   built to match the `docs/design` mockups. Feature screens are **mock-first** — backed by typed
>   `composables/api/` services that swap to the real API as each backend module lands.
> - **Remaining:** asset core, attachments, barcode, approval, assignment, maintenance,
>   depreciation, reporting, and import — plus the **PRD v1.1 bank-FAM** additions (mutasi,
>   stock opname, BAST, dual-basis depreciation, disposal, value-tiered approval, intangible) —
>   see [docs/PROGRESS.md](docs/PROGRESS.md).
>
> Design docs were updated to **PRD v1.1** (bank fixed-asset scope) with sourced regulatory citations
> (PSAK 16/19/48, PMK 72/2023, POJK 17/2023 & 18/POJK.03/2016) in [docs/PRD.md](docs/PRD.md) Lampiran A.
> The core schema (migrations `000001`–`000014`) is implemented; v1.1 schema additions are planned as
> migrations `000015`–`000021` (not yet written).

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.25 · Gin |
| Database | PostgreSQL 16 (via sqlc + golang-migrate) |
| Cache & state | Redis 7 |
| Object storage | MinIO (S3-compatible) |
| Frontend | Nuxt 4 (Vue 3 + Vite, SPA) · Nuxt UI · Pinia · i18n (id/en) |
| Frontend tests | Vitest + @nuxt/test-utils (unit + runtime) · Playwright (e2e) |
| API docs | OpenAPI 3.1 (hand-maintained) · Scalar UI · Spectral lint |
| DevOps | Docker Compose · GitHub Actions |

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
│   │   ├── middleware/     # RequireAuth · RequirePermission · scope
│   │   └── server/         # NewRouter composition root
│   ├── db/{migrations,queries,sqlc}/   # SQL source · sqlc queries · generated Go
│   ├── api/openapi.yaml    # Hand-maintained spec (served at /docs, Spectral-linted)
│   └── sqlc.yaml
├── frontend/               # Nuxt 4 SPA
│   └── app/
│       ├── components/     # Global component library (auto-imported)
│       ├── composables/    # incl. api/ — typed service layer (mock-first today)
│       ├── layouts/        # default (shell) + auth
│       ├── pages/          # incl. master/ (offices · employees · reference)
│       ├── mock/           # In-memory fixtures behind the api/ services
│       ├── middleware/ · stores/ · utils/
│       └── ../i18n/locales/{id,en}.json · ../test/ (Vitest) · ../e2e/ (Playwright)
├── docs/                   # PRD.md · DATABASE.md · ERD.md · PROGRESS.md · DESIGN_BRIEF.md · adr/ · design/ mockups
├── docker-compose.yml      # Production-like full stack (compiled images; CI e2e)
├── docker-compose.dev.yml  # Dev: infra by default · full stack + live reload via `--profile app watch`
└── .github/workflows/ci.yml # CI: backend build/vet/test · frontend lint/typecheck/test/build · Spectral · e2e

```

## Docker Compose configurations

Two self-contained compose files cover the workflows (no `-f a -f b` overlay):

| File | Purpose | Command |
|---|---|---|
| `docker-compose.yml` | Production-like full stack (compiled images) — used by CI e2e | `docker compose up --build` |
| `docker-compose.dev.yml` | Infra only (Postgres · Redis · MinIO) — run app on host | `docker compose -f docker-compose.dev.yml up -d` |
| `docker-compose.dev.yml` *(profile `app`)* | Full dev stack **with live reload** (Air + Vite HMR) via Docker Compose `watch` | `docker compose -f docker-compose.dev.yml --profile app watch` |

## Run everything in Docker (full stack)

```bash
docker compose up --build
```
Brings up PostgreSQL, Redis, MinIO, runs database migrations, then starts the API
and the frontend:
- Frontend → http://localhost:3000
- API → http://localhost:8080 (docs at `/docs`)
- MinIO console → http://localhost:9001

### Full stack with live reload (`docker compose watch`)

Bring up the dev stack and have source edits hot-reload inside the containers — the
backend rebuilds via [Air](https://github.com/air-verse/air), the frontend via Vite HMR:

```bash
docker compose -f docker-compose.dev.yml --profile app watch
```

Docker Compose **syncs** changed files into each container's own filesystem (rather than
bind-mounting), so the in-container watchers see changes via normal **inotify** — including
on Windows/WSL2, where bind-mount events aren't delivered. No polling needed.

#### How changes are handled

`develop.watch` rules per service:

- **Source edits** (`action: sync`) → synced into the container; Air rebuilds the Go binary,
  Vite hot-reloads the Nuxt app. Near-instant.
- **Dependency changes** (`action: rebuild`) → editing `go.mod`/`go.sum` or
  `package.json`/`pnpm-lock.yaml` rebuilds that service's image (so `go mod download` /
  `pnpm install` run), then restarts it. `node_modules` stays in a named volume and is never
  synced from the host.

Seed a superadmin (one-off, while the stack runs) — from the host, since the
`backend` image ships only the API binary:
```bash
cd backend
go run ./cmd/createadmin -email admin@inventra.local -password admin12345
```
It connects to the same Postgres exposed on `localhost:5433`.

Stop: `docker compose down` (add `-v` to also drop data volumes).

## Local development (run on host)

### Prerequisites
- Docker Desktop · Go 1.25+ · Node.js 24+ · pnpm 11+

### 1. Start infrastructure
```bash
docker compose -f docker-compose.dev.yml up -d
```
- PostgreSQL → `localhost:5433`
- Redis → `localhost:6379`
- MinIO → `localhost:9000` (console `localhost:9001`, `minioadmin` / `minioadmin123`)

### 2. Backend
```bash
cd backend
cp .env.example .env
go run ./cmd/api
```
API at `http://localhost:8080`. Liveness: `GET /health`. Readiness (checks PostgreSQL + Redis): `GET /health/ready`.

### 3. Frontend
```bash
cd frontend
cp .env.example .env
pnpm install
pnpm dev
```
App at `http://localhost:3000`.

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

# Frontend
cd frontend
pnpm dev | pnpm build | pnpm lint | pnpm typecheck
pnpm test                 # Vitest unit + Nuxt-runtime tests
pnpm test:e2e             # Playwright e2e (needs backend stack up + seeded admin)
```

## License
MIT
