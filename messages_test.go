package kuzco

import (
	"encoding/json"
	"testing"

	"github.com/tmc/langchaingo/llms"
)

func TestMessagesToKronk_SystemAndUser(t *testing.T) {
	msgs := []llms.MessageContent{
		{Role: llms.ChatMessageTypeSystem, Parts: []llms.ContentPart{llms.TextContent{Text: "you are a helper"}}},
		{Role: llms.ChatMessageTypeHuman, Parts: []llms.ContentPart{llms.TextContent{Text: "hi"}}},
	}

	got, err := messagesToKronk(msgs)
	if err != nil {
		t.Fatalf("messagesToKronk: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 messages, got %d", len(got))
	}
	if got[0]["role"] != "system" || got[1]["role"] != "user" {
		t.Fatalf("unexpected roles: %+v", got)
	}
	if _, err := json.Marshal(got); err != nil {
		t.Fatalf("json marshal: %v", err)
	}
}
