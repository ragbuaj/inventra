// Package floor implements CRUD for the floors master-data resource, split into
// dto / service / handler / routes. Floors are office-scoped: callers may only act
// on floors whose office is within their data scope.
package floor

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// ErrOfficeOutOfScope is returned when the target office is outside the caller's scope.
var ErrOfficeOutOfScope = errors.New("office is outside your scope")

type Service struct {
	q *sqlc.Queries
}

func NewService(q *sqlc.Queries) *Service { return &Service{q: q} }

type CreateInput struct {
	OfficeID uuid.UUID
	Name     string
	Level    *int32
}

type UpdateInput struct{ CreateInput }

// List returns floors of a given office, after verifying it is within scope.
func (s *Service) List(ctx context.Context, all bool, ids []uuid.UUID, officeID uuid.UUID, search string, limit, offset int32) ([]sqlc.MasterdataFloor, int64, error) {
	if !common.InScope(all, ids, officeID) {
		return nil, 0, ErrOfficeOutOfScope
	}
	rows, err := s.q.ListFloorsByOffice(ctx, sqlc.ListFloorsByOfficeParams{OfficeID: officeID, Search: search, Lim: limit, Off: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountFloorsByOffice(ctx, sqlc.CountFloorsByOfficeParams{OfficeID: officeID, Search: search})
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.MasterdataFloor, error) {
	f, err := s.q.GetFloor(ctx, sqlc.GetFloorParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return f, common.MapDBError(err)
	}
	return f, nil
}

func (s *Service) Create(ctx context.Context, all bool, ids []uuid.UUID, in CreateInput) (sqlc.MasterdataFloor, error) {
	if !common.InScope(all, ids, in.OfficeID) {
		return sqlc.MasterdataFloor{}, ErrOfficeOutOfScope
	}
	f, err := s.q.CreateFloor(ctx, sqlc.CreateFloorParams{OfficeID: in.OfficeID, Name: in.Name, Level: in.Level})
	if err != nil {
		return f, common.MapDBError(err)
	}
	return f, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID, in UpdateInput) (before, after sqlc.MasterdataFloor, err error) {
	if !common.InScope(all, ids, in.OfficeID) {
		return before, after, ErrOfficeOutOfScope
	}
	cur, err := s.q.GetFloor(ctx, sqlc.GetFloorParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return before, after, common.MapDBError(err)
	}
	f, err := s.q.UpdateFloor(ctx, sqlc.UpdateFloorParams{OfficeID: in.OfficeID, Name: in.Name, Level: in.Level, ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return cur, f, common.MapDBError(err)
	}
	return cur, f, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.MasterdataFloor, error) {
	cur, err := s.q.GetFloor(ctx, sqlc.GetFloorParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return cur, common.MapDBError(err)
	}
	n, err := s.q.SoftDeleteFloor(ctx, sqlc.SoftDeleteFloorParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return cur, err
	}
	if n == 0 {
		return cur, common.ErrNotFound
	}
	return cur, nil
}
