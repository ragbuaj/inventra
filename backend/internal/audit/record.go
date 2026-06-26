package audit

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// Record writes a best-effort audit entry for a successful mutation. The actor is
// taken from the auth context and the IP from the request. A failed insert is
// logged and swallowed — auditing must never break the user's operation.
// A nil svc is a no-op (keeps wiring optional / test-friendly).
func Record(c *gin.Context, svc *Service, action Action, entityType string, entityID uuid.UUID, officeID *uuid.UUID, changes any) {
	if svc == nil {
		return
	}
	var actor *uuid.UUID
	if uid, err := uuid.Parse(c.GetString(middleware.CtxUserID)); err == nil {
		actor = &uid
	}
	if err := svc.Log(c.Request.Context(), LogInput{
		ActorID:    actor,
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		Changes:    changes,
		IP:         c.ClientIP(),
		OfficeID:   officeID,
	}); err != nil {
		slog.Warn("audit log write failed", "entity_type", entityType, "entity_id", entityID, "action", action, "error", err)
	}
}

// callerScope resolves the caller's office data-scope for the audit module,
// mirroring masterdata's scopedDeps.callerOfficeScope.
func callerScope(c *gin.Context, q *sqlc.Queries, scope *authz.ScopeService) (bool, []uuid.UUID, error) {
	uid, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		return false, nil, err
	}
	user, err := q.GetUserByID(c.Request.Context(), uid)
	if err != nil {
		return false, nil, err
	}
	sc, err := scope.Resolve(c.Request.Context(), user.RoleID, user.OfficeID, "audit")
	if err != nil {
		return false, nil, err
	}
	switch sc.Level {
	case sqlc.SharedScopeLevelGlobal:
		return true, nil, nil
	case sqlc.SharedScopeLevelOwn:
		if user.OfficeID != nil {
			return false, []uuid.UUID{*user.OfficeID}, nil
		}
		return false, []uuid.UUID{}, nil
	default: // office / office_subtree
		return false, sc.OfficeIDs, nil
	}
}
