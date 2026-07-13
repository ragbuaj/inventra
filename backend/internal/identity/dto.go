package identity

import (
	"time"

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
	Status       string  `json:"status"`
	AvatarURL    *string `json:"avatar_url"`
	GoogleLinked bool    `json:"google_linked"`
}

func newUserResponse(u sqlc.IdentityUser) userResponse {
	resp := userResponse{
		ID:           u.ID.String(),
		Name:         u.Name,
		Email:        u.Email,
		RoleID:       u.RoleID.String(),
		Status:       string(u.Status),
		AvatarURL:    u.AvatarUrl,
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
	ID           string
	Name         string
	Email        string
	Phone        *string
	RoleID       string
	OfficeID     *string
	EmployeeID   *string
	Status       string
	AvatarURL    *string
	GoogleLinked bool
	JoinedAt     time.Time
}

// profileFromRow maps a sqlc.GetUserProfileRow into a ProfileView.
func profileFromRow(row sqlc.GetUserProfileRow) ProfileView {
	v := ProfileView{
		ID:           row.ID.String(),
		Name:         row.Name,
		Email:        row.Email,
		Phone:        row.EmployeePhone,
		RoleID:       row.RoleID.String(),
		Status:       string(row.Status),
		AvatarURL:    row.AvatarUrl,
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
