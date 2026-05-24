# Kuzco: langchaingo Adapter for ardanlabs/kronk - Phase 1: Data / Type Mapping

**Source PRD**: docs/PRDs/active/2026-05-24-2003-kuzco-langchaingo-adapter.md
**PRD ID**: PRD-2026-05-24-2003
**Phase**: 1 of 3
**Status**: Ready
**Created**: May 24, 2026
**Author**: thetnaingtn

---

## Objective

Establish the foundational type system and pure translation helpers that move data between langchaingo's `llms` types and kronk's `model.D` / `model.ChatResponse` shapes. No I/O, no network — just the deterministic, table-testable mappers the adapter surface in Phase 2 will compose. Phase 2 cannot start until these helpers and the `kuzco.LLM` skeleton exist and compile.

## Scope

### In Scope

- Add `github.com/ardanlabs/kronk v1.25.3` to `go.mod`.
- Define `kuzco.LLM` struct, `Option` functional-options pattern, and `New(k *kronk.Kronk, opts ...Option) *LLM` constructor (method bodies left as stubs for Phase 2).
- Implement `messagesToKronk([]llms.MessageContent) ([]map[string]any, error)`.
- Implement `toolsToKronk([]llms.Tool) []map[string]any`.
- Implement `applyCallOptions(d model.D, opts llms.CallOptions)`.
- Implement `chatResponseToContent(model.ChatResponse) *llms.ContentResponse`.

### Out of Scope

- `Call`, `GenerateContent`, `GenerateContentStream` bodies (Phase 2).
- Integration test against a real GGUF (Phase 3).
- Multimodal (image/audio) parts — return a typed error for now.

---

## Inputs

| Input | Source | Notes |
| ----- | ------ | ----- |
| `llms.Model` interface contract | `github.com/tmc/langchaingo@v0.1.14/llms/llms.go` | Determines required method signatures. |
| `llms.MessageContent`, `ContentPart` variants | `langchaingo/llms` | `TextContent`, `ToolCall`, `ToolCallResponse` must be handled; binary/image rejected. |
| `llms.CallOptions` fields | `langchaingo/llms/options.go` | Maps to `temperature`, `top_p`, `max_tokens`, `stop`, `seed`, `tools`, `tool_choice`, `stream`. |
| kronk `model.D` payload shape | `github.com/ardanlabs/kronk@v1.25.3/sdk/kronk/chat.go` and `sdk/kronk/model` | OpenAI-style chat-completion JSON keys. |
| kronk `model.ChatResponse` / `Usage` / `Choices` | `kronk/sdk/kronk/response.go`, `kronk/sdk/kronk/model` | Source for content, tool calls, finish reason, token usage. |

## Dependencies

| Dependency | Type | Required Before | Notes |
| ---------- | ---- | --------------- | ----- |
| `kronk` v1.25.3 module available locally | External | Task 1 | Already cached at `~/go/pkg/mod/github.com/ardanlabs/kronk@v1.25.3`. |
| Decision: default request timeout for `ensureDeadline` | Decision | Task 2 | Phase 2 will use it; Phase 1 just exposes the option. |

---

## Implementation Tasks

### Task 1: Add kronk dependency

- [ ] Run `go get github.com/ardanlabs/kronk@v1.25.3`.
- [ ] Run `go mod tidy`.
- [ ] Confirm `go build ./...` still succeeds with the placeholder `kuzco.go`.

**Acceptance Criteria:**

- `go.mod` lists `github.com/ardanlabs/kronk v1.25.3`.
- `go.sum` is updated.
- `go build ./...` exits 0.

**Files / Areas:**

- `go.mod`, `go.sum` — add dep.

### Task 2: Define `LLM`, `Option`, and `New`

- [ ] Replace placeholder `TestMe` content in `kuzco.go`; keep package `kuzco`.
- [ ] Define `type LLM struct { k *kronk.Kronk; defaultTimeout time.Duration }`.
- [ ] Define `type Option func(*LLM)` with at least `WithDefaultTimeout(d time.Duration) Option`.
- [ ] Define `func New(k *kronk.Kronk, opts ...Option) *LLM` applying options with a sane default (e.g. 60s).
- [ ] Add stub methods `Call` and `GenerateContent` that return `errors.New("not implemented")` so the type implements `llms.Model` at compile time.
- [ ] Add `var _ llms.Model = (*LLM)(nil)`.

**Acceptance Criteria:**

- `go vet ./...` clean.
- `go build ./...` succeeds with the interface assertion in place.

**Files / Areas:**

- `kuzco.go` — type, options, constructor, stubs, assertion.

### Task 3: `messagesToKronk`

- [ ] New file `messages.go`.
- [ ] Iterate `[]llms.MessageContent`; map role using `llms.ChatMessageType` → string (`"system" | "user" | "assistant" | "tool"`).
- [ ] For each `ContentPart`:
  - `llms.TextContent` → append text to message `content`.
  - `llms.ToolCall` (only on assistant messages) → emit OpenAI-style `tool_calls: [{id, type:"function", function:{name, arguments}}]`.
  - `llms.ToolCallResponse` → emit role `"tool"` message with `tool_call_id` + `content`.
  - `llms.BinaryContent` / `llms.ImageURLContent` → return `fmt.Errorf("kuzco: unsupported part type %T", part)`.
- [ ] Return `[]map[string]any` ready for embedding under `d["messages"]`.

**Acceptance Criteria:**

- Table tests cover: system+human, multi-turn human/assistant, assistant tool call, tool response, unsupported part rejection.
- Output JSON-marshals into the OpenAI-chat shape kronk expects (verified by `json.Marshal` round-trip in a test).

**Files / Areas:**

- `messages.go` — helper.
- `messages_test.go` — table tests (created in Phase 3, but stub a single passing test here to anchor the function).

### Task 4: `toolsToKronk`

- [ ] In `messages.go`, add `toolsToKronk([]llms.Tool) []map[string]any`.
- [ ] Each `llms.Tool` → `{"type": tool.Type, "function": {"name": ..., "description": ..., "parameters": ...}}`.
- [ ] Return `nil` for an empty/`nil` slice so callers can pass it straight to `d["tools"]` only when set.

**Acceptance Criteria:**

- `nil` in → `nil` out (no empty array in payload).
- One tool in → one map with `type` and `function` keys.

**Files / Areas:**

- `messages.go` — helper.

### Task 5: `applyCallOptions`

- [ ] Add `applyCallOptions(d model.D, opts llms.CallOptions)` in `messages.go`.
- [ ] Map only set fields:
  - `MaxTokens > 0` → `d["max_tokens"]`.
  - `Temperature != 0` (or use a pointer-style guard via langchaingo defaults) → `d["temperature"]`.
  - `TopP != 0` → `d["top_p"]`.
  - `len(StopWords) > 0` → `d["stop"]`.
  - `Seed != 0` → `d["seed"]`.
  - `len(Tools) > 0` → `d["tools"] = toolsToKronk(opts.Tools)`.
  - `ToolChoice != nil` → `d["tool_choice"]`.
  - `StreamingFunc != nil` → `d["stream"] = true`.
- [ ] Document each mapping with a one-line comment only where the kronk key differs from the option field name.

**Acceptance Criteria:**

- Default `llms.CallOptions{}` leaves `d` untouched apart from keys the caller set elsewhere.
- A fully-populated `CallOptions` produces every documented key.

**Files / Areas:**

- `messages.go` — helper.

### Task 6: `chatResponseToContent`

- [ ] In `messages.go`, add `chatResponseToContent(resp model.ChatResponse) *llms.ContentResponse`.
- [ ] For each `resp.Choices`:
  - Map `Message.Content` → `ContentChoice.Content`.
  - Map `Message.ToolCalls` → `[]llms.ToolCall` (preserve `ID`, `Type`, `FunctionCall.Name`, `FunctionCall.Arguments`).
  - Map finish reason string → `ContentChoice.StopReason`.
  - Populate `ContentChoice.GenerationInfo` with `PromptTokens`, `CompletionTokens`, `TotalTokens` (int) from `resp.Usage` when non-zero. Add `CachedTokens` if kronk reports it.

**Acceptance Criteria:**

- Empty `resp.Choices` produces a `ContentResponse` with an empty `Choices` slice (no panic).
- Usage fields surface in `GenerationInfo` with the exact keys `llmtest.testTokenCounting` looks for.

**Files / Areas:**

- `messages.go` — helper.

---

## Verification

### Automated

```bash
go mod tidy
go vet ./...
go build ./...
```

(Phase 1 mappers are exercised by tests added in Phase 3; here we only need the package to compile and vet clean.)

### Manual

1. Open `kuzco.go` and confirm `var _ llms.Model = (*LLM)(nil)` is present and the file compiles in editor.
2. Open `messages.go` and walk through each helper signature, confirming it matches the keys listed in PRD §Backend.

---

## Risks

| Risk | Likelihood | Mitigation |
| ---- | ---------- | ---------- |
| kronk `model.ChatResponse` field names differ from assumptions (e.g. `Delta` vs `Message`) | Medium | Open `kronk/sdk/kronk/response.go` and `model/` before writing the mapper; encode actual field names, not guesses. |
| `llms.CallOptions` uses zero-vs-set ambiguity for `Temperature` / `TopP` | Medium | Mirror what existing langchaingo providers (openai, ollama) do — they treat zero as "use default" and skip the key. Document the choice. |
| OpenAI-shape `tool_choice` accepts string OR object; mistyped value rejected by kronk | Low | Pass through whatever `opts.ToolChoice` is (it is `any` in langchaingo). |

## Open Questions

- What does kronk's exact `model.D` key for tool responses look like — does it accept the OpenAI `role:"tool", tool_call_id:...` shape verbatim, or does it require a kronk-specific wrapper? (Inspect `chat.go` and `model/` before finalising Task 3.)
- Does kronk's `Usage` struct expose cached/prompt-cache tokens, or only prompt/completion/total? (Affects optional `CachedTokens` key.)

## Definition of Done

- [ ] All implementation tasks completed
- [ ] `go vet ./...` and `go build ./...` clean
- [ ] `var _ llms.Model = (*LLM)(nil)` compiles
- [ ] kronk dependency pinned at v1.25.3 in `go.mod`
- [ ] No unresolved blockers carried into Phase 2

---

## Handoff Notes

Phase 2 will fill in `Call`, `GenerateContent`, and `GenerateContentStream` using these helpers. The `defaultTimeout` field on `LLM` is in place specifically so Phase 2's `ensureDeadline(ctx)` can read it without changing the constructor. If Task 3's investigation reveals kronk needs a non-OpenAI message shape, update the helper and call it out in the Phase 2 handoff so the request builder uses the correct keys.
