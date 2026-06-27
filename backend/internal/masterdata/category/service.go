// Package category implements CRUD for the asset categories master-data resource,
// split into dto / service / handler / routes. Categories are global reference
// data (not office-scoped).
package category

import (
	"context"

	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Service holds the data-access for categories.
type Service struct {
	q *sqlc.Queries
}

func NewService(q *sqlc.Queries) *Service { return &Service{q: q} }

// CreateInput is the domain input for creating/updating a category.
type CreateInput struct {
	Name         string
	Code         *string
	ParentID     *uuid.UUID
	DeprMethod   *sqlc.SharedDepreciationMethod
	UsefulLifeMo *int32
	SalvageRate  *string
	// Bank fixed-asset (PRD v1.1) accounting/tax defaults.
	AssetClass   sqlc.SharedAssetClass
	FiscalGroup  *sqlc.SharedFiscalAssetGroup
	FiscalLifeMo *int32
	GLAccount    *string
	CapThreshold *string
	IsActive     bool
}

// UpdateInput mirrors CreateInput for updates.
type UpdateInput struct{ CreateInput }

func (s *Service) List(ctx context.Context, search string, limit, offset int32) ([]sqlc.MasterdataCategory, int64, error) {
	rows, err := s.q.ListCategories(ctx, sqlc.ListCategoriesParams{Search: search, Lim: limit, Off: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountCategories(ctx, search)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (sqlc.MasterdataCategory, error) {
	cat, err := s.q.GetCategory(ctx, id)
	if err != nil {
		return cat, common.MapDBError(err)
	}
	return cat, nil
}

func (s *Service) Create(ctx context.Context, in CreateInput) (sqlc.MasterdataCategory, error) {
	cat, err := s.q.CreateCategory(ctx, sqlc.CreateCategoryParams{
		Name:                      in.Name,
		Code:                      in.Code,
		ParentID:                  in.ParentID,
		DefaultDepreciationMethod: in.DeprMethod,
		DefaultUsefulLifeMonths:   in.UsefulLifeMo,
		DefaultSalvageRate:        in.SalvageRate,
		AssetClass:                in.AssetClass,
		DefaultFiscalGroup:        in.FiscalGroup,
		DefaultFiscalLifeMonths:   in.FiscalLifeMo,
		GlAccountCode:             in.GLAccount,
		CapitalizationThreshold:   in.CapThreshold,
		IsActive:                  in.IsActive,
	})
	if err != nil {
		return cat, common.MapDBError(err)
	}
	return cat, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, in UpdateInput) (before, after sqlc.MasterdataCategory, err error) {
	cur, err := s.q.GetCategory(ctx, id)
	if err != nil {
		return before, after, common.MapDBError(err)
	}
	cat, err := s.q.UpdateCategory(ctx, sqlc.UpdateCategoryParams{
		ID:                        id,
		Name:                      in.Name,
		Code:                      in.Code,
		ParentID:                  in.ParentID,
		DefaultDepreciationMethod: in.DeprMethod,
		DefaultUsefulLifeMonths:   in.UsefulLifeMo,
		DefaultSalvageRate:        in.SalvageRate,
		AssetClass:                in.AssetClass,
		DefaultFiscalGroup:        in.FiscalGroup,
		DefaultFiscalLifeMonths:   in.FiscalLifeMo,
		GlAccountCode:             in.GLAccount,
		CapitalizationThreshold:   in.CapThreshold,
		IsActive:                  in.IsActive,
	})
	if err != nil {
		return cur, cat, common.MapDBError(err)
	}
	return cur, cat, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) (sqlc.MasterdataCategory, error) {
	cur, err := s.q.GetCategory(ctx, id)
	if err != nil {
		return cur, common.MapDBError(err)
	}
	n, err := s.q.SoftDeleteCategory(ctx, id)
	if err != nil {
		return cur, err
	}
	if n == 0 {
		return cur, common.ErrNotFound
	}
	return cur, nil
}
