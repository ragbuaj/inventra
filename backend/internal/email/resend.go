package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// resendEndpoint is the Resend transactional-email API.
// Source: https://resend.com/docs/api-reference/emails/send-email
const resendEndpoint = "https://api.resend.com/emails"

// resendHTTPTimeout bounds each API call so a stuck request cannot hang the
// send goroutine (the AsyncMailer already detaches the caller; this bounds the
// worker itself).
const resendHTTPTimeout = 10 * time.Second

// ResendSender delivers transactional mail via the Resend HTTP API instead of
// SMTP. It implements Sender.
//
// Request contract (Source: https://resend.com/docs/api-reference/emails/send-email):
//
//	POST https://api.resend.com/emails
//	Authorization: Bearer <api key>
//	Content-Type: application/json
//	{ "from": "...", "to": ["..."], "subject": "...", "html": "...", "text": "..." }
//
// Success is HTTP 200 with a JSON body { "id": "<uuid>" }.
type ResendSender struct {
	apiKey   string
	from     string
	fromName string
	client   *http.Client
	endpoint string // overridable in tests
	logger   *slog.Logger
}

// NewResendSender builds a ResendSender from Options (Provider == "resend").
func NewResendSender(opts Options, logger *slog.Logger) *ResendSender {
	return &ResendSender{
		apiKey:   opts.APIKey,
		from:     opts.From,
		fromName: opts.FromName,
		client:   &http.Client{Timeout: resendHTTPTimeout},
		endpoint: resendEndpoint,
		logger:   logger,
	}
}

// resendRequest is the JSON body of POST /emails (only the fields we send).
type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html,omitempty"`
	Text    string   `json:"text,omitempty"`
}

// fromHeader renders the sender as "Name <address>" when a display name is set,
// matching what the Resend API accepts for the `from` field.
func (s *ResendSender) fromHeader() string {
	if s.fromName == "" {
		return s.from
	}
	return fmt.Sprintf("%s <%s>", s.fromName, s.from)
}

// Send posts a single message to the Resend API. A non-2xx response is turned
// into an error carrying the status and a short body snippet — never the API
// key (which only ever travels in the request Authorization header, and is not
// echoed in responses).
func (s *ResendSender) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	payload, err := json.Marshal(resendRequest{
		From:    s.fromHeader(),
		To:      []string{to},
		Subject: subject,
		HTML:    htmlBody,
		Text:    textBody,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Cap the body read so a misbehaving/oversized error response can't blow up memory.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("resend: send failed: status %d: %s", resp.StatusCode, resendErrorMessage(body))
	}
	return nil
}

// resendErrorMessage extracts a human-readable message from a Resend error
// body when possible, falling back to the raw (trimmed) body. Resend error
// bodies look like {"statusCode":N,"message":"...","name":"..."} or
// {"error":{"message":"..."}} depending on the failure.
func resendErrorMessage(body []byte) string {
	var flat struct {
		Message string `json:"message"`
		Name    string `json:"name"`
	}
	if err := json.Unmarshal(body, &flat); err == nil && flat.Message != "" {
		return flat.Message
	}
	var nested struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &nested); err == nil && nested.Error.Message != "" {
		return nested.Error.Message
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "(empty response body)"
	}
	return trimmed
}
