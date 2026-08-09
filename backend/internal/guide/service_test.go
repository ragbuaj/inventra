package guide

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// mapDBError is the only translation between Postgres and the sentinels the
// handler turns into status codes. A code that stops being recognised here does
// not fail loudly — it falls through to the default branch and surfaces as a
// 500, which is how a duplicate slug would start reading as a server fault.
func TestMapDBError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"nil tetap nil", nil, nil},
		{"tidak ada baris", pgx.ErrNoRows, ErrNotFound},
		{"tidak ada baris terbungkus", fmt.Errorf("query modul: %w", pgx.ErrNoRows), ErrNotFound},
		{"pelanggaran unik", &pgconn.PgError{Code: "23505"}, ErrSlugExists},
		{"pelanggaran unik terbungkus", fmt.Errorf("simpan: %w", &pgconn.PgError{Code: "23505"}), ErrSlugExists},
		{"kunci asing", &pgconn.PgError{Code: "23503"}, ErrInvalidRef},
		{"pelanggaran check", &pgconn.PgError{Code: "23514"}, ErrInvalidSteps},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapDBError(tc.in)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("mapDBError(%v) = %v, mau nil", tc.in, got)
				}
				return
			}
			if !errors.Is(got, tc.want) {
				t.Fatalf("mapDBError(%v) = %v, mau %v", tc.in, got, tc.want)
			}
		})
	}
}

// Kode yang tidak dikenali harus lewat apa adanya, bukan dipaksa menjadi salah
// satu sentinel. Memetakan not-null violation menjadi "slug sudah dipakai" akan
// menyesatkan penulis dan menyembunyikan bug dari log.
func TestMapDBErrorPassesUnknownErrorsThrough(t *testing.T) {
	for name, in := range map[string]error{
		"kolom not null":     &pgconn.PgError{Code: "23502"},
		"serialization":      &pgconn.PgError{Code: "40001"},
		"koneksi terputus":   errors.New("dial tcp: connection refused"),
		"konteks dibatalkan": context.Canceled,
	} {
		t.Run(name, func(t *testing.T) {
			got := mapDBError(in)
			if got != in {
				t.Fatalf("mapDBError(%v) = %v, mau error yang sama persis", in, got)
			}
			for _, sentinel := range []error{ErrNotFound, ErrSlugExists, ErrInvalidRef, ErrInvalidSteps} {
				if errors.Is(got, sentinel) {
					t.Fatalf("error tak dikenal dipetakan menjadi %v", sentinel)
				}
			}
		})
	}
}

// Batas daftar langkah, diuji di tepinya: satu di bawah, tepat di batas, dan
// satu di atas. Yang di atas sudah diuji sebelumnya; yang TEPAT di batas belum,
// dan justru itu nilai yang paling mudah tergeser oleh salah satu.
func TestValidateStepsAtTheBoundary(t *testing.T) {
	build := func(n int) []Step {
		out := make([]Step, n)
		for i := range out {
			out[i] = Step{TextID: fmt.Sprintf("Langkah %d", i)}
		}
		return out
	}
	for _, n := range []int{0, 1, maxSteps - 1, maxSteps} {
		raw, err := validateSteps(build(n))
		if err != nil {
			t.Fatalf("%d langkah ditolak: %v", n, err)
		}
		var decoded []Step
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("%d langkah menghasilkan json rusak: %v", n, err)
		}
		if len(decoded) != n {
			t.Fatalf("%d langkah tersimpan sebagai %d", n, len(decoded))
		}
	}
	if _, err := validateSteps(build(maxSteps + 1)); !errors.Is(err, ErrInvalidSteps) {
		t.Fatalf("%d langkah error = %v, mau ErrInvalidSteps", maxSteps+1, err)
	}
}

// Yang disimpan adalah hasil enkode ulang, bukan apa yang datang. Itulah yang
// menjaga bentuk jsonb tetap persis seperti yang dibaca halaman pembaca, jadi
// diuji pada string JSON-nya — bukan pada struct-nya.
func TestValidateStepsStoresACanonicalEncoding(t *testing.T) {
	raw, err := validateSteps([]Step{
		{TextID: "  Buka halaman Masuk  ", TextEn: ptr("  Open the Sign In page  ")},
		{TextID: "Isi email", TextEn: ptr("   ")}, // dikosongkan penulis: berarti belum diterjemahkan
		{TextID: "Tekan Masuk"},                   // memang tidak pernah punya versi Inggris
	})
	if err != nil {
		t.Fatalf("validateSteps: %v", err)
	}

	const want = `[{"text_id":"Buka halaman Masuk","text_en":"Open the Sign In page"},` +
		`{"text_id":"Isi email"},{"text_id":"Tekan Masuk"}]`
	if string(raw) != want {
		t.Fatalf("hasil enkode = %s\nmau           = %s", raw, want)
	}
}

// Daftar langkah kosong harus tersimpan sebagai [] — bukan null. Sebuah null di
// kolom ini melanggar CHECK jsonb_typeof(steps) = 'array' pada migrasi 000050,
// jadi kegagalannya baru terlihat saat menyimpan ke database sungguhan.
func TestValidateStepsEncodesEmptyAsArray(t *testing.T) {
	for name, in := range map[string][]Step{"nil": nil, "kosong": {}} {
		t.Run(name, func(t *testing.T) {
			raw, err := validateSteps(in)
			if err != nil {
				t.Fatalf("validateSteps: %v", err)
			}
			if string(raw) != "[]" {
				t.Fatalf("hasil = %s, mau []", raw)
			}
		})
	}
}

// Satu langkah tak bertext_id membatalkan seluruh daftar, di posisi mana pun ia
// berada. Validasi yang berhenti di elemen pertama akan meloloskan yang kedua.
func TestValidateStepsRejectsABlankStepAnywhere(t *testing.T) {
	for name, steps := range map[string][]Step{
		"di awal":   {{TextID: " "}, {TextID: "Sah"}},
		"di tengah": {{TextID: "Sah"}, {TextID: ""}, {TextID: "Sah juga"}},
		"di akhir":  {{TextID: "Sah"}, {TextID: "\t\n"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := validateSteps(steps); !errors.Is(err, ErrInvalidSteps) {
				t.Fatalf("error = %v, mau ErrInvalidSteps", err)
			}
		})
	}
}

func TestTrimPtr(t *testing.T) {
	if got := trimPtr(nil); got != nil {
		t.Fatalf("trimPtr(nil) = %v, mau nil", *got)
	}
	for _, blank := range []string{"", " ", "\t", "\n  \n"} {
		if got := trimPtr(&blank); got != nil {
			t.Fatalf("trimPtr(%q) = %q, mau nil — judul kosong tersimpan sebagai NULL", blank, *got)
		}
	}
	v := "  Ringkasan alur  "
	got := trimPtr(&v)
	if got == nil || *got != "Ringkasan alur" {
		t.Fatalf("trimPtr(%q) = %v, mau \"Ringkasan alur\"", v, got)
	}
	if v != "  Ringkasan alur  " {
		t.Fatalf("trimPtr mengubah nilai asalnya menjadi %q", v)
	}
}
