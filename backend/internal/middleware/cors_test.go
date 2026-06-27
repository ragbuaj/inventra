package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

const testOrigin = "http://localhost:3000"

func newCORSRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(testOrigin))
	r.POST("/auth/login", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestCORSAllowsConfiguredOrigin(t *testing.T) {
	r := newCORSRouter()

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.Header.Set("Origin", testOrigin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != testOrigin {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, testOrigin)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want \"true\"", got)
	}
}

func TestCORSAllowsAndExposesRequestID(t *testing.T) {
	r := newCORSRouter()

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.Header.Set("Origin", testOrigin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// The SPA sends X-Request-ID on every call (ADR-0002); without it in
	// Allow-Headers the browser blocks every authenticated request.
	if got := w.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "X-Request-ID") {
		t.Fatalf("Access-Control-Allow-Headers = %q, must include X-Request-ID", got)
	}
	if got := w.Header().Get("Access-Control-Expose-Headers"); !strings.Contains(got, "X-Request-ID") {
		t.Fatalf("Access-Control-Expose-Headers = %q, must include X-Request-ID", got)
	}
}

func TestCORSPreflightShortCircuits(t *testing.T) {
	r := newCORSRouter()

	req := httptest.NewRequest(http.MethodOptions, "/auth/login", nil)
	req.Header.Set("Origin", testOrigin)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatal("preflight response missing Access-Control-Allow-Methods")
	}
}

func TestCORSIgnoresUnknownOrigin(t *testing.T) {
	r := newCORSRouter()

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.Header.Set("Origin", "http://evil.example")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty for an unlisted origin", got)
	}
}
