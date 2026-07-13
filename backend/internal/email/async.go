package email

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// asyncSendTimeout bounds each detached send so a stuck SMTP dial cannot leak
// goroutines indefinitely.
const asyncSendTimeout = 15 * time.Second

// AsyncMailer wraps a *Mailer so account-security emails are dispatched on a
// detached goroutine instead of blocking the caller. This removes a timing
// side-channel: without it, endpoints like POST /auth/password/forgot return
// near-instantly for unknown emails but only after a full SMTP round-trip
// (up to 10s) for valid ones, letting an attacker infer account existence
// from response latency alone.
//
// The inner mailer is still called synchronously from the goroutine's POV;
// only the caller (e.g. internal/identity.Service) is decoupled from the
// send's latency and outcome. Send failures are logged, never surfaced to
// the caller.
type AsyncMailer struct {
	inner  *Mailer
	logger *slog.Logger

	// wg lets tests deterministically wait for a dispatched send to finish
	// instead of racing a background goroutine. Harmless in production.
	wg sync.WaitGroup
}

// NewAsyncMailer builds an AsyncMailer wrapping inner. logger must not be nil.
func NewAsyncMailer(inner *Mailer, logger *slog.Logger) *AsyncMailer {
	return &AsyncMailer{inner: inner, logger: logger}
}

// SendPasswordReset dispatches the reset email asynchronously and returns nil
// immediately.
func (a *AsyncMailer) SendPasswordReset(_ context.Context, to, name, link string) error {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), asyncSendTimeout)
		defer cancel()
		if err := a.inner.SendPasswordReset(ctx, to, name, link); err != nil {
			a.logger.Error("send password reset email failed", "error", err)
		}
	}()
	return nil
}

// SendPasswordChanged dispatches the password-changed notice asynchronously
// and returns nil immediately.
func (a *AsyncMailer) SendPasswordChanged(_ context.Context, to, name string) error {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), asyncSendTimeout)
		defer cancel()
		if err := a.inner.SendPasswordChanged(ctx, to, name); err != nil {
			a.logger.Error("send password changed email failed", "error", err)
		}
	}()
	return nil
}

// SendEmailChangeVerify dispatches the email-change verification link
// asynchronously and returns nil immediately.
func (a *AsyncMailer) SendEmailChangeVerify(_ context.Context, to, name, link string) error {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), asyncSendTimeout)
		defer cancel()
		if err := a.inner.SendEmailChangeVerify(ctx, to, name, link); err != nil {
			a.logger.Error("send email change verify email failed", "error", err)
		}
	}()
	return nil
}

// SendEmailChanged dispatches the email-changed notice asynchronously and
// returns nil immediately.
func (a *AsyncMailer) SendEmailChanged(_ context.Context, to, name, newEmail string) error {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), asyncSendTimeout)
		defer cancel()
		if err := a.inner.SendEmailChanged(ctx, to, name, newEmail); err != nil {
			a.logger.Error("send email changed email failed", "error", err)
		}
	}()
	return nil
}

// Wait blocks until all dispatched sends have completed. Intended for
// deterministic unit tests; harmless (a no-op once drained) in production.
func (a *AsyncMailer) Wait() {
	a.wg.Wait()
}
