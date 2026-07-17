package assignment

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// Outbox event types produced by this module. The notification consumer keys off
// these strings; changing one is a wire-contract change.
const (
	EventAssignmentCheckin = "assignment_checkin"
)

// AggregateAssignments is the outbox aggregate_type for assignments.
const AggregateAssignments = "assignments"

// AssignmentCheckinEvent is the outbox payload for an asset checked back in. It
// is deliberately self-contained: the consumer runs later and must not have to
// re-read state that may have changed by then, so the recipient and every i18n
// param travel with the event.
//
// The recipient is AssignedByID, the user who checked the asset out. It is
// already a concrete user id (checkoutTx stores the acting user). The custodian
// is deliberately NOT notified: assignments.employee_id points at
// masterdata.employees, which are not login users, and there is no mapping from
// an employee to a user. That gap is avoided here, not closed (spec section 4).
type AssignmentCheckinEvent struct {
	AssignmentID uuid.UUID `json:"assignment_id"`
	AssetID      uuid.UUID `json:"asset_id"`
	AssetTag     string    `json:"asset_tag"`
	AssetName    string    `json:"asset_name"`
	// AssignedByID is the recipient: the user who checked the asset out.
	AssignedByID uuid.UUID `json:"assigned_by_id"`
	// CheckedInByID is the acting user. Carried for traceability and for future
	// consumers (an email channel) that may want it; self-notification is
	// already suppressed before the event is written (see enqueueCheckin).
	CheckedInByID uuid.UUID `json:"checked_in_by_id"`
}

// enqueueCheckin writes the assignment_checkin outbox row using the caller's
// transaction-bound queries. It must be called with qtx, never s.q: the row has
// to land in the same transaction as the business change so a rollback leaves no
// orphan event and a commit can never lose one. A failure here therefore aborts
// the business transaction by design. This differs from audit.Record, which is
// post-commit and best-effort, and that difference is deliberate: an audit entry
// may be lost without corrupting anything, an event may not.
//
// Self-notification is suppressed HERE rather than in the consumer. The event
// exists solely to tell someone else their asset came back; when the person
// checking in is the same person who checked out there is no recipient at all,
// so there is nothing to publish. Suppressing at the producer also means every
// consumer on the stream (a future email channel included) inherits the rule for
// free, instead of each having to re-derive it, and it keeps the outbox and the
// stream free of messages whose only possible outcome is to be dropped. The
// acting user is known right here inside the transaction, so no extra state
// needs to travel for the check.
func (s *Service) enqueueCheckin(ctx context.Context, qtx *sqlc.Queries, a sqlc.AssignmentAssignment, asset sqlc.AssetAsset, checkedInBy uuid.UUID) error {
	if a.AssignedByID == checkedInBy {
		return nil
	}
	payload, err := json.Marshal(AssignmentCheckinEvent{
		AssignmentID:  a.ID,
		AssetID:       asset.ID,
		AssetTag:      asset.AssetTag,
		AssetName:     asset.Name,
		AssignedByID:  a.AssignedByID,
		CheckedInByID: checkedInBy,
	})
	if err != nil {
		return err
	}
	_, err = qtx.EnqueueOutbox(ctx, sqlc.EnqueueOutboxParams{
		EventType:     EventAssignmentCheckin,
		AggregateType: AggregateAssignments,
		AggregateID:   a.ID,
		Payload:       payload,
	})
	return mapDBError(err)
}
