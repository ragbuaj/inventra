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
