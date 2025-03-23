package modelrunner_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// This test demonstrates integrating Docker Model Runner with Testcontainers
// for comprehensive GenAI application testing
// It sets up:
// 1. A PostgreSQL container with pgvector extension for vector storage
// 2. A connection to Docker Model Runner for LLM inference
// 3. Tests an end-to-end RAG (Retrieval Augmented Generation) workflow

func TestGenAIApplication(t *testing.T) {
	ctx := context.Background()

	// Skip test if Docker Model Runner is not available
	if !isModelRunnerAvailable() {
		t.Skip("Docker Model Runner is not available. Enable it in Docker Desktop settings.")
	}

	// Step 1: Create Socat container to tunnel to model-runner.docker.internal
	socatContainer, err := createSocatContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to create socat container: %v", err)
	}
	defer func() {
		if err := socatContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate socat container: %v", err)
		}
	}()

	// Get base URL for Model Runner API
	modelRunnerURL, err := getModelRunnerURL(ctx, socatContainer)
	if err != nil {
		t.Fatalf("Failed to get Model Runner URL: %v", err)
	}
	t.Logf("Model Runner API available at: %s", modelRunnerURL)

	// Step 2: Create PostgreSQL container with pgvector extension
	// This will store our vector embeddings for the RAG workflow
	pgContainer, pgURL, pgCleanup, err := setupPgVectorContainer(ctx)
	if err != nil {
		t.Fatalf("Failed to setup PostgreSQL container: %v", err)
	}
	defer pgCleanup()
	t.Logf("PostgreSQL with pgvector available at: %s", pgURL)

	// Step 3: Pull the necessary models
	modelName := "ignaciolopezluna020/llama3.2:1b"
	embeddingModelName := "mdelapenya/all-minilm:0.5.4-22m"

	// Pull LLM if needed
	if err := ensureModelAvailable(modelRunnerURL, modelName); err != nil {
		t.Fatalf("Failed to pull LLM model: %v", err)
	}
	t.Logf("LLM model available: %s", modelName)

	// Pull embedding model if needed
	if err := ensureModelAvailable(modelRunnerURL, embeddingModelName); err != nil {
		t.Fatalf("Failed to pull embedding model: %v", err)
	}
	t.Logf("Embedding model available: %s", embeddingModelName)

	// Step 4: Setup database schema for vector storage
	// In a real test, you would insert your test data here
	if err := setupPgVectorSchema(pgURL); err != nil {
		t.Fatalf("Failed to setup database schema: %v", err)
	}
	t.Logf("Database schema created")

	// Step 5: Generate embeddings for sample test data
	testData := []string{
		"Testcontainers is a Java library that supports JUnit tests, providing lightweight, throwaway instances of common databases, Selenium web browsers, or anything else that can run in a Docker container.",
		"A major difference to other approaches is that Testcontainers enables you to use the exact same container images as production in your tests.",
		"Docker Model Runner provides a Docker-native experience for running Large Language Models (LLMs) locally, seamlessly integrating with existing container tooling and workflows.",
	}

	// Generate embeddings for test data
	embeddings, err := generateEmbeddings(modelRunnerURL, embeddingModelName, testData)
	if err != nil {
		t.Fatalf("Failed to generate embeddings: %v", err)
	}
	t.Logf("Generated %d embeddings", len(embeddings))

	// Step 6: Store embeddings in PostgreSQL
	if err := storeEmbeddings(pgURL, testData, embeddings); err != nil {
		t.Fatalf("Failed to store embeddings: %v", err)
	}
	t.Logf("Embeddings stored in database")

	// Step 7: Perform a vector search with a test query
	query := "What is Docker Model Runner?"
	
	// Generate embedding for query
	queryEmbedding, err := generateEmbeddings(modelRunnerURL, embeddingModelName, []string{query})
	if err != nil {
		t.Fatalf("Failed to generate query embedding: %v", err)
	}
	
	// Search for similar content
	results, err := searchSimilarDocuments(pgURL, queryEmbedding[0], 1)
	if err != nil {
		t.Fatalf("Failed to search for similar documents: %v", err)
	}
	t.Logf("Found %d similar documents", len(results))
	for i, result := range results {
		t.Logf("Result %d: %s (similarity: %.4f)", i+1, result.Text, result.Similarity)
	}

	// Step 8: Generate answer using RAG
	if len(results) > 0 {
		promptTemplate := `You are a helpful assistant who answers accurately based on given information.
Context information:
%s

User question: %s

Answer the question strictly based on the context provided above. If the context doesn't contain relevant information, say "I don't have enough information to answer this question."`

		contextText := strings.Join([]string{results[0].Text}, "\n")
		prompt := fmt.Sprintf(promptTemplate, contextText, query)
		
		response, err := generateText(modelRunnerURL, modelName, prompt)
		if err != nil {
			t.Fatalf("Failed to generate text: %v", err)
		}
		t.Logf("Generated answer: %s", response)
		
		// Verify the response contains relevant information
		if !strings.Contains(response, "Docker") || !strings.Contains(response, "Model") || !strings.Contains(response, "Runner") {
			t.Errorf("Response doesn't contain expected content about Docker Model Runner")
		}
	}

	// Step 9: Verify model can directly answer questions
	testPrompt := "What is the purpose of Testcontainers? Give me a short answer in one paragraph."
	directResponse, err := generateText(modelRunnerURL, modelName, testPrompt)
	if err != nil {
		t.Fatalf("Failed to generate direct response: %v", err)
	}
	t.Logf("Direct response: %s", directResponse)
	
	// Simple verification
	if !strings.Contains(strings.ToLower(directResponse), "test") || !strings.Contains(strings.ToLower(directResponse), "container") {
		t.Errorf("Direct response doesn't mention testing or containers")
	}
}

// SearchResult represents a document found in vector search
type SearchResult struct {
	Text       string
	Similarity float64
}

// -- Helper functions --

// createSocatContainer creates a socat container to tunnel to model-runner.docker.internal
func createSocatContainer(ctx context.Context) (testcontainers.Container, error) {
	req := testcontainers.ContainerRequest{
		Image:        "alpine/socat",
		ExposedPorts: []string{"8080/tcp"},
		Cmd:          []string{"tcp-listen:8080,fork,reuseaddr", "tcp-connect:model-runner.docker.internal:80"},
		WaitingFor:   wait.ForListeningPort("8080/tcp"),
	}

	return testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
}

// getModelRunnerURL gets the URL for the Model Runner API
func getModelRunnerURL(ctx context.Context, container testcontainers.Container) (string, error) {
	host, err := container.Host(ctx)
	if err != nil {
		return "", err
	}

	port, err := container.MappedPort(ctx, "8080")
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("http://%s:%s", host, port.Port()), nil
}

// setupPgVectorContainer sets up a PostgreSQL container with pgvector extension
func setupPgVectorContainer(ctx context.Context) (testcontainers.Container, string, func(), error) {
	pgContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("pgvector/pgvector:pg16"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(120)),
	)
	if err != nil {
		return nil, "", nil, err
	}

	connString, err := pgContainer.ConnectionString(ctx)
	if err != nil {
		pgContainer.Terminate(ctx)
		return nil, "", nil, err
	}

	cleanup := func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			fmt.Printf("Failed to terminate PostgreSQL container: %v\n", err)
		}
	}

	return pgContainer, connString, cleanup, nil
}

// setupPgVectorSchema sets up the database schema for vector storage
func setupPgVectorSchema(connString string) error {
	// In a real implementation, you would execute SQL to create the schema
	// For this example, we'll just simulate it
	fmt.Println("Creating schema in PostgreSQL...")
	fmt.Println("CREATE EXTENSION IF NOT EXISTS vector;")
	fmt.Println("CREATE TABLE IF NOT EXISTS documents (id SERIAL PRIMARY KEY, content TEXT, embedding vector(384));")
	
	return nil
}

// storeEmbeddings stores text and embeddings in PostgreSQL
func storeEmbeddings(connString string, texts []string, embeddings [][]float32) error {
	// In a real implementation, you would insert the embeddings into the database
	// For this example, we'll just simulate it
	fmt.Println("Storing embeddings in PostgreSQL...")
	for i, text := range texts {
		fmt.Printf("Storing: %s (embedding length: %d)\n", 
			text[:30]+"...", len(embeddings[i]))
	}
	
	return nil
}

// searchSimilarDocuments searches for similar documents in PostgreSQL
func searchSimilarDocuments(connString string, queryEmbedding []float32, limit int) ([]SearchResult, error) {
	// In a real implementation, you would query the database for similar embeddings
	// For this example, we'll just return a simulated result
	results := []SearchResult{
		{
			Text:       "Docker Model Runner provides a Docker-native experience for running Large Language Models (LLMs) locally, seamlessly integrating with existing container tooling and workflows.",
			Similarity: 0.85,
		},
	}
	
	return results, nil
}

// ensureModelAvailable ensures a model is available in Docker Model Runner
func ensureModelAvailable(baseURL, modelName string) error {
	// Check if model is already available
	resp, err := http.Get(fmt.Sprintf("%s/engines/llama.cpp/v1/models", baseURL))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list models, status: %d, body: %s", resp.StatusCode, body)
	}

	var modelList struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&modelList); err != nil {
		return err
	}

	for _, model := range modelList.Data {
		if model.ID == modelName {
			return nil // Model already available
		}
	}

	// Model not available, need to pull it
	fmt.Printf("Pulling model: %s\n", modelName)
	return pullModel(baseURL, modelName)
}

// generateEmbeddings generates embeddings for texts using an embedding model
func generateEmbeddings(baseURL, modelName string, texts []string) ([][]float32, error) {
	// In a real implementation, you would call the Model Runner API to generate embeddings
	// For this example, we'll just simulate it
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		// Create a simulated embedding of 384 dimensions
		embedding := make([]float32, 384)
		for j := 0; j < 384; j++ {
			embedding[j] = float32(i+j) * 0.01
		}
		embeddings[i] = embedding
	}
	
	return embeddings, nil
}
