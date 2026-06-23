# Inventra — Asset Management System

Web application for managing an organization's **physical assets / inventory**: catalog,
check-out/check-in, maintenance, depreciation, reporting, and approvals — with
configurable role-based authorization across a hierarchical office structure
(Pusat → Wilayah → Cabang → Outlet).

Full requirements: [docs/PRD.md](docs/PRD.md).

> Status: **foundation scaffold**. Feature modules are built in phases per the PRD roadmap (§10).

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.25 · Gin |
| Database | PostgreSQL 16 (via sqlc + golang-migrate) |
| Cache & state | Redis 7 |
| Object storage | MinIO (S3-compatible) |
| Frontend | Nuxt 4 (Vue 3 + Vite) · Nuxt UI · Pinia · i18n (id/en) |
| DevOps | Docker Compose · GitHub Actions |

## Project Structure

```
asset-management/
├── backend/
│   ├── cmd/api/            # Entry point (HTTP server, graceful shutdown)
│   ├── internal/
│   │   ├── config/         # Env configuration
│   │   └── server/         # Router + middleware
│   ├── db/
│   │   ├── migrations/     # golang-migrate SQL files
│   │   ├── queries/        # sqlc query files (per module)
│   │   └── sqlc/           # Generated Go (after `sqlc generate`)
│   └── sqlc.yaml
├── frontend/               # Nuxt 4 app (scaffolded from the official `ui` template)
├── docs/PRD.md
├── docker-compose.yml      # Full stack (infra + migrate + backend + frontend)
├── docker-compose.dev.yml  # Infra only (Postgres + Redis + MinIO)
└── .github/workflows/ci.yml # CI: backend + frontend build/test

```

## Run everything in Docker (full stack)

```bash
docker compose up --build
```
Brings up PostgreSQL, Redis, MinIO, runs database migrations, then starts the API
and the frontend:
- Frontend → http://localhost:3000
- API → http://localhost:8080 (docs at `/docs`)
- MinIO console → http://localhost:9001

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
```

## License
MIT
