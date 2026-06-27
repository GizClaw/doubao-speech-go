# Realtime multi-turn example

This example demonstrates one **single realtime session** with multiple turns:

1. Open session with initial prompt + generation props
2. Send round-1 user message and receive streaming events until final
3. Update history before round-2
4. Update prompt before round-2
5. Update generation props before round-2
6. Send round-2 user message and receive events until final
7. Configure typed ASR/TTS/dialog extras, including optional web search
8. Trigger one `ClientInterrupt` request
9. Close session twice to verify idempotent close

### API coverage in this example

Covered directly in `main.go`:

- `OpenSession`
- `SendUserMessage`
- `SendText` (alias)
- `RecvEvent`
- `Recv` (iterator form)
- `UpdateHistory`
- `ReplaceHistory`
- `UpdatePrompt`
- `UpdateProps`
- `RealtimeASRExtra`
- `RealtimeDialogExtra`
- `Interrupt` (`ClientInterrupt`, event `515`)
- `Close` (idempotent)

Not covered in this single example (recommended scenarios):

- `Dial` + `StartSession`: when you need explicit connection lifecycle control
- `Connect`: when you prefer one-shot connect+session API (equivalent semantics to `OpenSession`)
- `SendAudio` + `EndASR`: microphone/PCM and push-to-talk scenarios
- `SayHello`: greeting bootstrap flow before user turn
- `SendTTSText`: server-side TTS text streaming scenarios

## Requirements

- `DOUBAO_APP_ID` or `DOUBAO_REALTIME_APP_ID`
- `DOUBAO_API_KEY` or `DOUBAO_REALTIME_API_KEY`
- `DOUBAO_VOLC_WEBSEARCH_API_KEY` (optional, enables typed `dialog.extra.enable_volc_websearch`)

## Run

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_API_KEY=<your_api_key> \
DOUBAO_VOLC_WEBSEARCH_API_KEY=<your_search_api_key> \
go run ./examples/realtime -mode text -model 1.2.1.1
```

## Key flags

- `-speaker`: TTS speaker/voice ID (default: `zh_female_cancan`)
- `-mode`: input mode: `realtime`, `keep_alive`, `push_to_talk`, `text`, or `audio_file` (default: `text`)
- `-model`: realtime model version, for example `1.2.1.1` or `2.2.0.0`; legacy aliases such as `O` and `SC` are normalized
- `-round1`: first user message
- `-round2`: second user message

## API-Key-Backed E2E Smoke

Use this example with a local App ID config value and API key for live Realtime
smoke checks:

```bash
set -a
source .env
set +a

go run ./examples/realtime -mode text -round1 "Reply with exactly: pong" -round2 "Reply with exactly: done"
go run ./examples/realtime -mode realtime -pcm examples/asr_v2_sauc_ws/sample_zh_16k.pcm
go run ./examples/realtime -mode push_to_talk -pcm examples/asr_v2_sauc_ws/sample_zh_16k.pcm
go run ./examples/realtime -mode push_to_talk -pcm examples/asr_v2_sauc_ws/sample_zh_16k.pcm -interrupt
go run ./examples/realtime -mode push_to_talk -pcm examples/asr_v2_sauc_ws/sample_zh_16k.pcm -tts-text "This is an SDK text synthesis smoke test."
```

## Voice list (realtime-compatible references)

> Table update date: **2026-03-02**

| voice_id | Language | Gender / style | Remark |
|---|---|---|---|
| `zh_female_cancan` | Chinese (Mandarin) | Female / standard | Default in this example, commonly used in VolcEngine realtime samples |
| `BV700_streaming` | Chinese (Mandarin) | Female / standard (Cancan) | BytePlus Speech "Supported voice and languages" |
| `BV701_streaming` | Chinese (Mandarin) | Male / expressive (Qingcang) | Supports multi-emotion in official docs |
| `BV138_streaming` | English (US) | Female / expressive (Lawrence) | Dialog expressive voice in official docs |
| `BV027_streaming` | English (US) | Female / formal (Amelia) | General English voice |
| `BV520_streaming` | Japanese | Female / outgoing (Himari) | Japanese voice option |

## Official sources

- Realtime API entry (VolcEngine):
  - https://www.volcengine.com/docs/6561/1594356
- Realtime/TTS voice list references:
  - https://docs.byteplus.com/en/docs/speech/docs-voice-parameters-1
  - https://www.volcengine.com/docs/6561/1257544

## Default speaker note

`main.go` uses `zh_female_cancan` by default (`-speaker` flag).
If your account is configured with BytePlus/BV voice IDs, pass a BV ID explicitly, for example:

```bash
go run ./examples/realtime -speaker BV700_streaming
```
