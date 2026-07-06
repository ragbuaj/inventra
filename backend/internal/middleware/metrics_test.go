package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestMetricsMiddlewareRecordsRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(Metrics())
	r.GET("/api/v1/things/:id", func(c *gin.Context) { c.Status(http.StatusOK) })
	// promhttp.Handler() scrapes the default gatherer, where the promauto
	// collectors in the observability package are registered.
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// exercise the instrumented route
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/things/42", nil))

	// scrape /metrics and assert the counter recorded the ROUTE TEMPLATE, not /42
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := w.Body.String()
	if !strings.Contains(body, `http_requests_total{method="GET",route="/api/v1/things/:id",status="200"} 1`) {
		t.Fatalf("counter not recorded with route template; body:\n%s", body)
	}
	if !strings.Contains(body, "http_request_duration_seconds_bucket") {
		t.Fatalf("duration histogram missing; body:\n%s", body)
	}
}

// TestMetricsMiddlewareRecordsPanicAs500 guards the middleware registration order
// in internal/server/router.go: Metrics must be registered BEFORE Recovery so that
// Metrics is the OUTER wrapper. Gin unwinds middleware post-c.Next() code in
// reverse registration order, so when a handler panics, Recovery (inner-registered-
// later would be wrong here; it must be the one closer to the handler) catches it
// and sets status 500, then control returns up through Metrics' post-c.Next() code,
// which records the request with status="500". If Metrics were registered after
// Recovery (inner wrapper), the panic would unwind through Metrics before Recovery
// catches it, skipping the recording entirely — this test would then fail.
func TestMetricsMiddlewareRecordsPanicAs500(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	// Registration order matters: Metrics first (outer), then Recovery (inner) —
	// matching the fixed router.go ordering.
	r.Use(Metrics())
	r.Use(gin.Recovery())
	r.GET("/api/v1/boom", func(c *gin.Context) { panic("boom") })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/boom", nil))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := w.Body.String()
	if !strings.Contains(body, `http_requests_total{method="GET",route="/api/v1/boom",status="500"} 1`) {
		t.Fatalf("panic-induced 500 not recorded; body:\n%s", body)
	}
}
