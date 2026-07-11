//go:build integration

package report_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/internal/report"
)

// ─── FAM report harness (transfers / disposals / opname) ─────────────────────

// insertTransfer seeds an inter-office transfer with an explicit created_at so
// the period filter (which keys on created_at::date) is deterministic. shipped,
// received and bastNo are nullable — pass nil to leave them NULL.
func insertTransfer(t *testing.T, pool *pgxpool.Pool, asset, from, to, requestedBy uuid.UUID, status string, shipped, received, bastNo *string, createdAt string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO transfer.asset_transfers
		   (asset_id, from_office_id, to_office_id, status, requested_by_id,
		    shipped_date, received_date, bast_no, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		asset, from, to, status, requestedBy, shipped, received, bastNo, createdAt)
	require.NoError(t, err)
}

// insertDisposal seeds a disposal (one per asset — the unique index forbids
// two live disposals on the same asset).
func insertDisposal(t *testing.T, pool *pgxpool.Pool, asset, createdBy uuid.UUID, method, disposalDate, proceeds, bookValue, gainLoss string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO disposal.disposals
		   (asset_id, method, disposal_date, proceeds, book_value_at_disposal, gain_loss, created_by_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		asset, method, disposalDate, proceeds, bookValue, gainLoss, createdBy)
	require.NoError(t, err)
}

func insertOpnameSession(t *testing.T, pool *pgxpool.Pool, office, startedBy uuid.UUID, name, period, status string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO stockopname.stock_opname_sessions
		   (office_id, name, period, status, started_by_id)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		office, name, period, status, startedBy).Scan(&id))
	return id
}

func insertOpnameItem(t *testing.T, pool *pgxpool.Pool, session, asset uuid.UUID, result string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO stockopname.stock_opname_items (session_id, asset_id, result)
		 VALUES ($1, $2, $3)`,
		session, asset, result)
	require.NoError(t, err)
}

// ─── transfers ───────────────────────────────────────────────────────────────

// TestReportTransfers drives the mutasi report over a custom window: rows,
// status KPIs, the per-destination chart, NULL date/bast handling, out-of-window
// and out-of-scope exclusion, and — critically — visibility when only the
// DESTINATION office is in scope.
func TestReportTransfers(t *testing.T) {
	svc, pool := newSvc(t)
	e := baseEnv(t, pool)

	// A third office C, destination of an out-of-A-scope transfer.
	var officeC uuid.UUID
	var typeID uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.office_types (name) VALUES ($1) RETURNING id`,
		"TC "+e.cat1Name).Scan(&typeID))
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
		 VALUES (NULL, $1, $2, $3) RETURNING id`, typeID, "C "+e.cat1Name, "OC"+uuid.New().String()[:6]).Scan(&officeC))

	aName := officeName(t, pool, e.officeA)
	bName := officeName(t, pool, e.officeB)

	t1a := intangibleAsset(t, pool, e, e.cat1, e.officeA, "T1", "Aset T1")
	t2a := intangibleAsset(t, pool, e, e.cat1, e.officeA, "T2", "Aset T2")
	t3a := intangibleAsset(t, pool, e, e.cat1, e.officeB, "T3", "Aset T3")
	tOldA := intangibleAsset(t, pool, e, e.cat1, e.officeA, "TOLD", "Aset lama")

	shipped, received, bast := "2025-03-12", "2025-03-14", "BAST-001"
	// T1: A→B received, with dates + bast, in window.
	insertTransfer(t, pool, t1a, e.officeA, e.officeB, e.user, "received", &shipped, &received, &bast, "2025-03-15")
	// T2: A→B in_transit, NULL dates + bast, in window.
	insertTransfer(t, pool, t2a, e.officeA, e.officeB, e.user, "in_transit", nil, nil, nil, "2025-03-16")
	// Tout: B→C in_transit (neither endpoint is A), in window.
	insertTransfer(t, pool, t3a, e.officeB, officeC, e.user, "in_transit", nil, nil, nil, "2025-03-17")
	// Told: A→B but created BEFORE the window — excluded on date.
	insertTransfer(t, pool, tOldA, e.officeA, e.officeB, e.user, "received", nil, nil, nil, "2025-02-01")

	cur, prev := customPeriod(t, "2025-03-01", "2025-03-31")

	// ── Scope A: sees T1 + T2 (source A), not Tout (B→C) nor Told (out of window).
	pA := aScope(e)
	pA.Cur, pA.Prev = cur, prev
	res, err := svc.Run(context.Background(), "transfers", pA)
	require.NoError(t, err)

	rows, ok := res.Rows.([]report.TransferRow)
	require.True(t, ok, "rows are []TransferRow")
	require.Len(t, rows, 2, "T1+T2 only (Tout out of scope, Told out of window)")

	// Rows are ordered by created_at DESC → T2 (03-16) then T1 (03-15).
	assert.Equal(t, "Aset T2", rows[0].AssetName)
	assert.Equal(t, "Aset T1", rows[1].AssetName)

	// T1 content (received, with dates + bast).
	got1 := rows[1]
	assert.Equal(t, aName, got1.FromOffice)
	assert.Equal(t, bName, got1.ToOffice)
	assert.Equal(t, "received", got1.Status)
	assert.Equal(t, "2025-03-12", got1.ShippedDate)
	assert.Equal(t, "2025-03-14", got1.ReceivedDate)
	assert.Equal(t, "BAST-001", got1.BastNo)

	// T2 content (in_transit, NULLs render as empty strings).
	got2 := rows[0]
	assert.Equal(t, "in_transit", got2.Status)
	assert.Empty(t, got2.ShippedDate)
	assert.Empty(t, got2.ReceivedDate)
	assert.Empty(t, got2.BastNo)

	k := kpiMap(res)
	assert.Equal(t, "2", k["total"])
	assert.Equal(t, "1", k["in_transit"])
	assert.Equal(t, "1", k["received"])

	// Chart: destination office B, count 2.
	require.Len(t, res.Chart, 1)
	assert.Equal(t, bName, res.Chart[0].Label)
	assert.Equal(t, "2", res.Chart[0].Value)

	assert.Empty(t, res.Totals, "transfers carry no money tfoot")
	assert.EqualValues(t, 2, res.RowCount)
	assert.False(t, res.Truncated)

	// ── Destination-only scope: scope to B. T1 (A→B) must still be visible even
	// though B is not the source office — proving from-OR-to scope.
	pB := report.ReportParams{All: false, OfficeIDs: []uuid.UUID{e.officeB}, RowLimit: 1000}
	pB.Cur, pB.Prev = cur, prev
	resB, err := svc.Run(context.Background(), "transfers", pB)
	require.NoError(t, err)
	rowsB := resB.Rows.([]report.TransferRow)
	require.Len(t, rowsB, 3, "T1+T2 (to B) and Tout (from B)")
	var sawT1 bool
	for _, r := range rowsB {
		if r.AssetName == "Aset T1" {
			sawT1 = true
			assert.Equal(t, aName, r.FromOffice, "destination office sees an inbound transfer from A")
			assert.Equal(t, bName, r.ToOffice)
		}
	}
	assert.True(t, sawT1, "transfer visible when only the destination office is in scope")
	assert.Equal(t, "2", kpiMap(resB)["in_transit"], "T2 + Tout are in_transit under B scope")
}

// ─── disposals + GL recap ─────────────────────────────────────────────────────

// TestReportDisposals drives one gain + one loss disposal over a custom window:
// rows, KPIs, per-method chart, money totals, and out-of-window/out-of-scope
// exclusion.
func TestReportDisposals(t *testing.T) {
	svc, pool := newSvc(t)
	e := baseEnv(t, pool)

	gainA := intangibleAsset(t, pool, e, e.cat1, e.officeA, "DG", "Aset Untung")
	lossA := intangibleAsset(t, pool, e, e.cat1, e.officeA, "DL", "Aset Rugi")
	outWin := intangibleAsset(t, pool, e, e.cat1, e.officeA, "DW", "Aset luar window")
	scopeB := intangibleAsset(t, pool, e, e.cat1, e.officeB, "DB", "Aset kantor B")

	// Gain: proceeds 120M, book 100M → +20M (sale, 03-20).
	insertDisposal(t, pool, gainA, e.user, "sale", "2025-03-20", "120000000.00", "100000000.00", "20000000.00")
	// Loss: proceeds 30M, book 50M → −20M (write_off, 03-10).
	insertDisposal(t, pool, lossA, e.user, "write_off", "2025-03-10", "30000000.00", "50000000.00", "-20000000.00")
	// Out of window (04-05) — excluded on date.
	insertDisposal(t, pool, outWin, e.user, "sale", "2025-04-05", "1000.00", "500.00", "500.00")
	// Office B (03-15) — out of an A-only scope.
	insertDisposal(t, pool, scopeB, e.user, "donation", "2025-03-15", "0.00", "700.00", "-700.00")

	cur, prev := customPeriod(t, "2025-03-01", "2025-03-31")
	p := aScope(e)
	p.Cur, p.Prev = cur, prev

	res, err := svc.Run(context.Background(), "disposals", p)
	require.NoError(t, err)

	rows, ok := res.Rows.([]report.DisposalRow)
	require.True(t, ok)
	require.Len(t, rows, 2, "gain+loss only (out-of-window + office-B excluded)")

	// Ordered by disposal_date DESC → sale (03-20) then write_off (03-10).
	assert.Equal(t, "Aset Untung", rows[0].AssetName)
	assert.Equal(t, "sale", rows[0].Method)
	assert.Equal(t, "120000000.00", rows[0].Proceeds)
	assert.Equal(t, "100000000.00", rows[0].BookValue)
	assert.Equal(t, "20000000.00", rows[0].GainLoss)
	assert.Equal(t, "2025-03-20", rows[0].Date)
	assert.Equal(t, "Aset Rugi", rows[1].AssetName)
	assert.Equal(t, "-20000000.00", rows[1].GainLoss)

	k := kpiMap(res)
	assert.Equal(t, "2", k["total_disposals"])
	assert.Equal(t, "150000000.00", k["total_proceeds"], "120M+30M")
	assert.Equal(t, "0.00", k["total_gain_loss"], "+20M −20M")

	// Money totals over the returned rows.
	assert.Equal(t, "150000000.00", res.Totals["book_value"], "100M+50M")
	assert.Equal(t, "150000000.00", res.Totals["proceeds"])
	assert.Equal(t, "0.00", res.Totals["gain_loss"])

	// Chart: net gain/loss per method, raw enum labels, ordered by enum.
	require.Len(t, res.Chart, 2)
	assert.Equal(t, "sale", res.Chart[0].Label)
	assert.Equal(t, "20000000.00", res.Chart[0].Value)
	assert.Equal(t, "write_off", res.Chart[1].Label)
	assert.Equal(t, "-20000000.00", res.Chart[1].Value)

	assert.EqualValues(t, 2, res.RowCount)
}

// TestDisposalGlRecap drives the journal recap: the four lines balance
// (total_debit == total_credit, Balanced true), zero rows are omitted, and the
// account codes come from the report.gl.* settings (empty when unset).
func TestDisposalGlRecap(t *testing.T) {
	svc, pool := newSvc(t)
	e := baseEnv(t, pool)

	gainA := intangibleAsset(t, pool, e, e.cat1, e.officeA, "GG", "Aset Untung")
	lossA := intangibleAsset(t, pool, e, e.cat1, e.officeA, "GL", "Aset Rugi")
	insertDisposal(t, pool, gainA, e.user, "sale", "2025-03-20", "120000000.00", "100000000.00", "20000000.00")
	insertDisposal(t, pool, lossA, e.user, "write_off", "2025-03-10", "30000000.00", "50000000.00", "-20000000.00")

	// Seed a cash-account code; the other three stay unset (empty string).
	_, err := pool.Exec(context.Background(),
		`INSERT INTO identity.app_settings (key, value, value_type)
		 VALUES ('report.gl.cash_account', '1101', 'string')`)
	require.NoError(t, err)

	cur, prev := customPeriod(t, "2025-03-01", "2025-03-31")
	p := aScope(e)
	p.Cur, p.Prev = cur, prev

	recap, err := svc.DisposalGlRecap(context.Background(), p)
	require.NoError(t, err)

	// Four lines: Dr Kas 150M, Dr Rugi 20M, Cr Nilai Buku 150M, Cr Laba 20M.
	require.Len(t, recap.Rows, 4)
	byName := map[string]report.GlRow{}
	for _, r := range recap.Rows {
		byName[r.AccountName] = r
	}
	assert.Equal(t, "150000000.00", byName["Kas/Bank"].Debit)
	assert.Equal(t, "1101", byName["Kas/Bank"].AccountCode, "code from app_settings")
	assert.Equal(t, "0.00", byName["Kas/Bank"].Credit)
	assert.Equal(t, "20000000.00", byName["Rugi Pelepasan Aset"].Debit)
	assert.Equal(t, "150000000.00", byName["Nilai Buku Aset Dilepas"].Credit)
	assert.Empty(t, byName["Nilai Buku Aset Dilepas"].AccountCode, "unset setting -> empty code")
	assert.Equal(t, "20000000.00", byName["Laba Pelepasan Aset"].Credit)

	assert.Equal(t, "170000000.00", recap.TotalDebit, "150M proceeds + 20M loss")
	assert.Equal(t, "170000000.00", recap.TotalCredit, "150M book + 20M gain")
	assert.True(t, recap.Balanced)
}

// TestDisposalGlRecapGainOnly: a single gain disposal yields only the cash-debit
// and the two book/gain-credit lines (no loss line), still balanced.
func TestDisposalGlRecapGainOnly(t *testing.T) {
	svc, pool := newSvc(t)
	e := baseEnv(t, pool)

	gainA := intangibleAsset(t, pool, e, e.cat1, e.officeA, "G1", "Aset Untung")
	insertDisposal(t, pool, gainA, e.user, "sale", "2025-03-20", "120000000.00", "100000000.00", "20000000.00")

	cur, prev := customPeriod(t, "2025-03-01", "2025-03-31")
	p := aScope(e)
	p.Cur, p.Prev = cur, prev

	recap, err := svc.DisposalGlRecap(context.Background(), p)
	require.NoError(t, err)

	// No loss row (zero amount omitted): Dr Kas, Cr Nilai Buku, Cr Laba.
	require.Len(t, recap.Rows, 3)
	for _, r := range recap.Rows {
		assert.NotEqual(t, "Rugi Pelepasan Aset", r.AccountName, "zero loss row omitted")
	}
	assert.Equal(t, "120000000.00", recap.TotalDebit)
	assert.Equal(t, "120000000.00", recap.TotalCredit, "100M book + 20M gain")
	assert.True(t, recap.Balanced)
}

// TestDisposalGlRecapEmpty: no disposals in period -> no rows, zero totals,
// balanced by construction.
func TestDisposalGlRecapEmpty(t *testing.T) {
	svc, pool := newSvc(t)
	e := baseEnv(t, pool)

	cur, prev := customPeriod(t, "2025-03-01", "2025-03-31")
	p := aScope(e)
	p.Cur, p.Prev = cur, prev

	recap, err := svc.DisposalGlRecap(context.Background(), p)
	require.NoError(t, err)
	assert.Empty(t, recap.Rows)
	assert.Equal(t, "0.00", recap.TotalDebit)
	assert.Equal(t, "0.00", recap.TotalCredit)
	assert.True(t, recap.Balanced)
}

// ─── opname ───────────────────────────────────────────────────────────────────

// TestReportOpname drives closed-session accounting: variance counting, the
// closed-only filter (open sessions excluded), office scope (out-of-scope
// sessions invisible), and the summed KPIs.
func TestReportOpname(t *testing.T) {
	svc, pool := newSvc(t)
	e := baseEnv(t, pool)

	// S1 — closed session in A with 3 items → variance 2 (not_found + damaged).
	s1 := insertOpnameSession(t, pool, e.officeA, e.user, "Opname Q1", "2025-03-05", "closed")
	i1 := intangibleAsset(t, pool, e, e.cat1, e.officeA, "O1", "Aset O1")
	i2 := intangibleAsset(t, pool, e, e.cat1, e.officeA, "O2", "Aset O2")
	i3 := intangibleAsset(t, pool, e, e.cat1, e.officeA, "O3", "Aset O3")
	insertOpnameItem(t, pool, s1, i1, "found")
	insertOpnameItem(t, pool, s1, i2, "not_found")
	insertOpnameItem(t, pool, s1, i3, "damaged")

	// S2 — OPEN session in A → excluded (closed-only).
	s2 := insertOpnameSession(t, pool, e.officeA, e.user, "Opname belum tutup", "2025-03-06", "open")
	i4 := intangibleAsset(t, pool, e, e.cat1, e.officeA, "O4", "Aset O4")
	insertOpnameItem(t, pool, s2, i4, "misplaced")

	// S3 — closed session in office B → out of an A-only scope.
	s3 := insertOpnameSession(t, pool, e.officeB, e.user, "Opname B", "2025-03-07", "closed")
	i5 := intangibleAsset(t, pool, e, e.cat1, e.officeB, "O5", "Aset O5")
	insertOpnameItem(t, pool, s3, i5, "not_found")

	// S4 — closed session in A but OUT OF WINDOW (Feb) → excluded on period.
	insertOpnameSession(t, pool, e.officeA, e.user, "Opname lama", "2025-02-15", "closed")

	cur, prev := customPeriod(t, "2025-03-01", "2025-03-31")
	p := aScope(e)
	p.Cur, p.Prev = cur, prev

	res, err := svc.Run(context.Background(), "opname", p)
	require.NoError(t, err)

	rows, ok := res.Rows.([]report.OpnameRow)
	require.True(t, ok)
	require.Len(t, rows, 1, "only S1 (open/out-of-scope/out-of-window excluded)")

	got := rows[0]
	assert.Equal(t, s1.String(), got.SessionID)
	assert.Equal(t, "Opname Q1", got.Name)
	assert.Equal(t, officeName(t, pool, e.officeA), got.OfficeName)
	assert.Equal(t, "2025-03-05", got.Period)
	assert.Equal(t, "closed", got.Status)
	assert.EqualValues(t, 3, got.TotalItems)
	assert.EqualValues(t, 2, got.Variance, "not_found + damaged (found excluded)")

	k := kpiMap(res)
	assert.Equal(t, "1", k["sessions"])
	assert.Equal(t, "3", k["total_items"])
	assert.Equal(t, "2", k["total_variance"])

	assert.Empty(t, res.Totals, "opname carries no money tfoot")
	assert.EqualValues(t, 1, res.RowCount)

	// Chart: one bar (session), value = variance.
	require.Len(t, res.Chart, 1)
	assert.Equal(t, "Opname Q1", res.Chart[0].Label)
	assert.Equal(t, "2", res.Chart[0].Value)

	_ = s2
	_ = s3
}

// ─── helper ───────────────────────────────────────────────────────────────────

func officeName(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var name string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT name FROM masterdata.offices WHERE id = $1`, id).Scan(&name))
	return name
}
