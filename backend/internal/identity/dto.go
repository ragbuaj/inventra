package identity

import (
	"time"

	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/auth"
)

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"` // access token lifetime, seconds
}

func newTokenResponse(p auth.TokenPair) tokenResponse {
	return tokenResponse{
		AccessToken: p.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(p.AccessExpiresAt).Seconds()),
	}
}

type userResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Email        string  `json:"email"`
	RoleID       string  `json:"role_id"`
	OfficeID     *string `json:"office_id"`
	EmployeeID   *string `json:"employee_id"`
	Status string `json:"status"`
	// HasAvatar reports whether an avatar object exists; the object key itself
	// is never serialized. Fetch the image from GET /auth/avatar.
	HasAvatar    bool `json:"has_avatar"`
	GoogleLinked bool `json:"google_linked"`
}

func newUserResponse(u sqlc.IdentityUser) userResponse {
	resp := userResponse{
		ID:           u.ID.String(),
		Name:         u.Name,
		Email:        u.Email,
		RoleID:       u.RoleID.String(),
		Status:       string(u.Status),
		HasAvatar:    u.AvatarKey != nil,
		GoogleLinked: u.GoogleID != nil,
	}
	if u.OfficeID != nil {
		s := u.OfficeID.String()
		resp.OfficeID = &s
	}
	if u.EmployeeID != nil {
		s := u.EmployeeID.String()
		resp.EmployeeID = &s
	}
	return resp
}

// forgotPasswordRequest starts a reset; response is always 200 (anti-enumeration).
type forgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// resetPasswordRequest completes a reset with the emailed token.
type resetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// changePasswordRequest changes the authenticated user's password.
type changePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ProfileView is the caller's own profile, including the linked employee's
// phone number. It deliberately omits password_hash and the raw google_id
// (exposed only as GoogleLinked) — never serialize those.
type ProfileView struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	Phone        *string   `json:"phone"`
	RoleID       string    `json:"role_id"`
	RoleName     *string   `json:"role_name"`
	OfficeID     *string   `json:"office_id"`
	OfficeName   *string   `json:"office_name"`
	EmployeeID   *string   `json:"employee_id"`
	EmployeeName *string   `json:"employee_name"`
	// Employee master-data detail, all nil when the user has no linked employee.
	EmployeeCode   *string `json:"employee_code"`
	EmployeeStatus *string `json:"employee_status"`
	DepartmentName *string `json:"department_name"`
	PositionName   *string `json:"position_name"`
	Status string `json:"status"`
	// HasAvatar reports whether an avatar object exists; the object key itself
	// is never serialized. Fetch the image from GET /auth/avatar.
	HasAvatar    bool      `json:"has_avatar"`
	GoogleLinked bool      `json:"google_linked"`
	JoinedAt     time.Time `json:"joined_at"`
}

// profileFromRow maps a sqlc.GetUserProfileRow into a ProfileView.
func profileFromRow(row sqlc.GetUserProfileRow) ProfileView {
	v := ProfileView{
		ID:           row.ID.String(),
		Name:         row.Name,
		Email:        row.Email,
		Phone:        row.EmployeePhone,
		RoleID:       row.RoleID.String(),
		RoleName:     row.RoleName,
		OfficeName:   row.OfficeName,
		EmployeeName:   row.EmployeeName,
		EmployeeCode:   row.EmployeeCode,
		DepartmentName: row.DepartmentName,
		PositionName:   row.PositionName,
		Status:         string(row.Status),
		HasAvatar:    row.AvatarKey != nil,
		GoogleLinked: row.GoogleID != nil,
		JoinedAt:     row.CreatedAt.Time,
	}
	if row.OfficeID != nil {
		s := row.OfficeID.String()
		v.OfficeID = &s
	}
	if row.EmployeeID != nil {
		s := row.EmployeeID.String()
		v.EmployeeID = &s
	}
	if row.EmployeeStatus != nil {
		s := string(*row.EmployeeStatus)
		v.EmployeeStatus = &s
	}
	return v
}

// ptrOrNil returns nil for an empty string, else a pointer to it — used to
// clear a nullable column when the caller submits an empty value.
func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// updateProfileRequest updates the caller's own display name and (if linked)
// employee phone number.
type updateProfileRequest struct {
	Name  string `json:"name" binding:"required"`
	Phone string `json:"phone"`
}

// emailChangeRequest starts an email-change flow: verifies the current
// password and emails a confirmation link to the NEW address.
type emailChangeRequest struct {
	NewEmail        string `json:"new_email" binding:"required,email"`
	CurrentPassword string `json:"current_password" binding:"required"`
}

// emailConfirmRequest completes an email change with the emailed token.
type emailConfirmRequest struct {
	Token string `json:"token" binding:"required"`
}

// SessionView is one active device session in the caller's session list.
// browser/os/device_type are derived from the stored user-agent; location is a
// best-effort GeoIP city/country (empty when unresolved — the UI falls back to
// the IP). It never carries the raw refresh token or JTI.
type SessionView struct {
	ID         string    `json:"id"`
	Browser    string    `json:"browser"`
	OS         string    `json:"os"`
	DeviceType string    `json:"device_type"`
	IPAddress  string    `json:"ip_address"`
	Location   string    `json:"location"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	Current    bool      `json:"current"`
}

// passwordChangeRequestRequest verifies the caller's current password and
// triggers a password-reset email (the actual change happens via the reset
// link, same as a forgotten-password flow).
type passwordChangeRequestRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
}

// officeIDFromView parses a ProfileView's string office id (if any) back into
// a *uuid.UUID for audit logging. A malformed/absent id yields nil rather
// than failing the request — auditing must never break the caller's flow.
func officeIDFromView(s *string) *uuid.UUID {
	if s == nil {
		return nil
	}
	if id, err := uuid.Parse(*s); err == nil {
		return &id
	}
	return nil
}
