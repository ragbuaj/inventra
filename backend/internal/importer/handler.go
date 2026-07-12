// This file implements the HTTP layer for the bulk-import engine: DTO
// binding, per-target authorization, multipart upload handling, and response
// serialization on top of the Service (job lifecycle) and the generic
// parser/template/errreport building blocks defined elsewhere in this
// package.
package importer

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/audit"
	"github.com/ragbuaj/inventra/internal/authz"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
	"github.com/ragbuaj/inventra/internal/storage"
)

// scopeModule is the data_scope_policies module string for this package,
// mirroring the "imports" module the worker resolves via resolveMakerScope
// (see worker.go) — kept identical so a caller's configured scope for
// imports means the same thing whether resolved from an HTTP request or
// from the async worker.
const scopeModule = "imports"

// Handler is the HTTP handler for the bulk-import module.
type Handler struct {
	svc    *Service
	perm   *authz.PermissionService
	scoped common.ScopedDeps
	aud    *audit.Service
}

// NewHandler builds a Handler with its service, permission checker, scope
// resolver, and audit sink.
func NewHandler(svc *Service, perm *authz.PermissionService, scoped common.ScopedDeps, aud *audit.Service) *Handler {
	return &Handler{svc: svc, perm: perm, scoped: scoped, aud: aud}
}

// svcError maps importer sentinel errors to HTTP status codes (contract b/h):
// ErrNotFound->404, ErrForbidden->403, ErrUnknownTarget->422, ErrBadState->409,
// ErrConflict->409, ErrBadFormat->400 (a parser/template sentinel that can
// also surface from Service.CreateJob), else 500.
//
// NOTE (contract b): Service.mapDBError maps a Postgres 23503 (foreign-key
// violation) to ErrForbidden, which this then reports as 403. In practice
// that mapping is latent-unreachable on the import job's own write paths —
// created_by_id is always a valid, already-authenticated user, and office_id
// is left nil at CreateJob time (see service.go) — so no import-job write
// today can violate a FK it doesn't control. If a 23503 ever does surface
// here it is really a bad-reference (400-class) condition, not a genuine
// authorization denial; the 403 mapping is inherited from the shared
// mapDBError and accepted as harmless given the above, not because 403 is
// semantically correct for it.
func (h *Handler) svcError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, ErrUnknownTarget):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case errors.Is(err, ErrBadState):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, ErrBadFormat):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
	}
}

// checkTargetPermission resolves target's guarding permission key (contract
// d) and reports whether the caller's role holds it, writing the appropriate
// error response (422/403/500) and returning false if not. Callers MUST
// return immediately when this returns false.
func (h *Handler) checkTargetPermission(c *gin.Context, target string) bool {
	key, err := h.svc.PermissionKey(target)
	if err != nil {
		h.svcError(c, err)
		return false
	}
	roleID, err := uuid.Parse(c.GetString(middleware.CtxRoleID))
	if err != nil {
		common.WriteError(c, common.ErrForbidden)
		return false
	}
	allowed, err := h.perm.Has(c.Request.Context(), roleID, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return false
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return false
	}
	return true
}

// callerID resolves the authenticated caller's user id from the Gin context.
func callerID(c *gin.Context) (uuid.UUID, error) {
	return uuid.Parse(c.GetString(middleware.CtxUserID))
}

// sanitizeFilename guards against object-key injection (contract c): it
// strips all C0 control characters (bytes < 0x20, which includes CR/LF/NUL/
// tab/ESC) plus DEL (0x7f) so no control character can survive into the
// object key or the JSON filename field (defense against log/terminal
// injection if the filename is later echoed), normalizes both path separator
// styles, and takes only the final path component. Empty, ".", ".." and "/"
// results are rejected — callers must treat a false ok as a 400.
func sanitizeFilename(name string) (clean string, ok bool) {
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)
	switch name {
	case "", ".", "..", "/":
		return "", false
	}
	return name, true
}

// formatFromFilename derives the import format ("csv"/"xlsx") from a
// sanitized filename's extension; ok is false for anything else.
func formatFromFilename(name string) (format string, ok bool) {
	switch strings.ToLower(path.Ext(name)) {
	case ".csv":
		return "csv", true
	case ".xlsx":
		return "xlsx", true
	default:
		return "", false
	}
}

// safeSlug renders target as a filesystem/header-safe token for use in a
// generated download filename (template/error-report).
func safeSlug(s string) string {
	return strings.NewReplacer(":", "-", "/", "-", "\\", "-", " ", "-").Replace(s)
}

// attachmentDisposition returns a safe RFC 6266 "attachment" Content-Disposition
// header value. It strips CR/LF control characters and lets
// mime.FormatMediaType handle quoting/non-ASCII encoding so a filename built
// from user/target-controlled input cannot inject raw header bytes.
func attachmentDisposition(filename string) string {
	clean := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, filename)
	v := mime.FormatMediaType("attachment", map[string]string{"filename": clean})
	if v == "" {
		return `attachment; filename="download"`
	}
	return v
}

// template handles GET /imports/template?target=&format= — streams a
// header-only CSV/XLSX file for the given target.
func (h *Handler) template(c *gin.Context) {
	target := c.Query("target")
	if target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target is required"})
		return
	}
	if !h.checkTargetPermission(c, target) {
		return
	}
	t, err := h.svc.target(target)
	if err != nil {
		h.svcError(c, err)
		return
	}
	format := c.DefaultQuery("format", "xlsx")
	body, contentType, ext, err := BuildTemplate(format, t.Columns())
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", attachmentDisposition(safeSlug(target)+"-template."+ext))
	c.Data(http.StatusOK, contentType, body)
}

// create handles POST /imports (multipart file, target) — uploads and
// registers a new import job for the async worker to validate.
func (h *Handler) create(c *gin.Context) {
	// Cap the request body to maxBytes+1 so oversize is detected without
	// buffering unbounded data (mirrors asset.uploadAttachment).
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.svc.MaxBytes()+1)

	fileHeader, err := c.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file field"})
		return
	}

	target := strings.TrimSpace(c.PostForm("target"))
	if target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target is required"})
		return
	}
	if !h.checkTargetPermission(c, target) {
		return
	}

	cleanName, ok := sanitizeFilename(fileHeader.Filename)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filename"})
		return
	}
	format, ok := formatFromFilename(cleanName)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported file format (use .csv or .xlsx)"})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read file"})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file too large"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot read file"})
		return
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	// Contract (e): the real per-row office scope is enforced later by the
	// target's ValidateRows, run by the worker against the maker's actual
	// resolved scope (see worker.go resolveMakerScope) — there is no office
	// to gate against yet at create time (CreateJob always leaves office_id
	// nil until validation). This call is defense-in-depth: it threads and
	// fails closed on the caller's identity/scope resolution, without
	// inventing an office check CreateJob has no target for.
	if _, _, err := h.scoped.CallerOfficeScope(c, scopeModule); err != nil {
		common.WriteError(c, err)
		return
	}

	uid, err := callerID(c)
	if err != nil {
		common.WriteError(c, common.ErrForbidden)
		return
	}

	job, err := h.svc.CreateJob(c.Request.Context(), target, format, cleanName, contentType, data, uid)
	if err != nil {
		h.svcError(c, err)
		return
	}

	audit.Record(c, h.aud, audit.ActionCreate, "import_jobs", job.ID, job.OfficeID, audit.Diff(nil, jobToMap(job)))
	c.JSON(http.StatusCreated, jobToMap(job))
}

// list handles GET /imports?target=&limit=&offset= — the caller's own jobs.
func (h *Handler) list(c *gin.Context) {
	uid, err := callerID(c)
	if err != nil {
		common.WriteError(c, common.ErrForbidden)
		return
	}
	target := c.Query("target")
	if target != "" && !h.checkTargetPermission(c, target) {
		return
	}

	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<31-1)

	jobs, total, err := h.svc.ListJobs(c.Request.Context(), uid, target, limit, offset)
	if err != nil {
		h.svcError(c, err)
		return
	}
	data := make([]map[string]any, 0, len(jobs))
	for _, j := range jobs {
		data = append(data, jobToMap(j))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

// loadOwnedJob resolves :id, loads the job (enforcing ownership via
// Service.GetJob/assertOwner — contract g), and checks the caller holds the
// job target's guarding permission. Returns ok=false if it already wrote a
// response.
func (h *Handler) loadOwnedJob(c *gin.Context) (job sqlc.ImportImportJob, ok bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return job, false
	}
	uid, err := callerID(c)
	if err != nil {
		common.WriteError(c, common.ErrForbidden)
		return job, false
	}
	job, err = h.svc.GetJob(c.Request.Context(), id, uid)
	if err != nil {
		h.svcError(c, err)
		return job, false
	}
	if !h.checkTargetPermission(c, job.Target) {
		return job, false
	}
	return job, true
}

// enrichJob adds the best-effort Redis validate-phase progress and, for
// asset jobs routed through maker-checker approval, a derived
// approval_status (contract f). Neither lookup failing anything about the
// base response — both are omitted on error rather than failing the request.
func (h *Handler) enrichJob(c *gin.Context, job sqlc.ImportImportJob, m map[string]any) {
	ctx := c.Request.Context()
	if h.svc.rdb != nil {
		if raw, err := h.svc.rdb.Get(ctx, progressKey(job.ID)).Result(); err == nil {
			var p progress
			if json.Unmarshal([]byte(raw), &p) == nil {
				m["progress"] = p
			}
		}
	}
	if job.Target == "asset" && job.RequestID != nil {
		if req, err := h.svc.q.GetRequest(ctx, *job.RequestID); err == nil {
			m["approval_status"] = string(req.Status)
		}
	}
}

// get handles GET /imports/:id.
func (h *Handler) get(c *gin.Context) {
	job, ok := h.loadOwnedJob(c)
	if !ok {
		return
	}
	m := jobToMap(job)
	h.enrichJob(c, job, m)
	c.JSON(http.StatusOK, m)
}

// rows handles GET /imports/:id/rows?only_errors=&limit=&offset=.
func (h *Handler) rows(c *gin.Context) {
	job, ok := h.loadOwnedJob(c)
	if !ok {
		return
	}
	onlyErrors := c.Query("only_errors") == "true"
	limit := common.ClampInt(c.Query("limit"), 20, 1, 100)
	offset := common.ClampInt(c.Query("offset"), 0, 0, 1<<31-1)

	ctx := c.Request.Context()
	rowsList, err := h.svc.q.ListImportRows(ctx, sqlc.ListImportRowsParams{
		JobID: job.ID, OnlyErrors: onlyErrors, Off: offset, Lim: limit,
	})
	if err != nil {
		common.WriteError(c, common.MapDBError(err))
		return
	}
	total, err := h.svc.q.CountImportRows(ctx, sqlc.CountImportRowsParams{
		JobID: job.ID, OnlyErrors: onlyErrors,
	})
	if err != nil {
		common.WriteError(c, common.MapDBError(err))
		return
	}
	data := make([]map[string]any, 0, len(rowsList))
	for _, r := range rowsList {
		data = append(data, rowToMap(r))
	}
	c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "limit": limit, "offset": offset})
}

// confirm handles POST /imports/:id/confirm — moves a validated job to
// confirmed so the async worker executes it.
func (h *Handler) confirm(c *gin.Context) {
	job, ok := h.loadOwnedJob(c)
	if !ok {
		return
	}
	uid, err := callerID(c)
	if err != nil {
		common.WriteError(c, common.ErrForbidden)
		return
	}
	confirmed, err := h.svc.ConfirmJob(c.Request.Context(), job.ID, uid)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "import_jobs", confirmed.ID, confirmed.OfficeID,
		audit.Diff(jobToMap(job), jobToMap(confirmed)))
	c.JSON(http.StatusOK, jobToMap(confirmed))
}

// cancel handles POST /imports/:id/cancel — cancels a pending/validated job.
func (h *Handler) cancel(c *gin.Context) {
	job, ok := h.loadOwnedJob(c)
	if !ok {
		return
	}
	uid, err := callerID(c)
	if err != nil {
		common.WriteError(c, common.ErrForbidden)
		return
	}
	cancelled, err := h.svc.CancelJob(c.Request.Context(), job.ID, uid)
	if err != nil {
		h.svcError(c, err)
		return
	}
	audit.Record(c, h.aud, audit.ActionUpdate, "import_jobs", cancelled.ID, cancelled.OfficeID,
		audit.Diff(jobToMap(job), jobToMap(cancelled)))
	c.JSON(http.StatusOK, jobToMap(cancelled))
}

// errorReport handles GET /imports/:id/error-report — streams a downloadable
// file listing the job's failed rows plus a "keterangan" error column.
func (h *Handler) errorReport(c *gin.Context) {
	job, ok := h.loadOwnedJob(c)
	if !ok {
		return
	}
	t, err := h.svc.target(job.Target)
	if err != nil {
		h.svcError(c, err)
		return
	}

	format := c.DefaultQuery("format", job.Format)
	// Serve the durable stored report when present and the requested format
	// matches what was persisted (job.Format). A mismatched ?format= or a null
	// key (older jobs) falls through to on-demand generation below.
	//
	// Approval-gated targets (NeedsApproval()==true, currently only "asset")
	// are EXCLUDED from this fast path: their real row creation happens later
	// in the approval executor (see asset/executor.go's assetImportExec.Execute),
	// which can append execute-time failures (e.g. a mid-batch dup-tag TOCTOU)
	// to job.FailedRows well after the validate phase already persisted
	// error_report_key. Serving that stored object would silently omit those
	// execute-time failures, so approval-gated targets always rebuild the
	// report fresh from the job's current failed rows instead. Non-approval
	// targets don't have this gap — their execute phase re-runs
	// storeErrorReport itself (see worker.go's executePhase), so the stored
	// object there is always current.
	if job.ErrorReportKey != nil && !t.NeedsApproval() && strings.EqualFold(format, job.Format) {
		rc, info, err := h.svc.store.Get(c.Request.Context(), *job.ErrorReportKey)
		if err == nil {
			defer rc.Close()
			ext := job.Format
			c.Header("X-Content-Type-Options", "nosniff")
			c.Header("Content-Disposition", attachmentDisposition("import-errors-"+job.ID.String()+"."+ext))
			c.DataFromReader(http.StatusOK, info.Size, info.ContentType, rc, nil)
			return
		}
		if !errors.Is(err, storage.ErrObjectNotFound) {
			common.WriteError(c, err)
			return
		}
		// object missing → fall through to on-demand build
	}

	limit := int32(h.svc.maxRows)
	if limit <= 0 {
		limit = 1 << 20
	}
	rowsList, err := h.svc.q.ListImportRows(c.Request.Context(), sqlc.ListImportRowsParams{
		JobID: job.ID, OnlyErrors: true, Off: 0, Lim: limit,
	})
	if err != nil {
		common.WriteError(c, common.MapDBError(err))
		return
	}

	body, contentType, ext, err := BuildErrorReport(format, t.Columns(), rowsList)
	if err != nil {
		h.svcError(c, err)
		return
	}

	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", attachmentDisposition("import-errors-"+job.ID.String()+"."+ext))
	c.Data(http.StatusOK, contentType, body)
}
