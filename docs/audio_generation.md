# Audio Generation

Official documentation: <https://www.volcengine.com/docs/6561/2550782>

This page maps the VolcEngine non-streaming HTTP audio generation API to the
current SDK surface. The API can generate audio from natural-language prompts,
multiple reference audio clips, or one reference image. Typical use cases
include audiobooks, dubbing, games, sound effects, and voice design.

The current upstream maximum output duration is 120 seconds.

## Endpoint

```text
POST https://openspeech.bytedance.com/api/v3/tts/create
```

## SDK Coverage

Implemented by `client.AudioGeneration.Create`.

This API is distinct from the TTS HTTP streaming and TTS WebSocket bidirectional
streaming APIs.

The SDK exposes typed request structs:

- `AudioGenerationCreateRequest`
- `AudioGenerationReference`
- `AudioGenerationAudioConfig`
- `AudioGenerationWatermark`
- `AudioGenerationAIGCMeta`

The SDK validates the documented model value, `text_prompt` length, reference
mutual-exclusion rules, reference counts, output audio format/sample rate, and
rate/pitch ranges before sending the request.

## Authentication

API-key authentication:

```http
X-Api-Key: <api-key>
```

The SDK sends:

```http
Content-Type: application/json
X-Api-Key: <api-key>
X-Api-App-Id: <app-id, when configured>
X-Api-Request-Id: <request-id>
```

`X-Api-App-Id` is request/application configuration, not an authentication
factor.

`X-Api-Request-Id` is the client request trace ID. Set
`AudioGenerationCreateRequest.RequestID` to correlate with an internal trace ID,
or leave it empty and the SDK will generate one.

The response header can include:

| Header | Meaning |
| --- | --- |
| `X-Tt-Logid` | Server log ID. Include it when consulting or reporting issues. |

## Request Body

Top-level fields:

| Field | Type | Required | SDK field | Notes |
| --- | --- | --- | --- | --- |
| `model` | string | yes | `Model` | Current supported value: `seed-audio-1.0`. SDK constant: `ModelSeedAudio10`. |
| `text_prompt` | string | yes | `TextPrompt` | Prompt or text content used to synthesize audio. Maximum 2048 characters. |
| `references` | array | no | `References` | Reference resources. Omit for pure text generation. |
| `audio_config` | object | no | `AudioConfig` | Output audio configuration. |
| `watermark` | object | no | `Watermark` | Explicit and metadata watermark configuration. Omit to disable watermarking. |

### Text Prompt Modes

`text_prompt` behavior depends on `references`:

| Mode | References | `text_prompt` behavior |
| --- | --- | --- |
| Pure text generation | No references | Prompt describes the target audio to generate. |
| Reference-audio generation | Audio references present | May reference uploaded audio by `@音频N`; numbering starts at 1 in upload order. |
| Reference-image generation | Image reference present | Can contain only the text to synthesize. |

Example reference-audio prompt:

```text
Use @音频1 as the narrator style, then read: welcome to the demo.
```

## References

`references` is optional. Each item is either an audio reference or an image
reference. Audio and image references cannot be mixed in one request.

Audio reference fields:

| Field | Type | Required | SDK field | Notes |
| --- | --- | --- | --- | --- |
| `speaker` | string | one of `speaker`, `audio_data`, `audio_url` | `Speaker` | Voice ID. Supports Doubao TTS 2.0 voices or clone voices. |
| `audio_data` | string | one of `speaker`, `audio_data`, `audio_url` | `AudioData` | Base64-encoded reference audio. |
| `audio_url` | string | one of `speaker`, `audio_data`, `audio_url` | `AudioURL` | Remote reference audio URL. |

Audio reference limits:

- At most 3 audio references.
- Each reference audio clip can be at most 30 seconds.
- Each reference audio clip must be at most 10 MB.
- Supported formats: `wav`, `mp3`, `pcm`, `ogg_opus`.
- Reference order maps to `@音频1`, `@音频2`, and `@音频3` in `text_prompt`.

Image reference fields:

| Field | Type | Required | SDK field | Notes |
| --- | --- | --- | --- | --- |
| `image_data` | string | one of `image_data`, `image_url` | `ImageData` | Base64-encoded reference image. |
| `image_url` | string | one of `image_data`, `image_url` | `ImageURL` | Remote reference image URL. |

Image reference limits:

- At most 1 image reference.
- Image size must be at most 10 MB.
- Supported formats: `jpeg`, `png`, `webp`.
- Image references cannot be used together with `speaker`, `audio_data`, or
  `audio_url`.

## Audio Config

`audio_config` controls generated output audio.

| Field | Type | Required | SDK field | Default | Notes |
| --- | --- | --- | --- | --- | --- |
| `format` | string | no | `Format` | `wav` | `wav`, `mp3`, `pcm`, or `ogg_opus`. |
| `sample_rate` | int | no | `SampleRate` | `24000` | `8000`, `16000`, `24000`, `32000`, `44100`, or `48000`. |
| `speech_rate` | int | no | `SpeechRate` | `0` | Range `[-50,100]`; `100` is 2x, `-50` is 0.5x. |
| `loudness_rate` | int | no | `LoudnessRate` | `0` | Range `[-50,100]`; `100` is 2x volume, `-50` is 0.5x. |
| `pitch_rate` | int | no | `PitchRate` | `0` | Range `[-12,12]`. |

## Watermark

`watermark` is optional. If omitted, no watermark is added by default.

| Field | Type | Required | SDK field | Default | Notes |
| --- | --- | --- | --- | --- | --- |
| `aigc_watermark` | bool | no | `AIGCWatermark` | `false` | Explicit watermark switch; adds an audio rhythm marker at the end. |
| `aigc_metadata` | object | no | `AIGCMetadata` | omitted | Hidden metadata watermark in the generated audio header. |

`aigc_metadata` fields:

| Field | Type | SDK field | Default | Notes |
| --- | --- | --- | --- | --- |
| `enable` | bool | `Enable` | `false` | Enables hidden metadata watermark. |
| `content_producer` | string | `ContentProducer` | empty | Name or code of the synthesis service provider. |
| `produce_id` | string | `ProduceID` | empty | Content production ID. |
| `content_propagator` | string | `ContentPropagator` | empty | Name or code of the propagation service provider. |
| `propagate_id` | string | `PropagateID` | empty | Content propagation ID. |

## Request Example

```go
resp, err := client.AudioGeneration.Create(ctx, &doubaospeech.AudioGenerationCreateRequest{
	Model:      doubaospeech.ModelSeedAudio10,
	TextPrompt: "Read this line in a natural and warm narrator voice: hello from the audio generation API.",
	AudioConfig: &doubaospeech.AudioGenerationAudioConfig{
		Format:     doubaospeech.FormatMP3,
		SampleRate: doubaospeech.SampleRate24000,
	},
})
```

Reference-audio example:

```go
resp, err := client.AudioGeneration.Create(ctx, &doubaospeech.AudioGenerationCreateRequest{
	Model:      doubaospeech.ModelSeedAudio10,
	TextPrompt: "Use @音频1 as the speaking style, then read: welcome to the demo.",
	References: []doubaospeech.AudioGenerationReference{
		{AudioURL: "https://example.com/reference.mp3"},
	},
})
```

## Response

Response headers:

| Header | Meaning |
| --- | --- |
| `X-Tt-Logid` | Server log ID for support/debugging. |

Response body fields:

| Field | Type | SDK field | Notes |
| --- | --- | --- | --- |
| `code` | int | `Code` | Status code. See the upstream error-code document for details. |
| `message` | string | `Message` | Status detail. |
| `audio` | string | `AudioBase64`, decoded into `Audio` | Base64-encoded generated audio. |
| `duration` | float | `Duration` | Processed audio duration in seconds. Can differ from `original_duration` after speed or post-processing. |
| `original_duration` | float | `OriginalDuration` | Original model-output duration in seconds. Billing uses this value. Upper limit: 120 seconds. |
| `url` | string | `URL` | Temporary audio URL. Upstream validity: 2 hours. |

`AudioGenerationCreateResponse` also exposes request metadata:

| SDK field | Source |
| --- | --- |
| `ReqID` | Request ID from response body aliases, or generated/supplied `X-Api-Request-Id`. |
| `TraceID` | Response body metadata when present. |
| `LogID` | Response body metadata or `X-Tt-Logid`. |
| `Extra` | Unknown response fields preserved as raw JSON. |

## Example

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/audio_generation \
  -prompt "Read this line in a natural and warm narrator voice: hello from the audio generation API." \
  -format mp3 \
  -output audio_generation_voiceover.mp3
```
