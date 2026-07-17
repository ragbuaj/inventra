package notification

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// --- fake store ---------------------------------------------------------

// fakeStore is an in-memory notificationStore. It reproduces the real queries'
// user_id predicate faithfully -- that predicate is what the cross-user tests
// below assert on, so the fake must not shortcut it.
type fakeStore struct {
	rows []sqlc.NotificationNotification

	listErr  error
	countErr error
	markErr  error
	allErr   error

	// lastList records the params the handler passed through, so the clamp and
	// filter assertions can inspect them.
	lastList  sqlc.ListNotificationsParams
	lastCount sqlc.CountNotificationsParams
	markedAll []uuid.UUID
}

// matches mirrors the WHERE clause of ListNotifications/CountNotifications.
func matches(n sqlc.NotificationNotification, userID uuid.UUID, unreadOnly, readOnly bool) bool {
	if n.UserID != userID || n.DeletedAt.Valid {
		return false
	}
	if unreadOnly && n.ReadAt.Valid {
		return false
	}
	if readOnly && !n.ReadAt.Valid {
		return false
	}
	return true
}

func (f *fakeStore) filtered(userID uuid.UUID, unreadOnly, readOnly bool) []sqlc.NotificationNotification {
	out := []sqlc.NotificationNotification{}
	for _, n := range f.rows {
		if matches(n, userID, unreadOnly, readOnly) {
			out = append(out, n)
		}
	}
	return out
}

func (f *fakeStore) ListNotifications(_ context.Context, a sqlc.ListNotificationsParams) ([]sqlc.NotificationNotification, error) {
	f.lastList = a
	if f.listErr != nil {
		return nil, f.listErr
	}
	all := f.filtered(a.UserID, a.UnreadOnly, a.ReadOnly)
	off := int(a.Off)
	if off > len(all) {
		off = len(all)
	}
	end := off + int(a.Lim)
	if end > len(all) {
		end = len(all)
	}
	return all[off:end], nil
}

func (f *fakeStore) CountNotifications(_ context.Context, a sqlc.CountNotificationsParams) (int64, error) {
	f.lastCount = a
	if f.countErr != nil {
		return 0, f.countErr
	}
	return int64(len(f.filtered(a.UserID, a.UnreadOnly, a.ReadOnly))), nil
}

func (f *fakeStore) CountUnreadNotifications(_ context.Context, userID uuid.UUID) (int64, error) {
	if f.countErr != nil {
		return 0, f.countErr
	}
	return int64(len(f.filtered(userID, true, false))), nil
}

func (f *fakeStore) MarkNotificationRead(_ context.Context, a sqlc.MarkNotificationReadParams) (sqlc.NotificationNotification, error) {
	if f.markErr != nil {
		return sqlc.NotificationNotification{}, f.markErr
	}
	for i := range f.rows {
		n := &f.rows[i]
		// user_id is part of the predicate, not just the lookup: another user's
		// id must miss and surface as pgx.ErrNoRows.
		if n.ID == a.ID && n.UserID == a.UserID && !n.DeletedAt.Valid {
			n.ReadAt = ts(time.Now())
			return *n, nil
		}
	}
	return sqlc.NotificationNotification{}, pgx.ErrNoRows
}

func (f *fakeStore) MarkAllNotificationsRead(_ context.Context, userID uuid.UUID) error {
	if f.allErr != nil {
		return f.allErr
	}
	f.markedAll = append(f.markedAll, userID)
	for i := range f.rows {
		if f.rows[i].UserID == userID && !f.rows[i].DeletedAt.Valid {
			f.rows[i].ReadAt = ts(time.Now())
		}
	}
	return nil
}

// --- helpers ------------------------------------------------------------

func ts(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// notif builds a row owned by userID. read decides whether read_at is set.
func notif(userID uuid.UUID, typ sqlc.SharedNotificationType, read bool) sqlc.NotificationNotification {
	n := sqlc.NotificationNotification{
		ID:        uuid.New(),
		UserID:    userID,
		Type:      typ,
		Params:    []byte(`{"asset":"Laptop","when":"besok"}`),
		CreatedAt: ts(time.Now()),
	}
	if read {
		n.ReadAt = ts(time.Now())
	}
	return n
}

// newRouter mounts the real routes with a stub auth middleware that stamps
// CtxUserID the way RequireAuth does.
func newRouter(fs *fakeStore, caller *uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewHandler(NewService(fs))
	r := gin.New()
	authMW := func(c *gin.Context) {
		if caller != nil {
			c.Set(middleware.CtxUserID, caller.String())
		}
		c.Next()
	}
	RegisterRoutes(r.Group("/api/v1"), h, authMW)
	return r
}

func do(t *testing.T, r http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", w.Body.String(), err)
	}
	return got
}

// --- list ---------------------------------------------------------------

func TestList_Success(t *testing.T) {
	u := uuid.New()
	other := uuid.New()
	fs := &fakeStore{rows: []sqlc.NotificationNotification{
		notif(u, sqlc.SharedNotificationTypeMaintenanceDue, false),
		notif(u, sqlc.SharedNotificationTypeApprovalPending, true),
		notif(other, sqlc.SharedNotificationTypeAssetReturned, false),
	}}
	w := do(t, newRouter(fs, &u), http.MethodGet, "/api/v1/notifications")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	got := decode(t, w)
	data, _ := got["data"].([]any)
	if len(data) != 2 {
		t.Fatalf("want the caller's 2 rows only, got %d: %s", len(data), w.Body.String())
	}
	if got["total"] != float64(2) {
		t.Fatalf("want total 2 (other user's row excluded), got %v", got["total"])
	}
	if got["limit"] != float64(20) || got["offset"] != float64(0) {
		t.Fatalf("want default limit 20 / offset 0, got %v / %v", got["limit"], got["offset"])
	}
	if fs.lastList.UserID != u {
		t.Fatalf("query must be scoped to the caller: want %v, got %v", u, fs.lastList.UserID)
	}
}

// TestList_ParamsSerializeAsObject pins the jsonb passthrough: params must reach
// the client as an object, not the base64 string a raw []byte would marshal to.
func TestList_ParamsSerializeAsObject(t *testing.T) {
	u := uuid.New()
	n := notif(u, sqlc.SharedNotificationTypeMaintenanceDue, false)
	entityType := "assets"
	entityID := uuid.New()
	n.EntityType, n.EntityID = &entityType, &entityID
	fs := &fakeStore{rows: []sqlc.NotificationNotification{n}}

	w := do(t, newRouter(fs, &u), http.MethodGet, "/api/v1/notifications")
	got := decode(t, w)
	item := got["data"].([]any)[0].(map[string]any)

	params, ok := item["params"].(map[string]any)
	if !ok {
		t.Fatalf("params must serialize as a JSON object, got %T (%v)", item["params"], item["params"])
	}
	if params["asset"] != "Laptop" || params["when"] != "besok" {
		t.Fatalf("params must survive intact, got %v", params)
	}
	if item["type"] != "maintenance_due" {
		t.Fatalf("want type maintenance_due, got %v", item["type"])
	}
	if item["entity_type"] != "assets" || item["entity_id"] != entityID.String() {
		t.Fatalf("want entity assets/%s, got %v/%v", entityID, item["entity_type"], item["entity_id"])
	}
	if item["read_at"] != nil {
		t.Fatalf("want read_at null on an unread row, got %v", item["read_at"])
	}
	// Internal plumbing must not leak into the feed.
	for _, k := range []string{"user_id", "dedup_key", "deleted_at", "updated_at"} {
		if _, present := item[k]; present {
			t.Fatalf("must not serialize internal column %q: %s", k, w.Body.String())
		}
	}
}

// TestList_ParamsFallback covers a NULL or malformed jsonb column: the response
// must stay valid JSON rather than emitting a broken params value.
func TestList_ParamsFallback(t *testing.T) {
	cases := map[string][]byte{
		"nil params":       nil,
		"empty params":     {},
		"malformed params": []byte("{not json"),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			u := uuid.New()
			n := notif(u, sqlc.SharedNotificationTypeAssetReturned, false)
			n.Params = raw
			fs := &fakeStore{rows: []sqlc.NotificationNotification{n}}

			w := do(t, newRouter(fs, &u), http.MethodGet, "/api/v1/notifications")
			if w.Code != http.StatusOK {
				t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
			}
			got := decode(t, w)
			item := got["data"].([]any)[0].(map[string]any)
			params, ok := item["params"].(map[string]any)
			if !ok || len(params) != 0 {
				t.Fatalf("want params {} fallback, got %T (%v)", item["params"], item["params"])
			}
		})
	}
}

func TestList_Empty(t *testing.T) {
	u := uuid.New()
	w := do(t, newRouter(&fakeStore{}, &u), http.MethodGet, "/api/v1/notifications")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	got := decode(t, w)
	data, ok := got["data"].([]any)
	if !ok {
		t.Fatalf("data must be an empty array, never null: %s", w.Body.String())
	}
	if len(data) != 0 || got["total"] != float64(0) {
		t.Fatalf("want empty feed, got %s", w.Body.String())
	}
}

// TestList_OtherUsersFeedIsInvisible is the read half of user isolation: a
// caller with no rows of their own sees nothing, however full the table is.
func TestList_OtherUsersFeedIsInvisible(t *testing.T) {
	u, other := uuid.New(), uuid.New()
	fs := &fakeStore{rows: []sqlc.NotificationNotification{
		notif(other, sqlc.SharedNotificationTypeApprovalPending, false),
		notif(other, sqlc.SharedNotificationTypeApprovalDecided, true),
	}}
	w := do(t, newRouter(fs, &u), http.MethodGet, "/api/v1/notifications")
	got := decode(t, w)
	if len(got["data"].([]any)) != 0 || got["total"] != float64(0) {
		t.Fatalf("caller must never see another user's feed: %s", w.Body.String())
	}
}

func TestList_ReadFilter(t *testing.T) {
	cases := []struct {
		name           string
		query          string
		wantUnreadOnly bool
		wantReadOnly   bool
		wantCount      int
	}{
		{name: "absent returns the whole feed", query: "", wantCount: 3},
		{name: "read=false returns unread only", query: "?read=false", wantUnreadOnly: true, wantCount: 2},
		{name: "read=true returns read only", query: "?read=true", wantReadOnly: true, wantCount: 1},
		{name: "unparseable read widens to the whole feed", query: "?read=maybe", wantCount: 3},
		{name: "empty read widens to the whole feed", query: "?read=", wantCount: 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := uuid.New()
			fs := &fakeStore{rows: []sqlc.NotificationNotification{
				notif(u, sqlc.SharedNotificationTypeMaintenanceDue, false),
				notif(u, sqlc.SharedNotificationTypeApprovalPending, false),
				notif(u, sqlc.SharedNotificationTypeApprovalDecided, true),
			}}
			w := do(t, newRouter(fs, &u), http.MethodGet, "/api/v1/notifications"+tc.query)
			if w.Code != http.StatusOK {
				t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
			}
			got := decode(t, w)
			if len(got["data"].([]any)) != tc.wantCount || got["total"] != float64(tc.wantCount) {
				t.Fatalf("want %d rows, got %s", tc.wantCount, w.Body.String())
			}
			if fs.lastList.UnreadOnly != tc.wantUnreadOnly || fs.lastList.ReadOnly != tc.wantReadOnly {
				t.Fatalf("want unread_only=%v read_only=%v, got %v/%v",
					tc.wantUnreadOnly, tc.wantReadOnly, fs.lastList.UnreadOnly, fs.lastList.ReadOnly)
			}
			// total must be counted under the same filter as the page.
			if fs.lastCount.UnreadOnly != fs.lastList.UnreadOnly || fs.lastCount.ReadOnly != fs.lastList.ReadOnly {
				t.Fatalf("count filter must match list filter: list %v/%v vs count %v/%v",
					fs.lastList.UnreadOnly, fs.lastList.ReadOnly, fs.lastCount.UnreadOnly, fs.lastCount.ReadOnly)
			}
		})
	}
}

func TestList_LimitOffsetClamp(t *testing.T) {
	cases := []struct {
		name       string
		query      string
		wantLimit  int32
		wantOffset int32
	}{
		{name: "defaults", query: "", wantLimit: 20, wantOffset: 0},
		{name: "limit above 100 clamps to 100", query: "?limit=500", wantLimit: 100, wantOffset: 0},
		{name: "limit exactly 100 is kept", query: "?limit=100", wantLimit: 100, wantOffset: 0},
		{name: "limit below 1 clamps to 1", query: "?limit=0", wantLimit: 1, wantOffset: 0},
		{name: "negative limit clamps to 1", query: "?limit=-5", wantLimit: 1, wantOffset: 0},
		{name: "non-numeric limit falls back to the default", query: "?limit=abc", wantLimit: 20, wantOffset: 0},
		{name: "offset passes through", query: "?offset=2", wantLimit: 20, wantOffset: 2},
		{name: "negative offset clamps to 0", query: "?offset=-3", wantLimit: 20, wantOffset: 0},
		{name: "limit and offset together", query: "?limit=1&offset=1", wantLimit: 1, wantOffset: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := uuid.New()
			fs := &fakeStore{rows: []sqlc.NotificationNotification{
				notif(u, sqlc.SharedNotificationTypeMaintenanceDue, false),
				notif(u, sqlc.SharedNotificationTypeApprovalPending, false),
				notif(u, sqlc.SharedNotificationTypeApprovalDecided, true),
			}}
			w := do(t, newRouter(fs, &u), http.MethodGet, "/api/v1/notifications"+tc.query)
			if w.Code != http.StatusOK {
				t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
			}
			if fs.lastList.Lim != tc.wantLimit || fs.lastList.Off != tc.wantOffset {
				t.Fatalf("want lim=%d off=%d passed to the query, got lim=%d off=%d",
					tc.wantLimit, tc.wantOffset, fs.lastList.Lim, fs.lastList.Off)
			}
			got := decode(t, w)
			if got["limit"] != float64(tc.wantLimit) || got["offset"] != float64(tc.wantOffset) {
				t.Fatalf("response must echo the clamped values %d/%d, got %v/%v",
					tc.wantLimit, tc.wantOffset, got["limit"], got["offset"])
			}
			// total is the unpaginated match count, independent of the page window.
			if got["total"] != float64(3) {
				t.Fatalf("want total 3 regardless of paging, got %v", got["total"])
			}
		})
	}
}

// TestList_Pagination walks a page window and asserts the page shrinks while
// total stays whole.
func TestList_Pagination(t *testing.T) {
	u := uuid.New()
	fs := &fakeStore{rows: []sqlc.NotificationNotification{
		notif(u, sqlc.SharedNotificationTypeMaintenanceDue, false),
		notif(u, sqlc.SharedNotificationTypeApprovalPending, false),
		notif(u, sqlc.SharedNotificationTypeApprovalDecided, true),
	}}
	r := newRouter(fs, &u)

	w := do(t, r, http.MethodGet, "/api/v1/notifications?limit=2&offset=0")
	got := decode(t, w)
	if len(got["data"].([]any)) != 2 || got["total"] != float64(3) {
		t.Fatalf("page 1: want 2 of 3, got %s", w.Body.String())
	}
	w = do(t, r, http.MethodGet, "/api/v1/notifications?limit=2&offset=2")
	got = decode(t, w)
	if len(got["data"].([]any)) != 1 || got["total"] != float64(3) {
		t.Fatalf("page 2: want 1 of 3, got %s", w.Body.String())
	}
	w = do(t, r, http.MethodGet, "/api/v1/notifications?limit=2&offset=99")
	got = decode(t, w)
	if len(got["data"].([]any)) != 0 || got["total"] != float64(3) {
		t.Fatalf("past the end: want 0 of 3, got %s", w.Body.String())
	}
}

func TestList_StoreErrors(t *testing.T) {
	t.Run("list error is 500", func(t *testing.T) {
		u := uuid.New()
		fs := &fakeStore{listErr: errors.New("boom")}
		w := do(t, newRouter(fs, &u), http.MethodGet, "/api/v1/notifications")
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("want 500, got %d: %s", w.Code, w.Body.String())
		}
	})
	t.Run("count error is 500", func(t *testing.T) {
		u := uuid.New()
		fs := &fakeStore{countErr: errors.New("boom")}
		w := do(t, newRouter(fs, &u), http.MethodGet, "/api/v1/notifications")
		if w.Code != http.StatusInternalServerError {
			t.Fatalf("want 500, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// --- unread-count -------------------------------------------------------

func TestUnreadCount_Shape(t *testing.T) {
	u, other := uuid.New(), uuid.New()
	fs := &fakeStore{rows: []sqlc.NotificationNotification{
		notif(u, sqlc.SharedNotificationTypeMaintenanceDue, false),
		notif(u, sqlc.SharedNotificationTypeApprovalPending, false),
		notif(u, sqlc.SharedNotificationTypeApprovalDecided, true),
		notif(other, sqlc.SharedNotificationTypeAssetReturned, false),
	}}
	w := do(t, newRouter(fs, &u), http.MethodGet, "/api/v1/notifications/unread-count")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	got := decode(t, w)
	// Same shape as GET /requests/inbox/count: a bare {"count": n}.
	if len(got) != 1 {
		t.Fatalf("want exactly one key {count}, got %s", w.Body.String())
	}
	if got["count"] != float64(2) {
		t.Fatalf("want count 2 (read row and other user's row excluded), got %v", got["count"])
	}
}

func TestUnreadCount_Zero(t *testing.T) {
	u := uuid.New()
	fs := &fakeStore{rows: []sqlc.NotificationNotification{
		notif(u, sqlc.SharedNotificationTypeApprovalDecided, true),
	}}
	w := do(t, newRouter(fs, &u), http.MethodGet, "/api/v1/notifications/unread-count")
	if got := decode(t, w); got["count"] != float64(0) {
		t.Fatalf("want count 0, got %v", got["count"])
	}
}

func TestUnreadCount_StoreError(t *testing.T) {
	u := uuid.New()
	fs := &fakeStore{countErr: errors.New("boom")}
	w := do(t, newRouter(fs, &u), http.MethodGet, "/api/v1/notifications/unread-count")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- mark read ----------------------------------------------------------

func TestMarkRead_Success(t *testing.T) {
	u := uuid.New()
	n := notif(u, sqlc.SharedNotificationTypeMaintenanceDue, false)
	fs := &fakeStore{rows: []sqlc.NotificationNotification{n}}

	w := do(t, newRouter(fs, &u), http.MethodPost, "/api/v1/notifications/"+n.ID.String()+"/read")
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	got := decode(t, w)
	if got["id"] != n.ID.String() {
		t.Fatalf("want the marked row echoed, got %v", got["id"])
	}
	if got["read_at"] == nil {
		t.Fatalf("want read_at set after mark-read, got null: %s", w.Body.String())
	}
	if fs.rows[0].ReadAt.Valid != true {
		t.Fatalf("row must actually be marked read in the store")
	}
}

// TestMarkRead_OtherUsersNotificationIs404 is the core security assertion: an id
// that exists but belongs to someone else must be indistinguishable from an id
// that does not exist. A 403 would confirm the id is real.
func TestMarkRead_OtherUsersNotificationIs404(t *testing.T) {
	u, other := uuid.New(), uuid.New()
	theirs := notif(other, sqlc.SharedNotificationTypeApprovalPending, false)
	fs := &fakeStore{rows: []sqlc.NotificationNotification{theirs}}

	w := do(t, newRouter(fs, &u), http.MethodPost, "/api/v1/notifications/"+theirs.ID.String()+"/read")
	if w.Code != http.StatusNotFound {
		t.Fatalf("marking another user's notification must be 404 (never 403), got %d: %s", w.Code, w.Body.String())
	}
	if fs.rows[0].ReadAt.Valid {
		t.Fatalf("another user's notification must not be mutated")
	}

	// Compare against a truly absent id: the two responses must be identical, or
	// the status/body difference itself leaks that the id exists.
	missing := do(t, newRouter(fs, &u), http.MethodPost, "/api/v1/notifications/"+uuid.New().String()+"/read")
	if missing.Code != w.Code || missing.Body.String() != w.Body.String() {
		t.Fatalf("existing-but-foreign id must be indistinguishable from an unknown id: %d %q vs %d %q",
			w.Code, w.Body.String(), missing.Code, missing.Body.String())
	}
}

func TestMarkRead_NotFound(t *testing.T) {
	u := uuid.New()
	w := do(t, newRouter(&fakeStore{}, &u), http.MethodPost, "/api/v1/notifications/"+uuid.New().String()+"/read")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestMarkRead_SoftDeletedIs404 pins the deleted_at predicate: an auto-resolved
// notification is gone, not merely hidden.
func TestMarkRead_SoftDeletedIs404(t *testing.T) {
	u := uuid.New()
	n := notif(u, sqlc.SharedNotificationTypeApprovalPending, false)
	n.DeletedAt = ts(time.Now())
	fs := &fakeStore{rows: []sqlc.NotificationNotification{n}}

	w := do(t, newRouter(fs, &u), http.MethodPost, "/api/v1/notifications/"+n.ID.String()+"/read")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404 for a soft-deleted notification, got %d: %s", w.Code, w.Body.String())
	}
}

func TestMarkRead_InvalidUUID(t *testing.T) {
	for _, id := range []string{"not-a-uuid", "123", "%20"} {
		t.Run(id, func(t *testing.T) {
			u := uuid.New()
			w := do(t, newRouter(&fakeStore{}, &u), http.MethodPost, "/api/v1/notifications/"+id+"/read")
			if w.Code != http.StatusBadRequest {
				t.Fatalf("want 400 for id %q, got %d: %s", id, w.Code, w.Body.String())
			}
		})
	}
}

func TestMarkRead_Idempotent(t *testing.T) {
	u := uuid.New()
	n := notif(u, sqlc.SharedNotificationTypeMaintenanceDue, false)
	fs := &fakeStore{rows: []sqlc.NotificationNotification{n}}
	r := newRouter(fs, &u)
	path := "/api/v1/notifications/" + n.ID.String() + "/read"

	if w := do(t, r, http.MethodPost, path); w.Code != http.StatusOK {
		t.Fatalf("first mark-read: want 200, got %d", w.Code)
	}
	if w := do(t, r, http.MethodPost, path); w.Code != http.StatusOK {
		t.Fatalf("re-marking an already-read notification must stay 200, got %d", w.Code)
	}
}

func TestMarkRead_StoreError(t *testing.T) {
	u := uuid.New()
	fs := &fakeStore{markErr: errors.New("boom")}
	w := do(t, newRouter(fs, &u), http.MethodPost, "/api/v1/notifications/"+uuid.New().String()+"/read")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- mark all read ------------------------------------------------------

func TestMarkAllRead_Success(t *testing.T) {
	u, other := uuid.New(), uuid.New()
	fs := &fakeStore{rows: []sqlc.NotificationNotification{
		notif(u, sqlc.SharedNotificationTypeMaintenanceDue, false),
		notif(u, sqlc.SharedNotificationTypeApprovalPending, false),
		notif(other, sqlc.SharedNotificationTypeAssetReturned, false),
	}}
	r := newRouter(fs, &u)

	w := do(t, r, http.MethodPost, "/api/v1/notifications/read-all")
	if w.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d: %s", w.Code, w.Body.String())
	}
	if len(fs.markedAll) != 1 || fs.markedAll[0] != u {
		t.Fatalf("mark-all must be scoped to the caller, got %v", fs.markedAll)
	}
	// The other user's unread row must survive.
	if fs.rows[2].ReadAt.Valid {
		t.Fatalf("mark-all must not touch another user's notifications")
	}
	if cw := do(t, r, http.MethodGet, "/api/v1/notifications/unread-count"); decode(t, cw)["count"] != float64(0) {
		t.Fatalf("want unread count 0 after mark-all, got %s", cw.Body.String())
	}
}

func TestMarkAllRead_EmptyFeed(t *testing.T) {
	u := uuid.New()
	w := do(t, newRouter(&fakeStore{}, &u), http.MethodPost, "/api/v1/notifications/read-all")
	if w.Code != http.StatusNoContent {
		t.Fatalf("mark-all on an empty feed must still be 204, got %d", w.Code)
	}
}

func TestMarkAllRead_StoreError(t *testing.T) {
	u := uuid.New()
	fs := &fakeStore{allErr: errors.New("boom")}
	w := do(t, newRouter(fs, &u), http.MethodPost, "/api/v1/notifications/read-all")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --- routing / auth -----------------------------------------------------

// TestUnauthenticated covers the defensive path: RequireAuth normally rejects
// first, but no handler may fall back to serving a feed with no caller id.
func TestUnauthenticated(t *testing.T) {
	cases := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/notifications"},
		{http.MethodGet, "/api/v1/notifications/unread-count"},
		{http.MethodPost, "/api/v1/notifications/read-all"},
		{http.MethodPost, "/api/v1/notifications/" + uuid.New().String() + "/read"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := do(t, newRouter(&fakeStore{}, nil), tc.method, tc.path)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("want 401 without a caller, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

// TestRoutes_StaticSegmentsWin guards the route tree: "unread-count" and
// "read-all" must not be swallowed by the ":id" wildcard.
func TestRoutes_StaticSegmentsWin(t *testing.T) {
	u := uuid.New()
	fs := &fakeStore{rows: []sqlc.NotificationNotification{
		notif(u, sqlc.SharedNotificationTypeMaintenanceDue, false),
	}}
	r := newRouter(fs, &u)

	if w := do(t, r, http.MethodGet, "/api/v1/notifications/unread-count"); w.Code != http.StatusOK {
		t.Fatalf("unread-count must route to its own handler, got %d: %s", w.Code, w.Body.String())
	}
	if w := do(t, r, http.MethodPost, "/api/v1/notifications/read-all"); w.Code != http.StatusNoContent {
		t.Fatalf("read-all must route to its own handler, got %d: %s", w.Code, w.Body.String())
	}
}
