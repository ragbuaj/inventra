package email

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestResend builds a ResendSender pointed at a test server endpoint.
func newTestResend(endpoint, apiKey string) *ResendSender {
	s := NewResendSender(Options{APIKey: apiKey, From: "no-reply@inventra.local", FromName: "Inventra"}, discardLogger())
	s.endpoint = endpoint
	return s
}

func TestResendSender_Send_Success(t *testing.T) {
	var gotAuth, gotContentType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"11111111-1111-1111-1111-111111111111"}`))
	}))
	defer srv.Close()

	s := newTestResend(srv.URL, "re_test_key")
	if err := s.Send(context.Background(), "u@example.com", "Subject", "<b>hi</b>", "hi"); err != nil {
		t.Fatalf("send: %v", err)
	}

	if gotAuth != "Bearer re_test_key" {
		t.Fatalf("Authorization header = %q, want bearer with api key", gotAuth)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q", gotContentType)
	}

	var req resendRequest
	if err := json.Unmarshal([]byte(gotBody), &req); err != nil {
		t.Fatalf("request body not valid JSON: %v (%s)", err, gotBody)
	}
	if len(req.To) != 1 || req.To[0] != "u@example.com" {
		t.Fatalf("to = %v, want [u@example.com]", req.To)
	}
	if req.From != "Inventra <no-reply@inventra.local>" {
		t.Fatalf("from = %q, want display-name form", req.From)
	}
	if req.Subject != "Subject" || req.HTML != "<b>hi</b>" || req.Text != "hi" {
		t.Fatalf("body fields not forwarded: %+v", req)
	}
}

func TestResendSender_Send_ErrorStatus_SurfacesMessageNotKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"statusCode":422,"name":"validation_error","message":"Invalid ` + "`to`" + ` field"}`))
	}))
	defer srv.Close()

	s := newTestResend(srv.URL, "re_super_secret_key")
	err := s.Send(context.Background(), "bad", "Subject", "<b>hi</b>", "hi")
	if err == nil {
		t.Fatal("expected an error on 422")
	}
	if !strings.Contains(err.Error(), "422") || !strings.Contains(err.Error(), "Invalid") {
		t.Fatalf("error should carry status + message, got %q", err.Error())
	}
	// The API key must NEVER leak into an error surfaced to logs/callers.
	if strings.Contains(err.Error(), "re_super_secret_key") {
		t.Fatalf("API key leaked into error: %q", err.Error())
	}
}

func TestResendSender_FromHeader_NoNameUsesBareAddress(t *testing.T) {
	s := NewResendSender(Options{APIKey: "k", From: "no-reply@inventra.local"}, discardLogger())
	if got := s.fromHeader(); got != "no-reply@inventra.local" {
		t.Fatalf("fromHeader without name = %q, want bare address", got)
	}
}

func TestNewSender_Provider(t *testing.T) {
	cases := []struct {
		name    string
		opts    Options
		wantTyp string
	}{
		{"disabled always logs", Options{Enabled: false, Provider: "resend", APIKey: "k", Host: "smtp"}, "*email.LogSender"},
		{"resend with key", Options{Enabled: true, Provider: "resend", APIKey: "k"}, "*email.ResendSender"},
		{"resend without key falls back to log", Options{Enabled: true, Provider: "resend", APIKey: ""}, "*email.LogSender"},
		{"explicit log", Options{Enabled: true, Provider: "log", Host: "smtp", APIKey: "k"}, "*email.LogSender"},
		{"smtp with host", Options{Enabled: true, Provider: "smtp", Host: "smtp.example.com"}, "*email.SMTPSender"},
		{"empty provider with host is smtp", Options{Enabled: true, Provider: "", Host: "smtp.example.com"}, "*email.SMTPSender"},
		{"smtp without host falls back to log", Options{Enabled: true, Provider: "smtp", Host: ""}, "*email.LogSender"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NewSender(c.opts, discardLogger())
			if typ := reflectTypeName(got); typ != c.wantTyp {
				t.Fatalf("NewSender(%+v) = %s, want %s", c.opts, typ, c.wantTyp)
			}
		})
	}
}

func reflectTypeName(v any) string {
	switch v.(type) {
	case *LogSender:
		return "*email.LogSender"
	case *SMTPSender:
		return "*email.SMTPSender"
	case *ResendSender:
		return "*email.ResendSender"
	default:
		return "unknown"
	}
}
