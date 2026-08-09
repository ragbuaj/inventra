package guide

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// ─── svcError ────────────────────────────────────────────────────────────────

// The whole status vocabulary of this module, in one table. Every sentinel the
// service can return has exactly one status, and getting one wrong is invisible
// from the Go side: the request still completes, only the client reads it as the
// wrong kind of failure — a 500 for a duplicate slug, a 422 for a file that was
// merely too big.
func TestSvcErrorStatusMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"tidak ditemukan", ErrNotFound, http.StatusNotFound},
		{"slug bentrok", ErrSlugExists, http.StatusConflict},
		{"referensi tidak sah", ErrInvalidRef, http.StatusUnprocessableEntity},
		{"langkah tidak sah", ErrInvalidSteps, http.StatusUnprocessableEntity},
		{"tautan video tidak sah", ErrInvalidVideoURL, http.StatusUnprocessableEntity},
		{"lampiran melebihi batas", ErrTooManyAttachments, http.StatusUnprocessableEntity},
		{"jenis berkas ditolak", ErrUnsupportedType, http.StatusUnsupportedMediaType},
		{"berkas terlalu besar", ErrTooLarge, http.StatusRequestEntityTooLarge},

		// Terbungkus juga harus tetap terbaca: sentinel yang dibungkus konteks
		// tambahan tidak boleh diam-diam jatuh ke cabang default.
		{"tidak ditemukan terbungkus", fmt.Errorf("ambil modul: %w", ErrNotFound), http.StatusNotFound},
		{"terlalu besar terbungkus", fmt.Errorf("unggah: %w", ErrTooLarge), http.StatusRequestEntityTooLarge},
	}

	h := &Handler{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/guide/modules", nil)

			h.svcError(c, tc.err)

			if w.Code != tc.want {
				t.Fatalf("status = %d, mau %d (%s)", w.Code, tc.want, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), `"error"`) {
				t.Fatalf("badan respons tanpa field error: %s", w.Body.String())
			}
		})
	}
}

// Error yang tidak dikenali menjadi 500 yang buram. Pesan aslinya milik log,
// bukan milik klien — sebuah kegagalan koneksi database yang bocor ke respons
// akan menyebutkan host, nama basis data, dan kadang pengguna.
func TestSvcErrorHidesUnknownErrorDetail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/guide/modules", nil)

	(&Handler{}).svcError(c, fmt.Errorf("dial tcp 10.0.0.5:5432: connect: connection refused"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, mau 500", w.Code)
	}
	for _, leaked := range []string{"10.0.0.5", "5432", "dial tcp", "connection refused"} {
		if strings.Contains(w.Body.String(), leaked) {
			t.Fatalf("respons membocorkan %q: %s", leaked, w.Body.String())
		}
	}
}

// ─── contentDisposition ──────────────────────────────────────────────────────

// Nama berkas datang dari pengunggah dan berakhir di sebuah header. Baris baru
// di dalamnya berarti header sisipan, jadi yang diperiksa adalah bahwa nilainya
// tetap satu baris dan tetap terurai kembali menjadi nama yang sama.
func TestContentDispositionNeverInjectsHeaders(t *testing.T) {
	cases := map[string]string{
		"CRLF lalu header palsu": "panduan.pdf\r\nSet-Cookie: sesi=curian",
		"LF saja":                "panduan\n.pdf",
		"CR saja":                "panduan\r.pdf",
		"CRLF di awal":           "\r\nX-Injected: 1",
	}
	for name, filename := range cases {
		t.Run(name, func(t *testing.T) {
			got := contentDisposition(filename)
			if strings.ContainsAny(got, "\r\n") {
				t.Fatalf("nilai header masih memuat baris baru: %q", got)
			}
			if strings.Contains(strings.ToLower(got), "set-cookie") && !strings.Contains(got, `"`) {
				t.Fatalf("teks sisipan lolos tanpa dikutip: %q", got)
			}
			if _, _, err := mime.ParseMediaType(got); err != nil {
				t.Fatalf("nilai header tidak terurai: %q (%v)", got, err)
			}
		})
	}
}

func TestContentDispositionRoundTrips(t *testing.T) {
	cases := map[string]string{
		"biasa":               "panduan-registrasi-aset.pdf",
		"ada spasi":           "Panduan Registrasi Aset.pdf",
		"ada tanda kutip":     `panduan "resmi".pdf`,
		"ada titik koma":      "panduan;versi2.pdf",
		"bukan ASCII":         "Panduan Peminjaman Aset – Ringkas.pdf",
		"ada tanda backslash": `panduan\aset.pdf`,
		"panjang tapak batas": strings.Repeat("a", 200) + ".pdf",
	}
	for name, filename := range cases {
		t.Run(name, func(t *testing.T) {
			got := contentDisposition(filename)
			typ, params, err := mime.ParseMediaType(got)
			if err != nil {
				t.Fatalf("nilai header tidak terurai: %q (%v)", got, err)
			}
			if typ != "inline" {
				t.Fatalf("disposisi = %q, mau inline — PDF panduan dibuka di tempat, bukan diunduh", typ)
			}
			if params["filename"] != filename {
				t.Fatalf("filename = %q, mau %q", params["filename"], filename)
			}
		})
	}
}

// Nama yang tidak menyisakan apa pun setelah dibersihkan tetap harus
// menghasilkan header yang SAH — itu properti yang benar-benar penting di sini.
//
// TEMUAN (dicatat, tidak diperbaiki di sesi ini): cabang cadangan
// `if v == "" { return "inline; filename=\"panduan.pdf\"" }` tidak pernah
// tercapai. mime.FormatMediaType mengembalikan `inline; filename=""` untuk nama
// kosong, bukan string kosong, sehingga yang terkirim adalah filename kosong dan
// peramban jatuh ke nama dari URL ("content"). Handler menutup jalur yang biasa
// (nama nil atau kosong diganti "panduan.pdf" sebelum memanggil ini), jadi ini
// hanya terpicu oleh nama yang berisi CR/LF saja. Test ini mengunci perilaku
// yang ADA, bukan yang seharusnya.
func TestContentDispositionAlwaysProducesAValidHeader(t *testing.T) {
	for name, filename := range map[string]string{
		"kosong":     "",
		"hanya CRLF": "\r\n",
		"hanya LF":   "\n",
	} {
		t.Run(name, func(t *testing.T) {
			got := contentDisposition(filename)
			typ, params, err := mime.ParseMediaType(got)
			if err != nil || typ != "inline" {
				t.Fatalf("header tidak sah: %q (%v)", got, err)
			}
			if strings.ContainsAny(got, "\r\n") {
				t.Fatalf("header memuat baris baru: %q", got)
			}
			if params["filename"] != "" {
				t.Fatalf("filename = %q — perilaku berubah, perbarui temuan ini", params["filename"])
			}
		})
	}
}

// ─── parseID ─────────────────────────────────────────────────────────────────

// Identitas yang tidak terurai dijawab 400 pada setiap rute yang menerimanya —
// bukan 404, bukan 500, dan bukan panic. Handler di sini sengaja tanpa service;
// shim pemulih pada router uji menyerap panic yang muncul kalau permintaan
// terlanjur berjalan, sehingga yang tersisa untuk diperiksa adalah jawabannya.
func TestMalformedIDIsAnswered400(t *testing.T) {
	paths := map[string]struct{ method, path string }{
		"ambil modul":    {http.MethodGet, "/api/v1/guide/modules/%s"},
		"ubah modul":     {http.MethodPatch, "/api/v1/guide/modules/%s"},
		"hapus modul":    {http.MethodDelete, "/api/v1/guide/modules/%s"},
		"tambah video":   {http.MethodPost, "/api/v1/guide/modules/%s/attachments/video"},
		"tambah dokumen": {http.MethodPost, "/api/v1/guide/modules/%s/attachments/document"},
		"ubah lampiran":  {http.MethodPatch, "/api/v1/guide/attachments/%s"},
		"hapus lampiran": {http.MethodDelete, "/api/v1/guide/attachments/%s"},
		"unduh lampiran": {http.MethodGet, "/api/v1/guide/attachments/%s/content"},
	}
	bad := map[string]string{
		"bukan uuid":         "bukan-uuid",
		"angka":              "12345",
		"uuid terpotong":     "44444444-4444-4444-4444",
		"heksadesimal cacat": "44444444-4444-4444-4444-44444444444g",
		"upaya sql":          "1'%20OR%20'1'='1",
	}

	for routeName, route := range paths {
		for idName, id := range bad {
			t.Run(routeName+"/"+idName, func(t *testing.T) {
				var hit []string
				r := newGuardedRouter(&hit)
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest(route.method, fmt.Sprintf(route.path, id), nil))

				if w.Code != http.StatusBadRequest {
					t.Fatalf("status = %d, mau 400 (%s)", w.Code, w.Body.String())
				}
			})
		}
	}
}

// UUID yang sah dalam huruf besar tetap harus diterima — beberapa klien
// menormalkan begitu, dan menolaknya akan terbaca sebagai berkas hilang.
func TestUppercaseUUIDIsAccepted(t *testing.T) {
	// Sengaja memakai identitas yang mengandung huruf heksadesimal — versi
	// huruf besar dari identitas yang seluruhnya angka tidak membuktikan apa pun.
	const withLetters = "abcdef01-2345-6789-abcd-ef0123456789"

	var hit []string
	r := newGuardedRouter(&hit)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet,
		"/api/v1/guide/attachments/"+strings.ToUpper(withLetters)+"/content", nil))

	if w.Code == http.StatusBadRequest {
		t.Fatalf("uuid huruf besar ditolak sebagai tidak sah: %s", w.Body.String())
	}
}

// ─── caller / actor ──────────────────────────────────────────────────────────

// caller() memutuskan siapa yang melihat media dan siapa yang melihat draf.
// Setiap jalur yang tidak bisa dipastikan harus turun ke "tidak berwenang",
// bukan naik.
func TestCallerResolvesConservatively(t *testing.T) {
	newCtx := func(userID, roleID string) *gin.Context {
		gin.SetMode(gin.TestMode)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/guide/modules", nil)
		if userID != "" {
			c.Set(middleware.CtxUserID, userID)
		}
		if roleID != "" {
			c.Set(middleware.CtxRoleID, roleID)
		}
		return c
	}

	cases := []struct {
		name             string
		userID, roleID   string
		signedIn, manage bool
	}{
		{"tamu tanpa identitas", "", "", false, false},
		{"tamu dengan sisa role", "", sampleUUID, false, false},
		{"sesi tanpa layanan izin", sampleUUID, sampleUUID, true, false},
		{"sesi dengan role tak terurai", sampleUUID, "bukan-uuid", true, false},
		{"sesi tanpa role sama sekali", sampleUUID, "", true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// perm sengaja nil: layanan izin yang tidak tersedia bukan wewenang.
			signedIn, manage := (&Handler{}).caller(newCtx(tc.userID, tc.roleID))
			if signedIn != tc.signedIn || manage != tc.manage {
				t.Fatalf("caller = (%v, %v), mau (%v, %v)", signedIn, manage, tc.signedIn, tc.manage)
			}
		})
	}
}

// actor() menempel di jejak audit. Identitas yang tidak terurai harus menjadi
// UUID nol, bukan panic dan bukan identitas orang lain.
func TestActorDegradesToNilUUID(t *testing.T) {
	for name, userID := range map[string]string{
		"tanpa sesi":     "",
		"bukan uuid":     "sesi-lama",
		"uuid terpotong": "44444444-4444",
	} {
		t.Run(name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if userID != "" {
				c.Set(middleware.CtxUserID, userID)
			}
			if got := (&Handler{}).actor(c); got != uuid.Nil {
				t.Fatalf("actor = %v, mau UUID nol", got)
			}
		})
	}
}

// ─── limitAwareBody ──────────────────────────────────────────────────────────

// Pembungkus ini ada karena parser multipart menelan *http.MaxBytesError dan
// melaporkannya sebagai header MIME rusak. Ia harus mencatat batas yang
// terlampaui di tempat kejadian — dan tidak menyalakan tanda itu untuk badan
// permintaan yang wajar.
func TestLimitAwareBodyRecordsTheTrip(t *testing.T) {
	read := func(size int, cap int64) bool {
		b := &limitAwareBody{ReadCloser: http.MaxBytesReader(
			httptest.NewRecorder(), io.NopCloser(bytes.NewReader(make([]byte, size))), cap)}
		_, _ = io.ReadAll(b)
		return b.tripped
	}

	if read(64, 1024) {
		t.Fatal("badan di bawah batas ditandai terlalu besar")
	}
	if read(1024, 1024) {
		t.Fatal("badan tepat di batas ditandai terlalu besar")
	}
	if !read(1025, 1024) {
		t.Fatal("badan satu byte di atas batas tidak tercatat")
	}
	if !read(1<<20, 1024) {
		t.Fatal("badan jauh di atas batas tidak tercatat")
	}
}

// ─── bentuk respons ──────────────────────────────────────────────────────────

// Modul tanpa lampiran harus mengirim [] dan bukan null: klien memetakan daftar
// ini tanpa memeriksa nil, jadi sebuah null menjatuhkan kartu modulnya.
func TestModuleResponseNeverSerializesNullCollections(t *testing.T) {
	raw, err := json.Marshal(moduleOnlyResponse(sqlc.GuideGuideModule{
		ID:      uuid.MustParse(sampleUUID),
		Slug:    "baru",
		Icon:    "i-lucide-book-open",
		Status:  sqlc.SharedGuideStatusDraft,
		TitleID: "Modul baru",
	}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	body := string(raw)
	for _, want := range []string{`"attachments":[]`, `"steps":[]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("respons tidak memuat %s: %s", want, body)
		}
	}
	// Skalar yang memang kosong tetap boleh null — yang tidak boleh adalah koleksi.
	for _, forbidden := range []string{`"attachments":null`, `"steps":null`} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("respons memuat %s: %s", forbidden, body)
		}
	}
}
