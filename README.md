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
- Podcast generation WebSocket API with per-round events and retry checkpoints

## Public API Surface

Create a client with an App ID config value and API-key authentication. Most
services use `WithAPIKey`; Podcast uses its API-specific app-key/access-key
pair through `WithAppKey` and `WithAccessKey`.

```go
client := doubaospeech.NewClient(appID, doubaospeech.WithAPIKey(apiKey))
```

Available service fields:

| Service field | API |
| --- | --- |
| `client.ASRV2` | Streaming ASR |
| `client.TTSV2` | TTS HTTP Chunked and bidirectional WebSocket synthesis |
| `client.Realtime` | Realtime speech 1.0 |
| `client.RealtimeDuplex` | Realtime full-duplex dialogue |
| `client.ASTTranslate` | AST simultaneous interpretation |
| `client.AudioGeneration` | Audio Generation HTTP API |
| `client.Podcast` | Podcast generation WebSocket API |
| `client.VoiceClone` | Voice clone upload, status, and activation |

## Roadmap

The detailed API inventory lives in [`docs/`](docs/). Current implementation
status by interface:

| Mark | API | Endpoint | SDK surface | Notes |
| --- | --- | --- | --- | --- |
| 🟢 | Streaming ASR bidirectional | [Streaming ASR](docs/streaming_asr.md) | `client.ASRV2.OpenStreamSession` | Streams audio-only frames, exposes the complete typed BigASR request configuration, and parses full-server JSON results. |
| ⚪ | Streaming ASR input-only result | [Streaming ASR](docs/streaming_asr.md) | none | Needs endpoint selection, final-packet behavior, and typed nostream options. |
| ⚪ | Streaming ASR optimized bidirectional | [Streaming ASR](docs/streaming_asr.md) | none | Needs endpoint selection and typed async/nonstream option coverage. |
| 🟡 | TTS WebSocket bidirectional | [TTS v2](docs/tts_v2.md) | `client.TTSV2.OpenStreamSession` | Lifecycle, text request, cancel, reuse, and audio chunks are implemented. Current config types cover `speaker`, `format`, `sample_rate`, and `resource_id`; advanced 2.0 fields still need typed options. |
| ⚪ | TTS WebSocket unidirectional stream | [TTS v1](docs/tts_v1.md) | none | Listed in upstream TTS API inventory. |
| 🟢 | TTS HTTP Chunked stream | [TTS v1](docs/tts_v1.md) | `client.TTSV2.Stream` | Streams HTTP response frames and decodes audio chunks. |
| ⚪ | TTS HTTP SSE stream | [TTS v1](docs/tts_v1.md) | none | Listed in upstream TTS API inventory. |
| 🟢 | Realtime speech 1.0 | [Realtime speech](docs/realtime_speech.md) | `client.Realtime.OpenSession` | Core session, typed StartSession provider fields, text/audio modes, UpdateConfig, RAG text, conversation CRUD/truncate/delete, push-to-talk, TTS text injection, parsed events, and local context helpers are implemented. |
| 🟢 | Realtime full-duplex 3.0 | [Realtime duplex](docs/realtime_duplex.md) | `client.RealtimeDuplex.OpenSession` | JSON event flow, typed `extension.asr`/`extension.tts`/`extension.dialog`, audio append/commit, replacement text, context CRUD, response cancel, function-call result return, and parsed events are implemented. |
| 🟢 | AST simultaneous interpretation | [Simultaneous interpretation](docs/simultaneous_interpretation.md) | `client.ASTTranslate.OpenSession` | Supports S2T/S2S start, audio upload, config update, finish, parsed subtitle/TTS/usage/muted events, and protobuf transport. |
| 🟢 | Audio generation | [Audio generation](docs/audio_generation.md) | `client.AudioGeneration.Create` | Supports text prompts, audio/image/speaker references, audio config, watermark config, decoded audio, and temporary URL response fields. |
| 🟢 | Podcast generation | [Podcast generation](docs/podcast.md) | `client.Podcast.OpenSession` | Streams typed round/audio/completion events and accepts `retry_info` for durable per-round resume. |
| 🟢 | Voice clone v1 workflow | [Voice clone](docs/voice_clone.md) | `client.VoiceClone.Upload`, `GetStatus`, `Activate` | Supports training upload, status polling, and activation. |
| ⚪ | Voice clone v3 | [Voice clone](docs/voice_clone.md) | none | Needs a typed client for `speaker_id`, `custom_speaker_id`, `audio`, `extra_params`, `speaker_status`, `model_type`, and demo-audio response fields. |

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
- `docs/podcast.md`

## License

This project is licensed under the MIT License. See [LICENSE](./LICENSE).
