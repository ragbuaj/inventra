package asset

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/ragbuaj/inventra/internal/barcode"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

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
