//go:build integration

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/ragbuaj/inventra/internal/testsupport"
)

func newSessionStore(t *testing.T) *TokenStore {
	t.Helper()
	return NewTokenStore(testsupport.NewRedis(t))
}

func TestSessionStore_SaveListDelete(t *testing.T) {
	ctx := context.Background()
	store := newSessionStore(t)
	const user = "user-1"

	meta := SessionMeta{UserID: user, UserAgent: "Chrome", IP: "1.2.3.4", Location: "Jakarta, ID", RefreshJTI: "r1"}
	if err := store.SaveSession(ctx, "s1", meta, time.Hour); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	sessions, err := store.ListSessions(ctx, user)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("want 1 session, got %d", len(sessions))
	}
	got := sessions[0]
	if got.ID != "s1" || got.UserAgent != "Chrome" || got.IP != "1.2.3.4" || got.Location != "Jakarta, ID" || got.RefreshJTI != "r1" {
		t.Fatalf("unexpected session: %+v", got)
	}
	if got.CreatedAt.IsZero() || got.LastSeenAt.IsZero() {
		t.Fatalf("timestamps must be set: %+v", got)
	}

	alive, err := store.SessionAlive(ctx, "s1")
	if err != nil || !alive {
		t.Fatalf("SessionAlive: alive=%v err=%v", alive, err)
	}

	jti, err := store.DeleteSession(ctx, user, "s1")
	if err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if jti != "r1" {
		t.Fatalf("DeleteSession must return the refresh jti, got %q", jti)
	}
	if alive, _ := store.SessionAlive(ctx, "s1"); alive {
		t.Fatal("session must be dead after DeleteSession")
	}
	if sessions, _ := store.ListSessions(ctx, user); len(sessions) != 0 {
		t.Fatalf("index must be empty after delete, got %d", len(sessions))
	}
}

func TestSessionStore_TouchUpdatesLastSeenAndJTI(t *testing.T) {
	ctx := context.Background()
	store := newSessionStore(t)
	const user = "user-2"

	base := SessionMeta{UserID: user, UserAgent: "Safari", IP: "9.9.9.9", RefreshJTI: "old"}
	if err := store.SaveSession(ctx, "s2", base, time.Hour); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}
	first, _ := store.ListSessions(ctx, user)
	seenAt := first[0].CreatedAt.Add(time.Minute)

	if err := store.TouchSession(ctx, "s2", user, "new", seenAt, time.Hour); err != nil {
		t.Fatalf("TouchSession: %v", err)
	}
	after, _ := store.ListSessions(ctx, user)
	if len(after) != 1 {
		t.Fatalf("touch must not create a new session, got %d", len(after))
	}
	if after[0].RefreshJTI != "new" {
		t.Fatalf("refresh jti must rotate, got %q", after[0].RefreshJTI)
	}
	if !after[0].LastSeenAt.After(first[0].LastSeenAt) {
		t.Fatalf("last_seen must advance: before=%v after=%v", first[0].LastSeenAt, after[0].LastSeenAt)
	}
}

func TestSessionStore_ListOrdersByLastSeenDescAndPrunes(t *testing.T) {
	ctx := context.Background()
	store := newSessionStore(t)
	const user = "user-3"

	_ = store.SaveSession(ctx, "a", SessionMeta{UserID: user, RefreshJTI: "ra"}, time.Hour)
	_ = store.SaveSession(ctx, "b", SessionMeta{UserID: user, RefreshJTI: "rb"}, time.Hour)
	// Make "b" the most recently seen.
	_ = store.TouchSession(ctx, "b", user, "rb2", time.Now().Add(time.Hour), time.Hour)

	sessions, err := store.ListSessions(ctx, user)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 || sessions[0].ID != "b" {
		t.Fatalf("want b first (newest), got %+v", sessions)
	}

	// Simulate an expired metadata hash whose index entry lingers → must be pruned.
	if err := store.rdb.Del(ctx, sessionKey("a")).Err(); err != nil {
		t.Fatalf("del: %v", err)
	}
	sessions, _ = store.ListSessions(ctx, user)
	if len(sessions) != 1 || sessions[0].ID != "b" {
		t.Fatalf("expired session must be pruned, got %+v", sessions)
	}
	if n, _ := store.rdb.SCard(ctx, userSessionKey(user)).Result(); n != 1 {
		t.Fatalf("index must be pruned to 1, got %d", n)
	}
}

func TestSessionStore_DeleteAll(t *testing.T) {
	ctx := context.Background()
	store := newSessionStore(t)
	const user = "user-4"

	_ = store.SaveSession(ctx, "x", SessionMeta{UserID: user, RefreshJTI: "rx"}, time.Hour)
	_ = store.SaveSession(ctx, "y", SessionMeta{UserID: user, RefreshJTI: "ry"}, time.Hour)

	jtis, err := store.DeleteAllSessions(ctx, user)
	if err != nil {
		t.Fatalf("DeleteAllSessions: %v", err)
	}
	if len(jtis) != 2 {
		t.Fatalf("want 2 refresh jtis returned, got %v", jtis)
	}
	if sessions, _ := store.ListSessions(ctx, user); len(sessions) != 0 {
		t.Fatalf("all sessions must be gone, got %d", len(sessions))
	}
}
