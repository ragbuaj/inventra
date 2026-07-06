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
