# Realtime example

This example opens one numeric-event Realtime session and exercises text or
16 kHz mono PCM input. The initial system/persona instruction is supplied with
`-instructions`; the SDK maps it to `dialog.system_role` for O20 and
`dialog.character_manifest` for SC20.

`-model` and `-speaker` are required. Speaker compatibility changes outside
the SDK, so choose a voice that is enabled for the selected model and account.
The example does not infer one voice that works across both model families.

## Credentials

- `DOUBAO_APP_ID` or `DOUBAO_REALTIME_APP_ID`
- `DOUBAO_API_KEY` or `DOUBAO_REALTIME_API_KEY`
- `DOUBAO_VOLC_WEBSEARCH_API_KEY` (optional)
- `DOUBAO_REALTIME_MODEL` (or pass `-model`)
- `DOUBAO_REALTIME_SPEAKER` (or pass `-speaker`)

The resource ID is a WebSocket handshake header. Configure it with
`DOUBAO_REALTIME_RESOURCE_ID` / `WithResourceID`; it is not session JSON.

## Key flags

- `-model`: `1.2.1.1` (O20) or `2.2.0.0` (SC20); documented aliases are normalized
- `-speaker`: model/account-compatible opaque voice ID
- `-instructions`: canonical initial system/persona instruction
- `-expect-response`: optional substring assertion for credential-backed smoke
- `-mode`: `realtime`, `keep_alive`, `push_to_talk`, `text`, or `audio_file`
- `-pcm`: 16 kHz mono signed-int16 little-endian input file

TTS output is configured independently: the SDK default is 24 kHz mono
`pcm_s16le`. Do not treat the 16 kHz ASR input file as the TTS output contract.

## Local smoke

Load credentials without printing them, then provide separate model-compatible
speakers:

```bash
set -a
source .env
set +a

go run ./examples/realtime \
  -mode push_to_talk \
  -model 1.2.1.1 \
  -speaker "$DOUBAO_REALTIME_O20_SPEAKER" \
  -instructions '只回答：收到。' \
  -expect-response '收到' \
  -pcm examples/asr_v2_sauc_ws/sample_zh_16k.pcm

go run ./examples/realtime \
  -mode push_to_talk \
  -model 2.2.0.0 \
  -speaker "$DOUBAO_REALTIME_SC20_SPEAKER" \
  -instructions '只回答：收到。' \
  -expect-response '收到' \
  -pcm examples/asr_v2_sauc_ws/sample_zh_16k.pcm
```

Other useful paths:

```bash
go run ./examples/realtime -mode text -model 1.2.1.1 -speaker "$DOUBAO_REALTIME_O20_SPEAKER"
go run ./examples/realtime -mode push_to_talk -model 1.2.1.1 -speaker "$DOUBAO_REALTIME_O20_SPEAKER" -interrupt
go run ./examples/realtime -mode push_to_talk -model 1.2.1.1 -speaker "$DOUBAO_REALTIME_O20_SPEAKER" -tts-text "SDK TTS smoke"
```

`RealtimeConfig.Prompt`, `Props`, `History`, and their local update helpers are
retained compatibility extensions. The current official event `100`/`501`
examples do not establish them as the audio-dialogue System Prompt contract;
use `Instructions` for that purpose.

## Official sources

- [Realtime API](https://www.volcengine.com/docs/6561/1594356)
- [Product updates](https://www.volcengine.com/docs/6561/162929)
- [Voice list](https://www.volcengine.com/docs/6561/1257544)
