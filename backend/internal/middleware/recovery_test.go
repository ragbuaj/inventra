package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRecoveryReturns500AndLogsStructured(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	r := gin.New()
	r.Use(RequestID(), Recovery(log))
	r.GET("/boom", func(c *gin.Context) { panic("kaboom") })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.Header.Set(RequestHeaderID, "rid-9")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "internal server error") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "kaboom") || strings.Contains(w.Body.String(), "goroutine") {
		t.Fatalf("panic detail leaked to client: %s", w.Body.String())
	}
	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m); err != nil {
		t.Fatalf("parse %q: %v", buf.String(), err)
	}
	if m["request_id"] != "rid-9" || m["msg"] != "panic recovered" {
		t.Fatalf("log attrs: %v", m)
	}
	if _, ok := m["stack"]; !ok {
		t.Fatal("stack attr missing from log")
	}
	if m["error"] != "kaboom" {
		t.Fatalf("log error attr wrong: %v", m["error"])
	}
	if m["path"] != "/boom" {
		t.Fatalf("log path attr wrong: %v", m["path"])
	}
}

func TestRecoveryPassesNormalRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	r := gin.New()
	r.Use(RequestID(), Recovery(log))
	r.GET("/ok", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("normal request broke: %d", w.Code)
	}
}
