package identity

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/auth"
	"github.com/ragbuaj/inventra/internal/config"
)

type fakeStore struct {
	byEmail map[string]sqlc.IdentityUser
	byID    map[uuid.UUID]sqlc.IdentityUser
	updated map[uuid.UUID]string // userID -> new hash
}

func (f *fakeStore) GetUserByID(_ context.Context, id uuid.UUID) (sqlc.IdentityUser, error) {
	u, ok := f.byID[id]
	if !ok {
		return sqlc.IdentityUser{}, pgx.ErrNoRows
	}
	return u, nil
}
func (f *fakeStore) GetUserByEmail(_ context.Context, e string) (sqlc.IdentityUser, error) {
	u, ok := f.byEmail[e]
	if !ok {
		return sqlc.IdentityUser{}, pgx.ErrNoRows
	}
	return u, nil
}
func (f *fakeStore) LinkGoogleID(_ context.Context, _ sqlc.LinkGoogleIDParams) error { return nil }
func (f *fakeStore) UpdateUserPassword(_ context.Context, a sqlc.UpdateUserPasswordParams) error {
	if f.updated == nil {
		f.updated = map[uuid.UUID]string{}
	}
	f.updated[a.ID] = *a.PasswordHash
	return nil
}

type fakeMailer struct{ resetLink, changedTo string }

func (m *fakeMailer) SendPasswordReset(_ context.Context, _, _, link string) error {
	m.resetLink = link
	return nil
}
func (m *fakeMailer) SendPasswordChanged(_ context.Context, to, _ string) error {
	m.changedTo = to
	return nil
}

func activeUserEmail(t *testing.T, email string) sqlc.IdentityUser {
	t.Helper()
	h, _ := auth.HashPassword("oldpassword")
	return sqlc.IdentityUser{ID: uuid.New(), Email: email, Name: "Budi", Status: sqlc.SharedUserStatusActive, PasswordHash: &h}
}

func newTestService(t *testing.T, fs *fakeStore, fm *fakeMailer) *Service {
	t.Helper()
	cfg := &config.Config{JWTSecret: "test-secret", JWTAccessTTL: 15 * time.Minute, JWTRefreshTTL: time.Hour}
	tm := auth.NewTokenManager(cfg)
	// Reset-token store needs a real Redis client; ResetPassword tests that need
	// it are integration-level. These unit tests exercise ChangePassword and
	// RequestPasswordReset (mailer/store fakes) + epoch logic only.
	return NewService(fs, tm, nil, fm, 30*time.Minute, "https://app")
}

func TestChangePassword_WrongOld(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc := newTestService(t, fs, &fakeMailer{})
	if _, err := svc.ChangePassword(context.Background(), u.ID, "nope", "brandnewpass"); err != ErrInvalidCredentials {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestChangePassword_WeakNew(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc := newTestService(t, fs, &fakeMailer{})
	if _, err := svc.ChangePassword(context.Background(), u.ID, "oldpassword", "short"); err != ErrWeakPassword {
		t.Fatalf("want ErrWeakPassword, got %v", err)
	}
}

func TestChangePassword_Success_UpdatesHashAndNotifies(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	fm := &fakeMailer{}
	svc := newTestService(t, fs, fm)
	if _, err := svc.ChangePassword(context.Background(), u.ID, "oldpassword", "brandnewpass"); err != nil {
		t.Fatalf("change: %v", err)
	}
	if _, ok := fs.updated[u.ID]; !ok {
		t.Fatalf("password not updated")
	}
	if fm.changedTo != "u@x.com" {
		t.Fatalf("notification not sent")
	}
}

func TestRequestPasswordReset_UnknownEmail_SilentOK(t *testing.T) {
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{}}
	fm := &fakeMailer{}
	svc := newTestService(t, fs, fm)
	if err := svc.RequestPasswordReset(context.Background(), "ghost@x.com"); err != nil {
		t.Fatalf("want nil (anti-enumeration), got %v", err)
	}
	if fm.resetLink != "" {
		t.Fatalf("no email should be sent for unknown account")
	}
}

func TestRequestPasswordReset_GoogleOnly_SilentOK(t *testing.T) {
	u := activeUserEmail(t, "g@x.com")
	u.PasswordHash = nil // Google-only
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{"g@x.com": u}}
	fm := &fakeMailer{}
	svc := newTestService(t, fs, fm)
	if err := svc.RequestPasswordReset(context.Background(), "g@x.com"); err != nil {
		t.Fatalf("got %v", err)
	}
	if fm.resetLink != "" {
		t.Fatalf("Google-only account must not receive a reset link")
	}
}
