package kuzco

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ardanlabs/kronk/sdk/kronk"
	"github.com/ardanlabs/kronk/sdk/kronk/model"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms"
)

var (
	_ llms.Model                = (*LLM)(nil)
	_ embeddings.EmbedderClient = (*LLM)(nil)
)

// TruncateDirection selects which end kronk truncates when embedding input
// exceeds the model's context. Values match kronk's documented strings
// (see kronk/sdk/kronk/embedding.go).
type TruncateDirection string

const (
	TruncateRight TruncateDirection = "right"
	TruncateLeft  TruncateDirection = "left"
)

// embedOpts holds the embedding-specific options written by the WithEmbedding*
// constructor options. It is read when building the kronk embedding payload.
// See more details at https://pkg.go.dev/github.com/ardanlabs/kronk/sdk/kronk#Kronk.Embeddings
type embedOpts struct {
	truncate          *bool // nil = unset, distinct from set-to-false
	truncateDirection TruncateDirection
	dimension         int
}

type LLM struct {
	k              *kronk.Kronk
	defaultTimeout time.Duration
	embed          embedOpts
}

type Option func(*LLM)

func WithDefaultTimeout(d time.Duration) Option {
	return func(l *LLM) {
		l.defaultTimeout = d
	}
}

// WithEmbeddingTruncate controls whether kronk truncates embedding input that
// exceeds the model's context. The value is stored as a pointer so that an
// explicit false is distinguishable from the option never being set.
func WithEmbeddingTruncate(v bool) Option {
	return func(l *LLM) {
		l.embed.truncate = &v
	}
}

// WithEmbeddingTruncateDirection selects which end kronk truncates. Invalid
// directions are a silent no-op.
func WithEmbeddingTruncateDirection(d TruncateDirection) Option {
	return func(l *LLM) {
		if d == TruncateRight || d == TruncateLeft {
			l.embed.truncateDirection = d
		}
	}
}

// WithEmbeddingDimension sets the output embedding dimension. Non-positive
// values are a silent no-op.
func WithEmbeddingDimension(n int) Option {
	return func(l *LLM) {
		if n > 0 {
			l.embed.dimension = n
		}
	}
}

func New(k *kronk.Kronk, opts ...Option) *LLM {
	l := &LLM{
		k:              k,
		defaultTimeout: 60 * time.Second,
	}
	for _, o := range opts {
		o(l)
	}
	return l
}

func (l *LLM) Call(ctx context.Context, prompt string, options ...llms.CallOption) (string, error) {
	resp, err := l.GenerateContent(ctx, []llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	}, options...)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", errors.New("kuzco: empty response")
	}
	return resp.Choices[0].Content, nil
}

func (l *LLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
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
	defer cancel()

	in, err := l.k.ChatStreaming(ctx, d)
	if err != nil {
		return nil, fmt.Errorf("kuzco: chat: %w", err)
	}

	msg := &model.ResponseMessage{Role: "assistant"}
	final := model.ChatResponse{Choices: []model.Choice{{Message: msg}}}
	var seeded bool

	for chunk := range in {
		if !seeded && chunk.ID != "" {
			final.ID = chunk.ID
			final.Object = chunk.Object
			final.Created = chunk.Created
			final.Model = chunk.Model
			final.SystemFingerprint = chunk.SystemFingerprint
			seeded = true
		}

		if chunk.Usage != nil {
			final.Usage = chunk.Usage
		}

		if len(chunk.Choices) == 0 {
			continue
		}
		c := chunk.Choices[0]

		if c.FinishReasonPtr != nil {
			final.Choices[0].FinishReasonPtr = c.FinishReasonPtr
		}

		if c.Delta != nil {
			msg.Content += c.Delta.Content
			msg.Reasoning += c.Delta.Reasoning

			if co.StreamingFunc != nil && c.Delta.Content != "" {
				if err := co.StreamingFunc(ctx, []byte(c.Delta.Content)); err != nil {
					cancel()
					return nil, fmt.Errorf("kuzco: chat: streaming-func: %w", err)
				}
			}
		}

		if c.Message != nil {
			if c.Message.Content != "" {
				msg.Content = c.Message.Content
			}
			if c.Message.Reasoning != "" {
				msg.Reasoning = c.Message.Reasoning
			}
			if len(c.Message.ToolCalls) > 0 {
				msg.ToolCalls = c.Message.ToolCalls
			}
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("kuzco: chat: %w", err)
	}

	return chatResponseToContent(final), nil
}

func (l *LLM) ensureDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, l.defaultTimeout)
}
