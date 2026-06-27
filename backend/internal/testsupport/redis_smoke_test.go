//go:build integration

package testsupport_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ragbuaj/inventra/internal/testsupport"
)

func TestNewRedisPingsAndStores(t *testing.T) {
	client := testsupport.NewRedis(t)
	ctx := context.Background()

	require.NoError(t, client.Set(ctx, "k", "v", 0).Err())
	got, err := client.Get(ctx, "k").Result()
	require.NoError(t, err)
	assert.Equal(t, "v", got)
}
