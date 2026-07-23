//go:build integration

package approval_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	"github.com/ragbuaj/inventra/internal/depreciation"
	"github.com/ragbuaj/inventra/internal/disposal"
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
		 asset.assets CASCADE`)
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

// seedBrand inserts a masterdata.brands row and returns its id.
func seedBrand(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.brands (name) VALUES ($1) RETURNING id`, name).Scan(&id))
	return id
}

// seedModel inserts a masterdata.models row under the given brand and returns its id.
func seedModel(t *testing.T, pool *pgxpool.Pool, brandID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.models (brand_id, name) VALUES ($1, $2) RETURNING id`,
		brandID, name).Scan(&id))
	return id
}

// seedUnit inserts a masterdata.units row and returns its id.
func seedUnit(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.units (name) VALUES ($1) RETURNING id`, name).Scan(&id))
	return id
}

// seedVendor inserts a masterdata.vendors row and returns its id.
func seedVendor(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.vendors (name) VALUES ($1) RETURNING id`, name).Scan(&id))
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

// TestApproval_AssetCreate_FullFieldSet submits an asset_create request whose
// payload carries every optional field the create form supports
// (brand/model/unit/vendor ids, PO number, funding source, warranty expiry,
// notes) and verifies the created asset row persists all of them. It also
// covers the sad path: a malformed brand_id in the payload fails the
// final-step approve with asset.ErrInvalidRef and leaves no asset behind.
func TestApproval_AssetCreate_FullFieldSet(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "ELK")

	brandID := seedBrand(t, pool, "Acme")
	modelID := seedModel(t, pool, brandID, "X100")
	unitID := seedUnit(t, pool, "Unit")
	vendorID := seedVendor(t, pool, "Acme Supplier")

	officeRoleID := lookupRole(t, pool, "Kepala Unit")
	maker := seedUser(t, pool, officeRoleID, "makerfull@test.local")
	approver1 := seedUser(t, pool, officeRoleID, "approverfull@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetCreate, assetSvc.CreateExecutor())

	officeID := tr.CabangID
	catIDStr := catID.String()
	officeIDStr := officeID.String()
	brandIDStr := brandID.String()
	modelIDStr := modelID.String()
	unitIDStr := unitID.String()
	vendorIDStr := vendorID.String()
	poNumber := "PO-2026-001"
	fundingSource := "capex"
	warrantyExpiry := "2028-01-15"
	notes := "Full field-set integration test"
	serialNumber := "SN-FULL-001"
	purchaseCost := "5000000"
	purchaseDate := "2026-01-10"

	payload, err := json.Marshal(asset.AssetCreatePayload{
		Name:           "Full Field Laptop",
		CategoryID:     catIDStr,
		OfficeID:       officeIDStr,
		AssetClass:     "intangible",
		PurchaseCost:   &purchaseCost,
		PurchaseDate:   &purchaseDate,
		SerialNumber:   &serialNumber,
		BrandID:        &brandIDStr,
		ModelID:        &modelIDStr,
		UnitID:         &unitIDStr,
		VendorID:       &vendorIDStr,
		PONumber:       &poNumber,
		FundingSource:  &fundingSource,
		WarrantyExpiry: &warrantyExpiry,
		Notes:          &notes,
	})
	require.NoError(t, err)

	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   purchaseCost, // 5,000,000 falls in the 0-10M single-step (office) band
		OfficeID: officeID,
		Payload:  payload,
		Maker:    maker,
	})
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusPending, req.Status)
	assert.Equal(t, int32(1), req.CurrentStep)

	caller1 := buildCaller(approver1, officeRoleID, false, []uuid.UUID{tr.CabangID})
	req, err = svc.Decide(ctx, req.ID, caller1, true, nil)
	require.NoError(t, err)
	assert.Equal(t, sqlc.SharedRequestStatusApproved, req.Status)

	assets, total, err := assetSvc.List(ctx, asset.ListInput{AllScope: true, OfficeIDs: nil, Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	got := assets[0]
	assert.Equal(t, "Full Field Laptop", got.Name)
	if assert.NotNil(t, got.BrandID) {
		assert.Equal(t, brandID, *got.BrandID)
	}
	if assert.NotNil(t, got.ModelID) {
		assert.Equal(t, modelID, *got.ModelID)
	}
	if assert.NotNil(t, got.UnitID) {
		assert.Equal(t, unitID, *got.UnitID)
	}
	if assert.NotNil(t, got.VendorID) {
		assert.Equal(t, vendorID, *got.VendorID)
	}
	if assert.NotNil(t, got.PoNumber) {
		assert.Equal(t, poNumber, *got.PoNumber)
	}
	if assert.NotNil(t, got.FundingSource) {
		assert.Equal(t, fundingSource, *got.FundingSource)
	}
	if assert.True(t, got.WarrantyExpiry.Valid) {
		assert.Equal(t, warrantyExpiry, got.WarrantyExpiry.Time.Format("2006-01-02"))
	}
	if assert.NotNil(t, got.Notes) {
		assert.Equal(t, notes, *got.Notes)
	}

	// Sad path: a malformed brand_id fails the final-step approve with
	// asset.ErrInvalidRef, and no new asset row is created for this request.
	t.Run("malformed brand_id fails approve with invalid reference", func(t *testing.T) {
		badBrandID := "not-a-uuid"
		badPayload, err := json.Marshal(asset.AssetCreatePayload{
			Name:       "Bad Brand Asset",
			CategoryID: catIDStr,
			OfficeID:   officeIDStr,
			AssetClass: "intangible",
			BrandID:    &badBrandID,
		})
		require.NoError(t, err)

		maker2 := seedUser(t, pool, officeRoleID, "makerbadbrand@test.local")
		approver2 := seedUser(t, pool, officeRoleID, "approverbadbrand@test.local")

		req2, err := svc.Submit(ctx, approval.SubmitInput{
			Type:     sqlc.SharedRequestTypeAssetCreate,
			Amount:   "5000000",
			OfficeID: officeID,
			Payload:  badPayload,
			Maker:    maker2,
		})
		require.NoError(t, err)

		caller2 := buildCaller(approver2, officeRoleID, false, []uuid.UUID{tr.CabangID})
		_, err = svc.Decide(ctx, req2.ID, caller2, true, nil)
		require.Error(t, err)
		assert.ErrorIs(t, err, asset.ErrInvalidRef)

		_, total, err := assetSvc.List(ctx, asset.ListInput{AllScope: true, OfficeIDs: nil, Limit: 10, Offset: 0})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total, "no new asset should have been created for the failed request")
	})
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
	deprSvc := depreciation.NewService(q, pool)
	disposalSvc := disposal.NewService(q, pool, svc, deprSvc)
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetDisposal, disposalSvc.Executor())

	targetEntity := "assets"
	disposalPayload, err := json.Marshal(disposal.DisposalPayload{Method: "write_off", DisposalDate: "2026-07-01"})
	require.NoError(t, err)
	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:         sqlc.SharedRequestTypeAssetDisposal,
		Amount:       "1000000", // 1M < 5M → 1-step office chain
		OfficeID:     tr.CabangID,
		TargetEntity: &targetEntity,
		TargetID:     &assetID,
		Payload:      disposalPayload,
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
	deprSvc := depreciation.NewService(q, pool)
	disposalSvc := disposal.NewService(q, pool, svc, deprSvc)
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetDisposal, disposalSvc.Executor())

	targetEntity := "assets"
	disposalPayload, err := json.Marshal(disposal.DisposalPayload{Method: "write_off", DisposalDate: "2026-07-01"})
	require.NoError(t, err)
	// Submit the disposal referencing otherOfficeID but asset lives in CabangID
	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:         sqlc.SharedRequestTypeAssetDisposal,
		Amount:       "1000000",
		OfficeID:     otherOfficeID, // mismatch!
		TargetEntity: &targetEntity,
		TargetID:     &assetID,
		Payload:      disposalPayload,
		Maker:        maker,
	})
	require.NoError(t, err)

	caller := buildCaller(approver, officeRoleID, false, []uuid.UUID{otherOfficeID, tr.CabangID, tr.WilayahID})
	_, err = svc.Decide(ctx, req.ID, caller, true, nil)
	require.ErrorIs(t, err, approval.ErrInvalidRef, "cross-office disposal should fail with ErrInvalidRef at executor")

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
	rows, total, err := svc.List(ctx, false, []uuid.UUID{tr.CabangID}, "", "", 10, 0, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, rows, 1)
	assert.Equal(t, &tr.CabangID, rows[0].ApprovalRequest.OfficeID)

	// Global scope sees all
	rows, total, err = svc.List(ctx, true, nil, "", "", 10, 0, nil)
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
	deprSvc := depreciation.NewService(q, pool)
	disposalSvc := disposal.NewService(q, pool, svc, deprSvc)
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetDisposal, disposalSvc.Executor())

	// Use 1-step disposal threshold: amount < 5M
	_, err := pool.Exec(ctx, `TRUNCATE approval.approval_thresholds`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx,
		`INSERT INTO approval.approval_thresholds
		   (request_type, amount_from, amount_to, required_level, step_order)
		 VALUES ('asset_disposal', 0, NULL, 'office', 1)`)
	require.NoError(t, err)

	targetEntity := "assets"
	disposalPayload, err := json.Marshal(disposal.DisposalPayload{Method: "write_off", DisposalDate: "2026-07-01"})
	require.NoError(t, err)
	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:         sqlc.SharedRequestTypeAssetDisposal,
		Amount:       "1000000",
		OfficeID:     tr.CabangID,
		TargetEntity: &targetEntity,
		TargetID:     &assetID,
		Payload:      disposalPayload,
		Maker:        maker,
	})
	require.NoError(t, err)

	// The approver tries to approve, executor will fail because asset is already disposed
	caller := buildCaller(approver, officeRoleID, false, []uuid.UUID{tr.CabangID})
	_, err = svc.Decide(ctx, req.ID, caller, true, nil)
	require.Error(t, err, "executor should fail on invalid transition disposed→disposed")
	// Verify the error is the expected invalid-reference error (the disposal executor
	// treats an illegal status transition as approval.ErrInvalidRef).
	assert.True(t, errors.Is(err, approval.ErrInvalidRef), "expected approval.ErrInvalidRef, got: %v", err)

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

	// A MAKER may view their OWN request even when the request's office lies
	// OUTSIDE their data scope — parity with the mine=true list bypass. Here
	// outsideUser (scoped to otherOffice only) submits a request for Cabang.
	outsiderOwnReq, err := svc.Submit(ctx, approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   "500000",
		OfficeID: tr.CabangID,
		Payload:  []byte(`{"name":"Own","category_id":"` + uuid.New().String() + `","office_id":"` + tr.CabangID.String() + `","asset_class":"intangible"}`),
		Maker:    outsideUserID,
	})
	require.NoError(t, err)

	callGetReq := func(reqID, userID, roleID uuid.UUID) (int, map[string]any) {
		stubAuth := func(c *gin.Context) {
			c.Set(middleware.CtxUserID, userID.String())
			c.Set(middleware.CtxRoleID, roleID.String())
			c.Next()
		}
		r := gin.New()
		v1 := r.Group("/api/v1")
		approval.RegisterRoutes(v1, h, stubAuth, permSvc)
		w := httptest.NewRecorder()
		httpReq, _ := http.NewRequest(http.MethodGet, "/api/v1/requests/"+reqID.String(), nil)
		r.ServeHTTP(w, httpReq)
		var body map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		return w.Code, body
	}

	makerCode, makerBody := callGetReq(outsiderOwnReq.ID, outsideUserID, outsideRoleID)
	assert.Equal(t, http.StatusOK, makerCode,
		"a maker must be able to view their own request regardless of office scope")
	// The bypass must return the maker's actual request, not an empty/masked shell.
	assert.Equal(t, outsiderOwnReq.ID.String(), makerBody["id"],
		"the maker's own request body must be returned through the scope bypass")
}

// strPtr returns a pointer to the given string.
func strPtr(s string) *string { return &s }

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
		rows, total, err := svc.List(ctx, true, nil, "pending", "asset_create", 20, 0, nil)
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

// TestApproval_EnrichedReads_SoftDeletedActors verifies that soft-deleting the
// maker user and the request's office does not hide the request itself — the
// enrichment LEFT JOINs simply resolve to nil names, per the LEFT JOIN
// visibility contract (a soft-deleted actor/office must not orphan the
// request from list views).
func TestApproval_EnrichedReads_SoftDeletedActors(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "SDA")

	officeRoleID := lookupRole(t, pool, "Kepala Unit")
	maker := seedUser(t, pool, officeRoleID, "maker.softdel@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)
	assetSvc := asset.NewService(q, pool, storage.NewFake(), 0, "")
	svc.RegisterExecutor(sqlc.SharedRequestTypeAssetCreate, assetSvc.CreateExecutor())

	catIDStr := catID.String()
	officeIDStr := tr.CabangID.String()
	payload, _ := json.Marshal(asset.AssetCreatePayload{
		Name: "Soft Delete Test Asset", CategoryID: catIDStr, OfficeID: officeIDStr,
		AssetClass: "intangible",
	})
	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type: sqlc.SharedRequestTypeAssetCreate, Amount: "1000000",
		OfficeID: tr.CabangID, Payload: payload, Maker: maker,
	})
	require.NoError(t, err)

	// Soft-delete the maker user and the office.
	_, err = pool.Exec(ctx, `UPDATE identity.users SET deleted_at = now() WHERE id = $1`, maker)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE masterdata.offices SET deleted_at = now() WHERE id = $1`, tr.CabangID)
	require.NoError(t, err)

	rows, total, err := svc.List(ctx, true, nil, "", "", 20, 0, nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))

	var found *sqlc.ListRequestsEnrichedRow
	for i := range rows {
		if rows[i].ApprovalRequest.ID == req.ID {
			found = &rows[i]
			break
		}
	}
	require.NotNil(t, found, "request must still appear in List after its maker/office are soft-deleted")
	assert.Nil(t, found.RequestedByName, "requested_by_name must be nil once the maker is soft-deleted")
	assert.Nil(t, found.OfficeName, "office_name must be nil once the office is soft-deleted")
}

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

// TestApproval_FieldMasking_HandlerWiring drives the real HTTP handler (gin
// engine built via approval.RegisterRoutes, exactly like
// TestApproval_GetRequest_ScopeEnforced) end-to-end and proves that
// h.filterMap actually executes on the list/get response path — not just that
// the underlying authz.FilterView helper works in isolation (that is already
// covered by TestApproval_FieldMasking_Requests). If someone deleted the
// h.filterMap call sites in list/get, this test must fail.
func TestApproval_FieldMasking_HandlerWiring(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "HWM")

	makerRoleID := lookupRole(t, pool, "Kepala Unit")
	maker := seedUser(t, pool, makerRoleID, "maker.handlerwiring@test.local")

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)

	// Submit a request against Cabang. No executor is registered and the
	// request is never decided — only read paths (list/get) are under test.
	req, err := svc.Submit(ctx, approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   "1000000",
		OfficeID: tr.CabangID,
		Payload:  []byte(`{"name":"Handler Wiring Asset","category_id":"` + catID.String() + `","office_id":"` + tr.CabangID.String() + `","asset_class":"intangible"}`),
		Reason:   strPtr("handler wiring regression test"),
		Maker:    maker,
	})
	require.NoError(t, err)

	// Staf: real seeded role, default data-scope "own" (module '*'). Per
	// common.ScopedDeps.CallerOfficeScope, "own" resolves to the caller's own
	// office — so placing the Staf user in Cabang (the request's office) is
	// enough for them to see the row on both list and get, with no need to
	// seed an extra data_scope_policies override.
	stafRoleID := lookupRole(t, pool, "Staf")
	stafUser := seedUser(t, pool, stafRoleID, "staf.handlerwiring@test.local")
	_, err = pool.Exec(ctx,
		`UPDATE identity.users SET office_id = $1 WHERE id = $2`, tr.CabangID, stafUser)
	require.NoError(t, err)

	// Deny view on amount + payload for Staf on entity "requests" (same
	// INSERT shape as TestApproval_FieldMasking_Requests above).
	_, err = pool.Exec(ctx, `
		INSERT INTO identity.field_permissions (role_id, entity, field, can_view, can_edit)
		VALUES ($1, 'requests', 'amount', false, false),
		       ($1, 'requests', 'payload', false, false)`, stafRoleID)
	require.NoError(t, err)

	// Superadmin: global scope + no field-permission policy for "requests" →
	// default-allow control. Global scope means no office placement is needed.
	adminRoleID := lookupRole(t, pool, "Superadmin")
	adminUser := seedUser(t, pool, adminRoleID, "admin.handlerwiring@test.local")

	gin.SetMode(gin.TestMode)
	auditSvc := audit.NewService(q)
	fieldSvc := authz.NewFieldService(q, rdb)
	scoped := common.ScopedDeps{Q: q, Scope: scopeSvc}
	h := approval.NewHandler(svc, fieldSvc, scoped, auditSvc)
	permSvc := authz.NewPermissionService(q, rdb)

	// doGet builds a fresh gin engine with a stub auth MW injecting the given
	// user/role IDs (bypassing real JWT) and drives a GET against path,
	// decoding the JSON body into a map for inspection.
	doGet := func(path string, userID, roleID uuid.UUID) (int, map[string]any) {
		stubAuth := func(c *gin.Context) {
			c.Set(middleware.CtxUserID, userID.String())
			c.Set(middleware.CtxRoleID, roleID.String())
			c.Next()
		}
		r := gin.New()
		v1 := r.Group("/api/v1")
		approval.RegisterRoutes(v1, h, stubAuth, permSvc)
		w := httptest.NewRecorder()
		httpReq, err := http.NewRequest(http.MethodGet, path, nil)
		require.NoError(t, err)
		r.ServeHTTP(w, httpReq)
		var body map[string]any
		if w.Body.Len() > 0 {
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		}
		return w.Code, body
	}

	t.Run("list masks amount for Staf but keeps id/status", func(t *testing.T) {
		code, body := doGet("/api/v1/requests", stafUser, stafRoleID)
		require.Equal(t, http.StatusOK, code)
		rows, ok := body["data"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, rows, "Staf placed in Cabang must see the Cabang request on list")
		for _, raw := range rows {
			row, ok := raw.(map[string]any)
			require.True(t, ok)
			assert.NotContains(t, row, "amount")
			assert.Contains(t, row, "id")
			assert.Contains(t, row, "status")
		}
	})

	t.Run("get masks amount and payload for Staf but keeps steps", func(t *testing.T) {
		code, body := doGet("/api/v1/requests/"+req.ID.String(), stafUser, stafRoleID)
		require.Equal(t, http.StatusOK, code)
		assert.NotContains(t, body, "amount")
		assert.NotContains(t, body, "payload")
		assert.Contains(t, body, "steps")
	})

	t.Run("list keeps amount for Superadmin (default-allow control)", func(t *testing.T) {
		code, body := doGet("/api/v1/requests", adminUser, adminRoleID)
		require.Equal(t, http.StatusOK, code)
		rows, ok := body["data"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, rows)
		found := false
		for _, raw := range rows {
			row, ok := raw.(map[string]any)
			require.True(t, ok)
			if row["id"] == req.ID.String() {
				found = true
				assert.Contains(t, row, "amount")
			}
		}
		assert.True(t, found, "Superadmin (global scope) must see the request on list")
	})

	t.Run("get keeps amount and payload for Superadmin (default-allow control)", func(t *testing.T) {
		code, body := doGet("/api/v1/requests/"+req.ID.String(), adminUser, adminRoleID)
		require.Equal(t, http.StatusOK, code)
		assert.Contains(t, body, "amount")
		assert.Contains(t, body, "payload")
		assert.Contains(t, body, "steps")
	})

	// /requests/inbox additionally requires the request.decide permission
	// (RegisterRoutes attaches middleware.RequirePermission for it) and,
	// beyond that, ListInboxCandidatesEnriched only returns rows whose
	// current pending step matches the caller's approver eligibility
	// (role/office tier) — a third orthogonal condition on top of scope and
	// field permissions. Reproducing that eligibility setup here would
	// duplicate large parts of TestApproval_AssetCreate_ThreeStep without
	// adding coverage for the field-masking wiring itself (inbox calls the
	// exact same h.filterMap as list), so it is intentionally left to the
	// existing approval-flow tests. list + get above already exercise every
	// call site of h.filterMap in the handler.
}

// TestApproval_ThresholdPreview covers Service.PreviewChain directly against a
// deterministic 2-band asset_disposal configuration, then drives the real
// GET /api/v1/approval-thresholds/preview endpoint end-to-end (stub auth +
// approval.RegisterRoutes + httptest, exactly like
// TestApproval_FieldMasking_HandlerWiring) to prove the handler + route wiring.
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

	// HTTP wiring: real gin engine via approval.RegisterRoutes + stub auth MW.
	// The seeded Superadmin role already has request.create (the gate on this
	// route), so no extra permission seeding is needed.
	gin.SetMode(gin.TestMode)
	auditSvc := audit.NewService(q)
	fieldSvc := authz.NewFieldService(q, rdb)
	scoped := common.ScopedDeps{Q: q, Scope: scopeSvc}
	h := approval.NewHandler(svc, fieldSvc, scoped, auditSvc)
	permSvc := authz.NewPermissionService(q, rdb)

	adminRoleID := lookupRole(t, pool, "Superadmin")
	adminUser := seedUser(t, pool, adminRoleID, "admin.thresholdpreview@test.local")

	doGet := func(path string) (int, []byte) {
		stubAuth := func(c *gin.Context) {
			c.Set(middleware.CtxUserID, adminUser.String())
			c.Set(middleware.CtxRoleID, adminRoleID.String())
			c.Next()
		}
		r := gin.New()
		v1 := r.Group("/api/v1")
		approval.RegisterRoutes(v1, h, stubAuth, permSvc)
		w := httptest.NewRecorder()
		httpReq, err := http.NewRequest(http.MethodGet, path, nil)
		require.NoError(t, err)
		r.ServeHTTP(w, httpReq)
		return w.Code, w.Body.Bytes()
	}

	t.Run("HTTP: 200 with matching step for valid type+amount", func(t *testing.T) {
		code, body := doGet("/api/v1/approval-thresholds/preview?request_type=asset_disposal&amount=500000")
		require.Equal(t, http.StatusOK, code)
		assert.Contains(t, string(body), `"required_level":"office"`)
	})

	t.Run("HTTP: 400 for invalid request_type", func(t *testing.T) {
		code, _ := doGet("/api/v1/approval-thresholds/preview?request_type=bogus&amount=500000")
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("HTTP: 400 for fraction amount (would reach DB as invalid numeric)", func(t *testing.T) {
		code, _ := doGet("/api/v1/approval-thresholds/preview?request_type=asset_disposal&amount=" + url.QueryEscape("1/3"))
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("HTTP: 400 for non-numeric amount", func(t *testing.T) {
		code, _ := doGet("/api/v1/approval-thresholds/preview?request_type=asset_disposal&amount=abc")
		assert.Equal(t, http.StatusBadRequest, code)
	})

	t.Run("HTTP: 400 for missing amount", func(t *testing.T) {
		code, _ := doGet("/api/v1/approval-thresholds/preview?request_type=asset_disposal")
		assert.Equal(t, http.StatusBadRequest, code)
	})
}

// TestApproval_InboxCount drives GET /api/v1/requests/inbox/count end-to-end
// (real gin engine via approval.RegisterRoutes + stub auth MW, exactly like
// TestApproval_FieldMasking_HandlerWiring) and verifies two things: the
// returned count equals len(GET /requests/inbox's data) for the same eligible
// decider, and a caller lacking the request.decide permission is rejected
// with 403 rather than being counted.
func TestApproval_InboxCount(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	tr := seedTieredOfficeTree(t, pool)
	catID := seedCategory(t, pool, "IBC")

	officeRoleID := lookupRole(t, pool, "Kepala Unit")
	maker := seedUser(t, pool, officeRoleID, "maker.inboxcount@test.local")
	approver := seedUser(t, pool, officeRoleID, "approver.inboxcount@test.local")
	// Place the approver in Cabang so their office_subtree scope (kepala_unit's
	// default data-scope policy) covers the request's office.
	_, err := pool.Exec(ctx,
		`UPDATE identity.users SET office_id = $1 WHERE id = $2`, tr.CabangID, approver)
	require.NoError(t, err)

	q := sqlc.New(pool)
	scopeSvc := authz.NewScopeService(q, rdb)
	svc := approval.NewService(q, pool, scopeSvc, rdb)

	catIDStr := catID.String()
	officeIDStr := tr.CabangID.String()
	payload, _ := json.Marshal(asset.AssetCreatePayload{
		Name:       "Inbox Count Asset",
		CategoryID: catIDStr,
		OfficeID:   officeIDStr,
		AssetClass: "intangible",
	})
	// 1,000,000 falls in the 1-step (office) band per migration 000016.
	_, err = svc.Submit(ctx, approval.SubmitInput{
		Type:     sqlc.SharedRequestTypeAssetCreate,
		Amount:   "1000000",
		OfficeID: tr.CabangID,
		Payload:  payload,
		Maker:    maker,
	})
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	auditSvc := audit.NewService(q)
	fieldSvc := authz.NewFieldService(q, rdb)
	scoped := common.ScopedDeps{Q: q, Scope: scopeSvc}
	h := approval.NewHandler(svc, fieldSvc, scoped, auditSvc)
	permSvc := authz.NewPermissionService(q, rdb)

	doGet := func(path string, userID, roleID uuid.UUID) (int, map[string]any) {
		stubAuth := func(c *gin.Context) {
			c.Set(middleware.CtxUserID, userID.String())
			c.Set(middleware.CtxRoleID, roleID.String())
			c.Next()
		}
		r := gin.New()
		v1 := r.Group("/api/v1")
		approval.RegisterRoutes(v1, h, stubAuth, permSvc)
		w := httptest.NewRecorder()
		httpReq, err := http.NewRequest(http.MethodGet, path, nil)
		require.NoError(t, err)
		r.ServeHTTP(w, httpReq)
		var body map[string]any
		if w.Body.Len() > 0 {
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		}
		return w.Code, body
	}

	t.Run("count equals length of inbox data for an eligible decider", func(t *testing.T) {
		code, inboxBody := doGet("/api/v1/requests/inbox", approver, officeRoleID)
		require.Equal(t, http.StatusOK, code)
		data, ok := inboxBody["data"].([]any)
		require.True(t, ok)
		require.NotEmpty(t, data, "approver placed in Cabang must see the pending request in inbox")

		code, countBody := doGet("/api/v1/requests/inbox/count", approver, officeRoleID)
		require.Equal(t, http.StatusOK, code)
		count, ok := countBody["count"].(float64)
		require.True(t, ok, "response must have a numeric count field")
		assert.Equal(t, float64(len(data)), count, "inbox/count must equal len(inbox data) for the same caller")
	})

	t.Run("caller without request.decide is rejected with 403", func(t *testing.T) {
		stafRoleID := lookupRole(t, pool, "Staf")
		stafUser := seedUser(t, pool, stafRoleID, "staf.inboxcount@test.local")
		code, body := doGet("/api/v1/requests/inbox/count", stafUser, stafRoleID)
		assert.Equal(t, http.StatusForbidden, code)
		assert.NotContains(t, body, "count", "a rejected caller must not receive a count")
	})
}
