# Depreciation Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dual-basis depreciation (commercial PSAK 16 + fiscal PMK 72/2023): pure calculation engine filling the existing `depreciation.depreciation_entries` read model, a period state machine (open→computed→closed) with manual run per ADR-0010 stage 1, the `/depreciation` screen 1:1 from its mockup, journal exports (xlsx+PDF), and the deferred integrations (server-computed disposal book value + approval-amount basis, fiscal values on the Disposal screen, the Depreciation tab on Asset Detail).

**Architecture:** `internal/depreciation` follows ADR-0008 with an extra `engine.go` of pure functions (no DB) carrying all calculation rules — iterative month-walk that makes estimate changes and impairment prospective *by construction*. `ComputePeriod` is idempotent, wrapped in a Postgres advisory lock, regenerates all non-closed entries from the global closed watermark, and maintains the commercial summary columns on `asset.assets`. Disposal gains a dependency on the depreciation service to compute `book_value_at_disposal` server-side (maker input removed from the contract) and to base the approval amount on book value.

**Tech Stack:** Go 1.25 + Gin + sqlc (pgx/v5) + `math/big.Rat` + `go-pdf/fpdf` (existing) + **`github.com/xuri/excelize/v2` (new dependency)**; Nuxt 4 + Nuxt UI + Vitest + Playwright.

**Spec (read first):** `docs/superpowers/specs/2026-07-05-depreciation-module-design.md` · ADR: `docs/adr/0010-background-job-execution.md`

## Global Constraints

- Branch `feat/depreciation-module` (exists; spec+ADR committed). Conventional Commits (`feat(depreciation): …`, `feat(db): …`, `fix(disposal): …`); **NEVER add AI attribution**.
- Never hand-edit `backend/db/sqlc/`; `sqlc generate` from `backend/`. Migration `000023` (up+down, soft-delete/trigger conventions).
- ADR-0008 split; scope enforced read+write; money as decimal strings; `big.Rat` half-up 2dp for arithmetic.
- Frontend: ESLint `commaDangle: 'never'`, 1tbs; i18n id/en parity; `U*` components; `useApiClient`.
- Mockup fidelity 1:1 to `docs/design/Depresiasi.dc.html` except spec bagian 6 deviations (a)–(e).
- Verify per task: backend `go build ./... ; go vet ./... ; go test ./...` (+ `-tags=integration` for touched pkg); frontend `pnpm lint ; pnpm typecheck ; pnpm test`.
- Local e2e: Docker stack + `RATELIMIT_ENABLED=false` (already in backend/.env). Migration 000023 must be applied to the dev DB before live checks (`docker compose -f docker-compose.dev.yml --profile app up -d migrate backend` after building).

**Domain rules quick-reference (engine — normative, from spec bagian 2):**
- Start month = month of `purchase_date` (full-month). Skip+report: `capitalized=false`, missing cost/date, unresolved commercial params, status `disposed`. `excluded_from_valuation` still depreciates. Intangible = same path.
- Commercial: params = asset override ?? category default (`salvage = asset.salvage_value ?? round(cost × default_salvage_rate)`); straight line monthly = `(opening − salvage)/remaining_months`; declining = `opening × (2/lifeYears)/12` floored at salvage; last month absorbs rounding so final closing == salvage exactly. Fully-depreciated ⇒ no new entries.
- Fiscal (PMK 72/2023, **no salvage**): params from `fiscal_group` constant table — kelompok_1 48m (SL 25%/DB 50%), kelompok_2 96m (12.5%/25%), kelompok_3 192m (6.25%/12.5%), kelompok_4 240m (5%/10%), bangunan_permanen 240m SL-only, bangunan_non_permanen 120m SL-only, non_susut ⇒ no fiscal entries. Method follows the asset's commercial method when valid for the group, else straight line; DB final month absorbs the whole remainder (disusutkan sekaligus). SL fiscal monthly = `(opening − 0)/remaining_months` (equivalent to cost/life with catch-up-safe form).
- Impairment: commercial only; `impairment_loss += (book_value − recoverable)`, `book_value = recoverable`; engine's iterative form spreads the new base prospectively.

---

### Task 1: Migration 000023 + permission catalog

**Files:**
- Create: `backend/db/migrations/000023_depreciation_periods.up.sql`, `.down.sql`
- Modify: `backend/internal/authzadmin/catalog.go`
- Test: `backend/internal/authzadmin/catalog_test.go` (extend or create)

**Interfaces:**
- Produces: table `depreciation.depreciation_periods` + enum `shared.depreciation_period_status`; index `idx_depr_basis_period`; permissions `depreciation.view`/`depreciation.manage` seeded (Superadmin, Manager? NO — **Superadmin only** per PRD bagian 2.1) + data-scope module `depreciation`; app_settings key `depreciation.accumulated_gl_account`; catalog group "Penyusutan Aset". sqlc model `DepreciationDepreciationPeriod`.

- [ ] **Step 1: Write the failing catalog test**

Append to (or create) `backend/internal/authzadmin/catalog_test.go`:

```go
func TestCatalog_DepreciationPermissions(t *testing.T) {
	if !IsKnownPermission("depreciation.view") {
		t.Fatal("depreciation.view must be a known permission")
	}
	if !IsKnownPermission("depreciation.manage") {
		t.Fatal("depreciation.manage must be a known permission")
	}
	found := false
	for _, m := range ScopeModules() {
		if m == "depreciation" {
			found = true
		}
	}
	if !found {
		t.Fatal("scope module 'depreciation' missing")
	}
	// The key must not be duplicated (it used to live in the Cadangan group).
	count := 0
	for _, g := range permissionCatalog {
		for _, it := range g.Items {
			if it.Key == "depreciation.manage" {
				count++
			}
		}
	}
	if count != 1 {
		t.Fatalf("depreciation.manage appears %d times, want 1", count)
	}
}
```

Run: `go test ./internal/authzadmin/ -run TestCatalog_Depreciation` → FAIL (view unknown / module missing). 

- [ ] **Step 2: Update catalog.go**

In `permissionCatalog`: REMOVE `{"depreciation.manage", "Kelola penyusutan"}` from the `Cadangan` group and add a new group after "Penghapusan Aset":

```go
	{Group: "Penyusutan Aset", Items: []PermissionItem{
		{"depreciation.view", "Lihat depresiasi"},
		{"depreciation.manage", "Jalankan & tutup periode depresiasi, catat impairment"},
	}},
```

In `ScopeModules()` add `"transfers", "disposals", "depreciation"` if missing (check current list — it lacks transfers/disposals too; add all three so the Data Scope screen can configure them; keep order stable with `*` first).

- [ ] **Step 3: Write the migration**

`backend/db/migrations/000023_depreciation_periods.up.sql`:

```sql
-- Depreciation period state machine + module seeds. Spec:
-- docs/superpowers/specs/2026-07-05-depreciation-module-design.md · ADR-0010 stage 1.

CREATE TYPE shared.depreciation_period_status AS ENUM ('open', 'computed', 'closed');

CREATE TABLE depreciation.depreciation_periods (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  period        date NOT NULL,
  status        shared.depreciation_period_status NOT NULL DEFAULT 'open',
  computed_at   timestamptz,
  computed_by   uuid REFERENCES identity.users (id),
  closed_at     timestamptz,
  closed_by     uuid REFERENCES identity.users (id),
  asset_count   int NOT NULL DEFAULT 0,
  total_amount  numeric(18,2) NOT NULL DEFAULT 0,
  skipped_count int NOT NULL DEFAULT 0,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz
);
CREATE UNIQUE INDEX uq_depr_period ON depreciation.depreciation_periods (period) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_depr_periods_set_updated BEFORE UPDATE ON depreciation.depreciation_periods
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();

CREATE INDEX idx_depr_basis_period ON depreciation.depreciation_entries (basis, period);

-- Journal credit account (global; Superadmin-editable later via app_settings CRUD).
INSERT INTO identity.app_settings (key, value, value_type, description)
SELECT 'depreciation.accumulated_gl_account', '1.2.9.001', 'string',
       'GL account credited by the depreciation journal (Akumulasi Penyusutan) — placeholder, confirm with bank COA'
WHERE NOT EXISTS (SELECT 1 FROM identity.app_settings WHERE key = 'depreciation.accumulated_gl_account' AND deleted_at IS NULL);

-- Permissions: Superadmin ONLY (PRD bagian 2.1: konfigurasi & jalankan depresiasi).
INSERT INTO identity.role_permissions (role_id, permission_key)
SELECT r.id, p.key
FROM identity.roles r
CROSS JOIN (VALUES ('depreciation.view'), ('depreciation.manage')) AS p(key)
WHERE r.deleted_at IS NULL AND r.name = 'Superadmin'
ON CONFLICT DO NOTHING;

-- Data-scope for module 'depreciation' (mirror 000021 pattern).
INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
SELECT r.id, 'depreciation', (CASE
    WHEN r.name = 'Superadmin'                                 THEN 'global'
    WHEN r.name IN ('Kepala Kanwil', 'Kepala Unit', 'Manager') THEN 'office_subtree'
    ELSE 'office'
  END)::shared.scope_level
FROM identity.roles r
WHERE r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
```

`.down.sql`:

```sql
DELETE FROM identity.data_scope_policies WHERE module = 'depreciation';
DELETE FROM identity.role_permissions WHERE permission_key IN ('depreciation.view', 'depreciation.manage');
DELETE FROM identity.app_settings WHERE key = 'depreciation.accumulated_gl_account';
DROP INDEX IF EXISTS depreciation.idx_depr_basis_period;
DROP TABLE IF EXISTS depreciation.depreciation_periods;
DROP TYPE IF EXISTS shared.depreciation_period_status;
```

- [ ] **Step 4: Generate + verify**

From `backend/`: `sqlc generate` (model appears) → `go build ./... ; go vet ./... ; go test ./...` all green (catalog test now PASS). Any pre-existing integration run (e.g. `go test -tags=integration ./internal/authzadmin/` if it exists) stays green — testsupport applies the new migration automatically.

- [ ] **Step 5: Commit**

```bash
git add backend/db/migrations/000023_depreciation_periods.up.sql backend/db/migrations/000023_depreciation_periods.down.sql backend/db/sqlc backend/internal/authzadmin
git commit -m "feat(db): depreciation periods state machine, permissions and journal account seed (migration 000023)"
```

---

### Task 2: Pure calculation engine (`engine.go`) — the heaviest TDD task

**Files:**
- Create: `backend/internal/depreciation/engine.go`
- Test: `backend/internal/depreciation/engine_test.go`

**Interfaces (produced — Tasks 4/7 consume these EXACT signatures):**

```go
package depreciation

// Params are fully-resolved calculation inputs for one (asset, basis).
type Params struct {
	Method      sqlc.SharedDepreciationMethod
	LifeMonths  int32
	Cost        string    // decimal string
	Salvage     string    // "0" for fiscal
	Start       time.Time // first day of purchase month (UTC)
	FinalAbsorb bool      // true for fiscal declining_balance: last month absorbs remainder
}

// Entry is one generated month.
type Entry struct {
	Period  time.Time // first day of month
	Opening string
	Amount  string
	Closing string
	Method  sqlc.SharedDepreciationMethod
}

// Skip explains why an asset produces no entries for a basis.
type Skip struct{ Reason string } // reasons: "not_capitalized","no_cost","no_purchase_date","missing_params","disposed","non_susut","building_requires_straight_line"

// Walk generates the months AFTER lastPeriod (nil ⇒ from p.Start) through target inclusive.
// lastClosing (nil ⇒ p.Cost) is the opening for the first generated month. Returns nil
// (no error) when the asset is already fully depreciated or target < first month.
func Walk(p Params, lastPeriod *time.Time, lastClosing *string, target time.Time) ([]Entry, error)

// ResolveCommercial / ResolveFiscal resolve Params from asset+category (asset override
// wins; category default fallback). A non-nil *Skip means "no entries, report reason".
func ResolveCommercial(a sqlc.AssetAsset, c sqlc.MasterdataCategory) (*Params, *Skip)
func ResolveFiscal(a sqlc.AssetAsset, c sqlc.MasterdataCategory) (*Params, *Skip)

// FiscalRule is the verified PMK 72/2023 parameter table (exported for tests/UI docs).
type FiscalRule struct {
	LifeMonths       int32
	StraightLinePct  string // annual, e.g. "25"
	DecliningPct     string // annual, "" = declining not allowed (buildings)
}
var FiscalRules = map[sqlc.SharedFiscalAssetGroup]FiscalRule{ /* kelompok_1..bangunan_non_permanen per quick-reference */ }

// roundHalfUp2 exposed for reuse: *big.Rat → decimal string, 2dp, half-up.
func roundHalfUp2(r *big.Rat) string
```

Walk internals (normative): iterate month by month; `remaining = LifeMonths − monthsElapsed(Start, period)`; straight line `amount = (opening − salvage) / remaining` recomputed EVERY month from current opening (this is what makes impairment/estimate changes prospective); declining `amount = opening × rate/12` where commercial `rate = 2/(LifeMonths/12)` and fiscal rate = table pct/100; clamp `amount ≤ opening − salvage`; when `remaining == 1` (last month) or FinalAbsorb-and-last-month: `amount = opening − salvage` exactly; stop when `opening == salvage`. All arithmetic in `big.Rat`; each Entry's strings via `roundHalfUp2`, and the NEXT month's opening = the ROUNDED closing (so the ledger is self-consistent to the cent).

- [ ] **Step 1: Write the failing test suite FIRST** — table-driven, one behavior per case. Minimum required cases (write them all; add more if a rule feels untested):

```go
// engine_test.go — package depreciation (white-box), testify assert/require.
// Helper: mustWalk(t, p, target) []Entry; date(y, m) time.Time.

// --- Walk: commercial straight line ---
// SL_48m_no_salvage: cost 18_500_000, 48m, salvage 0 → first amount 385416.67,
//   48 entries total when target = start+47mo; SUM(amount) == cost exactly; last closing "0.00".
// SL_salvage: cost 240_000_000+60_000_000 salvage (Innova case: cost 300jt salvage 60jt, 96m)
//   → monthly 2_500_000.00; entry 96 closing == "60000000.00" exactly.
// SL_rounding_absorb: cost 1000, 3m, salvage 0 → 333.33, 333.33, 333.34 (last absorbs).
// SL_memo_value_rp1: cost 5_000_000, 12m, salvage 1 → last closing "1.00" exactly.
// --- Walk: commercial declining balance ---
// DB_floor_at_salvage: rate 2/lifeYears applied to opening; closing never < salvage;
//   verify amount month 2 < amount month 1 (declining property).
// --- Walk: fiscal ---
// FIS_SL_kelompok1: 48m 25%/yr → same as commercial SL with salvage 0.
// FIS_DB_kelompok1_final_absorb: 50%/yr on opening; entry at month 48 amount == whole
//   remaining opening, closing "0.00" (disusutkan sekaligus).
// --- Walk: resumption/catch-up ---
// resume_from_last: run Walk to month 10, feed entry10.Period/Closing back as lastPeriod/
//   lastClosing, Walk to month 20 → concatenation identical to a single Walk to month 20.
// catch_up_multi_year: Start 2020-05, target 2026-07 → 75 entries, periods contiguous.
// fully_depreciated_no_new: lastPeriod = final month, lastClosing == salvage → Walk returns nil.
// target_before_start: returns nil.
// --- Walk: prospective changes ---
// impairment_prospective: SL 48m cost 4800; after 12 months (closing 3600) caller sets
//   lastClosing "1000" (post-impairment book value) → next amount == 1000/36 rounded;
//   final closing still salvage exactly.
// salvage_change_prospective: change p.Salvage between resumed walks → future months
//   spread (opening − newSalvage) over remaining; closed history untouched by construction.
// --- Resolvers ---
// commercial_asset_override_wins / commercial_category_fallback (incl. salvage from
//   default_salvage_rate: cost 10_000_000 × rate "0.1000" → salvage "1000000.00").
// commercial_missing_params → Skip{"missing_params"}; not_capitalized; no_cost; no_purchase_date; disposed.
// fiscal_non_susut → Skip; fiscal_no_group → Skip.
// fiscal_building_falls_back_to_SL: asset method declining_balance + group bangunan_* →
//   Params.Method == straight_line (FALLBACK, not a skip — buildings are always SL fiscally).
// fiscal_salvage_always_zero; fiscal_life_from_table_not_asset (asset.fiscal_life_months
//   ignored in favor of table — table is normative).
// fiscal_method_follows_commercial_when_valid: commercial DB + kelompok_2 → fiscal DB.
// --- rounding ---
// roundHalfUp2: "0.005"→"0.01", "1.004"→"1.00", negatives not required (amounts non-negative).
```

Write REAL Go test code for every case above (the comment list is the required coverage map — each bullet becomes ≥1 concrete test with exact expected strings; compute expecteds by hand/bc and hardcode them).

- [ ] **Step 2: RED** — `go test ./internal/depreciation/` fails to compile (package absent). Correct.

- [ ] **Step 3: Implement `engine.go`** per the interfaces + normative rules above. Note the building rule resolution: fiscal buildings force `straight_line` (fallback, not skip); the Skip reason `"building_requires_straight_line"` is NOT used — remove it from the reasons list docstring.

- [ ] **Step 4: GREEN** — full case list passes; `go vet ./...` clean.

- [ ] **Step 5: Commit** — `feat(depreciation): dual-basis calculation engine (PSAK 16 + PMK 72/2023)`

---

### Task 3: Queries + compute/close service + integration tests

**Files:**
- Create: `backend/db/queries/depreciation.sql`
- Create: `backend/internal/depreciation/service.go`
- Test: `backend/internal/depreciation/depreciation_integration_test.go` (new; copy the harness helper style from `backend/internal/disposal/disposal_integration_test.go` — resetAll/seedOfficeWithType/seedCategory/seedUser/lookupRole; add `TRUNCATE depreciation.depreciation_entries, depreciation.depreciation_periods` to resetAll)

**Interfaces:**
- Consumes: Task 2 engine.
- Produces (Tasks 4–7 consume):

```go
func NewService(q *sqlc.Queries, pool *pgxpool.Pool) *Service
// Compute is idempotent; advisory-locked; regenerates ALL non-closed entries (period >
// closed watermark, ≤ target) for every eligible asset; updates assets commercial summary;
// upserts the period row → computed with run summary. Errors: ErrPeriodClosed.
func (s *Service) ComputePeriod(ctx context.Context, period time.Time, actor uuid.UUID) (RunSummary, error)
type RunSummary struct{ AssetCount int; TotalAmount string; SkippedCount int; Skipped []SkippedAsset }
type SkippedAsset struct{ AssetID uuid.UUID; Reason string }
// Close: only from computed; sequential (every earlier period row must be closed).
// Errors: ErrPeriodNotComputed, ErrPriorPeriodOpen, ErrPeriodClosed (already).
func (s *Service) ClosePeriod(ctx context.Context, period time.Time, actor uuid.UUID) error
func (s *Service) Periods(ctx context.Context) ([]PeriodInfo, error) // + virtual current-month open row
type PeriodInfo struct{ Period time.Time; Status string; AssetCount int; TotalAmount string; SkippedCount int }
// BookValueAsOf: commercial closing of the last entry with period ≤ asOf month;
// fallback (no entries at all): asset.purchase_cost; both absent → "0".
func (s *Service) BookValueAsOf(ctx context.Context, assetID uuid.UUID, asOf time.Time) (string, error)
var ErrPeriodClosed, ErrPeriodNotComputed, ErrPriorPeriodOpen, ErrNotFound error
```

`db/queries/depreciation.sql` (complete file to create):

```sql
-- Depreciation engine queries. See docs/DATABASE.md bagian 4.4 and spec 2026-07-05.

-- name: AdvisoryLockDepreciation :exec
-- Transaction-scoped exclusive lock; released automatically at COMMIT/ROLLBACK.
SELECT pg_advisory_xact_lock(hashtext('depreciation.compute'));

-- name: GetDepreciationPeriod :one
SELECT * FROM depreciation.depreciation_periods WHERE period = $1 AND deleted_at IS NULL;

-- name: ListDepreciationPeriods :many
SELECT * FROM depreciation.depreciation_periods WHERE deleted_at IS NULL ORDER BY period DESC;

-- name: LastClosedPeriod :one
SELECT period FROM depreciation.depreciation_periods
WHERE status = 'closed' AND deleted_at IS NULL ORDER BY period DESC LIMIT 1;

-- name: CountOpenEarlierPeriods :one
SELECT count(*) FROM depreciation.depreciation_periods
WHERE deleted_at IS NULL AND period < $1 AND status <> 'closed';

-- name: UpsertPeriodComputed :one
INSERT INTO depreciation.depreciation_periods (period, status, computed_at, computed_by, asset_count, total_amount, skipped_count)
VALUES (sqlc.arg(period), 'computed', now(), sqlc.arg(computed_by), sqlc.arg(asset_count), sqlc.arg(total_amount), sqlc.arg(skipped_count))
ON CONFLICT (period) WHERE deleted_at IS NULL
DO UPDATE SET status = 'computed', computed_at = now(), computed_by = EXCLUDED.computed_by,
              asset_count = EXCLUDED.asset_count, total_amount = EXCLUDED.total_amount,
              skipped_count = EXCLUDED.skipped_count
RETURNING *;

-- name: SetPeriodClosed :one
UPDATE depreciation.depreciation_periods SET status = 'closed', closed_at = now(), closed_by = $2
WHERE period = $1 AND status = 'computed' AND deleted_at IS NULL RETURNING *;

-- name: DeleteEntriesAfterWatermark :exec
-- Regeneration window: everything past the closed watermark up to the target period.
DELETE FROM depreciation.depreciation_entries
WHERE deleted_at IS NULL AND period > sqlc.arg(watermark) AND period <= sqlc.arg(target);

-- name: DeleteEntriesThrough :exec
-- First-ever run (no watermark): clear everything ≤ target.
DELETE FROM depreciation.depreciation_entries
WHERE deleted_at IS NULL AND period <= sqlc.arg(target);

-- name: LastEntryAtOrBefore :one
SELECT * FROM depreciation.depreciation_entries
WHERE asset_id = $1 AND basis = $2 AND deleted_at IS NULL AND period <= $3
ORDER BY period DESC LIMIT 1;

-- name: InsertDepreciationEntry :exec
INSERT INTO depreciation.depreciation_entries (asset_id, basis, period, opening_value, depreciation_amount, closing_value, method)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListAssetsForDepreciation :many
-- Every capitalized, non-deleted asset with its category (engine resolves/skips per-asset).
SELECT sqlc.embed(a), sqlc.embed(c)
FROM asset.assets a
JOIN masterdata.categories c ON c.id = a.category_id
WHERE a.deleted_at IS NULL;

-- name: UpdateAssetDepreciationSummary :exec
UPDATE asset.assets SET accumulated_depreciation = sqlc.arg(accumulated), book_value = sqlc.arg(book_value)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListAssetEntries :many
SELECT * FROM depreciation.depreciation_entries
WHERE asset_id = $1 AND deleted_at IS NULL ORDER BY basis, period;

-- name: ListEntriesForPeriod :many
-- Schedule/journal source: entries of one period+basis joined to asset+category+office.
SELECT sqlc.embed(e), sqlc.embed(a),
       c.name AS category_name, c.gl_account_code AS gl_account_code,
       o.name AS office_name
FROM depreciation.depreciation_entries e
JOIN asset.assets a ON a.id = e.asset_id
JOIN masterdata.categories c ON c.id = a.category_id
LEFT JOIN masterdata.offices o ON o.id = a.office_id AND o.deleted_at IS NULL
WHERE e.deleted_at IS NULL AND e.period = sqlc.arg(period) AND e.basis = sqlc.arg(basis)
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]));
```

- [ ] **Step 1: Failing integration tests** — write these (full harness like disposal's; drive `svc.ComputePeriod`/`ClosePeriod` directly):

```
TestDepreciation_Compute_HappyPath: seed office+category(SL 48m, salvage_rate 0.1)+asset
  (cost 18_500_000, purchase_date 3 months ago) → Compute(currentMonth) → entries exist for
  BOTH bases (fiscal via category default_fiscal_group kelompok_1) from purchase month to
  target; assets.accumulated_depreciation/book_value updated (commercial, = sums);
  period row status computed with asset_count=1, total_amount == sum of the target month's
  commercial amounts.
TestDepreciation_Compute_Idempotent: Compute twice → identical entry set (count + a
  sampled row equal), no unique violations.
TestDepreciation_Compute_SkippedReporting: asset without purchase_cost + capitalized=false
  asset → RunSummary.SkippedCount == 2 with reasons.
TestDepreciation_StateMachine: Close before compute → ErrPeriodNotComputed; Compute →
  Close OK; Compute again on closed → ErrPeriodClosed; Close(month2) with month1 closed →
  requires month2 computed first; CountOpenEarlierPeriods guard: computing month2 then
  closing month2 while month1 (has row, computed) not closed → ErrPriorPeriodOpen.
TestDepreciation_ClosedWatermark_Immutable: compute m1, close m1, capture m1 entry values;
  compute m2 → m1 entries byte-identical (not regenerated); m2 opening == m1 closing.
TestDepreciation_AdvisoryLock: two goroutines ComputePeriod concurrently → both succeed
  sequentially (no duplicate-key errors), final state consistent.
TestDepreciation_BookValueAsOf: with entries → returns last closing ≤ asOf; without any
  entries → purchase_cost fallback; nil cost → "0".
```

- [ ] **Step 2: RED** (compile). **Step 3:** `sqlc generate` after writing the query file; implement `service.go`:

Compute skeleton (normative): begin tx → `AdvisoryLockDepreciation` → load period row (closed → ErrPeriodClosed) → watermark := LastClosedPeriod (pgx.ErrNoRows ⇒ none) → `DeleteEntriesAfterWatermark`/`DeleteEntriesThrough` → `ListAssetsForDepreciation` → per asset per basis: resolve (skip → collect), `LastEntryAtOrBefore(asset, basis, watermark)` (only when watermark exists; commercial resumption closing may be overridden by a lower `asset.book_value` — see Task 5's impairment rule), `engine.Walk(...)`, insert entries ≤ target. Commercial summary per asset: `accumulated = SumAssetAmounts(asset, 'commercial')` (Σ of ALL commercial amounts — impairment is NOT depreciation expense, so never derive accum as cost − closing), `book_value = closing of the last commercial entry` (fallback: cost − impairment_loss when the asset has no entries), via `UpdateAssetDepreciationSummary`. Add this query to depreciation.sql:

```sql
-- name: SumAssetAmounts :one
SELECT COALESCE(SUM(depreciation_amount), 0)::text FROM depreciation.depreciation_entries
WHERE asset_id = $1 AND basis = $2 AND deleted_at IS NULL;
```

total_amount = Σ commercial amounts of the TARGET month only; asset_count = assets with target-month commercial amount > 0 → `UpsertPeriodComputed` → commit. NOTE assets with status `disposed` are skipped for NEW generation but their historical entries (pre-watermark) are preserved; when regenerating (they're past watermark) their entries up to the month BEFORE disposal-month… simplest normative rule (state it in code comment + test): disposed assets are fully skipped from regeneration — their non-closed entries are deleted and not regenerated; their history survives only in closed periods. This is acceptable because disposal typically happens after periods close; document in the code.

- [ ] **Step 4: GREEN** — `go test -tags=integration ./internal/depreciation/` + unit + build/vet all green.
- [ ] **Step 5: Commit** — `feat(depreciation): compute/close period service with advisory lock and asset summaries`

---

### Task 4: Read endpoints + dto/handler/routes + router wiring

**Files:**
- Create: `backend/internal/depreciation/{dto,handler,routes}.go`
- Modify: `backend/internal/server/router.go` (wire after the disposal block)
- Test: extend `depreciation_integration_test.go` (HTTP-wiring test pattern = `TestApproval_ThresholdPreview`'s stub-auth + RegisterRoutes + httptest, in `internal/approval/integration_test.go`)

**Interfaces (produced — frontend Task 10 consumes):**
- `GET /api/v1/depreciation/periods` (view) → `{data:[{period:"2026-07", status, asset_count, total_amount, skipped_count}]}` — period serialized `YYYY-MM`; includes virtual current-month `open` row when absent.
- `POST /api/v1/depreciation/periods/:period/compute` (manage; `:period` = `YYYY-MM`) → 200 `{period, status:"computed", asset_count, total_amount, skipped_count}`; 409 closed.
- `POST /api/v1/depreciation/periods/:period/close` (manage) → 200 `{period, status:"closed"}`; 409/422 per sentinel (ErrPeriodNotComputed→422, ErrPriorPeriodOpen→422, closed→409).
- `GET /api/v1/depreciation/schedule?period=YYYY-MM&basis=commercial|fiscal&search=&category_id=&office_id=` (view + scope `depreciation`) → `{kpi:{total_cost,total_accumulated,total_book_value,period_expense}, rows:[{asset_id, asset_name, asset_tag, category_name, office_name, method, life_months, opening, amount, accumulated, closing, impaired:boolean, fully_depreciated:boolean}], totals:{opening,amount,accumulated,closing}}`. Rows = union of (entries for the period) plus capitalized parameterized assets WITHOUT an entry this period (fully-depreciated ⇒ amount "0.00", opening=closing=book value, accumulated from asset summary/entries). KPIs across all rows. search/category/office filters applied server-side (search ILIKE name/tag).
- `GET /api/v1/depreciation/journal?period=&basis=` (view + scope) → `{rows:[{account_code, account_name, debit, credit}], total_debit, total_credit, balanced:true}` — debit per category GL ("Beban Penyusutan — {category}"; null GL → code "-", name "(tanpa akun GL)"), one credit row from app_setting `depreciation.accumulated_gl_account` ("Akumulasi Penyusutan").
- `GET /api/v1/assets/:id/depreciation` (asset.view + asset scope; mounted from depreciation routes on the assets group path) → `{masked:false, computed_book_value, entries:[{basis, period:"YYYY-MM", opening, amount, closing, method}]}`; when the caller's role is denied `book_value` on entity `assets` (reuse `fieldSvc.ForEntity` + check the returned policy for book_value view=false) → `{masked:true, computed_book_value:null, entries:[]}`.

Handler notes: period param parsing `time.Parse("2006-01", …)` → 400 on garbage; svcError maps the new sentinels; audit.Record on compute (`audit.ActionUpdate`, entity "depreciation_periods") and close. Router wiring per the excerpt pattern:

```go
		depreciationSvc := depreciation.NewService(queries, d.Pool)
		depreciationHandler := depreciation.NewHandler(depreciationSvc, fieldSvc, common.ScopedDeps{Q: queries, Scope: scopeSvc}, auditSvc)
		depreciation.RegisterRoutes(api, depreciationHandler,
			requireAuth,
			middleware.RequirePermission(permSvc, "depreciation.manage"),
			middleware.RequirePermission(permSvc, "depreciation.view"),
			middleware.RequirePermission(permSvc, "asset.view"),
		)
```

- [ ] Steps: (1) failing integration tests — HTTP subtests: compute+close via routes (200s, 409 closed, 422 out-of-order), schedule returns KPI+rows incl. a fully-depreciated asset with amount "0.00", journal balanced (total_debit==total_credit; null-GL row grouping), asset-depreciation endpoint (entries + masked variant by seeding a deny field_permissions row for a role, mirroring `TestApproval_FieldMasking_Requests`); (2) RED; (3) implement dto/handler/routes + wire router; (4) GREEN full package + `go test ./...`; (5) commit `feat(depreciation): period run/close, schedule, journal and asset schedule endpoints`.

---

### Task 5: Impairment endpoint

**Files:** Modify `backend/internal/depreciation/{service,dto,handler,routes}.go`; Test: extend integration file.

**Interfaces:** `POST /api/v1/assets/:id/impairment` (gate `depreciation.manage` + scope `depreciation` on the asset's office) body `{"recoverable_amount":"1000000","reason":"…"}` (both required; amount validated with a local plain-decimal guard — same char-scan+`big.Rat` approach as approval's `parsePlainDecimal`). Service `RecordImpairment(ctx, assetID, recoverable string, reason string, actor uuid.UUID) (sqlc.AssetAsset, error)`: tx → load asset (404) → current book value := `asset.book_value` (nil → 422 `ErrNoBookValue` "run depreciation first") → recoverable must be `>= 0` and `< book_value` (422 `ErrInvalidRecoverable`) → `impairment_loss += (book − recoverable)`, `book_value = recoverable` via new query `ApplyAssetImpairment` (UPDATE both columns, RETURNING *) → `audit.Record` with `Diff(before, after)` money-field maps → commit. Response: `{book_value, impairment_loss, accumulated_depreciation}`.

**Normative engine-integration rule (single rule — implement exactly this):** in `ComputePeriod`, when resolving each asset's commercial resumption point, the resumption closing = `asset.book_value` **when it is non-nil and LOWER than the last closed entry's closing** (an impairment happened since), else the entry's closing. One `if` in the service; the engine stays untouched — its `(opening − salvage)/remaining` form spreads the impaired base prospectively. Closed history is untouched by construction. No period-state guard is needed: impairment mid-open-period simply takes effect on the next recompute.

Integration tests: happy (loss accumulates, book drops, audit row), guard recoverable ≥ book → 422, no book value → 422, permission gate, post-impairment recompute spreads impaired base prospectively (assert next month amount == (recoverable − salvage)/remaining rounded).

Commit: `feat(depreciation): impairment write-down (PSAK 48) with prospective schedule adjustment`.

---

### Task 6: Journal export (xlsx + PDF)

**Files:** Create `backend/internal/depreciation/export.go`; modify routes (`GET /depreciation/journal/export?period=&basis=&format=xlsx|pdf`, gate view+scope); `go get github.com/xuri/excelize/v2` (go.mod/go.sum change); Test: extend integration file.

xlsx: sheet "Jurnal Penyusutan"; header row Kode Akun|Nama Akun|Debit|Kredit; data rows; TOTAL row; column widths set; stream via `f.WriteToBuffer()`; response headers `Content-Disposition: attachment; filename="jurnal-penyusutan-<period>-<basis>.xlsx"`, content type `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet`. PDF: mirror the label-PDF pattern (`fpdf.New("P","mm","A4","")`, company name via `GetAppSetting("label.company_name")` header, title + basis/period subtitle, table, TOTAL, "Jurnal seimbang — debit = kredit." footer note; `pdf.Output(&buf)`; `c.Data(200, "application/pdf", …)`). Tests: xlsx magic bytes `PK\x03\x04` + non-empty; PDF magic `%PDF`; 400 bad format; permission gate. Commit `feat(depreciation): journal export to xlsx and pdf`.

---

### Task 7: Disposal integration (server-computed book value + amount basis)

**Files:**
- Modify: `backend/internal/disposal/{dto,service}.go`, `backend/internal/server/router.go` (disposal service gains dep), `backend/internal/disposal/disposal_integration_test.go` (constructor calls + new tests), `backend/internal/transfer/transfer_integration_test.go` ONLY if it constructs disposal (it doesn't — skip).

**Changes (surgical, against the verbatim current code):**
1. `dto.go` `SubmitRequest`: DELETE the `BookValue *string \`json:"book_value_at_disposal"\`` field. `DisposalPayload.BookValue` STAYS (now server-filled). `SubmitInput` keeps `BookValue *string` but it is no longer set from the request — the service fills it.
2. `service.go`: `NewService(q, pool, appr, depr *depreciation.Service)` (import cycle check: depreciation must not import disposal — it doesn't). In `Submit`, replace the amount block:

```go
	// Server-computed commercial book value as of the disposal month — both the
	// approval-amount basis and book_value_at_disposal (spec 2026-07-05 decision #3).
	asOf, derr := time.Parse("2006-01-02", in.DisposalDate)
	if derr != nil {
		return sqlc.ApprovalRequest{}, ErrInvalidRef
	}
	bookValue, err := s.depr.BookValueAsOf(ctx, in.AssetID, asOf)
	if err != nil {
		return sqlc.ApprovalRequest{}, err
	}
	in.BookValue = &bookValue
	amount := bookValue
```

(`BookValueAsOf` already falls back to purchase_cost → conservative; "0" when both absent.)
3. `handler.go` submit: stop reading `req.BookValue` (field gone) — pass nil; SubmitInput.BookValue set by service.
4. Router: `disposalSvc := disposal.NewService(queries, d.Pool, approvalSvc, depreciationSvc)` — MOVE the depreciation service construction ABOVE the disposal block.
5. OpenAPI: remove `book_value_at_disposal` from `DisposalSubmitRequest` (Task 9 consolidates, but do the yaml edit here to keep the contract honest in one commit — yes, edit here).

Integration tests (extend disposal file): submit with computed entries → payload/row `book_value_at_disposal` == last closing (assert via created disposal row after approval) and approval request amount == that value; no entries → falls back to purchase_cost; maker cannot inject a book value (request with the old JSON field → binding ignores unknown field, row still server value). Update ALL existing `disposal.NewService(` call sites in tests (grep) to pass a real `depreciation.NewService(q, pool)`.

Commit: `feat(disposal): server-computed book value and book-value-based approval amount`.

---

### Task 8: OpenAPI sync (all backend)

**Files:** `backend/api/openapi.yaml`.

Document: tag `Depreciation`; schemas `DepreciationPeriod` (period `YYYY-MM` string, status enum, counts, total), `DepreciationScheduleRow`, `DepreciationScheduleResponse` (kpi/rows/totals), `DepreciationJournalRow`+response, `AssetDepreciationResponse` (masked/computed_book_value/entries), `ImpairmentRequest`; all 8 paths incl. export (binary responses `application/pdf` + xlsx content types) + impairment; `DisposalSubmitRequest` field removal verified (done in Task 7 — verify, don't duplicate); descriptions mention permission gates + basis semantics + PMK/PSAK refs briefly. Gate: Spectral 0 errors (1 pre-existing AssetCreatePayload warning OK; orphan nothing). Commit `docs(api): depreciation module endpoints and schemas`.

---

### Task 9: Frontend foundations — composable, meta, i18n, nav

**Files:**
- Create: `frontend/app/composables/api/useDepreciation.ts`, `frontend/app/constants/depreciationMeta.ts`
- Modify: `frontend/i18n/locales/{id,en}.json` (new `depreciation.*` section — extract exact strings from the mockup's `CH.id`/`CH.en` tables; the spec's bagian 4 lists the required keys incl. additions: reminder banner, no-manage note, amortization term, impairment-disabled-fiscal tooltip), `frontend/app/utils/nav.ts` (+item after disposals: labelKey `nav.depreciation`, icon `i-lucide-trending-down`, to `/depreciation`, permission `depreciation.view`), nav label keys both locales
- Test: `frontend/test/unit/depreciation-meta.spec.ts`; update `nav-model.spec.ts`

**Interfaces (produced — Tasks 10–11 consume EXACTLY):**

```ts
// depreciationMeta.ts
export type DepreciationBasis = 'commercial' | 'fiscal'
export type PeriodStatus = 'open' | 'computed' | 'closed'
export const PERIOD_STATUS_TONE: Record<PeriodStatus, BadgeColor> = { open: 'warning', computed: 'info', closed: 'neutral' }
export const BASIS_META: Record<DepreciationBasis, { labelKey: string, refKey: string }> = {
  commercial: { labelKey: 'depreciation.basis.commercial', refKey: 'depreciation.basis.refCommercial' },
  fiscal: { labelKey: 'depreciation.basis.fiscal', refKey: 'depreciation.basis.refFiscal' }
}

// useDepreciation.ts — all via useApiClient; requestBlob for exports
export interface DepreciationPeriod { period: string, status: PeriodStatus, asset_count: number, total_amount: string, skipped_count: number }
export interface ScheduleRow { asset_id: string, asset_name: string, asset_tag: string, category_name: string | null, office_name: string | null, method: string, life_months: number, opening: string, amount: string, accumulated: string, closing: string, impaired: boolean, fully_depreciated: boolean }
export interface ScheduleResponse { kpi: { total_cost: string, total_accumulated: string, total_book_value: string, period_expense: string }, rows: ScheduleRow[], totals: { opening: string, amount: string, accumulated: string, closing: string } }
export interface JournalRow { account_code: string, account_name: string, debit: string, credit: string }
export interface JournalResponse { rows: JournalRow[], total_debit: string, total_credit: string, balanced: boolean }
export interface AssetDepreciationEntry { basis: DepreciationBasis, period: string, opening: string, amount: string, closing: string, method: string }
export interface AssetDepreciationResponse { masked: boolean, computed_book_value: string | null, entries: AssetDepreciationEntry[] }
useDepreciation(): {
  periods(): Promise<DepreciationPeriod[]>
  compute(period: string): Promise<DepreciationPeriod>
  close(period: string): Promise<DepreciationPeriod>
  schedule(q: { period: string, basis: DepreciationBasis, search?: string, category_id?: string, office_id?: string }): Promise<ScheduleResponse>
  journal(period: string, basis: DepreciationBasis): Promise<JournalResponse>
  exportJournal(period: string, basis: DepreciationBasis, format: 'xlsx' | 'pdf'): Promise<Blob>
  assetSchedule(assetId: string): Promise<AssetDepreciationResponse>
  recordImpairment(assetId: string, recoverable: string, reason: string): Promise<{ book_value: string, impairment_loss: string }>
}
```

Steps: failing meta unit test (tone map, basis meta keys) → RED → implement all + i18n both locales (structural parity) → `pnpm test` exit 0, lint/typecheck clean → commit `feat(depreciation): frontend composable, meta constants, i18n and nav`.

---

### Task 10: `/depreciation` page + component tests

**Files:** Create `frontend/app/pages/depreciation.vue`; Test: `frontend/test/nuxt/depreciation.spec.ts`.

**Open `docs/design/Depresiasi.dc.html` first — visual source of truth.** Build per spec bagian 4: header+subtitle; basis segmented toggle (chips PSAK 16 / PMK 72/2023) driving ALL data; 4 KPI tiles; Jalankan Periode panel (period select from `periods()`, status badge, Hitung/Tutup/closed-badge by state, preview `asset_count`+`total_amount`, computed note, **reminder banner** when current month open-and-never-computed, manage-gated with disabled+note pattern from transfers.vue); tabs Jadwal per Aset (search/category/office filters — category via `useCategories().tree()`, office via `useOffices().list({limit:100})`; table columns exactly per mockup; impaired violet icon; fully-depreciated rows amount 0; tfoot TOTAL; empty state; row action Catat Penurunan Nilai → impairment modal per mockup (current book value, recoverable input Rp, loss preview red, reason textarea, Simpan & Sesuaikan) — action disabled with tooltip when basis is fiscal (deviation (b)) or when !manage) and Rekap Siap-Jurnal (subtitle + Ekspor PDF/Excel buttons → `exportJournal` blob → anchor download, revoke URL; journal table + TOTAL + balanced banner). Amortisasi wording: rows of intangible assets show method label "Amortisasi — Garis Lurus" style suffix? — mockup doesn't differentiate; keep method badge as mockup, add the intangible nuance ONLY if the mockup shows it (it doesn't — skip, note nothing). Loading/error/empty per fetch; testids: `depr-basis-toggle`, `depr-kpi-*`, `depr-period-select`, `depr-compute`, `depr-close`, `depr-reminder`, `depr-schedule-row`, `depr-impair`, `depr-impair-save`, `depr-journal-row`, `depr-export-xlsx`, `depr-export-pdf`.

Component spec ≥18 cases (vi.mock `useApproval`-style all composables): KPI render; basis toggle refetches with `basis:'fiscal'`; period states (open→Hitung visible; computed→Tutup + note; closed→badge + select disabled); compute calls composable + refresh; reminder banner shown/hidden; manage-gate disabled+note; schedule rows incl. impaired icon + fully-depreciated zero row; filters call schedule with params; empty state; impairment modal (loss preview computation, save calls recordImpairment with exact args, disabled on fiscal basis); journal renders + balanced banner; export triggers blob download (assert exportJournal called + anchor click spied); loading/error states.

Gates: targeted spec green; full `pnpm test` exit 0; lint/typecheck/build. Commit `feat(depreciation): Depresiasi screen wired (run/close, schedule, journal, impairment)`.

---

### Task 11: Asset Detail tab + Disposal screen updates

**Files:**
- Modify: `frontend/app/pages/assets/[tag]/index.vue` (add `v-else-if="tab === 'depr'"` branch BEFORE the generic v-else — see the tabs array at lines ~151 and the generic block at ~489)
- Modify: `frontend/app/pages/disposals.vue` + `frontend/app/composables/api/useDisposals.ts`
- Modify: `frontend/i18n/locales/{id,en}.json` (subtitle key rename)
- Test: extend `frontend/test/nuxt/assets-detail.spec.ts` + `frontend/test/nuxt/disposals.spec.ts`

Detail tab: on first activation fetch `assetSchedule(asset.id)`; render small basis toggle + table (Periode | Nilai Awal | Beban | Nilai Akhir | Metode) filtered by chosen basis; `masked:true` → lock-icon masked state (reuse `assets.masked` i18n); empty entries → "belum pernah dihitung" i18n; loading/error states.

Disposal updates (against the verbatim excerpts):
1. `useDisposals.ts` `DisposalSubmitInput`: DELETE `book_value_at_disposal` field.
2. `disposals.vue` submit body: remove the `book_value_at_disposal: asset.book_value ?? null,` line.
3. Valuation card: fiscal book value cell (testid `disposal-valuation-book-fiscal`) ← real value: on asset select also call `useDepreciation().assetSchedule(asset.id)`; fiscal book value = cost − Σ fiscal amounts (or closing of last fiscal entry; use last closing, fallback "—" when no fiscal entries); commercial book value display can also prefer `computed_book_value` over the stale asset column. Remove the `fiscalTooltip` title (deviation: tooltip "menunggu modul depresiasi" deleted); gain/loss card fiscal row ← proceeds − fiscal book value when available else "—".
4. Chain card: preview amount ← `computed_book_value ?? asset.purchase_cost` (mirrors the server basis switch); i18n: add key `disposal.chain.basedOn` with value "berdasar nilai buku: {value}" / "based on book value: {value}" and use it (keep the old `basedOnBookValue` key removed if unreferenced after this — grep).
5. Component tests updated: submit body has NO book_value_at_disposal (negative assertion); fiscal cell renders real value from stubbed assetSchedule; chain preview called with computed_book_value.

Gates: both specs green, full suite exit 0, lint/typecheck/build. Commit `feat(depreciation): asset detail schedule tab and real fiscal values on disposal screen`.

---

### Task 12: E2E — depreciation lifecycle + disposal spec update

**Files:** Create `frontend/e2e/depreciation.spec.ts`; modify `frontend/e2e/disposals.spec.ts` (its gain/loss "—" assertions change: after this branch a computed asset has real book value).

Conventions: serial mode, RUN uniqueness, file-local helpers from approval.spec.ts, `login(page)` admin (Superadmin = has depreciation.*), no fixed sleeps. Stack: dev Docker + `RATELIMIT_ENABLED=false` + migration 000023 applied (controller ensures).

depreciation.spec.ts flow: beforeAll API — office/category (SL 48m, salvage_rate 0, fiscal_group kelompok_1)/asset via maker-checker with purchase_cost 4_800_000 and purchase_date = 3 months ago (payload purchase_date accepted) + checker user. Tests: (1) `/depreciation` shows reminder banner + period open → Hitung → status Terhitung + preview counts + KPI populated + schedule row for the asset (amount = 100_000.00 = 4.8jt/48); (2) basis toggle Fiskal → same row with fiscal params (kelompok_1: same 48m SL here → same amount; assert reference chip changes); (3) journal tab → balanced banner + export xlsx responds a file (verify via request interception or download event — Playwright `page.waitForEvent('download')`); (4) Tutup Periode → badge Ditutup + Hitung rejected (button absent); (5) impairment: pick the row → modal → recoverable 1_000_000 → save → assert via API `GET /assets/:id/depreciation` that `computed_book_value` reflects the drop (book_value now 1_000_000) and the schedule row shows the impaired icon after refresh; (6) asset detail tab shows entries; (7) disposal of that asset via API submit → approval amount == current book value (assert via `GET /requests` amount field) — the full-circle integration proof.

disposals.spec.ts: adjust any assertion that expected "—" for gain/loss/book-value paths IF its fixture asset now has entries — its assets are freshly created without compute runs, so book value falls back to purchase_cost — gain/loss becomes proceeds − purchase_cost (real number, not "—"): update assertions accordingly (compute expected from the spec's own fixture numbers).

Gates: lint/typecheck; each spec green against live stack; FULL `pnpm test:e2e -- --workers=1` exit 0. Commit `test(depreciation): e2e lifecycle incl. impairment and disposal book-value integration`.

---

### Task 13: Full gates, mockup side-by-side, PROGRESS.md + DATABASE.md

1. Full sweep: backend build/vet/test + `-tags=integration ./...` FULL; Spectral; frontend lint/typecheck/test/build; full e2e workers=1.
2. Side-by-side `Depresiasi.dc.html` vs `http://localhost:3000/depreciation` (Playwright MCP tools; serve mockup via local static server; seed data via API; light+dark) — checklist: header/toggle/KPI/run-panel 3 states/schedule table anatomy/journal/impairment modal/empty states. Only spec bagian 6 deviations (a)–(e) allowed.
3. `docs/DATABASE.md`: add `depreciation_periods` to bagian 4.4 data dictionary + migration mapping bagian 6 (000023) + note DB-Q7 resolved per implementation (commercial summary on assets; fiscal from entries).
4. `docs/PROGRESS.md`: tick **Dual-basis depreciation** + **Journal-ready export** items; close the disposal basis-switch follow-up; update the Scheduler item to reference ADR-0010 stages 2/3; record deviations (a)–(e) + limitations (useful-life revision UI, opening-balance import, Rp-1 flat category default); note the disposed-asset regeneration rule (history survives only in closed periods); new Done entry + next-session pointer refresh (remaining: stock opname / assignment / maintenance / global search / reporting).
5. Commit `docs(progress): mark depreciation module done`. NO push/PR (controller runs final review first).
