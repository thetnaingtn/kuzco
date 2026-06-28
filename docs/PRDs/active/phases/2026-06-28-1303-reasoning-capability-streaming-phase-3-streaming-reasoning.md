# Reasoning Capability & Streaming Passthrough - Phase 3: Streaming Reasoning

**Source PRD**: ../2026-06-28-1303-reasoning-capability-streaming.md
**PRD ID**: PRD-2026-06-28-1303
**Phase**: 3 of 3
**Status**: Ready
**Created**: June 28, 2026
**Author**: thetnaingtn

---

## Objective

Forward kronk's per-token reasoning deltas (`Delta.Reasoning`) to langchaingo's
`StreamingReasoningFunc` in **both** streaming entry points — the `GenerateContent` accumulation loop
(`kuzco.go`) and the `GenerateContentStream` emit loop (`stream.go`). Today reasoning is accumulated
into the final message but never streamed live, so streaming callers see no reasoning until the end.
The existing content-only `StreamingFunc` path stays untouched. Finish with a README note documenting
reasoning support.

langchaingo signature: `StreamingReasoningFunc func(ctx context.Context, reasoningChunk, chunk []byte) error`
(set via `llms.WithStreamingReasoningFunc`). kronk carries content and reasoning on separate delta
fields, so a pure-reasoning delta has empty content — mirror the Anthropic adapter, which passes an
empty content chunk for thinking deltas.

## Scope

### In Scope

- `GenerateContent` (`kuzco.go:146-156`): when `c.Delta.Reasoning != ""` and
  `co.StreamingReasoningFunc != nil`, call
  `co.StreamingReasoningFunc(ctx, []byte(c.Delta.Reasoning), []byte(c.Delta.Content))`; on error,
  `cancel()` and return a wrapped error mirroring the existing `StreamingFunc` handling.
- `GenerateContentStream` (`stream.go`): add `chunkReasoning` helper (paralleling `chunkDelta`) and
  forward reasoning deltas to `StreamingReasoningFunc` when non-empty.
- Unit tests for both paths feeding fake reasoning deltas; assert ordered delivery and that content
  `StreamingFunc` is unaffected.
- README reasoning-support note (interface + streaming reasoning).

### Out of Scope

- Capability flag (Phase 1) and usage key (Phase 2).
- Changing `StreamingFunc` content behavior.

---

## Inputs

| Input | Source | Notes |
| ----- | ------ | ----- |
| `StreamingReasoningFunc` field | `langchaingo/llms/options.go:26` | `func(ctx, reasoningChunk, chunk []byte) error`. |
| `WithStreamingReasoningFunc` | `langchaingo/llms/options.go:170` | Option setter callers use. |
| Accumulation loop | `kuzco.go:119-176` | Already does `msg.Reasoning += c.Delta.Reasoning` (line 148) and content `StreamingFunc` (line 150). |
| Emit loop + `chunkDelta` | `stream.go` | Content-only forwarding via `chunkDelta`. |
| Anthropic reference pattern | `langchaingo/llms/anthropic/internal/anthropicclient/messages.go:489` | `handleThinkingDelta` passes empty content chunk for pure reasoning. |

## Dependencies

| Dependency | Type | Required Before | Notes |
| ---------- | ---- | --------------- | ----- |
| Phase 1 | Logical | recommended | Capability flag should exist before advertising streaming reasoning in README. Not a hard code dependency. |

---

## Implementation Tasks

### Task 1: Failing streaming tests (both paths)

- [ ] Add a unit test driving `GenerateContent` with a fake kronk stream containing reasoning deltas
      (`Delta{Reasoning: "th", Content: ""}`, then content deltas) and a recording
      `WithStreamingReasoningFunc`; assert reasoning chunks arrive in order and the content
      `StreamingFunc` only receives content.
- [ ] Add the equivalent test for `GenerateContentStream`.
- [ ] Follow existing fake-stream style (table-driven, `model.ChatResponse` fixtures) used in the
      streaming tests; confirm both fail before Tasks 2–3.

**Acceptance Criteria:**

- Both tests fail before implementation; assert ordering and content/reasoning separation.

**Files / Areas:**

- `kuzco_llm_test.go` / a streaming-focused `*_test.go`

### Task 2: Forward reasoning in `GenerateContent`

- [ ] In the `if c.Delta != nil` block (`kuzco.go:146`), after the reasoning accumulation, add:
      ```go
      if co.StreamingReasoningFunc != nil && c.Delta.Reasoning != "" {
          if err := co.StreamingReasoningFunc(ctx, []byte(c.Delta.Reasoning), []byte(c.Delta.Content)); err != nil {
              cancel()
              return nil, fmt.Errorf("kuzco: chat: streaming-reasoning-func: %w", err)
          }
      }
      ```
- [ ] Leave the existing content `StreamingFunc` call unchanged.

**Acceptance Criteria:**

- Reasoning deltas reach `StreamingReasoningFunc` in order; content path unchanged; error cancels and
  wraps like `StreamingFunc`.

**Files / Areas:**

- `kuzco.go`

### Task 3: Forward reasoning in `GenerateContentStream`

- [ ] Add `func chunkReasoning(resp model.ChatResponse) string` paralleling `chunkDelta`, returning
      `resp.Choices[0].Delta.Reasoning` when present.
- [ ] In the emit loop, after the `StreamingFunc` content block, add a `StreamingReasoningFunc` block:
      when `co.StreamingReasoningFunc != nil` and `chunkReasoning(chunk) != ""`, call it with the
      reasoning chunk and the content chunk; on error, `return` (mirroring the existing `StreamingFunc`
      early return that ends the goroutine).

**Acceptance Criteria:**

- Reasoning deltas reach `StreamingReasoningFunc`; existing per-chunk `ContentResponse` emission and
  content `StreamingFunc` unchanged.

**Files / Areas:**

- `stream.go`

### Task 4: README documentation

- [ ] Update `README.md` to note kuzco satisfies `llms.ReasoningModel` (`SupportsReasoning()`),
      maps `WithThinkingMode` to kronk's `enable_thinking`/`reasoning_effort`, and streams reasoning
      deltas via `WithStreamingReasoningFunc`, plus `GenerationInfo["ReasoningTokens"]`.

**Acceptance Criteria:**

- README mentions the new interface, streaming reasoning, and the usage key.

**Files / Areas:**

- `README.md`

---

## Verification

### Automated

```bash
make run-tests       # unit tests, both streaming paths
make run-llm-test    # LLM integration; Reasoning subtest exercises live reasoning
go vet ./...
```

### Manual

1. Call `GenerateContent` with `WithThinkingMode(ThinkingModeMedium)` and both
   `WithStreamingReasoningFunc` and `WithStreamingFunc`; confirm reasoning chunks arrive live and
   content chunks still arrive separately.
2. Repeat with `GenerateContentStream`; confirm parity.

---

## Risks

| Risk | Likelihood | Mitigation |
| ---- | ---------- | ---------- |
| Two streaming paths drift (one fixed, not the other) | Medium | Tasks 2 and 3 plus Task 1 tests cover both `GenerateContent` and `GenerateContentStream`. |
| Error handling diverges from existing pattern | Low | Mirror `StreamingFunc`: `cancel()`+wrap in `GenerateContent`, early `return` in the `GenerateContentStream` goroutine. |
| Calling `StreamingReasoningFunc` with non-empty content double-delivers content | Low | Pure-reasoning deltas have empty content; content delivery stays solely on the `StreamingFunc` path. Tests assert separation. |

## Open Questions

- None.

## Definition of Done

- [ ] All tasks completed; both streaming paths forward reasoning
- [ ] `make run-tests` and `make run-llm-test` green
- [ ] `go vet ./...` clean
- [ ] README updated
- [ ] PRD Status set to Completed and spec moved to `docs/PRDs/done/`

---

## Handoff Notes

This is the final phase. After merge, update the PRD **Status** to **Completed**, fill the PRD
**Summary**, and move both the PRD and its phase files from `active/` to `done/` per CLAUDE.md.
