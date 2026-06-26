package audit

import (
	"encoding/json"
	"time"

	"github.com/ragbuaj/inventra/db/sqlc"
)

// listResponse is the standard paginated envelope.
type listResponse struct {
	Data   []map[string]any `json:"data"`
	Total  int64            `json:"total"`
	Limit  int32            `json:"limit"`
	Offset int32            `json:"offset"`
}

// auditToMap serializes one audit row (with the joined actor) for the API.
func auditToMap(r sqlc.ListAuditLogsRow) map[string]any {
	m := map[string]any{
		"id":          r.ID.String(),
		"entity_type": r.EntityType,
		"entity_id":   r.EntityID.String(),
		"action":      string(r.Action),
		"ip":          r.Ip,
		"changes":     rawJSON(r.Changes),
	}
	if r.ActorID != nil {
		m["actor"] = map[string]any{
			"id":    r.ActorID.String(),
			"name":  r.ActorName,
			"email": r.ActorEmail,
		}
	} else {
		m["actor"] = nil
	}
	if r.OfficeID != nil {
		m["office_id"] = r.OfficeID.String()
	} else {
		m["office_id"] = nil
	}
	if r.CreatedAt.Valid {
		m["created_at"] = r.CreatedAt.Time.Format(time.RFC3339)
	} else {
		m["created_at"] = nil
	}
	return m
}

// rawJSON returns the stored changes blob as a decoded value (or nil).
func rawJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil
	}
	return v
}
