package weaviate

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	tcweaviate "github.com/testcontainers/testcontainers-go/modules/weaviate"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/vectorstores/weaviate"
)

func NewStore(ctx context.Context, embedder embeddings.Embedder) (weaviate.Store, *tcweaviate.WeaviateContainer, error) {
	ctr, err := tcweaviate.Run(ctx, "semitechnologies/weaviate:1.27.2", testcontainers.WithReuseByName("weaviate-db"))
	if err != nil {
		return weaviate.Store{}, nil, fmt.Errorf("run weaviate container: %w", err)
	}

	schema, host, err := ctr.HttpHostAddress(ctx)
	if err != nil {
		return weaviate.Store{}, nil, fmt.Errorf("get weaviate container address: %w", err)
	}

	s, err := weaviate.New(
		weaviate.WithScheme(schema),
		weaviate.WithHost(host),
		weaviate.WithIndexName("Testcontainers"),
		weaviate.WithEmbedder(embedder),
	)

	return s, ctr, err
}
