# Kuzco: langchaingo Adapter for ardanlabs/kronk - Phase 2: Adapter Surface

**Source PRD**: docs/PRDs/active/2026-05-24-2003-kuzco-langchaingo-adapter.md
**PRD ID**: PRD-2026-05-24-2003
**Phase**: 2 of 3
**Status**: Ready
**Created**: May 24, 2026
**Author**: thetnaingtn

---

## Objective

Wire the pure mappers from Phase 1 into the three live methods the `llms.Model` interface (and `llmtest`'s capability probes) expects: `Call`, `GenerateContent`, and `GenerateContentStream`. After this phase, a caller with a working `*kronk.Kronk` can drive it through langchaingo. Phase 3 then validates that behaviour with the full `llmtest` suite.

## Scope

### In Scope

- Implement `GenerateContent(ctx, messages, opts...) (*llms.ContentResponse, error)`.
- Implement `Call(ctx, prompt, opts...) (string, error)` delegating to `GenerateContent`.
- Implement `GenerateContentStream(ctx, messages, opts...) (<-chan llms.ContentResponse, error)` over `kronk.ChatStreaming`.
- `ensureDeadline(ctx)` helper using `LLM.defaultTimeout`.
- `StreamingFunc` invocation per chunk when set in `CallOptions`.
- `doc.go` with a runnable usage example.

### Out of Scope

- `llmtest.TestLLM` integration test wiring (Phase 3).
- Embedding / rerank / tokenize methods (PRD §Out of Scope).
- Multimodal parts (rejected by Phase 1's `messagesToKronk`).

---

## Inputs

| Input | Source | Notes |
| ----- | ------ | ----- |
| Phase 1 mappers | `messages.go` | `messagesToKronk`, `toolsToKronk`, `applyCallOptions`, `chatResponseToContent`. |
| `kronk.Chat(ctx, model.D)` signature | `kronk/sdk/kronk/chat.go` | Returns `(model.ChatResponse, error)`. Requires ctx with deadline. |
| `kronk.ChatStreaming(ctx, model.D)` signature | `kronk/sdk/kronk/chat.go` | Returns `(<-chan model.ChatResponse, error)`. Also requires deadline. |
| `llms.CallOption` builder | `langchaingo/llms/options.go` | `llms.CallOptions{}` + `for _, o := range opts { o(&co) }`. |
| Streaming capability shape | `langchaingo/testing/llmtest/llmtest.go` `supportsStreaming` | Probes for method with exact signature `GenerateContentStream(context.Context, []llms.MessageContent, ...llms.CallOption) (<-chan llms.ContentResponse, error)`. |

## Dependencies

| Dependency | Type | Required Before | Notes |
| ---------- | ---- | --------------- | ----- |
| Phase 1 complete | Code | Task 1 | All four mappers must exist and compile. |
| Decision on streaming chunk semantics (`Delta` vs `Message`) | Decision | Task 3 | Affects whether per-chunk content is incremental or cumulative. Inspect kronk's stream first. |

---

## Implementation Tasks

### Task 1: `ensureDeadline` helper

- [ ] Add unexported `ensureDeadline(ctx context.Context) (context.Context, context.CancelFunc)` in `kuzco.go`.
- [ ] If `_, ok := ctx.Deadline(); ok` → return `(ctx, func(){})`.
- [ ] Otherwise return `context.WithTimeout(ctx, llm.defaultTimeout)`.

**Acceptance Criteria:**

- Caller-supplied deadline is preserved.
- Missing deadline is filled and the returned cancel is safe to defer.

**Files / Areas:**

- `kuzco.go` — method on `*LLM`.

### Task 2: `GenerateContent`

- [ ] Build `co := llms.CallOptions{}` and apply each `opt`.
- [ ] Construct `d := model.D{}`; set `d["messages"]` from `messagesToKronk(messages)` (return error on failure); call `applyCallOptions(d, co)`.
- [ ] Wrap ctx via `ensureDeadline`; defer cancel.
- [ ] Call `llm.k.Chat(ctx, d)`; on error wrap with `fmt.Errorf("kuzco: chat: %w", err)`.
- [ ] Return `chatResponseToContent(resp), nil`.

**Acceptance Criteria:**

- Returns non-nil `*llms.ContentResponse` with at least one choice for a non-streaming success.
- Token usage surfaces in `Choices[0].GenerationInfo`.
- Errors from the mapper or kronk are wrapped, not swallowed.

**Files / Areas:**

- `kuzco.go` — replace stub.

### Task 3: `GenerateContentStream`

- [ ] New file `stream.go`.
- [ ] Same option/payload build as `GenerateContent`; set `d["stream"] = true`.
- [ ] Wrap ctx with `ensureDeadline`.
- [ ] Call `llm.k.ChatStreaming(ctx, d)`; on error return immediately.
- [ ] Spawn a goroutine that:
  - reads from kronk channel,
  - converts each `model.ChatResponse` chunk via `chatResponseToContent`,
  - if `co.StreamingFunc != nil`, invokes it with the chunk's text bytes (only the delta text — see open question),
  - sends `llms.ContentResponse` (value, not pointer — channel element type matches `llmtest` probe) on out channel,
  - closes out channel when kronk channel closes or ctx is done; cancels ctx on exit.
- [ ] Return `(<-chan llms.ContentResponse, nil)`.

**Acceptance Criteria:**

- Method signature exactly matches the `llmtest.supportsStreaming` probe.
- Output channel closes after the kronk stream ends.
- `StreamingFunc` fires at least once for a multi-chunk response.

**Files / Areas:**

- `stream.go` — new file.

### Task 4: `Call`

- [ ] Implement `Call(ctx, prompt, opts...)` by building a single human `MessageContent` and delegating to `llm.GenerateContent`.
- [ ] Return `resp.Choices[0].Content` or `errors.New("kuzco: empty response")` when no choices.

**Acceptance Criteria:**

- Matches `llms.GenerateFromSinglePrompt` expectations (used by `llmtest.testCall`).

**Files / Areas:**

- `kuzco.go`.

### Task 5: `doc.go`

- [ ] Add `doc.go` with package comment and an `Example` showing `kronk.New(...)` → `kuzco.New(k)` → `llms.GenerateFromSinglePrompt(...)`.

**Acceptance Criteria:**

- `go doc github.com/thetnaingtn/kuzco` renders the example.
- `go vet ./...` clean.

**Files / Areas:**

- `doc.go` — new file.

---

## Verification

### Automated

```bash
go vet ./...
go build ./...
go test ./... -run TestCompile -count=1
```

(Behavioural tests come in Phase 3; here we only confirm interface conformance and compilation.)

### Manual

1. In a scratch script, construct a `*kronk.Kronk` with a tiny model and call `kuzco.New(k).Call(ctx, "Say OK")` — confirm non-empty string returned.
2. Call `GenerateContent` with `llms.WithStreamingFunc(func(_ context.Context, b []byte) error { fmt.Print(string(b)); return nil })` and confirm chunks print incrementally before the final return.
3. Confirm that calling with a `context.Background()` does not error from kronk's "no deadline" guard.

---

## Risks

| Risk | Likelihood | Mitigation |
| ---- | ---------- | ---------- |
| Kronk streams emit cumulative `Message` rather than incremental `Delta` content — `StreamingFunc` would receive duplicates | Medium | Inspect `kronk/sdk/kronk/chat.go` streaming path; prefer `Choices[0].Delta` over `Message` when present; document the chosen semantics. |
| Channel element-type mismatch (`*ContentResponse` vs `ContentResponse`) makes the streaming probe skip the test | High if wrong | Match `llmtest.supportsStreaming` signature exactly: element type is `llms.ContentResponse` (value). |
| Cancelling ctx after streaming returns races the goroutine | Medium | Capture cancel in the goroutine's defer; do not cancel in the caller's path until the channel closes. |
| `StreamingFunc` errors are ignored | Low | If it returns an error, stop forwarding and close the channel; surface the error via a final chunk's `GenerationInfo["StreamError"]` or just log — decide and document. |

## Open Questions

- Does kronk's streaming `ChatResponse.Delta.Content` already contain only the new tokens, or is it cumulative? Determines whether `StreamingFunc` gets just the delta bytes.
- Should `Call` set a sensible `WithMaxTokens` default when none is provided, to avoid runaway generation on local models? (langchaingo's `GenerateFromSinglePrompt` passes options through unchanged.)

## Definition of Done

- [ ] All four methods implemented (`Call`, `GenerateContent`, `GenerateContentStream`, plus internal `ensureDeadline`)
- [ ] Compile-time `var _ llms.Model = (*LLM)(nil)` still holds
- [ ] Streaming method has the exact signature `llmtest.supportsStreaming` probes for
- [ ] `doc.go` example compiles
- [ ] `go vet ./...` clean

---

## Handoff Notes

Phase 3 will replace the placeholder `TestMe` test with `llmtest.TestLLM` integration plus unit tests for the Phase 1 mappers. If the streaming-delta decision in Task 3 had to deviate from incremental-only, note it in the integration test setup so the test reader understands why chunk content may look cumulative when printed.
