# Import Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build bulk CSV/XLSX import end-to-end — a generic `internal/importer` backend engine with per-target importers (asset, employee, office, reference:provinces, reference:cities), an async DB-queue worker, batch approval for assets, and a reusable frontend wizard — replacing the mock (`frontend/app/mock/assets.ts` deleted).

**Architecture:** One backend module `internal/importer` owns job lifecycle, MinIO upload, CSV/XLSX parsing, template + error-report generation, and an async worker (DB-queue via `FOR UPDATE SKIP LOCKED`). Each target implements a `TargetImporter` interface registered in `NewRouter`, living in its own domain package. Master-data targets execute inline in one transaction; the asset target submits a single `asset_import` approval request whose executor creates all assets on final approval. Frontend: a `ImportWizard.vue` component driven by a real-`$fetch` `useImports` composable, polling job status.

**Tech Stack:** Go 1.25 + Gin, pgx/v5, sqlc, golang-migrate, excelize (XLSX), Redis (progress), MinIO (storage.Storage). Nuxt 4 + Nuxt UI (`U*`), Vitest + Playwright.

## Global Constraints

- Go module path: `github.com/ragbuaj/inventra`. Package dir is `internal/importer` (Go reserves `import`).
- Money/numeric columns are **Go `string`** (sqlc override) — never float. Parse for comparison only.
- Every table: `created_at`/`updated_at`/`deleted_at`, `BEFORE UPDATE` trigger `shared.set_updated_at()`, partial-unique `WHERE deleted_at IS NULL`.
- Enums live in `shared`. Add enum values with `ALTER TYPE ... ADD VALUE` (cannot run inside a txn block that also uses the new value — see Task 1 note).
- List endpoints return `{data, total, limit, offset}`; `limit` clamped 1–100 via `clampInt`.
- Data scope enforced on **read and write**, resolved via `common.ScopedDeps.CallerOfficeScope(c, module)`. Import module string: `"imports"`.
- Do NOT hand-edit `backend/db/sqlc/` — edit `db/queries/*.sql` + migrations, then `sqlc generate`.
- Keep `backend/api/openapi.yaml` in sync (Spectral-linted).
- Frontend: SPA (`ssr:false`); build on `U*` components; theme via tokens; i18n mandatory (`id` default + `en`); API via `runtimeConfig.public.apiBase`; ESLint `commaDangle:'never'` + 1tbs.
- Conventional Commits with scope `import` (security fixes `fix(security)`). No AI attribution in commits.
- Verify gates before "done": `go build ./...`, `go vet ./...`, `go test ./...`, `go test -tags=integration ./... -p 1`, Spectral lint, `pnpm lint`, `pnpm typecheck`, `pnpm test`, `pnpm build`.
- Update `docs/PROGRESS.md` and the Obsidian vault (`D:\Obsidian\inventra`) when the module lands.

**Batch approval rule (design decision):** an asset batch = ONE `asset_import` approval request; threshold matched on total batch value; executor creates all assets on final approval. Master-data targets execute directly (no approval). One batch = one office.

---

## Phase 0 — Schema, queries, config

### Task 1: Migration `000030_import_module`

**Files:**
- Create: `backend/db/migrations/000030_import_module.up.sql`
- Create: `backend/db/migrations/000030_import_module.down.sql`

**Interfaces:**
- Produces: enum values `shared.import_status` += `validated,confirmed,executing,awaiting_approval,cancelled`; `shared.request_type` += `asset_import`. New columns on `import.import_jobs` (`office_id`, `request_id`, `confirmed_at`, `error_key`). New table `import.import_rows`.

- [ ] **Step 1: Write the up migration**

`ALTER TYPE ... ADD VALUE` cannot be used in the same transaction that later references the new value, and golang-migrate wraps each file in a txn. Put the enum additions in this migration (they only *declare* values; no query in this file uses them), and rely on later Go code (separate txns) to use them. Add `IF NOT EXISTS` for idempotency.

```sql
-- Import module: batch approval for assets, per-row detail, job routing office.
-- See docs/superpowers/specs/2026-07-12-import-module-design.md.

ALTER TYPE shared.import_status ADD VALUE IF NOT EXISTS 'validated';
ALTER TYPE shared.import_status ADD VALUE IF NOT EXISTS 'confirmed';
ALTER TYPE shared.import_status ADD VALUE IF NOT EXISTS 'executing';
ALTER TYPE shared.import_status ADD VALUE IF NOT EXISTS 'awaiting_approval';
ALTER TYPE shared.import_status ADD VALUE IF NOT EXISTS 'cancelled';

ALTER TYPE shared.request_type ADD VALUE IF NOT EXISTS 'asset_import';

ALTER TABLE import.import_jobs
  ADD COLUMN IF NOT EXISTS office_id     uuid REFERENCES masterdata.offices (id),
  ADD COLUMN IF NOT EXISTS request_id    uuid REFERENCES approval.approval_requests (id),
  ADD COLUMN IF NOT EXISTS confirmed_at  timestamptz,
  ADD COLUMN IF NOT EXISTS error_key     text;

CREATE INDEX IF NOT EXISTS idx_import_office ON import.import_jobs (office_id);

CREATE TABLE IF NOT EXISTS import.import_rows (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  job_id      uuid NOT NULL REFERENCES import.import_jobs (id) ON DELETE CASCADE,
  row_no      int  NOT NULL,
  data        jsonb NOT NULL DEFAULT '{}',
  valid       boolean NOT NULL DEFAULT false,
  errors      jsonb NOT NULL DEFAULT '[]',
  result_ref  text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_import_rows_job_rowno
  ON import.import_rows (job_id, row_no) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_import_rows_job_valid ON import.import_rows (job_id, valid);
CREATE TRIGGER trg_import_rows_set_updated BEFORE UPDATE ON import.import_rows
  FOR EACH ROW EXECUTE FUNCTION shared.set_updated_at();
```

- [ ] **Step 2: Write the down migration**

Enum values cannot be dropped in Postgres without recreating the type; the down migration removes only what it safely can and documents the rest.

```sql
DROP TABLE IF EXISTS import.import_rows;

DROP INDEX IF EXISTS import.idx_import_office;
ALTER TABLE import.import_jobs
  DROP COLUMN IF EXISTS office_id,
  DROP COLUMN IF EXISTS request_id,
  DROP COLUMN IF EXISTS confirmed_at,
  DROP COLUMN IF EXISTS error_key;

-- NOTE: shared.import_status / shared.request_type enum values are NOT removed
-- (Postgres cannot DROP an enum label). They are inert if unused.
```

- [ ] **Step 3: Apply the migration**

Run:
```bash
cd backend
export DATABASE_URL="postgres://inventra:secret@localhost:5433/inventra_dev?sslmode=disable"
migrate -path db/migrations -database "$DATABASE_URL" up
```
Expected: `30/u import_module (…)` success. Verify: `migrate -path db/migrations -database "$DATABASE_URL" version` prints `30`.

- [ ] **Step 4: Verify down/up round-trips**

Run:
```bash
migrate -path db/migrations -database "$DATABASE_URL" down 1
migrate -path db/migrations -database "$DATABASE_URL" up
```
Expected: both succeed (down drops table+columns; up re-adds; `ADD VALUE IF NOT EXISTS` no-ops on the enum).

- [ ] **Step 5: Commit**

```bash
git add backend/db/migrations/000030_import_module.up.sql backend/db/migrations/000030_import_module.down.sql
git commit -m "feat(db): import module — import_rows table, job routing office, batch enums"
```

---

### Task 2: sqlc queries for import jobs & rows

**Files:**
- Create: `backend/db/queries/importer.sql`
- Modify: `backend/db/sqlc/*` (generated — via `sqlc generate`, do not hand-edit)

**Interfaces:**
- Produces sqlc methods: `CreateImportJob`, `GetImportJob`, `GetImportJobForUpdate`, `ListImportJobs`, `CountImportJobs`, `ClaimPendingJob`, `ClaimConfirmedJob`, `UpdateJobStatus`, `SetJobValidated`, `SetJobResult`, `SetJobRequest`, `RecoverStuckJobs`, `InsertImportRow`, `ListImportRows`, `ListValidImportRows`, `CountImportRows`, `MarkRowResult`, `MarkRowFailed`, `CountRowsByValidity`.

- [ ] **Step 1: Write the query file**

Column overrides for money already exist in `sqlc.yaml`. Write queries following the existing `db/queries/*.sql` style (`-- name: X :one|:many|:exec|:execrows`). Full file:

```sql
-- name: CreateImportJob :one
INSERT INTO import.import_jobs (target, format, filename, object_key, office_id, total_rows, created_by_id, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending')
RETURNING *;

-- name: GetImportJob :one
SELECT * FROM import.import_jobs WHERE id = $1 AND deleted_at IS NULL;

-- name: GetImportJobForUpdate :one
SELECT * FROM import.import_jobs WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: ListImportJobs :many
SELECT * FROM import.import_jobs
WHERE deleted_at IS NULL
  AND created_by_id = @created_by
  AND (@target::text = '' OR target = @target::text)
ORDER BY created_at DESC
LIMIT @lim OFFSET @off;

-- name: CountImportJobs :one
SELECT count(*) FROM import.import_jobs
WHERE deleted_at IS NULL
  AND created_by_id = @created_by
  AND (@target::text = '' OR target = @target::text);

-- name: ClaimPendingJob :one
SELECT * FROM import.import_jobs
WHERE status = 'pending' AND deleted_at IS NULL
ORDER BY created_at
FOR UPDATE SKIP LOCKED
LIMIT 1;

-- name: ClaimConfirmedJob :one
SELECT * FROM import.import_jobs
WHERE status = 'confirmed' AND deleted_at IS NULL
ORDER BY confirmed_at
FOR UPDATE SKIP LOCKED
LIMIT 1;

-- name: UpdateJobStatus :one
UPDATE import.import_jobs SET status = $2 WHERE id = $1 RETURNING *;

-- name: SetJobValidated :one
UPDATE import.import_jobs
SET status = 'validated', total_rows = $2, success_rows = $3, failed_rows = $4
WHERE id = $1 RETURNING *;

-- name: SetJobResult :one
UPDATE import.import_jobs
SET status = $2, success_rows = $3, failed_rows = $4, error_key = $5, finished_at = now()
WHERE id = $1 RETURNING *;

-- name: SetJobRequest :one
UPDATE import.import_jobs
SET status = 'awaiting_approval', request_id = $2
WHERE id = $1 RETURNING *;

-- name: ConfirmJob :one
UPDATE import.import_jobs
SET status = 'confirmed', confirmed_at = now()
WHERE id = $1 AND status = 'validated'
RETURNING *;

-- name: CancelJob :one
UPDATE import.import_jobs
SET status = 'cancelled', finished_at = now()
WHERE id = $1 AND status IN ('pending', 'validated')
RETURNING *;

-- name: RecoverStuckJobs :execrows
UPDATE import.import_jobs
SET status = CASE status WHEN 'processing' THEN 'pending'::shared.import_status
                         WHEN 'executing'  THEN 'confirmed'::shared.import_status
                         ELSE status END
WHERE status IN ('processing', 'executing') AND deleted_at IS NULL;

-- name: InsertImportRow :one
INSERT INTO import.import_rows (job_id, row_no, data, valid, errors)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListImportRows :many
SELECT * FROM import.import_rows
WHERE job_id = $1 AND deleted_at IS NULL
  AND (@only_errors::bool = false OR valid = false)
ORDER BY row_no
LIMIT @lim OFFSET @off;

-- name: CountImportRows :one
SELECT count(*) FROM import.import_rows
WHERE job_id = $1 AND deleted_at IS NULL
  AND (@only_errors::bool = false OR valid = false);

-- name: ListValidImportRows :many
SELECT * FROM import.import_rows
WHERE job_id = $1 AND valid = true AND deleted_at IS NULL
ORDER BY row_no;

-- name: MarkRowResult :exec
UPDATE import.import_rows SET result_ref = $2 WHERE id = $1;

-- name: MarkRowFailed :exec
UPDATE import.import_rows SET valid = false, errors = $2 WHERE id = $1;
```

- [ ] **Step 2: Regenerate sqlc**

Run: `cd backend && sqlc generate`
Expected: no errors; new methods appear under `db/sqlc/importer.sql.go`.

- [ ] **Step 3: Verify build**

Run: `cd backend && go build ./...`
Expected: builds clean (nothing consumes the new methods yet, but generated code must compile).

- [ ] **Step 4: Commit**

```bash
git add backend/db/queries/importer.sql backend/db/sqlc/
git commit -m "feat(db): sqlc queries for import jobs and rows"
```

---

### Task 3: Importer config knobs

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/.env.example`

**Interfaces:**
- Produces: `Config.ImportMaxRows int`, `Config.ImportMaxBytes int64`, `Config.ImportWorkerEnabled bool`, `Config.ImportWorkerPoll time.Duration`.

- [ ] **Step 1: Add fields to the Config struct**

In `config.go`, after `LabelLogoPath string` inside the struct:
```go
	// Bulk import.
	ImportMaxRows       int
	ImportMaxBytes      int64
	ImportWorkerEnabled bool
	ImportWorkerPoll    time.Duration
```

- [ ] **Step 2: Populate them in Load()**

After `LabelLogoPath: getEnv(...)` in the returned struct:
```go
		ImportMaxRows:       getEnvInt("IMPORT_MAX_ROWS", 10000),
		ImportMaxBytes:      int64(getEnvInt("IMPORT_MAX_BYTES", 10*1024*1024)),
		ImportWorkerEnabled: getEnvBool("IMPORT_WORKER_ENABLED", true),
		ImportWorkerPoll:    getEnvDuration("IMPORT_WORKER_POLL", 2*time.Second),
```

- [ ] **Step 3: Document in .env.example**

Append:
```
# Bulk import (CSV/XLSX)
IMPORT_MAX_ROWS=10000
IMPORT_MAX_BYTES=10485760
IMPORT_WORKER_ENABLED=true
IMPORT_WORKER_POLL=2s
```

- [ ] **Step 4: Verify + commit**

Run: `cd backend && go build ./...`
Expected: clean.
```bash
git add backend/internal/config/config.go backend/.env.example
git commit -m "feat(import): config knobs for row/byte caps and worker"
```

---

## Phase 1 — Engine core

### Task 4: Core types & TargetImporter interface

**Files:**
- Create: `backend/internal/importer/target.go`
- Test: `backend/internal/importer/target_test.go`

**Interfaces:**
- Produces:
```go
type ColumnSpec struct { Name string; Required bool; Kind string } // Kind: "text"|"date"|"decimal"|"lookup"
type RawRow struct { RowNo int; Cells map[string]string }
type CellError struct { Column string `json:"column"`; ErrorKey string `json:"error_key"` }
type RowResult struct { RowNo int; Valid bool; Data map[string]string; Errors []CellError; NormalizedRef string }
type Scope struct { AllScope bool; OfficeIDs []uuid.UUID; UserID uuid.UUID }
type Job struct { ID uuid.UUID; Target, Format, Filename string; OfficeID *uuid.UUID; TotalRows int }
type Row struct { ID uuid.UUID; RowNo int; Data map[string]string }
type TargetImporter interface {
    Target() string
    Columns() []ColumnSpec
    ValidateRows(ctx context.Context, rows []RawRow, scope Scope) ([]RowResult, error)
    Execute(ctx context.Context, qtx *sqlc.Queries, job Job, validRows []Row) (created int, err error)
    NeedsApproval() bool
}
type registry map[string]TargetImporter
```
- Registry helpers: `func (r registry) get(target string) (TargetImporter, bool)`, `func (r registry) targets() []string`.

- [ ] **Step 1: Write the failing test**

```go
package importer

import "testing"

type stubTarget struct{ name string }

func (s stubTarget) Target() string             { return s.name }
func (s stubTarget) Columns() []ColumnSpec       { return []ColumnSpec{{Name: "a", Required: true, Kind: "text"}} }
func (s stubTarget) ValidateRows(ctx any, rows any, sc any) any { return nil } // replaced below
func (s stubTarget) NeedsApproval() bool          { return false }

func TestRegistryGet(t *testing.T) {
	r := registry{}
	r["asset"] = assetStubForTest{}
	got, ok := r.get("asset")
	if !ok || got.Target() != "asset" {
		t.Fatalf("expected asset target, got ok=%v", ok)
	}
	if _, ok := r.get("nope"); ok {
		t.Fatal("expected miss for unknown target")
	}
	if len(r.targets()) != 1 {
		t.Fatalf("expected 1 target, got %d", len(r.targets()))
	}
}
```
> Note: define a minimal `assetStubForTest` in the test that fully satisfies `TargetImporter` with the real signatures once the interface exists. The scaffolding above is illustrative; the real test uses the final interface types.

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/importer/ -run TestRegistryGet`
Expected: FAIL (package/types undefined).

- [ ] **Step 3: Write target.go**

Define the structs and interface exactly as in **Interfaces** above, plus:
```go
func (r registry) get(target string) (TargetImporter, bool) { t, ok := r[target]; return t, ok }
func (r registry) targets() []string {
	out := make([]string, 0, len(r))
	for k := range r { out = append(out, k) }
	sort.Strings(out)
	return out
}
```
Rewrite `assetStubForTest` in the test to implement the final interface.

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go test ./internal/importer/ -run TestRegistryGet`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/importer/target.go backend/internal/importer/target_test.go
git commit -m "feat(import): TargetImporter interface and registry"
```

---

### Task 5: CSV/XLSX parser

**Files:**
- Create: `backend/internal/importer/parser.go`
- Test: `backend/internal/importer/parser_test.go`

**Interfaces:**
- Consumes: `ColumnSpec` (Task 4), `storage.Storage`.
- Produces:
```go
var (
    ErrBadFormat   = errors.New("importer: unsupported format")
    ErrBadHeader   = errors.New("importer: header does not match template")
    ErrTooManyRows = errors.New("importer: row count exceeds limit")
    ErrEmptyFile   = errors.New("importer: file has no data rows")
)
// Parse reads an already-downloaded file body and returns rows keyed by column name.
// format is "csv" or "xlsx". cols defines the required header (order-insensitive, case-insensitive).
func Parse(format string, body []byte, cols []ColumnSpec, maxRows int) ([]RawRow, error)
// errorKeyFor maps a parser sentinel to an i18n key stored on the job.
func errorKeyFor(err error) string
```

- [ ] **Step 1: Write failing tests**

```go
package importer

import (
	"strings"
	"testing"
)

var testCols = []ColumnSpec{
	{Name: "nama", Required: true, Kind: "text"},
	{Name: "harga", Required: true, Kind: "decimal"},
}

func TestParseCSV_OK(t *testing.T) {
	csv := "nama,harga\nMeja,1000\nKursi,2000\n"
	rows, err := Parse("csv", []byte(csv), testCols, 100)
	if err != nil { t.Fatalf("unexpected err: %v", err) }
	if len(rows) != 2 { t.Fatalf("want 2 rows, got %d", len(rows)) }
	if rows[0].Cells["nama"] != "Meja" || rows[1].Cells["harga"] != "2000" {
		t.Fatalf("bad cell values: %+v", rows)
	}
	if rows[0].RowNo != 1 { t.Fatalf("want RowNo 1, got %d", rows[0].RowNo) }
}

func TestParseCSV_BadHeader(t *testing.T) {
	_, err := Parse("csv", []byte("wrong,cols\n1,2\n"), testCols, 100)
	if err == nil || !strings.Contains(err.Error(), "header") {
		t.Fatalf("want header error, got %v", err)
	}
}

func TestParseCSV_HeaderCaseInsensitiveAndReordered(t *testing.T) {
	rows, err := Parse("csv", []byte("HARGA,Nama\n1000,Meja\n"), testCols, 100)
	if err != nil { t.Fatalf("unexpected err: %v", err) }
	if rows[0].Cells["nama"] != "Meja" || rows[0].Cells["harga"] != "1000" {
		t.Fatalf("column mapping wrong: %+v", rows[0])
	}
}

func TestParseCSV_TooManyRows(t *testing.T) {
	var b strings.Builder
	b.WriteString("nama,harga\n")
	for i := 0; i < 5; i++ { b.WriteString("x,1\n") }
	_, err := Parse("csv", []byte(b.String()), testCols, 3)
	if err != ErrTooManyRows { t.Fatalf("want ErrTooManyRows, got %v", err) }
}

func TestParseCSV_Empty(t *testing.T) {
	_, err := Parse("csv", []byte("nama,harga\n"), testCols, 100)
	if err != ErrEmptyFile { t.Fatalf("want ErrEmptyFile, got %v", err) }
}

func TestParse_BadFormat(t *testing.T) {
	_, err := Parse("pdf", []byte("x"), testCols, 100)
	if err != ErrBadFormat { t.Fatalf("want ErrBadFormat, got %v", err) }
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/importer/ -run TestParse`
Expected: FAIL (undefined `Parse`).

- [ ] **Step 3: Implement parser.go**

Use `encoding/csv` for CSV and `github.com/xuri/excelize/v2` for XLSX (already a dependency — used by `internal/report/export.go`; confirm with `grep excelize backend/go.mod`, and `go get` it if missing). Build a case-insensitive header→index map, validate every `ColumnSpec.Name` is present, trim cells, assign `RowNo` starting at 1, enforce `maxRows`, reject when no data rows. XLSX: read the first sheet via `f.GetRows(sheetName)`.

```go
package importer

import (
	"bytes"
	"encoding/csv"
	"errors"
	"strings"

	"github.com/xuri/excelize/v2"
)

var (
	ErrBadFormat   = errors.New("importer: unsupported format")
	ErrBadHeader   = errors.New("importer: header does not match template")
	ErrTooManyRows = errors.New("importer: row count exceeds limit")
	ErrEmptyFile   = errors.New("importer: file has no data rows")
)

func Parse(format string, body []byte, cols []ColumnSpec, maxRows int) ([]RawRow, error) {
	var records [][]string
	switch strings.ToLower(format) {
	case "csv":
		r := csv.NewReader(bytes.NewReader(body))
		r.FieldsPerRecord = -1
		recs, err := r.ReadAll()
		if err != nil { return nil, ErrBadFormat }
		records = recs
	case "xlsx":
		f, err := excelize.OpenReader(bytes.NewReader(body))
		if err != nil { return nil, ErrBadFormat }
		defer f.Close()
		sheets := f.GetSheetList()
		if len(sheets) == 0 { return nil, ErrBadFormat }
		recs, err := f.GetRows(sheets[0])
		if err != nil { return nil, ErrBadFormat }
		records = recs
	default:
		return nil, ErrBadFormat
	}
	if len(records) == 0 { return nil, ErrBadHeader }

	header := records[0]
	idx := map[string]int{}
	for i, h := range header { idx[strings.ToLower(strings.TrimSpace(h))] = i }
	for _, c := range cols {
		if _, ok := idx[strings.ToLower(c.Name)]; !ok { return nil, ErrBadHeader }
	}

	data := records[1:]
	if len(data) == 0 { return nil, ErrEmptyFile }
	if len(data) > maxRows { return nil, ErrTooManyRows }

	out := make([]RawRow, 0, len(data))
	for i, rec := range data {
		cells := map[string]string{}
		for _, c := range cols {
			j := idx[strings.ToLower(c.Name)]
			v := ""
			if j < len(rec) { v = strings.TrimSpace(rec[j]) }
			cells[c.Name] = v
		}
		out = append(out, RawRow{RowNo: i + 1, Cells: cells})
	}
	return out, nil
}

func errorKeyFor(err error) string {
	switch {
	case errors.Is(err, ErrBadHeader):   return "badHeader"
	case errors.Is(err, ErrTooManyRows): return "tooManyRows"
	case errors.Is(err, ErrEmptyFile):   return "emptyFile"
	case errors.Is(err, ErrBadFormat):   return "badFormat"
	default:                             return "parseFailed"
	}
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go test ./internal/importer/ -run TestParse`
Expected: PASS (all 6).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/importer/parser.go backend/internal/importer/parser_test.go
git commit -m "feat(import): CSV/XLSX parser with header + row-cap validation"
```

---

### Task 6: Template generator

**Files:**
- Create: `backend/internal/importer/template.go`
- Test: `backend/internal/importer/template_test.go`

**Interfaces:**
- Consumes: `ColumnSpec`.
- Produces: `func BuildTemplate(format string, cols []ColumnSpec) (body []byte, contentType, ext string, err error)` — a header-only file (CSV or XLSX). Required columns keep their bare name (the `*` marker lives only in UI badges, not the machine header).

- [ ] **Step 1: Write failing test**

```go
func TestBuildTemplateCSV(t *testing.T) {
	body, ct, ext, err := BuildTemplate("csv", testCols)
	if err != nil { t.Fatal(err) }
	if ext != "csv" || ct != "text/csv" { t.Fatalf("bad meta: %s %s", ct, ext) }
	if string(body) != "nama,harga\n" { t.Fatalf("bad body: %q", string(body)) }
}

func TestBuildTemplateXLSX(t *testing.T) {
	body, _, ext, err := BuildTemplate("xlsx", testCols)
	if err != nil { t.Fatal(err) }
	if ext != "xlsx" { t.Fatalf("bad ext %s", ext) }
	rows, err := Parse("xlsx", body, testCols, 100)
	if err != ErrEmptyFile { t.Fatalf("template should have header only, got rows=%v err=%v", rows, err) }
}

func TestBuildTemplateBadFormat(t *testing.T) {
	if _, _, _, err := BuildTemplate("pdf", testCols); err != ErrBadFormat {
		t.Fatalf("want ErrBadFormat, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/importer/ -run TestBuildTemplate`
Expected: FAIL.

- [ ] **Step 3: Implement template.go**

CSV: join column names with a trailing newline. XLSX: `excelize.NewFile()`, write header cells on row 1, `f.WriteToBuffer()`.

- [ ] **Step 4: Run to verify it passes**

Run: `cd backend && go test ./internal/importer/ -run TestBuildTemplate`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/importer/template.go backend/internal/importer/template_test.go
git commit -m "feat(import): CSV/XLSX template generator"
```

---

### Task 7: Error-report generator

**Files:**
- Create: `backend/internal/importer/errreport.go`
- Test: `backend/internal/importer/errreport_test.go`

**Interfaces:**
- Consumes: `ColumnSpec`, sqlc `ImportImportRow`.
- Produces: `func BuildErrorReport(format string, cols []ColumnSpec, rows []sqlc.ImportImportRow) (body []byte, contentType, ext string, err error)` — original columns + a trailing `keterangan` column joining each failed row's `errors[].error_key`. Only rows with `valid = false` are included by the caller.

- [ ] **Step 1: Write failing test**

Build two `sqlc.ImportImportRow` values (one with `Errors` JSON `[{"column":"harga","error_key":"harga"}]`), call `BuildErrorReport("csv", testCols, rows)`, assert the CSV has header `nama,harga,keterangan` and the data row ends with `harga`. Assert `BuildErrorReport("pdf", ...)` returns `ErrBadFormat`.

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/importer/ -run TestBuildErrorReport`
Expected: FAIL.

- [ ] **Step 3: Implement errreport.go**

Unmarshal `row.Data` (jsonb) into `map[string]string` for cell values, and `row.Errors` into `[]CellError`; write original columns in `cols` order + a joined `keterangan`. Support csv + xlsx like Task 6.

- [ ] **Step 4: Run to verify it passes** — Run the same command; Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/importer/errreport.go backend/internal/importer/errreport_test.go
git commit -m "feat(import): failed-row error-report generator"
```

---

### Task 8: Job service — lifecycle & sentinels

**Files:**
- Create: `backend/internal/importer/service.go`
- Test: `backend/internal/importer/service_test.go`

**Interfaces:**
- Consumes: `*sqlc.Queries`, `*pgxpool.Pool`, `storage.Storage`, `*redis.Client`, `registry`, `Config` caps.
- Produces:
```go
var (
    ErrNotFound      = errors.New("importer: not found")
    ErrForbidden     = errors.New("importer: forbidden")
    ErrUnknownTarget = errors.New("importer: unknown target")
    ErrBadState      = errors.New("importer: illegal state transition")
    ErrConflict      = errors.New("importer: duplicate")
)
type Service struct { /* q, pool, store, rdb, reg, maxRows, maxBytes, bucketPrefix */ }
func NewService(q *sqlc.Queries, pool *pgxpool.Pool, store storage.Storage, rdb *redis.Client, maxRows int, maxBytes int64) *Service
func (s *Service) RegisterTarget(t TargetImporter)
func (s *Service) target(name string) (TargetImporter, error)          // ErrUnknownTarget
func (s *Service) PermissionKey(target string) (string, error)         // maps target→perm key
func mapDBError(err error) error
```
- `PermissionKey`: `asset`→`asset.manage`; `employee`→`masterdata.employee.manage`; `office`→`masterdata.office.manage`; `reference:*`→`masterdata.global.manage`; unknown→`ErrUnknownTarget`.

- [ ] **Step 1: Write failing tests** — pure-logic, no DB:

```go
func TestPermissionKey(t *testing.T) {
	s := &Service{reg: registry{}}
	s.reg["asset"] = assetStubForTest{}
	s.reg["reference:cities"] = assetStubForTest{} // any registered target
	cases := map[string]string{
		"asset":            "asset.manage",
		"reference:cities": "masterdata.global.manage",
	}
	for target, want := range cases {
		got, err := s.PermissionKey(target)
		if err != nil || got != want { t.Fatalf("%s: got %q err %v", target, got, err) }
	}
	if _, err := s.PermissionKey("ghost"); err != ErrUnknownTarget {
		t.Fatalf("want ErrUnknownTarget, got %v", err)
	}
}
```
(`assetStubForTest` from Task 4; ensure its `Target()` matches the map key when registering — register one stub per key.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/importer/ -run TestPermissionKey`
Expected: FAIL.

- [ ] **Step 3: Implement service.go**

Struct + constructor + `RegisterTarget` (keys registry by `t.Target()`) + `target` + `PermissionKey` (switch on prefix `strings.HasPrefix(target, "reference:")`) + `mapDBError` (copy the approval-module pattern: `pgx.ErrNoRows`→`ErrNotFound`, `23505`→`ErrConflict`, `23503`→`ErrForbidden`/invalid-ref). Add `CreateJob`, `GetJob`, `ListJobs`, `ConfirmJob`, `CancelJob` methods (DB-backed — exercised by integration tests in Phase 5, not unit tests). `CreateJob` flow: resolve target → validate format in {csv,xlsx} → cap body size → upload to `imports/<jobID>/<filename>` via `store.Put` (generate jobID first with `uuid.New()`, but persist via `CreateImportJob` — reconcile by using the returned job's ID for the key; simpler: insert job first, then Put under its ID, then no key update needed since key is deterministic `imports/<id>/<filename>` and stored as `object_key`). Store `object_key` on create.

> Access control helpers (used by handler): `func (s *Service) assertOwner(job sqlc.ImportImportJob, userID uuid.UUID) error` returns `ErrForbidden` when `job.CreatedByID != userID`.

- [ ] **Step 4: Run to verify it passes** — Run same command; Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/importer/service.go backend/internal/importer/service_test.go
git commit -m "feat(import): job service, sentinels, target-permission mapping"
```

---

### Task 9: Worker — validate & execute phases

**Files:**
- Create: `backend/internal/importer/worker.go`
- Test: `backend/internal/importer/worker_test.go`

**Interfaces:**
- Consumes: `*Service`, `*pgxpool.Pool`, `*redis.Client`, `approval.Submitter` (a narrow interface the worker calls to open an asset batch request), `time.Duration` poll.
- Produces:
```go
type Submitter interface {
    Submit(ctx context.Context, in approval.SubmitInput) (sqlc.ApprovalRequest, error)
}
type Worker struct { /* svc, pool, rdb, sub, poll */ }
func NewWorker(svc *Service, pool *pgxpool.Pool, rdb *redis.Client, sub Submitter, poll time.Duration) *Worker
func (w *Worker) Recover(ctx context.Context) error          // reset stuck jobs at startup
func (w *Worker) Run(ctx context.Context)                    // loop until ctx done
func (w *Worker) tick(ctx context.Context) (didWork bool, err error) // one pass: claim+process
func progressKey(jobID uuid.UUID) string                     // "import:progress:<id>"
```

- [ ] **Step 1: Write failing test** — progress-key + phase-selection helper:

```go
func TestProgressKey(t *testing.T) {
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	if progressKey(id) != "import:progress:00000000-0000-0000-0000-000000000001" {
		t.Fatalf("bad key %s", progressKey(id))
	}
}
```
Also test a pure helper `func aggregate(results []RowResult) (success, failed int)` counting valid vs invalid.

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/importer/ -run 'TestProgressKey|TestAggregate'`
Expected: FAIL.

- [ ] **Step 3: Implement worker.go**

- `Recover`: call `q.RecoverStuckJobs`.
- `Run`: `ticker := time.NewTicker(w.poll)`; on each tick call `tick`; honor `ctx.Done()`.
- `tick`:
  1. **Validate phase** — `ClaimPendingJob` in a txn (`FOR UPDATE SKIP LOCKED`); if found: set `processing`; download `object_key` from storage; `Parse`; on parse error → `SetJobResult(failed, error_key)`; else `ValidateRows`; `InsertImportRow` per result; write Redis progress `{phase:"validate",done,total}`; `SetJobValidated(total, success, failed)`.
  2. **Execute phase** — `ClaimConfirmedJob`; set `executing`; load `ListValidImportRows`; if `target.NeedsApproval()`: compute total value (sum `harga`), call `sub.Submit` with `SubmitInput{Type: asset_import, Amount: total, OfficeID: *job.OfficeID, TargetEntity:"import_job", Payload: {job_id,...}, Maker: job.CreatedByID}`, then `SetJobRequest(job.ID, req.ID)`. Else run `target.Execute` inside a txn, then `SetJobResult(completed, created, failed, "")`.
- Redis progress writes use `w.rdb.Set(ctx, progressKey(id), json, time.Hour)`.

- [ ] **Step 4: Run to verify it passes** — Run same command; Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/importer/worker.go backend/internal/importer/worker_test.go
git commit -m "feat(import): async DB-queue worker (validate + execute phases)"
```

---

## Phase 2 — Target importers

### Task 10: Asset importer + `asset_import` executor

**Files:**
- Create: `backend/internal/asset/importer.go`
- Test: `backend/internal/asset/importer_test.go`
- Modify: `backend/internal/asset/executor.go` (add `assetImportExec`)

**Interfaces:**
- Consumes: `importer.ColumnSpec/RawRow/RowResult/Scope/Job/Row`, `*Service` (asset), `*sqlc.Queries`.
- Produces:
```go
func (s *Service) Importer() importer.TargetImporter        // returns assetImporter{s}
func (s *Service) ImportExecutor() approval.Executor        // returns assetImportExec{s}
// AssetImportPayload — JSON stored in approval_requests.payload for asset_import.
type AssetImportPayload struct { JobID string `json:"job_id"`; Filename string `json:"filename"`; TotalRows int `json:"total_rows"`; TotalValue string `json:"total_value"`; OfficeID string `json:"office_id"` }
```

**Columns** (`assetImporter.Columns()`): `asset_tag`(opt,text) · `nama`(req,text) · `kategori`(req,lookup) · `kantor`(req,lookup) · `tgl_beli`(req,date) · `harga`(req,decimal) · `vendor`(opt,lookup) · `lokasi`(opt,lookup).

**Validation rules** (`ValidateRows`) — resolve lookups once up-front (load categories, offices, rooms, vendors within scope into name/code→id maps, case-insensitive), then per row set `errFields`/`error_key`:
- required empty → `required`
- `kategori` not found → `kat`; `kantor` not found → `kantor`; `vendor` present but not found → `vendor`; `lokasi` present but room not in that office → `lokasi`
- `tgl_beli` not `YYYY-MM-DD` → `tgl`
- `harga` not decimal ≥ 0 → `harga`
- `asset_tag` present: bad format / exists in DB / duplicate within file → `dupTag`
- all rows must share one `kantor`; a differing office → `multiOffice`
- resolved office not in `scope` → `scope`
- `NormalizedRef` = resolved office UUID string for the first valid row (worker reads it to set `job.OfficeID`).

**Execute** (called by the `asset_import` executor, not the worker): for each valid row, resolve tag (`GenerateAssetTag` when `asset_tag` empty), `CreateAsset` with `AssetClass=tangible`, `Capitalized=true`, `CreatedByID=job maker`; on unique conflict, `MarkRowFailed` (error_key `dupTag`) and continue; `MarkRowResult(tag)` on success; return count.

`NeedsApproval()` → `true`.

- [ ] **Step 1: Write failing unit tests** — pure validation, table-driven, using an interface-injected lookup set (define `assetImporter` to take resolved maps so `ValidateRows` is testable without DB): assert each error_key fires (`required`, `kat`, `kantor`, `tgl`, `harga`, `dupTag` for in-file dup, `multiOffice`, `scope`), and a fully-valid batch yields all-valid results with `NormalizedRef` set.

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/asset/ -run TestAssetImporterValidate`
Expected: FAIL.

- [ ] **Step 3: Implement importer.go + assetImportExec**

Follow `createExec` in `executor.go` for the executor shape: unmarshal `AssetImportPayload`, verify `payload.OfficeID == req.OfficeID` (defense-in-depth), load job, assert `awaiting_approval`, load valid rows, run `assetImporter.Execute` inside the passed `qtx`, then `SetJobResult(completed,...)`. Register nothing here — wiring is Task 14.

- [ ] **Step 4: Run to verify it passes** — Run same command; Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/asset/importer.go backend/internal/asset/importer_test.go backend/internal/asset/executor.go
git commit -m "feat(import): asset target importer + asset_import approval executor"
```

---

### Task 11: Employee importer

**Files:**
- Create: `backend/internal/masterdata/employee/importer.go`
- Test: `backend/internal/masterdata/employee/importer_test.go`

**Interfaces:**
- Produces: `func (s *Service) Importer() importer.TargetImporter`.

**Columns**: `kode`(req,text) · `nama`(req,text) · `email`(opt,text) · `telepon`(opt,text) · `kantor`(req,lookup) · `status`(req,text, one of `active|inactive`). (Department/position optional lookups may be added later — out of scope; `Columns()` lists only these.)

**Validation**: required empty → `required`; `kantor` not found / out of scope → `kantor` / `scope`; `status` not in enum → `status`; `email` malformed → `email`; `kode` duplicate in file or exists in DB → `dupKode`. `NormalizedRef` unused (no batch office rule for employees, but still enforce each row's office ∈ scope).
**Execute**: per valid row `s.Create(ctx, allScope, officeIDs, CreateInput{...})` mapping to the service's `CreateInput` (Task ref: `employee/service.go`); on `common.MapDBError`==conflict `MarkRowFailed(dupKode)` and continue. `NeedsApproval()`→`false`.

> Because master-data `Execute` runs inside the worker's txn via `qtx`, add a thin `s.CreateTx(ctx, qtx, ...)` variant OR have the importer build `sqlc.CreateEmployeeParams` directly against `qtx` (preferred — avoids threading scope through service). Use `qtx.CreateEmployee(...)` directly with pre-validated params.

- [ ] **Step 1: Write failing unit tests** — validation table for each error_key + valid batch.
- [ ] **Step 2: Run** `cd backend && go test ./internal/masterdata/employee/ -run TestEmployeeImporter` → FAIL.
- [ ] **Step 3: Implement importer.go** as specified.
- [ ] **Step 4: Run** same → PASS.
- [ ] **Step 5: Commit**
```bash
git add backend/internal/masterdata/employee/importer.go backend/internal/masterdata/employee/importer_test.go
git commit -m "feat(import): employee target importer"
```

---

### Task 12: Office importer

**Files:**
- Create: `backend/internal/masterdata/office/importer.go`
- Test: `backend/internal/masterdata/office/importer_test.go`

**Interfaces:**
- Produces: `func (s *Service) Importer() importer.TargetImporter`.

**Columns**: from `office/service.go` `CreateInput` — `kode`(req,text) · `nama`(req,text) · `tipe`/`tier`(req,text enum) · `induk`(opt,lookup by parent office code) · plus required fields present in `CreateInput` (read the struct at implementation time and map each; do not invent fields). **Validation**: required empty → `required`; enum invalid → `tier`; parent `induk` not found or (for scoped callers) not within scope → `induk`/`scope`; `kode` dup → `dupKode`. **Execute**: build `sqlc.CreateOfficeParams` against `qtx` from validated values. `NeedsApproval()`→`false`.

- [ ] **Step 1: Write failing tests** — validation table.
- [ ] **Step 2: Run** `go test ./internal/masterdata/office/ -run TestOfficeImporter` → FAIL.
- [ ] **Step 3: Implement** (mirror Task 11 against office fields).
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit**
```bash
git add backend/internal/masterdata/office/importer.go backend/internal/masterdata/office/importer_test.go
git commit -m "feat(import): office target importer"
```

---

### Task 13: Reference importer (provinces, cities)

**Files:**
- Create: `backend/internal/masterdata/reference/importer.go`
- Test: `backend/internal/masterdata/reference/importer_test.go`

**Interfaces:**
- Produces: `func NewImporter(engine *Engine, resource string) importer.TargetImporter` — parameterized so `reference:provinces` and `reference:cities` are two instances. `Target()` returns `"reference:" + resource`.

**Columns**: from `reference/resources.go` — `provinces`: `nama`(req,text) [+ `kode` if the resource defines one]; `cities`: `nama`(req,text) · `provinsi`(req,lookup by province name/code). Read `referenceResources` at implementation time for the authoritative column list. **Validation**: required empty → `required`; `cities.provinsi` unresolved → `provinsi`; dup name → `dupNama`. **Execute**: insert via the reference engine's parameterized insert against `qtx` (add an `InsertTx(ctx, qtx, resource, values)` seam on `Engine` if none exists). `NeedsApproval()`→`false`.

- [ ] **Step 1: Write failing tests** — validation for provinces + cities.
- [ ] **Step 2: Run** `go test ./internal/masterdata/reference/ -run TestReferenceImporter` → FAIL.
- [ ] **Step 3: Implement** importer.go + any `Engine.InsertTx` seam needed.
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit**
```bash
git add backend/internal/masterdata/reference/importer.go backend/internal/masterdata/reference/importer_test.go backend/internal/masterdata/reference/engine.go
git commit -m "feat(import): reference target importer (provinces, cities)"
```

---

## Phase 3 — HTTP layer & wiring

### Task 14: DTOs, handler, routes

**Files:**
- Create: `backend/internal/importer/dto.go`
- Create: `backend/internal/importer/handler.go`
- Create: `backend/internal/importer/routes.go`
- Test: `backend/internal/importer/dto_test.go`

**Interfaces:**
- Consumes: `*Service`, `*authz.PermissionService`, `common.ScopedDeps`, `audit.Service`.
- Produces: `type Handler struct{...}`; `func NewHandler(...) *Handler`; `func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMW gin.HandlerFunc)`; response serializers `jobToMap(job)`, `rowToMap(row)`.

**Endpoints** (`/imports`, all `authMW`; per-target permission checked *inside* handlers via `svc.PermissionKey(target)` + `permSvc.HasPermission`):
| Method | Path | Handler |
|---|---|---|
| GET | `/imports/template` | `template` (query `target`,`format`) |
| POST | `/imports` | `create` (multipart `file`,`target`) |
| GET | `/imports` | `list` |
| GET | `/imports/:id` | `get` (+ Redis progress + derived approval status) |
| GET | `/imports/:id/rows` | `rows` |
| POST | `/imports/:id/confirm` | `confirm` |
| POST | `/imports/:id/cancel` | `cancel` |
| GET | `/imports/:id/error-report` | `errorReport` |

Handler pattern: bind/validate → resolve permission for target → scope check → service → serialize → respond; sentinel→HTTP via `svcError` (`ErrNotFound`→404, `ErrForbidden`→403, `ErrUnknownTarget`→422, `ErrBadState`→409, `ErrConflict`→409, else 500). Multipart upload cap via `http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes+1)` (copy the attachment handler pattern). Record audit on create/confirm/cancel.

- [ ] **Step 1: Write failing test** — `dto_test.go` asserting `jobToMap` includes `id,target,status,total_rows,success_rows,failed_rows` and omits nothing sensitive; `svcError` maps each sentinel to the right status.
- [ ] **Step 2: Run** `cd backend && go test ./internal/importer/ -run 'TestJobToMap|TestSvcError'` → FAIL.
- [ ] **Step 3: Implement dto.go, handler.go, routes.go.**
- [ ] **Step 4: Run** → PASS; then `go build ./...` → clean.
- [ ] **Step 5: Commit**
```bash
git add backend/internal/importer/dto.go backend/internal/importer/handler.go backend/internal/importer/routes.go backend/internal/importer/dto_test.go
git commit -m "feat(import): HTTP handler, DTOs, routes"
```

---

### Task 15: Wire module into NewRouter + start worker

**Files:**
- Modify: `backend/internal/server/router.go`
- Modify: `backend/cmd/api/main.go` (start worker goroutine + graceful stop)

**Interfaces:**
- Consumes: everything from Tasks 8–14; `assetSvc.Importer()`, `assetSvc.ImportExecutor()`, `employeeSvc.Importer()`, etc. — access to each masterdata sub-service is via the `masterdata` aggregate; expose importers through it (add accessor methods to `masterdata` if the sub-services aren't already reachable from `NewRouter`).

- [ ] **Step 1: Register the asset_import executor**

In `router.go`, next to the other `RegisterExecutor` calls:
```go
approvalSvc.RegisterExecutor(sqlc.SharedRequestTypeAssetImport, assetSvc.ImportExecutor())
```

- [ ] **Step 2: Construct importer service + register targets + routes**

After the report block:
```go
importerSvc := importer.NewService(queries, d.Pool, d.Storage, d.Redis, d.Cfg.ImportMaxRows, d.Cfg.ImportMaxBytes)
importerSvc.RegisterTarget(assetSvc.Importer())
importerSvc.RegisterTarget(masterdata.EmployeeImporter())
importerSvc.RegisterTarget(masterdata.OfficeImporter())
importerSvc.RegisterTarget(masterdata.ReferenceImporter("provinces"))
importerSvc.RegisterTarget(masterdata.ReferenceImporter("cities"))
importerHandler := importer.NewHandler(importerSvc, permSvc, common.ScopedDeps{Q: queries, Scope: scopeSvc}, auditSvc)
importer.RegisterRoutes(api, importerHandler, requireAuth)
```
Add a way to reach `importerSvc` and `approvalSvc.Submit` from `main.go` (return them from `NewRouter` via a small struct, or construct the worker inside `NewRouter` and expose a `Start(ctx)`/`Stop()` handle on `Deps`). Simplest: have `NewRouter` build the `*importer.Worker` and attach it to a package-level or returned handle; recommended — change `NewRouter` to also return `*importer.Worker` (update `main.go` call site).

- [ ] **Step 3: Start the worker in main.go**

```go
if cfg.ImportWorkerEnabled {
	if err := worker.Recover(ctx); err != nil { log.Error("import worker recover", "error", err) }
	go worker.Run(ctx)
}
```
Wire `ctx` cancellation into existing graceful shutdown.

- [ ] **Step 4: Verify**

Run: `cd backend && go build ./... && go vet ./...`
Expected: clean. Bring up infra + run the API; `POST /api/v1/imports` with a tiny CSV returns 201 and the worker moves the job to `validated` (check `GET /imports/:id`).

- [ ] **Step 5: Commit**
```bash
git add backend/internal/server/router.go backend/cmd/api/main.go backend/internal/masterdata/masterdata.go
git commit -m "feat(import): wire importer module + start async worker"
```

---

### Task 16: OpenAPI spec

**Files:**
- Modify: `backend/api/openapi.yaml`

- [ ] **Step 1: Add the `Import` tag + 8 paths + schemas** (`ImportJob`, `ImportRow`, `ImportList`) mirroring the handler responses, following the existing style in the file.
- [ ] **Step 2: Lint**

Run: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: 0 errors (existing known warnings only).
- [ ] **Step 3: Commit**
```bash
git add backend/api/openapi.yaml
git commit -m "docs(import): OpenAPI paths and schemas for import module"
```

---

## Phase 4 — Frontend

### Task 17: `useImports` composable

**Files:**
- Create: `frontend/app/composables/api/useImports.ts`
- Test: `frontend/test/composables/useImports.spec.ts`

**Interfaces:**
- Produces:
```ts
interface ImportJob { id: string; target: string; status: string; total_rows: number; success_rows: number; failed_rows: number; request_id?: string; error_key?: string; approval_status?: string }
interface ImportRow { row_no: number; valid: boolean; data: Record<string,string>; errors: { column: string; error_key: string }[]; result_ref?: string }
function useImports(): {
  uploadImport(target: string, file: File): Promise<ImportJob>
  getJob(id: string): Promise<ImportJob>
  getRows(id: string, opts: { onlyErrors?: boolean; limit?: number; offset?: number }): Promise<{ data: ImportRow[]; total: number }>
  confirmJob(id: string): Promise<ImportJob>
  cancelJob(id: string): Promise<ImportJob>
  listJobs(target: string): Promise<{ data: ImportJob[]; total: number }>
  templateUrl(target: string, format: 'csv' | 'xlsx'): string
  errorReportUrl(id: string): string
}
```
All calls go through `useApi()`/`$fetch` against `runtimeConfig.public.apiBase` (follow `useReports.ts`).

- [ ] **Step 1: Write failing unit tests** — stub `$fetch`, assert each verb hits the right method+path with the right body (multipart for upload); assert `templateUrl`/`errorReportUrl` build correct URLs; assert error propagation.
- [ ] **Step 2: Run** `cd frontend && pnpm test useImports` → FAIL.
- [ ] **Step 3: Implement useImports.ts** following `useReports.ts` conventions.
- [ ] **Step 4: Run** → PASS.
- [ ] **Step 5: Commit**
```bash
git add frontend/app/composables/api/useImports.ts frontend/test/composables/useImports.spec.ts
git commit -m "feat(import): useImports composable (real fetch)"
```

---

### Task 18: `ImportWizard.vue` component

**Files:**
- Create: `frontend/app/components/import/ImportWizard.vue`
- Test: `frontend/test/components/ImportWizard.spec.ts`

**Interfaces:**
- Props: `target: string`, `permission: string`.
- Consumes: `useImports` (Task 17).

Reproduce `docs/design/Import Aset.dc.html` anatomy (stepper, upload card, template row, columns badges, validate table, result card) but driven by real data. Replace the mock file-picker with a real `<input type="file" accept=".csv,.xlsx">` (max 10 MB client check). Steps:
1. Upload → `uploadImport` → poll `getJob` (~1.5s) while `pending|processing` → step 2 when `validated`, or show `error_key` message when `failed`.
2. Preview via `getRows` (server pagination, "only errors" toggle), valid/error counts from job → confirm → `confirmJob` → poll while `confirmed|executing`.
3. Result: master-data → created/failed + error-report download. Asset → **"Diajukan untuk persetujuan"** state (from `awaiting_approval` + `request_id`), polling until `completed`/rejected (derived).
On mount, `listJobs(target)` → if an active job exists, resume to the matching step.

- [ ] **Step 1: Write failing runtime test** (`// @vitest-environment nuxt`, `mountSuspended`) — stub `useImports`; assert step-1 renders columns + upload control; simulate a validated job → step 2 renders row table with error highlighting; simulate an asset `awaiting_approval` job → step 3 shows the approval-pending state (resolved i18n text). Cover loading/empty/error/failed-job states.
- [ ] **Step 2: Run** `cd frontend && pnpm test ImportWizard` → FAIL.
- [ ] **Step 3: Implement ImportWizard.vue** with `U*` components + semantic tokens + i18n keys.
- [ ] **Step 4: Run** → PASS; `pnpm lint && pnpm typecheck` clean.
- [ ] **Step 5: Commit**
```bash
git add frontend/app/components/import/ImportWizard.vue frontend/test/components/ImportWizard.spec.ts
git commit -m "feat(import): reusable ImportWizard component"
```

---

### Task 19: Wire asset import page + master-data entry points + i18n + delete mock

**Files:**
- Modify: `frontend/app/pages/assets/import.vue` (replace mock body with `<ImportWizard target="asset" permission="asset.manage" />`; fix `definePageMeta` permission to `asset.manage`)
- Create: `frontend/app/pages/masterdata/import.vue` (reads `?target=` query, renders `ImportWizard`; permission per target)
- Modify: master-data list pages (Pegawai, Kantor, Referensi) — add an "Import" button linking to `/masterdata/import?target=…`
- Modify: `frontend/app/constants/approvalMeta.ts` + `frontend/app/utils/approvalPayload.ts` — add `asset_import` meta (label, icon, payload summary renderer)
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json` — all new strings incl. `assets.import.errors.*` (`required,kat,kantor,vendor,lokasi,tgl,harga,dupTag,multiOffice,scope`), job-status labels, approval-pending copy, master-data import titles
- Delete: `frontend/app/mock/assets.ts`
- Modify: any remaining `mock/assets.ts` consumers (grep) + their tests

**Interfaces:**
- Consumes: `ImportWizard` (Task 18).

- [ ] **Step 1: Grep consumers of the mock**

Run: `cd frontend && grep -rn "mock/assets" app test e2e`
Expected: a list — every file must be rewired to real composables or stubbed in tests before deletion (see memory: rewiring a composable breaks other consumers' tests).

- [ ] **Step 2: Write/adjust failing tests** for the asset import page (mounts `ImportWizard` with `target="asset"`) and the master-data import page (reads query). Run → FAIL.
- [ ] **Step 3: Implement** the page rewires, entry buttons, approval meta, i18n keys; delete `mock/assets.ts`; fix broken consumer tests (stub their API calls so the suite doesn't hit `:8080`).
- [ ] **Step 4: Verify**

Run: `cd frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build`
Expected: all green; no test hits a real backend (full-suite exit code 0).
- [ ] **Step 5: Commit**
```bash
git add frontend/app frontend/i18n
git rm frontend/app/mock/assets.ts
git commit -m "feat(import): wire asset + master-data import pages, remove mock/assets"
```

---

## Phase 5 — Integration, E2E, docs, gate

### Task 20: Backend integration tests

**Files:**
- Create: `backend/internal/importer/import_integration_test.go` (build tag `//go:build integration`)

Cover (follow existing `*_integration_test.go` harness — seeded DB, real pool/redis/storage fake or minio):
- Full asset cycle: upload CSV (valid+invalid rows) → worker validates → `GET rows` → confirm → worker submits `asset_import` → approve via approval service → assets created, job `completed`, `result_ref` set.
- Asset reject → job shows derived rejected; error-report downloadable.
- Employee cycle: upload → validate → confirm → execute → `completed`, employees created.
- 403: caller lacking `asset.manage` cannot create an asset import; lacking `masterdata.employee.manage` cannot create employee import.
- Scope: upload with a `kantor` outside caller scope → rows flagged `scope`; cross-office batch → `multiOffice`.
- Access: user B cannot GET user A's job (403); the approval-eligible approver *can* read the job's rows.
- Recovery: a job left `processing` is reset to `pending` by `Recover`.
- Pagination on `/rows`; `only_errors` filter.

- [ ] **Step 1: Write the integration tests.**
- [ ] **Step 2: Run**

Run: `cd backend && go test -tags=integration ./internal/importer/ -p 1`
Expected: PASS.
- [ ] **Step 3: Full integration gate**

Run: `cd backend && go test -tags=integration ./... -p 1`
Expected: all packages green (per memory: run the whole suite after shared-signature changes — enum + router touched).
- [ ] **Step 4: Commit**
```bash
git add backend/internal/importer/import_integration_test.go
git commit -m "test(import): backend integration coverage for all targets"
```

---

### Task 21: Playwright E2E

**Files:**
- Create: `frontend/e2e/import.spec.ts`
- Create: `frontend/e2e/fixtures/` sample CSV/XLSX (valid+invalid rows)

Scenarios (real backend + seeded admin; per memory: unique name+code per run, assert-after-search, wait-modal-closed, clear cookies+localStorage on mid-test user switch):
- Asset import happy path: download template, upload fixture (valid+invalid), preview errors, confirm → "diajukan untuk persetujuan"; switch to an approver user (clear cookies+localStorage, re-login), approve; back as maker → job `completed`; verify a created asset appears in Katalog.
- Employee import: upload → confirm → completed → employee visible in Pegawai list.
- Error report download after a partially-invalid batch.
- Validation rejection surfaced in the preview table (e.g. `multiOffice`, bad date).

- [ ] **Step 1: Write the e2e spec + fixtures.**
- [ ] **Step 2: Run** (needs stack up + seeded admin)

Run: `cd frontend && pnpm test:e2e import`
Expected: PASS.
- [ ] **Step 3: Commit**
```bash
git add frontend/e2e/import.spec.ts frontend/e2e/fixtures
git commit -m "test(import): Playwright e2e for asset + employee import flows"
```

---

### Task 22: Mockup comparison, docs, PROGRESS, vault, final gate

**Files:**
- Modify: `docs/PROGRESS.md`
- Modify: Obsidian vault `D:\Obsidian\inventra` — `Modul/Peta Modul.md`, `Proyek/Status & Roadmap.md`, a product-decision note (batch-approval + one-office rule) in `Keputusan/Produk/`, a session note `Catatan/2026-07-12-*.md`

- [ ] **Step 1: Side-by-side mockup comparison**

Open the built `/assets/import` and `docs/design/Import Aset.dc.html` in a browser (light + dark). Verify 1:1 on the parts the mockup covers; confirm the approved deviations (a)–(e) from the spec are the only differences. Capture screenshots.

- [ ] **Step 2: Run the full CI gate**

Run in order, expect all green:
```bash
cd backend && go build ./... && go vet ./... && go test ./... && go test -tags=integration ./... -p 1
npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml
cd ../frontend && pnpm lint && pnpm typecheck && pnpm test && pnpm build
```

- [ ] **Step 3: Update PROGRESS.md** — tick the import checklist item with a one-line note + record the approved deviations (a)–(e) and honest limitations (only 2 reference targets wired; employee/office columns limited to the listed fields; no MinIO-stored error report); refresh the "▶ Next session — start here" block (next candidate: notifications, or remaining master-data import targets).

- [ ] **Step 4: Update the Obsidian vault** — Peta Modul (import module now built), Status & Roadmap, the product-decision note, and a session note.

- [ ] **Step 5: Commit**
```bash
git add docs/PROGRESS.md
git commit -m "docs(import): mark import module done, record deviations + limits"
```

---

## Self-Review Notes (coverage map)

- Spec §2 (data model) → Task 1, 2. §3 (authz) → Task 8 `PermissionKey`, Task 14 handler checks, Task 10–13 scope in validation, Task 20 tests. §4 (backend engine) → Tasks 4–9, 14, 15. §4.2 (asset approval flow) → Task 10 + Task 15 executor wiring. §4.3 (endpoints) → Task 14, 16. §4.4 (asset validation rules) → Task 10. §5 (frontend) → Tasks 17–19. §6 (error handling) → Tasks 5, 9, 14, 18. §7 (testing) → Tasks 4–14 unit, 20 integration, 21 e2e. §8 (out of scope) → respected (intangible, MinIO error report, extra entities, notifications excluded).
- Deviations (a)–(e) implemented in Tasks 18–19, recorded in Task 22.
- Type consistency: `TargetImporter` (Task 4) signatures are consumed unchanged in Tasks 10–13; `Submitter`/`approval.SubmitInput` (Task 9) matches `approval.Service.Submit` (verified in `service.go`); `AssetImportPayload` (Task 10) consumed by the executor in the same task.
