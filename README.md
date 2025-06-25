# Generative AI with Testcontainers for Golang

This project demonstrates how to use [Testcontainers for Golang](https://github.com/testcontainers/testcontainers-go) to create a seamless development environment for building Generative AI applications.

## Project Structure

1. [`01-hello-world`](./01-hello-world): Contains a simple example of using a language model to generate text.
1. [`02-streaming`](./02-streaming): Contains an example of using a language model to generate text in streaming mode.
1. [`03-chat`](./03-chat): Contains an example of using a language model to generate text in a chat application.
1. [`04-vision-model`](./04-vision-model): Contains an example of using a vision model to generate text from images.
1. [`05-augmented-generation`](./05-augmented-generation): Contains an example of augmenting the prompt with additional information to generate more accurate text.
1. [`06-embeddings`](./06-embeddings): Contains an example of generating embeddings from text and calculating similarity between them.
1. [`07-rag`](./07-rag): Contains an example of applying RAG (Retrieval-Augmented Generation) to generate better responses.
1. [`08-testing`](./08-testing): Contains an example with the evolution of testing our Generative AI applications, from an old school approach to a more modern one using Evaluator Agents.
1. [`09-huggingface`](./09-huggingface): Contains an example of using a HuggingFace model with Ollama.
1. [`10-functions`](./10-functions): Contains an example of using functions in a language model.

## Prerequisites

- Go 1.23 or higher
- Docker

## Setup

1. Clone the repository:
    ```sh
    git clone https://github.com/mdelapenya/generative-ai-with-testcontainers.git
    cd generative-ai-with-testcontainers
    ```

## Running the Examples

To run the examples, navigate to the desired directory and run the `go run .` command. For example, to run the `1-hello-world` example:

```sh
cd 1-hello-world
go run .
```

## Local Models

All the Local images used in these example projects are available on Docker Hub under the [GenAI Catalog](https://hub.docker.com/catalogs/gen-ai). These are the models used in the examples:

- `ai/llama3.2:1B-Q4_0`: used for building chats.
- `ai/qwen3:0.6B-Q4_0`: used for building chats.
- `ai/mxbai-embed-large:335M-F16`: used for generating embeddings.

You can pull them all using the `pull-models.sh` script.

## Docker Images

All the Docker images used in these example projects are available on Docker Hub under the https://hub.docker.com/u/mdelapenya repository. They have been built using an automated process in GitHub Actions, and you can find the source code in the following Github repository: https://github.com/mdelapenya/dockerize-ollama-models.

Each image basically starts from a base Ollama image, and then pulls the required models to run the examples. As a consequence, they are ready to be used in the examples without any additional setup, for you to just pull the given image and run it.

The images used in the examples are described below, grouped by model type. You can pull them using the `pull-images.sh` script.

#### Multilingual large language models

The Llama 3.2 collection of multilingual large language models (LLMs) is a collection of pretrained and instruction-tuned generative models in 1B and 3B sizes (text in/text out). The Llama 3.2 instruction-tuned text only models are optimized for multilingual dialogue use cases, including agentic retrieval and summarization tasks. They outperform many of the available open source and closed chat models on common industry benchmarks.

- `mdelapenya/llama3.2:0.5.4-1b`
- `mdelapenya/llama3.2:0.5.4-3b`

#### Decoder language models

Qwen2 is a language model series including decoder language models of different model sizes. For each size, we release the base language model and the aligned chat model. It is based on the Transformer architecture with SwiGLU activation, attention QKV bias, group query attention, etc. Additionally, we have an improved tokenizer adaptive to multiple natural languages and codes.

- `mdelapenya/qwen2:0.5.4-0.5b`

#### Vision models

Moondream is a small vision language model designed to run efficiently on edge devices. 
- `mdelapenya/moondream:0.5.4-1.8b`

#### Sentence transformers models

This model maps sentences & paragraphs to a 384 dimensional dense vector space and can be used for tasks like clustering or semantic search.

- `mdelapenya/all-minilm:0.5.4-22m`
