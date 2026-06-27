package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ragbuaj/inventra/internal/config"
)

// The router must mount the correlation middleware: a request to /health
// returns 200 and the response carries an echoed X-Request-ID. Pool/Redis are
// nil here because /health touches neither; the feature-module constructors
// only store their deps.
func TestRouterHealthEchoesRequestID(t *testing.T) {
	d := Deps{
		Cfg: &config.Config{Env: "test"},
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	r := NewRouter(d)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("/health status: %d", w.Code)
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Fatal("router did not echo X-Request-ID — RequestID middleware not mounted")
	}
}
