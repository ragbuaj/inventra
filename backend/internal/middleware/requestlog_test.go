package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func bufLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), &buf
}

func TestRequestIDGeneratesAndEchoes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	if w.Header().Get(RequestHeaderID) == "" {
		t.Fatal("expected a generated X-Request-ID echoed in the response")
	}
}

func TestRequestIDPreservesInbound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(RequestHeaderID, "abc-123")
	r.ServeHTTP(w, req)
	if got := w.Header().Get(RequestHeaderID); got != "abc-123" {
		t.Fatalf("inbound id not preserved: %s", got)
	}
}

func TestRequestLoggerEmitsStructuredLine(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log, buf := bufLogger()
	r := gin.New()
	r.Use(RequestID(), RequestLogger(log))
	r.GET("/x", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(RequestHeaderID, "rid-1")
	r.ServeHTTP(w, req)

	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m); err != nil {
		t.Fatalf("parse %q: %v", buf.String(), err)
	}
	if m["request_id"] != "rid-1" || m["method"] != "GET" || m["path"] != "/x" {
		t.Fatalf("missing attrs: %v", m)
	}
	if _, ok := m["latency_ms"]; !ok {
		t.Fatal("latency_ms missing")
	}
	if m["status"].(float64) != 200 {
		t.Fatalf("status: %v", m["status"])
	}
}

func TestRequestLoggerSkipsHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log, buf := bufLogger()
	r := gin.New()
	r.Use(RequestID(), RequestLogger(log))
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if buf.Len() != 0 {
		t.Fatalf("/health must not be logged: %s", buf.String())
	}
}

func TestRequestLoggerLevelByStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log, buf := bufLogger()
	r := gin.New()
	r.Use(RequestID(), RequestLogger(log))
	r.GET("/boom", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m["level"] != "ERROR" {
		t.Fatalf("status 500 must log at ERROR, got %v", m["level"])
	}
}

func TestRequestLoggerIncludesUserWhenSet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	log, buf := bufLogger()
	r := gin.New()
	r.Use(RequestID(), RequestLogger(log))
	r.GET("/u", func(c *gin.Context) {
		c.Set(CtxUserID, "user-7")
		c.Set(CtxRoleID, "role-3")
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/u", nil))
	var m map[string]any
	json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m)
	if m["user_id"] != "user-7" || m["role_id"] != "role-3" {
		t.Fatalf("user/role not logged: %v", m)
	}
}
