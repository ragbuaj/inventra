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
