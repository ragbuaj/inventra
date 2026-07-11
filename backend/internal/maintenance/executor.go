package maintenance

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
)

// maintenanceExec creates the corrective 'scheduled' record on final approval of
// a Staf damage report, inside the commit tx. The asset is NOT flipped here —
// it flips when the Manager starts the work (record -> in_progress, FR-4.3).
type maintenanceExec struct{ s *Service }

func (e maintenanceExec) Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error {
	if req.TargetID == nil {
		return approval.ErrInvalidRef
	}
	var p MaintenancePayload
	if err := json.Unmarshal(req.Payload, &p); err != nil {
		return approval.ErrInvalidRef
	}
	a, err := qtx.GetAsset(ctx, *req.TargetID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return approval.ErrInvalidRef
		}
		return err
	}
	if a.Status == sqlc.SharedAssetStatusDisposed || a.Status == sqlc.SharedAssetStatusLost {
		return approval.ErrConflict // asset no longer maintainable at approval time
	}
	problemID, err := uuid.Parse(p.ProblemCategoryID)
	if err != nil {
		return approval.ErrInvalidRef
	}
	desc := "Laporan kerusakan"
	if p.Description != nil && *p.Description != "" {
		desc = *p.Description
	}
	_, err = qtx.CreateMaintRecord(ctx, sqlc.CreateMaintRecordParams{
		AssetID:           *req.TargetID,
		ProblemCategoryID: &problemID,
		Type:              sqlc.SharedMaintenanceTypeCorrective,
		Status:            sqlc.SharedMaintenanceStatusScheduled,
		ScheduledDate:     pgtype.Date{Time: time.Now(), Valid: true},
		Description:       desc,
		ReportedByID:      &req.RequestedByID,
	})
	return err
}

// Executor returns the maintenance approval executor.
func (s *Service) Executor() approval.Executor { return maintenanceExec{s} }
