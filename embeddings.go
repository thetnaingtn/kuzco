package kuzco

import (
	"errors"
	"sort"

	"github.com/ardanlabs/kronk/sdk/kronk/model"
)

var errEmptyInput = errors.New("kuzco: embeddings: texts must not be empty")

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
