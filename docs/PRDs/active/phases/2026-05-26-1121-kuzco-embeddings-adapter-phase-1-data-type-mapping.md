# Kuzco: langchaingo embeddings.EmbedderClient Adapter - Phase 1: Data / Type Mapping

**Source PRD**: docs/PRDs/active/2026-05-26-1121-kuzco-embeddings-adapter.md
**PRD ID**: PRD-2026-05-26-1121
**Phase**: 1 of 3
**Status**: Ready
**Created**: May 26, 2026
**Author**: thetnaingtn

---

## Objective

Establish the response-translation primitive and input-validation contract that the adapter surface (Phase 2) will sit on top of. Specifically: a deterministic mapper from `model.EmbedReponse` to `[][]float32` ordered by `EmbedData.Index`, plus a typed error for empty input. Doing this first gives Phase 2 a fully unit-tested foundation, so the only thing Phase 2 needs to add is the kronk call and deadline plumbing.

## Scope

### In Scope

- An empty-input typed error usable by `CreateEmbedding` (e.g. unexported `errEmptyInput` sentinel; export only if downstream `errors.Is` callers need it — leave unexported by default to stay minimal).
- Internal helper `embedResponseToVectors(resp model.EmbedReponse) [][]float32` that sorts `resp.Data` by `EmbedData.Index` then returns `[][]float32`.
- Table-driven unit tests covering the helper and the empty-input error.

### Out of Scope

- The `CreateEmbedding` method itself (Phase 2).
- The compile-time `embeddings.EmbedderClient` assertion (Phase 2).
- Any kronk invocation or deadline logic (Phase 2).
- Real-model integration testing (Phase 3).

---

## Inputs

| Input | Source | Notes |
| ----- | ------ | ----- |
| `model.EmbedReponse{Object, Created, Model, Data []EmbedData, Usage}` | `github.com/ardanlabs/kronk/sdk/kronk/model/models.go:984` | `EmbedData` carries `Object`, `Index`, `Embedding []float32`. |
| Expected return shape `[][]float32` | langchaingo `embeddings.EmbedderClient.CreateEmbedding` signature | One inner slice per input text, in the input's original order. |
| Existing kuzco style | `kuzco.go`, `messages.go` (mapper helpers, error wrapping prefix `kuzco:`) | Keep helper unexported, table-driven tests, no logging. |

## Dependencies

| Dependency | Type | Required Before | Notes |
| ---------- | ---- | --------------- | ----- |
| `github.com/ardanlabs/kronk v1.26.1` | Code | All tasks | Already pinned in `go.mod`. |
| Decision: exported vs unexported error sentinel | Decision | Task 1 | Default: unexported. Re-evaluate if callers need `errors.Is`. |

---

## Implementation Tasks

### Task 1: Define empty-input error sentinel

- [ ] In a new `embeddings.go` file, declare `var errEmptyInput = errors.New("kuzco: embeddings: texts must not be empty")`.
- [ ] Keep unexported unless Phase 2 review identifies a need to expose it.

**Acceptance Criteria:**

- `errEmptyInput` is reachable from the same package and matches `kuzco:`-prefixed error convention used elsewhere in the package.

**Files / Areas:**

- `embeddings.go` (new) - declare the sentinel.

### Task 2: Implement `embedResponseToVectors` helper

- [ ] Add unexported `embedResponseToVectors(resp model.EmbedReponse) [][]float32` in `embeddings.go`.
- [ ] Copy `resp.Data` into a local slice (avoid mutating kronk's return value).
- [ ] Sort the local copy by `EmbedData.Index` ascending using `sort.Slice` or `slices.SortFunc`.
- [ ] Build `out := make([][]float32, len(data))` and assign `out[i] = data[i].Embedding`.
- [ ] Return `out` (nil-safe: empty `Data` → empty slice, not nil panic).

**Acceptance Criteria:**

- Helper is pure (no side effects, no logging).
- In-order input passes through unchanged.
- Out-of-order input is reordered by `Index`.
- Each returned row is the same underlying `[]float32` slice from `EmbedData.Embedding` (no copy required — kronk hands ownership to the caller).

**Files / Areas:**

- `embeddings.go` (new) - add helper below the error sentinel.

### Task 3: Unit tests for mapper and error

- [ ] Create `embeddings_test.go` with package `kuzco`.
- [ ] Table-driven `TestEmbedResponseToVectors` covering: empty `Data`, single row, two rows in order, two rows reversed by `Index`, three rows with mixed `Index` values, and a case verifying per-row dimensionality is preserved (length of inner slice matches input).
- [ ] Add a separate `TestErrEmptyInput` (or include as a row) verifying `errEmptyInput` has the expected message — this becomes the contract for Phase 2's `CreateEmbedding` to return.

**Acceptance Criteria:**

- `go test ./... -run 'TestEmbedResponseToVectors|TestErrEmptyInput' -v` passes.
- Tests use the same table-driven style as `messages_test.go`.
- Tests do not import kronk's runtime model loader — they only construct `model.EmbedReponse` literals.

**Files / Areas:**

- `embeddings_test.go` (new) - all unit tests for this phase.

---

## Verification

### Automated

```bash
go vet ./...
go test ./... -run 'TestEmbedResponseToVectors|TestErrEmptyInput' -v
```

### Manual

1. Open `embeddings.go` and confirm the helper is unexported and the error sentinel is `kuzco:`-prefixed.
2. Open `embeddings_test.go` and confirm reordering test fails if the sort is removed (sanity check on coverage).

---

## Risks

| Risk | Likelihood | Mitigation |
| ---- | ---------- | ---------- |
| Reading `EmbedData.Embedding` while kronk still holds a reference. | Low | Kronk's response struct returns owned slices; do not mutate, just borrow. |
| Choosing exported vs unexported error wrong for downstream `errors.Is`. | Low | Start unexported; flip to exported during Phase 2 if needed (cheap rename). |

## Open Questions

- None — this phase is pure mapping work; all kronk semantics are known from `sdk/kronk/model/embed.go` and `sdk/kronk/model/models.go`.

## Definition of Done

- [ ] All implementation tasks completed
- [ ] Acceptance criteria verified
- [ ] Automated checks passing
- [ ] Manual verification completed
- [ ] No unresolved blockers remain

---

## Handoff Notes

Phase 2 only needs to: (a) write the public `CreateEmbedding` method that validates empty input via `errEmptyInput`, applies `ensureDeadline`, calls `l.k.Embeddings(ctx, model.D{"input": texts})`, and pipes the response through `embedResponseToVectors`; (b) add the compile-time interface assertion. Everything below the kronk boundary is already covered by Phase 1's unit tests.
