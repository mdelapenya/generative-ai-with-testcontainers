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
	modelName      = "llama3.2"
	modelTag       = "1B-Q4_0"
	fqModelName    = modelNamespace + "/" + modelName + ":" + modelTag
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("run: %s", err)
	}
}

func run() (err error) {
	dmrCtr, err := dmr.Run(context.Background(), dmr.WithModel(fqModelName), testcontainers.WithReuseByName("chat-model"))
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
		return fmt.Errorf("openai new: %w", err)
	}
	ctx := context.Background()
	content := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, "You are a fellow Go developer."),
		llms.TextParts(llms.ChatMessageTypeHuman, "Provide 3 short bullet points explaining why Go is awesome"),
	}

	// The response from the model happens when the model finishes processing the input, which it's usually slow.
	completion, err := llm.GenerateContent(ctx, content)
	if err != nil {
		return fmt.Errorf("llm generate content: %w", err)
	}

	for _, choice := range completion.Choices {
		fmt.Println(choice.Content)
	}

	return nil
}
