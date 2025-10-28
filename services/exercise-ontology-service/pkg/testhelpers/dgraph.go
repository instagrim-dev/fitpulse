package testhelpers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// StartDgraph launches a standalone Dgraph container and applies the exercise schema.
func StartDgraph(ctx context.Context) (testcontainers.Container, string, error) {
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
	if err != nil {
		return nil, "", err
	}

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(context.Background())
		return nil, "", err
	}
	port, err := container.MappedPort(ctx, "8080/tcp")
	if err != nil {
		container.Terminate(context.Background())
		return nil, "", err
	}

	endpoint := "http://" + host + ":" + port.Port()
	schemaContents, err := loadSchema()
	if err != nil {
		container.Terminate(context.Background())
		return nil, "", err
	}

	if err := applySchema(ctx, endpoint, schemaContents); err != nil {
		container.Terminate(context.Background())
		return nil, "", err
	}

	return container, endpoint, nil
}

func applySchema(ctx context.Context, endpoint string, schema string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/alter", strings.NewReader(schema))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/dql")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("dgraph schema load failed: %s", resp.Status)
	}
	return nil
}

// loadSchema reads the canonical exercise schema used to initialise Dgraph instances.
func loadSchema() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("unable to resolve schema path")
	}
	path := filepath.Join(filepath.Dir(filename), "../../../../db/dgraph/schema/exercise.schema")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
