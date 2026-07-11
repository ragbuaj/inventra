// Package maintenance implements preventive maintenance schedules and the
// maintenance record state machine (FR-4.x), plus the Staf damage-report path
// via the generic approval engine. Split into dto / service (executor/handler/
// routes land in a later task, ADR-0008); the service holds business rules +
// data-scope enforcement (Gin-free), scoped by the asset's office.
package maintenance

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/asset"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

var (
	ErrNotFound             = errors.New("maintenance: not found")
	ErrOutOfScope           = errors.New("maintenance: office out of scope")
	ErrInvalidRef           = errors.New("maintenance: invalid reference")
	ErrAssetNotMaintainable = errors.New("maintenance: asset is disposed or lost")
	ErrAssetBusy            = errors.New("maintenance: asset is in transfer")
	ErrInvalidTransition    = errors.New("maintenance: invalid status transition")
	ErrTerminal             = errors.New("maintenance: record is completed/cancelled")
	ErrScheduleMismatch     = errors.New("maintenance: schedule belongs to another asset")
	ErrDuplicatePending     = errors.New("maintenance: a pending report already exists for this asset")
	ErrInvalidInterval      = errors.New("maintenance: interval must be >= 1 month")
)

func mapDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503":
			return ErrInvalidRef
		}
	}
	return err
}

// Service implements the maintenance module's business rules.
type Service struct {
	q      *sqlc.Queries
	pool   *pgxpool.Pool
	appr   *approval.Service
	assets *asset.Service
}

func NewService(q *sqlc.Queries, pool *pgxpool.Pool, appr *approval.Service, assets *asset.Service) *Service {
	return &Service{q: q, pool: pool, appr: appr, assets: assets}
}

// ScheduleInput is the service-level payload for creating a schedule.
type ScheduleInput struct {
	AssetID               uuid.UUID
	MaintenanceCategoryID *uuid.UUID
	IntervalMonths        int32
	StartDate             string // "2006-01-02" -> first next_due_date
}

// ScheduleUpdateInput is the service-level payload for patching a schedule.
type ScheduleUpdateInput struct {
	MaintenanceCategoryID *uuid.UUID
	IntervalMonths        *int32
	IsActive              *bool
}

// RecordInput is the service-level payload for creating a maintenance record.
type RecordInput struct {
	AssetID               uuid.UUID
	ScheduleID            *uuid.UUID
	MaintenanceCategoryID *uuid.UUID
	ProblemCategoryID     *uuid.UUID
	Type                  sqlc.SharedMaintenanceType
	Status                sqlc.SharedMaintenanceStatus // "" -> scheduled
	ScheduledDate         *string
	CompletedDate         *string
	Cost                  *string
	VendorID              *uuid.UUID
	PerformedBy           *string
	Description           string
	ReportedByID          *uuid.UUID
}

// RecordUpdateInput is the service-level payload for patching a maintenance record.
type RecordUpdateInput struct {
	Status                *sqlc.SharedMaintenanceStatus
	MaintenanceCategoryID *uuid.UUID
	ScheduledDate         *string
	CompletedDate         *string
	Cost                  *string
	VendorID              *uuid.UUID
	Description           *string
}

// ReportInput is the service-level payload for a Staf damage report.
type ReportInput struct {
	AssetID           uuid.UUID
	ProblemCategoryID uuid.UUID
	Description       *string
	Photo             *PhotoInput // nil when no file uploaded
}

// PhotoInput carries an already-read multipart file for a damage report.
type PhotoInput struct {
	Filename    string
	ContentType string
	Data        []byte
}

// toDate parses an optional "2006-01-02" string into a pgtype.Date. A nil or
// empty pointer yields SQL NULL (Valid: false), which the UPDATE queries'
// COALESCE treats as "leave the column unchanged".
func toDate(s *string) (pgtype.Date, error) {
	if s == nil || *s == "" {
		return pgtype.Date{}, nil
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

// validTransition reports whether a record may move from -> to.
// scheduled -> scheduled|in_progress|completed|cancelled
// in_progress -> in_progress|completed|cancelled
// completed / cancelled are terminal.
func validTransition(from, to sqlc.SharedMaintenanceStatus) bool {
	if from == to {
		return from != sqlc.SharedMaintenanceStatusCompleted && from != sqlc.SharedMaintenanceStatusCancelled
	}
	switch from {
	case sqlc.SharedMaintenanceStatusScheduled:
		return true
	case sqlc.SharedMaintenanceStatusInProgress:
		return to == sqlc.SharedMaintenanceStatusCompleted || to == sqlc.SharedMaintenanceStatusCancelled
	default:
		return false
	}
}

// CreateSchedule creates a preventive maintenance schedule for an asset within
// the caller's data scope.
func (s *Service) CreateSchedule(ctx context.Context, all bool, ids []uuid.UUID, in ScheduleInput) (sqlc.MaintenanceMaintenanceSchedule, error) {
	if in.IntervalMonths < 1 {
		return sqlc.MaintenanceMaintenanceSchedule{}, ErrInvalidInterval
	}
	a, err := s.q.GetAsset(ctx, in.AssetID)
	if err != nil {
		return sqlc.MaintenanceMaintenanceSchedule{}, mapDBError(err)
	}
	if !common.InScope(all, ids, a.OfficeID) {
		return sqlc.MaintenanceMaintenanceSchedule{}, ErrOutOfScope
	}
	if a.Status == sqlc.SharedAssetStatusDisposed || a.Status == sqlc.SharedAssetStatusLost {
		return sqlc.MaintenanceMaintenanceSchedule{}, ErrAssetNotMaintainable
	}
	start, err := time.Parse("2006-01-02", in.StartDate)
	if err != nil {
		return sqlc.MaintenanceMaintenanceSchedule{}, ErrInvalidRef
	}
	sch, err := s.q.CreateMaintSchedule(ctx, sqlc.CreateMaintScheduleParams{
		AssetID:               in.AssetID,
		MaintenanceCategoryID: in.MaintenanceCategoryID,
		IntervalMonths:        in.IntervalMonths,
		NextDueDate:           pgtype.Date{Time: start, Valid: true},
	})
	return sch, mapDBError(err)
}

// UpdateSchedule patches a schedule already loaded (and scope-checked) via
// GetMaintScheduleScoped. When the interval changes and the schedule has a
// last_done_date, next_due_date is recomputed from it; otherwise it is left
// untouched (COALESCE keeps the stored value).
func (s *Service) UpdateSchedule(ctx context.Context, all bool, ids []uuid.UUID, id uuid.UUID, in ScheduleUpdateInput) (sqlc.MaintenanceMaintenanceSchedule, error) {
	if in.IntervalMonths != nil && *in.IntervalMonths < 1 {
		return sqlc.MaintenanceMaintenanceSchedule{}, ErrInvalidInterval
	}
	cur, err := s.q.GetMaintScheduleScoped(ctx, sqlc.GetMaintScheduleScopedParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return sqlc.MaintenanceMaintenanceSchedule{}, mapDBError(err)
	}
	var nextDue pgtype.Date
	if in.IntervalMonths != nil && cur.LastDoneDate.Valid {
		nextDue = pgtype.Date{Time: cur.LastDoneDate.Time.AddDate(0, int(*in.IntervalMonths), 0), Valid: true}
	}
	updated, err := s.q.UpdateMaintSchedule(ctx, sqlc.UpdateMaintScheduleParams{
		MaintenanceCategoryID: in.MaintenanceCategoryID,
		IntervalMonths:        in.IntervalMonths,
		IsActive:              in.IsActive,
		NextDueDate:           nextDue,
		ID:                    id,
	})
	return updated, mapDBError(err)
}

// DeleteSchedule scope-checks then soft-deletes a schedule.
func (s *Service) DeleteSchedule(ctx context.Context, all bool, ids []uuid.UUID, id uuid.UUID) error {
	if _, err := s.q.GetMaintScheduleScoped(ctx, sqlc.GetMaintScheduleScopedParams{ID: id, AllScope: all, OfficeIds: ids}); err != nil {
		return mapDBError(err)
	}
	n, err := s.q.SoftDeleteMaintSchedule(ctx, id)
	if err != nil {
		return mapDBError(err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListSchedules returns a scoped, paginated, enriched page + total.
func (s *Service) ListSchedules(ctx context.Context, all bool, ids []uuid.UUID, isActive *bool, limit, offset int32) ([]sqlc.ListMaintSchedulesEnrichedRow, int64, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	rows, err := s.q.ListMaintSchedulesEnriched(ctx, sqlc.ListMaintSchedulesEnrichedParams{
		AllScope: all, OfficeIds: ids, IsActive: isActive, Off: offset, Lim: limit,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	total, err := s.q.CountMaintSchedules(ctx, sqlc.CountMaintSchedulesParams{AllScope: all, OfficeIds: ids, IsActive: isActive})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	return rows, total, nil
}

// completedOrToday reads the record's completed_date, assuming the service
// layer has already defaulted it to today when the target status is
// completed and no date was supplied (applyStatusEffects never writes the
// record row itself).
func completedOrToday(rec sqlc.MaintenanceMaintenanceRecord) time.Time {
	if rec.CompletedDate.Valid {
		return rec.CompletedDate.Time
	}
	return time.Now()
}

// applyStatusEffects flips the asset + touches the linked schedule after a
// record lands in status rec.Status. prev is the pre-update status ("" on
// create) — effects only depend on the resulting rec.Status and current asset
// state, so prev is documentation for the caller rather than branched on here.
func (s *Service) applyStatusEffects(ctx context.Context, qtx *sqlc.Queries, rec sqlc.MaintenanceMaintenanceRecord, a sqlc.AssetAsset, prev sqlc.SharedMaintenanceStatus) error {
	switch rec.Status {
	case sqlc.SharedMaintenanceStatusInProgress:
		switch a.Status {
		case sqlc.SharedAssetStatusAvailable, sqlc.SharedAssetStatusAssigned:
			if _, err := qtx.SetAssetStatus(ctx, sqlc.SetAssetStatusParams{ID: a.ID, Status: sqlc.SharedAssetStatusUnderMaintenance}); err != nil {
				return mapDBError(err)
			}
		case sqlc.SharedAssetStatusUnderMaintenance:
			// already flagged — no-op
		default: // in_transfer etc.
			return ErrAssetBusy
		}
	case sqlc.SharedMaintenanceStatusCompleted, sqlc.SharedMaintenanceStatusCancelled:
		// Completed: touch the linked schedule (last_done_date/next_due_date).
		if rec.Status == sqlc.SharedMaintenanceStatusCompleted && rec.ScheduleID != nil {
			sched, err := qtx.GetMaintScheduleScoped(ctx, sqlc.GetMaintScheduleScopedParams{ID: *rec.ScheduleID, AllScope: true, OfficeIds: []uuid.UUID{}})
			if err != nil {
				return mapDBError(err)
			}
			done := completedOrToday(rec)
			next := done.AddDate(0, int(sched.IntervalMonths), 0)
			if _, err := qtx.TouchMaintScheduleDone(ctx, sqlc.TouchMaintScheduleDoneParams{
				ID:           sched.ID,
				LastDoneDate: pgtype.Date{Time: done, Valid: true},
				NextDueDate:  pgtype.Date{Time: next, Valid: true},
			}); err != nil {
				return mapDBError(err)
			}
		}
		// Release the asset only if it is under_maintenance and this was its last
		// active record.
		if a.Status == sqlc.SharedAssetStatusUnderMaintenance {
			n, err := qtx.CountActiveMaintRecordsByAsset(ctx, sqlc.CountActiveMaintRecordsByAssetParams{AssetID: a.ID, ExcludeID: &rec.ID})
			if err != nil {
				return mapDBError(err)
			}
			if n == 0 {
				// Cross-module rule (assignment module): the asset may still be
				// checked out to an employee while under maintenance (e.g. a
				// laptop reported broken while assigned). Releasing it must not
				// blindly set 'available' — that would let it show up in
				// /assignments/available while still held, and the next
				// borrow-approval would violate the one-active-assignment-per-
				// asset unique index. Restore 'assigned' when an active
				// assignment still exists; otherwise release to 'available'.
				releaseStatus := sqlc.SharedAssetStatusAvailable
				if _, err := qtx.GetActiveAssignmentByAsset(ctx, a.ID); err == nil {
					releaseStatus = sqlc.SharedAssetStatusAssigned
				} else if !errors.Is(err, pgx.ErrNoRows) {
					return mapDBError(err)
				}
				if _, err := qtx.SetAssetStatus(ctx, sqlc.SetAssetStatusParams{ID: a.ID, Status: releaseStatus}); err != nil {
					return mapDBError(err)
				}
			}
		}
	}
	return nil
}

// CreateRecord creates a maintenance record (preventive or corrective) within
// the caller's data scope, applying create-time status effects atomically.
func (s *Service) CreateRecord(ctx context.Context, all bool, ids []uuid.UUID, createdBy uuid.UUID, in RecordInput) (sqlc.MaintenanceMaintenanceRecord, error) {
	a, err := s.q.GetAsset(ctx, in.AssetID)
	if err != nil {
		return sqlc.MaintenanceMaintenanceRecord{}, mapDBError(err)
	}
	if !common.InScope(all, ids, a.OfficeID) {
		return sqlc.MaintenanceMaintenanceRecord{}, ErrOutOfScope
	}
	if a.Status == sqlc.SharedAssetStatusDisposed || a.Status == sqlc.SharedAssetStatusLost {
		return sqlc.MaintenanceMaintenanceRecord{}, ErrAssetNotMaintainable
	}

	status := in.Status
	if status == "" {
		status = sqlc.SharedMaintenanceStatusScheduled
	}
	reportedBy := in.ReportedByID
	if reportedBy == nil {
		reportedBy = &createdBy
	}

	if in.ScheduleID != nil {
		sched, err := s.q.GetMaintScheduleScoped(ctx, sqlc.GetMaintScheduleScopedParams{ID: *in.ScheduleID, AllScope: all, OfficeIds: ids})
		if err != nil {
			return sqlc.MaintenanceMaintenanceRecord{}, mapDBError(err)
		}
		if sched.AssetID != in.AssetID {
			return sqlc.MaintenanceMaintenanceRecord{}, ErrScheduleMismatch
		}
	}

	scheduledDate, err := toDate(in.ScheduledDate)
	if err != nil {
		return sqlc.MaintenanceMaintenanceRecord{}, ErrInvalidRef
	}
	completedDate, err := toDate(in.CompletedDate)
	if err != nil {
		return sqlc.MaintenanceMaintenanceRecord{}, ErrInvalidRef
	}
	// completed_date defaulting happens here, before insert — applyStatusEffects
	// never writes the record row itself.
	if status == sqlc.SharedMaintenanceStatusCompleted && !completedDate.Valid {
		completedDate = pgtype.Date{Time: time.Now(), Valid: true}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlc.MaintenanceMaintenanceRecord{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	rec, err := qtx.CreateMaintRecord(ctx, sqlc.CreateMaintRecordParams{
		AssetID:               in.AssetID,
		ScheduleID:            in.ScheduleID,
		MaintenanceCategoryID: in.MaintenanceCategoryID,
		ProblemCategoryID:     in.ProblemCategoryID,
		Type:                  in.Type,
		Status:                status,
		ScheduledDate:         scheduledDate,
		CompletedDate:         completedDate,
		Cost:                  in.Cost,
		VendorID:              in.VendorID,
		PerformedBy:           in.PerformedBy,
		Description:           in.Description,
		ReportedByID:          reportedBy,
	})
	if err != nil {
		return sqlc.MaintenanceMaintenanceRecord{}, mapDBError(err)
	}
	if err := s.applyStatusEffects(ctx, qtx, rec, a, ""); err != nil {
		return sqlc.MaintenanceMaintenanceRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return sqlc.MaintenanceMaintenanceRecord{}, err
	}
	return rec, nil
}

// UpdateRecord patches a record already loaded (and scope-checked) via
// GetMaintRecordScoped, validates the status transition, and applies the
// resulting status effects atomically.
func (s *Service) UpdateRecord(ctx context.Context, all bool, ids []uuid.UUID, id uuid.UUID, in RecordUpdateInput) (sqlc.MaintenanceMaintenanceRecord, error) {
	cur, err := s.q.GetMaintRecordScoped(ctx, sqlc.GetMaintRecordScopedParams{ID: id, AllScope: all, OfficeIds: ids})
	if err != nil {
		return sqlc.MaintenanceMaintenanceRecord{}, mapDBError(err)
	}
	if cur.Status == sqlc.SharedMaintenanceStatusCompleted || cur.Status == sqlc.SharedMaintenanceStatusCancelled {
		return sqlc.MaintenanceMaintenanceRecord{}, ErrTerminal
	}
	if in.Status != nil && !validTransition(cur.Status, *in.Status) {
		return sqlc.MaintenanceMaintenanceRecord{}, ErrInvalidTransition
	}

	a, err := s.q.GetAsset(ctx, cur.AssetID)
	if err != nil {
		return sqlc.MaintenanceMaintenanceRecord{}, mapDBError(err)
	}

	scheduledDate, err := toDate(in.ScheduledDate)
	if err != nil {
		return sqlc.MaintenanceMaintenanceRecord{}, ErrInvalidRef
	}
	completedDate, err := toDate(in.CompletedDate)
	if err != nil {
		return sqlc.MaintenanceMaintenanceRecord{}, ErrInvalidRef
	}
	targetStatus := cur.Status
	if in.Status != nil {
		targetStatus = *in.Status
	}
	// completed_date defaulting happens here, before update, only when the
	// record is landing on completed for the first time (no date supplied and
	// none already stored) — applyStatusEffects never writes the record row.
	if targetStatus == sqlc.SharedMaintenanceStatusCompleted && !completedDate.Valid && !cur.CompletedDate.Valid {
		completedDate = pgtype.Date{Time: time.Now(), Valid: true}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return sqlc.MaintenanceMaintenanceRecord{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	qtx := s.q.WithTx(tx)

	updated, err := qtx.UpdateMaintRecord(ctx, sqlc.UpdateMaintRecordParams{
		Status:                in.Status,
		MaintenanceCategoryID: in.MaintenanceCategoryID,
		ScheduledDate:         scheduledDate,
		CompletedDate:         completedDate,
		Cost:                  in.Cost,
		VendorID:              in.VendorID,
		Description:           in.Description,
		ID:                    id,
	})
	if err != nil {
		return sqlc.MaintenanceMaintenanceRecord{}, mapDBError(err)
	}
	if err := s.applyStatusEffects(ctx, qtx, updated, a, cur.Status); err != nil {
		return sqlc.MaintenanceMaintenanceRecord{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return sqlc.MaintenanceMaintenanceRecord{}, err
	}
	return updated, nil
}

// GetRecord returns one scoped, enriched record.
func (s *Service) GetRecord(ctx context.Context, id uuid.UUID, all bool, ids []uuid.UUID) (sqlc.GetMaintRecordEnrichedRow, error) {
	r, err := s.q.GetMaintRecordEnriched(ctx, sqlc.GetMaintRecordEnrichedParams{ID: id, AllScope: all, OfficeIds: ids})
	return r, mapDBError(err)
}

// ListRecords returns a scoped, paginated, enriched page + total.
func (s *Service) ListRecords(ctx context.Context, all bool, ids []uuid.UUID, status, mtype, search string, limit, offset int32) ([]sqlc.ListMaintRecordsEnrichedRow, int64, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	var st *sqlc.SharedMaintenanceStatus
	if status != "" {
		v := sqlc.SharedMaintenanceStatus(status)
		st = &v
	}
	var mt *sqlc.SharedMaintenanceType
	if mtype != "" {
		v := sqlc.SharedMaintenanceType(mtype)
		mt = &v
	}
	var sr *string
	if search != "" {
		sr = &search
	}
	rows, err := s.q.ListMaintRecordsEnriched(ctx, sqlc.ListMaintRecordsEnrichedParams{
		AllScope: all, OfficeIds: ids, Status: st, Mtype: mt, Search: sr, Off: offset, Lim: limit,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	total, err := s.q.CountMaintRecords(ctx, sqlc.CountMaintRecordsParams{AllScope: all, OfficeIds: ids, Status: st, Mtype: mt, Search: sr})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	return rows, total, nil
}

// ListByAsset returns a scoped, enriched maintenance history for one asset.
func (s *Service) ListByAsset(ctx context.Context, assetID uuid.UUID, all bool, ids []uuid.UUID) ([]sqlc.ListMaintRecordsByAssetEnrichedRow, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	rows, err := s.q.ListMaintRecordsByAssetEnriched(ctx, sqlc.ListMaintRecordsByAssetEnrichedParams{AssetID: assetID, AllScope: all, OfficeIds: ids})
	return rows, mapDBError(err)
}

// Attention returns under_maintenance assets with no active maintenance
// record — the "Perlu Tindak Lanjut" queue.
func (s *Service) Attention(ctx context.Context, all bool, ids []uuid.UUID) ([]sqlc.ListMaintAttentionAssetsRow, error) {
	if ids == nil {
		ids = []uuid.UUID{}
	}
	rows, err := s.q.ListMaintAttentionAssets(ctx, sqlc.ListMaintAttentionAssetsParams{AllScope: all, OfficeIds: ids})
	return rows, mapDBError(err)
}

// SubmitReport opens a maintenance-type approval request for a Staf damage
// report. The asset must be in the caller's scope and maintainable; an
// optional photo is uploaded as an asset attachment first and referenced from
// the payload. Duplicate-guarded: one pending report per (asset, maker).
func (s *Service) SubmitReport(ctx context.Context, caller approval.Caller, in ReportInput) (sqlc.ApprovalRequest, error) {
	a, err := s.q.GetAsset(ctx, in.AssetID)
	if err != nil {
		return sqlc.ApprovalRequest{}, mapDBError(err)
	}
	if !common.InScope(caller.AllScope, caller.OfficeIDs, a.OfficeID) {
		return sqlc.ApprovalRequest{}, ErrOutOfScope
	}
	if a.Status == sqlc.SharedAssetStatusDisposed || a.Status == sqlc.SharedAssetStatusLost {
		return sqlc.ApprovalRequest{}, ErrAssetNotMaintainable
	}

	pending, err := s.q.CountPendingMaintRequests(ctx, sqlc.CountPendingMaintRequestsParams{AssetID: &in.AssetID, RequestedByID: caller.UserID})
	if err != nil {
		return sqlc.ApprovalRequest{}, mapDBError(err)
	}
	if pending > 0 {
		return sqlc.ApprovalRequest{}, ErrDuplicatePending
	}

	var attachmentID *string
	if in.Photo != nil {
		att, err := s.assets.UploadAttachment(ctx, asset.UploadInput{
			AssetID:     in.AssetID,
			Filename:    in.Photo.Filename,
			ContentType: in.Photo.ContentType,
			Data:        in.Photo.Data,
			CreatedBy:   caller.UserID,
		})
		if err != nil {
			// Asset-service sentinels (ErrUnsupportedType/ErrTooLarge) bubble up
			// unwrapped so the handler can map them to 422.
			return sqlc.ApprovalRequest{}, err
		}
		id := att.ID.String()
		attachmentID = &id
	}

	payload, err := marshalReportPayload(in.AssetID.String(), in.ProblemCategoryID.String(), in.Description, attachmentID)
	if err != nil {
		return sqlc.ApprovalRequest{}, err
	}

	entity := "asset"
	targetID := in.AssetID
	return s.appr.Submit(ctx, approval.SubmitInput{
		Type:         sqlc.SharedRequestTypeMaintenance,
		Amount:       "0",
		OfficeID:     a.OfficeID,
		TargetEntity: &entity,
		TargetID:     &targetID,
		Payload:      payload,
		Reason:       in.Description,
		Maker:        caller.UserID,
	})
}

// CreateCorrectiveFromOpname creates a scheduled corrective maintenance
// record as the follow-up for a stock-opname item marked "damaged".
func (s *Service) CreateCorrectiveFromOpname(ctx context.Context, caller approval.Caller, assetID uuid.UUID, note *string) (uuid.UUID, error) {
	a, err := s.q.GetAsset(ctx, assetID)
	if err != nil {
		return uuid.Nil, mapDBError(err)
	}
	if !common.InScope(caller.AllScope, caller.OfficeIDs, a.OfficeID) {
		return uuid.Nil, ErrOutOfScope
	}
	if a.Status == sqlc.SharedAssetStatusDisposed || a.Status == sqlc.SharedAssetStatusLost {
		return uuid.Nil, ErrAssetNotMaintainable
	}
	desc := "Tindak lanjut stock opname: aset rusak"
	if note != nil && *note != "" {
		desc = *note
	}
	today := pgtype.Date{Time: time.Now(), Valid: true}
	rec, err := s.q.CreateMaintRecord(ctx, sqlc.CreateMaintRecordParams{
		AssetID:       assetID,
		Type:          sqlc.SharedMaintenanceTypeCorrective,
		Status:        sqlc.SharedMaintenanceStatusScheduled,
		ScheduledDate: today,
		Description:   desc,
		ReportedByID:  &caller.UserID,
	})
	if err != nil {
		return uuid.Nil, mapDBError(err)
	}
	return rec.ID, nil
}
