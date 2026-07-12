package employee

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

// Employee import column names.
const (
	colCode   = "kode"
	colName   = "nama"
	colEmail  = "email"
	colPhone  = "telepon"
	colOffice = "kantor"
	colStatus = "status"

	colDepartment = "departemen"
	colPosition   = "jabatan"
)

// importLookupLimit bounds the office lookup page. Office volume is
// reference-scale (a bank's org tree), so a single generous page is
// sufficient. Mirrors internal/asset/importer.go.
const importLookupLimit = 100000

// validStatuses are the accepted (lower-cased) SharedUserStatus enum labels
// for the "status" column, matched case-insensitively.
var validStatuses = map[string]sqlc.SharedUserStatus{
	string(sqlc.SharedUserStatusActive):    sqlc.SharedUserStatusActive,
	string(sqlc.SharedUserStatusInactive):  sqlc.SharedUserStatusInactive,
	string(sqlc.SharedUserStatusSuspended): sqlc.SharedUserStatusSuspended,
}

// employeeLookups holds the case-insensitive lookup maps the employee
// importer validates rows against. Keys are lower-cased. Built once per batch
// by buildEmployeeLookups; consumed by the pure validateEmployeeRows.
type employeeLookups struct {
	offices       map[string]uuid.UUID
	existingCodes map[string]bool
	departments   map[string]uuid.UUID
	positions     map[string]uuid.UUID
}

// employeeImporter is the employee import target: it validates a batch of
// employee rows and creates them directly (no approval). It implements
// importer.TargetImporter.
type employeeImporter struct{ s *Service }

// Importer returns the employee import target for registration with the
// generic import engine.
func (s *Service) Importer() importer.TargetImporter { return employeeImporter{s} }

// Target returns the importer's registry key.
func (employeeImporter) Target() string { return "employee" }

// NeedsApproval reports that employee (master-data) imports are created
// directly by the worker, without maker-checker approval.
func (employeeImporter) NeedsApproval() bool { return false }

// Columns describes the employee import template.
func (employeeImporter) Columns() []importer.ColumnSpec {
	return []importer.ColumnSpec{
		{Name: colCode, Required: true, Kind: "text"},
		{Name: colName, Required: true, Kind: "text"},
		{Name: colEmail, Required: false, Kind: "text"},
		{Name: colPhone, Required: false, Kind: "text"},
		{Name: colOffice, Required: true, Kind: "lookup"},
		{Name: colStatus, Required: true, Kind: "text"},
		{Name: colDepartment, Required: false, Kind: "lookup"},
		{Name: colPosition, Required: false, Kind: "lookup"},
	}
}

// ValidateRows loads the lookup sets scoped to the caller, then runs the pure
// row validation. Splitting the DB step (buildEmployeeLookups) from the pure
// step (validateEmployeeRows) keeps the business rules unit-testable without
// a database — mirrors internal/asset/importer.go.
func (e employeeImporter) ValidateRows(ctx context.Context, rows []importer.RawRow, scope importer.Scope) ([]importer.RowResult, error) {
	lk, err := e.buildEmployeeLookups(ctx, scope)
	if err != nil {
		return nil, err
	}
	return validateEmployeeRows(rows, lk, scope), nil
}

// buildEmployeeLookups loads offices (scoped to the caller) and existing
// employee codes (global — codes are globally unique, see
// uq_employees_code) into case-insensitive lookup maps.
func (e employeeImporter) buildEmployeeLookups(ctx context.Context, scope importer.Scope) (employeeLookups, error) {
	lk := employeeLookups{
		offices:       map[string]uuid.UUID{},
		existingCodes: map[string]bool{},
		departments:   map[string]uuid.UUID{},
		positions:     map[string]uuid.UUID{},
	}

	offs, err := e.s.q.ListOffices(ctx, sqlc.ListOfficesParams{
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

	codes, err := e.s.q.ListEmployeeCodes(ctx)
	if err != nil {
		return lk, err
	}
	for _, c := range codes {
		if k := normCode(c); k != "" {
			lk.existingCodes[k] = true
		}
	}

	depts, err := e.s.q.ListDepartmentsLookup(ctx)
	if err != nil {
		return lk, err
	}
	for _, d := range depts {
		addKey(lk.departments, d.Name, d.ID)
		if d.Code != nil {
			addKey(lk.departments, *d.Code, d.ID)
		}
	}
	positions, err := e.s.q.ListPositionsLookup(ctx)
	if err != nil {
		return lk, err
	}
	for _, p := range positions {
		addKey(lk.positions, p.Name, p.ID)
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

// normCode lower-cases and trims an employee code for case-insensitive
// duplicate detection.
func normCode(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// trim is a short alias for strings.TrimSpace used throughout row parsing.
func trim(s string) string { return strings.TrimSpace(s) }

// isPlausibleEmail reports whether s looks like a syntactically plausible
// email address: it contains exactly an "@" with a non-empty local part, and
// the domain part contains a "." that is neither the first nor last
// character (per the task brief: "contains @ and a dot in the domain; keep
// it simple").
func isPlausibleEmail(s string) bool {
	at := strings.Index(s, "@")
	if at <= 0 || at == len(s)-1 {
		return false
	}
	local, domain := s[:at], s[at+1:]
	if strings.ContainsAny(local, " @") {
		return false
	}
	dot := strings.Index(domain, ".")
	if dot <= 0 || dot == len(domain)-1 {
		return false
	}
	return !strings.ContainsAny(domain, " @")
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

// validateEmployeeRows validates raw employee rows against pre-loaded lookups
// and the caller's data scope, returning one RowResult per input row (same
// order). It performs NO database access: all resolution is against lk, so
// the full rule set is unit-testable with hand-built lookups.
//
// Unlike the asset importer, employees have no batch-office rule: each row's
// office is resolved and scope-checked independently, and NormalizedRef is
// left empty (employee imports never need approval routing — NeedsApproval()
// is false). A valid row's resolved office id is stamped into Data under
// "_office_id" for Execute to consume without re-resolving.
func validateEmployeeRows(rows []importer.RawRow, lk employeeLookups, scope importer.Scope) []importer.RowResult {
	results := make([]importer.RowResult, len(rows))
	seenCodes := map[string]bool{}

	for i, raw := range rows {
		data := map[string]string{
			colCode:       trim(raw.Cells[colCode]),
			colName:       trim(raw.Cells[colName]),
			colEmail:      trim(raw.Cells[colEmail]),
			colPhone:      trim(raw.Cells[colPhone]),
			colOffice:     trim(raw.Cells[colOffice]),
			colStatus:     trim(raw.Cells[colStatus]),
			colDepartment: trim(raw.Cells[colDepartment]),
			colPosition:   trim(raw.Cells[colPosition]),
		}
		var errs []importer.CellError
		add := func(col, key string) { errs = append(errs, importer.CellError{Column: col, ErrorKey: key}) }

		// Required columns.
		for _, col := range []string{colCode, colName, colOffice, colStatus} {
			if data[col] == "" {
				add(col, "required")
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

		// status.
		if v := data[colStatus]; v != "" {
			if _, ok := validStatuses[strings.ToLower(v)]; !ok {
				add(colStatus, "status")
			}
		}

		// email (optional).
		if v := data[colEmail]; v != "" {
			if !isPlausibleEmail(v) {
				add(colEmail, "email")
			}
		}

		// departemen (optional): resolve by name OR code.
		var departmentID uuid.UUID
		hasDept := false
		if v := data[colDepartment]; v != "" {
			if id, ok := lk.departments[normKey(v)]; ok {
				departmentID = id
				hasDept = true
			} else {
				add(colDepartment, "departemen")
			}
		}
		// jabatan (optional): resolve by name.
		var positionID uuid.UUID
		hasPos := false
		if v := data[colPosition]; v != "" {
			if id, ok := lk.positions[normKey(v)]; ok {
				positionID = id
				hasPos = true
			} else {
				add(colPosition, "jabatan")
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

		if hasOffice {
			data["_office_id"] = officeID.String()
		}
		if hasDept {
			data["_department_id"] = departmentID.String()
		}
		if hasPos {
			data["_position_id"] = positionID.String()
		}

		valid := len(errs) == 0
		res := importer.RowResult{
			RowNo:  raw.RowNo,
			Valid:  valid,
			Data:   data,
			Errors: errs,
			// Employees have no batch-office rule and never need approval
			// routing, so NormalizedRef is deliberately left empty.
		}
		if !valid {
			// Drop internal resolution stamps from invalid rows to keep their
			// persisted data clean (they never reach the executor).
			delete(res.Data, "_office_id")
			delete(res.Data, "_department_id")
			delete(res.Data, "_position_id")
		}
		results[i] = res
	}

	return results
}

// Execute creates one employee per validated row, inside the worker's single
// execute transaction (employee imports have NeedsApproval() == false, so the
// generic worker calls this directly — see executePhase in worker.go).
//
// TX-POISONING DEFENSE: exactly like internal/asset/importer.go's createRows,
// this runs inside ONE shared transaction for the whole batch. In PostgreSQL a
// unique-violation (23505) POISONS the whole transaction — every subsequent
// command fails with 25P02 "current transaction is aborted" — so a single
// `kode` collision at CreateEmployee time would abort and roll back the
// ENTIRE employee batch instead of just that one row. To keep the "fail one
// row, continue" design working, we make CreateEmployee never fire a 23505 in
// the common cases: before every insert we pre-check the code's availability
// (against codes consumed earlier in THIS batch and against the DB) so a
// taken code is skipped as a failed row rather than inserted. GetEmployeeByCode
// is a side-effect-free SELECT; returning pgx.ErrNoRows does NOT poison the tx.
func (e employeeImporter) Execute(ctx context.Context, qtx *sqlc.Queries, job importer.Job, validRows []importer.Row) (int, error) {
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
		officeID, err := uuid.Parse(r.Data["_office_id"])
		if err != nil {
			return created, common.ErrInvalidReference
		}

		var deptID, posID *uuid.UUID
		if v := trim(r.Data["_department_id"]); v != "" {
			id, pErr := uuid.Parse(v)
			if pErr != nil {
				return created, common.ErrInvalidReference
			}
			deptID = &id
		}
		if v := trim(r.Data["_position_id"]); v != "" {
			id, pErr := uuid.Parse(v)
			if pErr != nil {
				return created, common.ErrInvalidReference
			}
			posID = &id
		}

		code := trim(r.Data[colCode])
		codeKey := normCode(code)

		// Pre-check availability BEFORE inserting, so CreateEmployee is never
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
		if _, gErr := qtx.GetEmployeeByCode(ctx, code); gErr == nil {
			// A row already exists for this code — skip it as failed.
			if fErr := markFailed(r.ID); fErr != nil {
				return created, fErr
			}
			continue
		} else if !errors.Is(gErr, pgx.ErrNoRows) {
			// Any error other than "no rows" is a real DB error.
			return created, common.MapDBError(gErr)
		}

		status := validStatuses[strings.ToLower(trim(r.Data[colStatus]))]

		var email, phone *string
		if v := trim(r.Data[colEmail]); v != "" {
			email = &v
		}
		if v := trim(r.Data[colPhone]); v != "" {
			phone = &v
		}

		emp, err := qtx.CreateEmployee(ctx, sqlc.CreateEmployeeParams{
			Code:         code,
			Name:         r.Data[colName],
			Email:        email,
			Phone:        phone,
			AvatarKey:    nil,
			DepartmentID: deptID,
			PositionID:   posID,
			OfficeID:     officeID,
			Status:       status,
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
			// code via GetEmployeeByCode and skips that row.
			return created, common.MapDBError(err)
		}

		usedCodes[codeKey] = true
		if err := qtx.MarkRowResult(ctx, sqlc.MarkRowResultParams{ID: r.ID, ResultRef: &emp.Code}); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}
