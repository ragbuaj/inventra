// Package office implements CRUD for the offices master-data resource, split into
// dto / service / handler / routes. The service holds business logic + data-scope
// enforcement (no Gin); the handler maps HTTP ↔ service and records audit.
package office

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Scope-violation sentinels (mapped to HTTP 403 by the handler).
var (
	ErrParentOutOfScope   = errors.New("office must be placed under an office within your scope")
	ErrReparentOutOfScope = errors.New("cannot reparent the office outside your scope")
)

// Service holds the data-access + business rules for offices.
type Service struct {
	q *sqlc.Queries
}

func NewService(q *sqlc.Queries) *Service { return &Service{q: q} }

// CreateInput is the domain input for creating/updating an office (UUIDs resolved).
type CreateInput struct {
	ParentID     *uuid.UUID
	OfficeTypeID uuid.UUID
	ProvinceID   *uuid.UUID
	CityID       *uuid.UUID
	Name         string
	Code         string
	Address      *string
	IsActive     bool
	Latitude     *float64
	Longitude    *float64
}

// UpdateInput mirrors CreateInput for updates.
type UpdateInput struct{ CreateInput }

// List returns offices within the caller's scope, plus the total count.
func (s *Service) List(ctx context.Context, all bool, ids []uuid.UUID, search string, limit, offset int32) ([]sqlc.MasterdataOffice, int64, error) {
	rows, err := s.q.ListOffices(ctx, sqlc.ListOfficesParams{AllScope: all, OfficeIds: ids, Search: search, Lim: limit, Off: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountOffices(ctx, sqlc.CountOfficesParams{AllScope: all, OfficeIds: ids, Search: search})
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// Get returns one office within the caller's scope.
func (s *Service) Get(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.MasterdataOffice, error) {
	o, err := s.q.GetOffice(ctx, sqlc.GetOfficeParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return o, common.MapDBError(err)
	}
	return o, nil
}

// Create inserts an office. A scoped caller may only place it under a parent
// within their scope.
func (s *Service) Create(ctx context.Context, all bool, ids []uuid.UUID, in CreateInput) (sqlc.MasterdataOffice, error) {
	if !all && (in.ParentID == nil || !common.InScope(all, ids, *in.ParentID)) {
		return sqlc.MasterdataOffice{}, ErrParentOutOfScope
	}
	o, err := s.q.CreateOffice(ctx, sqlc.CreateOfficeParams{
		ParentID:     in.ParentID,
		OfficeTypeID: in.OfficeTypeID,
		ProvinceID:   in.ProvinceID,
		CityID:       in.CityID,
		Name:         in.Name,
		Code:         in.Code,
		Address:      in.Address,
		IsActive:     in.IsActive,
		Latitude:     in.Latitude,
		Longitude:    in.Longitude,
	})
	if err != nil {
		return o, common.MapDBError(err)
	}
	return o, nil
}

// Update modifies an office within scope and returns (before, after) for auditing.
// A scoped caller may not reparent the office outside their scope.
func (s *Service) Update(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID, in UpdateInput) (before, after sqlc.MasterdataOffice, err error) {
	cur, err := s.q.GetOffice(ctx, sqlc.GetOfficeParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return before, after, common.MapDBError(err)
	}
	if !all && !common.SamePtr(in.ParentID, cur.ParentID) && (in.ParentID == nil || !common.InScope(all, ids, *in.ParentID)) {
		return cur, after, ErrReparentOutOfScope
	}
	o, err := s.q.UpdateOffice(ctx, sqlc.UpdateOfficeParams{
		ParentID:     in.ParentID,
		OfficeTypeID: in.OfficeTypeID,
		ProvinceID:   in.ProvinceID,
		CityID:       in.CityID,
		Name:         in.Name,
		Code:         in.Code,
		Address:      in.Address,
		IsActive:     in.IsActive,
		Latitude:     in.Latitude,
		Longitude:    in.Longitude,
		ID:           id,
		AllScope:     all,
		OfficeIds:    ids,
	})
	if err != nil {
		return cur, o, common.MapDBError(err)
	}
	return cur, o, nil
}

// Delete soft-deletes an office within scope, returning the removed row for audit.
func (s *Service) Delete(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.MasterdataOffice, error) {
	cur, err := s.q.GetOffice(ctx, sqlc.GetOfficeParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return cur, common.MapDBError(err)
	}
	n, err := s.q.SoftDeleteOffice(ctx, sqlc.SoftDeleteOfficeParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return cur, err
	}
	if n == 0 {
		return cur, common.ErrNotFound
	}
	return cur, nil
}
