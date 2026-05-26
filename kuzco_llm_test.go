//go:build integration

// Package kuzco_test runs the langchaingo llmtest suite against a real
// *kronk.Kronk to verify the adapter end-to-end.
//
// This file is compiled in only when the `integration` build tag is set:
//
//	go test -tags=integration ./...
//
// Without the tag the file is excluded from the build entirely, so the
// default `go test ./...` invocation does not pull in kronk or download
// any GGUFs. With the tag set but `MODEL_URL` unset, TestLLM skips.
//
// The integration test downloads both the native llama.cpp library bundle
// (via kronk's libs downloader, which pins the matching release for the
// vendored kronk version) and the GGUF model (via kronk's models
// downloader) on first run. Subsequent runs hit the on-disk cache.
//
// Required:
//
//	MODEL_URL  Fully qualified HuggingFace download URL of a GGUF model.
//	           When unset, TestLLM is skipped.
//
// Optional:
//
//	KUZCO_TEST_CACHE_DIR  Base directory for cached libs and models. When
//	                      unset, kronk's defaults (~/.kronk/) are used so
//	                      repeat runs reuse prior downloads.
//	KRONK_HF_TOKEN        HuggingFace token for gated models.
//
// Recommended reference model (small, tool-capable, public):
//
//	https://huggingface.co/unsloth/Qwen2.5-1.5B-Instruct-GGUF/resolve/main/Qwen2.5-1.5B-Instruct-Q8_0.gguf
//
// Example:
//
//	MODEL_URL=https://huggingface.co/unsloth/Qwen2.5-1.5B-Instruct-GGUF/resolve/main/Qwen2.5-1.5B-Instruct-Q8_0.gguf \
//	  go test -tags=integration ./... -run TestLLM -v -race
//
// The embedding integration test lives in kuzco_embedding_test.go, shares
// this same build tag, and gates on its own EMBED_MODEL_URL env var.
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
	"github.com/tmc/langchaingo/testing/llmtest"

	"github.com/thetnaingtn/kuzco"
)

func TestLLM(t *testing.T) {
	modelURL := os.Getenv("MODEL_URL")
	if modelURL == "" {
		t.Skip("set MODEL_URL to run llmtest integration")
	}

	ctx := context.Background()
	log := applog.FmtLogger
	cacheDir := os.Getenv("KUZCO_TEST_CACHE_DIR")

	// Download (or reuse cached) llama.cpp library bundle matching the
	// kronk version vendored in go.mod, then point kronk at it.
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

	// Download (or reuse cached) GGUF model from MODEL_URL.
	var mods *models.Models
	if cacheDir != "" {
		mods, err = models.NewWithPaths(cacheDir)
	} else {
		mods, err = models.New()
	}
	if err != nil {
		t.Fatalf("models.New: %v", err)
	}
	mp, err := mods.Download(ctx, log, modelURL)
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

	t.Run("CreateEmbedding_unsupported", func(t *testing.T) {
		llm := kuzco.New(k)
		_, err := llm.CreateEmbedding(ctx, []string{"x"})
		if err == nil {
			t.Fatalf("want error from CreateEmbedding on chat-only model, got nil")
		}
		msg := err.Error()
		if !strings.HasPrefix(msg, "kuzco: embeddings:") {
			t.Logf("err = %v", err)
			t.Fatalf("want error prefixed with %q, got %q", "kuzco: embeddings:", msg)
		}
		if !strings.Contains(msg, "doesn't support embedding") {
			t.Logf("err = %v", err)
			t.Fatalf("want error containing %q, got %q", "doesn't support embedding", msg)
		}
	})

	llmtest.TestLLM(t, kuzco.New(k))
}
