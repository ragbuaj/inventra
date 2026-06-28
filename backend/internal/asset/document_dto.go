package asset

import (
	"github.com/google/uuid"
	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// DocumentCreateRequest is the JSON body for creating an asset document (metadata only;
// the file is attached separately via the file sub-resource).
type DocumentCreateRequest struct {
	DocType          string  `json:"doc_type" binding:"required,oneof=bast_acquisition bast_transfer bast_disposal invoice contract other"`
	DocNo            *string `json:"doc_no"`
	DocDate          *string `json:"doc_date"`
	Counterparty     *string `json:"counterparty"`
	RelatedRequestID *string `json:"related_request_id" binding:"omitempty,uuid"`
}

// DocumentUpdateRequest is the JSON body for editing document metadata.
type DocumentUpdateRequest struct {
	DocType          string  `json:"doc_type" binding:"required,oneof=bast_acquisition bast_transfer bast_disposal invoice contract other"`
	DocNo            *string `json:"doc_no"`
	DocDate          *string `json:"doc_date"`
	Counterparty     *string `json:"counterparty"`
	RelatedRequestID *string `json:"related_request_id" binding:"omitempty,uuid"`
}

// toInput parses the create request into a DocumentInput (defined in document.go).
func (r DocumentCreateRequest) toInput(assetID, createdBy uuid.UUID) (DocumentInput, error) {
	date, err := parseDate(r.DocDate)
	if err != nil {
		return DocumentInput{}, err
	}
	reqID, err := common.ParseUUIDPtr(r.RelatedRequestID)
	if err != nil {
		return DocumentInput{}, err
	}
	return DocumentInput{
		AssetID:          assetID,
		DocType:          sqlc.SharedAssetDocumentType(r.DocType),
		DocNo:            r.DocNo,
		DocDate:          date,
		Counterparty:     r.Counterparty,
		RelatedRequestID: reqID,
		CreatedBy:        createdBy,
	}, nil
}

// toUpdateInput parses the update request into a DocumentUpdateInput (defined in document.go).
func (r DocumentUpdateRequest) toUpdateInput() (DocumentUpdateInput, error) {
	date, err := parseDate(r.DocDate)
	if err != nil {
		return DocumentUpdateInput{}, err
	}
	reqID, err := common.ParseUUIDPtr(r.RelatedRequestID)
	if err != nil {
		return DocumentUpdateInput{}, err
	}
	return DocumentUpdateInput{
		DocType:          sqlc.SharedAssetDocumentType(r.DocType),
		DocNo:            r.DocNo,
		DocDate:          date,
		Counterparty:     r.Counterparty,
		RelatedRequestID: reqID,
	}, nil
}

// documentToMap serializes a document for the API response. object_key is intentionally
// omitted (storage-internal); has_file is derived so callers can show a download affordance.
func documentToMap(d sqlc.AssetAssetDocument) map[string]any {
	return map[string]any{
		"id":                  d.ID.String(),
		"asset_id":            d.AssetID.String(),
		"doc_type":            string(d.DocType),
		"doc_no":              d.DocNo,
		"doc_date":            dateStr(d.DocDate),
		"counterparty":        d.Counterparty,
		"related_request_id":  common.UUIDPtrStr(d.RelatedRequestID),
		"related_transfer_id": common.UUIDPtrStr(d.RelatedTransferID),
		"related_disposal_id": common.UUIDPtrStr(d.RelatedDisposalID),
		"has_file":            d.ObjectKey != nil,
		"created_by_id":       common.UUIDPtrStr(d.CreatedByID),
		"created_at":          common.TsStr(d.CreatedAt),
		"updated_at":          common.TsStr(d.UpdatedAt),
	}
}
