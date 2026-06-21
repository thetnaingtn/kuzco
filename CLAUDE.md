# CLAUDE.md

## Project

`github.com/thetnaingtn/kuzco` — a Go adapter that wraps `github.com/ardanlabs/kronk` (local LLM inference) to satisfy `github.com/tmc/langchaingo`'s `llms.Model` and embedder interfaces. Flat single-package layout at the repo root.

## Commands

- Use `make run-tests` to run all the unit tests.
- Use `make run-llm-test` to run LLM model integration test. Use this when any changes related to kuzco implmentation of llms.Model interface was made.
- Use `make run-embedding-test` to run embedding integration test. Use this when any changes related to kuzco implementation of embeddings.EmbedderClient was made


## Workflow

- TDD is mendatory. No Exception.
- Work is driven by phased PRDs under `docs/PRDs/`. Each PR typically maps to one phase (see merge history `feat: ...(#N)`). Move spec files between `active/` and `done/` as phases complete.
- Commits use Conventional Commits: `feat:`, `fix:`, `chore:`, etc.
- **Always** use the *Explore* subagent when looking for specific kronk or langchaingo functions, files, methods etc.
- After a phase is successfully implemented, update it **Status** to **Completed**. A PRD **Status** have to update to **Completed** after all of its phases are **Completed** and update it summary.
- Update the README.md when new configuration options was added, implement new interface etc.

## Adapter Gotchas

These are easy to regress — verify before changing translation code:

- **Use `model.D`, not `map[string]any`** in message/tool translation. Kronk's validators reject plain maps.
- **Default `ToolChoice` to `"auto"`** when `Tools` are present and the caller didn't set one. langchaingo callers usually omit it, and without `"auto"` the model won't emit tool calls.
- **Always ensure a context deadline** via `ensureDeadline` (default 60s). Kronk requires one; passing a deadline-less context will fail.
- **Embeddings** only work against models where `modelInfo.IsEmbedModel` is true.
- **Embedding options are configured on `kuzco.New`, not per call** — langchaingo's `EmbedderClient.CreateEmbedding` signature has no `Option` parameter, so `WithEmbeddingTruncate` and `WithEmbeddingTruncateDirection` must be passed at construction time.
