package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/auth"
	"github.com/ragbuaj/inventra/internal/config"
)

type fakeStore struct {
	byEmail map[string]sqlc.IdentityUser
	byID    map[uuid.UUID]sqlc.IdentityUser
	updated map[uuid.UUID]string // userID -> new hash

	profiles     map[uuid.UUID]sqlc.GetUserProfileRow
	nameUpdates  map[uuid.UUID]string
	emailUpdates map[uuid.UUID]string
	phoneUpdates map[uuid.UUID]*string // employeeID -> new phone (nil clears)
}

func (f *fakeStore) GetUserByID(_ context.Context, id uuid.UUID) (sqlc.IdentityUser, error) {
	u, ok := f.byID[id]
	if !ok {
		return sqlc.IdentityUser{}, pgx.ErrNoRows
	}
	return u, nil
}
func (f *fakeStore) GetUserByEmail(_ context.Context, e string) (sqlc.IdentityUser, error) {
	u, ok := f.byEmail[e]
	if !ok {
		return sqlc.IdentityUser{}, pgx.ErrNoRows
	}
	return u, nil
}
func (f *fakeStore) LinkGoogleID(_ context.Context, _ sqlc.LinkGoogleIDParams) error { return nil }
func (f *fakeStore) UpdateUserPassword(_ context.Context, a sqlc.UpdateUserPasswordParams) error {
	if f.updated == nil {
		f.updated = map[uuid.UUID]string{}
	}
	f.updated[a.ID] = *a.PasswordHash
	return nil
}
func (f *fakeStore) GetUserProfile(_ context.Context, id uuid.UUID) (sqlc.GetUserProfileRow, error) {
	row, ok := f.profiles[id]
	if !ok {
		return sqlc.GetUserProfileRow{}, pgx.ErrNoRows
	}
	return row, nil
}
func (f *fakeStore) UpdateUserName(_ context.Context, a sqlc.UpdateUserNameParams) (sqlc.IdentityUser, error) {
	u, ok := f.byID[a.ID]
	if !ok {
		return sqlc.IdentityUser{}, pgx.ErrNoRows
	}
	u.Name = a.Name
	f.byID[a.ID] = u
	if f.nameUpdates == nil {
		f.nameUpdates = map[uuid.UUID]string{}
	}
	f.nameUpdates[a.ID] = a.Name
	if row, ok := f.profiles[a.ID]; ok {
		row.Name = a.Name
		f.profiles[a.ID] = row
	}
	return u, nil
}
func (f *fakeStore) UpdateUserEmail(_ context.Context, a sqlc.UpdateUserEmailParams) (sqlc.IdentityUser, error) {
	u, ok := f.byID[a.ID]
	if !ok {
		return sqlc.IdentityUser{}, pgx.ErrNoRows
	}
	u.Email = a.Email
	f.byID[a.ID] = u
	if f.emailUpdates == nil {
		f.emailUpdates = map[uuid.UUID]string{}
	}
	f.emailUpdates[a.ID] = a.Email
	if row, ok := f.profiles[a.ID]; ok {
		row.Email = a.Email
		f.profiles[a.ID] = row
	}
	return u, nil
}
func (f *fakeStore) UpdateEmployeePhone(_ context.Context, a sqlc.UpdateEmployeePhoneParams) error {
	if f.phoneUpdates == nil {
		f.phoneUpdates = map[uuid.UUID]*string{}
	}
	f.phoneUpdates[a.ID] = a.Phone
	for id, row := range f.profiles {
		if row.EmployeeID != nil && *row.EmployeeID == a.ID {
			row.EmployeePhone = a.Phone
			f.profiles[id] = row
		}
	}
	return nil
}

type fakeMailer struct {
	resetLink, changedTo string
	resetTo              string
	verifyLink           string
	verifyTo             string
	emailChangedTo       string
}

func (m *fakeMailer) SendPasswordReset(_ context.Context, to, _, link string) error {
	m.resetTo = to
	m.resetLink = link
	return nil
}
func (m *fakeMailer) SendPasswordChanged(_ context.Context, to, _ string) error {
	m.changedTo = to
	return nil
}
func (m *fakeMailer) SendEmailChangeVerify(_ context.Context, to, _, link string) error {
	m.verifyTo = to
	m.verifyLink = link
	return nil
}
func (m *fakeMailer) SendEmailChanged(_ context.Context, to, _, _ string) error {
	m.emailChangedTo = to
	return nil
}

func activeUserEmail(t *testing.T, email string) sqlc.IdentityUser {
	t.Helper()
	h, _ := auth.HashPassword("oldpassword")
	return sqlc.IdentityUser{ID: uuid.New(), Email: email, Name: "Budi", Status: sqlc.SharedUserStatusActive, PasswordHash: &h}
}

func newTestService(t *testing.T, fs *fakeStore, fm *fakeMailer) *Service {
	t.Helper()
	cfg := &config.Config{JWTSecret: "test-secret", JWTAccessTTL: 15 * time.Minute, JWTRefreshTTL: time.Hour}
	tm := auth.NewTokenManager(cfg)
	// The token store wraps an UNREACHABLE Redis (fast dial timeout) rather than
	// nil: it lets branches that call s.store fail fast with an error instead of
	// panicking on a nil receiver, so we can unit test every pre-store branch
	// (bad credentials, same-email, email-in-use, unknown token, ...) without a
	// real Redis. Tests asserting an actual store WRITE (save reset/email-change
	// token) are integration-level — see service_integration_test.go.
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 50 * time.Millisecond})
	return NewService(fs, tm, auth.NewTokenStore(rdb), fm, nil, 30*time.Minute, "https://app")
}

func TestChangePassword_WrongOld(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc := newTestService(t, fs, &fakeMailer{})
	if _, err := svc.ChangePassword(context.Background(), u.ID, "nope", "brandnewpass"); err != ErrInvalidCredentials {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestChangePassword_WeakNew(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc := newTestService(t, fs, &fakeMailer{})
	if _, err := svc.ChangePassword(context.Background(), u.ID, "oldpassword", "short"); err != ErrWeakPassword {
		t.Fatalf("want ErrWeakPassword, got %v", err)
	}
}

func TestChangePassword_Success_UpdatesHashAndNotifies(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	fm := &fakeMailer{}
	svc := newTestService(t, fs, fm)
	if _, err := svc.ChangePassword(context.Background(), u.ID, "oldpassword", "brandnewpass"); err != nil {
		t.Fatalf("change: %v", err)
	}
	if _, ok := fs.updated[u.ID]; !ok {
		t.Fatalf("password not updated")
	}
	if fm.changedTo != "u@x.com" {
		t.Fatalf("notification not sent")
	}
}

func TestRequestPasswordReset_UnknownEmail_SilentOK(t *testing.T) {
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{}}
	fm := &fakeMailer{}
	svc := newTestService(t, fs, fm)
	if err := svc.RequestPasswordReset(context.Background(), "ghost@x.com"); err != nil {
		t.Fatalf("want nil (anti-enumeration), got %v", err)
	}
	if fm.resetLink != "" {
		t.Fatalf("no email should be sent for unknown account")
	}
}

func TestRequestPasswordReset_GoogleOnly_SilentOK(t *testing.T) {
	u := activeUserEmail(t, "g@x.com")
	u.PasswordHash = nil // Google-only
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{"g@x.com": u}}
	fm := &fakeMailer{}
	svc := newTestService(t, fs, fm)
	if err := svc.RequestPasswordReset(context.Background(), "g@x.com"); err != nil {
		t.Fatalf("got %v", err)
	}
	if fm.resetLink != "" {
		t.Fatalf("Google-only account must not receive a reset link")
	}
}

func TestRequestPasswordReset_InactiveUser_SilentOK(t *testing.T) {
	u := activeUserEmail(t, "inactive@x.com")
	u.Status = sqlc.SharedUserStatusInactive
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{"inactive@x.com": u}}
	fm := &fakeMailer{}
	svc := newTestService(t, fs, fm)
	if err := svc.RequestPasswordReset(context.Background(), "inactive@x.com"); err != nil {
		t.Fatalf("want nil (anti-enumeration), got %v", err)
	}
	if fm.resetLink != "" {
		t.Fatalf("inactive account must not receive a reset link")
	}
}

// --- RequestEmailChange ---------------------------------------------------

func TestRequestEmailChange_WrongPassword_ErrInvalidCredentials(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc := newTestService(t, fs, &fakeMailer{})
	err := svc.RequestEmailChange(context.Background(), u.ID, "new@x.com", "nope")
	if err != ErrInvalidCredentials {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestRequestEmailChange_GoogleOnly_ErrInvalidCredentials(t *testing.T) {
	u := activeUserEmail(t, "g@x.com")
	u.PasswordHash = nil // Google-only accounts have no password to verify.
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc := newTestService(t, fs, &fakeMailer{})
	err := svc.RequestEmailChange(context.Background(), u.ID, "new@x.com", "whatever")
	if err != ErrInvalidCredentials {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestRequestEmailChange_SameEmail_ErrSameEmail(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc := newTestService(t, fs, &fakeMailer{})
	// Case-insensitive match against the current email must also be rejected.
	err := svc.RequestEmailChange(context.Background(), u.ID, "U@X.com", "oldpassword")
	if err != ErrSameEmail {
		t.Fatalf("want ErrSameEmail, got %v", err)
	}
}

func TestRequestEmailChange_EmailInUse_ErrEmailInUse(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	other := activeUserEmail(t, "taken@x.com")
	fs := &fakeStore{
		byID:    map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		byEmail: map[string]sqlc.IdentityUser{"taken@x.com": other},
	}
	svc := newTestService(t, fs, &fakeMailer{})
	err := svc.RequestEmailChange(context.Background(), u.ID, "taken@x.com", "oldpassword")
	if err != ErrEmailInUse {
		t.Fatalf("want ErrEmailInUse, got %v", err)
	}
}

// --- RequestPasswordChange -------------------------------------------------

func TestRequestPasswordChange_WrongPassword_ErrInvalidCredentials(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc := newTestService(t, fs, &fakeMailer{})
	err := svc.RequestPasswordChange(context.Background(), u.ID, "nope")
	if err != ErrInvalidCredentials {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

func TestRequestPasswordChange_GoogleOnly_ErrInvalidCredentials(t *testing.T) {
	u := activeUserEmail(t, "g@x.com")
	u.PasswordHash = nil
	fs := &fakeStore{byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u}}
	svc := newTestService(t, fs, &fakeMailer{})
	err := svc.RequestPasswordChange(context.Background(), u.ID, "whatever")
	if err != ErrInvalidCredentials {
		t.Fatalf("want ErrInvalidCredentials, got %v", err)
	}
}

// --- ConfirmEmailChange ------------------------------------------------------

func TestConfirmEmailChange_UnknownToken_ErrInvalidToken(t *testing.T) {
	fs := &fakeStore{}
	svc := newTestService(t, fs, &fakeMailer{})
	_, err := svc.ConfirmEmailChange(context.Background(), "not-a-real-token")
	if err != ErrInvalidToken {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}

// --- UpdateProfile / GetProfile ---------------------------------------------

func TestUpdateProfile_EmptyName_ErrInvalidInput(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{
		byID:     map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{u.ID: {ID: u.ID, Name: u.Name, Email: u.Email}},
	}
	svc := newTestService(t, fs, &fakeMailer{})
	_, err := svc.UpdateProfile(context.Background(), u.ID, "   ", "0800")
	if err != ErrInvalidInput {
		t.Fatalf("want ErrInvalidInput, got %v", err)
	}
}

func TestUpdateProfile_TrimsNameWhitespace(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{
		byID:     map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{u.ID: {ID: u.ID, Name: u.Name, Email: u.Email}},
	}
	svc := newTestService(t, fs, &fakeMailer{})
	view, err := svc.UpdateProfile(context.Background(), u.ID, "  Budi Baru  ", "")
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if view.Name != "Budi Baru" {
		t.Fatalf("want trimmed name %q, got %q", "Budi Baru", view.Name)
	}
	if fs.nameUpdates[u.ID] != "Budi Baru" {
		t.Fatalf("want the STORED name trimmed, got %q", fs.nameUpdates[u.ID])
	}
}

func TestUpdateProfile_WithEmployee_UpdatesOwnEmployeePhoneOnly(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	empID := uuid.New()
	fs := &fakeStore{
		byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{
			u.ID: {ID: u.ID, Name: u.Name, Email: u.Email, EmployeeID: &empID},
		},
	}
	svc := newTestService(t, fs, &fakeMailer{})
	view, err := svc.UpdateProfile(context.Background(), u.ID, "Budi Baru", "0812-3456")
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	if view.Name != "Budi Baru" {
		t.Fatalf("want updated name, got %q", view.Name)
	}
	// The ONLY employee row touched must be the caller's own employee id —
	// UpdateProfile never accepts an employee id from the caller.
	if len(fs.phoneUpdates) != 1 {
		t.Fatalf("want exactly one phone update, got %d", len(fs.phoneUpdates))
	}
	phone, ok := fs.phoneUpdates[empID]
	if !ok {
		t.Fatalf("expected phone update for caller's own employee id %s", empID)
	}
	if phone == nil || *phone != "0812-3456" {
		t.Fatalf("want phone 0812-3456, got %v", phone)
	}
	if view.Phone == nil || *view.Phone != "0812-3456" {
		t.Fatalf("want profile view phone 0812-3456, got %v", view.Phone)
	}
}

func TestUpdateProfile_WhitespaceOnlyPhone_ClearsToNil(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	empID := uuid.New()
	fs := &fakeStore{
		byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{
			u.ID: {ID: u.ID, Name: u.Name, Email: u.Email, EmployeeID: &empID},
		},
	}
	svc := newTestService(t, fs, &fakeMailer{})
	view, err := svc.UpdateProfile(context.Background(), u.ID, "Budi Baru", "   ")
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	phone, ok := fs.phoneUpdates[empID]
	if !ok {
		t.Fatalf("expected phone update for caller's own employee id %s", empID)
	}
	if phone != nil {
		t.Fatalf("want nil phone (whitespace-only trims to empty), got %q", *phone)
	}
	if view.Phone != nil {
		t.Fatalf("want nil profile view phone, got %q", *view.Phone)
	}
}

func TestUpdateProfile_PhoneWithSurroundingWhitespace_IsTrimmed(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	empID := uuid.New()
	fs := &fakeStore{
		byID: map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{
			u.ID: {ID: u.ID, Name: u.Name, Email: u.Email, EmployeeID: &empID},
		},
	}
	svc := newTestService(t, fs, &fakeMailer{})
	view, err := svc.UpdateProfile(context.Background(), u.ID, "Budi Baru", " 0812 ")
	if err != nil {
		t.Fatalf("UpdateProfile: %v", err)
	}
	phone, ok := fs.phoneUpdates[empID]
	if !ok || phone == nil || *phone != "0812" {
		t.Fatalf("want trimmed phone 0812, got %v", phone)
	}
	if view.Phone == nil || *view.Phone != "0812" {
		t.Fatalf("want profile view phone 0812, got %v", view.Phone)
	}
}

func TestUpdateProfile_WithoutEmployee_SkipsPhoneUpdate(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	fs := &fakeStore{
		byID:     map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{u.ID: {ID: u.ID, Name: u.Name, Email: u.Email}}, // EmployeeID nil
	}
	svc := newTestService(t, fs, &fakeMailer{})
	view, err := svc.UpdateProfile(context.Background(), u.ID, "Budi Baru", "0812-3456")
	if err != nil {
		t.Fatalf("UpdateProfile (no employee) must not panic/error, got %v", err)
	}
	if len(fs.phoneUpdates) != 0 {
		t.Fatalf("want no phone update when caller has no linked employee, got %v", fs.phoneUpdates)
	}
	if view.Phone != nil {
		t.Fatalf("want nil phone (no employee), got %v", *view.Phone)
	}
}

func TestGetProfile_GoogleLinkedReflectsGoogleID(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	googleSub := "sub-123"
	fs := &fakeStore{
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{
			u.ID: {ID: u.ID, Name: u.Name, Email: u.Email, GoogleID: &googleSub},
		},
	}
	svc := newTestService(t, fs, &fakeMailer{})
	view, err := svc.GetProfile(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if !view.GoogleLinked {
		t.Fatalf("want GoogleLinked=true when google_id is set")
	}
}

func TestGetProfile_EnrichedNames(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	role, office, emp := "Asset Manager", "Cabang Jakarta Selatan", "Andi Saputra"
	fs := &fakeStore{
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{
			u.ID: {ID: u.ID, Name: u.Name, Email: u.Email, RoleName: &role, OfficeName: &office, EmployeeName: &emp},
		},
	}
	svc := newTestService(t, fs, &fakeMailer{})
	view, err := svc.GetProfile(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if view.RoleName == nil || *view.RoleName != role {
		t.Fatalf("want role_name %q, got %v", role, view.RoleName)
	}
	if view.OfficeName == nil || *view.OfficeName != office {
		t.Fatalf("want office_name %q, got %v", office, view.OfficeName)
	}
	if view.EmployeeName == nil || *view.EmployeeName != emp {
		t.Fatalf("want employee_name %q, got %v", emp, view.EmployeeName)
	}
}

func TestGetProfile_NullOfficeEmployeeNames(t *testing.T) {
	u := activeUserEmail(t, "u@x.com")
	role := "Superadmin"
	fs := &fakeStore{
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{
			u.ID: {ID: u.ID, Name: u.Name, Email: u.Email, RoleName: &role}, // no office/employee link
		},
	}
	svc := newTestService(t, fs, &fakeMailer{})
	view, err := svc.GetProfile(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if view.OfficeName != nil {
		t.Fatalf("want nil office_name for unlinked office, got %q", *view.OfficeName)
	}
	if view.EmployeeName != nil {
		t.Fatalf("want nil employee_name for unlinked employee, got %q", *view.EmployeeName)
	}
	if view.RoleName == nil || *view.RoleName != role {
		t.Fatalf("want role_name %q, got %v", role, view.RoleName)
	}
}

func TestGetProfile_NotFound(t *testing.T) {
	fs := &fakeStore{}
	svc := newTestService(t, fs, &fakeMailer{})
	if _, err := svc.GetProfile(context.Background(), uuid.New()); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("want pgx.ErrNoRows, got %v", err)
	}
}
