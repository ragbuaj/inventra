package email

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// captureSender records the last message for assertions.
type captureSender struct {
	to, subject, html, text string
	calls                   int
}

func (c *captureSender) Send(_ context.Context, to, subject, html, text string) error {
	c.to, c.subject, c.html, c.text, c.calls = to, subject, html, text, c.calls+1
	return nil
}

func TestMailer_SendPasswordReset_RendersLinkAndName(t *testing.T) {
	cap := &captureSender{}
	m := NewMailer(cap)
	if err := m.SendPasswordReset(context.Background(), "u@example.com", "Budi", "https://app/reset-password?token=abc"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if cap.calls != 1 || cap.to != "u@example.com" {
		t.Fatalf("unexpected recipient/calls: %q %d", cap.to, cap.calls)
	}
	if !strings.Contains(cap.html, "https://app/reset-password?token=abc") || !strings.Contains(cap.text, "token=abc") {
		t.Fatalf("link missing from bodies")
	}
	if !strings.Contains(cap.html, "Budi") {
		t.Fatalf("name missing from html body")
	}
}

func TestMailer_SendPasswordChanged_RendersName(t *testing.T) {
	cap := &captureSender{}
	m := NewMailer(cap)
	if err := m.SendPasswordChanged(context.Background(), "u@example.com", "Budi"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if !strings.Contains(cap.text, "Budi") || cap.subject == "" {
		t.Fatalf("changed notice not rendered: subj=%q", cap.subject)
	}
}

func TestNewSender_FallsBackToLogSenderWhenDisabled(t *testing.T) {
	s := NewSender(Options{Enabled: false, Host: "smtp.example.com"}, discardLogger())
	if _, ok := s.(*LogSender); !ok {
		t.Fatalf("expected LogSender, got %T", s)
	}
}
