// Package common holds shared helpers for the masterdata resource packages
// (office, category, employee, floor, room, reference): sentinel errors, DB-error
// mapping, HTTP error writing, UUID/timestamp helpers, and pagination clamping.
//
// Each resource lives in its own sub-package with the standard dto/service/handler/
// routes split and depends on this package for the cross-cutting plumbing.
package common

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
)

// Shared sentinel errors for masterdata resources.
var (
	ErrNotFound         = errors.New("not found")
	ErrConflict         = errors.New("a record with this unique value already exists")
	ErrInvalidReference = errors.New("invalid reference")
	ErrCheckViolation   = errors.New("value violates a field constraint")
	ErrForbidden        = errors.New("forbidden")
)

// MapDBError translates pgx/Postgres errors into package sentinel errors.
func MapDBError(err error) error {
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
		case "23514": // check_violation (e.g. building_classifications max_floors < min_floors)
			return ErrCheckViolation
		}
	}
	return err
}

// WriteError maps a (sentinel) error to its HTTP status + JSON body.
func WriteError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidReference):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrCheckViolation):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

// ParseUUIDPtr parses an optional UUID string ("" / nil → nil).
func ParseUUIDPtr(s *string) (*uuid.UUID, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// UUIDPtrStr renders an optional UUID as an optional string.
func UUIDPtrStr(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

// TsStr renders a nullable timestamptz as an optional RFC3339 string.
func TsStr(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.Format(time.RFC3339)
	return &s
}

// DateStr renders a nullable date as an optional "2006-01-02" string.
func DateStr(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format("2006-01-02")
	return &s
}

// BoolOr returns *p or def when p is nil.
func BoolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// ClampInt parses raw and clamps it into [min, max], falling back to def.
func ClampInt(raw string, def, min, max int32) int32 {
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
