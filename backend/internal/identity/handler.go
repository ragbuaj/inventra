package identity

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/ratelimit"
)

// Handler exposes the identity HTTP endpoints.
type Handler struct {
	svc         *Service
	perms       *authz.PermissionService
	scopes      *authz.ScopeService
	limiter     ratelimit.Allower
	loginPerMin int
}

// NewHandler builds the identity Handler.
func NewHandler(svc *Service, perms *authz.PermissionService, scopes *authz.ScopeService, limiter ratelimit.Allower, loginPerMin int) *Handler {
	return &Handler{svc: svc, perms: perms, scopes: scopes, limiter: limiter, loginPerMin: loginPerMin}
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
	pair, _, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		h.authError(c, err)
		return
	}
	c.JSON(http.StatusOK, newTokenResponse(pair))
}

func (h *Handler) refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	pair, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		h.authError(c, err)
		return
	}
	c.JSON(http.StatusOK, newTokenResponse(pair))
}

func (h *Handler) logout(c *gin.Context) {
	var req logoutRequest
	_ = c.ShouldBindJSON(&req) // refresh_token optional

	jti, _ := c.Get(middleware.CtxAccessJTI)
	exp, _ := c.Get(middleware.CtxAccessExp)
	if err := h.svc.Logout(c.Request.Context(), jti.(string), exp.(time.Time), req.RefreshToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "logout failed"})
		return
	}
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
