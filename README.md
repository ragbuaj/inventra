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
├── docker-compose.dev.yml  # Postgres + Redis + MinIO
└── .github/workflows/ci.yml # CI: backend + frontend build/test
```

## Getting Started

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
API at `http://localhost:8080` · health check: `GET /health`.

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

# Frontend
cd frontend
pnpm dev | pnpm build | pnpm lint | pnpm typecheck
```

## License
MIT
