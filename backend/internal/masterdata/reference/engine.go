// Package reference is the generic CRUD engine for simple reference master data
// (flat tables of text/bool/uuid columns + id/timestamps/deleted_at). Complex
// entities with enums/numerics/self-references (categories, offices, employees,
// floors, rooms) have their own packages instead.
package reference

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

type colType int

const (
	typeText colType = iota
	typeBool
	typeUUID
	typeEnum
)

type column struct {
	Name     string   // db column
	Type     colType  // text / bool / uuid / enum
	Required bool     // must be present (non-empty) on write
	Search   bool     // included in the ILIKE search filter
	Default  bool     // default for typeBool when absent
	Enum     []string // allowed values for typeEnum
	EnumType string   // postgres enum type name for the cast, e.g. "shared.approver_level"
}

type resource struct {
	Path    string // route segment, e.g. "office-types"
	Table   string // table name within the masterdata schema
	OrderBy string // default ordering column
	Columns []column
}

type engine struct {
	pool *pgxpool.Pool
}

// selectExpr lists id (as text), the writable columns (uuid as text), and timestamps.
func (r resource) selectExpr() string {
	parts := []string{"id::text AS id"}
	for _, c := range r.Columns {
		if c.Type == typeUUID || c.Type == typeEnum {
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

func (e *engine) list(ctx context.Context, r resource, search string, limit, offset int32) ([]map[string]any, int64, error) {
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

func (e *engine) get(ctx context.Context, r resource, id uuid.UUID) (map[string]any, error) {
	q := fmt.Sprintf("SELECT %s FROM masterdata.%s WHERE id = $1 AND deleted_at IS NULL", r.selectExpr(), r.Table)
	rows, err := e.pool.Query(ctx, q, id)
	if err != nil {
		return nil, err
	}
	m, err := pgx.CollectExactlyOneRow(rows, pgx.RowToMap)
	if err != nil {
		return nil, common.MapDBError(err)
	}
	return m, nil
}

func (e *engine) write(ctx context.Context, r resource, id *uuid.UUID, body map[string]any) (map[string]any, error) {
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
			ph[i] = placeholder(i+1, c)
			args = append(args, vals[i])
		}
		q = fmt.Sprintf("INSERT INTO masterdata.%s (%s) VALUES (%s) RETURNING %s",
			r.Table, strings.Join(cols, ", "), strings.Join(ph, ", "), r.selectExpr())
	} else { // update
		sets := make([]string, len(r.Columns))
		args = append(args, *id)
		for i, c := range r.Columns {
			sets[i] = fmt.Sprintf("%s = %s", c.Name, placeholder(i+2, c))
			args = append(args, vals[i])
		}
		q = fmt.Sprintf("UPDATE masterdata.%s SET %s WHERE id = $1 AND deleted_at IS NULL RETURNING %s",
			r.Table, strings.Join(sets, ", "), r.selectExpr())
	}

	rows, err := e.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, common.MapDBError(err)
	}
	m, err := pgx.CollectExactlyOneRow(rows, pgx.RowToMap)
	if err != nil {
		return nil, common.MapDBError(err)
	}
	return m, nil
}

func (e *engine) del(ctx context.Context, r resource, id uuid.UUID) (bool, error) {
	tag, err := e.pool.Exec(ctx,
		fmt.Sprintf("UPDATE masterdata.%s SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL", r.Table), id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func placeholder(n int, c column) string {
	switch c.Type {
	case typeUUID:
		return fmt.Sprintf("$%d::uuid", n)
	case typeEnum:
		return fmt.Sprintf("$%d::%s", n, c.EnumType)
	default:
		return fmt.Sprintf("$%d", n)
	}
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
		case typeEnum:
			if !present || raw == nil {
				out[i] = nil
				continue
			}
			s, ok := raw.(string)
			if !ok {
				return nil, fmt.Errorf("%s must be a string", c.Name)
			}
			if strings.TrimSpace(s) == "" {
				out[i] = nil
				continue
			}
			if !slices.Contains(c.Enum, s) {
				return nil, fmt.Errorf("%s must be one of %s", c.Name, strings.Join(c.Enum, ", "))
			}
			out[i] = s
		}
	}
	return out, nil
}
