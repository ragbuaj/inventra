package authz

import (
	"context"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/ragbuaj/inventra/db/sqlc"
)

// FieldPolicy is the per-field view/edit permission for a role.
type FieldPolicy struct {
	CanView bool `json:"can_view"`
	CanEdit bool `json:"can_edit"`
}

// FieldService loads per-field permissions (field_permissions) for a role, cached in Redis.
type FieldService struct {
	q   *sqlc.Queries
	rdb *redis.Client
}

// NewFieldService builds a FieldService.
func NewFieldService(q *sqlc.Queries, rdb *redis.Client) *FieldService {
	return &FieldService{q: q, rdb: rdb}
}

// ForEntity returns the field policies for the role on a given entity.
// Fields without an explicit policy are not present in the map (treated as visible).
func (s *FieldService) ForEntity(ctx context.Context, roleID uuid.UUID, entity string) (map[string]FieldPolicy, error) {
	byEntity, err := s.forRole(ctx, roleID)
	if err != nil {
		return nil, err
	}
	return byEntity[entity], nil
}

func (s *FieldService) forRole(ctx context.Context, roleID uuid.UUID) (map[string]map[string]FieldPolicy, error) {
	key := "authz:fields:" + roleID.String()
	var cached map[string]map[string]FieldPolicy
	if cacheGetJSON(ctx, s.rdb, key, &cached) {
		return cached, nil
	}
	rows, err := s.q.ListFieldPermissionsByRole(ctx, roleID)
	if err != nil {
		return nil, err
	}
	byEntity := make(map[string]map[string]FieldPolicy)
	for _, r := range rows {
		if byEntity[r.Entity] == nil {
			byEntity[r.Entity] = make(map[string]FieldPolicy)
		}
		byEntity[r.Entity][r.Field] = FieldPolicy{CanView: r.CanView, CanEdit: r.CanEdit}
	}
	cacheSetJSON(ctx, s.rdb, key, byEntity, defaultTTL)
	return byEntity, nil
}

// FilterView removes fields the role may not view from a serialized record.
// Fields with no explicit policy stay visible (default-allow).
func FilterView(policies map[string]FieldPolicy, data map[string]any) {
	for field, p := range policies {
		if !p.CanView {
			delete(data, field)
		}
	}
}
