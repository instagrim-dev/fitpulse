//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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
		"../../../../../db/postgres/migrations/0003_activity_event_log.up.sql",
		"../../../../../db/postgres/migrations/0004_identity_tokens.up.sql",
	}

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	defer pool.Close()

	for _, rel := range files {
		path := resolvePath(t, rel)
		contents, readErr := os.ReadFile(path)
		require.NoError(t, readErr)

		for _, stmt := range splitStatements(string(contents)) {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			_, execErr := pool.Exec(ctx, stmt)
			if execErr != nil {
				fmt.Printf("migration failure path=%s type=%T err=%v stmt=%s\n", path, execErr, execErr, stmt)
				var pgErr *pgconn.PgError
				if errors.As(execErr, &pgErr) {
					require.NoErrorf(t, execErr, "executing migration %s (sqlstate=%s message=%s detail=%s position=%s, hint=%s, where=%s)", path, pgErr.SQLState(), pgErr.Message, pgErr.Detail, pgErr.Position, pgErr.Hint, pgErr.Where)
				}
				require.NoErrorf(t, execErr, "executing migration %s (type=%T)", path, execErr)
			}
		}
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

func splitStatements(script string) []string {
	var (
		statements []string
		builder    strings.Builder
		inComment  bool
	)
	for i := 0; i < len(script); i++ {
		if !inComment && i+1 < len(script) && script[i] == '-' && script[i+1] == '-' {
			inComment = true
		}
		if script[i] == '\n' {
			inComment = false
		}
		if inComment {
			continue
		}
		if script[i] == ';' {
			statements = append(statements, builder.String())
			builder.Reset()
			continue
		}
		builder.WriteByte(script[i])
	}
	if strings.TrimSpace(builder.String()) != "" {
		statements = append(statements, builder.String())
	}
	return statements
}
