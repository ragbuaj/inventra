//go:build integration

package testsupport

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ragbuaj/inventra/internal/storage"
)

// NewMinIO starts a MinIO container and returns a ready MinIOStorage (bucket "inventra-test"
// created). The container is terminated via t.Cleanup.
func NewMinIO(t *testing.T) *storage.MinIOStorage {
	t.Helper()
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:latest",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     "minioadmin",
			"MINIO_ROOT_PASSWORD": "minioadmin123",
		},
		Cmd: []string{"server", "/data"},
		// Wait for the S3 API to actually serve HTTP, not just for the TCP port to
		// open — MinIO starts listening before the API is ready, which otherwise
		// races the first bucket call into a "connection reset by peer" in CI.
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("9000/tcp"),
			wait.ForHTTP("/minio/health/live").WithPort("9000/tcp"),
		).WithStartupTimeoutDefault(60 * time.Second),
	}

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(ctr) })

	host, err := ctr.Host(ctx)
	require.NoError(t, err)
	port, err := ctr.MappedPort(ctx, "9000")
	require.NoError(t, err)
	endpoint := fmt.Sprintf("%s:%s", host, port.Port())

	store, err := storage.NewMinIOStorage(endpoint, "minioadmin", "minioadmin123", "inventra-test", false)
	require.NoError(t, err)
	require.NoError(t, store.EnsureBucket(ctx))
	return store
}
