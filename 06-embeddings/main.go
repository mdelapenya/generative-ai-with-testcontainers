package main

import (
	"context"
	"fmt"
	"log"

	"github.com/chewxy/math32"
	"github.com/testcontainers/testcontainers-go"
	dmr "github.com/testcontainers/testcontainers-go/modules/dockermodelrunner"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms/openai"
)

const (
	modelNamespace = "ai"
	modelName      = "mxbai-embed-large"
	modelTag       = "335M-F16"
	fqModelName    = modelNamespace + "/" + modelName + ":" + modelTag
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("run: %s", err)
	}
}

func run() (err error) {
	dmrCtr, err := dmr.Run(context.Background(), dmr.WithModel(fqModelName), testcontainers.WithReuseByName("embeddings-model"))
	if err != nil {
		return err
	}
	defer func() {
		err = testcontainers.TerminateContainer(dmrCtr)
		if err != nil {
			err = fmt.Errorf("terminate container: %w", err)
		}
	}()

	opts := []openai.Option{
		openai.WithBaseURL(dmrCtr.OpenAIEndpoint()),
		openai.WithEmbeddingModel(fqModelName),
		openai.WithToken("foo"), // No API key needed for Model Runner
	}

	llm, err := openai.New(opts...)
	if err != nil {
		return fmt.Errorf("openai new: %w", err)
	}

	embedder, err := embeddings.NewEmbedder(llm)
	if err != nil {
		return fmt.Errorf("embedder new: %w", err)
	}

	docs := []string{
		"A cat is a small domesticated carnivorous mammal",
		"A tiger is a large carnivorous feline mammal",
		"Testcontainers is a Go package that supports JUnit tests, providing lightweight, throwaway instances of common databases, web browsers, or anything else that can run in a Docker container",
		"Docker is a platform designed to help developers build, share, and run container applications. We handle the tedious setup, so you can focus on the code.",
	}

	vecs, err := embedder.EmbedDocuments(context.Background(), docs)
	if err != nil {
		return fmt.Errorf("embed query: %w", err)
	}

	fmt.Println("Similarities:")
	for i := range docs {
		for j := range docs {
			fmt.Printf("%6s ~ %6s = %0.2f\n", docs[i], docs[j], cosineSimilarity(vecs[i], vecs[j]))
		}
	}

	return nil
}

// cosineSimilarity calculates the cosine similarity between two vectors.
// See https://github.com/tmc/langchaingo/blob/238d1c713de3ca983e8f6066af6b9080c9b0e088/examples/cybertron-embedding-example/cybertron-embedding.go#L19
func cosineSimilarity(x, y []float32) float32 {
	if len(x) != len(y) {
		log.Fatal("x and y have different lengths")
	}

	var dot, nx, ny float32

	for i := range x {
		nx += x[i] * x[i]
		ny += y[i] * y[i]
		dot += x[i] * y[i]
	}

	return dot / (math32.Sqrt(nx) * math32.Sqrt(ny))
}
