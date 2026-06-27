// Package room implements CRUD for the rooms master-data resource, split into
// dto / service / handler / routes. Rooms are scoped through their floor's office:
// callers may only act on rooms whose floor is within their data scope.
package room

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// ErrFloorOutOfScope is returned when the target floor is outside the caller's scope.
var ErrFloorOutOfScope = errors.New("floor is outside your scope")

type Service struct {
	q *sqlc.Queries
}

func NewService(q *sqlc.Queries) *Service { return &Service{q: q} }

type CreateInput struct {
	FloorID uuid.UUID
	Name    string
	Code    *string
}

type UpdateInput struct{ CreateInput }

// floorInScope reports whether the floor exists within the caller's office scope.
func (s *Service) floorInScope(ctx context.Context, floorID uuid.UUID, all bool, ids []uuid.UUID) bool {
	_, err := s.q.GetFloor(ctx, sqlc.GetFloorParams{ID: floorID, AllScope: all, OfficeIds: ids})
	return err == nil
}

// FloorOffice resolves the office a floor belongs to (for audit office scoping).
func (s *Service) FloorOffice(ctx context.Context, floorID uuid.UUID, all bool, ids []uuid.UUID) *uuid.UUID {
	fl, err := s.q.GetFloor(ctx, sqlc.GetFloorParams{ID: floorID, AllScope: all, OfficeIds: ids})
	if err != nil {
		return nil
	}
	return &fl.OfficeID
}

func (s *Service) List(ctx context.Context, all bool, ids []uuid.UUID, floorID uuid.UUID, search string, limit, offset int32) ([]sqlc.MasterdataRoom, int64, error) {
	if !s.floorInScope(ctx, floorID, all, ids) {
		return nil, 0, ErrFloorOutOfScope
	}
	rows, err := s.q.ListRoomsByFloor(ctx, sqlc.ListRoomsByFloorParams{FloorID: floorID, Search: search, Lim: limit, Off: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountRoomsByFloor(ctx, sqlc.CountRoomsByFloorParams{FloorID: floorID, Search: search})
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.MasterdataRoom, error) {
	r, err := s.q.GetRoom(ctx, sqlc.GetRoomParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return r, common.MapDBError(err)
	}
	return r, nil
}

func (s *Service) Create(ctx context.Context, all bool, ids []uuid.UUID, in CreateInput) (sqlc.MasterdataRoom, error) {
	if !s.floorInScope(ctx, in.FloorID, all, ids) {
		return sqlc.MasterdataRoom{}, ErrFloorOutOfScope
	}
	r, err := s.q.CreateRoom(ctx, sqlc.CreateRoomParams{FloorID: in.FloorID, Name: in.Name, Code: in.Code})
	if err != nil {
		return r, common.MapDBError(err)
	}
	return r, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID, in UpdateInput) (before, after sqlc.MasterdataRoom, err error) {
	if !s.floorInScope(ctx, in.FloorID, all, ids) {
		return before, after, ErrFloorOutOfScope
	}
	cur, err := s.q.GetRoom(ctx, sqlc.GetRoomParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return before, after, common.MapDBError(err)
	}
	r, err := s.q.UpdateRoom(ctx, sqlc.UpdateRoomParams{FloorID: in.FloorID, Name: in.Name, Code: in.Code, ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return cur, r, common.MapDBError(err)
	}
	return cur, r, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.MasterdataRoom, error) {
	cur, err := s.q.GetRoom(ctx, sqlc.GetRoomParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return cur, common.MapDBError(err)
	}
	n, err := s.q.SoftDeleteRoom(ctx, sqlc.SoftDeleteRoomParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return cur, err
	}
	if n == 0 {
		return cur, common.ErrNotFound
	}
	return cur, nil
}
