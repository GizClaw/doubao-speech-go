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

Create a client with an App ID config value and API-key authentication. The
API key is the only authentication factor; App ID is request/application
configuration used by services that still require it.

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

The detailed API inventory lives in [`docs/`](docs/). Current implementation
status by interface:

| API | Endpoint | SDK surface | Status | Notes |
| --- | --- | --- | --- | --- |
| Streaming ASR bidirectional | `wss://openspeech.bytedance.com/api/v3/sauc/bigmodel` | `client.ASRV2.OpenStreamSession` | Implemented | Streams audio-only frames and parses full-server JSON results. Some upstream request options are not yet typed. See [`docs/streaming_asr.md`](docs/streaming_asr.md). |
| Streaming ASR input-only result | `wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_nostream` | none | Documented only | Needs endpoint selection, final-packet behavior, and typed nostream options. |
| Streaming ASR optimized bidirectional | `wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_async` | none | Documented only | Needs endpoint selection and typed async/nonstream option coverage. |
| TTS WebSocket bidirectional | `wss://openspeech.bytedance.com/api/v3/tts/bidirection` | `client.TTSV2.OpenStreamSession` | Partially implemented | Lifecycle, text request, cancel, reuse, and audio chunks are implemented. Current config types cover `speaker`, `format`, `sample_rate`, and `resource_id`; advanced 2.0 fields still need typed options. See [`docs/tts_v2.md`](docs/tts_v2.md). |
| TTS WebSocket unidirectional stream | `wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream` | none | Documented only | Listed in upstream TTS API inventory. |
| TTS HTTP Chunked stream | `https://openspeech.bytedance.com/api/v3/tts/unidirectional` | `client.TTSV2.Stream` | Implemented | Streams HTTP response frames and decodes audio chunks. See [`docs/tts_v1.md`](docs/tts_v1.md). |
| TTS HTTP SSE stream | `https://openspeech.bytedance.com/api/v3/tts/unidirectional/sse` | none | Documented only | Listed in upstream TTS API inventory. |
| Realtime speech 1.0 | `wss://openspeech.bytedance.com/api/v3/realtime/dialogue` | `client.Realtime.OpenSession` | Implemented | Core session, typed StartSession provider fields, text/audio modes, UpdateConfig, RAG text, conversation CRUD/truncate/delete, push-to-talk, TTS text injection, parsed events, and local context helpers are implemented. See [`docs/realtime_speech.md`](docs/realtime_speech.md). |
| Realtime full-duplex 3.0 | `wss://openspeech.bytedance.com/api/v3/duplex/realtime/dialogue` | `client.RealtimeDuplex.OpenSession` | Implemented | JSON event flow, typed `extension.asr`/`extension.tts`/`extension.dialog`, audio append/commit, replacement text, context CRUD, response cancel, function-call result return, and parsed events are implemented. See [`docs/realtime_duplex.md`](docs/realtime_duplex.md). |
| AST simultaneous interpretation | `wss://openspeech.bytedance.com/api/v4/ast/v2/translate` | `client.ASTTranslate.OpenSession` | Implemented | Supports S2T/S2S start, audio upload, config update, finish, parsed subtitle/TTS/usage/muted events, and protobuf transport. See [`docs/simultaneous_interpretation.md`](docs/simultaneous_interpretation.md). |
| Audio generation | `https://openspeech.bytedance.com/api/v3/tts/create` | `client.AudioGeneration.Create` | Implemented | Supports text prompts, audio/image/speaker references, audio config, watermark config, decoded audio, and temporary URL response fields. See [`docs/audio_generation.md`](docs/audio_generation.md). |
| Voice clone legacy workflow | `/api/v1/mega_tts/audio/upload`, `/api/v1/mega_tts/status`, `/api/v1/mega_tts/audio/activate` | `client.VoiceClone.Upload`, `GetStatus`, `Activate` | Implemented | Current SDK workflow for training, polling, and activation. See [`docs/voice_clone.md`](docs/voice_clone.md). |
| Voice clone v3 | `https://openspeech.bytedance.com/api/v3/tts/voice_clone` | none | Documented only | Needs a typed client for `speaker_id`, `custom_speaker_id`, `audio`, `extra_params`, `speaker_status`, `model_type`, and demo-audio response fields. |

Near-term work:

- [ ] Add typed ASR mode selection for `bigmodel_nostream` and `bigmodel_async`.
- [ ] Expand typed TTS WebSocket 2.0 options for subtitle, usage, cache, AIGC metadata, explicit language/dialect, `context_texts`, `section_id`, and tag parsing.
- [ ] Add TTS WebSocket unidirectional and HTTP SSE helpers.
- [ ] Add typed Voice Clone v3 client for `POST /api/v3/tts/voice_clone`.
- [ ] Broaden examples so every implemented interface has a runnable CLI smoke path.

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
