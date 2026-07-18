package identity

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // register jpeg decoder
	_ "image/png"  // register png decoder
	"io"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/storage"
)

// avatarDim is the square edge every stored avatar is normalized to. The UI
// renders it at 60px (and 96px on the header), so 512 leaves room for HiDPI
// without storing anything close to the upload limit.
const avatarDim = 512

// avatarAllowedMIME is deliberately narrower than the asset-attachment
// allowlist: the profile screen advertises "JPG atau PNG", and every upload is
// re-encoded to JPEG anyway.
var avatarAllowedMIME = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
}

// avatarKey is the object key for a user's avatar. It is derived solely from
// the user id — never from the uploaded filename — so a hostile filename can
// neither traverse paths nor collide with another user's object. Because every
// upload is re-encoded to JPEG the extension is constant, which also means a
// re-upload overwrites the previous object instead of orphaning it.
func avatarKey(userID uuid.UUID) string {
	return fmt.Sprintf("users/%s/avatar.jpg", userID)
}

// UploadAvatar validates, normalizes and stores the caller's avatar, then
// records the object key. The image is cropped to a centered square and
// re-encoded to JPEG, which strips any EXIF metadata (including GPS) that the
// original file carried.
func (s *Service) UploadAvatar(ctx context.Context, userID uuid.UUID, data []byte, contentType string) (ProfileView, error) {
	if s.storage == nil {
		return ProfileView{}, ErrAvatarUnavailable
	}
	if !avatarAllowedMIME[contentType] {
		return ProfileView{}, ErrUnsupportedType
	}
	if int64(len(data)) > s.avatarMaxBytes {
		return ProfileView{}, ErrTooLarge
	}
	// Decoding is also the real content check: a file that merely claims to be
	// image/jpeg but isn't decodable is rejected here rather than stored.
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ProfileView{}, ErrUnsupportedType
	}
	var buf bytes.Buffer
	if err := imaging.Encode(&buf, imaging.Fill(img, avatarDim, avatarDim, imaging.Center, imaging.Lanczos), imaging.JPEG, imaging.JPEGQuality(85)); err != nil {
		return ProfileView{}, err
	}

	key := avatarKey(userID)
	body := buf.Bytes()
	if err := s.storage.Put(ctx, key, bytes.NewReader(body), int64(len(body)), "image/jpeg"); err != nil {
		return ProfileView{}, err
	}
	if err := s.q.UpdateUserAvatarKey(ctx, sqlcUpdateAvatarParams(userID, &key)); err != nil {
		// Best-effort rollback so a failed write doesn't leave an orphan object
		// (same ordering as asset attachments).
		_ = s.storage.Remove(ctx, key)
		return ProfileView{}, err
	}
	return s.GetProfile(ctx, userID)
}

// RemoveAvatar clears the caller's avatar. The database row is cleared first:
// if the object delete then fails the user still sees the avatar gone, and the
// orphaned object is overwritten by the next upload (the key is stable).
func (s *Service) RemoveAvatar(ctx context.Context, userID uuid.UUID) (ProfileView, error) {
	row, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return ProfileView{}, err
	}
	if row.AvatarKey == nil {
		return s.GetProfile(ctx, userID)
	}
	if err := s.q.UpdateUserAvatarKey(ctx, sqlcUpdateAvatarParams(userID, nil)); err != nil {
		return ProfileView{}, err
	}
	if s.storage != nil {
		_ = s.storage.Remove(ctx, *row.AvatarKey) // best-effort
	}
	return s.GetProfile(ctx, userID)
}

// GetAvatar streams the caller's stored avatar. The caller must close the
// returned reader. ErrNoAvatar is returned when none is set, and also when the
// row points at an object that is missing from storage.
func (s *Service) GetAvatar(ctx context.Context, userID uuid.UUID) (io.ReadCloser, storage.ObjectInfo, error) {
	if s.storage == nil {
		return nil, storage.ObjectInfo{}, ErrAvatarUnavailable
	}
	row, err := s.q.GetUserByID(ctx, userID)
	if err != nil {
		return nil, storage.ObjectInfo{}, err
	}
	if row.AvatarKey == nil {
		return nil, storage.ObjectInfo{}, ErrNoAvatar
	}
	rc, info, err := s.storage.Get(ctx, *row.AvatarKey)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			return nil, storage.ObjectInfo{}, ErrNoAvatar
		}
		return nil, storage.ObjectInfo{}, err
	}
	return rc, info, nil
}
