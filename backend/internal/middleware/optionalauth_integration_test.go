//go:build integration

package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/ragbuaj/inventra/internal/auth"
)

// The unit tests prove OptionalAuth degrades to guest on every broken input.
// These two prove the other half — that it actually resolves a live caller, and
// that a revoked session demotes rather than rejects. Both need a real token
// store, so they live behind the integration tag with the RequireAuth tests.

func TestOptionalAuth_LiveSessionResolvesCaller(t *testing.T) {
	ctx := context.Background()
	tm, store := newAuthDeps(t)

	pair, err := tm.Issue("user-1", "role-1", "sess-1", auth.AudienceWeb)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if err := store.SaveSession(ctx, "sess-1", auth.SessionMeta{UserID: "user-1", RefreshJTI: pair.RefreshJTI}, time.Hour); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	w := serveOptional(tm, store, "Bearer "+pair.AccessToken)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	var body struct {
		User  string `json:"user"`
		Role  string `json:"role"`
		Guest bool   `json:"guest"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, w.Body.String())
	}
	if body.Guest {
		t.Fatal("live session resolved as guest; attachments would stay hidden from a signed-in reader")
	}
	if body.User != "user-1" || body.Role != "role-1" {
		t.Fatalf("identity = %q/%q, want user-1/role-1", body.User, body.Role)
	}
}

func TestOptionalAuth_RevokedSessionDemotesInsteadOfRejecting(t *testing.T) {
	ctx := context.Background()
	tm, store := newAuthDeps(t)

	pair, err := tm.Issue("user-2", "role-1", "sess-2", auth.AudienceWeb)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if err := store.SaveSession(ctx, "sess-2", auth.SessionMeta{UserID: "user-2", RefreshJTI: pair.RefreshJTI}, time.Hour); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	if _, err := store.DeleteSession(ctx, "user-2", "sess-2"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	// RequireAuth answers 401 here. OptionalAuth must not: this is a public page,
	// and a 401 makes the browser client clear the session and redirect to login.
	w := serveOptional(tm, store, "Bearer "+pair.AccessToken)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for a revoked session on a public route: %s", w.Code, w.Body.String())
	}
	if !contains(w.Body.String(), `"guest":true`) {
		t.Fatalf("body = %s, want the revoked caller treated as a guest", w.Body.String())
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
