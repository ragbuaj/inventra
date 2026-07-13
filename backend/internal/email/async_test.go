package email

import (
	"context"
	"strings"
	"testing"
)

func TestAsyncMailer_SendPasswordReset_DelegatesToInner(t *testing.T) {
	cap := &captureSender{}
	inner := NewMailer(cap)
	a := NewAsyncMailer(inner, discardLogger())

	if err := a.SendPasswordReset(context.Background(), "u@example.com", "Budi", "https://app/reset-password?token=abc"); err != nil {
		t.Fatalf("send: %v", err)
	}
	a.Wait()

	if cap.calls != 1 || cap.to != "u@example.com" {
		t.Fatalf("unexpected recipient/calls: %q %d", cap.to, cap.calls)
	}
	if !strings.Contains(cap.html, "https://app/reset-password?token=abc") || !strings.Contains(cap.text, "token=abc") {
		t.Fatalf("link missing from bodies")
	}
}

func TestAsyncMailer_SendPasswordChanged_DelegatesToInner(t *testing.T) {
	cap := &captureSender{}
	inner := NewMailer(cap)
	a := NewAsyncMailer(inner, discardLogger())

	if err := a.SendPasswordChanged(context.Background(), "u@example.com", "Budi"); err != nil {
		t.Fatalf("send: %v", err)
	}
	a.Wait()

	if cap.calls != 1 || cap.to != "u@example.com" {
		t.Fatalf("unexpected recipient/calls: %q %d", cap.to, cap.calls)
	}
	if !strings.Contains(cap.text, "Budi") {
		t.Fatalf("name missing from changed notice body")
	}
}
