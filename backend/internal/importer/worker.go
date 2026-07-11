// This file implements the async DB-queue worker that drives a bulk-import
// job through its two remaining phases after upload: validate (parse the
// stored file, run the target's business validation, persist per-row
// results) and execute (either open a maker-checker approval request for
// targets that need one, or run the target's Execute directly). The worker
// polls the import_jobs table using SELECT ... FOR UPDATE SKIP LOCKED so
// multiple worker instances can run concurrently without double-processing a
// job.
//
// Deliberately no dependency on the asset package: the importer package is
// imported BY asset (for TargetImporter), so referencing any asset-package
// type here would create an import cycle. The asset approval payload is
// therefore built as a generic JSON object (see buildAssetPayload) — the
// asset executor (a later task) unmarshals it by the same field names.
package importer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/approval"
	"github.com/ragbuaj/inventra/internal/authz"
)

// Submitter is the narrow slice of approval.Service the worker depends on: it
// opens a maker-checker approval request for import targets that need
// approval before executing (currently only "asset"). Defined here (rather
// than depended on from approval) so the worker can be unit-tested against a
// stub without pulling in the whole approval service.
type Submitter interface {
	Submit(ctx context.Context, in approval.SubmitInput) (sqlc.ApprovalRequest, error)
}

// Worker polls the import job queue and drives jobs through the validate and
// execute phases. One tick does at most one unit of work (one validate OR one
// execute) so callers can bound how much work happens per poll interval.
type Worker struct {
	svc   *Service
	pool  *pgxpool.Pool
	rdb   *redis.Client
	sub   Submitter
	scope *authz.ScopeService
	poll  time.Duration
}

// NewWorker constructs a Worker. poll is the interval between ticks in Run;
// a non-positive poll defaults to 2s (a zero-value time.Duration would make
// time.NewTicker panic).
func NewWorker(svc *Service, pool *pgxpool.Pool, rdb *redis.Client, sub Submitter, scope *authz.ScopeService, poll time.Duration) *Worker {
	if poll <= 0 {
		poll = 2 * time.Second
	}
	return &Worker{svc: svc, pool: pool, rdb: rdb, sub: sub, scope: scope, poll: poll}
}

// progressKey returns the Redis key used to publish live validate-phase
// progress for the given job, read by the job-status endpoint/UI.
func progressKey(jobID uuid.UUID) string {
	return "import:progress:" + jobID.String()
}

// aggregate counts how many of the given RowResults are valid vs invalid.
func aggregate(results []RowResult) (success, failed int) {
	for _, r := range results {
		if r.Valid {
			success++
		} else {
			failed++
		}
	}
	return success, failed
}

// progress is the shape written to Redis so pollers can report validate-phase
// progress without hitting Postgres.
type progress struct {
	Phase string `json:"phase"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

// resolveMakerScope resolves the maker's real data scope for the "imports"
// module, mirroring internal/masterdata/common/scope.go CallerOfficeScope but
// without a Gin context (the worker runs outside any HTTP request — there is
// no request to resolve the caller from, so the maker's user row is looked up
// directly by ID). This closes the M4 scope-bypass finding: validatePhase
// must enforce the SAME per-row visibility ValidateRows would see from a live
// request, not an unconditional AllScope:true.
func (w *Worker) resolveMakerScope(ctx context.Context, makerID uuid.UUID) (Scope, error) {
	user, err := w.svc.q.GetUserByID(ctx, makerID)
	if err != nil {
		return Scope{}, err
	}
	sc, err := w.scope.Resolve(ctx, user.RoleID, user.OfficeID, "imports")
	if err != nil {
		return Scope{}, err
	}
	switch sc.Level {
	case sqlc.SharedScopeLevelGlobal:
		return Scope{AllScope: true, UserID: makerID}, nil
	case sqlc.SharedScopeLevelOwn:
		if user.OfficeID != nil {
			return Scope{AllScope: false, OfficeIDs: []uuid.UUID{*user.OfficeID}, UserID: makerID}, nil
		}
		return Scope{AllScope: false, OfficeIDs: []uuid.UUID{}, UserID: makerID}, nil
	default: // office / office_subtree
		return Scope{AllScope: false, OfficeIDs: sc.OfficeIDs, UserID: makerID}, nil
	}
}

// Recover resets any job left in an in-flight state (processing/executing) by
// a worker that crashed mid-phase, so it is picked up again. Call once at
// startup before Run.
func (w *Worker) Recover(ctx context.Context) error {
	_, err := w.svc.q.RecoverStuckJobs(ctx)
	return err
}

// Run polls at the configured interval until ctx is cancelled. Errors from an
// individual tick are swallowed (logged by a future observability pass) so a
// transient failure on one job does not stop the loop.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = w.tick(ctx)
		}
	}
}

// tick performs one pass: try to claim and validate a pending job; if none is
// pending, try to claim and execute a confirmed job. didWork reports whether
// a job was claimed (regardless of whether it ultimately succeeded).
func (w *Worker) tick(ctx context.Context) (didWork bool, err error) {
	did, err := w.validatePhase(ctx)
	if err != nil || did {
		return did, err
	}
	return w.executePhase(ctx)
}

// validatePhase claims at most one pending job, parses its stored file,
// validates the rows against the target's business rules, persists each
// row's result, and moves the job to "validated" (or "failed" on a parse
// error). All row inserts and the final status transition happen in one
// transaction so a crash mid-phase leaves the job in "processing" for
// Recover to reclaim rather than half-populated.
func (w *Worker) validatePhase(ctx context.Context) (didWork bool, err error) {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := w.svc.q.WithTx(tx)

	job, err := qtx.ClaimPendingJob(ctx)
	if err != nil {
		// No pending job (or a genuine query error) — either way there is
		// nothing to validate this tick.
		return false, nil //nolint:nilerr
	}

	if _, err := qtx.UpdateJobStatus(ctx, sqlc.UpdateJobStatusParams{
		ID:     job.ID,
		Status: sqlc.SharedImportStatusProcessing,
	}); err != nil {
		return true, err
	}

	target, err := w.svc.target(job.Target)
	if err != nil {
		return true, err
	}

	if job.ObjectKey == nil {
		if _, sErr := qtx.SetJobResult(ctx, sqlc.SetJobResultParams{
			ID:       job.ID,
			Status:   sqlc.SharedImportStatusFailed,
			ErrorKey: strPtr("parseFailed"),
		}); sErr != nil {
			return true, sErr
		}
		return true, tx.Commit(ctx)
	}

	reader, _, err := w.svc.store.Get(ctx, *job.ObjectKey)
	if err != nil {
		if _, sErr := qtx.SetJobResult(ctx, sqlc.SetJobResultParams{
			ID:       job.ID,
			Status:   sqlc.SharedImportStatusFailed,
			ErrorKey: strPtr("parseFailed"),
		}); sErr != nil {
			return true, sErr
		}
		return true, tx.Commit(ctx)
	}
	body, err := readAllAndClose(reader)
	if err != nil {
		if _, sErr := qtx.SetJobResult(ctx, sqlc.SetJobResultParams{
			ID:       job.ID,
			Status:   sqlc.SharedImportStatusFailed,
			ErrorKey: strPtr("parseFailed"),
		}); sErr != nil {
			return true, sErr
		}
		return true, tx.Commit(ctx)
	}

	rawRows, err := Parse(job.Format, body, target.Columns(), w.svc.maxRows)
	if err != nil {
		key := errorKeyFor(err)
		if _, sErr := qtx.SetJobResult(ctx, sqlc.SetJobResultParams{
			ID:       job.ID,
			Status:   sqlc.SharedImportStatusFailed,
			ErrorKey: &key,
		}); sErr != nil {
			return true, sErr
		}
		return true, tx.Commit(ctx)
	}

	// F1/M4: resolve the maker's REAL data scope instead of hardcoding
	// AllScope:true, which bypassed the maker's office scope entirely.
	// Fail closed (never AllScope:true) if scope resolution itself errors.
	scope, err := w.resolveMakerScope(ctx, job.CreatedByID)
	if err != nil {
		key := "scopeResolveFailed"
		if _, sErr := qtx.SetJobResult(ctx, sqlc.SetJobResultParams{
			ID:       job.ID,
			Status:   sqlc.SharedImportStatusFailed,
			ErrorKey: &key,
		}); sErr != nil {
			return true, sErr
		}
		return true, tx.Commit(ctx)
	}
	results, err := target.ValidateRows(ctx, rawRows, scope)
	if err != nil {
		return true, err
	}

	for _, r := range results {
		data, mErr := json.Marshal(r.Data)
		if mErr != nil {
			return true, mErr
		}
		errs, mErr := json.Marshal(r.Errors)
		if mErr != nil {
			return true, mErr
		}
		if _, iErr := qtx.InsertImportRow(ctx, sqlc.InsertImportRowParams{
			JobID:  job.ID,
			RowNo:  int32(r.RowNo),
			Data:   data,
			Valid:  r.Valid,
			Errors: errs,
		}); iErr != nil {
			return true, iErr
		}
	}

	success, failed := aggregate(results)

	// Publish progress best-effort — a Redis failure must not abort the
	// validate transaction.
	if w.rdb != nil {
		if payload, mErr := json.Marshal(progress{Phase: "validate", Done: len(results), Total: len(results)}); mErr == nil {
			_ = w.rdb.Set(ctx, progressKey(job.ID), payload, time.Hour).Err()
		}
	}

	// Persist the batch office resolved by ValidateRows (via NormalizedRef on
	// the first valid row), so the execute phase knows where to route the
	// approval and the job carries its office for scope purposes. Targets
	// whose rows carry no office ref (NormalizedRef empty/absent) leave this
	// nil, which is harmless.
	office := firstValidOffice(results)

	if _, err := qtx.SetJobValidated(ctx, sqlc.SetJobValidatedParams{
		ID:          job.ID,
		TotalRows:   int32(len(results)),
		SuccessRows: int32(success),
		FailedRows:  int32(failed),
		OfficeID:    office,
	}); err != nil {
		return true, err
	}

	if err := tx.Commit(ctx); err != nil {
		return true, err
	}
	return true, nil
}

// executePhase claims at most one confirmed job and either opens a
// maker-checker approval request (targets that NeedsApproval) or runs the
// target's Execute directly inside a transaction, then records the outcome.
func (w *Worker) executePhase(ctx context.Context) (didWork bool, err error) {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := w.svc.q.WithTx(tx)

	job, err := qtx.ClaimConfirmedJob(ctx)
	if err != nil {
		return false, nil //nolint:nilerr
	}

	if _, err := qtx.UpdateJobStatus(ctx, sqlc.UpdateJobStatusParams{
		ID:     job.ID,
		Status: sqlc.SharedImportStatusExecuting,
	}); err != nil {
		return true, err
	}

	target, err := w.svc.target(job.Target)
	if err != nil {
		return true, err
	}

	rows, err := qtx.ListValidImportRows(ctx, job.ID)
	if err != nil {
		return true, err
	}

	domainRows := make([]Row, 0, len(rows))
	for _, r := range rows {
		var data map[string]string
		if len(r.Data) > 0 {
			if uErr := json.Unmarshal(r.Data, &data); uErr != nil {
				return true, uErr
			}
		}
		domainRows = append(domainRows, Row{ID: r.ID, RowNo: int(r.RowNo), Data: data})
	}

	if target.NeedsApproval() {
		// F3/M1: idempotency guard. job.RequestID is set together with
		// status -> awaiting_approval by SetJobRequest, so a job reclaimed
		// via ClaimConfirmedJob after a crash (Recover resets
		// executing -> confirmed, never touching request_id) always has
		// RequestID == nil here — this is a cheap first check, not the real
		// guard. Kept anyway since it's a free win if it's ever non-nil.
		if job.RequestID != nil {
			if _, err := qtx.SetJobRequest(ctx, sqlc.SetJobRequestParams{
				ID:        job.ID,
				RequestID: job.RequestID,
			}); err != nil {
				return true, err
			}
			if err := tx.Commit(ctx); err != nil {
				return true, err
			}
			return true, nil
		}

		// F3: the real closer. A prior run may have crashed between Submit
		// committing its own transaction and SetJobRequest persisting
		// request_id (which happens in a separate, later statement — see
		// below). Look up whether an approval request already exists for
		// this batch (target_entity='import_job', target_id=job.ID) before
		// submitting again; if one does, link it instead of creating a
		// duplicate. Queried via qtx (this tx, still open/uncommitted at
		// this point) — READ COMMITTED still lets it see any request
		// committed by a prior crashed run.
		existingID, findErr := qtx.FindActiveImportRequest(ctx, &job.ID)
		switch {
		case findErr == nil:
			if _, err := qtx.SetJobRequest(ctx, sqlc.SetJobRequestParams{
				ID:        job.ID,
				RequestID: &existingID,
			}); err != nil {
				return true, err
			}
			if err := tx.Commit(ctx); err != nil {
				return true, err
			}
			return true, nil
		case errors.Is(findErr, pgx.ErrNoRows):
			// No existing request — proceed with the normal Submit path.
		default:
			// Fail closed: an indeterminate lookup must not risk a
			// duplicate Submit. The claiming tx is still open here — roll it
			// back first to release the job row's lock before failJob opens
			// its own transaction (same pattern as the Execute error path
			// below).
			_ = tx.Rollback(ctx)
			if fErr := w.failJob(ctx, job.ID, "submitFailed"); fErr != nil {
				return true, fErr
			}
			return true, findErr
		}

		var officeID uuid.UUID
		if job.OfficeID != nil {
			officeID = *job.OfficeID
		}

		// F4/M3: an approval target with no resolvable office cannot be
		// routed to a maker-checker chain (approval.Submit rejects
		// uuid.Nil). Fail the job explicitly instead of calling Submit and
		// stranding it in "executing".
		if officeID == uuid.Nil {
			if _, err := qtx.SetJobResult(ctx, sqlc.SetJobResultParams{
				ID:       job.ID,
				Status:   sqlc.SharedImportStatusFailed,
				ErrorKey: strPtr("noOffice"),
			}); err != nil {
				return true, err
			}
			if err := tx.Commit(ctx); err != nil {
				return true, err
			}
			return true, nil
		}

		totalValue, sumErr := sumHarga(domainRows)
		if sumErr != nil {
			return true, sumErr
		}

		payload, mErr := json.Marshal(map[string]any{
			"job_id":      job.ID.String(),
			"filename":    job.Filename,
			"total_rows":  len(domainRows),
			"total_value": totalValue,
			"office_id":   officeID.String(),
		})
		if mErr != nil {
			return true, mErr
		}

		// Submit runs against the shared pool (not this tx) since approval
		// owns its own transactional boundary; committing the "executing"
		// status transition here first keeps the job row consistent even if
		// Submit fails.
		if err := tx.Commit(ctx); err != nil {
			return true, err
		}

		targetEntity := "import_job"
		req, subErr := w.sub.Submit(ctx, approval.SubmitInput{
			Type:         sqlc.SharedRequestTypeAssetImport,
			Amount:       totalValue,
			OfficeID:     officeID,
			TargetEntity: &targetEntity,
			TargetID:     &job.ID,
			Payload:      payload,
			Maker:        job.CreatedByID,
		})
		if subErr != nil {
			// F2/M2: terminal-fail instead of leaving the job stuck in
			// "executing" forever on a deterministic error (e.g.
			// approval.ErrNoThreshold) — a poison pill that would otherwise
			// loop forever across Recover/retry cycles.
			if fErr := w.failJob(ctx, job.ID, "submitFailed"); fErr != nil {
				return true, fErr
			}
			return true, subErr
		}

		if _, err := w.svc.q.SetJobRequest(ctx, sqlc.SetJobRequestParams{
			ID:        job.ID,
			RequestID: &req.ID,
		}); err != nil {
			return true, err
		}
		return true, nil
	}

	created, execErr := target.Execute(ctx, qtx, Job{
		ID:        job.ID,
		Target:    job.Target,
		Format:    job.Format,
		Filename:  job.Filename,
		OfficeID:  job.OfficeID,
		TotalRows: int(job.TotalRows),
	}, domainRows)
	if execErr != nil {
		// F2/M2: terminal-fail instead of leaving the job stuck in
		// "executing". The claiming tx is still open (uncommitted) here —
		// roll it back first to release the job row's lock before failJob
		// opens its own transaction to record the failure.
		_ = tx.Rollback(ctx)
		if fErr := w.failJob(ctx, job.ID, "executeFailed"); fErr != nil {
			return true, fErr
		}
		return true, execErr
	}

	if _, err := qtx.SetJobResult(ctx, sqlc.SetJobResultParams{
		ID:          job.ID,
		Status:      sqlc.SharedImportStatusCompleted,
		SuccessRows: int32(created),
		FailedRows:  int32(len(domainRows) - created),
	}); err != nil {
		return true, err
	}

	if err := tx.Commit(ctx); err != nil {
		return true, err
	}
	return true, nil
}

// failJob transitions a job to "failed" with the given error_key in its own
// freshly-committed transaction. Used by executePhase to give a job a
// terminal outcome when a deterministic error (approval Submit or target
// Execute) occurs, instead of leaving it stuck in an in-flight status where
// Recover would just reclaim and retry it forever (a poison pill). Any
// caller with an outer transaction still open on the same job row must roll
// it back before calling this, to avoid a self-deadlock on the row lock.
func (w *Worker) failJob(ctx context.Context, jobID uuid.UUID, errorKey string) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	qtx := w.svc.q.WithTx(tx)
	if _, err := qtx.SetJobResult(ctx, sqlc.SetJobResultParams{
		ID:       jobID,
		Status:   sqlc.SharedImportStatusFailed,
		ErrorKey: &errorKey,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// firstValidOffice returns the office UUID parsed from the first valid
// result's NormalizedRef, or nil if there is no valid row or its
// NormalizedRef is empty/not a UUID (targets whose rows carry no office ref).
func firstValidOffice(results []RowResult) *uuid.UUID {
	for _, r := range results {
		if !r.Valid || r.NormalizedRef == "" {
			continue
		}
		id, err := uuid.Parse(r.NormalizedRef)
		if err != nil {
			return nil
		}
		return &id
	}
	return nil
}

// sumHarga sums the "harga" cell across the given rows as an exact decimal
// string using math/big.Rat (no float precision loss). "harga" is
// asset-specific, which is acceptable here because asset is the only
// registered target with NeedsApproval() == true.
func sumHarga(rows []Row) (string, error) {
	total := new(big.Rat)
	for _, r := range rows {
		v, ok := r.Data["harga"]
		if !ok || v == "" {
			continue
		}
		amt, ok := new(big.Rat).SetString(v)
		if !ok {
			return "", fmt.Errorf("importer: invalid decimal %q in row %d", v, r.RowNo)
		}
		total.Add(total, amt)
	}
	return total.FloatString(2), nil
}

// strPtr returns a pointer to s — a tiny convenience for building optional
// sqlc params inline.
func strPtr(s string) *string { return &s }

// readAllAndClose reads r to completion and closes it, propagating either
// error.
func readAllAndClose(r io.ReadCloser) ([]byte, error) {
	defer r.Close()
	return io.ReadAll(r)
}
