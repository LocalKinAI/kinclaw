---
name: tts
description: |
  Synthesize speech from text and play it through the macOS speakers.
  Talks to a local LocalKin Service Audio Server (default :8001 / Kokoro).
  When `record` is running with audio=true, the spoken audio is
  captured into the recording — use this in place of `shell say` for
  high-quality multilingual narration in demo videos. Set the
  TTS_ENDPOINT env var to point at a different server.
command:
  - sh
  - -c
  - |
    T="$1"
    S="$2"
    W="$3"
    # Kernel strips unsubstituted {{vars}} to "", so empty == "param
    # not passed". Don't add `[ "$X" = "{{name}}" ]` sentinels — those
    # self-defeat when the caller passes a real value.
    # Default to a Chinese female voice when the text contains CJK
    # characters, otherwise let the server pick. The server silently
    # falls back to English-only Kokoro on missing speaker, which
    # mispronounces Chinese as the literal phrase "Chinese letter".
    if [ -z "$S" ] && printf '%s' "$T" | LC_ALL=C grep -q '[^[:print:][:space:]]'; then
      S="${TTS_DEFAULT_ZH_SPEAKER:-zf_xiaoxiao}"
    fi
    if [ -n "$S" ]; then
      PAYLOAD=$(jq -nc --arg t "$T" --arg s "$S" '{text:$t,speaker:$s}')
    else
      PAYLOAD=$(jq -nc --arg t "$T" '{text:$t}')
    fi
    OUT=$(mktemp -t kinclaw-tts).wav
    HTTP=$(printf '%s' "$PAYLOAD" \
      | curl -sS -X POST "${TTS_ENDPOINT:-http://localhost:8001}/synthesize" \
          -H 'Content-Type: application/json' \
          --data-binary @- \
          -o "$OUT" -w '%{http_code}')
    if [ "$HTTP" != "200" ]; then
      echo "tts: server returned HTTP $HTTP" >&2
      cat "$OUT" >&2
      rm -f "$OUT"
      exit 1
    fi
    # wait=false (default): play in background, return immediately so
    # the agent can continue acting while audio is still narrating.
    # During `record` this gives parallel narration + action without
    # the recording capturing dead air.
    # wait=true: block until afplay finishes — use only when the next
    # action visually depends on what was just said.
    if [ "$W" = "true" ]; then
      afplay "$OUT" || exit $?
      printf 'spoken: %s\nspeaker: %s\nmode: blocking\npath: %s\n' "$T" "${S:-<server default>}" "$OUT"
    else
      ( afplay "$OUT" >/dev/null 2>&1 ) &
      printf 'spoken: %s\nspeaker: %s\nmode: background pid=%d\npath: %s\n' "$T" "${S:-<server default>}" "$!" "$OUT"
    fi
  - "_"
args:
  - "{{text}}"
  - "{{speaker}}"
  - "{{wait}}"
schema:
  text:
    type: string
    description: Text to speak. Required. Captured into video by `record` when audio=true.
    required: true
  speaker:
    type: string
    description: |
      Kokoro speaker id. **The server's field name is `speaker`, not `voice`** — passing the wrong key is silently ignored and falls back to the English model, which mispronounces Chinese as "chinese letter". Examples:
      - Chinese female: `zf_xiaoxiao` (default for CJK text), `zf_xiaobei`, `zf_xiaoni`
      - Chinese male: `zm_yunxi`, `zm_yunjian`
      - English female: `af_bella`, `af_sarah`
      - English male: `am_adam`, `am_michael`
      Omit to let the skill auto-pick: CJK text gets `zf_xiaoxiao`, ASCII gets the server default.
    required: false
  wait:
    type: string
    description: |
      "true" or "false" (default false). Default false plays in the background and returns immediately, so the agent keeps acting while the narration plays — recommended during `record` to avoid burning recording time on dead air. Pass "true" only when the next action visually depends on what was just said (rare).
    required: false
timeout: 120
---

# tts — speech synthesis via LocalKin Service Audio (Kokoro)

Wraps the LocalKin Service Audio API at `:8001/synthesize` and plays
the returned WAV through the macOS default output (`afplay`).

## Why this is a SKILL.md and not a native skill

It's three lines of curl + afplay. Pushing it into `pkg/skill/` would
violate the "thin kernel + fat skill" thesis and make it harder for
users to fork. As an external SKILL.md it's also a forge template:
the next HTTP service that needs wrapping can be modeled on this file.

## How `record` captures the narration

`record action=start audio=true` enables ScreenCaptureKit's system-audio
tap. `afplay` writes to the default output device, which the tap
captures. End result: the spoken text shows up on the video's audio
track without any extra plumbing.

## Examples

```
tts text="接下来我会打开计算器" voice="zf_xiaoxiao"
tts text="Now I'll open Safari and search for KinClaw"
```

## Override the endpoint

```bash
TTS_ENDPOINT=http://otherbox:8001 kinclaw -soul souls/pilot.soul.md
```

## Failure modes

- `tts: server returned HTTP 000` — server isn't running on the configured port.
- `tts: server returned HTTP 4xx/5xx` — server is up but rejected the request; the body is echoed to stderr.
- `afplay: ...` — playback failed (no audio device, sandboxed environment).
