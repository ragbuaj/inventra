package transfer

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// Outbox event types for the transfer (mutasi) lifecycle. The notification
// consumer keys off these strings; changing one is a wire-contract change.
//
// The approval chain already notifies the origin-office approvers and the maker
// (approval_pending / approval_decided). These four cover the logistics stages
// that the generic engine does not: the origin office is told a transfer is
// ready to ship / was received / was returned, and — the gap these close — the
// destination office is told an asset is on its way to them (in_transit).
const (
	EventTransferApproved  = "transfer_approved"
	EventTransferInTransit = "transfer_in_transit"
	EventTransferReceived  = "transfer_received"
	EventTransferReturned  = "transfer_returned"
)

// AggregateTransfers is the outbox aggregate_type for transfer events.
const AggregateTransfers = "transfers"

// TransferEvent is the self-contained outbox payload shared by every
// transfer-stage event. It carries everything the consumer needs to pick
// recipients (from/to office) and render the message (asset tag/name) without
// re-reading state that may have moved on by consume time.
type TransferEvent struct {
	TransferID   uuid.UUID `json:"transfer_id"`
	AssetID      uuid.UUID `json:"asset_id"`
	AssetTag     string    `json:"asset_tag"`
	AssetName    string    `json:"asset_name"`
	FromOfficeID uuid.UUID `json:"from_office_id"`
	ToOfficeID   uuid.UUID `json:"to_office_id"`
}

// enqueueTransferEvent writes one transfer-stage outbox row using the caller's
// transaction-bound queries. It must be called with qtx, never s.q: the event
// has to share the fate of the state change it announces, so a rollback leaves
// no orphan event and a commit can never lose one.
func (s *Service) enqueueTransferEvent(ctx context.Context, qtx *sqlc.Queries, eventType string, ev TransferEvent) error {
	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	_, err = qtx.EnqueueOutbox(ctx, sqlc.EnqueueOutboxParams{
		EventType:     eventType,
		AggregateType: AggregateTransfers,
		AggregateID:   ev.TransferID,
		Payload:       payload,
	})
	return mapDBError(err)
}

// transferEventFor builds a TransferEvent from a transfer row, reading the asset
// (via the caller's tx) for the tag/name the notification renders. from/to
// office come from the transfer row itself, not the asset, so a Receive that has
// already relocated the asset still reports the correct origin and destination.
func (s *Service) transferEventFor(ctx context.Context, qtx *sqlc.Queries, t sqlc.TransferAssetTransfer) (TransferEvent, error) {
	a, err := qtx.GetAsset(ctx, t.AssetID)
	if err != nil {
		return TransferEvent{}, mapDBError(err)
	}
	return TransferEvent{
		TransferID:   t.ID,
		AssetID:      t.AssetID,
		AssetTag:     a.AssetTag,
		AssetName:    a.Name,
		FromOfficeID: t.FromOfficeID,
		ToOfficeID:   t.ToOfficeID,
	}, nil
}
