# TTS 1.0

Official documentation: <https://www.volcengine.com/docs/6561/1329505>

This page maps the VolcEngine TTS API-list and bidirectional WebSocket TTS
reference to the current SDK surface. The upstream page covers several TTS
transport variants and a large shared request surface for TTS 1.0, TTS 2.0,
ICL 1.0, and ICL 2.0.

## API List

| Endpoint | Recommended scenario | Capability | SDK coverage |
| --- | --- | --- | --- |
| `wss://openspeech.bytedance.com/api/v3/tts/bidirection` | WebSocket realtime interaction; streaming text input and streaming audio output. | TTS, voice clone, mix voice. | Implemented by `TTSV2.OpenStreamSession`. |
| `wss://openspeech.bytedance.com/api/v3/tts/unidirectional/stream` | WebSocket one-shot text input and streaming audio output. | TTS, voice clone, mix voice. | Not implemented as a dedicated helper. |
| `https://openspeech.bytedance.com/api/v3/tts/unidirectional` | HTTP Chunked one-shot text input and streaming audio output. | TTS, voice clone, mix voice. | Implemented by `TTSV2.Stream`. |
| `https://openspeech.bytedance.com/api/v3/tts/unidirectional/sse` | HTTP SSE one-shot text input and streaming audio output. | TTS, voice clone, mix voice. | Not implemented as a dedicated helper. |

## Bidirectional WebSocket Scope

Endpoint:

```text
wss://openspeech.bytedance.com/api/v3/tts/bidirection
```

The bidirectional API accepts fragmented or very long text and organizes it into
appropriate synthesis sentences. For LLM integration, upstream recommends
feeding streaming LLM text directly to this API instead of adding client-side
sentence splitting or buffering.

Connection reuse:

- Send `StartConnection` once per WebSocket connection.
- After `ConnectionStarted`, send `StartSession`.
- Send text through `TaskRequest` while receiving audio.
- Send `FinishSession` immediately when there is no more text for that session.
- Wait for `SessionFinished` before starting the next session on the same
  WebSocket connection.
- One WebSocket connection supports multiple sequential sessions, but not
  multiple concurrent sessions.
- Send `FinishConnection` when no more sessions are needed.

## SDK Coverage

`TTSV2.OpenStreamSession` currently supports the bidirectional flow:

- `StartConnection`
- `StartSession`
- text `TaskRequest`
- streamed audio frames
- `CancelSession`
- `FinishSession`
- sequential session reuse on one WebSocket connection

Current `TTSV2WSConfig` typed fields:

| SDK field | Upstream path | Notes |
| --- | --- | --- |
| `Speaker` | `req_params.speaker` | Required speaker / voice ID. |
| `Format` | `req_params.audio_params.format` | Defaults to `mp3` in upstream and SDK normalization unless set. |
| `SampleRate` | `req_params.audio_params.sample_rate` | Defaults to 24000 in SDK. |
| `ResourceID` | `X-Api-Resource-Id` | Defaults to `seed-tts-2.0` through client resource resolution. |

`TTSV2.Stream` implements HTTP Chunked `POST /api/v3/tts/unidirectional` and
types `text`, `speaker`, output audio parameters, emotion/language, and
`mix_speaker`.

Many upstream bidirectional fields are documented below but are not yet typed by
the WebSocket SDK. Add typed fields and tests when exposing them; do not add a
generic public passthrough map.

## Authentication

API-key WebSocket headers:

```http
X-Api-Key: <api-key>
X-Api-Resource-Id: <resource-id>
X-Api-Connect-Id: <unique-connect-id>
```

When configured, the SDK may also send `X-Api-App-Id` as request/application
configuration. It is not an authentication factor. `X-Api-Connect-Id` is
optional but recommended for troubleshooting. Upstream notes that it must be
unique per connection/session attempt and should not be reused after a failed
reconnect.

Resource IDs:

| Resource ID | Model/effect | Billing product |
| --- | --- | --- |
| `seed-tts-2.0` | Doubao TTS 2.0 voices only. | TTS 2.0 character billing. |
| `seed-tts-1.0` | Doubao TTS 1.0 voices only. | TTS 1.0 character billing. |
| `seed-tts-1.0-concurr` | Doubao TTS 1.0 voices only. | TTS 1.0 concurrent billing. |
| `seed-icl-2.0` | Voice clone 2.0 effect. | Voice clone 2.0 character billing. |
| `seed-icl-1.0` | Voice clone 1.0 effect. | Voice clone 1.0 character billing. |
| `seed-icl-1.0-concurr` | Voice clone 1.0 effect. | Voice clone 1.0 concurrent billing. |

Extra request header:

| Header | Required | Meaning |
| --- | --- | --- |
| `X-Control-Require-Usage-Tokens-Return` | no | Controls whether `SessionFinished` returns usage. Use `*`, or supported markers such as `text_words`; multiple markers are comma-separated. |

Handshake response header:

| Header | Meaning |
| --- | --- |
| `X-Tt-Logid` | Server log ID. Log it for troubleshooting. |

## Binary Protocol

WebSocket uses binary frames. Integers are big-endian.

Each frame contains:

- at least a 4-byte header
- optional fields depending on flags and event type
- payload size
- payload

Header bytes:

| Byte | Left 4 bits | Right 4 bits | Meaning |
| ---: | --- | --- | --- |
| `0` | protocol version | header size | Currently `0b0001` and 4-byte header size. |
| `1` | message type | message-specific flags | Message kind and optional event marker. |
| `2` | serialization method | compression method | Raw/JSON and no-compression/gzip. |
| `3` | reserved | reserved | `0x00`. |

Serialization and compression:

| Field | Values |
| --- | --- |
| Serialization | `0b0000`: raw bytes, mainly audio; `0b0001`: JSON. |
| Compression | `0b0000`: none; `0b0001`: gzip. |

Message types:

| Message type | Meaning | Flags | Contains event number | Notes |
| --- | --- | --- | --- | --- |
| `0b0001` | Full-client request | `0b0100` | yes | JSON request, session setup, or text task. |
| `0b1001` | Full-server response | `0b0100` | yes | JSON response, front-end info, mixed text/audio metadata. |
| `0b1011` | Audio-only response | `0b0100` | yes | Server audio data. |
| `0b1111` | Error information | none | no | Error frame. |

Optional fields can include:

- event number
- connect id size and connect id for connection events
- session id size and session id for session events
- error code for error frames

## Request Payload

TTS service parameters take effect at `StartSession`. Text takes effect at
`TaskRequest`.

Top-level payload fields:

| Field | Type | Required | Notes |
| --- | --- | --- | --- |
| `user` | object | no | User metadata. |
| `user.uid` | string | no | User ID. |
| `event` | int | yes | Event code. |
| `namespace` | string | no | `BidirectionalTTS` for connection start. |
| `req_params` | object | session/task dependent | TTS request parameters. |

Core `req_params` fields:

| Field | Type | Required | Default | Notes |
| --- | --- | --- | --- | --- |
| `text` | string | yes for `TaskRequest` | empty | Input text. Bidirectional streaming does not support SSML. |
| `model` | string | no | `seed-tts-2.0-standard` | Voice clone 2.0 only. `seed-tts-2.0-standard` has lower latency and does not support `context_texts` or `use_tag_parser`; `seed-tts-2.0-expressive` supports them but can be less stable. |
| `speaker` | string | yes | empty | Voice ID. |
| `audio_params` | object | yes | empty | Output audio parameters. |
| `additions` | JSON string | no | empty | User custom parameters. |
| `mix_speaker` | object | no | empty | Mix speaker configuration for TTS 1.0 voices. |

`audio_params` fields:

| Field | Type | Default | Notes |
| --- | --- | --- | --- |
| `format` | string | `mp3` | `mp3`, `ogg_opus`, `pcm`; upstream accepts `wav`, but streaming `wav` can return repeated WAV headers, so `pcm` is recommended. |
| `sample_rate` | number | `24000` | One of `8000`, `16000`, `22050`, `24000`, `32000`, `44100`, `48000`. |
| `bit_rate` | number | service default | MP3/OGG bitrate. Default range is 64k to 160k. With `additions.disable_default_bit_rate=true`, lower than 64k can be set. Upstream recommends setting `bit_rate` for MP3/OGG to avoid quality loss. |
| `emotion` | string | empty | Emotional voice parameter for supported voices. |
| `emotion_scale` | number | `4` | Emotion intensity after setting `emotion`; range 1 to 5. |
| `speech_rate` | number | `0` | Range `[-50,100]`; `100` is 2x, `-50` is 0.5x. |
| `loudness_rate` | number | `0` | Range `[-50,100]`; mix voices do not support this. |
| `enable_timestamp` | bool | `false` | Returns word timestamps with `TTSSentenceEnd`; only TTS 1.0 and ICL 1.0. |
| `enable_subtitle` | bool | `false` | Adds `TTSSubtitle` events; only TTS 2.0 and ICL 2.0. Chinese and English only; not supported with LaTeX or SSML. |

`additions` fields:

| Field | Type | Default | Notes |
| --- | --- | --- | --- |
| `silence_duration` | number | `0` | Adds trailing silence at the end of the input text, range 0 to 30000 ms. |
| `enable_language_detector` | bool | `false` | Automatic language detection. |
| `disable_markdown_filter` | bool | `false` | `true` parses and removes Markdown syntax, for example `**hello**` is read as `hello`; `false` keeps raw characters. |
| `disable_emoji_filter` | bool | `false` | Keeps emoji display in text; upstream recommends using it with timestamp parameters. |
| `mute_cut_threshold` | string | empty | Silence threshold; used with `mute_cut_remain_ms`. |
| `mute_cut_remain_ms` | string | empty | Silence length to retain. Unsupported by TTS 2.0 and ICL 2.0. MP3 can retain up to about 100 ms leading silence due to format constraints. |
| `enable_latex_tn` | bool | `false` | Enables LaTeX formula reading; requires `disable_markdown_filter=true`. |
| `latex_parser` | string | empty | `v2` enables stronger LID-based LaTeX parsing; requires `disable_markdown_filter=true`. |
| `max_length_to_filter_parenthesis` | int | `100` | `0` means do not filter parenthesized text; `100` means filter it. |
| `explicit_language` | string | empty | Explicit synthesis language; see language table below. |
| `context_language` | string | empty | Reference language for the model. |
| `explicit_dialect` | string | empty | Explicit dialect; currently `zh_female_vv_uranus_bigtts` supports `dongbei`, `shaanxi`, `sichuan`. |
| `unsupported_char_ratio_thresh` | float | `0.3` | Returns an error if unsupported characters exceed this ratio; max `1.0`. |
| `aigc_watermark` | bool | `false` | Adds rhythm marker at the end of generated audio. |
| `aigc_metadata` | object | empty | Hidden metadata watermark in audio header; supports `mp3`, `wav`, `ogg_opus`. |
| `cache_config` | object | empty | Enables cache for identical text; cache retention is 1 hour. Cached data does not include timestamps. |
| `post_process` | object | empty | Post-processing configuration. |
| `context_texts` | string list | `null` | Voice instructions for TTS 2.0 voices and expressive ICL 2.0. Only the first string currently takes effect. Not billed. |
| `section_id` | string | empty | Multi-turn session ID for serial synthesis in the same context; TTS 2.0 and ICL 2.0 only. |
| `use_tag_parser` | bool | `false` | Enables COT voice-tag parsing; expressive ICL 2.0 only. Single sentence text is recommended to be under 64 chars including tags. |
| `disable_default_bit_rate` | bool | `false` | Allows `bit_rate` below the default range. |

`aigc_metadata` fields:

| Field | Type | Default | Notes |
| --- | --- | --- | --- |
| `enable` | bool | `false` | Enables hidden metadata watermark. |
| `content_producer` | string | empty | Synthesis service provider name or code. |
| `produce_id` | string | empty | Content production ID. |
| `content_propagator` | string | empty | Propagation service provider name or code. |
| `propagate_id` | string | empty | Propagation ID. |

`cache_config` fields:

| Field | Type | Default | Notes |
| --- | --- | --- | --- |
| `text_type` | int | `1` | Use with cache switches. |
| `use_cache` | bool | `true` when enabled | Reads cached audio for identical full text. |
| `use_segment_cache` | bool | `true` when enabled | Segment-level cache; lower first-packet latency than full `use_cache` in bidirectional streaming. |

`post_process` fields:

| Field | Type | Default | Notes |
| --- | --- | --- | --- |
| `pitch` | int | `0` | Pitch range `[-12,12]`. |

Explicit language values:

| Scenario | Values / notes |
| --- | --- |
| Premium voices and ICL 1.0 | Empty means normal Chinese-English mixed reading. `crosslingual` enables multilingual front-end for `zh`, `en`, `ja`, `es-mx`, `id`, `pt-br`. Explicit values include `zh-cn`, `en`, `ja`, `es-mx`, `id`, `pt-br`. |
| DIT clone `model_type=2` | Recommended to set an explicit language. Empty or `zh,en,ja,es-mx,id,pt-br,de,fr` enables multilingual front-end. Explicit values include `zh-cn`, `en`, `ja`, `es-mx`, `id`, `pt-br`, `de`, `fr`. |
| DIT clone `model_type=3` | Explicit language is required. Supported: empty normal Chinese-English mix, `zh-cn`, `en`. |
| ICL 2.0 `model_type=4` | Non-Chinese/English synthesis must set explicit language, and synthesis text plus original clone prompt must match that language. Supported: empty normal Chinese-English mix, `zh-cn`, `en`. |
| ICL 2.0 `model_type=5` | Supported: empty normal Chinese-English mix, `zh-cn`, `en`, `ja`, `es-mx`, `id`, `pt-br`, `ko`. |

`context_language` values:

| Value | Meaning |
| --- | --- |
| empty | Western European languages use English as reference. |
| `id` | Western European languages use Indonesian as reference. |
| `es` | Western European languages use Mexican Spanish as reference. |
| `pt` | Western European languages use Brazilian Portuguese as reference. |

`explicit_dialect` notes:

- Supported dialects for `zh_female_vv_uranus_bigtts`: `dongbei`,
  `shaanxi`, `sichuan`.
- Passing a dialect with an incompatible voice or incompatible
  `explicit_language` returns a parameter error.

`context_texts` examples:

```json
{
  "context_texts": ["Can you speak more slowly?"]
}
```

```json
{
  "context_texts": ["Can you speak with a very painful tone?"]
}
```

`use_tag_parser` example:

```text
<cot text=urgent>Work takes most of life</cot>, and only great work brings satisfaction.
```

## Mix Speaker

`mix_speaker` is supported only for TTS 1.0 voices.

| Field | Type | Notes |
| --- | --- | --- |
| `mix_speaker.speakers` | list | Up to 3 source speakers. Use `req_params.speaker=custom_mix_bigtts`. |
| `mix_speaker.speakers[].source_speaker` | string | TTS 1.0 voice or voice-clone ID. For voice clone, use `S_` IDs or queried `icl_` IDs; `DiT_` and `saturn_` IDs are not supported. |
| `mix_speaker.speakers[].mix_factor` | float | Influence factor. All factors must sum to 1. |

Upstream warns that mixing voices with very different styles, such as male and
female voices at `0.5/0.5`, can occasionally produce unstable switching.

Single-speaker example:

```json
{
  "user": {
    "uid": "12345"
  },
  "event": 100,
  "req_params": {
    "text": "hello",
    "speaker": "zh_female_shuangkuaisisi_moon_bigtts",
    "audio_params": {
      "format": "mp3",
      "sample_rate": 24000
    }
  }
}
```

Mix-speaker example:

```json
{
  "user": {
    "uid": "12345"
  },
  "req_params": {
    "text": "hello",
    "speaker": "custom_mix_bigtts",
    "audio_params": {
      "format": "mp3",
      "sample_rate": 24000
    },
    "mix_speaker": {
      "speakers": [
        {
          "source_speaker": "zh_male_bvlazysheep",
          "mix_factor": 0.3
        },
        {
          "source_speaker": "BV120_streaming",
          "mix_factor": 0.3
        },
        {
          "source_speaker": "zh_male_ahu_conversation_wvae_bigtts",
          "mix_factor": 0.4
        }
      ]
    }
  }
}
```

## Response Frames

Handshake:

- success: HTTP status 200
- failure: non-200 status; body contains failure reason

WebSocket text frames generally report exceptional errors.

Normal binary response payload fields:

| Field | Type | Notes |
| --- | --- | --- |
| `data` | bytes | Binary audio data. |
| `event` | number | Event code. |
| `res_params.text` | string | Sentence after text segmentation. |

Error frames use message type `0b1111`, JSON serialization, no compression, and
carry an error code plus error-message object.

## Events

| Code | Event | Type | Direction | Notes |
| ---: | --- | --- | --- | --- |
| `1` | `StartConnection` | connection | uplink | Declare WebSocket connection creation after HTTP upgrade. |
| `2` | `FinishConnection` | connection | uplink | End connection. |
| `50` | `ConnectionStarted` | connection | downlink | Connection started. |
| `51` | `ConnectionFailed` | connection | downlink | Connection failed. |
| `52` | `ConnectionFinished` | connection | downlink | Connection ended. |
| `100` | `StartSession` | session | uplink | Create session. |
| `101` | `CancelSession` | session | uplink | Cancel session. |
| `102` | `FinishSession` | session | uplink | Finish session. |
| `150` | `SessionStarted` | session | downlink | Session started. |
| `151` | `SessionCanceled` | session | downlink | Session canceled. |
| `152` | `SessionFinished` | session | downlink | Session finished; can carry usage when requested. |
| `153` | `SessionFailed` | session | downlink | Session failed. |
| `200` | `TaskRequest` | data | uplink | Send synthesis text/content. |
| `350` | `TTSSentenceStart` | data | downlink | TTS sentence starts. |
| `351` | `TTSSentenceEnd` | data | downlink | TTS sentence ends. |
| `352` | `TTSResponse` | data | downlink | TTS audio data. |

`CancelSession` notes:

- Cancels the current session and releases server resources.
- Best sent after `SessionStarted` and before `FinishSession`.
- After receiving `SessionCanceled`, create a new session before synthesizing
  more text.

`SessionFinished`, `SessionFailed`, and `SessionCanceled` response metadata can
contain `status_code` and `message`. `SessionFinished` can also include:

```json
{
  "usage": {
    "text_words": 4
  }
}
```

when `X-Control-Require-Usage-Tokens-Return` is active.

## Timestamp And Subtitle Differences

| Aspect | TTS 1.0 / ICL 1.0 | TTS 2.0 / ICL 2.0 |
| --- | --- | --- |
| Enabling field | `audio_params.enable_timestamp=true` | `audio_params.enable_subtitle=true` |
| Events | Multiple `TTSSentenceStart` and `TTSSentenceEnd` events for multiple clauses. Subtitle/timestamp follows `TTSSentenceEnd`. | One `TTSSentenceStart` and `TTSSentenceEnd`; multiple `TTSSubtitle` events when subtitle is enabled. |
| Return timing | Next sentence audio begins after the previous sentence timestamp returns. | Subtitle recognition does not block synthesis. A subtitle can arrive after audio for a later sentence has already begun. |
| Word basis | `words[].word` is based on TN text. | `words[].word` is based on original text. |
| Languages | Chinese and English only; not small languages or dialects. | Chinese and English only; not small languages or dialects. |
| LaTeX | `enable_latex_tn=true` can still return subtitles. | `enable_latex_tn=true` returns no subtitle and no API error. |
| SSML | Non-empty `req_params.ssml` can return subtitles. | Non-empty `req_params.ssml` returns no subtitle and no API error. |

Timestamp/subtitle payload shape:

```json
{
  "phonemes": [],
  "text": "2019年1月8日，软件2.0版本发布。",
  "words": [
    {
      "confidence": 0.8766515,
      "startTime": 0.155,
      "endTime": 0.295,
      "word": "二"
    }
  ]
}
```

## Frame Examples

Connection event frames use `Full-client request` or `Full-server response` with
event number. `StartConnection` and `FinishConnection` payloads are `{}`.

Session event frames include the session ID:

- `StartSession`: JSON payload contains `user` and `req_params`.
- `FinishSession`: JSON payload is `{}`.
- `CancelSession`: JSON payload is `{}`.
- `SessionStarted`: JSON payload is `{}`.
- `SessionFinished`: JSON payload contains response metadata.

Data frames:

- Audio-only request with event: message type `0b0010`, flag `0b0100`,
  serialization raw, includes `Event_TaskRequest`, session ID, and audio bytes.
- Text request with event: message type `0b0001`, flag `0b0100`,
  serialization JSON, includes `Event_TaskRequest`, session ID, and text JSON.

The upstream document marks audio-only frames without an event number as
deprecated/struck through.

## Error Codes

| Code | Meaning |
| ---: | --- |
| `20000000` | Success. |
| `45000000` | Generic client error. |
| `45000001` | Invalid request parameter. |
| `55000000` | Generic server error. |
| `55000001` | Server session error. |

## Examples

HTTP Chunked:

```bash
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/tts_v2/http_stream
```

WebSocket bidirectional:

```bash
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/tts_v2/websocket
```
