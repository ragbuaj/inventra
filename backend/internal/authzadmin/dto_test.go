package authzadmin

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/ragbuaj/inventra/db/sqlc"
)

func TestRoleToMap(t *testing.T) {
	desc := "desc"
	r := sqlc.IdentityRole{
		ID: uuid.New(), Code: "auditor", Name: "Auditor", Description: &desc, IsSystem: false,
		CreatedAt: pgtype.Timestamptz{Valid: false},
	}
	m := roleToMap(r)
	if m["code"] != "auditor" || m["name"] != "Auditor" || m["is_system"] != false {
		t.Fatalf("unexpected map: %v", m)
	}
	if m["description"] != &desc {
		t.Fatalf("description not carried")
	}
}
