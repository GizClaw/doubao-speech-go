# Realtime Duplex

Official documentation: <https://www.volcengine.com/docs/6561/2549778>

This page maps the VolcEngine Realtime Speech Model 3.0 full-duplex
end-to-end speech API to the current SDK surface. The upstream product is also
called Seeduplex. It provides lower-latency, more human-like voice dialogue over
WebSocket text JSON events.

Read the upstream integration guide before production integration:
<https://www.volcengine.com/docs/6561/2549732>

## Demo Attachments

Upstream demo attachments:

| Language | Attachment |
| --- | --- |
| Go | `go1.24_duplex_demo.zip` |
| Python | `python3.7_duplex_demo.zip` |

Run the SDK example:

```bash
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/realtime_duplex
```

## Endpoint

```text
wss://openspeech.bytedance.com/api/v3/duplex/realtime/dialogue
```

The required upstream model value is:

```text
1.2.6.0
```

## API Difference

| SDK API | Endpoint | Wire format | Event identity |
| --- | --- | --- | --- |
| `Client.Realtime` | `/api/v3/realtime/dialogue` | ByteDance binary frames | numeric event IDs |
| `Client.RealtimeDuplex` | `/api/v3/duplex/realtime/dialogue` | WebSocket text JSON | string event types |

## Authentication

API-key authentication uses:

```http
X-Api-Key: <api-key>
```

The SDK also sends `X-Api-App-Id` when the client was constructed with an app
ID. App ID is request/application configuration, not an authentication factor.

The WebSocket handshake response can include:

| Header | Meaning |
| --- | --- |
| `X-Tt-Logid` | Server log ID. Include this value when asking VolcEngine support to locate a problem. |

The SDK exposes the handshake log ID through `RealtimeDuplexSession.LogID()`.

## SDK Coverage

Implemented by `RealtimeDuplex.OpenSession`.

The SDK supports:

- `session.create`
- `session.update`
- `session.close`
- `input_audio_buffer.append`
- `input_audio_buffer.commit`
- `speech_text_buffer.append`
- `speech_text_buffer.commit`
- `speech_text_buffer.replacement.append`
- `speech_text_buffer.replacement.commit`
- `response.cancel`
- conversation item create, update, retrieve, and delete
- function-call result submission through `conversation.item.create`
- parsed session, ASR, text, audio, context, function-call, usage, and error events

## Session Create And Update

Use `session.create` to create a session and `session.update` to update it.
Both events carry a `session` object.

| JSON path | Type | Required | SDK field | Notes |
| --- | --- | --- | --- | --- |
| `type` | string | yes | internal | `session.create` or `session.update`. |
| `session.id` | string | no | `RealtimeDuplexSessionConfig.ID` | Corresponds to the old `dialog_id`; pass it to continue historical dialogue. SDK generates an ID if omitted. |
| `session.model` | string | yes | `Model` | Fixed upstream value `1.2.6.0`; SDK default is `RealtimeDuplexModelDefault`. |
| `session.instructions` | string | no | `Instructions` | System message for response content and voice style. Model internal SP plus context share 12K. |
| `session.audio` | object | yes | `Audio` | Input and output audio configuration. |
| `session.audio.input.format` | object | yes | `Audio.Input.Format` | Input sample rate must be 16 kHz. Supported types: `pcm`, `speech_opus`. |
| `session.audio.output.format` | object | yes | `Audio.Output.Format` | Output sample rate is fixed at 24 kHz. Supported types: `pcm_s16le`, `ogg_opus`. |
| `session.audio.output.voice` | string | yes upstream | `Audio.Output.Voice` | Voice name. Official voice list also includes clone voices. |
| `session.audio.output.speed` | number | no | `Audio.Output.Speed` | Range `[-50,100]`; `100` is 2x, `-50` is 0.5x. |
| `session.audio.output.loudness` | number | no | `Audio.Output.Loudness` | Range `[-50,100]`; `100` is 2x volume, `-50` is 0.5x. |
| `session.tools` | array | no | `Tools` | Function-calling tool definitions using standard JSON Schema. |
| `extension` | object | no | `Extension` | Model-specific extension fields. See the typed extension section below. |

Supported audio constants:

| Constant | Wire value | Notes |
| --- | --- | --- |
| `RealtimeDuplexAudioPCM` | `pcm` | Input format. |
| `RealtimeDuplexAudioOpus` | `speech_opus` | Input format. |
| `RealtimeDuplexAudioPCMS16LE` | `pcm_s16le` | Output format. |
| `RealtimeDuplexAudioOggOpus` | `ogg_opus` | Output format. |

Minimal SDK config:

```go
cfg := doubaospeech.DefaultRealtimeDuplexConfig()
cfg.Session.Audio.Output.Voice = "zh_female_vv_jupiter_bigtts"
```

## Extension Fields

Upstream describes `extension` as model-specific `asr`, `tts`, and `dialog`
configuration. The public SDK surface is limited to the typed extension fields
listed below so callers can see exactly which provider fields are supported.

Supported extension fields:

| JSON path | Go field | Type | Default | Notes |
| --- | --- | --- | --- | --- |
| `extension.asr.audio_info` | `RealtimeASRConfig.AudioInfo` | object | service default | Input audio metadata such as `pcm` or `speech_opus`. |
| `extension.asr.extra` | `RealtimeASRConfig.Extra` | object | service default | ASR VAD, two-pass recognition, hotwords, and correction rules. |
| `extension.tts.speaker` | `RealtimeTTSConfig.Speaker` | string | service default | TTS speaker override. Prefer Duplex-native `session.audio.output.voice` for normal voice selection. |
| `extension.tts.audio_config` | `RealtimeTTSConfig.AudioConfig` | object | service default | TTS rate/loudness fields that match old Realtime StartSession. |
| `extension.tts.extra` | `RealtimeTTSConfig.Extra` | object | service default | TTS dialect, AIGC metadata, and clone model fields. |
| `extension.dialog.dialog_id` | `RealtimeDuplexDialogExtension.DialogID` | string | generated/session default | Old dialog ID for context continuity. |
| `extension.dialog.location` | `RealtimeDuplexDialogExtension.Location` | object | service default | User location for web search precision. |
| `extension.dialog.dialog_context` | `RealtimeDuplexDialogExtension.DialogContext` | array | empty | Initial dialogue context. |
| `extension.dialog.extra.audit_response` | `RealtimeDuplexDialogExtra.AuditResponse` | `string` | service default | Response text used when dialogue audit blocks an answer. |
| `extension.dialog.extra.enable_loudness_norm` | `RealtimeDuplexDialogExtra.EnableLoudnessNorm` | `*bool` | service default | Enables dialogue output loudness normalization when set. |
| `extension.dialog.extra.enable_music` | `RealtimeDuplexDialogExtra.EnableMusic` | `*bool` | service default | Enables or disables generated background music when set. Use a pointer so explicit `false` is serialized. |
| `extension.dialog.extra.enable_volc_websearch` | `RealtimeDuplexDialogExtra.EnableVolcWebsearch` | `*bool` | service default | Enables built-in web search when set. |
| `extension.dialog.extra.volc_websearch_api_key` | `RealtimeDuplexDialogExtra.VolcWebsearchAPIKey` | string | empty | Search API key. Read it from environment; do not hardcode it. |

`extension.s2s` and top-level `extension.extra` are not implemented. If those
fields are needed later, add typed structs and tests before adding them to the
public SDK surface.

Core Duplex concepts should use Duplex-native fields instead of old Realtime
extras: model and prompt use `session.model` and `session.instructions`; audio
codecs, voice, speed, and loudness use `session.audio`; function calling uses
`session.tools`.

## Upstream Client Events

### Greeting / Speech Text

`speech_text_buffer.commit` sends a complete greeting or direct TTS text.

| Field | Type | Required | SDK API | Notes |
| --- | --- | --- | --- | --- |
| `type` | string | yes | `CommitSpeechText` / `SendSpeechText` | Fixed value `speech_text_buffer.commit`. |
| `event_id` | string | no | `RealtimeDuplexSpeechTextRequest.EventID` | Client-generated event ID for matching and tracing. |
| `text` | string | no | `Text` | Text to synthesize. |

The SDK also supports `speech_text_buffer.append` through `AppendSpeechText` for
streaming speech-text fragments.

### Audio Upload

`input_audio_buffer.append` sends one audio packet as a WebSocket JSON text
frame.

| Field | Type | Required | SDK API | Notes |
| --- | --- | --- | --- | --- |
| `type` | string | yes | `SendAudio` | Fixed value `input_audio_buffer.append`. |
| `event_id` | string | no | generated internally | Client-generated event ID for matching and tracing. |
| `audio` | string | yes | `SendAudio` | Base64-encoded audio bytes. |

`input_audio_buffer.commit` force-commits the input audio buffer. Use it when
the current audio query has clearly ended and the client wants to force endpoint
detection.

| Field | Type | Required | SDK API |
| --- | --- | --- | --- |
| `type` | string | yes | `CommitAudio` |
| `event_id` | string | no | generated internally |

### Response Replacement

Use replacement speech-text events when you do not want the model's chat result
and want to directly specify TTS text.

| Event type | SDK API | Notes |
| --- | --- | --- |
| `speech_text_buffer.replacement.append` | `AppendReplacementSpeechText` | Streaming replacement text. |
| `speech_text_buffer.replacement.commit` | `CommitReplacementSpeechText` | Final replacement packet. |

Both events support:

| Field | Type | Required | SDK field |
| --- | --- | --- | --- |
| `event_id` | string | no | `RealtimeDuplexSpeechTextRequest.EventID` |
| `text` | string | no | `Text` |

### Context Management

Context-management events use `items`.

| Operation | Event type | SDK API | Upstream rule |
| --- | --- | --- | --- |
| Create | `conversation.item.create` | `CreateConversationItems` | Add items to context. Can initialize history or insert function-call results. Up to 20 rounds / 40 complete QA items per request. |
| Update | `conversation.item.update` | `UpdateConversationItems` | Update the text for a specified `item_id`; it can be a `question_id` or `reply_id`. |
| Retrieve | `conversation.item.retrieve` | `RetrieveConversationItems` | Without `item_id`, returns the latest 20 complete dialogue rounds; with `item_id`, returns that round. |
| Delete | `conversation.item.delete` | `DeleteConversationItems` | Deletes by dialogue round. Passing a user-side item deletes the paired assistant reply, and vice versa. |

Conversation item fields exposed by the SDK:

| Field | Type | Notes |
| --- | --- | --- |
| `id` | string | Conversation item ID. |
| `type` | string | For example `message`. |
| `role` | string | `user`, `assistant`, or `tool`. |
| `call_id` | string | Function-call ID for tool results. |
| `status` | string | Item status when returned by the service. |
| `content` | array | Content blocks. |
| `content[].type` | string | For tool results, use `input_text`. |
| `content[].text` | string | Content text. |

### Client Interrupt

`response.cancel` cancels the in-flight response. Use it when the client
interrupts server playback and wants to move to the next recognition turn.

| Field | Type | Required | SDK API |
| --- | --- | --- | --- |
| `type` | string | yes | `CancelResponse` |
| `event_id` | string | no | generated internally |

### Function-Call Result Return

When the service sends `response.function_call_arguments.done`, execute the
local tool and return the result using `conversation.item.create` with
`role=tool`.

| Field | Type | Required | SDK field | Notes |
| --- | --- | --- | --- | --- |
| `type` | string | yes | internal | Fixed value `conversation.item.create`. |
| `items` | array | yes | generated by `SendFunctionCallOutputs` | Tool-result items. |
| `items[].call_id` | string | yes | `RealtimeDuplexFunctionCallOutput.CallID` | Must exactly match the corresponding `call_id` from the service event. |
| `items[].role` | string | yes | fixed by SDK | Fixed value `tool`. |
| `items[].content` | array | yes | generated by SDK | Function output content. |
| `items[].content[].type` | string | yes | fixed by SDK | Fixed value `input_text`. |
| `items[].content[].text` | string | yes | `Output` | Tool execution result. |

### Close Session

`session.close` ends the session.

| Field | Type | Required | SDK API |
| --- | --- | --- | --- |
| `type` | string | yes | `Close` |

## Server Events

Response event type is carried by the `type` field. The SDK parses known events
into `RealtimeDuplexEvent` and preserves the raw JSON payload for unknown
events.

| Category | Event | SDK parsing | Meaning |
| --- | --- | --- | --- |
| Session | `session.created` | `SessionID` | Session started successfully. Returned `session.id` is the old `dialog_id` and can continue recent conversation context. |
| Session | `session.updated` | `SessionID` | Ack for `session.update`. |
| Session | `session.closed` | raw event | Session ended. |
| Audio | `input_audio_buffer.committed` | raw event | Input audio buffer was committed; marks the end of a user audio input. |
| ASR | `conversation.item.input_audio_transcription.started` | `ItemID`, `Delta`, `Transcript` | First recognized audio text event; can be used to interrupt local playback. |
| ASR | `conversation.item.input_audio_transcription.delta` | `ItemID`, `Delta`, `Transcript` | User speech transcription text. |
| ASR | `conversation.item.input_audio_transcription.completed` | `ItemID`, `Delta`, `Transcript` | Model considers user speech ended. |
| ASR | `conversation.item.input_audio_transcription.failed` | raw event | ASR failed. |
| Chat | `response.output_text.delta` | `QuestionID`, `ResponseID`, `Delta`, `Text` | Model response text delta. |
| Chat | `response.output_text.done` | `QuestionID`, `ResponseID`, `Delta`, `Text` | Model response text finished. |
| TTS | `response.output_audio.started` | `QuestionID`, `ResponseID`, `TTSType`, `StatusCode` | TTS output started. |
| TTS | `response.output_audio.delta` | `QuestionID`, `ResponseID`, `Delta`, `Audio` | Base64 audio delta decoded into `Audio`. |
| TTS | `response.output_audio.done` | `QuestionID`, `ResponseID`, `TTSType`, `StatusCode` | TTS finished. `status_code="20000002"` means the model detected user-exit intent. |
| Context | `conversation.item.added` | `ConversationItems` | Ack for context create. |
| Context | `conversation.item.retrieved` | `ConversationItems` | Ack for context retrieve. |
| Context | `conversation.item.updated` | `ConversationItems` | Ack for context update. |
| Context | `conversation.item.deleted` | `ConversationItems` | Ack for context delete. If nothing is deleted, upstream can return `status_code:40000010` and `message:"empty conversation deleted messages"`. |
| FC | `response.function_call_arguments.done` | `FunctionCalls` | Function-call arguments completed. Each item contains `call_id`, `name`, and JSON-string `arguments`. Client must return tool output with the same `call_id`. |
| Usage | `response.done` | `Usage` raw JSON | One interaction finished and returned usage statistics. |
| Usage | `response.canceled` | raw event | Ack for `response.cancel`. |
| Error | `error` | `Error` | Error event. See upstream integration guide for detailed error codes. |

TTS `tts_type` values listed by upstream:

| Value | Meaning |
| --- | --- |
| `audit_content_risky` | Audio returned after content audit risk. |
| `chat_tts_text` | Client-specified text-to-speech audio. |
| `network` | Built-in web-search audio. |
| `default` | Default chat audio. |

Function-call downlink shape:

```json
{
  "type": "response.function_call_arguments.done",
  "items": [
    {
      "call_id": "call-1",
      "name": "lookup_weather",
      "arguments": "{\"city\":\"Beijing\"}"
    }
  ]
}
```

Function-call result uplink shape:

```json
{
  "type": "conversation.item.create",
  "items": [
    {
      "call_id": "call-1",
      "role": "tool",
      "content": [
        {
          "type": "input_text",
          "text": "{\"temperature\":26}"
        }
      ]
    }
  ]
}
```
