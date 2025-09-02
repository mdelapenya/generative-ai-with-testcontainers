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
1. [`09-huggingface`](./09-huggingface): Contains an example of using a HuggingFace model with Docker Model Runner.
1. [`10-functions`](./10-functions): Contains an example of using functions in a language model.

## Prerequisites

- Go 1.23 or higher
- Docker
- Docker Model Runner (available in Docker Desktop), since v4.41.0

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

All the local models used in these example projects are available on Docker Hub under the [GenAI Catalog](https://hub.docker.com/catalogs/gen-ai). These are the models used in the examples:

- `ai/llama3.2:1B-Q4_0`: used for building chats.
- `ai/llama3.2:3B-Q4_K_M`: used for building chats using functions.
- `ai/qwen3:0.6B-Q4_0`: used for building chats.
- `ai/mxbai-embed-large:335M-F16`: used for generating embeddings.
- `hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF`: used for demonstrating how to use a HuggingFace model with Docker Model Runner.

You can pull them all using the `pull-models.sh` script.

### Multilingual large language models

Llama 3.2 introduced lightweight 1B and 3B models at bfloat16 (BF16) precision, later adding quantized versions. The quantized models are significantly faster, with a much lower memory footprint and reduced power consumption, while maintaining nearly the same accuracy as their BF16 counterparts.

- `ai/llama3.2:1B-Q4_0`
- `ai/llama3.2:3B-Q4_K_M`

More information about this model can be found in [Docker Hub](https://hub.docker.com/r/ai/llama3.2).

### Decoder language models

Qwen3 is the latest generation in the Qwen LLM family, designed for top-tier performance in coding, math, reasoning, and language tasks. It includes both dense and Mixture-of-Experts (MoE) models, offering flexible deployment from lightweight apps to large-scale research. Qwen3 introduces dual reasoning modes—"thinking" for complex tasks and "non-thinking" for fast responses—giving users dynamic control over performance. It outperforms prior models in reasoning, instruction following, and code generation, while excelling in creative writing and dialogue. With strong agentic and tool-use capabilities and support for over 100 languages, Qwen3 is optimized for multilingual, multi-domain applications.

- `ai/qwen3:0.6B-Q4_0`

More information about this model can be found in [Docker Hub](https://hub.docker.com/r/ai/qwen3).

#### Sentence transformers models

mxbai-embed-large-v1 is a state-of-the-art English language embedding model developed by Mixedbread AI. It converts text into dense vector representations, capturing the semantic essence of the input. Trained on a vast dataset exceeding 700 million pairs using contrastive training methods and fine-tuned on over 30 million high-quality triplets with the AnglE loss function, this model adapts to a wide range of topics and domains, making it suitable for various real-world applications and Retrieval-Augmented Generation (RAG) use cases.

- `ai/mxbai-embed-large:335M-F16`

More information about this model can be found in [Docker Hub](https://hub.docker.com/r/ai/mxbai-embed-large).

## Docker Images

The project uses some Docker images to support the examples. You can pull them all using the `pull-images.sh` script.

### Supporting Docker Images

The following Docker images are used to support the examples:

- `testcontainers/ryuk:0.11.0`: used to manage the lifecycle of the containers.
- `semitechnologies/weaviate:1.27.2`: used to store the embeddings when doing RAG.
- `pgvector/pgvector:pg16`: used to store the embeddings when doing RAG.

### Vision model

The Docker image used in the vision model example project is available on Docker Hub under the https://hub.docker.com/u/mdelapenya repository. It has been built using an automated process in GitHub Actions, and you can find the source code in the following Github repository: https://github.com/mdelapenya/dockerize-ollama-models.

The image basically starts from a base Ollama image, and then pulls the required model (moondream) to run the examples. As a consequence, it is ready to be used in the examples without any additional setup, for you to just pull the given image and run it.

Moondream is a small vision language model designed to run efficiently on edge devices. 

- `mdelapenya/moondream:0.11.8-1.8b`
