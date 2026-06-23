package authz

import (
	"context"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/ragbuaj/inventra/db/sqlc"
)

// Scope is the effective data-access scope for a caller in a given module.
type Scope struct {
	Level     sqlc.SharedScopeLevel `json:"level"`
	OfficeIDs []uuid.UUID           `json:"office_ids,omitempty"` // for office / office_subtree
}

// ScopeService resolves the configurable per-row data scope (data_scope_policies)
// and computes office subtrees, both cached in Redis.
type ScopeService struct {
	q   *sqlc.Queries
	rdb *redis.Client
}

// NewScopeService builds a ScopeService.
func NewScopeService(q *sqlc.Queries, rdb *redis.Client) *ScopeService {
	return &ScopeService{q: q, rdb: rdb}
}

// Resolve returns the effective scope for the role in the module, given the
// caller's placement office (nil for global accounts such as superadmin).
func (s *ScopeService) Resolve(ctx context.Context, roleID uuid.UUID, officeID *uuid.UUID, module string) (Scope, error) {
	level, err := s.effectiveLevel(ctx, roleID, module)
	if err != nil {
		return Scope{}, err
	}

	switch level {
	case sqlc.SharedScopeLevelGlobal:
		return Scope{Level: level}, nil
	case sqlc.SharedScopeLevelOwn:
		return Scope{Level: level}, nil
	case sqlc.SharedScopeLevelOffice:
		if officeID == nil {
			return Scope{Level: sqlc.SharedScopeLevelOwn}, nil // no placement -> safest
		}
		return Scope{Level: level, OfficeIDs: []uuid.UUID{*officeID}}, nil
	case sqlc.SharedScopeLevelOfficeSubtree:
		if officeID == nil {
			return Scope{Level: sqlc.SharedScopeLevelOwn}, nil
		}
		ids, err := s.subtree(ctx, *officeID)
		if err != nil {
			return Scope{}, err
		}
		return Scope{Level: level, OfficeIDs: ids}, nil
	default:
		return Scope{Level: sqlc.SharedScopeLevelOwn}, nil
	}
}

// effectiveLevel picks the per-module override if present, else the role default ('*').
func (s *ScopeService) effectiveLevel(ctx context.Context, roleID uuid.UUID, module string) (sqlc.SharedScopeLevel, error) {
	policies, err := s.policies(ctx, roleID)
	if err != nil {
		return "", err
	}
	var def sqlc.SharedScopeLevel = sqlc.SharedScopeLevelOwn // conservative fallback
	found := false
	for _, p := range policies {
		if p.Module == module {
			return p.ScopeLevel, nil
		}
		if p.Module == "*" {
			def, found = p.ScopeLevel, true
		}
	}
	if found {
		return def, nil
	}
	return def, nil
}

func (s *ScopeService) policies(ctx context.Context, roleID uuid.UUID) ([]sqlc.IdentityDataScopePolicy, error) {
	key := "authz:scope:" + roleID.String()
	var cached []sqlc.IdentityDataScopePolicy
	if cacheGetJSON(ctx, s.rdb, key, &cached) {
		return cached, nil
	}
	policies, err := s.q.ListDataScopePolicies(ctx, roleID)
	if err != nil {
		return nil, err
	}
	cacheSetJSON(ctx, s.rdb, key, policies, defaultTTL)
	return policies, nil
}

func (s *ScopeService) subtree(ctx context.Context, officeID uuid.UUID) ([]uuid.UUID, error) {
	key := "authz:subtree:" + officeID.String()
	var cached []uuid.UUID
	if cacheGetJSON(ctx, s.rdb, key, &cached) {
		return cached, nil
	}
	ids, err := s.q.GetOfficeSubtree(ctx, officeID)
	if err != nil {
		return nil, err
	}
	cacheSetJSON(ctx, s.rdb, key, ids, defaultTTL)
	return ids, nil
}
