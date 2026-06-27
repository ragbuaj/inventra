package reference

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// RegisterRoutes mounts every declared reference resource under its own path.
// Reads are open to any authenticated user; writes require the global-manage permission.
func RegisterRoutes(rg *gin.RouterGroup, pool *pgxpool.Pool, aud *audit.Service, authMW, requireManage gin.HandlerFunc) {
	e := &engine{pool: pool}
	for _, res := range referenceResources {
		res := res
		g := rg.Group("/" + res.Path)
		g.GET("", authMW, func(c *gin.Context) { list(c, e, res) })
		g.GET("/:id", authMW, func(c *gin.Context) { get(c, e, res) })
		g.POST("", authMW, requireManage, func(c *gin.Context) { create(c, e, aud, res) })
		g.PUT("/:id", authMW, requireManage, func(c *gin.Context) { update(c, e, aud, res) })
		g.DELETE("/:id", authMW, requireManage, func(c *gin.Context) { remove(c, e, aud, res) })
	}
}

// entityID extracts the row's uuid from the generic result map.
func entityID(m map[string]any) (uuid.UUID, bool) {
	s, ok := m["id"].(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func list(c *gin.Context, e *engine, r resource) {
	search := c.Query("search")
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<31-1)
	data, total, err := e.list(c.Request.Context(), r, search, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func get(c *gin.Context, e *engine, r resource) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	m, err := e.get(c.Request.Context(), r, id)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, m)
}

func create(c *gin.Context, e *engine, aud *audit.Service, r resource) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m, err := e.write(c.Request.Context(), r, nil, body)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	if id, ok := entityID(m); ok {
		audit.Record(c, aud, audit.ActionCreate, r.Table, id, nil, audit.Diff(nil, m))
	}
	c.JSON(http.StatusCreated, m)
}

func update(c *gin.Context, e *engine, aud *audit.Service, r resource) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	before, err := e.get(c.Request.Context(), r, id)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	m, err := e.write(c.Request.Context(), r, &id, body)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	audit.Record(c, aud, audit.ActionUpdate, r.Table, id, nil, audit.Diff(before, m))
	c.JSON(http.StatusOK, m)
}

func remove(c *gin.Context, e *engine, aud *audit.Service, r resource) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	before, err := e.get(c.Request.Context(), r, id)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	ok, err := e.del(c.Request.Context(), r, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete"})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": common.ErrNotFound.Error()})
		return
	}
	audit.Record(c, aud, audit.ActionDelete, r.Table, id, nil, audit.Diff(before, nil))
	c.Status(http.StatusNoContent)
}
