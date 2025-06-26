package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/testcontainers/testcontainers-go"
	dmr "github.com/testcontainers/testcontainers-go/modules/dockermodelrunner"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

const (
	modelRegistry  = "hf.co" // Hugginface model registry
	modelNamespace = "bartowski"
	modelName      = "Llama-3.2-1B-Instruct-GGUF"
	modelTag       = "Q4_K_M"
	fqModelName    = modelRegistry + "/" + modelNamespace + "/" + modelName + ":" + modelTag
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("run: %s", err)
	}
}

func run() (err error) {
	// Huggingface needs a lower case model name
	sanitisedFqModelName := strings.ToLower(fqModelName)

	dmrCtr, err := dmr.Run(context.Background(), dmr.WithModel(sanitisedFqModelName), testcontainers.WithReuseByName("hugginface-model"))
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
		openai.WithModel(sanitisedFqModelName),
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
	completion, generateContentErr := llm.GenerateContent(ctx, content, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		fmt.Print(string(chunk))
		return nil
	}))
	if generateContentErr != nil {
		err = fmt.Errorf("llm generate content: %w", generateContentErr)
		return
	}

	for _, choice := range completion.Choices {
		fmt.Println(choice.Content)
	}

	return nil
}
