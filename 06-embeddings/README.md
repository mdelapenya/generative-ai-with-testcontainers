# 06-embeddings

Contains a simple example of using a language model to calculate the embeddings for a given set of texts, and calculate the similarity between them.

## Libraries Involved

- `github.com/testcontainers/testcontainers-go`: [Testcontainers for Golang](https://github.com/testcontainers/testcontainers-go) is library for running Docker containers for integration tests.
- `github.com/testcontainers/testcontainers-go/modules/dockermodelrunner`: A module for running local language models using Testcontainers and the Docker Model Runner component of Docker Desktop.
- `github.com/tmc/langchaingo`: A library for interacting with language models.
- `github.com/tmc/langchaingo/llms/openai`: A specific implementation of the language model interface for OpenAI.

## Code Explanation

The code in `main.go` sets up and runs a local language model using Docker Model Runner through Testcontainers, then uses the model to obtain an embedder and calculate the embeddings for a set of texts. It then calculates the similarity between the embeddings of the texts.

### Main Functions

- `main()`: The entry point of the application. It calls the `run()` function and logs any errors.
- `run()`: The main logic of the application. It performs the following steps:
  1. Runs a local model using the [Docker Model Runner container](https://golang.testcontainers.org/modules/dockermodelrunner/). The model used is `ai/mxbai-embed-large:335M-F16`, which is available in [Docker's GenAI catalog](https://hub.docker.com/catalogs/gen-ai).
  2. Creates a new OpenAI embedding model instance, using the container's OpenAI-compatible endpoint.
  4. Defines a set of texts for which we want to calculate the embeddings.
  5. Calculates the embeddings for the texts.
  6. Calculates the similarity between the embeddings of the texts, displaying the results in the console.

## Running the Example

To run the example, navigate to the `06-embeddings` directory and run the following command:

```sh
go run -v .
```

The application will start a local language model and generate the embeddings for the provided texts.
It will then calculate the similarity between the embeddings and display the results in the console.

```shell
Similarities:
A cat is a small domesticated carnivorous mammal ~ A cat is a small domesticated carnivorous mammal = 1.00
A cat is a small domesticated carnivorous mammal ~ A tiger is a large carnivorous feline mammal = 0.73
A cat is a small domesticated carnivorous mammal ~ Testcontainers is a Go package that supports JUnit tests, providing lightweight, throwaway instances of common databases, web browsers, or anything else that can run in a Docker container = 0.32
A cat is a small domesticated carnivorous mammal ~ Docker is a platform designed to help developers build, share, and run container applications. We handle the tedious setup, so you can focus on the code. = 0.35
A tiger is a large carnivorous feline mammal ~ A cat is a small domesticated carnivorous mammal = 0.73
A tiger is a large carnivorous feline mammal ~ A tiger is a large carnivorous feline mammal = 1.00
A tiger is a large carnivorous feline mammal ~ Testcontainers is a Go package that supports JUnit tests, providing lightweight, throwaway instances of common databases, web browsers, or anything else that can run in a Docker container = 0.26
A tiger is a large carnivorous feline mammal ~ Docker is a platform designed to help developers build, share, and run container applications. We handle the tedious setup, so you can focus on the code. = 0.29
Testcontainers is a Go package that supports JUnit tests, providing lightweight, throwaway instances of common databases, web browsers, or anything else that can run in a Docker container ~ A cat is a small domesticated carnivorous mammal = 0.32
Testcontainers is a Go package that supports JUnit tests, providing lightweight, throwaway instances of common databases, web browsers, or anything else that can run in a Docker container ~ A tiger is a large carnivorous feline mammal = 0.26
Testcontainers is a Go package that supports JUnit tests, providing lightweight, throwaway instances of common databases, web browsers, or anything else that can run in a Docker container ~ Testcontainers is a Go package that supports JUnit tests, providing lightweight, throwaway instances of common databases, web browsers, or anything else that can run in a Docker container = 1.00
Testcontainers is a Go package that supports JUnit tests, providing lightweight, throwaway instances of common databases, web browsers, or anything else that can run in a Docker container ~ Docker is a platform designed to help developers build, share, and run container applications. We handle the tedious setup, so you can focus on the code. = 0.70
Docker is a platform designed to help developers build, share, and run container applications. We handle the tedious setup, so you can focus on the code. ~ A cat is a small domesticated carnivorous mammal = 0.35
Docker is a platform designed to help developers build, share, and run container applications. We handle the tedious setup, so you can focus on the code. ~ A tiger is a large carnivorous feline mammal = 0.29
Docker is a platform designed to help developers build, share, and run container applications. We handle the tedious setup, so you can focus on the code. ~ Testcontainers is a Go package that supports JUnit tests, providing lightweight, throwaway instances of common databases, web browsers, or anything else that can run in a Docker container = 0.70
Docker is a platform designed to help developers build, share, and run container applications. We handle the tedious setup, so you can focus on the code. ~ Docker is a platform designed to help developers build, share, and run container applications. We handle the tedious setup, so you can focus on the code. = 1.00
```
