package kuzco

import (
	"encoding/json"
	"fmt"

	"github.com/ardanlabs/kronk/sdk/kronk/model"
	"github.com/tmc/langchaingo/llms"
)

func roleToKronk(t llms.ChatMessageType) (string, error) {
	switch t {
	case llms.ChatMessageTypeSystem:
		return "system", nil
	case llms.ChatMessageTypeHuman, llms.ChatMessageTypeGeneric:
		return "user", nil
	case llms.ChatMessageTypeAI:
		return "assistant", nil
	case llms.ChatMessageTypeTool, llms.ChatMessageTypeFunction:
		return "tool", nil
	default:
		return "", fmt.Errorf("kuzco: unsupported role %q", t)
	}
}

func messagesToKronk(msgs []llms.MessageContent) ([]model.D, error) {
	out := make([]model.D, 0, len(msgs))
	for _, m := range msgs {
		role, err := roleToKronk(m.Role)
		if err != nil {
			return nil, err
		}

		var content string
		var toolCalls []model.D

		for _, part := range m.Parts {
			switch p := part.(type) {
			case llms.TextContent:
				content += p.Text
			case llms.ToolCall:
				tc := model.D{
					"id":   p.ID,
					"type": "function",
				}
				if p.FunctionCall != nil {
					tc["function"] = model.D{
						"name":      p.FunctionCall.Name,
						"arguments": p.FunctionCall.Arguments,
					}
				}
				toolCalls = append(toolCalls, tc)
			case llms.ToolCallResponse:
				out = append(out, model.D{
					"role":         "tool",
					"tool_call_id": p.ToolCallID,
					"content":      p.Content,
				})
			case llms.BinaryContent, llms.ImageURLContent:
				return nil, fmt.Errorf("kuzco: unsupported part type %T", part)
			default:
				return nil, fmt.Errorf("kuzco: unsupported part type %T", part)
			}
		}

		if content == "" && len(toolCalls) == 0 {
			continue
		}

		msg := model.D{"role": role}
		if content != "" {
			msg["content"] = content
		}
		if len(toolCalls) > 0 {
			msg["tool_calls"] = toolCalls
		}
		out = append(out, msg)
	}
	return out, nil
}

func toolsToKronk(tools []llms.Tool) []model.D {
	if len(tools) == 0 {
		return nil
	}
	out := make([]model.D, 0, len(tools))
	for _, t := range tools {
		out = append(out, model.D{
			"type": t.Type,
			"function": model.D{
				"name":        t.Function.Name,
				"description": t.Function.Description,
				"parameters":  t.Function.Parameters,
			},
		})
	}
	return out
}

func applyCallOptions(d model.D, opts llms.CallOptions) {
	if opts.MaxTokens > 0 {
		d["max_tokens"] = opts.MaxTokens
	}
	if opts.Temperature != 0 {
		d["temperature"] = opts.Temperature
	}
	if opts.TopP != 0 {
		d["top_p"] = opts.TopP
	}
	if len(opts.StopWords) > 0 {
		d["stop"] = opts.StopWords
	}
	if opts.Seed != 0 {
		d["seed"] = opts.Seed
	}
	if len(opts.Tools) > 0 {
		d["tools"] = toolsToKronk(opts.Tools)
	}
	switch {
	case opts.ToolChoice != nil:
		d["tool_choice"] = opts.ToolChoice
	case len(opts.Tools) > 0:
		// langchaingo callers (including llmtest) commonly pass Tools
		// without setting ToolChoice; default to "auto" so the model
		// is actually prompted to emit tool calls.
		d["tool_choice"] = "auto"
	}
	if opts.StreamingFunc != nil {
		d["stream"] = true
	}
}

func chatResponseToContent(resp model.ChatResponse) *llms.ContentResponse {
	out := &llms.ContentResponse{
		Choices: make([]*llms.ContentChoice, 0, len(resp.Choices)),
	}
	for _, c := range resp.Choices {
		msg := c.Message
		if msg == nil {
			msg = c.Delta
		}
		cc := &llms.ContentChoice{StopReason: c.FinishReason()}
		if msg != nil {
			cc.Content = msg.Content
			cc.ReasoningContent = msg.Reasoning
			for _, tc := range msg.ToolCalls {
				args, _ := json.Marshal(map[string]any(tc.Function.Arguments))
				cc.ToolCalls = append(cc.ToolCalls, llms.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					FunctionCall: &llms.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: string(args),
					},
				})
			}
		}
		if u := resp.Usage; u != nil {
			gi := map[string]any{}
			if u.PromptTokens != 0 {
				gi["PromptTokens"] = u.PromptTokens
			}
			if u.CompletionTokens != 0 {
				gi["CompletionTokens"] = u.CompletionTokens
			}
			if u.TotalTokens != 0 {
				gi["TotalTokens"] = u.TotalTokens
			}
			if len(gi) > 0 {
				cc.GenerationInfo = gi
			}
		}
		out.Choices = append(out.Choices, cc)
	}
	return out
}
