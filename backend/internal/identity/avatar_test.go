package identity

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ragbuaj/inventra/db/sqlc"
)

// pngBytes renders a w x h solid-colour PNG — valid, decodable input.
func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 120, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// avatarFixture wires a user that exists in both the user map and the profile
// map, so GetProfile succeeds after an avatar write.
func avatarFixture(t *testing.T) (*Service, *fakeStore, uuid.UUID) {
	t.Helper()
	u := activeUserEmail(t, "avatar@x.com")
	fs := &fakeStore{
		byEmail:  map[string]sqlc.IdentityUser{u.Email: u},
		byID:     map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{u.ID: {ID: u.ID, Name: u.Name, Email: u.Email, RoleID: u.RoleID, Status: u.Status}},
	}
	svc := newTestService(t, fs, &fakeMailer{})
	return svc, fs, u.ID
}

func TestUploadAvatar_StoresNormalizedJPEG(t *testing.T) {
	u := activeUserEmail(t, "avatar@x.com")
	fs := &fakeStore{
		byEmail:  map[string]sqlc.IdentityUser{u.Email: u},
		byID:     map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{u.ID: {ID: u.ID, Name: u.Name, Email: u.Email, RoleID: u.RoleID, Status: u.Status}},
	}
	svc, objStore := newTestServiceWithStorage(t, fs, &fakeMailer{})

	view, err := svc.UploadAvatar(context.Background(), u.ID, pngBytes(t, 800, 400), "image/png")
	if err != nil {
		t.Fatalf("UploadAvatar: %v", err)
	}
	if !view.HasAvatar {
		t.Error("expected HasAvatar true in the returned profile")
	}

	key := avatarKey(u.ID)
	if !objStore.Has(key) {
		t.Fatalf("expected object at %q, have %v", key, objStore.ObjsKeys())
	}
	rc, info, err := objStore.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()
	if info.ContentType != "image/jpeg" {
		t.Errorf("stored content type = %q, want image/jpeg", info.ContentType)
	}
	// A PNG upload must be re-encoded to JPEG and cropped to a centered square.
	data, _ := io.ReadAll(rc)
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode stored object: %v", err)
	}
	if format != "jpeg" {
		t.Errorf("stored format = %q, want jpeg", format)
	}
	if cfg.Width != avatarDim || cfg.Height != avatarDim {
		t.Errorf("stored size = %dx%d, want %dx%d", cfg.Width, cfg.Height, avatarDim, avatarDim)
	}
}

func TestUploadAvatar_RejectsUnsupportedType(t *testing.T) {
	svc, _, userID := avatarFixture(t)
	// A real, decodable PNG — but declared as a type the profile screen doesn't
	// offer, so it must be refused on the declared type alone.
	for _, ct := range []string{"image/webp", "application/pdf", "image/gif", "", "text/plain"} {
		if _, err := svc.UploadAvatar(context.Background(), userID, pngBytes(t, 10, 10), ct); !errors.Is(err, ErrUnsupportedType) {
			t.Errorf("content type %q: err = %v, want ErrUnsupportedType", ct, err)
		}
	}
}

func TestUploadAvatar_RejectsUndecodableData(t *testing.T) {
	svc, _, userID := avatarFixture(t)
	// Claims to be a PNG but isn't — the decode step is the real content check.
	if _, err := svc.UploadAvatar(context.Background(), userID, []byte("not an image at all"), "image/png"); !errors.Is(err, ErrUnsupportedType) {
		t.Errorf("err = %v, want ErrUnsupportedType", err)
	}
}

func TestUploadAvatar_RejectsOversize(t *testing.T) {
	u := activeUserEmail(t, "avatar@x.com")
	fs := &fakeStore{
		byEmail:  map[string]sqlc.IdentityUser{u.Email: u},
		byID:     map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{u.ID: {ID: u.ID}},
	}
	svc, objStore := newTestServiceWithStorage(t, fs, &fakeMailer{})
	svc.avatarMaxBytes = 100

	if _, err := svc.UploadAvatar(context.Background(), u.ID, pngBytes(t, 200, 200), "image/png"); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
	if keys := objStore.ObjsKeys(); len(keys) != 0 {
		t.Errorf("rejected upload must not store anything, have %v", keys)
	}
}

func TestUploadAvatar_RollsBackObjectWhenDBWriteFails(t *testing.T) {
	u := activeUserEmail(t, "avatar@x.com")
	fs := &fakeStore{
		byEmail:   map[string]sqlc.IdentityUser{u.Email: u},
		byID:      map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles:  map[uuid.UUID]sqlc.GetUserProfileRow{u.ID: {ID: u.ID}},
		avatarErr: errors.New("db down"),
	}
	svc, objStore := newTestServiceWithStorage(t, fs, &fakeMailer{})

	if _, err := svc.UploadAvatar(context.Background(), u.ID, pngBytes(t, 50, 50), "image/png"); err == nil {
		t.Fatal("expected an error when the avatar-key write fails")
	}
	if keys := objStore.ObjsKeys(); len(keys) != 0 {
		t.Errorf("expected the stored object to be rolled back, have %v", keys)
	}
}

func TestUploadAvatar_OverwritesPreviousObject(t *testing.T) {
	u := activeUserEmail(t, "avatar@x.com")
	fs := &fakeStore{
		byEmail:  map[string]sqlc.IdentityUser{u.Email: u},
		byID:     map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{u.ID: {ID: u.ID}},
	}
	svc, objStore := newTestServiceWithStorage(t, fs, &fakeMailer{})

	for i := 0; i < 3; i++ {
		if _, err := svc.UploadAvatar(context.Background(), u.ID, pngBytes(t, 60+i, 60+i), "image/png"); err != nil {
			t.Fatalf("upload %d: %v", i, err)
		}
	}
	// The key is derived from the user id alone, so re-uploading must not
	// accumulate orphaned objects.
	if keys := objStore.ObjsKeys(); len(keys) != 1 {
		t.Errorf("expected exactly 1 stored object after 3 uploads, have %v", keys)
	}
}

func TestUploadAvatar_AcceptsJPEG(t *testing.T) {
	svc, _, userID := avatarFixture(t)
	img := image.NewRGBA(image.Rect(0, 0, 120, 90))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	if _, err := svc.UploadAvatar(context.Background(), userID, buf.Bytes(), "image/jpeg"); err != nil {
		t.Fatalf("UploadAvatar(jpeg): %v", err)
	}
}

func TestUploadAvatar_NoStorageConfigured(t *testing.T) {
	svc, _, userID := avatarFixture(t)
	svc.storage = nil
	if _, err := svc.UploadAvatar(context.Background(), userID, pngBytes(t, 10, 10), "image/png"); !errors.Is(err, ErrAvatarUnavailable) {
		t.Errorf("err = %v, want ErrAvatarUnavailable", err)
	}
}

func TestGetAvatar_NotSet(t *testing.T) {
	svc, _, userID := avatarFixture(t)
	if _, _, err := svc.GetAvatar(context.Background(), userID); !errors.Is(err, ErrNoAvatar) {
		t.Errorf("err = %v, want ErrNoAvatar", err)
	}
}

func TestGetAvatar_RowPointsAtMissingObject(t *testing.T) {
	u := activeUserEmail(t, "avatar@x.com")
	key := avatarKey(u.ID)
	u.AvatarKey = &key
	fs := &fakeStore{
		byEmail:  map[string]sqlc.IdentityUser{u.Email: u},
		byID:     map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{u.ID: {ID: u.ID}},
	}
	svc, _ := newTestServiceWithStorage(t, fs, &fakeMailer{}) // storage is empty

	// A dangling key must read as "no avatar", not as a 500.
	if _, _, err := svc.GetAvatar(context.Background(), u.ID); !errors.Is(err, ErrNoAvatar) {
		t.Errorf("err = %v, want ErrNoAvatar", err)
	}
}

func TestGetAvatar_ReturnsStoredBytes(t *testing.T) {
	u := activeUserEmail(t, "avatar@x.com")
	fs := &fakeStore{
		byEmail:  map[string]sqlc.IdentityUser{u.Email: u},
		byID:     map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{u.ID: {ID: u.ID}},
	}
	svc, _ := newTestServiceWithStorage(t, fs, &fakeMailer{})
	if _, err := svc.UploadAvatar(context.Background(), u.ID, pngBytes(t, 70, 70), "image/png"); err != nil {
		t.Fatalf("UploadAvatar: %v", err)
	}

	rc, info, err := svc.GetAvatar(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("GetAvatar: %v", err)
	}
	defer rc.Close()
	if info.ContentType != "image/jpeg" || info.Size == 0 {
		t.Errorf("info = %+v, want jpeg with non-zero size", info)
	}
	data, _ := io.ReadAll(rc)
	if _, _, err := image.Decode(bytes.NewReader(data)); err != nil {
		t.Errorf("served bytes are not a decodable image: %v", err)
	}
}

func TestGetAvatar_UnknownUser(t *testing.T) {
	fs := &fakeStore{byEmail: map[string]sqlc.IdentityUser{}, byID: map[uuid.UUID]sqlc.IdentityUser{}}
	svc, _ := newTestServiceWithStorage(t, fs, &fakeMailer{})
	if _, _, err := svc.GetAvatar(context.Background(), uuid.New()); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("err = %v, want pgx.ErrNoRows", err)
	}
}

func TestRemoveAvatar_ClearsKeyAndObject(t *testing.T) {
	u := activeUserEmail(t, "avatar@x.com")
	fs := &fakeStore{
		byEmail:  map[string]sqlc.IdentityUser{u.Email: u},
		byID:     map[uuid.UUID]sqlc.IdentityUser{u.ID: u},
		profiles: map[uuid.UUID]sqlc.GetUserProfileRow{u.ID: {ID: u.ID}},
	}
	svc, objStore := newTestServiceWithStorage(t, fs, &fakeMailer{})
	if _, err := svc.UploadAvatar(context.Background(), u.ID, pngBytes(t, 40, 40), "image/png"); err != nil {
		t.Fatalf("UploadAvatar: %v", err)
	}

	view, err := svc.RemoveAvatar(context.Background(), u.ID)
	if err != nil {
		t.Fatalf("RemoveAvatar: %v", err)
	}
	if view.HasAvatar {
		t.Error("expected HasAvatar false after removal")
	}
	if keys := objStore.ObjsKeys(); len(keys) != 0 {
		t.Errorf("expected the object to be deleted, have %v", keys)
	}
	if _, _, err := svc.GetAvatar(context.Background(), u.ID); !errors.Is(err, ErrNoAvatar) {
		t.Errorf("GetAvatar after removal: err = %v, want ErrNoAvatar", err)
	}
}

func TestRemoveAvatar_NoAvatarIsANoop(t *testing.T) {
	svc, _, userID := avatarFixture(t)
	// Removing a non-existent avatar is idempotent, not an error.
	view, err := svc.RemoveAvatar(context.Background(), userID)
	if err != nil {
		t.Fatalf("RemoveAvatar: %v", err)
	}
	if view.HasAvatar {
		t.Error("expected HasAvatar false")
	}
}

func TestAvatarKey_IsDerivedFromUserIDOnly(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	if avatarKey(a) == avatarKey(b) {
		t.Error("two users must not share an avatar key")
	}
	if got, want := avatarKey(a), "users/"+a.String()+"/avatar.jpg"; got != want {
		t.Errorf("avatarKey = %q, want %q", got, want)
	}
}
