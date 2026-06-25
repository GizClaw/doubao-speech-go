# Voice Clone

Official documentation: <https://www.volcengine.com/docs/6561/2534906>

This page tracks the current VolcEngine voice-clone training API against the SDK
surface. The official page uses one training endpoint and identifies 2.0 model
status through fields such as `model_type = 5`.

## Current Official Endpoint

```text
POST https://openspeech.bytedance.com/api/v3/tts/voice_clone
```

This endpoint is documented but not implemented by the SDK yet.

## Current SDK Coverage

The existing `VoiceClone` client implements an older workflow:

- `POST /api/v1/mega_tts/audio/upload`
- status polling
- voice activation

Public methods include:

- `VoiceClone.Upload`
- `VoiceClone.GetStatus`
- `VoiceClone.Activate`
- upload-and-wait helpers

## Request Shape To Implement

The newer official endpoint accepts:

- `speaker_id`
- optional `custom_speaker_id`
- `audio.data` as base64-encoded audio bytes
- `audio.format`
- optional reference `text`
- `language`
- `extra_params.demo_text`
- `extra_params.enable_audio_denoise`
- `extra_params.disable_volume_normalization`

Audio formats listed by the official page:

- `wav`
- `mp3`
- `ogg`
- `m4a`
- `aac`
- `pcm`

For `pcm`, the official page requires 24 kHz mono audio.

## Response Shape To Implement

The newer response includes:

- `code`
- `message`
- `available_training_times`
- `create_time`
- `language`
- `speaker_id`
- `status`
- `speaker_status`
- `model_type`
- `demo_audio`

Training states `Success = 2` and `Active = 4` are usable for TTS synthesis.

## Example

Current older workflow:

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_TOKEN=<your_token> \
go run ./examples/voice_clone -speaker-id <speaker_id> -audio /path/to/sample.wav
```
