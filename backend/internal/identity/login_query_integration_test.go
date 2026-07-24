//go:build integration

package identity_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// TestGetUserByLogin_EmailOrUsername verifies the Fase 7 login lookup matches by
// username (NIP) and by email case-insensitively (citext), against a real DB with
// migration 000045 applied.
func TestGetUserByLogin_EmailOrUsername(t *testing.T) {
	pool := testsupport.NewPostgres(t)
	testsupport.Reset(t, pool)
	ctx := context.Background()
	q := sqlc.New(pool)

	var roleID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO identity.roles (code, name) VALUES ('r-login', 'Login Role') RETURNING id`).Scan(&roleID))
	var userID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO identity.users (name, email, username, role_id) VALUES ('Andi', 'andi@bank.local', 'NIP001', $1) RETURNING id`,
		roleID).Scan(&userID))

	byNip, err := q.GetUserByLogin(ctx, "NIP001")
	require.NoError(t, err)
	assert.Equal(t, userID, byNip.ID)

	byEmail, err := q.GetUserByLogin(ctx, "ANDI@BANK.LOCAL") // citext: case-insensitive
	require.NoError(t, err)
	assert.Equal(t, userID, byEmail.ID)

	_, err = q.GetUserByLogin(ctx, "does-not-exist")
	require.ErrorIs(t, err, pgx.ErrNoRows)
}
