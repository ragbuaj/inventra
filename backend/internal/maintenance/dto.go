package maintenance

import (
	"encoding/json"

	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// CreateScheduleRequest is the POST /maintenance/schedules body.
type CreateScheduleRequest struct {
	AssetID               string  `json:"asset_id" binding:"required,uuid"`
	MaintenanceCategoryID *string `json:"maintenance_category_id" binding:"omitempty,uuid"`
	IntervalMonths        int32   `json:"interval_months" binding:"required,min=1"`
	StartDate             string  `json:"start_date" binding:"required"` // "2006-01-02" -> first next_due_date
}

// UpdateScheduleRequest is the PATCH /maintenance/schedules/:id body.
type UpdateScheduleRequest struct {
	MaintenanceCategoryID *string `json:"maintenance_category_id" binding:"omitempty,uuid"`
	IntervalMonths        *int32  `json:"interval_months" binding:"omitempty,min=1"`
	IsActive              *bool   `json:"is_active"`
}

// CreateRecordRequest is the POST /maintenance/records body (Tambah Catatan slideover).
type CreateRecordRequest struct {
	AssetID               string  `json:"asset_id" binding:"required,uuid"`
	ScheduleID            *string `json:"schedule_id" binding:"omitempty,uuid"`
	MaintenanceCategoryID *string `json:"maintenance_category_id" binding:"omitempty,uuid"`
	ProblemCategoryID     *string `json:"problem_category_id" binding:"omitempty,uuid"`
	Type                  string  `json:"type" binding:"required,oneof=preventive corrective"`
	Status                string  `json:"status" binding:"omitempty,oneof=scheduled in_progress completed cancelled"`
	ScheduledDate         *string `json:"scheduled_date"` // "2006-01-02"
	CompletedDate         *string `json:"completed_date"`
	Cost                  *string `json:"cost"`
	VendorID              *string `json:"vendor_id" binding:"omitempty,uuid"`
	Description           string  `json:"description" binding:"required"`
}

// UpdateRecordRequest is the PATCH /maintenance/records/:id body (edit slideover).
type UpdateRecordRequest struct {
	Status                *string `json:"status" binding:"omitempty,oneof=scheduled in_progress completed cancelled"`
	MaintenanceCategoryID *string `json:"maintenance_category_id" binding:"omitempty,uuid"`
	ScheduledDate         *string `json:"scheduled_date"`
	CompletedDate         *string `json:"completed_date"`
	Cost                  *string `json:"cost"`
	VendorID              *string `json:"vendor_id" binding:"omitempty,uuid"`
	Description           *string `json:"description"`
}

// ReportForm is the POST /maintenance/reports multipart form (Staf damage report).
// The optional photo file arrives as form file "photo" (read in the handler).
type ReportForm struct {
	AssetID           string  `form:"asset_id" binding:"required,uuid"`
	ProblemCategoryID string  `form:"problem_category_id" binding:"required,uuid"`
	Description       *string `form:"description"`
}

// MaintenancePayload is the JSON stored in approval.requests.payload.
type MaintenancePayload struct {
	AssetID           string  `json:"asset_id"`
	ProblemCategoryID string  `json:"problem_category_id"`
	Description       *string `json:"description"`
	AttachmentID      *string `json:"attachment_id"`
}

func marshalReportPayload(assetID, problemID string, desc, attachmentID *string) ([]byte, error) {
	return json.Marshal(MaintenancePayload{AssetID: assetID, ProblemCategoryID: problemID, Description: desc, AttachmentID: attachmentID})
}

// toScheduleResponse serializes a schedule row.
func toScheduleResponse(s sqlc.MaintenanceMaintenanceSchedule) map[string]any {
	return map[string]any{
		"id":                      s.ID.String(),
		"asset_id":                s.AssetID.String(),
		"maintenance_category_id": uuidPtrStr(s.MaintenanceCategoryID),
		"interval_months":         s.IntervalMonths,
		"last_done_date":          common.DateStr(s.LastDoneDate),
		"next_due_date":           common.DateStr(s.NextDueDate),
		"is_active":               s.IsActive,
		"created_at":              common.TsStr(s.CreatedAt),
		"updated_at":              common.TsStr(s.UpdatedAt),
	}
}

// toRecordResponse serializes a record row.
func toRecordResponse(r sqlc.MaintenanceMaintenanceRecord) map[string]any {
	return map[string]any{
		"id":                      r.ID.String(),
		"asset_id":                r.AssetID.String(),
		"schedule_id":             uuidPtrStr(r.ScheduleID),
		"maintenance_category_id": uuidPtrStr(r.MaintenanceCategoryID),
		"problem_category_id":     uuidPtrStr(r.ProblemCategoryID),
		"type":                    string(r.Type),
		"status":                  string(r.Status),
		"scheduled_date":          common.DateStr(r.ScheduledDate),
		"completed_date":          common.DateStr(r.CompletedDate),
		"cost":                    r.Cost,
		"vendor_id":               uuidPtrStr(r.VendorID),
		"performed_by":            r.PerformedBy,
		"description":             r.Description,
		"reported_by_id":          uuidPtrStr(r.ReportedByID),
		"created_at":              common.TsStr(r.CreatedAt),
		"updated_at":              common.TsStr(r.UpdatedAt),
	}
}

func uuidPtrStr(u *uuid.UUID) any {
	if u == nil {
		return nil
	}
	return u.String()
}

// enrichScheduleMap adds resolved display names to a serialized schedule.
func enrichScheduleMap(m map[string]any, assetName, assetTag string, officeName, categoryName *string) map[string]any {
	m["asset_name"] = assetName
	m["asset_tag"] = assetTag
	m["office_name"] = officeName
	m["category_name"] = categoryName
	return m
}

// enrichRecordMap adds resolved display names to a serialized record.
func enrichRecordMap(m map[string]any, assetName, assetTag string, officeName, categoryName, problemName, vendorName, reportedByName *string) map[string]any {
	m["asset_name"] = assetName
	m["asset_tag"] = assetTag
	m["office_name"] = officeName
	m["category_name"] = categoryName
	m["problem_name"] = problemName
	m["vendor_name"] = vendorName
	m["reported_by_name"] = reportedByName
	return m
}
