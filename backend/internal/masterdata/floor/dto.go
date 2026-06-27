package floor

import (
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Request is the create/update payload for a floor.
type Request struct {
	OfficeID string `json:"office_id" binding:"required,uuid"`
	Name     string `json:"name" binding:"required"`
	Level    *int32 `json:"level"`
}

// toInput resolves the request into a service CreateInput. OfficeID is guaranteed
// valid by the `uuid` binding tag.
func (r Request) toInput() CreateInput {
	return CreateInput{
		OfficeID: uuid.MustParse(r.OfficeID),
		Name:     r.Name,
		Level:    r.Level,
	}
}

// Response is the serialized floor.
type Response struct {
	ID        string  `json:"id"`
	OfficeID  string  `json:"office_id"`
	Name      string  `json:"name"`
	Level     *int32  `json:"level"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

func toResponse(f sqlc.MasterdataFloor) Response {
	return Response{
		ID:        f.ID.String(),
		OfficeID:  f.OfficeID.String(),
		Name:      f.Name,
		Level:     f.Level,
		CreatedAt: common.TsStr(f.CreatedAt),
		UpdatedAt: common.TsStr(f.UpdatedAt),
	}
}
