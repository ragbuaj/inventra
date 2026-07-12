package reference

import (
	"testing"

	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/importer"
)

// --- shared test helpers (mirrors employee/office importer_test.go) --------

// allScope returns a scope that permits every office (global caller). The
// reference importers (provinces, cities) have no office-scope concept, but
// ValidateRows still takes an importer.Scope per the TargetImporter contract.
func allScope() importer.Scope {
	return importer.Scope{AllScope: true, UserID: uuid.New()}
}

// row builds a RawRow from column/value pairs.
func row(no int, cells map[string]string) importer.RawRow {
	return importer.RawRow{RowNo: no, Cells: cells}
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

// --- provinces ------------------------------------------------------------

// mkProvinceLookups builds a hand-crafted provinceLookups (no DB): one
// existing code and one existing name, so the "already exists in DB" branches
// of the kode/nama rules can be exercised without a database.
func mkProvinceLookups() provinceLookups {
	return provinceLookups{
		existingCodes: map[string]bool{"jb": true},
		existingNames: map[string]bool{"jawa barat": true},
	}
}

func validProvinceCells() map[string]string {
	return map[string]string{
		provColName: "Jawa Tengah",
		provColCode: "JT",
	}
}

func TestReferenceImporterProvinceRows_Required(t *testing.T) {
	lk := mkProvinceLookups()
	cells := validProvinceCells()
	cells[provColName] = ""

	results := validateProvinceRows([]importer.RawRow{row(1, cells)}, lk)
	if results[0].Valid {
		t.Fatalf("expected invalid row when nama is empty")
	}
	if !hasErr(results[0], "required") {
		t.Fatalf("expected required error, got %v", errKeys(results[0]))
	}
}

func TestReferenceImporterProvinceRows_DupKode(t *testing.T) {
	lk := mkProvinceLookups()

	cases := []struct {
		name string
		code string
	}{
		{"exists in db", "JB"},
		{"exists in db case-insensitive", "jb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cells := validProvinceCells()
			cells[provColCode] = tc.code
			cells[provColName] = "Nama Unik " + tc.code // avoid tripping dupNama instead
			results := validateProvinceRows([]importer.RawRow{row(1, cells)}, lk)
			if results[0].Valid {
				t.Fatalf("expected invalid row for taken kode %q", tc.code)
			}
			if !hasErr(results[0], "dupKode") {
				t.Fatalf("expected dupKode, got %v", errKeys(results[0]))
			}
		})
	}

	t.Run("in-file duplicate", func(t *testing.T) {
		r1 := validProvinceCells()
		r1[provColCode] = "SU"
		r1[provColName] = "Sumatera Utara"
		r2 := validProvinceCells()
		r2[provColCode] = "su" // same code, different case
		r2[provColName] = "Sumatera Utara Baru"

		results := validateProvinceRows([]importer.RawRow{row(1, r1), row(2, r2)}, lk)
		if !results[0].Valid {
			t.Fatalf("row 1 expected valid, got %v", errKeys(results[0]))
		}
		if results[1].Valid || !hasErr(results[1], "dupKode") {
			t.Fatalf("row 2 expected dupKode, got %v", errKeys(results[1]))
		}
	})
}

func TestReferenceImporterProvinceRows_DupNama(t *testing.T) {
	lk := mkProvinceLookups()

	t.Run("exists in db (case-insensitive)", func(t *testing.T) {
		cells := validProvinceCells()
		cells[provColName] = "jawa barat"
		cells[provColCode] = "" // avoid tripping dupKode instead
		results := validateProvinceRows([]importer.RawRow{row(1, cells)}, lk)
		if results[0].Valid {
			t.Fatalf("expected invalid row for duplicate nama")
		}
		if !hasErr(results[0], "dupNama") {
			t.Fatalf("expected dupNama, got %v", errKeys(results[0]))
		}
	})

	t.Run("in-file duplicate", func(t *testing.T) {
		r1 := validProvinceCells()
		r1[provColName] = "Kalimantan Timur"
		r1[provColCode] = ""
		r2 := validProvinceCells()
		r2[provColName] = "kalimantan timur" // same name, different case
		r2[provColCode] = ""

		results := validateProvinceRows([]importer.RawRow{row(1, r1), row(2, r2)}, lk)
		if !results[0].Valid {
			t.Fatalf("row 1 expected valid, got %v", errKeys(results[0]))
		}
		if results[1].Valid || !hasErr(results[1], "dupNama") {
			t.Fatalf("row 2 expected dupNama, got %v", errKeys(results[1]))
		}
	})
}

func TestReferenceImporterProvinceRows_ValidBatch(t *testing.T) {
	lk := mkProvinceLookups()

	r1 := validProvinceCells()
	r2 := validProvinceCells()
	r2[provColName] = "Bali"
	r2[provColCode] = "" // kode is optional

	results := validateProvinceRows([]importer.RawRow{row(1, r1), row(2, r2)}, lk)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, res := range results {
		if !res.Valid {
			t.Fatalf("row %d expected valid, got %v", i+1, errKeys(res))
		}
	}
}

func TestReferenceImporterProvinceContract(t *testing.T) {
	imp := NewImporter(nil, "provinces")
	if imp.Target() != "reference:provinces" {
		t.Fatalf("Target() = %q, want reference:provinces", imp.Target())
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
		{provColName, true, "text"},
		{provColCode, false, "text"},
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

// --- cities -----------------------------------------------------------------

// cityFixedIDs holds resolved province UUIDs so assertions can compare
// stamped Data against known values.
type cityFixedIDs struct {
	jateng uuid.UUID // "jawa tengah" / "jt"
}

// mkCityLookups builds a hand-crafted cityLookups (no DB): one pre-seeded
// existing city code ("bdg", lower-cased — distinct from validCityCells'
// default "SMG" so unrelated tests don't accidentally collide) so the
// "already exists in DB" branch of the kode rule can be exercised without a
// database.
func mkCityLookups() (cityLookups, cityFixedIDs) {
	ids := cityFixedIDs{jateng: uuid.New()}
	lk := cityLookups{
		provinces: map[string]uuid.UUID{
			"jawa tengah": ids.jateng,
			"jt":          ids.jateng,
		},
		existingCodes: map[string]bool{"bdg": true},
	}
	return lk, ids
}

func validCityCells() map[string]string {
	return map[string]string{
		cityColName:     "Semarang",
		cityColProvince: "Jawa Tengah",
		cityColCode:     "SMG",
	}
}

func TestReferenceImporterCityRows_Required(t *testing.T) {
	lk, _ := mkCityLookups()

	cases := []struct {
		name   string
		mutate func(map[string]string)
	}{
		{"nama empty", func(c map[string]string) { c[cityColName] = "" }},
		{"provinsi empty", func(c map[string]string) { c[cityColProvince] = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cells := validCityCells()
			tc.mutate(cells)
			results := validateCityRows([]importer.RawRow{row(1, cells)}, lk)
			if results[0].Valid {
				t.Fatalf("expected invalid row")
			}
			if !hasErr(results[0], "required") {
				t.Fatalf("expected required error, got %v", errKeys(results[0]))
			}
		})
	}
}

func TestReferenceImporterCityRows_ProvinceMiss(t *testing.T) {
	lk, _ := mkCityLookups()
	cells := validCityCells()
	cells[cityColProvince] = "Provinsi Tak Ada"

	results := validateCityRows([]importer.RawRow{row(1, cells)}, lk)
	if results[0].Valid {
		t.Fatalf("expected invalid row for unknown provinsi")
	}
	if !hasErr(results[0], "provinsi") {
		t.Fatalf("expected provinsi error, got %v", errKeys(results[0]))
	}
}

func TestReferenceImporterCityRows_ProvinceByCode(t *testing.T) {
	lk, ids := mkCityLookups()
	cells := validCityCells()
	cells[cityColProvince] = "jt" // resolve by code, case-insensitive

	results := validateCityRows([]importer.RawRow{row(1, cells)}, lk)
	if !results[0].Valid {
		t.Fatalf("expected valid row, got %v", errKeys(results[0]))
	}
	if results[0].Data["_province_id"] != ids.jateng.String() {
		t.Fatalf("_province_id = %q, want %q", results[0].Data["_province_id"], ids.jateng.String())
	}
}

func TestReferenceImporterCityRows_DupKodeExistingInDB(t *testing.T) {
	lk, _ := mkCityLookups()

	cases := []struct {
		name string
		code string
	}{
		{"exists in db", "BDG"},
		{"exists in db case-insensitive", "bdg"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cells := validCityCells()
			cells[cityColCode] = tc.code
			results := validateCityRows([]importer.RawRow{row(1, cells)}, lk)
			if results[0].Valid {
				t.Fatalf("expected invalid row for taken kode %q", tc.code)
			}
			if !hasErr(results[0], "dupKode") {
				t.Fatalf("expected dupKode, got %v", errKeys(results[0]))
			}
		})
	}

	t.Run("fresh code stays valid", func(t *testing.T) {
		cells := validCityCells()
		cells[cityColCode] = "FRESH"
		results := validateCityRows([]importer.RawRow{row(1, cells)}, lk)
		if !results[0].Valid {
			t.Fatalf("row expected valid, got %v", errKeys(results[0]))
		}
	})
}

func TestReferenceImporterCityRows_DupKodeInFile(t *testing.T) {
	lk, _ := mkCityLookups()

	r1 := validCityCells()
	r1[cityColCode] = "SMG"
	r2 := validCityCells()
	r2[cityColName] = "Semarang Baru"
	r2[cityColCode] = "smg" // same code, different case

	results := validateCityRows([]importer.RawRow{row(1, r1), row(2, r2)}, lk)
	if !results[0].Valid {
		t.Fatalf("row 1 expected valid, got %v", errKeys(results[0]))
	}
	if results[1].Valid || !hasErr(results[1], "dupKode") {
		t.Fatalf("row 2 expected dupKode, got %v", errKeys(results[1]))
	}
}

func TestReferenceImporterCityRows_ValidBatch(t *testing.T) {
	lk, ids := mkCityLookups()

	r1 := validCityCells()
	r2 := validCityCells()
	r2[cityColName] = "Surakarta"
	r2[cityColCode] = "" // kode is optional

	results := validateCityRows([]importer.RawRow{row(1, r1), row(2, r2)}, lk)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, res := range results {
		if !res.Valid {
			t.Fatalf("row %d expected valid, got %v", i+1, errKeys(res))
		}
		if res.Data["_province_id"] != ids.jateng.String() {
			t.Fatalf("row %d _province_id = %q, want %q", i+1, res.Data["_province_id"], ids.jateng.String())
		}
	}
}

func TestReferenceImporterCityRows_InvalidRowDropsStamp(t *testing.T) {
	lk, _ := mkCityLookups()
	cells := validCityCells()
	cells[cityColName] = "" // invalid, but provinsi still resolves

	results := validateCityRows([]importer.RawRow{row(1, cells)}, lk)
	if results[0].Valid {
		t.Fatalf("expected invalid row")
	}
	if _, ok := results[0].Data["_province_id"]; ok {
		t.Fatalf("expected _province_id stamp dropped from invalid row, got %q", results[0].Data["_province_id"])
	}
}

func TestReferenceImporterCityContract(t *testing.T) {
	imp := NewImporter(nil, "cities")
	if imp.Target() != "reference:cities" {
		t.Fatalf("Target() = %q, want reference:cities", imp.Target())
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
		{cityColName, true, "text"},
		{cityColProvince, true, "lookup"},
		{cityColCode, false, "text"},
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

// --- brands -----------------------------------------------------------------

func mkBrandLookups() brandLookups {
	return brandLookups{existingNames: map[string]bool{"canon": true}}
}

func TestReferenceImporterBrandRows(t *testing.T) {
	lk := mkBrandLookups()
	t.Run("required", func(t *testing.T) {
		res := validateBrandRows([]importer.RawRow{row(1, map[string]string{brandColName: ""})}, lk)
		if res[0].Valid || !hasErr(res[0], "required") {
			t.Fatalf("want required, got %v", errKeys(res[0]))
		}
	})
	t.Run("dup in db (case-insensitive)", func(t *testing.T) {
		res := validateBrandRows([]importer.RawRow{row(1, map[string]string{brandColName: "Canon"})}, lk)
		if res[0].Valid || !hasErr(res[0], "dupNama") {
			t.Fatalf("want dupNama, got %v", errKeys(res[0]))
		}
	})
	t.Run("in-file dup", func(t *testing.T) {
		res := validateBrandRows([]importer.RawRow{
			row(1, map[string]string{brandColName: "Epson"}),
			row(2, map[string]string{brandColName: "epson"}),
		}, lk)
		if !res[0].Valid {
			t.Fatalf("row1 want valid, got %v", errKeys(res[0]))
		}
		if res[1].Valid || !hasErr(res[1], "dupNama") {
			t.Fatalf("row2 want dupNama, got %v", errKeys(res[1]))
		}
	})
	t.Run("contract", func(t *testing.T) {
		imp := NewImporter(nil, "brands")
		if imp.Target() != "reference:brands" || imp.NeedsApproval() {
			t.Fatalf("bad contract: %q %v", imp.Target(), imp.NeedsApproval())
		}
		if cols := imp.Columns(); len(cols) != 1 || cols[0].Name != brandColName || !cols[0].Required {
			t.Fatalf("bad columns: %+v", imp.Columns())
		}
	})
}

// --- units ------------------------------------------------------------------

func mkUnitLookups() unitLookups {
	return unitLookups{existingNames: map[string]bool{"unit": true}}
}

func TestReferenceImporterUnitRows(t *testing.T) {
	lk := mkUnitLookups()
	t.Run("required", func(t *testing.T) {
		res := validateUnitRows([]importer.RawRow{row(1, map[string]string{unitColName: "", unitColSymbol: "pcs"})}, lk)
		if res[0].Valid || !hasErr(res[0], "required") {
			t.Fatalf("want required, got %v", errKeys(res[0]))
		}
	})
	t.Run("symbol optional + valid", func(t *testing.T) {
		res := validateUnitRows([]importer.RawRow{row(1, map[string]string{unitColName: "Meter", unitColSymbol: ""})}, lk)
		if !res[0].Valid {
			t.Fatalf("want valid, got %v", errKeys(res[0]))
		}
	})
	t.Run("dup in db", func(t *testing.T) {
		res := validateUnitRows([]importer.RawRow{row(1, map[string]string{unitColName: "Unit"})}, lk)
		if res[0].Valid || !hasErr(res[0], "dupNama") {
			t.Fatalf("want dupNama, got %v", errKeys(res[0]))
		}
	})
	t.Run("contract", func(t *testing.T) {
		imp := NewImporter(nil, "units")
		if imp.Target() != "reference:units" || imp.NeedsApproval() {
			t.Fatalf("bad contract")
		}
		if cols := imp.Columns(); len(cols) != 2 || cols[0].Name != unitColName || cols[1].Name != unitColSymbol {
			t.Fatalf("bad columns: %+v", imp.Columns())
		}
	})
}
