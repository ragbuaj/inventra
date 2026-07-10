package stockopname

import (
	"time"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// CreateSessionRequest is the POST /stock-opname/sessions body.
type CreateSessionRequest struct {
	OfficeID string  `json:"office_id" binding:"required,uuid"`
	Name     *string `json:"name"`
	Period   string  `json:"period" binding:"required"` // "2006-01" or "2006-01-02"
}

// SetResultRequest is the PATCH /stock-opname/sessions/:id/items/:itemId body.
type SetResultRequest struct {
	Result string  `json:"result" binding:"required,oneof=found not_found damaged misplaced pending"`
	Note   *string `json:"note"`
}

// ScanRequest is the POST /stock-opname/sessions/:id/scan body.
type ScanRequest struct {
	AssetTag string `json:"asset_tag" binding:"required"`
}

// FollowupRequest is the POST /stock-opname/sessions/:id/items/:itemId/follow-up body.
type FollowupRequest struct {
	ToOfficeID *string `json:"to_office_id" binding:"omitempty,uuid"`
	ToRoomID   *string `json:"to_room_id" binding:"omitempty,uuid"`
	Reason     *string `json:"reason"`
}

// parsePeriod parses a period string in "2006-01" (normalized to the first of
// the month) or "2006-01-02" form.
func parsePeriod(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01", s); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02", s)
}

// toSessionResponse serializes a stock-opname session for API responses,
// enriched with resolved display names and (when available) KPI counters.
func toSessionResponse(s sqlc.StockopnameStockOpnameSession, officeName, startedByName, closedByName *string, kpi *SessionKpis) map[string]any {
	m := map[string]any{
		"id":              s.ID.String(),
		"office_id":       s.OfficeID.String(),
		"name":            s.Name,
		"period":          common.DateStr(s.Period),
		"status":          string(s.Status),
		"started_by_id":   s.StartedByID.String(),
		"started_at":      common.TsStr(s.StartedAt),
		"closed_by_id":    common.UUIDPtrStr(s.ClosedByID),
		"closed_at":       common.TsStr(s.ClosedAt),
		"created_at":      common.TsStr(s.CreatedAt),
		"updated_at":      common.TsStr(s.UpdatedAt),
		"office_name":     officeName,
		"started_by_name": startedByName,
		"closed_by_name":  closedByName,
	}
	if kpi != nil {
		m["total"] = kpi.Total
		m["found"] = kpi.Found
		m["pending"] = kpi.Pending
		m["variance"] = kpi.Variance
	}
	return m
}

// toItemResponse serializes an enriched stock-opname item row for API responses.
func toItemResponse(r sqlc.ListOpnameItemsEnrichedRow) map[string]any {
	it := r.StockopnameStockOpnameItem
	return map[string]any{
		"id":                  it.ID.String(),
		"session_id":          it.SessionID.String(),
		"asset_id":            it.AssetID.String(),
		"asset_name":          r.AssetName,
		"asset_tag":           r.AssetTag,
		"office_name":         r.OfficeName,
		"room_name":           r.RoomName,
		"floor_name":          r.FloorName,
		"expected":            it.Expected,
		"result":              string(it.Result),
		"note":                it.Note,
		"counted_by_name":     r.CountedByName,
		"counted_at":          common.TsStr(it.CountedAt),
		"followup_request_id": common.UUIDPtrStr(it.FollowupRequestID),
		"followup_record_id":  common.UUIDPtrStr(it.FollowupRecordID),
	}
}
