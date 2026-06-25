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

## Example

The example demonstrates a complete local loop: generate an old realtime prompt,
send it to the duplex API, handle function-call events, and transcribe the
duplex audio output for logging.

```bash
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/realtime_duplex
```
