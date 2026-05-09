# Pre-v1.14.2 baseline regressions

Issues fixed in earlier versions that we want to keep dead. Run this
suite when you suspect *any* of these failure modes have come back —
which usually shows up as the agent doing something obviously
counterproductive.

These are **lessons-learned debugging traps**, not architectural
verifications. If you've never seen any of these, you don't need to
run this; only useful when something feels off.

---

## R1 · knowledge_search shouldn't trip the circuit breaker

**Symptom that came back:** agent reads from local knowledge → sees
an error/empty → reads again → triggers "consecutive same output"
trigger → gives up.

**What we want:** `knowledge_search` returning empty is a normal
"didn't find anything" state, not a bug. Should not be counted as
a circuit-breaker hit.

```
Cowork:  "查一下我笔记里关于 Hermes Agent 的内容"
         (or any topic the local KB doesn't have)

Pass:    Researcher returns "本地未找到，已转用 web_search ..."
         and continues to a real answer.
Fail:    Pilot reports "circuit breaker tripped" or aborts the
         turn after 1-2 knowledge_search calls.
```

## R2 · Memory shouldn't grow unbounded

**Symptom that came back:** every turn appends `_finding_<n>` /
`_step_<n>` durable memories, hundreds accumulate, model context
gets polluted, search retrieval starts returning noise.

**What we want:** transient `_<name>` entries are cleared between
turns; only meaningful save calls survive.

```
1. Open Cowork, run 3-4 deep research turns.
2. After each, check:
   curl -s http://localhost:5001/memory | jq 'length'

Pass:    count grows by 0-2 per turn (only durable saves).
Fail:    count grows by 5+ per turn (transient leak).

Reset:   Click "Clear" in Cowork → memory should drop to ~0.
```

## R3 · web_search should prefer SearXNG, fail gracefully on DDG

**Symptom that came back:** DDG anti-bot block returns HTML →
agent thinks it has results → quotes random page chrome as the
answer.

**What we want:** SearXNG (local) tried first; DDG fallback gets
"actionable error" pointing the model at `web_scrape` rather
than silent garbage.

```
Cowork:  "今天美股新闻"

Pass:    Output looks like it came from SearXNG (clean snippets,
         no "DuckDuckGo" branding in the result text).
Fail:    Output contains "Try DuckDuckGo!", or empty results
         followed by hallucinated answer.

If SearXNG itself is down:
Pass:    "SearXNG unreachable; web_search returned 0 hits.
         Try web_scrape <url> if you have a URL in mind."
Fail:    Silent empty / unrelated answer.
```

## R4 · Spawn detached delivery should hit the SwiftUI bubble

**Symptom that came back:** pilot spawns researcher, dispatch returns
in 200µs ("turn ends"), researcher runs 2-5 minutes, finishes, but
the report **never appears in Cowork** — `assistantIndex` stale-guard
in `SpotlightContentView.swift::handleLocalEvent` drops the
`spawn_done` SSE event.

**What we want:** spawn_done arrives as a fresh user-facing bubble,
even though the originating turn is long over.

```
Cowork:  "派 researcher 调研 Apple 市值变化"
         (any task the pilot will detach — usually anything
         that takes >90s)

Wait 2-5 minutes. Don't close Cowork. Keep using it for other
chat in the meantime.

Pass:    A "researcher 已完成" bubble appears with the report
         attached, even if you've sent other messages since.
Fail:    Pilot says "researcher already done" but report never
         appears; or worse, you see nothing and silently lose
         the deliverable.
```

## R5 · Soul should be hot — not stale on rebuild

**Symptom that came back:** edited `souls/pilot.soul.md`, ran
`make run`, but pilot still using yesterday's soul because
`install.sh -n` (no-clobber) skipped the copy.

**What we want:** repo soul edits are live as soon as
KinClawMac restarts.

```
1. Edit souls/pilot.soul.md — add a unique watermark string
   to the soul (e.g. "WATERMARK_TEST_<timestamp>").
2. cd kinclaw-mac && make kill && make run
3. In Cowork: "你的 soul 里有 WATERMARK_TEST 这个标记吗？"

Pass:    "是的，我看到 WATERMARK_TEST_<timestamp>"
Fail:    "没有这个标记" → soul didn't refresh.
```

## R6 · Session reset should not dump memory

**Symptom that came back:** "Clear" button in Cowork wiped both
session AND memory → agent forgot user's name, preferences, etc.

**What we want:** Clear resets conversation state; memory persists.

```
1. Tell Cowork:  "记住我的名字是 Jacky"
2. Wait for memory.save confirmation.
3. Click "Clear" / 清除会话.
4. New empty conversation. Ask: "我叫什么？"

Pass:    "你叫 Jacky" (memory survived).
Fail:    "我不知道" (memory was reset along with session).
```

---

## Outcome log

```
2026-05-08 (initial)    R1-6 not yet re-verified post-v1.14.2.
                        All known to be PASS as of v1.14.0
                        (validated on the original fix dates).
```
