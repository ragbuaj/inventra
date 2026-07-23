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
// department outside their office scope (including a global NULL-office one).
// Mapped to HTTP 403 by the handler.
var ErrOfficeOutOfScope = errors.New("department office must be within your scope")

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

// Create inserts a department within the caller's scope.
func (s *Service) Create(ctx context.Context, all bool, ids []uuid.UUID, in CreateInput) (sqlc.MasterdataDepartment, error) {
	if err := requireWritableOffice(all, ids, in.OfficeID); err != nil {
		return sqlc.MasterdataDepartment{}, err
	}
	d, err := s.q.CreateDepartment(ctx, sqlc.CreateDepartmentParams{
		Name:     in.Name,
		Code:     in.Code,
		OfficeID: in.OfficeID,
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
	// The existing row must be writable (current office in scope, not a global one).
	if err = requireWritableOffice(all, ids, cur.OfficeID); err != nil {
		return cur, after, err
	}
	// The new office must also be within scope.
	if err = requireWritableOffice(all, ids, in.OfficeID); err != nil {
		return cur, after, err
	}
	d, err := s.q.UpdateDepartment(ctx, sqlc.UpdateDepartmentParams{
		Name:     in.Name,
		Code:     in.Code,
		OfficeID: in.OfficeID,
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
