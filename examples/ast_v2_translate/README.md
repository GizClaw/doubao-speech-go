# AST V2 realtime translation example

This example streams a local 16 kHz mono int16 audio file into the AST V2 realtime translation WebSocket API.

## Run

Legacy credential mode:

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_ACCESS_KEY=<your_access_key> \
go run ./examples/ast_v2_translate \
  -audio examples/asr_v2_sauc_ws/sample_zh_16k.pcm \
  -mode s2t \
  -source zh \
  -target en
```

API key mode:

```bash
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/ast_v2_translate -mode s2t -source zh -target en
```

The example also accepts this repository's realtime `.env` names for local smoke testing:

- `DOUBAO_REALTIME_APP_ID`
- `DOUBAO_REALTIME_ACCESS_KEY`

## S2S

```bash
DOUBAO_APP_ID=<your_app_id> \
DOUBAO_ACCESS_KEY=<your_access_key> \
go run ./examples/ast_v2_translate \
  -mode s2s \
  -source zh \
  -target en \
  -speaker zh_female_vv_uranus_bigtts \
  -target-format ogg_opus
```

The input audio must match the SDK config sent to AST: 16 kHz, mono, signed 16-bit PCM/WAV-compatible data.
