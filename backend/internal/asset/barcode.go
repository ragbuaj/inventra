package asset

import (
	"context"

	sqlc "github.com/ragbuaj/inventra/db/sqlc"
)

// GetByTag fetches an asset by its unique asset_tag (for scan lookup).
func (s *Service) GetByTag(ctx context.Context, tag string) (sqlc.AssetAsset, error) {
	a, err := s.q.GetAssetByTag(ctx, tag)
	return a, mapDBError(err)
}
