# Kuzco: langchaingo Adapter for ardanlabs/kronk - Phase 3: Test Suite & Verification

**Source PRD**: docs/PRDs/active/2026-05-24-2003-kuzco-langchaingo-adapter.md
**PRD ID**: PRD-2026-05-24-2003
**Phase**: 3 of 3
**Status**: Ready
**Created**: May 24, 2026
**Author**: thetnaingtn

---

## Objective

Prove the adapter is correct by running the official `llmtest.TestLLM` suite against a real `*kronk.Kronk` and by exercising the Phase 1 mappers with deterministic unit tests. This phase is what closes out the PRD: until `llmtest.TestLLM` passes end-to-end, the adapter is not "compatible" in the sense the PRD requires.

## Scope

### In Scope

- Remove placeholder `TestMe` in `kuzco.go`.
- New `kuzco_test.go` running `llmtest.TestLLM`, env-gated on `KUZCO_TEST_MODEL_PATH`.
- New `messages_test.go` with table-driven tests for `messagesToKronk`, `toolsToKronk`, `applyCallOptions`, `chatResponseToContent`.
- `TestCompile` (or `var _ llms.Model`) sanity test that always runs.
- `go vet ./...` + `go test ./... -race` clean.

### Out of Scope

- Shipping or vendoring a GGUF model (user supplies via env var).
- Benchmarking, perf tuning, or coverage gating.
- Wider langchaingo chain integration tests (Manual Verification only).

---

## Inputs

| Input | Source | Notes |
| ----- | ------ | ----- |
| `llmtest.TestLLM(t, model)` entry point | `langchaingo/testing/llmtest/llmtest.go` | Runs Core (`Call`, `GenerateContent`) and probed Capabilities (`Streaming`, `ToolCalls`, `Reasoning`, `Caching`, `TokenCounting`). |
| Phase 2 adapter | `kuzco.go`, `stream.go` | The system under test. |
| Phase 1 mappers | `messages.go` | Unit-tested here directly. |
| `KUZCO_TEST_MODEL_PATH` env var | New convention | Path to a local GGUF; when unset, integration test calls `t.Skip`. |

## Dependencies

| Dependency | Type | Required Before | Notes |
| ---------- | ---- | --------------- | ----- |
| Phase 2 complete | Code | Task 1 | Adapter methods must be implemented. |
| Local GGUF available | External | Task 1 manual run | Required for the integration test to actually execute (CI may skip). |

---

## Implementation Tasks

### Task 1: Strip placeholder, add integration test

- [ ] In `kuzco.go`, remove the placeholder `TestMe` and its imports (`testing`, `llmtest`) — package file stays implementation-only.
- [ ] Create `kuzco_test.go` (package `kuzco_test` to keep it black-box):
  - Look up `os.Getenv("KUZCO_TEST_MODEL_PATH")`; if empty, `t.Skip("set KUZCO_TEST_MODEL_PATH to run llmtest integration")`.
  - Call `kronk.Init(...)` if/where required (verify against `kronk/sdk/kronk/init.go`), build `*kronk.Kronk` via `kronk.New(model.WithModelPath(path))`.
  - Defer any kronk shutdown method if exposed.
  - Build `llm := kuzco.New(k)`.
  - Call `llmtest.TestLLM(t, llm)`.
- [ ] Function name `TestLLM` so it runs under `go test -run TestLLM`.

**Acceptance Criteria:**

- `go test ./...` with the env var unset passes (test is skipped, not failed).
- `KUZCO_TEST_MODEL_PATH=… go test ./... -run TestLLM -v` runs `Core/Call`, `Core/GenerateContent`, and the `Capabilities/Streaming` + `Capabilities/TokenCounting` subtests with all green.

**Files / Areas:**

- `kuzco.go` — delete `TestMe`.
- `kuzco_test.go` — new black-box integration entry.

### Task 2: Unit tests for message mapping

- [ ] New `messages_test.go` (package `kuzco`, white-box).
- [ ] `TestMessagesToKronk` table-driven cases:
  - system + human → two messages, correct roles.
  - human → assistant → human chain → three messages in order.
  - assistant with `llms.ToolCall` part → `tool_calls` array with `id`, `type=function`, `function.name`, `function.arguments`.
  - `llms.ToolCallResponse` part → role `"tool"`, `tool_call_id`, `content` set.
  - unsupported `llms.BinaryContent` → returns error containing `"unsupported part"`.
- [ ] Round-trip a representative output through `json.Marshal` and assert key presence (`messages[0].role`, `messages[1].tool_calls[0].function.name`).

**Acceptance Criteria:**

- All cases pass; failure messages cite which case failed.

**Files / Areas:**

- `messages_test.go`.

### Task 3: Unit tests for options + tools

- [ ] `TestApplyCallOptions` table:
  - empty options → `d` unchanged.
  - all-fields set → every documented key present with the right value.
  - `StreamingFunc != nil` → `d["stream"] == true`.
  - `Tools` set → `d["tools"]` is non-empty slice.
- [ ] `TestToolsToKronk`:
  - nil/empty → nil.
  - one tool → one map with `type` + `function` keys.

**Acceptance Criteria:**

- Tests pass; assertions use `reflect.DeepEqual` or `cmp.Diff` for clarity.

**Files / Areas:**

- `messages_test.go`.

### Task 4: Unit test for response mapping

- [ ] `TestChatResponseToContent`:
  - response with one text choice + usage → `Choices[0].Content` set; `GenerationInfo["PromptTokens"|"CompletionTokens"|"TotalTokens"]` populated as `int`.
  - response with tool calls → `Choices[0].ToolCalls` length and fields match.
  - empty `Choices` → returns non-nil `*ContentResponse` with empty slice (no panic).

**Acceptance Criteria:**

- All assertions pass; `GenerationInfo` keys match exactly what `llmtest.testTokenCounting` looks up.

**Files / Areas:**

- `messages_test.go`.

### Task 5: Sanity compile test

- [ ] Add `TestCompile` (no-op body) plus `var _ llms.Model = (*LLM)(nil)` (already in `kuzco.go`) so CI without a model still exercises the package.

**Acceptance Criteria:**

- `go test ./... -count=1` passes with env unset.

**Files / Areas:**

- `kuzco_test.go`.

---

## Verification

### Automated

```bash
go vet ./...
go test ./... -race -count=1
# Full integration run (requires GGUF):
KUZCO_TEST_MODEL_PATH=/path/to/model.gguf go test ./... -run TestLLM -v -race
```

### Manual

1. Run the integration test with a small instruct-tuned GGUF and confirm `llmtest` reports green for Core + Streaming + TokenCounting.
2. From a scratch `main.go`, build `chains.NewLLMChain(kuzco.New(k), prompt)` and invoke it; confirm the chain produces output (smoke test of real-world use).
3. Re-run the integration test with `-run TestLLM/Core` and `-run TestLLM/Capabilities/Streaming` to confirm targeted subtests can be selected.

---

## Risks

| Risk | Likelihood | Mitigation |
| ---- | ---------- | ---------- |
| `llmtest`'s `Capabilities/ToolCalls` subtest fires only for tool-capable models — failure may mean model, not adapter | Medium | Use a known tool-calling instruct model (e.g. a small Qwen/Llama variant) when verifying; document the recommended model in `kuzco_test.go`. |
| `Capabilities/Caching` may take noticeable time on a slow local box | Low | Acceptable — it's an integration test gated by env var. |
| Race detector flags the streaming goroutine | Medium | Make sure the channel send and ctx-cancel happen in the goroutine, not the caller; only close once. |
| `kronk.Init` requires a library path the test machine doesn't have | Medium | Document the required `kronk.Init(...)` call and library path env var (e.g. `KRONK_LIB`) in the test file's top comment. |

## Open Questions

- Which small GGUF should be the reference for local verification? Pick one and document it as a comment in `kuzco_test.go`.
- Should the integration test additionally run `llmtest.ValidateLLM` before `TestLLM` to surface clearer error messages on basic breakage?
- **Revisit `GenerateContentStream`**: it is NOT part of the `llms.Model` interface — only `Call` and `GenerateContent` are. `llmtest` discovers streaming via reflection (`supportsStreaming` probes for the exact method signature on the concrete type), so the method is an optional capability, not an interface requirement. Phase 2 implemented it to unlock `Capabilities/Streaming` in `llmtest`. During Phase 3, decide whether to:
  - keep it as-is (current default — needed for the streaming subtest to run),
  - drop it entirely (skips the streaming capability test cleanly), or
  - move it behind a build tag / separate type if we want a "core-only" adapter variant.
  Do not modify `GenerateContentStream` until this decision is made; treat the Phase 2 implementation as the baseline.

## Definition of Done

- [ ] Placeholder `TestMe` removed
- [ ] `messages_test.go` covers all four mappers with table-driven cases
- [ ] `kuzco_test.go` runs `llmtest.TestLLM` env-gated
- [ ] `go vet ./...` and `go test ./... -race` clean with env unset
- [ ] Documented manual run with a real GGUF shows `llmtest` Core + Streaming + TokenCounting green
- [ ] PRD `Summary` section in the source PRD is filled in after the manual run

---

## Handoff Notes

Once Phase 3's manual run is complete, write the PRD's `## Summary` section (2–3 sentences on what shipped) and move `2026-05-24-2003-kuzco-langchaingo-adapter.md` from `docs/PRDs/active/` to a `completed/` sibling. The phase files can stay where they are or move alongside the source PRD — pick whichever convention this repo settles on.
