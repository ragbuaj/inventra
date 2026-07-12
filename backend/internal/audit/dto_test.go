package audit

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
)

// TestAuditToMap_IncludesActorRoleAndOfficeName pins the serialized shape:
// actor.role and top-level office_name must be present when the underlying
// row has them (from the ListAuditLogs LEFT JOINs).
func TestAuditToMap_IncludesActorRoleAndOfficeName(t *testing.T) {
	actorID := uuid.New()
	officeID := uuid.New()
	name := "Auditor Satu"
	email := "auditor.satu@test.local"
	role := "auditor-role"
	officeName := "KP Test"

	r := sqlc.ListAuditLogsRow{
		ID:         uuid.New(),
		ActorID:    &actorID,
		EntityType: "office",
		EntityID:   uuid.New(),
		Action:     sqlc.SharedAuditActionCreate,
		OfficeID:   &officeID,
		ActorName:  &name,
		ActorEmail: &email,
		ActorRole:  &role,
		OfficeName: &officeName,
	}

	m := auditToMap(r)

	actor, ok := m["actor"].(map[string]any)
	require.True(t, ok, "actor should serialize as a map: %#v", m["actor"])
	actorRole, ok := actor["role"].(*string)
	require.True(t, ok, "actor.role should be a *string: %#v", actor["role"])
	require.NotNil(t, actorRole)
	assert.Equal(t, "auditor-role", *actorRole)
	assert.Equal(t, "KP Test", m["office_name"])
}

// TestAuditToMap_OfficeNameAndRoleNilWhenMissing covers the NULL-safe path:
// no actor at all (nil ActorID) and no matching office (nil OfficeName) must
// serialize as JSON null, not be omitted or panic.
func TestAuditToMap_OfficeNameAndRoleNilWhenMissing(t *testing.T) {
	r := sqlc.ListAuditLogsRow{
		ID:         uuid.New(),
		EntityType: "orphan",
		EntityID:   uuid.New(),
		Action:     sqlc.SharedAuditActionCreate,
		OfficeID:   nil,
		OfficeName: nil,
		CreatedAt:  pgtype.Timestamptz{},
	}

	m := auditToMap(r)

	assert.Nil(t, m["actor"])
	assert.Nil(t, m["office_name"])
	assert.Nil(t, m["office_id"])
}
