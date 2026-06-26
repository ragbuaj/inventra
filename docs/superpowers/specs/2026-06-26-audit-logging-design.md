# Audit Logging — Backend Design

Date: 2026-06-26
Status: Approved (decisions confirmed with user)

## Goal

Wire a centralized **audit writer** into every mutating endpoint and expose **audit-view**
endpoints, filling the existing `audit.audit_logs` table (created in migration 000004 but never
written to). Powers the already-built **Audit Trail** frontend screen.

## Decisions (confirmed)

- **`changes` payload = full before/after diff.** Update handlers fetch the prior row and store
  `{ "field": { "before": ..., "after": ... } }` for changed fields. Create stores
  `{ field: { after } }` (the new snapshot); delete stores `{ field: { before } }` (the removed row).
- **Audit view is office-scoped now.** Add an `office_id` column to `audit_logs`; populate it with
  the **entity's** office on write; filter the list by the caller's resolved office scope.

## Architecture

Approach: a shared **`audit.Service`** called explicitly from handlers (one line after a successful
mutation) — matches the codebase's module pattern and is the "writer wired into handlers" approach.
**Best-effort:** a failed audit write is logged (`slog`) but never fails the user's request.

### 1. Migration `000014_audit_office`

```sql
-- up
ALTER TABLE audit.audit_logs ADD COLUMN office_id uuid;          -- nullable: global actions have none
CREATE INDEX idx_audit_office ON audit.audit_logs (office_id);
-- down: drop index + column
```

Cross-schema FK to `masterdata.offices` is intentionally omitted (audit is append-only and must
survive office deletion) — consistent with the append-only `audit_logs` design.

### 2. Queries `db/queries/audit.sql` → `sqlc generate`

- `InsertAuditLog(actor_id, entity_type, entity_id, action, changes, ip, office_id)`
- `ListAuditLogs` — LEFT JOIN `identity.users` for actor name/email; filters: `actor_id`,
  `entity_type`, `action`, `from`/`to` (created_at), `search` (entity_type/entity_id text);
  office scope (`all_scope bool`, `office_ids uuid[]`); `ORDER BY created_at DESC`; `limit/offset`.
- `CountAuditLogs` — same filters + scope.

Scope predicate: `(all_scope OR office_id = ANY(office_ids))`. NULL-office (global) rows are visible
only to all-scope callers (admin) — global master-data changes are an admin concern.

### 3. Module `internal/audit/`

- **`service.go`** — `Service{ q *sqlc.Queries }`; `Log(ctx, LogInput) error`, `List(ctx, ListFilter)`
  + `Count`; sentinel errors + `mapDBError`. `LogInput{ ActorID, EntityType, EntityID, Action,
  Changes any, IP, OfficeID }` — marshals `Changes` to JSON.
- **`dto.go`** — `auditToMap` response (id, actor `{id,name,email}`, entity_type, entity_id, action,
  changes, ip, office_id, created_at) + list-query binding.
- **`handler.go`** — `GET /audit` (list + filters + pagination, `{data,total,limit,offset}`),
  resolves caller scope via `authz.ScopeService` module `"audit"`; `svcError` mapping.
- **`routes.go`** — `RegisterRoutes(rg, h, authMW, requireAuditView)` where
  `requireAuditView = RequirePermission(permSvc, "audit.view")` (already seeded for
  superadmin/kanwil/unit).

### 4. Writer helper

`audit.Record(c *gin.Context, svc *Service, action, entityType string, entityID uuid.UUID, officeID *uuid.UUID, changes any)` —
pulls `actor_id` from `middleware.CtxUserID`, `ip` from `c.ClientIP()`, calls `svc.Log`, swallows
errors into `slog.Warn`. Plus a `audit.Diff(before, after any) map` helper that builds the
changed-fields map from two structs (via JSON round-trip).

### 5. Wire into existing mutations

Thread `*audit.Service` into the modules and call `Record` after each successful create/update/delete:

- **masterdata**: offices, floors, rooms, employees, categories, and the generic reference engine
  (`ref.go`, covering 11 resources). `RegisterRoutes` gains an `*audit.Service` param.
  - `office_id`: offices → the office's own id; floors/rooms/employees → their `office_id`;
    categories + reference resources are global → `nil`.
  - Updates fetch the prior row (offices already does) to build the before/after diff.
- **user**: create/update/delete (+ status change) — `office_id` = the user's placement office.

Login/logout are auth events (no entity row; `entity_id` is `NOT NULL`) — out of scope for v1.

### 6. Wiring + docs + tests

- `NewRouter`: construct `audit.NewService(queries)`, register its routes, pass it to
  `masterdata.RegisterRoutes` and the `user` handler.
- `backend/api/openapi.yaml`: add `GET /audit` (+ schema) and Spectral-lint clean.
- Go tests: audit service `Log` inserts a row with the marshalled diff; `List` honors entity_type /
  action / scope filters; `Diff` produces only changed fields.

## Gates

`go build ./...`, `go vet ./...`, `go test ./...`, `sqlc generate` (no diff), Spectral lint — all green.
Apply migration 000014 to the dev DB and rebuild the backend so the running stack matches.
