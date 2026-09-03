package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

const nsPrefix = "/api/v1"

// newNoStoreRouter mounts NoStore globally, exactly as router.go does, so the
// tests exercise the real mount point rather than a bare handler chain.
func newNoStoreRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(NoStore(nsPrefix))
	r.GET("/api/v1/assets", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"data": []string{}}) })
	r.GET("/api/v1", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v1beta/assets", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/openapi.yaml", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func nsGet(t *testing.T, r http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestNoStoreSetsHeaderUnderPrefix(t *testing.T) {
	w := nsGet(t, newNoStoreRouter(), "/api/v1/assets")
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-store")
	}
}

// The prefix itself is under the policy, not just its children.
func TestNoStoreSetsHeaderOnPrefixItself(t *testing.T) {
	w := nsGet(t, newNoStoreRouter(), "/api/v1")
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-store")
	}
}

// Matching must be on the path SEGMENT: /api/v1beta is a different API, and a
// naive strings.HasPrefix would sweep it in.
func TestNoStoreIgnoresPrefixLookalikePath(t *testing.T) {
	w := nsGet(t, newNoStoreRouter(), "/api/v1beta/assets")
	if got := w.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("Cache-Control = %q on a lookalike path, want empty", got)
	}
}

// Routes outside the API surface (health probes, the OpenAPI spec, the docs
// viewer, /metrics) are left alone — they carry no authenticated data and some
// of them want to stay cacheable.
func TestNoStoreLeavesNonAPIRoutesAlone(t *testing.T) {
	r := newNoStoreRouter()
	for _, path := range []string{"/health", "/openapi.yaml"} {
		if got := nsGet(t, r, path).Header().Get("Cache-Control"); got != "" {
			t.Fatalf("%s: Cache-Control = %q, want empty", path, got)
		}
	}
}

// "Apa pun status codenya": the header rides on every response the API can
// produce, including the ones a handler aborts before writing a body.
func TestNoStoreSetsHeaderOnEveryStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(NoStore(nsPrefix))
	r.GET("/api/v1/ok", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v1/created", func(c *gin.Context) { c.Status(http.StatusCreated) })
	r.GET("/api/v1/nocontent", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.GET("/api/v1/denied", func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	})
	r.GET("/api/v1/unauthorized", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	})
	r.GET("/api/v1/throttled", func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limited"})
	})
	r.GET("/api/v1/boom", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusInternalServerError)
	})

	cases := []struct {
		path string
		want int
	}{
		{"/api/v1/ok", http.StatusOK},
		{"/api/v1/created", http.StatusCreated},
		{"/api/v1/nocontent", http.StatusNoContent},
		{"/api/v1/denied", http.StatusForbidden},
		{"/api/v1/unauthorized", http.StatusUnauthorized},
		{"/api/v1/throttled", http.StatusTooManyRequests},
		{"/api/v1/boom", http.StatusInternalServerError},
	}
	for _, tc := range cases {
		w := nsGet(t, r, tc.path)
		if w.Code != tc.want {
			t.Fatalf("%s: status = %d, want %d", tc.path, w.Code, tc.want)
		}
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("%s (%d): Cache-Control = %q, want %q", tc.path, w.Code, got, "no-store")
		}
	}
}

// A request that matches no route never enters the /api/v1 route GROUP, which is
// exactly why the middleware is mounted globally and gates on the path.
func TestNoStoreSetsHeaderOnUnroutedAPIPaths(t *testing.T) {
	r := newNoStoreRouter()

	w := nsGet(t, r, "/api/v1/does-not-exist")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("404: Cache-Control = %q, want %q", got, "no-store")
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/assets", nil))
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("unrouted method: Cache-Control = %q, want %q", got, "no-store")
	}
}

// A handler that wants a stricter value must win, and the two values must not
// pile up into "no-store, private, no-store" (gin's c.Header is Set, not Add).
func TestNoStoreLetsHandlerRefineWithoutDuplicating(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(NoStore(nsPrefix))
	r.GET("/api/v1/users/me/avatar", func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		c.Status(http.StatusOK)
	})

	w := nsGet(t, r, "/api/v1/users/me/avatar")
	values := w.Header().Values("Cache-Control")
	if len(values) != 1 {
		t.Fatalf("Cache-Control values = %v, want exactly one", values)
	}
	if values[0] != "private, no-store" {
		t.Fatalf("Cache-Control = %q, want the handler's %q", values[0], "private, no-store")
	}
}

// The identical value set by a handler must also collapse to one header line.
func TestNoStoreDoesNotDuplicateIdenticalHandlerValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(NoStore(nsPrefix))
	r.GET("/api/v1/guide", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Status(http.StatusOK)
	})

	values := nsGet(t, r, "/api/v1/guide").Header().Values("Cache-Control")
	if len(values) != 1 || values[0] != "no-store" {
		t.Fatalf("Cache-Control values = %v, want exactly [no-store]", values)
	}
}

// A panicking handler still produces a response; the header is already in the
// header map by then, so it survives the unwind.
func TestNoStoreSurvivesPanicRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(NoStore(nsPrefix), Recovery(slog.New(slog.NewJSONHandler(io.Discard, nil))))
	r.GET("/api/v1/panic", func(c *gin.Context) { panic("boom") })

	w := nsGet(t, r, "/api/v1/panic")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("after panic: Cache-Control = %q, want %q", got, "no-store")
	}
}

// The policy is scoped by the prefix it is constructed with, not by a constant
// baked into the middleware.
func TestNoStoreHonoursConfiguredPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(NoStore("/internal"))
	r.GET("/internal/thing", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v1/thing", func(c *gin.Context) { c.Status(http.StatusOK) })

	if got := nsGet(t, r, "/internal/thing").Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("/internal/thing: Cache-Control = %q, want %q", got, "no-store")
	}
	if got := nsGet(t, r, "/api/v1/thing").Header().Get("Cache-Control"); got != "" {
		t.Fatalf("/api/v1/thing under a different prefix: Cache-Control = %q, want empty", got)
	}
}

// Every HTTP verb the API exposes is covered, not just GET.
func TestNoStoreCoversAllMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(NoStore(nsPrefix))
	h := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.GET("/api/v1/x", h)
	r.POST("/api/v1/x", h)
	r.PUT("/api/v1/x", h)
	r.PATCH("/api/v1/x", h)
	r.DELETE("/api/v1/x", h)

	for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(m, "/api/v1/x", nil))
		if got := w.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("%s: Cache-Control = %q, want %q", m, got, "no-store")
		}
	}
}

// A query string must not defeat the segment match.
func TestNoStoreMatchesPathIgnoringQuery(t *testing.T) {
	w := nsGet(t, newNoStoreRouter(), "/api/v1/assets?limit=20&offset=40")
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want %q", got, "no-store")
	}
}
