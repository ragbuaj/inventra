package notification

import (
	"encoding/json"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// notificationToMap serializes a notification row for API responses. Internal
// plumbing columns (user_id -- always the caller, dedup_key, updated_at,
// deleted_at) are omitted.
func notificationToMap(n sqlc.NotificationNotification) map[string]any {
	return map[string]any{
		"id":          n.ID.String(),
		"type":        string(n.Type),
		"params":      rawParams(n.Params),
		"entity_type": n.EntityType,
		"entity_id":   common.UUIDPtrStr(n.EntityID),
		"read_at":     common.TsStr(n.ReadAt),
		"created_at":  common.TsStr(n.CreatedAt),
	}
}

// rawParams passes the jsonb params column through as JSON. Without the
// RawMessage wrapper a []byte marshals to a base64 string, and the frontend
// renders each message from type plus these params. A NULL or malformed column
// degrades to an empty object rather than breaking the response.
func rawParams(b []byte) json.RawMessage {
	if len(b) == 0 || !json.Valid(b) {
		return json.RawMessage("{}")
	}
	return json.RawMessage(b)
}
