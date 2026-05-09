# kinclaw on WebArena

[WebArena](https://github.com/web-arena-x/webarena) is the de facto
benchmark for **web-based** computer-use agents. ~170 tasks across
6 self-hosted Docker websites (Reddit clone, GitLab clone, OneStopShop,
Magento, Map, Wikipedia). The agent's job is "open this page,
do this multi-step web task, prove the world changed."

**Status:** 📐 designed, not implemented. See [Roadmap](#roadmap).

## Why this is a good fit for kinclaw

WebArena uses **Playwright** to drive headed Chromium. kinclaw's
"web claw" is also Playwright. Same primitive, no impedance.

The agent contract — input observations + ID-based actions — maps
to kinclaw's existing tool calls without much translation:

| WebArena | kinclaw web claw equivalent |
|---|---|
| Observation: `accessibility_tree` of current page | `web.semantic_dump` (DOM + ARIA roles) |
| Observation: `screenshot` | `web.screenshot` |
| Action: `click [123]` | `web.click_by_id 123` |
| Action: `type [123] "hello"` | `web.fill_by_id 123 hello` |
| Action: `scroll up` / `scroll down` | `web.scroll up` / `web.scroll down` |
| Action: `goto [url]` | `web.navigate url` |

Where there's friction:

- WebArena uses **integer element IDs** assigned by its observation
  formatter; kinclaw's web claw uses CSS selectors / accessible
  names. The adapter has to maintain a `WebArena_id → kinclaw_locator`
  map per page-load and inject it into kinclaw's observation.
- WebArena evaluates by replaying the trajectory + checking final
  page state (DOM matches a regex, URL matches a pattern, fetch
  returns expected JSON). The adapter just lets WebArena's evaluator
  run after kinclaw exits — no kinclaw-side instrumentation needed.

## Why we expect kinclaw to do OK here

Anthropic's Computer Use scores ~25% on WebArena; GPT-4o + SoM
~15%. kinclaw's web claw is basically equivalent to Computer Use's
browser path (both Playwright over headed Chrome), so kinclaw's
score should land in roughly the same band — **20-30% range** for
v1.14.2 + claude-sonnet-4-5.

If kinclaw scores significantly under that, the adapter is broken
(not the agent). If significantly over, suspicious of leakage.

## Setup (planned, not implemented)

Following WebArena's [environment_docker README](https://github.com/web-arena-x/webarena/tree/main/environment_docker):

```bash
# 1. Pull WebArena's 6 site images
cd benchmarks/webarena
./setup.sh   # ← TODO: implement
# downloads + starts:
#   localhost:7770   (shopping)
#   localhost:7780   (shopping admin)
#   localhost:8023   (gitlab)
#   localhost:9999   (reddit)
#   localhost:3000   (map)
#   localhost:8888   (wikipedia)

# 2. Verify all 6 are reachable
curl -s localhost:7770/ | head -c 200
# (etc)

# 3. Run kinclaw on the first 20 WebArena tasks
./run.sh AGENT=../../kinclaw TASKS=0-19
```

## Adapter design (planned, not implemented)

```
benchmarks/webarena/adapter/
├── translator.go    WebArena observation ↔ kinclaw skill input mapping
├── runner.go        kinclaw -exec invocation + trajectory capture
├── reporter.go      collect WebArena's eval verdicts → unified results.json
└── id_map.go        WebArena integer-id ↔ Playwright locator bookkeeping
```

The adapter is a Go binary that:

1. Reads WebArena's task config (task_id + intent + sites + eval criteria)
2. Boots a fresh Playwright session pointed at the relevant Docker site
3. Builds an initial observation in WebArena format (accessibility tree)
4. Calls kinclaw with: `kinclaw -exec "$intent\n\n[OBSERVATION]:\n$obs"`
5. Parses kinclaw's tool calls from stdout; for each `web.click_by_id N`,
   translates back to WebArena's `click [N]` and executes via Playwright
6. Re-observes, repeats until kinclaw exits or hits step limit
7. Runs WebArena's task evaluator → records pass/fail
8. Writes per-task `results/<run-stamp>/<task-id>.json`

The "kinclaw exits" signal is currently `kinclaw -exec` returning, but
WebArena tasks are multi-step, so we likely need either:
- Long-running kinclaw via stdin (kinclaw doesn't currently support this in -exec mode)
- OR each "step" is a fresh `kinclaw -exec`, with the prior trajectory
  passed as context (simpler, what we'll do v1)

## Soul

WebArena tasks are multi-step web puzzles. Pilot soul (chat-style)
isn't ideal — we want a more single-minded "achieve this goal"
soul. Plan: write `souls/webarena.soul.md` that:

- Disables memory (each task is fresh)
- Disables spawn (we don't want to fork researchers mid-task)
- Disables screen / input claws (force web-only)
- Tightens circuit breaker (don't let it loop forever)

## Non-goals

- We will NOT cheat by reading WebArena's evaluator and tailoring
  actions to satisfy it directly. Agent only sees the natural-language
  intent + page state.
- We will NOT pre-fetch task answers. Each run is fresh.
- We will NOT drop tasks that score 0 to inflate the average.

## Roadmap

```
v0.1   Adapter scaffolded (this README, no code)              ← here
v0.2   docker-compose pinning WebArena sites + setup.sh
v0.3   adapter runner.go that invokes kinclaw + executes
v0.4   id_map.go robust enough for the GitLab subset
v0.5   first run on 20 tasks; debug
v0.6   first run on full 170 tasks; publish results.json
```

Each step is roughly a day of focused work.

## Reference

- WebArena paper: <https://webarena.dev>
- WebArena GitHub: <https://github.com/web-arena-x/webarena>
- Anthropic Computer Use evaluation on WebArena (their numbers):
  search "Anthropic claude-3-5-sonnet WebArena"
- VisualWebArena (sister benchmark, image-heavy task variant):
  <https://github.com/web-arena-x/visualwebarena>
