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
	// keys, when non-nil, makes Has report true only for keys in the set (used
	// to exercise RequireAnyPermission's per-key OR semantics). When nil, Has
	// falls back to the flat allow flag.
	keys map[string]bool
	err  error
}

func (f fakeChecker) Has(_ context.Context, _ uuid.UUID, key string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	if f.keys != nil {
		return f.keys[key], nil
	}
	return f.allow, nil
}
func (f fakeChecker) List(context.Context, uuid.UUID) ([]string, error) { return nil, nil }

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

func runRequireAnyPermission(checker fakeChecker, roleID string, keys ...string) int {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if roleID != "" {
		c.Set(CtxRoleID, roleID)
	}
	RequireAnyPermission(checker, keys...)(c)
	return w.Code // 200 when c.Next() runs without writing a status
}

func TestRequireAnyPermission(t *testing.T) {
	role := uuid.NewString()
	keys := []string{"role.manage", "scope.manage", "fieldperm.manage"}

	// Granted when the role holds one of several keys (the last one here), even
	// though it lacks the others.
	granted := fakeChecker{keys: map[string]bool{"fieldperm.manage": true}}
	if code := runRequireAnyPermission(granted, role, keys...); code != http.StatusOK {
		t.Fatalf("one of several granted: want 200, got %d", code)
	}

	// 403 when the role holds none of the keys.
	none := fakeChecker{keys: map[string]bool{"asset.view": true}}
	if code := runRequireAnyPermission(none, role, keys...); code != http.StatusForbidden {
		t.Fatalf("none granted: want 403, got %d", code)
	}

	// 401 when no role is present in the context.
	if code := runRequireAnyPermission(granted, "", keys...); code != http.StatusUnauthorized {
		t.Fatalf("no role: want 401, got %d", code)
	}

	// 500 when the checker errors.
	if code := runRequireAnyPermission(fakeChecker{err: errors.New("boom")}, role, keys...); code != http.StatusInternalServerError {
		t.Fatalf("checker error: want 500, got %d", code)
	}
}
