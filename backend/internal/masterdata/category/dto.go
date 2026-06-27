package category

import (
	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Request is the create/update payload for a category.
type Request struct {
	Name                      string  `json:"name" binding:"required"`
	Code                      *string `json:"code"`
	ParentID                  *string `json:"parent_id" binding:"omitempty,uuid"`
	DefaultDepreciationMethod *string `json:"default_depreciation_method" binding:"omitempty,oneof=straight_line declining_balance"`
	DefaultUsefulLifeMonths   *int32  `json:"default_useful_life_months"`
	DefaultSalvageRate        *string `json:"default_salvage_rate"`
	// Bank fixed-asset (PRD v1.1) accounting/tax defaults.
	AssetClass              *string `json:"asset_class" binding:"omitempty,oneof=tangible intangible"`
	DefaultFiscalGroup      *string `json:"default_fiscal_group" binding:"omitempty,oneof=kelompok_1 kelompok_2 kelompok_3 kelompok_4 bangunan_permanen bangunan_non_permanen non_susut"`
	DefaultFiscalLifeMonths *int32  `json:"default_fiscal_life_months"`
	GlAccountCode           *string `json:"gl_account_code"`
	CapitalizationThreshold *string `json:"capitalization_threshold"`
	IsActive                *bool   `json:"is_active"`
}

func (r Request) toInput() (CreateInput, error) {
	parent, err := common.ParseUUIDPtr(r.ParentID)
	if err != nil {
		return CreateInput{}, err
	}
	var method *sqlc.SharedDepreciationMethod
	if r.DefaultDepreciationMethod != nil {
		m := sqlc.SharedDepreciationMethod(*r.DefaultDepreciationMethod)
		method = &m
	}
	assetClass := sqlc.SharedAssetClassTangible
	if r.AssetClass != nil && *r.AssetClass != "" {
		assetClass = sqlc.SharedAssetClass(*r.AssetClass)
	}
	var fiscalGroup *sqlc.SharedFiscalAssetGroup
	if r.DefaultFiscalGroup != nil && *r.DefaultFiscalGroup != "" {
		g := sqlc.SharedFiscalAssetGroup(*r.DefaultFiscalGroup)
		fiscalGroup = &g
	}
	return CreateInput{
		Name:         r.Name,
		Code:         r.Code,
		ParentID:     parent,
		DeprMethod:   method,
		UsefulLifeMo: r.DefaultUsefulLifeMonths,
		SalvageRate:  r.DefaultSalvageRate,
		AssetClass:   assetClass,
		FiscalGroup:  fiscalGroup,
		FiscalLifeMo: r.DefaultFiscalLifeMonths,
		GLAccount:    r.GlAccountCode,
		CapThreshold: r.CapitalizationThreshold,
		IsActive:     common.BoolOr(r.IsActive, true),
	}, nil
}

// Response is the serialized category.
type Response struct {
	ID                        string  `json:"id"`
	Name                      string  `json:"name"`
	Code                      *string `json:"code"`
	ParentID                  *string `json:"parent_id"`
	DefaultDepreciationMethod *string `json:"default_depreciation_method"`
	DefaultUsefulLifeMonths   *int32  `json:"default_useful_life_months"`
	DefaultSalvageRate        *string `json:"default_salvage_rate"`
	AssetClass                string  `json:"asset_class"`
	DefaultFiscalGroup        *string `json:"default_fiscal_group"`
	DefaultFiscalLifeMonths   *int32  `json:"default_fiscal_life_months"`
	GlAccountCode             *string `json:"gl_account_code"`
	CapitalizationThreshold   *string `json:"capitalization_threshold"`
	IsActive                  bool    `json:"is_active"`
	CreatedAt                 *string `json:"created_at"`
	UpdatedAt                 *string `json:"updated_at"`
}

func toResponse(c sqlc.MasterdataCategory) Response {
	var method *string
	if c.DefaultDepreciationMethod != nil {
		s := string(*c.DefaultDepreciationMethod)
		method = &s
	}
	var fiscalGroup *string
	if c.DefaultFiscalGroup != nil {
		s := string(*c.DefaultFiscalGroup)
		fiscalGroup = &s
	}
	return Response{
		ID:                        c.ID.String(),
		Name:                      c.Name,
		Code:                      c.Code,
		ParentID:                  common.UUIDPtrStr(c.ParentID),
		DefaultDepreciationMethod: method,
		DefaultUsefulLifeMonths:   c.DefaultUsefulLifeMonths,
		DefaultSalvageRate:        c.DefaultSalvageRate,
		AssetClass:                string(c.AssetClass),
		DefaultFiscalGroup:        fiscalGroup,
		DefaultFiscalLifeMonths:   c.DefaultFiscalLifeMonths,
		GlAccountCode:             c.GlAccountCode,
		CapitalizationThreshold:   c.CapitalizationThreshold,
		IsActive:                  c.IsActive,
		CreatedAt:                 common.TsStr(c.CreatedAt),
		UpdatedAt:                 common.TsStr(c.UpdatedAt),
	}
}
