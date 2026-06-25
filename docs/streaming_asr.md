# Streaming ASR

Official documentation: <https://www.volcengine.com/docs/6561/1354869>

This page maps the VolcEngine big-model streaming ASR SAUC WebSocket API to the
current SDK surface.

## Endpoints

| Mode | Endpoint | SDK coverage |
| --- | --- | --- |
| Bidirectional streaming | `wss://openspeech.bytedance.com/api/v3/sauc/bigmodel` | Implemented by `ASRV2.OpenStreamSession` |
| Streaming input / no-stream result | `wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_nostream` | Documented, not exposed as a dedicated high-level SDK mode |
| Optimized bidirectional streaming | `wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_async` | Documented, not exposed as a dedicated high-level SDK mode |

The official page lists resource IDs for both ASR 1.0 and ASR 2.0. They share
the same SAUC WebSocket protocol, so this repository keeps one SDK document
instead of separate 1.0 and 2.0 documents.

## Implemented SDK API

```go
client := doubaospeech.NewClient(appID,
	doubaospeech.WithAPIKey(apiKey),
	doubaospeech.WithResourceID(doubaospeech.ResourceASRV2Duration),
)

session, err := client.ASRV2.OpenStreamSession(ctx, doubaospeech.ASRV2StreamConfig{
	Audio: doubaospeech.ASRV2AudioConfig{
		Format:  "pcm",
		Codec:   "raw",
		Rate:    16000,
		Bits:    16,
		Channel: 1,
	},
	Request: doubaospeech.ASRV2RequestConfig{
		ModelName: "bigmodel",
	},
})
```

The SDK sends the initial full-client request, streams audio-only frames, parses
server result frames, and handles final packet semantics for the implemented
`bigmodel` endpoint.

## Authentication

Recommended API-key headers:

```http
X-Api-Key: <api-key>
X-Api-App-Id: <app-id>
X-Api-Resource-Id: <resource-id>
X-Api-Request-Id: <uuid>
X-Api-Sequence: -1
```

Common resource IDs:

| Product | Billing mode | Resource ID |
| --- | --- | --- |
| Streaming ASR 1.0 | duration | `volc.bigasr.sauc.duration` |
| Streaming ASR 1.0 | concurrent | `volc.bigasr.sauc.concurrent` |
| Streaming ASR 2.0 | duration | `volc.seedasr.sauc.duration` |
| Streaming ASR 2.0 | concurrent | `volc.seedasr.sauc.concurrent` |

## Audio Requirements

- PCM or WAV payloads must contain `pcm_s16le` audio.
- Sample rate must be `16000`.
- Bit depth must be `16`.
- Mono audio is the default and recommended.
- The official recommendation is 100 to 200 ms per packet. For bidirectional
  streaming, 200 ms packets are preferred.

## Options To Track

The upstream API exposes many request options that are not all typed in this SDK
yet, including:

- `bigmodel_nostream` and `bigmodel_async` mode selection
- non-stream second-pass recognition
- speaker clustering
- language detection and auto-language mode
- emotion, gender, speech-rate, and volume detection
- POI and music function-call helpers
- richer contextual prompt payloads, including ASR 2.0 visual context

## Example

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/asr_v2_sauc_ws
```
