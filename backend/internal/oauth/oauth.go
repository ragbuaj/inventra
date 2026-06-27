// Package oauth implements Google OIDC sign-in (authorization-code + PKCE).
// Network operations sit behind Exchanger/Verifier interfaces so the flow is
// unit-testable without a real Google round-trip (ADR-0009).
package oauth

import (
	"context"
	"errors"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
)

// Errors.
var (
	ErrDisabled         = errors.New("google sign-in is not configured")
	ErrEmailNotVerified = errors.New("google email is not verified")
)

// Verifier verifies a raw OIDC ID token and returns the claims we use.
type Verifier interface {
	Verify(ctx context.Context, rawIDToken string) (email string, emailVerified bool, sub string, err error)
}

// Exchanger swaps an auth code (with its PKCE verifier) for the raw id_token.
type Exchanger interface {
	Exchange(ctx context.Context, code, codeVerifier string) (rawIDToken string, err error)
}

// Config holds the provider/client settings.
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Issuer       string
}

// Service drives the sign-in flow. When disabled (no client id / discovery
// failure), AuthCodeURL/Exchange return ErrDisabled.
type Service struct {
	enabled  bool
	oauthCfg *oauth2.Config
	exch     Exchanger
	verifier Verifier
	state    *stateStore
}

// New builds the Service. Empty ClientID → disabled (no discovery). A discovery
// error is returned to the caller, which should log it and run disabled.
func New(ctx context.Context, cfg Config, rdb *redis.Client) (*Service, error) {
	if cfg.ClientID == "" {
		return &Service{enabled: false}, nil
	}
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return &Service{enabled: false}, err
	}
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}
	return &Service{
		enabled:  true,
		oauthCfg: oauthCfg,
		exch:     googleExchanger{cfg: oauthCfg},
		verifier: googleVerifier{v: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID})},
		state:    &stateStore{kv: redisKV{rdb: rdb}, ttl: 5 * time.Minute},
	}, nil
}

// Enabled reports whether Google sign-in is configured.
func (s *Service) Enabled() bool { return s.enabled }

// AuthCodeURL generates a state + PKCE verifier, stores them, and returns the
// Google consent URL.
func (s *Service) AuthCodeURL(ctx context.Context) (string, string, error) {
	if !s.enabled {
		return "", "", ErrDisabled
	}
	state, err := randToken(32)
	if err != nil {
		return "", "", err
	}
	pkce := oauth2.GenerateVerifier()
	if err := s.state.Save(ctx, state, pkce); err != nil {
		return "", "", err
	}
	url := s.oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(pkce))
	return url, state, nil
}

// Exchange validates the single-use state, exchanges the code, verifies the ID
// token, and returns the verified email + subject.
func (s *Service) Exchange(ctx context.Context, code, state string) (string, string, error) {
	if !s.enabled {
		return "", "", ErrDisabled
	}
	pkce, err := s.state.Consume(ctx, state)
	if err != nil {
		return "", "", err // ErrStateInvalid
	}
	raw, err := s.exch.Exchange(ctx, code, pkce)
	if err != nil {
		return "", "", err
	}
	email, verified, sub, err := s.verifier.Verify(ctx, raw)
	if err != nil {
		return "", "", err
	}
	if !verified {
		return "", "", ErrEmailNotVerified
	}
	return email, sub, nil
}

// googleExchanger wraps oauth2.Config.Exchange and extracts the raw id_token.
type googleExchanger struct{ cfg *oauth2.Config }

func (g googleExchanger) Exchange(ctx context.Context, code, codeVerifier string) (string, error) {
	tok, err := g.cfg.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return "", err
	}
	raw, ok := tok.Extra("id_token").(string)
	if !ok || raw == "" {
		return "", errors.New("oauth: no id_token in token response")
	}
	return raw, nil
}

// googleVerifier wraps a go-oidc verifier and pulls the claims we need.
type googleVerifier struct{ v *oidc.IDTokenVerifier }

func (g googleVerifier) Verify(ctx context.Context, raw string) (string, bool, string, error) {
	idt, err := g.v.Verify(ctx, raw)
	if err != nil {
		return "", false, "", err
	}
	var c struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idt.Claims(&c); err != nil {
		return "", false, "", err
	}
	return c.Email, c.EmailVerified, idt.Subject, nil
}
