# Kuzco Embedding Options - Phase 3: Integration Test and Docs

**Source PRD**: ../2026-05-31-0859-kuzco-embedding-options.md
**PRD ID**: PRD-2026-05-31-0859
**Phase**: 3 of 3
**Status**: Ready
**Created**: May 31, 2026
**Author**: thetnaingtn

---

## Objective

Prove that configured options actually round-trip through `kuzco → kronk → GGUF` against a real embed-capable model, and update docs so callers discover that embedding options are configured on `kuzco.New` rather than per call.

This phase is gated on Phase 2 (helper + payload merge must exist before integration testing can exercise it).

## Scope

### In Scope

- A new integration test case in `kuzco_embedding_test.go` (under the existing `integration` build tag) that demonstrates `WithEmbeddingTruncate(true)` changes runtime behavior against an over-long input.
- A conditional Matryoshka dimension assertion when the loaded model supports it; otherwise a documented `t.Skip` with the reason.
- A one-line addition to `CLAUDE.md`'s adapter-gotchas section noting that embedding options live on `New`.
- A short usage example in the package-level doc comment in `kuzco.go`.

### Out of Scope

- README creation — no README exists; the user did not ask for one.
- Changing existing integration test infrastructure (kronk lib download, model cache, env-var contract).
- Per-call options (blocked upstream).
- Telemetry, logging, or metrics around option usage.

---

## Inputs

| Input | Source | Notes |
| ----- | ------ | ----- |
| Existing integration harness | `kuzco_embedding_test.go` lines 48-97 | Reuses `libs.New` / `models.Download` / `kronk.Init` setup; build tag `integration`; env vars `EMBED_MODEL_URL` (required), `KUZCO_TEST_CACHE_DIR` and `KRONK_HF_TOKEN` (optional). |
| Default recommended embed model | `kuzco_embedding_test.go` line 25 doc comment | `bge-small-en-v1.5-q8_0.gguf` — **not** Matryoshka-capable. |
| Phase 2 helper | `embeddings.go` `buildEmbedPayload` | End-to-end test exercises the wired path; no need to call the helper directly here. |
| Existing CLAUDE.md gotchas section | `CLAUDE.md` "Adapter Gotchas" | Append one line, do not restructure. |
| kuzco.go package doc | `kuzco.go` line 1 | No package doc comment exists today; add a short one above `package kuzco`. |

## Dependencies

| Dependency | Type | Required Before | Notes |
| ---------- | ---- | --------------- | ----- |
| Phase 2 (helper + payload wiring) | Code | Task 1 | Without the merge, the integration test would prove nothing. |
| `EMBED_MODEL_URL` set in CI / local | External | Task 1 (runtime) | Test skips cleanly when unset; matches the existing pattern. |

---

## Implementation Tasks

### Task 1: Add truncation integration sub-test

- [x] In `kuzco_embedding_test.go`, add a third `t.Run` block `TruncateRoundTrip` inside `TestEmbeddings`.
- [x] Build an input string deliberately longer than the model's context window. For `bge-small-en-v1.5` (512-token context) a 4096-character lorem-ipsum-style string is safely over the limit; generate it inline via `strings.Repeat("the quick brown fox jumps over the lazy dog. ", 200)` or similar.
- [x] Construct a baseline `*kuzco.LLM` with no embedding options and call `CreateEmbedding` with the over-long input. Assert this returns a non-nil error (kronk surfaces context overflow). If kronk silently truncates instead of erroring, log `t.Logf` and skip the rest of the sub-test rather than failing — this is a kronk-side contract we don't own.
- [x] Construct a second `*kuzco.LLM` from the same `*kronk.Kronk` with `kuzco.WithEmbeddingTruncate(true)`. Call `CreateEmbedding` with the same input. Assert no error and a non-empty `[][]float32` with `len(vecs) == 1` and `len(vecs[0]) > 0`.
- [x] Cleanup is already handled by the parent test's `t.Cleanup(k.Unload)` — do not double-unload.

**Acceptance Criteria:**

- When `EMBED_MODEL_URL=<bge-small-en-v1.5>`, the sub-test passes: baseline errors, truncated call succeeds.
- When `EMBED_MODEL_URL` is unset, the entire `TestEmbeddings` (including this sub-test) is skipped — existing behavior preserved.
- No regression in `EmbedDocuments` / `EmbedQuery` sub-tests.

**Files / Areas:**

- `kuzco_embedding_test.go` — new `t.Run("TruncateRoundTrip", ...)` block.

### Task 2: Conditional Matryoshka dimension sub-test

- [x] Add a fourth `t.Run("MatryoshkaDimension", ...)` sub-test that calls `t.Skip("requires a Matryoshka-capable embed model (e.g. nomic-embed-text-v1.5); set EMBED_MODEL_URL accordingly")` **unless** the loaded model is Matryoshka. There is no clean kronk-side feature flag for "is matryoshka"; gate on an opt-in env var `EMBED_MODEL_MATRYOSHKA_DIMS` (comma-separated supported dims) that the test parses and uses to drive a sub-test per dim.
- [x] When the env var is set, for each declared dim `N`, build `*kuzco.LLM` with `kuzco.WithEmbeddingDimension(N)`, call `CreateEmbedding` with a short input, and assert the returned vector length equals `N`.
- [x] Document the env-var contract in the file-level doc comment at the top of `kuzco_embedding_test.go` so future readers know how to opt in.

**Acceptance Criteria:**

- Sub-test skips cleanly against bge-small (the default recommended model).
- When run with a Matryoshka model and `EMBED_MODEL_MATRYOSHKA_DIMS=128,256,512`, all three asserted vector lengths match.

**Files / Areas:**

- `kuzco_embedding_test.go` — new sub-test and a one-paragraph doc-comment addition near the top.

### Task 3: Update `CLAUDE.md` and package doc

- [x] Append one bullet to `CLAUDE.md`'s "Adapter Gotchas" section: "**Embedding options are configured on `kuzco.New`, not per call** — langchaingo's `EmbedderClient.CreateEmbedding` signature has no `Option` parameter, so `WithEmbeddingTruncate`, `WithEmbeddingTruncateDirection`, and `WithEmbeddingDimension` must be passed at construction time."
- [x] Add a package-level doc comment (placed in the existing `doc.go`, which already holds the package doc — see Note below — not in `kuzco.go`):

  ```go
  // Package kuzco adapts a *kronk.Kronk into a langchaingo llms.Model and
  // embeddings.EmbedderClient.
  //
  // Embedding behavior is configured via constructor options on New:
  //
  //	llm := kuzco.New(k,
  //		kuzco.WithEmbeddingTruncate(true),
  //		kuzco.WithEmbeddingTruncateDirection(kuzco.TruncateLeft),
  //		kuzco.WithEmbeddingDimension(256), // Matryoshka downsize
  //	)
  //
  // Per-call options are not supported because langchaingo's
  // EmbedderClient.CreateEmbedding signature does not accept variadic options.
  ```

- [x] Do not create a README — none exists today.

> **Note (deviation):** The spec assumed no package doc comment existed and placed it in `kuzco.go`. One already exists in `doc.go`. Go allows only one canonical package doc comment, so the new overview was added to the existing `doc.go` comment instead of duplicating it in `kuzco.go`. `go doc` renders it correctly.

**Acceptance Criteria:**

- `go doc github.com/thetnaingtn/kuzco` renders the new package overview with the example.
- `CLAUDE.md` shows the new gotcha bullet alongside the existing four.

**Files / Areas:**

- `CLAUDE.md` — one new bullet.
- `doc.go` — updated package doc comment block (the canonical package-doc file).

---

## Verification

### Automated

```bash
# Unit tests (no model needed)
go test -v -count=1 ./...

# Integration with a small public embed model
EMBED_MODEL_URL=https://huggingface.co/CompendiumLabs/bge-small-en-v1.5-gguf/resolve/main/bge-small-en-v1.5-q8_0.gguf \
  go test -tags=integration -v -run TestEmbeddings ./...

# Optional: Matryoshka assertion against nomic-embed-text-v1.5
EMBED_MODEL_URL=<nomic-embed-text-v1.5 gguf URL> \
EMBED_MODEL_MATRYOSHKA_DIMS=128,256,512 \
  go test -tags=integration -v -run TestEmbeddings/MatryoshkaDimension ./...

go vet ./...
```

### Manual

1. Build a small program using the example from the new package doc comment. Confirm it compiles and calls `CreateEmbedding` successfully against a running kronk.
2. Run the program once without `WithEmbeddingTruncate(true)` against an over-long input. Confirm kronk returns a context-overflow error. Re-run with the option and confirm success — matches the PRD's Manual Verification step 3.

---

## Risks

| Risk | Likelihood | Mitigation |
| ---- | ---------- | ---------- |
| Kronk silently truncates instead of erroring on overflow | Medium | The integration test logs and skips the negative-case assertion rather than failing — preserves CI green while still proving the positive case. |
| No public Matryoshka GGUF in CI cache | High | Default recommended model is non-Matryoshka. The dimension sub-test opts in via `EMBED_MODEL_MATRYOSHKA_DIMS`; absence skips cleanly. |
| Integration test slow due to model download | Medium | Reuses the existing `KUZCO_TEST_CACHE_DIR` cache; downloads happen once per machine. |
| Package doc example drifts from real API | Low | Doc comment lives in the same file as the symbols; reviewers will catch mismatches. |

## Open Questions

- Should we add a hardcoded list of known Matryoshka model URL substrings (e.g. "nomic-embed", "mxbai-embed") to auto-detect support, instead of requiring `EMBED_MODEL_MATRYOSHKA_DIMS`? Deferred — the env-var contract is explicit and avoids false positives.
- Is there value in publishing this example under `examples/` as a runnable program? Out of scope for this PRD; revisit if other adapter consumers ask.

## Definition of Done

- [x] All implementation tasks completed
- [x] Acceptance criteria verified
- [x] `go test -v -count=1 ./...` passes (unit)
- [~] `go test -tags=integration -v -run TestEmbeddings ./...` against Qwen3-Embedding-0.6B: `EmbedDocuments`, `EmbedQuery`, and `TruncateRoundTrip` PASS; `MatryoshkaDimension/{128,256,512}` FAIL — kronk returns the native 1024-dim vector regardless of the requested `dimension` (only `/1024` passes). The `dimension` payload field is not honored end-to-end for this model/kronk version; needs follow-up (verify kronk's expected request key vs. llama.cpp Matryoshka support). `truncate` is proven to round-trip.
- [x] `go vet ./...` clean
- [x] `CLAUDE.md` and package doc updated
- [ ] PR merged via Conventional Commit (`feat: embedding options for kronk`)

---

## Handoff Notes

This is the final phase. After merge, move the source PRD from `docs/PRDs/active/` to `docs/PRDs/done/` and update the PRD's **Status** to `Completed`, then fill in the PRD's **Summary** section (currently `_To be filled in after implementation._`) with 2-3 sentences describing what shipped and any deviations from the original plan.

If a follow-up emerges (e.g. per-call options via an upstream langchaingo PR, or auto-detection of Matryoshka models), open a new PRD rather than amending this one.
