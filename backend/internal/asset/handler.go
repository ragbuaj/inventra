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
// It returns an error when ForEntity fails (e.g. Redis down) so callers can fail-closed
// rather than leaking sensitive financial fields.
// A nil policies map with no error is the legitimate "no policy / default-allow" case and
// is handled normally by FilterView (which is itself default-allow).
func (h *Handler) filterMap(c *gin.Context, m map[string]any) (map[string]any, error) {
	roleID, err := uuid.Parse(c.GetString(middleware.CtxRoleID))
	if err != nil {
		return m, nil
	}
	policies, err := h.fieldSvc.ForEntity(c.Request.Context(), roleID, "assets")
	if err != nil {
		return nil, err
	}
	if policies != nil {
		authz.FilterView(policies, m)
	}
	return m, nil
}

// validAssetStatuses and validAssetClasses hold the known enum values used for
// query-param validation in list. Values must match the shared.asset_status and
// shared.asset_class Postgres enums.
var validAssetStatuses = map[string]bool{
	"available":         true,
	"assigned":          true,
	"under_maintenance": true,
	"in_transfer":       true,
	"retired":           true,
	"disposed":          true,
	"lost":              true,
}

var validAssetClasses = map[string]bool{
	"tangible":   true,
	"intangible": true,
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
	// Finding 4: validate status and asset_class before casting to enum types.
	if s := c.Query("status"); s != "" {
		if !validAssetStatuses[s] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return
		}
		v := sqlc.SharedAssetStatus(s)
		in.Status = &v
	}
	if s := c.Query("asset_class"); s != "" {
		if !validAssetClasses[s] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset_class"})
			return
		}
		v := sqlc.SharedAssetClass(s)
		in.AssetClass = &v
	}

	rows, total, err := h.svc.List(c.Request.Context(), in)
	if err != nil {
		// Finding 3: route list service errors through svcError for consistency.
		svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		// Finding 2: fail-closed on filterMap error — do not emit unmasked map.
		masked, err := h.filterMap(c, assetToMap(a))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve field permissions"})
			return
		}
		data = append(data, masked)
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
	// Finding 2: fail-closed on filterMap error.
	masked, err := h.filterMap(c, assetToMap(a))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve field permissions"})
		return
	}
	c.JSON(http.StatusOK, masked)
}

func (h *Handler) update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// Finding 1: authorize BEFORE any mutation.
	// Step 1: fetch current asset to obtain its office for scope check.
	cur, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		svcError(c, err)
		return
	}
	// Step 2: resolve caller's data scope.
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}
	// Step 3: enforce scope gate before touching the DB with a write.
	if !common.InScope(all, ids, cur.OfficeID) {
		common.WriteError(c, common.ErrForbidden)
		return
	}

	// Step 4: bind and validate the request body.
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

	// Step 5: perform the mutation.
	before, after, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "assets", after.ID, &after.OfficeID,
		audit.Diff(assetToMap(before), assetToMap(after)))
	// Finding 2: fail-closed on filterMap error.
	masked, err := h.filterMap(c, assetToMap(after))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve field permissions"})
		return
	}
	c.JSON(http.StatusOK, masked)
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
