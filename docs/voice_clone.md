# Voice Clone

Official documentation: <https://www.volcengine.com/docs/6561/2534906>

Voice clone uploads speech samples and trains a custom voice for speech
synthesis.

## Current Official Endpoint

```text
POST https://openspeech.bytedance.com/api/v3/tts/voice_clone
```

## SDK Coverage

The current SDK implements the older voice-clone workflow, not the newer
`/api/v3/tts/voice_clone` endpoint.

Implemented methods:

- `VoiceClone.Upload`
- `VoiceClone.Submit`
- `VoiceClone.GetStatus`
- `VoiceClone.Activate`
- task wait helpers through `Task[VoiceCloneStatus]`

Implemented endpoints:

| Operation | Endpoint |
| --- | --- |
| Upload training audio | `POST /api/v1/mega_tts/audio/upload` |
| Poll status | `POST /api/v1/mega_tts/status` |
| Activate voice | `POST /api/v1/mega_tts/audio/activate` |

The official v3 endpoint fields below are documented as the target API surface.
Add typed SDK support before calling that endpoint; do not add a generic
passthrough map.

## Official V3 Request Headers

| Header | Required | Meaning |
| --- | --- | --- |
| `Content-Type` | yes | Fixed value `application/json`. |
| `X-Api-Key` | yes | API key authentication value. |
| `X-Api-Request-Id` | yes | Client request ID, usually a UUID. |

The upstream response can include:

| Header | Meaning |
| --- | --- |
| `X-Tt-Logid` | Server log ID for support/debugging. |

## Official V3 Request Body

Top-level fields:

| Field | Type | Required | Current SDK equivalent | Notes |
| --- | --- | --- | --- | --- |
| `speaker_id` | string | yes | `VoiceCloneRequest.SpeakerID` / `VoiceID` in older workflow | Unique voice ID. |
| `custom_speaker_id` | string | no | not implemented | Custom voice ID for postpaid voices. |
| `audio` | object | yes | `Audio`, `AudioFormat`, `AudioFileName` in older workflow | Training audio. |
| `text` | string | no | `Text` | Reference text read by the speaker. If the WER gap is too large, training can fail with `45001109 WERError`. |
| `language` | int | no | `Language` | Audio language enum. |
| `extra_params` | object | no | not implemented | Demo text and preprocessing options. |

### Custom Speaker ID

When using a custom postpaid voice ID, upstream requires:

```json
{
  "speaker_id": "custom_speaker_id",
  "custom_speaker_id": "custom_zh_xxx"
}
```

`custom_speaker_id` naming rules:

- Length: 8 to 256 characters.
- Allowed characters: digits, uppercase/lowercase letters, hyphen, underscore.
- Must start with an English letter.
- First and last character cannot be `-` or `_`.
- Must be unique under the same account ID.
- Must not conflict with official premium voice names or official prefixes and
  suffixes.

Upstream conflict/format regex:

```go
`^((?i:S_|ICL_|MIX_|DiT_|BV)|[a-z]{2}_|(?i:(wvae|moon|mercury|venus|earth|mars|jupiter|saturn|uranus|neptune|pluto|umm)_)).*|.*_(?i:bigtts|bigtts_cc|tob|cs_tob|streaming)$|^[^a-zA-Z]|.*[-_]$|^.{0,7}$|^.{257,}$|.*[^a-zA-Z0-9_-].*`
```

Upstream billing warning: the first formal synthesis call is treated as
activation and charges the voice slot fee. Confirm audition quality before
formal synthesis.

### Audio Object

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `data` | string | yes | Base64-encoded binary audio bytes. |
| `format` | string | yes for `pcm` and `m4a`; optional for other listed formats | Audio format. |

Audio constraints:

- Supported formats: `wav`, `mp3`, `ogg`, `m4a`, `aac`, `pcm`.
- `pcm` must be 24 kHz mono.
- Maximum uploaded file size: 10 MB.

### Language

General voice-clone language enum:

| Value | Language |
| ---: | --- |
| `0` | Chinese, default |
| `1` | English |
| `2` | Japanese |
| `3` | Spanish |
| `4` | Indonesian |
| `5` | Portuguese |
| `8` | Korean |

End-to-end realtime speech model voice-clone language enum:

| Value | Language |
| ---: | --- |
| `0` | Chinese, default |
| `1` | English |

Audio content must match the selected language.

### Extra Params

| Field | Type | Default | Notes |
| --- | --- | --- | --- |
| `demo_text` | string | empty | Audition text, length 4 to 300 characters. If `language` is specified, text must use that language or synthesis can fail. Longer demo text increases registration time. |
| `enable_audio_denoise` | bool | `false` | Enables denoise. Recommended for noisy samples; can affect voice details. |
| `disable_volume_normalization` | bool | `false` | Disables volume normalization. Disabling can make generated volume closer to prompt audio and improve prompt similarity. |

## Official V3 Response

| Field | Type | Notes |
| --- | --- | --- |
| `code` | int | Request status code. Failed HTTP requests use non-200 HTTP status. |
| `message` | string | Status or failure details. |
| `available_training_times` | int | Remaining training attempts for the `speaker_id`. |
| `create_time` | int | Creation time. |
| `language` | int | Language enum. |
| `speaker_id` | string | Unique voice ID. |
| `status` | int | Training status. Status `2` or `4` can be used by TTS synthesis. |
| `speaker_status` | object list | Model-specific speaker status details. |
| `speaker_status[].model_type` | int | Voice clone 2.0 uses `model_type=5` in this response. |
| `demo_audio` | string | Audition audio URL. Returned in `Success`; valid for one hour. Download if needed. |

Response language enum:

| Value | Language |
| ---: | --- |
| `0` | Chinese, default |
| `1` | English |
| `2` | Japanese |
| `3` | Spanish |
| `4` | Indonesian |
| `5` | Portuguese |
| `6` | German |
| `7` | French |
| `8` | Korean |

Training status enum:

| Value | Status | Meaning |
| ---: | --- | --- |
| `0` | `NotFound` | Voice not found. |
| `1` | `Training` | Training in progress. |
| `2` | `Success` | Training succeeded and can be used for TTS. |
| `3` | `Failed` | Training failed. |
| `4` | `Active` | Voice is active and can be used for TTS. |

## Current SDK Older Workflow

Current `VoiceCloneRequest` fields:

| SDK field | Older endpoint field | Notes |
| --- | --- | --- |
| `VoiceID` | `speaker_id` | Custom voice identifier. |
| `SpeakerID` | `speaker_id` | Alias of `VoiceID`. |
| `Audio` | `audios[].audio_bytes` | Raw audio bytes; SDK base64-encodes them. |
| `AudioFormat` | `audios[].audio_format` | Optional; inferred from `AudioFileName` for `wav`, `mp3`, `ogg`, `m4a`, `aac`, `pcm`. |
| `AudioFileName` | local inference only | Used to infer audio format when `AudioFormat` is empty. |
| `Text` | `audios[].text` | Reference text. |
| `Language` | `language` | Language enum. |
| `ModelType` | `model_type` | `4` selects ICL 2.0 default resource in current SDK. |
| `Source` | `source` | Defaults to `2`. |
| `ResourceID` | `Resource-Id` header | Defaults to `seed-icl-1.0`, or `seed-icl-2.0` when `ModelType=4`. |
| `PollInterval` | local task polling | Poll interval for status wait helpers. |

Current `VoiceCloneStatus` fields:

| SDK field | Meaning |
| --- | --- |
| `TaskID` | Upload/status task ID if returned. |
| `SpeakerID` / `VoiceID` | Voice identifier. |
| `Status` | Normalized task status. |
| `RawStatus` / `RawStatusCode` | Original service status. |
| `StatusCode` / `StatusMessage` | Failure details when present. |
| `Version` | Voice/model version when returned. |
| `DemoAudio` | Demo audio URL when returned. |
| `CreateTime` | Creation time. |
| `ReqID`, `TraceID`, `LogID` | Diagnostic metadata. |

Current SDK example:

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/voice_clone -speaker-id <speaker_id> -audio /path/to/sample.wav
```
