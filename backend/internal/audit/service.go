// Package audit records create/update/delete actions to the append-only
// audit.audit_logs table and serves an office-scoped, filterable read model.
// The writer is best-effort: callers use Record (see record.go), which never
// fails the user's request if the audit insert errors.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ragbuaj/inventra/db/sqlc"
)

// Action aliases the generated enum for ergonomic call sites.
type Action = sqlc.SharedAuditAction

const (
	ActionCreate = sqlc.SharedAuditActionCreate
	ActionUpdate = sqlc.SharedAuditActionUpdate
	ActionDelete = sqlc.SharedAuditActionDelete
)

// ErrNotFound is returned when no audit row matches.
var ErrNotFound = errors.New("not found")

// Service reads/writes audit logs.
type Service struct {
	q *sqlc.Queries
}

// NewService builds the audit Service.
func NewService(q *sqlc.Queries) *Service {
	return &Service{q: q}
}

// LogInput describes a single audit event. Changes is marshalled to JSON
// (nil → SQL NULL); IP empty → NULL.
type LogInput struct {
	ActorID    *uuid.UUID
	EntityType string
	EntityID   uuid.UUID
	Action     Action
	Changes    any
	IP         string
	OfficeID   *uuid.UUID
}

// Log inserts one audit row.
func (s *Service) Log(ctx context.Context, in LogInput) error {
	var raw []byte
	if in.Changes != nil {
		b, err := json.Marshal(in.Changes)
		if err != nil {
			return err
		}
		raw = b
	}
	var ip *string
	if in.IP != "" {
		ip = &in.IP
	}
	_, err := s.q.InsertAuditLog(ctx, sqlc.InsertAuditLogParams{
		ActorID:    in.ActorID,
		EntityType: in.EntityType,
		EntityID:   in.EntityID,
		Action:     in.Action,
		Changes:    raw,
		Ip:         ip,
		OfficeID:   in.OfficeID,
	})
	return err
}

// ListFilter holds the audit-view query (office scope + optional filters + paging).
type ListFilter struct {
	AllScope   bool
	OfficeIDs  []uuid.UUID
	ActorID    *uuid.UUID
	EntityType *string
	Action     *Action
	From       *time.Time
	To         *time.Time
	Search     string
	Limit      int32
	Offset     int32
}

// List returns audit rows (newest first) and the total count for the filter.
func (s *Service) List(ctx context.Context, f ListFilter) ([]sqlc.ListAuditLogsRow, int64, error) {
	officeIDs := f.OfficeIDs
	if officeIDs == nil {
		officeIDs = []uuid.UUID{}
	}
	rows, err := s.q.ListAuditLogs(ctx, sqlc.ListAuditLogsParams{
		AllScope:   f.AllScope,
		OfficeIds:  officeIDs,
		ActorID:    f.ActorID,
		EntityType: f.EntityType,
		Action:     f.Action,
		FromTs:     tsPtr(f.From),
		ToTs:       tsPtr(f.To),
		Search:     f.Search,
		Lim:        f.Limit,
		Off:        f.Offset,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	total, err := s.q.CountAuditLogs(ctx, sqlc.CountAuditLogsParams{
		AllScope:   f.AllScope,
		OfficeIds:  officeIDs,
		ActorID:    f.ActorID,
		EntityType: f.EntityType,
		Action:     f.Action,
		FromTs:     tsPtr(f.From),
		ToTs:       tsPtr(f.To),
		Search:     f.Search,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	return rows, total, nil
}

// Diff builds a changed-fields map { field: {before, after} } from two snapshots.
// Create passes (nil, after) → every field as {after}; delete passes (before, nil)
// → every field as {before}; update passes (before, after) → only changed fields.
// created_at / updated_at are ignored (noise).
func Diff(before, after any) map[string]map[string]any {
	b := toMap(before)
	a := toMap(after)
	out := map[string]map[string]any{}
	seen := map[string]struct{}{}
	for k := range b {
		seen[k] = struct{}{}
	}
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range seen {
		if k == "created_at" || k == "updated_at" {
			continue
		}
		bv, bok := b[k]
		av, aok := a[k]
		if bok && aok && reflect.DeepEqual(bv, av) {
			continue
		}
		entry := map[string]any{}
		if bok {
			entry["before"] = bv
		}
		if aok {
			entry["after"] = av
		}
		out[k] = entry
	}
	return out
}

func toMap(v any) map[string]any {
	if v == nil {
		return map[string]any{}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return map[string]any{}
	}
	m := map[string]any{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]any{}
	}
	return m
}

func tsPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func mapDBError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// audit reads are simple; nothing else to translate today.
		return err
	}
	return err
}
