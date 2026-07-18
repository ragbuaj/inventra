# Mutasi + Penghapusan Screens Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Mutasi Aset (`/transfers`) and Penghapusan Aset (`/disposals`) screens 1:1 from their mockups, wired to the completed backends — including the user-approved backend contract additions (migration 000022: `condition_sent`/`transfer_date`/`returned`; `POST /transfers/:id/reject-receive`; enriched reads for both modules; `GET /approval-thresholds/preview`).

**Architecture:** Backend first (migration → contract fields → reject-receive → enriched reads via `sqlc.embed` LEFT JOINs, mirroring PR #48 → threshold preview → OpenAPI), then frontend foundations (meta constants, i18n, caller `office_id` in auth state, nav), composables, shared components (`AssetSearchPicker`, region/history utils), the two pages, real-backend e2e, and final gates + mockup side-by-side + PROGRESS.md.

**Tech Stack:** Go 1.25 + Gin + sqlc (pgx/v5) + golang-migrate + testcontainers; Nuxt 4 + Nuxt UI + Vitest (`mountSuspended`) + Playwright.

**Spec (read first):** `docs/superpowers/specs/2026-07-05-transfer-disposal-screens-design.md`
**Contract reference (exact current code excerpts):** `.superpowers/sdd/contract-report.md` — sections: bagian 1 DDL, bagian 2 queries, bagian 3 transfer module, bagian 4 disposal module, bagian 5 approval, bagian 6 frontend signatures, bagian 9 OpenAPI excerpts, bagian 10 integration-test harness patterns. Consult it before touching a file you haven't read.

## Global Constraints

- Branch: `feat/transfer-disposal-screens` (already created; spec committed).
- Conventional Commits, lowercase, imperative (`feat(transfer): …`, `feat(disposal): …`, `feat(approval): …`, `feat(db): …`). **NEVER add Co-Authored-By / AI attribution.**
- Never hand-edit `backend/db/sqlc/`; edit `db/queries/*.sql` + migrations, run `sqlc generate` from `backend/`.
- ADR-0008 module split: service = logic + sentinel errors (no Gin); dto = serialization; handler = HTTP mapping; routes = mounting.
- Scope enforced on read AND write for every new endpoint; FilterView untouched by this feature (transfer/disposal money fields are not in the field catalog yet — out of scope).
- Frontend: ESLint `commaDangle: 'never'`, 1tbs; i18n id/en for every string (mockup Indonesian strings are the `id` values); `U*` Nuxt UI components; API via `useApiClient`; list endpoints `{data,total,limit,offset}`.
- Mockup fidelity 1:1 to `docs/design/Mutasi Aset.dc.html` and `docs/design/Penghapusan Aset.dc.html` EXCEPT the nine approved deviations (spec bagian 8 (a)–(i)). Open both mockups in a browser before building each page.
- Verify per task: backend `go build ./... ; go vet ./... ; go test ./...` (+ `-tags=integration` for the touched package); frontend `pnpm lint ; pnpm typecheck ; pnpm test` from `frontend/`.
- Local e2e needs the Docker stack + `RATELIMIT_ENABLED=false` (already set in `backend/.env`).

---

### Task 1: Migration 000022 + transfer contract fields (condition_sent, transfer_date)

**Files:**
- Create: `backend/db/migrations/000022_transfer_condition_return.up.sql`, `.down.sql`
- Modify: `backend/db/queries/transfers.sql` (`CreateTransfer`)
- Modify: `backend/internal/transfer/dto.go` (SubmitRequest, TransferPayload, marshalPayload, toResponse)
- Modify: `backend/internal/transfer/service.go` (SubmitInput threading), `backend/internal/transfer/executor.go`
- Modify: `backend/internal/transfer/handler.go` (submit binding passthrough)
- Test: `backend/internal/transfer/transfer_integration_test.go`, `backend/internal/transfer/dto_test.go`

**Interfaces:**
- Consumes: existing `transfer.Service.Submit(ctx, caller, SubmitInput)`, `marshalPayload`, harness helpers `newHarness`/`seedAssetWithCost`/`approveThroughChain` (see contract-report bagian 3/bagian 10).
- Produces: `POST /transfers` accepts `condition_sent` (`baik|rusak_ringan|rusak_berat`, optional at API level) and `transfer_date` (`YYYY-MM-DD`, optional at API level — UI enforces required, spec deviation (i)); `Transfer` response carries `condition_sent`, `transfer_date`, `return_note` (all nullable). Enum value `returned` exists on `shared.transfer_status` (used by Task 2). sqlc type `sqlc.NullSharedTransferCondition` / constants `sqlc.SharedTransferConditionBaik` etc. exist after generate.

- [ ] **Step 1: Write the migration**

`backend/db/migrations/000022_transfer_condition_return.up.sql`:

```sql
-- Transfer condition + planned date + returned state (spec 2026-07-05, decisions #1/#2).
CREATE TYPE shared.transfer_condition AS ENUM ('baik', 'rusak_ringan', 'rusak_berat');

-- Postgres >= 12 allows ADD VALUE inside the migration transaction as long as the
-- new value is not used in the same transaction (it isn't — only later requests use it).
ALTER TYPE shared.transfer_status ADD VALUE IF NOT EXISTS 'returned';

ALTER TABLE transfer.asset_transfers
  ADD COLUMN condition_sent shared.transfer_condition,
  ADD COLUMN transfer_date  date,
  ADD COLUMN return_note    text;
```

`backend/db/migrations/000022_transfer_condition_return.down.sql`:

```sql
ALTER TABLE transfer.asset_transfers
  DROP COLUMN IF EXISTS condition_sent,
  DROP COLUMN IF EXISTS transfer_date,
  DROP COLUMN IF EXISTS return_note;

DROP TYPE IF EXISTS shared.transfer_condition;

-- NOTE: PostgreSQL cannot remove a value from an enum; the 'returned' value on
-- shared.transfer_status intentionally survives the down migration (harmless).
```

- [ ] **Step 2: Extend CreateTransfer + regenerate**

In `backend/db/queries/transfers.sql`, replace `CreateTransfer` with:

```sql
-- name: CreateTransfer :one
INSERT INTO transfer.asset_transfers (
  asset_id, from_office_id, to_office_id, to_room_id, status,
  reason, requested_by_id, approved_by_id, request_id, condition_sent, transfer_date
) VALUES (
  sqlc.arg(asset_id), sqlc.arg(from_office_id), sqlc.arg(to_office_id), sqlc.narg(to_room_id),
  'approved', sqlc.narg(reason), sqlc.arg(requested_by_id), sqlc.narg(approved_by_id), sqlc.narg(request_id),
  sqlc.narg(condition_sent), sqlc.narg(transfer_date)
)
RETURNING *;
```

Run from `backend/`: `sqlc generate` → `CreateTransferParams` gains `ConditionSent sqlc.NullSharedTransferCondition` + `TransferDate pgtype.Date`; model gains the 3 columns. `go build ./...` must pass (executor call site now fails to compile — fix in Step 4).

- [ ] **Step 3: Write the failing unit + integration tests**

Append to `backend/internal/transfer/dto_test.go` (payload round-trip — model on the existing `TestMarshalPayload_RoundTrip`):

```go
func TestMarshalPayload_ConditionAndDate(t *testing.T) {
	from, to := uuid.New(), uuid.New()
	cond := "rusak_ringan"
	date := "2026-07-10"
	raw, err := marshalPayload(from, to, nil, nil, &cond, &date)
	require.NoError(t, err)
	var p TransferPayload
	require.NoError(t, json.Unmarshal(raw, &p))
	require.NotNil(t, p.ConditionSent)
	assert.Equal(t, "rusak_ringan", *p.ConditionSent)
	require.NotNil(t, p.TransferDate)
	assert.Equal(t, "2026-07-10", *p.TransferDate)
}
```

Append to `backend/internal/transfer/transfer_integration_test.go` (reuse `newHarness` + `approveThroughChain` exactly as the existing happy-path test does — read that test first):

```go
// TestTransfer_ConditionAndDate_RoundTrip: submit carries condition_sent+transfer_date
// through the approval payload into the transfer row created by the executor.
func TestTransfer_ConditionAndDate_RoundTrip(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	assetID := seedAssetWithCost(t, h.pool, "TRF-COND-1", "Proyektor Epson", h.catID, h.fromOffice, "1000000")
	maker := seedUser(t, h.pool, h.officeRoleID, "maker.cond@test.local")

	cond := "rusak_ringan"
	date := "2026-07-10"
	req, err := h.tsvc.Submit(ctx, buildCaller(maker, h.officeRoleID, true, nil), transfer.SubmitInput{
		AssetID: assetID, ToOfficeID: h.toOffice, ConditionSent: &cond, TransferDate: &date,
	})
	require.NoError(t, err)
	approveThroughChain(t, h, req.ID, maker)

	row, err := h.q.GetOpenTransferForAsset(ctx, assetID)
	require.NoError(t, err)
	require.True(t, row.ConditionSent.Valid)
	assert.Equal(t, sqlc.SharedTransferConditionRusakRingan, row.ConditionSent.SharedTransferCondition)
	require.True(t, row.TransferDate.Valid)
	assert.Equal(t, "2026-07-10", row.TransferDate.Time.Format("2006-01-02"))
}
```

Adapt harness field names (`h.pool`, `h.catID`, `h.fromOffice`, `h.toOffice`, `h.officeRoleID`, `h.tsvc`, `h.q`) and the `approveThroughChain` signature to what the file actually defines — read it first; keep assertions identical. If `SubmitInput` is constructed with uuid.UUID fields in the existing tests, match that.

- [ ] **Step 4: Run tests to verify they fail, then implement**

Run: `go test ./internal/transfer/` → compile FAIL (marshalPayload arity, SubmitInput fields). Then implement:

`dto.go` — extend `SubmitRequest`, `TransferPayload`, `marshalPayload`, `toResponse`:

```go
type SubmitRequest struct {
	AssetID       string  `json:"asset_id" binding:"required,uuid"`
	ToOfficeID    string  `json:"to_office_id" binding:"required,uuid"`
	ToRoomID      *string `json:"to_room_id" binding:"omitempty,uuid"`
	Reason        *string `json:"reason"`
	ConditionSent *string `json:"condition_sent" binding:"omitempty,oneof=baik rusak_ringan rusak_berat"`
	TransferDate  *string `json:"transfer_date"` // "2006-01-02"; UI requires it, API keeps it optional (spec deviation (i))
}
```

`TransferPayload` gains `ConditionSent *string \`json:"condition_sent"\`` and `TransferDate *string \`json:"transfer_date"\``. `marshalPayload(fromOffice, toOffice uuid.UUID, toRoom *uuid.UUID, reason, conditionSent, transferDate *string)` sets them. `toResponse` adds:

```go
	"condition_sent": condStr(t.ConditionSent),
	"transfer_date":  common.DateStr(t.TransferDate),
	"return_note":    t.ReturnNote,
```

with a small helper in dto.go:

```go
// condStr renders the nullable condition enum as *string for JSON.
func condStr(c sqlc.NullSharedTransferCondition) *string {
	if !c.Valid {
		return nil
	}
	s := string(c.SharedTransferCondition)
	return &s
}
```

`service.go` — `SubmitInput` gains `ConditionSent *string` + `TransferDate *string`; `Submit` validates `TransferDate` format when present (`time.Parse("2006-01-02", *in.TransferDate)` → `ErrInvalidRef` on failure) and passes both into `marshalPayload`.

`handler.go` submit — map the two new body fields into `SubmitInput`.

`executor.go` — unmarshal the payload's new fields and set them on `CreateTransferParams`:

```go
	params.ConditionSent = sqlc.NullSharedTransferCondition{}
	if p.ConditionSent != nil {
		params.ConditionSent = sqlc.NullSharedTransferCondition{SharedTransferCondition: sqlc.SharedTransferCondition(*p.ConditionSent), Valid: true}
	}
	if p.TransferDate != nil {
		t, err := time.Parse("2006-01-02", *p.TransferDate)
		if err != nil {
			return ErrInvalidRef
		}
		params.TransferDate = pgtype.Date{Time: t, Valid: true}
	}
```

(match the executor's existing error style — read it; if it returns `approval.ErrConflict`-style sentinels use the module's existing invalid-ref sentinel.)

- [ ] **Step 5: Verify green**

Run: `go build ./... ; go vet ./... ; go test ./...` then `go test -tags=integration ./internal/transfer/` — ALL green (new + pre-existing).

- [ ] **Step 6: Commit**

```bash
git add backend/db/migrations/000022_transfer_condition_return.up.sql backend/db/migrations/000022_transfer_condition_return.down.sql backend/db/queries/transfers.sql backend/db/sqlc backend/internal/transfer
git commit -m "feat(transfer): condition_sent, transfer_date and returned state groundwork (migration 000022)"
```

---

### Task 2: `POST /transfers/:id/reject-receive` (status `returned`)

**Files:**
- Modify: `backend/db/queries/transfers.sql` (add `SetTransferReturned`), regenerate sqlc
- Modify: `backend/internal/transfer/dto.go` (RejectReceiveRequest), `service.go` (RejectReceive), `handler.go` (rejectReceive + svcError untouched), `routes.go`
- Test: `backend/internal/transfer/transfer_integration_test.go`

**Interfaces:**
- Consumes: Task 1's `returned` enum value; existing scope plumbing (`common.InScope`, `CallerOfficeScope` via handler helper — read how `receive` resolves scope and mirror it).
- Produces: `POST /transfers/{id}/reject-receive` (gate `transfer.manage`), body `{"note": string|null}`; 200 → Transfer with `status="returned"` + `return_note`; 409 when status ≠ `in_transit`; 403 when the **to-office** is out of the caller's scope. Asset does NOT move.

- [ ] **Step 1: Write the failing integration test**

Append to `transfer_integration_test.go` (drive a transfer to `in_transit` the same way the existing receive test does — submit → approve → `Ship`):

```go
func TestTransfer_RejectReceive(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()

	t.Run("happy path: returned, note stored, asset stays at origin", func(t *testing.T) {
		assetID := seedAssetWithCost(t, h.pool, "TRF-RET-1", "Scanner Fujitsu", h.catID, h.fromOffice, "500000")
		maker := seedUser(t, h.pool, h.officeRoleID, "maker.ret1@test.local")
		req, err := h.tsvc.Submit(ctx, buildCaller(maker, h.officeRoleID, true, nil), transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.toOffice})
		require.NoError(t, err)
		approveThroughChain(t, h, req.ID, maker)
		row, err := h.q.GetOpenTransferForAsset(ctx, assetID)
		require.NoError(t, err)
		_, err = h.tsvc.Ship(ctx, buildCaller(maker, h.officeRoleID, true, nil), row.ID, nil)
		require.NoError(t, err)

		note := "kondisi tidak sesuai"
		receiver := seedUser(t, h.pool, h.officeRoleID, "receiver.ret1@test.local")
		out, err := h.tsvc.RejectReceive(ctx, buildCaller(receiver, h.officeRoleID, true, nil), row.ID, &note)
		require.NoError(t, err)
		assert.Equal(t, sqlc.SharedTransferStatusReturned, out.Status)
		require.NotNil(t, out.ReturnNote)
		assert.Equal(t, note, *out.ReturnNote)

		// Asset must still live at the origin office.
		a, err := h.q.GetAsset(ctx, assetID)
		require.NoError(t, err)
		assert.Equal(t, h.fromOffice, a.OfficeID)
	})

	t.Run("guard: only in_transit can be returned", func(t *testing.T) {
		assetID := seedAssetWithCost(t, h.pool, "TRF-RET-2", "Switch Cisco", h.catID, h.fromOffice, "500000")
		maker := seedUser(t, h.pool, h.officeRoleID, "maker.ret2@test.local")
		req, err := h.tsvc.Submit(ctx, buildCaller(maker, h.officeRoleID, true, nil), transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.toOffice})
		require.NoError(t, err)
		approveThroughChain(t, h, req.ID, maker)
		row, _ := h.q.GetOpenTransferForAsset(ctx, assetID)
		// still status=approved (not shipped)
		_, err = h.tsvc.RejectReceive(ctx, buildCaller(maker, h.officeRoleID, true, nil), row.ID, nil)
		assert.ErrorIs(t, err, transfer.ErrInvalidState)
	})

	t.Run("guard: to-office scope enforced", func(t *testing.T) {
		assetID := seedAssetWithCost(t, h.pool, "TRF-RET-3", "UPS APC", h.catID, h.fromOffice, "500000")
		maker := seedUser(t, h.pool, h.officeRoleID, "maker.ret3@test.local")
		req, err := h.tsvc.Submit(ctx, buildCaller(maker, h.officeRoleID, true, nil), transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.toOffice})
		require.NoError(t, err)
		approveThroughChain(t, h, req.ID, maker)
		row, _ := h.q.GetOpenTransferForAsset(ctx, assetID)
		_, err = h.tsvc.Ship(ctx, buildCaller(maker, h.officeRoleID, true, nil), row.ID, nil)
		require.NoError(t, err)
		// caller scoped ONLY to the from-office (not destination) → out of scope
		outsider := seedUser(t, h.pool, h.officeRoleID, "outsider.ret3@test.local")
		_, err = h.tsvc.RejectReceive(ctx, buildCaller(outsider, h.officeRoleID, false, []uuid.UUID{h.fromOffice}), row.ID, nil)
		assert.ErrorIs(t, err, transfer.ErrOutOfScope)
	})
}
```

(Adapt `Ship`'s real signature from service.go — read it; if it takes a date pointer or a ShipInput, match. Keep the three assertions.)

- [ ] **Step 2: Run to verify compile failure**

Run: `go test -tags=integration ./internal/transfer/ -run TestTransfer_RejectReceive` → FAIL: `h.tsvc.RejectReceive` undefined. Correct RED.

- [ ] **Step 3: Implement**

`transfers.sql` — append:

```sql
-- name: SetTransferReturned :one
-- Receiving side declines the shipment: terminal 'returned', asset never moved.
UPDATE transfer.asset_transfers
SET status = 'returned',
    return_note = sqlc.narg(return_note),
    received_by_id = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status = 'in_transit' AND deleted_at IS NULL
RETURNING *;
```

`sqlc generate`. `dto.go`:

```go
// RejectReceiveRequest is the POST /transfers/:id/reject-receive body.
type RejectReceiveRequest struct {
	Note *string `json:"note"`
}
```

`service.go` — read `Receive` in service.go FIRST and mirror its structure exactly: fetch the row with the same load call Receive uses, require `tr.Status == sqlc.SharedTransferStatusInTransit` (else `ErrInvalidState`), require `common.InScope(caller.AllScope, caller.OfficeIDs, tr.ToOfficeID)` (else `ErrOutOfScope`). Shape:

```go
// RejectReceive declines an in-transit shipment on behalf of the destination office.
// The asset never moved, so nothing is relocated; the row terminates as 'returned'.
func (s *Service) RejectReceive(ctx context.Context, caller approval.Caller, id uuid.UUID, note *string) (sqlc.TransferAssetTransfer, error) {
	// 1) load row exactly like Receive does (same query + mapDBError)
	// 2) guard status == in_transit → ErrInvalidState
	// 3) guard InScope(caller, tr.ToOfficeID) → ErrOutOfScope
	// 4) then:
```

then:

```go
	out, err := s.q.SetTransferReturned(ctx, sqlc.SetTransferReturnedParams{
		ID: id, ReturnNote: note, ActorID: &caller.UserID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return out, ErrInvalidState // status raced away from in_transit
		}
		return out, mapDBError(err)
	}
	return out, nil
```

(If `ActorID` generates as `*uuid.UUID` vs `uuid.UUID`, match the generated param type. NOTE the pseudo-line in the snippet above is intentional guidance, not code — write the real fetch per Receive's pattern.)

`handler.go`:

```go
// rejectReceive handles POST /transfers/:id/reject-receive.
func (h *Handler) rejectReceive(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body RejectReceiveRequest
	if err := c.ShouldBindJSON(&body); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	caller, err := h.callerFromCtx(c) // reuse the handler's existing caller/scope helper — read how receive builds it
	if err != nil {
		common.WriteError(c, err)
		return
	}
	out, err := h.svc.RejectReceive(c, caller, id, body.Note)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "transfers", out.ID, &out.ToOfficeID, audit.Diff(nil, toResponse(out)))
	c.JSON(http.StatusOK, toResponse(out))
}
```

(Match the audit call style used by `receive` — if receive doesn't audit, skip the audit line for consistency; check first.)

`routes.go` — add under the transfers group: `g.POST("/:id/reject-receive", authMW, manage, h.rejectReceive)` (same middleware chain as ship/receive).

- [ ] **Step 4: Verify green**

Run: `go build ./... ; go vet ./... ; go test ./...` then `go test -tags=integration ./internal/transfer/` — all green.

- [ ] **Step 5: Commit**

```bash
git add backend/db/queries/transfers.sql backend/db/sqlc backend/internal/transfer
git commit -m "feat(transfer): reject-receive endpoint terminates in-transit shipment as returned"
```

---

### Task 3: Transfer enriched reads

**Files:**
- Modify: `backend/db/queries/transfers.sql` (enriched variants; DELETE superseded plain queries)
- Modify: `backend/internal/transfer/service.go` (List/Get/ListByAsset return types), `dto.go` (enriched serialization), `handler.go`
- Test: `backend/internal/transfer/transfer_integration_test.go`

**Interfaces:**
- Consumes: PR #48's `sqlc.embed` pattern (see `db/queries/approval.sql` GetRequestEnriched for the reference shape).
- Produces: list/get/listByAsset responses additionally carry `asset_name`, `asset_tag`, `from_office_name`, `to_office_name`, `to_room_name`, `requested_by_name`, `received_by_name` (all `string|null`). Ship/receive/reject-receive still return the plain `toResponse` shape (pages refresh via list — same contract note as approval decide).

- [ ] **Step 1: Write the failing integration test**

```go
// TestTransfer_EnrichedReads: list/get carry resolved asset/office/user names;
// soft-deleted join targets keep the row visible with nil names.
func TestTransfer_EnrichedReads(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	assetID := seedAssetWithCost(t, h.pool, "TRF-ENR-1", "Server Dell R750", h.catID, h.fromOffice, "9000000")
	maker := seedUser(t, h.pool, h.officeRoleID, "maker.enr@test.local")
	req, err := h.tsvc.Submit(ctx, buildCaller(maker, h.officeRoleID, true, nil), transfer.SubmitInput{AssetID: assetID, ToOfficeID: h.toOffice})
	require.NoError(t, err)
	approveThroughChain(t, h, req.ID, maker)

	rows, total, err := h.tsvc.List(ctx, true, nil, "", 20, 0)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(1))
	require.NotEmpty(t, rows)
	row := rows[0]
	require.NotNil(t, row.AssetName)
	assert.Equal(t, "Server Dell R750", *row.AssetName)
	require.NotNil(t, row.AssetTag)
	assert.Equal(t, "TRF-ENR-1", *row.AssetTag)
	require.NotNil(t, row.FromOfficeName)
	require.NotNil(t, row.ToOfficeName)
	require.NotNil(t, row.RequestedByName)
	assert.Equal(t, "maker.enr@test.local", *row.RequestedByName)

	t.Run("soft-deleted asset keeps row visible with nil name", func(t *testing.T) {
		_, err := h.pool.Exec(ctx, `UPDATE asset.assets SET deleted_at = now() WHERE id = $1`, assetID)
		require.NoError(t, err)
		rows, _, err := h.tsvc.List(ctx, true, nil, "", 20, 0)
		require.NoError(t, err)
		found := false
		for _, r := range rows {
			if r.TransferAssetTransfer.AssetID == assetID {
				found = true
				assert.Nil(t, r.AssetName)
			}
		}
		assert.True(t, found)
	})
}
```

(Adapt `List`'s real signature — read it; current one likely `List(ctx, all, ids, status, limit, offset)`. Keep the name/nil assertions.)

- [ ] **Step 2: Run → compile FAIL (no AssetName). Then add the enriched queries**

Replace `GetTransfer`/`ListTransfers`/`ListTransfersByAsset` in `transfers.sql` with enriched variants (KEEP `CountTransfers`, `GetOpenTransferForAsset`, `SetTransfer*`, guards). All three share these joins — repeat them per query (sqlc has no fragments):

```sql
-- name: ListTransfersEnriched :many
SELECT sqlc.embed(tr),
       a.name   AS asset_name,
       a.asset_tag AS asset_tag,
       fo.name  AS from_office_name,
       tof.name AS to_office_name,
       rm.name  AS to_room_name,
       ru.name  AS requested_by_name,
       rcu.name AS received_by_name
FROM transfer.asset_transfers tr
LEFT JOIN asset.assets a        ON a.id  = tr.asset_id        AND a.deleted_at IS NULL
LEFT JOIN masterdata.offices fo ON fo.id = tr.from_office_id  AND fo.deleted_at IS NULL
LEFT JOIN masterdata.offices tof ON tof.id = tr.to_office_id  AND tof.deleted_at IS NULL
LEFT JOIN masterdata.rooms rm   ON rm.id = tr.to_room_id      AND rm.deleted_at IS NULL
LEFT JOIN identity.users ru     ON ru.id = tr.requested_by_id AND ru.deleted_at IS NULL
LEFT JOIN identity.users rcu    ON rcu.id = tr.received_by_id AND rcu.deleted_at IS NULL
WHERE tr.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean
       OR tr.from_office_id = ANY(sqlc.arg(office_ids)::uuid[])
       OR tr.to_office_id   = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.transfer_status IS NULL OR tr.status = sqlc.narg(status))
ORDER BY tr.created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);
```

`GetTransferEnriched :one` = same SELECT/joins with `WHERE tr.id = sqlc.arg(id) AND tr.deleted_at IS NULL AND (scope…)`; `ListTransfersByAssetEnriched :many` = same with `WHERE tr.asset_id = sqlc.arg(asset_id) …` ordered by created_at DESC. **Delete** the plain `GetTransfer`/`ListTransfers`/`ListTransfersByAsset` definitions UNLESS service internals still call them (Ship/Receive/RejectReceive load rows — if they use `GetTransfer`, keep it and note it as internal-only). Check with grep before deleting. `sqlc generate`.

- [ ] **Step 3: Swap service + serialize**

Service `List`/`Get`/`ListByAsset` switch to the enriched queries and return the enriched row types (embedded field is `TransferAssetTransfer`). dto.go:

```go
// enrichTransferMap adds resolved display names to a serialized transfer.
func enrichTransferMap(m map[string]any, r sqlc.ListTransfersEnrichedRow) map[string]any {
	m["asset_name"] = r.AssetName
	m["asset_tag"] = r.AssetTag
	m["from_office_name"] = r.FromOfficeName
	m["to_office_name"] = r.ToOfficeName
	m["to_room_name"] = r.ToRoomName
	m["requested_by_name"] = r.RequestedByName
	m["received_by_name"] = r.ReceivedByName
	return m
}
```

Because Get/ByAsset generate distinct Row types with identical fields, add two thin overloads (or convert via a tiny local struct) — pick the least-duplication option that compiles cleanly; do NOT reflect. Handler list/get/listByAsset serialize `enrichTransferMap(toResponse(r.TransferAssetTransfer), r)`.

- [ ] **Step 4: Verify green + fix accessor fallout**

Existing integration tests indexing `rows[0].Status` etc. switch to `rows[0].TransferAssetTransfer.Status` (accessor-only changes). Run: `go build ./... ; go vet ./... ; go test ./... ; go test -tags=integration ./internal/transfer/` — all green.

- [ ] **Step 5: Commit**

```bash
git add backend/db/queries/transfers.sql backend/db/sqlc backend/internal/transfer
git commit -m "feat(transfer): enrich reads with asset, office, room and actor names"
```

---

### Task 4: Disposal enriched reads

**Files:**
- Modify: `backend/db/queries/disposals.sql`, `backend/internal/disposal/{service,dto,handler}.go`
- Test: `backend/internal/disposal/disposal_integration_test.go`

**Interfaces:**
- Consumes: Task 3's pattern.
- Produces: disposal list/get/listByAsset responses + `asset_name`, `asset_tag`, `office_name` (asset's office), `created_by_name` (all `string|null`).

Same 5-step shape as Task 3 — failing integration test first:

```go
func TestDisposal_EnrichedReads(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	assetID := seedAssetWithCost(t, h.pool, "DSP-ENR-1", "Printer HP Lama", h.catID, h.office, "6800000")
	maker := seedUser(t, h.pool, h.officeRoleID, "maker.dspenr@test.local")
	req, err := h.dsvc.Submit(ctx, buildCaller(maker, h.officeRoleID, true, nil), disposal.SubmitInput{
		AssetID: assetID, Method: "sale", DisposalDate: "2026-07-05",
	})
	require.NoError(t, err)
	approveThroughChain(t, h, req.ID, maker)

	rows, total, err := h.dsvc.List(ctx, true, nil, 20, 0)
	require.NoError(t, err)
	require.GreaterOrEqual(t, total, int64(1))
	require.NotEmpty(t, rows)
	require.NotNil(t, rows[0].AssetName)
	assert.Equal(t, "Printer HP Lama", *rows[0].AssetName)
	require.NotNil(t, rows[0].OfficeName)
	require.NotNil(t, rows[0].CreatedByName)
	assert.Equal(t, "maker.dspenr@test.local", *rows[0].CreatedByName)
}
```

Enriched query (repeat joins for Get/ByAsset variants; the asset join must stay an INNER-like scope filter — keep the existing `JOIN asset.assets a` for scoping and ADD the name columns from it, plus LEFT JOINs for office/user):

```sql
-- name: ListDisposalsEnriched :many
SELECT sqlc.embed(d),
       a.name      AS asset_name,
       a.asset_tag AS asset_tag,
       o.name      AS office_name,
       cu.name     AS created_by_name
FROM disposal.disposals d
JOIN asset.assets a             ON a.id = d.asset_id
LEFT JOIN masterdata.offices o  ON o.id = a.office_id       AND o.deleted_at IS NULL
LEFT JOIN identity.users cu     ON cu.id = d.created_by_id  AND cu.deleted_at IS NULL
WHERE d.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR a.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
ORDER BY d.created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);
```

(NOTE: `a.name`/`a.asset_tag` are non-null columns behind an INNER JOIN — sqlc emits them as non-pointer `string`; if so, relax the test's `*rows[0].AssetName` deref to direct equality. Verify after generate and keep types honest.) Serialization helper `enrichDisposalMap` mirrors Task 3. Delete superseded plain queries only if nothing internal still calls them. Commit:

```bash
git commit -m "feat(disposal): enrich reads with asset, office and actor names"
```

---

### Task 5: `GET /approval-thresholds/preview`

**Files:**
- Modify: `backend/internal/approval/service.go` (PreviewChain), `handler.go` (previewThresholds), `routes.go`
- Test: `backend/internal/approval/integration_test.go`

**Interfaces:**
- Consumes: existing `MatchThresholdSteps` + `buildChain` (service.go), `validRequestTypes` (dto.go), `middleware.RequirePermission(permSvc, "request.create")`.
- Produces: `GET /api/v1/approval-thresholds/preview?request_type=<type>&amount=<decimal>` → 200 `{"steps":[{"step_order":1,"required_level":"office"},…]}`; 400 invalid type/amount; 422 when no band matches. Frontend Task 8 consumes this via `useApprovalPreview`.

- [ ] **Step 1: Failing integration test** (service + HTTP wiring; model the HTTP part on `TestApproval_FieldMasking_HandlerWiring` in the same file — stub auth MW + `approval.RegisterRoutes` + httptest):

```go
func TestApproval_ThresholdPreview(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)

	// Deterministic bands: wipe seed, install 2-band config for asset_disposal.
	_, err := pool.Exec(ctx, `TRUNCATE approval.approval_thresholds`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO approval.approval_thresholds (request_type, amount_from, amount_to, required_level, step_order, is_active) VALUES
		('asset_disposal', 0, 10000000, 'office', 1, true),
		('asset_disposal', 10000000, NULL, 'office', 1, true),
		('asset_disposal', 10000000, NULL, 'wilayah', 2, true)`)
	require.NoError(t, err)
	require.NoError(t, rdb.Del(ctx, "approval:thresholds").Err())

	t.Run("low amount → single office step", func(t *testing.T) {
		steps, err := svc.PreviewChain(ctx, sqlc.SharedRequestTypeAssetDisposal, "500000")
		require.NoError(t, err)
		require.Len(t, steps, 1)
		assert.Equal(t, int32(1), steps[0].StepOrder)
		assert.Equal(t, "office", steps[0].RequiredLevel)
	})
	t.Run("high amount → two steps ordered", func(t *testing.T) {
		steps, err := svc.PreviewChain(ctx, sqlc.SharedRequestTypeAssetDisposal, "82000000")
		require.NoError(t, err)
		require.Len(t, steps, 2)
		assert.Equal(t, "wilayah", steps[1].RequiredLevel)
	})
	t.Run("no matching band → ErrNoThreshold", func(t *testing.T) {
		_, err := svc.PreviewChain(ctx, sqlc.SharedRequestTypeMaintenance, "100")
		assert.ErrorIs(t, err, approval.ErrNoThreshold)
	})
}
```

Plus an HTTP subtest driving `GET /api/v1/approval-thresholds/preview?request_type=asset_disposal&amount=500000` through `approval.RegisterRoutes` with the stub-auth pattern: 200 + body contains `"required_level":"office"`; and `?request_type=bogus` → 400. Seed the caller role's `request.create` permission the same way the existing HTTP-wiring test seeds permissions (read it; Superadmin seed role already has it).

- [ ] **Step 2: RED, then implement**

`service.go`:

```go
// PreviewStep is one step of a previewed approval chain (order + level only —
// band amounts are deliberately not exposed to non-admin callers).
type PreviewStep struct {
	StepOrder     int32  `json:"step_order"`
	RequiredLevel string `json:"required_level"`
}

// PreviewChain resolves the approval chain the engine would build for the given
// request type and amount, without creating anything.
func (s *Service) PreviewChain(ctx context.Context, t sqlc.SharedRequestType, amount string) ([]PreviewStep, error) {
	rows, err := s.q.MatchThresholdSteps(ctx, sqlc.MatchThresholdStepsParams{RequestType: t, Amount: amount})
	if err != nil {
		return nil, mapDBError(err)
	}
	chain := buildChain(rows)
	if len(chain) == 0 {
		return nil, ErrNoThreshold
	}
	out := make([]PreviewStep, 0, len(chain))
	for _, st := range chain {
		out = append(out, PreviewStep{StepOrder: st.Order, RequiredLevel: string(st.Level)})
	}
	return out, nil
}
```

`handler.go`:

```go
// previewThresholds handles GET /approval-thresholds/preview.
func (h *Handler) previewThresholds(c *gin.Context) {
	rt := c.Query("request_type")
	if !validRequestTypes[rt] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request_type"})
		return
	}
	amount := c.Query("amount")
	if _, ok := new(big.Rat).SetString(amount); !ok || amount == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid amount"})
		return
	}
	steps, err := h.svc.PreviewChain(c, sqlc.SharedRequestType(rt), amount)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"steps": steps})
}
```

`routes.go` — in the `/approval-thresholds` group add `t.GET("/preview", authMW, create, h.previewThresholds)` **before** any parametrized sibling; the existing group has no `/:id` GET so there is no conflict, but keep it first for clarity.

- [ ] **Step 3: Green + full package run; Step 4: Commit**

```bash
git commit -m "feat(approval): threshold chain preview endpoint for submit-side UIs"
```

---

### Task 6: OpenAPI sync (all backend changes)

**Files:** `backend/api/openapi.yaml`

Document, matching the file's existing style (see contract-report bagian 9 for the current blocks):
1. `TransferSubmitRequest` + `condition_sent` (enum) + `transfer_date` (date).
2. `Transfer` schema: + `condition_sent` (`[string,"null"]` enum), `transfer_date`, `return_note`; `status` enum + `returned`; + the 7 enrichment name fields (`[string,"null"]`).
3. New path `POST /api/v1/transfers/{id}/reject-receive` (body `{note}`, 200 Transfer, 401/403/404/409 refs; description: destination-office scope; asset does not move).
4. `Disposal` schema + `asset_name`/`asset_tag`/`office_name`/`created_by_name`.
5. New path `GET /api/v1/approval-thresholds/preview` (params request_type enum + amount string; 200 `ThresholdPreview {steps:[{step_order:int, required_level:enum}]}`; 400; 422; security bearerJWT; description: gate `request.create`, band amounts not exposed).

Gate: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml` → 0 errors (1 pre-existing AssetCreatePayload warning OK; do not orphan any schema).

```bash
git commit -m "docs(api): transfer condition/return + reject-receive, enriched reads, threshold preview"
```

---

### Task 7: Frontend foundations — meta constants, i18n, caller office_id, nav

**Files:**
- Create: `frontend/app/constants/transferMeta.ts`, `frontend/app/constants/disposalMeta.ts`
- Modify: `frontend/i18n/locales/id.json`, `en.json` (new `transfer` + `disposal` sections + 2 nav keys)
- Modify: `frontend/app/composables/useAuthApi.ts`, `frontend/app/stores/auth.ts` (carry `office_id`)
- Modify: `frontend/app/utils/nav.ts` (+2 items)
- Test: `frontend/test/unit/transfer-disposal-meta.spec.ts`; update `frontend/test/unit/nav-model.spec.ts` and any auth-store spec that asserts the `AuthUser` shape

**Interfaces:**
- Produces: `TransferStatus = 'pending'|'approved'|'in_transit'|'received'|'rejected'|'cancelled'|'returned'` — but UI unified history keys (Task 9) use a superset; the meta here maps ROW statuses + condition + method. Exact exports:

```ts
// transferMeta.ts
export type TransferRowStatus = 'approved' | 'in_transit' | 'received' | 'returned'
export type TransferCondition = 'baik' | 'rusak_ringan' | 'rusak_berat'
export const TRANSFER_STATUS_TONE: Record<TransferRowStatus, BadgeColor> = {
  approved: 'info', in_transit: 'info', received: 'success', returned: 'error'
}
export const CONDITION_TONE: Record<TransferCondition, BadgeColor> = {
  baik: 'success', rusak_ringan: 'warning', rusak_berat: 'error'
}
export const CONDITION_KEYS: TransferCondition[] = ['baik', 'rusak_ringan', 'rusak_berat']

// disposalMeta.ts
export type DisposalMethod = 'sale' | 'auction' | 'donation' | 'write_off'
export const METHOD_KEYS: DisposalMethod[] = ['sale', 'auction', 'donation', 'write_off']
export const METHOD_TONE: Record<DisposalMethod, BadgeColor> = {
  sale: 'info', auction: 'primary', donation: 'success', write_off: 'neutral'
}
```

- `AuthUser` gains `office_id: string | null`; `fetchMe`/`setSession` thread it through (MeResponse already has it — see contract-report bagian 6, it is currently discarded).
- Nav: two items after `nav.assignment`, before `nav.maintenance`:

```ts
      {
        labelKey: 'nav.transfers',
        icon: 'i-lucide-arrow-right-left',
        to: '/transfers',
        permission: 'transfer.view'
      },
      {
        labelKey: 'nav.disposals',
        icon: 'i-lucide-trash-2',
        to: '/disposals',
        permission: 'disposal.view'
      },
```

- i18n: full `transfer.*` + `disposal.*` sections; the mockup's Indonesian strings are the `id` values. **Source of truth for the exact strings: the mockup files themselves** — both `docs/design/Mutasi Aset.dc.html` and `docs/design/Penghapusan Aset.dc.html` contain complete `CH.id` / `CH.en` string tables in their embedded script; extract labels verbatim from there. At minimum: statuses (`transfer.status.{diajukan,approved,in_transit,received,returned,rejected,cancelled}` → "Diajukan"/"Disetujui"/"Dalam Pengiriman"/"Diterima"/"Dikembalikan"/"Ditolak"/"Dibatalkan"), flow legend, tabs, all form labels/placeholders/hints, inter-region alerts, inbox strings + toasts, history columns, ship modal, condition labels; disposal: tabs, form labels, valuation summary labels + PSAK/FISKAL chips + "menunggu modul depresiasi", gain/loss card, chain card (+"berdasar nilai perolehan: {v}", "band approval belum dikonfigurasi", "nilai perolehan tersembunyi untuk peran Anda"), sensitive banner, post-submit strings, timeline states (Disetujui/Menunggu/Antre + meta), history columns, method labels (Dijual/Lelang/Hibah/Musnah), statuses (Menunggu Approval/Ditolak/Dibatalkan/Selesai), attach-BAST modal. Mirror ALL keys in `en.json` (structural parity is CI-tested convention).

- [ ] Steps: (1) failing unit test `transfer-disposal-meta.spec.ts` asserting the exact exports above (tone maps incl. `returned: 'error'`, method keys array, condition keys); (2) RED; (3) implement constants + i18n + auth office_id + nav; (4) `pnpm vitest run test/unit/transfer-disposal-meta.spec.ts test/unit/nav-model.spec.ts` + fix any auth-store spec fallout (additive field — update the fixture, never weaken assertions); (5) `pnpm lint ; pnpm typecheck ; pnpm test` all green; (6) commit `feat(transfer,disposal): meta constants, i18n, nav items and caller office_id in auth state`.

---

### Task 8: Composables — `useTransfers`, `useDisposals`, `useApprovalPreview`

**Files:**
- Create: `frontend/app/composables/api/useTransfers.ts`, `useDisposals.ts`, `useApprovalPreview.ts`

**Interfaces (produced — Tasks 9–11 consume these EXACT shapes):**

```ts
// useTransfers.ts
export interface Transfer {
  id: string; asset_id: string; from_office_id: string; to_office_id: string
  to_room_id: string | null
  status: 'approved' | 'in_transit' | 'received' | 'returned'
  reason: string | null; requested_by_id: string; approved_by_id: string | null
  shipped_date: string | null; received_date: string | null; received_by_id: string | null
  bast_no: string | null; request_id: string | null
  condition_sent: TransferCondition | null; transfer_date: string | null; return_note: string | null
  asset_name: string | null; asset_tag: string | null
  from_office_name: string | null; to_office_name: string | null; to_room_name: string | null
  requested_by_name: string | null; received_by_name: string | null
  created_at: string | null; updated_at: string | null
}
export interface TransferSubmitInput {
  asset_id: string; to_office_id: string; to_room_id?: string | null
  reason?: string | null; condition_sent: TransferCondition; transfer_date: string
}
export interface ReceiveInput { bast_no?: string; received_date?: string; to_room_id?: string; file?: File | null }
useTransfers(): {
  list(q?: { status?: string, limit?: number, offset?: number }): Promise<{ data: Transfer[], total: number, limit: number, offset: number }>
  get(id: string): Promise<Transfer>
  submit(input: TransferSubmitInput): Promise<{ request_id: string, status: string }>
  ship(id: string, shippedDate?: string): Promise<Transfer>
  receive(id: string, input: ReceiveInput): Promise<Transfer>          // FormData multipart when file present, JSON otherwise
  rejectReceive(id: string, note?: string): Promise<Transfer>
}

// useDisposals.ts
export interface Disposal {
  id: string; asset_id: string
  method: DisposalMethod
  disposal_date: string | null; proceeds: string | null
  book_value_at_disposal: string | null; gain_loss: string | null
  bast_no: string | null; approved_by_id: string | null; request_id: string | null; created_by_id: string | null
  asset_name: string | null; asset_tag: string | null; office_name: string | null; created_by_name: string | null
  created_at: string | null; updated_at: string | null
}
export interface DisposalSubmitInput {
  asset_id: string; method: DisposalMethod; disposal_date: string
  proceeds?: string | null; book_value_at_disposal?: string | null
  bast_no?: string | null; reason?: string | null
}
export interface AttachDocumentInput { bast_no?: string; doc_no?: string; doc_date?: string; counterparty?: string; file?: File | null }
useDisposals(): {
  list(q?: { limit?: number, offset?: number }): Promise<{ data: Disposal[], total: number, limit: number, offset: number }>
  get(id: string): Promise<Disposal>
  submit(input: DisposalSubmitInput): Promise<{ request_id: string, status: string }>
  attachDocument(id: string, input: AttachDocumentInput): Promise<{ document_id: string, disposal_id: string }>
}

// useApprovalPreview.ts
export interface PreviewStep { step_order: number, required_level: string }
useApprovalPreview(): { preview(requestType: 'asset_disposal' | 'asset_transfer', amount: string): Promise<PreviewStep[]> }
// preview() GETs /approval-thresholds/preview and returns res.steps; errors propagate (422 = no band).
```

All via `useApiClient().request` (multipart: build `FormData`, do NOT set Content-Type manually — same pattern as `useAssetAttachments.upload`). Gate: `pnpm lint ; pnpm typecheck` clean (no consumers yet, so no transient red). Commit `feat(transfer,disposal): API composables incl. threshold preview`.

---

### Task 9: Shared frontend logic — `AssetSearchPicker`, region helper, history mergers (TDD)

**Files:**
- Create: `frontend/app/components/AssetSearchPicker.vue`
- Create: `frontend/app/utils/officeRegion.ts`, `frontend/app/utils/transferHistory.ts`, `frontend/app/utils/disposalHistory.ts`
- Test: `frontend/test/unit/office-region.spec.ts`, `frontend/test/unit/transfer-history.spec.ts`, `frontend/test/unit/disposal-history.spec.ts`, `frontend/test/nuxt/asset-search-picker.spec.ts`

**Interfaces (produced):**

```ts
// officeRegion.ts — pure, unit-tested
export interface OfficeNode { id: string, parent_id: string | null, office_type_id: string }
/** Climbs parents to the nearest ancestor whose office-type tier is 'wilayah'.
 *  Returns null when unresolvable (missing node, no wilayah ancestor, or a cycle). */
export function wilayahAncestor(officeId: string, nodes: Map<string, OfficeNode>, tierOf: (officeTypeId: string) => string | null | undefined): string | null
/** true = different regions, false = same region, null = cannot tell (render no alert). */
export function isInterRegion(a: string, b: string, nodes: Map<string, OfficeNode>, tierOf: (id: string) => string | null | undefined): boolean | null

// transferHistory.ts — merges approval requests + transfer rows into unified history rows
export type TransferHistoryStatus = 'diajukan' | 'ditolak_pengajuan' | 'dibatalkan' | 'approved' | 'in_transit' | 'received' | 'returned'
export interface TransferHistoryRow {
  key: string                       // request:<id> | transfer:<id>
  source: 'request' | 'transfer'
  id: string
  status: TransferHistoryStatus
  assetLabel: string                // enriched name+tag, or resolved target lookup, or '—'
  assetTag: string | null
  fromLabel: string; toLabel: string
  dateLabel: string                 // transfer_date → created_at fallback (pre-formatted)
  actorName: string | null
  bastNo: string | null
  interRegion: boolean | null
  canShip: boolean                  // status==='approved' && fromInScope && canManage
  raw: Transfer | ApprovalRequestRow
}
export function mergeTransferHistory(requests: ApprovalRequestRow[], transfers: Transfer[], opts: { fmtDate: (iso: string | null) => string, assetName: (targetId: string | null) => string | null, officeName: (id: string | null) => string | null, interRegion: (a: string, b: string) => boolean | null, canShip: (t: Transfer) => boolean }): TransferHistoryRow[]
// requests with status pending→'diajukan', rejected→'ditolak_pengajuan', cancelled→'dibatalkan';
// rows sorted by underlying created_at desc; transfer rows use enriched names with '—' fallbacks.

// disposalHistory.ts
export type DisposalHistoryStatus = 'menunggu' | 'ditolak' | 'dibatalkan' | 'selesai'
export interface DisposalHistoryRow {
  key: string; source: 'request' | 'disposal'; id: string
  status: DisposalHistoryStatus
  assetLabel: string; assetTag: string | null
  methodKey: DisposalMethod | null   // null for request rows (payload absent on list) → renders '—'
  proceeds: string | null; gainLoss: string | null
  dateLabel: string
  canAttach: boolean                 // source==='disposal' && canManage
  raw: Disposal | ApprovalRequestRow
}
export function mergeDisposalHistory(requests: ApprovalRequestRow[], disposals: Disposal[], opts: { fmtDate: (iso: string | null) => string, assetName: (targetId: string | null) => string | null, canAttach: boolean }): DisposalHistoryRow[]
```

```vue
<!-- AssetSearchPicker.vue — props/emits contract -->
props: { statuses: AssetStatus[], placeholder: string, hint?: string, disabled?: boolean }
emits: { 'select': (asset: Asset) => void }
```
Debounced (300ms) `useAssets().list({ search, status, limit: 20 })` per status in `statuses` (parallel, merged, de-duped by id) — dropdown rows: green dot + name + `tag · office` (office resolved via a `Map<string,string>` prop `officeNames`), empty state `t('common.assetPickerEmpty')` ("Tidak ada aset tersedia"), outside-click closes, selection emits the full Asset and fills the input.

- [ ] Steps: (1) write ALL unit tests first — officeRegion (same-region, inter-region, missing tier → null, cycle guard → null), transferHistory (request/row mapping per status incl. returned + sort + canShip false when not manage), disposalHistory (statuses + methodKey null on request rows) — with table-driven cases; (2) RED; (3) implement the three utils; (4) GREEN; (5) picker component + `mountSuspended` spec (stub `useAssets`: renders results, debounce with fake timers, empty state, emits select, disabled); (6) full `pnpm test` exit 0, lint/typecheck; (7) commit `feat(transfer,disposal): shared asset picker, region and history-merge helpers`.

---

### Task 10: Mutasi page (`/transfers`)

**Files:**
- Create: `frontend/app/pages/transfers.vue`
- Test: `frontend/test/nuxt/transfers.spec.ts`

**Interfaces:**
- Consumes: `useTransfers`, `useApproval().list` (requests type=asset_transfer), `useOffices().list`, `useFloors().listByOffice/roomsByFloor`, `useReference().list('office-types', …)` for tier (verify the exact key in `frontend/app/constants/referenceMeta` or `reference/resources.go` — it is the office-types resource key used by the Referensi screen), `AssetSearchPicker`, `mergeTransferHistory`, `wilayahAncestor/isInterRegion`, `TRANSFER_STATUS_TONE`/`CONDITION_*`, auth store `user.office_id`, `useCan('transfer.manage')`.

**Open the mockup first** (`docs/design/Mutasi Aset.dc.html` in a browser) — it is the visual source of truth. Build:

1. `definePageMeta({ middleware: 'can', permission: 'transfer.view' })`.
2. Header: title + caller office line (office name of `auth.user.office_id` resolved via offices map; hide the line when null).
3. **Flow legend** pill row: Diajukan → Disetujui → Dalam Pengiriman → Diterima (i18n; deviation (b) localizes `in_transfer`).
4. **Tabs** (persis mockup): Ajukan Mutasi / Kotak Masuk (badge = inbox count) / Riwayat.
5. **Ajukan tab**: `AssetSearchPicker` (statuses `['available']`, hint mockup); Kantor Asal read-only dashed (from selected asset office); Kantor Tujuan USelect (offices in scope minus origin); inter-region alert (violet) / same-region note (green) / nothing when `isInterRegion` returns null; Ruangan Tujuan (floors→rooms of destination, flattened, default "— (Belum ditentukan)"); Tanggal Mutasi (required); Kondisi Saat Dikirim (required, 3 options); Alasan textarea; Reset + submit button disabled until asset+destination+date+condition set; error banner text from mockup; submit → `useTransfers().submit()` → success banner (mockup template string) + form reset. Data-testids: `transfer-asset-picker`, `transfer-to-office`, `transfer-date`, `transfer-condition`, `transfer-submit`.
6. **Kotak Masuk tab**: `list({status:'in_transit', limit:100})` filtered client-side `to_office_id === auth.user.office_id` (when caller has an office; else show all rows returned by the scope-filtered list). Card per mockup: icon avatar, asset name + status badge + conditional Antar-Wilayah badge, mono tag, route row, "Diajukan oleh {requested_by_name}", condition badge, quoted reason. Actions (gated `transfer.manage`): **Terima** → `UModal` (received_date default today, room select of MY office via floors/rooms, bast_no input, optional file) → `receive()` → toast (mockup string) → reload inbox+history; **Tolak Terima** → modal (note textarea) → `rejectReceive()` → toast → reload. Empty state per mockup. Testids: `transfer-inbox-card`, `transfer-accept`, `transfer-reject-receive`.
7. **Riwayat tab**: fetch `useApproval().list({ type: 'asset_transfer', limit: 100 })` (filter statuses pending/rejected/cancelled client-side) + `useTransfers().list({limit:100})`, merge via `mergeTransferHistory`. Filter bar: search input (client-side over assetLabel/from/to) + status USelect (Semua + the 7 history statuses). Table columns per mockup: Aset (nama+tag mono, violet globe icon when interRegion), Asal → Tujuan, Tanggal, Pelaku (initials avatar + name), Status (dot+pill), No. BAST (mono text — deviation (f): NOT a link). `in_transit` rows get the info-tinted background. **Kirim** button on rows where `canShip` (deviation (a)): confirm modal with optional date → `ship()` → reload. Footer "Total {n} mutasi". Empty state per mockup. Testids: `transfer-history-row`, `transfer-ship`, `transfer-history-status`.
8. Loading skeletons + load-error/retry states on every fetch; all strings via i18n.

Component spec (`transfers.spec.ts`, stub all composables via `vi.mock`, ≥14 cases): mount loads inbox count + history; form disabled→enabled transitions; inter-region alert shown/hidden/absent per `isInterRegion` tri-state; submit calls composable with exact body + resets; inbox renders card fields + accept modal calls receive with FormData when file present; reject-receive sends note; history merges both sources (assert a `diajukan` row from requests + a `returned` row from transfers render with correct badges); ship button only on eligible rows and calls ship; status filter narrows; empty + error states; office line hidden when `office_id` null.

Gates: targeted spec green → full `pnpm test` exit 0 → lint/typecheck/build. Commit `feat(transfer): Mutasi Aset screen wired to /transfers (submit, inbox receive/reject, history + ship)`.

---

### Task 11: Penghapusan page (`/disposals`)

**Files:**
- Create: `frontend/app/pages/disposals.vue`
- Test: `frontend/test/nuxt/disposals.spec.ts`

**Interfaces:**
- Consumes: `useDisposals`, `useApproval().{list,get}`, `useApprovalPreview`, `useAssetAttachments` (evidence upload), `AssetSearchPicker` (statuses `['available','under_maintenance']`), `mergeDisposalHistory`, `METHOD_*`, `formatRupiah` (from `~/utils/format`), i18n `approval.level.*` (existing) for chain labels, `useCan('disposal.manage')`.

**Open the mockup first** (`docs/design/Penghapusan Aset.dc.html`). Build:

1. `definePageMeta({ middleware: 'can', permission: 'disposal.view' })`; title + subtitle per mockup.
2. **Tabs**: Ajukan Penghapusan / Riwayat.
3. **Ajukan tab — pre-submit** (grid `1fr 340px`, right sticky):
   - Card "Aset yang Dilepas": picker; once selected, **Ringkasan Valuasi** 2×2: Nilai Perolehan (`formatRupiah(asset.purchase_cost)`), Akumulasi Penyusutan (`− {v}` red), Nilai Buku chip PSAK (`book_value`), Nilai Buku chip FISKAL = "—" + tooltip "menunggu modul depresiasi" (deviation (d)). Masked money (absent field) renders "•••".
   - Card "Detail Pelepasan": Metode USelect (4 backend enum — deviation (c)); Nilai Jual/Terima (Rp input + mockup hint); Tanggal; No. BAST (mono); Alasan; **dropzone** foto bukti → per-file `useAssetAttachments().upload(asset.id, file)` immediately, rendered as removable chips (remove → `remove()`), hint "terlihat approver di Detail Aset" (decision #7 hybrid). Dropzone disabled until an asset is selected.
   - Right column: **Laba/Rugi card** — client compute `Number(proceeds) - Number(book_value)` (komersial); variants laba (green, `+ Rp …`), rugi (red, `− Rp …`), impas (neutral); breakdown rows per mockup + "Laba/rugi fiskal: —"; empty state when no asset/proceeds; when `book_value` masked/null → big value "—" + note. **Jenjang Persetujuan card** — on asset select call `useApprovalPreview().preview('asset_disposal', asset.purchase_cost)`; render row 1 "Maker — {auth.user.name}" (note "Pengaju") then one row per step: `t('approval.level.' + s.required_level)` + step number; subtitle "berdasar nilai perolehan: {formatRupiah(purchase_cost)}" (decision #4); 422 → "band approval belum dikonfigurasi"; `purchase_cost` absent (masked) → skip the call, render "nilai perolehan tersembunyi untuk peran Anda". Amber sensitive banner per mockup. Submit button (disabled until asset+date+method).
   - Submit → `useDisposals().submit({...})` with `book_value_at_disposal: asset.book_value ?? null` (read-only — never user-edited) → switch to **post-submit view**.
4. **Post-submit view** (replaces form): success banner; summary card (asset name, `tag · bast_no`, badge "Menunggu Approval", stat row Metode/Nilai Jual/Laba-Rugi — client-computed); **Timeline Approval Berlapis** from `useApproval().get(request_id)`: step done (green check, "Disetujui", `approver_name · decided_at`), current (`step_order === current_step`, amber clock, "Menunggu", "Menunggu tinjauan"), queued (muted, "Antre", "Menunggu tahap sebelumnya"); "Ajukan Penghapusan Lain" resets to the empty form.
5. **Riwayat tab**: `useApproval().list({type:'asset_disposal', limit:100})` (pending/rejected/cancelled) + `useDisposals().list({limit:100})` merged via `mergeDisposalHistory`. Filter: search + status select (Semua/Menunggu Approval/Ditolak/Dibatalkan/Selesai — deviation (h): no "Disetujui"). Columns: Aset, Metode (badge; request rows "—" per deviation (e)), Nilai Jual (right, "—" when null/0), Laba/Rugi (±colored `formatRupiah`), Tanggal, Status. **Lampirkan BAST** action on `canAttach` rows: modal (bast_no, doc_no, doc_date, counterparty, file) → `attachDocument()` → toast + reload. Footer "Total {n} pengajuan". Empty state per mockup.
6. Loading/error/empty states on every fetch; testids: `disposal-asset-picker`, `disposal-method`, `disposal-proceeds`, `disposal-date`, `disposal-submit`, `disposal-chain-card`, `disposal-gainloss-card`, `disposal-history-row`, `disposal-attach-bast`, `disposal-evidence-dropzone`.

Component spec (`disposals.spec.ts`, stubbed composables, ≥16 cases): valuation summary renders + masked "•••" + fiscal "—"; gain (green) / loss (red) / impas variants + empty state; chain card renders steps from preview + 422 fallback + masked-cost fallback; submit disabled/enabled + exact body incl. read-only book_value + switches to post-submit; timeline maps done/current/queued from steps+current_step; "Ajukan Penghapusan Lain" resets; evidence upload calls attachments composable + chip remove; history merge (menunggu/ditolak/selesai rows + method "—" on request rows); Lampirkan BAST modal sends multipart; filter; empty/error states.

Gates + commit `feat(disposal): Penghapusan Aset screen wired to /disposals (submit, gain/loss, chain preview, timeline, history + BAST)`.

---

### Task 12: E2E — transfer + disposal real-backend specs

**Files:**
- Create: `frontend/e2e/transfers.spec.ts`, `frontend/e2e/disposals.spec.ts`

Conventions: copy the file-local helpers (`authHeader`, `apiJson`, `login_`) and the serial-mode + unique-per-run (`RUN = Date.now()`) discipline from `frontend/e2e/approval.spec.ts`; UI login helper is `login(page)` (admin) — write a local `loginAs(page, email, password)` clone for the checker (see approval.spec.ts). Stack prerequisites: full Docker stack + seeded admin + `RATELIMIT_ENABLED=false`.

**transfers.spec.ts** (serial): beforeAll via API — office-type, TWO offices (A origin, B destination), category, one `available` asset at A (unique names), checker user (Superadmin role, SoD). Tests:
1. Submit via UI: `/transfers` → Ajukan tab → pick asset (type unique name into picker) → destination B → date → condition `rusak_ringan` → reason → submit → success banner; API-verify a pending `asset_transfer` request exists.
2. Approve via API as checker (`POST /requests/:id/approve`, body `{decision:'approve'}`) → UI Riwayat shows the row as "Disetujui" WITH the **Kirim** button → click → confirm modal → row becomes "Dalam Pengiriman".
3. As admin (whose office covers B — superadmin global): Kotak Masuk shows the card with condition badge "Rusak Ringan" → Terima modal (BAST no `BAST/E2E/${RUN}`) → toast → Riwayat row "Diterima" with the BAST number; API-verify the ASSET now lives at office B (`GET /assets/:id` office_id === B).
4. Reject-receive flow: second asset at A → submit → approve → ship (all via API using the composable-equivalent endpoints) → UI Kotak Masuk → Tolak Terima with note → row "Dikembalikan"; API-verify the asset still lives at office A.

**disposals.spec.ts** (serial): beforeAll — office, category, asset with `purchase_cost` in the LOWEST disposal band (see `db/migrations/000021_disposal_seed.up.sql` for the seeded bands — pick an amount inside band 1 so a single approval completes it), checker user. Tests:
1. Submit via UI: pick asset → valuation summary shows Perolehan + fiscal "—" → chain card renders ≥1 step ("berdasar nilai perolehan") → method Dijual → proceeds → date → submit → post-submit view with timeline (step 1 Menunggu).
2. Approve via API as checker → UI Riwayat: row "Selesai" with method badge + gain/loss; API-verify asset status === `disposed`.
3. Lampirkan BAST: on the Selesai row → modal → bast_no `BAP/E2E/${RUN}` + generated tiny file → submit → toast; API-verify via `GET /assets/:id/documents` a `bast_disposal` document exists.
4. Negative: history search for a nonsense string → empty state.

Gates: `pnpm lint ; pnpm typecheck`; run each new spec against the live stack (all pass); then FULL `pnpm test:e2e` exit 0 (classify any failure flake-vs-regression by isolated rerun; report honestly). Commit `test(transfer,disposal): real-backend e2e for both screens`.

---

### Task 13: Full gates, mockup side-by-side, PROGRESS.md

**Files:** `docs/PROGRESS.md`

1. Full gate sweep: backend `go build ./... ; go vet ./... ; go test ./...` + `go test -tags=integration ./...` (FULL module); Spectral; frontend `pnpm lint ; pnpm typecheck ; pnpm test ; pnpm build`; FULL `pnpm test:e2e`.
2. **Side-by-side (mandatory, both screens, light + dark)** with the Playwright MCP browser tools: mockup files (serve via a local static server — `file://` is blocked) vs `http://localhost:3000/transfers` and `/disposals` seeded with visible data (submit fixtures via API as in the e2e beforeAll). Checklist per mockup section (legend, tabs, form anatomy, inbox card anatomy, history columns, gain/loss variants, chain card, post-submit timeline, empty states). The ONLY allowed deviations are spec bagian 8 (a)–(i). Screenshot evidence to the session scratchpad. Fix any other gap before proceeding.
3. PROGRESS.md: mark *Next session* candidate **(d)** done (new numbered entry, dated, summarizing backend additions + both screens + e2e); update the two Bank-FAM `Remaining` bullets (transfer/disposal: "Frontend screen … still to build" → done notes mirroring previous entries' style); record deviations (a)–(i) per the catat-deviasi convention; add follow-ups — "switch disposal approval-amount basis to server-computed commercial book value once depreciation lands", "BAST link in Mutasi history when Dokumen BAST screen lands", "transfer/disposal money fields into field-permission catalog when needed"; refresh the "Next session — start here" pointer (remaining: stock opname / depreciation / assignment / maintenance / global search).
4. Commit `docs(progress): mark Mutasi + Penghapusan screens done`. Do NOT push / PR — the controller runs the final whole-branch review first.
