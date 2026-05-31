//go:build integration

// TestEmbeddings exercises the langchaingo embeddings.EmbedderClient
// adapter end-to-end against a real embed-capable GGUF.
//
// Compiled in only under the `integration` build tag (shared with the
// chat integration test in kuzco_llm_test.go; see that file for the
// broader infra contract).
//
// Required:
//
//	EMBED_MODEL_URL  Fully qualified HuggingFace download URL of an
//	                 embed-capable GGUF. When unset, TestEmbeddings is
//	                 skipped.
//
// Optional:
//
//	KUZCO_TEST_CACHE_DIR  Shared cache dir for libs/models — reuse with
//	                      kuzco_llm_test.go so the two runs share kronk's
//	                      lib bundle.
//	KRONK_HF_TOKEN        HuggingFace token for gated models.
//
// Recommended embed model (small, public):
//
//	https://huggingface.co/Qwen/Qwen3-Embedding-0.6B-GGUF/resolve/main/Qwen3-Embedding-0.6B-Q8_0.gguf
//
// Example:
//
//	EMBED_MODEL_URL=https://huggingface.co/Qwen/Qwen3-Embedding-0.6B-GGUF/resolve/main/Qwen3-Embedding-0.6B-Q8_0.gguf \
//	  go test -tags=integration ./... -run TestEmbeddings -v
package kuzco_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/ardanlabs/kronk/sdk/kronk"
	"github.com/ardanlabs/kronk/sdk/kronk/applog"
	kmodel "github.com/ardanlabs/kronk/sdk/kronk/model"
	"github.com/ardanlabs/kronk/sdk/tools/libs"
	"github.com/ardanlabs/kronk/sdk/tools/models"
	"github.com/tmc/langchaingo/embeddings"

	"github.com/thetnaingtn/kuzco"
)

func TestEmbeddings(t *testing.T) {
	embedURL := os.Getenv("EMBED_MODEL_URL")
	if embedURL == "" {
		t.Skip("set EMBED_MODEL_URL to run embeddings integration")
	}

	ctx := context.Background()
	log := applog.FmtLogger
	cacheDir := os.Getenv("KUZCO_TEST_CACHE_DIR")

	libOpts := []libs.Option{}
	if cacheDir != "" {
		libOpts = append(libOpts, libs.WithBasePath(cacheDir))
	}
	lib, err := libs.New(libOpts...)
	if err != nil {
		t.Fatalf("libs.New: %v", err)
	}
	if _, err := lib.Download(ctx, log); err != nil {
		t.Fatalf("libs.Download: %v", err)
	}
	if err := kronk.Init(kronk.WithLibPath(lib.LibsPath())); err != nil {
		t.Fatalf("kronk.Init: %v", err)
	}

	var mods *models.Models
	if cacheDir != "" {
		mods, err = models.NewWithPaths(cacheDir)
	} else {
		mods, err = models.New()
	}
	if err != nil {
		t.Fatalf("models.New: %v", err)
	}
	mp, err := mods.Download(ctx, log, embedURL)
	if err != nil {
		t.Fatalf("models.Download: %v", err)
	}
	if len(mp.ModelFiles) == 0 {
		t.Fatalf("models.Download: no model files returned")
	}

	k, err := kronk.New(kmodel.WithModelFiles(mp.ModelFiles))
	if err != nil {
		t.Fatalf("kronk.New: %v", err)
	}
	t.Cleanup(func() {
		_ = k.Unload(context.Background())
	})

	llm := kuzco.New(k)
	embedder, err := embeddings.NewEmbedder(llm)
	if err != nil {
		t.Fatalf("embeddings.NewEmbedder: %v", err)
	}

	var docDim int

	t.Run("EmbedDocuments", func(t *testing.T) {
		vecs, err := embedder.EmbedDocuments(ctx, []string{"hello", "world"})
		if err != nil {
			t.Fatalf("EmbedDocuments: %v", err)
		}
		if len(vecs) != 2 {
			t.Fatalf("want 2 vectors, got %d", len(vecs))
		}
		if len(vecs[0]) == 0 {
			t.Fatalf("vec[0] is empty")
		}
		if len(vecs[1]) == 0 {
			t.Fatalf("vec[1] is empty")
		}
		if len(vecs[0]) != len(vecs[1]) {
			t.Fatalf("dim mismatch: vec[0]=%d vec[1]=%d", len(vecs[0]), len(vecs[1]))
		}
		docDim = len(vecs[0])
	})

	t.Run("EmbedQuery", func(t *testing.T) {
		vec, err := embedder.EmbedQuery(ctx, "hi")
		if err != nil {
			t.Fatalf("EmbedQuery: %v", err)
		}
		if len(vec) == 0 {
			t.Fatalf("query vector is empty")
		}
		if docDim != 0 && len(vec) != docDim {
			t.Fatalf("dim mismatch: query=%d docs=%d", len(vec), docDim)
		}
	})

	// TruncateRoundTrip proves WithEmbeddingTruncate(true) round-trips through
	// kronk: an over-long input that the unconfigured adapter rejects succeeds
	// once truncation is enabled at construction time.
	t.Run("TruncateRoundTrip", func(t *testing.T) {
		// ~9000 chars — comfortably past any small embed model's context window.
		longInput := strings.Repeat("the quick brown fox jumps over the lazy dog. ", 200)

		baseline := kuzco.New(k)
		if _, err := baseline.CreateEmbedding(ctx, []string{longInput}); err == nil {
			t.Logf("baseline embed of over-long input did not error; kronk may have truncated silently")
			t.Skip("kronk did not surface a context-overflow error; cannot prove the truncate option changes behavior")
		}

		truncating := kuzco.New(k, kuzco.WithEmbeddingTruncate(true))
		vecs, err := truncating.CreateEmbedding(ctx, []string{longInput})
		if err != nil {
			t.Fatalf("CreateEmbedding with WithEmbeddingTruncate(true): %v", err)
		}
		if len(vecs) != 1 {
			t.Fatalf("want 1 vector, got %d", len(vecs))
		}
		if len(vecs[0]) == 0 {
			t.Fatalf("truncated vector is empty")
		}
	})
}
