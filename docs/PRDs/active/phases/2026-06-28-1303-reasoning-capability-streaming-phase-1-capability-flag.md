# Reasoning Capability & Streaming Passthrough - Phase 1: Capability Flag

**Source PRD**: ../2026-06-28-1303-reasoning-capability-streaming.md
**PRD ID**: PRD-2026-06-28-1303
**Phase**: 1 of 3
**Status**: Completed
**Created**: June 28, 2026
**Author**: thetnaingtn

---

## Objective

Make `*kuzco.LLM` satisfy langchaingo's `llms.ReasoningModel` interface so callers can feature-detect
reasoning support via `SupportsReasoning()` / `llms.SupportsReasoningModel(llm)`. Per the PRD,
`SupportsReasoning()` returns a constant `true`: kronk runs reasoning on by default and exposes no
per-model capability flag (no `IsReasoningModel` analogous to `IsEmbedModel`), so a constant is the
honest answer for kuzco.

## Scope

### In Scope

- New method `func (l *LLM) SupportsReasoning() bool` returning `true`, with a doc comment explaining
  the kronk rationale.
- Compile-time interface assertion `_ llms.ReasoningModel = (*LLM)(nil)`.
- Unit test asserting interface satisfaction and the `true` return.

### Out of Scope

- Streaming reasoning passthrough (Phase 3).
- `ReasoningTokens` usage surfacing (Phase 2).
- Any per-model heuristic — deliberately not implemented.

---

## Inputs

| Input | Source | Notes |
| ----- | ------ | ----- |
| `ReasoningModel` interface | `~/go/pkg/mod/github.com/tmc/langchaingo@v0.1.14/llms/llms.go` (~line 34) | Single method `SupportsReasoning() bool`. |
| Existing assertion block | `kuzco.go:15-18` | Already asserts `llms.Model` and `embeddings.EmbedderClient`. |
| `SupportsReasoningModel` helper | `langchaingo/llms/reasoning.go:145` | Type-asserts to `ReasoningModel` and calls `SupportsReasoning()`. |

## Dependencies

| Dependency | Type | Required Before | Notes |
| ---------- | ---- | --------------- | ----- |
| none | — | — | Self-contained; no other phase required first. |

---

## Implementation Tasks

### Task 1: Failing test for `ReasoningModel` satisfaction

- [ ] In `kuzco_compile_test.go` (package `kuzco`), add a compile-time assertion
      `var _ llms.ReasoningModel = (*LLM)(nil)` and a unit test
      `TestSupportsReasoning` asserting `New(nil).SupportsReasoning() == true`.
- [ ] Confirm the test fails to compile/run before Task 2 (method does not yet exist).

**Acceptance Criteria:**

- Test references `llms.ReasoningModel` and `SupportsReasoning()`; fails before Task 2.

**Files / Areas:**

- `kuzco_compile_test.go`

### Task 2: Implement `SupportsReasoning` + assertion

- [ ] Add `_ llms.ReasoningModel = (*LLM)(nil)` to the `var (...)` block at `kuzco.go:15`.
- [ ] Add `func (l *LLM) SupportsReasoning() bool { return true }` with a doc comment noting kronk
      enables thinking by default and exposes no per-model reasoning flag, so kuzco reports `true`
      unconditionally.

**Acceptance Criteria:**

- `*LLM` satisfies `llms.ReasoningModel`; `SupportsReasoning()` returns `true`.
- `llms.SupportsReasoningModel(New(nil))` returns `true`.

**Files / Areas:**

- `kuzco.go`

---

## Verification

### Automated

```bash
make run-tests
go vet ./...
```

### Manual

1. In a scratch test, call `llms.SupportsReasoningModel(kuzco.New(k))` and confirm `true`.

---

## Risks

| Risk | Likelihood | Mitigation |
| ---- | ---------- | ---------- |
| Always-`true` misleads callers for genuine non-reasoning models | Low | Documented as intentional; kronk exposes no flag to do better. Captured in PRD Risks. |

## Open Questions

- None.

## Definition of Done

- [x] Test added and passes
- [x] `make run-tests` green
- [x] `go vet ./...` clean

---

## Handoff Notes

Phase 2 and Phase 3 build on the same `*LLM`. Keep the assertion in the existing `var (...)` block so
all interface guarantees live in one place.
