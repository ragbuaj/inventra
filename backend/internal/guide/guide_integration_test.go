//go:build integration

package guide_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/auth"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/config"
	"github.com/ragbuaj/inventra/internal/guide"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/storage"
	"github.com/ragbuaj/inventra/internal/testsupport"
)

// These tests run the REAL middleware chain with REAL tokens rather than a stub
// that injects an identity. The point of this module is who may see what, so a
// stub would test the stub. Guest requests carry no Authorization header at all.

const validVideoURL = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

type harness struct {
	router *gin.Engine
	pool   *pgxpool.Pool
	store  *storage.MinIOStorage

	adminToken  string // holds guide.manage
	readerToken string // signed in, no guide.manage
	mobileToken string // holds guide.manage but on a mobile-audience token
}

func newHarness(t *testing.T, maxBytes int64) *harness {
	t.Helper()
	ctx := context.Background()

	pool := testsupport.NewPostgres(t)
	rdb := testsupport.NewRedis(t)
	store := testsupport.NewMinIO(t)

	// Start from an empty guide: migration 000051 seeds nine published modules,
	// and counting them here would couple these tests to that content.
	_, err := pool.Exec(ctx, `TRUNCATE guide.guide_attachments, guide.guide_modules CASCADE`)
	require.NoError(t, err)

	q := sqlc.New(pool)
	officeID := seedOffice(t, pool)
	adminRole := lookupRole(t, pool, "Superadmin") // migration 000050 grants guide.manage
	readerRole := seedRoleWithoutGuideManage(t, pool)

	adminID := seedUser(t, pool, "guide-admin", adminRole, officeID)
	readerID := seedUser(t, pool, "guide-reader", readerRole, officeID)

	cfg := &config.Config{JWTSecret: "test-secret", JWTAccessTTL: 15 * time.Minute, JWTRefreshTTL: time.Hour}
	tm := auth.NewTokenManager(cfg)
	store2 := auth.NewTokenStore(rdb)

	mint := func(userID, roleID uuid.UUID, audience string) string {
		pair, err := tm.Issue(userID.String(), roleID.String(), "", audience)
		require.NoError(t, err)
		return pair.AccessToken
	}

	permSvc := authz.NewPermissionService(q, rdb)
	svc := guide.NewService(q, pool, store, maxBytes)
	h := guide.NewHandler(svc, permSvc, audit.NewService(q))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	guide.RegisterRoutes(router.Group("/api/v1"), h,
		middleware.RequireAuth(tm, store2),
		middleware.OptionalAuth(tm, store2),
		middleware.RequirePermission(permSvc, "guide.manage"),
		middleware.RequireAudience(auth.AudienceWeb),
	)

	return &harness{
		router:      router,
		pool:        pool,
		store:       store,
		adminToken:  mint(adminID, adminRole, auth.AudienceWeb),
		readerToken: mint(readerID, readerRole, auth.AudienceWeb),
		mobileToken: mint(adminID, adminRole, auth.AudienceMobile),
	}
}

func (h *harness) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

func (h *harness) upload(t *testing.T, moduleID uuid.UUID, token, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	require.NoError(t, mw.WriteField("title_id", "Dokumen uji"))
	require.NoError(t, mw.WriteField("sort_order", "1"))

	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	// Deliberately claims PDF: the server must decide from the bytes, not this.
	hdr.Set("Content-Type", "application/pdf")
	fw, err := mw.CreatePart(hdr)
	require.NoError(t, err)
	_, err = fw.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/guide/modules/"+moduleID.String()+"/attachments/document", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, req)
	return w
}

// createModule inserts a module through the API and returns its id.
func (h *harness) createModule(t *testing.T, slug, status string) uuid.UUID {
	t.Helper()
	w := h.do(t, http.MethodPost, "/api/v1/guide/modules", h.adminToken, map[string]any{
		"slug": slug, "icon": "i-lucide-book-open", "status": status,
		"title_id": "Modul " + slug, "sort_order": 1,
		"steps": []map[string]any{{"text_id": "Langkah pertama"}},
	})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var resp struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return uuid.MustParse(resp.ID)
}

func pdfBytes() []byte  { return []byte("%PDF-1.4\n%%EOF\n") }
func htmlBytes() []byte { return []byte("<!doctype html><script>alert(1)</script>") }

// pdfOfSize builds a real-looking PDF of EXACTLY n bytes, so a size limit can be
// tested at the limit instead of far beyond it.
func pdfOfSize(n int) []byte {
	head := []byte("%PDF-1.4\n")
	tail := []byte("\n%%EOF\n")
	if n <= len(head)+len(tail) {
		panic("pdfOfSize: n terlalu kecil untuk sebuah PDF")
	}
	return append(append(head, bytes.Repeat([]byte("A"), n-len(head)-len(tail))...), tail...)
}

// ─── seeds ───────────────────────────────────────────────────────────────────

func seedOffice(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var typeID, officeID uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.office_types (name) VALUES ('GuideOfficeType') RETURNING id`).Scan(&typeID))
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO masterdata.offices (code, name, office_type_id) VALUES ('GDE01','Guide Office',$1) RETURNING id`,
		typeID).Scan(&officeID))
	return officeID
}

func lookupRole(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT id FROM identity.roles WHERE name = $1 AND deleted_at IS NULL`, name).Scan(&id))
	return id
}

func seedRoleWithoutGuideManage(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.roles (code, name, description)
		 VALUES ('guide_reader', 'GuideReader', 'reader for guide tests') RETURNING id`).Scan(&id))
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM identity.role_permissions
		 WHERE role_id = $1 AND permission_key = 'guide.manage' AND deleted_at IS NULL`, id).Scan(&n))
	require.Zero(t, n, "the reader role must not hold guide.manage")
	return id
}

func seedUser(t *testing.T, pool *pgxpool.Pool, name string, roleID, officeID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO identity.users (name, email, role_id, office_id, status)
		 VALUES ($1, $2, $3, $4, 'active') RETURNING id`,
		name, name+"@test.local", roleID, officeID).Scan(&id))
	return id
}

// ─── tests ───────────────────────────────────────────────────────────────────

// The feature's core promise: the page reads without a session, but its media
// does not leak to one.
func TestGuestReadsTextButNeverMedia(t *testing.T) {
	h := newHarness(t, 10<<20)
	moduleID := h.createModule(t, "publik", "published")

	w := h.do(t, http.MethodPost, "/api/v1/guide/modules/"+moduleID.String()+"/attachments/video",
		h.adminToken, map[string]any{"url": validVideoURL, "title_id": "Video uji"})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Equal(t, http.StatusCreated, h.upload(t, moduleID, h.adminToken, "panduan.pdf", pdfBytes()).Code)

	// No Authorization header at all.
	guest := h.do(t, http.MethodGet, "/api/v1/guide/modules", "", nil)
	require.Equal(t, http.StatusOK, guest.Code)
	require.Equal(t, "no-store", guest.Header().Get("Cache-Control"))
	require.Equal(t, "Authorization", guest.Header().Get("Vary"))

	body := guest.Body.String()
	require.Contains(t, body, "Modul publik", "teks panduan harus terbaca tanpa sesi")
	for _, leaked := range []string{"youtube_id", "dQw4w9WgXcQ", "file_url", "original_filename", "size_bytes", "object_key"} {
		require.NotContains(t, body, leaked, "respons tamu membocorkan %q", leaked)
	}
	require.Contains(t, body, `"locked":true`)

	// The same request with a plain session gets the media refs.
	reader := h.do(t, http.MethodGet, "/api/v1/guide/modules", h.readerToken, nil)
	require.Equal(t, http.StatusOK, reader.Code)
	require.Contains(t, reader.Body.String(), "dQw4w9WgXcQ")
	require.Contains(t, reader.Body.String(), "file_url")
	require.Contains(t, reader.Body.String(), `"locked":false`)
}

func TestFileStreamRequiresSessionOnly(t *testing.T) {
	h := newHarness(t, 10<<20)
	moduleID := h.createModule(t, "berkas", "published")
	created := h.upload(t, moduleID, h.adminToken, "panduan.pdf", pdfBytes())
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	var att struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &att))
	path := "/api/v1/guide/attachments/" + att.ID + "/content"

	// Guessing the id is not access: without a session it is 401.
	require.Equal(t, http.StatusUnauthorized, h.do(t, http.MethodGet, path, "", nil).Code)

	// A plain signed-in user needs no permission to read guide media.
	ok := h.do(t, http.MethodGet, path, h.readerToken, nil)
	require.Equal(t, http.StatusOK, ok.Code)
	require.Equal(t, "application/pdf", ok.Header().Get("Content-Type"))
	require.Equal(t, "nosniff", ok.Header().Get("X-Content-Type-Options"))
	require.Equal(t, "sandbox", ok.Header().Get("Content-Security-Policy"))
	require.Contains(t, ok.Header().Get("Content-Disposition"), "inline")
	require.Equal(t, pdfBytes(), ok.Body.Bytes())
}

func TestWritesRequireGuideManageAndWebAudience(t *testing.T) {
	h := newHarness(t, 10<<20)
	moduleID := h.createModule(t, "tulis", "published")

	payload := map[string]any{"slug": "baru", "icon": "i-lucide-book-open", "status": "draft", "title_id": "Baru"}

	require.Equal(t, http.StatusUnauthorized,
		h.do(t, http.MethodPost, "/api/v1/guide/modules", "", payload).Code)
	require.Equal(t, http.StatusForbidden,
		h.do(t, http.MethodPost, "/api/v1/guide/modules", h.readerToken, payload).Code,
		"user tanpa guide.manage tidak boleh menulis")
	require.Equal(t, http.StatusForbidden,
		h.do(t, http.MethodPost, "/api/v1/guide/modules", h.mobileToken, payload).Code,
		"token mobile tidak boleh menyentuh permukaan pengelolaan")

	// Same rule on every other write verb.
	require.Equal(t, http.StatusForbidden,
		h.do(t, http.MethodDelete, "/api/v1/guide/modules/"+moduleID.String(), h.readerToken, nil).Code)
	require.Equal(t, http.StatusForbidden,
		h.do(t, http.MethodPost, "/api/v1/guide/modules/"+moduleID.String()+"/attachments/video",
			h.readerToken, map[string]any{"url": validVideoURL, "title_id": "x"}).Code)
	require.Equal(t, http.StatusUnauthorized,
		h.upload(t, moduleID, "", "panduan.pdf", pdfBytes()).Code)
}

func TestDraftsAreInvisibleWithoutGuideManage(t *testing.T) {
	h := newHarness(t, 10<<20)
	h.createModule(t, "terbit", "published")
	h.createModule(t, "draf", "draft")

	// Guests and plain readers never see a draft, even asking for it explicitly.
	for name, token := range map[string]string{"tamu": "", "pembaca": h.readerToken} {
		t.Run(name, func(t *testing.T) {
			for _, path := range []string{"/api/v1/guide/modules", "/api/v1/guide/modules?status=all"} {
				w := h.do(t, http.MethodGet, path, token, nil)
				require.Equal(t, http.StatusOK, w.Code, "status=all tidak boleh ditolak, hanya diabaikan")
				require.NotContains(t, w.Body.String(), "Modul draf")
				require.Contains(t, w.Body.String(), "Modul terbit")
			}
		})
	}

	// The author sees drafts only when asking for them.
	require.NotContains(t, h.do(t, http.MethodGet, "/api/v1/guide/modules", h.adminToken, nil).Body.String(), "Modul draf")
	require.Contains(t, h.do(t, http.MethodGet, "/api/v1/guide/modules?status=all", h.adminToken, nil).Body.String(), "Modul draf")
}

// The cap is deliberately generous here: with a tiny cap the disguised file
// would trip the size limit first and this test would pass for the wrong reason.
func TestUploadRejectsDisguisedFile(t *testing.T) {
	h := newHarness(t, 10<<20)
	moduleID := h.createModule(t, "menyamar", "published")

	// Extension and part Content-Type both claim PDF; the bytes do not.
	require.Equal(t, http.StatusUnsupportedMediaType,
		h.upload(t, moduleID, h.adminToken, "menyamar.pdf", htmlBytes()).Code)

	list := h.do(t, http.MethodGet, "/api/v1/guide/modules", h.readerToken, nil)
	require.NotContains(t, list.Body.String(), "file_url", "tidak ada lampiran yang boleh tercatat")
}

func TestUploadRejectsOversizedFile(t *testing.T) {
	h := newHarness(t, 64) // smaller than any multipart body this test sends
	moduleID := h.createModule(t, "besar", "published")

	big := append(pdfBytes(), bytes.Repeat([]byte("A"), 4096)...)
	require.Equal(t, http.StatusRequestEntityTooLarge,
		h.upload(t, moduleID, h.adminToken, "besar.pdf", big).Code)

	list := h.do(t, http.MethodGet, "/api/v1/guide/modules", h.readerToken, nil)
	require.NotContains(t, list.Body.String(), "file_url", "unggahan yang ditolak tidak boleh menyisakan baris")
}

// The size rule is per FILE. The body cap must not quietly become a second,
// stricter rule: a file of exactly the advertised limit is legal, and the form
// tells authors so.
func TestUploadSizeLimitIsExactAndPerFile(t *testing.T) {
	const limit = 4096
	h := newHarness(t, limit)
	moduleID := h.createModule(t, "batas-ukuran", "published")

	atLimit := h.upload(t, moduleID, h.adminToken, "pas.pdf", pdfOfSize(limit))
	require.Equal(t, http.StatusCreated, atLimit.Code, atLimit.Body.String())

	over := h.upload(t, moduleID, h.adminToken, "lebih.pdf", pdfOfSize(limit+1))
	require.Equal(t, http.StatusRequestEntityTooLarge, over.Code, over.Body.String())

	// Far over the cap the body reader trips inside the multipart parser, which
	// then reports a malformed MIME header. The caller must still be told the
	// file is too large — not that their request was malformed.
	far := h.upload(t, moduleID, h.adminToken, "raksasa.pdf", pdfOfSize(limit+(2<<20)))
	require.Equal(t, http.StatusRequestEntityTooLarge, far.Code, far.Body.String())
	require.Contains(t, far.Body.String(), "size limit")
	require.NotContains(t, far.Body.String(), "MIME")
}

func TestVideoLinkValidationAtTheEdge(t *testing.T) {
	h := newHarness(t, 10<<20)
	moduleID := h.createModule(t, "video", "published")
	path := "/api/v1/guide/modules/" + moduleID.String() + "/attachments/video"

	for _, bad := range []string{
		"https://vimeo.com/123",
		"javascript:alert(1)",
		"https://evil.example.com/youtube.com/watch?v=dQw4w9WgXcQ",
		"http://youtube.com.evil.example.com/watch?v=dQw4w9WgXcQ",
	} {
		w := h.do(t, http.MethodPost, path, h.adminToken, map[string]any{"url": bad, "title_id": "x"})
		require.Equal(t, http.StatusUnprocessableEntity, w.Code, "tautan %q seharusnya ditolak", bad)
	}

	// Only the id survives a link carrying extra parameters.
	w := h.do(t, http.MethodPost, path, h.adminToken,
		map[string]any{"url": validVideoURL + "&t=90&list=PL1", "title_id": "Video"})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"youtube_id":"dQw4w9WgXcQ"`)
	require.NotContains(t, w.Body.String(), "t=90")
}

func TestAttachmentCapIsEnforced(t *testing.T) {
	h := newHarness(t, 10<<20)
	moduleID := h.createModule(t, "batas", "published")
	path := "/api/v1/guide/modules/" + moduleID.String() + "/attachments/video"

	for i := 0; i < 10; i++ {
		w := h.do(t, http.MethodPost, path, h.adminToken,
			map[string]any{"url": validVideoURL, "title_id": fmt.Sprintf("Video %d", i)})
		require.Equal(t, http.StatusCreated, w.Code, "lampiran ke-%d: %s", i+1, w.Body.String())
	}
	w := h.do(t, http.MethodPost, path, h.adminToken, map[string]any{"url": validVideoURL, "title_id": "Kesebelas"})
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	require.Contains(t, strings.ToLower(w.Body.String()), "limit")
}

// Deleting one attachment must actually remove the bytes; deleting a module must
// not — its rows are gone, so the files are unreachable but still recoverable.
func TestDeletionSemanticsForStoredObjects(t *testing.T) {
	h := newHarness(t, 10<<20)
	ctx := context.Background()

	moduleID := h.createModule(t, "hapus", "published")
	created := h.upload(t, moduleID, h.adminToken, "panduan.pdf", pdfBytes())
	require.Equal(t, http.StatusCreated, created.Code)
	var att struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &att))

	var key string
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT object_key FROM guide.guide_attachments WHERE id = $1`, att.ID).Scan(&key))
	_, _, err := h.store.Get(ctx, key)
	require.NoError(t, err, "objek harus ada setelah unggahan")

	require.Equal(t, http.StatusNoContent,
		h.do(t, http.MethodDelete, "/api/v1/guide/attachments/"+att.ID, h.adminToken, nil).Code)
	_, _, err = h.store.Get(ctx, key)
	require.ErrorIs(t, err, storage.ErrObjectNotFound, "menghapus lampiran harus membuang objeknya")

	require.Equal(t, http.StatusNotFound,
		h.do(t, http.MethodGet, "/api/v1/guide/attachments/"+att.ID+"/content", h.readerToken, nil).Code)

	// A second attachment, then delete the whole module.
	second := h.upload(t, moduleID, h.adminToken, "lain.pdf", pdfBytes())
	require.Equal(t, http.StatusCreated, second.Code)
	var att2 struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &att2))
	var key2 string
	require.NoError(t, h.pool.QueryRow(ctx,
		`SELECT object_key FROM guide.guide_attachments WHERE id = $1`, att2.ID).Scan(&key2))

	require.Equal(t, http.StatusNoContent,
		h.do(t, http.MethodDelete, "/api/v1/guide/modules/"+moduleID.String(), h.adminToken, nil).Code)

	require.Equal(t, http.StatusNotFound,
		h.do(t, http.MethodGet, "/api/v1/guide/attachments/"+att2.ID+"/content", h.readerToken, nil).Code,
		"berkas milik modul terhapus harus tak terjangkau")
	_, _, err = h.store.Get(ctx, key2)
	require.NoError(t, err, "objek milik modul terhapus sengaja dipertahankan")
}

func TestPublishToggleControlsReaderVisibility(t *testing.T) {
	h := newHarness(t, 10<<20)
	moduleID := h.createModule(t, "terbit-tarik", "published")

	require.Contains(t, h.do(t, http.MethodGet, "/api/v1/guide/modules", "", nil).Body.String(), "Modul terbit-tarik")

	w := h.do(t, http.MethodPatch, "/api/v1/guide/modules/"+moduleID.String(), h.adminToken, map[string]any{
		"slug": "terbit-tarik", "icon": "i-lucide-book-open", "status": "draft",
		"title_id": "Modul terbit-tarik", "sort_order": 1,
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"published_at":null`, "menarik ke draf mengosongkan published_at")

	require.NotContains(t, h.do(t, http.MethodGet, "/api/v1/guide/modules", "", nil).Body.String(), "Modul terbit-tarik")
}

// Media follows the status of its module. A draft is an authoring workspace, so
// its PDF is readable by an author and by nobody else — the attachment id is not
// a capability, exactly as the object key behind it is not one.
func TestDraftMediaIsReadableOnlyByAuthors(t *testing.T) {
	h := newHarness(t, 10<<20)
	moduleID := h.createModule(t, "draf-berkas", "draft")
	created := h.upload(t, moduleID, h.adminToken, "rahasia.pdf", pdfBytes())
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	var att struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &att))
	path := "/api/v1/guide/attachments/" + att.ID + "/content"

	// The author who is drafting it can still preview it.
	require.Equal(t, http.StatusOK, h.do(t, http.MethodGet, path, h.adminToken, nil).Code)

	// A signed-in user without guide.manage gets 404, NOT 403: the answer must be
	// indistinguishable from an id that never existed, or it confirms a draft is
	// there.
	denied := h.do(t, http.MethodGet, path, h.readerToken, nil)
	require.Equal(t, http.StatusNotFound, denied.Code, denied.Body.String())
	require.NotContains(t, denied.Body.String(), "draft")

	// And no session at all is still 401 — the earlier gate has not moved.
	require.Equal(t, http.StatusUnauthorized, h.do(t, http.MethodGet, path, "", nil).Code)
}

// Withdrawing a module has to withdraw its files. Every reader who loaded the
// page while it was published holds the attachment ids, so a rule that stopped
// at "any session" would leave the PDF downloadable forever after a document was
// published by mistake — the one containment step the spec relies on.
func TestPullingAModuleBackToDraftRevokesItsMedia(t *testing.T) {
	h := newHarness(t, 10<<20)
	moduleID := h.createModule(t, "tarik-berkas", "published")
	created := h.upload(t, moduleID, h.adminToken, "terlanjur.pdf", pdfBytes())
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())

	var att struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(created.Body.Bytes(), &att))
	path := "/api/v1/guide/attachments/" + att.ID + "/content"

	// While published, any signed-in reader may read it. This is the moment the
	// id becomes public knowledge.
	require.Equal(t, http.StatusOK, h.do(t, http.MethodGet, path, h.readerToken, nil).Code)

	w := h.do(t, http.MethodPatch, "/api/v1/guide/modules/"+moduleID.String(), h.adminToken, map[string]any{
		"slug": "tarik-berkas", "icon": "i-lucide-book-open", "status": "draft",
		"title_id": "Modul tarik-berkas", "sort_order": 1,
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	require.Equal(t, http.StatusNotFound,
		h.do(t, http.MethodGet, path, h.readerToken, nil).Code,
		"menarik modul ke draf harus ikut menarik berkasnya")
	require.Equal(t, http.StatusOK, h.do(t, http.MethodGet, path, h.adminToken, nil).Code,
		"penulisnya tetap bisa membukanya")
}

func TestSlugConflictIsReported(t *testing.T) {
	h := newHarness(t, 10<<20)
	h.createModule(t, "kembar", "published")

	w := h.do(t, http.MethodPost, "/api/v1/guide/modules", h.adminToken, map[string]any{
		"slug": "kembar", "icon": "i-lucide-book-open", "status": "draft", "title_id": "Kembar lagi",
	})
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}
