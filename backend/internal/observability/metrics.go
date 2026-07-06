// Package observability holds Prometheus metrics for the API. The collectors
// auto-register on the default registry at package init via promauto, so they
// are registered exactly once no matter how many times NewRouter is called
// (avoids duplicate-registration panics in tests).
package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// RequestsTotal counts HTTP requests by method, route template, and status.
var RequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests processed, by method, route, and status code.",
	},
	[]string{"method", "route", "status"},
)

// RequestDuration observes request latency by method and route template.
var RequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds, by method and route.",
		Buckets: prometheus.DefBuckets,
	},
	[]string{"method", "route"},
)
