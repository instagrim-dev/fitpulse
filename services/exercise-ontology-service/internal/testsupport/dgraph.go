//go:build integration

package testsupport

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var exerciseSchema string

func init() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		panic("unable to resolve testsupport filename for schema loading")
	}
	schemaPath := filepath.Join(filepath.Dir(filename), "../../../../db/dgraph/schema/exercise.schema")
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		panic(fmt.Sprintf("load dgraph schema: %v", err))
	}
	exerciseSchema = string(data)
}

// StartDgraph launches a standalone Dgraph container, waits for it to report healthy,
// applies the exercise ontology schema, and returns the running container plus the HTTP endpoint.
func StartDgraph(ctx context.Context, t *testing.T) (testcontainers.Container, string) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "dgraph/standalone:v23.1.0",
		ExposedPorts: []string{"8080/tcp"},
		WaitingFor: wait.ForHTTP("/health").
			WithPort("8080/tcp").
			WithStatusCodeMatcher(func(status int) bool { return status >= 200 && status < 500 }),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	mappedPort, err := container.MappedPort(ctx, "8080/tcp")
	require.NoError(t, err)

	endpoint := fmt.Sprintf("http://%s:%s", host, mappedPort.Port())
	applySchema(ctx, t, endpoint)

	return container, endpoint
}

func applySchema(ctx context.Context, t *testing.T, endpoint string) {
	t.Helper()

	client := &http.Client{Timeout: 5 * time.Second}
	require.Eventually(t, func() bool {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/alter", strings.NewReader(exerciseSchema))
		if err != nil {
			return false
		}
		req.Header.Set("Content-Type", "application/dql")

		resp, err := client.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		return resp.StatusCode < 300
	}, 30*time.Second, time.Second, "dgraph schema failed to apply")
}
