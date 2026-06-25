# Simultaneous Interpretation

Official documentation: <https://www.volcengine.com/docs/6561/1756902>

This page maps the VolcEngine AST simultaneous interpretation WebSocket API to
the current SDK surface. The API supports speech-to-text (`s2t`) and
speech-to-speech (`s2s`) translation.

## Endpoint

```text
wss://openspeech.bytedance.com/api/v4/ast/v2/translate
```

## SDK Coverage

Implemented by `ASTTranslate.OpenSession`.

The SDK supports:

- `StartSession`
- `TaskRequest` audio upload
- `UpdateConfig`
- `FinishSession`
- protobuf transport
- parsed source subtitle events
- parsed translation subtitle events
- parsed TTS audio events
- usage and muted-audio events

## Authentication

Recommended API-key headers:

```http
X-Api-Key: <api-key>
X-Api-Resource-Id: volc.service_type.10053
```

Legacy app-id/access-key headers are also supported by the upstream service.

## Language Modes

| Mode | Constraint | Languages |
| --- | --- | --- |
| `s2s` with public speaker | source and target are required; target must be Chinese or English | source: `lang_20` plus dialects; target: `zh` or `en` |
| `s2s` voice-clone mode | omit unsupported `speaker_id`; source or target must be Chinese or English | source: `lang_8`; target: `lang_8` |
| `s2t` | source and target are required; source or target must be Chinese or English | source: `lang_20` plus dialects; target: `lang_20` |

Public speakers listed by the official document:

- `zh_female_vv_uranus_bigtts`
- `zh_male_jingqiangkanye_emo_mars_bigtts`

## Audio Requirements

Source audio:

- format: `wav`
- codec: `raw`
- sample rate: `16000`
- bit depth: `16`
- channel: `1`

For `s2s`, target audio can be `pcm` or `ogg_opus` depending on the upstream
configuration.

## Example

```bash
DOUBAO_API_KEY=<your_api_key> \
go run ./examples/ast_v2_translate -mode s2t -source zh -target en
```
