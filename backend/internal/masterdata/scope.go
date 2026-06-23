package masterdata

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// scopedDeps resolves the caller's office-based data scope for list filtering.
type scopedDeps struct {
	q     *sqlc.Queries
	scope *authz.ScopeService
}

// callerOfficeScope returns (allScope, officeIDs) for the caller in the module.
// allScope=true means no office filter (global). Otherwise rows are limited to
// the returned office IDs (the caller's office subtree, office, or own office).
func (d scopedDeps) callerOfficeScope(c *gin.Context, module string) (bool, []uuid.UUID, error) {
	uid, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		return false, nil, err
	}
	user, err := d.q.GetUserByID(c.Request.Context(), uid)
	if err != nil {
		return false, nil, err
	}
	sc, err := d.scope.Resolve(c.Request.Context(), user.RoleID, user.OfficeID, module)
	if err != nil {
		return false, nil, err
	}

	switch sc.Level {
	case sqlc.SharedScopeLevelGlobal:
		return true, nil, nil
	case sqlc.SharedScopeLevelOwn:
		// For org-structure data, "own" resolves to the caller's own office.
		if user.OfficeID != nil {
			return false, []uuid.UUID{*user.OfficeID}, nil
		}
		return false, []uuid.UUID{}, nil
	default: // office / office_subtree
		return false, sc.OfficeIDs, nil
	}
}
