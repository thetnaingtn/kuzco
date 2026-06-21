EMBED_MODEL_URL := Qwen3-Embedding-0.6B-Q8_0
MODEL_URL := Qwen3-0.6B-Q8_0

run-tests:
	go test ./... -v -race

run-llm-test:
	MODEL_URL=${MODEL_URL} go test -tags=integration ./... -run TestLLM -v -race

run-embedding-test:
	EMBED_MODEL_URL=${EMBED_MODEL_URL} go test -tags=integration ./... -run TestEmbeddings -v

