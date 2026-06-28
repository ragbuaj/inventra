package asset

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ragbuaj/inventra/internal/barcode"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// LabelRequest is the request body for POST /assets/labels.
type LabelRequest struct {
	AssetIDs []string `json:"asset_ids"`
	Tags     []string `json:"tags"`
	Template string   `json:"template"` // btn (default) | generic
	Layout   string   `json:"layout"`   // roll (default) | sheet
	Size     string   `json:"size"`
	WidthMM  float64  `json:"w_mm"`
	HeightMM float64  `json:"h_mm"`
	MediaWMM float64  `json:"media_w_mm"`
	Columns  int      `json:"columns"`
	Mode     string   `json:"mode"` // barcode (default) | qr | both
	Fields   struct {
		Name   bool `json:"name"`
		Office bool `json:"office"`
	} `json:"fields"`
}

func (r LabelRequest) validate() error {
	if len(r.AssetIDs) == 0 && len(r.Tags) == 0 {
		return errors.New("provide asset_ids or tags")
	}
	switch r.Template {
	case "", "btn", "generic":
	default:
		return errors.New("template must be btn or generic")
	}
	switch r.Layout {
	case "", "roll", "sheet":
	default:
		return errors.New("layout must be roll or sheet")
	}
	switch r.Mode {
	case "", "barcode", "qr", "both":
	default:
		return errors.New("mode must be barcode, qr, or both")
	}
	return nil
}

// getByTag: GET /assets/by-tag/:tag — scan lookup.
// Out-of-scope assets return 404 (not 403) to avoid tag enumeration.
func (h *Handler) getByTag(c *gin.Context) {
	tag := c.Param("tag")
	a, err := h.svc.GetByTag(c.Request.Context(), tag)
	if err != nil {
		svcError(c, err) // ErrNotFound → 404
		return
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	if !common.InScope(all, ids, a.OfficeID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	masked, err := h.filterMap(c, assetToMap(a))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve field permissions"})
		return
	}
	c.JSON(http.StatusOK, masked)
}

// getBarcode: GET /assets/:id/barcode?type=code128|qr — returns a PNG barcode image.
// Out-of-scope assets return 403. Unknown type param returns 400.
func (h *Handler) getBarcode(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	a, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		svcError(c, err)
		return
	}
	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		common.WriteError(c, err)
		return
	}
	if !common.InScope(all, ids, a.OfficeID) {
		common.WriteError(c, common.ErrForbidden)
		return
	}

	typ := c.DefaultQuery("type", "code128")
	var png []byte
	switch typ {
	case "code128":
		png, err = barcode.EncodeCode128(a.AssetTag)
	case "qr":
		png, err = barcode.EncodeQR(a.AssetTag)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be code128 or qr"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "barcode encode failed"})
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "image/png", png)
}

// generateLabels: POST /assets/labels — renders a label PDF for the requested assets.
func (h *Handler) generateLabels(c *gin.Context) {
	var req LabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := req.validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	labelW, labelH, mediaW, err := resolveLabelDims(req.Size, req.WidthMM, req.HeightMM, req.MediaWMM)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tmpl := req.Template
	if tmpl == "" {
		tmpl = "btn"
	}
	layout := req.Layout
	if layout == "" {
		layout = "roll"
	}
	mode := req.Mode
	if mode == "" {
		mode = "barcode"
	}

	in := LabelInput{
		Opts: labelOpts{
			Template:   tmpl,
			Layout:     layout,
			LabelW:     labelW,
			LabelH:     labelH,
			MediaW:     mediaW,
			Columns:    req.Columns,
			Mode:       mode,
			ShowName:   req.Fields.Name,
			ShowOffice: req.Fields.Office,
		},
	}

	for _, sID := range req.AssetIDs {
		id, perr := uuid.Parse(sID)
		if perr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset id: " + sID})
			return
		}
		in.AssetIDs = append(in.AssetIDs, id)
	}
	in.Tags = req.Tags

	all, ids, err := h.scoped.CallerOfficeScope(c, scopeModule)
	if err != nil {
		common.WriteError(c, err)
		return
	}

	pdf, err := h.svc.BuildLabelPDF(c.Request.Context(), in, all, ids)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoAssets):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, common.ErrForbidden):
			common.WriteError(c, common.ErrForbidden)
		default:
			svcError(c, err)
		}
		return
	}

	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Disposition", `attachment; filename="labels.pdf"`)
	c.Data(http.StatusOK, "application/pdf", pdf)
}
