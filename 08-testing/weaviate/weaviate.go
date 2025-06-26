package weaviate

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	tcweaviate "github.com/testcontainers/testcontainers-go/modules/weaviate"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/vectorstores/weaviate"
)

// NewStore creates a new Weaviate store. It will use a weaviate container to store the data.
func NewStore(ctx context.Context, embedder embeddings.Embedder) (weaviate.Store, error) {
	schema, host, err := mustGetAddress(ctx)
	if err != nil {
		return weaviate.Store{}, fmt.Errorf("run weaviate: %w", err)
	}

	return weaviate.New(
		weaviate.WithScheme(schema),
		weaviate.WithHost(host),
		weaviate.WithIndexName("Testcontainers"),
		weaviate.WithEmbedder(embedder),
	)
}

func mustGetAddress(ctx context.Context) (string, string, error) {
	c, err := tcweaviate.Run(ctx, "semitechnologies/weaviate:1.27.2", testcontainers.WithReuseByName("weaviate-db"))
	if err != nil {
		return "", "", fmt.Errorf("run container: %w", err)
	}

	schema, host, err := c.HttpHostAddress(ctx)
	if err != nil {
		return "", "", fmt.Errorf("weaviate container address: %w", err)
	}

	return schema, host, nil
}
