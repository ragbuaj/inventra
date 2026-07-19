package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// signRaw builds a token straight from the given registered claims with the
// test manager's secret, bypassing Issue — used to craft legacy (no-aud) and
// hostile (wrong issuer / foreign aud) tokens.
func signRaw(t *testing.T, reg jwt.RegisteredClaims) string {
	t.Helper()
	claims := Claims{RegisteredClaims: reg, Type: TokenAccess}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("unit-test-secret"))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func TestIssueAndParse_AudienceRoundtrip(t *testing.T) {
	tm := testManager()
	for _, aud := range []string{AudienceWeb, AudienceMobile} {
		pair, err := tm.Issue("user-1", "role-1", "sid-1", aud)
		if err != nil {
			t.Fatalf("issue %s: %v", aud, err)
		}
		if pair.Audience != aud {
			t.Fatalf("pair.Audience: want %q, got %q", aud, pair.Audience)
		}
		access, err := tm.Parse(pair.AccessToken)
		if err != nil {
			t.Fatalf("parse access (%s): %v", aud, err)
		}
		if access.ClientAudience() != aud {
			t.Fatalf("access audience: want %q, got %q", aud, access.ClientAudience())
		}
		// The refresh token must carry the SAME audience — a client cannot
		// switch identity across a rotation without logging in again.
		refresh, err := tm.Parse(pair.RefreshToken)
		if err != nil {
			t.Fatalf("parse refresh (%s): %v", aud, err)
		}
		if refresh.ClientAudience() != aud {
			t.Fatalf("refresh audience: want %q, got %q", aud, refresh.ClientAudience())
		}
	}
}

func TestIssue_UnknownAudienceRejected(t *testing.T) {
	tm := testManager()
	if _, err := tm.Issue("user-1", "role-1", "sid-1", "desktop"); err == nil {
		t.Fatal("issuing an unknown audience must fail, not silently widen to web")
	}
}

func TestIssue_EmptyAudienceDefaultsToWeb(t *testing.T) {
	tm := testManager()
	pair, err := tm.Issue("user-1", "role-1", "sid-1", "")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if pair.Audience != AudienceWeb {
		t.Fatalf("want default audience web, got %q", pair.Audience)
	}
}

func TestParse_WrongIssuerRejected(t *testing.T) {
	tm := testManager()
	now := time.Now()
	token := signRaw(t, jwt.RegisteredClaims{
		Issuer:    "not-inventra",
		Subject:   "user-1",
		Audience:  jwt.ClaimStrings{AudienceWeb},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
	})
	if _, err := tm.Parse(token); err == nil {
		t.Fatal("a token from a foreign issuer must be rejected")
	}
}

func TestParse_UnknownAudienceRejected(t *testing.T) {
	tm := testManager()
	now := time.Now()
	token := signRaw(t, jwt.RegisteredClaims{
		Issuer:    "inventra",
		Subject:   "user-1",
		Audience:  jwt.ClaimStrings{"desktop"},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
	})
	if _, err := tm.Parse(token); err == nil {
		t.Fatal("a token with an unrecognized aud must be rejected")
	}
}

// A token issued before audiences existed (live production sessions at
// rollout) has no aud; it must still parse and be treated as a web client.
func TestParse_MissingAudienceIsWeb(t *testing.T) {
	tm := testManager()
	now := time.Now()
	token := signRaw(t, jwt.RegisteredClaims{
		Issuer:    "inventra",
		Subject:   "user-1",
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
	})
	claims, err := tm.Parse(token)
	if err != nil {
		t.Fatalf("a legacy no-aud token must remain valid, got %v", err)
	}
	if claims.ClientAudience() != AudienceWeb {
		t.Fatalf("legacy token audience: want web, got %q", claims.ClientAudience())
	}
}
