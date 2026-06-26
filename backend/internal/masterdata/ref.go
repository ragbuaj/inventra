package masterdata

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ragbuaj/inventra/internal/audit"
)

// Generic CRUD engine for simple reference master data (flat tables of
// text/bool/uuid columns + id/timestamps/deleted_at). Complex entities with
// enums/numerics/self-references (e.g. categories) use sqlc instead.

type colType int

const (
	typeText colType = iota
	typeBool
	typeUUID
)

type column struct {
	Name     string  // db column
	Type     colType // text / bool / uuid
	Required bool    // must be present (non-empty) on write
	Search   bool    // included in the ILIKE search filter
	Default  bool    // default for typeBool when absent
}

type resource struct {
	Path    string // route segment, e.g. "office-types"
	Table   string // table name within the masterdata schema
	OrderBy string // default ordering column
	Columns []column
}

type refEngine struct {
	pool *pgxpool.Pool
}

// selectExpr lists id (as text), the writable columns (uuid as text), and timestamps.
func (r resource) selectExpr() string {
	parts := []string{"id::text AS id"}
	for _, c := range r.Columns {
		if c.Type == typeUUID {
			parts = append(parts, c.Name+"::text AS "+c.Name)
		} else {
			parts = append(parts, c.Name)
		}
	}
	parts = append(parts, "created_at", "updated_at")
	return strings.Join(parts, ", ")
}

func (r resource) searchClause(argPos int) string {
	var cols []string
	for _, c := range r.Columns {
		if c.Search {
			cols = append(cols, fmt.Sprintf("%s ILIKE '%%' || $%d || '%%'", c.Name, argPos))
		}
	}
	if len(cols) == 0 {
		return ""
	}
	return fmt.Sprintf(" AND ($%d = '' OR %s)", argPos, strings.Join(cols, " OR "))
}

func (e *refEngine) list(ctx context.Context, r resource, search string, limit, offset int32) ([]map[string]any, int64, error) {
	table := "masterdata." + r.Table
	q := fmt.Sprintf("SELECT %s FROM %s WHERE deleted_at IS NULL%s ORDER BY %s LIMIT $2 OFFSET $3",
		r.selectExpr(), table, r.searchClause(1), r.OrderBy)
	rows, err := e.pool.Query(ctx, q, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	data, err := pgx.CollectRows(rows, pgx.RowToMap)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	cq := fmt.Sprintf("SELECT count(*) FROM %s WHERE deleted_at IS NULL%s", table, r.searchClause(1))
	if err := e.pool.QueryRow(ctx, cq, search).Scan(&total); err != nil {
		return nil, 0, err
	}
	return data, total, nil
}

func (e *refEngine) get(ctx context.Context, r resource, id uuid.UUID) (map[string]any, error) {
	q := fmt.Sprintf("SELECT %s FROM masterdata.%s WHERE id = $1 AND deleted_at IS NULL", r.selectExpr(), r.Table)
	rows, err := e.pool.Query(ctx, q, id)
	if err != nil {
		return nil, err
	}
	m, err := pgx.CollectExactlyOneRow(rows, pgx.RowToMap)
	if err != nil {
		return nil, mapDBError(err)
	}
	return m, nil
}

func (e *refEngine) write(ctx context.Context, r resource, id *uuid.UUID, body map[string]any) (map[string]any, error) {
	vals, err := coerce(r, body)
	if err != nil {
		return nil, err
	}

	var q string
	var args []any
	if id == nil { // create
		cols := make([]string, len(r.Columns))
		ph := make([]string, len(r.Columns))
		for i, c := range r.Columns {
			cols[i] = c.Name
			ph[i] = placeholder(i+1, c.Type)
			args = append(args, vals[i])
		}
		q = fmt.Sprintf("INSERT INTO masterdata.%s (%s) VALUES (%s) RETURNING %s",
			r.Table, strings.Join(cols, ", "), strings.Join(ph, ", "), r.selectExpr())
	} else { // update
		sets := make([]string, len(r.Columns))
		args = append(args, *id)
		for i, c := range r.Columns {
			sets[i] = fmt.Sprintf("%s = %s", c.Name, placeholder(i+2, c.Type))
			args = append(args, vals[i])
		}
		q = fmt.Sprintf("UPDATE masterdata.%s SET %s WHERE id = $1 AND deleted_at IS NULL RETURNING %s",
			r.Table, strings.Join(sets, ", "), r.selectExpr())
	}

	rows, err := e.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, mapDBError(err)
	}
	m, err := pgx.CollectExactlyOneRow(rows, pgx.RowToMap)
	if err != nil {
		return nil, mapDBError(err)
	}
	return m, nil
}

func (e *refEngine) del(ctx context.Context, r resource, id uuid.UUID) (bool, error) {
	tag, err := e.pool.Exec(ctx,
		fmt.Sprintf("UPDATE masterdata.%s SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL", r.Table), id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func placeholder(n int, t colType) string {
	if t == typeUUID {
		return fmt.Sprintf("$%d::uuid", n)
	}
	return fmt.Sprintf("$%d", n)
}

// coerce validates and converts the JSON body into ordered column arguments.
func coerce(r resource, body map[string]any) ([]any, error) {
	out := make([]any, len(r.Columns))
	for i, c := range r.Columns {
		raw, present := body[c.Name]
		switch c.Type {
		case typeText, typeUUID:
			if !present || raw == nil {
				if c.Required {
					return nil, fmt.Errorf("%s is required", c.Name)
				}
				out[i] = nil
				continue
			}
			s, ok := raw.(string)
			if !ok {
				return nil, fmt.Errorf("%s must be a string", c.Name)
			}
			if c.Required && strings.TrimSpace(s) == "" {
				return nil, fmt.Errorf("%s is required", c.Name)
			}
			if c.Type == typeUUID && s != "" {
				if _, err := uuid.Parse(s); err != nil {
					return nil, fmt.Errorf("%s must be a UUID", c.Name)
				}
			}
			if s == "" {
				out[i] = nil
			} else {
				out[i] = s
			}
		case typeBool:
			if !present || raw == nil {
				out[i] = c.Default
				continue
			}
			b, ok := raw.(bool)
			if !ok {
				return nil, fmt.Errorf("%s must be a boolean", c.Name)
			}
			out[i] = b
		}
	}
	return out, nil
}

// --- HTTP wiring ----------------------------------------------------------

func registerReference(rg *gin.RouterGroup, pool *pgxpool.Pool, aud *audit.Service, authMW, requireManage gin.HandlerFunc) {
	e := &refEngine{pool: pool}
	for _, res := range referenceResources {
		res := res
		g := rg.Group("/" + res.Path)
		g.GET("", authMW, func(c *gin.Context) { refList(c, e, res) })
		g.GET("/:id", authMW, func(c *gin.Context) { refGet(c, e, res) })
		g.POST("", authMW, requireManage, func(c *gin.Context) { refCreate(c, e, aud, res) })
		g.PUT("/:id", authMW, requireManage, func(c *gin.Context) { refUpdate(c, e, aud, res) })
		g.DELETE("/:id", authMW, requireManage, func(c *gin.Context) { refDelete(c, e, aud, res) })
	}
}

// refEntityID extracts the row's uuid from the generic result map.
func refEntityID(m map[string]any) (uuid.UUID, bool) {
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

func refList(c *gin.Context, e *refEngine, r resource) {
	search := c.Query("search")
	limit := clampInt(c.Query("limit"), 20, 1, 100)
	offset := clampInt(c.Query("offset"), 0, 0, 1<<31-1)
	data, total, err := e.list(c.Request.Context(), r, search, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

func refGet(c *gin.Context, e *refEngine, r resource) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	m, err := e.get(c.Request.Context(), r, id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, m)
}

func refCreate(c *gin.Context, e *refEngine, aud *audit.Service, r resource) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	m, err := e.write(c.Request.Context(), r, nil, body)
	if err != nil {
		writeError(c, err)
		return
	}
	if id, ok := refEntityID(m); ok {
		audit.Record(c, aud, audit.ActionCreate, r.Table, id, nil, audit.Diff(nil, m))
	}
	c.JSON(http.StatusCreated, m)
}

func refUpdate(c *gin.Context, e *refEngine, aud *audit.Service, r resource) {
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
		writeError(c, err)
		return
	}
	m, err := e.write(c.Request.Context(), r, &id, body)
	if err != nil {
		writeError(c, err)
		return
	}
	audit.Record(c, aud, audit.ActionUpdate, r.Table, id, nil, audit.Diff(before, m))
	c.JSON(http.StatusOK, m)
}

func refDelete(c *gin.Context, e *refEngine, aud *audit.Service, r resource) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	before, err := e.get(c.Request.Context(), r, id)
	if err != nil {
		writeError(c, err)
		return
	}
	ok, err := e.del(c.Request.Context(), r, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete"})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": ErrNotFound.Error()})
		return
	}
	audit.Record(c, aud, audit.ActionDelete, r.Table, id, nil, audit.Diff(before, nil))
	c.Status(http.StatusNoContent)
}
