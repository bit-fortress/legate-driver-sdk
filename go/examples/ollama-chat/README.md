# Ollama Chat WASM Driver

This Text Driver exposes Ollama's native `POST /api/chat` API through Legate's
`openai.chat_completions/2026-07-18` protocol contract.

Supported conversions include:

- buffered responses and native Ollama NDJSON streams converted to OpenAI SSE;
- text and base64 data-URL image messages;
- OpenAI `developer` messages as Ollama `system` messages;
- temperature, top-p, seed, stop, and token-limit options;
- JSON object and JSON Schema response formats;
- function definitions, assistant tool calls, and tool result messages;
- reasoning effort and Ollama thinking output;
- Ollama log probabilities and prompt/completion token usage; and
- stable OpenAI error envelopes with caller, endpoint, and model mapping outcomes.

Unknown OpenAI contract extensions are preserved for Ollama. Known fields with
unsupported generation semantics are rejected with an OpenAI-compatible 400
response before any upstream request is made.

## Build

Install TinyGo 0.39.0, then run:

```sh
make test
make verify
```

Upload `manifest.json` and the generated `driver.wasm` together.

## Provider configuration

For local Ollama:

- endpoint: `http://localhost:11434` (or the address reachable by Legate);
- config: `{}` or `{"authentication":"none"}`;
- credential: none.

For the hosted Ollama API:

- endpoint: `https://ollama.com`;
- config: `{"authentication":"bearer"}`;
- credential slot `api_key`: an Ollama API key.

The model configured in the Legate provider mapping is sent to Ollama. Responses
retain the client-facing Legate model-group name and Core-generated response ID.
