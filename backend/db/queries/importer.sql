-- name: CreateImportJob :one
INSERT INTO import.import_jobs (target, format, filename, object_key, office_id, total_rows, created_by_id, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending')
RETURNING *;

-- name: GetImportJob :one
SELECT * FROM import.import_jobs WHERE id = $1 AND deleted_at IS NULL;

-- name: GetImportJobForUpdate :one
SELECT * FROM import.import_jobs WHERE id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: ListImportJobs :many
SELECT * FROM import.import_jobs
WHERE deleted_at IS NULL
  AND created_by_id = @created_by
  AND (@target::text = '' OR target = @target::text)
ORDER BY created_at DESC
LIMIT @lim OFFSET @off;

-- name: CountImportJobs :one
SELECT count(*) FROM import.import_jobs
WHERE deleted_at IS NULL
  AND created_by_id = @created_by
  AND (@target::text = '' OR target = @target::text);

-- name: ClaimPendingJob :one
SELECT * FROM import.import_jobs
WHERE status = 'pending' AND deleted_at IS NULL
ORDER BY created_at
FOR UPDATE SKIP LOCKED
LIMIT 1;

-- name: ClaimConfirmedJob :one
SELECT * FROM import.import_jobs
WHERE status = 'confirmed' AND deleted_at IS NULL
ORDER BY confirmed_at
FOR UPDATE SKIP LOCKED
LIMIT 1;

-- name: UpdateJobStatus :one
UPDATE import.import_jobs SET status = $2 WHERE id = $1 RETURNING *;

-- name: SetJobValidated :one
UPDATE import.import_jobs
SET status = 'validated', total_rows = $2, success_rows = $3, failed_rows = $4
WHERE id = $1 RETURNING *;

-- name: SetJobResult :one
UPDATE import.import_jobs
SET status = $2, success_rows = $3, failed_rows = $4, error_key = $5, finished_at = now()
WHERE id = $1 RETURNING *;

-- name: SetJobRequest :one
UPDATE import.import_jobs
SET status = 'awaiting_approval', request_id = $2
WHERE id = $1 RETURNING *;

-- name: ConfirmJob :one
UPDATE import.import_jobs
SET status = 'confirmed', confirmed_at = now()
WHERE id = $1 AND status = 'validated'
RETURNING *;

-- name: CancelJob :one
UPDATE import.import_jobs
SET status = 'cancelled', finished_at = now()
WHERE id = $1 AND status IN ('pending', 'validated')
RETURNING *;

-- name: RecoverStuckJobs :execrows
UPDATE import.import_jobs
SET status = CASE status WHEN 'processing' THEN 'pending'::shared.import_status
                         WHEN 'executing'  THEN 'confirmed'::shared.import_status
                         ELSE status END
WHERE status IN ('processing', 'executing') AND deleted_at IS NULL;

-- name: InsertImportRow :one
INSERT INTO import.import_rows (job_id, row_no, data, valid, errors)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListImportRows :many
SELECT * FROM import.import_rows
WHERE job_id = $1 AND deleted_at IS NULL
  AND (@only_errors::bool = false OR valid = false)
ORDER BY row_no
LIMIT @lim OFFSET @off;

-- name: CountImportRows :one
SELECT count(*) FROM import.import_rows
WHERE job_id = $1 AND deleted_at IS NULL
  AND (@only_errors::bool = false OR valid = false);

-- name: ListValidImportRows :many
SELECT * FROM import.import_rows
WHERE job_id = $1 AND valid = true AND deleted_at IS NULL
ORDER BY row_no;

-- name: MarkRowResult :exec
UPDATE import.import_rows SET result_ref = $2 WHERE id = $1;

-- name: MarkRowFailed :exec
UPDATE import.import_rows SET valid = false, errors = $2 WHERE id = $1;
