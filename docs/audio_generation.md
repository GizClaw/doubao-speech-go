# Audio Generation

Official documentation: <https://www.volcengine.com/docs/6561/2550782>

This page tracks the VolcEngine Audio Generation 1.0 HTTP API.

The official document was checked on 2026-06-26.

## Endpoint

```text
POST https://openspeech.bytedance.com/api/v3/tts/create
```

## SDK Status

Implemented by `client.AudioGeneration.Create`.

This API is distinct from the implemented TTS HTTP Chunked and TTS WebSocket
streaming APIs.

## Authentication

Use the API-key-only client path:

```go
client := doubaospeech.NewClient(appID, doubaospeech.WithAPIKey(apiKey))
```

The SDK sends:

- `X-Api-Key`
- `X-Api-App-Id`
- `X-Api-Request-Id`

`X-Api-Request-Id` is generated when `AudioGenerationCreateRequest.RequestID` is
empty.

## Product Scope

The official API is a non-streaming HTTP audio-generation endpoint. It supports
generation from natural-language prompts and references such as audio or images.
Use cases include:

- audiobook audio
- dubbing
- sound effects
- game audio
- voice design

The current maximum output duration is 120 seconds.

## Request

```go
resp, err := client.AudioGeneration.Create(ctx, &doubaospeech.AudioGenerationCreateRequest{
    Model:      doubaospeech.ModelSeedAudio10,
    TextPrompt: "Generate a short cinematic notification sound.",
    AudioConfig: &doubaospeech.AudioGenerationAudioConfig{
        Format:     doubaospeech.FormatWAV,
        SampleRate: doubaospeech.SampleRate24000,
    },
})
```

### Top-Level Fields

- `model` is required. Current value: `seed-audio-1.0`.
- `text_prompt` is required and supports up to 2048 characters.
- `references` is optional.
- `audio_config` is optional.
- `watermark` is optional.

### Reference Inputs

`references` can be omitted for pure text-prompt generation.

For audio references, each item must contain exactly one of:

- `speaker`
- `audio_data`
- `audio_url`

For image references, each item must contain exactly one of:

- `image_data`
- `image_url`

Limits from the official docs:

- At most 3 audio references.
- Each audio reference can be at most 30 seconds and 10 MB.
- Supported audio reference formats: `wav`, `mp3`, `pcm`, `ogg_opus`.
- At most 1 image reference.
- Supported image reference formats: `jpeg`, `png`, `webp`.
- Audio and image references cannot be mixed in one request.

When using audio references, `text_prompt` can refer to them as `@音频1`,
`@音频2`, and `@音频3` in upload order.

### Audio Config

- `format`: `wav`, `mp3`, `pcm`, or `ogg_opus`; default is `wav`.
- `sample_rate`: `8000`, `16000`, `24000`, `32000`, `44100`, or `48000`; default is `24000`.
- `speech_rate`: `[-50, 100]`; `100` is 2.0x and `-50` is 0.5x.
- `loudness_rate`: `[-50, 100]`; `100` is 2.0x and `-50` is 0.5x.
- `pitch_rate`: `[-12, 12]`.

### Watermark

`watermark.aigc_watermark` enables an explicit rhythm marker at the end of the
generated audio.

`watermark.aigc_metadata` embeds metadata in the generated audio header.

## Response

`AudioGenerationCreateResponse` exposes:

- `Code`
- `Message`
- `AudioBase64`
- `Audio`
- `Duration`
- `OriginalDuration`
- `URL`
- `ReqID`
- `TraceID`
- `LogID`
- `Extra`

`Audio` is decoded from the official `audio` base64 response field. `URL` is a
temporary audio URL that is valid for 2 hours according to the official docs.

## Example

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/audio_generation \
  -prompt "Generate a short cinematic notification sound." \
  -format wav \
  -output audio_generation_output.wav
```
