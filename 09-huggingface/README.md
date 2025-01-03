# 09-huggingface

Contains a simple example of using a language model downloaded from HuggingFace to generate text.

## Libraries Involved

- `github.com/testcontainers/testcontainers-go`: [Testcontainers for Golang](https://github.com/testcontainers/testcontainers-go) is library for running Docker containers for integration tests.
- `github.com/testcontainers/testcontainers-go/modules/ollama`: A module for running Ollama language models using Testcontainers.
- `github.com/tmc/langchaingo`: A library for interacting with language models.
- `github.com/tmc/langchaingo/llms/ollama`: A specific implementation of the language model interface for Ollama.

## Code Explanation

The code in `main.go` sets up and runs a containerized Ollama language model using Testcontainers, then uses the model to generate text based on a given prompt.

The model is automatically downloaded from HuggingFace as a GUFF model, and created as an Ollama model.

### Main Functions

- `main()`: The entry point of the application. It calls the `run()` function and logs any errors.
- `run()`: The main logic of the application. It performs the following steps:
  1. Runs an Ollama container using Testcontainers. The image used does not exist the first time this program is run, so when failed, it builds the image from a base Ollama image.
    a. The base Ollama image is `ollama/ollama:0.5.4`.
    b. After the container is started, but before the program code is executed, the Huggingface tooling and GUFF model are installed in the container.
    c. The Ollama model is created from the GUFF model file.
  2. The container is committed to the image defined in the program, so subsequent runs of it find the image and do not need to build the image each time they are run, skipping the first step.
  3. Retrieves the connection string for the running container.
  4. Creates a new Ollama language model instance.
  5. Defines the content to be generated by the language model.
  6. Generates the content and prints it to the console.

## Running the Example

To run the example, navigate to the `09-huggingface` directory and run the following command:

```sh
go run .
```

The first time the application starts, it fails to find the image defined in it, so it uses an `ollama:ollama:0.5.4` base image instead, installing the Huggingface tooling, and creating the model from the given GUFF model file. Finally, it commits the running container to the image defined in the program, so subsequent runs of it find the image and do not need to build the image each time they are run. Then, the program will generate text based on the provided prompt. The generated text will be displayed in the console.

```shell
1. High-performance: Go is known for its high performance, making it ideal for real-time applications and systems that require low latency and high throughput.

2. Easy to learn: Go is a simple and easy-to-learn language, making it an excellent choice for beginners who want to learn programming.

3. Flexible: Go is flexible enough to be used in various domains, including web development, game development, and data analysis.

4. Open source: Go is open source, which means that the codebase is available for anyone to contribute to and modify. This makes it easy to collaborate with others and share knowledge.1. High-performance: Go is known for its high performance, making it ideal for real-time applications and systems that require low latency and high throughput.

2. Easy to learn: Go is a simple and easy-to-learn language, making it an excellent choice for beginners who want to learn programming.

3. Flexible: Go is flexible enough to be used in various domains, including web development, game development, and data analysis.

4. Open source: Go is open source, which means that the codebase is available for anyone to contribute to and modify. This makes it easy to collaborate with others and share knowledge.
```

Depending on the accuracy of the model, the generated text may not be exactly what you expect.
