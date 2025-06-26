package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/mdelapenya/genai-testcontainers-go/functions/tools/pokemon"
	"github.com/testcontainers/testcontainers-go"
	dmr "github.com/testcontainers/testcontainers-go/modules/dockermodelrunner"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

const (
	modelNamespace = "ai"
	modelName      = "llama3.2"
	modelTag       = "3B-Q4_K_M"
	fqModelName    = modelNamespace + "/" + modelName + ":" + modelTag
)

var availableTools = []llms.Tool{
	{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name: "fetchPokeAPI",
			Description: `A wrapper around PokeAPI. 
			Useful for when you need to answer general questions about pokemon. 
			You must call this function separately for each pokemon you want information about.
			Input should be a single pokemon name in lowercase, without quotes.`,
			Parameters: json.RawMessage(`{
					"type": "object", 
					"properties": {
						"pokemon": {
							"type": "string",
							"description": "A single pokemon name in lowercase, without quotes. E.g. pikachu. When comparing multiple pokemon, call this function once for each pokemon."
						}
					}, 
					"required": ["pokemon"]
				}`),
		},
	},
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("run: %s", err)
	}
}

func run() (err error) {
	const question string = "I have two pokemons, Gengar and Haunter. Please fetch information for both Gengar and Haunter individually so you can compare their move counts."

	log.Printf("Question: %s", question)

	// 3b model version is required to use Tools.
	// See https://hub.docker.com/r/ai/llama3.2
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
	defer func() {
		terminateErr := testcontainers.TerminateContainer(dmrCtr)
		if terminateErr != nil {
			err = fmt.Errorf("terminate container: %w", terminateErr)
		}
	}()

	opts := []openai.Option{
		openai.WithBaseURL(dmrCtr.OpenAIEndpoint()),
		openai.WithModel(fqModelName),
		openai.WithToken("foo"), // No API key needed for Model Runner
	}

	llm, err := openai.New(opts...)
	if err != nil {
		return fmt.Errorf("openai.New: %w", err)
	}

	messageHistory := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, question),
		llms.TextParts(llms.ChatMessageTypeSystem, `You are a helpful Pokemon assistant. When asked to compare multiple Pokemon, you MUST:
1. Call fetchPokeAPI once for EACH Pokemon mentioned
2. Only after getting information for ALL Pokemon, provide your comparison
3. Never make assumptions - always get data for each Pokemon individually.

As an example, if the user asks for Gengar and Haunter, you must call fetchPokeAPI twice, once for Gengar and once for Haunter.
`),
	}

	ctx := context.Background()

	for retries := 3; retries > 0; retries = retries - 1 {
		resp, err := llm.GenerateContent(ctx, messageHistory,
			llms.WithTools(availableTools),
			llms.WithTemperature(0.1), // Lower temperature for more consistent behavior
			llms.WithTopP(0.9),        // Adjust for better function calling
		)
		if err != nil {
			return fmt.Errorf("generateContent (%d): %w", retries, err)
		}

		respchoice := resp.Choices[0]

		assistantResponse := llms.TextParts(llms.ChatMessageTypeAI, respchoice.Content)
		for _, tc := range respchoice.ToolCalls {
			assistantResponse.Parts = append(assistantResponse.Parts, tc)
		}
		messageHistory = append(messageHistory, assistantResponse)

		toolsResponse, err := executeToolCalls(ctx, messageHistory, resp)
		if err != nil {
			return fmt.Errorf("executeToolCalls (%d): %w", retries, err)
		}
		messageHistory = append(messageHistory, toolsResponse...)
	}

	messageHistory = append(messageHistory, llms.TextParts(llms.ChatMessageTypeHuman, "Can you compare the two?"))

	// Send query to the model again, this time with a history containing its
	// request to invoke a tool and our response to the tool call.
	resp, err := llm.GenerateContent(ctx, messageHistory, llms.WithTools(availableTools))
	if err != nil {
		return fmt.Errorf("generateContent: %w", err)
	}

	fmt.Println(resp.Choices[0].Content)

	return nil
}

// executeToolCalls executes the tool calls in the response and returns the
// updated message history.
func executeToolCalls(ctx context.Context, messageHistory []llms.MessageContent, resp *llms.ContentResponse) ([]llms.MessageContent, error) {
	fmt.Println("Executing", len(resp.Choices[0].ToolCalls), "tool calls")
	for _, toolCall := range resp.Choices[0].ToolCalls {
		switch toolCall.FunctionCall.Name {
		case "fetchPokeAPI":
			var args struct {
				Pokemon string `json:"pokemon"`
			}
			if err := json.Unmarshal([]byte(toolCall.FunctionCall.Arguments), &args); err != nil {
				log.Fatal("invalid input: ", err)
			}

			p, err := pokemon.FetchAPI(ctx, args.Pokemon)
			if err != nil {
				return nil, fmt.Errorf("fetchPokeAPI: %w", err)
			}

			pokeAPICallResponse := llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						ToolCallID: toolCall.ID,
						Name:       toolCall.FunctionCall.Name,
						Content:    p,
					},
				},
			}

			messageHistory = append(messageHistory, pokeAPICallResponse)

		default:
			return nil, fmt.Errorf("unsupported tool: %s", toolCall.FunctionCall.Name)
		}
	}

	return messageHistory, nil
}
