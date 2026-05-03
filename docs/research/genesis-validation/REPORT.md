# Genesis Loop Validation: "use it once and it knows how"

**Run started**: 2026-04-27 18:47:57 -0700

Each task runs twice. The 2nd run should be **dramatically faster** because the agent forged a skill on the 1st run and reuses it directly on the 2nd.

| # | Task | Cold | Warm | Speedup | Forged skills | Warm used skill? |
|---|---|---:|---:|---:|---|---|
| 1 | notes_create | 45s / 0
0 calls | 32s / 0
0 calls | 140% | (none) | (no) |
