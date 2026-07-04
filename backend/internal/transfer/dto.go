package transfer

import (
	"encoding/json"

	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// SubmitRequest is the POST /transfers body.
type SubmitRequest struct {
	AssetID       string  `json:"asset_id" binding:"required,uuid"`
	ToOfficeID    string  `json:"to_office_id" binding:"required,uuid"`
	ToRoomID      *string `json:"to_room_id" binding:"omitempty,uuid"`
	Reason        *string `json:"reason"`
	ConditionSent *string `json:"condition_sent" binding:"omitempty,oneof=baik rusak_ringan rusak_berat"`
	TransferDate  *string `json:"transfer_date"` // "2006-01-02"; UI requires it, API keeps it optional (spec deviation (i))
}

// ShipRequest is the POST /transfers/:id/ship body (all optional).
type ShipRequest struct {
	ShippedDate *string `json:"shipped_date"` // "2006-01-02"
}

// ReceiveRequest is the POST /transfers/:id/receive body (multipart or JSON).
// The optional BAST file is read from the multipart form, not this struct.
type ReceiveRequest struct {
	BastNo       *string `json:"bast_no" form:"bast_no"`
	ReceivedDate *string `json:"received_date" form:"received_date"`
	ToRoomID     *string `json:"to_room_id" form:"to_room_id" binding:"omitempty,uuid"`
}

// RejectReceiveRequest is the POST /transfers/:id/reject-receive body.
type RejectReceiveRequest struct {
	Note *string `json:"note"`
}

// TransferPayload is the JSON stored in approval.requests.payload for asset_transfer.
type TransferPayload struct {
	FromOfficeID  string  `json:"from_office_id"`
	ToOfficeID    string  `json:"to_office_id"`
	ToRoomID      *string `json:"to_room_id"`
	Reason        *string `json:"reason"`
	ConditionSent *string `json:"condition_sent"`
	TransferDate  *string `json:"transfer_date"`
}

// toResponse serializes a transfer row for API responses (no sensitive columns).
func toResponse(t sqlc.TransferAssetTransfer) map[string]any {
	return map[string]any{
		"id":              t.ID.String(),
		"asset_id":        t.AssetID.String(),
		"from_office_id":  t.FromOfficeID.String(),
		"to_office_id":    t.ToOfficeID.String(),
		"to_room_id":      common.UUIDPtrStr(t.ToRoomID),
		"status":          string(t.Status),
		"reason":          t.Reason,
		"requested_by_id": t.RequestedByID.String(),
		"approved_by_id":  common.UUIDPtrStr(t.ApprovedByID),
		"shipped_date":    common.DateStr(t.ShippedDate),
		"received_date":   common.DateStr(t.ReceivedDate),
		"received_by_id":  common.UUIDPtrStr(t.ReceivedByID),
		"bast_no":         t.BastNo,
		"request_id":      common.UUIDPtrStr(t.RequestID),
		"created_at":      common.TsStr(t.CreatedAt),
		"updated_at":      common.TsStr(t.UpdatedAt),
		"condition_sent":  condStr(t.ConditionSent),
		"transfer_date":   common.DateStr(t.TransferDate),
		"return_note":     t.ReturnNote,
	}
}

// enrichTransferMap adds resolved asset/office/room/actor display names to a
// serialized transfer. Takes plain *string args (rather than a row type) so it
// works uniformly across List/Get/ListByAsset's distinct sqlc row types.
func enrichTransferMap(m map[string]any, assetName, assetTag, fromOfficeName, toOfficeName, toRoomName, requestedByName, receivedByName *string) map[string]any {
	m["asset_name"] = assetName
	m["asset_tag"] = assetTag
	m["from_office_name"] = fromOfficeName
	m["to_office_name"] = toOfficeName
	m["to_room_name"] = toRoomName
	m["requested_by_name"] = requestedByName
	m["received_by_name"] = receivedByName
	return m
}

// condStr renders the nullable condition enum as *string for JSON.
func condStr(c *sqlc.SharedTransferCondition) *string {
	if c == nil {
		return nil
	}
	s := string(*c)
	return &s
}

// marshalPayload builds the approval payload JSON for a submit.
func marshalPayload(fromOffice, toOffice uuid.UUID, toRoom *uuid.UUID, reason, conditionSent, transferDate *string) ([]byte, error) {
	p := TransferPayload{
		FromOfficeID:  fromOffice.String(),
		ToOfficeID:    toOffice.String(),
		Reason:        reason,
		ConditionSent: conditionSent,
		TransferDate:  transferDate,
	}
	if toRoom != nil {
		s := toRoom.String()
		p.ToRoomID = &s
	}
	return json.Marshal(p)
}
