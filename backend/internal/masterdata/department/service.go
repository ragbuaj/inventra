// Package department implements CRUD for the departments master-data resource,
// split into dto / service / handler / routes. Departments are office-scoped
// (legacy-parity Fase 6): a scoped caller sees departments in their office subtree
// plus shared legacy departments (NULL office_id), and may only create/edit/delete
// departments within their scope. Global (NULL-office) departments are editable
// only by a global-scope caller. Promoted off the generic reference engine so the
// data-scope layer is enforced on read AND write (per CLAUDE.md convention).
package department

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// ErrOfficeOutOfScope is returned when a scoped caller creates/edits/deletes a
// department outside their office scope. Mapped to HTTP 403 by the handler.
var ErrOfficeOutOfScope = errors.New("department office must be within your scope")

// ErrOfficeRequired is returned when a create/update omits the office. Every
// department belongs to an office — global (NULL-office) departments are no longer
// creatable/editable by any caller. Mapped to HTTP 400 by the handler.
var ErrOfficeRequired = errors.New("department office is required")

// ErrBlankName is returned when a department name is blank/whitespace-only.
// Mapped to HTTP 400 by the handler (via the toInput error path).
var ErrBlankName = errors.New("department name is required")

// ErrFloorOfficeMismatch is returned when a department references a floor that
// does not belong to the department's office (or is referenced without an office,
// or is outside the caller's scope). Mapped to HTTP 400 by the handler.
var ErrFloorOfficeMismatch = errors.New("floor must belong to the department office")

// Service holds the data-access + business rules for departments.
type Service struct {
	q *sqlc.Queries
}

// NewService builds the department service.
func NewService(q *sqlc.Queries) *Service { return &Service{q: q} }

// CreateInput is the domain input for creating/updating a department.
type CreateInput struct {
	Name     string
	Code     *string
	OfficeID *uuid.UUID
	FloorID  *uuid.UUID
	IsActive bool
}

// UpdateInput mirrors CreateInput for updates.
type UpdateInput struct{ CreateInput }

// requireWritableOffice enforces that a scoped (non-global) caller only writes a
// department whose office is within their scope. A NULL office (global department)
// is writable only by a global-scope caller.
func requireWritableOffice(all bool, ids []uuid.UUID, officeID *uuid.UUID) error {
	if all {
		return nil
	}
	if officeID == nil || !common.InScope(all, ids, *officeID) {
		return ErrOfficeOutOfScope
	}
	return nil
}

// validateFloor enforces that a department's floor (when set) belongs to the
// department's own office and is visible to the caller. A floor referenced
// without an office, a floor the caller cannot see, or a floor on a different
// office all fail with ErrFloorOfficeMismatch. A nil floor is always valid at
// this layer (the required-floor rule is enforced by the master-data UI).
func (s *Service) validateFloor(ctx context.Context, all bool, ids []uuid.UUID, officeID, floorID *uuid.UUID) error {
	if floorID == nil {
		return nil
	}
	if officeID == nil {
		return ErrFloorOfficeMismatch
	}
	fl, err := s.q.GetFloor(ctx, sqlc.GetFloorParams{ID: *floorID, AllScope: all, OfficeIds: ids})
	if err != nil {
		// Not found / out of scope -> treat as a floor/office mismatch (400) rather
		// than leaking a bare 404, since from the caller's side the floor is invalid.
		return ErrFloorOfficeMismatch
	}
	if fl.OfficeID != *officeID {
		return ErrFloorOfficeMismatch
	}
	return nil
}

// List returns departments within the caller's scope (plus shared NULL-office
// ones), with the total count.
func (s *Service) List(ctx context.Context, all bool, ids []uuid.UUID, search string, limit, offset int32) ([]sqlc.MasterdataDepartment, int64, error) {
	rows, err := s.q.ListDepartments(ctx, sqlc.ListDepartmentsParams{AllScope: all, OfficeIds: ids, Search: search, Lim: limit, Off: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountDepartments(ctx, sqlc.CountDepartmentsParams{AllScope: all, OfficeIds: ids, Search: search})
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// Get returns one department visible to the caller (subtree or global).
func (s *Service) Get(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.MasterdataDepartment, error) {
	d, err := s.q.GetDepartment(ctx, sqlc.GetDepartmentParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return d, common.MapDBError(err)
	}
	return d, nil
}

// Create inserts a department within the caller's scope. The office is mandatory
// (no global/NULL-office departments) and must be within the caller's scope.
func (s *Service) Create(ctx context.Context, all bool, ids []uuid.UUID, in CreateInput) (sqlc.MasterdataDepartment, error) {
	if in.OfficeID == nil {
		return sqlc.MasterdataDepartment{}, ErrOfficeRequired
	}
	if err := requireWritableOffice(all, ids, in.OfficeID); err != nil {
		return sqlc.MasterdataDepartment{}, err
	}
	if err := s.validateFloor(ctx, all, ids, in.OfficeID, in.FloorID); err != nil {
		return sqlc.MasterdataDepartment{}, err
	}
	d, err := s.q.CreateDepartment(ctx, sqlc.CreateDepartmentParams{
		Name:     in.Name,
		Code:     in.Code,
		OfficeID: in.OfficeID,
		FloorID:  in.FloorID,
		IsActive: in.IsActive,
	})
	if err != nil {
		return d, common.MapDBError(err)
	}
	return d, nil
}

// Update modifies a department within scope and returns (before, after) for audit.
// Both the current office AND the target office must be within the caller's scope,
// so a scoped caller can neither edit a global department nor move one out of (or
// into) another office's scope.
func (s *Service) Update(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID, in UpdateInput) (before, after sqlc.MasterdataDepartment, err error) {
	cur, err := s.q.GetDepartment(ctx, sqlc.GetDepartmentParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return before, after, common.MapDBError(err)
	}
	// The existing row must be writable (its office in the caller's scope). A legacy
	// NULL-office (global) department is only writable by a global-scope caller, who
	// can then migrate it by assigning a real office below.
	if err = requireWritableOffice(all, ids, cur.OfficeID); err != nil {
		return cur, after, err
	}
	// The new office is mandatory (departments cannot be globalized) and must be
	// within scope.
	if in.OfficeID == nil {
		return cur, after, ErrOfficeRequired
	}
	if err = requireWritableOffice(all, ids, in.OfficeID); err != nil {
		return cur, after, err
	}
	// The floor (when set) must belong to the department's new office.
	if err = s.validateFloor(ctx, all, ids, in.OfficeID, in.FloorID); err != nil {
		return cur, after, err
	}
	d, err := s.q.UpdateDepartment(ctx, sqlc.UpdateDepartmentParams{
		Name:     in.Name,
		Code:     in.Code,
		OfficeID: in.OfficeID,
		FloorID:  in.FloorID,
		IsActive: in.IsActive,
		ID:       id,
		AllScope: all,
		OfficeIds: ids,
	})
	if err != nil {
		return cur, d, common.MapDBError(err)
	}
	return cur, d, nil
}

// Delete soft-deletes a department within scope, returning the removed row for audit.
func (s *Service) Delete(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.MasterdataDepartment, error) {
	cur, err := s.q.GetDepartment(ctx, sqlc.GetDepartmentParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return cur, common.MapDBError(err)
	}
	if err = requireWritableOffice(all, ids, cur.OfficeID); err != nil {
		return cur, err
	}
	n, err := s.q.SoftDeleteDepartment(ctx, sqlc.SoftDeleteDepartmentParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return cur, err
	}
	if n == 0 {
		return cur, common.ErrNotFound
	}
	return cur, nil
}
