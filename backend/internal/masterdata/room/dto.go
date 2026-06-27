package room

import (
	"github.com/google/uuid"

	"github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// Request is the create/update payload for a room.
type Request struct {
	FloorID string  `json:"floor_id" binding:"required,uuid"`
	Name    string  `json:"name" binding:"required"`
	Code    *string `json:"code"`
}

// toInput resolves the request into a service CreateInput. FloorID is guaranteed
// valid by the `uuid` binding tag.
func (r Request) toInput() CreateInput {
	return CreateInput{
		FloorID: uuid.MustParse(r.FloorID),
		Name:    r.Name,
		Code:    r.Code,
	}
}

// Response is the serialized room.
type Response struct {
	ID        string  `json:"id"`
	FloorID   string  `json:"floor_id"`
	Name      string  `json:"name"`
	Code      *string `json:"code"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

func toResponse(r sqlc.MasterdataRoom) Response {
	return Response{
		ID:        r.ID.String(),
		FloorID:   r.FloorID.String(),
		Name:      r.Name,
		Code:      r.Code,
		CreatedAt: common.TsStr(r.CreatedAt),
		UpdatedAt: common.TsStr(r.UpdatedAt),
	}
}
