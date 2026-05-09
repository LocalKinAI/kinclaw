# kinclaw — benchmark portfolio

This directory holds adapters that let kinclaw participate in
public computer-use / web-agent benchmarks beyond its home court
([macbench](https://github.com/LocalKinAI/macbench)).

The goal is **honest cross-validation**: if kinclaw scores well on
macbench, that's our home turf. If kinclaw also scores meaningfully
on benchmarks from other research groups (especially ones designed
for different OSes / different task surfaces), the "kinclaw works"
claim isn't a fluke of our own benchmark design.

## Status board

| Benchmark | Platform | kinclaw fit | Status | Effort | Why |
|---|---|---|---|---|---|
| [macbench](https://github.com/LocalKinAI/macbench) | macOS native | 🟢 Reference agent | **v0.1 shipped, 50 tasks** | done | This is kinclaw's home — we built it |
| [WebArena](webarena/) | Web (Playwright + Docker) | 🟢 High | 📐 Designed, not implemented | ~1-2 days | kinclaw's web claw IS Playwright; same primitive |
| [OSWorld](osworld/) | Ubuntu / Windows VM | 🟡 Vision-only fallback | 📐 Designed, vision-only | ~1 week | AX useless inside VM; kinclaw degrades to screenshot+coords |
| [Online-Mind2Web](online-mind2web/) | Live web | 🟡 Likely high | 📋 Investigation TBD | ~2-3 days | Newer (2025-03) live variant of Mind2Web; similar shape to WebArena |
| Mind2Web (original) | Static action prediction | 🔴 Architectural mismatch | ❌ Skipped | — | Static dataset; tests "predict next action" not "execute task" |

Status legend:
- ✅ runnable
- 📐 designed (README + plan in place; no code)
- 📋 investigation (next step is to verify feasibility)
- ❌ skipped (with documented reason)

## Why this layout

We initially considered putting each adapter in its own repo
(LocalKinAI/kinclaw-webarena, etc.). Rejected because:

1. **Adapter ↔ kinclaw is tightly coupled** — every kinclaw release
   may need an adapter version-bump (CLI flags change, soul format
   changes). Same-repo lockstep is simpler than cross-repo
   coordination.
2. **The benchmarks themselves are upstream** — we don't fork OSWorld
   or WebArena. Adapters are thin glue; they don't need their own
   release cycle.
3. **Solo-founder budget** — 5 repos > 1 repo for someone single
   maintaining everything.

If an adapter grows large enough to deserve its own repo, extract.
Until then, in-tree.

## What an adapter contains

Each `<benchmark>/` subdirectory has:

```
<benchmark>/
├── README.md          ← positioning, setup, agent contract translation
├── setup.sh           ← idempotent: install upstream benchmark + deps
├── run.sh             ← entry point: invoke kinclaw on N tasks
├── adapter/           ← Go code: translate kinclaw I/O ↔ benchmark I/O
└── results/           ← gitignored — per-run outputs + scores
```

`run.sh` is the user-facing entry; everything else is implementation.

## Headline numbers (when we have them)

This table will fill in as runs complete. Each row is one (agent,
benchmark, version) tuple. Don't compare diagonally — the benchmarks
test different things.

| Agent | macbench v0.1 | WebArena | OSWorld | Online-Mind2Web |
|---|---|---|---|---|
| kinclaw v1.14.2 + claude-sonnet-4-5 | TBD | — | — | — |
| kinclaw v1.14.2 + deepseek-v4-pro | TBD | — | — | — |
| (reference) Anthropic Computer Use | n/a | ~25% | ~38-42% | n/a |
| (reference) GPT-4o + SoM | n/a | ~15% | ~12-15% | n/a |

References for cross-comparable numbers (when our score lands):
- OSWorld leaderboard: <https://os-world.github.io>
- WebArena: <https://webarena.dev>
- Mind2Web / Online-Mind2Web: <https://osu-nlp-group.github.io/Mind2Web/>

## Roadmap

```
this week     macbench v0.1 first real run            → published score
+1 week       WebArena adapter implemented + 1 run    → "we work cross-platform on web"
+2 weeks      OSWorld adapter (vision-only mode)      → cross-OS validation, honest caveats
+3 weeks      Online-Mind2Web adapter                  → second public web benchmark
+1 month      blog post + leaderboard page             → marketing surface

ongoing       grow macbench from 50 → 369 tasks       → OSWorld parity by year-end
```

## Honest claim ladder

What we're allowed to claim, by milestone:

| When | Claim | Evidence |
|---|---|---|
| macbench v0.1 first run | "kinclaw scores X% on a public macOS-native benchmark we maintain" | run.json |
| + WebArena run | "kinclaw also scores Y% on WebArena (Playwright web tasks)" | run.json + WebArena's own evaluator |
| + OSWorld vision-only | "kinclaw's vision-only fallback scores Z% on OSWorld; **AX-mode (kinclaw's actual capability) is measured by macbench, not this**" | OSWorld evaluator output + caveat docs |
| All four ≥ baseline | "kinclaw is a competitive computer-use agent on multiple public benchmarks, with its strongest results on macOS native (where it was designed)" | combined run.json reports |
