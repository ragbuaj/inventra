package approval

import (
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// SubmitRequest is the request body for POST /requests.
type SubmitRequest struct {
	Type     string          `json:"type" binding:"required,oneof=asset_create asset_disposal valuation_exclusion"`
	Amount   string          `json:"amount" binding:"required"`
	OfficeID string          `json:"office_id" binding:"required"`
	TargetID *string         `json:"target_id"`
	Payload  json.RawMessage `json:"payload"`
	Reason   *string         `json:"reason"`
}

func (r SubmitRequest) validate() error {
	if _, err := uuid.Parse(r.OfficeID); err != nil {
		return errors.New("invalid office_id")
	}
	if r.TargetID != nil {
		if _, err := uuid.Parse(*r.TargetID); err != nil {
			return errors.New("invalid target_id")
		}
	}
	return nil
}

// DecideRequest is the request body for POST /requests/:id/approve|reject.
type DecideRequest struct {
	Decision string  `json:"decision" binding:"required,oneof=approve reject"`
	Note     *string `json:"note"`
}

// ThresholdRequest is the request body for POST/PUT /approval-thresholds.
type ThresholdRequest struct {
	RequestType   string  `json:"request_type" binding:"required"`
	AmountFrom    string  `json:"amount_from" binding:"required"`
	AmountTo      *string `json:"amount_to"`
	RequiredLevel string  `json:"required_level" binding:"required"`
	StepOrder     int32   `json:"step_order" binding:"required"`
	IsActive      bool    `json:"is_active"`
}

func (r ThresholdRequest) toCreateParams() sqlc.CreateThresholdParams {
	return sqlc.CreateThresholdParams{
		RequestType:   sqlc.SharedRequestType(r.RequestType),
		AmountFrom:    r.AmountFrom,
		AmountTo:      r.AmountTo,
		RequiredLevel: sqlc.SharedApproverLevel(r.RequiredLevel),
		StepOrder:     r.StepOrder,
		IsActive:      r.IsActive,
	}
}

func (r ThresholdRequest) toUpdateParams(id uuid.UUID) sqlc.UpdateThresholdParams {
	return sqlc.UpdateThresholdParams{
		ID:            id,
		AmountFrom:    r.AmountFrom,
		AmountTo:      r.AmountTo,
		RequiredLevel: sqlc.SharedApproverLevel(r.RequiredLevel),
		StepOrder:     r.StepOrder,
		IsActive:      r.IsActive,
	}
}

// requestToMap serializes an ApprovalRequest for API responses.
func requestToMap(r sqlc.ApprovalRequest) map[string]any {
	return map[string]any{
		"id":              r.ID.String(),
		"type":            string(r.Type),
		"status":          string(r.Status),
		"amount":          r.Amount,
		"current_step":    r.CurrentStep,
		"office_id":       common.UUIDPtrStr(r.OfficeID),
		"target_id":       common.UUIDPtrStr(r.TargetID),
		"target_entity":   r.TargetEntity,
		"reason":          r.Reason,
		"requested_by_id": r.RequestedByID.String(),
		"decided_by_id":   common.UUIDPtrStr(r.DecidedByID),
		"decision_note":   r.DecisionNote,
		"created_at":      common.TsStr(r.CreatedAt),
	}
}

// thresholdToMap serializes an ApprovalApprovalThreshold for API responses.
func thresholdToMap(t sqlc.ApprovalApprovalThreshold) map[string]any {
	return map[string]any{
		"id":             t.ID.String(),
		"request_type":   string(t.RequestType),
		"amount_from":    t.AmountFrom,
		"amount_to":      t.AmountTo,
		"required_level": string(t.RequiredLevel),
		"step_order":     t.StepOrder,
		"is_active":      t.IsActive,
		"created_at":     common.TsStr(t.CreatedAt),
	}
}
