# Wire Master Data Referensi to the generic reference engine — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the Master Data Referensi screen (11 reference resources) from mock to the real generic reference engine — adding FK pickers (cities→provinces, models→brands), the `vendors` contact/address fields, the `is_active` key rename, and an editable `tier` enum on `office-types` (which makes the office map meaningful).

**Architecture:** Backend extends the generic reference engine with an enum column type (`typeEnum`) and adds `tier` to the `office-types` resource (no migration — the column already exists). Frontend rewrites `useReference` to call the engine over HTTP, enriches the resource descriptor with field types (`text`/`fk`/`select`) + `hasActive`, and rewrites the page to render FK/select pickers, resolve FK names in the table, and gate the active-toggle per resource.

**Tech Stack:** Go 1.25 + Gin + pgx/v5 (generic engine; backend); Nuxt 4 SPA + Nuxt UI + @nuxtjs/i18n + Vitest + Playwright (frontend).

## Global Constraints

- **English DTO keys.** The engine uses the exact backend column names as JSON keys: `name`, `code`, `symbol`, `email`, `phone`, `contact_name`, `address`, `province_id`, `brand_id`, `is_active`, `tier`. The frontend's one rename is **`active` → `is_active`**.
- **List envelope** `{ data, total, limit, offset }` (limit default 20, clamp 1–100). **Single-row** (get/create/update) is a **flat object, no envelope**. Delete → 204.
- Reference write endpoints are gated `authMW + masterdata.global.manage`; reads are `authMW` only. **No data-scope** (global). The page guard stays `definePageMeta({ middleware:'can', permission:'masterdata.global.manage' })`.
- **The descriptor key == the backend path** (e.g. `office-types`, `maintenance-categories`). `useReference` builds `/${key}`.
- **`hasActive` is false for `provinces` and `cities`** (those tables have no `is_active`); the active toggle/column is hidden for them.
- **`tier` is offered as 3 values** `pusat`/`wilayah`/`office` (labels reuse `map.tier.*` → Pusat/Wilayah/Cabang); nullable, not required. Engine validates the value (invalid → 400).
- FK fields (`province_id`, `brand_id`) are **required**; with no province/brand seeded the picker is empty until one is created via the UI.
- All frontend HTTP via `useApiClient().request`; i18n mandatory in BOTH `id.json` + `en.json`; no hardcoded user-facing strings; ESLint no-trailing-commas + 1tbs.
- Backend gates (from `backend/`): `go build ./...`, `go vet ./...`, `go test ./...`, **and `go test -tags=integration ./...`**, plus Spectral lint on `backend/api/openapi.yaml`.
- Frontend gates (from `frontend/`): `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`.
- Don't hand-edit `backend/db/sqlc/` (not touched here — the engine uses raw pgx, not sqlc).

---

### Task 1: Backend — generic engine enum support + `office-types.tier`

**Files:**
- Modify: `backend/internal/masterdata/reference/engine.go` (colType, column, selectExpr, placeholder, coerce)
- Modify: `backend/internal/masterdata/reference/resources.go` (office-types columns)
- Modify: `backend/api/openapi.yaml` (tier on office-types)
- Test: `backend/internal/masterdata/reference/engine_test.go` (NEW, pure coerce unit test)
- Test: `backend/internal/masterdata/reference/reference_integration_test.go` (NEW, //go:build integration)

**Interfaces:**
- Produces: a new `typeEnum` colType; `column` gains `Enum []string` + `EnumType string`. The `office-types` resource now has a `tier` column → `GET/POST/PUT /api/v1/office-types` accept/return `tier` (string `pusat|wilayah|office` or null). Frontend Task 3/4 consume `tier`.

- [ ] **Step 1: Write the failing pure unit test for enum coercion**

Create `backend/internal/masterdata/reference/engine_test.go`:
```go
package reference

import "testing"

func TestCoerceEnum(t *testing.T) {
	r := resource{Columns: []column{
		{Name: "tier", Type: typeEnum, EnumType: "shared.approver_level", Enum: []string{"pusat", "wilayah", "office"}},
	}}

	t.Run("valid value passes through", func(t *testing.T) {
		out, err := coerce(r, map[string]any{"tier": "wilayah"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out[0] != "wilayah" {
			t.Fatalf("got %v, want wilayah", out[0])
		}
	})

	t.Run("invalid value errors", func(t *testing.T) {
		if _, err := coerce(r, map[string]any{"tier": "bogus"}); err == nil {
			t.Fatal("expected error for invalid enum value")
		}
	})

	t.Run("absent maps to nil", func(t *testing.T) {
		out, err := coerce(r, map[string]any{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out[0] != nil {
			t.Fatalf("got %v, want nil", out[0])
		}
	})

	t.Run("empty string maps to nil", func(t *testing.T) {
		out, err := coerce(r, map[string]any{"tier": ""})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out[0] != nil {
			t.Fatalf("got %v, want nil", out[0])
		}
	})
}
```

- [ ] **Step 2: Run to verify it fails**

Run (from `backend/`): `go test ./internal/masterdata/reference/ -run TestCoerceEnum`
Expected: compile error / FAIL — `typeEnum`, `Enum`, `EnumType` undefined.

- [ ] **Step 3: Extend the engine with `typeEnum`**

In `backend/internal/masterdata/reference/engine.go`:

(a) Add `"slices"` to the imports (alongside `"fmt"`, `"strings"`).

(b) Add the enum colType — change the const block (currently `typeText`, `typeBool`, `typeUUID`):
```go
const (
	typeText colType = iota
	typeBool
	typeUUID
	typeEnum
)
```

(c) Extend the `column` struct:
```go
type column struct {
	Name     string   // db column
	Type     colType  // text / bool / uuid / enum
	Required bool     // must be present (non-empty) on write
	Search   bool     // included in the ILIKE search filter
	Default  bool     // default for typeBool when absent
	Enum     []string // allowed values for typeEnum
	EnumType string   // postgres enum type name for the cast, e.g. "shared.approver_level"
}
```

(d) In `selectExpr`, cast enum to text like uuid — change the loop condition:
```go
	for _, c := range r.Columns {
		if c.Type == typeUUID || c.Type == typeEnum {
			parts = append(parts, c.Name+"::text AS "+c.Name)
		} else {
			parts = append(parts, c.Name)
		}
	}
```

(e) Replace `placeholder` (now takes the column so it can cast enums):
```go
func placeholder(n int, c column) string {
	switch c.Type {
	case typeUUID:
		return fmt.Sprintf("$%d::uuid", n)
	case typeEnum:
		return fmt.Sprintf("$%d::%s", n, c.EnumType)
	default:
		return fmt.Sprintf("$%d", n)
	}
}
```
Update the two call sites in `write`:
- create branch: `ph[i] = placeholder(i+1, c.Type)` → `ph[i] = placeholder(i+1, c)`
- update branch: `sets[i] = fmt.Sprintf("%s = %s", c.Name, placeholder(i+2, c.Type))` → `sets[i] = fmt.Sprintf("%s = %s", c.Name, placeholder(i+2, c))`

(f) In `coerce`, add the enum case to the `switch c.Type` (after the `typeBool` case):
```go
		case typeEnum:
			if !present || raw == nil {
				out[i] = nil
				continue
			}
			s, ok := raw.(string)
			if !ok {
				return nil, fmt.Errorf("%s must be a string", c.Name)
			}
			if strings.TrimSpace(s) == "" {
				out[i] = nil
				continue
			}
			if !slices.Contains(c.Enum, s) {
				return nil, fmt.Errorf("%s must be one of %s", c.Name, strings.Join(c.Enum, ", "))
			}
			out[i] = s
```

- [ ] **Step 4: Run the unit test**

Run (from `backend/`): `go test ./internal/masterdata/reference/ -run TestCoerceEnum -v`
Expected: PASS (4 subtests).

- [ ] **Step 5: Add `tier` to the `office-types` resource**

In `backend/internal/masterdata/reference/resources.go`, replace the `office-types` entry (lines 7-10) with:
```go
	{Path: "office-types", Table: "office_types", OrderBy: "name", Columns: []column{
		{Name: "name", Type: typeText, Required: true, Search: true},
		{Name: "tier", Type: typeEnum, EnumType: "shared.approver_level", Enum: []string{"pusat", "wilayah", "office"}},
		{Name: "is_active", Type: typeBool, Default: true},
	}},
```

- [ ] **Step 6: Write the integration round-trip test**

Create `backend/internal/masterdata/reference/reference_integration_test.go`:
```go
//go:build integration

package reference

import (
	"context"
	"testing"

	"github.com/ragbuaj/inventra/internal/testsupport"
)

func officeTypesResource() resource {
	for _, r := range referenceResources {
		if r.Path == "office-types" {
			return r
		}
	}
	panic("office-types resource not found")
}

func TestOfficeTypeTierRoundTrip(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	testsupport.Reset(t, pool)
	e := &engine{pool: pool}
	ctx := context.Background()
	ot := officeTypesResource()

	t.Run("create + update round-trips tier", func(t *testing.T) {
		created, err := e.write(ctx, ot, nil, map[string]any{"name": "Kantor Pusat", "tier": "pusat", "is_active": true})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if created["tier"] != "pusat" {
			t.Fatalf("tier after create = %v, want pusat", created["tier"])
		}
		id := mustParseID(t, created["id"])
		updated, err := e.write(ctx, ot, &id, map[string]any{"name": "Kantor Pusat", "tier": "wilayah", "is_active": true})
		if err != nil {
			t.Fatalf("update: %v", err)
		}
		if updated["tier"] != "wilayah" {
			t.Fatalf("tier after update = %v, want wilayah", updated["tier"])
		}
	})

	t.Run("absent tier stored as null", func(t *testing.T) {
		created, err := e.write(ctx, ot, nil, map[string]any{"name": "Tanpa Tier"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if created["tier"] != nil {
			t.Fatalf("tier = %v, want nil", created["tier"])
		}
	})
}
```
Add the `mustParseID` helper at the bottom of the same file (the engine's `write` takes `*uuid.UUID`):
```go
func mustParseID(t *testing.T, v any) uuid.UUID {
	t.Helper()
	s, ok := v.(string)
	if !ok {
		t.Fatalf("id is not a string: %T", v)
	}
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("parse id: %v", err)
	}
	return id
}
```
and add `"github.com/google/uuid"` to this file's imports.

- [ ] **Step 7: Run build + tests**

Run (from `backend/`):
```bash
go build ./... && go vet ./...
go test ./internal/masterdata/reference/ -run TestCoerceEnum -v
go test -tags=integration ./internal/masterdata/reference/ -run TestOfficeTypeTierRoundTrip -v
go test ./...
```
Expected: build clean; both reference tests PASS; full non-integration suite green. (Integration needs the dev Postgres on :5433, already running.)

- [ ] **Step 8: Update OpenAPI + Spectral**

In `backend/api/openapi.yaml`, find the schema(s) that document the `office-types` reference resource (the reference engine may use a per-resource schema or a shared object — search for `office-types` / `office_types`). Add a `tier` property to the office-types create/update request schema **and** the response schema:
```yaml
        tier:
          type: [string, "null"]
          enum: [pusat, wilayah, office]
          description: Office tier (drives the office-map pin category).
```
(If the reference resources are documented generically with `additionalProperties: true` and no per-resource schema, add a one-line note in the office-types path description that it accepts `tier`; do not invent a structure that isn't there. Report what you found.)

Run (from repo root): `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: no errors.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/masterdata/reference/ backend/api/openapi.yaml
git commit -m "feat(masterdata): add enum column support + editable office-types tier to reference engine"
```

---

### Task 2: Frontend — `useReference` HTTP rewrite + unit test

**Files:**
- Modify (full rewrite): `frontend/app/composables/api/useReference.ts`
- Delete: `frontend/test/unit/reference-mock.spec.ts`
- Test: `frontend/test/unit/use-reference.spec.ts` (NEW)

**Interfaces:**
- Consumes: `useApiClient().request`.
- Produces: `useReference()` → `{ list, create, update, remove }` over `/${key}` (unchanged signatures: `list(key, query)→Paginated<ReferenceRow>`, `create(key, input)`, `update(key, id, input)`, `remove(key, id)`).

- [ ] **Step 1: Write the failing unit test**

Create `frontend/test/unit/use-reference.spec.ts`:
```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'

const request = vi.fn()
vi.mock('~/composables/useApiClient', () => ({ useApiClient: () => ({ request }) }))

// eslint-disable-next-line import/first
import { useReference } from '~/composables/api/useReference'
// eslint-disable-next-line import/first
import { referenceResources } from '~/composables/api/referenceResources'

beforeEach(() => request.mockReset())

describe('useReference', () => {
  it('declares all 11 reference resources (descriptor sanity)', () => {
    expect(referenceResources).toHaveLength(11)
  })

  it('list builds /key query (omits empty search) and returns the envelope', async () => {
    request.mockResolvedValueOnce({ data: [{ id: 'a', name: 'A' }], total: 1, limit: 20, offset: 0 })
    const res = await useReference().list('office-types', { limit: 20, offset: 0 })
    const path = request.mock.calls[0][0] as string
    expect(path).toContain('/office-types?')
    expect(path).toContain('limit=20')
    expect(path).toContain('offset=0')
    expect(path).not.toContain('search=')
    expect(res.total).toBe(1)
  })

  it('list includes search when present', async () => {
    request.mockResolvedValueOnce({ data: [], total: 0, limit: 20, offset: 0 })
    await useReference().list('cities', { search: 'jak', limit: 20, offset: 0 })
    expect(request.mock.calls[0][0]).toContain('search=jak')
  })

  it('create POSTs to /key with the body verbatim (is_active + FK keys)', async () => {
    request.mockResolvedValueOnce({ id: 'c1', name: 'Jakarta' })
    await useReference().create('cities', { province_id: 'p1', name: 'Jakarta', code: '31', is_active: true })
    expect(request).toHaveBeenCalledWith('/cities', { method: 'POST', body: { province_id: 'p1', name: 'Jakarta', code: '31', is_active: true } })
  })

  it('update PUTs to /key/:id', async () => {
    request.mockResolvedValueOnce({ id: 'o1', name: 'KP', tier: 'pusat' })
    await useReference().update('office-types', 'o1', { name: 'KP', tier: 'pusat', is_active: true })
    expect(request).toHaveBeenCalledWith('/office-types/o1', { method: 'PUT', body: { name: 'KP', tier: 'pusat', is_active: true } })
  })

  it('remove DELETEs /key/:id', async () => {
    request.mockResolvedValueOnce(undefined)
    await useReference().remove('brands', 'b1')
    expect(request).toHaveBeenCalledWith('/brands/b1', { method: 'DELETE' })
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `pnpm test -- use-reference`
Expected: FAIL (current `useReference` is mock-backed; paths/bodies don't match).

- [ ] **Step 3: Rewrite `useReference.ts`**

Replace `frontend/app/composables/api/useReference.ts` entirely with:
```ts
import type { ListQuery, Paginated, ReferenceRow } from '~/types'
import type { ReferenceKey } from './referenceResources'

/**
 * Reference master data, wired to the generic engine at /api/v1/<key>.
 * The descriptor key is the backend path. List is server-side search+pagination.
 */
export function useReference() {
  const { request } = useApiClient()

  async function list(key: ReferenceKey, query: ListQuery = {}): Promise<Paginated<ReferenceRow>> {
    const q = new URLSearchParams()
    q.set('limit', String(query.limit ?? 20))
    q.set('offset', String(query.offset ?? 0))
    if (query.search) q.set('search', String(query.search))
    return request<Paginated<ReferenceRow>>(`/${key}?${q.toString()}`)
  }

  async function create(key: ReferenceKey, input: Record<string, unknown>): Promise<ReferenceRow> {
    return request<ReferenceRow>(`/${key}`, { method: 'POST', body: input })
  }

  async function update(key: ReferenceKey, id: string, input: Record<string, unknown>): Promise<ReferenceRow> {
    return request<ReferenceRow>(`/${key}/${id}`, { method: 'PUT', body: input })
  }

  async function remove(key: ReferenceKey, id: string): Promise<void> {
    await request(`/${key}/${id}`, { method: 'DELETE' })
  }

  return { list, create, update, remove }
}
```

- [ ] **Step 4: Delete the mock unit test + run**

Run (from `frontend/`):
```bash
git rm frontend/test/unit/reference-mock.spec.ts
pnpm test -- use-reference
pnpm lint
```
Expected: `use-reference` PASS; lint clean. NOTE: the whole `pnpm test` suite is NOT green yet — `test/nuxt/master-reference.spec.ts` (mock-based) now fails because the page's `useReference` calls real HTTP with no stub. That is EXPECTED and fixed in Task 5. Do NOT touch the page or that test here.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/composables/api/useReference.ts frontend/test/unit/use-reference.spec.ts
git commit -m "feat(reference): wire useReference to the generic engine over HTTP"
```

---

### Task 3: Frontend — descriptor enrichment + `ReferenceRow` rename + i18n

**Files:**
- Modify: `frontend/app/composables/api/referenceResources.ts`
- Modify: `frontend/app/types/index.ts` (`ReferenceRow.active` → `is_active`)
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json`

**Interfaces:**
- Produces: `ReferenceField` gains `type?: 'text'|'fk'|'select'`, `fkResource?: ReferenceKey`, `options?: {value,labelKey}[]`, `required?: boolean`. `ReferenceDescriptor` gains `hasActive: boolean`. `ReferenceRow.is_active?: boolean` (was `active?`). Task 4 consumes these.

- [ ] **Step 1: Rewrite `referenceResources.ts`**

Replace `frontend/app/composables/api/referenceResources.ts` entirely with:
```ts
export type ReferenceKey
  = 'office-types' | 'departments' | 'positions' | 'units'
    | 'maintenance-categories' | 'problem-categories' | 'brands'
    | 'vendors' | 'provinces' | 'cities' | 'models'

export type ReferenceFieldType = 'text' | 'fk' | 'select'

export interface ReferenceFieldOption {
  value: string
  labelKey: string
}

export interface ReferenceField {
  key: string
  labelKey: string
  placeholder?: string
  type?: ReferenceFieldType         // default 'text'
  fkResource?: ReferenceKey         // for type:'fk' — source resource for options + name resolution
  options?: ReferenceFieldOption[]  // for type:'select' — static options
  required?: boolean
}

export interface ReferenceDescriptor {
  key: ReferenceKey
  labelKey: string
  hasActive: boolean                // false for provinces & cities (no is_active column)
  fields: ReferenceField[]
}

const nameField: ReferenceField = { key: 'name', labelKey: 'masterdata.reference.fields.name' }
const codeField: ReferenceField = { key: 'code', labelKey: 'masterdata.reference.fields.code' }

export const referenceResources: ReferenceDescriptor[] = [
  { key: 'office-types', labelKey: 'masterdata.reference.resources.office-types', hasActive: true, fields: [
    nameField,
    { key: 'tier', labelKey: 'masterdata.reference.fields.tier', type: 'select', options: [
      { value: 'pusat', labelKey: 'map.tier.pusat' },
      { value: 'wilayah', labelKey: 'map.tier.wilayah' },
      { value: 'office', labelKey: 'map.tier.office' }
    ] }
  ] },
  { key: 'departments', labelKey: 'masterdata.reference.resources.departments', hasActive: true, fields: [nameField] },
  { key: 'positions', labelKey: 'masterdata.reference.resources.positions', hasActive: true, fields: [nameField] },
  { key: 'units', labelKey: 'masterdata.reference.resources.units', hasActive: true, fields: [nameField, { key: 'symbol', labelKey: 'masterdata.reference.fields.symbol' }] },
  { key: 'maintenance-categories', labelKey: 'masterdata.reference.resources.maintenance-categories', hasActive: true, fields: [nameField] },
  { key: 'problem-categories', labelKey: 'masterdata.reference.resources.problem-categories', hasActive: true, fields: [nameField] },
  { key: 'brands', labelKey: 'masterdata.reference.resources.brands', hasActive: true, fields: [nameField] },
  { key: 'vendors', labelKey: 'masterdata.reference.resources.vendors', hasActive: true, fields: [
    nameField,
    { key: 'contact_name', labelKey: 'masterdata.reference.fields.contact_name' },
    { key: 'phone', labelKey: 'masterdata.reference.fields.phone' },
    { key: 'email', labelKey: 'masterdata.reference.fields.email' },
    { key: 'address', labelKey: 'masterdata.reference.fields.address' }
  ] },
  { key: 'provinces', labelKey: 'masterdata.reference.resources.provinces', hasActive: false, fields: [nameField, codeField] },
  { key: 'cities', labelKey: 'masterdata.reference.resources.cities', hasActive: false, fields: [
    { key: 'province_id', labelKey: 'masterdata.reference.fields.province', type: 'fk', fkResource: 'provinces', required: true },
    nameField,
    codeField
  ] },
  { key: 'models', labelKey: 'masterdata.reference.resources.models', hasActive: true, fields: [
    { key: 'brand_id', labelKey: 'masterdata.reference.fields.brand', type: 'fk', fkResource: 'brands', required: true },
    nameField
  ] }
]
```

- [ ] **Step 2: Rename `ReferenceRow.active` → `is_active`**

In `frontend/app/types/index.ts`, change the `ReferenceRow` interface (lines 139-145):
```ts
export interface ReferenceRow {
  id: string
  name: string
  code?: string
  is_active?: boolean
  [key: string]: unknown
}
```
(The `[key: string]: unknown` index already accommodates `province_id`/`brand_id`/`tier`.)

- [ ] **Step 3: Add i18n field keys (both locales)**

Read the `masterdata.reference.fields` object in `frontend/i18n/locales/id.json` and `en.json`. Add `province`, `brand`, `contact_name`, `address`, `tier` to BOTH (keep existing keys, no trailing commas):
- id: `"province": "Provinsi"`, `"brand": "Brand"`, `"contact_name": "Nama Kontak"`, `"address": "Alamat"`, `"tier": "Tingkat"`
- en: `"province": "Province"`, `"brand": "Brand"`, `"contact_name": "Contact name"`, `"address": "Address"`, `"tier": "Tier"`
(The tier OPTION labels reuse the existing `map.tier.{pusat,wilayah,office}` keys — do not re-add those.)

- [ ] **Step 4: Verify build/lint**

Run (from `frontend/`): `pnpm lint`
Expected: lint clean. NOTE: `pnpm typecheck` will now FAIL in `pages/master/reference.vue` (it reads `row.active`/`form.active`, and the descriptor lacks the new branches) — EXPECTED, fixed in Task 4. `pnpm test` for `use-reference` (Task 2) still passes.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/composables/api/referenceResources.ts frontend/app/types/index.ts frontend/i18n/locales/id.json frontend/i18n/locales/en.json
git commit -m "feat(reference): enrich descriptor (fk/select/hasActive) + is_active rename + i18n"
```

---

### Task 4: Frontend — rewrite `reference.vue` (FK/select pickers, FK name resolution, hasActive gating)

**Files:**
- Modify (script + template): `frontend/app/pages/master/reference.vue`

**Interfaces:**
- Consumes: `useReference` (Task 2) + the enriched descriptor + `ReferenceRow.is_active` (Task 3).

- [ ] **Step 1: Rewrite the `<script setup>`**

Replace the `<script setup lang="ts">` block of `frontend/app/pages/master/reference.vue` with:
```ts
import type { ReferenceRow, RowAction, TableSorting } from '~/types'
import type { ReferenceKey, ReferenceDescriptor, ReferenceField } from '~/composables/api/referenceResources'
import { referenceResources } from '~/composables/api/referenceResources'

definePageMeta({ middleware: 'can', permission: 'masterdata.global.manage' })

const { t } = useI18n()
const toast = useToast()
const can = useCan()
const { open: confirm } = useConfirm()
const api = useReference()

const resourceKey = ref<ReferenceKey>(referenceResources[0]!.key)
const descriptor = computed<ReferenceDescriptor>(() =>
  referenceResources.find(r => r.key === resourceKey.value) ?? referenceResources[0]!
)

const entityCounts = ref<Partial<Record<ReferenceKey, number>>>({})

const rows = ref<ReferenceRow[]>([])
const total = ref(0)
const limit = ref(20)
const offset = ref(0)
const search = ref('')
const sorting = ref<TableSorting>([])
const loading = ref(true)

// FK option data, keyed by the FK field key (e.g. 'province_id' → province rows).
// Used for BOTH the form picker and the table name resolution.
const fkData = ref<Record<string, { id: string, name: string }[]>>({})

const formOpen = ref(false)
const saving = ref(false)
const editingId = ref<string>()
const form = reactive<Record<string, unknown>>({ is_active: true })

const columns = computed(() => {
  const cols = descriptor.value.fields.map(f => ({ accessorKey: f.key, header: t(f.labelKey), sortable: true }))
  if (descriptor.value.hasActive) {
    cols.push({ accessorKey: 'is_active', header: t('masterdata.reference.statusColumn'), sortable: true })
  }
  return cols
})

// Items for a fk/select field's USelect ({ label, value }).
function fieldSelectItems(field: ReferenceField): { label: string, value: string }[] {
  if (field.type === 'fk') return (fkData.value[field.key] ?? []).map(o => ({ label: o.name, value: o.id }))
  if (field.type === 'select') return (field.options ?? []).map(o => ({ label: t(o.labelKey), value: o.value }))
  return []
}

// Resolve a FK id to its display name for the table cell.
function fkName(fieldKey: string, id: unknown): string {
  const found = (fkData.value[fieldKey] ?? []).find(o => o.id === id)
  return found?.name ?? '—'
}

// Resolve the tier enum value to its i18n label for the table cell.
function tierLabel(value: unknown): string {
  const field = descriptor.value.fields.find(f => f.key === 'tier')
  const opt = field?.options?.find(o => o.value === value)
  return opt ? t(opt.labelKey) : '—'
}

async function refresh() {
  loading.value = true
  try {
    const res = await api.list(resourceKey.value, { search: search.value, limit: limit.value, offset: offset.value })
    rows.value = res.data
    total.value = res.total
  } finally {
    loading.value = false
  }
}

async function loadFkOptions() {
  const next: Record<string, { id: string, name: string }[]> = {}
  for (const f of descriptor.value.fields) {
    if (f.type === 'fk' && f.fkResource) {
      const res = await api.list(f.fkResource, { limit: 100 })
      next[f.key] = res.data.map(r => ({ id: r.id, name: r.name }))
    }
  }
  fkData.value = next
}

async function fetchAllCounts() {
  const entries = await Promise.all(
    referenceResources.map(async (r) => {
      const res = await api.list(r.key, { limit: 1 })
      return [r.key, res.total] as [ReferenceKey, number]
    })
  )
  entityCounts.value = Object.fromEntries(entries)
}

function resetForm() {
  // eslint-disable-next-line @typescript-eslint/no-dynamic-delete
  for (const k of Object.keys(form)) delete form[k]
  for (const f of descriptor.value.fields) form[f.key] = ''
  form.is_active = true
}

function openCreate() {
  editingId.value = undefined
  resetForm()
  formOpen.value = true
}

function openEdit(row: ReferenceRow) {
  editingId.value = row.id
  resetForm()
  for (const f of descriptor.value.fields) form[f.key] = row[f.key] ?? ''
  form.is_active = row.is_active !== false
  formOpen.value = true
}

function validate(): boolean {
  for (const f of descriptor.value.fields) {
    if (f.required && !String(form[f.key] ?? '').trim()) {
      toast.add({ title: t('masterdata.reference.requiredField', { field: t(f.labelKey) }), color: 'error' })
      return false
    }
  }
  return true
}

async function onSubmit() {
  if (!validate()) return
  saving.value = true
  try {
    if (editingId.value) {
      await api.update(resourceKey.value, editingId.value, { ...form })
    } else {
      await api.create(resourceKey.value, { ...form })
    }
    formOpen.value = false
    await Promise.all([refresh(), fetchAllCounts()])
  } catch { /* useApiClient surfaces the error toast */ }
  finally {
    saving.value = false
  }
}

async function onDelete(row: ReferenceRow) {
  const ok = await confirm({
    title: t('common.delete'),
    description: t('masterdata.reference.deleteConfirm', { name: row.name })
  })
  if (!ok) return
  try {
    await api.remove(resourceKey.value, row.id)
    await Promise.all([refresh(), fetchAllCounts()])
  } catch { /* useApiClient surfaces the error toast */ }
}

function rowActions(row: Record<string, unknown>): RowAction[] {
  if (!can('masterdata.global.manage')) return []
  const r = row as unknown as ReferenceRow
  return [
    { label: t('common.edit'), icon: 'i-lucide-pencil', onSelect: () => openEdit(r) },
    { label: t('common.delete'), icon: 'i-lucide-trash-2', color: 'error', separator: true, onSelect: () => onDelete(r) }
  ]
}

async function toggleActive(row: ReferenceRow) {
  const prev = row.is_active !== false
  row.is_active = !prev
  try {
    await api.update(resourceKey.value, row.id, { is_active: !prev })
  } catch {
    row.is_active = prev
  }
}

function selectResource(key: ReferenceKey) {
  resourceKey.value = key
}

watch(resourceKey, async () => {
  offset.value = 0
  search.value = ''
  await Promise.all([refresh(), loadFkOptions()])
})
watch([search, offset], refresh)
onMounted(async () => {
  await Promise.all([refresh(), loadFkOptions(), fetchAllCounts()])
})
```

- [ ] **Step 2: Update the template**

Apply these template edits to `frontend/app/pages/master/reference.vue`:

(a) **ResourceTable cell slots** — replace the single `#active-cell` slot block with the renamed `#is_active-cell` plus the three resolved-value slots. Inside `<ResourceTable …>`:
```vue
          <!-- FK name + tier label + status cells -->
          <template #province_id-cell="{ row }">
            {{ fkName('province_id', (row as Record<string, unknown>).province_id) }}
          </template>
          <template #brand_id-cell="{ row }">
            {{ fkName('brand_id', (row as Record<string, unknown>).brand_id) }}
          </template>
          <template #tier-cell="{ row }">
            {{ tierLabel((row as Record<string, unknown>).tier) }}
          </template>
          <template #is_active-cell="{ row }">
            <button
              class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full border cursor-pointer transition-colors text-[11.5px] font-semibold"
              :class="(row as unknown as ReferenceRow).is_active !== false
                ? 'border-success/30 bg-success/10 text-success'
                : 'border-muted text-muted bg-muted'"
              @click="toggleActive(row as unknown as ReferenceRow)"
            >
              <span>
                {{ (row as unknown as ReferenceRow).is_active !== false
                  ? t('masterdata.reference.aktif')
                  : t('masterdata.reference.nonaktif') }}
              </span>
              <span
                class="relative flex-none w-7 h-[17px] rounded-full transition-colors"
                :class="(row as unknown as ReferenceRow).is_active !== false ? 'bg-success' : 'bg-muted'"
              >
                <span
                  class="absolute top-0.5 w-[13px] h-[13px] rounded-full bg-default shadow-sm transition-all"
                  :class="(row as unknown as ReferenceRow).is_active !== false ? 'left-[13px]' : 'left-0.5'"
                />
              </span>
            </button>
          </template>
```

(b) **FormModal field loop** — replace the `<UFormField v-for…>` + `<UInput>` block with a per-type branch, and gate the active toggle on `hasActive`:
```vue
      <div class="space-y-4">
        <UFormField
          v-for="field in descriptor.fields"
          :key="field.key"
          :label="t(field.labelKey)"
        >
          <USelect
            v-if="field.type === 'fk' || field.type === 'select'"
            :model-value="form[field.key] as string"
            :items="fieldSelectItems(field)"
            class="w-full"
            @update:model-value="form[field.key] = $event"
          />
          <UInput
            v-else
            :model-value="form[field.key] as string"
            class="w-full"
            @update:model-value="form[field.key] = $event"
          />
          <p
            v-if="field.type === 'fk' && fieldSelectItems(field).length === 0"
            class="text-xs text-warning mt-1"
          >
            {{ t('masterdata.reference.fkEmpty') }}
          </p>
        </UFormField>

        <!-- Aktif toggle row (only for resources that have is_active) -->
        <label
          v-if="descriptor.hasActive"
          class="flex items-center justify-between gap-2.5 px-[13px] py-[11px] rounded-[11px] bg-muted cursor-pointer"
        >
          <span>
            <span class="block text-[13.5px] font-semibold text-default">
              {{ t('masterdata.reference.aktif') }}
            </span>
            <span class="block text-xs text-muted">
              {{ t('masterdata.reference.aktifHint') }}
            </span>
          </span>
          <USwitch
            :model-value="form.is_active as boolean"
            @update:model-value="form.is_active = $event"
          />
        </label>
      </div>
```

- [ ] **Step 3: Add the two new i18n keys (both locales)**

The script references `masterdata.reference.requiredField` (with a `{field}` param) and `masterdata.reference.fkEmpty`. Add both under `masterdata.reference` in `frontend/i18n/locales/id.json` and `en.json`:
- id: `"requiredField": "{field} wajib diisi."`, `"fkEmpty": "Belum ada data — buat dulu di resource terkait."`
- en: `"requiredField": "{field} is required."`, `"fkEmpty": "No options yet — create one in the related resource first."`

- [ ] **Step 4: Verify lint + typecheck**

Run (from `frontend/`): `pnpm lint && pnpm typecheck`
Expected: both exit 0. NOTE: `pnpm test` is still red on `test/nuxt/master-reference.spec.ts` (mock-based) — fixed in Task 5.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/pages/master/reference.vue frontend/i18n/locales/id.json frontend/i18n/locales/en.json
git commit -m "feat(reference): page renders fk/select pickers, resolves fk names, gates active toggle"
```

---

### Task 5: Frontend — rewrite the component test

**Files:**
- Modify (full rewrite — the file EXISTS as a mock-based test): `frontend/test/nuxt/master-reference.spec.ts`

**Interfaces:**
- Consumes: the wired page + composable; mocks `~/composables/useApiClient`.

- [ ] **Step 1: Study the harness**

Read the CURRENT `frontend/test/nuxt/master-reference.spec.ts` (to be overwritten) AND a wired-screen test that mocks `useApiClient` with per-path routing + `useAuthStore().setSession(token, user, ['*'])` + `mountSuspended` (e.g. `frontend/test/nuxt/settings-users.spec.ts`). Read `frontend/app/pages/master/reference.vue` + `frontend/app/composables/api/referenceResources.ts` so your stub responses match the real calls. On mount the page calls: `list(<resource>)` for the active resource, `list(<fkResource>)` for any FK field, and `list(key,{limit:1})` ×11 for the sidebar counts.

- [ ] **Step 2: Write the rewritten test**

Overwrite `frontend/test/nuxt/master-reference.spec.ts` entirely. Mock `~/composables/useApiClient` with a `request(path, opts)` handler that routes by path + method and **captures** create/update bodies. Use `// @vitest-environment nuxt`. Set an admin session with `['*']`. Cover (assert REAL rendered text / captured bodies — no hollow checks):
- **Default load:** office-types rows render; the sidebar lists 11 resources with counts.
- **FK name resolution:** switch to `cities` (set `resourceKey` via `wrapper.vm` if the sidebar button click won't propagate) with a stubbed `cities` row `{id,name,province_id:'p1'}` and stubbed `provinces` `[{id:'p1',name:'DKI Jakarta'}]` → the city row cell shows **"DKI Jakarta"**, NOT `p1`.
- **FK picker + create:** on `cities`, open create, set `form.province_id='p1'` + `form.name='Bekasi'` (via `wrapper.vm`), submit → captured `POST /cities` body includes `province_id:'p1'` and `name:'Bekasi'`.
- **Required FK guard:** on `cities`, open create, leave `province_id` empty, submit → NO `POST` is sent (validate blocks it).
- **Select (tier):** on `office-types`, open create, set `form.tier='pusat'`, submit → captured `POST /office-types` body has `tier:'pusat'`.
- **hasActive gating:** the `is_active` column/toggle renders for `brands` but NOT for `provinces`/`cities` (assert the status column header text is absent when on `cities`).
- **Delete + search:** delete (confirm) → `DELETE /<key>/<id>`; typing search → `list` called with `search`.

Mirror the established `wrapper.vm as unknown as { … }` ref-mutation technique used by the other wired component tests (USelect clicks don't propagate in jsdom).

- [ ] **Step 3: Run the target test + full suite**

Run (from `frontend/`): `pnpm test -- master-reference` then `pnpm test`
Expected: target PASS; whole suite green.

- [ ] **Step 4: Commit**

```bash
git add frontend/test/nuxt/master-reference.spec.ts
git commit -m "test(reference): component test against the stubbed generic engine"
```

---

### Task 6: E2E + delete mock + mockup + PROGRESS + full gate

**Files:**
- Create: `frontend/e2e/master-reference.spec.ts`
- Delete: `frontend/app/mock/reference.ts` + its barrel export
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Delete the orphaned mock + barrel line**

Run (from repo root): `grep -rn "mock/reference" frontend/app frontend/test` (exclude the file itself). After Tasks 2-5 the only importer was `useReference.ts` (rewritten) + `reference-mock.spec.ts` (deleted). If ZERO importers remain, `git rm frontend/app/mock/reference.ts` and remove the `export * from './reference'` line from `frontend/app/mock/index.ts`. If anything still imports it, report and fix. Run `pnpm typecheck` to confirm no dangling import.

- [ ] **Step 2: Write the e2e**

Read `frontend/e2e/helpers.ts` (`login()`) + an existing wired e2e (`frontend/e2e/settings.spec.ts`) for the robust-locator style. Create `frontend/e2e/master-reference.spec.ts`: login, goto `/master/reference`. Cover deterministically against the real backend (seeded admin has `masterdata.global.manage`):
- The sidebar renders (the 11 resource labels — assert a couple via exact text from i18n `masterdata.reference.resources.*`).
- Create a **province** (select `provinces` in the sidebar, click Add, fill name, submit) and assert the new province row appears.
- Switch to `cities`, click Add, and assert the **province picker** is present and contains the province just created (open the USelect via trigger-click + `role="option"` — NEVER `selectOption`); pick it, fill the city name, submit; assert the city row shows the province NAME.
ROBUST locators only: text/role + `data-testid` where added; NO `selectOption`, NO `isVisible()` snapshot booleans driving control flow, NO silent `if(...)return`, NO `.first()`/`.last()` on broad `div`/`button` filters, NO `getByText(..., {exact:false})` that can match multiple elements (use `{exact:true}` for short labels). You likely CANNOT run `pnpm test:e2e` here (needs full stack); ensure it compiles + lints; CI runs it. State that in your report.

- [ ] **Step 3: Mockup fidelity comparison**

Read `docs/design/Master Data Referensi.dc.html` and the built `frontend/app/pages/master/reference.vue`. Verify the sidebar (11 resources + counts), header + Add, search, table, and FormModal match. The FK pickers (province/brand), the `tier` select on office-types, and the `vendors` contact_name/address fields are backend-supported additions (not deviations). APPROVED deviation: the `is_active` toggle/column is hidden for `provinces`/`cities` (those tables lack the column). Fix any OTHER genuine deviation; report the result.

- [ ] **Step 4: Update PROGRESS.md**

In `docs/PROGRESS.md`: mark Master Data Referensi ✅ wired to the generic engine (11 resources; FK pickers cities→provinces & models→brands; vendors contact_name/address; office-types **tier** editable; `is_active` rename). Note that the **office map is now meaningful** (office-type tier can be set → real Pusat/Wilayah/Cabang pins). Add a TODO note: cities/models need at least one province/brand created first (no seed). Refresh "▶ Next session — start here" → next master-data sub-project = **Kategori Aset** (rich DTO + tree). Don't fabricate status for unrelated screens.

- [ ] **Step 5: Full gate (backend + frontend)**

Run (from `backend/`): `go build ./... && go vet ./... && go test ./...`, then `go test -tags=integration ./...`, then `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml` (adjust the ruleset path to repo-root `.spectral.yaml`).
Run (from `frontend/`): `pnpm lint && pnpm typecheck && pnpm test && pnpm build`.
Expected: all green. (Integration needs dev Postgres up. E2E runs in CI.)

- [ ] **Step 6: Commit**

```bash
git add frontend/e2e/master-reference.spec.ts frontend/app/mock/index.ts docs/PROGRESS.md
git rm frontend/app/mock/reference.ts
git commit -m "test(reference): e2e + drop mock; wire Master Data Referensi end-to-end"
```

---

## Self-Review

**Spec coverage:**
- bagian 2.1 engine `typeEnum` (Enum/EnumType, placeholder cast, selectExpr text-cast, coerce validation) → Task 1. ✓
- bagian 2.2 office-types `tier` column → Task 1. ✓
- bagian 2.3 openapi + engine tests (valid/invalid/null) → Task 1 (pure coerce unit + integration round-trip). ✓
- bagian 3.1 `useReference` HTTP rewrite → Task 2. ✓
- bagian 3.2 descriptor enrichment (type/fk/select/hasActive; cities/models/vendors/office-types) → Task 3. ✓
- bagian 3.3 page (is_active rename, FK/select form rendering, FK name resolution, hasActive gating, required validation) → Task 4. ✓
- bagian 3.4 `ReferenceRow` rename → Task 3. ✓
- bagian 3.5 i18n (province/brand/contact_name/address/tier + requiredField/fkEmpty) → Task 3 + Task 4. ✓
- bagian 3.6 delete mock → Task 6. ✓
- bagian 4 tests (engine, unit, component, e2e) → Tasks 1/2/5/6. ✓
- bagian 5 done (delete mock+barrel, mockup, PROGRESS, gate) → Task 6. ✓
- bagian 6 risks (NULL::enum cast, invalid→400, empty FK picker, ReferenceRow cross-key) → handled in Tasks 1/3/4.

**Placeholder scan:** Tasks 5 & 6 give explicit assertion lists + "read X first" pointers (harness, helpers, mockup) with concrete scenarios; no "TODO"/"add validation"/"similar to". OpenAPI step (Task 1 Step 8) names the exact YAML to add and tells the implementer to report the structure found (the only spot that can't be fully pre-specified without the file open).

**Type consistency:** `ReferenceField{key,labelKey,placeholder?,type?,fkResource?,options?,required?}`, `ReferenceDescriptor{key,labelKey,hasActive,fields}`, `ReferenceRow.is_active?`, `useReference().{list,create,update,remove}` over `/${key}` — consistent across Tasks 2/3/4/5. Backend `column{…,Enum,EnumType}` + `typeEnum` + `placeholder(n,column)` consistent within Task 1. Page helpers `fieldSelectItems`/`fkName`/`tierLabel`/`fkData`/`loadFkOptions` all defined in Task 4's script and used in its template. The descriptor key (`office-types`, …) equals the backend path throughout.
