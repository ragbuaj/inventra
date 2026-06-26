// Package masterdata implements CRUD for reference master data
// (categories, offices, employees, ...). Categories serves as the pattern.
package masterdata

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// Shared service errors.
var (
	ErrNotFound         = errors.New("not found")
	ErrConflict         = errors.New("a record with this unique value already exists")
	ErrInvalidReference = errors.New("invalid reference")
)

// RegisterRoutes mounts all master-data endpoints. Read is open to any
// authenticated user; writes require the masterdata.global.manage permission.
func RegisterRoutes(rg *gin.RouterGroup, q *sqlc.Queries, pool *pgxpool.Pool, permSvc *authz.PermissionService, scopeSvc *authz.ScopeService, aud *audit.Service, authMW gin.HandlerFunc) {
	globalManage := middleware.RequirePermission(permSvc, "masterdata.global.manage")
	officeManage := middleware.RequirePermission(permSvc, "masterdata.office.manage")

	registerCategories(rg, q, aud, authMW, globalManage)
	registerReference(rg, pool, aud, authMW, globalManage)
	registerOffices(rg, q, scopeSvc, aud, authMW, officeManage)
	registerFloors(rg, q, scopeSvc, aud, authMW, officeManage)
	registerRooms(rg, q, scopeSvc, aud, authMW, officeManage)
	registerEmployees(rg, q, scopeSvc, aud, authMW, officeManage)
}

// --- shared helpers -------------------------------------------------------

func mapDBError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrConflict
		case "23503":
			return ErrInvalidReference
		}
	}
	return err
}

func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidReference):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

func parseUUIDPtr(s *string) (*uuid.UUID, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func uuidPtrStr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

func tsStr(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.Format(time.RFC3339)
	return &s
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
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
