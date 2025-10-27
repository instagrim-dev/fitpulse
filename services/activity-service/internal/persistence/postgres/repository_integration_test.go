//go:build integration

package postgres

import (
    "context"
    "os"
    "path/filepath"
    "runtime"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/stretchr/testify/require"
    postgrescontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

    "example.com/activity/internal/domain"
)

func TestRepositoryRespectsTenantIsolation(t *testing.T) {
	ctx := context.Background()

    pg, err := postgrescontainer.RunContainer(ctx,
        postgrescontainer.WithDatabase("fitness"),
        postgrescontainer.WithUsername("platform"),
        postgrescontainer.WithPassword("platform"),
    )
    require.NoError(t, err)
    t.Cleanup(func() { _ = pg.Terminate(ctx) })

    connStr, err := pg.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)

    require.NoError(t, waitForDatabase(ctx, connStr))

	runMigrations(t, ctx, connStr)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	repo := NewRepository(pool)

	aggregate := domain.ActivityAggregate{
		ID:           uuid.NewString(),
		TenantID:     uuid.NewString(),
		UserID:       uuid.NewString(),
		ActivityType: "test",
		StartedAt:    time.Now().UTC(),
		DurationMin:  30,
		Source:       "integration-test",
		Version:      "v1",
		State:        domain.ActivityStatePending,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	err = repo.Create(ctx, aggregate, "key-1")
	require.NoError(t, err)

	stored, err := repo.Get(ctx, aggregate.TenantID, aggregate.ID)
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, aggregate.ID, stored.ID)

	otherTenant := uuid.NewString()
	storedOther, err := repo.Get(ctx, otherTenant, aggregate.ID)
	require.NoError(t, err)
	require.Nil(t, storedOther, "RLS should prevent cross-tenant access")
}

func runMigrations(t *testing.T, ctx context.Context, connStr string) {
	files := []string{
		"../../../../../db/postgres/migrations/0001_init.up.sql",
		"../../../../../db/postgres/migrations/0002_outbox_dlq_retry.up.sql",
	}

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	for _, rel := range files {
		path := resolvePath(t, rel)
		contents, readErr := os.ReadFile(path)
		require.NoError(t, readErr)

		_, execErr := pool.Exec(ctx, string(contents))
		require.NoError(t, execErr)
	}
}

func resolvePath(t *testing.T, rel string) string {
    t.Helper()
    _, file, _, ok := runtime.Caller(0)
    require.True(t, ok)
    return filepath.Join(filepath.Dir(file), rel)
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
