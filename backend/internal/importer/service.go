// This file implements the job-lifecycle service for the bulk-import engine:
// sentinel errors, the target registry + permission-key mapping, and the
// DB-backed job CRUD (create/get/list/confirm/cancel) that sits on top of the
// generic parser/template/errreport building blocks defined elsewhere in this
// package.
package importer

import (
	"bytes"
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/storage"
)

// Sentinel errors — reused by the handler and worker.
var (
	ErrNotFound      = errors.New("importer: not found")
	ErrForbidden     = errors.New("importer: forbidden")
	ErrUnknownTarget = errors.New("importer: unknown target")
	ErrBadState      = errors.New("importer: illegal state transition")
	ErrConflict      = errors.New("importer: duplicate")
)

// mapDBError translates pgx/Postgres errors into package sentinel errors.
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
		case "23505":
			return ErrConflict
		case "23503":
			return ErrForbidden
		}
	}
	return err
}

// Service holds the data-access and business-logic layer for the bulk-import
// engine: the target registry plus DB-backed job lifecycle operations.
type Service struct {
	q        *sqlc.Queries
	pool     *pgxpool.Pool
	store    storage.Storage
	rdb      *redis.Client
	reg      registry
	maxRows  int
	maxBytes int64
}

// NewService constructs the import job service.
func NewService(q *sqlc.Queries, pool *pgxpool.Pool, store storage.Storage, rdb *redis.Client, maxRows int, maxBytes int64) *Service {
	return &Service{
		q:        q,
		pool:     pool,
		store:    store,
		rdb:      rdb,
		reg:      registry{},
		maxRows:  maxRows,
		maxBytes: maxBytes,
	}
}

// RegisterTarget registers a per-domain importer, keyed by its own Target().
func (s *Service) RegisterTarget(t TargetImporter) {
	s.reg[t.Target()] = t
}

// MaxBytes returns the configured maximum upload size in bytes (0 means
// unlimited). Exposed so the handler can cap the multipart request body via
// http.MaxBytesReader without reaching into the unexported field directly
// from outside the package (kept as an explicit accessor for clarity even
// though handler.go lives in the same package).
func (s *Service) MaxBytes() int64 { return s.maxBytes }

// target looks up a registered TargetImporter by name.
func (s *Service) target(name string) (TargetImporter, error) {
	t, ok := s.reg.get(name)
	if !ok {
		return nil, ErrUnknownTarget
	}
	return t, nil
}

// PermissionKey maps a target's registry name to the action-permission key
// (internal/authz) that guards it. Unlike target(), this does not require the
// target to be registered — it is a pure string mapping — but a target string
// matching none of the known targets/prefixes is still unknown.
func (s *Service) PermissionKey(target string) (string, error) {
	switch target {
	case "asset":
		return "asset.manage", nil
	case "employee":
		return "masterdata.employee.manage", nil
	case "office":
		return "masterdata.office.manage", nil
	}
	if strings.HasPrefix(target, "reference:") {
		return "masterdata.global.manage", nil
	}
	return "", ErrUnknownTarget
}

// assertOwner returns ErrForbidden when the given job was not created by
// userID. Used by the handler (and by this service's own job methods) to
// enforce that a caller may only act on their own import jobs.
func (s *Service) assertOwner(job sqlc.ImportImportJob, userID uuid.UUID) error {
	if job.CreatedByID != userID {
		return ErrForbidden
	}
	return nil
}

// CreateJob resolves the target, validates the upload's format/size, stores
// the file body in object storage, and inserts the job row. office_id is left
// nil at create time — it is set later once the worker has validated rows.
func (s *Service) CreateJob(ctx context.Context, target, format, filename, contentType string, body []byte, userID uuid.UUID) (sqlc.ImportImportJob, error) {
	if _, err := s.target(target); err != nil {
		return sqlc.ImportImportJob{}, err
	}

	format = strings.ToLower(format)
	switch format {
	case "csv", "xlsx":
	default:
		return sqlc.ImportImportJob{}, ErrBadFormat
	}

	if s.maxBytes > 0 && int64(len(body)) > s.maxBytes {
		return sqlc.ImportImportJob{}, ErrBadState
	}

	// object_key is independent of the job row's id: generate a unique key up
	// front, upload under it, then persist it on the job row at insert time.
	key := "imports/" + uuid.NewString() + "/" + filename
	if err := s.store.Put(ctx, key, bytes.NewReader(body), int64(len(body)), contentType); err != nil {
		return sqlc.ImportImportJob{}, err
	}

	job, err := s.q.CreateImportJob(ctx, sqlc.CreateImportJobParams{
		Target:      target,
		Format:      format,
		Filename:    filename,
		ObjectKey:   &key,
		OfficeID:    nil,
		TotalRows:   0,
		CreatedByID: userID,
	})
	if err != nil {
		return sqlc.ImportImportJob{}, mapDBError(err)
	}
	return job, nil
}

// GetJob fetches a job by id, enforcing that the caller owns it.
func (s *Service) GetJob(ctx context.Context, id, userID uuid.UUID) (sqlc.ImportImportJob, error) {
	job, err := s.q.GetImportJob(ctx, id)
	if err != nil {
		return job, mapDBError(err)
	}
	if err := s.assertOwner(job, userID); err != nil {
		return job, err
	}
	return job, nil
}

// ListJobs returns the caller's own jobs (optionally filtered by target),
// paginated, plus the total matching count.
func (s *Service) ListJobs(ctx context.Context, userID uuid.UUID, target string, limit, offset int32) ([]sqlc.ImportImportJob, int64, error) {
	rows, err := s.q.ListImportJobs(ctx, sqlc.ListImportJobsParams{
		CreatedBy: userID,
		Target:    target,
		Off:       offset,
		Lim:       limit,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	total, err := s.q.CountImportJobs(ctx, sqlc.CountImportJobsParams{
		CreatedBy: userID,
		Target:    target,
	})
	if err != nil {
		return nil, 0, mapDBError(err)
	}
	return rows, total, nil
}

// ConfirmJob moves an owned, validated job to "confirmed" so the async worker
// picks it up for execution. Returns ErrBadState if the job is not currently
// in the "validated" state (the UPDATE's WHERE clause matches no row).
func (s *Service) ConfirmJob(ctx context.Context, id, userID uuid.UUID) (sqlc.ImportImportJob, error) {
	job, err := s.q.GetImportJob(ctx, id)
	if err != nil {
		return job, mapDBError(err)
	}
	if err := s.assertOwner(job, userID); err != nil {
		return job, err
	}
	out, err := s.q.ConfirmJob(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return job, ErrBadState
		}
		return job, mapDBError(err)
	}
	return out, nil
}

// CancelJob cancels an owned job that is still pending or validated. Returns
// ErrBadState if the job is not in a cancellable state.
func (s *Service) CancelJob(ctx context.Context, id, userID uuid.UUID) (sqlc.ImportImportJob, error) {
	job, err := s.q.GetImportJob(ctx, id)
	if err != nil {
		return job, mapDBError(err)
	}
	if err := s.assertOwner(job, userID); err != nil {
		return job, err
	}
	out, err := s.q.CancelJob(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return job, ErrBadState
		}
		return job, mapDBError(err)
	}
	return out, nil
}
