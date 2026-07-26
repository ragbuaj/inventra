package asset

import (
	"testing"

	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/importer"
)

// --- test fixtures -------------------------------------------------------

// fixedIDs holds the resolved UUIDs used across the validation tests so
// assertions can compare NormalizedRef / stamped data against known values.
type fixedIDs struct {
	category uuid.UUID
	office   uuid.UUID // "Kantor Pusat" / "KP" — in scope
	office2  uuid.UUID // "Cabang Jakarta" / "CBJ" — in scope
	brand    uuid.UUID // "Daikin"
	employee uuid.UUID // NIP "12345"
	floor1   uuid.UUID // "Lantai 1" — lives in office
	floor2   uuid.UUID // "Lantai 2" — lives in office2
	room1    uuid.UUID // "Ruang Server" — lives in floor1 (office)
}

// mkLookups builds a hand-crafted assetLookups (no DB) plus the fixed IDs.
// It mirrors buildAssetLookups' output: name AND code keys for offices and
// categories, floors keyed by lower(name) -> []floorRef{id, officeID}, and
// rooms keyed by lower(name) -> []roomRef{id, floorID}.
func mkLookups() (assetLookups, fixedIDs) {
	ids := fixedIDs{
		category: uuid.New(),
		office:   uuid.New(),
		office2:  uuid.New(),
		brand:    uuid.New(),
		employee: uuid.New(),
		floor1:   uuid.New(),
		floor2:   uuid.New(),
		room1:    uuid.New(),
	}
	lk := assetLookups{
		categories: map[string]uuid.UUID{
			"elektronik": ids.category,
			"elk":        ids.category,
		},
		offices: map[string]uuid.UUID{
			"kantor pusat":   ids.office,
			"kp":             ids.office,
			"cabang jakarta": ids.office2,
			"cbj":            ids.office2,
		},
		brands: map[string]uuid.UUID{
			"daikin": ids.brand,
		},
		employees: map[string]uuid.UUID{
			"12345": ids.employee,
		},
		floors: map[string][]floorRef{
			"lantai 1": {{id: ids.floor1, officeID: ids.office}},
			"lantai 2": {{id: ids.floor2, officeID: ids.office2}},
		},
		rooms: map[string][]roomRef{
			"ruang server": {{id: ids.room1, floorID: ids.floor1}},
		},
	}
	return lk, ids
}

// allScope returns a scope that permits every office (global caller).
func allScope() importer.Scope {
	return importer.Scope{AllScope: true, UserID: uuid.New()}
}

// row builds a RawRow from column/value pairs.
func row(no int, cells map[string]string) importer.RawRow {
	return importer.RawRow{RowNo: no, Cells: cells}
}

// validCells returns a fully-valid cell set for the "Kantor Pusat" office with
// a "Floor - Room" First Location; callers mutate individual keys to construct
// a specific failure. Uses the new legacy-English column contract.
func validCells() map[string]string {
	return map[string]string{
		colOffice:     "Kantor Pusat",
		colCategory:   "Elektronik",
		colName:       "Laptop Dell",
		colLocation:   "Lantai 1 - Ruang Server",
		colLastLoc:    "Lantai 2 - Ruang Arsip", // ignored (layout parity only)
		colDate:       "2026-01-15",
		colPrice:      "15000000.00",
		colUser:       "12345 - Budi Santoso",
		colCondition:  "Baik",
		colLastChange: "2026-02-01", // ignored (layout parity only)
		colAssetCode:  "LEG-CODE-001",
		colBarcode:    "8991234567890",
		colBrand:      "Daikin",
		colCapacity:   "5PK",
		colSpk:        "SPK/012/2023",
	}
}

// hasErrOn reports whether res carries a CellError on col with error_key key.
func hasErrOn(res importer.RowResult, col, key string) bool {
	for _, e := range res.Errors {
		if e.Column == col && e.ErrorKey == key {
			return true
		}
	}
	return false
}

// errKeys lists the error_keys present on a result (for failure messages).
func errKeys(res importer.RowResult) []string {
	out := make([]string, 0, len(res.Errors))
	for _, e := range res.Errors {
		out = append(out, e.ErrorKey)
	}
	return out
}

// validateOne is a one-row convenience over validateAssetRows.
func validateOne(cells map[string]string, lk assetLookups, scope importer.Scope) importer.RowResult {
	return validateAssetRows([]importer.RawRow{row(1, cells)}, lk, scope)[0]
}

// --- happy path ----------------------------------------------------------

// TestAssetImporterValidateHappyPath_FloorRoom asserts a fully-valid row with a
// "Floor - Room" First Location resolves every lookup, stamps every internal id,
// and carries the office in NormalizedRef.
func TestAssetImporterValidateHappyPath_FloorRoom(t *testing.T) {
	lk, ids := mkLookups()

	res := validateOne(validCells(), lk, allScope())
	if !res.Valid {
		t.Fatalf("expected valid, got errors %v", errKeys(res))
	}
	if res.NormalizedRef != ids.office.String() {
		t.Fatalf("NormalizedRef = %q, want %q", res.NormalizedRef, ids.office.String())
	}

	want := map[string]string{
		"_office_id":   ids.office.String(),
		"_category_id": ids.category.String(),
		"_floor_id":    ids.floor1.String(),
		"_room_id":     ids.room1.String(),
		"_brand_id":    ids.brand.String(),
		"_pic_id":      ids.employee.String(),
		"_status":      "available",
		hargaKey:       "15000000.00",
	}
	for k, v := range want {
		if res.Data[k] != v {
			t.Fatalf("Data[%q] = %q, want %q", k, res.Data[k], v)
		}
	}
	// Free-text columns carried verbatim.
	if res.Data[colCapacity] != "5PK" || res.Data[colSpk] != "SPK/012/2023" {
		t.Fatalf("capacity/spk not carried: %q / %q", res.Data[colCapacity], res.Data[colSpk])
	}
	if res.Data[colAssetCode] != "LEG-CODE-001" || res.Data[colBarcode] != "8991234567890" {
		t.Fatalf("legacy code/barcode not carried: %q / %q", res.Data[colAssetCode], res.Data[colBarcode])
	}
}

// TestAssetImporterValidateHappyPath_FloorOnly asserts a First Location naming
// only a floor ("Lantai 1") is valid: the floor resolves and _room_id is empty.
func TestAssetImporterValidateHappyPath_FloorOnly(t *testing.T) {
	lk, ids := mkLookups()

	cells := validCells()
	cells[colLocation] = "Lantai 1" // floor only, no room
	res := validateOne(cells, lk, allScope())
	if !res.Valid {
		t.Fatalf("floor-only First Location should be valid, got %v", errKeys(res))
	}
	if res.Data["_floor_id"] != ids.floor1.String() {
		t.Fatalf("_floor_id = %q, want %q", res.Data["_floor_id"], ids.floor1.String())
	}
	if res.Data["_room_id"] != "" {
		t.Fatalf("_room_id = %q, want empty (floor-only)", res.Data["_room_id"])
	}
}

// TestAssetImporterValidateMinimalOptionals asserts a row omitting every
// optional column (price, user, brand, capacity, spk, legacy codes) is valid,
// with the corresponding stamps empty and harga blank.
func TestAssetImporterValidateMinimalOptionals(t *testing.T) {
	lk, ids := mkLookups()

	cells := map[string]string{
		colOffice:    "KP",  // by code
		colCategory:  "ELK", // by code
		colName:      "Meja Kayu",
		colLocation:  "Lantai 1",
		colCondition: "Baik",
	}
	res := validateOne(cells, lk, allScope())
	if !res.Valid {
		t.Fatalf("minimal row should be valid, got %v", errKeys(res))
	}
	if res.Data["_office_id"] != ids.office.String() {
		t.Fatalf("_office_id = %q, want %q (resolved by code)", res.Data["_office_id"], ids.office.String())
	}
	if res.Data["_category_id"] != ids.category.String() {
		t.Fatalf("_category_id = %q, want %q (resolved by code)", res.Data["_category_id"], ids.category.String())
	}
	for _, k := range []string{"_room_id", "_brand_id", "_pic_id"} {
		if res.Data[k] != "" {
			t.Fatalf("Data[%q] = %q, want empty", k, res.Data[k])
		}
	}
	if res.Data[hargaKey] != "" {
		t.Fatalf("empty Purchase Price should leave harga empty, got %q", res.Data[hargaKey])
	}
	if res.Data["_status"] != "available" {
		t.Fatalf("_status = %q, want available", res.Data["_status"])
	}
}

// --- required columns ----------------------------------------------------

// TestAssetImporterValidateRequired covers each required column: an empty value
// fails with "required" on that column.
func TestAssetImporterValidateRequired(t *testing.T) {
	lk, _ := mkLookups()

	for _, col := range []string{colOffice, colCategory, colName, colLocation, colCondition} {
		t.Run("required "+col, func(t *testing.T) {
			cells := validCells()
			cells[col] = ""
			res := validateOne(cells, lk, allScope())
			if res.Valid {
				t.Fatalf("expected invalid when %q empty", col)
			}
			if !hasErrOn(res, col, "required") {
				t.Fatalf("expected required on %q, got %v", col, res.Errors)
			}
		})
	}
}

// TestAssetImporterValidatePriceOptional asserts Purchase Price is optional:
// an empty price is valid and stamps harga="" (contributes 0 to approval).
func TestAssetImporterValidatePriceOptional(t *testing.T) {
	lk, _ := mkLookups()

	cells := validCells()
	cells[colPrice] = ""
	res := validateOne(cells, lk, allScope())
	if !res.Valid {
		t.Fatalf("empty Purchase Price should be valid, got %v", errKeys(res))
	}
	if res.Data[hargaKey] != "" {
		t.Fatalf("empty price should stamp harga=\"\", got %q", res.Data[hargaKey])
	}
}

// --- per-field lookup / format errors -----------------------------------

// TestAssetImporterValidateErrorKeys drives one failing column at a time and
// asserts the exact error_key the contract prescribes.
func TestAssetImporterValidateErrorKeys(t *testing.T) {
	lk, _ := mkLookups()

	cases := []struct {
		name   string
		mutate func(map[string]string)
		col    string
		key    string
	}{
		{"category miss", func(c map[string]string) { c[colCategory] = "Tak Ada" }, colCategory, "kat"},
		{"office miss", func(c map[string]string) { c[colOffice] = "Tak Ada" }, colOffice, "kantor"},
		{"floor miss", func(c map[string]string) { c[colLocation] = "Lantai 99 - Ruang Server" }, colLocation, "lokasi"},
		{"room miss", func(c map[string]string) { c[colLocation] = "Lantai 1 - Ruang Hantu" }, colLocation, "lokasi"},
		{"floor wrong office", func(c map[string]string) {
			// office2 has floor2 ("Lantai 2"); "Lantai 1" belongs to office, so
			// under office2 it can never resolve.
			c[colOffice] = "Cabang Jakarta"
			c[colLocation] = "Lantai 1"
		}, colLocation, "lokasi"},
		{"room wrong floor", func(c map[string]string) {
			// "Ruang Server" lives in floor1; asking for it under floor2 fails.
			c[colOffice] = "Cabang Jakarta"
			c[colLocation] = "Lantai 2 - Ruang Server"
		}, colLocation, "lokasi"},
		{"bad date format", func(c map[string]string) { c[colDate] = "15-01-2026" }, colDate, "tgl"},
		{"bad date value", func(c map[string]string) { c[colDate] = "2026-13-40" }, colDate, "tgl"},
		{"negative price", func(c map[string]string) { c[colPrice] = "-100" }, colPrice, "harga"},
		{"non-decimal price", func(c map[string]string) { c[colPrice] = "abc" }, colPrice, "harga"},
		{"fraction price", func(c map[string]string) { c[colPrice] = "1/2" }, colPrice, "harga"},
		{"signed price", func(c map[string]string) { c[colPrice] = "+100" }, colPrice, "harga"},
		{"scientific price", func(c map[string]string) { c[colPrice] = "1e5" }, colPrice, "harga"},
		{"pic miss", func(c map[string]string) { c[colUser] = "99999 - Orang Asing" }, colUser, "pemegang"},
		{"brand miss", func(c map[string]string) { c[colBrand] = "MerkTakAda" }, colBrand, "merk"},
		{"condition unknown", func(c map[string]string) { c[colCondition] = "Sedang Diperbaiki" }, colCondition, "kondisi"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cells := validCells()
			tc.mutate(cells)
			res := validateOne(cells, lk, allScope())
			if res.Valid {
				t.Fatalf("expected invalid, got valid")
			}
			if !hasErrOn(res, tc.col, tc.key) {
				t.Fatalf("expected %q on %q, got %v", tc.key, tc.col, res.Errors)
			}
		})
	}
}

// --- condition mapping ---------------------------------------------------

// TestAssetImporterValidateCondition covers Condition -> status, including the
// documented synonyms and the Rusak -> under_maintenance mapping.
func TestAssetImporterValidateCondition(t *testing.T) {
	lk, _ := mkLookups()

	cases := []struct {
		in   string
		want string
	}{
		{"Baik", "available"},
		{"baik", "available"},
		{"bagus", "available"},
		{"bersih", "available"},
		{"available", "available"},
		{"Rusak", "under_maintenance"},
		{"rusak", "under_maintenance"},
		{"under_maintenance", "under_maintenance"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			cells := validCells()
			cells[colCondition] = tc.in
			res := validateOne(cells, lk, allScope())
			if !res.Valid {
				t.Fatalf("condition %q should be valid, got %v", tc.in, errKeys(res))
			}
			if res.Data["_status"] != tc.want {
				t.Fatalf("condition %q -> _status %q, want %q", tc.in, res.Data["_status"], tc.want)
			}
		})
	}
}

// --- PIC (User) resolution ----------------------------------------------

// TestAssetImporterValidatePIC covers the User column: bare NIP and "NIP - Nama"
// both resolve; an empty User is valid with an empty _pic_id.
func TestAssetImporterValidatePIC(t *testing.T) {
	lk, ids := mkLookups()

	bare := validCells()
	bare[colUser] = "12345"
	if r := validateOne(bare, lk, allScope()); !r.Valid || r.Data["_pic_id"] != ids.employee.String() {
		t.Fatalf("bare NIP should resolve, valid=%v pic=%q", r.Valid, r.Data["_pic_id"])
	}

	display := validCells()
	display[colUser] = "12345 - Budi Santoso"
	if r := validateOne(display, lk, allScope()); !r.Valid || r.Data["_pic_id"] != ids.employee.String() {
		t.Fatalf("NIP - Nama should resolve, valid=%v pic=%q", r.Valid, r.Data["_pic_id"])
	}

	empty := validCells()
	empty[colUser] = ""
	if r := validateOne(empty, lk, allScope()); !r.Valid || r.Data["_pic_id"] != "" {
		t.Fatalf("empty User should be valid with empty pic, valid=%v pic=%q", r.Valid, r.Data["_pic_id"])
	}
}

// --- brand (Merk) resolution --------------------------------------------

// TestAssetImporterValidateBrand covers the optional Merk lookup: unknown ->
// "merk"; empty -> valid with empty _brand_id.
func TestAssetImporterValidateBrand(t *testing.T) {
	lk, ids := mkLookups()

	present := validCells()
	if r := validateOne(present, lk, allScope()); !r.Valid || r.Data["_brand_id"] != ids.brand.String() {
		t.Fatalf("known merk should resolve, valid=%v brand=%q", r.Valid, r.Data["_brand_id"])
	}

	empty := validCells()
	empty[colBrand] = ""
	if r := validateOne(empty, lk, allScope()); !r.Valid || r.Data["_brand_id"] != "" {
		t.Fatalf("empty merk should be valid with empty brand, valid=%v brand=%q", r.Valid, r.Data["_brand_id"])
	}
}

// --- scope ---------------------------------------------------------------

// TestAssetImporterValidateScope asserts a resolved office outside the caller's
// data scope is rejected with "scope".
func TestAssetImporterValidateScope(t *testing.T) {
	lk, ids := mkLookups()

	// Caller may see office2 only; validCells resolves to office (out of scope).
	scope := importer.Scope{AllScope: false, OfficeIDs: []uuid.UUID{ids.office2}, UserID: uuid.New()}
	res := validateOne(validCells(), lk, scope)
	if res.Valid {
		t.Fatalf("out-of-scope office should be invalid")
	}
	if !hasErrOn(res, colOffice, "scope") {
		t.Fatalf("expected scope on %q, got %v", colOffice, res.Errors)
	}
}

// TestAssetImporterValidateScopeInReach asserts a row inside the caller's scope
// validates cleanly (guards against the scope check being always-on).
func TestAssetImporterValidateScopeInReach(t *testing.T) {
	lk, ids := mkLookups()

	scope := importer.Scope{AllScope: false, OfficeIDs: []uuid.UUID{ids.office}, UserID: uuid.New()}
	res := validateOne(validCells(), lk, scope)
	if !res.Valid {
		t.Fatalf("in-scope office should be valid, got %v", errKeys(res))
	}
}

// --- multiOffice ---------------------------------------------------------

// TestAssetImporterValidateMultiOffice asserts the batch-office-consistency
// rule: the first resolved office wins the batch; a later row resolving a
// different office is flagged "multiOffice" even though it is individually
// well-formed and in scope.
func TestAssetImporterValidateMultiOffice(t *testing.T) {
	lk, ids := mkLookups()

	r1 := validCells() // Kantor Pusat, floor+room
	r2 := validCells()
	r2[colOffice] = "Cabang Jakarta" // office2
	r2[colLocation] = "Lantai 2"     // floor in office2 so the row is otherwise valid

	results := validateAssetRows([]importer.RawRow{row(1, r1), row(2, r2)}, lk, allScope())
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].Valid {
		t.Fatalf("row 1 expected valid, got %v", errKeys(results[0]))
	}
	if results[0].NormalizedRef != ids.office.String() {
		t.Fatalf("row 1 NormalizedRef = %q, want %q", results[0].NormalizedRef, ids.office.String())
	}
	if results[1].Valid {
		t.Fatalf("row 2 expected invalid (multiOffice)")
	}
	if !hasErrOn(results[1], colOffice, "multiOffice") {
		t.Fatalf("row 2 expected multiOffice, got %v", results[1].Errors)
	}
}

// TestAssetImporterValidateSameOfficeBatch asserts two rows resolving the SAME
// office both stay valid — the batch rule must not flag consistent batches.
func TestAssetImporterValidateSameOfficeBatch(t *testing.T) {
	lk, ids := mkLookups()

	r1 := validCells()
	r2 := validCells()
	r2[colName] = "Laptop Kedua"
	r2[colLocation] = "Lantai 1" // same office, floor-only

	results := validateAssetRows([]importer.RawRow{row(1, r1), row(2, r2)}, lk, allScope())
	for i, res := range results {
		if !res.Valid {
			t.Fatalf("row %d expected valid, got %v", i+1, errKeys(res))
		}
		if res.NormalizedRef != ids.office.String() {
			t.Fatalf("row %d NormalizedRef = %q, want %q", i+1, res.NormalizedRef, ids.office.String())
		}
	}
}

// --- invalid rows drop internal stamps ----------------------------------

// TestAssetImporterValidateInvalidDropsStamps asserts an invalid row does NOT
// leak resolved internal ids into its Data (the "_"-prefixed keys are deleted),
// though the harga stamp is retained for reporting.
func TestAssetImporterValidateInvalidDropsStamps(t *testing.T) {
	lk, _ := mkLookups()

	cells := validCells()
	cells[colName] = "" // force invalid via a required miss
	res := validateOne(cells, lk, allScope())
	if res.Valid {
		t.Fatalf("expected invalid row")
	}
	for _, k := range []string{"_office_id", "_category_id", "_floor_id", "_room_id", "_brand_id", "_pic_id", "_status"} {
		if _, ok := res.Data[k]; ok {
			t.Fatalf("invalid row must not carry stamp %q, got %q", k, res.Data[k])
		}
	}
	// harga is still stamped (not among the deleted keys) for the error report.
	if res.Data[hargaKey] != "15000000.00" {
		t.Fatalf("harga should survive on an invalid row, got %q", res.Data[hargaKey])
	}
	// NormalizedRef is not set for an invalid row.
	if res.NormalizedRef != "" {
		t.Fatalf("invalid row NormalizedRef = %q, want empty", res.NormalizedRef)
	}
}

// TestAssetImporterValidateIgnoredColumns asserts the layout-parity columns
// (Last Location, Last Change) are accepted but never cause an error.
func TestAssetImporterValidateIgnoredColumns(t *testing.T) {
	lk, _ := mkLookups()

	cells := validCells()
	cells[colLastLoc] = "Sembarang Isi"
	cells[colLastChange] = "kapan saja"
	res := validateOne(cells, lk, allScope())
	if !res.Valid {
		t.Fatalf("ignored columns must not affect validity, got %v", errKeys(res))
	}
}

// --- columns / needs-approval contract ----------------------------------

func TestAssetImporterColumns(t *testing.T) {
	imp := assetImporter{}
	if imp.Target() != "asset" {
		t.Fatalf("Target() = %q, want asset", imp.Target())
	}
	if !imp.NeedsApproval() {
		t.Fatalf("NeedsApproval() = false, want true")
	}

	cols := imp.Columns()
	want := []struct {
		name     string
		required bool
		kind     string
	}{
		{colOffice, true, "lookup"},
		{colCategory, true, "lookup"},
		{colName, true, "text"},
		{colLocation, true, "lookup"},
		{colLastLoc, false, "text"},
		{colDate, false, "date"},
		{colPrice, false, "decimal"},
		{colUser, false, "lookup"},
		{colCondition, true, "text"},
		{colLastChange, false, "text"},
		{colAssetCode, false, "text"},
		{colBarcode, false, "text"},
		{colBrand, false, "lookup"},
		{colCapacity, false, "text"},
		{colSpk, false, "text"},
	}
	if len(cols) != len(want) {
		t.Fatalf("Columns() len = %d, want %d", len(cols), len(want))
	}
	for i, w := range want {
		if cols[i].Name != w.name || cols[i].Required != w.required || cols[i].Kind != w.kind {
			t.Fatalf("col %d = %+v, want %+v", i, cols[i], w)
		}
	}
}

// --- small pure helpers --------------------------------------------------

// TestSplitLocation covers the "Floor - Room" / floor-only split.
func TestSplitLocation(t *testing.T) {
	cases := []struct {
		in    string
		floor string
		room  string
	}{
		{"Lantai 1 - Ruang Server", "Lantai 1", "Ruang Server"},
		{"Lantai 1", "Lantai 1", ""},
		{"  Lantai 2  ", "Lantai 2", ""},
		{"Lantai 3 - Ruang A - Sudut", "Lantai 3", "Ruang A - Sudut"},
	}
	for _, tc := range cases {
		f, r := splitLocation(tc.in)
		if f != tc.floor || r != tc.room {
			t.Fatalf("splitLocation(%q) = (%q,%q), want (%q,%q)", tc.in, f, r, tc.floor, tc.room)
		}
	}
}

// TestPicNIP covers extracting the NIP from bare / display User cells.
func TestPicNIP(t *testing.T) {
	cases := map[string]string{
		"12345":                "12345",
		"12345 - Budi Santoso": "12345",
		"  12345  ":            "12345",
		"12345 - Budi - Extra": "12345",
	}
	for in, want := range cases {
		if got := picNIP(in); got != want {
			t.Fatalf("picNIP(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestCondStatus covers the Condition -> (status, ok) mapping directly.
func TestCondStatus(t *testing.T) {
	ok := map[string]string{
		"Baik": "available", "bagus": "available", "bersih": "available", "available": "available",
		"Rusak": "under_maintenance", "under_maintenance": "under_maintenance",
	}
	for in, want := range ok {
		if got, valid := condStatus(in); !valid || got != want {
			t.Fatalf("condStatus(%q) = (%q,%v), want (%q,true)", in, got, valid, want)
		}
	}
	for _, in := range []string{"", "hancur", "sedang diperbaiki", "broken"} {
		if got, valid := condStatus(in); valid {
			t.Fatalf("condStatus(%q) = (%q,true), want ok=false", in, got)
		}
	}
}
