# asr_v2_sauc_ws example

This example runs the SDK's optimized SAUC bidirectional streaming ASR flow:

open WebSocket -> send metadata -> send audio frames -> receive interim/final
results -> close.

The current SDK endpoint is:

```text
wss://openspeech.bytedance.com/api/v3/sauc/bigmodel_async
```

Both modes explicitly send `enable_nonstream=false`. Audio is sent at real-time
cadence while results are received concurrently. Continuous input and PTT use
the same `client.ASRV2.OpenStreamSession` API.

## Run with API Key

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/asr_v2_sauc_ws \
  -mode ptt \
  -audio /path/to/audio.pcm \
  -resource-id volc.seedasr.sauc.duration
```

## Resource IDs

- ASR 1.0 duration: `volc.bigasr.sauc.duration`
- ASR 1.0 concurrent: `volc.bigasr.sauc.concurrent`
- ASR 2.0 duration: `volc.seedasr.sauc.duration`
- ASR 2.0 concurrent: `volc.seedasr.sauc.concurrent`

The default is `volc.seedasr.sauc.duration`. The API key must be granted for
the selected resource. `DOUBAO_APP_ID` is an App ID config value, not an
authentication factor.

## Audio Input

This example embeds a sample PCM fixture (`sample_zh_16k.pcm`). If `-audio` is
not provided, it uses the embedded sample automatically.

Because this sample is stored via Git LFS, run `git lfs pull` after cloning.

Input requirements:

- format: `pcm`
- sample rate: `16000`
- channel: mono
- sample width: signed 16-bit PCM

The default `-chunk-size` is `3200` bytes, or 100 ms of 16 kHz mono 16-bit PCM.
The official guidance is to send 100-200 ms packets at 100-200 ms intervals.

## PTT Finalization

```bash
go run ./examples/asr_v2_sauc_ws -mode ptt \
  -resource-id volc.bigasr.sauc.duration -expect 今天天气不错
```

PTT sends one copy of the input, followed by an empty final audio marker. It
continues receiving after sentence-level `IsFinal` results until the final
server packet ends `Recv()`. A successful run requires a non-empty final
transcript after audio EOS and provider audio duration within one packet of
the sent duration. `-expect` optionally requires a substring in that transcript;
continuous mode requires it at least once per input copy.

## Continuous Utterance Boundaries

```bash
go run ./examples/asr_v2_sauc_ws -mode continuous -turns 2 \
  -resource-id volc.bigasr.sauc.duration -expect 今天天气不错
```

Continuous mode sends multiple copies of the input in one session, with
`-silence=2s` after each copy. It requires a non-empty `definite` utterance
whose end time falls in each copy before any audio EOS is sent. Only after
every copy has passed this check does it send the final marker and drain the
session. It prints each unique utterance's text and time range, then a `PASS`
summary with the full transcript, finalization latency, and audio duration.
Missing boundaries, provider errors, incomplete finalization, and failed
transcript checks return a nonzero exit status.

Both modes default to `-end-window-size=200` and
`-force-to-speech-time=1000` (milliseconds). These match GizClaw's realtime
endpointing parameters; use `-end-window-size=500` to check the other builder
default. The overall example has a two-minute deadline. Export
`DOUBAO_APP_ID` and `DOUBAO_API_KEY` before running these commands.

If you see `requested resource not granted`, keep the same API key and switch
only to a resource that is actually enabled for that key.
