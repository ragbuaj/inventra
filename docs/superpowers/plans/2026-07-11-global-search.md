# Global Search Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Real `GET /api/v1/search` backend + rewire `useGlobalSearch` off mocks, per the approved spec `docs/superpowers/specs/2026-07-11-global-search-design.md`.

**Architecture:** New `internal/search` module (ADR-0008 4-file split) runs 5 scope-gated sqlc queries in parallel (errgroup) and returns a uniform read model (`groups[]`). Authorization mirrors each entity's existing list endpoint (permission checked programmatically, data scope via `CallerOfficeScope` per module). pg_trgm GIN indexes make `ILIKE '%q%'` indexed. Frontend swaps the mock aggregation inside `useGlobalSearch` for a `$fetch` call — `CommandPalette.vue` keeps its contract, gains a 250 ms debounce.

**Tech Stack:** Go 1.25/Gin/sqlc/pgx, golang.org/x/sync/errgroup, PostgreSQL pg_trgm, Nuxt 4/Vitest/Playwright.

## Global Constraints

- Branch: `feat/global-search` (already checked out). Conventional Commits with scope; **never add AI/Claude attribution**.
- Backend gates: `go build ./...`, `go vet ./...`, `go test ./...`, `go test -tags=integration ./... -count=1 -p 1`, Spectral lint.
- Frontend gates: `pnpm lint` (no trailing commas, 1tbs), `pnpm typecheck`, `pnpm test`, `pnpm build`.
- DTO English snake_case; all user-facing strings via i18n (no new keys needed — `search.group.*` and `approval.type.*` exist).
- Don't hand-edit `backend/db/sqlc/` — edit `db/queries/*.sql` + migrations, run `sqlc generate`.
- `q` minimum **2 characters** (trimmed, rune count); **5 items max per group** + `total`; group order `assets, employees, offices, users, requests`; ungated groups skipped silently.
- Dev DB: `export DATABASE_URL="postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable"` (infra up via `docker compose -f docker-compose.dev.yml up -d`).

---

### Task 1: Migration `000028_search_trgm`

**Files:**
- Create: `backend/db/migrations/000028_search_trgm.up.sql`
- Create: `backend/db/migrations/000028_search_trgm.down.sql`

**Interfaces:**
- Produces: pg_trgm extension + GIN trigram indexes used implicitly by Task 2's ILIKE queries. No Go surface.

- [ ] **Step 1: Write the up migration**

```sql
-- 000028_search_trgm.up.sql
-- Trigram indexes so ILIKE '%q%' (global search + existing list search) uses an index.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS assets_name_trgm_idx      ON asset.assets          USING gin (name gin_trgm_ops)          WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS assets_tag_trgm_idx       ON asset.assets          USING gin (asset_tag gin_trgm_ops)     WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS assets_serial_trgm_idx    ON asset.assets          USING gin (serial_number gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS employees_name_trgm_idx   ON masterdata.employees  USING gin (name gin_trgm_ops)          WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS employees_code_trgm_idx   ON masterdata.employees  USING gin (code gin_trgm_ops)          WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS offices_name_trgm_idx     ON masterdata.offices    USING gin (name gin_trgm_ops)          WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS offices_code_trgm_idx     ON masterdata.offices    USING gin (code gin_trgm_ops)          WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS users_name_trgm_idx       ON identity.users        USING gin (name gin_trgm_ops)          WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS requests_reason_trgm_idx  ON approval.requests     USING gin (reason gin_trgm_ops)        WHERE deleted_at IS NULL;
```

Note: `identity.users.email` is `citext` — a trigram index would need an expression index (`(email::text)`) that the existing `email ILIKE` predicate wouldn't use. The users table is tiny and Superadmin-gated; deliberately not indexed (recorded in spec).

- [ ] **Step 2: Write the down migration**

```sql
-- 000028_search_trgm.down.sql
-- Extension is left installed (cheap, may be used by other objects).
DROP INDEX IF EXISTS asset.assets_name_trgm_idx;
DROP INDEX IF EXISTS asset.assets_tag_trgm_idx;
DROP INDEX IF EXISTS asset.assets_serial_trgm_idx;
DROP INDEX IF EXISTS masterdata.employees_name_trgm_idx;
DROP INDEX IF EXISTS masterdata.employees_code_trgm_idx;
DROP INDEX IF EXISTS masterdata.offices_name_trgm_idx;
DROP INDEX IF EXISTS masterdata.offices_code_trgm_idx;
DROP INDEX IF EXISTS identity.users_name_trgm_idx;
DROP INDEX IF EXISTS approval.requests_reason_trgm_idx;
```

- [ ] **Step 3: Verify up → down → up against the dev DB**

Run (from `backend/`, bash):
```bash
export DATABASE_URL="postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable"
migrate -path db/migrations -database "$DATABASE_URL" up
migrate -path db/migrations -database "$DATABASE_URL" down 1
migrate -path db/migrations -database "$DATABASE_URL" up
```
Expected: each command exits 0; final version = 28, not dirty.

- [ ] **Step 4: Commit**

```bash
git add backend/db/migrations/000028_search_trgm.up.sql backend/db/migrations/000028_search_trgm.down.sql
git commit -m "feat(db): pg_trgm extension + trigram indexes for global search"
```

---

### Task 2: sqlc queries `db/queries/search.sql`

**Files:**
- Create: `backend/db/queries/search.sql`
- Generated (do not hand-edit): `backend/db/sqlc/search.sql.go` via `sqlc generate`

**Interfaces:**
- Consumes: Task 1's schema (columns already existed; indexes only).
- Produces (used by Task 3): `sqlc.SearchAssets(ctx, SearchAssetsParams{Q, AllScope, OfficeIds, Lim}) ([]SearchAssetsRow)`, and analogous `SearchEmployees`, `SearchOffices`, `SearchUsers(ctx, SearchUsersParams{Q, Lim})`, `SearchRequests`. Every row carries `Total int64` (window count).

- [ ] **Step 1: Write the queries**

```sql
-- Global search (command palette). Each query returns the top matches for one
-- entity plus the full match count via a window function. Callers gate by
-- permission + data scope; queries only enforce the office scope filter.

-- name: SearchAssets :many
SELECT id, name, asset_tag, status, count(*) OVER()::bigint AS total
FROM asset.assets
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (name ILIKE '%' || sqlc.arg(q) || '%'
       OR asset_tag ILIKE '%' || sqlc.arg(q) || '%'
       OR serial_number ILIKE '%' || sqlc.arg(q) || '%')
ORDER BY name
LIMIT sqlc.arg(lim);

-- name: SearchEmployees :many
SELECT id, name, code, count(*) OVER()::bigint AS total
FROM masterdata.employees
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (name ILIKE '%' || sqlc.arg(q) || '%' OR code ILIKE '%' || sqlc.arg(q) || '%')
ORDER BY name
LIMIT sqlc.arg(lim);

-- name: SearchOffices :many
SELECT id, name, code, count(*) OVER()::bigint AS total
FROM masterdata.offices
WHERE deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (name ILIKE '%' || sqlc.arg(q) || '%' OR code ILIKE '%' || sqlc.arg(q) || '%')
ORDER BY name
LIMIT sqlc.arg(lim);

-- name: SearchUsers :many
SELECT id, name, email, count(*) OVER()::bigint AS total
FROM identity.users
WHERE deleted_at IS NULL
  AND (name ILIKE '%' || sqlc.arg(q) || '%' OR email ILIKE '%' || sqlc.arg(q) || '%')
ORDER BY name
LIMIT sqlc.arg(lim);

-- name: SearchRequests :many
SELECT r.id, r.type, r.status, o.name AS office_name, count(*) OVER()::bigint AS total
FROM approval.requests r
LEFT JOIN masterdata.offices o ON o.id = r.office_id AND o.deleted_at IS NULL
WHERE r.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR r.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (r.reason ILIKE '%' || sqlc.arg(q) || '%' OR r.id::text ILIKE sqlc.arg(q) || '%')
ORDER BY r.created_at DESC
LIMIT sqlc.arg(lim);
```

Scope semantics note: `SearchOffices` filters on the office's **own id** (`id = ANY(...)`) — an office row *is* the scoped object, mirroring how the offices module scopes reads. The other queries filter on their `office_id` column.

- [ ] **Step 2: Generate and build**

Run (from `backend/`): `sqlc generate && go build ./...`
Expected: exit 0; `db/sqlc/search.sql.go` appears with the 5 functions and row structs listed under Produces.

- [ ] **Step 3: Commit**

```bash
git add backend/db/queries/search.sql backend/db/sqlc/
git commit -m "feat(search): scope-aware sqlc queries for global search"
```

---

### Task 3: Backend module `internal/search` + router wiring

**Files:**
- Create: `backend/internal/search/dto.go`
- Create: `backend/internal/search/service.go`
- Create: `backend/internal/search/handler.go`
- Create: `backend/internal/search/routes.go`
- Create: `backend/internal/search/service_test.go` (unit — no DB)
- Modify: `backend/internal/server/router.go` (wire after the audit module block)
- Modify: `backend/go.mod` (`golang.org/x/sync` moves indirect→direct via `go mod tidy`)

**Interfaces:**
- Consumes: Task 2's sqlc functions; `authz.PermissionService.Has(ctx, roleID uuid.UUID, key string) (bool, error)`; `common.ScopedDeps.CallerOfficeScope(c *gin.Context, module string) (bool, []uuid.UUID, error)`; `middleware.CtxRoleID`.
- Produces: `search.NewService(q *sqlc.Queries) *Service`; `(*Service).Search(ctx, Input) ([]Group, error)`; `search.TooShort(q string) bool`; `search.NewHandler(svc *Service, perms *authz.PermissionService, scoped common.ScopedDeps) *Handler`; `search.RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW gin.HandlerFunc)`; HTTP `GET /api/v1/search?q=` → `200 {"groups":[{"type","total","items":[{"id","title","subtitle","status","asset_tag?","request_type?"}]}]}`.

- [ ] **Step 1: Write failing unit tests** (`service_test.go`, package `search` — internal so it can test unexported mappers)

```go
package search

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/ragbuaj/inventra/db/sqlc"
)

func TestTooShort(t *testing.T) {
	assert.True(t, TooShort(""))
	assert.True(t, TooShort("  a  "))
	assert.False(t, TooShort("ab"))
	assert.False(t, TooShort(" ab "))
}

func TestAssetItem(t *testing.T) {
	id := uuid.New()
	it := assetItem(sqlc.SearchAssetsRow{ID: id, Name: "Laptop", AssetTag: "JKT01-X", Status: sqlc.SharedAssetStatusAvailable})
	assert.Equal(t, id.String(), it.ID)
	assert.Equal(t, "Laptop", it.Title)
	assert.Equal(t, "JKT01-X", it.Subtitle)
	assert.Equal(t, "available", *it.Status)
	assert.Equal(t, "JKT01-X", *it.AssetTag)
}

func TestRequestItem(t *testing.T) {
	id := uuid.New()
	off := "Cabang Jakarta"
	it := requestItem(sqlc.SearchRequestsRow{ID: id, Type: sqlc.SharedRequestTypeAssetCreate, Status: sqlc.SharedRequestStatusPending, OfficeName: &off})
	assert.Equal(t, "Cabang Jakarta", it.Title)
	assert.Equal(t, id.String()[:8], it.Subtitle)
	assert.Equal(t, "pending", *it.Status)
	assert.Equal(t, "asset_create", *it.RequestType)
}
```

Adapt field types to what sqlc actually generated in Task 2 (e.g. `OfficeName` may be `*string` or `pgtype.Text` — match it; if `pgtype.Text`, construct with `pgtype.Text{String: "Cabang Jakarta", Valid: true}`).

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/search/ -run 'TestTooShort|TestAssetItem|TestRequestItem' -v`
Expected: FAIL (package does not compile — functions undefined).

- [ ] **Step 3: Implement `dto.go`**

```go
package search

import (
	"github.com/ragbuaj/inventra/db/sqlc"
)

// Item is the uniform read-model row for one search hit.
type Item struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Subtitle    string  `json:"subtitle"`
	Status      *string `json:"status"`
	AssetTag    *string `json:"asset_tag,omitempty"`
	RequestType *string `json:"request_type,omitempty"`
}

// Group is one entity bucket in the response, capped at PerGroupLimit items.
type Group struct {
	Type  string `json:"type"`
	Total int64  `json:"total"`
	Items []Item `json:"items"`
}

func strPtr(s string) *string { return &s }

func assetItem(r sqlc.SearchAssetsRow) Item {
	return Item{
		ID:       r.ID.String(),
		Title:    r.Name,
		Subtitle: r.AssetTag,
		Status:   strPtr(string(r.Status)),
		AssetTag: strPtr(r.AssetTag),
	}
}

func employeeItem(r sqlc.SearchEmployeesRow) Item {
	return Item{ID: r.ID.String(), Title: r.Name, Subtitle: r.Code}
}

func officeItem(r sqlc.SearchOfficesRow) Item {
	return Item{ID: r.ID.String(), Title: r.Name, Subtitle: r.Code}
}

func userItem(r sqlc.SearchUsersRow) Item {
	return Item{ID: r.ID.String(), Title: r.Name, Subtitle: r.Email}
}

// requestItem: requests have no title column — Title carries the office name
// (may be empty); the frontend composes "type · office" via i18n.
func requestItem(r sqlc.SearchRequestsRow) Item {
	title := ""
	if r.OfficeName != nil {
		title = *r.OfficeName
	}
	return Item{
		ID:          r.ID.String(),
		Title:       title,
		Subtitle:    r.ID.String()[:8],
		Status:      strPtr(string(r.Status)),
		RequestType: strPtr(string(r.Type)),
	}
}
```

(Adapt `r.Email` / `r.OfficeName` accessors to the generated types — `citext` usually maps to `string`; a nullable join column may be `*string` or `pgtype.Text`.)

- [ ] **Step 4: Implement `service.go`**

```go
package search

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/ragbuaj/inventra/db/sqlc"
)

// PerGroupLimit caps items per entity group (command-palette rows).
const PerGroupLimit = 5

// MinQueryLen is the minimum trimmed rune count for a search to run.
const MinQueryLen = 2

// Gate carries one entity's resolved authorization for this caller.
type Gate struct {
	Enabled   bool
	AllScope  bool
	OfficeIDs []uuid.UUID
}

// Input is the caller-resolved search request. The handler decides gates;
// the service only orchestrates queries.
type Input struct {
	Q         string
	Assets    Gate
	Employees Gate
	Offices   Gate
	Requests  Gate
	Users     bool
}

type Service struct {
	q *sqlc.Queries
}

func NewService(q *sqlc.Queries) *Service { return &Service{q: q} }

// TooShort reports whether the trimmed query is below MinQueryLen runes.
func TooShort(q string) bool {
	return utf8.RuneCountInString(strings.TrimSpace(q)) < MinQueryLen
}

// Search runs the gated entity queries concurrently and returns non-empty
// groups in the fixed order assets, employees, offices, users, requests.
func (s *Service) Search(ctx context.Context, in Input) ([]Group, error) {
	q := strings.TrimSpace(in.Q)
	slots := make([]*Group, 5)
	eg, ctx := errgroup.WithContext(ctx)

	if in.Assets.Enabled {
		eg.Go(func() error {
			rows, err := s.q.SearchAssets(ctx, sqlc.SearchAssetsParams{
				Q: q, AllScope: in.Assets.AllScope, OfficeIds: in.Assets.OfficeIDs, Lim: PerGroupLimit,
			})
			if err != nil {
				return err
			}
			items := make([]Item, 0, len(rows))
			var total int64
			for _, r := range rows {
				total = r.Total
				items = append(items, assetItem(r))
			}
			if len(items) > 0 {
				slots[0] = &Group{Type: "assets", Total: total, Items: items}
			}
			return nil
		})
	}
	if in.Employees.Enabled {
		eg.Go(func() error {
			rows, err := s.q.SearchEmployees(ctx, sqlc.SearchEmployeesParams{
				Q: q, AllScope: in.Employees.AllScope, OfficeIds: in.Employees.OfficeIDs, Lim: PerGroupLimit,
			})
			if err != nil {
				return err
			}
			items := make([]Item, 0, len(rows))
			var total int64
			for _, r := range rows {
				total = r.Total
				items = append(items, employeeItem(r))
			}
			if len(items) > 0 {
				slots[1] = &Group{Type: "employees", Total: total, Items: items}
			}
			return nil
		})
	}
	if in.Offices.Enabled {
		eg.Go(func() error {
			rows, err := s.q.SearchOffices(ctx, sqlc.SearchOfficesParams{
				Q: q, AllScope: in.Offices.AllScope, OfficeIds: in.Offices.OfficeIDs, Lim: PerGroupLimit,
			})
			if err != nil {
				return err
			}
			items := make([]Item, 0, len(rows))
			var total int64
			for _, r := range rows {
				total = r.Total
				items = append(items, officeItem(r))
			}
			if len(items) > 0 {
				slots[2] = &Group{Type: "offices", Total: total, Items: items}
			}
			return nil
		})
	}
	if in.Users {
		eg.Go(func() error {
			rows, err := s.q.SearchUsers(ctx, sqlc.SearchUsersParams{Q: q, Lim: PerGroupLimit})
			if err != nil {
				return err
			}
			items := make([]Item, 0, len(rows))
			var total int64
			for _, r := range rows {
				total = r.Total
				items = append(items, userItem(r))
			}
			if len(items) > 0 {
				slots[3] = &Group{Type: "users", Total: total, Items: items}
			}
			return nil
		})
	}
	if in.Requests.Enabled {
		eg.Go(func() error {
			rows, err := s.q.SearchRequests(ctx, sqlc.SearchRequestsParams{
				Q: q, AllScope: in.Requests.AllScope, OfficeIds: in.Requests.OfficeIDs, Lim: PerGroupLimit,
			})
			if err != nil {
				return err
			}
			items := make([]Item, 0, len(rows))
			var total int64
			for _, r := range rows {
				total = r.Total
				items = append(items, requestItem(r))
			}
			if len(items) > 0 {
				slots[4] = &Group{Type: "requests", Total: total, Items: items}
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	out := make([]Group, 0, 5)
	for _, g := range slots {
		if g != nil {
			out = append(out, *g)
		}
	}
	return out, nil
}
```

Each goroutine writes only its own slot index — no mutex needed. Adapt `Lim` param type to the generated signature (likely `int32`).

- [ ] **Step 5: Implement `handler.go`**

```go
package search

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

type Handler struct {
	svc    *Service
	perms  *authz.PermissionService
	scoped common.ScopedDeps
}

func NewHandler(svc *Service, perms *authz.PermissionService, scoped common.ScopedDeps) *Handler {
	return &Handler{svc: svc, perms: perms, scoped: scoped}
}

// gateScoped resolves one entity gate: optional permission key + scope module.
// A missing permission disables the group silently (never a 403).
func (h *Handler) gateScoped(c *gin.Context, roleID uuid.UUID, permKey, module string) (Gate, error) {
	if permKey != "" {
		ok, err := h.perms.Has(c.Request.Context(), roleID, permKey)
		if err != nil || !ok {
			return Gate{}, err
		}
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, module)
	if err != nil {
		return Gate{}, err
	}
	return Gate{Enabled: true, AllScope: all, OfficeIDs: ids}, nil
}

func (h *Handler) search(c *gin.Context) {
	q := c.Query("q")
	if TooShort(q) {
		c.JSON(http.StatusOK, gin.H{"groups": []Group{}})
		return
	}
	roleID, err := uuid.Parse(c.GetString(middleware.CtxRoleID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	in := Input{Q: q}
	if in.Assets, err = h.gateScoped(c, roleID, "asset.view", "assets"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	if in.Employees, err = h.gateScoped(c, roleID, "", "employees"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	if in.Offices, err = h.gateScoped(c, roleID, "", "offices"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	if in.Requests, err = h.gateScoped(c, roleID, "", "requests"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	if in.Users, err = h.perms.Has(c.Request.Context(), roleID, "user.manage"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve permissions"})
		return
	}

	groups, err := h.svc.Search(c.Request.Context(), in)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"groups": groups})
}
```

- [ ] **Step 6: Implement `routes.go`**

```go
package search

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the global search endpoint. Auth only — per-entity
// permission/scope gating happens inside the handler (groups are skipped,
// never 403'd).
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW gin.HandlerFunc) {
	rg.GET("/search", authMW, h.search)
}
```

- [ ] **Step 7: Wire into `NewRouter`** (`backend/internal/server/router.go`, after the audit module block ~line 172, inside the same scope where `queries`, `permSvc`, `scopeSvc`, `requireAuth` live)

```go
searchSvc := search.NewService(queries)
searchHandler := search.NewHandler(searchSvc, permSvc, common.ScopedDeps{Q: queries, Scope: scopeSvc})
search.RegisterRoutes(api, searchHandler, requireAuth)
```

Add `"github.com/ragbuaj/inventra/internal/search"` to the imports.

- [ ] **Step 8: Tidy, build, vet, unit test**

Run (from `backend/`): `go mod tidy && go build ./... && go vet ./... && go test ./internal/search/ -v`
Expected: all PASS; `go.mod` now lists `golang.org/x/sync` as a direct dependency.

- [ ] **Step 9: Run the whole non-integration suite**

Run: `go test ./...`
Expected: PASS (no regressions).

- [ ] **Step 10: Commit**

```bash
git add backend/internal/search/ backend/internal/server/router.go backend/go.mod backend/go.sum
git commit -m "feat(search): internal/search module — gated per-entity global search endpoint"
```

---

### Task 4: Backend integration tests

**Files:**
- Create: `backend/internal/search/search_integration_test.go` (build tag `integration`, package `search_test`)

**Interfaces:**
- Consumes: `search.NewService/NewHandler/RegisterRoutes` (Task 3); `testsupport.NewPostgres(t)`, `testsupport.NewRedis(t)`, `testsupport.SeedRole`, `testsupport.SeedScopePolicy`, `testsupport.SeedEmployee`; seeding helpers modeled on `backend/internal/assignment/assignment_integration_test.go` (`seedOfficeWithType`, `seedCategory`, `seedAsset`, `seedUser`, permission-grant inserts).

- [ ] **Step 1: Write the harness + scenarios.** Mirror the harness in `internal/assignment/assignment_integration_test.go`: `pool := testsupport.NewPostgres(t)`, `rdb := testsupport.NewRedis(t)`, `q := sqlc.New(pool)`, build `search.NewHandler(search.NewService(q), authz.NewPermissionService(q, rdb), common.ScopedDeps{Q: q, Scope: authz.NewScopeService(q, rdb)})`, mount on a `gin.New()` router with a stub auth middleware that sets `middleware.CtxUserID`/`middleware.CtxRoleID` from the test's current user (copy the stub-auth pattern used by that file). Seed: two offices A and B (`seedOfficeWithType`), a `superadmin`-like role (global scope, permissions `asset.view` + `user.manage`) and a `staf`-like role (scope `own`, permission `asset.view` only) via `testsupport.SeedRole` + `testsupport.SeedScopePolicy(t, pool, roleID, "*", level)` + direct `INSERT INTO identity.role_permissions`; one asset per office (`seedAsset` with distinctive names like `"Laptop Alpha SRCH"`), one employee in office A, one pending approval request in office A with `reason = 'beli laptop SRCH'`.

Scenarios (each a `t.Run` doing `GET /search?q=...` and decoding `{"groups":[...]}`):

```
(a) admin_sees_all_groups        — q="SRCH": groups contain types assets, employees(?), requests; q matching the user's own name returns users group; assert group order follows assets,employees,offices,users,requests
(b) staf_scoped_and_no_users     — as staf in office A, q="SRCH": assets items only from office A (item subtitle = office-A asset tag; office-B asset absent); no "users" group present
(c) subtree_scope_filters        — role with office_subtree on office A (SeedScopePolicy module "assets"): sees A's asset, not B's
(d) requests_match_reason_and_id — q="beli laptop" returns requests group; q=first-8-chars-of-request-id also returns it (prefix match)
(e) short_query_empty            — q="a": 200 with groups == []
(f) limit_and_total              — seed 7 assets in office A named "Bulk SRCH n": q="Bulk SRCH" returns exactly 5 items with total == 7
```

Write assertions against the decoded JSON (`map[string]any` or typed structs); e.g. for (f): `assert.Len(t, items, 5)` and `assert.EqualValues(t, 7, group["total"])`.

- [ ] **Step 2: Run**

Run (from `backend/`): `go test -tags=integration ./internal/search/ -count=1 -v`
Expected: all 6 scenarios PASS (Docker must be running for testcontainers).

- [ ] **Step 3: Full integration gate** (per repo memory: run ALL packages after shared-surface changes)

Run: `go test -tags=integration ./... -count=1 -p 1`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/search/search_integration_test.go
git commit -m "test(search): integration coverage for scope, permission gating, limits"
```

---

### Task 5: OpenAPI spec

**Files:**
- Modify: `backend/api/openapi.yaml` — add tag `Search` to the top-level `tags:` block (~line 22, after `Maintenance`), add path `/api/v1/search`, add schemas `SearchGroup`/`SearchItem`.

**Interfaces:**
- Consumes: the HTTP contract from Task 3.

- [ ] **Step 1: Add the tag, path, and schemas**

Tag entry:
```yaml
  - name: Search
    description: Global search (command palette) across assets, employees, offices, users, and approval requests.
```

Path (follow the file's existing style — bearer security, `q` required):
```yaml
  /api/v1/search:
    get:
      tags: [Search]
      summary: Global search across entities
      description: >
        Returns up to 5 matches per entity group plus the full match count.
        Groups the caller is not authorized for (permission or data scope)
        are omitted silently. Queries shorter than 2 characters return an
        empty groups array. Group order is fixed: assets, employees, offices,
        users, requests.
      parameters:
        - name: q
          in: query
          required: true
          schema: { type: string, minLength: 2 }
          description: Search text (min 2 characters after trimming).
      responses:
        '200':
          description: Grouped search results
          content:
            application/json:
              schema:
                type: object
                properties:
                  groups:
                    type: array
                    items: { $ref: '#/components/schemas/SearchGroup' }
        '401': { $ref: '#/components/responses/Unauthorized' }
```

Schemas (match existing component style; if `#/components/responses/Unauthorized` doesn't exist, inline the 401 the way sibling paths do):
```yaml
    SearchItem:
      type: object
      properties:
        id: { type: string, format: uuid }
        title: { type: string, description: 'Display title. For requests: the office name (frontend composes "type · office").' }
        subtitle: { type: string, description: 'Secondary line: asset tag / employee code / office code / email / short request id.' }
        status: { type: string, nullable: true, description: 'Entity status enum (assets, requests) or null.' }
        asset_tag: { type: string, description: 'Present on assets items only.' }
        request_type: { type: string, description: 'Present on requests items only (shared.request_type enum).' }
    SearchGroup:
      type: object
      properties:
        type: { type: string, enum: [assets, employees, offices, users, requests] }
        total: { type: integer, description: 'Full match count for the group (items are capped at 5).' }
        items:
          type: array
          items: { $ref: '#/components/schemas/SearchItem' }
```

- [ ] **Step 2: Lint**

Run (repo root): `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: 0 errors (the pre-existing `AssetCreatePayload` unused-component warning may persist — unrelated).

- [ ] **Step 3: Commit**

```bash
git add backend/api/openapi.yaml
git commit -m "docs(api): document GET /search (global search) in OpenAPI"
```

---

### Task 6: Frontend — rewrite `useGlobalSearch` to the real endpoint

**Files:**
- Modify: `frontend/app/composables/api/useGlobalSearch.ts` (full rewrite)
- Modify: `frontend/test/nuxt/useGlobalSearch.spec.ts` (full rewrite)

**Interfaces:**
- Consumes: `useApiClient().request<T>(path)` (existing); `GET /search` contract (Task 3); `TYPE_META` from `~/constants/approvalMeta` (labelKey per request type — verify its exact export shape before use); existing types `SearchGroup`/`SearchItem`/`SearchEntityType` in `~/types` (unchanged).
- Produces: `useGlobalSearch().search(query: string): Promise<SearchGroup[]>` — same signature as today, so `CommandPalette.vue` keeps working.

- [ ] **Step 1: Rewrite the failing spec first** (`useGlobalSearch.spec.ts`, `// @vitest-environment nuxt`). Stub `useApiClient` with `mockNuxtImport` (same pattern as sibling composable specs, e.g. `use-offices`):

```ts
// @vitest-environment nuxt
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mockNuxtImport } from '@nuxt/test-utils/runtime'

const requestMock = vi.fn()
mockNuxtImport('useApiClient', () => () => ({ request: requestMock }))

describe('useGlobalSearch (real API)', () => {
  beforeEach(() => { requestMock.mockReset() })

  it('does not call the API for queries under 2 chars', async () => {
    const { search } = useGlobalSearch()
    expect(await search('')).toEqual([])
    expect(await search(' a ')).toEqual([])
    expect(requestMock).not.toHaveBeenCalled()
  })

  it('maps asset groups to SearchGroup with route/icon/labelKey', async () => {
    requestMock.mockResolvedValue({ groups: [{ type: 'assets', total: 7, items: [
      { id: '1', title: 'Laptop Dell', subtitle: 'JKT01-X', status: 'available', asset_tag: 'JKT01-X' }
    ] }] })
    const { search } = useGlobalSearch()
    const groups = await search('laptop')
    expect(requestMock).toHaveBeenCalledWith('/search?q=laptop')
    expect(groups).toHaveLength(1)
    expect(groups[0]).toMatchObject({ type: 'aset', labelKey: 'search.group.aset', total: 7 })
    expect(groups[0]!.items[0]).toMatchObject({
      type: 'aset', title: 'Laptop Dell', sub: 'JKT01-X',
      status: 'available', icon: 'i-lucide-package', to: '/assets/JKT01-X'
    })
  })

  it('composes the requests title from type + office via i18n', async () => {
    requestMock.mockResolvedValue({ groups: [{ type: 'requests', total: 1, items: [
      { id: 'abc12345-0000', title: 'Cabang Jakarta', subtitle: 'abc12345', status: 'pending', request_type: 'asset_create' }
    ] }] })
    const { search } = useGlobalSearch()
    const groups = await search('beli')
    expect(groups[0]!.type).toBe('pengajuan')
    expect(groups[0]!.items[0]!.title).toContain('Cabang Jakarta')
    expect(groups[0]!.items[0]!.title).not.toContain('approval.type')
    expect(groups[0]!.items[0]!.to).toBe('/approval')
  })

  it('maps employees/offices/users to list routes with null status', async () => {
    requestMock.mockResolvedValue({ groups: [
      { type: 'employees', total: 1, items: [{ id: 'e1', title: 'Budi', subtitle: 'EMP1', status: null }] },
      { type: 'offices', total: 1, items: [{ id: 'o1', title: 'KC Jakarta', subtitle: 'JKT01', status: null }] },
      { type: 'users', total: 1, items: [{ id: 'u1', title: 'Admin', subtitle: 'admin@x.id', status: null }] }
    ] })
    const { search } = useGlobalSearch()
    const groups = await search('ja')
    expect(groups.map(g => g.type)).toEqual(['pegawai', 'kantor', 'user'])
    expect(groups.map(g => g.items[0]!.to)).toEqual(['/master/employees', '/master/offices', '/settings/users'])
  })

  it('returns [] when the API returns no groups', async () => {
    requestMock.mockResolvedValue({ groups: [] })
    const { search } = useGlobalSearch()
    expect(await search('zzz')).toEqual([])
  })

  it('encodes the query', async () => {
    requestMock.mockResolvedValue({ groups: [] })
    const { search } = useGlobalSearch()
    await search('a b&c')
    expect(requestMock).toHaveBeenCalledWith(`/search?q=${encodeURIComponent('a b&c')}`)
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run (from `frontend/`): `pnpm vitest run test/nuxt/useGlobalSearch.spec.ts`
Expected: FAIL (composable still mock-backed; API never called, shapes differ).

- [ ] **Step 3: Rewrite the composable**

```ts
import type { SearchGroup, SearchItem, SearchEntityType } from '~/types'
import { TYPE_META } from '~/constants/approvalMeta'

type ApiGroupType = 'assets' | 'employees' | 'offices' | 'users' | 'requests'

interface ApiSearchItem {
  id: string
  title: string
  subtitle: string
  status: string | null
  asset_tag?: string
  request_type?: string
}

interface ApiSearchGroup {
  type: ApiGroupType
  total: number
  items: ApiSearchItem[]
}

const TYPE_MAP: Record<ApiGroupType, SearchEntityType> = {
  assets: 'aset',
  employees: 'pegawai',
  offices: 'kantor',
  users: 'user',
  requests: 'pengajuan'
}

const ICON: Record<SearchEntityType, string> = {
  aset: 'i-lucide-package',
  pegawai: 'i-lucide-user',
  kantor: 'i-lucide-building',
  user: 'i-lucide-shield',
  pengajuan: 'i-lucide-check-square'
}

const ROUTE: Record<SearchEntityType, string> = {
  aset: '/assets',
  pegawai: '/master/employees',
  kantor: '/master/offices',
  user: '/settings/users',
  pengajuan: '/approval'
}

export function useGlobalSearch() {
  const { request } = useApiClient()
  const { t } = useI18n()

  function itemTitle(type: SearchEntityType, it: ApiSearchItem): string {
    if (type !== 'pengajuan') return it.title
    const meta = it.request_type ? TYPE_META[it.request_type as keyof typeof TYPE_META] : undefined
    const label = meta ? t(meta.labelKey) : (it.request_type ?? '')
    return it.title ? `${label} · ${it.title}` : label
  }

  function itemTo(type: SearchEntityType, it: ApiSearchItem): string {
    if (type === 'aset' && it.asset_tag) return `/assets/${it.asset_tag}`
    return ROUTE[type]
  }

  async function search(query: string): Promise<SearchGroup[]> {
    const q = query.trim()
    if (q.length < 2) return []
    const res = await request<{ groups: ApiSearchGroup[] }>(`/search?q=${encodeURIComponent(q)}`)
    return (res.groups ?? []).map((g) => {
      const type = TYPE_MAP[g.type]
      return {
        type,
        labelKey: `search.group.${type}`,
        total: g.total,
        items: g.items.map<SearchItem>(it => ({
          type,
          title: itemTitle(type, it),
          sub: it.subtitle,
          status: it.status,
          icon: ICON[type],
          to: itemTo(type, it)
        }))
      }
    })
  }

  return { search }
}
```

Verify `TYPE_META`'s actual shape in `~/constants/approvalMeta` first (it exists per the approval screen work) and adjust the labelKey access accordingly.

- [ ] **Step 4: Run the spec**

Run: `pnpm vitest run test/nuxt/useGlobalSearch.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/composables/api/useGlobalSearch.ts frontend/test/nuxt/useGlobalSearch.spec.ts
git commit -m "feat(search): wire useGlobalSearch to real GET /search"
```

---

### Task 7: CommandPalette — debounce + request-status badge kind

**Files:**
- Modify: `frontend/app/components/CommandPalette.vue` (watcher + StatusBadge kind)
- Modify: `frontend/test/nuxt/CommandPalette.spec.ts` (stub the API, fake timers)

**Interfaces:**
- Consumes: `useGlobalSearch().search` (Task 6); `StatusBadge` prop `kind?: 'asset' | 'approval'` (already exists).

- [ ] **Step 1: Update the spec first.** In `CommandPalette.spec.ts`, stub the data source with `mockNuxtImport('useGlobalSearch', ...)` returning a `vi.fn()`-backed `search`, and wrap query-typing interactions with `vi.useFakeTimers()` + `await vi.advanceTimersByTimeAsync(250)` before asserting results. Add two cases:

```ts
it('debounces: rapid typing triggers one search call', async () => {
  // type "lap", then "lapt" within 250ms → advance timers → searchMock called once with "lapt"
})

it('renders request status badges with kind=approval', async () => {
  // searchMock returns a pengajuan group with status "pending";
  // assert the badge label resolves via approvalStatusMeta (e.g. "Menunggu"), not raw "pending"
})
```

Keep all existing cases (closed render, quick actions, empty state, Esc, permission-gated action, recent fill) — they now assert against the stubbed search.

- [ ] **Step 2: Run to verify the new cases fail**

Run: `pnpm vitest run test/nuxt/CommandPalette.spec.ts`
Expected: new cases FAIL (no debounce; badge uses default asset kind).

- [ ] **Step 3: Implement in `CommandPalette.vue`.** Replace the query watcher and clean up on unmount:

```ts
let debounceTimer: ReturnType<typeof setTimeout> | undefined

watch(query, (q) => {
  sel.value = 0
  if (debounceTimer) clearTimeout(debounceTimer)
  if (!q.trim()) {
    groups.value = []
    loading.value = false
    return
  }
  loading.value = true
  debounceTimer = setTimeout(async () => {
    const mine = ++seq
    const res = await search(q)
    if (mine === seq) {
      groups.value = res
      loading.value = false
    }
  }, 250)
})

onUnmounted(() => {
  if (debounceTimer) clearTimeout(debounceTimer)
  window.removeEventListener('keydown', onGlobalKey)
})
```

(Fold the existing `onUnmounted` listener removal into this one — don't register two.) In the template, pass the badge kind:

```html
<StatusBadge
  v-if="it.status"
  :status="it.status"
  :kind="g.type === 'pengajuan' ? 'approval' : 'asset'"
/>
```

- [ ] **Step 4: Run the spec**

Run: `pnpm vitest run test/nuxt/CommandPalette.spec.ts`
Expected: PASS (all old + 2 new cases).

- [ ] **Step 5: Commit**

```bash
git add frontend/app/components/CommandPalette.vue frontend/test/nuxt/CommandPalette.spec.ts
git commit -m "feat(search): debounce palette queries; approval-kind status badges"
```

---

### Task 8: Mock cleanup + full frontend gate

**Files:**
- Delete: `frontend/app/mock/offices.ts`, `frontend/app/mock/employees.ts`, `frontend/app/mock/users.ts`, `frontend/app/mock/approval.ts`
- Delete: `frontend/test/unit/approval-mock.spec.ts` (tests the deleted mock store)
- Keep (verified consumers elsewhere): `mock/helpers.ts` (useDashboard/useReports/useAccount), `mock/assets.ts` (pages/assets/import.vue), `mock/dashboard.ts`, `mock/reports.ts`, `mock/notifications.ts`

- [ ] **Step 1: Verify nothing else imports the four mocks**

Run (from `frontend/`): `grep -rn "mock/offices\|mock/employees\|mock/users\|mock/approval" app/ test/ e2e/`
Expected: zero matches outside the files being deleted. If a match appears, stop and resolve it before deleting.

- [ ] **Step 2: Delete the files**

```bash
git rm frontend/app/mock/offices.ts frontend/app/mock/employees.ts frontend/app/mock/users.ts frontend/app/mock/approval.ts frontend/test/unit/approval-mock.spec.ts
```

- [ ] **Step 3: Full frontend gate** (per repo memory: check the FULL suite exit code — a rewired composable can break other consumers' specs)

Run: `pnpm lint && pnpm typecheck && pnpm test && pnpm build`
Expected: all exit 0. If `mock-store.spec.ts`/`mock-helpers.spec.ts` reference deleted stores, trim only those references (they should only cover `mock/helpers.ts` infra).

- [ ] **Step 4: Commit**

```bash
git commit -m "refactor(search): drop orphaned mock stores (offices, employees, users, approval)"
```

---

### Task 9: Real-backend e2e

**Files:**
- Create: `frontend/e2e/global-search.spec.ts`

**Interfaces:**
- Consumes: the running dev stack + seeded admin (`admin@inventra.local` / `admin12345`); helper patterns from `frontend/e2e/assets.spec.ts` (API login → bearer token → API setup) and the e2e conventions: unique name+code per run, `RATELIMIT_ENABLED=false` on the backend, assert-after-search.

- [ ] **Step 1: Write the spec.** Model the login + API-request-context setup on `assets.spec.ts`. Scenario outline:

```ts
import { test, expect } from '@playwright/test'

const run = Date.now().toString(36)
const officeName = `Kantor Search E2E ${run}`
const officeCode = `SRCH${run}`.toUpperCase().slice(0, 12)

// beforeAll: API login as admin → create an office type (via reference engine, unique name)
// → POST /offices { name: officeName, code: officeCode, office_type_id } (mirror assets.spec.ts setup helpers)

test('palette finds a created office and navigates', async ({ page }) => {
  // UI login as admin (reuse the login helper pattern)
  await page.keyboard.press('Control+k')
  await page.getByPlaceholder(/cari/i).fill(officeName)
  // group header "Kantor" + the row with officeName appears (debounce: use expect polling, not fixed waits)
  await expect(page.getByRole('button', { name: new RegExp(officeName) })).toBeVisible()
  await page.getByRole('button', { name: new RegExp(officeName) }).click()
  await expect(page).toHaveURL(/\/master\/offices/)
})

test('palette shows the empty state for a no-hit query', async ({ page }) => {
  await page.keyboard.press('Control+k')
  await page.getByPlaceholder(/cari/i).fill(`zzz-no-hit-${run}`)
  await expect(page.getByText(/tidak ada hasil/i)).toBeVisible()
})
```

Adapt selectors to the palette's actual DOM (result rows are `<button>`s; the group header text is the i18n `search.group.kantor` label). Follow the file's sibling specs for the login helper and API context creation — copy those helpers into this spec rather than referencing them cross-file if they aren't exported.

- [ ] **Step 2: Run against the dev stack**

Prereqs: `docker compose -f docker-compose.dev.yml --profile app watch` (or host-run backend) + seeded admin + `RATELIMIT_ENABLED=false` (already in `backend/.env`).
Run (from `frontend/`): `pnpm test:e2e -- global-search.spec.ts`
Expected: 2/2 PASS.

- [ ] **Step 3: Commit**

```bash
git add frontend/e2e/global-search.spec.ts
git commit -m "test(search): real-backend e2e for the command palette"
```

---

### Task 10: Docs, vault, final gate sweep

**Files:**
- Modify: `docs/PROGRESS.md` — mark item 39 candidate (f) picked+done as item 40 (pattern of items 20/23/25…): summary line, the two approved deviations (250 ms debounce; requests title = type + office; also note the office-item "aktif" badge from the old mock is dropped — offices return `status: null`), mock-cleanup blast radius (what was deleted vs retained and why), and refresh the "▶ Next session — start here" block to point at the remaining candidate **(g) Reporting & Dashboard** + the tech-debt list.
- Modify (Obsidian vault `D:\Obsidian\inventra`): `Proyek/Status & Roadmap.md` (move global search to ✅, "Berikutnya" now (g) Reporting), `Modul/Peta Modul.md` (add search module + endpoint), new session note `Catatan/2026-07-11 Global search.md` (keputusan Opsi A + alasan penolakan B/C, ringkasan CQRS discussion), and index the product decision if `Keputusan/Produk/` conventions call for one (CQRS level-2 deferred to Reporting).

- [ ] **Step 1: Update PROGRESS.md and the vault files** as described above.

- [ ] **Step 2: Full gate sweep (everything CI enforces)**

```bash
cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... -count=1 -p 1
cd .. && npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml
cd frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build
```
Expected: all exit 0. (Full `pnpm test:e2e` optional locally — the new spec already ran in Task 9; the full suite hits documented dev-DB debris on other specs; CI runs it fresh.)

- [ ] **Step 3: Commit**

```bash
git add docs/PROGRESS.md
git commit -m "docs(progress): global search wired end-to-end; next: reporting"
```

(Vault lives outside the repo — no git add needed there.)

---

## Self-review notes

- Spec coverage: contract/min-length/limit (T3), authz table incl. silent skip (T3+T4), requests reason+id-prefix & composed title (T2/T3/T6), pg_trgm (T1), module split+wiring (T3), OpenAPI (T5), frontend rewrite+routes+status mapping (T6), debounce deviation (T7), mock blast radius (T8), integration+unit+component+e2e tests (T3/T4/T6/T7/T9), PROGRESS/vault + deviations recorded (T10). Out-of-scope items (see-all button, notifications/dashboard/reports mocks) intentionally untouched.
- Known adapt-points (flagged inline, not placeholders): generated sqlc field types (`OfficeName`, `Email`, `Lim`), `TYPE_META` export shape, e2e helper selectors — each step says exactly where to look.
