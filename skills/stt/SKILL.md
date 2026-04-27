---
name: stt
description: |
  Transcribe an audio file (wav/mp3/mp4/m4a) into text via a local
  LocalKin Service Audio Server (default :8000 / SenseVoice). Pair
  with `record action=start mic=true` to turn a recorded mic track
  into text. Set STT_ENDPOINT env var to point at a different server.
command:
  - sh
  - -c
  - |
    P="$1"
    L="$2"
    [ -z "$P" ] && { echo "stt: path required" >&2; exit 1; }
    # Expand leading ~ — Go's filepath doesn't, shells do.
    case "$P" in "~/"*) P="$HOME/${P#~/}";; "~") P="$HOME";; esac
    [ ! -f "$P" ] && { echo "stt: file not found: $P" >&2; exit 1; }
    # Kernel strips unsubstituted {{vars}} to "" — empty L means
    # caller didn't pass language, just skip the form field.
    if [ -n "$L" ]; then
      RESP=$(curl -sS -X POST "${STT_ENDPOINT:-http://localhost:8000}/transcribe" \
        -F "file=@$P" -F "language=$L" -w '\n%{http_code}')
    else
      RESP=$(curl -sS -X POST "${STT_ENDPOINT:-http://localhost:8000}/transcribe" \
        -F "file=@$P" -w '\n%{http_code}')
    fi
    HTTP=$(printf '%s' "$RESP" | tail -n1)
    BODY=$(printf '%s' "$RESP" | sed '$d')
    if [ "$HTTP" != "200" ]; then
      echo "stt: server returned HTTP $HTTP" >&2
      echo "$BODY" >&2
      exit 1
    fi
    # LocalKin Service Audio shape: {"text":"...","language":"...","confidence":...}.
    # If the server returns something else, fall back to raw.
    TEXT=$(printf '%s' "$BODY" | jq -r '.text // empty' 2>/dev/null)
    if [ -z "$TEXT" ]; then
      printf '%s\n' "$BODY"
    else
      printf 'text: %s\n' "$TEXT"
      LANG=$(printf '%s' "$BODY" | jq -r '.language // empty' 2>/dev/null)
      [ -n "$LANG" ] && printf 'language: %s\n' "$LANG"
      CONF=$(printf '%s' "$BODY" | jq -r '.confidence // empty' 2>/dev/null)
      [ -n "$CONF" ] && printf 'confidence: %s\n' "$CONF"
    fi
  - "_"
args:
  - "{{path}}"
  - "{{language}}"
schema:
  path:
    type: string
    description: Path to an audio file (wav/mp3/mp4/m4a). Leading "~" is expanded. Required.
    required: true
  language:
    type: string
    description: Optional language hint (auto, zh, en, ja, ko, yue, ...). Server default if omitted.
    required: false
timeout: 300
---

# stt — speech transcription via LocalKin Service Audio (SenseVoice)

Wraps the LocalKin Service Audio API at `:8000/transcribe`. Multipart
upload with one `file` field, returns `{"text", "language", "confidence"}`.

## Why this is a SKILL.md and not a native skill

It's a single multipart curl + jq extraction. Pure HTTP wrapper, no
state, no OS API binding — perfect external skill territory.

## Pairing with `record`

```
record action=start audio=false mic=true   → returns recording_id
... user speaks ...
record action=stop id=<recording_id>        → returns path: ~/.../rec-XXXX.mp4
stt path=~/.../rec-XXXX.mp4                 → returns the spoken text
```

The mp4 from `record` has its mic audio in a track that SenseVoice
reads directly — no ffmpeg extraction needed.

## Override the endpoint

```bash
STT_ENDPOINT=http://otherbox:8000 kinclaw -soul souls/pilot.soul.md
```

## Failure modes

- `stt: path required` / `stt: file not found` — caller-side argument problems.
- `stt: server returned HTTP ...` — server up but rejected; body is echoed.
- Connection refused — server isn't running on the configured port.
