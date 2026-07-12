// This file implements the HTTP response serialization for the bulk-import
// engine: mapping the sqlc job/row rows into the map[string]any shape
// returned by the handler. Internal-only plumbing (object_key, deleted_at,
// and any "_"-prefixed resolved-id keys stamped into a row's data by
// ValidateRows) is deliberately never exposed here.
package importer

import (
	"encoding/json"
	"strings"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// jobToMap serializes an import job into the API response shape. object_key
// (the object-storage path) and deleted_at (soft-delete bookkeeping) are
// intentionally never included — they are internal plumbing, not part of the
// job's public contract.
func jobToMap(job sqlc.ImportImportJob) map[string]any {
	return map[string]any{
		"id":            job.ID.String(),
		"target":        job.Target,
		"format":        job.Format,
		"filename":      job.Filename,
		"status":        string(job.Status),
		"total_rows":    job.TotalRows,
		"success_rows":  job.SuccessRows,
		"failed_rows":   job.FailedRows,
		"error_key":     job.ErrorKey,
		"created_by_id": job.CreatedByID.String(),
		"office_id":     common.UUIDPtrStr(job.OfficeID),
		"request_id":    common.UUIDPtrStr(job.RequestID),
		"created_at":    common.TsStr(job.CreatedAt),
		"updated_at":    common.TsStr(job.UpdatedAt),
		"finished_at":   common.TsStr(job.FinishedAt),
		"confirmed_at":  common.TsStr(job.ConfirmedAt),
	}
}

// rowToMap serializes a persisted import row into the API response shape.
// The row's `data` jsonb may carry internal, importer-resolved fields
// prefixed with "_" (e.g. "_office_id", "_category_id") that a TargetImporter
// stamps in ValidateRows for its own Execute phase to consume later — those
// are deliberately skipped here so the response only ever surfaces the
// user-facing columns the caller uploaded, alongside the row's outcome
// metadata (valid/errors/result_ref/row_no).
func rowToMap(row sqlc.ImportImportRow) map[string]any {
	m := map[string]any{
		"id":     row.ID.String(),
		"row_no": row.RowNo,
		"valid":  row.Valid,
	}

	if len(row.Data) > 0 {
		var data map[string]string
		if err := json.Unmarshal(row.Data, &data); err == nil {
			for k, v := range data {
				if strings.HasPrefix(k, "_") {
					continue
				}
				m[k] = v
			}
		}
	}

	cellErrors := []CellError{}
	if len(row.Errors) > 0 {
		_ = json.Unmarshal(row.Errors, &cellErrors)
	}
	m["errors"] = cellErrors

	if row.ResultRef != nil {
		m["result_ref"] = *row.ResultRef
	}

	return m
}
