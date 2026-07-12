package office

import (
	"testing"

	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/importer"
)

// --- test fixtures -------------------------------------------------------

// fixedIDs holds the resolved UUIDs used across the validation tests so
// assertions can compare stamped Data against known values.
type fixedIDs struct {
	officeType  uuid.UUID // "Kantor Cabang" — resolved office_type
	parent      uuid.UUID // "KANWIL01" — in scope
	parentOther uuid.UUID // "KANWIL02" — out of scope in the scope test
}

// mkLookups builds a hand-crafted officeLookups (no DB) plus the fixed IDs.
// existingCodes contains one code so the "already exists in DB" branch of the
// kode rule can be exercised without a database.
func mkLookups() (officeLookups, fixedIDs) {
	ids := fixedIDs{
		officeType:  uuid.New(),
		parent:      uuid.New(),
		parentOther: uuid.New(),
	}
	lk := officeLookups{
		officeTypes: map[string]uuid.UUID{
			"kantor cabang": ids.officeType,
		},
		parentOffices: map[string]uuid.UUID{
			"kanwil01": ids.parent,
			"kanwil02": ids.parentOther,
		},
		existingCodes: map[string]bool{
			"cb-0001": true,
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

// validCells returns a fully-valid cell set placed under the "KANWIL01"
// parent office; the caller can mutate individual keys to construct a
// specific failure.
func validCells() map[string]string {
	return map[string]string{
		colCode:  "CB-0100",
		colName:  "Cabang Jakarta Selatan",
		colTipe:  "Kantor Cabang",
		colInduk: "KANWIL01",
		colAktif: "true",
	}
}

// hasErr reports whether res carries a CellError with the given error_key.
func hasErr(res importer.RowResult, key string) bool {
	for _, e := range res.Errors {
		if e.ErrorKey == key {
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

// --- per-field error-key tests -------------------------------------------

func TestOfficeImporterValidateErrorKeys(t *testing.T) {
	lk, _ := mkLookups()

	cases := []struct {
		name   string
		mutate func(map[string]string)
		want   string
	}{
		{"required kode", func(c map[string]string) { c[colCode] = "" }, "required"},
		{"required nama", func(c map[string]string) { c[colName] = "" }, "required"},
		{"required tipe", func(c map[string]string) { c[colTipe] = "" }, "required"},
		{"tipe miss", func(c map[string]string) { c[colTipe] = "Tak Ada" }, "tipe"},
		{"induk miss", func(c map[string]string) { c[colInduk] = "Tak Ada" }, "induk"},
		{"aktif invalid", func(c map[string]string) { c[colAktif] = "mungkin" }, "aktif"},
		{"kode exists in db", func(c map[string]string) { c[colCode] = "CB-0001" }, "dupKode"},
		{"kode exists in db case-insensitive", func(c map[string]string) { c[colCode] = "cb-0001" }, "dupKode"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cells := validCells()
			tc.mutate(cells)
			results := validateOfficeRows([]importer.RawRow{row(1, cells)}, lk, allScope())
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			res := results[0]
			if res.Valid {
				t.Fatalf("expected row invalid, got valid")
			}
			if !hasErr(res, tc.want) {
				t.Fatalf("expected error_key %q, got %v", tc.want, errKeys(res))
			}
		})
	}
}

// --- tipe / induk case-insensitivity --------------------------------------

func TestOfficeImporterValidateCaseInsensitiveLookups(t *testing.T) {
	lk, ids := mkLookups()

	cells := validCells()
	cells[colTipe] = "KANTOR CABANG"
	cells[colInduk] = "kanwil01"

	results := validateOfficeRows([]importer.RawRow{row(1, cells)}, lk, allScope())
	if !results[0].Valid {
		t.Fatalf("expected valid, got %v", errKeys(results[0]))
	}
	if results[0].Data["_office_type_id"] != ids.officeType.String() {
		t.Fatalf("_office_type_id = %q, want %q", results[0].Data["_office_type_id"], ids.officeType.String())
	}
	if results[0].Data["_parent_id"] != ids.parent.String() {
		t.Fatalf("_parent_id = %q, want %q", results[0].Data["_parent_id"], ids.parent.String())
	}
}

// --- aktif boolean literals -------------------------------------------------

func TestOfficeImporterValidateAktifLiterals(t *testing.T) {
	lk, _ := mkLookups()

	for _, label := range []string{"true", "false", "ya", "tidak", "1", "0", "TRUE", "Ya"} {
		cells := validCells()
		cells[colAktif] = label
		results := validateOfficeRows([]importer.RawRow{row(1, cells)}, lk, allScope())
		if !results[0].Valid {
			t.Fatalf("aktif %q: expected valid, got %v", label, errKeys(results[0]))
		}
	}
}

func TestOfficeImporterValidateAktifEmptyDefaultsTrue(t *testing.T) {
	lk, _ := mkLookups()
	cells := validCells()
	cells[colAktif] = ""

	results := validateOfficeRows([]importer.RawRow{row(1, cells)}, lk, allScope())
	if !results[0].Valid {
		t.Fatalf("expected valid with empty aktif, got %v", errKeys(results[0]))
	}
}

// --- scope: resolved parent must be within the caller's scope -------------

func TestOfficeImporterValidateScopeParentOutOfScope(t *testing.T) {
	lk, ids := mkLookups()

	// Caller scope permits parentOther only; a row resolving to "KANWIL01"
	// (ids.parent) is out of scope.
	scope := importer.Scope{AllScope: false, OfficeIDs: []uuid.UUID{ids.parentOther}, UserID: uuid.New()}

	cells := validCells() // resolves induk to "KANWIL01" == ids.parent (not in scope)
	results := validateOfficeRows([]importer.RawRow{row(1, cells)}, lk, scope)
	if results[0].Valid {
		t.Fatalf("expected out-of-scope row invalid")
	}
	if !hasErr(results[0], "scope") {
		t.Fatalf("expected scope error, got %v", errKeys(results[0]))
	}
}

// --- scope: a scoped caller cannot create a root office (empty induk) -----

func TestOfficeImporterValidateScopeEmptyIndukForScopedCaller(t *testing.T) {
	lk, ids := mkLookups()

	scope := importer.Scope{AllScope: false, OfficeIDs: []uuid.UUID{ids.parent}, UserID: uuid.New()}

	cells := validCells()
	cells[colInduk] = ""

	results := validateOfficeRows([]importer.RawRow{row(1, cells)}, lk, scope)
	if results[0].Valid {
		t.Fatalf("expected invalid: scoped caller cannot create a root office")
	}
	if !hasErr(results[0], "scope") {
		t.Fatalf("expected scope error, got %v", errKeys(results[0]))
	}
}

// --- scope: an all-scope (global) caller MAY create a root office ---------

func TestOfficeImporterValidateAllScopeEmptyIndukAllowed(t *testing.T) {
	lk, _ := mkLookups()

	cells := validCells()
	cells[colInduk] = ""

	results := validateOfficeRows([]importer.RawRow{row(1, cells)}, lk, allScope())
	if !results[0].Valid {
		t.Fatalf("expected valid: global caller may create a root office, got %v", errKeys(results[0]))
	}
	if _, ok := results[0].Data["_parent_id"]; ok {
		t.Fatalf("expected no _parent_id stamp for root office, got %q", results[0].Data["_parent_id"])
	}
}

// --- in-file duplicate kode --------------------------------------------------

func TestOfficeImporterValidateInFileDuplicateKode(t *testing.T) {
	lk, _ := mkLookups()

	r1 := validCells()
	r1[colCode] = "CB-0200"
	r2 := validCells()
	r2[colCode] = "cb-0200" // same code, different case

	results := validateOfficeRows([]importer.RawRow{row(1, r1), row(2, r2)}, lk, allScope())
	if !results[0].Valid {
		t.Fatalf("row 1 expected valid, got %v", errKeys(results[0]))
	}
	if results[1].Valid {
		t.Fatalf("row 2 expected invalid (dupKode)")
	}
	if !hasErr(results[1], "dupKode") {
		t.Fatalf("row 2 expected dupKode, got %v", errKeys(results[1]))
	}
}

// --- fully-valid batch ---------------------------------------------------

func TestOfficeImporterValidateAllValid(t *testing.T) {
	lk, ids := mkLookups()

	r1 := validCells()
	r2 := validCells()
	r2[colCode] = "CB-0101"

	results := validateOfficeRows([]importer.RawRow{row(1, r1), row(2, r2)}, lk, allScope())
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, res := range results {
		if !res.Valid {
			t.Fatalf("row %d expected valid, got %v", i+1, errKeys(res))
		}
		if res.NormalizedRef != "" {
			t.Fatalf("row %d NormalizedRef = %q, want empty (office imports never need approval routing)", i+1, res.NormalizedRef)
		}
		if res.Data["_office_type_id"] != ids.officeType.String() {
			t.Fatalf("row %d _office_type_id = %q, want %q", i+1, res.Data["_office_type_id"], ids.officeType.String())
		}
		if res.Data["_parent_id"] != ids.parent.String() {
			t.Fatalf("row %d _parent_id = %q, want %q", i+1, res.Data["_parent_id"], ids.parent.String())
		}
	}
}

// --- invalid rows keep clean data (no leaked internal stamps) ------------

func TestOfficeImporterValidateInvalidRowDropsStamps(t *testing.T) {
	lk, _ := mkLookups()
	cells := validCells()
	cells[colAktif] = "bogus" // invalid, but tipe/induk still resolve

	results := validateOfficeRows([]importer.RawRow{row(1, cells)}, lk, allScope())
	if results[0].Valid {
		t.Fatalf("expected invalid row")
	}
	if _, ok := results[0].Data["_office_type_id"]; ok {
		t.Fatalf("expected _office_type_id stamp dropped from invalid row")
	}
	if _, ok := results[0].Data["_parent_id"]; ok {
		t.Fatalf("expected _parent_id stamp dropped from invalid row")
	}
}

// --- columns / needs-approval contract -----------------------------------

func TestOfficeImporterColumns(t *testing.T) {
	imp := officeImporter{}
	if imp.Target() != "office" {
		t.Fatalf("Target() = %q, want office", imp.Target())
	}
	if imp.NeedsApproval() {
		t.Fatalf("NeedsApproval() = true, want false")
	}

	cols := imp.Columns()
	want := []struct {
		name     string
		required bool
		kind     string
	}{
		{colCode, true, "text"},
		{colName, true, "text"},
		{colTipe, true, "lookup"},
		{colInduk, false, "lookup"},
		{colAktif, false, "text"},
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
