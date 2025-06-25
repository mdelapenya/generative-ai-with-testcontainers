# 03-chat

Contains a simple example of using a language model to generate text in an interactive chat application.

## Libraries Involved

- `github.com/testcontainers/testcontainers-go`: [Testcontainers for Golang](https://github.com/testcontainers/testcontainers-go) is library for running Docker containers for integration tests.
- `github.com/testcontainers/testcontainers-go/modules/dockermodelrunner`: A module for running local language models using Testcontainers and the Docker Model Runner component of Docker Desktop.
- `github.com/tmc/langchaingo`: A library for interacting with language models.
- `github.com/tmc/langchaingo/llms/openai`: A specific implementation of the language model interface for OpenAI.

## Code Explanation

The code in `main.go` sets up and runs a local language model using Docker Model Runner through Testcontainers, then uses the model to generate text based on an interactive prompt.

### Main Functions

- `main()`: The entry point of the application. It calls the `run()` function and logs any errors.
- `run()`: The main logic of the application. It performs the following steps:
  1. Runs a local model using the [Docker Model Runner container](https://golang.testcontainers.org/modules/dockermodelrunner/). The model used is `ai/llama3.2:1B-Q4_0`, which is available in [Docker's GenAI catalog](https://hub.docker.com/catalogs/gen-ai).
  2. Creates a new OpenAI language model instance, using the container's OpenAI-compatible endpoint.
  3. Defines an infinite loop to interact with the language model in a chat-like manner.
  4. Generates the content and prints it to the console based on the user's input.
  5. Exits the interactive loop if the user types `exit`, `quit`, or hits `Ctrl+C`.

## Running the Example

To run the example, navigate to the `03-chat` directory and run the following command:

```sh
go run -v .
```

The application will start a local language model and generate text based on the interactive prompt. The generated text will be displayed in the console, and the user can continue interacting with the model until they choose to exit.

```shell
go run .

You: what is the capital of Japan
The capital of Japan is Tokyo.
You: ^C
Interrupt signal received, ending chat session
```
