package main

import (
	"context"
	"embed"
	"fmt"
	"log"

	"github.com/mdelapenya/genai-testcontainers-go/testing/ai"
	"github.com/testcontainers/testcontainers-go"
	dmr "github.com/testcontainers/testcontainers-go/modules/dockermodelrunner"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/vectorstores"
)

const (
	question              string = "How I can enable verbose logging in Testcontainers Desktop?"
	modelNamespace               = "ai"
	embeddingsModelName          = "mxbai-embed-large"
	embeddingsModelTag           = "335M-F16"
	fqEmbeddingsModelName        = modelNamespace + "/" + embeddingsModelName + ":" + embeddingsModelTag
	modelName                    = "llama3.2"
	modelTag                     = "1B-Q4_0"
	fqModelName                  = modelNamespace + "/" + modelName + ":" + modelTag
)

//go:embed knowledge
var knowledge embed.FS

func main() {
	log.Println(question)
	if err := run(); err != nil {
		log.Fatalf("run: %s", err)
	}
}

func run() error {
	chatModel, chatCtr, err := buildChatModel()
	if err != nil {
		return fmt.Errorf("build chat model: %s", err)
	}
	defer func() {
		err = testcontainers.TerminateContainer(chatCtr)
		if err != nil {
			err = fmt.Errorf("terminate container: %w", err)
		}
	}()

	resp, err := straightAnswer(chatModel)
	if err != nil {
		log.Fatalf("straight chat: %s", err)
	}
	fmt.Println(">> Straight answer:\n", resp)

	resp, embeddingsCtr, err := raggedAnswer(chatModel)
	if err != nil {
		return fmt.Errorf("ragged chat: %s", err)
	}
	defer func() {
		err = testcontainers.TerminateContainer(embeddingsCtr)
		if err != nil {
			err = fmt.Errorf("terminate container: %w", err)
		}
	}()
	fmt.Println(">> Ragged answer:\n", resp)

	return nil
}

func straightAnswer(chatModel *openai.LLM) (string, error) {
	chatter := ai.NewChat(chatModel)

	return chatter.Chat(question)
}

func raggedAnswer(chatModel *openai.LLM) (string, *dmr.Container, error) {
	chatter, embeddingsCtr, err := buildRaggedChat(chatModel)
	if err != nil {
		return "", embeddingsCtr, fmt.Errorf("build ragged chat: %s", err)
	}

	s, err := chatter.Chat(question)
	if err != nil {
		return "", embeddingsCtr, fmt.Errorf("chat: %s", err)
	}

	return s, embeddingsCtr, nil
}

func buildRaggedChat(chatModel llms.Model) (ai.Chatter, *dmr.Container, error) {
	embeddingModel, embeddingsCtr, err := buildEmbeddingModel()
	if err != nil {
		return nil, embeddingsCtr, fmt.Errorf("build embedding model: %w", err)
	}

	embedder, err := embeddings.NewEmbedder(embeddingModel)
	if err != nil {
		return nil, embeddingsCtr, fmt.Errorf("new embedder: %w", err)
	}

	store, err := selectStore(context.Background(), embedder)
	if err != nil {
		return nil, embeddingsCtr, fmt.Errorf("new store: %w", err)
	}

	if err := ingestion(store); err != nil {
		return nil, embeddingsCtr, fmt.Errorf("ingestion: %w", err)
	}

	// Enrich the response with the relevant documents after the ingestion
	optionsVector := []vectorstores.Option{
		vectorstores.WithScoreThreshold(0.60), // use for precision, when you want to get only the most relevant documents
		//vectorstores.WithNameSpace("default"),            // use for set a namespace in the storage
		//vectorstores.WithFilters(map[string]interface{}{"language": "en"}), // use for filter the documents
		vectorstores.WithEmbedder(embedder), // use when you want add documents or doing similarity search
		//vectorstores.WithDeduplicater(vectorstores.NewSimpleDeduplicater()), //  This is useful to prevent wasting time on creating an embedding
	}

	maxResults := 3 // Number of relevant documents to return

	relevantDocs, err := store.SimilaritySearch(context.Background(), "cloud.logs.verbose", maxResults, optionsVector...)
	if err != nil {
		return nil, embeddingsCtr, fmt.Errorf("similarity search: %w", err)
	}
	log.Printf("Relevant documents for RAG: %d\n", len(relevantDocs))

	return ai.NewChat(chatModel, ai.WithRAGContext(relevantDocs)), embeddingsCtr, nil
}
