//go:build integration

package asset_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/asset"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/storage"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func resetAll(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	// Each test gets its own throwaway container (NewPostgres), so the DB is
	// already clean post-migration. We only truncate mutable schemas here,
	// leaving identity intact so migration-seeded roles remain available.
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`TRUNCATE approval.request_approvals, approval.requests,
		 asset.assets CASCADE`)
	require.NoError(t, err)
}

// seedOfficeWithType inserts a single-office setup (one type, one office) and
// returns the office ID.
func seedOfficeWithType(t *testing.T, pool *pgxpool.Pool, typeCode, officeCode string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	var typeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.office_types (name) VALUES ($1) RETURNING id`,
		typeCode).Scan(&typeID))

	var officeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, $2, $3) RETURNING id`,
		typeID, officeCode, officeCode).Scan(&officeID))

	return officeID
}

// seedCategory inserts a masterdata.categories row (intangible) and returns its id.
func seedCategory(t *testing.T, pool *pgxpool.Pool, code string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.categories (name, code, asset_class)
		 VALUES ($1, $2, 'intangible') RETURNING id`,
		code, code).Scan(&id))
	return id
}

// seedAssetDirect inserts an asset.assets row directly and returns its id.
func seedAssetDirect(t *testing.T, pool *pgxpool.Pool, tag, name string, categoryID, officeID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO asset.assets
		   (asset_tag, name, category_id, office_id, asset_class, capitalized, specifications, status)
		 VALUES ($1, $2, $3, $4, 'intangible', true, '{}', 'available')
		 RETURNING id`,
		tag, name, categoryID, officeID).Scan(&id))
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

// ─── tests ───────────────────────────────────────────────────────────────────

// TestAsset_FieldMasking_ByRole verifies that field permissions seeded in migration
// 000016 cause FilterView to strip cost/value fields from roles that lack view rights.
func TestAsset_FieldMasking_ByRole(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	q := sqlc.New(pool)
	fieldSvc := authz.NewFieldService(q, rdb)

	// Migration 000016 seeds roles: Superadmin, Manager, Kepala Kanwil, Kepala Unit, Staf.
	// Sejak migrasi 000037 ketiga kolom finansial (purchase_cost, book_value,
	// accumulated_depreciation) jadi SATU tier: visible untuk Superadmin + Manager,
	// hidden dari Kepala Unit/Kanwil + Staf.

	superadminRoleID := lookupRole(t, pool, "Superadmin")
	managerRoleID := lookupRole(t, pool, "Manager")
	stafRoleID := lookupRole(t, pool, "Staf")
	kepalaUnitRoleID := lookupRole(t, pool, "Kepala Unit")

	// Build a representative asset record as a map[string]any
	sampleRecord := func() map[string]any {
		return map[string]any{
			"id":                       uuid.New().String(),
			"name":                     "Laptop Test",
			"purchase_cost":            "5000000",
			"book_value":               "4000000",
			"accumulated_depreciation": "1000000",
		}
	}

	t.Run("superadmin sees all fields", func(t *testing.T) {
		rec := sampleRecord()
		pol, err := fieldSvc.ForEntity(ctx, superadminRoleID, "assets")
		require.NoError(t, err)
		authz.FilterView(pol, rec)
		assert.Contains(t, rec, "purchase_cost")
		assert.Contains(t, rec, "book_value")
		assert.Contains(t, rec, "accumulated_depreciation")
	})

	t.Run("manager sees all three financial fields (satu tier, migrasi 000037)", func(t *testing.T) {
		// Sejak 000037 ketiga kolom finansial jadi satu tier: Manager (seperti
		// Superadmin) melihat purchase_cost + book_value + accumulated_depreciation.
		rec := sampleRecord()
		pol, err := fieldSvc.ForEntity(ctx, managerRoleID, "assets")
		require.NoError(t, err)
		authz.FilterView(pol, rec)
		assert.Contains(t, rec, "purchase_cost")
		assert.Contains(t, rec, "book_value")
		assert.Contains(t, rec, "accumulated_depreciation")
	})

	t.Run("staf sees neither cost nor value fields", func(t *testing.T) {
		rec := sampleRecord()
		pol, err := fieldSvc.ForEntity(ctx, stafRoleID, "assets")
		require.NoError(t, err)
		authz.FilterView(pol, rec)
		assert.NotContains(t, rec, "purchase_cost")
		assert.NotContains(t, rec, "book_value")
		assert.NotContains(t, rec, "accumulated_depreciation")
	})

	t.Run("kepala unit sees neither cost nor value fields", func(t *testing.T) {
		rec := sampleRecord()
		pol, err := fieldSvc.ForEntity(ctx, kepalaUnitRoleID, "assets")
		require.NoError(t, err)
		authz.FilterView(pol, rec)
		assert.NotContains(t, rec, "purchase_cost")
		assert.NotContains(t, rec, "book_value")
		assert.NotContains(t, rec, "accumulated_depreciation")
	})

	t.Run("fields without explicit policy are not filtered", func(t *testing.T) {
		rec := sampleRecord()
		pol, err := fieldSvc.ForEntity(ctx, stafRoleID, "assets")
		require.NoError(t, err)
		authz.FilterView(pol, rec)
		// 'name' and 'id' have no policy → must remain
		assert.Contains(t, rec, "name")
		assert.Contains(t, rec, "id")
	})
}

// TestAsset_TagAtomicity_Sequential verifies that sequential calls to
// GenerateAssetTag inside separate transactions produce monotonically increasing
// sequence numbers without gaps.
func TestAsset_TagAtomicity_Sequential(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	_ = testsupport.NewRedis(t) // not used by asset service but keep for parity
	ctx := context.Background()
	resetAll(t, pool)

	officeID := seedOfficeWithType(t, pool, "BranchType", "JKT01")
	catID := seedCategory(t, pool, "ELK")

	q := sqlc.New(pool)
	svc := asset.NewService(q, pool, storage.NewFake(), 0, "")

	const n = 5
	tags := make([]string, n)
	for i := 0; i < n; i++ {
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		qtx := q.WithTx(tx)
		tag, seq, err := svc.GenerateAssetTag(ctx, qtx, officeID, catID, 2026)
		require.NoError(t, err)
		_, err = qtx.CreateAsset(ctx, sqlc.CreateAssetParams{
			AssetTag: tag, TagSeq: &seq, Name: "Asset", CategoryID: catID,
			OfficeID: officeID, AssetClass: sqlc.SharedAssetClassIntangible,
			Capitalized: true, Specifications: []byte("{}"),
		})
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
		tags[i] = tag
	}

	// All tags should be unique
	seen := make(map[string]bool)
	for _, tag := range tags {
		assert.False(t, seen[tag], "duplicate tag: %s", tag)
		seen[tag] = true
	}

	// Sequence must be strictly ascending (suffix 00001 through 00005)
	assert.Equal(t, "JKT01ELK202600001", tags[0])
	assert.Equal(t, "JKT01ELK202600002", tags[1])
	assert.Equal(t, "JKT01ELK202600005", tags[4])
}

// TestAsset_TagSeq_PerOfficeNotPerYear verifies the sequence is per-OFFICE and does
// NOT reset per year (the year only appears in the tag string, not the counter).
func TestAsset_TagSeq_PerOfficeNotPerYear(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	_ = testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	officeID := seedOfficeWithType(t, pool, "BranchType2", "BDG01")
	catID := seedCategory(t, pool, "FRN")

	q := sqlc.New(pool)
	svc := asset.NewService(q, pool, storage.NewFake(), 0, "")

	genTag := func(year int32) string {
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		qtx := q.WithTx(tx)
		tag, seq, err := svc.GenerateAssetTag(ctx, qtx, officeID, catID, year)
		require.NoError(t, err)
		_, err = qtx.CreateAsset(ctx, sqlc.CreateAssetParams{
			AssetTag: tag, TagSeq: &seq, Name: "Asset", CategoryID: catID,
			OfficeID: officeID, AssetClass: sqlc.SharedAssetClassIntangible,
			Capitalized: true, Specifications: []byte("{}"),
		})
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
		return tag
	}

	tag2025a := genTag(2025)
	tag2025b := genTag(2025)
	tag2026a := genTag(2026)
	tag2026b := genTag(2026)

	// Sequence is per-OFFICE and does NOT reset per year: year shows in the string,
	// but the running number keeps advancing across years.
	assert.Equal(t, "BDG01FRN202500001", tag2025a)
	assert.Equal(t, "BDG01FRN202500002", tag2025b)
	assert.Equal(t, "BDG01FRN202600003", tag2026a)
	assert.Equal(t, "BDG01FRN202600004", tag2026b)
}

// TestAsset_TagSeq_SurvivesTransferOut is the regression guard for a real bug:
// the sequence used to be derived from MAX(tag_seq) WHERE office_id = <office>.
// A transfer moves office_id to the destination and takes tag_seq with it, so the
// source office's MAX dropped and the NEXT create REISSUED that number — producing
// a duplicate asset_tag (23505, surfaced as an opaque 500) whenever the category
// and year happened to match, and violating the spec rule that a sequence number
// is never reused. Migration 000046 anchors the counter to tag_office_id (the
// ISSUING office), which a transfer never changes.
func TestAsset_TagSeq_SurvivesTransferOut(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	_ = testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	srcOffice := seedOfficeWithType(t, pool, "SrcType", "SRC01")
	dstOffice := seedOfficeWithType(t, pool, "DstType", "DST01")
	catID := seedCategory(t, pool, "SWL")

	q := sqlc.New(pool)
	svc := asset.NewService(q, pool, storage.NewFake(), 0, "")

	create := func() (uuid.UUID, string) {
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		qtx := q.WithTx(tx)
		tag, seq, err := svc.GenerateAssetTag(ctx, qtx, srcOffice, catID, 2026)
		require.NoError(t, err)
		row, err := qtx.CreateAsset(ctx, sqlc.CreateAssetParams{
			AssetTag: tag, TagSeq: &seq, Name: "Asset", CategoryID: catID,
			OfficeID: srcOffice, AssetClass: sqlc.SharedAssetClassIntangible,
			Capitalized: true, Specifications: []byte("{}"),
		})
		require.NoError(t, err)
		require.NoError(t, tx.Commit(ctx))
		return row.ID, tag
	}

	_, first := create()
	movedID, moved := create()
	assert.Equal(t, "SRC01SWL202600001", first)
	assert.Equal(t, "SRC01SWL202600002", moved)

	// Transfer the highest-numbered asset OUT of the source office (what the
	// transfer receive step does).
	_, err := q.SetAssetOffice(ctx, sqlc.SetAssetOfficeParams{ID: movedID, OfficeID: dstOffice, RoomID: nil})
	require.NoError(t, err)

	// The next create in the SOURCE office must continue at 3 — not reuse 2.
	_, next := create()
	assert.Equal(t, "SRC01SWL202600003", next,
		"sequence must not be reused after the top asset is transferred out")
	assert.NotEqual(t, moved, next, "reissuing the number would duplicate the moved asset's tag")

	// The destination office keeps its own (independent) sequence: the incoming
	// asset was issued by SRC01, so it must not advance DST01's counter.
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	qtx := q.WithTx(tx)
	dstTag, _, err := svc.GenerateAssetTag(ctx, qtx, dstOffice, catID, 2026)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	assert.Equal(t, "DST01SWL202600001", dstTag,
		"a transferred-in asset must not consume the destination office's sequence")
}

// TestAsset_ReadScope_OfficeFiltered verifies that List correctly filters assets
// by office ID when AllScope=false.
func TestAsset_ReadScope_OfficeFiltered(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	_ = testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	office1ID := seedOfficeWithType(t, pool, "OfficeType", "OFF1")
	// Second office with the same type (already inserted above); add a sibling
	var typeID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT office_type_id FROM masterdata.offices WHERE code = 'OFF1' LIMIT 1`).
		Scan(&typeID))
	var office2ID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, 'Office 2', 'OFF2') RETURNING id`, typeID).
		Scan(&office2ID))

	catID := seedCategory(t, pool, "SCO")

	q := sqlc.New(pool)
	svc := asset.NewService(q, pool, storage.NewFake(), 0, "")

	// Seed 2 assets in office1, 1 in office2
	seedAssetDirect(t, pool, "OFF1-SCO-2026-00001", "Asset A", catID, office1ID)
	seedAssetDirect(t, pool, "OFF1-SCO-2026-00002", "Asset B", catID, office1ID)
	seedAssetDirect(t, pool, "OFF2-SCO-2026-00001", "Asset C", catID, office2ID)

	t.Run("office1 scope returns only office1 assets", func(t *testing.T) {
		rows, total, err := svc.List(ctx, asset.ListInput{
			AllScope:  false,
			OfficeIDs: []uuid.UUID{office1ID},
			Limit:     10,
			Offset:    0,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, rows, 2)
		for _, a := range rows {
			assert.Equal(t, office1ID, a.OfficeID)
		}
	})

	t.Run("global scope returns all assets", func(t *testing.T) {
		rows, total, err := svc.List(ctx, asset.ListInput{
			AllScope:  true,
			OfficeIDs: nil,
			Limit:     10,
			Offset:    0,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(3), total)
		assert.Len(t, rows, 3)
	})

	t.Run("empty office filter returns no assets", func(t *testing.T) {
		rows, total, err := svc.List(ctx, asset.ListInput{
			AllScope:  false,
			OfficeIDs: []uuid.UUID{},
			Limit:     10,
			Offset:    0,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(0), total)
		assert.Len(t, rows, 0)
	})

	t.Run("office2 scope returns only office2 asset", func(t *testing.T) {
		rows, total, err := svc.List(ctx, asset.ListInput{
			AllScope:  false,
			OfficeIDs: []uuid.UUID{office2ID},
			Limit:     10,
			Offset:    0,
		})
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, rows, 1)
		assert.Equal(t, "Asset C", rows[0].Name)
	})
}

// TestAsset_UpdateWritesLocationAndPICHistory verifies svc.Update records a
// location-history row when floor/room changes and a PIC-history row when the
// PIC changes (Fase 3 legacy-parity).
func TestAsset_UpdateWritesLocationAndPICHistory(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	_ = testsupport.NewRedis(t)
	ctx := context.Background()
	resetAll(t, pool)

	officeID := seedOfficeWithType(t, pool, "HistType", "HST01")
	catID := seedCategory(t, pool, "HIS")

	var floorID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.floors (office_id, name) VALUES ($1, 'Lantai 1') RETURNING id`,
		officeID).Scan(&floorID))
	var roomID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.rooms (floor_id, name) VALUES ($1, 'Ruang A') RETURNING id`,
		floorID).Scan(&roomID))

	var empID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.employees (code, name, office_id) VALUES ('E-HIS-1', 'PIC Andi', $1) RETURNING id`,
		officeID).Scan(&empID))
	roleID := lookupRole(t, pool, "Superadmin")
	var actorID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, role_id) VALUES ('Actor His', 'actor-his@test.local', $1) RETURNING id`,
		roleID).Scan(&actorID))

	assetID := seedAssetDirect(t, pool, "HST01HIS202600001", "Aset Uji", catID, officeID)

	q := sqlc.New(pool)
	svc := asset.NewService(q, pool, storage.NewFake(), 0, "")

	baseInput := func() asset.UpdateInput {
		return asset.UpdateInput{Name: "Aset Uji", CategoryID: catID, Specifications: []byte("{}")}
	}

	// 1) Change floor+room -> one location-history row (source=edit).
	in := baseInput()
	in.FloorID = &floorID
	in.RoomID = &roomID
	_, _, err := svc.Update(ctx, assetID, in, actorID)
	require.NoError(t, err)

	locs, err := svc.ListLocationHistory(ctx, assetID)
	require.NoError(t, err)
	require.Len(t, locs, 1)
	assert.Equal(t, sqlc.SharedLocationChangeSourceEdit, locs[0].Source)
	require.NotNil(t, locs[0].RoomID)
	assert.Equal(t, roomID, *locs[0].RoomID)

	// 2) Assign a PIC (location unchanged) -> one active PIC-history row, no new location row.
	in = baseInput()
	in.FloorID = &floorID
	in.RoomID = &roomID
	in.PICEmployeeID = &empID
	_, _, err = svc.Update(ctx, assetID, in, actorID)
	require.NoError(t, err)

	pics, err := svc.ListPICHistory(ctx, assetID)
	require.NoError(t, err)
	require.Len(t, pics, 1)
	assert.Equal(t, empID, pics[0].PicEmployeeID)
	assert.False(t, pics[0].ReleasedAt.Valid, "new PIC must be active (released_at null)")

	locs, err = svc.ListLocationHistory(ctx, assetID)
	require.NoError(t, err)
	assert.Len(t, locs, 1)
}
