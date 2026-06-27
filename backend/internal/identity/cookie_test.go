package identity

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSetRefreshCookieAttributes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setRefreshCookie(c, "rt-123", time.Hour, false)
	sc := w.Header().Get("Set-Cookie")
	for _, want := range []string{"inventra_refresh=rt-123", "HttpOnly", "Path=/api/v1/auth", "SameSite=Lax", "Max-Age=3600"} {
		if !strings.Contains(sc, want) {
			t.Fatalf("Set-Cookie missing %q: %s", want, sc)
		}
	}
	if strings.Contains(sc, "Secure") {
		t.Fatalf("Secure must be absent when secure=false: %s", sc)
	}
}

func TestSetRefreshCookieSecureFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	setRefreshCookie(c, "rt", time.Hour, true)
	if !strings.Contains(w.Header().Get("Set-Cookie"), "Secure") {
		t.Fatalf("Secure must be present when secure=true: %s", w.Header().Get("Set-Cookie"))
	}
}

func TestClearRefreshCookieExpires(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	clearRefreshCookie(c, false)
	sc := w.Header().Get("Set-Cookie")
	if !strings.Contains(sc, "inventra_refresh=") || !strings.Contains(sc, "Max-Age=0") {
		t.Fatalf("clear should expire the cookie (Max-Age=0): %s", sc)
	}
}
