//go:build integration
// +build integration

package consumer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestPersistenceHandlerStoresEvent(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := setupPostgres(t, ctx)
	defer cleanup()

	handler := NewPersistenceHandler(pool)

	payload := json.RawMessage(`{"activity_id":"abc","tenant_id":"tenant-123"}`)
	msg := Message{
		EventType:     "activity.created",
		TenantID:      "tenant-123",
		SchemaID:      42,
		SchemaSubject: "activity_events-value",
		Topic:         "activity_events",
		Partition:     0,
		Offset:        5,
		Payload:       payload,
		Timestamp:     time.Now().UTC(),
	}

	require.NoError(t, handler.Handle(ctx, msg))

	var storedPayload []byte
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM activity_event_log`).Scan(&count))
	require.Equal(t, 1, count)
	err := pool.QueryRow(ctx, `SELECT payload FROM activity_event_log LIMIT 1`).Scan(&storedPayload)
	require.NoError(t, err)
	require.JSONEq(t, string(payload), string(storedPayload))
}

func setupPostgres(t *testing.T, ctx context.Context) (*pgxpool.Pool, func()) {
	t.Helper()

	pg, err := postgrescontainer.RunContainer(ctx,
		postgrescontainer.WithDatabase("fitness"),
		postgrescontainer.WithUsername("platform"),
		postgrescontainer.WithPassword("platform"),
	)
	require.NoError(t, err)

	connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	require.NoError(t, waitForDatabase(ctx, connStr))
	runMigrations(t, ctx, connStr)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	cleanup := func() {
		pool.Close()
		_ = pg.Terminate(ctx)
	}
	return pool, cleanup
}

func runMigrations(t *testing.T, ctx context.Context, connStr string) {
	t.Helper()

	migrationsPath := resolvePath(t, "../../../../db/postgres/migrations")
	files, err := filepath.Glob(filepath.Join(migrationsPath, "*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	sort.Strings(files)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	for _, file := range files {
		content, readErr := os.ReadFile(file)
		require.NoErrorf(t, readErr, "read migration %s", file)
		_, execErr := pool.Exec(ctx, string(content))
		require.NoErrorf(t, execErr, "execute migration %s", file)
	}
}

func waitForDatabase(ctx context.Context, connStr string) error {
	deadline := time.Now().Add(30 * time.Second)
	for {
		pool, err := pgxpool.New(ctx, connStr)
		if err == nil {
			err = pool.Ping(ctx)
			pool.Close()
			if err == nil {
				return nil
			}
		}
		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(time.Second)
	}
}

func resolvePath(t *testing.T, rel string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), rel)
}
