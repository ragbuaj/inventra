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
	"github.com/ragbuaj/inventra/internal/storage"
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
	return NewService(fs, tm, store, fm, nil, 30*time.Minute, "https://app", storage.NewFake(), 2*1024*1024), store
}

// --- Device sessions (Spec B) ---

func TestLogin_CreatesDeviceSession(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{"u@x.com": u}, byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc, _ := newIntegrationService(t, fs, &fakeMailer{})

	pair, _, err := svc.Login(context.Background(), "u@x.com", "oldpassword",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) Chrome/120.0 Safari/537.36", "8.8.8.8")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if pair.SID == "" {
		t.Fatal("login pair must carry a sid")
	}

	views, err := svc.ListSessions(context.Background(), u.ID, pair.SID)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(views) != 1 {
		t.Fatalf("want 1 session after login, got %d", len(views))
	}
	got := views[0]
	if got.ID != pair.SID || !got.Current {
		t.Fatalf("the single session must be the current one: %+v", got)
	}
	if got.Browser != "Chrome" || got.OS != "macOS" || got.DeviceType != deviceDesktop {
		t.Fatalf("device metadata mis-parsed: %+v", got)
	}
	if got.IPAddress != "8.8.8.8" {
		t.Fatalf("want IP 8.8.8.8, got %q", got.IPAddress)
	}
}

func TestRefresh_KeepsSameSessionAndBumpsLastSeen(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{"u@x.com": u}, byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc, _ := newIntegrationService(t, fs, &fakeMailer{})

	pair, _, err := svc.Login(context.Background(), "u@x.com", "oldpassword", "Chrome", "8.8.8.8")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	before, _ := svc.ListSessions(context.Background(), u.ID, pair.SID)

	refreshed, err := svc.Refresh(context.Background(), pair.RefreshToken, "Chrome", "8.8.8.8")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if refreshed.SID != pair.SID {
		t.Fatalf("refresh must preserve sid: was %q now %q", pair.SID, refreshed.SID)
	}
	after, err := svc.ListSessions(context.Background(), u.ID, refreshed.SID)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(after) != 1 {
		t.Fatalf("refresh must not create a new session, got %d", len(after))
	}
	if after[0].LastSeenAt.Before(before[0].LastSeenAt) {
		t.Fatalf("last_seen must not go backwards")
	}
}

func TestRevokeSession_KillsItAndForeignSidIsNotFound(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{"u@x.com": u}, byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc, store := newIntegrationService(t, fs, &fakeMailer{})

	// Two sessions for the same user (two logins).
	p1, _, _ := svc.Login(context.Background(), "u@x.com", "oldpassword", "Chrome", "8.8.8.8")
	p2, _, _ := svc.Login(context.Background(), "u@x.com", "oldpassword", "Safari", "9.9.9.9")

	// A sid the caller does not own → 404, and it must NOT delete anything.
	if err := svc.RevokeSession(context.Background(), u.ID, "someone-elses-sid"); err != ErrNotFound {
		t.Fatalf("want ErrNotFound for a foreign sid, got %v", err)
	}

	// Revoke p1's session while authenticated as p2 (current).
	if err := svc.RevokeSession(context.Background(), u.ID, p1.SID); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	if alive, _ := store.SessionAlive(context.Background(), p1.SID); alive {
		t.Fatal("revoked session must be dead (its access token would now 401)")
	}
	if alive, _ := store.SessionAlive(context.Background(), p2.SID); !alive {
		t.Fatal("the other session must still be alive")
	}
	// The revoked session's refresh token must also be invalidated.
	if _, err := svc.Refresh(context.Background(), p1.RefreshToken, "Chrome", "8.8.8.8"); err != ErrInvalidToken {
		t.Fatalf("revoked session must not refresh, got %v", err)
	}
}

func TestRevokeOtherSessions_KeepsOnlyCurrent(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{"u@x.com": u}, byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc, _ := newIntegrationService(t, fs, &fakeMailer{})

	_, _, _ = svc.Login(context.Background(), "u@x.com", "oldpassword", "Chrome", "1.1.1.1")
	_, _, _ = svc.Login(context.Background(), "u@x.com", "oldpassword", "Safari", "2.2.2.2")
	current, _, _ := svc.Login(context.Background(), "u@x.com", "oldpassword", "Edge", "3.3.3.3")

	revoked, err := svc.RevokeOtherSessions(context.Background(), u.ID, current.SID)
	if err != nil {
		t.Fatalf("RevokeOtherSessions: %v", err)
	}
	if revoked != 2 {
		t.Fatalf("want 2 revoked, got %d", revoked)
	}
	views, _ := svc.ListSessions(context.Background(), u.ID, current.SID)
	if len(views) != 1 || views[0].ID != current.SID {
		t.Fatalf("only the current session must remain, got %+v", views)
	}
}

func TestChangePassword_ClearsAllSessions(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{"u@x.com": u}, byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc, _ := newIntegrationService(t, fs, &fakeMailer{})

	cur, _, _ := svc.Login(context.Background(), "u@x.com", "oldpassword", "Chrome", "1.1.1.1")
	_, _, _ = svc.Login(context.Background(), "u@x.com", "oldpassword", "Safari", "2.2.2.2")

	if _, err := svc.ChangePassword(context.Background(), u.ID, "oldpassword", "brandnewpassword"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	views, err := svc.ListSessions(context.Background(), u.ID, cur.SID)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(views) != 0 {
		t.Fatalf("a password change must clear every session, got %d", len(views))
	}
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

func TestAdminInitiatePasswordReset_Success_SavesTokenAndSendsMail(t *testing.T) {
	u := activeUserEmail(t, "target@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	fm := &fakeMailer{}
	svc, store := newIntegrationService(t, fs, fm)

	email, err := svc.AdminInitiatePasswordReset(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("AdminInitiatePasswordReset: %v", err)
	}
	if email != "target@x.com" {
		t.Fatalf("want returned email target@x.com, got %q", email)
	}
	if fm.resetTo != "target@x.com" {
		t.Fatalf("want reset mail sent to target@x.com, got %q", fm.resetTo)
	}
	if fm.resetLink == "" || !strings.Contains(fm.resetLink, "token=") {
		t.Fatalf("want a reset link with a token, got %q", fm.resetLink)
	}
	// The saved token must be consumable exactly once and map to the target user.
	rawToken := fm.resetLink[strings.Index(fm.resetLink, "token=")+len("token="):]
	gotID, err := store.ConsumePasswordReset(context.Background(), auth.HashResetToken(rawToken))
	if err != nil {
		t.Fatalf("ConsumePasswordReset: %v", err)
	}
	if gotID != u.ID.String() {
		t.Fatalf("token maps to %q, want %q", gotID, u.ID.String())
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
