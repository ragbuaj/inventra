package department

import (
	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Request is the create/update body. Mirrors the shape the generic reference
// screen already sends for departments (name/code/office_id/is_active), so the
// frontend needs no change when departments move to this scoped endpoint.
type Request struct {
	Name     string  `json:"name" binding:"required"`
	Code     *string `json:"code"`
	OfficeID *string `json:"office_id"`
	IsActive *bool   `json:"is_active"`
}

// toInput resolves the request into the service CreateInput (UUIDs parsed).
func (r Request) toInput() (CreateInput, error) {
	officeID, err := common.ParseUUIDPtr(r.OfficeID)
	if err != nil {
		return CreateInput{}, err
	}
	code := r.Code
	if code != nil && *code == "" {
		code = nil // an empty code stays NULL so the partial-unique index ignores it
	}
	return CreateInput{
		Name:     r.Name,
		Code:     code,
		OfficeID: officeID,
		IsActive: common.BoolOr(r.IsActive, true),
	}, nil
}

// toResponse serializes a department row into the same map shape the generic
// reference engine emitted, so existing frontend consumers keep working.
func toResponse(d sqlc.MasterdataDepartment) map[string]any {
	return map[string]any{
		"id":         d.ID.String(),
		"name":       d.Name,
		"code":       d.Code,
		"office_id":  common.UUIDPtrStr(d.OfficeID),
		"is_active":  d.IsActive,
		"created_at": common.TsStr(d.CreatedAt),
		"updated_at": common.TsStr(d.UpdatedAt),
	}
}
