package pgvector

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/vectorstores/pgvector"
)

// NewStore creates a new PgVector store. It will use a Postgres container with the pgvector module to store the data.
func NewStore(ctx context.Context, embedder embeddings.Embedder) (pgvector.Store, error) {
	conn, err := mustGetConnection(ctx)
	if err != nil {
		return pgvector.Store{}, fmt.Errorf("pgvector container connection: %w", err)
	}

	return pgvector.New(
		ctx,
		pgvector.WithConnectionURL(conn),
		pgvector.WithEmbedder(embedder),
		pgvector.WithVectorDimensions(384),
		pgvector.WithCollectionName(`Testcontainers`),
		pgvector.WithCollectionTableName("tctable"),
	)
}

func mustGetConnection(ctx context.Context) (string, error) {
	c, err := tcpostgres.Run(ctx, "pgvector/pgvector:pg16",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
		tcpostgres.BasicWaitStrategies(),
		testcontainers.WithReuseByName("pgvector-db"),
	)
	if err != nil {
		return "", fmt.Errorf("run pgvector container: %w", err)
	}

	conn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return "", fmt.Errorf("get pgvector container connection string: %w", err)
	}

	return conn, nil
}
