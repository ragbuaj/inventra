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

// defaultOfficeKind falls back to konvensional when the kind is unset, so a
// zero-value CreateInput (or a request that omits office_kind) is valid against
// the NOT NULL shared.office_kind column (bank uses conventional only).
func defaultOfficeKind(k sqlc.SharedOfficeKind) sqlc.SharedOfficeKind {
	if k == "" {
		return sqlc.SharedOfficeKindKonvensional
	}
	return k
}

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
	// Legacy-parity Fase 5 fields.
	OwnershipStatus          *sqlc.SharedOfficeOwnership
	OfficeClassID            *uuid.UUID
	BuildingClassificationID *uuid.UUID
	FloorCount               *int32
	BuildingArea             *string
	OfficeKind               sqlc.SharedOfficeKind
	Description              *string
	HeadEmployeeID           *uuid.UUID
	Contact                  *string
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

// MapList returns geo-enriched offices within the caller's scope for the map screen.
func (s *Service) MapList(ctx context.Context, all bool, ids []uuid.UUID) ([]sqlc.ListOfficesMapRow, error) {
	return s.q.ListOfficesMap(ctx, sqlc.ListOfficesMapParams{AllScope: all, OfficeIds: ids})
}

// Tree returns the full scoped office set (unbounded) for building the hierarchy tree.
func (s *Service) Tree(ctx context.Context, all bool, ids []uuid.UUID) ([]sqlc.MasterdataOffice, error) {
	return s.q.ListOfficesTree(ctx, sqlc.ListOfficesTreeParams{AllScope: all, OfficeIds: ids})
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
	in.OfficeKind = defaultOfficeKind(in.OfficeKind)
	o, err := s.q.CreateOffice(ctx, sqlc.CreateOfficeParams{
		ParentID:                 in.ParentID,
		OfficeTypeID:             in.OfficeTypeID,
		ProvinceID:               in.ProvinceID,
		CityID:                   in.CityID,
		Name:                     in.Name,
		Code:                     in.Code,
		Address:                  in.Address,
		IsActive:                 in.IsActive,
		Latitude:                 in.Latitude,
		Longitude:                in.Longitude,
		OwnershipStatus:          in.OwnershipStatus,
		OfficeClassID:            in.OfficeClassID,
		BuildingClassificationID: in.BuildingClassificationID,
		FloorCount:               in.FloorCount,
		BuildingArea:             in.BuildingArea,
		OfficeKind:               in.OfficeKind,
		Description:              in.Description,
		HeadEmployeeID:           in.HeadEmployeeID,
		Contact:                  in.Contact,
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
	in.OfficeKind = defaultOfficeKind(in.OfficeKind)
	o, err := s.q.UpdateOffice(ctx, sqlc.UpdateOfficeParams{
		ParentID:                 in.ParentID,
		OfficeTypeID:             in.OfficeTypeID,
		ProvinceID:               in.ProvinceID,
		CityID:                   in.CityID,
		Name:                     in.Name,
		Code:                     in.Code,
		Address:                  in.Address,
		IsActive:                 in.IsActive,
		Latitude:                 in.Latitude,
		Longitude:                in.Longitude,
		OwnershipStatus:          in.OwnershipStatus,
		OfficeClassID:            in.OfficeClassID,
		BuildingClassificationID: in.BuildingClassificationID,
		FloorCount:               in.FloorCount,
		BuildingArea:             in.BuildingArea,
		OfficeKind:               in.OfficeKind,
		Description:              in.Description,
		HeadEmployeeID:           in.HeadEmployeeID,
		Contact:                  in.Contact,
		ID:                       id,
		AllScope:                 all,
		OfficeIds:                ids,
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
