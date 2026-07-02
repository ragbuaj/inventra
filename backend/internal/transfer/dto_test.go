package transfer

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalPayload_RoundTrip(t *testing.T) {
	from, to, room := uuid.New(), uuid.New(), uuid.New()
	reason := "relokasi cabang"
	raw, err := marshalPayload(from, to, &room, &reason)
	require.NoError(t, err)

	var p TransferPayload
	require.NoError(t, json.Unmarshal(raw, &p))
	assert.Equal(t, from.String(), p.FromOfficeID)
	assert.Equal(t, to.String(), p.ToOfficeID)
	require.NotNil(t, p.ToRoomID)
	assert.Equal(t, room.String(), *p.ToRoomID)
	assert.Equal(t, "relokasi cabang", *p.Reason)
}

func TestMarshalPayload_NilRoom(t *testing.T) {
	raw, err := marshalPayload(uuid.New(), uuid.New(), nil, nil)
	require.NoError(t, err)
	var p TransferPayload
	require.NoError(t, json.Unmarshal(raw, &p))
	assert.Nil(t, p.ToRoomID)
	assert.Nil(t, p.Reason)
}
