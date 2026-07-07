package assignment

import (
	"encoding/json"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// CheckoutRequest is the POST /assignments body (Manager direct check-out).
type CheckoutRequest struct {
	AssetID      string  `json:"asset_id" binding:"required,uuid"`
	EmployeeID   string  `json:"employee_id" binding:"required,uuid"`
	CheckoutDate string  `json:"checkout_date" binding:"required"` // "2006-01-02"
	DueDate      *string `json:"due_date"`
	ConditionOut *string `json:"condition_out"`
	Notes        *string `json:"notes"`
}

// CheckinRequest is the POST /assignments/:id/checkin body.
type CheckinRequest struct {
	CheckinDate      *string `json:"checkin_date"` // "2006-01-02"; defaults to now
	ConditionIn      *string `json:"condition_in"`
	NeedsMaintenance bool    `json:"needs_maintenance"`
}

// BorrowRequest is the POST /assignments/borrow body (Staf peminjaman).
type BorrowRequest struct {
	AssetID      string  `json:"asset_id" binding:"required,uuid"`
	DueDate      *string `json:"due_date"`
	ConditionOut *string `json:"condition_out"`
	Notes        *string `json:"notes"`
}

// BorrowPayload is the JSON stored in approval.requests.payload for an assignment request.
type BorrowPayload struct {
	AssetID      string  `json:"asset_id"`
	DueDate      *string `json:"due_date"`
	ConditionOut *string `json:"condition_out"`
	Notes        *string `json:"notes"`
}

func marshalBorrowPayload(in BorrowInput) ([]byte, error) {
	return json.Marshal(BorrowPayload{
		AssetID:      in.AssetID.String(),
		DueDate:      in.DueDate,
		ConditionOut: in.ConditionOut,
		Notes:        in.Notes,
	})
}

// toResponse serializes an assignment row (no sensitive columns).
func toResponse(a sqlc.AssignmentAssignment) map[string]any {
	return map[string]any{
		"id":             a.ID.String(),
		"asset_id":       a.AssetID.String(),
		"employee_id":    a.EmployeeID.String(),
		"assigned_by_id": a.AssignedByID.String(),
		"checkout_date":  common.TsStr(a.CheckoutDate),
		"due_date":       common.DateStr(a.DueDate),
		"checkin_date":   common.TsStr(a.CheckinDate),
		"condition_out":  a.ConditionOut,
		"condition_in":   a.ConditionIn,
		"status":         string(a.Status),
		"notes":          a.Notes,
		"created_at":     common.TsStr(a.CreatedAt),
		"updated_at":     common.TsStr(a.UpdatedAt),
	}
}

// enrichAssignmentMap adds resolved display names to a serialized assignment.
// assetName/assetTag are always populated (asset FK is NOT NULL); the rest are
// nullable (employee/assigned-by/office display names).
func enrichAssignmentMap(m map[string]any, assetName, assetTag string, employeeName, assignedByName, officeName *string) map[string]any {
	m["asset_name"] = assetName
	m["asset_tag"] = assetTag
	m["employee_name"] = employeeName
	m["assigned_by_name"] = assignedByName
	m["office_name"] = officeName
	return m
}
