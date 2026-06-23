package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type fakeChecker struct {
	allow bool
	err   error
}

func (f fakeChecker) Has(context.Context, uuid.UUID, string) (bool, error) { return f.allow, f.err }
func (f fakeChecker) List(context.Context, uuid.UUID) ([]string, error)     { return nil, nil }

func runRequirePermission(checker fakeChecker, roleID string) int {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if roleID != "" {
		c.Set(CtxRoleID, roleID)
	}
	RequirePermission(checker, "asset.create")(c)
	return w.Code // 200 when c.Next() runs without writing a status
}

func TestRequirePermission(t *testing.T) {
	role := uuid.NewString()

	if code := runRequirePermission(fakeChecker{allow: true}, role); code != http.StatusOK {
		t.Fatalf("granted: want 200, got %d", code)
	}
	if code := runRequirePermission(fakeChecker{allow: false}, role); code != http.StatusForbidden {
		t.Fatalf("denied: want 403, got %d", code)
	}
	if code := runRequirePermission(fakeChecker{err: errors.New("boom")}, role); code != http.StatusInternalServerError {
		t.Fatalf("error: want 500, got %d", code)
	}
	if code := runRequirePermission(fakeChecker{allow: true}, ""); code != http.StatusUnauthorized {
		t.Fatalf("no role: want 401, got %d", code)
	}
}
