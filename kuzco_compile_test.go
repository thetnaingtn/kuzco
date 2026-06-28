package kuzco_test

import (
	"testing"

	"github.com/tmc/langchaingo/llms"

	"github.com/thetnaingtn/kuzco"
)

// Compile-time assertion that *kuzco.LLM satisfies llms.ReasoningModel.
var _ llms.ReasoningModel = (*kuzco.LLM)(nil)

// TestCompile asserts the package compiles and the interface assertions
// in kuzco.go (llms.Model + embeddings.EmbedderClient) hold even when
// the integration build tag is off.
func TestCompile(t *testing.T) {}

// TestSupportsReasoning asserts kuzco reports reasoning support both via the
// method directly and through langchaingo's runtime feature-detection helper.
func TestSupportsReasoning(t *testing.T) {
	llm := kuzco.New(nil)

	if !llm.SupportsReasoning() {
		t.Fatal("SupportsReasoning() = false, want true")
	}

	if !llms.SupportsReasoningModel(llm) {
		t.Fatal("llms.SupportsReasoningModel(llm) = false, want true")
	}
}
