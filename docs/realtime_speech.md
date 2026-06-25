# Realtime Speech

Official documentation: <https://www.volcengine.com/docs/6561/1594356>

This page maps the VolcEngine end-to-end realtime speech API to the current SDK
surface. This is the numeric-event, binary-frame realtime API exposed as
`Client.Realtime`.

## Endpoint

```text
wss://openspeech.bytedance.com/api/v3/realtime/dialogue
```

Required upstream headers include:

```http
X-Api-Resource-Id: volc.speech.dialog
X-Api-App-Key: PlgvMymc7f3tQnJ6
```

The SDK provides these constants:

```go
doubaospeech.ResourceRealtime
doubaospeech.AppKeyRealtime
```

## SDK Coverage

Implemented by `Realtime.OpenSession`.

The SDK supports:

- `StartConnection`
- `StartSession`
- streaming audio input through `TaskRequest`
- text input through `ChatTextQuery`
- push-to-talk end signal through `EndASR`
- client interrupt through `ClientInterrupt`
- direct TTS text injection through `ChatTTSText`
- parsed ASR, chat, TTS, usage, error, and session events
- conversation create, update, retrieve, truncate, and delete helpers

## Model Values

The official document currently uses these normalized model values:

| Model family | SDK constant | Upstream value |
| --- | --- | --- |
| O 2.0 | `RealtimeModelO20` | `1.2.1.1` |
| SC 2.0 | `RealtimeModelSC20` | `2.2.0.0` |

The SDK normalizes common older aliases before sending `StartSession`.

## Input Modes

The upstream API supports:

- microphone streaming
- microphone with keep-alive mode
- push-to-talk
- text input
- audio-file streaming

For PCM microphone input, upload mono `pcm_s16le` at 16 kHz. The upstream
recommendation is 20 ms audio packets.

## Output Audio

By default, the server returns OGG Opus audio. The request can ask for PCM output
through the TTS audio configuration:

- `pcm` at 24 kHz, 32-bit
- `pcm_s16le` at 24 kHz, 16-bit

## Example

```bash
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/realtime -mode text -model 1.2.1.1
```
