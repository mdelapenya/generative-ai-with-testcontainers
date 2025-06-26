package main

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	dmr "github.com/testcontainers/testcontainers-go/modules/dockermodelrunner"
	"github.com/tmc/langchaingo/llms/openai"
)

func buildChatModel() (llm *openai.LLM, dmrCtr *dmr.Container, err error) {
	dmrCtr, err = dmr.Run(context.Background(), dmr.WithModel(fqModelName), testcontainers.WithReuseByName("chat-model"))
	if err != nil {
		return nil, dmrCtr, err
	}

	opts := []openai.Option{
		openai.WithBaseURL(dmrCtr.OpenAIEndpoint()),
		openai.WithModel(fqModelName),
		openai.WithToken("foo"), // No API key needed for Model Runner
		openai.WithResponseFormat(openai.ResponseFormatJSON),
	}

	llm, err = openai.New(opts...)
	if err != nil {
		return nil, dmrCtr, fmt.Errorf("openai new: %w", err)
	}

	return llm, dmrCtr, nil
}

func buildEmbeddingModel() (llm *openai.LLM, dmrCtr *dmr.Container, err error) {
	dmrCtr, err = dmr.Run(context.Background(), dmr.WithModel(fqEmbeddingsModelName), testcontainers.WithReuseByName("embeddings-model"))
	if err != nil {
		return nil, dmrCtr, err
	}

	opts := []openai.Option{
		openai.WithBaseURL(dmrCtr.OpenAIEndpoint()),
		openai.WithEmbeddingModel(fqEmbeddingsModelName),
		openai.WithToken("foo"), // No API key needed for Model Runner
	}

	llm, err = openai.New(opts...)
	if err != nil {
		return nil, dmrCtr, fmt.Errorf("openai new: %w", err)
	}

	return llm, dmrCtr, nil
}
