package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

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

	// listen for interrupt signals to end the chat session gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nInterrupt signal received, ending chat session")
		os.Exit(0)
	}()

	var conversation []llms.MessageContent

	reader := bufio.NewReader(os.Stdin)
	// Enter a conversation loop
	for {
		fmt.Print("\nYou: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read string: %w", err)
		}

		input = strings.TrimSpace(input)
		switch input {
		case "quit", "exit":
			fmt.Println("Ending chat session")
			os.Exit(0)
		}

		conversation = append(conversation, llms.TextParts(llms.ChatMessageTypeHuman, input))

		ctx := context.Background()
		_, err = llm.GenerateContent(ctx, conversation, llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			fmt.Print(string(chunk))
			return nil
		}))
		if err != nil {
			return fmt.Errorf("llm generate content: %w", err)
		}
	}
}
