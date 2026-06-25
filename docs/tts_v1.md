# TTS 1.0

Official documentation: <https://www.volcengine.com/docs/6561/1329505>

This page maps the VolcEngine TTS 1.0 API list to the current SDK surface. The
official page covers several TTS transport variants in one document.

## Official API List

| Transport | Endpoint | SDK coverage |
| --- | --- | --- |
| WebSocket bidirectional | `wss://openspeech.bytedance.com/api/v3/tts/bidirection` | Implemented by `TTSV2.OpenStreamSession` |
| WebSocket unidirectional stream | `wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream` | Not implemented as a dedicated helper |
| HTTP Chunked stream | `https://openspeech.bytedance.com/api/v3/tts/unidirectional` | Implemented by `TTSV2.Stream` |
| HTTP SSE stream | `https://openspeech.bytedance.com/api/v3/tts/unidirectional/sse` | Not implemented as a dedicated helper |

The old standalone local HTTP-stream document was removed because the HTTP
Chunked endpoint is covered here and by the README roadmap.

## Resource IDs

| Resource ID | Meaning |
| --- | --- |
| `seed-tts-2.0` | TTS 2.0 character billing |
| `seed-tts-1.0` | TTS 1.0 character billing |
| `seed-tts-1.0-concurr` | TTS 1.0 concurrent billing |
| `seed-icl-2.0` | voice clone 2.0 character billing |
| `seed-icl-1.0` | voice clone 1.0 character billing |
| `seed-icl-1.0-concurr` | voice clone 1.0 concurrent billing |

## Implemented Bidirectional Flow

`TTSV2.OpenStreamSession` supports:

- `StartConnection`
- `StartSession`
- text `TaskRequest`
- streamed audio responses
- `CancelSession`
- `FinishSession`
- sequential session reuse on one WebSocket connection

## Implemented HTTP Chunked Flow

`TTSV2.Stream` supports `POST /api/v3/tts/unidirectional` and returns streamed
audio chunks from the HTTP response.

## Important Options

The official API exposes more fields than the SDK currently types, including:

- subtitle and timestamp output
- explicit language and dialect
- cache configuration
- AIGC watermark and metadata
- markdown and emoji filtering
- `context_texts`
- `section_id`
- tag parser support for expressive cloned voices
- mix speaker configuration

These should be added as typed options incrementally.

## Examples

HTTP Chunked:

```bash
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/tts_v2/http_stream
```

WebSocket bidirectional:

```bash
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/tts_v2/websocket
```
