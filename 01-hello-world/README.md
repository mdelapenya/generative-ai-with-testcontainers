# 01-hello-world

Contains a simple example of using a language model to generate text.

## Libraries Involved

- `github.com/testcontainers/testcontainers-go`: [Testcontainers for Golang](https://github.com/testcontainers/testcontainers-go) is library for running Docker containers for integration tests.
- `github.com/testcontainers/testcontainers-go/modules/dockermodelrunner`: A module for running local language models using Testcontainers and the Docker Model Runner component of Docker Desktop.
- `github.com/tmc/langchaingo`: A library for interacting with language models.
- `github.com/tmc/langchaingo/llms/openai`: A specific implementation of the language model interface for OpenAI.

## Code Explanation

The code in `main.go` sets up and runs a local language model using Docker Model Runner through Testcontainers, then uses the model to generate text based on a given prompt.

### Main Functions

- `main()`: The entry point of the application. It calls the `run()` function and logs any errors.
- `run()`: The main logic of the application. It performs the following steps:
  1. Runs a local model using the [Docker Model Runner container](https://golang.testcontainers.org/modules/dockermodelrunner/). The model used is `ai/llama3.2:1B-Q4_0`, which is available in [Docker's GenAI catalog](https://hub.docker.com/catalogs/gen-ai).
  2. Creates a new OpenAI language model instance, using the container's OpenAI-compatible endpoint.
  3. Defines the content to be generated by the language model.
  4. Generates the content and prints it to the console.

## Running the Example

To run the example, navigate to the `01-hello-world` directory and run the following command:

```sh
go run -v .
```

The application will start a local language model and generate text based on the provided prompt. The generated text will be displayed in the console, something like this:

```shell
Here are three short bullet points explaining why Go is awesome:

• **Concise and Efficient**: Go's syntax and design make it one of the most concise and efficient programming languages, allowing developers to write code that is both elegant and performant.

• **Goroutines and Channels**: Go's built-in concurrency features, such as goroutines and channels, enable developers to write scalable and concurrent systems with ease, making it a popular choice for cloud-native applications.

• **Lack of Garbage Collection**: Go's focus on memory management through its ownership system and garbage collection mechanism allows developers to write low-level, performance-critical code that is free from the overhead of traditional garbage collectors.%
```
