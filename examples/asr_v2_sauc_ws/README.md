# asr_v2_sauc_ws example

This example runs the SDK's SAUC bidirectional streaming ASR flow:

open WebSocket -> send metadata -> send audio frames -> receive interim/final
results -> close.

The current SDK endpoint is:

```text
wss://openspeech.bytedance.com/api/v3/sauc/bigmodel
```

## Run with API Key

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/asr_v2_sauc_ws \
  -audio /path/to/audio.pcm \
  -resource-id volc.seedasr.sauc.duration
```

## Resource IDs

- ASR 1.0 duration: `volc.bigasr.sauc.duration`
- ASR 1.0 concurrent: `volc.bigasr.sauc.concurrent`
- ASR 2.0 duration: `volc.seedasr.sauc.duration`
- ASR 2.0 concurrent: `volc.seedasr.sauc.concurrent`

The default is `volc.seedasr.sauc.duration`. The credential must be granted for
the selected resource.

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

If you see `requested resource not granted`, keep the same credential and switch
only to a resource that is actually enabled for that credential.
