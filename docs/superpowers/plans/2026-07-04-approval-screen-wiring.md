# Approval Screen Wiring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the Pengajuan & Approval screen (`/approval`) from mock fixtures to the real `/api/v1/requests` backend (inbox + detail + approve/reject), with backend response enrichment (names + payload) and field-permission (FilterView) on the `requests` entity.

**Architecture:** Backend gains *enriched read* sqlc queries via `sqlc.embed` (request row + JOINed user/role/office names) so existing service/executor signatures stay intact; the handler serializes enrichment + payload (detail only) and applies `authz.FilterView` for entity `requests`. Frontend swaps `useApproval` to real `$fetch` (via `useApiClient`), moves type/status meta to `constants/approvalMeta.ts`, renders the Data section from `payload` via a pure `payloadToView` mapper, and maps the Pending tab to `GET /requests/inbox`.

**Tech Stack:** Go 1.25 + Gin + sqlc (pgx/v5) + testcontainers; Nuxt 4 + Nuxt UI + Vitest (`mountSuspended`) + Playwright.

**Spec:** `docs/superpowers/specs/2026-07-04-approval-screen-wiring-design.md` (read it first).

## Global Constraints

- Branch: `feat/approval-screen-wiring` (already created; spec committed).
- Conventional Commits, lowercase, imperative, scope per area: `feat(approval): ...`, `feat(authz): ...`, `docs(progress): ...`. **NEVER add Co-Authored-By / AI attribution.**
- Backend: never hand-edit `backend/db/sqlc/` — edit `db/queries/*.sql` and run `sqlc generate` (from `backend/`).
- Backend module split per ADR-0008: service = business logic + sentinel errors (no Gin); dto = serialization; handler = HTTP mapping.
- Frontend: ESLint `commaDangle: 'never'`, 1tbs braces; i18n mandatory (`i18n/locales/{id,en}.json`, no hardcoded UI strings); build on `U*` Nuxt UI components; API via `useApiClient` (never hardcode backend URL).
- All list endpoints return `{data, total, limit, offset}`.
- Verify per task: `go build ./... ; go vet ./... ; go test ./...` (backend) / `pnpm lint ; pnpm typecheck ; pnpm test` (frontend, from `frontend/`). Integration tests: `go test -tags=integration ./internal/approval/` (needs Docker running).
- Mockup fidelity: final screen must match `docs/design/Pengajuan Approval.dc.html` 1:1 **except** the 4 user-approved deviations listed in the spec §5.

---

### Task 1: Backend — enriched read queries + service + handler serialization

**Files:**
- Modify: `backend/db/queries/approval.sql` (append 4 queries)
- Generate: `backend/db/sqlc/` (via `sqlc generate`)
- Modify: `backend/internal/approval/service.go` (List, Inbox, GetWithSteps)
- Modify: `backend/internal/approval/dto.go` (new map helpers)
- Modify: `backend/internal/approval/handler.go` (list, inbox, get)
- Modify: `backend/internal/approval/integration_test.go` (new test + fix accessors in existing tests)

**Interfaces:**
- Consumes: existing `sqlc.Queries`, `requestToMap(sqlc.ApprovalRequest)`, helpers `resetAll`/`seedTieredOfficeTree`/`seedUser`/`lookupRole`/`buildCaller`/`seedCategory` in `integration_test.go`.
- Produces (used by Task 2 & 3):
  - `Service.List(...) ([]sqlc.ListRequestsEnrichedRow, int64, error)`
  - `Service.Inbox(...) ([]sqlc.ListInboxCandidatesEnrichedRow, error)`
  - `Service.GetWithSteps(...) (sqlc.GetRequestEnrichedRow, []sqlc.ListRequestApprovalsEnrichedRow, error)`
  - dto helpers `enrichRequestMap(m, name, role, office map[string]any-mutators)` and `stepToMap(row) map[string]any`
  - JSON contract: rows carry `requested_by_name`, `requested_by_role`, `office_name`; `GET /requests/:id` carries `payload` (object|null) and `steps[]` of `{step_order, required_level, approver_id, approver_name, decision, note, decided_at}`.

- [ ] **Step 1: Write the failing integration test**

Append to `backend/internal/approval/integration_test.go`:

```go
// TestApproval_EnrichedReads verifies List/Inbox/GetWithSteps return maker name,
// maker role, office name, and (detail) payload + per-step approver names.
func TestApproval_EnrichedReads(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "ENR")

	officeRoleID := lookupRole(t, pool, "Kepala Unit")
	maker := seedUser(t, pool, officeRoleID, "maker.enriched@test.local")
	approver := seedUser(t, pool, officeRoleID, "approver.enriched@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetCreate, assetSvc.CreateExecutor())

	catIDStr := catID.String()
	officeIDStr := tr.CabangID.String()
	payload, _ := json.Marshal(asset.AssetCreatePayload{
		Name: "Enriched Laptop", CategoryID: catIDStr, OfficeID: officeIDStr,
		AssetClass: "intangible", PurchaseCost: strPtr("1500000"),
	})
	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type: sqlc.SharedRequestTypeAssetCreate, Amount: "1500000",
		OfficeID: tr.CabangID, Payload: payload, Maker: maker,
	})
	require.NoError(t, err)

	t.Run("List rows carry names", func(t *testing.T) {
		rows, total, err := svc.List(ctx, true, nil, "pending", "asset_create", 20, 0)
		require.NoError(t, err)
		require.GreaterOrEqual(t, total, int64(1))
		require.NotEmpty(t, rows)
		row := rows[0]
		require.NotNil(t, row.RequestedByName)
		assert.Equal(t, "maker.enriched@test.local", *row.RequestedByName)
		require.NotNil(t, row.RequestedByRole)
		assert.Equal(t, "Kepala Unit", *row.RequestedByRole)
		require.NotNil(t, row.OfficeName)
		assert.Equal(t, "Cabang Alpha", *row.OfficeName)
	})

	t.Run("Inbox rows carry names", func(t *testing.T) {
		caller := buildCaller(approver, officeRoleID, true, nil)
		rows, err := svc.Inbox(ctx, caller)
		require.NoError(t, err)
		require.NotEmpty(t, rows)
		require.NotNil(t, rows[0].RequestedByName)
		assert.Equal(t, "maker.enriched@test.local", *rows[0].RequestedByName)
	})

	t.Run("GetWithSteps carries names, payload and approver name", func(t *testing.T) {
		// decide step 1 so a step has an approver
		caller := buildCaller(approver, officeRoleID, true, nil)
		_, err := svc.Decide(ctx, req.ID, caller, true, strPtr("ok"))
		require.NoError(t, err)

		row, steps, err := svc.GetWithSteps(ctx, req.ID)
		require.NoError(t, err)
		require.NotNil(t, row.RequestedByName)
		assert.Equal(t, "maker.enriched@test.local", *row.RequestedByName)
		require.NotNil(t, row.OfficeName)

		var p asset.AssetCreatePayload
		require.NoError(t, json.Unmarshal(row.ApprovalRequest.Payload, &p))
		assert.Equal(t, "Enriched Laptop", p.Name)

		require.NotEmpty(t, steps)
		decided := steps[0]
		require.NotNil(t, decided.ApproverName)
		assert.Equal(t, "approver.enriched@test.local", *decided.ApproverName)
		assert.Equal(t, int32(1), decided.ApprovalRequestApproval.StepOrder)
	})
}
```

If a `strPtr` helper does not already exist in this test file, add it near the other helpers:

```go
func strPtr(s string) *string { return &s }
```

(If a same-purpose helper already exists under another name, use that one instead and skip adding `strPtr`.)

- [ ] **Step 2: Run the test to verify it fails**

Run (from `backend/`): `go test -tags=integration ./internal/approval/ -run TestApproval_EnrichedReads`
Expected: **compile error** — `svc.List` returns `[]sqlc.ApprovalRequest` (no `.RequestedByName`), `sqlc.ListRequestsEnrichedRow` undefined. This is the correct RED (missing feature).

- [ ] **Step 3: Add the enriched queries**

Append to `backend/db/queries/approval.sql`:

```sql
-- Enriched read variants: request row + resolved maker/role/office names.
-- LEFT JOINs keep rows visible even when the user/office was soft-deleted.

-- name: GetRequestEnriched :one
SELECT sqlc.embed(r),
       u.name  AS requested_by_name,
       ro.name AS requested_by_role,
       o.name  AS office_name
FROM approval.requests r
LEFT JOIN identity.users u    ON u.id = r.requested_by_id AND u.deleted_at IS NULL
LEFT JOIN identity.roles ro   ON ro.id = u.role_id        AND ro.deleted_at IS NULL
LEFT JOIN masterdata.offices o ON o.id = r.office_id       AND o.deleted_at IS NULL
WHERE r.id = $1 AND r.deleted_at IS NULL;

-- name: ListRequestsEnriched :many
SELECT sqlc.embed(r),
       u.name  AS requested_by_name,
       ro.name AS requested_by_role,
       o.name  AS office_name
FROM approval.requests r
LEFT JOIN identity.users u    ON u.id = r.requested_by_id AND u.deleted_at IS NULL
LEFT JOIN identity.roles ro   ON ro.id = u.role_id        AND ro.deleted_at IS NULL
LEFT JOIN masterdata.offices o ON o.id = r.office_id       AND o.deleted_at IS NULL
WHERE r.deleted_at IS NULL
  AND (sqlc.arg(all_scope)::boolean OR r.office_id = ANY(sqlc.arg(office_ids)::uuid[]))
  AND (sqlc.narg(status)::shared.request_status IS NULL OR r.status = sqlc.narg(status))
  AND (sqlc.narg(type)::shared.request_type IS NULL OR r.type = sqlc.narg(type))
ORDER BY r.created_at DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: ListInboxCandidatesEnriched :many
SELECT sqlc.embed(r),
       u.name  AS requested_by_name,
       ro.name AS requested_by_role,
       o.name  AS office_name
FROM approval.requests r
LEFT JOIN identity.users u    ON u.id = r.requested_by_id AND u.deleted_at IS NULL
LEFT JOIN identity.roles ro   ON ro.id = u.role_id        AND ro.deleted_at IS NULL
LEFT JOIN masterdata.offices o ON o.id = r.office_id       AND o.deleted_at IS NULL
WHERE r.deleted_at IS NULL AND r.status = 'pending'
ORDER BY r.created_at ASC;

-- name: ListRequestApprovalsEnriched :many
SELECT sqlc.embed(a), u.name AS approver_name
FROM approval.request_approvals a
LEFT JOIN identity.users u ON u.id = a.approver_id AND u.deleted_at IS NULL
WHERE a.request_id = $1 AND a.deleted_at IS NULL
ORDER BY a.step_order;
```

Then run (from `backend/`): `sqlc generate` — expect new `ListRequestsEnrichedRow` etc. in `db/sqlc/` with an embedded `ApprovalRequest` / `ApprovalRequestApproval` field plus `RequestedByName/RequestedByRole/OfficeName/ApproverName *string`.

- [ ] **Step 4: Swap the service read methods to the enriched queries**

In `backend/internal/approval/service.go`:

`List` — change the query call and return type (params/filters identical):

```go
// List returns a paginated, scope-filtered slice of enriched requests plus the total count.
// Empty status/typ strings are treated as "no filter" (nil).
func (s *Service) List(ctx context.Context, all bool, ids []uuid.UUID, status, typ string, limit, offset int32) ([]sqlc.ListRequestsEnrichedRow, int64, error) {
	officeIDs := ids
	if officeIDs == nil {
		officeIDs = []uuid.UUID{}
	}
	var statusPtr *sqlc.SharedRequestStatus
	if status != "" {
		v := sqlc.SharedRequestStatus(status)
		statusPtr = &v
	}
	var typPtr *sqlc.SharedRequestType
	if typ != "" {
		v := sqlc.SharedRequestType(typ)
		typPtr = &v
	}
	rows, err := s.q.ListRequestsEnriched(ctx, sqlc.ListRequestsEnrichedParams{
		AllScope:  all,
		OfficeIds: officeIDs,
		Status:    statusPtr,
		Type:      typPtr,
		Off:       offset,
		Lim:       limit,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	total, err := s.q.CountRequests(ctx, sqlc.CountRequestsParams{
		AllScope:  all,
		OfficeIds: officeIDs,
		Status:    statusPtr,
		Type:      typPtr,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	return rows, total, nil
}
```

`Inbox` — iterate enriched candidates; eligibility logic unchanged, reading the embedded request:

```go
// Inbox returns all pending enriched requests for which the caller is currently
// eligible to decide.
func (s *Service) Inbox(ctx context.Context, caller Caller) ([]sqlc.ListInboxCandidatesEnrichedRow, error) {
	candidates, err := s.q.ListInboxCandidatesEnriched(ctx)
	if err != nil {
		return nil, mapDBError(err)
	}
	out := make([]sqlc.ListInboxCandidatesEnrichedRow, 0)
	for _, row := range candidates {
		req := row.ApprovalRequest
		approvals, err := s.q.ListRequestApprovals(ctx, req.ID)
		if err != nil {
			return nil, mapDBError(err)
		}
		var step sqlc.ApprovalRequestApproval
		var prior []uuid.UUID
		found := false
		for _, a := range approvals {
			if a.StepOrder < req.CurrentStep && a.ApproverID != nil {
				prior = append(prior, *a.ApproverID)
			}
			if a.StepOrder == req.CurrentStep {
				step = a
				found = true
			}
		}
		// Submit guarantees office_id is non-nil; the nil check here is purely defensive.
		if !found || req.OfficeID == nil {
			continue
		}
		anc, err := s.ancestorsFor(ctx, *req.OfficeID)
		if err != nil {
			return nil, err
		}
		to, ok := resolveTierOffice(anc, *req.OfficeID, step.RequiredLevel)
		if eligibleToDecide(caller, req, step, prior, to, ok) == nil {
			out = append(out, row)
		}
	}
	return out, nil
}
```

`GetWithSteps` — enriched get + enriched steps:

```go
// GetWithSteps fetches a single enriched approval request and its ordered,
// approver-name-enriched approval steps.
func (s *Service) GetWithSteps(ctx context.Context, id uuid.UUID) (sqlc.GetRequestEnrichedRow, []sqlc.ListRequestApprovalsEnrichedRow, error) {
	r, err := s.q.GetRequestEnriched(ctx, id)
	if err != nil {
		return r, nil, mapDBError(err)
	}
	steps, err := s.q.ListRequestApprovalsEnriched(ctx, id)
	if err != nil {
		return r, nil, mapDBError(err)
	}
	return r, steps, nil
}
```

Do NOT touch `Submit`, `Decide`, `Cancel`, executors, or threshold methods.

- [ ] **Step 5: Serialize enrichment in dto.go**

Append to `backend/internal/approval/dto.go`:

```go
// enrichRequestMap adds the resolved maker/role/office names to a serialized request.
func enrichRequestMap(m map[string]any, requestedByName, requestedByRole, officeName *string) map[string]any {
	m["requested_by_name"] = requestedByName
	m["requested_by_role"] = requestedByRole
	m["office_name"] = officeName
	return m
}

// stepToMap serializes an enriched approval step for API responses. Internal
// row-keeping columns (id, request_id, timestamps' bookkeeping) are omitted.
func stepToMap(row sqlc.ListRequestApprovalsEnrichedRow) map[string]any {
	a := row.ApprovalRequestApproval
	return map[string]any{
		"step_order":     a.StepOrder,
		"required_level": string(a.RequiredLevel),
		"approver_id":    common.UUIDPtrStr(a.ApproverID),
		"approver_name":  row.ApproverName,
		"decision":       string(a.Decision),
		"note":           a.Note,
		"decided_at":     common.TsStr(a.DecidedAt),
	}
}
```

- [ ] **Step 6: Update the handler (list, inbox, get)**

In `backend/internal/approval/handler.go`:

`list` — rows are now enriched:

```go
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, enrichRequestMap(requestToMap(r.ApprovalRequest), r.RequestedByName, r.RequestedByRole, r.OfficeName))
	}
```

`inbox` — same pattern:

```go
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, enrichRequestMap(requestToMap(r.ApprovalRequest), r.RequestedByName, r.RequestedByRole, r.OfficeName))
	}
```

`get` — enriched row, payload, explicit step maps (add `"encoding/json"` to imports):

```go
// get handles GET /requests/:id (returns enriched request + payload + its approval steps).
func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	row, steps, err := h.svc.GetWithSteps(c, id)
	if err != nil {
		h.svcError(c, err)
		return
	}
	r := row.ApprovalRequest
	// Enforce data scope: the caller may only view requests within their office scope.
	all, ids, err := h.scoped.CallerOfficeScope(c, "requests")
	if err != nil {
		common.WriteError(c, err)
		return
	}
	if r.OfficeID == nil || !common.InScope(all, ids, *r.OfficeID) {
		common.WriteError(c, common.ErrForbidden)
		return
	}
	out := enrichRequestMap(requestToMap(r), row.RequestedByName, row.RequestedByRole, row.OfficeName)
	var payload any
	if len(r.Payload) > 0 {
		_ = json.Unmarshal(r.Payload, &payload)
	}
	out["payload"] = payload
	stepMaps := make([]map[string]any, 0, len(steps))
	for _, st := range steps {
		stepMaps = append(stepMaps, stepToMap(st))
	}
	out["steps"] = stepMaps
	c.JSON(http.StatusOK, out)
}
```

- [ ] **Step 7: Fix compile fallout in existing integration tests**

Run (from `backend/`): `go build ./... ; go vet ./... ; go test ./...`
Then: `go test -tags=integration ./internal/approval/ -run 'TestApproval' 2>&1 | head -50` — existing tests that call `svc.List(...)` or `svc.Inbox(...)` and index row fields directly (e.g. `rows[0].Type`, `rows[0].ID`) must switch to the embedded accessor (`rows[0].ApprovalRequest.Type`). `svc.GetWithSteps` callers likewise. Fix ONLY accessor paths — no assertion semantics change.

- [ ] **Step 8: Run the new test to verify it passes**

Run: `go test -tags=integration ./internal/approval/ -run TestApproval_EnrichedReads`
Expected: PASS.
Then the full package: `go test -tags=integration ./internal/approval/` — all PASS.

- [ ] **Step 9: Commit**

```bash
git add backend/db/queries/approval.sql backend/db/sqlc backend/internal/approval
git commit -m "feat(approval): enrich request reads with maker/office names and expose payload+steps on detail"
```

---

### Task 2: Backend — FilterView on entity `requests`

**Files:**
- Modify: `backend/internal/approval/handler.go` (filterMap helper + apply in list/inbox/get)
- Modify: `backend/internal/approval/integration_test.go` (new test)

**Interfaces:**
- Consumes: `h.fieldSvc *authz.FieldService` (already on Handler), `authz.FilterView(policies, m)`, Task 1's serialization.
- Produces: entity key **`requests`** with maskable field keys **`amount`, `payload`, `reason`** (frontend Task 4 lists exactly these in `fieldCatalog.ts`).

- [ ] **Step 1: Write the failing integration test**

Append to `backend/internal/approval/integration_test.go` (model on `TestAsset_FieldMasking_ByRole` in `internal/asset/integration_test.go`):

```go
// TestApproval_FieldMasking_Requests verifies FilterView on entity "requests":
// a role denied view on amount/payload loses those keys; default-allow otherwise.
func TestApproval_FieldMasking_Requests(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	q := sqlc.New(pool)
	fieldSvc := authz.NewFieldService(q, rdb)

	stafRoleID := lookupRole(t, pool, "Staf")
	adminRoleID := lookupRole(t, pool, "Superadmin")

	// Deny view on amount + payload for Staf on entity "requests".
	_, err := pool.Exec(ctx, `
		INSERT INTO identity.field_permissions (role_id, entity, field, can_view, can_edit)
		VALUES ($1, 'requests', 'amount', false, false),
		       ($1, 'requests', 'payload', false, false)`, stafRoleID)
	require.NoError(t, err)

	sample := func() map[string]any {
		return map[string]any{
			"id": uuid.New().String(), "type": "asset_create", "status": "pending",
			"amount": "5000000", "payload": map[string]any{"name": "X"}, "reason": "r",
		}
	}

	t.Run("denied role loses amount and payload", func(t *testing.T) {
		rec := sample()
		pol, err := fieldSvc.ForEntity(ctx, stafRoleID, "requests")
		require.NoError(t, err)
		authz.FilterView(pol, rec)
		assert.NotContains(t, rec, "amount")
		assert.NotContains(t, rec, "payload")
		assert.Contains(t, rec, "reason") // no policy → default-allow
	})

	t.Run("role without policy keeps everything", func(t *testing.T) {
		rec := sample()
		pol, err := fieldSvc.ForEntity(ctx, adminRoleID, "requests")
		require.NoError(t, err)
		authz.FilterView(pol, rec)
		assert.Contains(t, rec, "amount")
		assert.Contains(t, rec, "payload")
	})
}
```

Note: if `identity.field_permissions` uses different column names, check migration `000004`/DATABASE.md and adjust the INSERT — the assertion body stays the same.

- [ ] **Step 2: Run test to verify current state**

Run: `go test -tags=integration ./internal/approval/ -run TestApproval_FieldMasking_Requests`
This test exercises `FieldService` directly, so it may already PASS — that proves the authz layer; the missing piece is the handler wiring. If it passes, continue (the handler-wiring proof is Step 3's code + Task 8's e2e); if it fails, fix the INSERT per the note.

- [ ] **Step 3: Wire filterMap into the handler**

In `backend/internal/approval/handler.go` add (mirrors `internal/asset/handler.go:36`; add imports `"github.com/ragbuaj/inventra/internal/authz"` is already there via fieldSvc — verify):

```go
// filterMap applies field-permission masking for the caller's role on the
// "requests" entity. Fails closed on ForEntity errors so sensitive amounts
// are never leaked when the policy store is unavailable.
func (h *Handler) filterMap(c *gin.Context, m map[string]any) (map[string]any, error) {
	roleID, err := uuid.Parse(c.GetString(middleware.CtxRoleID))
	if err != nil {
		return m, nil
	}
	policies, err := h.fieldSvc.ForEntity(c.Request.Context(), roleID, "requests")
	if err != nil {
		return nil, err
	}
	if policies != nil {
		authz.FilterView(policies, m)
	}
	return m, nil
}
```

Apply in `list` and `inbox` (replace the plain append from Task 1):

```go
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		m, err := h.filterMap(c, enrichRequestMap(requestToMap(r.ApprovalRequest), r.RequestedByName, r.RequestedByRole, r.OfficeName))
		if err != nil {
			common.WriteError(c, err)
			return
		}
		data = append(data, m)
	}
```

Apply in `get` right before `c.JSON` (after `out["steps"] = stepMaps`):

```go
	out, err = h.filterMap(c, out)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
```

- [ ] **Step 4: Verify green**

Run: `go build ./... ; go vet ./... ; go test ./...` then `go test -tags=integration ./internal/approval/`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/approval
git commit -m "feat(authz): enforce field permissions (FilterView) on approval request reads"
```

---

### Task 3: OpenAPI sync

**Files:**
- Modify: `backend/api/openapi.yaml`

**Interfaces:**
- Consumes: Task 1/2 JSON contract.
- Produces: schemas `Request` (enriched) and `RequestDetail` + `RequestStep`.

- [ ] **Step 1: Extend the `Request` schema**

Find `Request:` under `components.schemas` and add to its `properties` (types follow the existing nullable-string style used in the file):

```yaml
        requested_by_name:
          type: [string, "null"]
          description: Resolved maker display name (null if the user was deleted).
        requested_by_role:
          type: [string, "null"]
          description: Resolved maker role name.
        office_name:
          type: [string, "null"]
          description: Resolved originating office name.
```

- [ ] **Step 2: Add `RequestStep` + `RequestDetail` schemas and point `GET /requests/{id}` at it**

```yaml
    RequestStep:
      type: object
      description: One approval-chain step of a request.
      properties:
        step_order: { type: integer }
        required_level:
          type: string
          enum: [office, office_subtree, wilayah, pusat]
        approver_id: { type: [string, "null"], format: uuid }
        approver_name: { type: [string, "null"] }
        decision:
          type: string
          enum: [pending, approved, rejected, cancelled]
        note: { type: [string, "null"] }
        decided_at: { type: [string, "null"], format: date-time }

    RequestDetail:
      allOf:
        - $ref: "#/components/schemas/Request"
        - type: object
          properties:
            payload:
              type: [object, "null"]
              additionalProperties: true
              description: >-
                Raw request payload as submitted (AssetCreatePayload /
                DisposalPayload / TransferPayload shape by type). Absent when
                masked by field permissions.
            steps:
              type: array
              items: { $ref: "#/components/schemas/RequestStep" }
```

Update the `GET /api/v1/requests/{id}` 200 response `$ref` from `Request` to `RequestDetail`, and mention field masking in the `GET /requests` description (append: "Fields `amount`/`payload`/`reason` may be omitted per role field-permissions.").

- [ ] **Step 3: Lint**

Run (repo root): `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
Expected: 0 errors (the pre-existing `AssetCreatePayload` unused-component warning may disappear if you now `$ref` it from `RequestDetail.payload` description — do NOT force it; leaving the warning is fine).

- [ ] **Step 4: Commit**

```bash
git add backend/api/openapi.yaml
git commit -m "docs(api): document enriched request reads, request detail steps and payload"
```

---

### Task 4: Frontend — approvalMeta constants, fieldCatalog, i18n keys

**Files:**
- Create: `frontend/app/constants/approvalMeta.ts`
- Modify: `frontend/app/constants/fieldCatalog.ts`
- Modify: `frontend/i18n/locales/id.json`, `frontend/i18n/locales/en.json` (the `approval` section + `search.type` untouched)
- Create: `frontend/test/unit/approval-meta.spec.ts`

**Interfaces:**
- Produces: `RequestType`, `RequestStatus`, `REQUEST_TYPE_KEYS`, `TYPE_META`, `STATUS_TONE`, `STATUS_FILTERS` — consumed by Tasks 5–7. i18n keys `approval.type.asset_create|asset_disposal|asset_transfer|valuation_exclusion`, `approval.status.cancelled`, `approval.filter.cancelled`, `approval.notEligible`, `approval.noData`, `approval.resultCancelled`, `approval.loadError`, `approval.retry`, `approval.field.*` (see Step 3), `approval.action.cancelled`, `approval.action.pendingStep`.

- [ ] **Step 1: Write the failing unit test**

`frontend/test/unit/approval-meta.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { REQUEST_TYPE_KEYS, TYPE_META, STATUS_TONE, STATUS_FILTERS } from '~/constants/approvalMeta'

describe('constants/approvalMeta', () => {
  it('covers exactly the 4 submittable backend request types', () => {
    expect(REQUEST_TYPE_KEYS).toEqual(['asset_create', 'asset_disposal', 'asset_transfer', 'valuation_exclusion'])
  })

  it('marks disposal and valuation exclusion as sensitive', () => {
    expect(TYPE_META.asset_disposal.sensitive).toBe(true)
    expect(TYPE_META.valuation_exclusion.sensitive).toBe(true)
    expect(TYPE_META.asset_create.sensitive).toBe(false)
    expect(TYPE_META.asset_transfer.sensitive).toBe(false)
  })

  it('has a tone for every status incl. cancelled and a cancelled filter tab', () => {
    expect(STATUS_TONE.cancelled).toBe('neutral')
    expect(STATUS_FILTERS).toEqual(['pending', 'approved', 'rejected', 'cancelled', 'all'])
  })

  it('every type has an icon and tone', () => {
    for (const k of REQUEST_TYPE_KEYS) {
      expect(TYPE_META[k].icon).toMatch(/^i-lucide-/)
      expect(TYPE_META[k].tone).toBeTruthy()
    }
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `pnpm vitest run test/unit/approval-meta.spec.ts`
Expected: FAIL — module `~/constants/approvalMeta` not found.

- [ ] **Step 3: Implement**

`frontend/app/constants/approvalMeta.ts`:

```ts
import type { BadgeColor } from '~/types'

/** Backend shared.request_type values that currently have a submit path. */
export type RequestType = 'asset_create' | 'asset_disposal' | 'asset_transfer' | 'valuation_exclusion'
/** Backend shared.request_status values. */
export type RequestStatus = 'pending' | 'approved' | 'rejected' | 'cancelled'

export const REQUEST_TYPE_KEYS: RequestType[] = ['asset_create', 'asset_disposal', 'asset_transfer', 'valuation_exclusion']

export const TYPE_META: Record<RequestType, { icon: string, tone: BadgeColor, sensitive: boolean }> = {
  asset_create: { icon: 'i-lucide-package', tone: 'info', sensitive: false },
  asset_disposal: { icon: 'i-lucide-trash-2', tone: 'error', sensitive: true },
  asset_transfer: { icon: 'i-lucide-arrow-right-left', tone: 'primary', sensitive: false },
  valuation_exclusion: { icon: 'i-lucide-coins', tone: 'warning', sensitive: true }
}

export const STATUS_TONE: Record<RequestStatus, BadgeColor> = {
  pending: 'warning',
  approved: 'success',
  rejected: 'error',
  cancelled: 'neutral'
}

export const STATUS_FILTERS: (RequestStatus | 'all')[] = ['pending', 'approved', 'rejected', 'cancelled', 'all']
```

Add to `frontend/app/constants/fieldCatalog.ts` (after the `users` entry):

```ts
  {
    entity: 'requests',
    fields: ['amount', 'payload', 'reason']
  }
```

Update `frontend/i18n/locales/id.json` `approval` section — REPLACE the `type` block, EXTEND `filter`/`status`/`action`, and ADD new keys (keep all existing keys that remain referenced; the old `type.registrasi`-style keys stay for the mock-store tests only if still referenced — if nothing references them after Task 7, delete them):

```json
"type": {
  "asset_create": "Registrasi Aset",
  "asset_disposal": "Penghapusan Aset",
  "asset_transfer": "Mutasi Aset",
  "valuation_exclusion": "Pengecualian Valuasi"
},
"filter": {
  "pending": "Menunggu",
  "approved": "Disetujui",
  "rejected": "Ditolak",
  "cancelled": "Dibatalkan",
  "all": "Semua"
},
"status": {
  "pending": "Menunggu",
  "approved": "Disetujui",
  "rejected": "Ditolak",
  "cancelled": "Dibatalkan"
},
"action": {
  "submitted": "Mengajukan permintaan",
  "approved": "Menyetujui pengajuan",
  "rejected": "Menolak pengajuan",
  "cancelled": "Membatalkan pengajuan",
  "pending": "Menunggu persetujuan Anda",
  "pendingStep": "Menunggu persetujuan step {n} ({level})"
},
"notEligible": "Pengajuan ini menunggu approver lain atau di luar wewenang Anda.",
"noData": "Data pengajuan tidak tersedia.",
"resultCancelled": "Dibatalkan oleh pengaju",
"loadError": "Gagal memuat pengajuan.",
"retry": "Coba lagi",
"level": {
  "office": "Kantor",
  "office_subtree": "Kantor & Bawahan",
  "wilayah": "Kanwil",
  "pusat": "Kantor Pusat"
},
"field": {
  "assetName": "Nama Aset",
  "category": "Kategori",
  "assetClass": "Kelas Aset",
  "purchaseCost": "Biaya Perolehan",
  "purchaseDate": "Tanggal Perolehan",
  "serialNumber": "Nomor Seri",
  "poNumber": "Nomor PO",
  "fundingSource": "Sumber Dana",
  "assetStatus": "Status Aset",
  "method": "Metode",
  "disposalDate": "Tanggal Penghapusan",
  "proceeds": "Hasil Penjualan",
  "bookValue": "Nilai Buku",
  "bastNo": "No. BAST",
  "fromOffice": "Kantor Asal",
  "toOffice": "Kantor Tujuan",
  "toRoom": "Ruangan Tujuan",
  "valuation": "Status Valuasi",
  "active": "Aktif",
  "disposed": "Dihapus",
  "included": "Dihitung",
  "excluded": "Dikecualikan"
}
```

Mirror the same structure in `frontend/i18n/locales/en.json` with English values ("Asset Registration", "Asset Disposal", "Asset Transfer", "Valuation Exclusion", "Cancelled", "This request awaits another approver or is outside your authority.", "Request data unavailable.", "Cancelled by requester", "Waiting for step {n} approval ({level})", field labels "Asset Name"/"Category"/"Asset Class"/"Purchase Cost"/"Purchase Date"/"Serial Number"/"PO Number"/"Funding Source"/"Asset Status"/"Method"/"Disposal Date"/"Proceeds"/"Book Value"/"BAST No."/"From Office"/"To Office"/"To Room"/"Valuation"/"Active"/"Disposed"/"Included"/"Excluded", level labels "Office"/"Office & Subtree"/"Regional Office"/"Head Office").

- [ ] **Step 4: Run tests + typecheck**

Run: `pnpm vitest run test/unit/approval-meta.spec.ts` → PASS; `pnpm typecheck` → clean; `pnpm lint` → clean.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/constants frontend/i18n frontend/test/unit/approval-meta.spec.ts
git commit -m "feat(approval): real request-type/status meta constants, requests field catalog, i18n keys"
```

---

### Task 5: Frontend — rewrite `useApproval` to the real API

**Files:**
- Rewrite: `frontend/app/composables/api/useApproval.ts`

**Interfaces:**
- Consumes: `useApiClient().request<T>()`; types from `~/constants/approvalMeta`.
- Produces (consumed by Tasks 6–7):

```ts
useApproval(): {
  inbox(): Promise<ApprovalRequestRow[]>
  list(q?: ApprovalListQuery): Promise<ApprovalListPage>
  get(id: string): Promise<ApprovalRequestDetail>
  approve(id: string, note?: string): Promise<ApprovalRequestRow>
  reject(id: string, note?: string): Promise<ApprovalRequestRow>
}
```

- [ ] **Step 1: Rewrite the composable**

Replace the full contents of `frontend/app/composables/api/useApproval.ts`:

```ts
import type { RequestType, RequestStatus } from '~/constants/approvalMeta'

export interface ApprovalRequestRow {
  id: string
  type: RequestType
  status: RequestStatus
  amount?: string | null
  current_step: number
  office_id: string | null
  office_name: string | null
  target_id: string | null
  target_entity: string | null
  reason?: string | null
  requested_by_id: string
  requested_by_name: string | null
  requested_by_role: string | null
  decided_by_id: string | null
  decision_note: string | null
  created_at: string | null
}

export interface ApprovalStep {
  step_order: number
  required_level: string
  approver_id: string | null
  approver_name: string | null
  decision: RequestStatus
  note: string | null
  decided_at: string | null
}

export interface ApprovalRequestDetail extends ApprovalRequestRow {
  /** Raw submitted payload; absent/undefined when masked by field permissions. */
  payload?: Record<string, unknown> | null
  steps: ApprovalStep[]
}

export interface ApprovalListQuery {
  status?: RequestStatus
  type?: RequestType
  limit?: number
  offset?: number
}

export interface ApprovalListPage {
  data: ApprovalRequestRow[]
  total: number
  limit: number
  offset: number
}

/** Approval inbox + decisions, wired to /api/v1/requests. */
export function useApproval() {
  const { request } = useApiClient()

  async function inbox(): Promise<ApprovalRequestRow[]> {
    const res = await request<{ data: ApprovalRequestRow[], total: number }>('/requests/inbox')
    return res.data
  }

  async function list(q: ApprovalListQuery = {}): Promise<ApprovalListPage> {
    const query: Record<string, string | number> = {}
    if (q.status) query.status = q.status
    if (q.type) query.type = q.type
    if (q.limit !== undefined) query.limit = q.limit
    if (q.offset !== undefined) query.offset = q.offset
    return request<ApprovalListPage>('/requests', { query })
  }

  async function get(id: string): Promise<ApprovalRequestDetail> {
    return request<ApprovalRequestDetail>(`/requests/${id}`)
  }

  // The backend DecideRequest binding requires `decision` whenever a body is
  // present (only a fully-empty body is tolerated), so both calls send it
  // explicitly even though the endpoint is already action-specific.
  // NOTE: decide responses are the PLAIN request serialization (no
  // requested_by_name/office_name enrichment) — callers must not rely on the
  // enrichment fields here; the page refreshes via loadTab()+get() instead.
  async function approve(id: string, note?: string): Promise<ApprovalRequestRow> {
    return request<ApprovalRequestRow>(`/requests/${id}/approve`, {
      method: 'POST',
      body: { decision: 'approve', note: note || undefined }
    })
  }

  async function reject(id: string, note?: string): Promise<ApprovalRequestRow> {
    return request<ApprovalRequestRow>(`/requests/${id}/reject`, {
      method: 'POST',
      body: { decision: 'reject', note: note || undefined }
    })
  }

  return { inbox, list, get, approve, reject }
}
```

- [ ] **Step 2: Verify compile state**

Run: `pnpm typecheck`
Expected: **errors only in `app/pages/approval.vue` and `test/nuxt/approval.spec.ts`** (they still consume the old mock-shaped API — fixed in Task 7). If other files error, they are unnoticed consumers: stop and fix them per the wiring-composable lesson (`useGlobalSearch` must NOT be affected — it imports `approvalStore`, not `useApproval`).

- [ ] **Step 3: Commit**

```bash
git add frontend/app/composables/api/useApproval.ts
git commit -m "feat(approval): rewrite useApproval composable to real /requests API"
```

(The build is transiently red on the page/test — acceptable mid-branch; Task 7 restores green. If you prefer atomic green commits, squash Tasks 5–7 locally before push.)

---

### Task 6: Frontend — `payloadToView` mapper + rupiah util (TDD)

**Files:**
- Create: `frontend/app/utils/money.ts`
- Create: `frontend/app/utils/approvalPayload.ts`
- Create: `frontend/test/unit/approval-payload.spec.ts`

**Interfaces:**
- Consumes: `ApprovalRequestDetail` from Task 5; i18n `approval.field.*` keys from Task 4.
- Produces:

```ts
formatRupiah(v: string | number | null | undefined): string   // '—' when null/invalid
payloadToView(detail: ApprovalRequestDetail, t: (k: string, p?: Record<string, unknown>) => string, lookups?: PayloadLookups): PayloadView
interface PayloadLookups { categoryName?: (id: string) => string | undefined, officeName?: (id: string) => string | undefined }
type PayloadView = { layout: 'summary', rows: { label: string, value: string }[] } | { layout: 'diff', rows: { label: string, before: string, after: string }[] }
```

- [ ] **Step 1: Write the failing tests**

`frontend/test/unit/approval-payload.spec.ts`:

```ts
import { describe, it, expect } from 'vitest'
import { formatRupiah } from '~/utils/money'
import { payloadToView } from '~/utils/approvalPayload'
import type { ApprovalRequestDetail } from '~/composables/api/useApproval'

const t = (k: string, p?: Record<string, unknown>) => p ? `${k}:${JSON.stringify(p)}` : k

function detail(partial: Partial<ApprovalRequestDetail>): ApprovalRequestDetail {
  return {
    id: 'x', type: 'asset_create', status: 'pending', current_step: 1,
    office_id: 'o1', office_name: 'Cabang A', target_id: null, target_entity: null,
    requested_by_id: 'u1', requested_by_name: 'Andi', requested_by_role: 'Kepala Unit',
    decided_by_id: null, decision_note: null, created_at: '2026-07-04T09:00:00Z',
    steps: [],
    ...partial
  }
}

describe('formatRupiah', () => {
  it('formats a decimal string as IDR without fraction digits', () => {
    expect(formatRupiah('1500000')).toMatch(/^Rp\s?1\.500\.000$/)
  })
  it('returns em-dash for null, undefined and non-numeric input', () => {
    expect(formatRupiah(null)).toBe('—')
    expect(formatRupiah(undefined)).toBe('—')
    expect(formatRupiah('abc')).toBe('—')
  })
})

describe('payloadToView — asset_create', () => {
  it('maps the payload into summary rows with resolved names', () => {
    const v = payloadToView(detail({
      payload: {
        name: 'Laptop A', category_id: 'c1', asset_class: 'tangible',
        purchase_cost: '1500000', purchase_date: '2026-07-01', serial_number: 'SN1'
      }
    }), t, { categoryName: id => (id === 'c1' ? 'Elektronik' : undefined) })
    expect(v.layout).toBe('summary')
    const byLabel = Object.fromEntries(v.rows.map(r => [r.label, (r as { value: string }).value]))
    expect(byLabel['approval.field.assetName']).toBe('Laptop A')
    expect(byLabel['approval.field.category']).toBe('Elektronik')
    expect(byLabel['approval.field.purchaseCost']).toMatch(/1\.500\.000/)
  })

  it('falls back to the raw id when a lookup misses', () => {
    const v = payloadToView(detail({ payload: { name: 'X', category_id: 'c9' } }), t)
    const cat = v.rows.find(r => r.label === 'approval.field.category')
    expect((cat as { value: string } | undefined)?.value).toBe('c9')
  })

  it('returns empty rows for a null/masked payload', () => {
    expect(payloadToView(detail({ payload: null }), t).rows).toEqual([])
    expect(payloadToView(detail({}), t).rows).toEqual([])
  })
})

describe('payloadToView — asset_disposal', () => {
  it('renders a static status diff plus payload fields', () => {
    const v = payloadToView(detail({
      type: 'asset_disposal',
      payload: { method: 'sale', disposal_date: '2026-07-01', proceeds: '500000' }
    }), t)
    expect(v.layout).toBe('diff')
    const status = v.rows.find(r => r.label === 'approval.field.assetStatus') as { before: string, after: string }
    expect(status.before).toBe('approval.field.active')
    expect(status.after).toBe('approval.field.disposed')
    const method = v.rows.find(r => r.label === 'approval.field.method') as { after: string }
    expect(method.after).toBe('sale')
  })

  it('keeps the static status row even when the payload is missing', () => {
    const v = payloadToView(detail({ type: 'asset_disposal', payload: null }), t)
    expect(v.layout).toBe('diff')
    expect(v.rows.some(r => r.label === 'approval.field.assetStatus')).toBe(true)
  })
})

describe('payloadToView — asset_transfer', () => {
  it('maps offices through the lookup with raw-id fallback', () => {
    const v = payloadToView(detail({
      type: 'asset_transfer',
      payload: { from_office_id: 'o1', to_office_id: 'o2', reason: 'relokasi' }
    }), t, { officeName: id => (id === 'o1' ? 'Cabang A' : undefined) })
    expect(v.layout).toBe('summary')
    const byLabel = Object.fromEntries(v.rows.map(r => [r.label, (r as { value: string }).value]))
    expect(byLabel['approval.field.fromOffice']).toBe('Cabang A')
    expect(byLabel['approval.field.toOffice']).toBe('o2')
  })
})

describe('payloadToView — valuation_exclusion', () => {
  it('is a static diff that needs no payload', () => {
    const v = payloadToView(detail({ type: 'valuation_exclusion', payload: null }), t)
    expect(v.layout).toBe('diff')
    const row = v.rows[0] as { label: string, before: string, after: string }
    expect(row.label).toBe('approval.field.valuation')
    expect(row.before).toBe('approval.field.included')
    expect(row.after).toBe('approval.field.excluded')
  })
})
```

- [ ] **Step 2: Run to verify failure**

Run: `pnpm vitest run test/unit/approval-payload.spec.ts`
Expected: FAIL — modules not found.

- [ ] **Step 3: Implement**

`frontend/app/utils/money.ts`:

```ts
const idr = new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR', maximumFractionDigits: 0 })

/** Formats a backend decimal string / number as IDR; '—' when absent or invalid. */
export function formatRupiah(v: string | number | null | undefined): string {
  if (v === null || v === undefined || v === '') return '—'
  const n = typeof v === 'number' ? v : Number(v)
  if (!Number.isFinite(n)) return '—'
  return idr.format(n)
}
```

`frontend/app/utils/approvalPayload.ts`:

```ts
import type { ApprovalRequestDetail } from '~/composables/api/useApproval'
import { formatRupiah } from '~/utils/money'

export interface SummaryRow { label: string, value: string }
export interface DiffRow { label: string, before: string, after: string }
export type PayloadView
  = { layout: 'summary', rows: SummaryRow[] }
    | { layout: 'diff', rows: DiffRow[] }

export interface PayloadLookups {
  categoryName?: (id: string) => string | undefined
  officeName?: (id: string) => string | undefined
}

type Tfn = (k: string, p?: Record<string, unknown>) => string

function str(p: Record<string, unknown> | null | undefined, key: string): string | undefined {
  const v = p?.[key]
  return typeof v === 'string' && v !== '' ? v : undefined
}

/**
 * Maps a request's raw payload into the mockup's Data section shape.
 * asset_create/asset_transfer render as label:value summaries; asset_disposal
 * and valuation_exclusion render as before→after diffs (their status rows are
 * static — those transitions are implied by the request type, not the payload).
 * A masked/absent payload yields empty rows for payload-dependent fields only.
 */
export function payloadToView(detail: ApprovalRequestDetail, t: Tfn, lookups: PayloadLookups = {}): PayloadView {
  const p = (detail.payload ?? null) as Record<string, unknown> | null

  if (detail.type === 'asset_create') {
    if (!p) return { layout: 'summary', rows: [] }
    const rows: SummaryRow[] = []
    const push = (label: string, value?: string) => {
      if (value) rows.push({ label: t(label), value })
    }
    push('approval.field.assetName', str(p, 'name'))
    const catID = str(p, 'category_id')
    push('approval.field.category', catID ? (lookups.categoryName?.(catID) ?? catID) : undefined)
    push('approval.field.assetClass', str(p, 'asset_class'))
    if (str(p, 'purchase_cost')) push('approval.field.purchaseCost', formatRupiah(str(p, 'purchase_cost')))
    push('approval.field.purchaseDate', str(p, 'purchase_date'))
    push('approval.field.serialNumber', str(p, 'serial_number'))
    push('approval.field.poNumber', str(p, 'po_number'))
    push('approval.field.fundingSource', str(p, 'funding_source'))
    return { layout: 'summary', rows }
  }

  if (detail.type === 'asset_transfer') {
    if (!p) return { layout: 'summary', rows: [] }
    const rows: SummaryRow[] = []
    const office = (id?: string) => (id ? (lookups.officeName?.(id) ?? id) : undefined)
    const from = office(str(p, 'from_office_id'))
    const to = office(str(p, 'to_office_id'))
    if (from) rows.push({ label: t('approval.field.fromOffice'), value: from })
    if (to) rows.push({ label: t('approval.field.toOffice'), value: to })
    if (str(p, 'to_room_id')) rows.push({ label: t('approval.field.toRoom'), value: str(p, 'to_room_id')! })
    return { layout: 'summary', rows }
  }

  if (detail.type === 'asset_disposal') {
    const rows: DiffRow[] = [{
      label: t('approval.field.assetStatus'),
      before: t('approval.field.active'),
      after: t('approval.field.disposed')
    }]
    const add = (label: string, after?: string) => {
      if (after) rows.push({ label: t(label), before: '—', after })
    }
    add('approval.field.method', str(p, 'method'))
    add('approval.field.disposalDate', str(p, 'disposal_date'))
    if (str(p, 'proceeds')) add('approval.field.proceeds', formatRupiah(str(p, 'proceeds')))
    if (str(p, 'book_value_at_disposal')) add('approval.field.bookValue', formatRupiah(str(p, 'book_value_at_disposal')))
    add('approval.field.bastNo', str(p, 'bast_no'))
    return { layout: 'diff', rows }
  }

  // valuation_exclusion — fully static: the transition is the request itself.
  return {
    layout: 'diff',
    rows: [{
      label: t('approval.field.valuation'),
      before: t('approval.field.included'),
      after: t('approval.field.excluded')
    }]
  }
}
```

- [ ] **Step 4: Run to verify pass**

Run: `pnpm vitest run test/unit/approval-payload.spec.ts` → PASS. Then `pnpm lint` → clean.

- [ ] **Step 5: Commit**

```bash
git add frontend/app/utils/money.ts frontend/app/utils/approvalPayload.ts frontend/test/unit/approval-payload.spec.ts
git commit -m "feat(approval): payload-to-view mapper and shared rupiah formatter"
```

---

### Task 7: Frontend — rebind `approval.vue` + nav/gate + component tests

**Files:**
- Rewrite `<script setup>` + adjust template of: `frontend/app/pages/approval.vue`
- Modify: `frontend/app/utils/nav.ts` (remove `badgeCount: 8` from the approval item)
- Rewrite: `frontend/test/nuxt/approval.spec.ts`
- Check-only: `frontend/test/unit/approval-mock.spec.ts` stays passing (mock store untouched)

**Interfaces:**
- Consumes: `useApproval` (Task 5), `payloadToView` (Task 6), constants (Task 4), `useCategories`/`useReference`-style lookups: office names via `GET /offices?limit=100` using `useOffices` composable's list (reuse the same inline-lookup pattern as `frontend/app/pages/master/employees.vue`), category tree via `useCategories().tree()` (check its exported name in `frontend/app/composables/api/useCategories.ts` and use the real one).
- Produces: the final wired screen.

- [ ] **Step 1: Rewrite the page script**

Replace the entire `<script setup lang="ts">` block of `frontend/app/pages/approval.vue` with:

```ts
<script setup lang="ts">
import type { ApprovalRequestRow, ApprovalRequestDetail } from '~/composables/api/useApproval'
import type { RequestType, RequestStatus } from '~/constants/approvalMeta'
import type { BadgeColor } from '~/types'
import { useApproval } from '~/composables/api/useApproval'
import { TYPE_META, STATUS_TONE, REQUEST_TYPE_KEYS, STATUS_FILTERS } from '~/constants/approvalMeta'
import { payloadToView } from '~/utils/approvalPayload'

definePageMeta({ middleware: 'can', permission: 'request.decide' })

const TONE_SOFT: Record<BadgeColor, string> = {
  primary: 'bg-primary/15 text-primary',
  info: 'bg-info/15 text-info',
  success: 'bg-success/15 text-success',
  warning: 'bg-warning/15 text-warning',
  error: 'bg-error/15 text-error',
  neutral: 'bg-muted text-muted'
}
const TIMELINE_DOT: Record<string, string> = {
  submitted: 'bg-info',
  approved: 'bg-success',
  rejected: 'bg-error',
  cancelled: 'bg-muted',
  pending: 'bg-warning'
}

const { t, locale } = useI18n()
const api = useApproval()
const toast = useToast()

const rows = ref<ApprovalRequestRow[]>([])
const inboxRows = ref<ApprovalRequestRow[]>([])
const loading = ref(true)
const loadError = ref(false)
const filter = ref<RequestStatus | 'all'>('pending')
const typeFilter = ref<RequestType | 'all'>('all')
const selectedId = ref<string | null>(null)
const detail = ref<ApprovalRequestDetail | null>(null)
const detailLoading = ref(false)
const note = ref('')
const deciding = ref(false)

// FK name lookups for the Data section (same inline pattern as master/employees).
const categoryMap = ref(new Map<string, string>())
const officeMap = ref(new Map<string, string>())

const inboxIds = computed(() => new Set(inboxRows.value.map(r => r.id)))
const pendingCount = computed(() => inboxRows.value.length)

const filterTabs = computed(() => STATUS_FILTERS.map(k => ({ key: k, label: t(`approval.filter.${k}`) })))
const tipeItems = computed(() => [
  { value: 'all', label: t('approval.allTypes') },
  ...REQUEST_TYPE_KEYS.map(k => ({ value: k, label: t(`approval.type.${k}`) }))
])

function fmtDate(iso: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return new Intl.DateTimeFormat(locale.value === 'en' ? 'en-GB' : 'id-ID', {
    day: '2-digit', month: 'short', year: 'numeric'
  }).format(d)
}
function fmtDateTime(iso: string | null): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  const date = fmtDate(iso)
  const time = new Intl.DateTimeFormat('id-ID', { hour: '2-digit', minute: '2-digit', hour12: false }).format(d)
  return `${date} · ${time}`
}

function rowTitle(r: ApprovalRequestRow): string {
  return `${t(`approval.type.${r.type}`)} · ${r.office_name ?? '—'}`
}

const listRows = computed(() => rows.value.map((r) => {
  const meta = TYPE_META[r.type]
  return {
    id: r.id,
    icon: meta?.icon ?? 'i-lucide-file-question',
    iconSoft: TONE_SOFT[meta?.tone ?? 'neutral'],
    tipeLabel: t(`approval.type.${r.type}`),
    sensitive: meta?.sensitive ?? false,
    judul: rowTitle(r),
    pengaju: r.requested_by_name ?? '—',
    tgl: fmtDate(r.created_at),
    statusTone: STATUS_TONE[r.status] ?? 'neutral',
    statusLabel: t(`approval.status.${r.status}`),
    selected: r.id === selectedId.value
  }
}))

async function loadInbox() {
  inboxRows.value = await api.inbox()
}

async function loadTab() {
  loading.value = true
  loadError.value = false
  try {
    if (filter.value === 'pending') {
      await loadInbox()
      rows.value = typeFilter.value === 'all'
        ? inboxRows.value
        : inboxRows.value.filter(r => r.type === typeFilter.value)
    } else {
      const q = filter.value === 'all' ? {} : { status: filter.value }
      const page = await api.list({ ...q, type: typeFilter.value === 'all' ? undefined : typeFilter.value, limit: 100 })
      rows.value = page.data
    }
  } catch {
    loadError.value = true
    rows.value = []
  } finally {
    loading.value = false
  }
}

async function selectRequest(id: string) {
  selectedId.value = id
  note.value = ''
  detailLoading.value = true
  try {
    detail.value = await api.get(id)
  } catch {
    detail.value = null
  } finally {
    detailLoading.value = false
  }
}

watch([filter, typeFilter], async () => {
  selectedId.value = null
  detail.value = null
  note.value = ''
  await loadTab()
})

const view = computed(() => {
  const d = detail.value
  if (!d) return null
  const meta = TYPE_META[d.type]
  const dataView = payloadToView(d, t, {
    categoryName: id => categoryMap.value.get(id),
    officeName: id => officeMap.value.get(id)
  })

  const initials = (d.requested_by_name ?? '—').split(/\s+/).map(w => w[0]).slice(0, 2).join('').toUpperCase()

  type TimelineEntry = { action: string, actor: string, date: string, note: string, dot: string, line: boolean }
  const tl: TimelineEntry[] = [{
    action: t('approval.action.submitted'),
    actor: `${d.requested_by_name ?? '—'} · ${d.requested_by_role ?? '—'}`,
    date: fmtDateTime(d.created_at),
    note: '',
    dot: TIMELINE_DOT.submitted!,
    line: true
  }]
  for (const s of d.steps) {
    if (s.decision === 'approved' || s.decision === 'rejected') {
      tl.push({
        action: t(`approval.action.${s.decision}`),
        actor: `${s.approver_name ?? '—'} · ${t(`approval.level.${s.required_level}`)}`,
        date: fmtDateTime(s.decided_at),
        note: s.note ?? '',
        dot: TIMELINE_DOT[s.decision]!,
        line: true
      })
    }
  }
  if (d.status === 'cancelled') {
    tl.push({
      action: t('approval.action.cancelled'),
      actor: d.requested_by_name ?? '—',
      date: '—',
      note: d.decision_note ?? '',
      dot: TIMELINE_DOT.cancelled!,
      line: false
    })
  } else if (d.status === 'pending') {
    const cur = d.steps.find(s => s.step_order === d.current_step)
    tl.push({
      action: cur
        ? t('approval.action.pendingStep', { n: cur.step_order, level: t(`approval.level.${cur.required_level}`) })
        : t('approval.action.pending'),
      actor: '—',
      date: '—',
      note: '',
      dot: TIMELINE_DOT.pending!,
      line: false
    })
  }
  if (tl.length && d.status !== 'pending' && d.status !== 'cancelled') tl[tl.length - 1]!.line = false

  const decided = d.status === 'approved' || d.status === 'rejected'
  const lastStep = [...d.steps].reverse().find(s => s.decision === 'approved' || s.decision === 'rejected')
  const resultText = d.status === 'cancelled'
    ? t('approval.resultCancelled')
    : decided && lastStep
      ? (d.status === 'approved'
          ? t('approval.resultApproved', { actor: lastStep.approver_name ?? '—', date: fmtDateTime(lastStep.decided_at) })
          : t('approval.resultRejected', { actor: lastStep.approver_name ?? '—', date: fmtDateTime(lastStep.decided_at) }))
      : ''

  return {
    req: d,
    icon: meta?.icon ?? 'i-lucide-file-question',
    tone: meta?.tone ?? 'neutral',
    iconSoft: TONE_SOFT[meta?.tone ?? 'neutral'],
    tipeLabel: t(`approval.type.${d.type}`),
    sensitive: meta?.sensitive ?? false,
    statusTone: STATUS_TONE[d.status] ?? 'neutral',
    statusLabel: t(`approval.status.${d.status}`),
    judul: rowTitle(d),
    pengaju: d.requested_by_name ?? '—',
    role: d.requested_by_role ?? '—',
    kantor: d.office_name ?? '—',
    ini: initials || '—',
    tgl: fmtDate(d.created_at),
    isDiff: dataView.layout === 'diff',
    dataRows: dataView.rows,
    alasan: d.reason ?? '—',
    timeline: tl,
    pending: d.status === 'pending',
    eligible: d.status === 'pending' && inboxIds.value.has(d.id),
    decided: decided || d.status === 'cancelled',
    resultText,
    resultTone: d.status === 'approved' ? 'success' as const : d.status === 'rejected' ? 'error' as const : 'neutral' as const,
    resultIcon: d.status === 'approved' ? 'i-lucide-check' : d.status === 'rejected' ? 'i-lucide-x' : 'i-lucide-ban'
  }
})

async function decide(action: 'approve' | 'reject') {
  const d = detail.value
  if (!d || deciding.value) return
  deciding.value = true
  try {
    if (action === 'approve') await api.approve(d.id, note.value || undefined)
    else await api.reject(d.id, note.value || undefined)
    note.value = ''
    await loadTab()
    await selectRequest(d.id)
  } catch {
    // useApiClient already raised a toast; re-sync state (403 SoD / 409 stale step).
    await loadTab()
    if (selectedId.value) await selectRequest(selectedId.value)
  } finally {
    deciding.value = false
  }
}

async function loadLookups() {
  try {
    const { request } = useApiClient()
    const [cats, offs] = await Promise.all([
      request<{ data: { id: string, name: string }[] }>('/categories/tree'),
      request<{ data: { id: string, name: string }[] }>('/offices', { query: { limit: 100 } })
    ])
    categoryMap.value = new Map(cats.data.map(c => [c.id, c.name]))
    officeMap.value = new Map(offs.data.map(o => [o.id, o.name]))
  } catch {
    // Lookups are best-effort; the mapper falls back to raw ids.
  }
}

onMounted(async () => {
  await Promise.all([loadTab(), loadLookups()])
})
</script>
```

Note: check the real response shape of `GET /categories/tree` in `frontend/app/composables/api/useCategories.ts` before using it — if it returns a nested tree, flatten it (`const flat = (n: { id: string, name: string, children?: unknown[] }[]): { id: string, name: string }[] => ...`) or reuse the composable's own helper; if it returns a flat `{data}` list, the code above is correct as-is.

- [ ] **Step 2: Adjust the template**

Keep the template structure (mockup fidelity) with these exact deltas:

1. Every `detail.` reference becomes `view.` (rename the computed) — i.e. `v-if="view"`, `view.req`, etc.
2. Left-pane card body: `{{ r.pengaju }} · {{ r.tgl }}` stays (fields exist on the new `listRows`).
3. Detail "pengaju" card: `view.ini`, `view.pengaju`, `view.role`, `view.kantor`, `view.tgl` (replace `detail.req.ini/pengaju/kantor` and `detail.role`).
4. Data section: `v-for="(f, i) in view.dataRows"` in both branches (was `detail.diff` / `detail.summary`); add after the section header, wrapping the card, an empty-state when there are no rows:

```html
<div
  v-if="view.dataRows.length === 0"
  class="px-4 py-3.5 rounded-xl bg-default border border-default shadow-sm text-[13px] text-dimmed mb-[18px]"
>
  {{ t('approval.noData') }}
</div>
<div v-else class="bg-default border border-default rounded-xl shadow-sm overflow-hidden mb-[18px]">
  <!-- existing diff/summary templates, driven by view.isDiff + view.dataRows -->
</div>
```

5. Lampiran section: remove the `v-if="detail.req.files.length > 0"` chips branch entirely — always render the existing `v-else` empty text (`t('approval.noAttach')`).
6. Footer pending block: gate the actions on eligibility —

```html
<div v-if="view.pending" class="flex-none border-t border-default bg-default p-4 px-7">
  <div class="max-w-[680px]">
    <div v-if="!view.eligible" class="flex gap-2.5 items-center px-3 py-2.5 rounded-[10px] bg-muted border border-default text-muted text-[12.5px] leading-snug font-medium" data-testid="approval-not-eligible">
      <UIcon name="i-lucide-lock" class="size-4 flex-none" />
      {{ t('approval.notEligible') }}
    </div>
    <template v-else>
      <!-- existing sensitiveWarn banner + note UFormField + reject/approve UButtons,
           with @click="decide('rejected')" → @click="decide('reject')" and
           @click="decide('approved')" → @click="decide('approve')" -->
    </template>
  </div>
</div>
```

7. Result footer (`v-else` branch): support the neutral cancelled tone —

```html
:class="view.resultTone === 'success' ? 'bg-success/10 border-success/30 text-success'
  : view.resultTone === 'error' ? 'bg-error/10 border-error/30 text-error'
  : 'bg-muted border-default text-muted'"
```

8. Loading of the right pane: wrap the detail content in `v-if="detailLoading"` → `USkeleton` column (3 skeleton blocks), `v-else-if="view"` → existing content, `v-else` → existing placeholder.
9. Left pane: add a load-error state before the empty state:

```html
<div v-else-if="loadError" class="py-[50px] px-5 text-center" data-testid="approval-load-error">
  <div class="text-sm font-semibold mb-2">{{ t('approval.loadError') }}</div>
  <UButton size="sm" variant="soft" :label="t('approval.retry')" @click="loadTab" />
</div>
```

10. Add `data-testid`: filter tab buttons `data-testid="approval-tab-{f.key}"`, list card `data-testid="approval-card"`, approve button `data-testid="approval-approve"`, reject `data-testid="approval-reject"`, note input `data-testid="approval-note"`.

- [ ] **Step 3: Fix the nav item (badge + permission gate)**

In `frontend/app/utils/nav.ts`, change the `nav.approval` item: delete `badgeCount: 8` (mock-era hardcode; a real badge count needs a global inbox store — out of scope, note in PROGRESS.md) and add `permission: 'request.decide'` (the `NavItem.permission` field exists and AppSidebar gates on it — verify by checking how AppSidebar consumes `permission` before relying on it; if it does NOT filter, leave the item ungated and record that in PROGRESS.md instead):

```ts
      {
        labelKey: 'nav.approval',
        icon: 'i-lucide-check-square',
        to: '/approval',
        permission: 'request.decide'
      },
```

- [ ] **Step 4: Rewrite the component test**

Replace `frontend/test/nuxt/approval.spec.ts` entirely:

```ts
// @vitest-environment nuxt
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mountSuspended } from '@nuxt/test-utils/runtime'
import { flushPromises } from '@vue/test-utils'
import ApprovalPage from '~/pages/approval.vue'
import type { ApprovalRequestRow, ApprovalRequestDetail } from '~/composables/api/useApproval'

const row = (over: Partial<ApprovalRequestRow> = {}): ApprovalRequestRow => ({
  id: 'r1', type: 'asset_create', status: 'pending', amount: '1500000',
  current_step: 1, office_id: 'o1', office_name: 'Cabang Alpha',
  target_id: null, target_entity: null, reason: 'pengadaan',
  requested_by_id: 'u1', requested_by_name: 'Andi Saputra', requested_by_role: 'Kepala Unit',
  decided_by_id: null, decision_note: null, created_at: '2026-07-04T09:00:00Z',
  ...over
})
const detail = (over: Partial<ApprovalRequestDetail> = {}): ApprovalRequestDetail => ({
  ...row(), payload: { name: 'Laptop A', purchase_cost: '1500000' },
  steps: [{ step_order: 1, required_level: 'office', approver_id: null, approver_name: null, decision: 'pending', note: null, decided_at: null }],
  ...over
})

const inboxMock = vi.fn()
const listMock = vi.fn()
const getMock = vi.fn()
const approveMock = vi.fn()
const rejectMock = vi.fn()

vi.mock('~/composables/api/useApproval', () => ({
  useApproval: () => ({ inbox: inboxMock, list: listMock, get: getMock, approve: approveMock, reject: rejectMock })
}))
// Lookups fire a raw useApiClient request — stub it to avoid network.
mockNuxtImport('useApiClient', () => () => ({
  request: vi.fn().mockResolvedValue({ data: [] }),
  requestBlob: vi.fn(),
  refreshToken: vi.fn()
}))

beforeEach(() => {
  vi.clearAllMocks()
  inboxMock.mockResolvedValue([row()])
  listMock.mockResolvedValue({ data: [], total: 0, limit: 100, offset: 0 })
  getMock.mockResolvedValue(detail())
  approveMock.mockResolvedValue(row({ status: 'approved' }))
  rejectMock.mockResolvedValue(row({ status: 'rejected' }))
})

describe('pages/approval — wired', () => {
  it('loads the inbox on mount and renders a request card', async () => {
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    expect(inboxMock).toHaveBeenCalled()
    expect(w.text()).toContain('Andi Saputra')
    expect(w.text()).toContain('Cabang Alpha')
  })

  it('shows the empty state when the inbox is empty', async () => {
    inboxMock.mockResolvedValue([])
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    expect(w.text()).toContain('Tidak ada pengajuan')
  })

  it('shows the load-error state with retry when the inbox call fails', async () => {
    inboxMock.mockRejectedValue(new Error('boom'))
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    expect(w.find('[data-testid="approval-load-error"]').exists()).toBe(true)
  })

  it('switching to the approved tab queries the list endpoint with status', async () => {
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-tab-approved"]').trigger('click')
    await flushPromises()
    expect(listMock).toHaveBeenCalledWith(expect.objectContaining({ status: 'approved' }))
  })

  it('has a cancelled tab that queries status=cancelled', async () => {
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-tab-cancelled"]').trigger('click')
    await flushPromises()
    expect(listMock).toHaveBeenCalledWith(expect.objectContaining({ status: 'cancelled' }))
  })

  it('selecting a card fetches the detail and renders payload data + timeline', async () => {
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    expect(getMock).toHaveBeenCalledWith('r1')
    expect(w.text()).toContain('Laptop A')
    expect(w.text()).toContain('Mengajukan permintaan')
  })

  it('approve sends the note and refreshes', async () => {
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="approval-note"] input').setValue('ok!')
    await w.find('[data-testid="approval-approve"]').trigger('click')
    await flushPromises()
    expect(approveMock).toHaveBeenCalledWith('r1', 'ok!')
    expect(inboxMock.mock.calls.length).toBeGreaterThanOrEqual(2)
  })

  it('reject sends the note', async () => {
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="approval-reject"]').trigger('click')
    await flushPromises()
    expect(rejectMock).toHaveBeenCalledWith('r1', undefined)
  })

  it('a pending request NOT in the inbox shows the not-eligible lock instead of buttons', async () => {
    inboxMock.mockResolvedValue([])
    listMock.mockResolvedValue({ data: [row()], total: 1, limit: 100, offset: 0 })
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-tab-all"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    expect(w.find('[data-testid="approval-not-eligible"]').exists()).toBe(true)
    expect(w.find('[data-testid="approval-approve"]').exists()).toBe(false)
  })

  it('a cancelled request renders the neutral result banner', async () => {
    inboxMock.mockResolvedValue([])
    listMock.mockResolvedValue({ data: [row({ status: 'cancelled' })], total: 1, limit: 100, offset: 0 })
    getMock.mockResolvedValue(detail({ status: 'cancelled' }))
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-tab-cancelled"]').trigger('click')
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Dibatalkan oleh pengaju')
  })

  it('lampiran section always renders the permanent empty state', async () => {
    const w = await mountSuspended(ApprovalPage)
    await flushPromises()
    await w.find('[data-testid="approval-card"]').trigger('click')
    await flushPromises()
    expect(w.text()).toContain('Tidak ada lampiran')
  })
})
```

(`mockNuxtImport` comes from `@nuxt/test-utils/runtime` — add it to the import if the file doesn't have it: `import { mountSuspended, mockNuxtImport } from '@nuxt/test-utils/runtime'`. Adjust selectors if the note input renders differently — assert real behavior, don't weaken assertions.)

- [ ] **Step 5: Run the suites**

Run: `pnpm vitest run test/nuxt/approval.spec.ts` → PASS.
Run: `pnpm test` (full) → PASS incl. `approval-mock.spec.ts` (store untouched). Watch the exit code, not just the tail output.
Run: `pnpm lint ; pnpm typecheck ; pnpm build` → clean.

- [ ] **Step 6: Commit**

```bash
git add frontend/app/pages/approval.vue frontend/app/utils/nav.ts frontend/test/nuxt/approval.spec.ts
git commit -m "feat(approval): wire Pengajuan & Approval screen to real /requests (inbox, detail, decide)"
```

---

### Task 8: E2E — real-backend approval flow

**Files:**
- Create: `frontend/e2e/approval.spec.ts`

**Interfaces:**
- Consumes: helpers from `frontend/e2e/helpers.ts` (`login`, `EMAIL`, `PASSWORD`) and the API-setup pattern from `frontend/e2e/assets.spec.ts` (own `APIRequestContext`, `authHeader`, `apiJson`, `login_` — copy those three thin helpers into this spec, they are file-local there).
- Requires: full Docker stack + seeded admin (`pnpm test:e2e` prerequisites).

- [ ] **Step 1: Write the spec**

`frontend/e2e/approval.spec.ts` — follow the exact conventions at the top of `assets.spec.ts` (serial mode, unique-per-run names, assert-after-search):

```ts
import { test, expect, request } from '@playwright/test'
import type { APIRequestContext, APIResponse } from '@playwright/test'
import { login, EMAIL, PASSWORD } from './helpers'

// Pengajuan & Approval — real backend. Flow:
//  beforeAll (API): office-type → office → category; a second SoD checker user
//  (Superadmin role); submit TWO asset_create requests (amounts in the lowest
//  threshold band → single office-level step).
//  UI as checker: inbox shows both → open detail (Data section from payload)
//  → approve #1 with a note (timeline + result banner) → reject #2.
//  Then cancel a third request via API as maker → visible under tab Cancelled.

const API_BASE = `${process.env.E2E_API_BASE || 'http://localhost:8080/api/v1'}/`
const RUN = `${Date.now()}`

function authHeader(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` }
}
async function apiJson<T>(res: APIResponse): Promise<T> {
  if (!res.ok()) throw new Error(`API call failed: ${res.status()} ${res.url()} — ${await res.text()}`)
  return res.json() as Promise<T>
}
async function login_(api: APIRequestContext, email: string, password: string): Promise<string> {
  const res = await api.post('auth/login', { data: { email, password } })
  return (await apiJson<{ access_token: string }>(res)).access_token
}

test.describe('Approval — real backend (inbox + decide e2e)', () => {
  test.describe.configure({ mode: 'serial' })

  let api: APIRequestContext
  let adminToken: string
  let checkerEmail: string
  let checkerPassword: string
  let officeId: string
  let officeName: string
  let categoryId: string
  let approveName: string
  let rejectName: string
  let approveReqId: string
  let rejectReqId: string
  let cancelReqId: string

  async function submitCreate(name: string, cost: string): Promise<string> {
    const res = await api.post('requests', {
      headers: authHeader(adminToken),
      data: {
        type: 'asset_create',
        amount: cost,
        office_id: officeId,
        payload: {
          name, category_id: categoryId, office_id: officeId,
          asset_class: 'intangible', purchase_cost: cost
        }
      }
    })
    return (await apiJson<{ id: string }>(res)).id
  }

  test.beforeAll(async () => {
    api = await request.newContext({ baseURL: API_BASE })
    adminToken = await login_(api, EMAIL, PASSWORD)

    const ot = await apiJson<{ id: string }>(await api.post('office-types', {
      headers: authHeader(adminToken), data: { name: `E2E Appr OT ${RUN}` }
    }))
    officeName = `E2E Appr Office ${RUN}`
    const off = await apiJson<{ id: string }>(await api.post('offices', {
      headers: authHeader(adminToken),
      data: { name: officeName, code: `E2EAP${RUN}`, office_type_id: ot.id }
    }))
    officeId = off.id
    const cat = await apiJson<{ id: string }>(await api.post('categories', {
      headers: authHeader(adminToken),
      data: { name: `E2E Appr Cat ${RUN}`, code: `EAP${RUN}`, asset_class: 'intangible' }
    }))
    categoryId = cat.id

    const roles = await apiJson<{ data: { id: string, name: string }[] }>(
      await api.get('authz/roles', { headers: authHeader(adminToken) }))
    const superadmin = roles.data.find(r => r.name === 'Superadmin')
    if (!superadmin) throw new Error('Superadmin role not found')
    checkerEmail = `e2e.appr.checker.${RUN}@inventra.local`
    checkerPassword = `Checker${RUN}!`
    await apiJson(await api.post('users', {
      headers: authHeader(adminToken),
      data: { name: `E2E Appr Checker ${RUN}`, email: checkerEmail, password: checkerPassword, role_id: superadmin.id }
    }))

    approveName = `E2E Appr Laptop ${RUN}`
    rejectName = `E2E Appr Printer ${RUN}`
    approveReqId = await submitCreate(approveName, '750000')
    rejectReqId = await submitCreate(rejectName, '850000')
    cancelReqId = await submitCreate(`E2E Appr Cancelled ${RUN}`, '650000')
    await apiJson(await api.post(`requests/${cancelReqId}/cancel`, {
      headers: authHeader(adminToken), data: {}
    }))
  })

  test.afterAll(async () => {
    await api.dispose()
  })

  test('checker inbox lists the pending requests with maker + office names', async ({ page }) => {
    await login(page, checkerEmail, checkerPassword)
    await page.goto('/approval')
    const approveCard = page.locator('[data-testid="approval-card"]', { hasText: officeName }).first()
    await expect(approveCard).toBeVisible({ timeout: 10_000 })
  })

  test('detail renders the payload Data section and approve works with a note', async ({ page }) => {
    await login(page, checkerEmail, checkerPassword)
    await page.goto('/approval')
    // Two cards from this run share the office name; open the first and verify
    // by payload asset name after the detail loads — select the card whose
    // detail shows approveName (cards carry type+office as title).
    const cards = page.locator('[data-testid="approval-card"]', { hasText: officeName })
    await expect(cards.first()).toBeVisible({ timeout: 10_000 })
    const n = await cards.count()
    let found = false
    for (let i = 0; i < n; i++) {
      await cards.nth(i).click()
      const detailPane = page.locator('text=' + approveName)
      if (await detailPane.first().isVisible({ timeout: 3_000 }).catch(() => false)) {
        found = true
        break
      }
    }
    expect(found).toBe(true)

    await page.getByTestId('approval-note').locator('input').fill(`ok e2e ${RUN}`)
    await page.getByTestId('approval-approve').click()
    await expect(page.locator(`text=Disetujui oleh`).first()).toBeVisible({ timeout: 10_000 })
    // Timeline shows the checker's decision note.
    await expect(page.locator(`text=ok e2e ${RUN}`)).toBeVisible()
  })

  test('reject flow renders the red result banner', async ({ page }) => {
    await login(page, checkerEmail, checkerPassword)
    await page.goto('/approval')
    const cards = page.locator('[data-testid="approval-card"]', { hasText: officeName })
    await expect(cards.first()).toBeVisible({ timeout: 10_000 })
    const n = await cards.count()
    let found = false
    for (let i = 0; i < n; i++) {
      await cards.nth(i).click()
      if (await page.locator('text=' + rejectName).first().isVisible({ timeout: 3_000 }).catch(() => false)) {
        found = true
        break
      }
    }
    expect(found).toBe(true)
    await page.getByTestId('approval-reject').click()
    await expect(page.locator('text=Ditolak oleh').first()).toBeVisible({ timeout: 10_000 })
  })

  test('cancelled request appears under the Cancelled tab with neutral banner', async ({ page }) => {
    await login(page, checkerEmail, checkerPassword)
    await page.goto('/approval')
    await page.getByTestId('approval-tab-cancelled').click()
    const card = page.locator('[data-testid="approval-card"]', { hasText: officeName }).first()
    await expect(card).toBeVisible({ timeout: 10_000 })
    await card.click()
    await expect(page.locator('text=Dibatalkan oleh pengaju').first()).toBeVisible({ timeout: 10_000 })
  })

  test('approved request no longer sits in the pending inbox', async ({ page }) => {
    await login(page, checkerEmail, checkerPassword)
    await page.goto('/approval')
    await page.getByTestId('approval-tab-approved').click()
    await expect(page.locator('[data-testid="approval-card"]', { hasText: officeName }).first())
      .toBeVisible({ timeout: 10_000 })
  })
})
```

Check `frontend/e2e/helpers.ts` first: if `login(page, email, password)` has a different signature (some specs use `login(page)` for the admin only), adapt the calls — the checker login must use the created checker credentials.

- [ ] **Step 2: Lint + typecheck the spec**

Run: `pnpm lint ; pnpm typecheck` → clean. (Playwright specs are typechecked.)

- [ ] **Step 3: Run the e2e (stack must be up)**

Ensure the Docker stack is running (user runs `docker compose up --build` or dev watch) and the admin is seeded, then:
Run: `pnpm test:e2e -- approval.spec.ts` → all PASS. Then the FULL e2e suite: `pnpm test:e2e` → all PASS (regression check per project memory — check the process exit code).

- [ ] **Step 4: Commit**

```bash
git add frontend/e2e/approval.spec.ts
git commit -m "test(approval): real-backend e2e for inbox, decide, cancelled tab"
```

---

### Task 9: Full verification, mockup side-by-side, PROGRESS.md

**Files:**
- Modify: `docs/PROGRESS.md`

- [ ] **Step 1: Run every CI gate**

From `backend/`: `go build ./... ; go vet ./... ; go test ./...` then `go test -tags=integration ./...` (FULL module — approval signature changes may ripple; per project memory the full integration gate is mandatory after shared-signature changes).
From repo root: `npx --yes @stoplight/spectral-cli lint backend/api/openapi.yaml --ruleset .spectral.yaml`
From `frontend/`: `pnpm lint ; pnpm typecheck ; pnpm test ; pnpm build` then `pnpm test:e2e` (full suite).
Expected: everything green. Fix anything red before proceeding.

- [ ] **Step 2: Side-by-side mockup comparison (mandatory)**

Open `docs/design/Pengajuan Approval.dc.html` in a browser AND the running app at `/approval` (both light and dark mode). Verify 1:1: two-pane layout, tab bar, type filter select, card anatomy (icon chip, type label, sensitive icon, title, submitter · date, status badge), detail sections order (header badges → title → pengaju card → Data → Alasan → Lampiran → Timeline), footer action bar with note input + reject/approve buttons, sensitive warning banner, decided result banner, empty/placeholder states. The ONLY allowed deviations: (a) 5th Cancelled tab, (b) Lampiran permanent empty-state, (c) real type list (no peminjaman/maintenance), (d) card/detail titles built from type+office (payload absent on list). Fix any other gap.

- [ ] **Step 3: Update PROGRESS.md**

- In the "▶ Next session" block: mark item 23 candidate **(a)** as done — new numbered entry describing the wiring (backend enrichment + FilterView `requests` + screen + e2e), dated 2026-07-04, listing deviations (a)–(d) above (per the catat-deviasi convention).
- Under *Remaining → Frontend*: add/tick a "Pengajuan & Approval wired" entry mirroring the pattern of previous wiring entries; note `mock/approval.ts` retained for `useGlobalSearch`.
- Update the TODO #10 note (field-permission enforcement): `requests` is now enforced; remaining: `employees` + other masterdata entities.
- Note the removed hardcoded nav `badgeCount: 8` (real inbox badge deferred).
- Refresh the "Next session — start here" pointer to the remaining candidates (stock opname / depreciation / Mutasi–Disposal screens / global search).

- [ ] **Step 4: Commit**

```bash
git add docs/PROGRESS.md
git commit -m "docs(progress): mark Pengajuan & Approval screen wiring done"
```

- [ ] **Step 5: Push + PR**

```bash
git push -u origin feat/approval-screen-wiring
gh pr create --title "feat(approval): wire Pengajuan & Approval screen to real /api/v1/requests" --body-file <PR-body file>
```

PR body: summarize backend enrichment (+FilterView), frontend wiring, deviations (a)–(d), full gate results, e2e coverage. No AI attribution.
