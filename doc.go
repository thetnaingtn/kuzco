// Package kuzco adapts an *kronk.Kronk into a langchaingo llms.Model so
// callers can drive a local kronk inference instance through the langchaingo
// chat-completion surface (Call, GenerateContent, GenerateContentStream).
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
