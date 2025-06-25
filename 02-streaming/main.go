package main

import (
	"context"
	"fmt"
	"log"

	"github.com/testcontainers/testcontainers-go"
	dmr "github.com/testcontainers/testcontainers-go/modules/dockermodelrunner"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

const (
	modelNamespace = "ai"
	modelName      = "qwen3"
	modelTag       = "0.6B-Q4_0"
	fqModelName    = modelNamespace + "/" + modelName + ":" + modelTag
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("run: %s", err)
	}
}

func run() (err error) {
	dmrCtr, err := dmr.Run(context.Background(), dmr.WithModel(fqModelName), testcontainers.WithReuseByName("streaming-model"))
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
		openai.WithModel(fqModelName),
		openai.WithToken("foo"), // No API key needed for Model Runner
	}

	llm, err := openai.New(opts...)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	content := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, "Give me a detailed and long explanation of why Testcontainers for Go is great"),
	}

	// Streaming is needed because models are usually slow in responding, so showing progress is important.
	_, err = llm.GenerateContent(ctx, content, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		fmt.Print(string(chunk))
		return nil
	}))
	if err != nil {
		return fmt.Errorf("llm generate content: %w", err)
	}

	return nil
}
