package importer

import (
	"strings"
	"testing"
)

// TestSanitizeFilename exercises sanitizeFilename's traversal collapse and
// control-character stripping (contract c). It asserts real returned values,
// including the C0-control hardening (tab/ESC/CR/LF/NUL, and DEL) so no
// control byte can survive into the object key or the JSON filename field.
func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantClean string
		wantOK    bool
	}{
		{
			name:      "plain filename unchanged",
			in:        "report.csv",
			wantClean: "report.csv",
			wantOK:    true,
		},
		{
			name:      "unix traversal collapses to basename",
			in:        "../../etc/passwd",
			wantClean: "passwd",
			wantOK:    true,
		},
		{
			name:      "nested unix path collapses to basename",
			in:        "a/b/c.xlsx",
			wantClean: "c.xlsx",
			wantOK:    true,
		},
		{
			name: "backslash traversal is normalized and key-safe",
			in:   "..\\..\\x.csv",
			// Backslashes are normalized to "/" before path.Base, so this
			// collapses the same way a unix path would.
			wantClean: "x.csv",
			wantOK:    true,
		},
		{
			name:      "CRLF injection stripped",
			in:        "a\r\nb.csv",
			wantClean: "ab.csv",
			wantOK:    true,
		},
		{
			name:      "tab and ESC stripped (C0 hardening)",
			in:        "a\tb\x1bc.csv",
			wantClean: "abc.csv",
			wantOK:    true,
		},
		{
			name:      "NUL stripped",
			in:        "a\x00b.csv",
			wantClean: "ab.csv",
			wantOK:    true,
		},
		{
			name:      "backspace and DEL stripped (C0/DEL hardening)",
			in:        "a\x08b\x7fc.csv",
			wantClean: "abc.csv",
			wantOK:    true,
		},
		{
			name:   "empty rejected",
			in:     "",
			wantOK: false,
		},
		{
			name:   "dot rejected",
			in:     ".",
			wantOK: false,
		},
		{
			name:   "dotdot rejected",
			in:     "..",
			wantOK: false,
		},
		{
			name:   "slash-only rejected",
			in:     "/",
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clean, ok := sanitizeFilename(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("sanitizeFilename(%q) ok = %v, want %v (clean=%q)", tc.in, ok, tc.wantOK, clean)
			}
			if ok && clean != tc.wantClean {
				t.Fatalf("sanitizeFilename(%q) = %q, want %q", tc.in, clean, tc.wantClean)
			}
			if ok {
				for _, r := range clean {
					if r < 0x20 || r == 0x7f {
						t.Fatalf("sanitizeFilename(%q) = %q still contains control byte %U", tc.in, clean, r)
					}
				}
				if strings.Contains(clean, "/") {
					t.Fatalf("sanitizeFilename(%q) = %q is not key-safe (contains '/')", tc.in, clean)
				}
			}
		})
	}
}

// TestFormatFromFilename exercises the sanitized-filename -> import format
// derivation, including case-insensitivity and the "miss" (ok=false) path
// for unsupported extensions.
func TestFormatFromFilename(t *testing.T) {
	cases := []struct {
		in         string
		wantFormat string
		wantOK     bool
	}{
		{in: "report.csv", wantFormat: "csv", wantOK: true},
		{in: "report.xlsx", wantFormat: "xlsx", wantOK: true},
		{in: "REPORT.CSV", wantFormat: "csv", wantOK: true},
		{in: "Report.XLSX", wantFormat: "xlsx", wantOK: true},
		{in: "report.pdf", wantOK: false},
		{in: "report", wantOK: false},
		{in: "report.", wantOK: false},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			format, ok := formatFromFilename(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("formatFromFilename(%q) ok = %v, want %v (format=%q)", tc.in, ok, tc.wantOK, format)
			}
			if ok && format != tc.wantFormat {
				t.Fatalf("formatFromFilename(%q) = %q, want %q", tc.in, format, tc.wantFormat)
			}
		})
	}
}

// Note on target->permission-key mapping: TestPermissionKey in
// service_test.go already exercises Service.PermissionKey (the pure decision
// logic checkTargetPermission delegates to) for asset/employee/office/
// reference/unknown targets, including the ErrUnknownTarget (422) case. The
// remaining 403 (permission denied) / 500 (lookup error) wiring in
// checkTargetPermission requires a real Gin context + PermissionService
// (Redis-backed) and is covered by integration tests (Task 20), per the
// task's own guidance not to stand up a full router here.
