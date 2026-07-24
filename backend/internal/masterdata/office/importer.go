package office

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/importer"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Office import column names.
const (
	colCode  = "kode"
	colName  = "nama"
	colTipe  = "tipe"
	colInduk = "induk"
	colAktif = "aktif"
)

// importLookupLimit bounds the office/office-type lookup page. Office volume
// is reference-scale (a bank's org tree), so a single generous page is
// sufficient. Mirrors internal/masterdata/employee/importer.go.
const importLookupLimit = 100000

// activeLiterals are the accepted (lower-cased) boolean literals for the
// "aktif" column, matched case-insensitively. An empty cell defaults to
// active (true) — see validateOfficeRows.
var activeLiterals = map[string]bool{
	"true":  true,
	"1":     true,
	"ya":    true,
	"false": false,
	"0":     false,
	"tidak": false,
}

// officeLookups holds the case-insensitive lookup maps the office importer
// validates rows against. Keys are lower-cased. Built once per batch by
// buildOfficeLookups; consumed by the pure validateOfficeRows.
type officeLookups struct {
	// officeTypes maps a lower-cased office_type name to its id. office_types
	// has no code column (only name; see db/migrations/000006_masterdata.up.sql),
	// so the "tipe" column matches by name only.
	officeTypes map[string]uuid.UUID
	// parentOffices maps a lower-cased office code to its id, spanning ALL
	// offices regardless of the caller's scope. Using an unscoped set lets
	// validateOfficeRows distinguish "parent not found" (induk) from
	// "parent found but out of scope" (scope) as separate error_keys.
	parentOffices map[string]uuid.UUID
	// existingCodes is the set of all existing (non-deleted) office codes,
	// lower-cased, used to detect DB collisions with user-supplied kode
	// values during validation. Office codes are globally unique
	// (uq_offices_code), so this set is deliberately unscoped.
	existingCodes map[string]bool
}

// officeImporter is the office import target: it validates a batch of office
// rows and creates them directly (no approval). It implements
// importer.TargetImporter.
type officeImporter struct{ s *Service }

// Importer returns the office import target for registration with the
// generic import engine.
func (s *Service) Importer() importer.TargetImporter { return officeImporter{s} }

// Target returns the importer's registry key.
func (officeImporter) Target() string { return "office" }

// NeedsApproval reports that office (master-data) imports are created
// directly by the worker, without maker-checker approval.
func (officeImporter) NeedsApproval() bool { return false }

// Columns describes the office import template.
func (officeImporter) Columns() []importer.ColumnSpec {
	return []importer.ColumnSpec{
		{Name: colCode, Required: true, Kind: "text"},
		{Name: colName, Required: true, Kind: "text"},
		{Name: colTipe, Required: true, Kind: "lookup"},
		{Name: colInduk, Required: false, Kind: "lookup"},
		{Name: colAktif, Required: false, Kind: "text"},
	}
}

// ValidateRows loads the lookup sets (office types, all offices for the
// parent lookup + code-collision set), then runs the pure row validation.
// Splitting the DB step (buildOfficeLookups) from the pure step
// (validateOfficeRows) keeps the business rules unit-testable without a
// database — mirrors internal/masterdata/employee/importer.go.
func (o officeImporter) ValidateRows(ctx context.Context, rows []importer.RawRow, scope importer.Scope) ([]importer.RowResult, error) {
	lk, err := o.buildOfficeLookups(ctx)
	if err != nil {
		return nil, err
	}
	return validateOfficeRows(rows, lk, scope), nil
}

// buildOfficeLookups loads office types and ALL offices (unscoped — see
// officeLookups.parentOffices) into case-insensitive lookup maps.
func (o officeImporter) buildOfficeLookups(ctx context.Context) (officeLookups, error) {
	lk := officeLookups{
		officeTypes:   map[string]uuid.UUID{},
		parentOffices: map[string]uuid.UUID{},
		existingCodes: map[string]bool{},
	}

	types, err := o.s.q.ListOfficeTypesLookup(ctx)
	if err != nil {
		return lk, err
	}
	for _, t := range types {
		addKey(lk.officeTypes, t.Name, t.ID)
	}

	// Deliberately unscoped (AllScope: true) — see officeLookups.parentOffices.
	offs, err := o.s.q.ListOffices(ctx, sqlc.ListOfficesParams{
		AllScope:  true,
		OfficeIds: nil,
		Search:    "",
		Lim:       importLookupLimit,
		Off:       0,
	})
	if err != nil {
		return lk, err
	}
	for _, of := range offs {
		addKey(lk.parentOffices, of.Code, of.ID)
		if k := normCode(of.Code); k != "" {
			lk.existingCodes[k] = true
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

// normCode lower-cases and trims an office code for case-insensitive
// duplicate detection.
func normCode(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// trim is a short alias for strings.TrimSpace used throughout row parsing.
func trim(s string) string { return strings.TrimSpace(s) }

// validateOfficeRows validates raw office rows against pre-loaded lookups and
// the caller's data scope, returning one RowResult per input row (same
// order). It performs NO database access: all resolution is against lk, so
// the full rule set is unit-testable with hand-built lookups.
//
// A scoped caller (per office/service.go's Create) may only place an office
// under a parent within their own scope — so an empty "induk" is itself a
// scope failure for a scoped caller (they cannot create a root office). Only
// an AllScope (global) caller may create a root office. A valid row's
// resolved office_type id (and parent id, if any) is stamped into Data under
// "_office_type_id" / "_parent_id" for Execute to consume without
// re-resolving. NormalizedRef is left empty: office imports never need
// approval routing (NeedsApproval() is false).
func validateOfficeRows(rows []importer.RawRow, lk officeLookups, scope importer.Scope) []importer.RowResult {
	results := make([]importer.RowResult, len(rows))
	seenCodes := map[string]bool{}

	for i, raw := range rows {
		data := map[string]string{
			colCode:  trim(raw.Cells[colCode]),
			colName:  trim(raw.Cells[colName]),
			colTipe:  trim(raw.Cells[colTipe]),
			colInduk: trim(raw.Cells[colInduk]),
			colAktif: trim(raw.Cells[colAktif]),
		}
		var errs []importer.CellError
		add := func(col, key string) { errs = append(errs, importer.CellError{Column: col, ErrorKey: key}) }

		// Required columns.
		for _, col := range []string{colCode, colName, colTipe} {
			if data[col] == "" {
				add(col, "required")
			}
		}

		// tipe.
		var officeTypeID uuid.UUID
		hasType := false
		if v := data[colTipe]; v != "" {
			if id, ok := lk.officeTypes[normKey(v)]; ok {
				officeTypeID = id
				hasType = true
			} else {
				add(colTipe, "tipe")
			}
		}

		// induk (optional). A scoped caller may only create an office under a
		// parent within their scope; an empty induk means "create a root
		// office", which only an AllScope caller may do (mirrors
		// office/service.go's Create: nil ParentID + !all -> ErrParentOutOfScope).
		var parentID uuid.UUID
		hasParent := false
		if v := data[colInduk]; v != "" {
			if id, ok := lk.parentOffices[normKey(v)]; ok {
				parentID = id
				hasParent = true
			} else {
				add(colInduk, "induk")
			}
			if hasParent && !common.InScope(scope.AllScope, scope.OfficeIDs, parentID) {
				add(colInduk, "scope")
			}
		} else if !scope.AllScope {
			add(colInduk, "scope")
		}

		// aktif (optional, defaults to active/true when empty). The parsed
		// value itself isn't stamped: Execute re-derives it from the raw cell
		// via parseActive once the row is guaranteed valid.
		if v := data[colAktif]; v != "" {
			if _, ok := activeLiterals[strings.ToLower(v)]; !ok {
				add(colAktif, "aktif")
			}
		}

		// kode: not a duplicate within this file, not already in DB (both
		// case-insensitive).
		if v := data[colCode]; v != "" {
			key := normCode(v)
			switch {
			case lk.existingCodes[key]:
				add(colCode, "dupKode")
			case seenCodes[key]:
				add(colCode, "dupKode")
			default:
				seenCodes[key] = true
			}
		}

		if hasType {
			data["_office_type_id"] = officeTypeID.String()
		}
		if hasParent {
			data["_parent_id"] = parentID.String()
		}

		valid := len(errs) == 0
		res := importer.RowResult{
			RowNo:  raw.RowNo,
			Valid:  valid,
			Data:   data,
			Errors: errs,
			// Office imports have no batch-office rule and never need
			// approval routing, so NormalizedRef is deliberately left empty.
		}
		if !valid {
			// Drop internal resolution stamps from invalid rows to keep their
			// persisted data clean (they never reach the executor).
			delete(res.Data, "_office_type_id")
			delete(res.Data, "_parent_id")
		}
		results[i] = res
	}

	return results
}

// parseActive parses the "aktif" cell into a boolean, defaulting to true
// (active) when empty. The value is guaranteed to be a recognized literal or
// empty by validateOfficeRows before a row reaches Execute.
func parseActive(s string) bool {
	if s == "" {
		return true
	}
	return activeLiterals[strings.ToLower(s)]
}

// Execute creates one office per validated row, inside the worker's single
// execute transaction (office imports have NeedsApproval() == false, so the
// generic worker calls this directly — see executePhase in worker.go).
//
// TX-POISONING DEFENSE: exactly like internal/masterdata/employee/importer.go's
// Execute, this runs inside ONE shared transaction for the whole batch. In
// PostgreSQL a unique-violation (23505) POISONS the whole transaction — every
// subsequent command fails with 25P02 "current transaction is aborted" — so a
// single `kode` collision at CreateOffice time would abort and roll back the
// ENTIRE office batch instead of just that one row. To keep the "fail one row,
// continue" design working, we make CreateOffice never fire a 23505 in the
// common cases: before every insert we pre-check the code's availability
// (against codes consumed earlier in THIS batch and against the DB) so a
// taken code is skipped as a failed row rather than inserted. GetOfficeByCode
// is a side-effect-free SELECT; returning pgx.ErrNoRows does NOT poison the tx.
func (o officeImporter) Execute(ctx context.Context, qtx *sqlc.Queries, job importer.Job, validRows []importer.Row) (int, error) {
	created := 0
	// Codes consumed by earlier rows in THIS execution (lower-cased). Guards
	// against two rows in the same batch resolving to the same code before
	// either is visible to a DB read.
	usedCodes := map[string]bool{}

	markFailed := func(id uuid.UUID) error {
		errsJSON, mErr := json.Marshal([]importer.CellError{{Column: colCode, ErrorKey: "dupKode"}})
		if mErr != nil {
			return mErr
		}
		return qtx.MarkRowFailed(ctx, sqlc.MarkRowFailedParams{ID: id, Errors: errsJSON})
	}

	for _, r := range validRows {
		officeTypeID, err := uuid.Parse(r.Data["_office_type_id"])
		if err != nil {
			return created, common.ErrInvalidReference
		}

		var parentID *uuid.UUID
		if v := r.Data["_parent_id"]; v != "" {
			id, pErr := uuid.Parse(v)
			if pErr != nil {
				return created, common.ErrInvalidReference
			}
			parentID = &id
		}

		code := trim(r.Data[colCode])
		codeKey := normCode(code)

		// Pre-check availability BEFORE inserting, so CreateOffice is never
		// called with a taken code (no 23505 is triggered, no tx poisoning).
		if usedCodes[codeKey] {
			// Collides with an earlier row in this same batch.
			if fErr := markFailed(r.ID); fErr != nil {
				return created, fErr
			}
			continue
		}
		// Fresh DB existence check: catches a code that became taken since
		// validation (TOCTOU) or one already committed by a prior batch.
		if _, gErr := qtx.GetOfficeByCode(ctx, code); gErr == nil {
			// A row already exists for this code — skip it as failed.
			if fErr := markFailed(r.ID); fErr != nil {
				return created, fErr
			}
			continue
		} else if !errors.Is(gErr, pgx.ErrNoRows) {
			// Any error other than "no rows" is a real DB error.
			return created, common.MapDBError(gErr)
		}

		off, err := qtx.CreateOffice(ctx, sqlc.CreateOfficeParams{
			ParentID:     parentID,
			OfficeTypeID: officeTypeID,
			ProvinceID:   nil,
			CityID:       nil,
			Name:         r.Data[colName],
			Code:         code,
			Address:      nil,
			IsActive:     parseActive(trim(r.Data[colAktif])),
			Latitude:     nil,
			Longitude:    nil,
			// office_kind is NOT NULL (legacy-parity Fase 5); the import template
			// carries no such column, so fall back to the same default the service
			// applies. Leaving the zero value would send '' and fail the enum cast.
			OfficeKind: defaultOfficeKind(""),
		})
		if err != nil {
			// Residual concurrent-race window: the pre-check saw the code
			// free, but a genuinely simultaneous, still-uncommitted INSERT of
			// the same explicit code in another transaction can surface as
			// 23505 here. We do NOT swallow-and-continue on this 23505: doing
			// so inside the shared tx would already be poisoned. Instead we
			// return the error, aborting THIS execute attempt cleanly — the
			// worker fails the job (see executePhase's failJob path). Because
			// the pre-check is self-healing, a retry sees the now-committed
			// code via GetOfficeByCode and skips that row.
			return created, common.MapDBError(err)
		}

		usedCodes[codeKey] = true
		if err := qtx.MarkRowResult(ctx, sqlc.MarkRowResultParams{ID: r.ID, ResultRef: &off.Code}); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}
