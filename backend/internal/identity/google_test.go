package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/auth"
	"github.com/ragbuaj/inventra/internal/config"
)

// fakeUserStore implements the identity userStore seam.
type fakeUserStore struct {
	user    sqlc.IdentityUser
	getErr  error
	linked  *string
	linkErr error
}

func (f *fakeUserStore) GetUserByID(_ context.Context, _ uuid.UUID) (sqlc.IdentityUser, error) {
	return f.user, f.getErr
}
func (f *fakeUserStore) GetUserByEmail(_ context.Context, _ string) (sqlc.IdentityUser, error) {
	return f.user, f.getErr
}
func (f *fakeUserStore) LinkGoogleID(_ context.Context, p sqlc.LinkGoogleIDParams) error {
	f.linked = p.GoogleID
	return f.linkErr
}
func (f *fakeUserStore) UpdateUserPassword(_ context.Context, _ sqlc.UpdateUserPasswordParams) error {
	return nil
}

func newGoogleSvc(store userStore) *Service {
	cfg := &config.Config{JWTSecret: "test-secret-please-change", JWTAccessTTL: 15 * time.Minute, JWTRefreshTTL: time.Hour}
	// Unreachable Redis: issue()'s SaveRefresh returns an error fast (never panics),
	// so error-path tests run cleanly and the link side-effect is still observable.
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 50 * time.Millisecond})
	return NewService(store, auth.NewTokenManager(cfg), auth.NewTokenStore(rdb), &fakeMailer{}, 30*time.Minute, "https://app")
}

func activeUser() sqlc.IdentityUser {
	return sqlc.IdentityUser{ID: uuid.New(), Email: "a@b.com", RoleID: uuid.New(), Status: sqlc.SharedUserStatusActive}
}

func TestLoginWithGoogleNotProvisioned(t *testing.T) {
	svc := newGoogleSvc(&fakeUserStore{getErr: pgx.ErrNoRows})
	if _, _, err := svc.LoginWithGoogle(context.Background(), "x@y.com", "sub"); !errors.Is(err, ErrNotProvisioned) {
		t.Fatalf("expected ErrNotProvisioned, got %v", err)
	}
}

func TestLoginWithGoogleMismatch(t *testing.T) {
	u := activeUser()
	other := "another-sub"
	u.GoogleID = &other
	if _, _, err := newGoogleSvc(&fakeUserStore{user: u}).LoginWithGoogle(context.Background(), "a@b.com", "sub"); !errors.Is(err, ErrGoogleMismatch) {
		t.Fatalf("expected ErrGoogleMismatch, got %v", err)
	}
}

func TestLoginWithGoogleInactive(t *testing.T) {
	u := activeUser()
	u.Status = sqlc.SharedUserStatusInactive
	if _, _, err := newGoogleSvc(&fakeUserStore{user: u}).LoginWithGoogle(context.Background(), "a@b.com", "sub"); !errors.Is(err, ErrUserInactive) {
		t.Fatalf("expected ErrUserInactive, got %v", err)
	}
}

func TestLoginWithGoogleLinksWhenUnset(t *testing.T) {
	store := &fakeUserStore{user: activeUser()} // GoogleID nil → must link before issuing
	// issue() then fails (Redis unreachable) — irrelevant here; the assertion is
	// that an unlinked, active account gets its google_id linked.
	_, _, _ = newGoogleSvc(store).LoginWithGoogle(context.Background(), "a@b.com", "sub-123")
	if store.linked == nil || *store.linked != "sub-123" {
		t.Fatalf("expected LinkGoogleID called with sub-123, got %v", store.linked)
	}
}
