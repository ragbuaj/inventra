package identity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"github.com/ragbuaj/inventra/internal/oauth"
)

type fakeGoogle struct {
	url        string
	state      string
	urlErr     error
	email, sub string
	exErr      error
}

func (f fakeGoogle) AuthCodeURL(_ context.Context) (string, string, error) {
	return f.url, f.state, f.urlErr
}
func (f fakeGoogle) Exchange(_ context.Context, _, _ string) (string, string, error) {
	return f.email, f.sub, f.exErr
}

func newGoogleHandler(g googleAuth, store userStore) *Handler {
	h := &Handler{googleOAuth: g, frontendURL: "http://localhost:3000"}
	if store != nil {
		h.svc = newGoogleSvc(store) // from google_test.go
	}
	return h
}

func TestGoogleStartRedirectsToProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/auth/google", newGoogleHandler(fakeGoogle{url: "https://accounts.google.com/o/oauth2/v2/auth?x=1", state: "st"}, nil).googleStart)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/google", nil))
	if w.Code != http.StatusFound || !strings.Contains(w.Header().Get("Location"), "accounts.google.com") {
		t.Fatalf("expected 302 to Google, got %d %s", w.Code, w.Header().Get("Location"))
	}
}

func TestGoogleStartDisabledRedirectsError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/auth/google", newGoogleHandler(fakeGoogle{urlErr: oauth.ErrDisabled}, nil).googleStart)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/google", nil))
	loc := w.Header().Get("Location")
	if w.Code != http.StatusFound || !strings.Contains(loc, "oauth=error") || !strings.Contains(loc, "reason=disabled") {
		t.Fatalf("disabled start should redirect error: %d %s", w.Code, loc)
	}
}

func TestGoogleCallbackProviderError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/cb", newGoogleHandler(fakeGoogle{}, nil).googleCallback)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/cb?error=access_denied", nil))
	loc := w.Header().Get("Location")
	if w.Code != http.StatusFound || !strings.Contains(loc, "oauth=error") || !strings.Contains(loc, "reason=server") {
		t.Fatalf("provider error should redirect error reason=server: %d %s", w.Code, loc)
	}
}

func TestGoogleCallbackNotProvisioned(t *testing.T) {
	gin.SetMode(gin.TestMode)
	g := fakeGoogle{email: "x@y.com", sub: "sub"}
	h := newGoogleHandler(g, &fakeUserStore{getErr: pgx.ErrNoRows})
	r := gin.New()
	r.GET("/cb", h.googleCallback)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/cb?code=c&state=s", nil))
	loc := w.Header().Get("Location")
	if w.Code != http.StatusFound || !strings.Contains(loc, "reason=not_registered") {
		t.Fatalf("unprovisioned email should redirect reason=not_registered: %d %s", w.Code, loc)
	}
	// Never leak tokens/codes into the redirect URL.
	if strings.Contains(loc, "code=") || strings.Contains(loc, "token") {
		t.Fatalf("redirect leaked sensitive data: %s", loc)
	}
}
