package kuzco

import (
	"context"
	"errors"
	"time"

	"github.com/ardanlabs/kronk/sdk/kronk"
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
	return "", errors.New("kuzco: not implemented")
}

func (l *LLM) GenerateContent(ctx context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	return nil, errors.New("kuzco: not implemented")
}

var _ llms.Model = (*LLM)(nil)
