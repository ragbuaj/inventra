package report

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
	"github.com/ragbuaj/inventra/internal/middleware"
)

// Handler is the HTTP handler for the reporting module. It is read-only:
// report.view gates the JSON reads (dashboard summary + per-type report),
// report.export gates every file download.
type Handler struct {
	svc    *Service
	scoped common.ScopedDeps
}

// NewHandler builds a Handler with its service and scope resolver.
func NewHandler(svc *Service, scoped common.ScopedDeps) *Handler {
	return &Handler{svc: svc, scoped: scoped}
}

// reportTitles maps each report type to its Indonesian document title, used as
// the PDF/export title (and mirrored in the OpenAPI + frontend labels).
var reportTitles = map[string]string{
	"assets":       "Daftar Aset & Nilai Buku",
	"depreciation": "Depresiasi per Periode",
	"utilization":  "Utilisasi",
	"maintenance":  "Biaya Maintenance",
	"transfers":    "Mutasi Aset",
	"disposals":    "Penghapusan Aset",
	"opname":       "Stock Opname",
}

// validStatuses is the shared.asset_status membership set for the optional
// `status` filter (assets report). Anything outside it is a 400.
var validStatuses = map[string]bool{
	"available": true, "assigned": true, "under_maintenance": true,
	"in_transfer": true, "retired": true, "disposed": true, "lost": true,
}

// svcError maps report sentinel errors to HTTP status codes. Most validation
// errors are answered inline (parseCommon / the handlers return 400/403/422
// directly); this handles errors surfaced from a service call.
func (h *Handler) svcError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrOfficeOutOfScope):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidVariant):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
	case errors.Is(err, ErrInvalidPeriod), errors.Is(err, ErrInvalidReportType), errors.Is(err, ErrInvalidExportFormat):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		common.WriteError(c, err)
	}
}

// parseOptionalUUIDQuery parses an optional uuid query param; "" is nil/ok.
func parseOptionalUUIDQuery(c *gin.Context, name string) (*uuid.UUID, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + name})
		return nil, false
	}
	return &id, true
}

// parseCommon extracts the scope + validated filters shared by every endpoint.
// SECURITY-CRITICAL: an `office_id` filter outside the caller's scope is
// rejected with 403 BEFORE any service call — the office-name lookup and the
// dashboard/report aggregates would otherwise leak an out-of-scope office's
// data (the service resolves the drill-down office with AllScope:true).
func (h *Handler) parseCommon(c *gin.Context) (ReportParams, bool) {
	var p ReportParams
	cur, prev, err := ResolvePeriod(c.Query("period"), c.Query("date_from"), c.Query("date_to"), time.Now())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return p, false
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		common.WriteError(c, err)
		return p, false
	}
	officeFilter, ok := parseOptionalUUIDQuery(c, "office_id")
	if !ok {
		return p, false
	}
	categoryID, ok := parseOptionalUUIDQuery(c, "category_id")
	if !ok {
		return p, false
	}
	if officeFilter != nil && !common.InScope(all, ids, *officeFilter) {
		c.JSON(http.StatusForbidden, gin.H{"error": ErrOfficeOutOfScope.Error()})
		return p, false
	}
	var status *string
	if raw := c.Query("status"); raw != "" {
		if !validStatuses[raw] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
			return p, false
		}
		status = &raw
	}
	basis := c.Query("basis")
	if basis != "" && basis != "commercial" && basis != "fiscal" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid basis"})
		return p, false
	}
	p = ReportParams{
		All: all, OfficeIDs: ids, OfficeFilter: officeFilter, CategoryID: categoryID,
		Status: status, Basis: basis, Cur: cur, Prev: prev, RowLimit: jsonRowLimit,
	}
	return p, true
}

// periodLabel formats a window as "2006-01-02 – 2006-01-02" for the export
// subtitle (en-dash separator).
func periodLabel(cur DateRange) string {
	return fmt.Sprintf("%s – %s", cur.From.Format("2006-01-02"), cur.To.Format("2006-01-02"))
}

// callerName resolves the caller's display name (PrintedBy) from CtxUserID,
// tolerating any lookup failure by returning "" (the footer degrades, the
// export never fails on it).
func (h *Handler) callerName(c *gin.Context) string {
	uid, err := uuid.Parse(c.GetString(middleware.CtxUserID))
	if err != nil {
		return ""
	}
	u, err := h.scoped.Q.GetUserByID(c.Request.Context(), uid)
	if err != nil {
		return ""
	}
	return u.Name
}

// officeLabel renders the export's OfficeLabel: the drill-down office name when
// filtered (resolved with AllScope:true — safe because parseCommon already
// scope-checked the filter), else "Seluruh scope".
func (h *Handler) officeLabel(ctx context.Context, p ReportParams) (string, error) {
	if p.OfficeFilter == nil {
		return "Seluruh scope", nil
	}
	office, err := h.scoped.Q.GetOffice(ctx, sqlc.GetOfficeParams{ID: *p.OfficeFilter, AllScope: true})
	if err != nil {
		return "", err
	}
	return office.Name, nil
}

// exportMeta assembles the display metadata common to every export from the
// request + caller.
func (h *Handler) exportMeta(c *gin.Context, title string, p ReportParams) (ExportMeta, error) {
	officeLabel, err := h.officeLabel(c.Request.Context(), p)
	if err != nil {
		return ExportMeta{}, err
	}
	return ExportMeta{
		Title:       title,
		PeriodLabel: periodLabel(p.Cur),
		OfficeLabel: officeLabel,
		PrintedBy:   h.callerName(c),
		PrintedAt:   time.Now(),
	}, nil
}

// writeExport renders one of the two file formats with the attachment headers
// (nosniff + Content-Disposition), matching depreciation.journalExport exactly.
// xlsx/pdf are lazily invoked so only the requested renderer runs.
func (h *Handler) writeExport(c *gin.Context, format, filename string, xlsx, pdf func() ([]byte, error)) {
	switch format {
	case "xlsx":
		body, err := xlsx()
		if err != nil {
			h.svcError(c, err)
			return
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, filename))
		c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", body)
	case "pdf":
		body, err := pdf()
		if err != nil {
			h.svcError(c, err)
			return
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, filename))
		c.Data(http.StatusOK, "application/pdf", body)
	}
}

// dashboardSummary handles GET /dashboard/summary — the cached dashboard read.
func (h *Handler) dashboardSummary(c *gin.Context) {
	p, ok := h.parseCommon(c)
	if !ok {
		return
	}
	roleID, err := uuid.Parse(c.GetString(middleware.CtxRoleID))
	if err != nil {
		common.WriteError(c, common.ErrForbidden)
		return
	}
	sum, err := h.svc.CachedDashboardSummary(c.Request.Context(), roleID, p.All, p.OfficeIDs, p.OfficeFilter, p.Cur, p.Prev)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, sum)
}

// dashboardExport handles GET /dashboard/export — the UNCACHED dashboard,
// rendered as xlsx/pdf (guaranteed fresh: an export must not serve a stale
// cached snapshot).
func (h *Handler) dashboardExport(c *gin.Context) {
	p, ok := h.parseCommon(c)
	if !ok {
		return
	}
	format, err := parseExportFormat(c.Query("format"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	sum, err := h.svc.DashboardSummary(ctx, p.All, p.OfficeIDs, p.OfficeFilter, p.Cur, p.Prev)
	if err != nil {
		h.svcError(c, err)
		return
	}
	meta, err := h.exportMeta(c, "Ringkasan Dashboard", p)
	if err != nil {
		h.svcError(c, err)
		return
	}
	h.writeExport(c, format, exportFilename("dashboard", p.Cur),
		func() ([]byte, error) { return BuildDashboardXLSX(sum, meta) },
		func() ([]byte, error) { return h.svc.BuildDashboardPDF(ctx, sum, meta) },
	)
}

// run handles GET /reports/:type — the JSON per-type report (capped rows).
func (h *Handler) run(c *gin.Context) {
	typ, err := ParseReportType(c.Param("type"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, ok := h.parseCommon(c)
	if !ok {
		return
	}
	p.RowLimit = jsonRowLimit
	res, err := h.svc.Run(c.Request.Context(), typ, p)
	if err != nil {
		h.svcError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// runExport handles GET /reports/:type/export — the file download. The default
// "table" variant renders the per-type table (effectively-unbounded rows);
// variant=gl_recap is the disposal journal recap and is valid ONLY for the
// disposals type (else 422).
func (h *Handler) runExport(c *gin.Context) {
	typ, err := ParseReportType(c.Param("type"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	format, err := parseExportFormat(c.Query("format"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	variant := c.DefaultQuery("variant", "table")
	if variant != "table" && variant != "gl_recap" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": ErrInvalidVariant.Error()})
		return
	}
	if variant == "gl_recap" && typ != "disposals" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": ErrInvalidVariant.Error()})
		return
	}
	p, ok := h.parseCommon(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	if variant == "gl_recap" {
		recap, err := h.svc.DisposalGlRecap(ctx, p)
		if err != nil {
			h.svcError(c, err)
			return
		}
		meta, err := h.exportMeta(c, "Rekap Jurnal Penghapusan Aset", p)
		if err != nil {
			h.svcError(c, err)
			return
		}
		h.writeExport(c, format, exportFilename("disposals-gl", p.Cur),
			func() ([]byte, error) { return BuildGlRecapXLSX(recap, meta) },
			func() ([]byte, error) { return h.svc.BuildGlRecapPDF(ctx, recap, meta) },
		)
		return
	}

	// table variant: lift the row cap so the export carries the full result set.
	p.RowLimit = 1_000_000
	res, err := h.svc.Run(ctx, typ, p)
	if err != nil {
		h.svcError(c, err)
		return
	}
	meta, err := h.exportMeta(c, reportTitles[typ], p)
	if err != nil {
		h.svcError(c, err)
		return
	}
	h.writeExport(c, format, exportFilename(typ, p.Cur),
		func() ([]byte, error) { return BuildReportXLSX(res, meta) },
		func() ([]byte, error) { return h.svc.BuildReportPDF(ctx, res, meta) },
	)
}
