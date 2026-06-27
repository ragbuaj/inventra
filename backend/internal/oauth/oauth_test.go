package oauth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

type fakeExch struct {
	raw string
	err error
}

func (f fakeExch) Exchange(_ context.Context, _, _ string) (string, error) { return f.raw, f.err }

type fakeVer struct {
	email    string
	verified bool
	sub      string
	err      error
}

func (f fakeVer) Verify(_ context.Context, _ string) (string, bool, string, error) {
	return f.email, f.verified, f.sub, f.err
}

func testService(exch Exchanger, ver Verifier, kv *fakeKV) *Service {
	return &Service{
		enabled:  true,
		oauthCfg: &oauth2.Config{ClientID: "cid", RedirectURL: "http://localhost:8080/cb"},
		exch:     exch,
		verifier: ver,
		state:    &stateStore{kv: kv, ttl: time.Minute},
	}
}

func TestAuthCodeURLStoresStateAndIncludesIt(t *testing.T) {
	kv := newFakeKV()
	s := testService(fakeExch{}, fakeVer{}, kv)
	url, state, err := s.AuthCodeURL(context.Background())
	if err != nil || state == "" {
		t.Fatalf("authcodeurl: %q %v", state, err)
	}
	if !strings.Contains(url, "state="+state) {
		t.Fatalf("url missing state: %s", url)
	}
	if !strings.Contains(url, "code_challenge=") {
		t.Fatalf("url missing PKCE challenge: %s", url)
	}
	if !strings.Contains(url, "code_challenge_method=S256") {
		t.Fatalf("url missing S256 PKCE method: %s", url)
	}
	// state was stored (consumable once)
	if _, err := s.state.Consume(context.Background(), state); err != nil {
		t.Fatalf("state not stored: %v", err)
	}
}

func TestExchangeSuccess(t *testing.T) {
	kv := newFakeKV()
	s := testService(fakeExch{raw: "rawtoken"}, fakeVer{email: "a@b.com", verified: true, sub: "google-sub-1"}, kv)
	_ = s.state.Save(context.Background(), "st", "pkce")
	email, sub, err := s.Exchange(context.Background(), "code", "st")
	if err != nil || email != "a@b.com" || sub != "google-sub-1" {
		t.Fatalf("exchange: %q %q %v", email, sub, err)
	}
}

func TestExchangeRejectsUnverifiedEmail(t *testing.T) {
	kv := newFakeKV()
	s := testService(fakeExch{raw: "t"}, fakeVer{email: "a@b.com", verified: false, sub: "x"}, kv)
	_ = s.state.Save(context.Background(), "st", "pkce")
	if _, _, err := s.Exchange(context.Background(), "code", "st"); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got %v", err)
	}
}

func TestExchangeRejectsBadState(t *testing.T) {
	s := testService(fakeExch{raw: "t"}, fakeVer{verified: true}, newFakeKV())
	if _, _, err := s.Exchange(context.Background(), "code", "missing"); !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("expected ErrStateInvalid, got %v", err)
	}
}

func TestDisabledServiceRejects(t *testing.T) {
	s := &Service{enabled: false}
	if _, _, err := s.AuthCodeURL(context.Background()); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled AuthCodeURL: %v", err)
	}
	if _, _, err := s.Exchange(context.Background(), "c", "s"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled Exchange: %v", err)
	}
}

func TestExchangeStateSingleUse(t *testing.T) {
	kv := newFakeKV()
	s := testService(fakeExch{raw: "rawtoken"}, fakeVer{email: "a@b.com", verified: true, sub: "sub-1"}, kv)
	_ = s.state.Save(context.Background(), "st", "pkce")
	if _, _, err := s.Exchange(context.Background(), "code", "st"); err != nil {
		t.Fatalf("first exchange should succeed: %v", err)
	}
	// replay: same state must now be rejected (single-use)
	if _, _, err := s.Exchange(context.Background(), "code", "st"); !errors.Is(err, ErrStateInvalid) {
		t.Fatalf("replayed state must be ErrStateInvalid, got %v", err)
	}
}
