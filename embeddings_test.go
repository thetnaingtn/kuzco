package kuzco

import (
	"reflect"
	"testing"

	"github.com/ardanlabs/kronk/sdk/kronk/model"
	"github.com/google/go-cmp/cmp"
)

func TestBuildEmbedPayload(t *testing.T) {
	texts := []string{"hello", "world"}

	tests := []struct {
		name string
		opts []Option
		want model.D
	}{
		{
			name: "no opts",
			opts: nil,
			want: model.D{"input": texts},
		},
		{
			name: "truncate true only",
			opts: []Option{WithEmbeddingTruncate(true)},
			want: model.D{"input": texts, "truncate": true},
		},
		{
			name: "truncate false only",
			opts: []Option{WithEmbeddingTruncate(false)},
			want: model.D{"input": texts, "truncate": false},
		},
		{
			name: "direction left only",
			opts: []Option{WithEmbeddingTruncateDirection(TruncateLeft)},
			want: model.D{"input": texts, "truncate_direction": "left"},
		},
		{
			name: "direction right only",
			opts: []Option{WithEmbeddingTruncateDirection(TruncateRight)},
			want: model.D{"input": texts, "truncate_direction": "right"},
		},
		{
			name: "dimension 256 only",
			opts: []Option{WithEmbeddingDimension(256)},
			want: model.D{"input": texts, "dimension": 256},
		},
		{
			name: "all three combined",
			opts: []Option{
				WithEmbeddingTruncate(true),
				WithEmbeddingTruncateDirection(TruncateLeft),
				WithEmbeddingDimension(256),
			},
			want: model.D{
				"input":              texts,
				"truncate":           true,
				"truncate_direction": "left",
				"dimension":          256,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := New(nil, tc.opts...)
			got := l.buildEmbedPayload(texts)

			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("buildEmbedPayload mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEmbedResponseToVectors(t *testing.T) {
	tests := []struct {
		name string
		in   model.EmbedReponse
		want [][]float32
	}{
		{
			name: "empty data",
			in:   model.EmbedReponse{},
			want: [][]float32{},
		},
		{
			name: "single row",
			in: model.EmbedReponse{Data: []model.EmbedData{
				{Index: 0, Embedding: []float32{0.1}},
			}},
			want: [][]float32{{0.1}},
		},
		{
			name: "two rows in order",
			in: model.EmbedReponse{Data: []model.EmbedData{
				{Index: 0, Embedding: []float32{0.1, 0.2}},
				{Index: 1, Embedding: []float32{0.3, 0.4}},
			}},
			want: [][]float32{{0.1, 0.2}, {0.3, 0.4}},
		},
		{
			name: "two rows reversed by index",
			in: model.EmbedReponse{Data: []model.EmbedData{
				{Index: 1, Embedding: []float32{0.3, 0.4}},
				{Index: 0, Embedding: []float32{0.1, 0.2}},
			}},
			want: [][]float32{{0.1, 0.2}, {0.3, 0.4}},
		},
		{
			name: "three rows mixed index",
			in: model.EmbedReponse{Data: []model.EmbedData{
				{Index: 2, Embedding: []float32{0.5}},
				{Index: 0, Embedding: []float32{0.1}},
				{Index: 1, Embedding: []float32{0.3}},
			}},
			want: [][]float32{{0.1}, {0.3}, {0.5}},
		},
		{
			name: "per-row dimensionality preserved",
			in: model.EmbedReponse{Data: []model.EmbedData{
				{Index: 0, Embedding: []float32{0.1, 0.2, 0.3}},
				{Index: 1, Embedding: []float32{0.4, 0.5, 0.6, 0.7, 0.8}},
			}},
			want: [][]float32{{0.1, 0.2, 0.3}, {0.4, 0.5, 0.6, 0.7, 0.8}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := embedResponseToVectors(tc.in)
			if got == nil {
				t.Fatalf("want non-nil result, got nil")
			}
			if len(got) != len(tc.want) {
				t.Fatalf("want %d rows, got %d", len(tc.want), len(got))
			}
			for i := range tc.want {
				if !reflect.DeepEqual(got[i], tc.want[i]) {
					t.Fatalf("row %d: want %v, got %v", i, tc.want[i], got[i])
				}
			}
		})
	}
}

func TestEmbedResponseToVectorsDoesNotMutateInput(t *testing.T) {
	in := model.EmbedReponse{Data: []model.EmbedData{
		{Index: 2, Embedding: []float32{0.5}},
		{Index: 0, Embedding: []float32{0.1}},
		{Index: 1, Embedding: []float32{0.3}},
	}}
	_ = embedResponseToVectors(in)

	wantOrder := []int{2, 0, 1}
	for i, idx := range wantOrder {
		if in.Data[i].Index != idx {
			t.Fatalf("input mutated at %d: want Index=%d, got %d", i, idx, in.Data[i].Index)
		}
	}
}

func TestErrEmptyInput(t *testing.T) {
	const want = "kuzco: embeddings: texts must not be empty"
	if errEmptyInput == nil {
		t.Fatalf("errEmptyInput is nil")
	}
	if errEmptyInput.Error() != want {
		t.Fatalf("want %q, got %q", want, errEmptyInput.Error())
	}
}
