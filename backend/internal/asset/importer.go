package asset

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/importer"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Asset import column names. These mirror the legacy export layout the bank
// migrates from (English headers). The asset tag is NOT a column — it is always
// generated per-office. `Location Folder Name` is the office, `Asset Type Name`
// the category, `Location Name` a "Floor - Room" (or floor-only) location,
// `User` the PIC (by NIP), `Legacy Asset Code`/`Legacy Barcode` archive to the
// legacy_* fields.
const (
	colOffice    = "Location Folder Name"
	colCategory  = "Asset Type Name"
	colName      = "Asset Name"
	colLocation  = "Location Name"
	colDate      = "Purchase Date"
	colPrice     = "Purchase Price"
	colUser      = "User"
	colCondition = "Condition"
	colAssetCode = "Legacy Asset Code" // -> legacy_asset_code
	colBarcode   = "Legacy Barcode"    // -> legacy_barcode
	colBrand     = "Merk"
	colCapacity  = "Kapasitas"
	colSpk       = "SPK Number"
)

// hargaKey is the internal Data key the generic worker sums to compute the
// batch's maker-checker approval amount (see importer/worker.go sumHarga). The
// asset importer stamps the parsed Purchase Price here so the worker stays
// target-agnostic.
const hargaKey = "harga"

// importLookupLimit bounds the office lookup page. Office volume is reference-
// scale (a bank's org tree), so a single generous page is sufficient.
const importLookupLimit = 100000

// dropdownLimit caps how many options each template dropdown carries, so a large
// employee/room set does not bloat the generated template. Values beyond the cap
// are still accepted on import (the column is validated, not restricted).
const dropdownLimit = 2000

// dateLayout is the accepted purchase-date format for imported rows.
const dateLayout = "2006-01-02"

// decimalRe matches a non-negative plain decimal (no sign, no scientific
// notation) — the accepted form for the Purchase Price column.
var decimalRe = regexp.MustCompile(`^\d+(\.\d+)?$`)

// floorRef is a floor's id and the office it belongs to, used to resolve a
// row's Location Name floor within that row's resolved office.
type floorRef struct {
	id       uuid.UUID
	officeID uuid.UUID
}

// roomRef is a room's id together with its floor and office, used to resolve a
// "Floor - Room" Location Name to a room in the right floor+office.
type roomRef struct {
	id      uuid.UUID
	floorID uuid.UUID
}

// empRef is an employee's id and office, used to resolve the `User` (PIC) by NIP
// and enforce that the PIC belongs to the row's office.
type empRef struct {
	id       uuid.UUID
	officeID uuid.UUID
}

// assetLookups holds the case-insensitive name -> id maps the asset importer
// validates rows against. Keys are lower-cased. Built once per batch by
// buildAssetLookups; consumed by the pure validateAssetRows.
type assetLookups struct {
	categories map[string]uuid.UUID
	offices    map[string]uuid.UUID
	brands     map[string]uuid.UUID
	employees  map[string]empRef     // keyed by lower(NIP)
	floors     map[string][]floorRef // keyed by lower(floor name)
	rooms      map[string][]roomRef  // keyed by lower(room name)
}

// assetImporter is the asset import target: it validates a batch of asset rows
// and, once its maker-checker approval is granted, creates the assets. It
// implements importer.TargetImporter and importer.TemplateProvider.
type assetImporter struct{ s *Service }

// Importer returns the asset import target for registration with the generic
// import engine.
func (s *Service) Importer() importer.TargetImporter { return assetImporter{s} }

// Target returns the importer's registry key.
func (assetImporter) Target() string { return "asset" }

// NeedsApproval reports that asset imports must clear the value-tiered
// maker-checker approval flow before the rows are created.
func (assetImporter) NeedsApproval() bool { return true }

// Columns describes the asset import template (legacy English layout). Required
// columns (marked with "*" in the generated header): Location Folder Name,
// Asset Type Name, Asset Name, Location Name, Condition. Purchase Price is
// optional per the source layout — rows without it contribute 0 to the batch's
// approval amount.
func (assetImporter) Columns() []importer.ColumnSpec {
	return []importer.ColumnSpec{
		{Name: colOffice, Required: true, Kind: "lookup", Example: "Kantor Pusat"},
		{Name: colCategory, Required: true, Kind: "lookup", Example: "Chiller"},
		{Name: colName, Required: true, Kind: "text", Example: "Chiller Daikin 5PK"},
		{Name: colLocation, Required: true, Kind: "lookup", Example: "Lantai 1 - Ruang Server"},
		{Name: colDate, Required: false, Kind: "date", Example: "2023-05-10"},
		{Name: colPrice, Required: false, Kind: "decimal", Example: "15000000"},
		{Name: colUser, Required: false, Kind: "lookup", Example: "12345 - Budi Santoso"},
		{Name: colCondition, Required: true, Kind: "text", Example: "Baik"},
		{Name: colAssetCode, Required: false, Kind: "text", Example: "KP0CHL202300001"},
		{Name: colBarcode, Required: false, Kind: "text", Example: "8991234567890"},
		{Name: colBrand, Required: false, Kind: "lookup", Example: "Daikin"},
		{Name: colCapacity, Required: false, Kind: "text", Example: "5PK"},
		{Name: colSpk, Required: false, Kind: "text", Example: "SPK/012/BMIS/2023"},
	}
}

// exampleNote warns users away from filling the example sheet (it is never
// parsed — the parser only reads the first sheet, "Aset").
const exampleNote = `SHEET INI HANYA CONTOH - jangan diisi. Isi data Anda pada sheet "Aset". Sheet ini TIDAK akan diproses saat import.`

// TemplateSpec customizes the asset template: the fill sheet is "Aset", the
// example sheet "Contoh Pengisian" (with a note + two worked rows), required
// headers carry a "*", and lookup columns get dropdowns sourced from the
// caller's data scope (a hidden options sheet backs the drop lists).
func (a assetImporter) TemplateSpec(ctx context.Context, scope importer.Scope) (importer.TemplateSpec, error) {
	dropdowns, err := a.buildDropdowns(ctx, scope)
	if err != nil {
		return importer.TemplateSpec{}, err
	}
	return importer.TemplateSpec{
		DataSheet:    "Aset",
		ExampleSheet: "Contoh Pengisian",
		ExampleNote:  exampleNote,
		ExampleRows: [][]string{
			{"Kantor Pusat", "Chiller", "Chiller Daikin 5PK", "Lantai 1 - Ruang Server", "2023-05-10", "15000000", "12345 - Budi Santoso", "Baik", "KP0CHL202300001", "8991234567890", "Daikin", "5PK", "SPK/012/BMIS/2023"},
			{"Kantor Cabang Bekasi", "Genset", "Genset Perkins 100 kVA", "Lantai 1", "2021-11-02", "250000000", "", "Rusak", "KCB0GST202100007", "", "Perkins", "100 kVA", ""},
		},
		Dropdowns:    dropdowns,
		MarkRequired: true,
	}, nil
}

// buildDropdowns fetches the scoped option lists that back the template's
// lookup dropdowns. Condition is a fixed enum; the rest come from master data
// scoped to the caller. Each list is de-duplicated, sorted, and capped.
func (a assetImporter) buildDropdowns(ctx context.Context, scope importer.Scope) (map[string][]string, error) {
	q := a.s.q
	dd := map[string][]string{
		colCondition: {"Baik", "Rusak"},
	}

	offs, err := q.ListOffices(ctx, sqlc.ListOfficesParams{
		AllScope: scope.AllScope, OfficeIds: scope.OfficeIDs, Search: "", Lim: importLookupLimit, Off: 0,
	})
	if err != nil {
		return nil, err
	}
	officeNames := make([]string, 0, len(offs))
	for _, o := range offs {
		officeNames = append(officeNames, o.Name)
	}
	dd[colOffice] = sortedUniqueCapped(officeNames)

	cats, err := q.ListCategoryTree(ctx)
	if err != nil {
		return nil, err
	}
	catNames := make([]string, 0, len(cats))
	for _, c := range cats {
		catNames = append(catNames, c.Name)
	}
	dd[colCategory] = sortedUniqueCapped(catNames)

	floors, err := q.ListFloorsLookup(ctx, sqlc.ListFloorsLookupParams{AllScope: scope.AllScope, OfficeIds: scope.OfficeIDs})
	if err != nil {
		return nil, err
	}
	rooms, err := q.ListRoomsLookup(ctx, sqlc.ListRoomsLookupParams{AllScope: scope.AllScope, OfficeIds: scope.OfficeIDs})
	if err != nil {
		return nil, err
	}
	locations := make([]string, 0, len(floors)+len(rooms))
	for _, f := range floors {
		locations = append(locations, f.Name)
	}
	for _, r := range rooms {
		locations = append(locations, r.FloorName+" - "+r.Name)
	}
	dd[colLocation] = sortedUniqueCapped(locations)

	emps, err := q.ListEmployeeLookup(ctx)
	if err != nil {
		return nil, err
	}
	users := make([]string, 0, len(emps))
	for _, e := range emps {
		users = append(users, e.Code+" - "+e.Name)
	}
	dd[colUser] = sortedUniqueCapped(users)

	brnds, err := q.ListBrandsLookup(ctx)
	if err != nil {
		return nil, err
	}
	brandNames := make([]string, 0, len(brnds))
	for _, b := range brnds {
		brandNames = append(brandNames, b.Name)
	}
	dd[colBrand] = sortedUniqueCapped(brandNames)

	return dd, nil
}

// sortedUniqueCapped returns the distinct, sorted, non-empty values of in,
// truncated to dropdownLimit.
func sortedUniqueCapped(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	if len(out) > dropdownLimit {
		out = out[:dropdownLimit]
	}
	return out
}

// ValidateRows loads the lookup sets scoped to the caller, then runs the pure
// row validation. Splitting the DB step (buildAssetLookups) from the pure step
// (validateAssetRows) keeps the business rules unit-testable without a database.
func (a assetImporter) ValidateRows(ctx context.Context, rows []importer.RawRow, scope importer.Scope) ([]importer.RowResult, error) {
	lk, err := a.buildAssetLookups(ctx, scope)
	if err != nil {
		return nil, err
	}
	return validateAssetRows(rows, lk, scope), nil
}

// buildAssetLookups loads categories, offices, floors, rooms, brands, and
// employees into case-insensitive lookup maps. Offices/floors/rooms are scoped
// to the caller; categories, brands, and employees are global.
func (a assetImporter) buildAssetLookups(ctx context.Context, scope importer.Scope) (assetLookups, error) {
	lk := assetLookups{
		categories: map[string]uuid.UUID{},
		offices:    map[string]uuid.UUID{},
		brands:     map[string]uuid.UUID{},
		employees:  map[string]empRef{},
		floors:     map[string][]floorRef{},
		rooms:      map[string][]roomRef{},
	}

	cats, err := a.s.q.ListCategoryTree(ctx)
	if err != nil {
		return lk, err
	}
	for _, c := range cats {
		addKey(lk.categories, c.Name, c.ID)
		if c.Code != nil {
			addKey(lk.categories, *c.Code, c.ID)
		}
	}

	offs, err := a.s.q.ListOffices(ctx, sqlc.ListOfficesParams{
		AllScope: scope.AllScope, OfficeIds: scope.OfficeIDs, Search: "", Lim: importLookupLimit, Off: 0,
	})
	if err != nil {
		return lk, err
	}
	for _, o := range offs {
		addKey(lk.offices, o.Name, o.ID)
		addKey(lk.offices, o.Code, o.ID)
	}

	floors, err := a.s.q.ListFloorsLookup(ctx, sqlc.ListFloorsLookupParams{AllScope: scope.AllScope, OfficeIds: scope.OfficeIDs})
	if err != nil {
		return lk, err
	}
	for _, f := range floors {
		if k := normKey(f.Name); k != "" {
			lk.floors[k] = append(lk.floors[k], floorRef{id: f.ID, officeID: f.OfficeID})
		}
	}

	rooms, err := a.s.q.ListRoomsLookup(ctx, sqlc.ListRoomsLookupParams{AllScope: scope.AllScope, OfficeIds: scope.OfficeIDs})
	if err != nil {
		return lk, err
	}
	for _, r := range rooms {
		if k := normKey(r.Name); k != "" {
			lk.rooms[k] = append(lk.rooms[k], roomRef{id: r.ID, floorID: r.FloorID})
		}
	}

	brnds, err := a.s.q.ListBrandsLookup(ctx)
	if err != nil {
		return lk, err
	}
	for _, b := range brnds {
		addKey(lk.brands, b.Name, b.ID)
	}

	emps, err := a.s.q.ListEmployeeLookup(ctx)
	if err != nil {
		return lk, err
	}
	for _, e := range emps {
		if k := normKey(e.Code); k != "" {
			lk.employees[k] = empRef{id: e.ID, officeID: e.OfficeID}
		}
	}

	return lk, nil
}

// addKey inserts a lower-cased, trimmed name/code -> id entry, skipping empties.
func addKey(m map[string]uuid.UUID, name string, id uuid.UUID) {
	if k := normKey(name); k != "" {
		m[k] = id
	}
}

// normKey lower-cases and trims a lookup key.
func normKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// optText returns a pointer to the trimmed string, or nil when it is empty —
// used to store optional free-text columns (capacity, spk, legacy_*) as NULL.
func optText(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// picNIP extracts the employee NIP from a `User` cell. It accepts either a bare
// NIP ("12345") or the "NIP - Nama" display form ("12345 - Budi Santoso").
func picNIP(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, " - "); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// splitLocation splits a Location Name cell into floor and (optional) room:
// "Lantai 1 - Ruang Server" -> ("Lantai 1", "Ruang Server"); "Lantai 1" ->
// ("Lantai 1", "").
func splitLocation(s string) (floorName, roomName string) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, " - "); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+3:])
	}
	return s, ""
}

// condStatus maps a Condition cell to an asset status. It returns (status, ok):
// ok=false means the value is unrecognized. Callers only pass non-empty values
// (Condition is required, so empty is caught earlier).
func condStatus(s string) (string, bool) {
	switch normKey(s) {
	case "baik", "bagus", "bersih", "available":
		return "available", true
	case "rusak", "under_maintenance":
		return "under_maintenance", true
	default:
		return "", false
	}
}

// validateAssetRows validates raw asset rows against pre-loaded lookups and the
// caller's data scope, returning one RowResult per input row (same order). It
// performs NO database access: all resolution is against lk, so the full rule
// set is unit-testable with hand-built lookups.
//
// Batch rules: every row that resolves an office must resolve the SAME office
// (the first resolved one wins; differing rows get "multiOffice"). A valid
// row's NormalizedRef carries the resolved office UUID string so the worker can
// route the batch's approval; resolved ids are stamped into Data under
// "_"-prefixed keys for the executor. The Purchase Price is stamped under
// hargaKey so the worker can sum it for the approval amount.
func validateAssetRows(rows []importer.RawRow, lk assetLookups, scope importer.Scope) []importer.RowResult {
	type work struct {
		data      map[string]string
		errs      []importer.CellError
		office    uuid.UUID
		hasOffice bool
	}

	works := make([]work, len(rows))
	var batchOffice uuid.UUID
	batchSet := false

	for i, raw := range rows {
		data := map[string]string{
			colOffice:    trim(raw.Cells[colOffice]),
			colCategory:  trim(raw.Cells[colCategory]),
			colName:      trim(raw.Cells[colName]),
			colLocation:  trim(raw.Cells[colLocation]),
			colDate:      trim(raw.Cells[colDate]),
			colPrice:     trim(raw.Cells[colPrice]),
			colUser:      trim(raw.Cells[colUser]),
			colCondition: trim(raw.Cells[colCondition]),
			colAssetCode: trim(raw.Cells[colAssetCode]),
			colBarcode:   trim(raw.Cells[colBarcode]),
			colBrand:     trim(raw.Cells[colBrand]),
			colCapacity:  trim(raw.Cells[colCapacity]),
			colSpk:       trim(raw.Cells[colSpk]),
		}
		var errs []importer.CellError
		add := func(col, key string) { errs = append(errs, importer.CellError{Column: col, ErrorKey: key}) }

		// Required columns (Purchase Price is optional per the source layout).
		for _, col := range []string{colOffice, colCategory, colName, colLocation, colCondition} {
			if data[col] == "" {
				add(col, "required")
			}
		}

		// Asset Type Name -> category.
		var categoryID uuid.UUID
		if v := data[colCategory]; v != "" {
			if id, ok := lk.categories[normKey(v)]; ok {
				categoryID = id
			} else {
				add(colCategory, "kat")
			}
		}

		// Location Folder Name -> office.
		var officeID uuid.UUID
		hasOffice := false
		if v := data[colOffice]; v != "" {
			if id, ok := lk.offices[normKey(v)]; ok {
				officeID = id
				hasOffice = true
			} else {
				add(colOffice, "kantor")
			}
		}
		// Scope: a resolved office must be visible to the caller.
		if hasOffice && !scope.AllScope && !containsUUID(scope.OfficeIDs, officeID) {
			add(colOffice, "scope")
		}

		// Location Name -> floor (required) + room (optional), within the
		// row's office. Only resolvable once the office is known; when the
		// office is missing the office error already covers the row.
		var floorID, roomID uuid.UUID
		hasFloor, hasRoom := false, false
		if v := data[colLocation]; v != "" && hasOffice {
			fn, rn := splitLocation(v)
			for _, fr := range lk.floors[normKey(fn)] {
				if fr.officeID == officeID {
					floorID = fr.id
					hasFloor = true
					break
				}
			}
			if !hasFloor {
				add(colLocation, "lokasi")
			} else if rn != "" {
				for _, rr := range lk.rooms[normKey(rn)] {
					if rr.floorID == floorID {
						roomID = rr.id
						hasRoom = true
						break
					}
				}
				if !hasRoom {
					add(colLocation, "lokasi")
				}
			}
		}

		// Purchase Date (optional).
		if v := data[colDate]; v != "" {
			if _, err := time.Parse(dateLayout, v); err != nil {
				add(colDate, "tgl")
			}
		}

		// Purchase Price (optional). Stamp under hargaKey for the worker's sum
		// and the executor's cost (empty -> no cost, contributes 0).
		if v := data[colPrice]; v != "" && !decimalRe.MatchString(v) {
			add(colPrice, "harga")
		}
		data[hargaKey] = data[colPrice]

		// User -> PIC (optional), by NIP. The PIC must belong to the row's
		// office (pemegangScope) — a "Floor - Room" location is already office-
		// scoped, and the PIC is enforced the same way here.
		var picID uuid.UUID
		hasPic := false
		if v := data[colUser]; v != "" {
			if er, ok := lk.employees[normKey(picNIP(v))]; ok {
				if hasOffice && er.officeID != officeID {
					add(colUser, "pemegangScope")
				} else {
					picID = er.id
					hasPic = true
				}
			} else {
				add(colUser, "pemegang")
			}
		}

		// Merk -> brand (optional).
		var brandID uuid.UUID
		hasBrand := false
		if v := data[colBrand]; v != "" {
			if id, ok := lk.brands[normKey(v)]; ok {
				brandID = id
				hasBrand = true
			} else {
				add(colBrand, "merk")
			}
		}

		// Condition (required; empty caught above). Baik -> available,
		// Rusak -> under_maintenance.
		condVal := ""
		if v := data[colCondition]; v != "" {
			cv, ok := condStatus(v)
			if !ok {
				add(colCondition, "kondisi")
			} else {
				condVal = cv
			}
		}

		if hasOffice {
			if !batchSet {
				batchOffice = officeID
				batchSet = true
			}
			data["_office_id"] = officeID.String()
			data["_category_id"] = categoryID.String()
			if hasFloor {
				data["_floor_id"] = floorID.String()
			} else {
				data["_floor_id"] = ""
			}
			if hasRoom {
				data["_room_id"] = roomID.String()
			} else {
				data["_room_id"] = ""
			}
			if hasBrand {
				data["_brand_id"] = brandID.String()
			} else {
				data["_brand_id"] = ""
			}
			if hasPic {
				data["_pic_id"] = picID.String()
			} else {
				data["_pic_id"] = ""
			}
			data["_status"] = condVal
		}

		works[i] = work{data: data, errs: errs, office: officeID, hasOffice: hasOffice}
	}

	// Batch office consistency.
	if batchSet {
		for i := range works {
			if works[i].hasOffice && works[i].office != batchOffice {
				works[i].errs = append(works[i].errs, importer.CellError{Column: colOffice, ErrorKey: "multiOffice"})
			}
		}
	}

	results := make([]importer.RowResult, len(rows))
	for i, w := range works {
		valid := len(w.errs) == 0
		res := importer.RowResult{RowNo: rows[i].RowNo, Valid: valid, Data: w.data, Errors: w.errs}
		if valid {
			res.NormalizedRef = w.office.String()
		} else {
			for _, k := range []string{"_office_id", "_category_id", "_floor_id", "_room_id", "_brand_id", "_pic_id", "_status"} {
				delete(w.data, k)
			}
		}
		results[i] = res
	}
	return results
}

// createRows creates one asset per validated row inside the given transaction,
// reading the resolved ids stamped into each row's Data by validateAssetRows.
// The asset tag is always generated per issuing office (no user-supplied tag).
// maker is recorded as each asset's created_by. Returns the number of assets
// created.
func (a assetImporter) createRows(ctx context.Context, qtx *sqlc.Queries, maker *uuid.UUID, rows []importer.Row) (int, error) {
	created := 0

	markFailed := func(id uuid.UUID, col, key string) error {
		errsJSON, mErr := json.Marshal([]importer.CellError{{Column: col, ErrorKey: key}})
		if mErr != nil {
			return mErr
		}
		return qtx.MarkRowFailed(ctx, sqlc.MarkRowFailedParams{ID: id, Errors: errsJSON})
	}

	for _, r := range rows {
		officeID, err := uuid.Parse(r.Data["_office_id"])
		if err != nil {
			return created, ErrInvalidRef
		}
		categoryID, err := uuid.Parse(r.Data["_category_id"])
		if err != nil {
			return created, ErrInvalidRef
		}

		// Location: Location Name is required, so a valid row always resolved a
		// floor; room is optional (assets may stop at the floor level).
		floorStr := r.Data["_floor_id"]
		floorID, err := common.ParseUUIDPtr(&floorStr)
		if err != nil {
			return created, ErrInvalidRef
		}
		if floorID == nil {
			if fErr := markFailed(r.ID, colLocation, "lokasiRequired"); fErr != nil {
				return created, fErr
			}
			continue
		}
		roomStr := r.Data["_room_id"]
		roomID, err := common.ParseUUIDPtr(&roomStr)
		if err != nil {
			return created, ErrInvalidRef
		}
		brandStr := r.Data["_brand_id"]
		brandID, err := common.ParseUUIDPtr(&brandStr)
		if err != nil {
			return created, ErrInvalidRef
		}
		picStr := r.Data["_pic_id"]
		picID, err := common.ParseUUIDPtr(&picStr)
		if err != nil {
			return created, ErrInvalidRef
		}

		// Purchase Date (optional; zero pgtype.Date is NULL).
		var purchaseDate pgtype.Date
		year := int32(time.Now().Year())
		if ds := trim(r.Data[colDate]); ds != "" {
			pd, derr := parsePurchaseDate(&ds)
			if derr != nil {
				return created, fmt.Errorf("invalid %s: %w", colDate, derr)
			}
			purchaseDate = pd
			if pd.Valid {
				year = int32(pd.Time.Year())
			}
		}

		// Purchase Price (optional) -> nullable cost.
		var cost *string
		if h := trim(r.Data[hargaKey]); h != "" {
			cost = &h
		}

		tag, tagSeq, err := a.s.GenerateAssetTag(ctx, qtx, officeID, categoryID, year)
		if err != nil {
			return created, mapDBError(err)
		}

		createdAsset, err := qtx.CreateAsset(ctx, sqlc.CreateAssetParams{
			AssetTag:        tag,
			TagSeq:          &tagSeq,
			Name:            r.Data[colName],
			CategoryID:      categoryID,
			OfficeID:        officeID,
			FloorID:         floorID,
			RoomID:          roomID,
			VendorID:        nil,
			BrandID:         brandID,
			PicEmployeeID:   picID,
			Capacity:        optText(r.Data[colCapacity]),
			SpkNumber:       optText(r.Data[colSpk]),
			LegacyAssetCode: optText(r.Data[colAssetCode]),
			LegacyBarcode:   optText(r.Data[colBarcode]),
			AssetClass:      sqlc.SharedAssetClassTangible,
			Capitalized:     true,
			CreatedByID:     maker,
			PurchaseCost:    cost,
			PurchaseDate:    purchaseDate,
			Specifications:  []byte("{}"),
		})
		if err != nil {
			return created, mapDBError(err)
		}

		// Seed the location-history timeline (source=registration) — every
		// create path must, so an asset's location timeline is never empty.
		if hErr := qtx.InsertAssetLocationHistory(ctx, sqlc.InsertAssetLocationHistoryParams{
			AssetID:   createdAsset.ID,
			OfficeID:  createdAsset.OfficeID,
			FloorID:   createdAsset.FloorID,
			RoomID:    createdAsset.RoomID,
			Source:    sqlc.SharedLocationChangeSourceRegistration,
			MovedByID: maker,
		}); hErr != nil {
			return created, mapDBError(hErr)
		}

		// PIC history: seed the assignment timeline when a User was resolved.
		if picID != nil {
			if hErr := qtx.InsertAssetPICHistory(ctx, sqlc.InsertAssetPICHistoryParams{
				AssetID:       createdAsset.ID,
				PicEmployeeID: *picID,
				AssignedByID:  maker,
			}); hErr != nil {
				return created, mapDBError(hErr)
			}
		}

		// Condition: CreateAsset inserts 'available'; override for non-available
		// statuses (e.g. Rusak -> under_maintenance).
		if st := r.Data["_status"]; st != "" && st != string(sqlc.SharedAssetStatusAvailable) {
			if _, sErr := qtx.SetAssetStatus(ctx, sqlc.SetAssetStatusParams{
				ID:     createdAsset.ID,
				Status: sqlc.SharedAssetStatus(st),
			}); sErr != nil {
				return created, mapDBError(sErr)
			}
		}

		if err := qtx.MarkRowResult(ctx, sqlc.MarkRowResultParams{ID: r.ID, ResultRef: &tag}); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// Execute satisfies importer.TargetImporter. The asset target always requires
// approval (NeedsApproval() == true), so the generic worker never invokes this
// directly — asset imports are executed by the asset_import approval executor
// (see ImportExecutor), which calls createRows with the import job's maker.
func (a assetImporter) Execute(ctx context.Context, qtx *sqlc.Queries, job importer.Job, validRows []importer.Row) (int, error) {
	return a.createRows(ctx, qtx, nil, validRows)
}

// containsUUID reports whether id is present in ids.
func containsUUID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// trim is a short alias for strings.TrimSpace used throughout row parsing.
func trim(s string) string { return strings.TrimSpace(s) }
