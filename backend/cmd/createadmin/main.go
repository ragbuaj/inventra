// Command createadmin bootstraps a superadmin user (first-run / dev).
//
//	go run ./cmd/createadmin -email admin@inventra.local -name "Admin" -password "secret123"
package main

import (
	"context"
	"flag"
	"log"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/auth"
	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/db"
)

func main() {
	email := flag.String("email", "", "admin email")
	name := flag.String("name", "Superadmin", "display name")
	password := flag.String("password", "", "admin password")
	roleCode := flag.String("role", "superadmin", "role code")
	flag.Parse()

	if *email == "" || *password == "" {
		log.Fatal("usage: createadmin -email <email> -password <password> [-name <name>]")
	}

	cfg := config.Load()
	ctx := context.Background()

	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("db pool: %v", err)
	}
	defer pool.Close()

	q := sqlc.New(pool)

	role, err := q.GetRoleByCode(ctx, *roleCode)
	if err != nil {
		log.Fatalf("role %q not found: %v", *roleCode, err)
	}

	hash, err := auth.HashPassword(*password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}

	user, err := q.CreateUser(ctx, sqlc.CreateUserParams{
		Name:         *name,
		Email:        *email,
		PasswordHash: &hash,
		RoleID:       role.ID,
	})
	if err != nil {
		log.Fatalf("create user: %v", err)
	}

	log.Printf("created %s user: id=%s email=%s", *roleCode, user.ID, user.Email)
}
