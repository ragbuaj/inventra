package office

import (
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Request is the create/update payload for an office.
type Request struct {
	ParentID     *string `json:"parent_id" binding:"omitempty,uuid"`
	OfficeTypeID string  `json:"office_type_id" binding:"required,uuid"`
	ProvinceID   *string `json:"province_id" binding:"omitempty,uuid"`
	CityID       *string `json:"city_id" binding:"omitempty,uuid"`
	Name         string  `json:"name" binding:"required"`
	Code         string  `json:"code" binding:"required"`
	Address      *string `json:"address"`
	IsActive     *bool   `json:"is_active"`
}

// toInput resolves the request's UUID strings into a service CreateInput.
// OfficeTypeID is guaranteed valid by the `uuid` binding tag.
func (r Request) toInput() (CreateInput, error) {
	parent, err := common.ParseUUIDPtr(r.ParentID)
	if err != nil {
		return CreateInput{}, err
	}
	province, err := common.ParseUUIDPtr(r.ProvinceID)
	if err != nil {
		return CreateInput{}, err
	}
	city, err := common.ParseUUIDPtr(r.CityID)
	if err != nil {
		return CreateInput{}, err
	}
	return CreateInput{
		ParentID:     parent,
		OfficeTypeID: uuid.MustParse(r.OfficeTypeID),
		ProvinceID:   province,
		CityID:       city,
		Name:         r.Name,
		Code:         r.Code,
		Address:      r.Address,
		IsActive:     common.BoolOr(r.IsActive, true),
	}, nil
}

// Response is the serialized office.
type Response struct {
	ID           string  `json:"id"`
	ParentID     *string `json:"parent_id"`
	OfficeTypeID string  `json:"office_type_id"`
	ProvinceID   *string `json:"province_id"`
	CityID       *string `json:"city_id"`
	Name         string  `json:"name"`
	Code         string  `json:"code"`
	Address      *string `json:"address"`
	IsActive     bool    `json:"is_active"`
	CreatedAt    *string `json:"created_at"`
	UpdatedAt    *string `json:"updated_at"`
}

func toResponse(o sqlc.MasterdataOffice) Response {
	return Response{
		ID:           o.ID.String(),
		ParentID:     common.UUIDPtrStr(o.ParentID),
		OfficeTypeID: o.OfficeTypeID.String(),
		ProvinceID:   common.UUIDPtrStr(o.ProvinceID),
		CityID:       common.UUIDPtrStr(o.CityID),
		Name:         o.Name,
		Code:         o.Code,
		Address:      o.Address,
		IsActive:     o.IsActive,
		CreatedAt:    common.TsStr(o.CreatedAt),
		UpdatedAt:    common.TsStr(o.UpdatedAt),
	}
}
