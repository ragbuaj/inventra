package approval

import (
	"encoding/json"
	"errors"
	"math/big"

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
	if r.Type == "asset_create" {
		return r.validateAssetCreateAmount()
	}
	return nil
}

// validateAssetCreateAmount enforces amount == payload.purchase_cost * quantity
// (cost zero when absent; quantity defaults to 1), so a maker cannot understate the
// amount to route an asset_create through a lower approval band than its real total
// purchase cost requires. Batch registration (quantity > 1) is covered by the product.
func (r SubmitRequest) validateAssetCreateAmount() error {
	var p struct {
		PurchaseCost *string `json:"purchase_cost"`
		Quantity     int     `json:"quantity"`
	}
	if len(r.Payload) > 0 {
		if err := json.Unmarshal(r.Payload, &p); err != nil {
			return errors.New("invalid payload")
		}
	}
	amount, ok := parsePlainDecimal(r.Amount)
	if !ok {
		return errors.New("invalid amount")
	}
	cost := new(big.Rat) // zero when purchase_cost is absent
	if p.PurchaseCost != nil {
		if cost, ok = parsePlainDecimal(*p.PurchaseCost); !ok {
			return errors.New("invalid purchase_cost")
		}
	}
	qty := p.Quantity
	if qty == 0 {
		qty = 1
	}
	if qty < 0 {
		return errors.New("invalid quantity")
	}
	expected := new(big.Rat).Mul(cost, big.NewRat(int64(qty), 1))
	if amount.Cmp(expected) != 0 {
		return errors.New("amount must equal payload.purchase_cost * quantity")
	}
	return nil
}

// parsePlainDecimal parses a non-negative plain decimal string ("1000", "1000.50").
// Unlike bare big.Rat parsing it rejects fractions ("1/3"), exponents ("1e5"),
// signs and whitespace — anything Postgres numeric input would not accept here.
func parsePlainDecimal(s string) (*big.Rat, bool) {
	if s == "" {
		return nil, false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && c != '.' {
			return nil, false
		}
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, false
	}
	return r, true
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

var validRequestTypes = map[string]bool{
	"asset_create":        true,
	"asset_disposal":      true,
	"asset_transfer":      true,
	"assignment":          true,
	"maintenance":         true,
	"valuation_exclusion": true,
}

var validRequiredLevels = map[string]bool{
	"office":         true,
	"office_subtree": true,
	"wilayah":        true,
	"pusat":          true,
}

// validate checks enum fields for create (both request_type and required_level).
func (r ThresholdRequest) validate() error {
	if !validRequestTypes[r.RequestType] {
		return errors.New("invalid request_type")
	}
	if !validRequiredLevels[r.RequiredLevel] {
		return errors.New("invalid required_level")
	}
	return nil
}

// validateUpdate checks only required_level (request_type is immutable on update).
func (r ThresholdRequest) validateUpdate() error {
	if !validRequiredLevels[r.RequiredLevel] {
		return errors.New("invalid required_level")
	}
	return nil
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

// enrichRequestMap adds the resolved maker/role/office names to a serialized request.
func enrichRequestMap(m map[string]any, requestedByName, requestedByRole, officeName *string) map[string]any {
	m["requested_by_name"] = requestedByName
	m["requested_by_role"] = requestedByRole
	m["office_name"] = officeName
	return m
}

// stepToMap serializes an enriched approval step for API responses. Internal
// row-keeping columns (id, request_id, timestamps' bookkeeping) are omitted.
func stepToMap(row sqlc.ListRequestApprovalsEnrichedRow) map[string]any {
	a := row.ApprovalRequestApproval
	return map[string]any{
		"step_order":     a.StepOrder,
		"required_level": string(a.RequiredLevel),
		"approver_id":    common.UUIDPtrStr(a.ApproverID),
		"approver_name":  row.ApproverName,
		"decision":       string(a.Decision),
		"note":           a.Note,
		"decided_at":     common.TsStr(a.DecidedAt),
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
