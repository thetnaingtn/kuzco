package kuzco

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/ardanlabs/kronk/sdk/kronk/model"
	"github.com/tmc/langchaingo/llms"
)

func TestMessagesToKronk(t *testing.T) {
	tests := []struct {
		name    string
		msgs    []llms.MessageContent
		wantErr string
		check   func(t *testing.T, got []model.D)
	}{
		{
			name: "system and user",
			msgs: []llms.MessageContent{
				{Role: llms.ChatMessageTypeSystem, Parts: []llms.ContentPart{llms.TextContent{Text: "you are a helper"}}},
				{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{llms.TextContent{Text: "hi"}}},
			},
			check: func(t *testing.T, got []model.D) {
				if len(got) != 2 {
					t.Fatalf("want 2 messages, got %d", len(got))
				}
				if got[0]["role"] != "system" || got[1]["role"] != "user" {
					t.Fatalf("unexpected roles: %+v", got)
				}
				if got[0]["content"] != "you are a helper" || got[1]["content"] != "hi" {
					t.Fatalf("unexpected content: %+v", got)
				}
			},
		},
		{
			name: "human assistant human chain preserves order",
			msgs: []llms.MessageContent{
				{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{llms.TextContent{Text: "q1"}}},
				{Role: llms.ChatMessageTypeAI, Parts: []llms.ContentPart{llms.TextContent{Text: "a1"}}},
				{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{llms.TextContent{Text: "q2"}}},
			},
			check: func(t *testing.T, got []model.D) {
				wantRoles := []string{"user", "assistant", "user"}
				if len(got) != len(wantRoles) {
					t.Fatalf("want %d messages, got %d", len(wantRoles), len(got))
				}
				for i, r := range wantRoles {
					if got[i]["role"] != r {
						t.Fatalf("message %d: want role %q, got %q", i, r, got[i]["role"])
					}
				}
			},
		},
		{
			name: "assistant with tool call",
			msgs: []llms.MessageContent{
				{Role: llms.ChatMessageTypeAI, Parts: []llms.ContentPart{
					llms.ToolCall{
						ID:   "call_1",
						Type: "function",
						FunctionCall: &llms.FunctionCall{
							Name:      "lookup",
							Arguments: `{"q":"go"}`,
						},
					},
				}},
			},
			check: func(t *testing.T, got []model.D) {
				if len(got) != 1 {
					t.Fatalf("want 1 message, got %d", len(got))
				}
				if got[0]["role"] != "assistant" {
					t.Fatalf("want role assistant, got %v", got[0]["role"])
				}
				tcs, ok := got[0]["tool_calls"].([]model.D)
				if !ok || len(tcs) != 1 {
					t.Fatalf("want 1 tool_call, got %#v", got[0]["tool_calls"])
				}
				tc := tcs[0]
				if tc["id"] != "call_1" || tc["type"] != "function" {
					t.Fatalf("unexpected tool_call header: %+v", tc)
				}
				fn, ok := tc["function"].(model.D)
				if !ok {
					t.Fatalf("missing function map: %+v", tc)
				}
				if fn["name"] != "lookup" || fn["arguments"] != `{"q":"go"}` {
					t.Fatalf("unexpected function fields: %+v", fn)
				}

				// Round-trip JSON and confirm key presence.
				b, err := json.Marshal(got)
				if err != nil {
					t.Fatalf("marshal: %v", err)
				}
				var raw []map[string]any
				if err := json.Unmarshal(b, &raw); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if raw[0]["role"] != "assistant" {
					t.Fatalf("round-trip lost role: %+v", raw[0])
				}
				rtTC := raw[0]["tool_calls"].([]any)[0].(map[string]any)
				rtFn := rtTC["function"].(map[string]any)
				if rtFn["name"] != "lookup" {
					t.Fatalf("round-trip lost function.name: %+v", rtFn)
				}
			},
		},
		{
			name: "tool call response",
			msgs: []llms.MessageContent{
				{Role: llms.ChatMessageTypeTool, Parts: []llms.ContentPart{
					llms.ToolCallResponse{
						ToolCallID: "call_1",
						Name:       "lookup",
						Content:    "answer",
					},
				}},
			},
			check: func(t *testing.T, got []model.D) {
				if len(got) != 1 {
					t.Fatalf("want 1 message, got %d", len(got))
				}
				if got[0]["role"] != "tool" {
					t.Fatalf("want role tool, got %v", got[0]["role"])
				}
				if got[0]["tool_call_id"] != "call_1" {
					t.Fatalf("want tool_call_id=call_1, got %v", got[0]["tool_call_id"])
				}
				if got[0]["content"] != "answer" {
					t.Fatalf("want content=answer, got %v", got[0]["content"])
				}
			},
		},
		{
			name: "binary content unsupported",
			msgs: []llms.MessageContent{
				{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{
					llms.BinaryContent{MIMEType: "image/png", Data: []byte{0x89}},
				}},
			},
			wantErr: "unsupported part",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := messagesToKronk(tc.msgs)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("messagesToKronk: %v", err)
			}
			tc.check(t, got)
		})
	}
}

func TestToolsToKronk(t *testing.T) {
	if got := toolsToKronk(nil); got != nil {
		t.Fatalf("nil input: want nil, got %v", got)
	}
	if got := toolsToKronk([]llms.Tool{}); got != nil {
		t.Fatalf("empty input: want nil, got %v", got)
	}

	tools := []llms.Tool{{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "lookup",
			Description: "look something up",
			Parameters:  map[string]any{"type": "object"},
		},
	}}
	got := toolsToKronk(tools)
	if len(got) != 1 {
		t.Fatalf("want 1 tool, got %d", len(got))
	}
	if got[0]["type"] != "function" {
		t.Fatalf("want type=function, got %v", got[0]["type"])
	}
	if _, ok := got[0]["function"]; !ok {
		t.Fatalf("missing function key: %+v", got[0])
	}
}

func TestApplyCallOptions(t *testing.T) {
	t.Run("empty options", func(t *testing.T) {
		d := model.D{}
		applyCallOptions(d, llms.CallOptions{})
		if len(d) != 0 {
			t.Fatalf("want empty d, got %+v", d)
		}
	})

	t.Run("all fields set", func(t *testing.T) {
		d := model.D{}
		tools := []llms.Tool{{
			Type:     "function",
			Function: &llms.FunctionDefinition{Name: "lookup"},
		}}
		applyCallOptions(d, llms.CallOptions{
			MaxTokens:   128,
			Temperature: 0.7,
			TopP:        0.9,
			StopWords:   []string{"\n\n"},
			Seed:        42,
			Tools:       tools,
			ToolChoice:  "auto",
		})

		want := map[string]any{
			"max_tokens":  128,
			"temperature": 0.7,
			"top_p":       0.9,
			"seed":        42,
			"tool_choice": "auto",
		}
		for k, v := range want {
			if !reflect.DeepEqual(d[k], v) {
				t.Fatalf("d[%q]: want %v, got %v", k, v, d[k])
			}
		}
		if !reflect.DeepEqual(d["stop"], []string{"\n\n"}) {
			t.Fatalf("d[stop]: got %v", d["stop"])
		}
		ts, ok := d["tools"].([]model.D)
		if !ok || len(ts) != 1 {
			t.Fatalf("d[tools] not a non-empty slice: %#v", d["tools"])
		}
	})

	t.Run("streaming func sets stream true", func(t *testing.T) {
		d := model.D{}
		applyCallOptions(d, llms.CallOptions{
			StreamingFunc: func(_ context.Context, _ []byte) error { return nil },
		})
		if d["stream"] != true {
			t.Fatalf("want stream=true, got %v", d["stream"])
		}
	})

	t.Run("nil streaming func leaves stream unset", func(t *testing.T) {
		d := model.D{}
		applyCallOptions(d, llms.CallOptions{})
		if _, ok := d["stream"]; ok {
			t.Fatalf("want stream unset, got %v", d["stream"])
		}
	})

	t.Run("tools set produces non-empty d[tools] and defaults tool_choice to auto", func(t *testing.T) {
		d := model.D{}
		applyCallOptions(d, llms.CallOptions{
			Tools: []llms.Tool{{
				Type:     "function",
				Function: &llms.FunctionDefinition{Name: "lookup"},
			}},
		})
		ts, ok := d["tools"].([]model.D)
		if !ok || len(ts) == 0 {
			t.Fatalf("want non-empty d[tools], got %#v", d["tools"])
		}
		if d["tool_choice"] != "auto" {
			t.Fatalf("want tool_choice=auto when Tools set without explicit choice, got %v", d["tool_choice"])
		}
	})

	t.Run("explicit tool_choice wins over default", func(t *testing.T) {
		d := model.D{}
		applyCallOptions(d, llms.CallOptions{
			Tools: []llms.Tool{{
				Type:     "function",
				Function: &llms.FunctionDefinition{Name: "lookup"},
			}},
			ToolChoice: "required",
		})
		if d["tool_choice"] != "required" {
			t.Fatalf("want tool_choice=required, got %v", d["tool_choice"])
		}
	})

	t.Run("no tools means no tool_choice default", func(t *testing.T) {
		d := model.D{}
		applyCallOptions(d, llms.CallOptions{MaxTokens: 1})
		if _, ok := d["tool_choice"]; ok {
			t.Fatalf("want tool_choice unset when no tools, got %v", d["tool_choice"])
		}
	})
}

func TestApplyThinking(t *testing.T) {
	// optsWithMode builds CallOptions carrying the thinking config the way
	// langchaingo callers do: by applying the WithThinkingMode option.
	optsWithMode := func(mode llms.ThinkingMode) llms.CallOptions {
		co := llms.CallOptions{}
		llms.WithThinkingMode(mode)(&co)
		return co
	}

	t.Run("modes map to kronk reasoning controls", func(t *testing.T) {
		cases := []struct {
			mode         llms.ThinkingMode
			wantEnable   string
			wantReasonEf string
		}{
			{llms.ThinkingModeNone, "false", model.ReasoningEffortNone},
			{llms.ThinkingModeLow, "true", model.ReasoningEffortLow},
			{llms.ThinkingModeMedium, "true", model.ReasoningEffortMedium},
			{llms.ThinkingModeHigh, "true", model.ReasoningEffortHigh},
		}
		for _, tc := range cases {
			t.Run(string(tc.mode), func(t *testing.T) {
				d := model.D{}
				co := optsWithMode(tc.mode)
				applyThinking(d, &co)

				if d["enable_thinking"] != tc.wantEnable {
					t.Fatalf("enable_thinking: want %q, got %v", tc.wantEnable, d["enable_thinking"])
				}
				if d["reasoning_effort"] != tc.wantReasonEf {
					t.Fatalf("reasoning_effort: want %q, got %v", tc.wantReasonEf, d["reasoning_effort"])
				}
			})
		}
	})

	t.Run("auto leaves kronk defaults untouched", func(t *testing.T) {
		d := model.D{}
		co := optsWithMode(llms.ThinkingModeAuto)
		applyThinking(d, &co)
		if _, ok := d["enable_thinking"]; ok {
			t.Fatalf("want enable_thinking unset for auto, got %v", d["enable_thinking"])
		}
		if _, ok := d["reasoning_effort"]; ok {
			t.Fatalf("want reasoning_effort unset for auto, got %v", d["reasoning_effort"])
		}
	})

	t.Run("no thinking config is a no-op", func(t *testing.T) {
		d := model.D{}
		co := llms.CallOptions{}
		applyThinking(d, &co)
		if len(d) != 0 {
			t.Fatalf("want empty d when no thinking config, got %+v", d)
		}
	})

	t.Run("applied via applyCallOptions", func(t *testing.T) {
		d := model.D{}
		co := optsWithMode(llms.ThinkingModeNone)
		applyCallOptions(d, co)
		if d["enable_thinking"] != "false" {
			t.Fatalf("enable_thinking: want %q, got %v", "false", d["enable_thinking"])
		}
		if d["reasoning_effort"] != model.ReasoningEffortNone {
			t.Fatalf("reasoning_effort: want %q, got %v", model.ReasoningEffortNone, d["reasoning_effort"])
		}
	})
}

func TestChatResponseToContent(t *testing.T) {
	t.Run("text choice with usage", func(t *testing.T) {
		finish := "stop"
		resp := model.ChatResponse{
			Choices: []model.Choice{{
				Message:         &model.ResponseMessage{Role: "assistant", Content: "hello"},
				FinishReasonPtr: &finish,
			}},
			Usage: &model.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}
		out := chatResponseToContent(resp)
		if len(out.Choices) != 1 {
			t.Fatalf("want 1 choice, got %d", len(out.Choices))
		}
		c := out.Choices[0]
		if c.Content != "hello" {
			t.Fatalf("want content=hello, got %q", c.Content)
		}
		if c.StopReason != "stop" {
			t.Fatalf("want StopReason=stop, got %q", c.StopReason)
		}
		// llmtest's testTokenCounting looks up exactly these keys.
		for _, key := range []string{"PromptTokens", "CompletionTokens", "TotalTokens"} {
			v, ok := c.GenerationInfo[key]
			if !ok {
				t.Fatalf("GenerationInfo missing %q: %+v", key, c.GenerationInfo)
			}
			if _, ok := v.(int); !ok {
				t.Fatalf("GenerationInfo[%q] not int: %T", key, v)
			}
		}
	})

	t.Run("tool call choice", func(t *testing.T) {
		resp := model.ChatResponse{
			Choices: []model.Choice{{
				Message: &model.ResponseMessage{
					Role: "assistant",
					ToolCalls: []model.ResponseToolCall{{
						ID:   "call_1",
						Type: "function",
						Function: model.ResponseToolCallFunction{
							Name:      "lookup",
							Arguments: model.ToolCallArguments{"q": "go"},
						},
					}},
				},
			}},
		}
		out := chatResponseToContent(resp)
		if len(out.Choices) != 1 {
			t.Fatalf("want 1 choice, got %d", len(out.Choices))
		}
		tcs := out.Choices[0].ToolCalls
		if len(tcs) != 1 {
			t.Fatalf("want 1 tool call, got %d", len(tcs))
		}
		tc := tcs[0]
		if tc.ID != "call_1" || tc.Type != "function" {
			t.Fatalf("unexpected header: %+v", tc)
		}
		if tc.FunctionCall == nil || tc.FunctionCall.Name != "lookup" {
			t.Fatalf("unexpected function: %+v", tc.FunctionCall)
		}
		var args map[string]any
		if err := json.Unmarshal([]byte(tc.FunctionCall.Arguments), &args); err != nil {
			t.Fatalf("arguments not valid JSON: %v (%q)", err, tc.FunctionCall.Arguments)
		}
		if args["q"] != "go" {
			t.Fatalf("want arguments.q=go, got %v", args["q"])
		}
	})

	t.Run("empty choices does not panic", func(t *testing.T) {
		out := chatResponseToContent(model.ChatResponse{})
		if out == nil {
			t.Fatalf("want non-nil *ContentResponse")
		}
		if len(out.Choices) != 0 {
			t.Fatalf("want empty Choices, got %d", len(out.Choices))
		}
	})
}
