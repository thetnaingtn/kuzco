package kuzco

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ardanlabs/kronk/sdk/kronk"
	"github.com/ardanlabs/kronk/sdk/kronk/model"
	"github.com/tmc/langchaingo/llms"
)

type LLM struct {
	k              *kronk.Kronk
	defaultTimeout time.Duration
}

type Option func(*LLM)

func WithDefaultTimeout(d time.Duration) Option {
	return func(l *LLM) {
		l.defaultTimeout = d
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

	ctx, cancel := l.ensureDeadline(ctx)
	defer cancel()

	resp, err := l.k.Chat(ctx, d)
	if err != nil {
		return nil, fmt.Errorf("kuzco: chat: %w", err)
	}

	return chatResponseToContent(resp), nil
}

func (l *LLM) ensureDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, l.defaultTimeout)
}

var _ llms.Model = (*LLM)(nil)
