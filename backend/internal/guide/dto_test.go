package guide

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// bindJSON runs a body through the same binding path the handlers use, so these
// tests exercise the real validator and the real json tags rather than a
// hand-rolled approximation of them.
func bindJSON(t *testing.T, body string, dst any) error {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c.ShouldBindJSON(dst)
}

func videoRow() sqlc.GuideGuideAttachment {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	yt := "dQw4w9WgXcQ"
	return sqlc.GuideGuideAttachment{
		ID:        id,
		ModuleID:  uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Kind:      sqlc.SharedGuideAttachmentKindVideo,
		TitleID:   "Cara masuk",
		SortOrder: 1,
		YoutubeID: &yt,
	}
}

func documentRow() sqlc.GuideGuideAttachment {
	id := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	key := "guide/9f8e.pdf"
	name := "panduan-registrasi-aset.pdf"
	mime := "application/pdf"
	var size int64 = 1400000
	return sqlc.GuideGuideAttachment{
		ID:               id,
		ModuleID:         uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Kind:             sqlc.SharedGuideAttachmentKindDocument,
		TitleID:          "Ringkasan alur registrasi",
		SortOrder:        2,
		ObjectKey:        &key,
		OriginalFilename: &name,
		MimeType:         &mime,
		SizeBytes:        &size,
	}
}

// Inti keamanan fitur ini: apa pun yang bisa dipakai tamu untuk merekonstruksi
// media tidak boleh ikut terkirim. Diperiksa pada JSON yang benar-benar keluar,
// bukan pada struct-nya, karena tag omitempty-lah yang menentukan.
func TestAttachmentResponseHidesMediaFromGuests(t *testing.T) {
	for name, row := range map[string]sqlc.GuideGuideAttachment{
		"video":   videoRow(),
		"dokumen": documentRow(),
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(toAttachmentResponse(row, false))
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			body := string(raw)

			for _, forbidden := range []string{
				"youtube_id", "dQw4w9WgXcQ", // identitas video
				"file_url", "/content", // tautan berkas
				"original_filename", "panduan-registrasi-aset.pdf", // nama dokumen internal
				"size_bytes", "1400000", // ukurannya
				"object_key", "guide/9f8e.pdf", // kunci objek
			} {
				if strings.Contains(body, forbidden) {
					t.Fatalf("respons tamu memuat %q: %s", forbidden, body)
				}
			}
			if !strings.Contains(body, `"locked":true`) {
				t.Fatalf("respons tamu tidak ditandai locked: %s", body)
			}
			// Judul dan jenis memang boleh terlihat: itu yang membuat kartu
			// terkunci terbaca sebagai "ada video di sini", bukan sebagai rusak.
			if !strings.Contains(body, `"kind":`) || !strings.Contains(body, `"title_id":`) {
				t.Fatalf("respons tamu kehilangan judul atau jenis: %s", body)
			}
		})
	}
}

func TestAttachmentResponseGivesMediaToSignedInCallers(t *testing.T) {
	video := toAttachmentResponse(videoRow(), true)
	if video.Locked {
		t.Fatal("lampiran video masih terkunci untuk pemanggil yang sudah masuk")
	}
	if video.YoutubeID == nil || *video.YoutubeID != "dQw4w9WgXcQ" {
		t.Fatalf("youtube_id = %v, mau dQw4w9WgXcQ", video.YoutubeID)
	}
	if video.FileURL != nil {
		t.Fatalf("lampiran video tidak boleh punya file_url: %v", *video.FileURL)
	}

	doc := toAttachmentResponse(documentRow(), true)
	if doc.FileURL == nil || *doc.FileURL != "/guide/attachments/33333333-3333-3333-3333-333333333333/content" {
		t.Fatalf("file_url = %v", doc.FileURL)
	}
	if doc.YoutubeID != nil {
		t.Fatalf("lampiran dokumen tidak boleh punya youtube_id: %v", *doc.YoutubeID)
	}
	if doc.OriginalFilename == nil || doc.SizeBytes == nil {
		t.Fatal("nama berkas dan ukuran hilang untuk pemanggil yang sudah masuk")
	}
	// Kunci objek tidak pernah keluar, bahkan ke pemanggil yang berhak.
	raw, _ := json.Marshal(doc)
	if strings.Contains(string(raw), "guide/9f8e.pdf") {
		t.Fatalf("kunci objek bocor ke respons: %s", raw)
	}
}

const moduleBody = `{"slug":"panduan-baru","icon":"i-lucide-book-open","sort_order":3,` +
	`"status":"published","title_id":"Panduan Baru","title_en":"New Guide",` +
	`"body_id":"Isi","steps":[{"text_id":"Langkah satu"}]}`

// Aturan bisnis 5: modul baru selalu lahir sebagai draf. Yang menegakkannya
// adalah ketiadaan field-nya, bukan pemeriksaan yang bisa lupa dipanggil — jadi
// yang diuji adalah bahwa "status":"published" di badan permintaan benar-benar
// tidak sampai ke ModuleInput.
func TestCreateRequestNeverCarriesStatus(t *testing.T) {
	var req moduleCreateRequest
	if err := bindJSON(t, moduleBody, &req); err != nil {
		t.Fatalf("binding gagal: %v", err)
	}
	in := req.toInput()
	if in.Status != "" {
		t.Fatalf("status ikut terikat pada pembuatan: %q", in.Status)
	}
	// Sisa field tetap terbawa — kalau embedding memutusnya, ini yang merah.
	if in.Slug != "panduan-baru" || in.Icon != "i-lucide-book-open" || in.SortOrder != 3 {
		t.Fatalf("field modul hilang setelah binding: %+v", in)
	}
	if in.TitleID != "Panduan Baru" || in.TitleEn == nil || *in.TitleEn != "New Guide" {
		t.Fatalf("judul hilang setelah binding: %+v", in)
	}
	if len(in.Steps) != 1 || in.Steps[0].TextID != "Langkah satu" {
		t.Fatalf("langkah hilang setelah binding: %+v", in.Steps)
	}
}

// Pada pengubahan, status justru wajib: menerbitkan dan menarik kembali ke draf
// adalah penyuntingan modul yang sudah ada.
func TestUpdateRequestCarriesStatus(t *testing.T) {
	var req moduleUpdateRequest
	if err := bindJSON(t, moduleBody, &req); err != nil {
		t.Fatalf("binding gagal: %v", err)
	}
	if got := req.toInput().Status; got != sqlc.SharedGuideStatusPublished {
		t.Fatalf("status = %q, mau published", got)
	}
}

// Validasi harus tetap menembus struct yang disematkan. Tanpa test ini,
// moduleFields yang berhenti divalidasi tidak akan terlihat sampai ada payload
// tanpa judul yang lolos ke database.
func TestModuleRequestValidationStillFires(t *testing.T) {
	cases := map[string]string{
		"judul Indonesia kosong": `{"slug":"s","icon":"i","sort_order":1,"status":"draft","title_id":""}`,
		"slug kosong":            `{"slug":"","icon":"i","sort_order":1,"status":"draft","title_id":"Judul"}`,
		"ikon kosong":            `{"slug":"s","icon":"","sort_order":1,"status":"draft","title_id":"Judul"}`,
		"urutan negatif":         `{"slug":"s","icon":"i","sort_order":-1,"status":"draft","title_id":"Judul"}`,
		"langkah tanpa teks":     `{"slug":"s","icon":"i","sort_order":1,"status":"draft","title_id":"J","steps":[{"text_id":""}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			var create moduleCreateRequest
			if err := bindJSON(t, body, &create); err == nil {
				t.Error("pembuatan menerima payload yang tidak sah")
			}
			var update moduleUpdateRequest
			if err := bindJSON(t, body, &update); err == nil {
				t.Error("pengubahan menerima payload yang tidak sah")
			}
		})
	}
}

// Status yang tidak sah ditolak pada pengubahan, dan status yang hilang juga —
// `oneof` tanpa `required` akan meloloskan string kosong.
func TestUpdateRequestRejectsBadStatus(t *testing.T) {
	for name, body := range map[string]string{
		"status hilang":      `{"slug":"s","icon":"i","sort_order":1,"title_id":"Judul"}`,
		"status kosong":      `{"slug":"s","icon":"i","sort_order":1,"status":"","title_id":"Judul"}`,
		"status tak dikenal": `{"slug":"s","icon":"i","sort_order":1,"status":"archived","title_id":"Judul"}`,
	} {
		t.Run(name, func(t *testing.T) {
			var req moduleUpdateRequest
			if err := bindJSON(t, body, &req); err == nil {
				t.Fatal("pengubahan menerima status yang tidak sah")
			}
		})
	}
}

func TestModuleResponseDecodesSteps(t *testing.T) {
	mod := ModuleWithAttachments{
		Module: sqlc.GuideGuideModule{
			ID:        uuid.MustParse("22222222-2222-2222-2222-222222222222"),
			Slug:      "login-security",
			Icon:      "i-lucide-log-in",
			Status:    sqlc.SharedGuideStatusPublished,
			TitleID:   "Masuk dan Keamanan Akun",
			Steps:     []byte(`[{"text_id":"Buka halaman Masuk","text_en":"Open the Sign In page"},{"text_id":"Isi email"}]`),
			UpdatedAt: pgtype.Timestamptz{Valid: false},
		},
		Attachments: []sqlc.GuideGuideAttachment{videoRow()},
	}

	resp := toModuleResponse(mod, true)
	if len(resp.Steps) != 2 {
		t.Fatalf("jumlah langkah = %d, mau 2", len(resp.Steps))
	}
	if resp.Steps[0].TextEn == nil || *resp.Steps[0].TextEn != "Open the Sign In page" {
		t.Fatalf("langkah pertama kehilangan versi Inggris: %+v", resp.Steps[0])
	}
	if resp.Steps[1].TextEn != nil {
		t.Fatalf("langkah kedua seharusnya tanpa versi Inggris: %+v", resp.Steps[1])
	}
	if len(resp.Attachments) != 1 {
		t.Fatalf("jumlah lampiran = %d, mau 1", len(resp.Attachments))
	}
	if resp.UpdatedAt != nil {
		t.Fatalf("updated_at kosong seharusnya null, dapat %v", *resp.UpdatedAt)
	}
}

// Modul tanpa langkah dan modul dengan jsonb rusak sama-sama harus menghasilkan
// daftar kosong, bukan null — klien merender daftar tanpa memeriksa nil, dan
// satu baris rusak tidak boleh menjatuhkan seluruh halaman.
func TestDecodeStepsDegradesToEmptyList(t *testing.T) {
	for name, raw := range map[string][]byte{
		"nil":            nil,
		"kosong":         {},
		"array kosong":   []byte(`[]`),
		"bukan json":     []byte(`{`),
		"objek":          []byte(`{"text_id":"x"}`),
		"array angka":    []byte(`[1,2,3]`),
		"null literal":   []byte(`null`),
		"string tunggal": []byte(`"langkah"`),
	} {
		t.Run(name, func(t *testing.T) {
			steps := decodeSteps(raw)
			if steps == nil {
				t.Fatal("decodeSteps mengembalikan nil, mau daftar kosong")
			}
			if len(steps) != 0 {
				t.Fatalf("decodeSteps = %+v, mau kosong", steps)
			}
			out, err := json.Marshal(steps)
			if err != nil || string(out) != "[]" {
				t.Fatalf("serialisasi = %s (err %v), mau []", out, err)
			}
		})
	}
}
