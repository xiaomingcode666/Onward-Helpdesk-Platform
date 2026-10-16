# Local ASR

Loopback-only speech-to-text service used by the backend `local_whisper`
speech provider (customer voice tickets). It runs faster-whisper
`large-v3-turbo` on the local GPU and never calls a cloud service.

## Run

```bash
pip install -e .            # faster-whisper, numpy, fastapi, uvicorn
HF_HOME=<repo>/.local/hf-cache HF_HUB_OFFLINE=1 \
  uvicorn local_asr.app:app --host 127.0.0.1 --port 8791
```

`HF_HUB_OFFLINE=1` makes the model load only from the local cache. The model
is preloaded at startup. Environment overrides:

| Variable | Default |
|---|---|
| `RHD_ASR_MODEL` | `large-v3-turbo` |
| `RHD_ASR_DEVICE` | `cuda` (falls back to `cpu`/`int8` with a warning) |
| `RHD_ASR_COMPUTE_TYPE` | `float16` |

Backend configuration:

```yaml
speech:
  provider: local_whisper
  localWhisper:
    endpoint: http://127.0.0.1:8791
    timeoutSeconds: 60
```

## API

- `POST /v1/asr/transcribe?language=zh-CN`, body: PCM16 WAV
  (`Content-Type: audio/wav`) → `{"text", "language", "durationSeconds"}`.
  400 for non-WAV or empty audio, 503 when faster-whisper is missing.
- `GET /healthz` → `{"ok": true, "device": "cuda"}`.

## Test

```bash
pip install -e ".[test]"
pytest
```
