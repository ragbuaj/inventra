package identity

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/ratelimit"
)

// withCaller returns gin middleware that stamps CtxUserID, simulating what
// RequireAuth does on a real authed route (these tests call the handler
// directly, bypassing the router/middleware wiring).
func withCaller(id uuid.UUID) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.CtxUserID, id.String())
		c.Next()
	}
}

func doJSON(t *testing.T, r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// --- getProfile --------------------------------------------------------

func TestGetProfile_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	u := activeUserEmail(t, "u@x.com")
	role, office, emp := "Asset Manager", "Cabang Jakarta Selatan", "Andi Saputra"
	fs := &fakeStore{profiles: map[uuid.UUID]sqlc.GetUserProfileRow{
		u.ID: {ID: u.ID, Name: u.Name, Email: u.Email, RoleName: &role, OfficeName: &office, EmployeeName: &emp},
	}}
	h := &Handler{svc: newTestService(t, fs, &fakeMailer{})}
	r := gin.New()
	r.GET("/profile", withCaller(u.ID), h.getProfile)

	w := doJSON(t, r, http.MethodGet, "/profile", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["email"] != "u@x.com" {
		t.Fatalf("want email field u@x.com, got %v", got["email"])
	}
	if got["role_name"] != role || got["office_name"] != office || got["employee_name"] != emp {
		t.Fatalf("want enriched names role=%q office=%q employee=%q, got %v/%v/%v",
			role, office, emp, got["role_name"], got["office_name"], got["employee_name"])
	}
	if _, hasPwHash := got["password_hash"]; hasPwHash {
		t.Fatalf("must never serialize password_hash")
	}
	if _, hasGoogleID := got["google_id"]; hasGoogleID {
		t.Fatalf("must never serialize google_id")
	}
}

func TestGetProfileHandler_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fs := &fakeStore{}
	h := &Handler{svc: newTestService(t, fs, &fakeMailer{})}
	r := gin.New()
	r.GET("/profile", withCaller(uuid.New()), h.getProfile)

	w := doJSON(t, r, http.MethodGet, "/profile", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestGetProfile_Unauthorized_NoCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{} // no CtxUserID set, must 401 before touching (nil) svc
	r := gin.New()
	r.GET("/profile", h.getProfile)

	w := doJSON(t, r, http.MethodGet, "/profile", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

// --- updateProfile -------------------------------------------------------

func TestUpdateProfile_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{
		byID:     map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{u.ID: {ID: u.ID, Name: u.Name, Email: u.Email}},
	}
	h := &Handler{svc: newTestService(t, fs, &fakeMailer{})}
	r := gin.New()
	r.PUT("/profile", withCaller(u.ID), h.updateProfile)

	w := doJSON(t, r, http.MethodPut, "/profile", updateProfileRequest{Name: "  Budi Baru  ", Phone: "0812"})
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["name"] != "Budi Baru" {
		t.Fatalf("want trimmed name in response, got %v", got["name"])
	}
}

func TestUpdateProfile_EmptyName_422(t *testing.T) {
	gin.SetMode(gin.TestMode)
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{
		byID:     map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{u.ID: {ID: u.ID, Name: u.Name, Email: u.Email}},
	}
	h := &Handler{svc: newTestService(t, fs, &fakeMailer{})}
	r := gin.New()
	r.PUT("/profile", withCaller(u.ID), h.updateProfile)

	w := doJSON(t, r, http.MethodPut, "/profile", updateProfileRequest{Name: "   "})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateProfile_BindError_MissingName_400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	r := gin.New()
	r.PUT("/profile", withCaller(uuid.New()), h.updateProfile)

	w := doJSON(t, r, http.MethodPut, "/profile", map[string]any{"phone": "0812"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 (missing required name), got %d", w.Code)
	}
}

// --- requestEmailChange ---------------------------------------------------

func TestRequestEmailChange_WrongPassword_400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	u := activeUserEmail(t, "old@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	h := &Handler{svc: newTestService(t, fs, &fakeMailer{})}
	r := gin.New()
	r.POST("/email/change-request", withCaller(u.ID), h.requestEmailChange)

	// Must be 400, not 401: the frontend's authenticated-request interceptor
	// treats any 401 as an expired access token and force-logs the user out.
	w := doJSON(t, r, http.MethodPost, "/email/change-request", emailChangeRequest{NewEmail: "new@x.com", CurrentPassword: "wrong"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["error"] != "password salah" {
		t.Fatalf("want error message %q, got %v", "password salah", got["error"])
	}
}

func TestRequestEmailChange_SameEmail_409(t *testing.T) {
	gin.SetMode(gin.TestMode)
	u := activeUserEmail(t, "same@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	h := &Handler{svc: newTestService(t, fs, &fakeMailer{})}
	r := gin.New()
	r.POST("/email/change-request", withCaller(u.ID), h.requestEmailChange)

	w := doJSON(t, r, http.MethodPost, "/email/change-request", emailChangeRequest{NewEmail: "same@x.com", CurrentPassword: "oldpassword"})
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRequestEmailChange_EmailInUse_409(t *testing.T) {
	gin.SetMode(gin.TestMode)
	u := activeUserEmail(t, "old@x.com")
	other := activeUserEmail(t, "taken@x.com")
	fs := &fakeStore{
		byID:    map[uuid.UUID]sqlc.IdentityUser{u.ID: u, other.ID: other},
		byEmail: map[string]sqlc.IdentityUser{"taken@x.com": other},
	}
	h := &Handler{svc: newTestService(t, fs, &fakeMailer{})}
	r := gin.New()
	r.POST("/email/change-request", withCaller(u.ID), h.requestEmailChange)

	w := doJSON(t, r, http.MethodPost, "/email/change-request", emailChangeRequest{NewEmail: "taken@x.com", CurrentPassword: "oldpassword"})
	if w.Code != http.StatusConflict {
		t.Fatalf("want 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRequestEmailChange_BindError_InvalidEmail_400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	r := gin.New()
	r.POST("/email/change-request", withCaller(uuid.New()), h.requestEmailChange)

	w := doJSON(t, r, http.MethodPost, "/email/change-request", map[string]any{"new_email": "not-an-email", "current_password": "x"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 (invalid email format), got %d", w.Code)
	}
}

func TestRequestEmailChange_Unauthorized_NoCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	r := gin.New()
	r.POST("/email/change-request", h.requestEmailChange)

	w := doJSON(t, r, http.MethodPost, "/email/change-request", emailChangeRequest{NewEmail: "new@x.com", CurrentPassword: "x"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

// --- confirmEmailChange (public route) ------------------------------------

func TestConfirmEmailChange_BadToken_400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fs := &fakeStore{}
	h := &Handler{svc: newTestService(t, fs, &fakeMailer{})}
	r := gin.New()
	r.POST("/email/confirm", h.confirmEmailChange)

	w := doJSON(t, r, http.MethodPost, "/email/confirm", emailConfirmRequest{Token: "bogus"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestConfirmEmailChange_BindError_MissingToken_400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	r := gin.New()
	r.POST("/email/confirm", h.confirmEmailChange)

	w := doJSON(t, r, http.MethodPost, "/email/confirm", map[string]any{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 (missing token), got %d", w.Code)
	}
}

// --- requestPasswordChange -------------------------------------------------

func TestRequestPasswordChange_WrongPassword_400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	h := &Handler{svc: newTestService(t, fs, &fakeMailer{})}
	r := gin.New()
	r.POST("/password/change-request", withCaller(u.ID), h.requestPasswordChange)

	// Must be 400, not 401: the frontend's authenticated-request interceptor
	// treats any 401 as an expired access token and force-logs the user out.
	w := doJSON(t, r, http.MethodPost, "/password/change-request", passwordChangeRequestRequest{CurrentPassword: "wrong"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d: %s", w.Code, w.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["error"] != "password lama salah" {
		t.Fatalf("want error message %q, got %v", "password lama salah", got["error"])
	}
}

func TestRequestPasswordChange_BindError_MissingPassword_400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	r := gin.New()
	r.POST("/password/change-request", withCaller(uuid.New()), h.requestPasswordChange)

	w := doJSON(t, r, http.MethodPost, "/password/change-request", map[string]any{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 (missing current_password), got %d", w.Code)
	}
}

func TestRequestPasswordChange_Unauthorized_NoCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	r := gin.New()
	r.POST("/password/change-request", h.requestPasswordChange)

	w := doJSON(t, r, http.MethodPost, "/password/change-request", passwordChangeRequestRequest{CurrentPassword: "x"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

// --- forgotPassword (public route): anti-enumeration HTTP contract ---------

// passLimiter always allows, so these tests exercise the handler body rather
// than the rate-limit guard. (Named distinctly from the integration build's
// allowLimiter to avoid a redeclaration when both files compile together.)
type passLimiter struct{}

func (passLimiter) Allow(_ context.Context, _ string, _ int, _ bool) ratelimit.Result {
	return ratelimit.Result{Allowed: true}
}

// A known and an unknown address MUST yield an identical 200 response so the
// endpoint never reveals whether an account exists. The mobile Lupa Password
// screen relies on this: it shows the same confirmation for any input.
func TestForgotPassword_Always200_IdenticalBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	u := activeUserEmail(t, "known@x.com")
	fs := &fakeStore{
		byEmail: map[string]sqlc.IdentityUser{"known@x.com": u},
		byID:    map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
	}
	h := &Handler{svc: newTestService(t, fs, &fakeMailer{}), limiter: passLimiter{}, forgotPerMin: 100}
	r := gin.New()
	r.POST("/password/forgot", h.forgotPassword)

	known := doJSON(t, r, http.MethodPost, "/password/forgot", forgotPasswordRequest{Email: "known@x.com"})
	unknown := doJSON(t, r, http.MethodPost, "/password/forgot", forgotPasswordRequest{Email: "ghost@x.com"})

	if known.Code != http.StatusOK {
		t.Fatalf("known email: want 200, got %d: %s", known.Code, known.Body.String())
	}
	if unknown.Code != http.StatusOK {
		t.Fatalf("unknown email: want 200, got %d: %s", unknown.Code, unknown.Body.String())
	}
	if known.Body.String() != unknown.Body.String() {
		t.Fatalf("body must be identical (anti-enumeration): known=%q unknown=%q",
			known.Body.String(), unknown.Body.String())
	}
}

// A malformed address is a client format error (400), which is NOT enumeration
// — it says nothing about account existence.
func TestForgotPassword_InvalidEmail_400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{limiter: passLimiter{}, forgotPerMin: 100}
	r := gin.New()
	r.POST("/password/forgot", h.forgotPassword)

	w := doJSON(t, r, http.MethodPost, "/password/forgot", map[string]any{"email": "not-an-email"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for malformed email, got %d", w.Code)
	}
}

// --- device sessions: auth guards (no caller → 401 before touching svc) ----

func TestListSessions_Unauthorized_NoCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	r := gin.New()
	r.GET("/sessions", h.listSessions)

	w := doJSON(t, r, http.MethodGet, "/sessions", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestRevokeSession_Unauthorized_NoCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	r := gin.New()
	r.DELETE("/sessions/:id", h.revokeSession)

	w := doJSON(t, r, http.MethodDelete, "/sessions/some-sid", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestRevokeOtherSessions_Unauthorized_NoCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{}
	r := gin.New()
	r.POST("/sessions/revoke-others", h.revokeOtherSessions)

	w := doJSON(t, r, http.MethodPost, "/sessions/revoke-others", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}
