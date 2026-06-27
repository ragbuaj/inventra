//go:build integration

package testsupport

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
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
