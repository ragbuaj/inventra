package common

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// OfficeScopeFor translates a user's placement into the (allScope, officeIDs)
// pair every scope-aware query takes, for the given data_scope_policies module.
// allScope=true means no office filter (global); otherwise rows are limited to
// the returned office IDs.
//
// This is the single implementation of the rule. It exists apart from
// CallerOfficeScope because background workers (the notification fan-out and
// sweeper, the approval notifiable inverse) resolve users outside any HTTP
// request and so have no Gin context to resolve a caller from.
//
// The "own" branch is the reason a bare ScopeService.Resolve is not enough:
// Resolve leaves OfficeIDs empty for "own", which a caller would read as "no
// offices" and silently apply a narrower filter than intended.
func OfficeScopeFor(ctx context.Context, scope *authz.ScopeService, roleID uuid.UUID, officeID *uuid.UUID, module string) (bool, []uuid.UUID, error) {
	sc, err := scope.Resolve(ctx, roleID, officeID, module)
	if err != nil {
		return false, nil, err
	}
	switch sc.Level {
	case sqlc.SharedScopeLevelGlobal:
		return true, nil, nil
	case sqlc.SharedScopeLevelOwn:
		// For org-structure data, "own" resolves to the caller's own office.
		if officeID != nil {
			return false, []uuid.UUID{*officeID}, nil
		}
		return false, []uuid.UUID{}, nil
	default: // office / office_subtree
		return false, sc.OfficeIDs, nil
	}
}

// ScopedDeps resolves the caller's office-based data scope for list/row filtering.
// Resource handlers embed it to translate the caller into (allScope, officeIDs)
// before calling their service with those scope parameters.
type ScopedDeps struct {
	Q     *sqlc.Queries
	Scope *authz.ScopeService
}

// CallerOfficeScope returns (allScope, officeIDs) for the caller in the module.
// allScope=true means no office filter (global). Otherwise rows are limited to
// the returned office IDs (the caller's office subtree, office, or own office).
func (d ScopedDeps) CallerOfficeScope(c *gin.Context, module string) (bool, []uuid.UUID, error) {
	uid, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		return false, nil, err
	}
	user, err := d.Q.GetUserByID(c.Request.Context(), uid)
	if err != nil {
		return false, nil, err
	}
	return OfficeScopeFor(c.Request.Context(), d.Scope, user.RoleID, user.OfficeID, module)
}

// InScope reports whether target is permitted under the caller's scope.
func InScope(all bool, ids []uuid.UUID, target uuid.UUID) bool {
	if all {
		return true
	}
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

// SamePtr reports whether two optional UUIDs are equal (both nil counts as equal).
func SamePtr(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
