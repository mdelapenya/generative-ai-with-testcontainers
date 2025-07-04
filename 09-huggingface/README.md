# 09-huggingface

Contains a simple example of using a language model downloaded from HuggingFace to generate text.

## Libraries Involved

- `github.com/testcontainers/testcontainers-go`: [Testcontainers for Golang](https://github.com/testcontainers/testcontainers-go) is library for running Docker containers for integration tests.
- `github.com/testcontainers/testcontainers-go/modules/dockermodelrunner`: A module for running local language models using Testcontainers and the Docker Model Runner component of Docker Desktop.
- `github.com/tmc/langchaingo`: A library for interacting with language models.
- `github.com/tmc/langchaingo/llms/openai`: A specific implementation of the language model interface for OpenAI.

## Code Explanation

The code in `main.go` sets up and runs a local language model using Docker Model Runner through Testcontainers then uses the model to generate text based on a given prompt.

The model is automatically downloaded from HuggingFace as a GUFF model, and directly used in the Docker Model Runner.

### Main Functions

- `main()`: The entry point of the application. It calls the `run()` function and logs any errors.
- `run()`: The main logic of the application. It performs the following steps:
  1. Runs a local model using the [Docker Model Runner container](https://golang.testcontainers.org/modules/dockermodelrunner/). The model used is `hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF`, which is available in [HuggingFace](https://huggingface.co/bartowski/Llama-3.2-1B-Instruct-GGUF).
  2. The model name is sanitised to lower case, as Huggingface needs a lower case model name.
  3. Creates a new OpenAI language model instance, using the container's OpenAI-compatible endpoint.
  4. Defines the content to be generated by the language model.
  5. Generates the content and prints it to the console.

## Running the Example

To run the example, navigate to the `09-huggingface` directory and run the following command:

```sh
go run -v .
```

The application will start the GGUF model as a local language model and generate text based on the provided prompt. The generated text will be displayed in the console, something like this:

```shell
Here are three short bullet points explaining why Go is awesome:

• **Concurrency Powerhouse**: Go's concurrency features, such as goroutines and channels, make it an ideal choice for building high-performance, scalable applications, especially in cloud-native and microservices architectures.

• **Memory-Efficient**: Go's memory management is designed to be efficient, with a focus on avoiding unnecessary allocations and garbage collection. This results in faster startup times, lower memory usage, and improved overall system performance.

• **Simple, yet Powerful**: Go's syntax is designed to be easy to read and write, making it a great choice for beginners and experienced developers alike. Its simplicity belies its power, allowing developers to focus on building robust, reliable systems without getting bogged down in unnecessary complexity.Here are three short bullet points explaining why Go is awesome:

• **Concurrency Powerhouse**: Go's concurrency features, such as goroutines and channels, make it an ideal choice for building high-performance, scalable applications, especially in cloud-native and microservices architectures.

• **Memory-Efficient**: Go's memory management is designed to be efficient, with a focus on avoiding unnecessary allocations and garbage collection. This results in faster startup times, lower memory usage, and improved overall system performance.

• **Simple, yet Powerful**: Go's syntax is designed to be easy to read and write, making it a great choice for beginners and experienced developers alike. Its simplicity belies its power, allowing developers to focus on building robust, reliable systems without getting bogged down in unnecessary complexity.
```

Depending on the accuracy of the model, the generated text may not be exactly what you expect.
