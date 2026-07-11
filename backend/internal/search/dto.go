package search

import (
	"github.com/ragbuaj/inventra/db/sqlc"
)

// Item is the uniform read-model row for one search hit.
type Item struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Subtitle    string  `json:"subtitle"`
	Status      *string `json:"status"`
	AssetTag    *string `json:"asset_tag,omitempty"`
	RequestType *string `json:"request_type,omitempty"`
}

// Group is one entity bucket in the response, capped at PerGroupLimit items.
type Group struct {
	Type  string `json:"type"`
	Total int64  `json:"total"`
	Items []Item `json:"items"`
}

func strPtr(s string) *string { return &s }

func assetItem(r sqlc.SearchAssetsRow) Item {
	return Item{
		ID:       r.ID.String(),
		Title:    r.Name,
		Subtitle: r.AssetTag,
		Status:   strPtr(string(r.Status)),
		AssetTag: strPtr(r.AssetTag),
	}
}

func employeeItem(r sqlc.SearchEmployeesRow) Item {
	return Item{ID: r.ID.String(), Title: r.Name, Subtitle: r.Code}
}

func officeItem(r sqlc.SearchOfficesRow) Item {
	return Item{ID: r.ID.String(), Title: r.Name, Subtitle: r.Code}
}

func userItem(r sqlc.SearchUsersRow) Item {
	return Item{ID: r.ID.String(), Title: r.Name, Subtitle: r.Email}
}

// requestItem: requests have no title column — Title carries the office name
// (may be empty); the frontend composes "type · office" via i18n.
func requestItem(r sqlc.SearchRequestsRow) Item {
	title := ""
	if r.OfficeName != nil {
		title = *r.OfficeName
	}
	return Item{
		ID:          r.ID.String(),
		Title:       title,
		Subtitle:    r.ID.String()[:8],
		Status:      strPtr(string(r.Status)),
		RequestType: strPtr(string(r.Type)),
	}
}
