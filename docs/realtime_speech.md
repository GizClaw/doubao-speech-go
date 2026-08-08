# Realtime Speech

Official sources:

- Realtime wire/API contract: <https://www.volcengine.com/docs/6561/1594356>
- Dated capability updates: <https://www.volcengine.com/docs/6561/162929>
- Evolving voice inventory: <https://www.volcengine.com/docs/6561/1257544>

This page maps the VolcEngine end-to-end realtime speech API to the current SDK
surface. This is the numeric-event, binary-frame realtime API exposed as
`Client.Realtime`.

The field inventory below is based on the upstream Markdown reference for the
Realtime API. Keep this document as the local source of truth when deciding
which upstream fields should become typed SDK fields.

## Endpoint

```text
wss://openspeech.bytedance.com/api/v3/realtime/dialogue
```

Create the SDK client with an App ID config value and API-key authentication.
The API key is the only authentication factor; App ID is request/application
configuration for services that still require it.

```go
client := doubaospeech.NewClient(appID,
	doubaospeech.WithAPIKey(apiKey),
	doubaospeech.WithResourceID(doubaospeech.ResourceRealtime),
)
```

SDK request headers:

```http
X-Api-App-Id: <app-id>
X-Api-Key: <api-key>
X-Api-Resource-Id: volc.speech.dialog
```

`WithResourceID` is the only resource-ID owner for this API. The deprecated
`RealtimeConfig.ResourceID` cannot affect an already-open handshake and a
non-empty value is rejected instead of being silently ignored.

The handshake response can include `X-Tt-Logid`; log it for support/debugging.

## SDK Coverage

Implemented by `Realtime.OpenSession`.

The SDK supports:

- `StartConnection`
- `StartSession`
- streaming audio input through `TaskRequest`
- text input through `ChatTextQuery`
- push-to-talk end signal through `EndASR`
- client interrupt through `ClientInterrupt`
- direct TTS text injection through `ChatTTSText`
- config update through `UpdateConfig`
- external RAG text through `ChatRAGText`
- server-side conversation create/update/retrieve/truncate/delete helpers
- parsed ASR, chat, TTS, usage, error, and session events
- local history, prompt, and generation-props snapshots for future text turns

The SDK exposes upstream provider fields as typed structs. It does not accept
arbitrary `map[string]any` request passthrough fields.

## Model Families

The upstream model families differ by capability:

| Capability | O | O 2.0 | SC | SC 2.0 |
| --- | --- | --- | --- | --- |
| Premium voices `vv`, `xiaohe`, `yunzhou`, `xiaotian` | yes | yes | no | no |
| Configurable system prompt | yes | yes | yes | yes |
| Clone voices beginning with `ICL_` or `S_` | no | no | yes | no |
| Clone voices 2.0 beginning with `saturn_` or `S_` | no | yes | no | yes |
| Maximum context length | not listed | 12K | not listed | 12K |

Current typed SDK model constants:

| Model family | SDK constant | Upstream value |
| --- | --- | --- |
| O 2.0 | `RealtimeModelO20` | `1.2.1.1` |
| SC 2.0 | `RealtimeModelSC20` | `2.2.0.0` |

The SDK normalizes common older aliases before sending `StartSession`. A model
must be selected explicitly; there is no empty-model or inferred-family path.

`RealtimeConfig.Instructions` is the canonical SDK-owned initial persona/system
instruction. It is semantic and never appears as a literal JSON key:

| Selected model | Canonical wire target | Rejected opposite-family fields |
| --- | --- | --- |
| O20 / `1.2.1.1` | `dialog.system_role` | `dialog.character_manifest` |
| SC20 / `2.2.0.0` | `dialog.character_manifest` | `dialog.bot_name`, `dialog.system_role`, `dialog.speaking_style` |

An identical exact target is accepted idempotently. Conflicting values are
rejected before a frame is written. `speaking_style` remains an independent O20
control and is never populated from `Instructions`.

Upstream model notes:

- O means Omni, the multimodal route.
- SC means Strong Character, optimized for role-play and personified
  expression.
- O and SC base versions are no longer independently maintained; capabilities
  are converging on O 2.0 and SC 2.0.
- O/O 2.0 prompt fields: `bot_name`, `system_role`, and `speaking_style`.
- SC/SC 2.0 prompt field: `character_manifest`.
- O 2.0 improves reasoning, speech understanding/generation, singing, and
  audio-level hotfixes for TN/transcription/pronunciation issues.
- SC 2.0 improves role expression, character-control instructions, voice clone
  stability/similarity, and audio-level hotfixes.

## Audio Requirements

Client PCM input:

- format: PCM, uncompressed
- channel: mono
- sample rate: 16 kHz
- sample width: signed `int16`
- byte order: little endian
- recommended packet cadence: 20 ms per audio packet
- 16 kHz mono s16le 20 ms packet size: 640 bytes

The upstream service also supports microphone input in Opus form. The service
converts it to PCM internally:

```json
{
  "asr": {
    "audio_info": {
      "format": "speech_opus",
      "sample_rate": 16000,
      "channel": 1
    }
  }
}
```

Realtime TTS PCM output is 24 kHz mono. `pcm` means 32-bit float little-endian;
`pcm_s16le` means signed 16-bit little-endian. The SDK default is 24 kHz mono
`pcm_s16le`:

```json
{
  "tts": {
    "audio_config": {
      "channel": 1,
      "format": "pcm_s16le",
      "sample_rate": 24000
    }
  }
}
```

## Voice Configuration

Set the voice through `tts.speaker`.

```json
{
  "tts": {
    "speaker": "zh_male_xiaotian_jupiter_bigtts"
  }
}
```

O/O 2.0 voices listed by upstream:

| Voice ID | Notes |
| --- | --- |
| `zh_female_vv_jupiter_bigtts` | vv, lively female voice. |
| `zh_female_xiaohe_jupiter_bigtts` | xiaohe, sweet lively female voice with Taiwan accent. |
| `zh_male_yunzhou_jupiter_bigtts` | yunzhou, clear steady male voice. |
| `zh_male_xiaotian_jupiter_bigtts` | xiaotian, clear magnetic male voice. |
| `en_male_tim_uranus_bigtts` | Tim, US English, O 2.0 only. |
| `en_female_dacey_uranus_bigtts` | Dacey, US English, O 2.0 only. |
| `en_female_stokie_uranus_bigtts` | Stokie, US English, O 2.0 only. |

SC 2.0 voice list is maintained in the official voice-list document:
<https://www.volcengine.com/docs/6561/1257544>

Upstream custom clone voice notes:

- SC purchases are under the end-to-end realtime speech product.
- SC 2.0 purchases are under Voice Clone 2.0 and can be used by TTS and
  Realtime.
- Purchased clone voices may take around two minutes to become usable.
- Register SC clone voices with `Resource-Id: seed-icl-1.0`.
- Register SC 2.0 clone voices with `Resource-Id: seed-icl-2.0` and
  `model_type: 4`.
- Chinese is the best-supported language for end-to-end clone voices.
- Providing training-audio text is strongly recommended; it can improve clone
  quality and can be brought into the system prompt during synthesis.

## Quotas

Upstream quota terms:

- QPM: query per minute. One query corresponds to one `StartSession` event.
  Default: 60 QPM per AppID.
- TPM: tokens per minute across all consumed tokens. Default: 100,000 TPM.

## Input Modes

The upstream API supports microphone, muted microphone keep-alive, push-to-talk,
text input, and audio-file input.

`RealtimeInputMode` is serialized to `dialog.extra.input_mod`.

| Mode | SDK value | Upstream `input_mod` | Notes |
| --- | --- | --- | --- |
| Server VAD microphone | `RealtimeInputModeDefault` | omitted | Stream microphone audio; no extra silence required. |
| Muted microphone keep-alive | `RealtimeInputModeKeepAlive` | `keep_alive` | Use when the mic can be muted and no audio reaches the server. |
| Push-to-talk | `RealtimeInputModePushToTalk` | `push_to_talk` | Client must send `EndASR` when audio input ends. |
| Text input | `RealtimeInputModeText` | `text` | Service pads silence internally for stream alignment. |
| Audio file | `RealtimeInputModeAudioFile` | `audio_file` | Send the recording as timed stream chunks; 20 ms cadence recommended. |

Upstream examples:

```json
{
  "dialog": {
    "extra": {
      "input_mod": "keep_alive"
    }
  }
}
```

```json
{
  "dialog": {
    "extra": {
      "input_mod": "push_to_talk"
    }
  }
}
```

```json
{
  "dialog": {
    "extra": {
      "input_mod": "text"
    }
  }
}
```

```json
{
  "dialog": {
    "extra": {
      "input_mod": "audio_file"
    }
  }
}
```

After `FinishSession`, the service no longer returns events for that session,
but the WebSocket connection can be reused by sending another `StartSession`.
If the connection should not be reused, send `FinishConnection` and close the
socket.

The upstream document recommends carrying event and session IDs in the binary
optional fields to reduce client-side state complexity.

## Binary Protocol

The Realtime API uses a binary WebSocket protocol composed of:

- 4-byte header
- optional fields
- payload size
- payload

Header fields:

| Byte | Bits | Meaning |
| --- | --- | --- |
| 0 | left 4 bits | protocol version, currently `0b0001`. |
| 0 | right 4 bits | header size, currently `0b0001` for 4 bytes. |
| 1 | left 4 bits | message type. |
| 1 | right 4 bits | message-type-specific flags. |
| 2 | left 4 bits | serialization: `0b0000` raw, `0b0001` JSON. |
| 2 | right 4 bits | compression: `0b0000` none, `0b0001` gzip. |
| 3 | all bits | reserved, `0x00`. |

Message types:

| Bits | Meaning |
| --- | --- |
| `0b0001` | Full-client request for text events. |
| `0b1001` | Full-server response for text events. |
| `0b0010` | Audio-only client request. |
| `0b1011` | Audio-only server response. |
| `0b1111` | Error information. |

Optional fields:

| Field | Length | Notes |
| --- | --- | --- |
| `code` | 4 bytes | Error code, only for error frames. |
| `sequence` | 4 bytes | Optional client event sequence. |
| `event` | 4 bytes | Required for state-management events. |
| `connect id size` | 4 bytes | Only connection events can carry connect ID. |
| `connect id` | variable | Caller-generated connect ID. |
| `session id size` | 4 bytes | Session-level events carry session ID. |
| `session id` | variable | Caller-generated session ID. |

Error-frame payload:

```json
{
  "error": "error message"
}
```

## StartSession Request Surface

`StartSession` is event `100`.

Current SDK typed request coverage:

| JSON path | SDK field | Notes |
| --- | --- | --- |
| `asr.language` | `RealtimeASRConfig.Language` | SDK extension; not shown in the pasted upstream field table. |
| `tts.speaker` | `RealtimeTTSConfig.Speaker` | Required; compatibility with the selected model/account is documentation-only. |
| `tts.audio_config.channel` | `RealtimeAudioConfig.Channel` | Defaults to `1`. |
| `tts.audio_config.format` | `RealtimeAudioConfig.Format` | Defaults to `pcm_s16le`. |
| `tts.audio_config.sample_rate` | `RealtimeAudioConfig.SampleRate` | Defaults to `24000` for TTS output. |
| `tts.audio_config.bits` | `RealtimeAudioConfig.Bits` | Defaults to `16`. |
| `tts.audio_config.speech_rate` | `RealtimeAudioConfig.SpeechRate` | Range `[-50,100]`. |
| `tts.audio_config.loudness_rate` | `RealtimeAudioConfig.LoudnessRate` | Range `[-50,100]`. |
| `dialog.dialog_id` | `RealtimeDialogConfig.DialogID` | Loads recent conversation records for context continuity. |
| `dialog.bot_name` | `RealtimeDialogConfig.BotName` | O/O 2.0 only. Max 20 characters upstream. |
| `dialog.system_role` | `RealtimeConfig.Instructions` / `RealtimeDialogConfig.SystemRole` | Canonical/escape-hatch O20 target. |
| `dialog.speaking_style` | `RealtimeDialogConfig.SpeakingStyle` | O/O 2.0 only. |
| `dialog.character_manifest` | `RealtimeConfig.Instructions` / `RealtimeDialogConfig.CharacterManifest` | Canonical/escape-hatch SC20 target. |
| `dialog.extra.input_mod` | `RealtimeConfig.InputMode` | Mapped from `RealtimeInputMode`. |
| `dialog.extra.model` | `RealtimeConfig.Model` | Mandatory; normalized and retained on the session. |
| `prompt.system` | `RealtimePromptConfig.System` | Compatibility extension serialized by the SDK; not the documented System Prompt contract. |
| `prompt.variables` | `RealtimePromptConfig.Variables` | Compatibility extension serialized when non-empty. |
| `props.*` | `RealtimeGenerationProps` | Compatibility extension serialized when non-zero. |
| `history` | `RealtimeConfig.History` | Compatibility extension/local snapshot, not canonical instructions. |

Typed ASR fields:

| JSON path | Type | Notes |
| --- | --- | --- |
| `asr.audio_info.format` | string | Example: `speech_opus`. |
| `asr.audio_info.sample_rate` | int | Example: `16000`. |
| `asr.audio_info.channel` | int | Example: `1`. |
| `asr.extra.end_smooth_window_ms` | int | User stop-speech smoothing window; default 1500 ms; range `[500ms,50s]`. |
| `asr.extra.enable_custom_vad` | bool | Enables custom stop-speech parameters. |
| `asr.extra.enable_asr_twopass` | bool | Enables non-streaming second-pass recognition. |
| `asr.extra.boosting_table_id` | string | Hotword table ID, used with two-pass ASR. |
| `asr.extra.boosting_table_name` | string | Hotword table name, used with two-pass ASR. |
| `asr.extra.regex_correct_table_id` | string | Regex replacement table ID. |
| `asr.extra.regex_correct_table_name` | string | Regex replacement table name. |
| `asr.extra.context.hotwords[].word` | string | Inline custom hotwords. |
| `asr.extra.context.correct_words` | map string to string | Inline text replacement rules. |

Typed dialog fields:

| JSON path | Type | Notes |
| --- | --- | --- |
| `dialog.location.longitude` | float64 | User longitude for web search precision. |
| `dialog.location.latitude` | float64 | User latitude for web search precision. |
| `dialog.location.city` | string | City. |
| `dialog.location.country` | string | Defaults to China upstream. |
| `dialog.location.province` | string | Province. |
| `dialog.location.district` | string | District. |
| `dialog.location.town` | string | Town. |
| `dialog.location.country_code` | string | Defaults to `CN` upstream. |
| `dialog.location.address` | string | Address. |
| `dialog.dialog_context[].role` | string | Initial context role. Must be ordered user/assistant QA pairs. |
| `dialog.dialog_context[].text` | string | Initial context text. |
| `dialog.dialog_context[].timestamp` | int | Current time is filled when empty. |
| `dialog.extra.strict_audit` | bool | Strict safety audit when true; default true. |
| `dialog.extra.audit_response` | string | Custom response when user query hits safety audit. |
| `dialog.extra.enable_volc_websearch` | bool | Enables built-in web search. |
| `dialog.extra.volc_websearch_type` | string | `web`, `web_summary`, or `web_agent`; default `web`. |
| `dialog.extra.volc_websearch_api_key` | string | API key for VolcEngine search/search-agent service. |
| `dialog.extra.volc_websearch_bot_id` | string | Required for `web_agent`. |
| `dialog.extra.volc_websearch_result_count` | int | Max 10; default 10. |
| `dialog.extra.volc_websearch_no_result_message` | string | Message when search has no result. |
| `dialog.extra.enable_music` | bool | Singing capability; applies to model `1.2.1.1`. |
| `dialog.extra.enable_loudness_norm` | bool | Output loudness normalization for 2.0 models; default false. |
| `dialog.extra.enable_conversation_truncate` | bool | Enables context truncation for 2.0 models. |
| `dialog.extra.enable_user_query_exit` | bool | Emits an exit-intent signal in `TTSEnded`; default false. |

Typed TTS extra fields:

| JSON path | Type | Notes |
| --- | --- | --- |
| `tts.extra.explicit_dialect` | string | Only effective for 2.0 vv voice; values include `dongbei`, `sichuan`, `shaanxi`. |
| `tts.extra.aigc_metadata.enable` | bool | Enables hidden AIGC watermark metadata. |
| `tts.extra.aigc_metadata.content_producer` | string | Content producer name or code. |
| `tts.extra.aigc_metadata.produce_id` | string | Content production ID. |
| `tts.extra.aigc_metadata.content_propagator` | string | Content propagation provider name or code. |
| `tts.extra.aigc_metadata.propagate_id` | string | Content propagation ID. |
| `tts.extra.tts_2.0_model` | string | Clone voice effect; high-expression clone voices use `expressive`. |

Validation follows the documented stable capability boundaries:

- `enable_music=true` and non-empty `tts_2.0_model` are O20-only;
- web search type is `web`, `web_summary`, or `web_agent` here;
  `web_global_api` belongs to Realtime Duplex;
- enabled web search requires `volc_websearch_api_key`; `web_agent` also
  requires `volc_websearch_bot_id`;
- `explicit_dialect` accepts `dongbei`, `sichuan`, or `shaanxi` and only takes
  effect on compatible 2.0 `vv` voices;
- speaker IDs stay opaque because public and customer-specific inventories
  evolve independently of the SDK.

## UpdateConfig Request Surface

`UpdateConfig` is event `201`.

The upstream document says this event updates SP-related config during a call and
uses full replacement semantics. Include complete field information when using
it. The SDK exposes this through `RealtimeSession.UpdateConfig`.

Upstream fields include:

- `tts.speaker`
- `tts.audio_config.speech_rate`
- `tts.audio_config.loudness_rate`
- `dialog.bot_name`
- `dialog.system_role`
- `dialog.speaking_style`
- `dialog.dialog_id`
- `dialog.location.longitude`
- `dialog.location.latitude`
- `dialog.location.city`
- `dialog.location.country`
- `dialog.location.province`
- `dialog.location.district`
- `dialog.location.town`
- `dialog.location.country_code`
- `dialog.location.address`

The SDK projects exactly these fields. StartSession-only members such as TTS
format/channel/sample-rate/bits/extra and dialog context/extra/
`character_manifest` are rejected before write. O20 supports the listed live
prompt fields; SC20 rejects them because event `201` has no documented SC
instruction-replacement target.

## Client Events

| Event ID | Name | Event class | SDK support | Notes |
| --- | --- | --- | --- | --- |
| `1` | `StartConnection` | connection | yes | Declares WebSocket connection creation. |
| `2` | `FinishConnection` | connection | yes | Ends the WebSocket connection. |
| `100` | `StartSession` | session | yes | Starts a realtime session; payload includes ASR/TTS/dialog config. |
| `102` | `FinishSession` | session | yes | Ends the current session while leaving the WebSocket reusable. |
| `200` | `TaskRequest` | session | yes | Uploads audio bytes. |
| `201` | `UpdateConfig` | session | yes | Full replacement update for SP-related config. |
| `300` | `SayHello` | session | yes | Sends greeting text. |
| `400` | `EndASR` | session | yes | Required when push-to-talk audio input ends. |
| `500` | `ChatTTSText` | session | yes | Streams caller-provided TTS text. |
| `501` | `ChatTextQuery` | session | yes | Sends text query to the model. |
| `502` | `ChatRAGText` | session | yes | Sends external RAG text; external RAG string is capped at 4K chars. |
| `510` | `ConversationCreate` | context | yes | Appends complete QA-pair context items. |
| `511` | `ConversationUpdate` | context | yes | Updates text by `item_id`; item can be question or reply. |
| `512` | `ConversationRetrieve` | context | yes | Retrieves recent context or an item-specific context record. |
| `513` | `ConversationTruncate` | context | yes | 2.0 only; requires `enable_conversation_truncate`, `item_id`, and `audio_end_ms`. |
| `514` | `ConversationDelete` | context | yes | Deletes whole dialogue turns. |
| `515` | `ClientInterrupt` | session | yes | Interrupts server response in push-to-talk mode. |

Session IDs are carried only by the binary frame envelope. Event JSON is built
per operation:

| Event | Exact JSON fields |
| --- | --- |
| `102` | none (`{}`) |
| `201` | only the UpdateConfig fields listed above |
| `300` | `content` |
| `400` | none (`{}`), push-to-talk only |
| `500` | `start`, `content`, `end` |
| `501` | `content`, plus retained compatibility `history`/`prompt`/`props` when set |
| `502` | `external_rag` (opaque string, maximum 4,000 Unicode characters) |
| `510` | `items[].role`, `items[].text`, optional `items[].timestamp` |
| `511` | `items[].item_id`, `items[].text` |
| `512` | no `items` for latest, otherwise `items[].item_id` |
| `513` | `item_id`, `audio_end_ms`; requires StartSession truncation opt-in |
| `514` | `items[].item_id` |
| `515` | none (`{}`), push-to-talk only |

Event `510` accepts at most 40 items in complete alternating user/assistant
pairs. Timestamps must be all present or all omitted; supplied values must be
strictly increasing and not in the future. Event `511` requires non-empty item
ID and text. Per-event projection prevents fields from the shared public item
type leaking into another operation.

`ChatTTSText` frame sequence:

```json
{
  "start": true,
  "content": "",
  "end": false
}
```

```json
{
  "start": false,
  "content": "text to synthesize",
  "end": false
}
```

```json
{
  "start": false,
  "content": "",
  "end": true
}
```

If a new user query interrupts playback before the final end packet is sent,
the upstream document says the final packet does not need to be sent.

`ChatRAGText.external_rag` is expected to be a JSON-array string with entries
like the following. The SDK enforces only the character bound and does not
parse or reformat the caller-owned string:

```json
{
  "title": "document title",
  "content": "document content"
}
```

Conversation item payloads use:

```json
{
  "items": [
    {
      "role": "user",
      "text": "message text",
      "timestamp": 0
    }
  ]
}
```

Conversation update/retrieve/delete events use `item_id` where needed.

## Server Events

| Event ID | Name | SDK parsing | Notes |
| --- | --- | --- | --- |
| `50` | `ConnectionStarted` | metadata/raw payload | Connection created. |
| `51` | `ConnectionFailed` | error payload | Connection failed. |
| `52` | `ConnectionFinished` | metadata/raw payload | Connection ended. |
| `150` | `SessionStarted` | session `DialogID()` | Binary `SessionID()` and payload `dialog_id` remain distinct. |
| `152` | `SessionFinished` | final event | Session ended. |
| `153` | `SessionFailed` | error payload | Session failed. |
| `154` | `UsageResponse` | `RealtimeUsage` | Per-turn usage data. |
| `251` | `ConfigUpdated` | raw payload | Ack for `UpdateConfig`. |
| `350` | `TTSSentenceStart` | text, IDs, `TTSType` | `tts_type` can be `audit_content_risky`, `chat_tts_text`, `network`, `external_rag`, `sing`, or `default`. |
| `351` | `TTSSentenceEnd` | question/reply IDs | End of one synthesized sentence. |
| `352` | `TTSResponse` | audio bytes | Server audio payload. |
| `359` | `TTSEnded` | status code, IDs | `status_code="20000002"` indicates detected user exit intent. |
| `450` | `ASRInfo` | text/final | First ASR signal, useful to interrupt local playback. |
| `451` | `ASRResponse` | `RealtimeASRResult` | Recognition text and interim/final state. |
| `459` | `ASREnded` | final event | User speech ended. |
| `550` | `ChatResponse` | text, IDs | Model response text. |
| `553` | `ChatTextQueryConfirmed` | question ID | Ack for `ChatTextQuery`. |
| `559` | `ChatEnded` | question/reply IDs | Model response text ended. |
| `567` | `ConversationCreated` | `RealtimeEvent.Items` | Returns created context items. |
| `568` | `ConversationUpdated` | raw payload | Returns `{}` on success or a missing-item message on failure. |
| `569` | `ConversationRetrieved` | `RealtimeEvent.Items` | Returns context items. |
| `570` | `ConversationTruncated` | raw payload | Ack for truncation. |
| `571` | `ConversationDeleted` | items or nonfatal `Error` | Returns deleted items; non-success status/message remains observable. |
| `599` | `DialogCommonError` | error/status payload | Realtime dialogue error. |

The upstream document notes that server JSON payloads may contain extra fields
that clients do not need to handle.

Events `51` and `153` map provider `error` text to `CodeServerError` and are
terminal. Protocol error frames are also terminal. Events `571` and `599` map
`status_code`/`message` to `RealtimeEvent.Error` but remain normal observable
events, so the receive loop stays live. Numeric statuses populate `Error.Code`;
non-numeric statuses remain in `RealtimeEvent.StatusCode` and use
`CodeServerError`. No current O20/SC20-specific server-event schema difference
is documented.

## Error Codes

| Code | Message / keyword | Meaning |
| --- | --- | --- |
| `42000020` | `StartSession event payload asr extra is null` | `asr.extra` was sent as null. |
| `42000020` | `StartSession event payload tts extra is null` | `tts.extra` was sent as null. |
| `42000020` | `dialog.extra.model= ? cant support enable_music=true` | `enable_music` was used with an unsupported model; it applies to `1.2.1.1`. |
| `42000020` | `volc_websearch_bot_id is required` | `web_agent` search mode requires `volc_websearch_bot_id`. |
| `42000020` | `volc_websearch_api_key is required` | Web search modes require `volc_websearch_api_key`. |
| `45000003` | `Abnormal silence audio` | More than 10 minutes without dialogue interaction; service releases the connection. |
| `50000000` | `AudioQueryError` | Model chat inference error. |
| `50000000` | `Yaml: line 43: found unknown escape character` | Usually invalid escaping in `speaking_style` or `system_role`. |
| `55000001` | `ServerError` | Model chat inference error. |
| `55000001` | `ContextCanceled` | Client did not normally send `FinishSession`; upstream strongly recommends finishing before closing. |
| `55000001` | `ClientError:InvalidSpeaker` | Speaker and model version do not match. |
| `55000001` | `ExceededConcurrentDurationLimit` | Model chat inference error / duration limit. |
| `52000042` | `DialogAudioIdleTimeoutError` | Silence padding failed; upstream recommends `dialog.extra.input_mod=keep_alive`. |
| `50700000` | `CallWithTimeout: stream recv timeout` | Model chat timeout. |
| `50000000` | `ServerError:BigASRFailedCode:1022` | Model chat / ASR failure. |
| `52000022` | `AudioChatError` | Model chat error. |
| `52000035` | `S2SQueryConnectError` | Model chat connection error. |
| `52000016` | `AudioTTSIdleTimeoutError` | TTS synthesis timeout. |
| `52000011` | `AudioChatRecvTimeoutError` | Model text response timeout. |

The upstream document recommends reconnecting on server 5xx errors.

## Example

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/realtime -mode text -model 1.2.1.1 -speaker <o20-speaker> \
  -instructions '只回答：收到。' -expect-response '收到'
```
