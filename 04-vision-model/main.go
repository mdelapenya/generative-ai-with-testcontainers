package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"

	"github.com/testcontainers/testcontainers-go"
	tcollama "github.com/testcontainers/testcontainers-go/modules/ollama"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
)

//go:embed images/cat.jpeg
var catImage []byte

func main() {
	if err := run(); err != nil {
		log.Fatalf("run: %s", err)
	}
}

func run() error {
	c, err := tcollama.Run(context.Background(), "mdelapenya/moondream:0.11.8-1.8b", testcontainers.WithReuseByName("vision-model"))
	if err != nil {
		return err
	}
	defer func() {
		err = testcontainers.TerminateContainer(c)
		if err != nil {
			err = fmt.Errorf("terminate container: %w", err)
		}
	}()

	ollamaURL, err := c.ConnectionString(context.Background())
	if err != nil {
		return fmt.Errorf("connection string: %w", err)
	}

	llm, err := ollama.New(ollama.WithModel("moondream:1.8b"), ollama.WithServerURL(ollamaURL))
	if err != nil {
		return fmt.Errorf("ollama new: %w", err)
	}

	var content []llms.MessageContent

	content = append(content, llms.MessageContent{
		Role: llms.ChatMessageTypeHuman,
		Parts: []llms.ContentPart{
			llms.TextPart("Please tell me what you see in this image"),
			llms.BinaryPart("image/jpg", catImage),
		},
	})

	ctx := context.Background()
	resp, err := llm.GenerateContent(ctx, content, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		fmt.Print(string(chunk))
		return nil
	}))
	if err != nil {
		return fmt.Errorf("llm generate content: %w", err)
	}

	for _, choice := range resp.Choices {
		fmt.Println(choice.Content)
	}

	return nil
}
