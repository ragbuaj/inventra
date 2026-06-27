package ratelimit

import (
	"testing"
	"time"
)

func TestBackstopAllowsWithinBurst(t *testing.T) {
	b := newBackstop(100)
	for i := 0; i < 60; i++ {
		if !b.Allow("k", 60) {
			t.Fatalf("call %d should pass within burst of 60", i)
		}
	}
	if b.Allow("k", 60) {
		t.Fatal("61st immediate call should be denied (bucket drained)")
	}
}

func TestBackstopKeysIndependent(t *testing.T) {
	b := newBackstop(100)
	for i := 0; i < 60; i++ {
		b.Allow("a", 60)
	}
	if !b.Allow("b", 60) {
		t.Fatal("a different key must have its own bucket")
	}
}

func TestBackstopCapFailsOpen(t *testing.T) {
	b := newBackstop(2)
	b.Allow("a", 60)
	b.Allow("b", 60)
	if !b.Allow("c", 60) {
		t.Fatal("above the cap a new key must fail open (true)")
	}
	if b.len() != 2 {
		t.Fatalf("cap must bound the map at 2, got %d", b.len())
	}
}

func TestBackstopEvictsIdle(t *testing.T) {
	b := newBackstop(100)
	cur := time.Unix(1000, 0)
	b.now = func() time.Time { return cur }
	b.Allow("k", 60)
	cur = cur.Add(20 * time.Minute)
	b.evictIdle(10 * time.Minute)
	if b.len() != 0 {
		t.Fatalf("idle entry should be evicted, len=%d", b.len())
	}
}
