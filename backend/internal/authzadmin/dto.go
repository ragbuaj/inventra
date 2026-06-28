package authzadmin

import (
	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

type roleCreateRequest struct {
	Code        string  `json:"code" binding:"required"`
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
}

type roleUpdateRequest struct {
	Code        string  `json:"code"`
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
}

type permissionsRequest struct {
	Permissions []string `json:"permissions"`
}

type scopePolicyBody struct {
	Module     string `json:"module" binding:"required"`
	ScopeLevel string `json:"scope_level" binding:"required"`
}
type scopeRequest struct {
	Policies []scopePolicyBody `json:"policies"`
}

type fieldPermBody struct {
	Entity  string `json:"entity" binding:"required"`
	Field   string `json:"field" binding:"required"`
	CanView bool   `json:"can_view"`
	CanEdit bool   `json:"can_edit"`
}
type fieldsRequest struct {
	Fields []fieldPermBody `json:"fields"`
}

func roleToMap(r sqlc.IdentityRole) map[string]any {
	return map[string]any{
		"id":          r.ID.String(),
		"code":        r.Code,
		"name":        r.Name,
		"description": r.Description,
		"is_system":   r.IsSystem,
		"created_at":  common.TsStr(r.CreatedAt),
		"updated_at":  common.TsStr(r.UpdatedAt),
	}
}

func scopePolicyToMap(p sqlc.IdentityDataScopePolicy) map[string]any {
	return map[string]any{"module": p.Module, "scope_level": string(p.ScopeLevel)}
}

// fieldPermToMap serialises a ListFieldPermissionsByRoleRow (the concrete type
// returned by GetFieldPermissions / ListFieldPermissionsByRole).
func fieldPermToMap(f sqlc.ListFieldPermissionsByRoleRow) map[string]any {
	return map[string]any{"entity": f.Entity, "field": f.Field, "can_view": f.CanView, "can_edit": f.CanEdit}
}

func (r scopeRequest) toInputs() []ScopePolicyInput {
	out := make([]ScopePolicyInput, 0, len(r.Policies))
	for _, p := range r.Policies {
		out = append(out, ScopePolicyInput{Module: p.Module, ScopeLevel: p.ScopeLevel})
	}
	return out
}

func (r fieldsRequest) toInputs() []FieldPermInput {
	out := make([]FieldPermInput, 0, len(r.Fields))
	for _, f := range r.Fields {
		out = append(out, FieldPermInput{Entity: f.Entity, Field: f.Field, CanView: f.CanView, CanEdit: f.CanEdit})
	}
	return out
}
