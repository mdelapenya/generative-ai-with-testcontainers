# 02-streaming

Contains a simple example of using a language model to generate text in streaming mode.

## Libraries Involved

- `github.com/testcontainers/testcontainers-go`: A library for running Docker containers for integration tests.
- `github.com/testcontainers/testcontainers-go/modules/ollama`: A module for running Ollama language models using Testcontainers.
- `github.com/tmc/langchaingo`: A library for interacting with language models.
- `github.com/tmc/langchaingo/llms/ollama`: A specific implementation of the language model interface for Ollama.

## Code Explanation

The code in `main.go` sets up and runs a containerized Ollama language model using Testcontainers, then uses the model to generate text based on a given prompt.

### Main Functions

- `main()`: The entry point of the application. It calls the `run()` function and logs any errors.
- `run()`: The main logic of the application. It performs the following steps:
  1. Runs an Ollama container using Testcontainers for Golang. The image used is `ilopezluna/qwen2:0.3.13-0.5b`, loading the `qwen2:0.5b` model.
  2. Retrieves the connection string for the running container.
  3. Creates a new Ollama language model instance.
  4. Defines the content to be generated by the language model.
  5. Generates the content and prints it to the console.

## Running the Example

To run the example, navigate to the `02-streaming` directory and run the following command:

```sh
go run .
```

The application will start a containerized Ollama language model and generate text based on the provided prompt. The generated text will be displayed in the console.

```shell
Testcontainers is an open-source containerization platform that allows you to easily deploy, manage, and run applications on containers. It's designed specifically for the Go programming language and provides a powerful set of tools for building, testing, and deploying applications.
Here are some key reasons why Testcontainers for Go is great:

1. Scalability: Testcontainers can handle high volumes of concurrent requests without any issues or lag. This means that you can easily scale your application up or down based on demand without having to worry about performance issues.

2. Flexibility: Testcontainers provides a wide range of tools and features that allow you to customize your application's behavior and requirements. You can choose from a variety of containerization options, including Docker, Kubernetes, and more, which allows you to build, deploy, and run applications on any platform or operating system.

3. Security: Testcontainers is designed with security in mind, so it automatically handles the necessary steps for securing your application. This includes things like authentication, access control, and logging, making it easy to protect your application from external threats.

4. Easy-to-use: Testcontainers provides a user-friendly interface that allows you to easily manage your application's behavior and requirements. You can customize settings, run tests, deploy applications, and more in just a few clicks.

5. Community support: Testcontainers has a large and active community of developers who provide support for the platform. This means that if you encounter any issues or have questions, you can get help from people who know what they're doing.

6. Flexibility: Testcontainers is designed to be flexible, so it's easy to switch between different platforms or operating systems without having to worry about compatibility issues. You can choose from a variety of containerization options and run your application on any platform that supports the platform you want to use.

In summary, Testcontainers for Go is great because it provides scalability, flexibility, security, ease-of-use, community support, and flexibility. It's designed specifically for the Go programming language and allows you to easily build, deploy, and run applications on containers with a powerful set of tools and features.% 
```