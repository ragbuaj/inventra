package search

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/ragbuaj/inventra/db/sqlc"
)

func TestTooShort(t *testing.T) {
	assert.True(t, TooShort(""))
	assert.True(t, TooShort("  a  "))
	assert.False(t, TooShort("ab"))
	assert.False(t, TooShort(" ab "))
}

func TestAssetItem(t *testing.T) {
	id := uuid.New()
	it := assetItem(sqlc.SearchAssetsRow{ID: id, Name: "Laptop", AssetTag: "JKT01-X", Status: sqlc.SharedAssetStatusAvailable})
	assert.Equal(t, id.String(), it.ID)
	assert.Equal(t, "Laptop", it.Title)
	assert.Equal(t, "JKT01-X", it.Subtitle)
	assert.Equal(t, "available", *it.Status)
	assert.Equal(t, "JKT01-X", *it.AssetTag)
}

func TestRequestItem(t *testing.T) {
	id := uuid.New()
	off := "Cabang Jakarta"
	it := requestItem(sqlc.SearchRequestsRow{ID: id, Type: sqlc.SharedRequestTypeAssetCreate, Status: sqlc.SharedRequestStatusPending, OfficeName: &off})
	assert.Equal(t, "Cabang Jakarta", it.Title)
	assert.Equal(t, id.String()[:8], it.Subtitle)
	assert.Equal(t, "pending", *it.Status)
	assert.Equal(t, "asset_create", *it.RequestType)
}
