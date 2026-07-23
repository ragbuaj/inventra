package office

import (
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Request is the create/update payload for an office.
type Request struct {
	ParentID     *string  `json:"parent_id" binding:"omitempty,uuid"`
	OfficeTypeID string   `json:"office_type_id" binding:"required,uuid"`
	ProvinceID   *string  `json:"province_id" binding:"omitempty,uuid"`
	CityID       *string  `json:"city_id" binding:"omitempty,uuid"`
	Name         string   `json:"name" binding:"required"`
	Code         string   `json:"code" binding:"required"`
	Address      *string  `json:"address"`
	IsActive     *bool    `json:"is_active"`
	Latitude     *float64 `json:"latitude" binding:"omitempty,min=-90,max=90"`
	Longitude    *float64 `json:"longitude" binding:"omitempty,min=-180,max=180"`
	// Legacy-parity Fase 5 fields.
	OwnershipStatus          *string `json:"ownership_status" binding:"omitempty,oneof=sewa milik hg_pakai free"`
	OfficeClassID            *string `json:"office_class_id" binding:"omitempty,uuid"`
	BuildingClassificationID *string `json:"building_classification_id" binding:"omitempty,uuid"`
	FloorCount               *int32  `json:"floor_count" binding:"omitempty,min=0"`
	BuildingArea             *string `json:"building_area"`
	OfficeKind               *string `json:"office_kind" binding:"omitempty,oneof=konvensional syariah"`
	Description              *string `json:"description"`
	HeadEmployeeID           *string `json:"head_employee_id" binding:"omitempty,uuid"`
	Contact                  *string `json:"contact"`
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
	officeClass, err := common.ParseUUIDPtr(r.OfficeClassID)
	if err != nil {
		return CreateInput{}, err
	}
	buildingClass, err := common.ParseUUIDPtr(r.BuildingClassificationID)
	if err != nil {
		return CreateInput{}, err
	}
	headEmp, err := common.ParseUUIDPtr(r.HeadEmployeeID)
	if err != nil {
		return CreateInput{}, err
	}
	var ownership *sqlc.SharedOfficeOwnership
	if r.OwnershipStatus != nil && *r.OwnershipStatus != "" {
		v := sqlc.SharedOfficeOwnership(*r.OwnershipStatus)
		ownership = &v
	}
	// office_kind defaults to konvensional when absent (bank uses conventional only).
	kind := sqlc.SharedOfficeKindKonvensional
	if r.OfficeKind != nil && *r.OfficeKind != "" {
		kind = sqlc.SharedOfficeKind(*r.OfficeKind)
	}
	return CreateInput{
		ParentID:                 parent,
		OfficeTypeID:             uuid.MustParse(r.OfficeTypeID),
		ProvinceID:               province,
		CityID:                   city,
		Name:                     r.Name,
		Code:                     r.Code,
		Address:                  r.Address,
		IsActive:                 common.BoolOr(r.IsActive, true),
		Latitude:                 r.Latitude,
		Longitude:                r.Longitude,
		OwnershipStatus:          ownership,
		OfficeClassID:            officeClass,
		BuildingClassificationID: buildingClass,
		FloorCount:               r.FloorCount,
		BuildingArea:             r.BuildingArea,
		OfficeKind:               kind,
		Description:              r.Description,
		HeadEmployeeID:           headEmp,
		Contact:                  r.Contact,
	}, nil
}

// Response is the serialized office.
type Response struct {
	ID           string   `json:"id"`
	ParentID     *string  `json:"parent_id"`
	OfficeTypeID string   `json:"office_type_id"`
	ProvinceID   *string  `json:"province_id"`
	CityID       *string  `json:"city_id"`
	Name         string   `json:"name"`
	Code         string   `json:"code"`
	Address      *string  `json:"address"`
	IsActive     bool     `json:"is_active"`
	Latitude     *float64 `json:"latitude"`
	Longitude    *float64 `json:"longitude"`
	// Legacy-parity Fase 5 fields.
	OwnershipStatus          *string `json:"ownership_status"`
	OfficeClassID            *string `json:"office_class_id"`
	BuildingClassificationID *string `json:"building_classification_id"`
	FloorCount               *int32  `json:"floor_count"`
	BuildingArea             *string `json:"building_area"`
	OfficeKind               string  `json:"office_kind"`
	Description              *string `json:"description"`
	HeadEmployeeID           *string `json:"head_employee_id"`
	Contact                  *string `json:"contact"`
	CreatedAt                *string `json:"created_at"`
	UpdatedAt                *string `json:"updated_at"`
}

// ownershipStr renders a nullable office_ownership enum as an optional string.
func ownershipStr(o *sqlc.SharedOfficeOwnership) *string {
	if o == nil {
		return nil
	}
	s := string(*o)
	return &s
}

// MapResponse is one office on the Peta Lokasi map (resolved names + asset count).
type MapResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Code           string   `json:"code"`
	OfficeTypeName *string  `json:"office_type_name"`
	Tier           *string  `json:"tier"`
	ProvinceName   *string  `json:"province_name"`
	CityName       *string  `json:"city_name"`
	Address        *string  `json:"address"`
	AssetCount     int64    `json:"asset_count"`
	Latitude       *float64 `json:"latitude"`
	Longitude      *float64 `json:"longitude"`
}

func toMapResponse(r sqlc.ListOfficesMapRow) MapResponse {
	var tier *string
	if r.Tier != nil {
		s := string(*r.Tier)
		tier = &s
	}
	return MapResponse{
		ID:             r.ID.String(),
		Name:           r.Name,
		Code:           r.Code,
		OfficeTypeName: r.OfficeTypeName,
		Tier:           tier,
		ProvinceName:   r.ProvinceName,
		CityName:       r.CityName,
		Address:        r.Address,
		AssetCount:     r.AssetCount,
		Latitude:       r.Latitude,
		Longitude:      r.Longitude,
	}
}

func toResponse(o sqlc.MasterdataOffice) Response {
	return Response{
		ID:                       o.ID.String(),
		ParentID:                 common.UUIDPtrStr(o.ParentID),
		OfficeTypeID:             o.OfficeTypeID.String(),
		ProvinceID:               common.UUIDPtrStr(o.ProvinceID),
		CityID:                   common.UUIDPtrStr(o.CityID),
		Name:                     o.Name,
		Code:                     o.Code,
		Address:                  o.Address,
		IsActive:                 o.IsActive,
		Latitude:                 o.Latitude,
		Longitude:                o.Longitude,
		OwnershipStatus:          ownershipStr(o.OwnershipStatus),
		OfficeClassID:            common.UUIDPtrStr(o.OfficeClassID),
		BuildingClassificationID: common.UUIDPtrStr(o.BuildingClassificationID),
		FloorCount:               o.FloorCount,
		BuildingArea:             o.BuildingArea,
		OfficeKind:               string(o.OfficeKind),
		Description:              o.Description,
		HeadEmployeeID:           common.UUIDPtrStr(o.HeadEmployeeID),
		Contact:                  o.Contact,
		CreatedAt:                common.TsStr(o.CreatedAt),
		UpdatedAt:                common.TsStr(o.UpdatedAt),
	}
}
