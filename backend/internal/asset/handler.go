package asset

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

const scopeModule = "assets"

// Handler maps HTTP ↔ the asset service and records audit entries.
type Handler struct {
	svc      *Service
	fieldSvc *authz.FieldService
	scoped   common.ScopedDeps
	aud      *audit.Service
}

// NewHandler builds the asset handler.
func NewHandler(svc *Service, fieldSvc *authz.FieldService, scoped common.ScopedDeps, aud *audit.Service) *Handler {
	return &Handler{svc: svc, fieldSvc: fieldSvc, scoped: scoped, aud: aud}
}

// filterMap applies field-permission masking for the caller's role on the "assets" entity.
func (h *Handler) filterMap(c *gin.Context, m map[string]any) map[string]any {
	roleID, err := uuid.Parse(c.GetString(middleware.CtxRoleID))
	if err != nil {
		return m
	}
	policies, err := h.fieldSvc.ForEntity(c.Request.Context(), roleID, "assets")
	if err != nil || policies == nil {
		return m
	}
	authz.FilterView(policies, m)
	return m
}

func (h *Handler) list(c *gin.Context) {
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<31-1)

	in := ListInput{
		AllScope:  all,
		OfficeIDs: ids,
		Limit:     limit,
		Offset:    offset,
	}
	if s := c.Query("search"); s != "" {
		in.Search = &s
	}
	if s := c.Query("category_id"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			in.CategoryID = &id
		}
	}
	if s := c.Query("office_id"); s != "" {
		if id, err := uuid.Parse(s); err == nil {
			in.OfficeFilter = &id
		}
	}
	if s := c.Query("status"); s != "" {
		v := sqlc.SharedAssetStatus(s)
		in.Status = &v
	}
	if s := c.Query("asset_class"); s != "" {
		v := sqlc.SharedAssetClass(s)
		in.AssetClass = &v
	}

	rows, total, err := h.svc.List(c.Request.Context(), in)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list assets"})
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		data = append(data, h.filterMap(c, assetToMap(a)))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func (h *Handler) get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	a, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		svcError(c, err)
		return
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	if !common.InScope(all, ids, a.OfficeID) {
		common.WriteError(c, common.ErrForbidden)
		return
	}
	c.JSON(http.StatusOK, h.filterMap(c, assetToMap(a)))
}

func (h *Handler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req AssetUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	in, err := req.toInput()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Resolve scope before mutating — scope check uses the current row's office.
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	before, after, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		svcError(c, err)
		return
	}
	if !common.InScope(all, ids, before.OfficeID) {
		common.WriteError(c, common.ErrForbidden)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "assets", after.ID, &after.OfficeID,
		audit.Diff(assetToMap(before), assetToMap(after)))
	c.JSON(http.StatusOK, h.filterMap(c, assetToMap(after)))
}

// svcError maps asset service sentinel errors to HTTP status codes.
func svcError(c *gin.Context, err error) {
	switch err {
	case ErrNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case ErrConflict:
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case ErrInvalidRef:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case ErrInvalidState:
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case ErrRoomRequired:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}
