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
	d := l.buildEmbedPayload(texts)
	resp, err := l.k.Embeddings(ctx, d)
	if err != nil {
		return nil, fmt.Errorf("kuzco: embeddings: %w", err)
	}
	return embedResponseToVectors(resp), nil
}

// buildEmbedPayload materialises the kronk embedding request map for texts and
// merges any configured embedding options. The key names match kronk's
// documented strings (see kronk/sdk/kronk/embedding.go). When no options are
// set the payload is exactly model.D{"input": texts}.
func (l *LLM) buildEmbedPayload(texts []string) model.D {
	d := model.D{"input": texts}
	if l.embed.truncate != nil {
		d["truncate"] = *l.embed.truncate
	}
	if l.embed.truncateDirection != "" {
		d["truncate_direction"] = string(l.embed.truncateDirection)
	}
	return d
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
