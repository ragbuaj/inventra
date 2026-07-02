package disposal

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalPayload_RoundTrip(t *testing.T) {
	proceeds, book := "120000000.00", "100000000.00"
	raw, err := marshalPayload(SubmitInput{
		AssetID: uuid.New(), Method: "sale", DisposalDate: "2026-07-01",
		Proceeds: &proceeds, BookValue: &book,
	})
	require.NoError(t, err)
	var p DisposalPayload
	require.NoError(t, json.Unmarshal(raw, &p))
	assert.Equal(t, "sale", p.Method)
	assert.Equal(t, "2026-07-01", p.DisposalDate)
	require.NotNil(t, p.Proceeds)
	assert.Equal(t, "120000000.00", *p.Proceeds)
	require.NotNil(t, p.BookValue)
	assert.Equal(t, "100000000.00", *p.BookValue)
}

func TestMarshalPayload_NilOptionals(t *testing.T) {
	raw, err := marshalPayload(SubmitInput{AssetID: uuid.New(), Method: "write_off", DisposalDate: "2026-07-01"})
	require.NoError(t, err)
	var p DisposalPayload
	require.NoError(t, json.Unmarshal(raw, &p))
	assert.Nil(t, p.Proceeds)
	assert.Nil(t, p.BookValue)
	assert.Nil(t, p.BastNo)
}
