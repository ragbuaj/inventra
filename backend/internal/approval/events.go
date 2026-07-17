package approval

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// Outbox event types produced by this module. The notification consumer keys off
// these strings; changing one is a wire-contract change.
const (
	EventRequestDecided = "request_decided"
	// EventRequestSubmitted and EventChainAdvanced both mean "step N of this
	// request now awaits a decision" and fan out identically today. They stay
	// two event types rather than one because they are two different business
	// facts, and a later consumer (email) may well treat them differently; the
	// producer must not collapse that distinction on the consumer's behalf.
	EventRequestSubmitted = "request_submitted"
	EventChainAdvanced    = "chain_advanced"
)

// AggregateRequests is the outbox aggregate_type for approval requests.
const AggregateRequests = "requests"

// RequestDecidedEvent is the outbox payload for a terminally decided request
// (rejected, or approved at the final step). It is deliberately self-contained:
// the consumer runs later and must not have to re-read state that may have
// changed by then, so every field it needs to pick the recipient and build the
// i18n params travels with the event.
type RequestDecidedEvent struct {
	RequestID   uuid.UUID                `json:"request_id"`
	RequestType sqlc.SharedRequestType   `json:"request_type"`
	Status      sqlc.SharedRequestStatus `json:"status"`
	// MakerID is the recipient: the user who submitted the request
	// (requests.requested_by_id).
	MakerID     uuid.UUID  `json:"maker_id"`
	DecidedByID *uuid.UUID `json:"decided_by_id,omitempty"`
}

// RequestPendingEvent is the outbox payload for "step Step of request RequestID
// is now awaiting approval" -- emitted on submit (the first step) and on every
// chain advance (the next step).
//
// Unlike RequestDecidedEvent this payload does NOT name its recipients: the
// eligible approvers are a query away (data scope per candidate, office
// ancestors), and resolving them here would stretch the business transaction
// over that work. The consumer resolves them instead, against the state at
// consume time.
type RequestPendingEvent struct {
	RequestID   uuid.UUID              `json:"request_id"`
	RequestType sqlc.SharedRequestType `json:"request_type"`
	// Step is the step this event announces, captured at enqueue time. The
	// consumer compares it against the request's live current_step to tell a
	// current event from one the chain has already moved past.
	Step int32 `json:"step"`
}

// enqueueRequestPending writes a request_submitted or chain_advanced outbox row
// using the caller's transaction-bound queries. Like enqueueRequestDecided it
// must be called with qtx, never s.q: the event has to share the fate of the
// business change. req must be the row as it stands after the change, so
// req.CurrentStep is the step being announced.
func (s *Service) enqueueRequestPending(ctx context.Context, qtx *sqlc.Queries, eventType string, req sqlc.ApprovalRequest) error {
	payload, err := json.Marshal(RequestPendingEvent{
		RequestID:   req.ID,
		RequestType: req.Type,
		Step:        req.CurrentStep,
	})
	if err != nil {
		return err
	}
	_, err = qtx.EnqueueOutbox(ctx, sqlc.EnqueueOutboxParams{
		EventType:     eventType,
		AggregateType: AggregateRequests,
		AggregateID:   req.ID,
		Payload:       payload,
	})
	return mapDBError(err)
}

// enqueueRequestDecided writes the request_decided outbox row using the caller's
// transaction-bound queries. It must be called with qtx, never s.q: the row has
// to land in the same transaction as the business change so a rollback leaves no
// orphan event and a commit can never lose one. A failure here therefore aborts
// the business transaction by design.
func (s *Service) enqueueRequestDecided(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest, decidedBy uuid.UUID) error {
	payload, err := json.Marshal(RequestDecidedEvent{
		RequestID:   req.ID,
		RequestType: req.Type,
		Status:      req.Status,
		MakerID:     req.RequestedByID,
		DecidedByID: &decidedBy,
	})
	if err != nil {
		return err
	}
	_, err = qtx.EnqueueOutbox(ctx, sqlc.EnqueueOutboxParams{
		EventType:     EventRequestDecided,
		AggregateType: AggregateRequests,
		AggregateID:   req.ID,
		Payload:       payload,
	})
	return mapDBError(err)
}
