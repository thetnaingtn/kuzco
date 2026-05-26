package kuzco

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/ardanlabs/kronk/sdk/kronk/model"
)

var errEmptyInput = errors.New("kuzco: embeddings: texts must not be empty")

func (l *LLM) CreateEmbedding(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, errEmptyInput
	}
	ctx, cancel := l.ensureDeadline(ctx)
	defer cancel()
	resp, err := l.k.Embeddings(ctx, model.D{"input": texts})
	if err != nil {
		return nil, fmt.Errorf("kuzco: embeddings: %w", err)
	}
	return embedResponseToVectors(resp), nil
}

func embedResponseToVectors(resp model.EmbedReponse) [][]float32 {
	data := make([]model.EmbedData, len(resp.Data))
	copy(data, resp.Data)
	sort.Slice(data, func(i, j int) bool { return data[i].Index < data[j].Index })

	out := make([][]float32, len(data))
	for i := range data {
		out[i] = data[i].Embedding
	}
	return out
}
