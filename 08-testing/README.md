# 08-testing

Contains a simple example of using a language model to validate the answers of other language models, using an Evaluator Agent.

## Libraries Involved

- `github.com/testcontainers/testcontainers-go`: [Testcontainers for Golang](https://github.com/testcontainers/testcontainers-go) is library for running Docker containers for integration tests.
- `github.com/testcontainers/testcontainers-go/modules/dockermodelrunner`: A module for running local language models using Testcontainers and the Docker Model Runner component of Docker Desktop.
- `github.com/testcontainers/testcontainers-go/modules/postgres`: A module for running PgVector vector search engines using Testcontainers.
- `github.com/testcontainers/testcontainers-go/modules/weaviate`: A module for running Weaviate vector search engines using Testcontainers.
- `github.com/tmc/langchaingo`: A library for interacting with language models.
- `github.com/tmc/langchaingo/llms/openai`: A specific implementation of the language model interface for OpenAI.
- `github.com/tmc/langchaingo/vectorstores`: An interface for interacting with vector search engines.
- `github.com/tmc/langchaingo/vectorstores/pgvector`: A specific implementation of the vector store interface for PgVector.
- `github.com/tmc/langchaingo/vectorstores/weaviate`: A specific implementation of the vector store interface for Weaviate.

## Code Explanation

The code in `main.go` prints out two different responses for the same task: one for talking to a model in a straight manner, and the second using RAG. For that, it sets up and runs two local language models and a vector store using Testcontainers, then uses one of the models to generate the embeddings for a set of texts. It then uses the selected vector store to search for similar embeddings and generate text based on the augmented prompt using RAG.

The vector store to use is `weaviate` by default, but it can be changed to `pgvector` by setting the `VECTOR_STORE` environment variable to `pgvector`. 

- The image used for Weaviate is `semitechnologies/weaviate:1.27.2`.
- The image used for PgVector is `pgvector/pgvector:pg16`.

We are adding tests to demonstrate how to validate the answers of the language models. We will use an Evaluator Agent to do so.

### Main Functions

- `main()`: The entry point of the application. It calls the `run()` function and logs any errors.
- `run()`: The main logic of the application. It performs the following steps:
  1. Runs a local model for the embeddings, using the [Docker Model Runner container](https://golang.testcontainers.org/modules/dockermodelrunner/). The model used is `ai/mxbai-embed-large:335M-F16`, which is available in [Docker's GenAI catalog](https://hub.docker.com/catalogs/gen-ai).
  1. Runs a new local model for the chat, using the [Docker Model Runner container](https://golang.testcontainers.org/modules/dockermodelrunner/). The model used is `ai/llama3.2:1B-Q4_0`, which is available in [Docker's GenAI catalog](https://hub.docker.com/catalogs/gen-ai).
  1. The chat model is asked directly for a response to a fixed question. The model does not have any context about the question.
  1. From the local model for the embeddings, it creates a new local language model instance, which is used as the embedder for the RAG model.
  1. Runs a store container using Testcontainers, and it is used to store and retrieve embeddings for the RAG.
  1. Ingests some markdown documents about Testcontainers Cloud into the vector store, using the embedder. The files are ingested using chunks of 1024 characters.
  1. Performs a search in the store to retrieve the most similar embeddings to the original fixed question.
  1. If there are no results, the program exits with an error message.
  1. If there are results, the program builds a local chat language model (`ai/llama3.2:1B-Q4_0`), to talk to it using RAG.
  1. Using the relevant content from the store search results, the program generates a streaming response to the user's prompt.

## Running the Example

To run the example, navigate to the `08-testing` directory and run the following command:

```sh
go run -v .
```

The application will start two local language models and generate text based on the augmented prompt using RAG. The generated text will be displayed in the console.

```shell
2025/06/25 16:31:07 How I can enable verbose logging in Testcontainers Desktop?
>> Straight answer:
 To enable verbose logging in Testcontainers Desktop, you can use the `verbose` property when creating a Testcontainer instance. 

Here's an example:

Testcontainer testContainer = TestcontainerBuilder.create()
        .withDefaultSsl(true)
        .withDefaultSslVerify(false)
        .withDefaultSslTrustManagerFactory(new TrustManagerFactory() {
            @Override
            public TrustManagerFactoryTrustManagerFactory trustManagerFactory(TrustManagerFactoryTrustManagerFactory trustManagerFactoryTrustManagerFactory) {
                trustManagerFactoryTrustManagerFactory.setInsecureSsl(true);
                trustManagerFactoryTrustManagerFactory.setInsecureSslVerify(false);
                trustManagerFactoryTrustManagerFactory.setInsecureSslTrustManagerFactory(new InsecureSslTrustManagerFactory());
                return trustManagerFactoryTrustManagerFactoryTrustManagerFactory;
            }
        })
        .withDefaultSslTrustManagerFactory(new InsecureSslTrustManagerFactory())
        .withDefaultSslVerify(false)
        .withDefaultSslTrustManagerFactory(new TrustManagerFactory() {
            @Override
            public TrustManagerFactoryTrustManagerFactory trustManagerFactory(TrustManagerFactoryTrustManagerFactory trustManagerFactoryTrustManagerFactory) {
                trustManagerFactoryTrustManagerFactory.setInsecureSsl(true);
                trustManagerFactoryTrustManagerFactory.setInsecureSslVerify(false);
                trustManagerFactoryTrustManagerFactory.setInsecureSslTrustManagerFactory(new InsecureSslTrustManagerFactory());
                return trustManagerFactoryTrustManagerFactoryTrustManagerFactory;
            }
        })
        .withDefaultSslTrustManagerFactory(new InsecureSslTrustManagerFactory())
        .withDefaultSslVerify(false)
        .build();

This will enable verbose logging by setting the `verbose` property to `true` for each of the above TrustManagerFactoryTrustManagerFactory instances.
2025/06/25 16:31:18 Ingesting document: knowledge/txt/simple-local-development-with-testcontainers-desktop.txt
2025/06/25 16:31:18 Ingesting document: knowledge/txt/tc-guide-introducing-testcontainers.txt
2025/06/25 16:31:18 Ingesting document: knowledge/txt/tcc.txt
2025/06/25 16:31:24 Ingested 82 documents
2025/06/25 16:31:24 Relevant documents for RAG: 3
>> Ragged answer:
 To enable verbose logging in Testcontainers Desktop, you can add the following line to your `~/.testcontainers.properties` file:

`cloud.logs.verbose = true`

Alternatively, you can also use the following command in your terminal:

`open -W --stdout $(tty) --stderr $(tty) /Applications/Testcontainers\ Desktop.app`

This will redirect both stdout and stderr output to a specified location. The log file is rotated once it gets bigger than 5Mb.
```

## How to test this (1): String comparison

To test this, what we would usually do is to create a test file with two tests, one for the straight answer and another for the ragged answer. We would then run the tests and check if the output matches the expected output. Just take a look at the `main_test.go` file in the `08-testing` directory, and its `Test1_oldSchool` test function, and then run the tests:

```shell
go test -timeout 600s -run ^Test1_oldSchool/weaviate$ github.com/mdelapenya/genai-testcontainers-go/testing -v -count=1
```

## How to test this (2): Embeddings

In a second iteration, we remembered that we now know how to create emebeddings and calculate the cosine similarity. So we create two tests more in the test file, one for the straight answer and another for the ragged answer. We would then run the tests and check if the cosine similarity is higher thatn 0.8 (our threshold). Just take a look at the `main_test.go` file in the `08-testing` directory, and its `Test2_embeddings` test function, and then run the tests:

```shell
go test -timeout 600s -run ^Test2_embeddings/weaviate$ github.com/mdelapenya/genai-testcontainers-go/testing -v -count=1
```

## How to test this (3): Evaluator Agents

Finally, in a third iteration, we realised that we have a lot of power with LLMs, and it would be cool to use one to validate the answers. We could be as strict as needed defining the System and User prompts, in order for the evaluator agent to be very specific about the answer. We can even provide an output format for the answer, so the evaluator agent can check if the answer is correct. Just take a look at the `main_test.go` file in the `08-testing` directory, and its `Test3_evaluatorAgent` test function, and then run the tests:

```shell
go test -timeout 600s -run ^Test3_evaluatorAgent/weaviate$ github.com/mdelapenya/genai-testcontainers-go/testing -v -count=1
```
