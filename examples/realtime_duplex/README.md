# Realtime Duplex conversation example

This example runs a live multi-service conversation smoke test:

1. The existing `Client.Realtime` API generates a spoken prompt as 16 kHz PCM.
2. The new `Client.RealtimeDuplex` API receives that audio, transcribes it,
   calls configured function tools when requested, and returns text/audio.
3. `Client.ASRV2` (SAUC) best-effort transcribes the Duplex output audio so the
   example can log whether the returned speech content is reasonable when the
   API key has ASR resource access.

The example does not use PortAudio, microphone capture, playback, or `gopus`.

## Required environment

```bash
export DOUBAO_APP_ID=<your_app_id>

# Existing Realtime API key.
export DOUBAO_REALTIME_API_KEY=<your_realtime_api_key>

# Duplex API key. Omit this when the Realtime key is shared.
export DOUBAO_DUPLEX_API_KEY=<your_duplex_api_key>

# ASR SAUC API key. Omit this when the Realtime key is shared.
export DOUBAO_ASR_API_KEY=<your_asr_api_key>

# Optional Volc web-search API key for typed dialog.extra web-search fields.
export DOUBAO_VOLC_WEBSEARCH_API_KEY=<your_search_api_key>
```

`DOUBAO_API_KEY` is accepted as a shared fallback. The example also falls back to
`DOUBAO_REALTIME_API_KEY` when the same API key is valid for all three services.
`DOUBAO_APP_ID` is an App ID config value, not an authentication factor.

## Run

```bash
go run ./examples/realtime_duplex -rounds 2 -out-dir /tmp/realtime-duplex-smoke
```

The run logs:

- old Realtime generated text and PCM byte count
- Duplex ASR/text/audio events
- raw function-call structures from `response.function_call_arguments.done`
- deterministic local tool outputs sent back through `conversation.item.create`
- ASR SAUC transcript of the Duplex returned audio
- an ASR warning instead of failure when the API key lacks SAUC resource access

The example configures typed Duplex extension fields under `extension.asr`,
`extension.tts`, and `extension.dialog`, including optional
`extension.dialog.extra.enable_volc_websearch` when
`DOUBAO_VOLC_WEBSEARCH_API_KEY` is set.

The optional `-out-dir` writes:

- `roundN-old-realtime.pcm`
- `roundN-duplex.pcm`
- `roundN-duplex-asr.txt`

## Live test

Live tests are opt-in:

```bash
DOUBAO_RUN_LIVE=1 go test ./examples/realtime_duplex -run TestLiveRealtimeDuplex -count=1 -v
```
