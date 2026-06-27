package approval

import (
	"context"
	"testing"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

type stubExec struct{ called bool }

func (s *stubExec) Execute(context.Context, *sqlc.Queries, sqlc.ApprovalRequest) error {
	s.called = true
	return nil
}

func TestRegistryLookup(t *testing.T) {
	r := registry{}
	e := &stubExec{}
	r[sqlc.SharedRequestTypeAssetCreate] = e
	if r[sqlc.SharedRequestTypeAssetCreate] == nil {
		t.Fatal("expected executor registered")
	}
}
