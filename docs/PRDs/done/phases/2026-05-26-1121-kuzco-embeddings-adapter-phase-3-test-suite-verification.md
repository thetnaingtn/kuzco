# Kuzco: langchaingo embeddings.EmbedderClient Adapter - Phase 3: Test Suite & Verification

**Source PRD**: docs/PRDs/active/2026-05-26-1121-kuzco-embeddings-adapter.md
**PRD ID**: PRD-2026-05-26-1121
**Phase**: 3 of 3
**Status**: Completed
**Created**: May 26, 2026
**Author**: thetnaingtn

---

## Objective

Stand up the embedding integration test **and** unify integration-test gating across the package by switching from "env-var only" to `//go:build integration` plus env vars. Both the existing chat test (`kuzco_test.go`) and the new embedding test move behind the same build tag. The result: `go test ./...` runs only fast model-free units by default; `go test -tags=integration ./...` opts into the heavyweight end-to-end tests, with per-test env vars (`MODEL_URL`, `EMBED_MODEL_URL`) still acting as fine-grained skip gates inside the tagged build.

## Scope

### In Scope

- New `embeddings_integration_test.go` (build tag `integration` + env gate `EMBED_MODEL_URL`) exercising `embeddings.NewEmbedder(kuzco.New(k))` end-to-end against a real embed-capable GGUF.
- Negative-path sub-test: when the configured kronk model is chat-only, `CreateEmbedding` surfaces kronk's `"model doesn't support embedding"` error wrapped with `kuzco: embeddings:`. Cheapest implementation: piggyback on the existing chat integration test's already-loaded model.
- Migrate existing `kuzco_test.go` to `//go:build integration` so chat and embedding integration tests share one opt-in mechanism.
- Extract `TestCompile` from `kuzco_test.go` into a new untagged file (`kuzco_compile_test.go`) so the default `go test ./...` still has a compile + interface-assertion smoke test.
- Update the package-level doc comment to document the new invocation pattern (`go test -tags=integration ./...`).
- Confirm `go vet ./...` and both `go test ./...` and `go test -tags=integration ./...` are clean.

### Out of Scope

- Performance / latency benchmarks.
- Vector-store round-trips through chroma / pgvector (recommended in PRD manual-verification but not part of automated tests).
- Reranking and dimensions-reduction integration tests (deferred).
- Multi-build-tag schemes (e.g. separate `integration_chat` / `integration_embed` tags) — single `integration` tag is sufficient since env vars already provide per-test gating.

---

## Inputs

| Input | Source | Notes |
| ----- | ------ | ----- |
| Existing `kuzco_test.go` integration pattern | Repo root | `libs.New()` + `models.New()` + env-gated `MODEL_URL`; kronk lib bundle resolved per host. |
| `EMBED_MODEL_URL` (new env var) | PRD decision | Holds a URL to an embed-capable GGUF (e.g. `bge-small-en-v1.5-q8_0.gguf` or `nomic-embed-text-v1.5-Q8_0.gguf`). |
| Build-tag convention | Go toolchain | `//go:build integration` as the first non-blank line of each integration file (immediately before `package`). |
| `embeddings.NewEmbedder` | `github.com/tmc/langchaingo/embeddings/embedding.go:13` | Returns `*EmbedderImpl`; `EmbedDocuments` / `EmbedQuery` are the call sites under test. |
| Kronk chat-only error: `"embeddings: model doesn't support embedding"` | `sdk/kronk/model/embed.go:24` | Expected substring for the negative-path assertion. |

## Dependencies

| Dependency | Type | Required Before | Notes |
| ---------- | ---- | --------------- | ----- |
| Phase 2 complete | Code | All tasks | `CreateEmbedding` and interface assertion must compile under both the default and `integration` builds. |
| Network access to GGUF host | External | Task 1, Task 3 | Tests skip when their env var is empty even with the tag set. |
| Embed-capable GGUF + matching llama.cpp bundle | External | Task 1 | Resolved via kronk's `libs.New()`. |

---

## Implementation Tasks

### Task 1: Extract `TestCompile` into an untagged file

- [ ] Create `kuzco_compile_test.go` (no build tag) containing:
  ```go
  package kuzco_test

  import "testing"

  // TestCompile asserts the package compiles and interface assertions
  // in kuzco.go (llms.Model + embeddings.EmbedderClient) hold even when
  // the integration build tag is off.
  func TestCompile(t *testing.T) {}
  ```
- [ ] Remove `TestCompile` (and its doc comment) from `kuzco_test.go`.

**Acceptance Criteria:**

- `go test ./...` (no tag) runs `TestCompile` and any other unit tests; reports `ok` without invoking kronk.
- The `TestCompile` symbol is no longer referenced inside `kuzco_test.go`.

**Files / Areas:**

- `kuzco_compile_test.go` (new) - one-line no-op test.
- `kuzco_test.go` - removal of `TestCompile` definition.

### Task 2: Move `kuzco_test.go` behind the `integration` build tag

- [ ] Add `//go:build integration` as the first line of `kuzco_test.go` (blank line, then `package kuzco_test`).
- [ ] Update the package-level doc comment in `kuzco_test.go` to describe the new invocation:
  - Document `go test -tags=integration ./...` as the entry point.
  - Keep the existing env-var sections (`MODEL_URL`, `KUZCO_TEST_CACHE_DIR`, `KRONK_HF_TOKEN`); clarify that without the tag the file is excluded from the build entirely, and that with the tag but without `MODEL_URL` the test skips.
  - Add a note that the embedding integration test lives in `embeddings_integration_test.go` and shares the same tag plus its own `EMBED_MODEL_URL` env var.
- [ ] No body changes required to `TestLLM` itself.

**Acceptance Criteria:**

- `go test ./...` (no tag) does NOT compile `kuzco_test.go` (verify by temporarily breaking a line inside the file and confirming the default build still passes; revert).
- `MODEL_URL=... go test -tags=integration ./... -run TestLLM -v` still passes against the chat GGUF.
- `go test -tags=integration ./... -run TestLLM -v` (env unset) reports `SKIP`.

**Files / Areas:**

- `kuzco_test.go` - add build tag at top; refresh doc comment.

### Task 3: New `embeddings_integration_test.go` behind the same tag

- [ ] Create `embeddings_integration_test.go` with:
  ```go
  //go:build integration

  package kuzco_test
  ```
- [ ] Mirror the kronk bootstrap pattern from `kuzco_test.go`:
  - Read `EMBED_MODEL_URL`; `t.Skip` when empty.
  - Reuse `KUZCO_TEST_CACHE_DIR` so chat and embedding runs share kronk's library/model caches.
  - `libs.New()` + `lib.Download` + `kronk.Init(WithLibPath(...))` (safe to call again — kronk's `Init` is idempotent given the same lib path).
  - `models.New[WithPaths]()` + `mods.Download(ctx, log, embedURL)` → `mp.ModelFiles`.
  - `kronk.New(kmodel.WithModelFiles(mp.ModelFiles))` → defer `Unload`.
- [ ] Add a top-level `TestEmbeddings` that wraps with `llm := kuzco.New(k)` and `embedder, err := embeddings.NewEmbedder(llm)` (require err == nil), then defines two sub-tests:
  - `t.Run("EmbedDocuments", ...)` — call with `[]string{"hello", "world"}`, assert `len == 2`, both vectors non-empty, equal length.
  - `t.Run("EmbedQuery", ...)` — call with `"hi"`, assert vector non-empty, length matches the `EmbedDocuments` length.
- [ ] Document the test's contract at the top of the file (env vars, recommended embed GGUFs, build-tag requirement) — keep it short, link back to `kuzco_test.go` for shared infra.

**Acceptance Criteria:**

- `EMBED_MODEL_URL=... go test -tags=integration ./... -run TestEmbeddings -v` passes.
- `go test -tags=integration ./... -run TestEmbeddings -v` (env unset) reports `SKIP`.
- `go test ./... -run TestEmbeddings` (no tag) finds no matching test (file excluded from build) and exits 0.
- Vectors have consistent dimensionality across the two sub-tests.

**Files / Areas:**

- `embeddings_integration_test.go` (new) - integration test + package doc comment.

### Task 4: Negative-path sub-test for chat-only model

- [ ] Inside the existing `TestLLM` in `kuzco_test.go` (which already loads a chat GGUF), add a sub-test `t.Run("CreateEmbedding_unsupported", ...)` that:
  - Constructs `llm := kuzco.New(k)` (or reuses the one passed to `llmtest.TestLLM`).
  - Calls `_, err := llm.CreateEmbedding(ctx, []string{"x"})`.
  - Asserts `err != nil`, that `err.Error()` starts with `"kuzco: embeddings:"`, and that `strings.Contains(err.Error(), "doesn't support embedding")`.
- [ ] If the test would interfere with `llmtest.TestLLM` (e.g. ordering or shared state), put it in its own top-level `TestCreateEmbeddingUnsupported` function in the same file — also tagged `integration` and gated on `MODEL_URL`.

**Acceptance Criteria:**

- Sub-test passes when the chat model is loaded; cleanly skipped when `MODEL_URL` is unset.
- Failure mode is informative: error message printed on failure (`t.Logf("err = %v", err)`).

**Files / Areas:**

- `kuzco_test.go` - add sub-test under `TestLLM` (or sibling top-level function).

### Task 5: Full suite gate

- [ ] Run `go vet ./...` clean.
- [ ] Run `go vet -tags=integration ./...` clean.
- [ ] Run `go test ./...` (no tag, both env vars unset) — units pass, no integration tests compiled.
- [ ] Run `go test -tags=integration ./...` with both env vars unset — integration tests SKIP cleanly; units still pass.
- [ ] Run `go test -tags=integration ./...` with `MODEL_URL` set only — chat test + negative-path pass; embedding test SKIPs.
- [ ] Run `go test -tags=integration ./...` with `EMBED_MODEL_URL` set only — embedding test passes; chat test + negative-path SKIP.
- [ ] Run `go test -tags=integration ./...` with both env vars set — everything passes.
- [ ] Spot-check that the test does not flake when invoked twice in a row (model caching is idempotent).

**Acceptance Criteria:**

- All five `go test` invocations above are green / SKIP as expected.
- No new lint or vet warnings under either build.

**Files / Areas:**

- N/A (CI / local invocation).

---

## Verification

### Automated

```bash
# Default build — fast, no model required:
go vet ./...
go test ./... -v

# Integration build — both end-to-end tests compiled in:
go vet -tags=integration ./...
MODEL_URL=https://.../qwen2.5-...-Q8_0.gguf \
EMBED_MODEL_URL=https://.../bge-small-en-v1.5-q8_0.gguf \
  go test -tags=integration ./... -v

# Single-test runs:
MODEL_URL=...        go test -tags=integration ./... -run TestLLM -v
EMBED_MODEL_URL=...  go test -tags=integration ./... -run TestEmbeddings -v
```

### Manual

1. Confirm `go test ./... -v` (no tag) lists only `TestCompile` plus the Phase-1 unit tests; no kronk downloads occur.
2. With both env vars set, run `go test -tags=integration ./... -v` and confirm `TestLLM`, `TestLLM/CreateEmbedding_unsupported` (or sibling), `TestEmbeddings/EmbedDocuments`, and `TestEmbeddings/EmbedQuery` all pass.
3. With the tag set but env vars unset, confirm every integration test reports `SKIP`.
4. Plug the embedder into a langchaingo vector store (e.g. `chroma` or `pgvector`) outside the test suite for one-time confidence; not required for CI.

---

## Risks

| Risk | Likelihood | Mitigation |
| ---- | ---------- | ---------- |
| Forgetting the blank line between `//go:build integration` and the `package` declaration causes the directive to be parsed as a comment, not a build tag. | Med | Verify by toggling: introduce a deliberate syntax error in the file, then confirm `go test ./...` (no tag) still compiles — proves the file is excluded. |
| Calling `kronk.Init` twice across `TestLLM` and `TestEmbeddings` in the same `go test` invocation. | Low | Kronk's `Init` is idempotent given the same lib path; pass the same `WithLibPath` value (resolved from the shared cache). |
| Embed-capable GGUF URL is large / slow on cold cache. | Med | Pick the smallest practical model (BGE small ≈ 130 MB Q8_0); reuse `KUZCO_TEST_CACHE_DIR` between runs. |
| llama.cpp bundle mismatch between chat and embed runs. | Low | `libs.New()` resolves bundles per host; both tests use the same resolver and share the cache. |
| Negative-path test depends on kronk's exact error string. | Med | Match on a stable substring (`"doesn't support embedding"`) rather than the full message; revisit on kronk bumps. |
| CI without network/GPU. | High | Default build (no tag) is the CI gate — integration tests are not compiled in unless someone opts in. |

## Open Questions

- Which specific embed GGUF should the doc comment recommend? Suggest BGE small (broad availability, small size); confirm with maintainer before pinning a URL in the doc comment.
- Should the negative-path test live as a sub-test under `TestLLM` or as a sibling top-level function? Default: sub-test (shares the loaded model, cheaper). Flip to sibling if it causes ordering issues with `llmtest.TestLLM`.

## Definition of Done

- [ ] All implementation tasks completed
- [ ] Acceptance criteria verified
- [ ] Automated checks passing (default + `-tags=integration`)
- [ ] Manual verification completed
- [ ] No unresolved blockers remain

---

## Handoff Notes

Once Phase 3 is green, fill in the PRD's `Summary` section with the outcome (mirroring the prior PRD's post-implementation summary), then move `docs/PRDs/active/2026-05-26-1121-kuzco-embeddings-adapter.md` (and its `phases/` siblings) into `docs/PRDs/done/`. A follow-up PRD can pick up the **Future Work** items (embed options, rerank, batching policy) when there is a concrete use case.

Note for future maintainers: the `integration` build tag is now the package-wide convention for any end-to-end test that touches kronk + real GGUFs. New integration tests should add `//go:build integration` plus their own env-var gate (mirroring `MODEL_URL` / `EMBED_MODEL_URL`) rather than inventing per-test build tags.
