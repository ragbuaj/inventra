package guide

import (
	"encoding/json"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
	"github.com/ragbuaj/inventra/internal/masterdata/common"
)

// ---------- requests ----------

type stepRequest struct {
	TextID string  `json:"text_id" binding:"required,max=500"`
	TextEn *string `json:"text_en" binding:"omitempty,max=500"`
}

// moduleFields is everything an author writes on a module EXCEPT its status.
// Embedded by both request shapes so a field added here can never reach one
// endpoint and quietly miss the other.
type moduleFields struct {
	Slug      string        `json:"slug" binding:"required,max=120"`
	Icon      string        `json:"icon" binding:"required,max=80"`
	SortOrder int32         `json:"sort_order" binding:"gte=0"`
	TitleID   string        `json:"title_id" binding:"required,max=200"`
	TitleEn   *string       `json:"title_en" binding:"omitempty,max=200"`
	BodyID    *string       `json:"body_id" binding:"omitempty,max=2000"`
	BodyEn    *string       `json:"body_en" binding:"omitempty,max=2000"`
	Steps     []stepRequest `json:"steps" binding:"omitempty,max=50,dive"`
}

func (r moduleFields) toInput() ModuleInput {
	steps := make([]Step, 0, len(r.Steps))
	for _, s := range r.Steps {
		steps = append(steps, Step{TextID: s.TextID, TextEn: s.TextEn})
	}
	return ModuleInput{
		Slug:      r.Slug,
		Icon:      r.Icon,
		SortOrder: r.SortOrder,
		TitleID:   r.TitleID,
		TitleEn:   r.TitleEn,
		BodyID:    r.BodyID,
		BodyEn:    r.BodyEn,
		Steps:     steps,
	}
}

// moduleCreateRequest deliberately carries NO status field: a new module is
// always born a draft (spec business rule 5). A status sent by an older client
// is not bound and therefore cannot publish anything — the omission is the
// enforcement, not a check that could be forgotten. Service.Create pins the
// value again for any caller that does not come through HTTP.
type moduleCreateRequest struct {
	moduleFields
}

// moduleUpdateRequest is where status becomes writable: publishing and pulling
// back to draft are edits of an existing module, not part of creating one.
type moduleUpdateRequest struct {
	moduleFields
	Status string `json:"status" binding:"required,oneof=draft published"`
}

func (r moduleUpdateRequest) toInput() ModuleInput {
	in := r.moduleFields.toInput()
	in.Status = sqlc.SharedGuideStatus(r.Status)
	return in
}

type videoRequest struct {
	URL       string  `json:"url" binding:"required,max=500"`
	TitleID   string  `json:"title_id" binding:"required,max=200"`
	TitleEn   *string `json:"title_en" binding:"omitempty,max=200"`
	SortOrder int32   `json:"sort_order" binding:"gte=0"`
}

// documentRequest carries the non-file half of the multipart upload.
type documentRequest struct {
	TitleID   string  `form:"title_id" binding:"required,max=200"`
	TitleEn   *string `form:"title_en" binding:"omitempty,max=200"`
	SortOrder int32   `form:"sort_order" binding:"gte=0"`
}

type attachmentPatchRequest struct {
	TitleID   string  `json:"title_id" binding:"required,max=200"`
	TitleEn   *string `json:"title_en" binding:"omitempty,max=200"`
	SortOrder int32   `json:"sort_order" binding:"gte=0"`
}

// ---------- responses ----------

type stepResponse struct {
	TextID string  `json:"text_id"`
	TextEn *string `json:"text_en"`
}

// attachmentResponse is deliberately two-faced.
//
// For a caller WITHOUT a session it carries only what the public page needs to
// say "there is a video here, sign in to watch": kind, title, ordering, and
// locked=true. The video id, the file link, the filename, and the size are all
// omitted — a guest must not be able to reconstruct the media, nor learn the
// name of an internal document.
type attachmentResponse struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	TitleID   string  `json:"title_id"`
	TitleEn   *string `json:"title_en"`
	SortOrder int32   `json:"sort_order"`
	Locked    bool    `json:"locked"`

	YoutubeID        *string `json:"youtube_id,omitempty"`
	FileURL          *string `json:"file_url,omitempty"`
	OriginalFilename *string `json:"original_filename,omitempty"`
	SizeBytes        *int64  `json:"size_bytes,omitempty"`
}

type moduleResponse struct {
	ID          string               `json:"id"`
	Slug        string               `json:"slug"`
	Icon        string               `json:"icon"`
	SortOrder   int32                `json:"sort_order"`
	Status      string               `json:"status"`
	TitleID     string               `json:"title_id"`
	TitleEn     *string              `json:"title_en"`
	BodyID      *string              `json:"body_id"`
	BodyEn      *string              `json:"body_en"`
	Steps       []stepResponse       `json:"steps"`
	Attachments []attachmentResponse `json:"attachments"`
	PublishedAt *string              `json:"published_at"`
	UpdatedAt   *string              `json:"updated_at"`
}

// toAttachmentResponse projects one attachment for the given audience.
// signedIn is the ONLY gate: any authenticated user may consume attachments;
// guide.manage governs writing, not reading.
func toAttachmentResponse(a sqlc.GuideGuideAttachment, signedIn bool) attachmentResponse {
	out := attachmentResponse{
		ID:        a.ID.String(),
		Kind:      string(a.Kind),
		TitleID:   a.TitleID,
		TitleEn:   a.TitleEn,
		SortOrder: a.SortOrder,
		Locked:    !signedIn,
	}
	if !signedIn {
		return out
	}
	switch a.Kind {
	case sqlc.SharedGuideAttachmentKindVideo:
		out.YoutubeID = a.YoutubeID
	case sqlc.SharedGuideAttachmentKindDocument:
		url := "/guide/attachments/" + a.ID.String() + "/content"
		out.FileURL = &url
		out.OriginalFilename = a.OriginalFilename
		out.SizeBytes = a.SizeBytes
	}
	return out
}

func toModuleResponse(m ModuleWithAttachments, signedIn bool) moduleResponse {
	atts := make([]attachmentResponse, 0, len(m.Attachments))
	for _, a := range m.Attachments {
		atts = append(atts, toAttachmentResponse(a, signedIn))
	}
	return moduleResponse{
		ID:          m.Module.ID.String(),
		Slug:        m.Module.Slug,
		Icon:        m.Module.Icon,
		SortOrder:   m.Module.SortOrder,
		Status:      string(m.Module.Status),
		TitleID:     m.Module.TitleID,
		TitleEn:     m.Module.TitleEn,
		BodyID:      m.Module.BodyID,
		BodyEn:      m.Module.BodyEn,
		Steps:       decodeSteps(m.Module.Steps),
		Attachments: atts,
		PublishedAt: common.TsStr(m.Module.PublishedAt),
		UpdatedAt:   common.TsStr(m.Module.UpdatedAt),
	}
}

// moduleOnlyResponse serializes a bare module row (write endpoints, which have
// no attachments to report yet).
func moduleOnlyResponse(m sqlc.GuideGuideModule) moduleResponse {
	return toModuleResponse(ModuleWithAttachments{Module: m}, true)
}

// decodeSteps turns the stored jsonb into the response shape. A malformed value
// cannot normally exist (the service re-encodes what it validates, and a CHECK
// constraint keeps the outer type an array), so a decode failure degrades to an
// empty list rather than failing the whole page.
func decodeSteps(raw []byte) []stepResponse {
	steps := []stepResponse{}
	if len(raw) == 0 {
		return steps
	}
	var parsed []Step
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return steps
	}
	for _, s := range parsed {
		steps = append(steps, stepResponse{TextID: s.TextID, TextEn: s.TextEn})
	}
	return steps
}
