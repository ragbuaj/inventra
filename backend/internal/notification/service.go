// Package notification implements the per-user in-app notification feed.
// See docs/superpowers/specs/2026-07-17-notifications-design.md.
package notification

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// Service-level errors.
var (
	ErrNotFound = errors.New("not found")
)

// ReadFilter carries the tri-state "read" query filter of GET /notifications.
type ReadFilter int

// Read filter values: no filter, unread only, read only.
const (
	ReadFilterAll ReadFilter = iota
	ReadFilterUnread
	ReadFilterRead
)

// notificationStore is the data surface the Service needs (seam for tests).
// *sqlc.Queries satisfies it.
type notificationStore interface {
	ListNotifications(ctx context.Context, arg sqlc.ListNotificationsParams) ([]sqlc.NotificationNotification, error)
	CountNotifications(ctx context.Context, arg sqlc.CountNotificationsParams) (int64, error)
	CountUnreadNotifications(ctx context.Context, userID uuid.UUID) (int64, error)
	MarkNotificationRead(ctx context.Context, arg sqlc.MarkNotificationReadParams) (sqlc.NotificationNotification, error)
	MarkAllNotificationsRead(ctx context.Context, userID uuid.UUID) error
}

// Service reads and mutates the caller's own notification feed. Every method
// takes the caller's user id and threads it into the query predicate: the feed
// has no data-scope layer, ownership is the whole authorization model.
type Service struct {
	q notificationStore
}

// NewService builds the notification Service.
func NewService(q notificationStore) *Service {
	return &Service{q: q}
}

// ListInput describes one page of a user's feed.
type ListInput struct {
	UserID uuid.UUID
	Read   ReadFilter
	Limit  int32
	Offset int32
}

// List returns one page of the user's notifications plus the total matching the
// same filter.
func (s *Service) List(ctx context.Context, in ListInput) ([]sqlc.NotificationNotification, int64, error) {
	unreadOnly, readOnly := in.Read == ReadFilterUnread, in.Read == ReadFilterRead
	rows, err := s.q.ListNotifications(ctx, sqlc.ListNotificationsParams{
		UserID:     in.UserID,
		UnreadOnly: unreadOnly,
		ReadOnly:   readOnly,
		Off:        in.Offset,
		Lim:        in.Limit,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	total, err := s.q.CountNotifications(ctx, sqlc.CountNotificationsParams{
		UserID:     in.UserID,
		UnreadOnly: unreadOnly,
		ReadOnly:   readOnly,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	return rows, total, nil
}

// UnreadCount returns how many unread notifications the user has.
func (s *Service) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	n, err := s.q.CountUnreadNotifications(ctx, userID)
	if err != nil {
		return 0, mapDBError(err)
	}
	return n, nil
}

// MarkRead marks one notification read. userID is part of the update predicate,
// so a notification owned by someone else matches no row and comes back as
// ErrNotFound (the handler turns that into 404, never 403 -- a 403 would confirm
// the id exists).
func (s *Service) MarkRead(ctx context.Context, id, userID uuid.UUID) (sqlc.NotificationNotification, error) {
	row, err := s.q.MarkNotificationRead(ctx, sqlc.MarkNotificationReadParams{ID: id, UserID: userID})
	if err != nil {
		return sqlc.NotificationNotification{}, mapDBError(err)
	}
	return row, nil
}

// MarkAllRead marks every unread notification of the user read.
func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	if err := s.q.MarkAllNotificationsRead(ctx, userID); err != nil {
		return mapDBError(err)
	}
	return nil
}

// mapDBError translates driver errors into the package's sentinel errors.
func mapDBError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
