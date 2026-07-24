package department

import (
	"strings"

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
// Returns ErrBlankName when the name is blank/whitespace (gin's `required` only
// rejects the empty string, matching the old reference engine's TrimSpace check).
func (r Request) toInput() (CreateInput, error) {
	name := strings.TrimSpace(r.Name)
	if name == "" {
		return CreateInput{}, ErrBlankName
	}
	officeID, err := common.ParseUUIDPtr(r.OfficeID)
	if err != nil {
		return CreateInput{}, err
	}
	code := r.Code
	if code != nil {
		trimmed := strings.TrimSpace(*code)
		if trimmed == "" {
			code = nil // an empty/blank code stays NULL so the partial-unique index ignores it
		} else {
			code = &trimmed
		}
	}
	return CreateInput{
		Name:     name,
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
