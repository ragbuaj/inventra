package user

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ragbuaj/inventra/db/sqlc"
)

type createUserRequest struct {
	Name       string  `json:"name" binding:"required"`
	Email      string  `json:"email" binding:"required,email"`
	Password   string  `json:"password"`
	RoleID     string  `json:"role_id" binding:"required,uuid"`
	OfficeID   *string `json:"office_id" binding:"omitempty,uuid"`
	EmployeeID *string `json:"employee_id" binding:"omitempty,uuid"`
}

type updateUserRequest struct {
	Name       string  `json:"name" binding:"required"`
	RoleID     string  `json:"role_id" binding:"required,uuid"`
	Status     string  `json:"status" binding:"required,oneof=active inactive suspended"`
	OfficeID   *string `json:"office_id" binding:"omitempty,uuid"`
	EmployeeID *string `json:"employee_id" binding:"omitempty,uuid"`
}

type listResponse struct {
	Data   []map[string]any `json:"data"`
	Total  int64            `json:"total"`
	Limit  int32            `json:"limit"`
	Offset int32            `json:"offset"`
}

// userToMap builds a serialized user record (map form so field-permission
// filtering can drop fields the caller may not view). Sensitive fields
// (password_hash, google_id) are never included.
func userToMap(u sqlc.IdentityUser) map[string]any {
	return map[string]any{
		"id":            u.ID.String(),
		"name":          u.Name,
		"email":         u.Email,
		"role_id":       u.RoleID.String(),
		"office_id":     uuidPtrStr(u.OfficeID),
		"employee_id":   uuidPtrStr(u.EmployeeID),
		"status":        string(u.Status),
		"avatar_url":    u.AvatarUrl,
		"google_linked": u.GoogleID != nil,
		"created_at":    tsStr(u.CreatedAt),
		"updated_at":    tsStr(u.UpdatedAt),
	}
}

func uuidPtrStr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

func tsStr(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.Format(time.RFC3339)
	return &s
}
