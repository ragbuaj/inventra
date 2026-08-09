package guide

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

const sampleUUID = "44444444-4444-4444-4444-444444444444"

// newGuardedRouter mounts the guide routes with recording stand-ins for the four
// middlewares. A recover shim absorbs the nil-service panic from the real
// handlers, so the whole guard chain is observed rather than just its first link.
func newGuardedRouter(hit *[]string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		defer func() { _ = recover() }()
		c.Next()
	})
	guard := func(name string) gin.HandlerFunc {
		return func(c *gin.Context) { *hit = append(*hit, name); c.Next() }
	}
	RegisterRoutes(r.Group("/api/v1"), &Handler{},
		guard("auth"), guard("optional"), guard("manage"), guard("web"))
	return r
}

// Registering these paths on one router is not free of risk: Gin panics when a
// static segment collides with a wildcard at the same level, and this module
// mounts /modules/:id next to /modules/:id/attachments/... and /attachments/:aid.
// This fails at test time rather than at boot in production.
func TestRegisterRoutes(t *testing.T) {
	var hit []string
	r := newGuardedRouter(&hit)

	got := map[string]bool{}
	for _, ri := range r.Routes() {
		got[ri.Method+" "+ri.Path] = true
	}

	want := []string{
		"GET /api/v1/guide/modules",
		"GET /api/v1/guide/attachments/:aid/content",
		"GET /api/v1/guide/modules/:id",
		"POST /api/v1/guide/modules",
		"PATCH /api/v1/guide/modules/:id",
		"DELETE /api/v1/guide/modules/:id",
		"POST /api/v1/guide/modules/:id/attachments/video",
		"POST /api/v1/guide/modules/:id/attachments/document",
		"PATCH /api/v1/guide/attachments/:aid",
		"DELETE /api/v1/guide/attachments/:aid",
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("rute %q tidak terdaftar", w)
		}
	}
	if len(r.Routes()) != len(want) {
		t.Errorf("jumlah rute = %d, mau %d — ada rute tak terduga: %v", len(r.Routes()), len(want), got)
	}
}

// Aturan akses tiap rute diperiksa, bukan dipercaya. Membaca daftar modul TIDAK
// boleh menuntut sesi, membaca berkas HANYA menuntut sesi, dan setiap penulisan
// harus melewati ketiganya. Tertukar sedikit saja, seluruh premis fitur ini
// runtuh.
func TestRouteGuards(t *testing.T) {
	cases := []struct {
		name, method, path string
		want               []string
	}{
		{"daftar modul publik", http.MethodGet, "/api/v1/guide/modules", []string{"optional"}},
		{"berkas butuh sesi", http.MethodGet, "/api/v1/guide/attachments/" + sampleUUID + "/content", []string{"auth"}},
		{"ambil modul", http.MethodGet, "/api/v1/guide/modules/" + sampleUUID, []string{"auth", "web", "manage"}},
		{"buat modul", http.MethodPost, "/api/v1/guide/modules", []string{"auth", "web", "manage"}},
		{"ubah modul", http.MethodPatch, "/api/v1/guide/modules/" + sampleUUID, []string{"auth", "web", "manage"}},
		{"hapus modul", http.MethodDelete, "/api/v1/guide/modules/" + sampleUUID, []string{"auth", "web", "manage"}},
		{"tambah video", http.MethodPost, "/api/v1/guide/modules/" + sampleUUID + "/attachments/video", []string{"auth", "web", "manage"}},
		{"tambah dokumen", http.MethodPost, "/api/v1/guide/modules/" + sampleUUID + "/attachments/document", []string{"auth", "web", "manage"}},
		{"ubah lampiran", http.MethodPatch, "/api/v1/guide/attachments/" + sampleUUID, []string{"auth", "web", "manage"}},
		{"hapus lampiran", http.MethodDelete, "/api/v1/guide/attachments/" + sampleUUID, []string{"auth", "web", "manage"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var hit []string
			r := newGuardedRouter(&hit)
			r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(tc.method, tc.path, nil))

			if len(hit) != len(tc.want) {
				t.Fatalf("penjaga yang berjalan = %v, mau %v", hit, tc.want)
			}
			for i := range tc.want {
				if hit[i] != tc.want[i] {
					t.Fatalf("penjaga yang berjalan = %v, mau %v", hit, tc.want)
				}
			}
		})
	}
}

// Rute publik tidak boleh ikut membawa penjaga sesi, sebab itulah yang membuat
// halaman panduan terbaca tanpa login.
func TestPublicListDoesNotRequireAuth(t *testing.T) {
	var hit []string
	r := newGuardedRouter(&hit)
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/guide/modules", nil))

	for _, g := range hit {
		if g == "auth" || g == "manage" {
			t.Fatalf("rute daftar publik melewati penjaga %q: %v", g, hit)
		}
	}
}
