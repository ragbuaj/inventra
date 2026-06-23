package authz

import (
	"context"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/ragbuaj/inventra/db/sqlc"
)

// PermissionChecker is the per-action RBAC contract consumed by middleware.
type PermissionChecker interface {
	Has(ctx context.Context, roleID uuid.UUID, key string) (bool, error)
	List(ctx context.Context, roleID uuid.UUID) ([]string, error)
}

// PermissionService loads a role's action permissions (role_permissions), cached in Redis.
type PermissionService struct {
	q   *sqlc.Queries
	rdb *redis.Client
}

// NewPermissionService builds a PermissionService.
func NewPermissionService(q *sqlc.Queries, rdb *redis.Client) *PermissionService {
	return &PermissionService{q: q, rdb: rdb}
}

func permKey(roleID uuid.UUID) string { return "authz:perms:" + roleID.String() }

// List returns the permission keys granted to the role.
func (s *PermissionService) List(ctx context.Context, roleID uuid.UUID) ([]string, error) {
	var perms []string
	if cacheGetJSON(ctx, s.rdb, permKey(roleID), &perms) {
		return perms, nil
	}
	perms, err := s.q.ListRolePermissions(ctx, roleID)
	if err != nil {
		return nil, err
	}
	cacheSetJSON(ctx, s.rdb, permKey(roleID), perms, defaultTTL)
	return perms, nil
}

// Has reports whether the role holds the given permission key.
func (s *PermissionService) Has(ctx context.Context, roleID uuid.UUID, key string) (bool, error) {
	perms, err := s.List(ctx, roleID)
	if err != nil {
		return false, err
	}
	for _, p := range perms {
		if p == key {
			return true, nil
		}
	}
	return false, nil
}

// Invalidate clears the cached permissions for a role (call after changing role_permissions).
func (s *PermissionService) Invalidate(ctx context.Context, roleID uuid.UUID) error {
	return s.rdb.Del(ctx, permKey(roleID)).Err()
}
