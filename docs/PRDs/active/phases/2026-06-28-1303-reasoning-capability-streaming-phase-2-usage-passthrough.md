# Reasoning Capability & Streaming Passthrough - Phase 2: Usage Passthrough

**Source PRD**: ../2026-06-28-1303-reasoning-capability-streaming.md
**PRD ID**: PRD-2026-06-28-1303
**Phase**: 2 of 3
**Status**: Completed
**Created**: June 28, 2026
**Author**: thetnaingtn

---

## Objective

Surface kronk's `Usage.ReasoningTokens` as `GenerationInfo["ReasoningTokens"]` in the translated
`llms.ContentChoice`, so token accounting and langchaingo's `llms.ExtractThinkingTokens` no longer
under-report reasoning usage. The addition follows the existing zero-skip pattern in
`chatResponseToContent`.

## Scope

### In Scope

- Add `gi["ReasoningTokens"] = u.ReasoningTokens` (guarded by `u.ReasoningTokens != 0`) in
  `chatResponseToContent` (`messages.go`).
- Unit test asserting the key is present when `> 0` and absent when `0`.

### Out of Scope

- Capability flag (Phase 1).
- Streaming reasoning passthrough (Phase 3).
- Any change to the existing prompt/completion/total usage keys.

---

## Inputs

| Input | Source | Notes |
| ----- | ------ | ----- |
| `Usage.ReasoningTokens` | `~/go/pkg/mod/github.com/ardanlabs/kronk@v1.28.2/sdk/kronk/model/models.go` (~line 820) | `int` field on `Usage`. |
| Existing usage mapping | `messages.go:190-204` | Zero-skip pattern for `PromptTokens`, `CompletionTokens`, `TotalTokens`. |
| Consumer key | `langchaingo/llms/reasoning.go:243` | `ExtractThinkingTokens` reads `generationInfo["ReasoningTokens"].(int)`. |

## Dependencies

| Dependency | Type | Required Before | Notes |
| ---------- | ---- | --------------- | ----- |
| none | — | — | Independent of Phases 1 and 3. |

---

## Implementation Tasks

### Task 1: Failing test for `ReasoningTokens` in `GenerationInfo`

- [x] In `messages_test.go` (package `kuzco`), add a test that builds a `model.ChatResponse` with
      `Usage{ReasoningTokens: 7, ...}`, calls `chatResponseToContent`, and asserts
      `Choices[0].GenerationInfo["ReasoningTokens"] == 7`.
- [x] Add a zero case: `Usage{ReasoningTokens: 0}` → key absent from `GenerationInfo`.
- [x] Optionally assert `llms.ExtractThinkingTokens(gi).ThinkingTokens == 7` to prove round-trip.
- [x] Confirm the non-zero case fails before Task 2.

**Acceptance Criteria:**

- Non-zero case fails before Task 2; zero case passes both before and after (key never added).

**Files / Areas:**

- `messages_test.go`

### Task 2: Add the `ReasoningTokens` key

- [x] In `chatResponseToContent` (`messages.go`), inside the `if u := resp.Usage; u != nil` block,
      add:
      ```go
      if u.ReasoningTokens != 0 {
          gi["ReasoningTokens"] = u.ReasoningTokens
      }
      ```
      placed alongside the existing usage keys, preserving the `len(gi) > 0` guard before assigning
      `cc.GenerationInfo`.

**Acceptance Criteria:**

- `GenerationInfo["ReasoningTokens"]` present and equal to kronk's value when `> 0`; absent when `0`.
- Existing usage keys unchanged.

**Files / Areas:**

- `messages.go`

---

## Verification

### Automated

```bash
make run-tests
go vet ./...
```

### Manual

1. Run a reasoning prompt; inspect `resp.Choices[0].GenerationInfo["ReasoningTokens"]` is populated.

---

## Risks

| Risk | Likelihood | Mitigation |
| ---- | ---------- | ---------- |
| Type mismatch (`ExtractThinkingTokens` expects `int`) | Low | kronk `ReasoningTokens` is `int`; assign directly without conversion. |
| Key added even when zero, masking the absent-vs-zero distinction | Low | Guarded by `!= 0`, matching sibling keys; zero-case test enforces. |

## Open Questions

- None.

## Definition of Done

- [x] Tests added and pass
- [x] `make run-tests` green
- [x] `go vet ./...` clean

---

## Handoff Notes

`chatResponseToContent` is shared by the batch path (`GenerateContent`) and the per-chunk path
(`GenerateContentStream`), so this key now appears on streamed chunks too once kronk emits final
usage. Phase 3 touches the same response path for streaming deltas — coordinate test fixtures.
