package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/ragbuaj/inventra/internal/auth"
)

// runRequireAudience executes the middleware with the given context audience
// ("" leaves CtxAudience unset). 200 means c.Next() ran without a written status.
func runRequireAudience(ctxAudience string, allowed ...string) int {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if ctxAudience != "" {
		c.Set(CtxAudience, ctxAudience)
	}
	RequireAudience(allowed...)(c)
	return w.Code
}

func TestRequireAudience_WebOnly(t *testing.T) {
	// Web caller passes a web-only gate.
	if code := runRequireAudience(auth.AudienceWeb, auth.AudienceWeb); code != http.StatusOK {
		t.Fatalf("web on web-only: want 200, got %d", code)
	}
	// Mobile caller is denied 403 (ADR-0017 deny list).
	if code := runRequireAudience(auth.AudienceMobile, auth.AudienceWeb); code != http.StatusForbidden {
		t.Fatalf("mobile on web-only: want 403, got %d", code)
	}
	// Absent audience (legacy token / pre-audience session) counts as web.
	if code := runRequireAudience("", auth.AudienceWeb); code != http.StatusOK {
		t.Fatalf("absent audience on web-only: want 200 (treated as web), got %d", code)
	}
}

func TestRequireAudience_MobileOnly(t *testing.T) {
	if code := runRequireAudience(auth.AudienceMobile, auth.AudienceMobile); code != http.StatusOK {
		t.Fatalf("mobile on mobile-only: want 200, got %d", code)
	}
	if code := runRequireAudience(auth.AudienceWeb, auth.AudienceMobile); code != http.StatusForbidden {
		t.Fatalf("web on mobile-only: want 403, got %d", code)
	}
	// Absent audience is web, so a mobile-only route denies it.
	if code := runRequireAudience("", auth.AudienceMobile); code != http.StatusForbidden {
		t.Fatalf("absent audience on mobile-only: want 403, got %d", code)
	}
}

func TestRequireAudience_MultipleAllowed(t *testing.T) {
	if code := runRequireAudience(auth.AudienceMobile, auth.AudienceWeb, auth.AudienceMobile); code != http.StatusOK {
		t.Fatalf("mobile on web+mobile: want 200, got %d", code)
	}
}
