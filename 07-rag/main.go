package main

import (
	"context"
	"fmt"
	"log"

	"github.com/testcontainers/testcontainers-go"
	dmr "github.com/testcontainers/testcontainers-go/modules/dockermodelrunner"
	tcweaviate "github.com/testcontainers/testcontainers-go/modules/weaviate"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/vectorstores"

	"github.com/mdelapenya/genai-testcontainers-go/rag/weaviate"
)

const (
	modelNamespace        = "ai"
	embeddingsModelName   = "mxbai-embed-large"
	embeddingsModelTag    = "335M-F16"
	fqEmbeddingsModelName = modelNamespace + "/" + embeddingsModelName + ":" + embeddingsModelTag
	modelName             = "llama3.2"
	modelTag              = "1B-Q4_0"
	fqModelName           = modelNamespace + "/" + modelName + ":" + modelTag
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("run: %s", err)
	}
}

func run() error {
	embeddingLLM, embeddingsCtr, err := buildEmbeddingModel()
	if err != nil {
		return fmt.Errorf("build embedding model: %w", err)
	}
	defer func() {
		err = testcontainers.TerminateContainer(embeddingsCtr)
		if err != nil {
			err = fmt.Errorf("terminate container: %w", err)
		}
	}()

	embedder, err := embeddings.NewEmbedder(embeddingLLM)
	if err != nil {
		return fmt.Errorf("new embedder: %w", err)
	}

	store, weaviateCtr, err := buildEmbeddingStore(embedder)
	if err != nil {
		return fmt.Errorf("build embedding store: %w", err)
	}
	defer func() {
		err = testcontainers.TerminateContainer(weaviateCtr)
		if err != nil {
			err = fmt.Errorf("terminate container: %w", err)
		}
	}()

	if err := ingestion(store); err != nil {
		return fmt.Errorf("ingestion: %w", err)
	}

	optionsVector := []vectorstores.Option{
		vectorstores.WithScoreThreshold(0.80), // use for precision, when you want to get only the most relevant documents
		//vectorstores.WithNameSpace(""),            // use for set a namespace in the storage
		//vectorstores.WithFilters(map[string]interface{}{"language": "en"}), // use for filter the documents
		vectorstores.WithEmbedder(embedder), // use when you want add documents or doing similarity search
		//vectorstores.WithDeduplicater(vectorstores.NewSimpleDeduplicater()), //  This is useful to prevent wasting time on creating an embedding
	}

	relevantDocs, err := store.SimilaritySearch(context.Background(), "What is my favorite sport?", 1, optionsVector...)
	if err != nil {
		return fmt.Errorf("similarity search: %w", err)
	}

	if len(relevantDocs) == 0 {
		fmt.Println("No relevant content found")
		return nil
	}

	chatLLM, chatCtr, err := buildChatModel()
	if err != nil {
		return fmt.Errorf("build chat model: %w", err)
	}
	defer func() {
		err = testcontainers.TerminateContainer(chatCtr)
		if err != nil {
			err = fmt.Errorf("terminate container: %w", err)
		}
	}()

	response := fmt.Sprintf(`
What is your favourite sport?

Answer the question considering the following relevant content:
%s
`, relevantDocs[0].PageContent)

	fmt.Println(response)

	ctx := context.Background()
	originalContent := []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, response),
	}

	_, err = chatLLM.GenerateContent(
		ctx, originalContent,
		llms.WithTemperature(0.0001),
		llms.WithTopK(1),
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			fmt.Print(string(chunk))
			return nil
		}),
	)
	if err != nil {
		return fmt.Errorf("llm generate original content: %w", err)
	}

	return nil
}

func buildChatModel() (llm *openai.LLM, dmrCtr *dmr.Container, err error) {
	dmrCtr, err = dmr.Run(context.Background(), dmr.WithModel(fqModelName), testcontainers.WithReuseByName("chat-model"))
	if err != nil {
		return nil, dmrCtr, err
	}

	opts := []openai.Option{
		openai.WithBaseURL(dmrCtr.OpenAIEndpoint()),
		openai.WithModel(fqModelName),
		openai.WithToken("foo"), // No API key needed for Model Runner
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

func buildEmbeddingStore(embedder embeddings.Embedder) (vectorstores.VectorStore, *tcweaviate.WeaviateContainer, error) {
	store, ctr, err := weaviate.NewStore(context.Background(), embedder)
	if err != nil {
		return nil, ctr, fmt.Errorf("weaviate new store: %w", err)
	}

	return store, ctr, nil
}

func ingestion(store vectorstores.VectorStore) error {
	docs := []schema.Document{
		{
			PageContent: "I like football",
		},
		{
			PageContent: "The weather is good today.",
		},
	}

	_, err := store.AddDocuments(context.Background(), docs)
	if err != nil {
		return fmt.Errorf("add documents: %w", err)
	}

	return nil
}
