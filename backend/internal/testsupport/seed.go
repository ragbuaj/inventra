//go:build integration

package testsupport

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
)

// OfficeTree holds the IDs seeded by SeedOfficeTree. Shape:
//
//	Pusat
//	├── Wilayah  → Cabang
//	└── Wilayah2 → Cabang2
type OfficeTree struct {
	OfficeTypeID uuid.UUID
	Pusat        uuid.UUID
	Wilayah      uuid.UUID
	Cabang       uuid.UUID
	Wilayah2     uuid.UUID
	Cabang2      uuid.UUID
}

// SeedOfficeTree inserts one office type and a two-branch office hierarchy,
// returning their IDs. Call testsupport.Reset first if reusing a pool.
func SeedOfficeTree(t *testing.T, pool *pgxpool.Pool) OfficeTree {
	t.Helper()
	ctx := context.Background()

	var tree OfficeTree
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO masterdata.office_types (name) VALUES ('Kantor') RETURNING id`).
		Scan(&tree.OfficeTypeID))

	ins := func(name, code string, parent *uuid.UUID) uuid.UUID {
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO masterdata.offices (parent_id, office_type_id, name, code)
			 VALUES ($1, $2, $3, $4) RETURNING id`,
			parent, tree.OfficeTypeID, name, code).Scan(&id))
		return id
	}

	tree.Pusat = ins("Pusat", "P", nil)
	tree.Wilayah = ins("Wilayah 1", "W1", &tree.Pusat)
	tree.Cabang = ins("Cabang 1", "C1", &tree.Wilayah)
	tree.Wilayah2 = ins("Wilayah 2", "W2", &tree.Pusat)
	tree.Cabang2 = ins("Cabang 2", "C2", &tree.Wilayah2)
	return tree
}

// SeedRole inserts an identity.roles row and returns its id.
func SeedRole(t *testing.T, pool *pgxpool.Pool, code string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.roles (code, name) VALUES ($1, $1) RETURNING id`,
		code).Scan(&id))
	return id
}

// SeedScopePolicy inserts an identity.data_scope_policies row for a role.
func SeedScopePolicy(t *testing.T, pool *pgxpool.Pool, roleID uuid.UUID, module string, level sqlc.SharedScopeLevel) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO identity.data_scope_policies (role_id, module, scope_level)
		 VALUES ($1, $2, $3)`,
		roleID, module, string(level))
	require.NoError(t, err)
}

// SeedEmployee inserts a masterdata.employees row in the given office (status active)
// and returns its id.
func SeedEmployee(t *testing.T, pool *pgxpool.Pool, officeID uuid.UUID, code string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.employees (code, name, office_id)
		 VALUES ($1, $1, $2) RETURNING id`,
		code, officeID).Scan(&id))
	return id
}

// SeedFloor inserts a masterdata.floors row in the given office and returns its id.
func SeedFloor(t *testing.T, pool *pgxpool.Pool, officeID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.floors (office_id, name) VALUES ($1, $2) RETURNING id`,
		officeID, name).Scan(&id))
	return id
}

// SeedRoom inserts a masterdata.rooms row on the given floor and returns its id.
func SeedRoom(t *testing.T, pool *pgxpool.Pool, floorID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.rooms (floor_id, name) VALUES ($1, $2) RETURNING id`,
		floorID, name).Scan(&id))
	return id
}

// SeedFieldPermission inserts an identity.field_permissions row for a role.
func SeedFieldPermission(t *testing.T, pool *pgxpool.Pool, roleID uuid.UUID, entity, field string, canView, canEdit bool) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO identity.field_permissions (role_id, entity, field, can_view, can_edit)
		 VALUES ($1, $2, $3, $4, $5)`,
		roleID, entity, field, canView, canEdit)
	require.NoError(t, err)
}
