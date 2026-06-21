# Kuzco: langchaingo embeddings.EmbedderClient Adapter - Phase 2: Adapter Surface

**Source PRD**: docs/PRDs/active/2026-05-26-1121-kuzco-embeddings-adapter.md
**PRD ID**: PRD-2026-05-26-1121
**Phase**: 2 of 3
**Status**: Completed
**Created**: May 26, 2026
**Author**: thetnaingtn

---

## Objective

Expose the public surface that makes `*kuzco.LLM` usable as `embeddings.EmbedderClient`. This phase wires the validated Phase-1 mapper to a real kronk call, applies kronk's mandatory context deadline, adds the compile-time interface assertion, and ships a runnable doc example showing the embedder construction pattern.

## Scope

### In Scope

- `func (l *LLM) CreateEmbedding(ctx context.Context, texts []string) ([][]float32, error)` in `embeddings.go`.
- Reuse of the existing `ensureDeadline` helper from `kuzco.go:134`.
- Compile-time assertion `var _ embeddings.EmbedderClient = (*LLM)(nil)` co-located with the existing `var _ llms.Model = (*LLM)(nil)`.
- `ExampleNew_embedder` in `doc.go` demonstrating `kronk.New(...)` → `kuzco.New(k)` → `embeddings.NewEmbedder(llm)` → `EmbedQuery`.

### Out of Scope

- Real-model integration testing (Phase 3).
- Embedding options (`dimensions`, `truncate`) — deferred per PRD Future Work.
- Any new fields on `LLM` (no struct changes).

---

## Inputs

| Input | Source | Notes |
| ----- | ------ | ----- |
| `embedResponseToVectors`, `errEmptyInput` | Phase 1 (`embeddings.go`) | Already unit-tested. |
| `(*Kronk).Embeddings(ctx, model.D) (model.EmbedReponse, error)` | `github.com/ardanlabs/kronk/sdk/kronk/embedding.go:24` | Accepts `d["input"]` as `string`, `[]string`, or `[]any`; this adapter passes `[]string`. |
| `ensureDeadline(ctx) (context.Context, context.CancelFunc)` | `kuzco.go:134` | Already in use by `GenerateContent`; default 60s. |
| `embeddings.EmbedderClient` | `github.com/tmc/langchaingo/embeddings/embedding.go:34` | `CreateEmbedding(ctx, []string) ([][]float32, error)`. |

## Dependencies

| Dependency | Type | Required Before | Notes |
| ---------- | ---- | --------------- | ----- |
| Phase 1 helpers | Code | Task 1 | `embedResponseToVectors` and `errEmptyInput` must already exist. |
| `github.com/tmc/langchaingo v0.1.14` | Code | Task 2 | Already pinned in `go.mod`. |

---

## Implementation Tasks

### Task 1: Implement `CreateEmbedding`

- [ ] Append to `embeddings.go`:
  ```go
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
  ```
- [ ] Import `context`, `fmt`, `github.com/ardanlabs/kronk/sdk/kronk/model`, plus the langchaingo embeddings package for the assertion in Task 2.

**Acceptance Criteria:**

- Empty input returns `errEmptyInput` without calling kronk.
- Kronk errors are wrapped with the `kuzco: embeddings:` prefix and unwrap via `errors.Unwrap`/`errors.Is`.
- Context handed to kronk always has a deadline (either caller-provided or the 60s default).
- Result length equals `len(texts)`.

**Files / Areas:**

- `embeddings.go` - add method below Phase 1's helper.

### Task 2: Add compile-time interface assertion

- [ ] In `kuzco.go`, immediately after `var _ llms.Model = (*LLM)(nil)` (currently `kuzco.go:141`), add:
  ```go
  var _ embeddings.EmbedderClient = (*LLM)(nil)
  ```
- [ ] Add the `github.com/tmc/langchaingo/embeddings` import to `kuzco.go`.

**Acceptance Criteria:**

- `go build ./...` succeeds.
- Removing the new `CreateEmbedding` method causes a compile error citing the missing interface method (sanity check; revert).

**Files / Areas:**

- `kuzco.go` - add import + assertion.

### Task 3: Doc example

- [ ] In `doc.go`, add `ExampleNew_embedder` mirroring the existing `ExampleNew` style:
  - Construct `*kronk.Kronk` (can be `// ...` placeholder code per `testable example` rules if a full setup is unwieldy — match what `ExampleNew` already does).
  - Wrap with `kuzco.New(k)`.
  - Pass to `embeddings.NewEmbedder(llm)`.
  - Call `embedder.EmbedQuery(ctx, "hello")`.
  - Add a one-line comment explaining the GGUF must be embed-capable (`modelInfo.IsEmbedModel`).
- [ ] If `ExampleNew` uses `// Output:` markers, omit them here (network/model-dependent); otherwise match its style.

**Acceptance Criteria:**

- `go vet ./...` clean.
- `go doc github.com/thetnaingtun/kuzco` shows the new example.
- Example compiles via `go test -run Example ./...` (no execution required if no `// Output:` block).

**Files / Areas:**

- `doc.go` - add example.

---

## Verification

### Automated

```bash
go vet ./...
go build ./...
go test ./... -run 'TestEmbedResponseToVectors|TestErrEmptyInput|Example' -v
```

### Manual

1. Open `kuzco.go` and confirm both assertions sit together near the end of the file.
2. Open `embeddings.go` and confirm the method body is ~10 lines — no incidental complexity, no logging, no option plumbing.
3. `go doc github.com/thetnaingtun/kuzco.LLM.CreateEmbedding` returns the signature.

---

## Risks

| Risk | Likelihood | Mitigation |
| ---- | ---------- | ---------- |
| Forgetting to call the `cancel` returned by `ensureDeadline`. | Low | `defer cancel()` is a one-liner; same pattern is already in `GenerateContent`. |
| Passing `[]string` where kronk silently expects `[]any`. | Low | `embed.go:33` explicitly handles `[]string`; verified during exploration. |
| Doc example drifts from `ExampleNew` style. | Low | Copy `ExampleNew` and adjust the wrapping line. |

## Open Questions

- Should the example show `embedder.EmbedDocuments(ctx, [...])` or just `EmbedQuery`? Default: `EmbedQuery` (smaller surface, clearer intent). Implementer may include both if it stays under ~15 lines.

## Definition of Done

- [ ] All implementation tasks completed
- [ ] Acceptance criteria verified
- [ ] Automated checks passing
- [ ] Manual verification completed
- [ ] No unresolved blockers remain

---

## Handoff Notes

After this phase, the package compiles and the interface is satisfied. Phase 3 will exercise the path end-to-end against a real embed-capable GGUF (BGE / nomic-embed) gated on `EMBED_MODEL_URL`, plus do the chat-only-model error-path sanity check that proves the `kuzco: embeddings:` wrap propagates kronk's `"model doesn't support embedding"` error verbatim.
