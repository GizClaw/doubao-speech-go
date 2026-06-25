# Audio Generation Example

This example calls the non-streaming Audio Generation HTTP API.

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/audio_generation \
  -prompt "Generate a short cinematic notification sound." \
  -format wav \
  -output audio_generation_output.wav
```

Useful flags:

- `-model`: model ID, default `seed-audio-1.0`
- `-prompt`: required prompt or text content
- `-speaker`: optional speaker or cloned voice ID reference
- `-audio-url`: optional reference audio URL
- `-image-url`: optional reference image URL
- `-format`: output audio format, default `wav`
- `-sample-rate`: output sample rate, default `24000`
- `-output`: output file path
- `-timeout-sec`: request timeout

Do not pass speaker/audio and image references in the same request.
