//go:build integration

package authzadmin_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/internal/testsupport"
)

// TestSeed_BuiltinRolesUseEnforcedKeys verifies that the 000005 seed grants
// built-in roles the permission keys that the code actually enforces, and does
// NOT retain the stale keys (asset.create, asset.read, asset.update,
// asset.delete, asset.checkout, request.approve) from the old matrix.
//
// testsupport.NewPostgres applies all migrations (including the 000005 seed)
// and does NOT call Reset, so the seeded rows are present when assertions run.
func TestSeed_BuiltinRolesUseEnforcedKeys(t *testing.T) {
	pool := testsupport.NewPostgres(t) // applies all migrations incl. 000005; no truncation
	ctx := context.Background()

	has := func(roleCode, key string) bool {
		var n int
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT count(*) FROM identity.role_permissions rp
			JOIN identity.roles r ON r.id = rp.role_id
			WHERE r.code=$1 AND rp.permission_key=$2 AND rp.deleted_at IS NULL`, roleCode, key).Scan(&n))
		return n > 0
	}

	// Enforced keys that must be present.
	require.True(t, has("superadmin", "asset.manage"), "superadmin must have asset.manage")
	require.True(t, has("superadmin", "role.manage"), "superadmin must have role.manage")
	require.True(t, has("superadmin", "asset.view"), "superadmin must have asset.view")
	require.True(t, has("superadmin", "request.decide"), "superadmin must have request.decide")
	require.True(t, has("superadmin", "approval.config.manage"), "superadmin must have approval.config.manage")
	require.True(t, has("manager", "asset.manage"), "manager must have asset.manage")
	require.True(t, has("manager", "asset.view"), "manager must have asset.view")
	require.True(t, has("kepala_unit", "request.decide"), "kepala_unit must have request.decide")
	require.True(t, has("kepala_unit", "asset.view"), "kepala_unit must have asset.view")
	require.True(t, has("staf", "asset.view"), "staf must have asset.view")

	// Stale keys that must NOT be present.
	require.False(t, has("superadmin", "asset.create"), "stale asset.create must be gone from superadmin")
	require.False(t, has("superadmin", "asset.read"), "stale asset.read must be gone from superadmin")
	require.False(t, has("superadmin", "asset.update"), "stale asset.update must be gone from superadmin")
	require.False(t, has("superadmin", "asset.delete"), "stale asset.delete must be gone from superadmin")
	require.False(t, has("superadmin", "asset.checkout"), "stale asset.checkout must be gone from superadmin")
	require.False(t, has("superadmin", "request.approve"), "stale request.approve must be gone from superadmin")
	require.False(t, has("kepala_unit", "request.approve"), "stale request.approve must be gone from kepala_unit")
	require.False(t, has("manager", "asset.create"), "stale asset.create must be gone from manager")
	require.False(t, has("manager", "asset.read"), "stale asset.read must be gone from manager")
	require.False(t, has("staf", "asset.read"), "stale asset.read must be gone from staf")
}
