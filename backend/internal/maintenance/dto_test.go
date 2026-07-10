package maintenance

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

func TestMarshalReportPayload(t *testing.T) {
	desc := "layar pecah"
	att := "9f0d1c8e-0000-0000-0000-000000000001"
	b, err := marshalReportPayload("a-id", "p-id", &desc, &att)
	require.NoError(t, err)
	var p MaintenancePayload
	require.NoError(t, json.Unmarshal(b, &p))
	require.Equal(t, "a-id", p.AssetID)
	require.Equal(t, "p-id", p.ProblemCategoryID)
	require.Equal(t, "layar pecah", *p.Description)
	require.Equal(t, att, *p.AttachmentID)

	b, err = marshalReportPayload("a-id", "p-id", nil, nil)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(b, &p))
	require.Nil(t, p.Description)
	require.Nil(t, p.AttachmentID)
}

func TestValidTransition(t *testing.T) {
	sch, prog := sqlc.SharedMaintenanceStatusScheduled, sqlc.SharedMaintenanceStatusInProgress
	done, canc := sqlc.SharedMaintenanceStatusCompleted, sqlc.SharedMaintenanceStatusCancelled
	cases := []struct {
		from, to sqlc.SharedMaintenanceStatus
		ok       bool
	}{
		{sch, sch, true}, {sch, prog, true}, {sch, done, true}, {sch, canc, true},
		{prog, prog, true}, {prog, done, true}, {prog, canc, true}, {prog, sch, false},
		{done, prog, false}, {done, done, false}, {done, canc, false},
		{canc, sch, false}, {canc, canc, false},
	}
	for _, c := range cases {
		require.Equal(t, c.ok, validTransition(c.from, c.to), "%s -> %s", c.from, c.to)
	}
}
