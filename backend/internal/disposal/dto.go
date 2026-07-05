package disposal

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// SubmitRequest is the POST /disposals body. book_value_at_disposal is
// deliberately absent — the server computes it from the depreciation
// schedule (BookValueAsOf), so a caller cannot inject it (see service.go's
// Submit).
type SubmitRequest struct {
	AssetID      string  `json:"asset_id" binding:"required,uuid"`
	Method       string  `json:"method" binding:"required,oneof=sale auction donation write_off"`
	DisposalDate string  `json:"disposal_date" binding:"required"` // "2006-01-02"
	Proceeds     *string `json:"proceeds"`
	BastNo       *string `json:"bast_no"`
	Reason       *string `json:"reason"`
}

// DocumentRequest is the POST /disposals/:id/document body (multipart; file is a separate part).
type DocumentRequest struct {
	BastNo       *string `json:"bast_no" form:"bast_no"`
	DocNo        *string `json:"doc_no" form:"doc_no"`
	DocDate      *string `json:"doc_date" form:"doc_date"`
	Counterparty *string `json:"counterparty" form:"counterparty"`
}

// DisposalPayload is stored in approval.requests.payload for asset_disposal.
type DisposalPayload struct {
	Method       string  `json:"method"`
	DisposalDate string  `json:"disposal_date"`
	Proceeds     *string `json:"proceeds"`
	BookValue    *string `json:"book_value_at_disposal"`
	BastNo       *string `json:"bast_no"`
	Reason       *string `json:"reason"`
}

// parseDate converts an optional "2006-01-02" string to pgtype.Date (invalid → error).
func parseDate(s string) (pgtype.Date, error) {
	if s == "" {
		return pgtype.Date{}, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

// toResponse serializes a disposal row (no sensitive columns).
func toResponse(d sqlc.DisposalDisposal) map[string]any {
	return map[string]any{
		"id":                     d.ID.String(),
		"asset_id":               d.AssetID.String(),
		"method":                 string(d.Method),
		"disposal_date":          common.DateStr(d.DisposalDate),
		"proceeds":               d.Proceeds,
		"book_value_at_disposal": d.BookValueAtDisposal,
		"gain_loss":              d.GainLoss,
		"bast_no":                d.BastNo,
		"approved_by_id":         common.UUIDPtrStr(d.ApprovedByID),
		"request_id":             common.UUIDPtrStr(d.RequestID),
		"created_by_id":          common.UUIDPtrStr(d.CreatedByID),
		"created_at":             common.TsStr(d.CreatedAt),
		"updated_at":             common.TsStr(d.UpdatedAt),
	}
}

// enrichDisposalMap adds resolved asset/office/actor display names to a
// serialized disposal. asset_name/asset_tag come from an INNER-joined,
// non-nullable column (a disposal always has a live asset) so they take plain
// strings; office_name/created_by_name are LEFT-joined and may be nil.
func enrichDisposalMap(m map[string]any, assetName, assetTag string, officeName, createdByName *string) map[string]any {
	m["asset_name"] = assetName
	m["asset_tag"] = assetTag
	m["office_name"] = officeName
	m["created_by_name"] = createdByName
	return m
}

// marshalPayload builds the approval payload JSON for a submit.
func marshalPayload(in SubmitInput) ([]byte, error) {
	return json.Marshal(DisposalPayload{
		Method: in.Method, DisposalDate: in.DisposalDate,
		Proceeds: in.Proceeds, BookValue: in.BookValue, BastNo: in.BastNo, Reason: in.Reason,
	})
}
