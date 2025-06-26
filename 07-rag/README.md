# 07-rag

Contains a simple example of using a language model to answer questions based on a given prompt using RAG (Retrieval-Augmented Generation).

## Libraries Involved

- `github.com/testcontainers/testcontainers-go`: [Testcontainers for Golang](https://github.com/testcontainers/testcontainers-go) is library for running Docker containers for integration tests.
- `github.com/testcontainers/testcontainers-go/modules/dockermodelrunner`: A module for running local language models using Testcontainers and the Docker Model Runner component of Docker Desktop.
- `github.com/testcontainers/testcontainers-go/modules/weaviate`: A module for running Weaviate vector search engines using Testcontainers.
- `github.com/tmc/langchaingo`: A library for interacting with language models.
- `github.com/tmc/langchaingo/llms/openai`: A specific implementation of the language model interface for OpenAI.
- `github.com/tmc/langchaingo/vectorstores`: An interface for interacting with vector search engines.
- `github.com/tmc/langchaingo/vectorstores/weaviate`: A specific implementation of the vector store interface for Weaviate.

## Code Explanation

The code in `main.go` sets up and runs two local language models and a Weaviate vector store using Testcontainers, then uses one of the models to generate the embeddings for a set of texts. It then uses the Weaviate vector store to search for similar embeddings and generate text based on the augmented prompt using RAG.

### Main Functions

- `main()`: The entry point of the application. It calls the `run()` function and logs any errors.
- `run()`: The main logic of the application. It performs the following steps:
  1. Runs a local model for the embeddings, using the [Docker Model Runner container](https://golang.testcontainers.org/modules/dockermodelrunner/). The model used is `ai/mxbai-embed-large:335M-F16`, which is available in [Docker's GenAI catalog](https://hub.docker.com/catalogs/gen-ai).
  2. Creates a new OpenAI language model instance for the embeddings model, using the container's OpenAI-compatible endpoint. It's used to build the RAG store.
  4. Runs a Weaviate container as RAG store, using Testcontainers. The image used is `semitechnologies/weaviate:1.27.2`, and it is used to store and retrieve embeddings for the RAG.
  5. Ingests some example data into the Weaviate vector store.
  6. Performs a search in Weaviate to retrieve the most similar embeddings to a query.
  7. If there are no results, the program exits with an error message.
  8. If there are results, the program builds a local chat language model, using `ai/llama3.2:1B-Q4_0`, which is available in [Docker's GenAI catalog](https://hub.docker.com/catalogs/gen-ai).
  9. Using the relevant content from the Weaviate search results, the program generates a streaming response to the user's prompt.

## Running the Example

To run the example, navigate to the `07-rag` directory and run the following command:

```sh
go run -v .
```

The application will start two local language models and generate text based on the augmented prompt using RAG. The generated text will be displayed in the console, something like this:

```shell
What is your favourite sport?

Answer the question considering the following relevant content:
I like football

I'm glad you mentioned football. As a neutral AI, I don't have personal preferences or feelings, but I can tell you about the popularity of football (or soccer, as it's commonly known outside the US) and the reasons why many people enjoy it.

Football is a highly popular sport globally, with millions of fans worldwide. It's a game that requires skill, strategy, and physical fitness, making it an exciting and engaging experience for players and spectators alike. The World Cup, for example, is one of the most-watched sporting events in the world, with over 3.5 billion people tuning in to watch the final match.

In the US, the National Football League (NFL) is one of the most popular professional sports leagues, with a massive following and a strong fan base. The NFL has a huge impact on popular culture, with its players and coaches often being featured in movies, TV shows, and music.

What about you? Do you have a favorite football team or player?%
```
