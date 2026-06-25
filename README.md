# doubao-speech-go

[![CI Pass](https://github.com/GizClaw/doubao-speech-go/actions/workflows/ci.yml/badge.svg)](https://github.com/GizClaw/doubao-speech-go/actions/workflows/ci.yml)
[![Code Scan](https://github.com/GizClaw/doubao-speech-go/actions/workflows/codeql.yml/badge.svg)](https://github.com/GizClaw/doubao-speech-go/actions/workflows/codeql.yml)
[![Go Pass A+](https://goreportcard.com/badge/github.com/GizClaw/doubao-speech-go)](https://goreportcard.com/report/github.com/GizClaw/doubao-speech-go)

Go SDK for Doubao/Volc speech APIs.

## Features

- ASR V2 SAUC WebSocket streaming
- TTS HTTP Chunked streaming
- TTS WebSocket bidirectional streaming
- Realtime session API
- Realtime duplex dialogue API
- AST V2 realtime translation
- Voice clone upload + polling workflow
- Audio Generation HTTP API

## Public API Surface

Create a client with API-key authentication:

```go
client := doubaospeech.NewClient(appID, doubaospeech.WithAPIKey(apiKey))
```

Use the canonical service fields below. Short aliases such as `ASR`, `TTS`,
`AST`, and `Audio` are intentionally not exposed.

| Service field | API |
| --- | --- |
| `client.ASRV2` | Streaming ASR |
| `client.TTSV2` | TTS HTTP Chunked and bidirectional WebSocket synthesis |
| `client.Realtime` | Realtime speech 1.0 |
| `client.RealtimeDuplex` | Realtime full-duplex dialogue |
| `client.ASTTranslate` | AST simultaneous interpretation |
| `client.AudioGeneration` | Audio Generation HTTP API |
| `client.VoiceClone` | Voice clone upload, status, and activation |

## Roadmap

The detailed API inventory lives in [`docs/`](docs/). Current coverage is:

Implemented:

- [x] TTS HTTP Chunked stream: `TTSV2.Stream` implements `POST /api/v3/tts/unidirectional` with streamed JSON frames and decoded audio chunks.
- [x] TTS bidirectional WebSocket: `TTSV2.OpenStreamSession` supports `StartConnection`, `StartSession`, text `TaskRequest`, streamed audio responses, `CancelSession`, and sequential session reuse.
- [x] Realtime speech 1.0: `Realtime.OpenSession` supports text/audio input modes, push-to-talk, client interrupt, TTS text injection, parsed ASR/TTS/chat events, and conversation state helpers.
- [x] Realtime full-duplex: `RealtimeDuplex.OpenSession` supports JSON text-frame sessions, audio append/commit, speech-text replacement, response cancel, context CRUD, function-calling results, and streamed output events.
- [x] AST simultaneous interpretation: `ASTTranslate.OpenSession` supports S2T/S2S start, audio `TaskRequest`, `UpdateConfig`, `FinishSession`, parsed subtitle/TTS/usage/muted events, and protobuf transport.
- [x] Audio generation: `AudioGeneration.Create` implements `POST /api/v3/tts/create` with text prompts, speaker/audio/image references, typed audio config, watermark options, decoded audio bytes, and temporary audio URL response fields.

Partially implemented:

- [ ] Streaming ASR: `ASRV2.OpenStreamSession` implements `/api/v3/sauc/bigmodel`; `/api/v3/sauc/bigmodel_nostream` and `/api/v3/sauc/bigmodel_async` are documented but not exposed as high-level SDK modes yet.
- [ ] TTS 1.0/2.0 parameter surface: the SDK covers common speaker/audio/text fields, but advanced official fields such as subtitles, timestamps, usage token return, cache, AIGC watermarking, explicit language/dialect, `context_texts`, and tag parsing still need typed options.
- [ ] TTS WebSocket single-direction and SSE variants: `docs/tts_v1.md` lists `wss://.../tts/unidirectional/stream` and `/tts/unidirectional/sse`; only HTTP Chunked and bidirectional WebSocket are implemented.
- [ ] Voice clone: `VoiceClone.Upload`, `GetStatus`, and `Activate` implement the older `/api/v1/mega_tts/*` workflow; the newer documented `POST /api/v3/tts/voice_clone` interface in `docs/voice_clone.md` is not implemented yet.
- [ ] Realtime 1.0 and full-duplex extensions: core events and extension pass-through are present, but newly documented provider-specific fields should be periodically audited against upstream docs.

Planned:

- [ ] Voice clone v3: add a typed client for `POST /api/v3/tts/voice_clone` and map its `speaker_status`, `model_type`, and demo-audio response.
- [ ] ASR optimized/nostream modes: add typed selection for `bigmodel_async` and `bigmodel_nostream`, including final-packet semantics and language/options coverage.
- [ ] TTS single-direction WebSocket and SSE stream helpers.
- [ ] Broaden examples so every implemented roadmap item has a runnable CLI smoke path.

## Requirements

- Go `1.26+`
- Git LFS (for embedded audio fixture)

```bash
git lfs pull
```

## Install

```bash
go get github.com/GizClaw/doubao-speech-go
```

## Quick Start

Run the ASR V2 example:

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/asr_v2_sauc_ws
```

Run the Voice Clone example:

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/voice_clone -speaker-id <speaker_id> -audio /path/to/sample.wav
```

## Development

Format:

```bash
gofmt -w *.go internal/astproto/*.go internal/auth/*.go internal/protocol/*.go internal/transport/*.go internal/util/*.go examples/asr_v2_sauc_ws/*.go examples/ast_v2_translate/*.go examples/audio_generation/*.go examples/realtime/*.go examples/realtime_duplex/*.go examples/tts_v2/http_stream/*.go examples/tts_v2/websocket/*.go examples/voice_clone/*.go
```

Build / test / vet:

```bash
go build ./...
go test ./...
go vet ./...
```

Run one test:

```bash
go test . -run TestVoiceCloneUploadAndWaitSuccess -count=1 -v
```

## Documentation

- `docs/streaming_asr.md`
- `docs/simultaneous_interpretation.md`
- `docs/realtime_speech.md`
- `docs/realtime_duplex.md`
- `docs/tts_v1.md`
- `docs/tts_v2.md`
- `docs/voice_clone.md`
- `docs/audio_generation.md`

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE).
