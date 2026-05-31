# kuzco

`kuzco` is a thin Go adapter that lets you drive a local LLM through
[langchaingo](https://github.com/tmc/langchaingo). It wraps an
[`*kronk.Kronk`](https://github.com/ardanlabs/kronk) inference instance and
exposes it as a langchaingo [`llms.Model`](https://pkg.go.dev/github.com/tmc/langchaingo/llms#Model)
and [`embeddings.EmbedderClient`](https://pkg.go.dev/github.com/tmc/langchaingo/embeddings#EmbedderClient).

```
┌────────────────────┐     llms.Model /        ┌───────────┐     ┌──────────────┐
│   langchaingo app   │──── EmbedderClient ────▶│   kuzco   │────▶│ kronk (local │
│ (chains, agents…)   │                         │  adapter  │     │  GGUF infer) │
└────────────────────┘                         └───────────┘     └──────────────┘
```

## Why kuzco?

langchaingo has a rich ecosystem of chains, agents, retrievers, and embedders,
but its model implementations mostly target hosted APIs (OpenAI, Anthropic,
etc.). [kronk](https://github.com/ardanlabs/kronk) runs GGUF models locally on
top of `llama.cpp`, giving you fully on-device inference with no API keys and
no data leaving the machine.

The two don't speak the same interface out of the box. `kuzco` bridges them
**in-process** so you can:

- Run **local, private inference** behind the langchaingo surface you already use.
- Reuse existing langchaingo **chains, agents, and tools** without rewriting them.
- Swap a hosted model for a local GGUF (or back) by changing a single constructor.

## Two ways to use kronk with langchaingo

kronk can be reached two ways, and which langchaingo constructor you use
depends on the path you pick:

1. **In-process, via `kuzco`** *(this library)* — embed `*kronk.Kronk`
   directly in your Go process and wrap it with `kuzco.New`. No HTTP server, no
   ports, no serialization overhead. Use the `kuzco` constructor below.

2. **Over HTTP, via kronk's OpenAI-compatible API** — kronk also ships
   OpenAI-shaped HTTP handlers (`ChatStreamingHTTP`, `EmbeddingHTTP`, …) that
   speak the `chat/completions` / `embeddings` wire format. If you stand kronk
   up as a server, you don't need `kuzco` at all: point langchaingo's **OpenAI**
   constructor at the kronk endpoint via a custom base URL.

   ```go
   import "github.com/tmc/langchaingo/llms/openai"

   llm, err := openai.New(
       openai.WithBaseURL("http://localhost:8080/v1"), // your kronk server
       openai.WithToken("not-needed"),                 // any non-empty token
       openai.WithModel("your-model"),
   )
   ```

Reach for `kuzco` when you want kronk running inside the same process; reach
for the OpenAI constructor when kronk is a separate service. The rest of this
README covers the in-process `kuzco` path.

## Features

- **Chat completion** — `Call`, `GenerateContent`, and `GenerateContentStream`.
- **Streaming** — token-by-token via `WithStreamingFunc`, plus a channel-based stream API.
- **Tool / function calling** — langchaingo tools are translated to kronk's payload, with `ToolChoice` defaulting to `"auto"` when tools are present.
- **Embeddings** — implements `EmbedderClient` for embed-capable GGUF models, with truncation controls.
- **Sensible context handling** — automatically applies a default deadline (60s) when the caller's context has none, which kronk requires.

## Installation

```bash
go get github.com/thetnaingtn/kuzco
```

## Usage

`kuzco.New` takes a fully-configured `*kronk.Kronk`. Constructing kronk
(downloading the llama.cpp libraries and a GGUF model, then calling
`kronk.Init` / `kronk.New`) is kronk's concern — see the
[kronk SDK docs](https://pkg.go.dev/github.com/ardanlabs/kronk/sdk/kronk).
Once you have one, wrapping it is a single call.

### Chat completion

```go
package main

import (
	"context"
	"fmt"

	"github.com/ardanlabs/kronk/sdk/kronk"
	"github.com/thetnaingtn/kuzco"
	"github.com/tmc/langchaingo/llms"
)

func main() {
	// k is a fully-configured *kronk.Kronk (see kronk docs for setup).
	var k *kronk.Kronk

	llm := kuzco.New(k)

	resp, err := llms.GenerateFromSinglePrompt(context.Background(), llm, "Say OK")
	if err != nil {
		panic(err)
	}
	fmt.Println(resp)
}
```

### Streaming

```go
_, err := llm.GenerateContent(ctx,
	[]llms.MessageContent{
		llms.TextParts(llms.ChatMessageTypeHuman, "Write a haiku about Go"),
	},
	llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
		fmt.Print(string(chunk))
		return nil
	}),
)
```

### Embeddings

Embeddings only work against a GGUF whose `modelInfo.IsEmbedModel` is true.
Calling `CreateEmbedding` on a chat-only model returns an error.

```go
llm := kuzco.New(k)

embedder, err := embeddings.NewEmbedder(llm)
if err != nil {
	panic(err)
}

vec, err := embedder.EmbedQuery(context.Background(), "hello")
if err != nil {
	panic(err)
}
fmt.Println(len(vec))
```
### Examples

See more [examples](https://github.com/thetnaingtn/kronk-examples) for how to use kronk with kuzco


## Options

Pass these to `kuzco.New(k, opts...)`:

| Option | Description | Default |
| --- | --- | --- |
| `WithDefaultTimeout(d time.Duration)` | Timeout applied via context when the caller's context has no deadline. | `60s` |
| `WithEmbeddingTruncate(v bool)` | Whether kronk truncates embedding input that exceeds the model's context. Stored as a pointer so an explicit `false` differs from "unset". | unset |
| `WithEmbeddingTruncateDirection(d TruncateDirection)` | Which end to truncate: `TruncateLeft` or `TruncateRight`. Invalid values are a silent no-op. | unset |

> **TODO:** Support a Matryoshka embedding-dimension option (request a shorter
> output vector from Matryoshka-capable models).

Per-request generation parameters (max tokens, temperature, top-p, stop words,
seed, tools, tool choice, streaming) are passed through the standard
langchaingo [`llms.CallOption`](https://pkg.go.dev/github.com/tmc/langchaingo/llms#CallOption)
values at call time, e.g. `llms.WithTemperature(0.7)`.

## How it works

`kuzco` translates langchaingo types into kronk's request payloads
(`model.D`) and converts kronk's responses back into langchaingo types:

- **Messages** — langchaingo roles (`System`, `Human`, `AI`, `Tool`/`Function`) map to kronk's `system` / `user` / `assistant` / `tool` roles. Image and binary parts are not supported and return an error.
- **Tools** — langchaingo `Tool` definitions become kronk function-tool entries; tool calls and tool responses are round-tripped.
- **Chat is always streamed internally** — `GenerateContent` consumes kronk's streaming channel and assembles a final response, forwarding deltas to a `StreamingFunc` if one is set.

## Testing

```bash
go test -v ./...                    # unit tests (no network, no model downloads)
go test -tags=integration -v ./...  # full suite (downloads llama.cpp libs + GGUF)
```

Integration tests gate on environment variables and skip cleanly when unset:

- `MODEL_URL` — HuggingFace GGUF URL for the chat (`TestLLM`) suite.
- `EMBED_MODEL_URL` — HuggingFace GGUF URL for the embedding suite.
- `KUZCO_TEST_CACHE_DIR` *(optional)* — cache directory for downloaded libs/models (defaults to `~/.kronk/`).
- `KRONK_HF_TOKEN` *(optional)* — HuggingFace token for gated models.

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](./LICENSE) for details.
