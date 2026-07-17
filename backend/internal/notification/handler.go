package notification

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// Handler is the HTTP handler for the notification module.
type Handler struct {
	svc *Service
}

// NewHandler constructs a Handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// callerID reads the authenticated caller's user id off the context. It is the
// only authorization input the feed has, so an absent or unparseable subject is
// refused rather than defaulted.
func (h *Handler) callerID(c *gin.Context) (uuid.UUID, bool) {
	uid, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid subject"})
		return uuid.Nil, false
	}
	return uid, true
}

// svcError maps notification sentinel errors to HTTP status codes.
func (h *Handler) svcError(c *gin.Context, err error) {
	switch err {
	case ErrNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	default:
		common.WriteError(c, err)
	}
}

// readFilter parses the optional "read" query filter. Any value other than
// "true"/"false" is treated as absent, so a typo widens to the full feed rather
// than silently filtering.
func readFilter(raw string) ReadFilter {
	switch raw {
	case "true":
		return ReadFilterRead
	case "false":
		return ReadFilterUnread
	default:
		return ReadFilterAll
	}
}

// list handles GET /notifications.
func (h *Handler) list(c *gin.Context) {
	uid, ok := h.callerID(c)
	if !ok {
		return
	}
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<30)
	rows, total, err := h.svc.List(c, ListInput{
		UserID: uid,
		Read:   readFilter(c.Query("read")),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(rows))
	for _, n := range rows {
		data = append(data, notificationToMap(n))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

// unreadCount handles GET /notifications/unread-count -- the bell badge count.
func (h *Handler) unreadCount(c *gin.Context) {
	uid, ok := h.callerID(c)
	if !ok {
		return
	}
	n, err := h.svc.UnreadCount(c, uid)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": n})
}

// markRead handles POST /notifications/:id/read.
func (h *Handler) markRead(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	uid, ok := h.callerID(c)
	if !ok {
		return
	}
	out, err := h.svc.MarkRead(c, id, uid)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, notificationToMap(out))
}

// markAllRead handles POST /notifications/read-all.
func (h *Handler) markAllRead(c *gin.Context) {
	uid, ok := h.callerID(c)
	if !ok {
		return
	}
	if err := h.svc.MarkAllRead(c, uid); err != nil {
		h.svcError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
