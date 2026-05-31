# Kuzco: Embedding Options (truncate, truncate_direction, dimension)

**PRD ID**: PRD-2026-05-31-0859
**Status**: Draft
**Complexity**: Low
**Created**: May 31, 2026
**Author**: thetnaingtn

---

## Problem

`kuzco.LLM.CreateEmbedding` currently forwards only the `input` field to `kronk.Embeddings`, hardcoding kronk's defaults for every other knob. Kronk's embedding API exposes request-level options that callers need in production pipelines:

- `truncate` (bool): if true, truncate input to fit the model's context window (default: false — over-long inputs fail).
- `truncate_direction` (string): `"right"` (default) or `"left"`.
- `dimension` (int): requested output dimension for Matryoshka-trained embedding models (e.g. nomic-embed-v1.5, mxbai-embed-large) that can return a shorter vector without re-encoding.

Because langchaingo's `embeddings.EmbedderClient.CreateEmbedding(ctx, texts)` signature does **not** accept call options, there is no per-call hook for the caller to pass these. The only place to configure them is at adapter construction. Today `kuzco.New` exposes one `Option` (`WithDefaultTimeout`) and no way to tune embedding behavior, so callers feeding long documents to a BGE or nomic-embed GGUF either hit context-overflow errors or must pre-truncate every input themselves, and callers using Matryoshka models cannot shrink the output vector for storage/latency wins.

## Solution

Extend the existing functional-option pattern on `kuzco.New` with three new constructor options that configure embedding behavior for the lifetime of the `*LLM`:

```go
kuzco.New(k,
    kuzco.WithEmbeddingTruncate(true),
    kuzco.WithEmbeddingTruncateDirection(kuzco.TruncateLeft),
    kuzco.WithEmbeddingDimension(256), // Matryoshka downsize
)
```

Implementation outline:

- Add a private `embedOpts` struct field on `LLM` holding the configured values.
- Add `WithEmbeddingTruncate(bool) Option`, `WithEmbeddingTruncateDirection(TruncateDirection) Option`, and `WithEmbeddingDimension(int) Option`.
- Introduce a typed `TruncateDirection` string with two exported constants (`TruncateRight`, `TruncateLeft`) so callers cannot pass arbitrary strings.
- In `CreateEmbedding`, merge configured options into the `model.D` payload **only when set** — preserve current "no key sent" behavior when the caller never configured an option, so kronk's defaults remain authoritative.
- Treat non-positive `dimension` values as "unset" (zero-value guard) and skip invalid `TruncateDirection` values silently, since constructor options have no error return.

## Summary

_To be filled in after implementation._

---

## Scope

### In Scope

- Three new functional options on `kuzco.New`: `WithEmbeddingTruncate`, `WithEmbeddingTruncateDirection`, `WithEmbeddingDimension`.
- A typed `TruncateDirection` (`TruncateRight` / `TruncateLeft`) to keep the API safe.
- Threading configured options into the `model.D` map passed to `l.k.Embeddings`.
- Unit tests covering: option setters, payload-merge behavior (set vs unset, each option independently and combined), and rejection of invalid `TruncateDirection` and non-positive `dimension`.
- Integration test variant that runs an embedding call with `truncate=true` against the existing test model.

### Out of Scope

- Per-call embedding options (blocked by langchaingo's `EmbedderClient` signature).
- Adding chat/generation options through the same mechanism — chat already exposes `llms.CallOption` per call.
- Any change to `Kronk` or `langchaingo` upstream.
- Exposing additional kronk request fields beyond the three listed (deferred until concrete demand).
- Validating that the configured `dimension` is supported by the loaded model (kronk/llama.cpp surfaces this).

### Target Users

| Role                              | Impact                                                                                                                                |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| Application developer (Go)        | Can construct an embedder that tolerates long documents and right-sizes Matryoshka vectors without writing glue or pinning a fork.    |
| Library maintainer (this project) | Extension point established for future kronk request-level knobs without breaking the public API.                                     |

---

## Technical Design

### Architecture

```mermaid
flowchart LR
    A[App code] -->|kuzco.New + opts| B[*kuzco.LLM]
    B -->|CreateEmbedding ctx, texts| C[embeddings.go]
    C -->|merge embedOpts into model.D| D[kronk.Embeddings]
    D --> E[(GGUF embed model)]
    E -->|EmbedReponse| C
    C -->|[][]float32 sorted by Index| A
```

### Data Layer

No schema or persistence changes. The change is in-process in a single Go package; no database, no migrations.

### Backend

| Component       | Changes                                                                                                                                                                                                            |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `kuzco.go`      | Add `embedOpts` struct field on `LLM`. Add `TruncateDirection` type + `TruncateRight` / `TruncateLeft` constants. Add `WithEmbeddingTruncate`, `WithEmbeddingTruncateDirection`, `WithEmbeddingDimension` options. |
| `embeddings.go` | In `CreateEmbedding`, conditionally write `truncate`, `truncate_direction`, and `dimension` into the `model.D` payload from `l.embedOpts`.                                                                         |
| Tests           | New cases in `embeddings_test.go` for option merging; new integration case in `kuzco_embedding_test.go` exercising `truncate=true` (and `dimension` if test model supports it).                                    |

### Frontend

N/A — library has no UI.

---

## Implementation

### Phase 1: Option types and constructors

- [ ] Add `TruncateDirection` string type and exported constants `TruncateRight` (`"right"`), `TruncateLeft` (`"left"`) in `kuzco.go`.
- [ ] Add unexported `embedOpts` struct with `truncate *bool`, `truncateDirection TruncateDirection`, and `dimension int`, hung off `*LLM`.
- [ ] Implement `WithEmbeddingTruncate(bool) Option` (writes to `embedOpts.truncate` via a pointer so "set to false" is distinguishable from "unset").
- [ ] Implement `WithEmbeddingTruncateDirection(TruncateDirection) Option`; ignore invalid values (only `TruncateRight` / `TruncateLeft` apply).
- [ ] Implement `WithEmbeddingDimension(int) Option`; ignore non-positive values (zero-value guard).
- [ ] Unit tests in `embeddings_test.go` (or a new `options_test.go`) asserting each option mutates the struct as expected and that invalid values are no-ops.

### Phase 2: Payload merge in CreateEmbedding

- [ ] Update `CreateEmbedding` in `embeddings.go` to build the `model.D` map and merge `truncate` / `truncate_direction` / `dimension` only when set on `embedOpts`.
- [ ] Keep "no opts configured" output byte-identical to today's `model.D{"input": texts}` so existing kronk default behavior is preserved.
- [ ] Unit tests: build an `*LLM` with each option permutation and assert the produced `model.D` payload matches expectations. Use a thin seam (e.g. extract payload construction into a private helper that returns `model.D`) so the test can assert on the map without a live kronk.

### Phase 3: Integration test + docs

- [ ] Add an integration test in `kuzco_embedding_test.go` that runs `CreateEmbedding` against an over-long input with `WithEmbeddingTruncate(true)` and verifies it succeeds where the unconfigured embedder would fail.
- [ ] If the test embed model is Matryoshka-capable, add a case asserting `WithEmbeddingDimension(N)` produces vectors of length `N`; otherwise document the limitation and skip.
- [ ] Update `CLAUDE.md` adapter-gotchas section to mention the embedding option knobs.
- [ ] Update README (if present) or package doc comment with a short usage example.

---

## Security

| Concern          | Mitigation                                                                                                                                       |
| ---------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| Authorization    | N/A — in-process library; no new endpoints or auth surface.                                                                                      |
| Input validation | `TruncateDirection` is a typed string with two valid constants; invalid values and non-positive `dimension` are silently ignored to keep defaults. |
| Data exposure    | No new logging, telemetry, or persistence. Payload is forwarded to the same kronk endpoint already used.                                         |

---

## Testing

**Automated:**

```bash
go test -v ./...
```

(Integration cases under `kuzco_embedding_test.go` download a GGUF embed model via `MODEL_URL` and skip cleanly when network is unavailable, per the repo's existing pattern.)

**Manual Verification:**

1. Build a small program: `kuzco.New(k, kuzco.WithEmbeddingTruncate(true), kuzco.WithEmbeddingTruncateDirection(kuzco.TruncateLeft), kuzco.WithEmbeddingDimension(256))`.
2. Call `CreateEmbedding` with an input deliberately longer than the model's context window; confirm a non-error vector is returned and its length equals 256 (assuming a Matryoshka model is loaded).
3. Re-run without `WithEmbeddingTruncate(true)` and confirm kronk now returns a context-overflow error — proves the option is actually wired through.
4. Re-run without `WithEmbeddingDimension` and confirm the vector reverts to the model's native dimension.

---

## Risks

| Risk                                                       | Likelihood | Mitigation                                                                                                                       |
| ---------------------------------------------------------- | ---------- | -------------------------------------------------------------------------------------------------------------------------------- |
| Kronk renames or restructures the request keys             | Low        | Keys are passed as plain `model.D` strings; pin kronk version in `go.mod` and assert behavior in the integration test.           |
| `dimension` rejected by non-Matryoshka models              | Med        | Kronk surfaces the error; document that the option is model-dependent and demonstrate with a known Matryoshka GGUF in the example. |
| Constructor option silently ignores invalid values         | Med        | Document the constants and the positive-int requirement; consider an `exhaustive` linter rule once a linter is configured.       |
| Future demand for per-call options                         | Med        | Out of scope — would require an upstream langchaingo change. Document the constraint clearly in the package doc.                 |
| Pointer-bool field for `truncate` adds API ambiguity       | Low        | Internal-only field; public option signature stays `bool`. Pointer is just to distinguish "unset" from "set to false" inside.    |

---

## Definition of Done

- [ ] Implementation complete
- [ ] Tests passing (`go test -v ./...`)
- [ ] No new vet warnings (`go vet ./...`)
- [ ] CLAUDE.md and package doc updated
- [ ] PR approved and merged via Conventional Commit (`feat: embedding options for kronk`)

---

## Files Changed

| Category | Files                       | Description                                                                                                  |
| -------- | --------------------------- | ------------------------------------------------------------------------------------------------------------ |
| Backend  | `kuzco.go`                  | Add `TruncateDirection` type + constants, `embedOpts` field, three new `With...` constructor options.        |
| Backend  | `embeddings.go`             | Conditionally merge `truncate` / `truncate_direction` / `dimension` into the `model.D` payload.              |
| Tests    | `embeddings_test.go`        | Unit tests for option setters, invalid-value handling, and payload-merge behavior.                           |
| Tests    | `kuzco_embedding_test.go`   | Integration test exercising `WithEmbeddingTruncate(true)` (and `WithEmbeddingDimension` if model supports). |
| Docs     | `CLAUDE.md`                 | One-line note that embedding options are configured on `New`, not per call.                                  |

---

## Related

- **Depends on**: [PRD-2026-05-26-1121 — Kuzco embeddings adapter](./2026-05-26-1121-kuzco-embeddings-adapter.md) (provides `CreateEmbedding` this PRD extends).
- **Upstream constraint**: `github.com/tmc/langchaingo/embeddings.EmbedderClient.CreateEmbedding` has no `...Option` parameter, which is why options must live on `kuzco.New`.
- **Kronk reference**: `github.com/ardanlabs/kronk/sdk/kronk/embedding.go` documents `truncate`, `truncate_direction`, and `dimension`.

---

_Last updated: May 31, 2026_
