package guide

import (
	"errors"
	"strings"
	"testing"
)

func TestParseYouTubeIDAccepts(t *testing.T) {
	const want = "dQw4w9WgXcQ"
	cases := map[string]string{
		"watch":                  "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"watch tanpa www":        "https://youtube.com/watch?v=dQw4w9WgXcQ",
		"watch mobile":           "https://m.youtube.com/watch?v=dQw4w9WgXcQ",
		"watch http":             "http://www.youtube.com/watch?v=dQw4w9WgXcQ",
		"short link":             "https://youtu.be/dQw4w9WgXcQ",
		"shorts":                 "https://www.youtube.com/shorts/dQw4w9WgXcQ",
		"parameter waktu":        "https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=90s",
		"parameter playlist":     "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PL123&index=2",
		"parameter si":           "https://youtu.be/dQw4w9WgXcQ?si=AbCdEf",
		"shorts dengan query":    "https://youtube.com/shorts/dQw4w9WgXcQ?feature=share",
		"host huruf besar":       "https://WWW.YouTube.COM/watch?v=dQw4w9WgXcQ",
		"spasi di ujung":         "  https://youtu.be/dQw4w9WgXcQ  ",
		"fqdn titik di ujung":    "https://www.youtube.com./watch?v=dQw4w9WgXcQ",
		"id dengan tanda hubung": "https://youtu.be/a-b_c1D2E3F",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseYouTubeID(raw)
			if err != nil {
				t.Fatalf("ParseYouTubeID(%q) error: %v", raw, err)
			}
			expect := want
			if name == "id dengan tanda hubung" {
				expect = "a-b_c1D2E3F"
			}
			if got != expect {
				t.Fatalf("ParseYouTubeID(%q) = %q, mau %q", raw, got, expect)
			}
		})
	}
}

func TestParseYouTubeIDRejects(t *testing.T) {
	cases := map[string]string{
		// Daftar tautan bermusuhan dari AC14 pada spesifikasi.
		"host lain":            "https://vimeo.com/123456789",
		"skema javascript":     "javascript:alert(1)",
		"skema data":           "data:text/html,<script>alert(1)</script>",
		"youtube di path":      "https://evil.example.com/youtube.com/watch?v=dQw4w9WgXcQ",
		"youtube sebagai awal": "http://youtube.com.evil.example.com/watch?v=dQw4w9WgXcQ",

		// Bentuk lain yang harus tetap tertutup.
		"userinfo menyamar":     "https://youtube.com@evil.example.com/watch?v=dQw4w9WgXcQ",
		"subdomain asing":       "https://evil.youtube.com.co/watch?v=dQw4w9WgXcQ",
		"akhiran menyerupai":    "https://notyoutube.com/watch?v=dQw4w9WgXcQ",
		"skema file":            "file:///etc/passwd",
		"tanpa skema":           "www.youtube.com/watch?v=dQw4w9WgXcQ",
		"kosong":                "",
		"hanya spasi":           "   ",
		"tanpa parameter v":     "https://www.youtube.com/watch",
		"v kosong":              "https://www.youtube.com/watch?v=",
		"path tidak dikenal":    "https://www.youtube.com/results?search_query=x",
		"halaman kanal":         "https://www.youtube.com/@kanal",
		"id terlalu pendek":     "https://youtu.be/abc",
		"id terlalu panjang":    "https://youtu.be/dQw4w9WgXcQextra",
		"id karakter terlarang": "https://www.youtube.com/watch?v=dQw4w9WgXc<",
		"id dengan spasi":       "https://www.youtube.com/watch?v=dQw4w9WgX Q",
		"shorts tanpa id":       "https://www.youtube.com/shorts/",
		"youtu.be tanpa id":     "https://youtu.be/",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ParseYouTubeID(raw)
			if err == nil {
				t.Fatalf("ParseYouTubeID(%q) = %q, mau ditolak", raw, got)
			}
			if !errors.Is(err, ErrInvalidVideoURL) {
				t.Fatalf("ParseYouTubeID(%q) error = %v, mau ErrInvalidVideoURL", raw, err)
			}
			if got != "" {
				t.Fatalf("ParseYouTubeID(%q) mengembalikan id %q padahal ditolak", raw, got)
			}
		})
	}
}

// Identitas video tidak boleh membawa karakter yang bisa keluar dari atribut
// src saat disematkan, berapa pun panjangnya.
func TestParseYouTubeIDRejectsInjectionShapedIDs(t *testing.T) {
	for _, id := range []string{
		`"><script>`,
		"../../etc/pw",
		"dQw4w9WgXc'",
		"dQw4w9WgXc\"",
		"dQw4w9WgXc>",
		"dQw4w9WgXc%2F",
	} {
		raw := "https://youtu.be/" + id
		if _, err := ParseYouTubeID(raw); err == nil {
			t.Fatalf("ParseYouTubeID(%q) diterima, mau ditolak", raw)
		}
	}
}

func TestSniffPDF(t *testing.T) {
	pdf := []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n")
	if !isPDF(pdf) {
		t.Fatal("isPDF(PDF asli) = false")
	}

	// Yang menyamar: ekstensi dan Content-Type dikendalikan pengunggah, isinya tidak.
	bad := map[string][]byte{
		"html":               []byte("<!doctype html><script>alert(1)</script>"),
		"png":                {0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A},
		"zip":                {'P', 'K', 3, 4},
		"kosong":             {},
		"lebih pendek dari5": []byte("%PD"),
		"pdf tidak di awal":  append([]byte("GIF89a"), []byte("%PDF-1.7")...),
		"spasi di depan":     []byte("   %PDF-1.7"),
	}
	for name, data := range bad {
		t.Run(name, func(t *testing.T) {
			if isPDF(data) {
				t.Fatalf("isPDF(%s) = true, mau false", name)
			}
		})
	}
}

func TestValidateSteps(t *testing.T) {
	ok, err := validateSteps([]Step{{TextID: "Buka halaman Masuk"}, {TextID: "Isi email", TextEn: ptr("Enter email")}})
	if err != nil {
		t.Fatalf("validateSteps sah: %v", err)
	}
	if !strings.Contains(string(ok), `"text_id":"Buka halaman Masuk"`) {
		t.Fatalf("hasil serialisasi tidak memuat langkah pertama: %s", ok)
	}

	if _, err := validateSteps(nil); err != nil {
		t.Fatalf("daftar langkah kosong harus sah (modul tanpa langkah): %v", err)
	}

	if _, err := validateSteps([]Step{{TextID: "  "}}); !errors.Is(err, ErrInvalidSteps) {
		t.Fatalf("langkah dengan teks Indonesia kosong error = %v, mau ErrInvalidSteps", err)
	}

	tooMany := make([]Step, maxSteps+1)
	for i := range tooMany {
		tooMany[i] = Step{TextID: "x"}
	}
	if _, err := validateSteps(tooMany); !errors.Is(err, ErrInvalidSteps) {
		t.Fatalf("langkah melebihi batas error = %v, mau ErrInvalidSteps", err)
	}
}

func ptr[T any](v T) *T { return &v }
