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
	office   uuid.UUID // "kantor pusat" / "KP" — in scope
	office2  uuid.UUID // "cabang jakarta" / "CBJ" — in scope
	vendor   uuid.UUID
	room     uuid.UUID // "ruang server" / "RS01" — lives in office
}

// mkLookups builds a hand-crafted assetLookups (no DB) plus the fixed IDs. The
// existing-tag set contains one tag so the "already exists in DB" branch of the
// asset_tag rule can be exercised without a database.
func mkLookups() (assetLookups, fixedIDs) {
	ids := fixedIDs{
		category: uuid.New(),
		office:   uuid.New(),
		office2:  uuid.New(),
		vendor:   uuid.New(),
		room:     uuid.New(),
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
		vendors: map[string]uuid.UUID{
			"pt maju jaya": ids.vendor,
		},
		rooms: map[string][]roomRef{
			"ruang server": {{id: ids.room, officeID: ids.office}},
			"rs01":         {{id: ids.room, officeID: ids.office}},
		},
		existingTags: map[string]bool{
			"KP-ELK-2026-00001": true,
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

// validCells returns a fully-valid cell set for the "kantor pusat" office; the
// caller can mutate individual keys to construct a specific failure.
func validCells() map[string]string {
	return map[string]string{
		"asset_tag": "",
		"nama":      "Laptop Dell",
		"kategori":  "Elektronik",
		"kantor":    "Kantor Pusat",
		"tgl_beli":  "2026-01-15",
		"harga":     "15000000.00",
		"vendor":    "PT Maju Jaya",
		"lokasi":    "Ruang Server",
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

// --- per-field error-key tests ------------------------------------------

func TestAssetImporterValidateErrorKeys(t *testing.T) {
	lk, _ := mkLookups()

	cases := []struct {
		name   string
		mutate func(map[string]string)
		want   string
	}{
		{"required nama", func(c map[string]string) { c["nama"] = "" }, "required"},
		{"required kategori", func(c map[string]string) { c["kategori"] = "" }, "required"},
		{"required kantor", func(c map[string]string) { c["kantor"] = "" }, "required"},
		{"required tgl_beli", func(c map[string]string) { c["tgl_beli"] = "" }, "required"},
		{"required harga", func(c map[string]string) { c["harga"] = "" }, "required"},
		{"category miss", func(c map[string]string) { c["kategori"] = "Tak Ada" }, "kat"},
		{"office miss", func(c map[string]string) { c["kantor"] = "Tak Ada" }, "kantor"},
		{"vendor miss", func(c map[string]string) { c["vendor"] = "Tak Ada" }, "vendor"},
		{"room wrong office", func(c map[string]string) { c["kantor"] = "Cabang Jakarta" }, "lokasi"},
		{"room miss", func(c map[string]string) { c["lokasi"] = "Tak Ada" }, "lokasi"},
		{"bad date", func(c map[string]string) { c["tgl_beli"] = "15-01-2026" }, "tgl"},
		{"bad date value", func(c map[string]string) { c["tgl_beli"] = "2026-13-40" }, "tgl"},
		{"negative price", func(c map[string]string) { c["harga"] = "-100" }, "harga"},
		{"non-decimal price", func(c map[string]string) { c["harga"] = "abc" }, "harga"},
		{"fraction price", func(c map[string]string) { c["harga"] = "1/2" }, "harga"},
		{"tag exists in db", func(c map[string]string) { c["asset_tag"] = "KP-ELK-2026-00001" }, "dupTag"},
		{"tag bad format", func(c map[string]string) { c["asset_tag"] = "bad tag!!" }, "dupTag"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cells := validCells()
			tc.mutate(cells)
			results := validateAssetRows([]importer.RawRow{row(1, cells)}, lk, allScope())
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

// --- multiOffice --------------------------------------------------------

func TestAssetImporterValidateMultiOffice(t *testing.T) {
	lk, ids := mkLookups()

	r1 := validCells() // Kantor Pusat
	r2 := validCells()
	r2["kantor"] = "Cabang Jakarta" // different office
	r2["lokasi"] = ""               // avoid an unrelated lokasi error

	results := validateAssetRows(
		[]importer.RawRow{row(1, r1), row(2, r2)},
		lk, allScope(),
	)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Row 1 established the batch office and must stay valid.
	if !results[0].Valid {
		t.Fatalf("row 1 expected valid, got errors %v", errKeys(results[0]))
	}
	if results[0].NormalizedRef != ids.office.String() {
		t.Fatalf("row 1 NormalizedRef = %q, want %q", results[0].NormalizedRef, ids.office.String())
	}
	// Row 2 differs from the batch office -> multiOffice, invalid.
	if results[1].Valid {
		t.Fatalf("row 2 expected invalid (multiOffice)")
	}
	if !hasErr(results[1], "multiOffice") {
		t.Fatalf("row 2 expected multiOffice, got %v", errKeys(results[1]))
	}
}

// --- scope --------------------------------------------------------------

func TestAssetImporterValidateScope(t *testing.T) {
	lk, ids := mkLookups()

	// Caller scope permits office2 only; a row resolving office (office1) is
	// out of scope.
	scope := importer.Scope{AllScope: false, OfficeIDs: []uuid.UUID{ids.office2}, UserID: uuid.New()}

	cells := validCells() // resolves to "Kantor Pusat" == ids.office (not in scope)
	results := validateAssetRows([]importer.RawRow{row(1, cells)}, lk, scope)
	if results[0].Valid {
		t.Fatalf("expected out-of-scope row invalid")
	}
	if !hasErr(results[0], "scope") {
		t.Fatalf("expected scope error, got %v", errKeys(results[0]))
	}
}

// --- in-file duplicate tag ---------------------------------------------

func TestAssetImporterValidateInFileDuplicateTag(t *testing.T) {
	lk, _ := mkLookups()

	r1 := validCells()
	r1["asset_tag"] = "KP-ELK-2026-09999"
	r2 := validCells()
	r2["asset_tag"] = "kp-elk-2026-09999" // same tag, different case
	r2["lokasi"] = ""

	results := validateAssetRows([]importer.RawRow{row(1, r1), row(2, r2)}, lk, allScope())
	// First occurrence is fine.
	if !results[0].Valid {
		t.Fatalf("row 1 expected valid, got %v", errKeys(results[0]))
	}
	// Second occurrence is a case-insensitive in-file duplicate.
	if results[1].Valid {
		t.Fatalf("row 2 expected invalid (dupTag)")
	}
	if !hasErr(results[1], "dupTag") {
		t.Fatalf("row 2 expected dupTag, got %v", errKeys(results[1]))
	}
}

// --- fully-valid batch --------------------------------------------------

func TestAssetImporterValidateAllValid(t *testing.T) {
	lk, ids := mkLookups()

	r1 := validCells()               // vendor + lokasi present
	r2 := validCells()               // minimal optional fields
	r2["asset_tag"] = "KP-ELK-2026-12345" // explicit, valid, unique tag
	r2["vendor"] = ""
	r2["lokasi"] = ""

	results := validateAssetRows([]importer.RawRow{row(1, r1), row(2, r2)}, lk, allScope())
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, res := range results {
		if !res.Valid {
			t.Fatalf("row %d expected valid, got %v", i+1, errKeys(res))
		}
		if res.NormalizedRef != ids.office.String() {
			t.Fatalf("row %d NormalizedRef = %q, want %q", i+1, res.NormalizedRef, ids.office.String())
		}
		if res.Data["_office_id"] != ids.office.String() {
			t.Fatalf("row %d _office_id = %q, want %q", i+1, res.Data["_office_id"], ids.office.String())
		}
		if res.Data["_category_id"] != ids.category.String() {
			t.Fatalf("row %d _category_id = %q, want %q", i+1, res.Data["_category_id"], ids.category.String())
		}
	}

	// Row 1 carried vendor + lokasi -> resolved IDs stamped.
	if results[0].Data["_vendor_id"] != ids.vendor.String() {
		t.Fatalf("row 1 _vendor_id = %q, want %q", results[0].Data["_vendor_id"], ids.vendor.String())
	}
	if results[0].Data["_room_id"] != ids.room.String() {
		t.Fatalf("row 1 _room_id = %q, want %q", results[0].Data["_room_id"], ids.room.String())
	}
	// Row 2 had neither -> empty stamps.
	if results[1].Data["_vendor_id"] != "" {
		t.Fatalf("row 2 _vendor_id = %q, want empty", results[1].Data["_vendor_id"])
	}
	if results[1].Data["_room_id"] != "" {
		t.Fatalf("row 2 _room_id = %q, want empty", results[1].Data["_room_id"])
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
		{"asset_tag", false, "text"},
		{"nama", true, "text"},
		{"kategori", true, "lookup"},
		{"kantor", true, "lookup"},
		{"tgl_beli", true, "date"},
		{"harga", true, "decimal"},
		{"vendor", false, "lookup"},
		{"lokasi", false, "lookup"},
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
