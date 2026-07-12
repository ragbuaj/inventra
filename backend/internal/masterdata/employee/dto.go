package employee

import (
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Request is the create/update payload for an employee.
type Request struct {
	Code         string  `json:"code" binding:"required"`
	Name         string  `json:"name" binding:"required"`
	Email        *string `json:"email" binding:"omitempty,email"`
	Phone        *string `json:"phone"`
	AvatarKey    *string `json:"avatar_key"`
	DepartmentID *string `json:"department_id" binding:"omitempty,uuid"`
	PositionID   *string `json:"position_id" binding:"omitempty,uuid"`
	OfficeID     string  `json:"office_id" binding:"required,uuid"`
	Status       *string `json:"status" binding:"omitempty,oneof=active inactive suspended"`
}

// toInput resolves the request into a service CreateInput. OfficeID is guaranteed
// valid by the `uuid` binding tag; optional UUIDs are parsed leniently.
func (r Request) toInput() (CreateInput, error) {
	dept, err := common.ParseUUIDPtr(r.DepartmentID)
	if err != nil {
		return CreateInput{}, err
	}
	pos, err := common.ParseUUIDPtr(r.PositionID)
	if err != nil {
		return CreateInput{}, err
	}
	return CreateInput{
		Code:         r.Code,
		Name:         r.Name,
		Email:        r.Email,
		Phone:        r.Phone,
		AvatarKey:    r.AvatarKey,
		DepartmentID: dept,
		PositionID:   pos,
		OfficeID:     uuid.MustParse(r.OfficeID),
		Status:       statusOr(r.Status, sqlc.SharedUserStatusActive),
	}, nil
}

func statusOr(p *string, def sqlc.SharedUserStatus) sqlc.SharedUserStatus {
	if p == nil || *p == "" {
		return def
	}
	return sqlc.SharedUserStatus(*p)
}

// Response is the serialized employee.
type Response struct {
	ID           string  `json:"id"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Email        *string `json:"email"`
	Phone        *string `json:"phone"`
	AvatarKey    *string `json:"avatar_key"`
	DepartmentID *string `json:"department_id"`
	PositionID   *string `json:"position_id"`
	OfficeID     string  `json:"office_id"`
	Status       string  `json:"status"`
	CreatedAt    *string `json:"created_at"`
	UpdatedAt    *string `json:"updated_at"`
}

func toResponse(e sqlc.MasterdataEmployee) Response {
	return Response{
		ID:           e.ID.String(),
		Code:         e.Code,
		Name:         e.Name,
		Email:        e.Email,
		Phone:        e.Phone,
		AvatarKey:    e.AvatarKey,
		DepartmentID: common.UUIDPtrStr(e.DepartmentID),
		PositionID:   common.UUIDPtrStr(e.PositionID),
		OfficeID:     e.OfficeID.String(),
		Status:       string(e.Status),
		CreatedAt:    common.TsStr(e.CreatedAt),
		UpdatedAt:    common.TsStr(e.UpdatedAt),
	}
}

// employeeToMap serializes an employee to a map for field-permission masking
// (authz.FieldService.FilterEntity strips non-viewable fields in place).
func employeeToMap(e sqlc.MasterdataEmployee) map[string]any {
	return map[string]any{
		"id":            e.ID.String(),
		"code":          e.Code,
		"name":          e.Name,
		"email":         e.Email,
		"phone":         e.Phone,
		"avatar_key":    e.AvatarKey,
		"department_id": common.UUIDPtrStr(e.DepartmentID),
		"position_id":   common.UUIDPtrStr(e.PositionID),
		"office_id":     e.OfficeID.String(),
		"status":        string(e.Status),
		"created_at":    common.TsStr(e.CreatedAt),
		"updated_at":    common.TsStr(e.UpdatedAt),
	}
}
