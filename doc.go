// Package kuzco adapts a *kronk.Kronk into a langchaingo llms.Model and
// embeddings.EmbedderClient, so callers can drive a local kronk inference
// instance through langchaingo's chat-completion surface (Call,
// GenerateContent, GenerateContentStream) and its embedding surface
// (CreateEmbedding).
//
// Embedding behavior is configured via constructor options on New:
//
//	llm := kuzco.New(k,
//		kuzco.WithEmbeddingTruncate(true),
//		kuzco.WithEmbeddingTruncateDirection(kuzco.TruncateLeft),
//	)
//
// Per-call options are not supported because langchaingo's
// EmbedderClient.CreateEmbedding signature does not accept variadic options.
package kuzco

import (
	"context"
	"fmt"

	"github.com/ardanlabs/kronk/sdk/kronk"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
)

func ExampleNew() {
	// In real usage, construct a fully-configured *kronk.Kronk first.
	var k *kronk.Kronk

	llm := New(k)

	resp, err := llms.GenerateFromSinglePrompt(context.Background(), llm, "Say OK")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(resp)
}

func ExampleNew_embedder() {
	// The underlying GGUF must be embed-capable (modelInfo.IsEmbedModel). In real usage, construct a fully-configured *kronk.Kronk first.
	var k *kronk.Kronk

	llm := New(k)

	embedder, err := embeddings.NewEmbedder(llm)
	if err != nil {
		fmt.Println(err)
		return
	}

	vec, err := embedder.EmbedQuery(context.Background(), "hello")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(len(vec))
}
