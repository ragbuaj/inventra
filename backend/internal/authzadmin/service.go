package authzadmin

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
)

var (
	ErrNotFound          = errors.New("role not found")
	ErrConflict          = errors.New("role code already in use")
	ErrSystemRole        = errors.New("system role cannot be modified or deleted")
	ErrRoleInUse         = errors.New("role is assigned to one or more users")
	ErrUnknownPermission = errors.New("unknown permission key")
	ErrValidation        = errors.New("invalid request")
)

// Service manages roles and the three authorization-config tables, invalidating
// the authz caches after each mutation.
type Service struct {
	q     *sqlc.Queries
	pool  *pgxpool.Pool
	perm  *authz.PermissionService
	scope *authz.ScopeService
	field *authz.FieldService
}

// NewService builds the authzadmin Service.
func NewService(q *sqlc.Queries, pool *pgxpool.Pool, perm *authz.PermissionService, scope *authz.ScopeService, field *authz.FieldService) *Service {
	return &Service{q: q, pool: pool, perm: perm, scope: scope, field: field}
}

// RoleInput holds create/update fields for a role.
type RoleInput struct {
	Code        string
	Name        string
	Description *string
}

// ScopePolicyInput is one data-scope policy row in a replace-set.
type ScopePolicyInput struct {
	Module     string
	ScopeLevel string
}

// FieldPermInput is one field-permission row in a replace-set.
type FieldPermInput struct {
	Entity  string
	Field   string
	CanView bool
	CanEdit bool
}

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}

// ── Roles ────────────────────────────────────────────────────────────────────

func (s *Service) ListRoles(ctx context.Context) ([]sqlc.IdentityRole, error) {
	rows, err := s.q.ListRoles(ctx)
	return rows, err
}

func (s *Service) GetRole(ctx context.Context, id uuid.UUID) (sqlc.IdentityRole, error) {
	r, err := s.q.GetRole(ctx, id)
	return r, mapDBError(err)
}

func (s *Service) CreateRole(ctx context.Context, in RoleInput) (sqlc.IdentityRole, error) {
	if in.Code == "" || in.Name == "" {
		return sqlc.IdentityRole{}, ErrValidation
	}
	r, err := s.q.CreateRole(ctx, sqlc.CreateRoleParams{Code: in.Code, Name: in.Name, Description: in.Description})
	return r, mapDBError(err)
}

func (s *Service) UpdateRole(ctx context.Context, id uuid.UUID, in RoleInput) (before, after sqlc.IdentityRole, err error) {
	if in.Name == "" {
		return before, after, ErrValidation
	}
	before, err = s.q.GetRole(ctx, id)
	if err != nil {
		return before, after, mapDBError(err)
	}
	code := in.Code
	if code == "" {
		code = before.Code // empty = keep existing code
	}
	if before.IsSystem && code != before.Code {
		return before, after, ErrSystemRole // system role: code is immutable
	}
	after, err = s.q.UpdateRole(ctx, sqlc.UpdateRoleParams{ID: id, Code: code, Name: in.Name, Description: in.Description})
	return before, after, mapDBError(err)
}

// DeleteRole soft-deletes a custom role and cascade-soft-deletes its config rows,
// then invalidates all three caches. System or in-use roles are rejected.
func (s *Service) DeleteRole(ctx context.Context, id uuid.UUID) (sqlc.IdentityRole, error) {
	role, err := s.q.GetRole(ctx, id)
	if err != nil {
		return role, mapDBError(err)
	}
	if role.IsSystem {
		return role, ErrSystemRole
	}
	n, err := s.q.CountUsersByRole(ctx, id)
	if err != nil {
		return role, err
	}
	if n > 0 {
		return role, ErrRoleInUse
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return role, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)
	if _, err := qtx.SoftDeleteRolePermissionsByRole(ctx, id); err != nil {
		return role, err
	}
	if _, err := qtx.SoftDeleteDataScopePoliciesByRole(ctx, id); err != nil {
		return role, err
	}
	if _, err := qtx.SoftDeleteFieldPermissionsByRole(ctx, id); err != nil {
		return role, err
	}
	rows, err := qtx.SoftDeleteRole(ctx, id)
	if err != nil {
		return role, err
	}
	if rows == 0 {
		return role, ErrNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return role, err
	}
	_ = s.perm.Invalidate(ctx, id)
	_ = s.scope.Invalidate(ctx, id)
	_ = s.field.Invalidate(ctx, id)
	return role, nil
}

// ── Role permissions ─────────────────────────────────────────────────────────

func (s *Service) GetRolePermissions(ctx context.Context, roleID uuid.UUID) ([]string, error) {
	if _, err := s.q.GetRole(ctx, roleID); err != nil {
		return nil, mapDBError(err)
	}
	return s.q.ListRolePermissions(ctx, roleID)
}

// SetRolePermissions replaces the role's permission set (validated against the catalog).
func (s *Service) SetRolePermissions(ctx context.Context, roleID uuid.UUID, keys []string) error {
	if _, err := s.q.GetRole(ctx, roleID); err != nil {
		return mapDBError(err)
	}
	clean, err := dedupePermissions(keys)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)
	if _, err := qtx.SoftDeleteRolePermissionsByRole(ctx, roleID); err != nil {
		return err
	}
	for _, k := range clean {
		if _, err := qtx.InsertRolePermission(ctx, sqlc.InsertRolePermissionParams{RoleID: roleID, PermissionKey: k}); err != nil {
			return mapDBError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return s.perm.Invalidate(ctx, roleID)
}

// ── Data scope ───────────────────────────────────────────────────────────────

func (s *Service) GetScopePolicies(ctx context.Context, roleID uuid.UUID) ([]sqlc.IdentityDataScopePolicy, error) {
	if _, err := s.q.GetRole(ctx, roleID); err != nil {
		return nil, mapDBError(err)
	}
	return s.q.ListDataScopePolicies(ctx, roleID)
}

func (s *Service) SetScopePolicies(ctx context.Context, roleID uuid.UUID, in []ScopePolicyInput) error {
	if _, err := s.q.GetRole(ctx, roleID); err != nil {
		return mapDBError(err)
	}
	clean, err := validateScopePolicies(in)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)
	if _, err := qtx.SoftDeleteDataScopePoliciesByRole(ctx, roleID); err != nil {
		return err
	}
	for _, p := range clean {
		if _, err := qtx.InsertDataScopePolicy(ctx, sqlc.InsertDataScopePolicyParams{
			RoleID: roleID, Module: p.Module, ScopeLevel: sqlc.SharedScopeLevel(p.ScopeLevel),
		}); err != nil {
			return mapDBError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return s.scope.Invalidate(ctx, roleID)
}

// ── Field permissions ────────────────────────────────────────────────────────

func (s *Service) GetFieldPermissions(ctx context.Context, roleID uuid.UUID) ([]sqlc.ListFieldPermissionsByRoleRow, error) {
	if _, err := s.q.GetRole(ctx, roleID); err != nil {
		return nil, mapDBError(err)
	}
	return s.q.ListFieldPermissionsByRole(ctx, roleID)
}

func (s *Service) SetFieldPermissions(ctx context.Context, roleID uuid.UUID, in []FieldPermInput) error {
	if _, err := s.q.GetRole(ctx, roleID); err != nil {
		return mapDBError(err)
	}
	clean, err := validateFieldPerms(in)
	if err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)
	if _, err := qtx.SoftDeleteFieldPermissionsByRole(ctx, roleID); err != nil {
		return err
	}
	for _, f := range clean {
		if _, err := qtx.InsertFieldPermission(ctx, sqlc.InsertFieldPermissionParams{
			Entity: f.Entity, Field: f.Field, RoleID: roleID, CanView: f.CanView, CanEdit: f.CanEdit,
		}); err != nil {
			return mapDBError(err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return s.field.Invalidate(ctx, roleID)
}

// ── Pure validators ──────────────────────────────────────────────────────────

func dedupePermissions(keys []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if !IsKnownPermission(k) {
			return nil, ErrUnknownPermission
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out, nil
}

var validScopeLevel = map[string]bool{"global": true, "office_subtree": true, "office": true, "own": true}

func validateScopePolicies(in []ScopePolicyInput) ([]ScopePolicyInput, error) {
	seen := map[string]bool{}
	out := make([]ScopePolicyInput, 0, len(in))
	for _, p := range in {
		if p.Module == "" || !validScopeLevel[p.ScopeLevel] {
			return nil, ErrValidation
		}
		if seen[p.Module] {
			return nil, ErrValidation
		}
		seen[p.Module] = true
		out = append(out, p)
	}
	return out, nil
}

func validateFieldPerms(in []FieldPermInput) ([]FieldPermInput, error) {
	seen := map[string]bool{}
	out := make([]FieldPermInput, 0, len(in))
	for _, f := range in {
		if f.Entity == "" || f.Field == "" {
			return nil, ErrValidation
		}
		key := f.Entity + "\x00" + f.Field
		if seen[key] {
			return nil, ErrValidation
		}
		seen[key] = true
		out = append(out, f)
	}
	return out, nil
}
