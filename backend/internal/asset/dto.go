package asset

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// assetToMap serializes an asset to a map suitable for field-permission filtering.
// Sensitive financial keys (purchase_cost, book_value, accumulated_depreciation)
// are included so that authz.FilterView can drop them for roles that lack view permission.
func assetToMap(a sqlc.AssetAsset) map[string]any {
	var deprMethod *string
	if a.DepreciationMethod != nil {
		s := string(*a.DepreciationMethod)
		deprMethod = &s
	}
	var fiscalGroup *string
	if a.FiscalGroup != nil {
		s := string(*a.FiscalGroup)
		fiscalGroup = &s
	}
	return map[string]any{
		"id":                        a.ID.String(),
		"asset_tag":                 a.AssetTag,
		"name":                      a.Name,
		"category_id":               a.CategoryID.String(),
		"office_id":                 a.OfficeID.String(),
		"brand_id":                  common.UUIDPtrStr(a.BrandID),
		"model_id":                  common.UUIDPtrStr(a.ModelID),
		"room_id":                   common.UUIDPtrStr(a.RoomID),
		"unit_id":                   common.UUIDPtrStr(a.UnitID),
		"vendor_id":                 common.UUIDPtrStr(a.VendorID),
		"current_holder_employee_id": common.UUIDPtrStr(a.CurrentHolderEmployeeID),
		"created_by_id":             common.UUIDPtrStr(a.CreatedByID),
		"status":                    string(a.Status),
		"asset_class":               string(a.AssetClass),
		"serial_number":             a.SerialNumber,
		"purchase_date":             dateStr(a.PurchaseDate),
		// Sensitive financial fields — present pre-mask so FilterView can drop them.
		"purchase_cost":             a.PurchaseCost,
		"book_value":                a.BookValue,
		"accumulated_depreciation":  a.AccumulatedDepreciation,
		"salvage_value":             a.SalvageValue,
		"impairment_loss":           a.ImpairmentLoss,
		"po_number":                 a.PoNumber,
		"funding_source":            a.FundingSource,
		"warranty_expiry":           dateStr(a.WarrantyExpiry),
		// Legacy-parity fields (spec 2026-07-23).
		"floor_id":          common.UUIDPtrStr(a.FloorID),
		"pic_employee_id":   common.UUIDPtrStr(a.PicEmployeeID),
		"capacity":          a.Capacity,
		"lease_date":        dateStr(a.LeaseDate),
		"installation_date": dateStr(a.InstallationDate),
		"warranty_start":    dateStr(a.WarrantyStart),
		"capitalized":               a.Capitalized,
		"depreciation_method":       deprMethod,
		"useful_life_months":        a.UsefulLifeMonths,
		"fiscal_group":              fiscalGroup,
		"fiscal_life_months":        a.FiscalLifeMonths,
		"acquisition_bast_no":       a.AcquisitionBastNo,
		"excluded_from_valuation":   a.ExcludedFromValuation,
		"valuation_exclusion_reason": a.ValuationExclusionReason,
		"notes":                     a.Notes,
		"created_at":                common.TsStr(a.CreatedAt),
		"updated_at":                common.TsStr(a.UpdatedAt),
	}
}

// dateStr renders a pgtype.Date as a *string ("YYYY-MM-DD"), or nil if not valid.
func dateStr(d pgtype.Date) *string {
	if !d.Valid {
		return nil
	}
	s := d.Time.Format(time.DateOnly)
	return &s
}

// attachmentToMap serializes an asset attachment for the API response.
// Storage-internal fields (object_key, thumbnail_key) are intentionally omitted;
// has_thumbnail (bool) is derived from ThumbnailKey so callers can conditionally
// render thumbnails without knowing the storage path.
func attachmentToMap(a sqlc.AssetAssetAttachment) map[string]any {
	return map[string]any{
		"id":                a.ID.String(),
		"asset_id":          a.AssetID.String(),
		"kind":              string(a.Kind),
		"original_filename": a.OriginalFilename,
		"size_bytes":        a.SizeBytes,
		"mime_type":         a.MimeType,
		"has_thumbnail":     a.ThumbnailKey != nil,
		"created_at":        common.TsStr(a.CreatedAt),
	}
}

// AssetUpdateRequest is the PUT body for non-sensitive asset attributes.
// purchase_cost and asset_class are excluded (handled by dedicated operations).
type AssetUpdateRequest struct {
	Name          string  `json:"name" binding:"required"`
	CategoryID    string  `json:"category_id" binding:"required,uuid"`
	BrandID       *string `json:"brand_id" binding:"omitempty,uuid"`
	ModelID       *string `json:"model_id" binding:"omitempty,uuid"`
	RoomID        *string `json:"room_id" binding:"omitempty,uuid"`
	UnitID        *string `json:"unit_id" binding:"omitempty,uuid"`
	VendorID      *string `json:"vendor_id" binding:"omitempty,uuid"`
	SerialNumber  *string `json:"serial_number"`
	PONumber      *string `json:"po_number"`
	FundingSource *string `json:"funding_source"`
	// Dates as optional strings (YYYY-MM-DD).
	PurchaseDate   *string `json:"purchase_date"`
	WarrantyExpiry *string `json:"warranty_expiry"`
	Notes          *string `json:"notes"`
	// Legacy-parity fields (spec 2026-07-23).
	FloorID          *string `json:"floor_id" binding:"omitempty,uuid"`
	PICEmployeeID    *string `json:"pic_employee_id" binding:"omitempty,uuid"`
	Capacity         *string `json:"capacity"`
	LeaseDate        *string `json:"lease_date"`
	InstallationDate *string `json:"installation_date"`
	WarrantyStart    *string `json:"warranty_start"`
}

// toInput converts the request to an UpdateInput, parsing UUIDs and dates.
func (r AssetUpdateRequest) toInput() (UpdateInput, error) {
	catID, err := parseRequiredUUID(r.CategoryID)
	if err != nil {
		return UpdateInput{}, err
	}
	brandID, err := common.ParseUUIDPtr(r.BrandID)
	if err != nil {
		return UpdateInput{}, err
	}
	modelID, err := common.ParseUUIDPtr(r.ModelID)
	if err != nil {
		return UpdateInput{}, err
	}
	roomID, err := common.ParseUUIDPtr(r.RoomID)
	if err != nil {
		return UpdateInput{}, err
	}
	unitID, err := common.ParseUUIDPtr(r.UnitID)
	if err != nil {
		return UpdateInput{}, err
	}
	vendorID, err := common.ParseUUIDPtr(r.VendorID)
	if err != nil {
		return UpdateInput{}, err
	}
	purchaseDate, err := parseDate(r.PurchaseDate)
	if err != nil {
		return UpdateInput{}, err
	}
	warrantyExpiry, err := parseDate(r.WarrantyExpiry)
	if err != nil {
		return UpdateInput{}, err
	}
	floorID, err := common.ParseUUIDPtr(r.FloorID)
	if err != nil {
		return UpdateInput{}, err
	}
	picID, err := common.ParseUUIDPtr(r.PICEmployeeID)
	if err != nil {
		return UpdateInput{}, err
	}
	leaseDate, err := parseDate(r.LeaseDate)
	if err != nil {
		return UpdateInput{}, err
	}
	installationDate, err := parseDate(r.InstallationDate)
	if err != nil {
		return UpdateInput{}, err
	}
	warrantyStart, err := parseDate(r.WarrantyStart)
	if err != nil {
		return UpdateInput{}, err
	}
	return UpdateInput{
		Name:             r.Name,
		CategoryID:       catID,
		BrandID:          brandID,
		ModelID:          modelID,
		RoomID:           roomID,
		FloorID:          floorID,
		UnitID:           unitID,
		VendorID:         vendorID,
		SerialNumber:     r.SerialNumber,
		PONumber:         r.PONumber,
		FundingSource:    r.FundingSource,
		PurchaseDate:     purchaseDate,
		WarrantyExpiry:   warrantyExpiry,
		WarrantyStart:    warrantyStart,
		Capacity:         r.Capacity,
		LeaseDate:        leaseDate,
		InstallationDate: installationDate,
		PICEmployeeID:    picID,
		Notes:            r.Notes,
	}, nil
}

// parseRequiredUUID parses a mandatory UUID string.
func parseRequiredUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// parseDate parses an optional "YYYY-MM-DD" string into a pgtype.Date.
func parseDate(s *string) (pgtype.Date, error) {
	if s == nil || *s == "" {
		return pgtype.Date{}, nil
	}
	t, err := time.Parse(time.DateOnly, *s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}
