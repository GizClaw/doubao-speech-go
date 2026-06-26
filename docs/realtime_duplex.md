# Realtime Duplex

Official documentation: <https://www.volcengine.com/docs/6561/2549778>

This page maps the VolcEngine full-duplex realtime speech API to the current SDK
surface. The upstream product is Realtime Speech Model 3.0 full-duplex
end-to-end speech, also called Seeduplex.

## Endpoint

```text
wss://openspeech.bytedance.com/api/v3/duplex/realtime/dialogue
```

The required upstream model value is:

```text
1.2.6.0
```

## API Difference

| SDK API | Endpoint | Wire format | Event identity |
| --- | --- | --- | --- |
| `Client.Realtime` | `/api/v3/realtime/dialogue` | ByteDance binary frames | numeric event IDs |
| `Client.RealtimeDuplex` | `/api/v3/duplex/realtime/dialogue` | WebSocket text JSON | string event types |

## SDK Coverage

Implemented by `RealtimeDuplex.OpenSession`.

The SDK supports:

- `session.create`
- `session.update`
- `session.close`
- `input_audio_buffer.append`
- `input_audio_buffer.commit`
- `speech_text_buffer.commit`
- `speech_text_buffer.replacement.append`
- `speech_text_buffer.replacement.commit`
- `response.cancel`
- conversation item create, update, retrieve, and delete
- function-call result submission through `conversation.item.create`
- parsed session, ASR, text, audio, context, function-call, usage, and error events

## Audio Format

Input audio is base64-encoded inside JSON text frames.

Typical upstream requirements:

- input sample rate: 16 kHz
- input formats: `pcm` or `speech_opus`
- output sample rate: 24 kHz
- output formats: `pcm_s16le` or `ogg_opus`

## Extension Fields

`RealtimeDuplexExtension` is intentionally typed. The SDK only exposes extension
fields that it recognizes and treats as supported API surface; unknown provider
keys are not accepted through public structs.

The default recommendation for downstream schemas is to omit `extension` unless
one of the fields below is required. Core Duplex concepts should use typed
`session` fields instead: model and instructions live under `session.model` and
`session.instructions`, audio codecs and voice live under `session.audio`, and
function calling lives under `session.tools`.

Supported extension fields:

| JSON path | Go field | Type | Default | Notes |
| --- | --- | --- | --- | --- |
| `extension.dialog.extra.audit_response` | `RealtimeDuplexDialogExtra.AuditResponse` | `string` | service default | Response text used when dialogue audit blocks an answer. |
| `extension.dialog.extra.enable_loudness_norm` | `RealtimeDuplexDialogExtra.EnableLoudnessNorm` | `*bool` | service default | Enables dialogue output loudness normalization when set. |
| `extension.dialog.extra.enable_music` | `RealtimeDuplexDialogExtra.EnableMusic` | `*bool` | service default | Enables or disables generated background music when set. Use a pointer so explicit `false` is serialized. |

No stable SDK fields are currently exposed under `extension.asr`,
`extension.tts`, `extension.s2s`, or top-level `extension.extra`. If upstream
documents stable fields for those sections, add typed structs and tests before
exposing them.

Realtime 1.0 extras are not automatically portable to Duplex. Do not carry
forward Realtime 1.0 VAD window settings, web search flags, bot name, speaking
style, or character manifest into `RealtimeDuplexExtension`; they have no typed
Duplex extension field in this SDK. Use Duplex-native `session.instructions`,
`session.audio`, and `session.tools` instead.

## Example

The example demonstrates a complete local loop: generate an old realtime prompt,
send it to the duplex API, handle function-call events, and transcribe the
duplex audio output for logging.

```bash
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/realtime_duplex
```
