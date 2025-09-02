# 04-vision-model

Contains a simple example of using a language model to generate text from images.

## Libraries Involved

- `github.com/testcontainers/testcontainers-go`: [Testcontainers for Golang](https://github.com/testcontainers/testcontainers-go) is library for running Docker containers for integration tests.
- `github.com/testcontainers/testcontainers-go/modules/ollama`: A module for running Ollama language models using Testcontainers.
- `github.com/tmc/langchaingo`: A library for interacting with language models.
- `github.com/tmc/langchaingo/llms/ollama`: A specific implementation of the language model interface for Ollama.

## Code Explanation

The code in `main.go` sets up and runs a containerized Ollama language model using Testcontainers, then uses the model to generate text based on an image.

### Main Functions

- `main()`: The entry point of the application. It calls the `run()` function and logs any errors.
- `run()`: The main logic of the application. It performs the following steps:
  1. Runs an Ollama container using Testcontainers. The image used is `mdelapenya/moondream:0.11.8-1.8b`, loading the `moondream:1.8b` model.
  2. Retrieves the connection string for the running container.
  3. Creates a new Ollama language model instance.
  4. Defines a user prompt based on an image.
  5. Generates the content representing the image and prints it to the console.

## Running the Example

To run the example, navigate to the `04-vision-model` directory and run the following command:

```sh
go run .
```

The application will start a containerized Ollama language model and generate text based on the provided image. The generated text will be displayed in the console.

```shell
The image features a large orange cat sitting on the windowsill of a building. The cat is facing to the right, giving us a clear view of its face and body. It appears to be looking out the window or observing something outside. 

In addition to the main subject, there are two other cats visible in the image - one located near the top left corner and another further down on the windowsill. The scene captures the cat's attention as it sits comfortably on its perch by the window.% 
```
