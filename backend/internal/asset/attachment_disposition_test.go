package asset

import (
	"mime"
	"strings"
	"testing"
)

func TestContentDisposition_PlainFilename(t *testing.T) {
	out := contentDisposition("photo.jpg")
	if !strings.Contains(out, "filename") {
		t.Errorf("expected 'filename' in output, got: %s", out)
	}
	if strings.ContainsAny(out, "\r\n") {
		t.Errorf("output must not contain CR or LF, got: %q", out)
	}
}

func TestContentDisposition_QuoteInFilename(t *testing.T) {
	out := contentDisposition(`he"llo.pdf`)
	// The output must be parseable and round-trip the filename without a raw unescaped quote.
	_, params, err := mime.ParseMediaType(out)
	if err != nil {
		t.Fatalf("mime.ParseMediaType failed on %q: %v", out, err)
	}
	got := params["filename"]
	if got != `he"llo.pdf` {
		t.Errorf("filename round-trip mismatch: want %q, got %q", `he"llo.pdf`, got)
	}
}

func TestContentDisposition_CRLFInjection(t *testing.T) {
	out := contentDisposition("evil\r\nX-Injected: hdr\r\n.pdf")
	if strings.ContainsAny(out, "\r\n") {
		t.Errorf("output must not contain CR or LF, got: %q", out)
	}
}

func TestContentDisposition_EmptyFilename(t *testing.T) {
	// mime.FormatMediaType returns "" for empty filename; fallback must kick in.
	out := contentDisposition("")
	if out == "" {
		t.Error("expected non-empty fallback for empty filename")
	}
	if strings.ContainsAny(out, "\r\n") {
		t.Errorf("output must not contain CR or LF, got: %q", out)
	}
}
