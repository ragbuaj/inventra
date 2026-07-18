package identity

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/oauth"
	"github.com/ragbuaj/inventra/internal/ratelimit"
)

// googleAuth is the OAuth surface the handler needs (satisfied by *oauth.Service).
type googleAuth interface {
	AuthCodeURL(ctx context.Context) (url, state string, err error)
	Exchange(ctx context.Context, code, state string) (email, sub string, err error)
}

// Handler exposes the identity HTTP endpoints.
type Handler struct {
	svc          *Service
	perms        *authz.PermissionService
	scopes       *authz.ScopeService
	limiter      ratelimit.Allower
	loginPerMin  int
	secureCookie bool
	refreshTTL   time.Duration
	googleOAuth  googleAuth
	frontendURL  string
	audit        *audit.Service
	forgotPerMin int
	// avatarMaxBytes caps the multipart body of an avatar upload.
	avatarMaxBytes int64
}

// NewHandler builds the identity Handler.
func NewHandler(svc *Service, perms *authz.PermissionService, scopes *authz.ScopeService, limiter ratelimit.Allower, loginPerMin int, secureCookie bool, refreshTTL time.Duration, googleOAuth googleAuth, frontendURL string, auditSvc *audit.Service, forgotPerMin int, avatarMaxBytes int64) *Handler {
	return &Handler{svc: svc, perms: perms, scopes: scopes, limiter: limiter, loginPerMin: loginPerMin, secureCookie: secureCookie, refreshTTL: refreshTTL, googleOAuth: googleOAuth, frontendURL: frontendURL, audit: auditSvc, forgotPerMin: forgotPerMin, avatarMaxBytes: avatarMaxBytes}
}

// permissions returns the caller's effective RBAC permission keys.
func (h *Handler) permissions(c *gin.Context) {
	roleID, err := uuid.Parse(c.GetString(middleware.CtxRoleID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing role"})
		return
	}
	perms, err := h.perms.List(c.Request.Context(), roleID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load permissions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"permissions": perms})
}

// scope returns the caller's effective data scope for a module.
func (h *Handler) scope(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}
	user, err := h.svc.Me(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	sc, err := h.scopes.Resolve(c.Request.Context(), user.RoleID, user.OfficeID, c.Param("module"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	c.JSON(http.StatusOK, sc)
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	key := "login:acct:" + strings.ToLower(strings.TrimSpace(req.Email))
	if res := h.limiter.Allow(c.Request.Context(), key, h.loginPerMin, true); !res.Allowed {
		middleware.WriteRateLimited(c, res)
		return
	}
	pair, _, err := h.svc.Login(c.Request.Context(), req.Email, req.Password, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		h.authError(c, err)
		return
	}
	setRefreshCookie(c, pair.RefreshToken, h.refreshTTL, h.secureCookie)
	c.JSON(http.StatusOK, newTokenResponse(pair))
}

func (h *Handler) refresh(c *gin.Context) {
	rt, err := c.Cookie(refreshCookieName)
	if err != nil || rt == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing refresh token"})
		return
	}
	pair, err := h.svc.Refresh(c.Request.Context(), rt, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		h.authError(c, err)
		return
	}
	setRefreshCookie(c, pair.RefreshToken, h.refreshTTL, h.secureCookie)
	c.JSON(http.StatusOK, newTokenResponse(pair))
}

func (h *Handler) logout(c *gin.Context) {
	rt, _ := c.Cookie(refreshCookieName)
	jti, _ := c.Get(middleware.CtxAccessJTI)
	exp, _ := c.Get(middleware.CtxAccessExp)
	userID := c.GetString(middleware.CtxUserID)
	sid := c.GetString(middleware.CtxSessionID)
	if err := h.svc.Logout(c.Request.Context(), jti.(string), exp.(time.Time), rt, userID, sid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "logout failed"})
		return
	}
	clearRefreshCookie(c, h.secureCookie)
	c.JSON(http.StatusOK, gin.H{"status": "logged_out"})
}

func (h *Handler) me(c *gin.Context) {
	idStr, _ := c.Get(middleware.CtxUserID)
	userID, err := uuid.Parse(idStr.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}
	user, err := h.svc.Me(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, newUserResponse(user))
}

// listSessions returns the caller's active device sessions (current flagged).
func (h *Handler) listSessions(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}
	sessions, err := h.svc.ListSessions(c.Request.Context(), userID, c.GetString(middleware.CtxSessionID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load sessions"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": sessions})
}

// revokeSession kills one of the caller's own sessions. A sid that is not the
// caller's own returns 404 (never reveals or touches another user's session).
func (h *Handler) revokeSession(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}
	sid := c.Param("id")
	if err := h.svc.RevokeSession(c.Request.Context(), userID, sid); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	audit.Record(c, h.audit, audit.ActionUpdate, "user", userID, nil, gin.H{"event": "session_revoked", "session_id": sid})
	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

// revokeOtherSessions logs the caller out of every device except the current one.
func (h *Handler) revokeOtherSessions(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}
	revoked, err := h.svc.RevokeOtherSessions(c.Request.Context(), userID, c.GetString(middleware.CtxSessionID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	audit.Record(c, h.audit, audit.ActionUpdate, "user", userID, nil, gin.H{"event": "sessions_revoked_others", "revoked": revoked})
	c.JSON(http.StatusOK, gin.H{"revoked": revoked})
}

func (h *Handler) authError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrInvalidToken):
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
	case errors.Is(err, ErrUserInactive):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

// googleStart redirects the browser to Google's consent screen.
func (h *Handler) googleStart(c *gin.Context) {
	authURL, _, err := h.googleOAuth.AuthCodeURL(c.Request.Context())
	if err != nil {
		h.redirectAuthError(c, googleReason(err))
		return
	}
	c.Redirect(http.StatusFound, authURL)
}

// googleCallback completes the flow: validate, exchange, link-only login, set the
// refresh cookie, and redirect back to the SPA.
func (h *Handler) googleCallback(c *gin.Context) {
	if c.Query("error") != "" {
		h.redirectAuthError(c, "server")
		return
	}
	email, sub, err := h.googleOAuth.Exchange(c.Request.Context(), c.Query("code"), c.Query("state"))
	if err != nil {
		h.redirectAuthError(c, googleReason(err))
		return
	}
	pair, _, err := h.svc.LoginWithGoogle(c.Request.Context(), email, sub, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		h.redirectAuthError(c, googleReason(err))
		return
	}
	setRefreshCookie(c, pair.RefreshToken, h.refreshTTL, h.secureCookie)
	c.Redirect(http.StatusFound, h.frontendURL+"/login?oauth=success")
}

// redirectAuthError sends the browser back to the SPA login with a short, safe
// reason code. It never reflects user input into the Location.
func (h *Handler) redirectAuthError(c *gin.Context, reason string) {
	c.Redirect(http.StatusFound, h.frontendURL+"/login?oauth=error&reason="+url.QueryEscape(reason))
}

// googleReason maps an internal error to a fixed, non-sensitive reason code.
func googleReason(err error) string {
	switch {
	case errors.Is(err, oauth.ErrDisabled):
		return "disabled"
	case errors.Is(err, ErrNotProvisioned):
		return "not_registered"
	case errors.Is(err, ErrGoogleMismatch):
		return "account_mismatch"
	case errors.Is(err, ErrUserInactive):
		return "inactive"
	default:
		return "server"
	}
}

func (h *Handler) forgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	acctKey := "pwforgot:acct:" + strings.ToLower(strings.TrimSpace(req.Email))
	if res := h.limiter.Allow(c.Request.Context(), acctKey, h.forgotPerMin, true); !res.Allowed {
		middleware.WriteRateLimited(c, res)
		return
	}
	if err := h.svc.RequestPasswordReset(c.Request.Context(), strings.ToLower(strings.TrimSpace(req.Email))); err != nil {
		// Log server-side; never leak whether the address exists.
		slog.Error("password reset request failed", "error", err)
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *Handler) resetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.svc.ResetPassword(c.Request.Context(), req.Token, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidToken):
			c.JSON(http.StatusBadRequest, gin.H{"error": "tautan tidak valid atau kedaluwarsa"})
		case errors.Is(err, ErrWeakPassword):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	audit.Record(c, h.audit, audit.ActionUpdate, "user", user.ID, user.OfficeID, gin.H{"event": "password_reset"})
	c.JSON(http.StatusOK, gin.H{"status": "password_reset"})
}

func (h *Handler) changePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}
	user, err := h.svc.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			c.JSON(http.StatusBadRequest, gin.H{"error": "password lama salah"})
		case errors.Is(err, ErrWeakPassword):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	audit.Record(c, h.audit, audit.ActionUpdate, "user", user.ID, user.OfficeID, gin.H{"event": "password_changed"})
	clearRefreshCookie(c, h.secureCookie)
	c.JSON(http.StatusOK, gin.H{"status": "password_changed"})
}

// getProfile returns the caller's own profile.
func (h *Handler) getProfile(c *gin.Context) {
	userID, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}
	profile, err := h.svc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, profile)
}

// updateProfile sets the caller's display name and (if linked) employee phone.
func (h *Handler) updateProfile(c *gin.Context) {
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}
	profile, err := h.svc.UpdateProfile(c.Request.Context(), userID, req.Name, req.Phone)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	audit.Record(c, h.audit, audit.ActionUpdate, "user", userID, officeIDFromView(profile.OfficeID), gin.H{"event": "profile_updated"})
	c.JSON(http.StatusOK, profile)
}

// requestEmailChange verifies the current password and emails a confirmation
// link to the new address (the address is not changed until confirmed).
func (h *Handler) requestEmailChange(c *gin.Context) {
	var req emailChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}
	newEmail := strings.ToLower(strings.TrimSpace(req.NewEmail))
	if err := h.svc.RequestEmailChange(c.Request.Context(), userID, newEmail, req.CurrentPassword); err != nil {
		switch {
		// 400, not 401: the frontend's authenticated-request interceptor treats any
		// 401 as an expired access token and force-logs the user out. A wrong
		// *current* password here is a validation failure, not an auth failure.
		case errors.Is(err, ErrInvalidCredentials):
			c.JSON(http.StatusBadRequest, gin.H{"error": "password salah"})
		case errors.Is(err, ErrEmailInUse), errors.Is(err, ErrSameEmail):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	audit.Record(c, h.audit, audit.ActionUpdate, "user", userID, nil, gin.H{"event": "email_change_requested", "new_email": newEmail})
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// confirmEmailChange consumes the emailed token and updates the address. It
// is a PUBLIC endpoint (the confirmation link may be opened from any device,
// not necessarily an authenticated session), so the audit actor is the
// affected user itself, mirroring resetPassword's public-route convention.
func (h *Handler) confirmEmailChange(c *gin.Context) {
	var req emailConfirmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.svc.ConfirmEmailChange(c.Request.Context(), req.Token)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidToken):
			c.JSON(http.StatusBadRequest, gin.H{"error": "tautan tidak valid atau kedaluwarsa"})
		case errors.Is(err, ErrEmailInUse):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	audit.Record(c, h.audit, audit.ActionUpdate, "user", user.ID, user.OfficeID, gin.H{"event": "email_changed"})
	c.JSON(http.StatusOK, gin.H{"status": "email_changed"})
}

// requestPasswordChange verifies the current password then emails a reset
// link (reusing the forgot-password flow) to complete the change.
func (h *Handler) requestPasswordChange(c *gin.Context) {
	var req passwordChangeRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return
	}
	if err := h.svc.RequestPasswordChange(c.Request.Context(), userID, req.CurrentPassword); err != nil {
		switch {
		// 400, not 401: see requestEmailChange above — a wrong current password
		// must not trip the frontend's "access token expired" 401 interceptor.
		case errors.Is(err, ErrInvalidCredentials):
			c.JSON(http.StatusBadRequest, gin.H{"error": "password lama salah"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		}
		return
	}
	audit.Record(c, h.audit, audit.ActionUpdate, "user", userID, nil, gin.H{"event": "password_change_requested"})
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
