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
	dmrCtr, err := dmr.Run(context.Background(), dmr.WithModel(fqModelName), testcontainers.WithReuseByName("augmented-model"))
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

	originalMessage := `
		What is the current topic of the conference?
	`

	augmentedMessage := fmt.Sprintf(`
		%s

		Use the following bullet points to answer the question:
		- The Conference is about how to leverage Testcontainers for building Generative AI applications.
		- The meeting will explore how Testcontainers can be used to create a seamless development environment for AI projects.

		Do not indicate that you have been given any additional information.
		`, originalMessage)

	ctx := context.Background()
	originalContent := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, originalMessage),
	}

	originalCompletion, err := llm.GenerateContent(
		ctx, originalContent,
		llms.WithTemperature(0.0001),
		llms.WithTopK(1),
	)
	if err != nil {
		return fmt.Errorf("llm generate original content: %w", err)
	}

	fmt.Println("\nOriginal completion:")
	for _, choice := range originalCompletion.Choices {
		fmt.Println(choice.Content)
	}

	augmentedContent := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeSystem, augmentedMessage),
	}

	augmentedCompletion, err := llm.GenerateContent(
		ctx, augmentedContent,
		llms.WithTemperature(0.0001),
		llms.WithTopK(1),
	)
	if err != nil {
		return fmt.Errorf("llm generate original content: %w", err)
	}

	fmt.Println("\nAugmented completion:")
	for _, choice := range augmentedCompletion.Choices {
		fmt.Println(choice.Content)
	}

	return nil
}
