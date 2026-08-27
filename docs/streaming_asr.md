# Streaming ASR

Official documentation: <https://www.volcengine.com/docs/6561/1354869>

This page maps the VolcEngine big-model streaming ASR SAUC WebSocket API to the
current SDK surface. The upstream API supports bidirectional streaming,
streaming-input/non-streaming-result, and an optimized bidirectional endpoint.

## Endpoints

| Mode | Endpoint | Upstream behavior | SDK coverage |
| --- | --- | --- | --- |
| Bidirectional streaming | `wss://openspeech.bytedance.com/api/v3/sauc/bigmodel` | One response per input packet; returns recognized characters as soon as possible. | Implemented by `ASRV2.OpenStreamSession`. |
| Streaming input / no-stream result | `wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_nostream` | Returns recognition after input audio exceeds 15 seconds or after the final negative packet; higher accuracy. | Documented only; no dedicated SDK method yet. |
| Optimized bidirectional streaming | `wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_async` | Returns a new packet only when the result changes; upstream recommends this for better RTF and first/last-character latency. | Documented only; no dedicated SDK method yet. |

Packet guidance from upstream:

- Single-packet audio duration should be around 100 to 200 ms.
- Send interval should be around 100 to 200 ms.
- For bidirectional streaming, 200 ms packets are recommended for best
  performance.
- In streaming-input mode, when average audio duration is around 5 seconds,
  upstream says recognition can return within roughly 300 to 400 ms.

## SDK Coverage

Implemented by `ASRV2.OpenStreamSession`.

The SDK supports:

- API-key authentication headers plus request metadata headers
- `bigmodel` endpoint connection
- full-client JSON start frame
- audio-only request frames
- final negative audio packet through `SendAudio(ctx, audio, true)`
- full-server JSON response decoding
- error frame decoding
- parsed utterances and word timing
- result metadata (`reqid`, `trace_id`, `log_id`, `connect_id`)

Typed SDK request fields:

| SDK field | Upstream field | Notes |
| --- | --- | --- |
| `Format` | `audio.format` | Defaults to `pcm`; upstream supports `pcm`, `wav`, `ogg`, and `mp3`. |
| `SampleRate` | `audio.rate` / SDK payload currently uses `sample_rate` | Defaults to `16000`. |
| `Channel` / `Channels` | `audio.channel` | Defaults to `1`. |
| `Bits` | `audio.bits` | Defaults to `16`. |
| `Language` | `audio.language` | Only supported by `bigmodel_nostream` upstream. |
| `Codec` | `audio.codec` | `raw` or `opus`; omitted to use the service default. |
| `User` | `user` | Typed UID, device, platform, SDK-version, and app-version metadata. |
| `EnableITN` | `request.enable_itn` | Text inverse normalization. |
| `EnablePunc` | `request.enable_punc` | Punctuation. |
| `EnableDiarization` | SDK-specific request field | Current SDK sends `enable_diarization`; upstream document names speaker options differently. |
| `SpeakerNum` | SDK-specific request field | Current SDK sends `speaker_num`. |
| `Hotwords` | SDK-specific request field | Current SDK sends `hotwords`; upstream hotword configuration is under `request.corpus`. |
| `ResultType` | `request.result_type` | `single` or `full`; SDK defaults to `single`. |
| `Request` | `request` | Complete typed BigASR request configuration, including VAD, acceleration, detection, and corpus fields. |
| `ResourceID` | `X-Api-Resource-Id` | Defaults through the client resource configuration. |

Every documented BigASR request parameter below is represented by
`ASRV2RequestConfig`; the SDK does not expose an arbitrary request passthrough
map. Optional scalar fields use pointers so callers can distinguish omission
from an explicit `false` or `0`.

For example, configure forced endpointing without using an untyped request map:

```go
endWindowSize := 200
forceToSpeechTime := 0

config := &doubaospeech.ASRV2Config{
    Format:     doubaospeech.FormatPCM,
    SampleRate: doubaospeech.SampleRate16000,
    Request: &doubaospeech.ASRV2RequestConfig{
        EndWindowSize:     &endWindowSize,
        ForceToSpeechTime: &forceToSpeechTime,
    },
}
```

## Authentication

API-key authentication headers:

```http
X-Api-Key: <api-key>
X-Api-Resource-Id: <resource-id>
X-Api-Request-Id: <uuid>
X-Api-Sequence: -1
```

When configured, the SDK may also send `X-Api-App-Id` as request/application
configuration. It is not an authentication factor.

Resource IDs:

| Product | Billing mode | Resource ID |
| --- | --- | --- |
| Streaming ASR 1.0 | duration | `volc.bigasr.sauc.duration` |
| Streaming ASR 1.0 | concurrent | `volc.bigasr.sauc.concurrent` |
| Streaming ASR 2.0 | duration | `volc.seedasr.sauc.duration` |
| Streaming ASR 2.0 | concurrent | `volc.seedasr.sauc.concurrent` |

Handshake request example:

```http
GET /api/v3/sauc/bigmodel
Host: openspeech.bytedance.com
X-Api-Key: <api-key>
X-Api-Resource-Id: volc.bigasr.sauc.duration
X-Api-Connect-Id: <uuid>
```

The WebSocket handshake response can include:

| Header | Meaning |
| --- | --- |
| `X-Api-Connect-Id` | Connection trace ID. |
| `X-Tt-Logid` | Server log ID; log this value for troubleshooting. |

## Binary Protocol

WebSocket frame payloads use the SAUC binary protocol. Integers are big-endian.
Each frame contains:

- 4-byte header
- optional header extensions
- payload size
- payload

Header bytes:

| Byte | Left 4 bits | Right 4 bits | Meaning |
| ---: | --- | --- | --- |
| `0` | protocol version | header size | Version and header length. |
| `1` | message type | message-specific flags | Request/response kind and sequence/final flags. |
| `2` | serialization method | compression method | Payload encoding. |
| `3` | reserved | reserved | Reserved byte. |

Header field values:

| Field | Values |
| --- | --- |
| Protocol version | `0b0001`: version 1. |
| Header size | `0b0001`: 4 bytes. |
| Message type | `0b0001`: full client request; `0b0010`: audio-only request; `0b1001`: full server response; `0b1111`: error response. |
| Message type specific flags | `0b0000`: no sequence; `0b0001`: positive sequence follows; `0b0010`: final packet without sequence; `0b0011`: final packet with negative sequence. |
| Serialization | `0b0000`: raw/no serialization; `0b0001`: JSON. |
| Compression | `0b0000`: no compression; `0b0001`: gzip. |

## Full Client Request

The first request after WebSocket upgrade is a full-client request. It contains
audio metadata and recognition parameters, usually serialized as JSON.

Frame layout:

```text
header
payload_size uint32
payload
```

Request fields:

| Field | Level | Type | Required | SDK field | Notes |
| --- | ---: | --- | --- | --- | --- |
| `user` | 1 | object | no | client user ID | Helps server-side log filtering. |
| `user.uid` | 2 | string | no | `User.UID` or client user ID | Upstream recommends IMEI or MAC. |
| `user.did` | 2 | string | no | `User.DID` | Device name. |
| `user.platform` | 2 | string | no | `User.Platform` | For example `iOS`, `Android`, `Linux`. |
| `user.sdk_version` | 2 | string | no | `User.SDKVersion` | SDK version. |
| `user.app_version` | 2 | string | no | `User.AppVersion` | App version. |
| `audio` | 1 | object | yes | `ASRV2Config` audio fields | Audio metadata. |
| `request` | 1 | object | yes | `ASRV2Config` request fields | Recognition parameters. |

Audio fields:

| Field | Type | Required | SDK field | Notes |
| --- | --- | --- | --- | --- |
| `language` | string | no | `Language` | Upstream says this is only supported by `bigmodel_nostream`; second-pass mode does not support it. Empty language supports Chinese/English and several dialects by default. |
| `format` | string | yes | `Format` | `pcm`, `wav`, `ogg`, or `mp3`; PCM/WAV streams must be `pcm_s16le`. |
| `codec` | string | no | `Codec` | `raw` or `opus`; default `raw`. For `ogg`, codec must be `opus`; for `mp3`, codec is ignored. |
| `rate` | int | no | `SampleRate` | Defaults to `16000`; upstream currently supports only `16000`. |
| `bits` | int | no | `Bits` | Defaults to `16`; upstream currently supports only 16-bit. |
| `channel` | int | no | `Channel` / `Channels` | `1` mono or `2` stereo; default `1`. |

Language values for `audio.language` and `enable_auto_lang`:

| Language | Value |
| --- | --- |
| Chinese Mandarin | `zh-CN` |
| English | `en-US` |
| Japanese | `ja-JP` |
| Indonesian | `id-ID` |
| Spanish | `es-MX` |
| Portuguese | `pt-BR` |
| German | `de-DE` |
| French | `fr-FR` |
| Korean | `ko-KR` |
| Filipino | `fil-PH` |
| Malay | `ms-MY` |
| Thai | `th-TH` |
| Arabic | `ar-SA` |
| Italian | `it-IT` |
| Bengali | `bn-BD` |
| Greek | `el-GR` |
| Dutch | `nl-NL` |
| Russian | `ru-RU` |
| Turkish | `tr-TR` |
| Vietnamese | `vi-VN` |
| Polish | `pl-PL` |
| Romanian | `ro-RO` |
| Nepali | `ne-NP` |
| Ukrainian | `uk-UA` |
| Cantonese | `yue-CN` |

Request fields:

| Field | Type | SDK field | Notes |
| --- | --- | --- | --- |
| `model_name` | string | `Request.ModelName` | Upstream currently uses `bigmodel`. |
| `enable_nonstream` | bool | `Request.EnableNonstream` | Streaming plus non-streaming second-pass recognition; only supported by optimized bidirectional mode. Enables VAD segmentation and outputs `definite: true` only from nostream recognition. |
| `enable_itn` | bool | `Request.EnableITN` | Defaults to true upstream. Converts spoken text to written form, such as dates and currency. |
| `enable_speaker_info` | bool | `Request.EnableSpeakerInfo` | Speaker clustering; requires empty/default Chinese language or `zh-CN`. For async mode, `enable_nonstream` must be true. Requires `ssd_version="200"`. |
| `ssd_version` | string | `Request.SSDVersion` | `200` enables big-model SSD. Recommended with ASR 2.0, not recommended with ASR 1.0. |
| `enable_punc` | bool | `Request.EnablePunc` | Defaults to true upstream. |
| `enable_ddc` | bool | `Request.EnableDDC` | Semantic smoothing; removes or edits filler, repetition, and disfluent words. Defaults to false. |
| `output_zh_variant` | string | `Request.OutputZHVariant` | `traditional`, `tw`, or `hk`. |
| `enable_auto_lang` | bool | `Request.EnableAutoLanguage` | Only for `bigmodel_nostream`; automatically detects 25 supported languages. |
| `show_utterances` | bool | `Request.ShowUtterances` | Enables utterance, pause, and word-level information; defaults to true in the SDK. |
| `show_speech_rate` | bool | `Request.ShowSpeechRate` | Only for `bigmodel_nostream` and async; puts `speech_rate` in utterance additions. |
| `show_volume` | bool | `Request.ShowVolume` | Only for `bigmodel_nostream` and async; puts `volume` in utterance additions. |
| `enable_lid` | bool | `Request.EnableLanguageID` | Only for `bigmodel_nostream` and async; language detection labels in additions. |
| `enable_emotion_detection` | bool | `Request.EnableEmotionDetection` | Only for `bigmodel_nostream` and async; emotion labels in additions. |
| `enable_gender_detection` | bool | `Request.EnableGenderDetection` | Only for `bigmodel_nostream` and async; `male` / `female` labels in additions. |
| `result_type` | string | `Request.ResultType` | `full` for complete result, `single` for incremental result. |
| `enable_accelerate_text` | bool | `Request.EnableAccelerateText` | Accelerates first-character return with lower first-character accuracy. |
| `accelerate_score` | int | `Request.AccelerateScore` | Works with `enable_accelerate_text`; range `[0,20]`, higher means faster first character. |
| `vad_segment_duration` | int | `Request.VADSegmentDuration` | Semantic segmentation max silence threshold in ms; default `3000`; ignored when `end_window_size` is set. |
| `end_window_size` | int | `Request.EndWindowSize` | Enables forced endpointing with this silence duration in ms; minimum `200`; outputs `definite`. When omitted, the service can follow the longer semantic-segmentation path. |
| `force_to_speech_time` | int | `Request.ForceToSpeechTime` | Minimum audio duration before endpointing; requires `end_window_size`; recommended `1000`. Use a pointer to send an explicit `0`. |
| `sensitive_words_filter` | string | `Request.SensitiveWordsFilter` | Supports no-op, empty replacement, or `*` replacement. It is a pointer so an explicit empty string is preserved. |
| `enable_poi_fc` | bool | `Request.EnablePOIFC` | POI function call; supported by nostream and async with second-pass. |
| `enable_music_fc` | bool | `Request.EnableMusicFC` | Music function call; supported by nostream and async with second-pass. |
| `corpus` | object | `Request.Corpus` | Typed hotword, correction, and context settings. |

LID labels that can return recognition results:

| Label | Meaning |
| --- | --- |
| `singing_en` | English singing |
| `singing_mand` | Mandarin singing |
| `singing_dia_cant` | Cantonese singing |
| `speech_en` | English speech |
| `speech_mand` | Mandarin speech |
| `speech_dia_nan` | Minnan speech |
| `speech_dia_wuu` | Wu/Shanghainese speech |
| `speech_dia_cant` | Cantonese speech |
| `speech_dia_xina` | Southwestern Mandarin, including Sichuanese |
| `speech_dia_zgyu` | Central Plains Mandarin, including Shaanxi |
| `other_langs` | Other human language |
| `others` | Non-semantic human voice or non-human sound |

LID labels that can be detected but do not have recognition output:

| Label | Meaning |
| --- | --- |
| `singing_hi` | Hindi singing |
| `singing_ja` | Japanese singing |
| `singing_ko` | Korean singing |
| `singing_th` | Thai singing |
| `speech_hi` | Hindi speech |
| `speech_ja` | Japanese speech |
| `speech_ko` | Korean speech |
| `speech_th` | Thai speech |
| `speech_kk` | Kazakh speech |
| `speech_bo` | Tibetan speech |
| `speech_ug` | Uyghur speech |
| `speech_mn` | Mongolian speech |
| `speech_dia_ql` | Qiong-Lei speech |
| `speech_dia_hsn` | Xiang speech |
| `speech_dia_jin` | Jin speech |
| `speech_dia_hak` | Hakka speech |
| `speech_dia_chao` | Chaoshan speech |
| `speech_dia_juai` | Jianghuai Mandarin |
| `speech_dia_lany` | Lan-Yin Mandarin |
| `speech_dia_dbiu` | Northeastern Mandarin |
| `speech_dia_jliu` | Jiao-Liao Mandarin |
| `speech_dia_jlua` | Ji-Lu Mandarin |
| `speech_dia_cdo` | Mindong speech |
| `speech_dia_gan` | Gan speech |
| `speech_dia_mnp` | Minbei speech |
| `speech_dia_czh` | Hui speech |

Emotion labels:

| Label | Meaning |
| --- | --- |
| `angry` | Angry |
| `happy` | Happy |
| `neutral` | Neutral |
| `sad` | Sad |
| `surprise` | Surprise |

Corpus fields:

| Field | Type | SDK coverage | Notes |
| --- | --- | --- | --- |
| `boosting_table_name` | string | `Corpus.BoostingTableName` | Hotword table name. |
| `boosting_table_id` | string | `Corpus.BoostingTableID` | Hotword table ID. |
| `correct_table_name` | string | `Corpus.CorrectTableName` | Replacement-word table name. |
| `correct_table_id` | string | `Corpus.CorrectTableID` | Replacement-word table ID. |
| `context` | string | `Corpus.Context` | The SDK serializes typed hotwords, corrections, and dialogue context to the JSON string required upstream. Direct hotwords have higher priority than tables. |

`corpus.context` supports direct hotwords:

```json
{
  "hotwords": [
    {
      "word": "hotword 1"
    },
    {
      "word": "hotword 2"
    }
  ]
}
```

`corpus.context` also supports dialogue context. Limits from upstream:

- bidirectional streaming hotword context: 100 tokens
- nostream hotword context: 5000 words
- dialogue context: 800 tokens and up to 20 turns
- oversized dialogue context is truncated from newest to oldest, keeping newer
  turns first
- ASR 2.0 can include visual context via one `image_url`; image size limit is
  500 KB and supported formats are JPEG, JPG, and PNG

Example dialogue context:

```json
{
  "context_type": "dialog_ctx",
  "context_data": [
    {
      "text": "text1"
    },
    {
      "image_url": "https://example.com/image.jpg"
    },
    {
      "text": "text2"
    }
  ]
}
```

Full-client request example:

```json
{
  "user": {
    "uid": "388808088185088"
  },
  "audio": {
    "format": "wav",
    "rate": 16000,
    "bits": 16,
    "channel": 1,
    "language": "zh-CN"
  },
  "request": {
    "model_name": "bigmodel",
    "enable_itn": false,
    "enable_ddc": false,
    "enable_punc": false,
    "corpus": {
      "boosting_table_id": "hotword-table-id",
      "context": "{\"context_type\":\"dialog_ctx\",\"context_data\":[{\"text\":\"text1\"},{\"text\":\"text2\"}]}"
    }
  }
}
```

## Audio-Only Request

After the full-client request, send audio-only client requests. The audio bytes
must match the `audio` metadata from the start request.

Frame layout:

```text
header
payload_size uint32
payload
```

The payload is the compressed or uncompressed audio packet, depending on the
header compression flag. The final audio packet is indicated through the
message-specific flags. In the SDK, call:

```go
err := session.SendAudio(ctx, audioBytes, true)
```

to mark the final negative packet.

## Full Server Response

The service returns full-server response frames with sequence data and JSON
payloads.

Frame layout:

```text
header
sequence int32
payload_size uint32
payload
```

Response payload fields:

| Field | Level | Type | SDK field | Notes |
| --- | ---: | --- | --- | --- |
| `result` | 1 | object | `ASRV2Result` | Recognition result. |
| `result.text` | 2 | string | `Text` | Whole recognized text. |
| `result.utterances` | 2 | array | `Utterances` | Present when `show_utterances` is enabled. |
| `result.utterances[].text` | 3 | string | `ASRV2Utterance.Text` | Utterance text. |
| `result.utterances[].start_time` | 3 | int | `StartTime` | Start time in milliseconds. |
| `result.utterances[].end_time` | 3 | int | `EndTime` | End time in milliseconds. |
| `result.utterances[].definite` | 3 | bool | `Definite` / `IsFinal` | Whether this is a definite utterance. |
| `result.utterances[].words` | 3 | array | `Words` | Word-level timing. |
| `result.utterances[].words[].text` | 4 | string | `ASRV2Word.Text` | Word text. |
| `result.utterances[].words[].start_time` | 4 | int | `ASRV2Word.StartTime` | Word start time in milliseconds. |
| `result.utterances[].words[].end_time` | 4 | int | `ASRV2Word.EndTime` | Word end time in milliseconds. |
| `result.utterances[].words[].conf` | 4 | float | `ASRV2Word.Conf` | Word confidence. |
| `audio_info.duration` | 2 | int | `Duration` | Audio duration in milliseconds. |

Example:

```json
{
  "audio_info": {
    "duration": 3696
  },
  "result": {
    "text": "This is ByteDance.",
    "utterances": [
      {
        "definite": true,
        "start_time": 0,
        "end_time": 1705,
        "text": "This is ByteDance.",
        "words": [
          {
            "blank_duration": 0,
            "start_time": 740,
            "end_time": 860,
            "text": "This"
          }
        ]
      }
    ]
  }
}
```

## Error Frame

When the server cannot resolve a binary or transport protocol problem, it sends
an error response frame.

Frame layout:

```text
header
error_code uint32
error_message_size uint32
error_message UTF-8
```

Error codes:

| Code | Meaning | Notes |
| ---: | --- | --- |
| `20000000` | Success | |
| `45000001` | Invalid request parameters | Missing required field, invalid field value, or duplicated request. |
| `45000002` | Empty audio | |
| `45000081` | Packet wait timeout | |
| `45000151` | Invalid audio format | |
| `550xxxxx` | Internal service error | |
| `55000031` | Server busy | Service overload. |

## Message Flow Example

1. Client sends a full-client request:

```text
version: 0b0001
header size: 0b0001
message type: 0b0001
flags: 0b0000
serialization: 0b0001 JSON
compression: 0b0001 gzip
payload: gzip-compressed JSON request
```

2. Server responds with a full-server response:

```text
message type: 0b1001
flags: 0b0001
serialization: 0b0001 JSON
compression: 0b0001 gzip
sequence: 1
payload: gzip-compressed JSON response
```

3. Client sends audio-only packets:

```text
message type: 0b0010
flags: 0b0000
serialization: 0b0000 raw bytes
compression: 0b0001 gzip
payload: gzip-compressed audio bytes
```

4. Client sends the final audio-only packet:

```text
message type: 0b0010
flags: 0b0010
serialization: 0b0000 raw bytes
compression: 0b0001 gzip
payload: gzip-compressed audio bytes
```

5. Server sends the final result:

```text
message type: 0b1001
flags: 0b0011
serialization: 0b0001 JSON
compression: 0b0001 gzip
sequence: negative/final sequence
payload: gzip-compressed JSON response
```

## Demo Attachments

Upstream demo attachments:

| Language | Attachment |
| --- | --- |
| Python | `sauc_python.zip` |
| Go | `sauc_go.zip` |
| Java | `sauc.zip` |

Run the SDK example:

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/asr_v2_sauc_ws
```

Run the credential-backed VAD E2E test explicitly:

```bash
cp tests/e2e/.env.example tests/e2e/.env
# Fill tests/e2e/.env with the Dev ASR credential, then run:
go test ./tests/e2e -run TestASRV2EndpointingParametersReduceShortUtteranceLatency -count=1 -v
```

The live test sends the same short PCM utterance twice in real-time-sized
chunks. The baseline leaves endpointing parameters unset; the tuned trial sets
`end_window_size=200` and `force_to_speech_time=0`. Both trials stream silence
without an audio EOS and require a non-empty `definite` transcript. The tuned
trial must save at least 250 ms, proving that the typed endpointing parameters
reduce the provider-side short-utterance finalization delay rather than merely
serializing successfully. The test is skipped unless `DOUBAO_RUN_LIVE=1`
because it consumes paid provider requests. It loads `tests/e2e/.env` first and
falls back to the repository-root `.env`; already exported environment
variables take precedence over file values.
