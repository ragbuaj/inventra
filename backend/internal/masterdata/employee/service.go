// Package employee implements CRUD for the employees master-data resource, split
// into dto / service / handler / routes. Employees are office-scoped: callers may
// only act on employees whose office is within their data scope.
package employee

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// ErrOfficeOutOfScope is returned when the employee's office is outside the caller's scope.
var ErrOfficeOutOfScope = errors.New("employee office must be within your scope")

type Service struct {
	q *sqlc.Queries
}

func NewService(q *sqlc.Queries) *Service { return &Service{q: q} }

type CreateInput struct {
	Code         string
	Name         string
	Email        *string
	Phone        *string
	AvatarKey    *string
	DepartmentID *uuid.UUID
	PositionID   *uuid.UUID
	OfficeID     uuid.UUID
	Status       sqlc.SharedUserStatus
}

type UpdateInput struct{ CreateInput }

func (s *Service) List(ctx context.Context, all bool, ids []uuid.UUID, search string, limit, offset int32) ([]sqlc.MasterdataEmployee, int64, error) {
	rows, err := s.q.ListEmployees(ctx, sqlc.ListEmployeesParams{AllScope: all, OfficeIds: ids, Search: search, Lim: limit, Off: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountEmployees(ctx, sqlc.CountEmployeesParams{AllScope: all, OfficeIds: ids, Search: search})
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.MasterdataEmployee, error) {
	e, err := s.q.GetEmployee(ctx, sqlc.GetEmployeeParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return e, common.MapDBError(err)
	}
	return e, nil
}

func (s *Service) Create(ctx context.Context, all bool, ids []uuid.UUID, in CreateInput) (sqlc.MasterdataEmployee, error) {
	if !common.InScope(all, ids, in.OfficeID) {
		return sqlc.MasterdataEmployee{}, ErrOfficeOutOfScope
	}
	e, err := s.q.CreateEmployee(ctx, sqlc.CreateEmployeeParams{
		Code:         in.Code,
		Name:         in.Name,
		Email:        in.Email,
		Phone:        in.Phone,
		AvatarKey:    in.AvatarKey,
		DepartmentID: in.DepartmentID,
		PositionID:   in.PositionID,
		OfficeID:     in.OfficeID,
		Status:       in.Status,
	})
	if err != nil {
		return e, common.MapDBError(err)
	}
	return e, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID, in UpdateInput) (before, after sqlc.MasterdataEmployee, err error) {
	if !common.InScope(all, ids, in.OfficeID) {
		return before, after, ErrOfficeOutOfScope
	}
	cur, err := s.q.GetEmployee(ctx, sqlc.GetEmployeeParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return before, after, common.MapDBError(err)
	}
	e, err := s.q.UpdateEmployee(ctx, sqlc.UpdateEmployeeParams{
		Code:         in.Code,
		Name:         in.Name,
		Email:        in.Email,
		Phone:        in.Phone,
		AvatarKey:    in.AvatarKey,
		DepartmentID: in.DepartmentID,
		PositionID:   in.PositionID,
		OfficeID:     in.OfficeID,
		Status:       in.Status,
		ID:           id,
		AllScope:     all,
		OfficeIds:    ids,
	})
	if err != nil {
		return cur, e, common.MapDBError(err)
	}
	return cur, e, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.MasterdataEmployee, error) {
	cur, err := s.q.GetEmployee(ctx, sqlc.GetEmployeeParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return cur, common.MapDBError(err)
	}
	n, err := s.q.SoftDeleteEmployee(ctx, sqlc.SoftDeleteEmployeeParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return cur, err
	}
	if n == 0 {
		return cur, common.ErrNotFound
	}
	return cur, nil
}
