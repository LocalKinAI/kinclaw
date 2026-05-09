# KinClaw test suites

Four in-tree tiers + one external benchmark. Pick the tier
appropriate to what you changed.

| Tier | What it tests | Needs | Time | Who runs |
|---|---|---|---|---|
| **0 — Hygiene** | Compile, unit tests, version-string consistency | nothing | <30s | CI / agent |
| **1 — Kit smoke** | Each kit CLI starts + prints something sensible | nothing | <5s | CI / agent |
| **2 — Kit deep** | Each kit's headline verbs actually drive macOS | TCC granted on the kit binary | ~30s | you, manually |
| **3 — kinclaw verbs** | The migrated kit-bridge verbs work via `kinclaw -exec` | TCC granted on kinclaw | ~1min | you, manually |
| **4 — End-to-end** | KinClawMac shell + Cowork pilot + spawned researcher | KinClawMac running, brain online | ~3min | you, eyes |
| **5 — macbench** | 50 macOS native tasks via [LocalKinAI/macbench](https://github.com/LocalKinAI/macbench) | TCC + macbench cloned at ../macbench | ~10min | you, manually |

Run order on a day you suspect something broke:
**0 → 1 → 4**. Skip 2 and 3 unless 4 fails — they're for narrowing down
*which* layer broke, not for daily verification. **macbench is the
public-facing capability score**, run it before public releases or
when you want a hard number to defend.

```bash
make smoke           # tier 0 + 1, fully automated, no permission needed
./tests/tier3.sh     # tier 3 — drives a real app (Safari) via kinclaw -exec
make bench           # tier 5 — runs macbench with this kinclaw build
                     # (requires ../macbench checkout)
# tier 4 is just `cd kinclaw-mac && make kill && make run`,
# then chat in Cowork and watch what happens.
```

## What each tier proves

- **Tier 0** proves the build pipeline is still intact (no broken imports,
  no version-tag drift). Catches *publisher* mistakes.
- **Tier 1** proves the dylib loads + the CLI surface is wired. Catches
  *embed* / *purego* breakage.
- **Tier 2** proves the kit talks to macOS for real. Catches *TCC* /
  *ObjC framework* / *AX-API-changed-in-this-OS-update* problems.
- **Tier 3** proves the migrated kinclaw skill verbs still call into the
  kit correctly. **This is the tier specifically aimed at
  v1.14.2's kit-debt repayment.**
- **Tier 4** proves the agent loop still reasons correctly — soul,
  brain, spawn, memory, circuit breaker, all of it.
- **Tier 5 (macbench)** proves the agent **delivers value on real
  macOS tasks** — Finder rename, Calendar event, Safari multi-tab,
  Mail draft compose, system settings toggle. Pure capability score.
  Lives in a separate repo because the benchmark methodology is
  agent-agnostic — it can be used to measure Anthropic Computer Use,
  OpenAI CUA, or any other macOS agent.

## Per-version regression dossiers

Every release that changed something risky gets a doc in
`tests/regressions/`. Two so far:

- [`v1.14.2-kit-debt.md`](regressions/v1.14.2-kit-debt.md) —
  ~400 lines lifted out of `pkg/skill/` into kinax-go / sckit-go /
  input-go. Verifies the migrated paths still work.
- [`pre-v1.14.2-baseline.md`](regressions/pre-v1.14.2-baseline.md) —
  the long-running "agent didn't regress on its own" baseline:
  knowledge_search doesn't fake-trip the circuit breaker, memory
  doesn't blow up, web_search SearXNG-first / DDG-fallback works,
  spawn detached delivery hits the SwiftUI bubble.

When you ship something that touches existing behavior, add a
`tests/regressions/<vX.Y.Z>-<name>.md` with the same shape and link
it from this README.
