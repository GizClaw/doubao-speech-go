# Audio Generation

Official documentation: <https://www.volcengine.com/docs/6561/2550782>

This page tracks the VolcEngine Audio Generation 1.0 HTTP API. It is currently a
planned SDK feature.

## Endpoint

```text
POST https://openspeech.bytedance.com/api/v3/tts/create
```

## SDK Status

Not implemented yet.

This API is distinct from the implemented TTS HTTP Chunked and TTS WebSocket
streaming APIs.

## Product Scope

The official API is a non-streaming HTTP audio-generation endpoint. It supports
generation from natural-language prompts and references such as audio or images.
Use cases include:

- audiobook audio
- dubbing
- sound effects
- game audio
- voice design

## Request Areas To Model

The SDK should eventually model:

- prompt text
- speaker selection
- reference audio inputs
- reference image inputs
- generated audio format
- task or response status
- returned audio payload or URL

## Roadmap Note

Because this endpoint is not implemented, it should remain in the README
roadmap under planned work until a typed `AudioGeneration` client is added.
