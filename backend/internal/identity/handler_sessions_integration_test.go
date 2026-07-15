//go:build integration

package identity

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// withSession stamps both the caller id and the current session id, as
// RequireAuth does for a token carrying a sid.
func withSession(userID uuid.UUID, sid string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.CtxUserID, userID.String())
		c.Set(middleware.CtxSessionID, sid)
		c.Next()
	}
}

// seedTwoSessions logs the user in twice against a real-Redis service and
// returns the handler plus the two session ids (the first is treated as current).
func seedTwoSessions(t *testing.T) (*Handler, uuid.UUID, string, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{"u@x.com": u}, byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc, _ := newIntegrationService(t, fs, &fakeMailer{})
	p1, _, err := svc.Login(context.Background(), "u@x.com", "oldpassword", "Chrome", "1.1.1.1")
	if err != nil {
		t.Fatalf("login 1: %v", err)
	}
	p2, _, err := svc.Login(context.Background(), "u@x.com", "oldpassword", "Safari", "2.2.2.2")
	if err != nil {
		t.Fatalf("login 2: %v", err)
	}
	return &Handler{svc: svc}, u.ID, p1.SID, p2.SID
}

func TestHandlerListSessions_MarksCurrent(t *testing.T) {
	h, userID, current, other := seedTwoSessions(t)
	r := gin.New()
	r.GET("/sessions", withSession(userID, current), h.listSessions)

	w := doJSON(t, r, http.MethodGet, "/sessions", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data []SessionView `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("want 2 sessions, got %d", len(resp.Data))
	}
	var sawCurrent bool
	for _, s := range resp.Data {
		if s.ID == current && s.Current {
			sawCurrent = true
		}
		if s.ID == other && s.Current {
			t.Fatalf("the non-current session must not be flagged current")
		}
	}
	if !sawCurrent {
		t.Fatalf("the current session must be flagged: %+v", resp.Data)
	}
}

func TestHandlerRevokeSession_OwnAndForeign(t *testing.T) {
	h, userID, current, other := seedTwoSessions(t)
	r := gin.New()
	r.DELETE("/sessions/:id", withSession(userID, current), h.revokeSession)

	// Revoke the other session — 200.
	w := doJSON(t, r, http.MethodDelete, "/sessions/"+other, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 revoking own session, got %d: %s", w.Code, w.Body.String())
	}
	// A sid the caller does not own — 404.
	w = doJSON(t, r, http.MethodDelete, "/sessions/not-mine", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for a foreign sid, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerRevokeOtherSessions_KeepsCurrent(t *testing.T) {
	h, userID, current, _ := seedTwoSessions(t)
	r := gin.New()
	r.POST("/sessions/revoke-others", withSession(userID, current), h.revokeOtherSessions)

	w := doJSON(t, r, http.MethodPost, "/sessions/revoke-others", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Revoked int `json:"revoked"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Revoked != 1 {
		t.Fatalf("want 1 revoked, got %d", resp.Revoked)
	}
	// Only the current session remains.
	views, err := h.svc.ListSessions(context.Background(), userID, current)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(views) != 1 || views[0].ID != current {
		t.Fatalf("only the current session must remain, got %+v", views)
	}
}
