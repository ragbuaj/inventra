package employee

import (
	"testing"

	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/importer"
)

// --- test fixtures -------------------------------------------------------

// fixedIDs holds the resolved UUIDs used across the validation tests so
// assertions can compare stamped Data against known values.
type fixedIDs struct {
	office  uuid.UUID // "kantor pusat" / "KP" — in scope
	office2 uuid.UUID // "cabang jakarta" / "CBJ" — out of scope in the scope test
}

// mkLookups builds a hand-crafted employeeLookups (no DB) plus the fixed IDs.
// existingCodes contains one code so the "already exists in DB" branch of the
// kode rule can be exercised without a database.
func mkLookups() (employeeLookups, fixedIDs) {
	ids := fixedIDs{
		office:  uuid.New(),
		office2: uuid.New(),
	}
	lk := employeeLookups{
		offices: map[string]uuid.UUID{
			"kantor pusat":   ids.office,
			"kp":             ids.office,
			"cabang jakarta": ids.office2,
			"cbj":            ids.office2,
		},
		existingCodes: map[string]bool{
			"emp-0001": true,
		},
		departments: map[string]uuid.UUID{},
		positions:   map[string]uuid.UUID{},
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
		colCode:   "EMP-0100",
		colName:   "Budi Santoso",
		colEmail:  "budi.santoso@bank.co.id",
		colPhone:  "081234567890",
		colOffice: "Kantor Pusat",
		colStatus: "active",
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

func TestEmployeeImporterValidateErrorKeys(t *testing.T) {
	lk, _ := mkLookups()

	cases := []struct {
		name   string
		mutate func(map[string]string)
		want   string
	}{
		{"required kode", func(c map[string]string) { c[colCode] = "" }, "required"},
		{"required nama", func(c map[string]string) { c[colName] = "" }, "required"},
		{"required kantor", func(c map[string]string) { c[colOffice] = "" }, "required"},
		{"required status", func(c map[string]string) { c[colStatus] = "" }, "required"},
		{"office miss", func(c map[string]string) { c[colOffice] = "Tak Ada" }, "kantor"},
		{"status invalid", func(c map[string]string) { c[colStatus] = "banned" }, "status"},
		{"email no at", func(c map[string]string) { c[colEmail] = "budiATbank.co.id" }, "email"},
		{"email no domain dot", func(c map[string]string) { c[colEmail] = "budi@bank" }, "email"},
		{"email trailing dot domain", func(c map[string]string) { c[colEmail] = "budi@bank." }, "email"},
		{"kode exists in db", func(c map[string]string) { c[colCode] = "EMP-0001" }, "dupKode"},
		{"kode exists in db case-insensitive", func(c map[string]string) { c[colCode] = "emp-0001" }, "dupKode"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cells := validCells()
			tc.mutate(cells)
			results := validateEmployeeRows([]importer.RawRow{row(1, cells)}, lk, allScope())
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

// --- optional fields ------------------------------------------------------

func TestEmployeeImporterValidateOptionalFieldsEmpty(t *testing.T) {
	lk, _ := mkLookups()
	cells := validCells()
	cells[colEmail] = ""
	cells[colPhone] = ""

	results := validateEmployeeRows([]importer.RawRow{row(1, cells)}, lk, allScope())
	if !results[0].Valid {
		t.Fatalf("expected row valid with empty optional fields, got %v", errKeys(results[0]))
	}
}

// --- status enum labels ----------------------------------------------------

func TestEmployeeImporterValidateStatusLabels(t *testing.T) {
	lk, _ := mkLookups()

	for _, label := range []string{"active", "inactive", "suspended", "ACTIVE", "Inactive"} {
		cells := validCells()
		cells[colStatus] = label
		results := validateEmployeeRows([]importer.RawRow{row(1, cells)}, lk, allScope())
		if !results[0].Valid {
			t.Fatalf("status %q: expected valid, got %v", label, errKeys(results[0]))
		}
	}
}

// --- scope ------------------------------------------------------------------

func TestEmployeeImporterValidateScope(t *testing.T) {
	lk, ids := mkLookups()

	// Caller scope permits office2 only; a row resolving office (office1) is
	// out of scope.
	scope := importer.Scope{AllScope: false, OfficeIDs: []uuid.UUID{ids.office2}, UserID: uuid.New()}

	cells := validCells() // resolves to "Kantor Pusat" == ids.office (not in scope)
	results := validateEmployeeRows([]importer.RawRow{row(1, cells)}, lk, scope)
	if results[0].Valid {
		t.Fatalf("expected out-of-scope row invalid")
	}
	if !hasErr(results[0], "scope") {
		t.Fatalf("expected scope error, got %v", errKeys(results[0]))
	}
}

// --- in-file duplicate kode --------------------------------------------------

func TestEmployeeImporterValidateInFileDuplicateKode(t *testing.T) {
	lk, _ := mkLookups()

	r1 := validCells()
	r1[colCode] = "EMP-0200"
	r2 := validCells()
	r2[colCode] = "emp-0200" // same code, different case

	results := validateEmployeeRows([]importer.RawRow{row(1, r1), row(2, r2)}, lk, allScope())
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

func TestEmployeeImporterValidateAllValid(t *testing.T) {
	lk, ids := mkLookups()

	r1 := validCells()
	r2 := validCells()
	r2[colCode] = "EMP-0101"
	r2[colEmail] = ""
	r2[colPhone] = ""

	results := validateEmployeeRows([]importer.RawRow{row(1, r1), row(2, r2)}, lk, allScope())
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, res := range results {
		if !res.Valid {
			t.Fatalf("row %d expected valid, got %v", i+1, errKeys(res))
		}
		if res.NormalizedRef != "" {
			t.Fatalf("row %d NormalizedRef = %q, want empty (employees have no batch-office rule)", i+1, res.NormalizedRef)
		}
		if res.Data["_office_id"] != ids.office.String() {
			t.Fatalf("row %d _office_id = %q, want %q", i+1, res.Data["_office_id"], ids.office.String())
		}
	}
}

// --- invalid rows keep clean data (no leaked internal stamps) ------------

func TestEmployeeImporterValidateInvalidRowDropsStamp(t *testing.T) {
	lk, _ := mkLookups()
	cells := validCells()
	cells[colStatus] = "bogus" // invalid, but office still resolves

	results := validateEmployeeRows([]importer.RawRow{row(1, cells)}, lk, allScope())
	if results[0].Valid {
		t.Fatalf("expected invalid row")
	}
	if _, ok := results[0].Data["_office_id"]; ok {
		t.Fatalf("expected _office_id stamp dropped from invalid row, got %q", results[0].Data["_office_id"])
	}
}

// --- department / position (optional lookups) ----------------------------

func TestValidateEmployeeRows_DeptPositionResolved(t *testing.T) {
	deptID, posID := uuid.New(), uuid.New()
	lk := employeeLookups{
		offices:       map[string]uuid.UUID{"kantor pusat": uuid.New()},
		existingCodes: map[string]bool{},
		departments:   map[string]uuid.UUID{"ti": deptID, "dept-ti": deptID},
		positions:     map[string]uuid.UUID{"staf": posID},
	}
	// pick an office id the scope permits
	var offID uuid.UUID
	for _, v := range lk.offices {
		offID = v
	}
	cells := map[string]string{
		colCode: "E001", colName: "Budi", colOffice: "Kantor Pusat", colStatus: "active",
		colDepartment: "TI", colPosition: "Staf",
	}
	res := validateEmployeeRows([]importer.RawRow{row(1, cells)}, lk, importer.Scope{AllScope: true})
	if !res[0].Valid {
		t.Fatalf("expected valid, got %v", errKeys(res[0]))
	}
	if res[0].Data["_department_id"] != deptID.String() {
		t.Fatalf("_department_id=%q want %q", res[0].Data["_department_id"], deptID.String())
	}
	if res[0].Data["_position_id"] != posID.String() {
		t.Fatalf("_position_id=%q want %q", res[0].Data["_position_id"], posID.String())
	}
	_ = offID
}

func TestValidateEmployeeRows_DeptPositionOptional(t *testing.T) {
	lk := employeeLookups{
		offices:       map[string]uuid.UUID{"kantor pusat": uuid.New()},
		existingCodes: map[string]bool{},
		departments:   map[string]uuid.UUID{},
		positions:     map[string]uuid.UUID{},
	}
	cells := map[string]string{colCode: "E002", colName: "Sari", colOffice: "Kantor Pusat", colStatus: "active"}
	res := validateEmployeeRows([]importer.RawRow{row(1, cells)}, lk, importer.Scope{AllScope: true})
	if !res[0].Valid {
		t.Fatalf("empty dept/position must be valid, got %v", errKeys(res[0]))
	}
	if _, ok := res[0].Data["_department_id"]; ok {
		t.Fatalf("no dept stamp expected")
	}
}

func TestValidateEmployeeRows_DeptPositionMiss(t *testing.T) {
	lk := employeeLookups{
		offices:       map[string]uuid.UUID{"kantor pusat": uuid.New()},
		existingCodes: map[string]bool{},
		departments:   map[string]uuid.UUID{},
		positions:     map[string]uuid.UUID{},
	}
	cells := map[string]string{
		colCode: "E003", colName: "Tono", colOffice: "Kantor Pusat", colStatus: "active",
		colDepartment: "Tak Ada", colPosition: "Hantu",
	}
	res := validateEmployeeRows([]importer.RawRow{row(1, cells)}, lk, importer.Scope{AllScope: true})
	if res[0].Valid || !hasErr(res[0], "departemen") || !hasErr(res[0], "jabatan") {
		t.Fatalf("expected departemen+jabatan errors, got %v", errKeys(res[0]))
	}
	if _, ok := res[0].Data["_department_id"]; ok {
		t.Fatalf("invalid row must drop dept stamp")
	}
}

// --- columns / needs-approval contract -----------------------------------

func TestEmployeeImporterColumns(t *testing.T) {
	imp := employeeImporter{}
	if imp.Target() != "employee" {
		t.Fatalf("Target() = %q, want employee", imp.Target())
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
		{colEmail, false, "text"},
		{colPhone, false, "text"},
		{colOffice, true, "lookup"},
		{colStatus, true, "text"},
		{colDepartment, false, "lookup"},
		{colPosition, false, "lookup"},
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
