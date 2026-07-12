package asset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/importer"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Asset import column names. `harga` is fixed — the generic worker sums this
// column across the batch to compute the maker-checker approval amount.
const (
	colTag      = "asset_tag"
	colName     = "nama"
	colCategory = "kategori"
	colOffice   = "kantor"
	colDate     = "tgl_beli"
	colPrice    = "harga"
	colVendor   = "vendor"
	colRoom     = "lokasi"
)

// importLookupLimit bounds the office lookup page. Office volume is reference-
// scale (a bank's org tree), so a single generous page is sufficient.
const importLookupLimit = 100000

// dateLayout is the accepted purchase-date format for imported rows.
const dateLayout = "2006-01-02"

// decimalRe matches a non-negative plain decimal (no sign, no scientific
// notation, no fraction) — the accepted form for the `harga` column.
var decimalRe = regexp.MustCompile(`^\d+(\.\d+)?$`)

// tagRe matches an acceptable user-supplied asset_tag: an alphanumeric start
// followed by alphanumerics or the separators . _ / - , up to 64 chars total.
var tagRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,63}$`)

// roomRef is a room's id together with the office it belongs to (resolved via
// its floor), used to enforce that a row's `lokasi` names a room in that row's
// resolved office.
type roomRef struct {
	id       uuid.UUID
	officeID uuid.UUID
}

// assetLookups holds the case-insensitive name/code -> id maps the asset
// importer validates rows against. Keys are lower-cased; existingTags is keyed
// by upper-cased tag (asset tags are upper-case codes). Built once per batch by
// buildAssetLookups; consumed by the pure validateAssetRows.
type assetLookups struct {
	categories   map[string]uuid.UUID
	offices      map[string]uuid.UUID
	vendors      map[string]uuid.UUID
	rooms        map[string][]roomRef
	existingTags map[string]bool
}

// assetImporter is the asset import target: it validates a batch of asset rows
// and, once its maker-checker approval is granted, creates the assets. It
// implements importer.TargetImporter.
type assetImporter struct{ s *Service }

// Importer returns the asset import target for registration with the generic
// import engine.
func (s *Service) Importer() importer.TargetImporter { return assetImporter{s} }

// Target returns the importer's registry key.
func (assetImporter) Target() string { return "asset" }

// NeedsApproval reports that asset imports must clear the value-tiered
// maker-checker approval flow before the rows are created.
func (assetImporter) NeedsApproval() bool { return true }

// Columns describes the asset import template. `harga` is required and fixed —
// the worker sums it for the batch approval amount.
func (assetImporter) Columns() []importer.ColumnSpec {
	return []importer.ColumnSpec{
		{Name: colTag, Required: false, Kind: "text"},
		{Name: colName, Required: true, Kind: "text"},
		{Name: colCategory, Required: true, Kind: "lookup"},
		{Name: colOffice, Required: true, Kind: "lookup"},
		{Name: colDate, Required: true, Kind: "date"},
		{Name: colPrice, Required: true, Kind: "decimal"},
		{Name: colVendor, Required: false, Kind: "lookup"},
		{Name: colRoom, Required: false, Kind: "lookup"},
	}
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

// buildAssetLookups loads categories, offices, rooms, vendors, and existing
// asset tags into case-insensitive lookup maps. Offices and rooms are loaded
// scoped to the caller (all_scope / office_ids); categories, vendors, and the
// existing-tag set are global (tags are globally unique).
func (a assetImporter) buildAssetLookups(ctx context.Context, scope importer.Scope) (assetLookups, error) {
	lk := assetLookups{
		categories:   map[string]uuid.UUID{},
		offices:      map[string]uuid.UUID{},
		vendors:      map[string]uuid.UUID{},
		rooms:        map[string][]roomRef{},
		existingTags: map[string]bool{},
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
		AllScope:  scope.AllScope,
		OfficeIds: scope.OfficeIDs,
		Search:    "",
		Lim:       importLookupLimit,
		Off:       0,
	})
	if err != nil {
		return lk, err
	}
	for _, o := range offs {
		addKey(lk.offices, o.Name, o.ID)
		addKey(lk.offices, o.Code, o.ID)
	}

	rooms, err := a.s.q.ListRoomsLookup(ctx, sqlc.ListRoomsLookupParams{
		AllScope:  scope.AllScope,
		OfficeIds: scope.OfficeIDs,
	})
	if err != nil {
		return lk, err
	}
	for _, r := range rooms {
		addRoom(lk.rooms, r.Name, roomRef{id: r.ID, officeID: r.OfficeID})
		if r.Code != nil {
			addRoom(lk.rooms, *r.Code, roomRef{id: r.ID, officeID: r.OfficeID})
		}
	}

	vends, err := a.s.q.ListVendorsLookup(ctx)
	if err != nil {
		return lk, err
	}
	for _, v := range vends {
		addKey(lk.vendors, v.Name, v.ID)
	}

	tags, err := a.s.q.ListAssetTags(ctx)
	if err != nil {
		return lk, err
	}
	for _, t := range tags {
		if k := normTag(t); k != "" {
			lk.existingTags[k] = true
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

// addRoom appends a room reference under a lower-cased, trimmed name/code key.
func addRoom(m map[string][]roomRef, name string, ref roomRef) {
	if k := normKey(name); k != "" {
		m[k] = append(m[k], ref)
	}
}

// normKey lower-cases and trims a lookup key.
func normKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// normTag upper-cases and trims an asset tag for case-insensitive comparison.
func normTag(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// validateAssetRows validates raw asset rows against pre-loaded lookups and the
// caller's data scope, returning one RowResult per input row (same order). It
// performs NO database access: all resolution is against lk, so the full rule
// set is unit-testable with hand-built lookups.
//
// Batch rules: every row that resolves an office must resolve the SAME office
// (the first resolved one wins; differing rows get "multiOffice"). A valid
// row's NormalizedRef carries the resolved office UUID string so the worker can
// route the batch's approval; the resolved category/office/vendor/room ids are
// stamped into Data under "_"-prefixed keys for the executor to consume without
// re-resolving (avoiding drift across the approval window).
func validateAssetRows(rows []importer.RawRow, lk assetLookups, scope importer.Scope) []importer.RowResult {
	type work struct {
		data      map[string]string
		errs      []importer.CellError
		office    uuid.UUID
		hasOffice bool
	}

	works := make([]work, len(rows))
	seenTags := map[string]bool{}
	var batchOffice uuid.UUID
	batchSet := false

	for i, raw := range rows {
		data := map[string]string{
			colTag:      trim(raw.Cells[colTag]),
			colName:     trim(raw.Cells[colName]),
			colCategory: trim(raw.Cells[colCategory]),
			colOffice:   trim(raw.Cells[colOffice]),
			colDate:     trim(raw.Cells[colDate]),
			colPrice:    trim(raw.Cells[colPrice]),
			colVendor:   trim(raw.Cells[colVendor]),
			colRoom:     trim(raw.Cells[colRoom]),
		}
		var errs []importer.CellError
		add := func(col, key string) { errs = append(errs, importer.CellError{Column: col, ErrorKey: key}) }

		// Required columns.
		for _, col := range []string{colName, colCategory, colOffice, colDate, colPrice} {
			if data[col] == "" {
				add(col, "required")
			}
		}

		// kategori.
		var categoryID uuid.UUID
		if v := data[colCategory]; v != "" {
			if id, ok := lk.categories[normKey(v)]; ok {
				categoryID = id
			} else {
				add(colCategory, "kat")
			}
		}

		// kantor.
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

		// tgl_beli.
		if v := data[colDate]; v != "" {
			if _, err := time.Parse(dateLayout, v); err != nil {
				add(colDate, "tgl")
			}
		}

		// harga.
		if v := data[colPrice]; v != "" {
			if !decimalRe.MatchString(v) {
				add(colPrice, "harga")
			}
		}

		// vendor (optional).
		var vendorID uuid.UUID
		hasVendor := false
		if v := data[colVendor]; v != "" {
			if id, ok := lk.vendors[normKey(v)]; ok {
				vendorID = id
				hasVendor = true
			} else {
				add(colVendor, "vendor")
			}
		}

		// lokasi (optional): must be a room in this row's resolved office.
		var roomID uuid.UUID
		hasRoom := false
		if v := data[colRoom]; v != "" {
			for _, rr := range lk.rooms[normKey(v)] {
				if hasOffice && rr.officeID == officeID {
					roomID = rr.id
					hasRoom = true
					break
				}
			}
			if !hasRoom {
				add(colRoom, "lokasi")
			}
		}

		// asset_tag (optional): valid format, not already in DB, not a
		// duplicate within this file (all case-insensitive).
		if v := data[colTag]; v != "" {
			key := normTag(v)
			switch {
			case !tagRe.MatchString(v):
				add(colTag, "dupTag")
			case lk.existingTags[key]:
				add(colTag, "dupTag")
			case seenTags[key]:
				add(colTag, "dupTag")
			default:
				seenTags[key] = true
			}
		}

		if hasOffice {
			if !batchSet {
				batchOffice = officeID
				batchSet = true
			}
			// Stash resolved optional ids for the finalize pass.
			if hasVendor {
				data["_vendor_id"] = vendorID.String()
			} else {
				data["_vendor_id"] = ""
			}
			if hasRoom {
				data["_room_id"] = roomID.String()
			} else {
				data["_room_id"] = ""
			}
			data["_category_id"] = categoryID.String()
			data["_office_id"] = officeID.String()
		}

		works[i] = work{data: data, errs: errs, office: officeID, hasOffice: hasOffice}
	}

	// Batch office consistency: flag rows whose resolved office differs from the
	// first resolved office.
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
		res := importer.RowResult{
			RowNo:  rows[i].RowNo,
			Valid:  valid,
			Data:   w.data,
			Errors: w.errs,
		}
		if valid {
			// Valid rows always resolved an in-scope office; expose it for the
			// worker to route the approval.
			res.NormalizedRef = w.office.String()
		} else {
			// Drop internal resolution stamps from invalid rows to keep their
			// persisted data clean (they never reach the executor).
			delete(w.data, "_category_id")
			delete(w.data, "_office_id")
			delete(w.data, "_vendor_id")
			delete(w.data, "_room_id")
		}
		results[i] = res
	}
	return results
}

// createRows creates one asset per validated row inside the given transaction,
// reading the resolved ids stamped into each row's Data by validateAssetRows.
// maker is recorded as each asset's created_by. A unique-tag conflict marks
// just that row failed (dupTag) and continues — a late collision must not abort
// the whole approved batch. Returns the number of assets created.
func (a assetImporter) createRows(ctx context.Context, qtx *sqlc.Queries, maker *uuid.UUID, rows []importer.Row) (int, error) {
	created := 0
	for _, r := range rows {
		officeID, err := uuid.Parse(r.Data["_office_id"])
		if err != nil {
			return created, ErrInvalidRef
		}
		categoryID, err := uuid.Parse(r.Data["_category_id"])
		if err != nil {
			return created, ErrInvalidRef
		}
		roomStr := r.Data["_room_id"]
		roomID, err := common.ParseUUIDPtr(&roomStr)
		if err != nil {
			return created, ErrInvalidRef
		}
		vendorStr := r.Data["_vendor_id"]
		vendorID, err := common.ParseUUIDPtr(&vendorStr)
		if err != nil {
			return created, ErrInvalidRef
		}

		dateStr := r.Data[colDate]
		purchaseDate, derr := parsePurchaseDate(&dateStr)
		if derr != nil {
			return created, fmt.Errorf("invalid %s: %w", colDate, derr)
		}
		year := int32(time.Now().Year())
		if purchaseDate.Valid {
			year = int32(purchaseDate.Time.Year())
		}

		tag := trim(r.Data[colTag])
		if tag == "" {
			tag, err = a.s.GenerateAssetTag(ctx, qtx, officeID, categoryID, year)
			if err != nil {
				return created, mapDBError(err)
			}
		}

		harga := r.Data[colPrice]
		_, err = qtx.CreateAsset(ctx, sqlc.CreateAssetParams{
			AssetTag:       tag,
			Name:           r.Data[colName],
			CategoryID:     categoryID,
			OfficeID:       officeID,
			RoomID:         roomID,
			VendorID:       vendorID,
			AssetClass:     sqlc.SharedAssetClassTangible,
			Capitalized:    true,
			CreatedByID:    maker,
			PurchaseCost:   &harga,
			PurchaseDate:   purchaseDate,
			Specifications: []byte("{}"),
		})
		if err != nil {
			if errors.Is(mapDBError(err), ErrConflict) {
				// Late unique-tag collision — fail just this row, keep going.
				errsJSON, mErr := json.Marshal([]importer.CellError{{Column: colTag, ErrorKey: "dupTag"}})
				if mErr != nil {
					return created, mErr
				}
				if fErr := qtx.MarkRowFailed(ctx, sqlc.MarkRowFailedParams{ID: r.ID, Errors: errsJSON}); fErr != nil {
					return created, fErr
				}
				continue
			}
			return created, mapDBError(err)
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
// This delegation is kept for interface completeness; maker is unset here.
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
