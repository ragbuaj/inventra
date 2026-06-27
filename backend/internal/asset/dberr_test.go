package asset

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapDBError(t *testing.T) {
	t.Run("nil passes through as nil", func(t *testing.T) {
		if got := mapDBError(nil); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("pgx.ErrNoRows → ErrNotFound", func(t *testing.T) {
		if got := mapDBError(pgx.ErrNoRows); !errors.Is(got, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", got)
		}
	})

	t.Run("23505 unique violation → ErrConflict", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23505"}
		if got := mapDBError(err); !errors.Is(got, ErrConflict) {
			t.Fatalf("expected ErrConflict, got %v", got)
		}
	})

	t.Run("23503 foreign key violation → ErrInvalidRef", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23503"}
		if got := mapDBError(err); !errors.Is(got, ErrInvalidRef) {
			t.Fatalf("expected ErrInvalidRef, got %v", got)
		}
	})

	t.Run("23514 check constraint violation → ErrRoomRequired", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23514"}
		if got := mapDBError(err); !errors.Is(got, ErrRoomRequired) {
			t.Fatalf("expected ErrRoomRequired, got %v", got)
		}
	})

	t.Run("unknown error passes through unchanged", func(t *testing.T) {
		sentinel := errors.New("some other error")
		if got := mapDBError(sentinel); !errors.Is(got, sentinel) {
			t.Fatalf("expected original error, got %v", got)
		}
	})
}
