#! /bin/bash

docker model pull ai/llama3.2:1B-Q4_0 &
docker model pull ai/qwen3:0.6B-Q4_0 &
docker model pull ai/mxbai-embed-large:335M-F16 &
docker model pull hf.co/bartowski/Llama-3.2-1B-Instruct-GGUF &

wait
