# TTS 2.0

Official documentation: <https://www.volcengine.com/docs/6561/2532486>

This page tracks the VolcEngine TTS 2.0 bidirectional WebSocket API against the
current SDK surface.

## Endpoint

```text
wss://openspeech.bytedance.com/api/v3/tts/bidirection
```

## SDK Coverage

The core bidirectional WebSocket flow is implemented by `TTSV2.OpenStreamSession`.

Supported operations:

- connect with API-key or legacy credentials
- `StartConnection`
- `StartSession`
- text `TaskRequest`
- streamed audio frames
- session finish
- session cancel
- sequential session reuse

## Resource IDs

| Resource ID | Meaning |
| --- | --- |
| `seed-tts-2.0` | TTS 2.0 |
| `seed-icl-2.0` | voice clone 2.0 |

## Audio Output

The official API supports:

- `mp3`
- `ogg_opus`
- `pcm`

For streaming use cases, prefer `pcm` or `ogg_opus`. Avoid `wav` in streaming
flows because repeated WAV headers can appear across returned chunks.

## 2.0-Specific Fields To Track

The official API exposes several 2.0-oriented fields that should be typed in the
SDK over time:

- `context_texts`
- `section_id`
- `use_tag_parser`
- `aigc_metadata`
- `explicit_language`
- `explicit_dialect`
- `cache_config`
- `enable_subtitle`

## Example

```bash
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/tts_v2/websocket
```
