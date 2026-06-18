# AST V2 Translate API

This document describes the VolcEngine AST realtime translation API protocol notes.

Current SDK boundary: this repository provides typed Go wrappers for AST realtime translation through `Client.ASTTranslate` / `Client.AST`. This file also keeps protocol notes for debugging and future extension.

## Example Path

```text
examples/ast_v2_translate
```

## Overview

AST provides realtime access to the simultaneous interpretation model over WebSocket. It supports:

- S2S: Speech-to-Speech
- S2T: Speech-to-Text
- cloning the source speaker's voice in supported S2S flows

Endpoint:

```text
wss://openspeech.bytedance.com/api/v4/ast/v2/translate
```

Resource ID:

```text
volc.service_type.10053
```

## Language Support

| Mode | Language setting mode | Supported languages | Count | Notes |
|---|---|---|---:|---|
| S2S | source and target language required; source or target must be Chinese or English | `zh`, `en`, `pt`, `es`, `ja`, `id`, `de`, `fr` | 8 | If target is `zh` or `en`, public TTS speakers can be used |
| S2S | automatic Chinese/English detection | `zh`, `en` | 2 | no language switch required |
| S2T | source and target language required; source or target must be Chinese or English; dialects can only be source languages | foreign languages: `zh`, `en`, `pt`, `es`, `ja`, `id`, `de`, `fr`, `ru`, `it`, `ko`, `ar`, `tr`, `ms`, `vi`, `th`, `nl`, `ro`, `pl`, `cs`; dialects: Cantonese, Shanghainese | 20 foreign languages + 2 dialects | text output only |

S2S public speakers:

- `zh_female_vv_uranus_bigtts`
- `zh_male_jingqiangkanye_emo_mars_bigtts`

## Authentication

When using the new speech console, use API key authentication.

| Header | Required | Description | Example |
|---|---:|---|---|
| `X-Api-Key` | yes | API key from VolcEngine speech console | `your-api-key` |
| `X-Api-Resource-Id` | yes | fixed AST resource ID | `volc.service_type.10053` |

Python header example:

```python
headers = {
    "X-Api-Key": "your-api-key",
    "X-Api-Resource-Id": "volc.service_type.10053",
}
```

When using the old speech console:

| Header | Required | Description | Example |
|---|---:|---|---|
| `X-Api-App-Id` | yes | App ID from old console | `123456789` |
| `X-Api-Access-Key` | yes | Access token from old console | `your-access-key` |
| `X-Api-Resource-Id` | yes | fixed AST resource ID | `volc.service_type.10053` |

Python header example:

```python
headers = {
    "X-Api-App-Id": "123456789",
    "X-Api-Access-Key": "your-access-key",
    "X-Api-Resource-Id": "volc.service_type.10053",
}
```

The WebSocket handshake response includes:

| Header | Description | Example |
|---|---|---|
| `X-Tt-Logid` | server log ID; print it for troubleshooting | `202407261553070FACFE6D19421815D605` |

HTTP upgrade sketch:

```text
GET /api/v4/ast/v2/translate
Host: openspeech.bytedance.com
X-Api-Key: your-api-key
X-Api-Resource-Id: volc.service_type.10053

# Response header
X-Tt-Logid: 202407261553070FACFE6D19421815D605
```

## Transport Protocol

AST uses WebSocket with protobuf payloads.

The SDK keeps a small internal protobuf encoder/decoder for the fields required by AST V2 translation.

## SDK Quick Start

```go
ctx := context.Background()

client := doubaospeech.NewClient(
	appID,
	doubaospeech.WithV2APIKey(accessKey, ""),
	doubaospeech.WithResourceID(doubaospeech.ResourceASTTranslate),
	doubaospeech.WithUserID("ast-user"),
)

cfg := doubaospeech.DefaultASTTranslateConfig()
cfg.Mode = doubaospeech.ASTTranslateModeS2T
cfg.SourceLanguage = "zh"
cfg.TargetLanguage = "en"

session, err := client.ASTTranslate.OpenSession(ctx, &cfg)
if err != nil {
	return err
}
defer session.Close()

if err := session.SendAudio(ctx, pcm16kMonoChunk); err != nil {
	return err
}
if err := session.Finish(ctx); err != nil {
	return err
}

for {
	evt, err := session.RecvEvent(ctx)
	if err != nil {
		return err
	}
	if evt == nil || evt.IsFinal {
		break
	}
	if evt.Type == doubaospeech.ASTEventTranslationSubtitleResponse {
		fmt.Println(evt.Text)
	}
}
```

Covered by this SDK:

- `ASTTranslate.OpenSession(ctx, cfg)`: dial WebSocket, send `StartSession`, and wait for `SessionStarted`
- `ASTTranslateSession.SendAudio(ctx, audio)`: send `TaskRequest`
- `ASTTranslateSession.UpdateConfig(ctx, update)`: update corpus terms
- `ASTTranslateSession.Finish(ctx)`: send `FinishSession`
- `ASTTranslateSession.RecvEvent(ctx)` and `Recv()`: receive parsed server events
- `ASTTranslateSession.Close()`: best-effort finish and close

## Event IDs

Client event types:

| Event | Value | Description |
|---|---:|---|
| `StartSession` | 100 | session start request |
| `UpdateConfig` | 201 | update parameters |
| `TaskRequest` | 200 | send audio data |
| `FinishSession` | 102 | finish session |

Server event types:

| Event | Value | Description |
|---|---:|---|
| `SessionStarted` | 150 | session started |
| `SourceSubtitleStart` | 650 | source subtitle starts |
| `SourceSubtitleResponse` | 651 | source subtitle data |
| `SourceSubtitleEnd` | 652 | source subtitle ends |
| `TranslationSubtitleStart` | 653 | translation subtitle starts |
| `TranslationSubtitleResponse` | 654 | translation subtitle data |
| `TranslationSubtitleEnd` | 655 | translation subtitle ends |
| `TTSSentenceStart` | 350 | TTS starts |
| `TTSResponse` | 352 | TTS audio data |
| `TTSSentenceEnd` | 351 | TTS ends |
| `UsageResponse` | 154 | billing/usage |
| `SessionFinished` | 152 | session finished normally |
| `SessionFailed` | 153 | session failed |
| `AudioMuted` | 250 | silence event |

## StartSession Request

The first protobuf message after the WebSocket upgrade is `StartSession`.

Required top-level fields:

- `request_meta.session_id`: required; UUID recommended
- `event`: required; `100`
- `request`: required
- `source_audio`: required
- `target_audio`: required for S2S, optional for S2T

Common fields:

| Field | Level | Type | Required | Notes |
|---|---:|---|---:|---|
| `request_meta` | 1 | dict | yes | request metadata |
| `request_meta.session_id` | 2 | string | yes | UUID recommended |
| `event` | 1 | int32 enum | yes | `100` for `StartSession` |
| `user` | 1 | dict | no | log filtering metadata |
| `user.uid` | 2 | string | no | IMEI or MAC recommended |
| `user.did` | 2 | string | no | device name |
| `user.platform` | 2 | string | no | iOS, Android, Linux |
| `user.sdk_version` | 2 | string | no | SDK version |
| `request.mode` | 2 | string | no | `s2t` or `s2s` |
| `request.speaker_id` | 2 | string | no | public speaker; invalid or empty value falls back to source speaker cloning |
| `request.is_custom_speaker` | 2 | bool | no | default `false`; whether external speaker is a cloned voice |
| `request.tts_resource_id` | 2 | string | required for public/clone TTS path | `seed-tts-1.0`, `seed-tts-2.0`, `seed-icl-1.0`, or `seed-icl-2.0` |
| `request.speech_rate` | 2 | number | no | `[-50, 100]`; `100` means 2.0x, `-50` means 0.5x |
| `request.source_language` | 2 | string | yes | see language table |
| `request.target_language` | 2 | string | yes | see language table |
| `request.enable_source_language_detect` | 2 | bool | no | returns detected source language in `SourceSubtitleEnd` |
| `request.corpus` | 2 | dict | no | hotwords, replacements, and glossary; total entries <= 1000 |
| `source_audio.format` | 2 | string | yes | `wav` only |
| `source_audio.codec` | 2 | string | no | `raw` only |
| `source_audio.rate` | 2 | int | yes | must be `16000` |
| `source_audio.bits` | 2 | int | yes | must be `16` |
| `source_audio.channel` | 2 | int | yes | must be `1`; mono only |
| `target_audio.format` | 2 | string | S2S yes | `pcm` or `ogg_opus` |
| `target_audio.rate` | 2 | int | S2S yes | default `24000`; supports `16000` or `24000` |

`target_audio` notes:

- `pcm` at 16 kHz defaults to 16-bit integer.
- `pcm` at 24 kHz defaults to 32-bit float.
- `ogg_opus` defaults to 32-bit float and fixed 48 kHz output; `rate` does not change the Opus output sample rate.

Corpus fields:

| Field | Type | Description |
|---|---|---|
| `hot_words_list` | `[]string` | inline hotwords; highest priority |
| `boosting_table_id` | string | hotword table ID |
| `boosting_table_name` | string | hotword table name |
| `correct_words` | JSON string | replacement map, highest priority |
| `regex_correct_table_id` | string | replacement table ID |
| `regex_correct_table_name` | string | replacement table name |
| `glossary_list` | `map[string]string` | translation glossary, highest priority |
| `glossary_table_id` | string | glossary table ID |
| `glossary_table_name` | string | glossary table name |

Start request example:

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
    "tts_resource_id": "seed-tts-2.0",
    "speech_rate": 0,
    "source_language": "zh",
    "target_language": "en",
    "corpus": {
      "hot_words_list": ["视频直播", "赛事直播"],
      "boosting_table_id": "",
      "boosting_table_name": "",
      "correct_words": "{\"接受\":\"接收\",\"Accept\":\"Receive\"}",
      "regex_correct_table_id": "",
      "regex_correct_table_name": "",
      "glossary_list": {
        "人工智能": "Machine Learning"
      },
      "glossary_table_id": "",
      "glossary_table_name": ""
    }
  }
}
```

## Language Rules

Language sets:

| Set | Count | Languages |
|---|---:|---|
| `lang_8` | 8 | Chinese, English, German, French, Spanish, Indonesian, Japanese, Portuguese |
| `lang_20` | 20 | Chinese, English, German, French, Spanish, Indonesian, Japanese, Portuguese, Korean, Turkish, Malay, Dutch, Romanian, Polish, Czech, Arabic, Thai, Vietnamese, Russian, Italian |
| dialects | 2 | Cantonese `yue-CN`, Shanghainese `sh-CN` |

Mode constraints:

| Mode | Constraints | Supported languages |
|---|---|---|
| S2T | source and target required; source or target must be Chinese/English; supports Chinese-English mixed translation with `zhen` | source: `lang_20` and dialects; target: `lang_20` |
| S2S with public speaker | pass a supported `speaker_id`; source and target required; target must be Chinese or English; supports `zhen` | source: `lang_20` and dialects; target: Chinese/English |
| S2S with speaker cloning | do not pass `speaker_id`, or pass an unsupported speaker; source and target required; source or target must be Chinese/English; supports `zhen` | source: `lang_8`; target: `lang_8` |

Language codes:

| Language | Code | Notes |
|---|---|---|
| Chinese | `zh` | one of the Chinese/English pair |
| English | `en` | one of the Chinese/English pair |
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
| Cantonese | `yue-CN` | dialect, source only |
| Shanghainese | `sh-CN` | dialect, source only |
| Chinese-English mixed translation | `zhen` | both `source_language` and `target_language` must be `zhen` |

Usage notes:

- Set `source_language` and `target_language` to the parameter values above.
- For `mode=s2t`, follow S2T language constraints.
- For `mode=s2s` with a supported `speaker_id`, follow S2S public speaker constraints.
- For `mode=s2s` without a supported `speaker_id`, follow S2S speaker cloning constraints.

## TaskRequest Audio

Wait for `SessionStarted` before sending parameter packets or audio packets.

`TaskRequest` sends audio data in the protobuf request body. The audio must match the `source_audio` format declared in `StartSession`.

Recommended packet size:

- 16 kHz
- 16-bit
- mono
- WAV/PCM
- 80 ms per packet

Fields:

| Field | Level | Type | Required | Notes |
|---|---:|---|---:|---|
| `event` | 1 | int32 enum | yes | `200` for `TaskRequest` |
| `source_audio` | 1 | dict | yes | source audio info |
| `source_audio.data` | 2 | bytes | yes | audio bytes |

Example shape:

```json
{
  "event": 200,
  "source_audio": {
    "data": "binary audio bytes"
  }
}
```

## UpdateConfig

`UpdateConfig` updates corpus/intervention terms during a session.

Current restriction: language and `mode` cannot be switched inside a session. To switch language or mode, start a new session.

Example:

```json
{
  "event": 201,
  "request": {
    "mode": "s2s",
    "corpus": {
      "hot_words_list": ["视频直播", "赛事直播"],
      "boosting_table_id": "",
      "boosting_table_name": "",
      "correct_words": "{\"接受\":\"接收\",\"Accept\":\"Receive\"}",
      "regex_correct_table_id": "",
      "regex_correct_table_name": "",
      "glossary_list": {
        "人工智能": "Machine Learning"
      },
      "glossary_table_id": "",
      "glossary_table_name": ""
    }
  }
}
```

## FinishSession

Send `FinishSession` after all audio has been sent:

```json
{
  "event": 102
}
```

## Server Response Fields

Common response fields:

| Field | Level | Type | Description |
|---|---:|---|---|
| `response_meta` | 1 | dict | response metadata |
| `response_meta.status_code` | 2 | int | error code |
| `response_meta.message` | 2 | string | error message |
| `response_meta.billing` | 2 | dict | billing details, only returned by `UsageResponse` |
| `response_meta.billing.duration_msec` | 3 | int | audio duration in milliseconds |
| `response_meta.billing.items` | 3 | array | billing items |
| `response_meta.billing.items.unit` | 4 | string | `output_text_tokens`, `output_audio_tokens`, `input_audio_tokens` |
| `response_meta.billing.items.quantity` | 4 | float | consumed token quantity |
| `event` | 1 | int | server event ID |
| `text` | 1 | string | source or translated text |
| `data` | 1 | raw | binary response data |
| `start_time` | 1 | int | start time in milliseconds |
| `end_time` | 1 | int | end time in milliseconds |
| `spk_chg` | 1 | bool | whether speaker changed; appears on subtitle start events |
| `muted_duration_ms` | 1 | int | approximate silence duration in milliseconds |

## Server Event Examples

Session started:

```json
{
  "event": 150
}
```

Source subtitle starts:

```json
{
  "event": 650,
  "start_time": 0,
  "spk_chg": false
}
```

Source subtitle response:

```json
{
  "event": 651,
  "text": "原文文本"
}
```

Source subtitle ends:

```json
{
  "event": 652,
  "start_time": 0,
  "end_time": 1000,
  "text": "原文文本"
}
```

When source language detection is enabled, `SourceSubtitleEnd` can include:

```json
{
  "event": 652,
  "text": "今日天气不错",
  "detected_language": "zh"
}
```

Translation subtitle starts:

```json
{
  "event": 653,
  "start_time": 0,
  "spk_chg": false
}
```

Translation subtitle response:

```json
{
  "event": 654,
  "text": "translated text"
}
```

Translation subtitle ends:

```json
{
  "event": 655,
  "start_time": 0,
  "end_time": 1000,
  "text": "translated text"
}
```

TTS starts:

```json
{
  "event": 350,
  "start_time": 0
}
```

TTS audio data:

```json
{
  "event": 352,
  "data": "binary audio data"
}
```

TTS ends:

```json
{
  "event": 351,
  "data": "binary audio data",
  "start_time": 0,
  "end_time": 1000
}
```

Usage response:

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

Session finished:

```json
{
  "event": 152
}
```

Session failed:

```json
{
  "event": 153
}
```

Audio muted:

```json
{
  "event": 250,
  "muted_duration_ms": 3000
}
```

`AudioMuted` is returned when VAD detects silence. The first response is after around 2 seconds of silence, then roughly once per additional second. The value is approximate.

## Error Message From Server

When the server cannot handle a binary or transport protocol issue, it sends an error message using the response metadata shape above. One example is sending a serialization format that the server does not support.

## Error Codes

| Code | Meaning | Description |
|---:|---|---|
| 20000000 | success | request succeeded |
| 45000001 | invalid request parameter | required field missing, invalid value, or duplicate request |
| 45000002 | empty audio | audio data is empty |
| 45000081 | packet wait timeout | timed out waiting for packets |
| 45000151 | invalid audio format | audio format is incorrect |
| 550xxxxx | internal processing error | service-side processing failed |
| 55000031 | server busy | service overloaded and cannot process the request |
