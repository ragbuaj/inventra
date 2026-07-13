//go:build integration

package identity

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/auth"
	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// newIntegrationService builds a Service over a REAL (throwaway, testcontainers)
// Redis so the store-touching success paths — which the fast-unreachable-redis
// unit tests in service_test.go cannot exercise past the store call — can be
// asserted end to end (token actually saved, consumable exactly once, mail
// sent to the right address).
func newIntegrationService(t *testing.T, fs *fakeStore, fm *fakeMailer) (*Service, *auth.TokenStore) {
	t.Helper()
	cfg := &config.Config{JWTSecret: "test-secret", JWTAccessTTL: 15 * time.Minute, JWTRefreshTTL: time.Hour}
	tm := auth.NewTokenManager(cfg)
	store := auth.NewTokenStore(testsupport.NewRedis(t))
	return NewService(fs, tm, store, fm, 30*time.Minute, "https://app"), store
}

func TestRequestEmailChange_Success_SavesTokenAndNotifiesNewEmail(t *testing.T) {
	u := activeUserEmail(t, "old@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	fm := &fakeMailer{}
	svc, _ := newIntegrationService(t, fs, fm)

	if err := svc.RequestEmailChange(context.Background(), u.ID, "new@x.com", "oldpassword"); err != nil {
		t.Fatalf("RequestEmailChange: %v", err)
	}
	if fm.verifyTo != "new@x.com" {
		t.Fatalf("verify link must be emailed to the NEW address, got %q", fm.verifyTo)
	}
	if fm.verifyLink == "" || !strings.Contains(fm.verifyLink, "token=") {
		t.Fatalf("want a verify link with a token, got %q", fm.verifyLink)
	}
}

func TestConfirmEmailChange_Success_UpdatesEmailAndNotifiesOldAddress(t *testing.T) {
	u := activeUserEmail(t, "old@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	fm := &fakeMailer{}
	svc, store := newIntegrationService(t, fs, fm)

	raw, hash, err := auth.GenerateEmailChangeToken()
	if err != nil {
		t.Fatalf("GenerateEmailChangeToken: %v", err)
	}
	if err := store.SaveEmailChange(context.Background(), hash, u.ID.String(), "new@x.com", time.Minute); err != nil {
		t.Fatalf("SaveEmailChange: %v", err)
	}

	updated, err := svc.ConfirmEmailChange(context.Background(), raw)
	if err != nil {
		t.Fatalf("ConfirmEmailChange: %v", err)
	}
	if updated.Email != "new@x.com" {
		t.Fatalf("want email updated to new@x.com, got %q", updated.Email)
	}
	if fs.emailUpdates[u.ID] != "new@x.com" {
		t.Fatalf("want UpdateUserEmail called with new@x.com, got %q", fs.emailUpdates[u.ID])
	}
	if fm.emailChangedTo != "old@x.com" {
		t.Fatalf("the OLD address must be notified of the change, got %q", fm.emailChangedTo)
	}

	// Single-use: confirming again with the same raw token must fail.
	if _, err := svc.ConfirmEmailChange(context.Background(), raw); err != ErrInvalidToken {
		t.Fatalf("want ErrInvalidToken on token replay, got %v", err)
	}
}

func TestConfirmEmailChange_EmailTakenMeanwhile_ErrEmailInUse(t *testing.T) {
	u := activeUserEmail(t, "old@x.com")
	other := activeUserEmail(t, "new@x.com") // someone else grabbed it after the request
	fs := &fakeStore{
		byID:    map[uuid.UUID]sqlc.IdentityUser{u.ID: u, other.ID: other},
		byEmail: map[string]sqlc.IdentityUser{"new@x.com": other},
	}
	fm := &fakeMailer{}
	svc, store := newIntegrationService(t, fs, fm)

	raw, hash, err := auth.GenerateEmailChangeToken()
	if err != nil {
		t.Fatalf("GenerateEmailChangeToken: %v", err)
	}
	if err := store.SaveEmailChange(context.Background(), hash, u.ID.String(), "new@x.com", time.Minute); err != nil {
		t.Fatalf("SaveEmailChange: %v", err)
	}

	if _, err := svc.ConfirmEmailChange(context.Background(), raw); err != ErrEmailInUse {
		t.Fatalf("want ErrEmailInUse, got %v", err)
	}
}

func TestRequestPasswordChange_Success_SavesTokenAndSendsMail(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	fm := &fakeMailer{}
	svc, _ := newIntegrationService(t, fs, fm)

	if err := svc.RequestPasswordChange(context.Background(), u.ID, "oldpassword"); err != nil {
		t.Fatalf("RequestPasswordChange: %v", err)
	}
	if fm.resetTo != "u@x.com" {
		t.Fatalf("want reset mail sent to u@x.com, got %q", fm.resetTo)
	}
	if fm.resetLink == "" || !strings.Contains(fm.resetLink, "token=") {
		t.Fatalf("want a reset link with a token, got %q", fm.resetLink)
	}
}
