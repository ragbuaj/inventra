# Wire Peta Lokasi (Office Map) to a real geo endpoint — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the Peta Lokasi screen a real backend — add office coordinates + a geo-enriched `GET /offices/map` endpoint — and rewire the frontend map from mock to that endpoint.

**Architecture:** Backend gains `latitude`/`longitude` columns on `masterdata.offices` (settable via the office create/update API) and a new scoped `ListOfficesMap` query that LEFT JOINs office-type/province/city names and a per-office asset count, served at `GET /api/v1/offices/map`. Frontend rewrites `useOfficeMap` to call it, switches the map's category model from 4 hardcoded "jenis" to the 3 `office_types.tier` buckets, and rebinds the page/Leaflet component to English fields.

**Tech Stack:** Go 1.25 + Gin + sqlc (pgx/v5) + golang-migrate (backend); Nuxt 4 SPA + Nuxt UI + Leaflet + @nuxtjs/i18n + Vitest + Playwright (frontend).

## Global Constraints

- **English DTO keys + UUID identity** everywhere; never serialize raw Indonesian field names.
- **Coordinates use `double precision`** (NOT `numeric`): the sqlc config overrides `pg_catalog.numeric` → Go `string`, which would force lat/lng through string parsing. `double precision` → sqlc `*float64` → JSON `number` directly (Leaflet + range validation use the number as-is).
- Map endpoint **reuses `CallerOfficeScope(c, "offices")`** (same `AllScope bool` + `OfficeIds []uuid.UUID` params as every other office read).
- Map endpoint is **`authMW` + scope only — NO `RequirePermission`** (consistent with `GET /offices`). The frontend page keeps its existing guard `definePageMeta({ middleware:'can', permission:'masterdata.office.manage' })` unchanged.
- **Tier → category mapping:** backend returns `office_types.tier` (`pusat`/`wilayah`/`office`/`office_subtree`/null). Frontend maps it to `OfficeTier`: `pusat`→`pusat`, `wilayah`→`wilayah`, **everything else (incl. null, `office`, `office_subtree`) → `office`** (the "Cabang" bucket). Legend shows 3 categories.
- Map list response envelope is **`{ "data": [...] }`** (no pagination — the map loads the whole in-scope set).
- Frontend: all HTTP via `useApiClient().request`; i18n mandatory in BOTH `id.json` and `en.json`; no hardcoded user-facing strings; ESLint no-trailing-commas + 1tbs.
- Backend gates (run from `backend/`): `go build ./...`, `go vet ./...`, `go test ./...`, **and `go test -tags=integration ./...`** (shared-signature changes touch sqlc params), plus Spectral lint on `backend/api/openapi.yaml`.
- Frontend gates (run from `frontend/`): `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`.
- After editing migrations/queries: `sqlc generate` (from `backend/`). Never hand-edit `backend/db/sqlc/`.
- Conventional Commits with scope; no AI attribution in commits.

---

### Task 1: Office coordinates — migration + write DTO/query/response

Adds `latitude`/`longitude` to the offices schema and threads them through the office create/update/response so coordinates can be set via the real API (the map needs data; the later Offices screen will expose the inputs).

**Files:**
- Create: `backend/db/migrations/000018_office_coordinates.up.sql`, `backend/db/migrations/000018_office_coordinates.down.sql`
- Modify: `backend/db/queries/offices.sql` (CreateOffice, UpdateOffice)
- Modify: `backend/internal/masterdata/office/dto.go` (Request, Response, toInput, toResponse)
- Modify: `backend/internal/masterdata/office/service.go` (CreateInput, Create, Update)
- Regenerate: `backend/db/sqlc/` (via `sqlc generate`)
- Test: `backend/internal/masterdata/office/office_integration_test.go` (add a coordinate test)

**Interfaces:**
- Produces: `office.CreateInput` gains `Latitude *float64`, `Longitude *float64`. `office.Response` gains `Latitude *float64 json:"latitude"`, `Longitude *float64 json:"longitude"`. sqlc `MasterdataOffice` gains `Latitude *float64`, `Longitude *float64`; `CreateOfficeParams`/`UpdateOfficeParams` gain `Latitude *float64`, `Longitude *float64`. Task 2 reads these columns.

- [ ] **Step 1: Write the migration**

`backend/db/migrations/000018_office_coordinates.up.sql`:
```sql
-- Office geographic coordinates (for the Peta Lokasi / office-map screen).
-- double precision (not numeric): sqlc maps numeric -> Go string; float8 -> *float64,
-- which serializes as a JSON number for the map client.
ALTER TABLE masterdata.offices
  ADD COLUMN latitude  double precision,
  ADD COLUMN longitude double precision;
```

`backend/db/migrations/000018_office_coordinates.down.sql`:
```sql
ALTER TABLE masterdata.offices
  DROP COLUMN latitude,
  DROP COLUMN longitude;
```

- [ ] **Step 2: Apply the migration + verify**

Run (from `backend/`, dev DB on :5433):
```bash
export DATABASE_URL="postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable"
migrate -path db/migrations -database "$DATABASE_URL" up
```
Expected: migration `000018` applied, no error. (If the dev DB isn't running: `docker compose -f docker-compose.dev.yml up -d` first.)

- [ ] **Step 3: Extend the CreateOffice/UpdateOffice queries**

In `backend/db/queries/offices.sql`, replace the `CreateOffice` and `UpdateOffice` blocks (currently lines 31-49) with:
```sql
-- name: CreateOffice :one
INSERT INTO masterdata.offices (
  parent_id, office_type_id, province_id, city_id, name, code, address, is_active, latitude, longitude
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: UpdateOffice :one
UPDATE masterdata.offices
SET parent_id = sqlc.narg(parent_id),
    office_type_id = sqlc.arg(office_type_id),
    province_id = sqlc.narg(province_id),
    city_id = sqlc.narg(city_id),
    name = sqlc.arg(name),
    code = sqlc.arg(code),
    address = sqlc.narg(address),
    is_active = sqlc.arg(is_active),
    latitude = sqlc.narg(latitude),
    longitude = sqlc.narg(longitude)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND (sqlc.arg(all_scope)::bool OR id = ANY(sqlc.arg(office_ids)::uuid[]))
RETURNING *;
```

- [ ] **Step 4: Regenerate sqlc**

Run (from `backend/`): `sqlc generate`
Expected: no error. `git diff db/sqlc/` shows `MasterdataOffice`, `CreateOfficeParams`, `UpdateOfficeParams` now carry `Latitude *float64` and `Longitude *float64`.

- [ ] **Step 5: Thread lat/lng through the DTO + service**

In `backend/internal/masterdata/office/dto.go`:
- Add to the `Request` struct (after `Address`):
```go
	Latitude  *float64 `json:"latitude" binding:"omitempty,min=-90,max=90"`
	Longitude *float64 `json:"longitude" binding:"omitempty,min=-180,max=180"`
```
- In `toInput()`, add `Latitude: r.Latitude,` and `Longitude: r.Longitude,` to the returned `CreateInput{...}`.
- Add to the `Response` struct (after `Address`):
```go
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
```
- In `toResponse()`, add `Latitude: o.Latitude,` and `Longitude: o.Longitude,`.

In `backend/internal/masterdata/office/service.go`:
- Add to `CreateInput` (after `Address *string`):
```go
	Latitude  *float64
	Longitude *float64
```
- In `Create()`, add `Latitude: in.Latitude,` and `Longitude: in.Longitude,` to `sqlc.CreateOfficeParams{...}`.
- In `Update()`, add `Latitude: in.Latitude,` and `Longitude: in.Longitude,` to `sqlc.UpdateOfficeParams{...}`.

- [ ] **Step 6: Write the failing integration test**

In `backend/internal/masterdata/office/office_integration_test.go`, add a top-level helper near the other helpers (after `rowIDs`):
```go
func f64(v float64) *float64 { return &v }
```
and add this test function (sibling to `TestOfficeDataScope`):
```go
func TestOfficeCoordinates(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	q := sqlc.New(pool)
	svc := office.NewService(q)
	ctx := context.Background()

	t.Run("create stores and returns coordinates", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		created, err := svc.Create(ctx, true, nil, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			Name: "Coord Office", Code: "COORD", IsActive: true,
			Latitude: f64(-6.1754), Longitude: f64(106.8272),
		})
		require.NoError(t, err)
		require.NotNil(t, created.Latitude)
		require.NotNil(t, created.Longitude)
		assert.InDelta(t, -6.1754, *created.Latitude, 1e-9)
		assert.InDelta(t, 106.8272, *created.Longitude, 1e-9)
	})

	t.Run("update changes coordinates", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		_, after, err := svc.Update(ctx, tree.Cabang, true, nil, office.UpdateInput{
			CreateInput: office.CreateInput{
				ParentID: &tree.Wilayah, OfficeTypeID: tree.OfficeTypeID,
				Name: "Cabang 1", Code: "C1", IsActive: true,
				Latitude: f64(-6.29), Longitude: f64(106.80),
			},
		})
		require.NoError(t, err)
		require.NotNil(t, after.Latitude)
		assert.InDelta(t, -6.29, *after.Latitude, 1e-9)
	})
}
```

- [ ] **Step 7: Run build + the test**

Run (from `backend/`):
```bash
go build ./... && go vet ./...
go test -tags=integration ./internal/masterdata/office/ -run TestOfficeCoordinates -v
```
Expected: build clean; `TestOfficeCoordinates` PASS (2 subtests). (Integration tests need the dev Postgres up.)

- [ ] **Step 8: Commit**

```bash
git add backend/db/migrations/000018_office_coordinates.up.sql backend/db/migrations/000018_office_coordinates.down.sql backend/db/queries/offices.sql backend/db/sqlc/ backend/internal/masterdata/office/dto.go backend/internal/masterdata/office/service.go backend/internal/masterdata/office/office_integration_test.go
git commit -m "feat(masterdata): add office latitude/longitude to schema + write API"
```

---

### Task 2: `GET /offices/map` — geo-enriched, scoped map endpoint

Adds the read-only map query (resolved type/province/city names + per-office asset count), a service method, a map DTO, the handler, the route, and OpenAPI.

**Files:**
- Modify: `backend/db/queries/offices.sql` (add `ListOfficesMap`)
- Regenerate: `backend/db/sqlc/`
- Modify: `backend/internal/masterdata/office/service.go` (add `MapList`)
- Modify: `backend/internal/masterdata/office/dto.go` (add `MapResponse` + `toMapResponse`)
- Modify: `backend/internal/masterdata/office/handler.go` (add `mapList`)
- Modify: `backend/internal/masterdata/office/routes.go` (register `GET /offices/map`)
- Modify: `backend/api/openapi.yaml` (document the endpoint + lat/lng on office schemas)
- Test: `backend/internal/masterdata/office/office_integration_test.go` (add `TestOfficeMapList`)

**Interfaces:**
- Consumes: the lat/lng columns from Task 1.
- Produces: `GET /api/v1/offices/map` → `{ "data": [ {id,name,code,office_type_name,tier,province_name,city_name,address,asset_count,latitude,longitude} ] }`. `office.MapList(ctx, all bool, ids []uuid.UUID) ([]sqlc.ListOfficesMapRow, error)`. Frontend Task 3 consumes this JSON shape.

- [ ] **Step 1: Add the ListOfficesMap query**

Append to `backend/db/queries/offices.sql`:
```sql
-- name: ListOfficesMap :many
-- Geo-enriched, scoped office list for the Peta Lokasi screen: resolves
-- office-type/province/city names + a per-office (non-deleted) asset count.
SELECT
  o.id, o.name, o.code, o.address, o.latitude, o.longitude,
  ot.name AS office_type_name,
  ot.tier AS tier,
  p.name  AS province_name,
  c.name  AS city_name,
  (SELECT count(*) FROM asset.assets a
     WHERE a.office_id = o.id AND a.deleted_at IS NULL) AS asset_count
FROM masterdata.offices o
LEFT JOIN masterdata.office_types ot ON ot.id = o.office_type_id AND ot.deleted_at IS NULL
LEFT JOIN masterdata.provinces    p  ON p.id  = o.province_id    AND p.deleted_at IS NULL
LEFT JOIN masterdata.cities       c  ON c.id  = o.city_id        AND c.deleted_at IS NULL
WHERE o.deleted_at IS NULL
  AND o.is_active = true
  AND (sqlc.arg(all_scope)::bool OR o.id = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY o.name;
```

- [ ] **Step 2: Regenerate sqlc + inspect the row type**

Run (from `backend/`): `sqlc generate`
Expected: a new `ListOfficesMapParams{AllScope bool; OfficeIds []uuid.UUID}` and `ListOfficesMapRow` with fields `ID uuid.UUID`, `Name string`, `Code string`, `Address *string`, `Latitude *float64`, `Longitude *float64`, `OfficeTypeName *string`, `Tier *SharedApproverLevel`, `ProvinceName *string`, `CityName *string`, `AssetCount int64`. (If field names/types differ, use the actual generated names in the steps below.)

- [ ] **Step 3: Add the MapList service method**

In `backend/internal/masterdata/office/service.go`, add after `List`:
```go
// MapList returns geo-enriched offices within the caller's scope for the map screen.
func (s *Service) MapList(ctx context.Context, all bool, ids []uuid.UUID) ([]sqlc.ListOfficesMapRow, error) {
	return s.q.ListOfficesMap(ctx, sqlc.ListOfficesMapParams{AllScope: all, OfficeIds: ids})
}
```

- [ ] **Step 4: Add the map DTO**

In `backend/internal/masterdata/office/dto.go`, add at the end:
```go
// MapResponse is one office on the Peta Lokasi map (resolved names + asset count).
type MapResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Code           string   `json:"code"`
	OfficeTypeName *string  `json:"office_type_name"`
	Tier           *string  `json:"tier"`
	ProvinceName   *string  `json:"province_name"`
	CityName       *string  `json:"city_name"`
	Address        *string  `json:"address"`
	AssetCount     int64    `json:"asset_count"`
	Latitude       *float64 `json:"latitude"`
	Longitude      *float64 `json:"longitude"`
}

func toMapResponse(r sqlc.ListOfficesMapRow) MapResponse {
	var tier *string
	if r.Tier != nil {
		s := string(*r.Tier)
		tier = &s
	}
	return MapResponse{
		ID:             r.ID.String(),
		Name:           r.Name,
		Code:           r.Code,
		OfficeTypeName: r.OfficeTypeName,
		Tier:           tier,
		ProvinceName:   r.ProvinceName,
		CityName:       r.CityName,
		Address:        r.Address,
		AssetCount:     r.AssetCount,
		Latitude:       r.Latitude,
		Longitude:      r.Longitude,
	}
}
```

- [ ] **Step 5: Add the handler**

In `backend/internal/masterdata/office/handler.go`, add after `list` (before `get`):
```go
func (h *Handler) mapList(c *gin.Context) {
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	rows, err := h.svc.MapList(c.Request.Context(), all, ids)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list office map"})
		return
	}
	data := make([]MapResponse, 0, len(rows))
	for _, r := range rows {
		data = append(data, toMapResponse(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": data})
}
```

- [ ] **Step 6: Register the route**

In `backend/internal/masterdata/office/routes.go`, add the map route inside `RegisterRoutes` (Gin v1.12 allows a static `/map` segment alongside `/:id`):
```go
	g := rg.Group("/offices")
	g.GET("/map", authMW, h.mapList)
	g.GET("", authMW, h.list)
	g.GET("/:id", authMW, h.get)
	g.POST("", authMW, requireManage, h.create)
	g.PUT("/:id", authMW, requireManage, h.update)
	g.DELETE("/:id", authMW, requireManage, h.delete)
```

- [ ] **Step 7: Write the failing integration test**

In `backend/internal/masterdata/office/office_integration_test.go`, add:
```go
func TestOfficeMapList(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	q := sqlc.New(pool)
	svc := office.NewService(q)
	ctx := context.Background()

	t.Run("resolves names + coords, asset_count zero without assets", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		provID := uuid.New()
		cityID := uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO masterdata.provinces (id, name, code) VALUES ($1,$2,$3)`, provID, "DKI Jakarta", "31")
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO masterdata.cities (id, province_id, name, code) VALUES ($1,$2,$3,$4)`, cityID, provID, "Jakarta Pusat", "3171")
		require.NoError(t, err)

		created, err := svc.Create(ctx, true, nil, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			ProvinceID: &provID, CityID: &cityID,
			Name: "Map Office", Code: "MAP1", IsActive: true,
			Latitude: f64(-6.1754), Longitude: f64(106.8272),
		})
		require.NoError(t, err)

		rows, err := svc.MapList(ctx, true, nil)
		require.NoError(t, err)

		var got *sqlc.ListOfficesMapRow
		for i := range rows {
			if rows[i].ID == created.ID {
				got = &rows[i]
			}
		}
		require.NotNil(t, got, "created office present in map list")
		require.NotNil(t, got.OfficeTypeName)
		assert.NotEmpty(t, *got.OfficeTypeName)
		require.NotNil(t, got.ProvinceName)
		assert.Equal(t, "DKI Jakarta", *got.ProvinceName)
		require.NotNil(t, got.CityName)
		assert.Equal(t, "Jakarta Pusat", *got.CityName)
		require.NotNil(t, got.Latitude)
		assert.InDelta(t, -6.1754, *got.Latitude, 1e-9)
		assert.Equal(t, int64(0), got.AssetCount)
	})

	t.Run("respects data scope", func(t *testing.T) {
		testsupport.Reset(t, pool)
		tree := testsupport.SeedOfficeTree(t, pool)

		outOfScope, err := svc.Create(ctx, true, nil, office.CreateInput{
			ParentID: &tree.Pusat, OfficeTypeID: tree.OfficeTypeID,
			Name: "Under Pusat", Code: "UP1", IsActive: true,
		})
		require.NoError(t, err)

		rows, err := svc.MapList(ctx, false, []uuid.UUID{tree.Wilayah, tree.Cabang})
		require.NoError(t, err)
		ids := make(map[uuid.UUID]bool, len(rows))
		for _, r := range rows {
			ids[r.ID] = true
		}
		assert.True(t, ids[tree.Cabang], "in-scope office present")
		assert.False(t, ids[outOfScope.ID], "out-of-scope office absent")
		assert.False(t, ids[tree.Pusat], "ancestor out of scope absent")
	})
}
```

- [ ] **Step 8: Run build + tests**

Run (from `backend/`):
```bash
go build ./... && go vet ./...
go test -tags=integration ./internal/masterdata/office/ -run 'TestOfficeMapList|TestOfficeCoordinates' -v
go test ./...
```
Expected: build clean; map + coordinate tests PASS; full non-integration suite green.

- [ ] **Step 9: Update OpenAPI + lint**

In `backend/api/openapi.yaml`:
- Add a `GET /offices/map` path: summary "Office map", security same as other authenticated reads, 200 response `{ data: array of OfficeMapItem }` where `OfficeMapItem` has `id, name, code, office_type_name (nullable), tier (nullable enum pusat|wilayah|office|office_subtree), province_name (nullable), city_name (nullable), address (nullable), asset_count (integer), latitude (nullable number), longitude (nullable number)`.
- Add `latitude` (number, nullable) and `longitude` (number, nullable) to the office create/update request schema and the office response schema.

Run (from repo root): `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: no errors.

- [ ] **Step 10: Commit**

```bash
git add backend/db/queries/offices.sql backend/db/sqlc/ backend/internal/masterdata/office/ backend/api/openapi.yaml
git commit -m "feat(masterdata): add scoped GET /offices/map geo endpoint"
```

---

### Task 3: Frontend data layer — types, tier meta, `useOfficeMap`

Rewrites the map's type model to tier-based English fields, moves the category meta out of mock into a constants file, and rewires the composable to the real endpoint.

**Files:**
- Modify: `frontend/app/types/index.ts` (replace `OfficeJenis`/`MapOffice`)
- Create: `frontend/app/constants/officeMapMeta.ts`
- Modify (full rewrite): `frontend/app/composables/api/useOfficeMap.ts`
- Test: `frontend/test/unit/use-office-map.spec.ts`

**Interfaces:**
- Consumes: `GET /offices/map` from Task 2.
- Produces: `OfficeTier = 'pusat'|'wilayah'|'office'`; `MapOffice` (English fields below); `tierMeta`/`TIER_ORDER` from `~/constants/officeMapMeta`; `useOfficeMap().list(): Promise<MapOffice[]>`. Tasks 4 consume all of these.

- [ ] **Step 1: Replace the types**

In `frontend/app/types/index.ts`, replace the `OfficeJenis` + `MapOffice` block (lines 191-204) with:
```ts
export type OfficeTier = 'pusat' | 'wilayah' | 'office'

export interface MapOffice {
  id: string
  name: string
  code: string
  office_type_name: string | null
  tier: OfficeTier
  province_name: string | null
  city_name: string | null
  address: string | null
  asset_count: number
  latitude: number | null
  longitude: number | null
}
```

- [ ] **Step 2: Create the tier meta constants**

`frontend/app/constants/officeMapMeta.ts`:
```ts
import type { OfficeTier } from '~/types'

/**
 * Office tier → i18n label key, pin CSS var, and soft badge classes.
 * 3 buckets (office_types.tier): pusat / wilayah / office (Cabang).
 */
export const tierMeta: Record<OfficeTier, {
  labelKey: string
  pinVar: string
  softBg: string
  softText: string
  icon: string
}> = {
  pusat: { labelKey: 'map.tier.pusat', pinVar: '--pin-pusat', softBg: 'bg-primary/10', softText: 'text-primary', icon: 'i-lucide-landmark' },
  wilayah: { labelKey: 'map.tier.wilayah', pinVar: '--pin-wilayah', softBg: 'bg-info/10', softText: 'text-info', icon: 'i-lucide-building-2' },
  office: { labelKey: 'map.tier.office', pinVar: '--pin-cabang', softBg: 'bg-warning/10', softText: 'text-warning', icon: 'i-lucide-building' }
}

export const TIER_ORDER: OfficeTier[] = ['pusat', 'wilayah', 'office']
```

- [ ] **Step 3: Write the failing unit test**

`frontend/test/unit/use-office-map.spec.ts`:
```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
// eslint-disable-next-line import/first
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useOfficeMap } from '~/composables/api/useOfficeMap'

beforeEach(() => request.mockReset())

describe('useOfficeMap', () => {
  it('GETs /offices/map and passes through resolved fields', async () => {
    request.mockResolvedValueOnce({ data: [{
      id: 'o1', name: 'Kantor Pusat', code: 'PST', office_type_name: 'Kantor Pusat',
      tier: 'pusat', province_name: 'DKI Jakarta', city_name: 'Jakarta Pusat',
      address: 'Jl. Merdeka 1', asset_count: 12, latitude: -6.1754, longitude: 106.8272
    }] })
    const rows = await useOfficeMap().list()
    expect(request).toHaveBeenCalledWith('/offices/map')
    expect(rows).toHaveLength(1)
    expect(rows[0]).toMatchObject({ id: 'o1', name: 'Kantor Pusat', tier: 'pusat', province_name: 'DKI Jakarta', asset_count: 12, latitude: -6.1754 })
  })

  it('maps null / office_subtree / office tier to the office bucket', async () => {
    request.mockResolvedValueOnce({ data: [
      { id: 'a', name: 'A', code: 'A', office_type_name: null, tier: null, province_name: null, city_name: null, address: null, asset_count: 0, latitude: null, longitude: null },
      { id: 'b', name: 'B', code: 'B', office_type_name: 'X', tier: 'office_subtree', province_name: null, city_name: null, address: null, asset_count: 0, latitude: null, longitude: null },
      { id: 'c', name: 'C', code: 'C', office_type_name: 'Y', tier: 'office', province_name: null, city_name: null, address: null, asset_count: 0, latitude: null, longitude: null }
    ] })
    const rows = await useOfficeMap().list()
    expect(rows.map(r => r.tier)).toEqual(['office', 'office', 'office'])
  })

  it('keeps pusat and wilayah tiers', async () => {
    request.mockResolvedValueOnce({ data: [
      { id: 'a', name: 'A', code: 'A', office_type_name: 'X', tier: 'pusat', province_name: null, city_name: null, address: null, asset_count: 0, latitude: null, longitude: null },
      { id: 'b', name: 'B', code: 'B', office_type_name: 'Y', tier: 'wilayah', province_name: null, city_name: null, address: null, asset_count: 0, latitude: null, longitude: null }
    ] })
    const rows = await useOfficeMap().list()
    expect(rows.map(r => r.tier)).toEqual(['pusat', 'wilayah'])
  })
})
```

- [ ] **Step 4: Run to verify it fails**

Run (from `frontend/`): `pnpm test -- use-office-map`
Expected: FAIL (current `useOfficeMap` returns the mock array / wrong shape).

- [ ] **Step 5: Rewrite `useOfficeMap.ts`**

Replace `frontend/app/composables/api/useOfficeMap.ts` entirely with:
```ts
import type { MapOffice, OfficeTier } from '~/types'

interface MapOfficeDTO {
  id: string
  name: string
  code: string
  office_type_name: string | null
  tier: string | null
  province_name: string | null
  city_name: string | null
  address: string | null
  asset_count: number
  latitude: number | null
  longitude: number | null
}

function toTier(raw: string | null): OfficeTier {
  return raw === 'pusat' || raw === 'wilayah' ? raw : 'office'
}

export function useOfficeMap() {
  const { request } = useApiClient()

  async function list(): Promise<MapOffice[]> {
    const res = await request<{ data: MapOfficeDTO[] }>('/offices/map')
    return res.data.map(o => ({
      id: o.id,
      name: o.name,
      code: o.code,
      office_type_name: o.office_type_name,
      tier: toTier(o.tier),
      province_name: o.province_name,
      city_name: o.city_name,
      address: o.address,
      asset_count: o.asset_count,
      latitude: o.latitude,
      longitude: o.longitude
    }))
  }

  return { list }
}
```

- [ ] **Step 6: Run the test + lint**

Run (from `frontend/`): `pnpm test -- use-office-map && pnpm lint`
Expected: PASS, lint clean. NOTE: `pnpm typecheck` will still FAIL in `OfficeMap.client.vue`, `map.vue`, and `mock/officeMap.ts` (old field names) — EXPECTED, fixed in Task 4 (mock deleted in Task 5).

- [ ] **Step 7: Commit**

```bash
git add frontend/app/types/index.ts frontend/app/constants/officeMapMeta.ts frontend/app/composables/api/useOfficeMap.ts frontend/test/unit/use-office-map.spec.ts
git commit -m "feat(master-map): tier-based MapOffice types + useOfficeMap wired to /offices/map"
```

---

### Task 4: Frontend UI — Leaflet component, page rebind, i18n

Rebinds the Leaflet pin component and the map page to the new English/tier fields, adds load-error+retry, filters pins to offices with coordinates, and updates i18n.

**Files:**
- Modify: `frontend/app/components/OfficeMap.client.vue`
- Modify: `frontend/app/pages/master/map.vue`
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json` (map.* keys)
- Test: `frontend/test/nuxt/master-map.spec.ts`

**Interfaces:**
- Consumes: `MapOffice`, `OfficeTier`, `tierMeta`, `TIER_ORDER`, `useOfficeMap` from Task 3.

- [ ] **Step 1: Update the i18n map block**

Read the `map` block in both `frontend/i18n/locales/id.json` and `frontend/i18n/locales/en.json`. Then:
- Remove the old per-category keys `map.jenis.pusat/wilayah/cabang/outlet` (if present).
- Add `map.tier.pusat`, `map.tier.wilayah`, `map.tier.office`:
  - id: `"pusat": "Pusat"`, `"wilayah": "Wilayah"`, `"office": "Cabang"` (under `"tier": { ... }`)
  - en: `"pusat": "Head Office"`, `"wilayah": "Region"`, `"office": "Branch"`
- Add `map.loadError` + `map.retry`:
  - id: `"loadError": "Gagal memuat peta kantor."`, `"retry": "Coba lagi"`
  - en: `"loadError": "Failed to load office map."`, `"retry": "Retry"`
- Keep existing `map.jenisAll`, `map.provAll`, `map.summary`, `map.title`, `map.searchPlaceholder`, `map.emptyListTitle/Sub`, `map.emptyMapTitle/Sub`, `map.resetTip/resetLabel`, `map.registeredAssets`, `map.viewOffice`, `map.openMaps`, `map.usageNote` unchanged.

- [ ] **Step 2: Update `OfficeMap.client.vue`**

In `frontend/app/components/OfficeMap.client.vue`:
- Replace the import `import { jenisMeta } from '~/mock/officeMap'` with `import { tierMeta } from '~/constants/officeMapMeta'`.
- In `pinHtml`, change `jenisMeta[o.jenis].pinVar` → `tierMeta[o.tier].pinVar`.
- In `render()` and `fitAll()` and the `selectedId` watcher, the marker positions use `o.lat`/`o.lng` — change to `o.latitude`/`o.longitude`, and guard nulls. Replace `render()` body's loop and `fitAll()` with:
```ts
function render() {
  if (!map) return
  for (const m of markers.values()) {
    m.remove()
  }
  markers = new Map()
  for (const o of props.offices) {
    if (o.latitude == null || o.longitude == null) continue
    const selected = o.id === props.selectedId
    const m = L.marker([o.latitude, o.longitude], { icon: icon(o, selected), zIndexOffset: selected ? 1000 : 0 })
    m.on('click', () => {
      emit('select', o.id)
    })
    m.addTo(map)
    markers.set(o.id, m)
  }
}

function fitAll() {
  if (!map) return
  const pts = props.offices.filter(o => o.latitude != null && o.longitude != null).map(o => [o.latitude as number, o.longitude as number] as [number, number])
  if (pts.length === 0) return
  map.fitBounds(L.latLngBounds(pts), { padding: [48, 48] })
}
```
- In the `selectedId` watcher, change `map.flyTo([o.lat, o.lng], ...)` to guard + `o.latitude`/`o.longitude`:
```ts
watch(() => props.selectedId, (id) => {
  render()
  const o = props.offices.find(x => x.id === id)
  if (o && o.latitude != null && o.longitude != null && map) map.flyTo([o.latitude, o.longitude], Math.max(map.getZoom(), 12), { duration: 0.5 })
})
```

- [ ] **Step 3: Rebind `map.vue`**

In `frontend/app/pages/master/map.vue`:
- Script imports: replace `import type { MapOffice, OfficeJenis } from '~/types'` → `import type { MapOffice, OfficeTier } from '~/types'`; replace `import { jenisMeta, JENIS_ORDER } from '~/mock/officeMap'` → `import { tierMeta, TIER_ORDER } from '~/constants/officeMapMeta'`.
- State: `const fJenis = ref<'all' | OfficeJenis>('all')` → `const fTier = ref<'all' | OfficeTier>('all')`; add `const loadFailed = ref(false)`.
- Replace `onMounted` with a `reload()`:
```ts
async function reload() {
  loading.value = true
  loadFailed.value = false
  try {
    offices.value = await list()
  } catch {
    loadFailed.value = true
  } finally {
    loading.value = false
  }
}
onMounted(reload)
```
- `provinces` computed: `offices.value.map(o => o.prov)` → `offices.value.map(o => o.province_name).filter((p): p is string => !!p)`.
- `filtered` computed: `o.nama`→`o.name`, `o.kode`→`o.code`, `o.jenis !== fJenis.value`→`o.tier !== fTier.value`, `o.prov !== fProv.value`→`o.province_name !== fProv.value`. (Rename `fJenis`→`fTier` throughout.)
- `summaryText`: `o.kota`→`o.city_name`, `o.prov`→`o.province_name` (Sets of possibly-null; that's fine for counting distinct, but filter nulls: `new Set(filtered.value.map(o => o.city_name).filter(Boolean)).size`).
- `jenisItems` → keep the variable name used in template OR rename to `tierItems`; map over `TIER_ORDER` with `tierMeta[j].labelKey`. Update the `<USelect v-model="fJenis">` to `v-model="fTier"` and `:items="tierItems"`.
- `watch(fJenis, ...)` → `watch(fTier, ...)`.
- Pass only coordinate-bearing offices to the Leaflet component: add `const mapped = computed(() => filtered.value.filter(o => o.latitude != null && o.longitude != null))` and change `<OfficeMap :offices="filtered" ...>` → `:offices="mapped"`. Keep the list panel bound to `filtered` (offices without coords still listed).
- Template field rebinds in the list rows + detail card: `office.nama`→`office.name`, `office.kode`→`office.code`, `jenisMeta[office.jenis]`→`tierMeta[office.tier]`, `office.kota`→`office.city_name`, `office.prov`→`office.province_name` (render `{{ office.city_name }}{{ office.province_name ? ', ' + office.province_name : '' }}`), `selected.nama/kode/alamat/kota/prov/aset`→`selected.name/code/address/city_name/province_name/asset_count`, `jenisMeta[selected.jenis]`→`tierMeta[selected.tier]`, `JENIS_ORDER`→`TIER_ORDER`.
- "Open in Maps" link: guard null coords — wrap the `<a :href="googleMapsUrl(selected.lat, selected.lng)">` in `<template v-if="selected.latitude != null && selected.longitude != null">` and use `googleMapsUrl(selected.latitude, selected.longitude)`.
- Add a load-error block: when `loadFailed`, render an error panel (in the left list area and/or a top-level overlay) with `{{ $t('map.loadError') }}` + a retry `UButton` calling `reload`. Minimal placement: replace the list-panel body's `<template v-if="loading">…</template><template v-else>…</template>` so there's a `v-else-if="loadFailed"` branch:
```vue
          <div
            v-else-if="loadFailed"
            class="px-4 py-10 text-center"
          >
            <p class="text-[13.5px] font-semibold mb-2">
              {{ $t('map.loadError') }}
            </p>
            <UButton
              color="neutral"
              variant="subtle"
              size="sm"
              @click="reload"
            >
              {{ $t('map.retry') }}
            </UButton>
          </div>
```
  (Keep the existing loading skeleton as `v-if="loading"` and the populated rows as the final `v-else`.)

- [ ] **Step 4: Write the component test**

`frontend/test/nuxt/master-map.spec.ts` (study `frontend/test/nuxt/settings-audit.spec.ts` for the `vi.mock('~/composables/useApiClient')` + `useAuthStore().setSession` + `mountSuspended` harness first):
```ts
// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'

const request = vi.fn()
// eslint-disable-next-line import/first
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useAuthStore } from '~/stores/auth'
// eslint-disable-next-line import/first
import MapPage from '~/pages/master/map.vue'

const OFFICES = [
  { id: 'o1', name: 'Kantor Pusat', code: 'PST', office_type_name: 'Kantor Pusat', tier: 'pusat', province_name: 'DKI Jakarta', city_name: 'Jakarta Pusat', address: 'Jl. Merdeka 1', asset_count: 12, latitude: -6.1754, longitude: 106.8272 },
  { id: 'o2', name: 'Cabang Bekasi', code: 'BKS01', office_type_name: 'Kantor Cabang', tier: 'office', province_name: 'Jawa Barat', city_name: 'Bekasi', address: 'Jl. A. Yani 1', asset_count: 3, latitude: -6.2383, longitude: 106.9756 },
  { id: 'o3', name: 'Cabang Tanpa Koordinat', code: 'NOC', office_type_name: 'Kantor Cabang', tier: 'office', province_name: 'Banten', city_name: 'Tangerang', address: 'Jl. X', asset_count: 0, latitude: null, longitude: null }
]

beforeEach(() => {
  request.mockReset()
  useAuthStore().setSession('t', { id: 'u', name: 'Admin', email: 'a@x.id', role_id: 'r', role_name: '' }, ['*'])
})

async function mountMap() {
  const wrapper = await mountSuspended(MapPage, { global: { stubs: { OfficeMap: true } } })
  await new Promise(r => setTimeout(r, 50))
  return wrapper
}

describe('Peta Lokasi page', () => {
  it('renders office rows with resolved names + tier labels', async () => {
    request.mockResolvedValueOnce({ data: OFFICES })
    const wrapper = await mountMap()
    const text = wrapper.text()
    expect(request).toHaveBeenCalledWith('/offices/map')
    expect(text).toContain('Kantor Pusat')
    expect(text).toContain('Cabang Bekasi')
    expect(text).toContain('Jakarta Pusat')
    expect(text).toContain('Bekasi')
  })

  it('filters the list by province', async () => {
    request.mockResolvedValueOnce({ data: OFFICES })
    const wrapper = await mountMap()
    ;(wrapper.vm as unknown as { fProv: string }).fProv = 'Jawa Barat'
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Cabang Bekasi')
    expect(text).not.toContain('Kantor Pusat')
  })

  it('selecting an office shows its detail (address + asset count)', async () => {
    request.mockResolvedValueOnce({ data: OFFICES })
    const wrapper = await mountMap()
    ;(wrapper.vm as unknown as { selId: string | null }).selId = 'o1'
    await wrapper.vm.$nextTick()
    const card = wrapper.find('[data-testid="office-detail-card"]')
    expect(card.exists()).toBe(true)
    expect(card.text()).toContain('Jl. Merdeka 1')
    expect(card.text()).toContain('12')
  })

  it('shows the error state + retry on load failure, then recovers', async () => {
    request.mockRejectedValueOnce(new Error('500'))
    const wrapper = await mountMap()
    expect(wrapper.text()).toContain('Gagal memuat peta kantor.')
    request.mockResolvedValueOnce({ data: OFFICES })
    await wrapper.find('button').trigger('click') // retry button (first button in error panel)
    await new Promise(r => setTimeout(r, 50))
    expect(wrapper.text()).toContain('Kantor Pusat')
  })

  it('renders the empty state when no offices', async () => {
    request.mockResolvedValueOnce({ data: [] })
    const wrapper = await mountMap()
    expect(wrapper.text()).toContain('Belum ada')
  })
})
```
Adapt the empty-state assertion text to the actual `map.emptyListTitle` value you read in Step 1 (e.g. the id string). If the retry-button selector is ambiguous, scope it to the error panel (e.g. add a `data-testid="map-retry"` to the retry button in Step 3 and select that). Assert REAL rendered text — no hollow checks.

- [ ] **Step 5: Run frontend checks**

Run (from `frontend/`):
```bash
pnpm test -- master-map
pnpm lint
pnpm typecheck
```
Expected: target test PASS; lint clean; typecheck clean EXCEPT `mock/officeMap.ts` (deleted in Task 5). If typecheck still flags `mock/officeMap.ts`, that's expected here.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/components/OfficeMap.client.vue frontend/app/pages/master/map.vue frontend/i18n/locales/id.json frontend/i18n/locales/en.json frontend/test/nuxt/master-map.spec.ts
git commit -m "feat(master-map): rebind page + Leaflet to tier-based geo fields, add load-error state"
```

---

### Task 5: E2E + delete mock + mockup compare + PROGRESS + full gate

**Files:**
- Create: `frontend/e2e/master-map.spec.ts` (or add a block to an existing master/settings e2e spec)
- Delete: `frontend/app/mock/officeMap.ts`
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Delete the orphaned mock + verify no importers**

Run (from repo root): `grep -rn "mock/officeMap" frontend/app frontend/test` (exclude the file itself).
After Tasks 3-4, the only former importers (`useOfficeMap.ts`, `map.vue`, `OfficeMap.client.vue`) now import from `~/constants/officeMapMeta`. If ZERO importers remain, `git rm frontend/app/mock/officeMap.ts`. If anything still imports it, fix that import to `~/constants/officeMapMeta` (for `tierMeta`/`TIER_ORDER`) and report — do not leave a broken import.

- [ ] **Step 2: Write the e2e**

Read `frontend/e2e/helpers.ts` (`login()`) and an existing wired e2e (`frontend/e2e/settings.spec.ts`) for the established robust-locator style. Create `frontend/e2e/master-map.spec.ts`:
```ts
import { test, expect } from '@playwright/test'
import { login } from './helpers'

test.describe('Peta Lokasi', () => {
  test('loads the office map screen', async ({ page }) => {
    await login(page)
    await page.goto('/master/map')

    // Heading renders (read the real map.title string and match it).
    await expect(page.getByRole('heading', { name: 'Peta Lokasi' })).toBeVisible()

    // The left list shows either office rows or the empty-list state (data may be
    // empty in CI — no offices with coordinates seeded). Auto-waiting .or().
    await expect(
      page.getByText('Belum ada', { exact: false }).or(page.locator('button:has(.font-mono)').first())
    ).toBeVisible()

    // The map panel header/legend renders (the summary strip).
    await expect(page.getByText('Pusat', { exact: false })).toBeVisible()
  })
})
```
Adjust `'Peta Lokasi'` and `'Belum ada'` to the real i18n strings (`map.title`, `map.emptyListTitle`). ROBUST locators only: text/role-based; for any USelect use trigger-click + `role="option"` (never `selectOption`); NO `isVisible()` snapshot booleans driving control flow; NO silent `if(...) return`; NO `.last()`/`.first()` on broad `div` filters (the `button:has(.font-mono)` above targets the office-row code chip specifically — if brittle, prefer a `data-testid` on the row added in Task 4). You likely CANNOT run `pnpm test:e2e` here (needs the full backend stack + seeded admin); ensure it compiles + lints; CI runs it. State that in your report.

- [ ] **Step 3: Mockup fidelity comparison**

Read `docs/design/Peta Lokasi.dc.html` and the built `frontend/app/pages/master/map.vue`. Verify the 2-column layout (list panel + map panel), filters (search + 2 selects), legend, summary strip, and detail card match. APPROVED deviation: the legend shows **3 tier categories** (Pusat/Wilayah/Cabang) instead of the mockup's 4 (Outlet folded into Cabang — `office_types.tier` cannot distinguish them). Fix any OTHER genuine deviation; report the result.

- [ ] **Step 4: Update PROGRESS.md**

In `docs/PROGRESS.md`:
- Mark Peta Lokasi ✅ wired to `GET /offices/map` (office lat/lng columns + geo endpoint with resolved type/province/city names + per-office asset count; data-scoped). Note this is the first of the master-data wiring batch.
- Add TODO notes: `office_types.tier` is not yet editable (the office-types reference resource doesn't expose `tier`) → offices with null tier render as **Cabang**; the map shows an empty-state until offices have coordinates (no production seed); per-office asset count is real but 0 until the asset module is populated.
- Refresh "▶ Next session — start here" → next master-data sub-project = **Referensi** (11 generic resources; add FK pickers for cities→province_id & models→brand_id; optionally expose office_types.tier).

- [ ] **Step 5: Full gate (backend + frontend)**

Run (from `backend/`):
```bash
go build ./... && go vet ./... && go test ./...
go test -tags=integration ./...
npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset ../.spectral.yaml
```
Run (from `frontend/`):
```bash
pnpm lint && pnpm typecheck && pnpm test && pnpm build
```
Expected: all green. (`go test -tags=integration ./...` needs the dev Postgres up; the Spectral ruleset path is `.spectral.yaml` at repo root — adjust the relative path to where you run it. E2E runs in CI.)

- [ ] **Step 6: Commit**

```bash
git add frontend/e2e/master-map.spec.ts docs/PROGRESS.md
git rm frontend/app/mock/officeMap.ts
git commit -m "test(master-map): e2e + drop mock; wire Peta Lokasi end-to-end"
```

---

## Self-Review

**Spec coverage:**
- §2.1 migration `000018` lat/lng → Task 1 (uses `double precision`, resolving the spec's numeric↔number open question per Global Constraints). ✓
- §2.2 office write DTO/response lat/lng + validation → Task 1. ✓
- §2.3 `ListOfficesMap` (JOINs + asset_count + scope) + `GET /offices/map` (authMW+scope) → Task 2. ✓
- §2.4 openapi + Go tests (scope, name resolution, asset_count, lat/lng) → Task 1 (coords) + Task 2 (map query/openapi). ✓
- §3.1 `useOfficeMap` rewrite + §3.2 types/`officeMapMeta` → Task 3. ✓
- §3.3 page rebind + load-error/retry + coord-filtered pins + null guards → Task 4. ✓
- §3.4 `OfficeMap` Leaflet field rename → Task 4. ✓
- §3.5 i18n tier labels + loadError/retry → Task 4. ✓
- §4 tests unit/component/e2e → Tasks 3/4/5. ✓
- §5 done (delete mock, mockup compare, PROGRESS, gate) → Task 5. ✓
- §6 risks (tier null→Cabang, empty map, route conflict, numeric↔number) → handled: tier mapping in Task 3 + constraint; route `/map` before `/:id` (Gin 1.12 supports static+param) in Task 2; double precision in Task 1.

**Placeholder scan:** Tasks 4-5 contain explicit "read X first" pointers (settings-audit harness, helpers.ts, mockup) with concrete assertion lists and full code — no "TODO"/"add validation"/"similar to" placeholders. The i18n step names exact keys/values for both locales.

**Type consistency:** `MapOffice{id,name,code,office_type_name,tier,province_name,city_name,address,asset_count,latitude,longitude}`, `OfficeTier='pusat'|'wilayah'|'office'`, `tierMeta`/`TIER_ORDER`, `useOfficeMap().list()` consistent across Tasks 3/4. Backend: `office.CreateInput.{Latitude,Longitude}`, `office.Response.{Latitude,Longitude}`, `MapResponse` JSON keys = frontend `MapOfficeDTO` keys (id,name,code,office_type_name,tier,province_name,city_name,address,asset_count,latitude,longitude); `office.MapList` → `[]sqlc.ListOfficesMapRow`; `tier` serialized `*string` from `*SharedApproverLevel`, mapped to `OfficeTier` via `toTier` (null/`office`/`office_subtree`→`office`). Page guard unchanged `masterdata.office.manage`; map endpoint authMW+scope only. All consistent.
