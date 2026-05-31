# Kuzco Embedding Options - Phase 2: Payload Merge in `CreateEmbedding`

**Source PRD**: ../2026-05-31-0859-kuzco-embedding-options.md
**PRD ID**: PRD-2026-05-31-0859
**Phase**: 2 of 3
**Status**: Ready
**Created**: May 31, 2026
**Author**: thetnaingtn

---

## Objective

Thread the `embedOpts` values that Phase 1 added into the `model.D` payload that `CreateEmbedding` forwards to `l.k.Embeddings`. Configured options must reach kronk; the "no opts configured" payload must remain byte-identical to today's `model.D{"input": texts}` so existing callers see no change.

Extract the payload construction into a private helper (`buildEmbedPayload`) so the option matrix can be table-tested without a live kronk — this is the test seam the PRD calls out.

## Scope

### In Scope

- A private method `buildEmbedPayload(texts []string) model.D` on `*LLM` that materialises the request map and conditionally merges `truncate` / `truncate_direction` / `dimension`.
- Refactor `CreateEmbedding` in `embeddings.go` to delegate to that helper.
- Table-driven unit tests covering every option permutation: none set, each in isolation, all three combined, plus the "truncate explicitly false" case (pointer-bool sentinel from Phase 1).

### Out of Scope

- Any change to option types or constructors (Phase 1).
- Integration tests against a real GGUF embed model (Phase 3).
- Documentation updates (Phase 3).
- Changes to the response-side translation (`embedResponseToVectors` stays as-is).

---

## Inputs

| Input | Source | Notes |
| ----- | ------ | ----- |
| Current payload shape | `embeddings.go` line 20 | `l.k.Embeddings(ctx, model.D{"input": texts})`. |
| Option state to merge | `LLM.embed` from Phase 1 | `truncate *bool`, `truncateDirection TruncateDirection`, `dimension int`. |
| Kronk key names | `~/go/pkg/mod/github.com/ardanlabs/kronk@v1.26.1/sdk/kronk/embedding.go` | `truncate`, `truncate_direction`, `dimension`. |
| `model.D` type | `github.com/ardanlabs/kronk/sdk/kronk/model` | Plain `map[string]any` alias kronk's validators accept. |

## Dependencies

| Dependency | Type | Required Before | Notes |
| ---------- | ---- | --------------- | ----- |
| Phase 1 (option types + `embed` field on `LLM`) | Code | Task 1 | Helper can't reference `l.embed` until Phase 1 lands. |

---

## Implementation Tasks

### Task 1: Extract `buildEmbedPayload` helper and refactor `CreateEmbedding`

- [ ] In `embeddings.go`, add `func (l *LLM) buildEmbedPayload(texts []string) model.D`.
- [ ] Start the map with `model.D{"input": texts}`.
- [ ] If `l.embed.truncate != nil`, set `d["truncate"] = *l.embed.truncate`.
- [ ] If `l.embed.truncateDirection != ""`, set `d["truncate_direction"] = string(l.embed.truncateDirection)`.
- [ ] If `l.embed.dimension > 0`, set `d["dimension"] = l.embed.dimension`.
- [ ] In `CreateEmbedding`, replace the inline map literal with `d := l.buildEmbedPayload(texts)` and pass `d` to `l.k.Embeddings(ctx, d)`.
- [ ] Preserve the existing empty-input guard (`errEmptyInput`) and `ensureDeadline` call ordering exactly.

**Acceptance Criteria:**

- `CreateEmbedding` behavior is unchanged when no options are configured — payload is `model.D{"input": texts}` and nothing more.
- When options are configured, the corresponding keys appear in the payload with the documented kronk values.
- All existing tests (`go test -v -count=1 ./...`) still pass.

**Files / Areas:**

- `embeddings.go` — add helper, refactor `CreateEmbedding`.

### Task 2: Table-driven payload tests

- [ ] In `embeddings_test.go` (in `package kuzco` so the test can call the unexported helper directly), add `TestBuildEmbedPayload` with table cases:
  - **no opts** → keys exactly `{"input"}`.
  - **truncate=true only** → keys `{"input", "truncate"}`, `truncate == true`.
  - **truncate=false only** (set via `WithEmbeddingTruncate(false)`) → keys `{"input", "truncate"}`, `truncate == false`. This proves the pointer-bool sentinel works.
  - **direction=left only** → keys `{"input", "truncate_direction"}`, value `"left"`.
  - **direction=right only** → keys `{"input", "truncate_direction"}`, value `"right"`.
  - **dimension=256 only** → keys `{"input", "dimension"}`, value `256`.
  - **all three combined** → keys `{"input", "truncate", "truncate_direction", "dimension"}` with expected values.
- [ ] Each case builds `*LLM` via `New(nil, opts...)` (nil kronk is fine — the helper does not call it) and inspects the returned `model.D` directly.
- [ ] Assert key set with a helper like `keysOf(d)` or by checking `len(d)` plus explicit lookups — avoid `reflect.DeepEqual` against a full literal because `texts` is a slice and equality semantics get noisy.

**Acceptance Criteria:**

- All seven table cases pass.
- The "truncate=false" case fails before the pointer-bool change and passes after — i.e., the test actually exercises the sentinel.
- No live kronk required; the test runs offline.

**Files / Areas:**

- `embeddings_test.go` — `TestBuildEmbedPayload` table.

---

## Verification

### Automated

```bash
go test -v -count=1 ./...
go vet ./...
```

Also confirm the existing compile-time assertions still hold:

```bash
go test -v -run TestCompile -count=1 ./...
```

(`kuzco_compile_test.go` asserts `*LLM` satisfies `embeddings.EmbedderClient` and `llms.Model` — those interface checks must not regress.)

### Manual

1. Inspect a `git diff` of `embeddings.go` and confirm the no-opts path produces the same map literal as today (just routed through the helper).
2. Spot-check one unit test by inverting the expected key set — confirm it fails — then revert. Proves the assertions are tight.

---

## Risks

| Risk | Likelihood | Mitigation |
| ---- | ---------- | ---------- |
| Helper accidentally allocates the map with extra keys when no opts set | Medium | The "no opts" table case asserts `len(d) == 1` and key set `{"input"}` exactly. |
| Map iteration order leaks into a test assertion | Low | Tests assert on keys/values directly, never on map literal equality. |
| `model.D` field type mismatch (e.g. passing `int64` where kronk expects `int`) | Low | `dimension` is plain `int`; kronk decodes via `any`-typed map so Go's default int kind is fine. |
| Refactor changes error wrapping or deadline behavior | Low | Task 1 explicitly says preserve the empty-input guard and `ensureDeadline` ordering; spot-check in code review. |

## Open Questions

- Should the helper be a method on `*LLM` or a free function taking `embedOpts`? Prefer the method — it keeps option state encapsulated and the test seam clean.

## Definition of Done

- [ ] All implementation tasks completed
- [ ] Acceptance criteria verified
- [ ] `go test -v -count=1 ./...` passes
- [ ] `go vet ./...` clean
- [ ] No unresolved blockers remain

---

## Handoff Notes

Phase 3 will exercise the full path against a real embed model. Keep `buildEmbedPayload` exported-package-only (lowercase) — it is a test seam, not a public API. If the table-test helper for key-set comparison is useful elsewhere, leave it in `embeddings_test.go`; do not move it to a shared helpers file until a second user appears.
