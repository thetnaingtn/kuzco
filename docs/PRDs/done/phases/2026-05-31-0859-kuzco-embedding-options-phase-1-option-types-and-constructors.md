# Kuzco Embedding Options - Phase 1: Option Types and Constructors

**Source PRD**: ../2026-05-31-0859-kuzco-embedding-options.md
**PRD ID**: PRD-2026-05-31-0859
**Phase**: 1 of 3
**Status**: Completed
**Created**: May 31, 2026
**Author**: thetnaingtn

---

## Objective

Land the public API surface for embedding options on `kuzco.New` — the typed `TruncateDirection`, its two constants, and the three functional options (`WithEmbeddingTruncate`, `WithEmbeddingTruncateDirection`, `WithEmbeddingDimension`) — plus the internal `embedOpts` field on `*LLM` they write into.

This phase ships compile-time additions only. `CreateEmbedding`'s runtime payload is unchanged in Phase 1, so no caller is affected and no kronk request shape moves. Phase 2 reads the new state; Phase 3 proves it end-to-end. Splitting this off keeps the API/state change reviewable on its own and lets the merge happen even if Phase 2 needs another iteration.

## Scope

### In Scope

- Public `TruncateDirection` string type with exported constants `TruncateRight` (`"right"`) and `TruncateLeft` (`"left"`).
- Unexported `embedOpts` struct on `*LLM` with `truncate *bool`, `truncateDirection TruncateDirection`, `dimension int`.
- Three new constructor options: `WithEmbeddingTruncate`, `WithEmbeddingTruncateDirection`, `WithEmbeddingDimension`. Invalid `TruncateDirection` values and non-positive `dimension` values are silent no-ops.
- Unit tests asserting each option mutates `embedOpts` correctly and that invalid values are ignored.

### Out of Scope

- Wiring `embedOpts` into the `model.D` payload sent to kronk (Phase 2).
- Integration tests against a live GGUF embed model (Phase 3).
- Documentation updates in `CLAUDE.md` and package doc comment (Phase 3).

---

## Inputs

| Input | Source | Notes |
| ----- | ------ | ----- |
| Functional-option pattern already in use | `kuzco.go` lines 25-42 (`Option`, `WithDefaultTimeout`, `New`) | New options must match this signature shape: `func(*LLM)`. |
| Embedding options kronk exposes | `~/go/pkg/mod/github.com/ardanlabs/kronk@v1.26.1/sdk/kronk/embedding.go` | Documents `truncate` (bool), `truncate_direction` (string), `dimension` (int). |
| PRD solution outline | Source PRD §Solution + §Phase 1 | Pointer-bool for `truncate` to distinguish "unset" from "set to false". |

## Dependencies

| Dependency | Type | Required Before | Notes |
| ---------- | ---- | --------------- | ----- |
| (none) | — | — | Phase 1 is the foundation; no prior phase blocks it. |

---

## Implementation Tasks

### Task 1: Add `TruncateDirection` type and constants

- [x] In `kuzco.go`, declare `type TruncateDirection string`.
- [x] Declare exported constants `TruncateRight TruncateDirection = "right"` and `TruncateLeft TruncateDirection = "left"`.
- [x] Place these near the top of the file, above `LLM`, alongside the existing interface assertions.

**Acceptance Criteria:**

- `kuzco.TruncateRight` and `kuzco.TruncateLeft` compile as `TruncateDirection` values.
- The string values match kronk's documented strings exactly (`"right"`, `"left"`).

**Files / Areas:**

- `kuzco.go` — new type + two constants.

### Task 2: Add `embedOpts` struct and field on `LLM`

- [x] Declare an unexported `embedOpts` struct with fields `truncate *bool`, `truncateDirection TruncateDirection`, `dimension int`.
- [x] Add an `embed embedOpts` field on `LLM` (or inline the fields — prefer the struct for grouping).
- [x] Confirm `New` leaves the new field at its zero value (no constructor change needed beyond the field declaration).

**Acceptance Criteria:**

- `(&LLM{}).embed` is the zero `embedOpts` — pointer nil, direction empty string, dimension `0`.
- No existing test breaks (`go test -v ./...` still passes after this task alone).

**Files / Areas:**

- `kuzco.go` — struct + field.

### Task 3: Implement the three constructor options + unit tests

- [x] Add `WithEmbeddingTruncate(v bool) Option` that sets `l.embed.truncate = &v`.
- [x] Add `WithEmbeddingTruncateDirection(d TruncateDirection) Option` that sets `l.embed.truncateDirection = d` **only when** `d == TruncateRight || d == TruncateLeft`; otherwise no-op.
- [x] Add `WithEmbeddingDimension(n int) Option` that sets `l.embed.dimension = n` **only when** `n > 0`; otherwise no-op.
- [x] In `embeddings_test.go` (or a new `options_test.go` if preferred — match existing test layout), add unit tests:
  - Each option, when applied via `New`, writes the expected value into `embed`.
  - `WithEmbeddingTruncate(false)` produces a non-nil pointer to `false` (distinguishable from unset).
  - `WithEmbeddingTruncateDirection("up")` leaves `embed.truncateDirection` empty.
  - `WithEmbeddingDimension(0)` and `WithEmbeddingDimension(-1)` leave `embed.dimension == 0`.
- [x] Tests live in `package kuzco` (not `_test`) so they can read the unexported `embed` field directly.

**Acceptance Criteria:**

- All new unit tests pass.
- `go vet ./...` reports no new diagnostics.
- The existing test suite (`go test -v ./...`) still passes — no behavioral change for callers that don't use the new options.

**Files / Areas:**

- `kuzco.go` — three `With...` functions.
- `embeddings_test.go` — new test cases (or a new `options_test.go`).

---

## Verification

### Automated

```bash
go test -v -count=1 ./...
go vet ./...
```

### Manual

1. In a scratch program, call `kuzco.New(k, kuzco.WithEmbeddingTruncate(true), kuzco.WithEmbeddingTruncateDirection(kuzco.TruncateLeft), kuzco.WithEmbeddingDimension(256))` and confirm it compiles and returns a non-nil `*LLM`.
2. Call `kuzco.New(k, kuzco.WithEmbeddingTruncateDirection("nonsense"))`; confirm compile-time success and that runtime behavior is the same as omitting the option (verified via the unit test, no manual harness needed beyond compilation).

---

## Risks

| Risk | Likelihood | Mitigation |
| ---- | ---------- | ---------- |
| Tests live in a `_test` package and cannot read `embed` | Low | Place new tests in `package kuzco`. Existing tests in this repo already mix both styles — match what's already in `embeddings_test.go`. |
| Pointer-bool design surprises a future reader | Low | Single short comment on the `truncate` field explaining "nil = unset, distinct from set-to-false". |
| Constant string drift vs. kronk | Low | String literals match kronk's documented values; add a one-line code comment pointing at `kronk/sdk/kronk/embedding.go`. |

## Open Questions

- Should the option setter for an invalid direction or non-positive dimension log a warning? PRD says silent no-op; defer logging until a real caller asks for it.

## Definition of Done

- [x] All implementation tasks completed
- [x] Acceptance criteria verified
- [x] `go test -v -count=1 ./...` passes
- [x] `go vet ./...` clean
- [x] No unresolved blockers remain

---

## Handoff Notes

Phase 2 will read `l.embed` from `CreateEmbedding` to build the `model.D` payload. Keep the field name (`embed`) and struct name (`embedOpts`) stable — Phase 2's test seam (`buildEmbedPayload`) will reference them directly. Do not change `CreateEmbedding`'s body in this phase; the goal is to keep the diff small and reviewable.
