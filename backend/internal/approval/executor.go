package approval

import (
	"context"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// Executor performs the real side effect of an approved request, inside the approval-commit tx.
type Executor interface {
	Execute(ctx context.Context, qtx *sqlc.Queries, req sqlc.ApprovalRequest) error
}

type registry map[sqlc.SharedRequestType]Executor
