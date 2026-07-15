package auth

import (
	"testing"
	"time"

	"github.com/ragbuaj/inventra/internal/config"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("s3cret-pass")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "s3cret-pass" {
		t.Fatal("hash must not equal plaintext")
	}
	if !VerifyPassword(hash, "s3cret-pass") {
		t.Fatal("expected correct password to verify")
	}
	if VerifyPassword(hash, "wrong-pass") {
		t.Fatal("expected wrong password to fail")
	}
}

func testManager() *TokenManager {
	return NewTokenManager(&config.Config{
		JWTSecret:     "unit-test-secret",
		JWTAccessTTL:  15 * time.Minute,
		JWTRefreshTTL: time.Hour,
	})
}

func TestIssueAndParse(t *testing.T) {
	tm := testManager()
	pair, err := tm.Issue("user-123", "role-abc", "sess-xyz")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if pair.AccessJTI == pair.RefreshJTI {
		t.Fatal("access and refresh JTIs must differ")
	}
	if pair.SID != "sess-xyz" {
		t.Fatalf("expected pair.SID sess-xyz, got %q", pair.SID)
	}

	access, err := tm.Parse(pair.AccessToken)
	if err != nil {
		t.Fatalf("parse access: %v", err)
	}
	if access.Subject != "user-123" || access.RoleID != "role-abc" || access.Type != TokenAccess {
		t.Fatalf("unexpected access claims: %+v", access)
	}
	if access.SID != "sess-xyz" {
		t.Fatalf("expected access sid sess-xyz, got %q", access.SID)
	}

	refresh, err := tm.Parse(pair.RefreshToken)
	if err != nil {
		t.Fatalf("parse refresh: %v", err)
	}
	if refresh.Type != TokenRefresh {
		t.Fatalf("expected refresh type, got %q", refresh.Type)
	}
	// The sid must be identical on both tokens so a rotation preserves session identity.
	if refresh.SID != "sess-xyz" {
		t.Fatalf("expected refresh sid sess-xyz, got %q", refresh.SID)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	tm := testManager()
	if _, err := tm.Parse("not-a-jwt"); err == nil {
		t.Fatal("expected error parsing garbage token")
	}
}
