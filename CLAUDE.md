# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`github.com/thetnaingtn/kuzco` — a Go adapter that wraps `github.com/ardanlabs/kronk` (local LLM inference) to satisfy `github.com/tmc/langchaingo`'s `llms.Model` and embedder interfaces. Flat single-package layout at the repo root.

## Commands

- Tests: `go test -v ./...` — run the full suite (unit + integration). Integration tests download GGUF model files and llama.cpp libs at runtime via the `MODEL_URL` env var; they skip cleanly when network is unavailable.
- Build: standard `go build ./...`. No Makefile.

## Workflow

- Work is driven by phased PRDs under `docs/PRDs/`. Each PR typically maps to one phase (see merge history `feat: ...(#N)`). Move spec files between `active/` and `done/` as phases complete.
- Commits use Conventional Commits: `feat:`, `fix:`, `chore:`, etc.

## Adapter Gotchas

These are easy to regress — verify before changing translation code:

- **Use `model.D`, not `map[string]any`** in message/tool translation. Kronk's validators reject plain maps.
- **Default `ToolChoice` to `"auto"`** when `Tools` are present and the caller didn't set one. langchaingo callers usually omit it, and without `"auto"` the model won't emit tool calls.
- **Always ensure a context deadline** via `ensureDeadline` (default 60s). Kronk requires one; passing a deadline-less context will fail.
- **Embeddings** only work against models where `modelInfo.IsEmbedModel` is true.
