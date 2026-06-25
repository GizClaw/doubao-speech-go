# Realtime Duplex Dialogue API Usage

This document describes the Doubao / VolcEngine full-duplex realtime dialogue
WebSocket API exposed by this SDK as `Client.RealtimeDuplex`.

This API is distinct from `Client.Realtime`:

| API | Endpoint | Wire format | Event identity |
| --- | --- | --- | --- |
| `Client.Realtime` | `/api/v3/realtime/dialogue` | ByteDance binary frames | numeric event IDs |
| `Client.RealtimeDuplex` | `/api/v3/duplex/realtime/dialogue` | WebSocket text JSON | string event types |

## Endpoint

```text
wss://openspeech.bytedance.com/api/v3/duplex/realtime/dialogue
```

## Authentication

The current reference sample uses an API key in the WebSocket request header:

```http
X-Api-Key: <api-key>
```

Configure it with:

```go
client := doubaospeech.NewClient("", doubaospeech.WithAPIKey(apiKey))
```

The service can return `X-Tt-Logid` in the WebSocket handshake response. The SDK
preserves it on `RealtimeDuplexSession.LogID()`.

## Quick Start

```go
ctx := context.Background()

client := doubaospeech.NewClient("", doubaospeech.WithAPIKey(apiKey))

cfg := doubaospeech.DefaultRealtimeDuplexConfig()
cfg.Session.Instructions = "You are a concise assistant."
cfg.Session.Audio.Output.Voice = "zh_male_xiaotian_jupiter_bigtts"

session, err := client.RealtimeDuplex.OpenSession(ctx, &cfg)
if err != nil {
	return err
}
defer session.Close()

if err := session.SendAudio(ctx, pcmChunk); err != nil {
	return err
}
if err := session.CommitAudio(ctx); err != nil {
	return err
}

for {
	evt, err := session.RecvEvent(ctx)
	if err != nil {
		return err
	}
	if evt.Type == doubaospeech.RealtimeDuplexEventResponseOutputTextDelta {
		fmt.Print(evt.Delta)
	}
	if len(evt.Audio) > 0 {
		// Persist or play returned audio bytes.
	}
	if evt.Type == doubaospeech.RealtimeDuplexEventResponseOutputAudioDone {
		break
	}
}
```

## Example

The repository includes a multi-service live conversation example. It uses the
existing `Client.Realtime` API to generate a spoken prompt, sends that audio to
`Client.RealtimeDuplex`, handles Duplex function-call events, and uses
`Client.ASRV2` SAUC to transcribe the Duplex output audio for logging:

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_API_KEY=<your_api_key> \
DOUBAO_DUPLEX_API_KEY=<your_duplex_api_key> \
go run ./examples/realtime_duplex -rounds 2 -out-dir /tmp/realtime-duplex-smoke
```

It intentionally does not use PortAudio, microphone capture, playback, or Opus
encoding.

## Session Config

`DefaultRealtimeDuplexConfig()` returns:

- model: `1.2.6.0`
- input audio: `pcm`, 16 kHz
- output audio: `pcm_s16le`, 24 kHz

Important session fields:

- `Session.ID`: previous dialogue/session id for continuation.
- `Session.Model`: model version.
- `Session.Instructions`: system prompt and style guidance.
- `Session.Audio.Input.Format`: input format and rate.
- `Session.Audio.Output.Format`: output format and rate.
- `Session.Audio.Output.Voice`: voice name.
- `Session.Tools`: function-calling tool definitions.
- `Extension`: provider-specific `asr`, `tts`, `dialog`, `s2s`, and `extra`
  pass-through objects.

Known audio formats from the docs and sample:

- input: `pcm`, `speech_opus`
- output: `pcm_s16le`, `ogg_opus`

## SDK API Coverage

Covered by this SDK:

- `RealtimeDuplex.OpenSession(ctx, cfg)`
- `RealtimeDuplex.Connect(ctx, cfg)`
- `RealtimeDuplexSession.UpdateSession(ctx, cfg)`
- `RealtimeDuplexSession.SendAudio(ctx, audio)`
- `RealtimeDuplexSession.CommitAudio(ctx)`
- `RealtimeDuplexSession.AppendSpeechText(ctx, req)`
- `RealtimeDuplexSession.SendSpeechText(ctx, req)`
- `RealtimeDuplexSession.CommitSpeechText(ctx, req)`
- `RealtimeDuplexSession.AppendReplacementSpeechText(ctx, req)`
- `RealtimeDuplexSession.CommitReplacementSpeechText(ctx, req)`
- `RealtimeDuplexSession.CancelResponse(ctx)`
- `RealtimeDuplexSession.CreateConversationItems(ctx, items...)`
- `RealtimeDuplexSession.UpdateConversationItems(ctx, items...)`
- `RealtimeDuplexSession.RetrieveConversationItems(ctx, items...)`
- `RealtimeDuplexSession.DeleteConversationItems(ctx, items...)`
- `RealtimeDuplexSession.SendFunctionCallOutputs(ctx, outputs...)`
- `RealtimeDuplexSession.RecvEvent(ctx)`
- `RealtimeDuplexSession.Recv()`
- `RealtimeDuplexSession.Close()`

## Client Events

The SDK sends JSON WebSocket text frames for:

- `session.create`
- `session.update`
- `speech_text_buffer.append`
- `speech_text_buffer.commit`
- `input_audio_buffer.append`
- `input_audio_buffer.commit`
- `speech_text_buffer.replacement.append`
- `speech_text_buffer.replacement.commit`
- `conversation.item.create`
- `conversation.item.update`
- `conversation.item.retrieve`
- `conversation.item.delete`
- `response.cancel`
- `session.close`

## Server Events

`RecvEvent` parses known server events and preserves the original payload in
`RealtimeDuplexEvent.Raw`.

Known server event families:

- session lifecycle: `session.created`, `session.updated`, `session.closed`
- input audio acknowledgement: `input_audio_buffer.committed`
- ASR: `conversation.item.input_audio_transcription.*`
- text output: `response.output_text.*`
- audio output: `response.output_audio.*`
- context management: `conversation.item.*`
- function calling: `response.function_call_arguments.done`
- usage/cancel: `response.done`, `response.canceled`
- errors: `error`

Unknown event types are delivered without failing so callers can inspect newer
service behavior through `Raw`.

## Function Calling

Declare tools in `RealtimeDuplexSessionConfig.Tools`. When the service emits
`response.function_call_arguments.done`, `RealtimeDuplexEvent.FunctionCalls`
contains call ids, names, and raw JSON argument strings.

Return tool outputs with:

```go
err := session.SendFunctionCallOutputs(ctx, doubaospeech.RealtimeDuplexFunctionCallOutput{
	CallID: call.CallID,
	Output: result,
})
```

The SDK sends these as `conversation.item.create` items with role `tool`.
