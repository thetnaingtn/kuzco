package kuzco

import (
	"context"
	"fmt"

	"github.com/ardanlabs/kronk/sdk/kronk/model"
	"github.com/tmc/langchaingo/llms"
)

func (l *LLM) GenerateContentStream(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (<-chan llms.ContentResponse, error) {
	co := llms.CallOptions{}
	for _, opt := range options {
		opt(&co)
	}

	kmsgs, err := messagesToKronk(messages)
	if err != nil {
		return nil, err
	}

	d := model.D{}
	d["messages"] = kmsgs
	applyCallOptions(d, co)
	d["stream"] = true

	ctx, cancel := l.ensureDeadline(ctx)

	in, err := l.k.ChatStreaming(ctx, d)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("kuzco: chat-streaming: %w", err)
	}

	out := make(chan llms.ContentResponse)

	go func() {
		defer cancel()
		defer close(out)

		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-in:
				if !ok {
					return
				}

				if co.StreamingFunc != nil {
					if delta := chunkDelta(chunk); delta != "" {
						if err := co.StreamingFunc(ctx, []byte(delta)); err != nil {
							return
						}
					}
				}

				resp := chatResponseToContent(chunk)
				select {
				case <-ctx.Done():
					return
				case out <- *resp:
				}
			}
		}
	}()

	return out, nil
}

func chunkDelta(resp model.ChatResponse) string {
	if len(resp.Choices) == 0 {
		return ""
	}
	if d := resp.Choices[0].Delta; d != nil {
		return d.Content
	}
	return ""
}
