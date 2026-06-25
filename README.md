# Inventra — Asset Management System

Web application for managing an organization's **physical assets / inventory**: catalog,
check-out/check-in, maintenance, depreciation, reporting, and approvals — with
configurable role-based authorization across a hierarchical office structure
(Pusat → Wilayah → Cabang → Outlet).

Full requirements: [docs/PRD.md](docs/PRD.md) · database design: [docs/DATABASE.md](docs/DATABASE.md) ·
live progress tracker: [docs/PROGRESS.md](docs/PROGRESS.md).

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
>   depreciation, reporting, and import — see [docs/PROGRESS.md](docs/PROGRESS.md).

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
├── docs/                   # PRD.md · DATABASE.md · PROGRESS.md · DESIGN_BRIEF.md · design/ mockups
├── docker-compose.yml      # Full stack (infra + migrate + backend + frontend)
├── docker-compose.dev.yml  # Infra only (Postgres + Redis + MinIO)
├── docker-compose.watch.yml # Live-reload overlay (Go via Air, Nuxt via Vite HMR)
└── .github/workflows/ci.yml # CI: backend build/vet/test · frontend lint/typecheck/test/build · Spectral · e2e

```

## Docker Compose configurations

Three compose files cover the different workflows — combine them with `-f` as needed:

| File | Purpose | Command |
|---|---|---|
| `docker-compose.yml` | Full stack: infra + migrate + backend + frontend | `docker compose up --build` |
| `docker-compose.dev.yml` | Infra only (Postgres · Redis · MinIO) — run app on host | `docker compose -f docker-compose.dev.yml up -d` |
| `docker-compose.watch.yml` | Live-reload overlay (Go via Air, Nuxt via Vite HMR) — layered on the base file | `docker compose -f docker-compose.yml -f docker-compose.watch.yml up --build` |

## Run everything in Docker (full stack)

```bash
docker compose up --build
```
Brings up PostgreSQL, Redis, MinIO, runs database migrations, then starts the API
and the frontend:
- Frontend → http://localhost:3000
- API → http://localhost:8080 (docs at `/docs`)
- MinIO console → http://localhost:9001

### Full stack with live reload

Run the base stack with the watch overlay to hot-reload source edits inside the
containers — the backend rebuilds via [Air](https://github.com/air-verse/air), the
frontend via Vite HMR. Source is bind-mounted, so saving a file triggers a reload:

```bash
docker compose -f docker-compose.yml -f docker-compose.watch.yml up --build
```

> On Windows/WSL2 bind mounts, filesystem events (inotify) aren't delivered, so the
> watchers fall back to **polling** — reloads may lag a second or two behind a save.

#### Adding dependencies under watch

Live reload covers **source edits**, not dependency changes — handle new libraries
explicitly:

- **Backend (Go):** after `go get <lib>`, import it in a `.go` file. Saving that file
  triggers an Air rebuild, and `go build` fetches the new module into the container's
  module cache (needs network). Editing only `go.mod`/`go.sum` won't trigger a reload —
  Air watches `.go` files only.
- **Frontend (Nuxt):** `node_modules` lives in a named volume (not the bind mount), so a
  new entry in `package.json` is **not** installed automatically. Install it inside the
  running container, or recreate the volume:

  ```bash
  # Install into the running container (fastest)
  docker compose -f docker-compose.yml -f docker-compose.watch.yml exec frontend pnpm install

  # …or rebuild from scratch (drops and re-seeds node_modules)
  docker compose -f docker-compose.yml -f docker-compose.watch.yml down
  docker volume rm inventra_frontend-node-modules
  docker compose -f docker-compose.yml -f docker-compose.watch.yml up --build
  ```

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
