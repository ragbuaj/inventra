package audit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
)

// Handler exposes the read-only audit-trail endpoints.
type Handler struct {
	svc   *Service
	scope *authz.ScopeService
	q     *sqlc.Queries
}

// NewHandler builds the audit Handler.
func NewHandler(svc *Service, scope *authz.ScopeService, q *sqlc.Queries) *Handler {
	return &Handler{svc: svc, scope: scope, q: q}
}

func (h *Handler) list(c *gin.Context) {
	all, ids, err := callerScope(c, h.q, h.scope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve scope"})
		return
	}

	f := ListFilter{
		AllScope:  all,
		OfficeIDs: ids,
		Search:    c.Query("search"),
		Limit:     clampInt(c.Query("limit"), 20, 1, 100),
		Offset:    clampInt(c.Query("offset"), 0, 0, 1<<31-1),
	}
	if v := c.Query("actor_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid actor_id"})
			return
		}
		f.ActorID = &id
	}
	if v := c.Query("entity_type"); v != "" {
		f.EntityType = &v
	}
	if v := c.Query("action"); v != "" {
		if !validAction(v) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action"})
			return
		}
		a := Action(v)
		f.Action = &a
	}
	if v := c.Query("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from (RFC3339)"})
			return
		}
		f.From = &t
	}
	if v := c.Query("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to (RFC3339)"})
			return
		}
		f.To = &t
	}

	rows, total, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list audit logs"})
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, auditToMap(r))
	}
	c.JSON(http.StatusOK, listResponse{Data: data, Total: total, Limit: f.Limit, Offset: f.Offset})
}

func validAction(s string) bool {
	switch Action(s) {
	case ActionCreate, ActionUpdate, ActionDelete:
		return true
	default:
		return false
	}
}

func clampInt(raw string, def, min, max int32) int32 {
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	v := int32(n)
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
