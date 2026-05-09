# kinclaw on Online-Mind2Web

[Online-Mind2Web](https://github.com/OSU-NLP-Group/Online-Mind2Web)
(2025-03) is the live-execution variant of [Mind2Web](https://github.com/OSU-NLP-Group/Mind2Web).
The original Mind2Web is a static dataset (predict the next action
given a cached HTML snapshot); the online variant runs the agent
against real websites in real time, like WebArena.

**Status:** 📋 investigation pending. See [Decision points](#decision-points).

## Why this is on the list

If we ship a WebArena adapter (high feasibility — same Playwright
primitive as kinclaw's web claw), Online-Mind2Web is a likely
"second public web benchmark" win because:

- Same general shape: live browser, natural-language task, agent
  acts step-by-step
- Different task taxonomy: Mind2Web focused on real public websites
  (United Airlines, IMDb, etc.) instead of WebArena's self-hosted
  Reddit/GitLab clones
- Cross-validation: two web-agent benchmarks giving similar numbers
  means kinclaw's web claw works generally, not just on a specific
  task design

## Why not yet — open questions

Before committing to an adapter, verify:

1. **Does Online-Mind2Web use Playwright?** WebArena does; if
   Mind2Web uses Selenium / playwright-py / a custom driver, the
   adapter shape differs.
2. **What's the agent contract?** WebArena's `click [N]` ID format
   vs Mind2Web original's element candidate selection — Online-Mind2Web's
   live mode might inherit either.
3. **How is success evaluated?** WebArena: post-task DOM/URL match
   + side-effect check. Mind2Web original: action-step accuracy.
   Online-Mind2Web: TBD — read its README before designing.
4. **Are real websites used?** If yes, scores are reproducible only
   so long as the live site doesn't change layout. WebArena solved
   this with self-hosted Dockers — does Online-Mind2Web?
5. **Is there a leaderboard?** If WebArena and OSWorld both have
   public leaderboards but Online-Mind2Web is academic-only, the
   marketing value is lower.

## Decision points

```
Q1  Online-Mind2Web uses Playwright       →  proceed to adapter design
Q1  Online-Mind2Web uses something else   →  evaluate whether kinclaw's
                                              web claw can be adapted

Q2  Live sites change frequently           →  scores are noisy, may not
                                              be worth the effort
Q2  Sites are cached / stable              →  good candidate

Q3  Public leaderboard exists              →  high marketing value
Q3  Academic-only                          →  lower priority
```

After v0.2 of the WebArena adapter (so we know what "an adapter for
a web-agent benchmark" looks like in our codebase), spend ~half a day
walking through Online-Mind2Web's repo + answering Q1-Q3. Either:

- **Promote to designed/implemented** — write a real README.md +
  scaffold setup.sh / run.sh (~3 days work after that)
- **Demote** — write a paragraph here explaining why we aren't doing
  it, and remove from the parent README.md status board

## Why we're NOT touching Mind2Web (original / static)

The original Mind2Web is a dataset of cached HTML snapshots paired
with target actions. The "agent" is a model that predicts the next
DOM element to click given the snapshot. That's a model-evaluation
task, not an agent-execution task.

kinclaw's whole architecture is **execution** — soul, brain, 5-claw,
spawn, memory. None of that surface area is exercised by predicting
"which DOM element should be clicked given this static HTML." Running
kinclaw on Mind2Web would mean stripping kinclaw down to "just the
brain" and asking it to do classification — that's testing
DeepSeek-V4-Pro / claude-sonnet-4-5 / kimi-k3, not testing kinclaw.

If you want a static web-task evaluation, run a vision-LLM directly
with a Mind2Web evaluator — not via kinclaw. The numbers would be
the same as just running the model.

## Reference

- Mind2Web original (static): <https://github.com/OSU-NLP-Group/Mind2Web>
- Online-Mind2Web (live, what we'd target):
  <https://github.com/OSU-NLP-Group/Online-Mind2Web>
- Mind2Web project page: <https://osu-nlp-group.github.io/Mind2Web/>
