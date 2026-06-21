// Package llmtest is a local fork of langchaingo's testing/llmtest suite,
// adapted so callers can thread extra llms.CallOption values into every
// subtest.
//
// The upstream llmtest.TestLLM helper hardcodes its call options (tiny
// max_tokens, no thinking control) and exposes no injection point, while
// TestLLMWithOptions only forwards options to a reduced subset of tests
// (Call, GenerateContent, Streaming). kronk runs reasoning on by default and
// reasoning shares the max_tokens budget with the response, so those tiny
// budgets starve the answer on reasoning models. This fork forwards the
// caller's options (e.g. llms.WithThinkingMode(llms.ThinkingModeNone)) into
// the full capability suite — ToolCalls, Reasoning, Caching, TokenCounting
// included.
//
// The helper names mirror upstream (supportsReasoning, testGenerateContent,
// etc.) so this stays easy to diff against langchaingo.
package llmtest

import (
	"context"
	"strings"
	"testing"

	"github.com/tmc/langchaingo/llms"
)

// streamer is the optional streaming interface a model may implement.
type streamer interface {
	GenerateContentStream(context.Context, []llms.MessageContent, ...llms.CallOption) (<-chan llms.ContentResponse, error)
}

// withOpts returns base with the caller's extra options appended so they win
// on conflict (applyCallOptions applies options in order, last write wins).
func withOpts(extra []llms.CallOption, base ...llms.CallOption) []llms.CallOption {
	return append(base, extra...)
}

// TestLLM tests an LLM implementation, probing its capabilities and running the
// matching subtests. Any opts are forwarded into every model call so callers
// can, for example, disable reasoning with llms.WithThinkingMode(ThinkingModeNone).
func TestLLM(t *testing.T, model llms.Model, opts ...llms.CallOption) {
	t.Helper()

	t.Run("Core", func(t *testing.T) {
		t.Run("Call", func(t *testing.T) {
			testCall(t, model, opts)
		})

		t.Run("GenerateContent", func(t *testing.T) {
			testGenerateContent(t, model, opts)
		})
	})

	t.Run("Capabilities", func(t *testing.T) {
		if supportsStreaming(model) {
			t.Run("Streaming", func(t *testing.T) {
				testStreaming(t, model, opts)
			})
		}

		if supportsTools(model, opts) {
			t.Run("ToolCalls", func(t *testing.T) {
				testToolCalls(t, model, opts)
			})
		}

		if supportsReasoning(model, opts) {
			t.Run("Reasoning", func(t *testing.T) {
				testReasoning(t, model, opts)
			})
		}

		t.Run("Caching", func(t *testing.T) {
			testCaching(t, model, opts)
		})

		t.Run("TokenCounting", func(t *testing.T) {
			testTokenCounting(t, model, opts)
		})
	})
}

// Capability detection functions

// supportsStreaming checks if the model supports streaming.
func supportsStreaming(model llms.Model) bool {
	_, ok := model.(streamer)
	return ok
}

// supportsTools probes if the model supports tool calls.
func supportsTools(model llms.Model, opts []llms.CallOption) bool {
	ctx := context.Background()
	tools := []llms.Tool{
		{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        "test_tool",
				Description: "Test tool",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}

	messages := []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{llms.TextPart("test")},
		},
	}

	_, err := model.GenerateContent(ctx, messages, withOpts(opts,
		llms.WithTools(tools),
		llms.WithMaxTokens(1),
	)...)

	if err != nil && strings.Contains(strings.ToLower(err.Error()), "not support") {
		return false
	}
	return err == nil || !strings.Contains(strings.ToLower(err.Error()), "tool")
}

// supportsReasoning checks if the model reports reasoning/thinking tokens.
func supportsReasoning(model llms.Model, opts []llms.CallOption) bool {
	if reasoner, ok := model.(interface{ SupportsReasoning() bool }); ok {
		return reasoner.SupportsReasoning()
	}

	ctx := context.Background()
	messages := []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{llms.TextPart("test")},
		},
	}

	resp, err := model.GenerateContent(ctx, messages, withOpts(opts,
		llms.WithMaxTokens(10),
		llms.WithThinkingMode(llms.ThinkingModeLow),
	)...)

	if err == nil && resp != nil && len(resp.Choices) > 0 {
		if genInfo := resp.Choices[0].GenerationInfo; genInfo != nil {
			if _, ok := genInfo["ThinkingTokens"]; ok {
				return true
			}
		}
	}

	return false
}

// Core test implementations

func testCall(t *testing.T, model llms.Model, opts []llms.CallOption) {
	t.Helper()
	ctx := context.Background()

	result, err := llms.GenerateFromSinglePrompt(ctx, model,
		"Reply with 'OK' and nothing else",
		withOpts(opts, llms.WithMaxTokens(10))...)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	if result == "" {
		t.Error("Call returned empty result")
	}
}

func testGenerateContent(t *testing.T, model llms.Model, opts []llms.CallOption) {
	t.Helper()
	ctx := context.Background()

	messages := []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{llms.TextPart("Reply with 'Hello' and nothing else")},
		},
	}

	resp, err := model.GenerateContent(ctx, messages, withOpts(opts, llms.WithMaxTokens(10))...)
	if err != nil {
		t.Fatalf("GenerateContent failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("No choices in response")
	}

	content := strings.ToLower(resp.Choices[0].Content)
	if !strings.Contains(content, "hello") {
		t.Errorf("Expected 'hello' in response, got: %s", resp.Choices[0].Content)
	}
}

func testStreaming(t *testing.T, model llms.Model, opts []llms.CallOption) {
	t.Helper()
	ctx := context.Background()

	messages := []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{llms.TextPart("Count from 1 to 3")},
		},
	}

	s, ok := model.(streamer)
	if !ok {
		t.Skip("Model doesn't support streaming")
	}

	stream, err := s.GenerateContentStream(ctx, messages, withOpts(opts, llms.WithMaxTokens(50))...)
	if err != nil {
		t.Fatalf("GenerateContentStream failed: %v", err)
	}

	var chunks []string
	for chunk := range stream {
		if len(chunk.Choices) > 0 {
			chunks = append(chunks, chunk.Choices[0].Content)
		}
	}

	if len(chunks) == 0 {
		t.Error("No chunks received from stream")
	}

	if strings.Join(chunks, "") == "" {
		t.Error("Stream produced no content")
	}
}

func testToolCalls(t *testing.T, model llms.Model, opts []llms.CallOption) {
	t.Helper()
	ctx := context.Background()

	tools := []llms.Tool{
		{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        "get_weather",
				Description: "Get the weather for a location",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{
							"type":        "string",
							"description": "The city and country",
						},
					},
					"required": []string{"location"},
				},
			},
		},
	}

	messages := []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{llms.TextPart("What's the weather in San Francisco?")},
		},
	}

	resp, err := model.GenerateContent(ctx, messages, withOpts(opts,
		llms.WithTools(tools),
		llms.WithMaxTokens(100),
	)...)
	if err != nil {
		t.Fatalf("GenerateContent with tools failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("No choices in response")
	}

	choice := resp.Choices[0]
	if len(choice.ToolCalls) == 0 {
		t.Log("No tool calls in response (model may not support tools)")
	} else if choice.ToolCalls[0].FunctionCall.Name != "get_weather" {
		t.Errorf("Expected get_weather tool call, got: %s", choice.ToolCalls[0].FunctionCall.Name)
	}
}

func testReasoning(t *testing.T, model llms.Model, opts []llms.CallOption) {
	t.Helper()

	if reasoner, ok := model.(interface{ SupportsReasoning() bool }); ok && !reasoner.SupportsReasoning() {
		t.Skip("Model doesn't support reasoning")
	}

	ctx := context.Background()
	messages := []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{llms.TextPart("What is 25 + 17? Think step by step.")},
		},
	}

	// The reasoning test wants thinking enabled, so its WithThinkingMode comes
	// after the caller opts and overrides any global ThinkingModeNone.
	resp, err := model.GenerateContent(ctx, messages, append(
		withOpts(opts, llms.WithMaxTokens(200)),
		llms.WithThinkingMode(llms.ThinkingModeMedium),
	)...)
	if err != nil {
		t.Fatalf("GenerateContent failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("No choices in response")
	}

	if !strings.Contains(resp.Choices[0].Content, "42") {
		t.Log("Answer might be incorrect (expected 42)")
	}

	if genInfo := resp.Choices[0].GenerationInfo; genInfo != nil {
		if thinkingTokens, ok := genInfo["ThinkingTokens"].(int); ok {
			t.Logf("Used %d thinking tokens", thinkingTokens)
		}
	}
}

func testCaching(t *testing.T, model llms.Model, opts []llms.CallOption) {
	t.Helper()
	ctx := context.Background()

	longContext := strings.Repeat("This is cached context. ", 50)
	messages := []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeSystem,
			Parts: []llms.ContentPart{llms.TextPart(longContext)},
		},
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{llms.TextPart("Say 'OK'")},
		},
	}

	if _, err := model.GenerateContent(ctx, messages, withOpts(opts, llms.WithMaxTokens(10))...); err != nil {
		t.Fatalf("First call failed: %v", err)
	}

	resp2, err := model.GenerateContent(ctx, messages, withOpts(opts, llms.WithMaxTokens(10))...)
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	if genInfo := resp2.Choices[0].GenerationInfo; genInfo != nil {
		if cached, ok := genInfo["CachedTokens"].(int); ok && cached > 0 {
			t.Logf("Cached %d tokens", cached)
		}
	}
}

func testTokenCounting(t *testing.T, model llms.Model, opts []llms.CallOption) {
	t.Helper()
	ctx := context.Background()

	messages := []llms.MessageContent{
		{
			Role:  llms.ChatMessageTypeHuman,
			Parts: []llms.ContentPart{llms.TextPart("Count to 5")},
		},
	}

	resp, err := model.GenerateContent(ctx, messages, withOpts(opts, llms.WithMaxTokens(50))...)
	if err != nil {
		t.Fatalf("GenerateContent failed: %v", err)
	}

	if len(resp.Choices) == 0 {
		t.Fatal("No choices in response")
	}

	genInfo := resp.Choices[0].GenerationInfo
	if genInfo == nil {
		t.Skip("No generation info provided")
	}

	var hasTokenInfo bool
	for _, field := range []string{"TotalTokens", "PromptTokens", "CompletionTokens"} {
		if v, ok := genInfo[field].(int); ok && v > 0 {
			hasTokenInfo = true
			t.Logf("%s: %d", field, v)
		}
	}

	if !hasTokenInfo {
		t.Log("No token counting information provided")
	}
}
