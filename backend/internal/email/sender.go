// Package email sends transactional mail (password reset / change notices).
// It is provider-agnostic: any SMTP relay works via Options, and a LogSender
// fallback keeps dev/CI functional without a real relay.
package email

import (
	"context"
	"log/slog"

	mail "github.com/wneessen/go-mail"
)

// Sender delivers a single message. Implementations must be safe for concurrent use.
type Sender interface {
	Send(ctx context.Context, to, subject, htmlBody, textBody string) error
}

// Options configures the SMTP sender (mapped from env in the composition root).
type Options struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
	TLS      string // "none" | "starttls" | "tls"
}

// NewSender returns an SMTPSender when enabled with a host, else a LogSender.
func NewSender(opts Options, logger *slog.Logger) Sender {
	if !opts.Enabled || opts.Host == "" {
		return &LogSender{logger: logger, from: opts.From}
	}
	return &SMTPSender{opts: opts, logger: logger}
}

// LogSender logs the message instead of sending — used in dev/CI without a relay.
type LogSender struct {
	logger *slog.Logger
	from   string
}

func (s *LogSender) Send(_ context.Context, to, subject, _, textBody string) error {
	s.logger.Info("email (log-only)", "from", s.from, "to", to, "subject", subject, "body", textBody)
	return nil
}

// SMTPSender delivers via go-mail over the configured relay.
type SMTPSender struct {
	opts   Options
	logger *slog.Logger
}

func (s *SMTPSender) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	m := mail.NewMsg()
	if err := m.FromFormat(s.opts.FromName, s.opts.From); err != nil {
		return err
	}
	if err := m.To(to); err != nil {
		return err
	}
	m.Subject(subject)
	m.SetBodyString(mail.TypeTextPlain, textBody)
	m.AddAlternativeString(mail.TypeTextHTML, htmlBody)

	clientOpts := []mail.Option{mail.WithPort(s.opts.Port), mail.WithTimeout(10_000_000_000)}
	switch s.opts.TLS {
	case "tls":
		clientOpts = append(clientOpts, mail.WithSSLPort(false))
	case "starttls":
		clientOpts = append(clientOpts, mail.WithTLSPolicy(mail.TLSMandatory))
	default:
		clientOpts = append(clientOpts, mail.WithTLSPolicy(mail.NoTLS))
	}
	if s.opts.Username != "" {
		clientOpts = append(clientOpts, mail.WithSMTPAuth(mail.SMTPAuthPlain),
			mail.WithUsername(s.opts.Username), mail.WithPassword(s.opts.Password))
	}
	c, err := mail.NewClient(s.opts.Host, clientOpts...)
	if err != nil {
		return err
	}
	return c.DialAndSendWithContext(ctx, m)
}
