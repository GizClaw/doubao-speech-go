# Simultaneous Interpretation

Official documentation: <https://www.volcengine.com/docs/6561/1756902>

This page maps the VolcEngine AST simultaneous interpretation WebSocket API to
the current SDK surface. The upstream API supports speech-to-text (`s2t`) and
speech-to-speech (`s2s`) translation over WebSocket with protobuf payloads.

## Endpoint

```text
wss://openspeech.bytedance.com/api/v4/ast/v2/translate
```

## SDK Coverage

Implemented by `ASTTranslate.OpenSession`.

The SDK currently supports:

- API-key authentication headers plus request metadata headers
- `StartSession`
- `TaskRequest` audio upload
- `UpdateConfig`
- `FinishSession`
- protobuf transport
- parsed source subtitle events
- parsed translation subtitle events
- parsed TTS audio events
- usage and muted-audio events
- deterministic receive backpressure handling

The SDK exposes typed request fields through `ASTTranslateConfig`,
`ASTTranslateUpdate`, `ASTAudioConfig`, `ASTTargetAudioConfig`, and
`ASTTranslateCorpus`; it does not require users to construct arbitrary
protobuf maps.

## Authentication

API-key authentication headers:

```http
X-Api-Key: <api-key>
X-Api-Resource-Id: volc.service_type.10053
```

The SDK also sends `X-Api-App-Id` when configured and `X-Api-Connect-Id` for
request/application identification and connection tracing. `X-Api-App-Id` is
not an authentication factor. After a successful WebSocket handshake, the
service may return `X-Tt-Logid`; log it when debugging server-side issues.

## Supported Modes And Languages

High-level upstream support:

| Mode | Language setting | Supported languages | Notes |
| --- | --- | --- | --- |
| `s2s` | Explicit source and target language | `zh`, `en`, `pt`, `es`, `ja`, `id`, `de`, `fr` | Source or target must be `zh` or `en`. |
| `s2s` | Automatic source switching | `zh`, `en` | Automatic language switching only covers Chinese and English. |
| `s2t` | Explicit source and target language | `zh`, `en`, `pt`, `es`, `ja`, `id`, `de`, `fr`, `ru`, `it`, `ko`, `ar`, `tr`, `ms`, `vi`, `th`, `nl`, `ro`, `pl`, `cs` | Source or target must be `zh` or `en`; dialects are only valid as source languages. |

Public `s2s` speakers listed by the upstream document:

- `zh_female_vv_uranus_bigtts`
- `zh_male_jingqiangkanye_emo_mars_bigtts`

Language groups:

| Group | Count | Values |
| --- | --- | --- |
| `lang_8` | 8 | `zh`, `en`, `de`, `fr`, `es`, `id`, `ja`, `pt` |
| `lang_20` | 20 | `zh`, `en`, `de`, `fr`, `es`, `id`, `ja`, `pt`, `ko`, `tr`, `ms`, `nl`, `ro`, `pl`, `cs`, `ar`, `th`, `vi`, `ru`, `it` |
| dialects | 2 | `yue-CN`, `sh-CN` |

Mode constraints:

| Mode | Constraint | Supported languages |
| --- | --- | --- |
| `s2t` | Source and target are required; source or target must be Chinese or English; `zhen` supports Chinese-English mixed reversal. | source: `lang_20` and dialects; target: `lang_20` |
| `s2s` with public speaker | Source and target are required; target must be Chinese or English; `zhen` supports Chinese-English mixed reversal. | source: `lang_20` and dialects; target: `zh` or `en` |
| `s2s` voice clone | Omit `speaker_id`, or pass no supported public speaker; source or target must be Chinese or English; `zhen` supports Chinese-English mixed reversal. | source: `lang_8`; target: `lang_8` |

Language values:

| Language | Value | Notes |
| --- | --- | --- |
| Chinese | `zh` | Chinese/English anchor language. |
| English | `en` | Chinese/English anchor language. |
| German | `de` | |
| French | `fr` | |
| Spanish | `es` | |
| Indonesian | `id` | |
| Japanese | `ja` | |
| Portuguese | `pt` | |
| Korean | `ko` | |
| Turkish | `tr` | |
| Malay | `ms` | |
| Dutch | `nl` | |
| Romanian | `ro` | |
| Polish | `pl` | |
| Czech | `cs` | |
| Arabic | `ar` | |
| Thai | `th` | |
| Vietnamese | `vi` | |
| Russian | `ru` | |
| Italian | `it` | |
| Cantonese | `yue-CN` | Dialect; source language only. |
| Shanghainese | `sh-CN` | Dialect; source language only. |
| Chinese-English mixed reversal | `zhen` | `source_language` and `target_language` must both be `zhen`. |

## Events

Client event IDs:

| Event | ID | Description |
| --- | ---: | --- |
| `StartSession` | `100` | Start a translation session. |
| `TaskRequest` | `200` | Send audio bytes. |
| `UpdateConfig` | `201` | Update corpus/intervention parameters. |
| `FinishSession` | `102` | Finish the session. |

Server event IDs:

| Event | ID | Description |
| --- | ---: | --- |
| `SessionStarted` | `150` | Session started. |
| `UsageResponse` | `154` | Billing and usage response. |
| `AudioMuted` | `250` | VAD detected muted/silent audio. |
| `TTSSentenceStart` | `350` | TTS output starts. |
| `TTSSentenceEnd` | `351` | TTS output ends. |
| `TTSResponse` | `352` | TTS audio bytes. |
| `SourceSubtitleStart` | `650` | Source subtitle segment starts. |
| `SourceSubtitleResponse` | `651` | Source subtitle partial text. |
| `SourceSubtitleEnd` | `652` | Source subtitle segment ends. |
| `TranslationSubtitleStart` | `653` | Translation subtitle segment starts. |
| `TranslationSubtitleResponse` | `654` | Translation subtitle partial text. |
| `TranslationSubtitleEnd` | `655` | Translation subtitle segment ends. |
| `SessionFinished` | `152` | Session finished normally. |
| `SessionFailed` | `153` | Session failed. |

## StartSession Request

The first request after the WebSocket upgrade is `StartSession`. The SDK sends a
protobuf `TranslateRequest`.

| Field | Level | Type | Required | SDK field | Notes |
| --- | ---: | --- | --- | --- | --- |
| `request_meta` | 1 | object | yes | `ASTTranslateConfig.SessionID` | Request metadata. |
| `request_meta.session_id` | 2 | string | yes | `SessionID` | Use a UUID or SDK-generated session ID. |
| `event` | 1 | int32 | yes | internal | `100` for `StartSession`. |
| `user` | 1 | object | no | `ASTTranslateConfig.User` | Helps server-side log filtering. |
| `user.uid` | 2 | string | no | `ASTUser.UID` | SDK falls back to client user ID. |
| `user.did` | 2 | string | no | `ASTUser.DID` | Device identifier. |
| `user.platform` | 2 | string | no | `ASTUser.Platform` | For example `iOS`, `Android`, `Linux`. |
| `user.sdk_version` | 2 | string | no | `ASTUser.SDKVersion` | SDK version string. |
| `request` | 1 | object | yes | `ASTTranslateConfig` | Translation request parameters. |
| `request.mode` | 2 | string | no | `Mode` | `s2t` or `s2s`; defaults to `s2t`. |
| `request.speaker_id` | 2 | string | no | `SpeakerID` | Public output voice; invalid/missing values trigger voice-clone behavior. |
| `request.is_custom_speaker` | 2 | bool | no | `IsCustomSpeaker` | Whether the external speaker is a clone voice; default `false`. |
| `request.tts_resource_id` | 2 | string | conditional | `TTSResourceID` | Required for public/clone TTS voice paths; examples include `seed-tts-1.0`, `seed-tts-2.0`, `seed-icl-1.0`, `seed-icl-2.0`. |
| `request.speech_rate` | 2 | number | no | `SpeechRate` | Range `[-50,100]`; `100` is 2x, `-50` is 0.5x. |
| `request.source_language` | 2 | string | no | `SourceLanguage` | See language constraints above. |
| `request.target_language` | 2 | string | no | `TargetLanguage` | See language constraints above. |
| `request.enable_source_language_detect` | 2 | bool | no | `EnableSourceLanguageDetect` | Adds detected source language to `SourceSubtitleEnd`. |
| `request.corpus` | 2 | object | no | `Corpus` | Hotword, replacement-word, and glossary settings. Total corpus entries must not exceed 1000. |
| `source_audio` | 1 | object | yes | `SourceAudio` | Source audio format. |
| `target_audio` | 1 | object | required for `s2s` | `TargetAudio` | Target audio format. |
| `denoise` | 1 | bool | no | `Denoise` | SDK defaults this to `false`. |

Corpus fields:

| Field | Type | SDK field | Notes |
| --- | --- | --- | --- |
| `hot_words_list` | array of string | `HotWords` | Source subtitle recognition hotwords; higher priority than hotword tables. |
| `boosting_table_id` | string | `BoostingTableID` | Hotword table ID. |
| `boosting_table_name` | string | `BoostingTableName` | Hotword table name. |
| `correct_words` | JSON string map | `CorrectWords` | Replacement-word map for source and translated subtitles. SDK accepts `map[string]string` and serializes it. |
| `regex_correct_table_id` | string | `RegexCorrectTableID` | Replacement-word table ID. |
| `regex_correct_table_name` | string | `RegexCorrectTableName` | Replacement-word table name. |
| `glossary_list` | object map | `Glossary` | Translation glossary map; higher priority than glossary tables. |
| `glossary_table_id` | string | `GlossaryTableID` | Glossary table ID. |
| `glossary_table_name` | string | `GlossaryTableName` | Glossary table name. |

Source audio fields:

| Field | Type | Required | SDK default | Notes |
| --- | --- | --- | --- | --- |
| `format` | string | yes | `wav` | Upstream currently only supports `wav`. |
| `codec` | string | no | `raw` | PCM encoding. |
| `rate` | int | no | `16000` | Must be `16000`. |
| `bits` | int | no | `16` | Must be `16`. |
| `channel` | int | no | `1` | Upstream currently requires mono. |

Target audio fields:

| Field | Type | Required | SDK default | Notes |
| --- | --- | --- | --- | --- |
| `format` | string | required for `s2s` | `ogg_opus` for `s2s` | Supports `pcm` and `ogg_opus`. |
| `rate` | int | required for `s2s` | `48000` for `s2s` | PCM supports `16000` and `24000`; `ogg_opus` output sample rate is fixed at `48000`. |
| `bits` | int | no | empty | PCM at 16000 Hz defaults to 16-bit int; PCM at 24000 Hz defaults to 32-bit float. |
| `channel` | int | no | `1` for `s2s` | Mono output. |

Example:

```json
{
  "request_meta": {
    "session_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
  },
  "event": 100,
  "user": {
    "uid": "388808088185088",
    "did": "device-id"
  },
  "source_audio": {
    "format": "wav",
    "codec": "raw",
    "rate": 16000,
    "bits": 16,
    "channel": 1
  },
  "target_audio": {
    "format": "pcm",
    "rate": 24000
  },
  "request": {
    "mode": "s2s",
    "speaker_id": "zh_female_vv_uranus_bigtts",
    "speech_rate": 0,
    "source_language": "zh",
    "target_language": "en",
    "corpus": {
      "hot_words_list": ["video livestream", "smart home"],
      "boosting_table_id": "",
      "boosting_table_name": "",
      "correct_words": "{\"Accept\":\"Receive\"}",
      "regex_correct_table_id": "",
      "regex_correct_table_name": "",
      "glossary_list": {
        "artificial intelligence": "machine learning"
      },
      "glossary_table_id": "",
      "glossary_table_name": ""
    }
  }
}
```

## TaskRequest Audio Upload

After sending `StartSession`, wait for `SessionStarted` before sending
`TaskRequest` audio packets. The audio bytes must match the `source_audio`
configuration. Upstream recommends 16 kHz, 16-bit, mono `wav`/PCM audio and
about 80 ms per packet.

| Field | Level | Type | Required | SDK API | Notes |
| --- | ---: | --- | --- | --- | --- |
| `event` | 1 | int32 | yes | internal | `200` for `TaskRequest`. |
| `source_audio` | 1 | object | yes | internal | Source audio object. |
| `source_audio.data` | 2 | bytes | yes | `ASTTranslateSession.SendAudio` | Raw audio packet bytes. |

Example:

```json
{
  "event": 200,
  "source_audio": {
    "data": "<audio bytes>"
  }
}
```

## UpdateConfig Request

`UpdateConfig` updates corpus/intervention parameters inside the current
session. The upstream service does not support switching language or `mode`
inside the current session; create a new session for those changes.

| Field | Type | SDK API | Notes |
| --- | --- | --- | --- |
| `event` | int32 | internal | `201` for `UpdateConfig`. |
| `request.mode` | string | not updated by SDK | Included in the upstream example, but language/mode switching is not supported in-session. |
| `request.corpus` | object | `ASTTranslateSession.UpdateConfig` | Updates hotwords, replacement words, and glossary entries. |

Example:

```json
{
  "event": 201,
  "request": {
    "mode": "s2s",
    "corpus": {
      "hot_words_list": ["video livestream", "smart home"],
      "boosting_table_id": "",
      "boosting_table_name": "",
      "correct_words": "{\"Accept\":\"Receive\"}",
      "regex_correct_table_id": "",
      "regex_correct_table_name": "",
      "glossary_list": {
        "artificial intelligence": "machine learning"
      },
      "glossary_table_id": "",
      "glossary_table_name": ""
    }
  }
}
```

## FinishSession Request

Send `FinishSession` after all audio packets have been uploaded.

```json
{
  "event": 102
}
```

## Server Response Fields

The service returns protobuf `TranslateResponse` messages. The SDK maps the key
fields to `ASTTranslateEvent`.

| Field | Level | Type | SDK field | Notes |
| --- | ---: | --- | --- | --- |
| `response_meta` | 1 | object | `SessionID`, `Usage`, `Error` | Response metadata. |
| `response_meta.status_code` | 2 | int | `Error.Code` | Server status code. |
| `response_meta.message` | 2 | string | `Error.Message` | Server message. |
| `response_meta.billing` | 2 | object | `Usage` | Present on `UsageResponse`. |
| `response_meta.billing.duration_msec` | 3 | int | `Usage.DurationMS` | Audio duration in milliseconds. |
| `response_meta.billing.items` | 3 | array | `Usage.Items` | Billing details. |
| `response_meta.billing.items.unit` | 4 | string | `Usage.Items[].Unit` | `output_text_tokens`, `output_audio_tokens`, or `input_audio_tokens`. |
| `response_meta.billing.items.quantity` | 4 | float | `Usage.Items[].Quantity` | Token amount. |
| `event` | 1 | int32 | `Type` | Server event ID. |
| `text` | 1 | string | `Text` | Source or translated text. |
| `data` | 1 | bytes | `Audio` | TTS audio bytes. |
| `start_time` | 1 | int | `StartTimeMS` | Start timestamp in milliseconds. |
| `end_time` | 1 | int | `EndTimeMS` | End timestamp in milliseconds. |
| `spk_chg` | 1 | bool | `SpeakerChanged` | Set on subtitle starts when speaker changes. |
| `detected_language` | 1 | string | `DetectedLanguage` | Returned on `SourceSubtitleEnd` when language detection is enabled. |
| `muted_duration_ms` | 1 | int | `MutedDurationMS` | Approximate muted duration in milliseconds. |

## Server Event Payloads

`SessionStarted`:

```json
{
  "event": 150
}
```

`SourceSubtitleStart`:

```json
{
  "event": 650,
  "start_time": 1000,
  "spk_chg": false
}
```

`SourceSubtitleResponse`:

```json
{
  "event": 651,
  "text": "source text"
}
```

`SourceSubtitleEnd`:

```json
{
  "event": 652,
  "start_time": 1000,
  "end_time": 2200,
  "text": "source text",
  "detected_language": "zh"
}
```

`TranslationSubtitleStart`:

```json
{
  "event": 653,
  "start_time": 1000,
  "spk_chg": false
}
```

`TranslationSubtitleResponse`:

```json
{
  "event": 654,
  "text": "translated text"
}
```

`TranslationSubtitleEnd`:

```json
{
  "event": 655,
  "start_time": 1000,
  "end_time": 2200,
  "text": "translated text"
}
```

`TTSSentenceStart`:

```json
{
  "event": 350,
  "start_time": 1000
}
```

`TTSResponse`:

```json
{
  "event": 352,
  "data": "<audio bytes>"
}
```

`TTSSentenceEnd`:

```json
{
  "event": 351,
  "data": "<audio bytes>",
  "start_time": 1000,
  "end_time": 2200
}
```

`UsageResponse`:

```json
{
  "event": 154,
  "response_meta": {
    "session_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "status_code": 20000000,
    "message": "OK",
    "billing": {
      "items": [
        {
          "unit": "output_text_tokens",
          "quantity": 15.0
        },
        {
          "unit": "output_audio_tokens",
          "quantity": 11.0
        },
        {
          "unit": "input_audio_tokens",
          "quantity": 4.0
        }
      ],
      "duration_msec": 640
    }
  }
}
```

`SessionFinished`:

```json
{
  "event": 152
}
```

`SessionFailed`:

```json
{
  "event": 153
}
```

`AudioMuted`:

```json
{
  "event": 250,
  "muted_duration_ms": 3000
}
```

`AudioMuted` is returned when VAD detects silence. The upstream document notes
that the first muted event is returned after roughly 2 seconds of silence, and
then about once per additional second. The duration is approximate.

## Error Messages And Codes

When the server cannot resolve a binary/protobuf transport problem, it returns
an error message using the same response metadata shape.

| Code | Meaning | Notes |
| ---: | --- | --- |
| `20000000` | Success | |
| `45000001` | Invalid request parameters | Missing required field, invalid field value, or duplicated request. |
| `45000002` | Empty audio | |
| `45000081` | Packet wait timeout | |
| `45000151` | Invalid audio format | |
| `550xxxxx` | Internal service error | |
| `55000031` | Server busy | Service overload. |

## Example

```bash
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/ast_v2_translate -mode s2t -source zh -target en
```
