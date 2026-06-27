package oauth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeKV struct {
	m   map[string]string
	err error
}

func newFakeKV() *fakeKV { return &fakeKV{m: map[string]string{}} }

func (f *fakeKV) Set(_ context.Context, key, val string, _ time.Duration) error {
	if f.err != nil {
		return f.err
	}
	f.m[key] = val
	return nil
}

func (f *fakeKV) GetDel(_ context.Context, key string) (string, error) {
	v, ok := f.m[key]
	if !ok {
		return "", errMissing
	}
	delete(f.m, key)
	return v, nil
}

func TestStateStoreSaveConsumeSingleUse(t *testing.T) {
	kv := newFakeKV()
	s := &stateStore{kv: kv, ttl: time.Minute}
	if err := s.Save(context.Background(), "st", "verifier-1"); err != nil {
		t.Fatalf("save: %v", err)
	}
	v, err := s.Consume(context.Background(), "st")
	if err != nil || v != "verifier-1" {
		t.Fatalf("consume: %q %v", v, err)
	}
	// single-use: second consume fails
	if _, err := s.Consume(context.Background(), "st"); !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("second consume should be ErrStateInvalid, got %v", err)
	}
}

func TestStateStoreUnknownState(t *testing.T) {
	s := &stateStore{kv: newFakeKV(), ttl: time.Minute}
	if _, err := s.Consume(context.Background(), "nope"); !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("unknown state should be ErrStateInvalid, got %v", err)
	}
}

func TestRandTokenUniqueURLSafe(t *testing.T) {
	a, err := randToken(32)
	if err != nil {
		t.Fatalf("randToken: %v", err)
	}
	b, _ := randToken(32)
	if a == b || a == "" {
		t.Fatalf("tokens should be non-empty and unique: %q %q", a, b)
	}
	for _, c := range a {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			t.Fatalf("token not URL-safe: %q", a)
		}
	}
}
