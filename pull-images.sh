#! /bin/bash

readonly OLLAMA_VERSION="0.5.4"

docker pull ollama/ollama:${OLLAMA_VERSION}

docker pull testcontainers/ryuk:0.11.0 &
docker pull mdelapenya/moondream:${OLLAMA_VERSION}-1.8b &
docker pull semitechnologies/weaviate:1.27.2 &
docker pull pgvector/pgvector:pg16 &

wait
