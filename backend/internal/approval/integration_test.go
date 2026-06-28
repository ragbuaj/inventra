//go:build integration

package approval_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/asset"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/storage"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func resetAll(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// Each test gets its own throwaway container (NewPostgres), so the DB is
	// already clean post-migration. We only truncate the mutable schemas to
	// guard against any shared-pool scenarios, without touching identity/masterdata
	// so migration-seeded roles remain available.
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`TRUNCATE approval.request_approvals, approval.requests,
		 asset.asset_tag_counters, asset.assets CASCADE`)
	require.NoError(t, err)
}

// tieredOfficeTree holds the IDs seeded by seedTieredOfficeTree.
// Shape: Pusat → Wilayah → Cabang (single branch).
type tieredOfficeTree struct {
	PusatTypeID   uuid.UUID
	WilayahTypeID uuid.UUID
	CabangTypeID  uuid.UUID
	PusatID       uuid.UUID
	WilayahID     uuid.UUID
	CabangID      uuid.UUID
	CabangCode    string
}

// seedTieredOfficeTree inserts three office_types (pusat/wilayah/office tier) and
// three offices forming a single parent→child→grandchild chain.
func seedTieredOfficeTree(t *testing.T, pool *pgxpool.Pool) tieredOfficeTree {
	t.Helper()
	ctx := context.Background()
	var tr tieredOfficeTree

	// office_types with tier column
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.office_types (name, tier)
		 VALUES ('Kantor Pusat', 'pusat'::shared.approver_level) RETURNING id`).
		Scan(&tr.PusatTypeID))

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.office_types (name, tier)
		 VALUES ('Kantor Wilayah', 'wilayah'::shared.approver_level) RETURNING id`).
		Scan(&tr.WilayahTypeID))

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.office_types (name, tier)
		 VALUES ('Kantor Cabang', 'office'::shared.approver_level) RETURNING id`).
		Scan(&tr.CabangTypeID))

	// offices
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, 'Pusat', 'PST') RETURNING id`,
		tr.PusatTypeID).Scan(&tr.PusatID))

	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES ($1, $2, 'Wilayah I', 'WIL') RETURNING id`,
		tr.PusatID, tr.WilayahTypeID).Scan(&tr.WilayahID))

	tr.CabangCode = "CBG"
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES ($1, $2, 'Cabang Alpha', 'CBG') RETURNING id`,
		tr.WilayahID, tr.CabangTypeID).Scan(&tr.CabangID))

	return tr
}

// seedUser inserts an identity.users row and returns its UUID.
func seedUser(t *testing.T, pool *pgxpool.Pool, roleID uuid.UUID, email string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.users (name, email, role_id, status)
		 VALUES ($1, $2, $3, 'active') RETURNING id`,
		email, email, roleID).Scan(&id))
	return id
}

// lookupRole queries identity.roles by name and returns its id.
func lookupRole(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT id FROM identity.roles WHERE name = $1 AND deleted_at IS NULL LIMIT 1`,
		name).Scan(&id))
	return id
}

// buildCaller returns an approval.Caller with the given parameters.
func buildCaller(userID, roleID uuid.UUID, allScope bool, officeIDs []uuid.UUID) approval.Caller {
	return approval.Caller{
		UserID:    userID,
		RoleID:    roleID,
		AllScope:  allScope,
		OfficeIDs: officeIDs,
	}
}

// seedCategory inserts a masterdata.categories row and returns its id.
func seedCategory(t *testing.T, pool *pgxpool.Pool, code string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.categories (name, code, asset_class)
		 VALUES ($1, $2, 'intangible') RETURNING id`,
		code, code).Scan(&id))
	return id
}

// seedAsset inserts an asset.assets row directly (bypassing the approval flow) and
// returns its id. Uses asset_class=intangible to avoid the room FK constraint.
func seedAsset(t *testing.T, pool *pgxpool.Pool, tag, name string, categoryID, officeID uuid.UUID, status string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO asset.assets
		   (asset_tag, name, category_id, office_id, asset_class, capitalized, specifications, status)
		 VALUES ($1, $2, $3, $4, 'intangible', true, '{}', $5::shared.asset_status)
		 RETURNING id`,
		tag, name, categoryID, officeID, status).Scan(&id))
	return id
}

// ─── tests ───────────────────────────────────────────────────────────────────

// TestApproval_AssetCreate_ThreeStep submits an asset_create request for 150M
// (which triggers the 3-step chain per migration 000016) and drives it through all
// three approvers: office → wilayah → pusat. At the end the request status must be
// approved and an asset row must exist.
func TestApproval_AssetCreate_ThreeStep(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "ELK")

	officeRoleID := lookupRole(t, pool, "Kepala Unit")
	wilayahRoleID := lookupRole(t, pool, "Kepala Kanwil")
	pusatRoleID := lookupRole(t, pool, "Superadmin")

	maker := seedUser(t, pool, officeRoleID, "maker3step@test.local")
	approver1 := seedUser(t, pool, officeRoleID, "approver1@test.local")
	approver2 := seedUser(t, pool, wilayahRoleID, "approver2@test.local")
	approver3 := seedUser(t, pool, pusatRoleID, "approver3@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetCreate, assetSvc.CreateExecutor())

	officeID := tr.CabangID
	catIDStr := catID.String()
	officeIDStr := officeID.String()
	payload, err := json.Marshal(asset.AssetCreatePayload{
		Name:       "Laptop 150M",
		CategoryID: catIDStr,
		OfficeID:   officeIDStr,
		AssetClass: "intangible",
	})
	require.NoError(t, err)

	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   "150000000",
		OfficeID: officeID,
		Payload:  payload,
		Maker:    maker,
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusPending, req.Status)
	assert.Equal(t, int32(1), req.CurrentStep)

	// Step 1: office approver (cabang scope)
	caller1 := buildCaller(approver1, officeRoleID, false, []uuid.UUID{tr.CabangID})
	req, err = svc.Decide(ctx, req.ID, caller1, true, nil)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusPending, req.Status)
	assert.Equal(t, int32(2), req.CurrentStep)

	// Step 2: wilayah approver (wilayah scope covers wilayah+cabang)
	caller2 := buildCaller(approver2, wilayahRoleID, false, []uuid.UUID{tr.WilayahID, tr.CabangID})
	req, err = svc.Decide(ctx, req.ID, caller2, true, nil)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusPending, req.Status)
	assert.Equal(t, int32(3), req.CurrentStep)

	// Step 3: pusat approver (global scope)
	caller3 := buildCaller(approver3, pusatRoleID, true, nil)
	req, err = svc.Decide(ctx, req.ID, caller3, true, nil)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusApproved, req.Status)

	// Asset must have been created
	assets, total, err := assetSvc.List(ctx, asset.ListInput{AllScope: true, OfficeIDs: nil, Limit: 10, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "Laptop 150M", assets[0].Name)

	// Assert the generated asset tag matches the expected format built from the seeded codes.
	// Office "CBG" (Cabang Alpha), category "ELK", current year, first sequence.
	expectedTag := fmt.Sprintf("%s-%s-%d-%05d", tr.CabangCode, "ELK", time.Now().Year(), 1)
	assert.Equal(t, expectedTag, assets[0].AssetTag)
}

// TestApproval_SoD_MakerCannotApprove verifies that the maker of a request cannot
// also act as an approver (segregation of duty).
func TestApproval_SoD_MakerCannotApprove(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "SRV")

	roleID := lookupRole(t, pool, "Kepala Unit")
	maker := seedUser(t, pool, roleID, "sodmaker@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetCreate, assetSvc.CreateExecutor())

	officeIDStr := tr.CabangID.String()
	catIDStr := catID.String()
	payload, _ := json.Marshal(asset.AssetCreatePayload{
		Name:       "PC Office",
		CategoryID: catIDStr,
		OfficeID:   officeIDStr,
		AssetClass: "intangible",
	})

	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   "1000000", // 1M — 1-step office chain
		OfficeID: tr.CabangID,
		Payload:  payload,
		Maker:    maker,
	})
	require.NoError(t, err)

	// Maker tries to approve their own request
	caller := buildCaller(maker, roleID, false, []uuid.UUID{tr.CabangID})
	_, err = svc.Decide(ctx, req.ID, caller, true, nil)
	require.ErrorIs(t, err, approval.ErrSelfApproval)
}

// TestApproval_SoD_PriorApproverCannotApproveNextStep verifies the SoD rule that
// once a user has approved any earlier step they cannot approve a later step in the
// same request (prior-approver check in eligibleToDecide).
func TestApproval_SoD_PriorApproverCannotApproveNextStep(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "SOD")

	officeRoleID := lookupRole(t, pool, "Kepala Unit")
	wilayahRoleID := lookupRole(t, pool, "Kepala Kanwil")

	maker := seedUser(t, pool, officeRoleID, "maker.sodstep@test.local")
	// approver1 holds the office role and is eligible at step 1 (office tier)
	approver1 := seedUser(t, pool, officeRoleID, "approver1.sodstep@test.local")
	// approver2 would be the intended wilayah approver for step 2, but we won't use them
	_ = seedUser(t, pool, wilayahRoleID, "approver2.sodstep@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetCreate, assetSvc.CreateExecutor())

	catIDStr := catID.String()
	officeIDStr := tr.CabangID.String()
	payload, _ := json.Marshal(asset.AssetCreatePayload{
		Name:       "Laptop SoD Test",
		CategoryID: catIDStr,
		OfficeID:   officeIDStr,
		AssetClass: "intangible",
	})

	// 50M falls in the 2-step band (office → wilayah) per migration 000016
	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   "50000000",
		OfficeID: tr.CabangID,
		Payload:  payload,
		Maker:    maker,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), req.CurrentStep)

	// approver1 is eligible at step 1 (office scope covers Cabang) — this must succeed.
	caller1 := buildCaller(approver1, officeRoleID, false, []uuid.UUID{tr.CabangID})
	req, err = svc.Decide(ctx, req.ID, caller1, true, nil)
	require.NoError(t, err, "approver1 should succeed at step 1")
	assert.Equal(t, sqlc.SharedRequestStatusPending, req.Status)
	assert.Equal(t, int32(2), req.CurrentStep)

	// Now approver1 attempts step 2 — this must be blocked by the prior-approver SoD rule.
	// Give caller1 wilayah-scope so the only block is the SoD rule, not a scope/tier failure.
	caller1Broad := buildCaller(approver1, officeRoleID, false, []uuid.UUID{tr.WilayahID, tr.CabangID})
	_, err = svc.Decide(ctx, req.ID, caller1Broad, true, nil)
	require.ErrorIs(t, err, approval.ErrSelfApproval, "prior approver must not be allowed to approve the next step")
}

// TestApproval_RejectMidChain_NoAssetCreated verifies that rejecting in the middle
// of a multi-step chain finalises the request as rejected and no asset is created.
func TestApproval_RejectMidChain_NoAssetCreated(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "NET")

	officeRoleID := lookupRole(t, pool, "Kepala Unit")
	wilayahRoleID := lookupRole(t, pool, "Kepala Kanwil")

	maker := seedUser(t, pool, officeRoleID, "maker.reject@test.local")
	approver1 := seedUser(t, pool, officeRoleID, "approver.reject1@test.local")
	approver2 := seedUser(t, pool, wilayahRoleID, "approver.reject2@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetCreate, assetSvc.CreateExecutor())

	catIDStr := catID.String()
	officeIDStr := tr.CabangID.String()
	payload, _ := json.Marshal(asset.AssetCreatePayload{
		Name:       "Switch 50M",
		CategoryID: catIDStr,
		OfficeID:   officeIDStr,
		AssetClass: "intangible",
	})

	// 50M → 2-step chain (office + wilayah)
	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   "50000000",
		OfficeID: tr.CabangID,
		Payload:  payload,
		Maker:    maker,
	})
	require.NoError(t, err)

	// Step 1: approve
	caller1 := buildCaller(approver1, officeRoleID, false, []uuid.UUID{tr.CabangID})
	req, err = svc.Decide(ctx, req.ID, caller1, true, nil)
	require.NoError(t, err)
	assert.Equal(t, int32(2), req.CurrentStep)

	// Step 2: reject
	caller2 := buildCaller(approver2, wilayahRoleID, false, []uuid.UUID{tr.WilayahID, tr.CabangID})
	note := "tidak layak"
	req, err = svc.Decide(ctx, req.ID, caller2, false, &note)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusRejected, req.Status)

	// No asset must exist
	_, total, err := assetSvc.List(ctx, asset.ListInput{AllScope: true, OfficeIDs: nil, Limit: 10, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
}

// TestApproval_AssetDisposal_ApproveChain verifies that approving a disposal request
// transitions the asset's status to 'disposed'.
func TestApproval_AssetDisposal_ApproveChain(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "DSP")

	officeRoleID := lookupRole(t, pool, "Kepala Unit")
	assetID := seedAsset(t, pool, "CBG-DSP-2026-00001", "Meja Tua", catID, tr.CabangID, "available")

	maker := seedUser(t, pool, officeRoleID, "maker.disposal@test.local")
	approver := seedUser(t, pool, officeRoleID, "approver.disposal@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetDisposal, assetSvc.DisposalExecutor())

	targetEntity := "assets"
	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:         sqlc.SharedRequestTypeAssetDisposal,
		Amount:       "1000000", // 1M < 5M → 1-step office chain
		OfficeID:     tr.CabangID,
		TargetEntity: &targetEntity,
		TargetID:     &assetID,
		Maker:        maker,
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusPending, req.Status)

	caller := buildCaller(approver, officeRoleID, false, []uuid.UUID{tr.CabangID})
	req, err = svc.Decide(ctx, req.ID, caller, true, nil)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusApproved, req.Status)

	// Asset status must now be disposed
	updated, err := assetSvc.Get(ctx, assetID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedAssetStatusDisposed, updated.Status)
}

// TestApproval_AssetDisposal_CrossOfficeRejected verifies that the disposal executor
// rejects when the asset's office does not match the request's office.
func TestApproval_AssetDisposal_CrossOfficeRejected(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "CRS")

	// Create a second office branch so we can plant the asset in one, but submit
	// the request against the other.
	var otherOfficeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES ($1, $2, 'Cabang Beta', 'CBB') RETURNING id`,
		tr.WilayahID, tr.CabangTypeID).Scan(&otherOfficeID))

	assetID := seedAsset(t, pool, "CBG-CRS-2026-00001", "Kursi Tua", catID, tr.CabangID, "available")

	officeRoleID := lookupRole(t, pool, "Kepala Unit")
	maker := seedUser(t, pool, officeRoleID, "maker.cross@test.local")
	approver := seedUser(t, pool, officeRoleID, "approver.cross@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetDisposal, assetSvc.DisposalExecutor())

	targetEntity := "assets"
	// Submit the disposal referencing otherOfficeID but asset lives in CabangID
	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:         sqlc.SharedRequestTypeAssetDisposal,
		Amount:       "1000000",
		OfficeID:     otherOfficeID, // mismatch!
		TargetEntity: &targetEntity,
		TargetID:     &assetID,
		Maker:        maker,
	})
	require.NoError(t, err)

	caller := buildCaller(approver, officeRoleID, false, []uuid.UUID{otherOfficeID, tr.CabangID, tr.WilayahID})
	_, err = svc.Decide(ctx, req.ID, caller, true, nil)
	require.ErrorIs(t, err, asset.ErrInvalidRef, "cross-office disposal should fail with ErrInvalidRef at executor")

	// Asset must remain unchanged — still available, not disposed.
	reloaded, rerr := assetSvc.Get(ctx, assetID)
	require.NoError(t, rerr)
	assert.Equal(t, sqlc.SharedAssetStatusAvailable, reloaded.Status, "asset status must be unchanged after cross-office disposal rejection")
}

// TestApproval_ValuationExclusion_SetsFlag verifies that approving a
// valuation_exclusion request sets excluded_from_valuation=true on the asset.
func TestApproval_ValuationExclusion_SetsFlag(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "VEX")
	assetID := seedAsset(t, pool, "CBG-VEX-2026-00001", "Server Lama", catID, tr.CabangID, "available")

	wilayahRoleID := lookupRole(t, pool, "Kepala Kanwil")
	officeRoleID := lookupRole(t, pool, "Kepala Unit")
	maker := seedUser(t, pool, officeRoleID, "maker.vex@test.local")
	approver := seedUser(t, pool, wilayahRoleID, "approver.vex@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	svc.RegisterExecutor(sqlc.SharedRequestTypeValuationExclusion, assetSvc.ExclusionExecutor())

	targetEntity := "assets"
	reason := "nilai tidak material"
	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:         sqlc.SharedRequestTypeValuationExclusion,
		Amount:       "0",
		OfficeID:     tr.CabangID,
		TargetEntity: &targetEntity,
		TargetID:     &assetID,
		Reason:       &reason,
		Maker:        maker,
	})
	require.NoError(t, err)

	// valuation_exclusion has 1-step wilayah chain per migration 000016
	caller := buildCaller(approver, wilayahRoleID, false, []uuid.UUID{tr.WilayahID, tr.CabangID})
	req, err = svc.Decide(ctx, req.ID, caller, true, nil)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusApproved, req.Status)

	updated, err := assetSvc.Get(ctx, assetID)
	require.NoError(t, err)
	assert.True(t, updated.ExcludedFromValuation)
}

// TestApproval_ValuationExclusion_CrossOfficeRejected verifies that the exclusion
// executor rejects when the asset's office does not match the request office.
func TestApproval_ValuationExclusion_CrossOfficeRejected(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "VCR")

	var otherOfficeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES ($1, $2, 'Cabang Gamma', 'CBG2') RETURNING id`,
		tr.WilayahID, tr.CabangTypeID).Scan(&otherOfficeID))

	assetID := seedAsset(t, pool, "CBG-VCR-2026-00001", "Router Lama", catID, tr.CabangID, "available")

	wilayahRoleID := lookupRole(t, pool, "Kepala Kanwil")
	officeRoleID := lookupRole(t, pool, "Kepala Unit")
	maker := seedUser(t, pool, officeRoleID, "maker.vcr@test.local")
	approver := seedUser(t, pool, wilayahRoleID, "approver.vcr@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	svc.RegisterExecutor(sqlc.SharedRequestTypeValuationExclusion, assetSvc.ExclusionExecutor())

	targetEntity := "assets"
	reason := "mismatch test"
	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:         sqlc.SharedRequestTypeValuationExclusion,
		Amount:       "0",
		OfficeID:     otherOfficeID, // mismatch!
		TargetEntity: &targetEntity,
		TargetID:     &assetID,
		Reason:       &reason,
		Maker:        maker,
	})
	require.NoError(t, err)

	caller := buildCaller(approver, wilayahRoleID, false, []uuid.UUID{tr.WilayahID, tr.CabangID, otherOfficeID})
	_, err = svc.Decide(ctx, req.ID, caller, true, nil)
	require.ErrorIs(t, err, asset.ErrInvalidRef, "cross-office valuation exclusion should fail with ErrInvalidRef at executor")

	// Asset must remain unchanged — excluded_from_valuation still false.
	reloaded, rerr := assetSvc.Get(ctx, assetID)
	require.NoError(t, rerr)
	assert.False(t, reloaded.ExcludedFromValuation, "asset excluded_from_valuation must be unchanged after cross-office exclusion rejection")
}

// TestApproval_Cancel_MakerOnly verifies that only the original maker can cancel
// a pending request.
func TestApproval_Cancel_MakerOnly(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "CAN")

	roleID := lookupRole(t, pool, "Kepala Unit")
	maker := seedUser(t, pool, roleID, "maker.cancel@test.local")
	nonMaker := seedUser(t, pool, roleID, "nonmaker.cancel@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetCreate, assetSvc.CreateExecutor())

	catIDStr := catID.String()
	officeIDStr := tr.CabangID.String()
	payload, _ := json.Marshal(asset.AssetCreatePayload{
		Name:       "Asset Cancel Test",
		CategoryID: catIDStr,
		OfficeID:   officeIDStr,
		AssetClass: "intangible",
	})

	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   "1000000",
		OfficeID: tr.CabangID,
		Payload:  payload,
		Maker:    maker,
	})
	require.NoError(t, err)

	// Non-maker cannot cancel → should get ErrNotFound (no matching row)
	_, err = svc.Cancel(ctx, req.ID, nonMaker)
	require.Error(t, err)
	require.ErrorIs(t, err, approval.ErrNotFound)

	// Maker can cancel
	cancelled, err := svc.Cancel(ctx, req.ID, maker)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusCancelled, cancelled.Status)
}

// TestApproval_ListRequests_ScopeFiltered verifies that List returns only requests
// belonging to the caller's office IDs when AllScope=false.
func TestApproval_ListRequests_ScopeFiltered(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "LST")

	// Second cabang in a different wilayah branch
	var otherCabangID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES ($1, $2, 'Cabang Delta', 'CBD') RETURNING id`,
		tr.WilayahID, tr.CabangTypeID).Scan(&otherCabangID))

	roleID := lookupRole(t, pool, "Kepala Unit")
	maker1 := seedUser(t, pool, roleID, "maker.list1@test.local")
	maker2 := seedUser(t, pool, roleID, "maker.list2@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetCreate, assetSvc.CreateExecutor())

	catIDStr := catID.String()

	submit := func(officeID uuid.UUID, maker uuid.UUID, name string) {
		officeIDStr := officeID.String()
		payload, _ := json.Marshal(asset.AssetCreatePayload{
			Name:       name,
			CategoryID: catIDStr,
			OfficeID:   officeIDStr,
			AssetClass: "intangible",
		})
		_, err := svc.Submit(ctx, approval.SubmitInput{
			Type:     sqlc.SharedRequestTypeAssetCreate,
			Amount:   "1000000",
			OfficeID: officeID,
			Payload:  payload,
			Maker:    maker,
		})
		require.NoError(t, err)
	}

	submit(tr.CabangID, maker1, "Asset Cabang Alpha")
	submit(otherCabangID, maker2, "Asset Cabang Delta")

	// Caller scoped to CabangID only
	rows, total, err := svc.List(ctx, false, []uuid.UUID{tr.CabangID}, "", "", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, rows, 1)
	assert.Equal(t, &tr.CabangID, rows[0].OfficeID)

	// Global scope sees all
	rows, total, err = svc.List(ctx, true, nil, "", "", 10, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, rows, 2)
}

// TestApproval_ThresholdEdit_TakesEffect verifies that editing the threshold
// configuration immediately changes how many steps a new submission gets.
func TestApproval_ThresholdEdit_TakesEffect(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "THR")

	officeRoleID := lookupRole(t, pool, "Kepala Unit")
	wilayahRoleID := lookupRole(t, pool, "Kepala Kanwil")
	maker := seedUser(t, pool, officeRoleID, "maker.thr@test.local")
	approver1 := seedUser(t, pool, officeRoleID, "approver.thr1@test.local")
	approver2 := seedUser(t, pool, wilayahRoleID, "approver.thr2@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetCreate, assetSvc.CreateExecutor())

	// Wipe existing thresholds and install a 1-step config
	_, err := pool.Exec(ctx, `TRUNCATE approval.approval_thresholds`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO approval.approval_thresholds
		   (request_type, amount_from, amount_to, required_level, step_order)
		 VALUES ('asset_create', 0, NULL, 'office', 1)`)
	require.NoError(t, err)

	catIDStr := catID.String()
	officeIDStr := tr.CabangID.String()
	makePayload := func(name string) []byte {
		p, _ := json.Marshal(asset.AssetCreatePayload{
			Name:       name,
			CategoryID: catIDStr,
			OfficeID:   officeIDStr,
			AssetClass: "intangible",
		})
		return p
	}

	// Submit with 1-step config; after step-1 approval the request should be approved
	req1, err := svc.Submit(ctx, approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   "1000000",
		OfficeID: tr.CabangID,
		Payload:  makePayload("Asset 1Step"),
		Maker:    maker,
	})
	require.NoError(t, err)

	caller1 := buildCaller(approver1, officeRoleID, false, []uuid.UUID{tr.CabangID})
	req1, err = svc.Decide(ctx, req1.ID, caller1, true, nil)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusApproved, req1.Status, "1-step: should be approved after step 1")

	// Now add a second step to the threshold band
	_, err = pool.Exec(ctx,
		`INSERT INTO approval.approval_thresholds
		   (request_type, amount_from, amount_to, required_level, step_order)
		 VALUES ('asset_create', 0, NULL, 'wilayah', 2)`)
	require.NoError(t, err)

	// Submit again; now requires 2 steps
	req2, err := svc.Submit(ctx, approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   "1000000",
		OfficeID: tr.CabangID,
		Payload:  makePayload("Asset 2Step"),
		Maker:    maker,
	})
	require.NoError(t, err)

	// Step 1 approve → still pending (step 2 needed)
	req2, err = svc.Decide(ctx, req2.ID, caller1, true, nil)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusPending, req2.Status, "2-step: should still be pending after step 1")
	assert.Equal(t, int32(2), req2.CurrentStep)

	// Step 2 approve by wilayah approver
	caller2 := buildCaller(approver2, wilayahRoleID, false, []uuid.UUID{tr.WilayahID, tr.CabangID})
	req2, err = svc.Decide(ctx, req2.ID, caller2, true, nil)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusApproved, req2.Status)
}

// TestApproval_ExecutorAtomicity_RollbackOnError verifies that when the executor
// fails (e.g., invalid status transition), the transaction is rolled back and the
// request remains pending.
func TestApproval_ExecutorAtomicity_RollbackOnError(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "ATM")

	// Insert an already-disposed asset; transitioning disposed→disposed is invalid
	assetID := seedAsset(t, pool, "CBG-ATM-2026-00001", "ATM Rusak", catID, tr.CabangID, "disposed")

	officeRoleID := lookupRole(t, pool, "Kepala Unit")
	maker := seedUser(t, pool, officeRoleID, "maker.atom@test.local")
	approver := seedUser(t, pool, officeRoleID, "approver.atom@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetDisposal, assetSvc.DisposalExecutor())

	// Use 1-step disposal threshold: amount < 5M
	_, err := pool.Exec(ctx, `TRUNCATE approval.approval_thresholds`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO approval.approval_thresholds
		   (request_type, amount_from, amount_to, required_level, step_order)
		 VALUES ('asset_disposal', 0, NULL, 'office', 1)`)
	require.NoError(t, err)

	targetEntity := "assets"
	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:         sqlc.SharedRequestTypeAssetDisposal,
		Amount:       "1000000",
		OfficeID:     tr.CabangID,
		TargetEntity: &targetEntity,
		TargetID:     &assetID,
		Maker:        maker,
	})
	require.NoError(t, err)

	// The approver tries to approve, executor will fail because asset is already disposed
	caller := buildCaller(approver, officeRoleID, false, []uuid.UUID{tr.CabangID})
	_, err = svc.Decide(ctx, req.ID, caller, true, nil)
	require.Error(t, err, "executor should fail on invalid transition disposed→disposed")
	// Verify the error is the expected invalid state error
	assert.True(t, errors.Is(err, asset.ErrInvalidState), "expected ErrInvalidState, got: %v", err)

	// The request must still be pending (transaction was rolled back)
	reloaded, err := q.GetRequest(ctx, req.ID)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusPending, reloaded.Status, "request should still be pending after executor rollback")

	// The asset must also be unchanged — still disposed (the failed executor did not alter it).
	reloadedAsset, aerr := assetSvc.Get(ctx, assetID)
	require.NoError(t, aerr)
	assert.Equal(t, sqlc.SharedAssetStatusDisposed, reloadedAsset.Status, "asset status must remain disposed after rollback")
}

// TestApproval_GetRequest_ScopeEnforced verifies that GET /requests/:id is scope-gated:
// a caller whose data scope does not cover the request's office receives 403, while a
// caller with global scope receives 200.
//
// The scope gate lives in the HTTP handler (handler.go `get`), so this test drives it
// via net/http/httptest with a minimal gin router that injects context keys in place of
// the real JWT middleware.
func TestApproval_GetRequest_ScopeEnforced(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)

	// Seed a second, isolated office that has NO parent/child relationship with Cabang.
	var otherOfficeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, 'Kantor Lain', 'OTH') RETURNING id`,
		tr.CabangTypeID).Scan(&otherOfficeID))

	// Seed two roles: one with "office" scope (cabang-only), one with "global" scope.
	cabangRoleID := testsupport.SeedRole(t, pool, "cabang-viewer-"+uuid.New().String()[:8])
	testsupport.SeedScopePolicy(t, pool, cabangRoleID, "*", sqlc.SharedScopeLevelOffice)

	globalRoleID := testsupport.SeedRole(t, pool, "global-viewer-"+uuid.New().String()[:8])
	testsupport.SeedScopePolicy(t, pool, globalRoleID, "*", sqlc.SharedScopeLevelGlobal)

	// Seed users. cabangUser is placed in Cabang; globalUser has no office placement (superadmin-style).
	cabangUserID := seedUser(t, pool, cabangRoleID, "cabang.viewer@test.local")
	_, err := pool.Exec(ctx,
		`UPDATE identity.users SET office_id = $1 WHERE id = $2`,
		tr.CabangID, cabangUserID)
	require.NoError(t, err)

	globalUserID := seedUser(t, pool, globalRoleID, "global.viewer@test.local")

	// outsideUser is placed in otherOffice (a completely different office subtree).
	outsideRoleID := testsupport.SeedRole(t, pool, "outside-viewer-"+uuid.New().String()[:8])
	testsupport.SeedScopePolicy(t, pool, outsideRoleID, "*", sqlc.SharedScopeLevelOffice)
	outsideUserID := seedUser(t, pool, outsideRoleID, "outside.viewer@test.local")
	_, err = pool.Exec(ctx,
		`UPDATE identity.users SET office_id = $1 WHERE id = $2`,
		otherOfficeID, outsideUserID)
	require.NoError(t, err)

	// Submit a request for Cabang (the target office).
	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)

	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   "1000000",
		OfficeID: tr.CabangID,
		Payload:  []byte(`{"name":"Test","category_id":"` + uuid.New().String() + `","office_id":"` + tr.CabangID.String() + `","asset_class":"intangible"}`),
		Maker:    cabangUserID,
	})
	require.NoError(t, err)

	// Build the gin handler under test.
	gin.SetMode(gin.TestMode)
	auditSvc := audit.NewService(q)
	fieldSvc := authz.NewFieldService(q, rdb)
	scoped := common.ScopedDeps{Q: q, Scope: scopeSvc}
	h := approval.NewHandler(svc, fieldSvc, scoped, auditSvc)
	permSvc := authz.NewPermissionService(q, rdb)

	// callGet builds a fresh gin engine with a stub auth MW that injects the given
	// user/role IDs (bypassing real JWT) and drives GET /api/v1/requests/:id.
	callGet := func(userID, roleID uuid.UUID) int {
		stubAuth := func(c *gin.Context) {
			c.Set(middleware.CtxUserID, userID.String())
			c.Set(middleware.CtxRoleID, roleID.String())
			c.Next()
		}
		r := gin.New()
		v1 := r.Group("/api/v1")
		approval.RegisterRoutes(v1, h, stubAuth, permSvc)
		w := httptest.NewRecorder()
		httpReq, _ := http.NewRequest(http.MethodGet, "/api/v1/requests/"+req.ID.String(), nil)
		r.ServeHTTP(w, httpReq)
		return w.Code
	}

	// outsideUser's office scope is "otherOffice" only → must be 403.
	assert.Equal(t, http.StatusForbidden, callGet(outsideUserID, outsideRoleID),
		"caller scoped to a different office must receive 403")

	// globalUser has global scope → must see the request.
	assert.Equal(t, http.StatusOK, callGet(globalUserID, globalRoleID),
		"caller with global scope must receive 200")

	// cabangUser is placed in Cabang (same office as request) → 200.
	assert.Equal(t, http.StatusOK, callGet(cabangUserID, cabangRoleID),
		"caller scoped to the request's own office must receive 200")
}
