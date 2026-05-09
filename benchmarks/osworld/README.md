# kinclaw on OSWorld (vision-only mode)

[OSWorld](https://github.com/xlang-ai/OSWorld) is the de facto
benchmark for **desktop** computer-use agents — 369 tasks across
Ubuntu and Windows VMs. Anthropic's Computer Use, OpenAI's CUA,
and most academic agent papers report on it.

**Status:** 📐 designed, vision-only mode planned. See [Roadmap](#roadmap).

## Why this is awkward for kinclaw

OSWorld runs the target apps inside a **VM** (VMware Fusion / VirtualBox /
Docker+KVM on Linux hosts). The agent's job is to drive Linux apps —
LibreOffice, GIMP, VLC, Chrome — that live entirely inside that VM.

Three of kinclaw's five claws are macOS-specific and **cannot reach
into a VM**:

| Claw | macOS host | Inside the VM |
|---|---|---|
| `screen` (sckit-go) | ✅ Can capture VMware window | ✅ pixels are visible |
| `input` (input-go via CGEvent) | ✅ Can synthesize | 🟡 Goes via VMware's input forwarding to guest — works |
| `ui` (kinax-go via AX) | ✅ Sees VMware Fusion app itself | ❌ AX is OS-level; Linux apps don't expose macOS AX |
| `record` (kinrec) | ✅ Records macOS desktop | n/a |
| `web` (Playwright) | ✅ via macOS Chrome | ❌ Linux Chrome inside VM |

So kinclaw on OSWorld degrades to:

- **screen**: capture VMware Fusion window (treats it as just pixels)
- **input**: forward clicks/keystrokes through VMware to guest
- **ui**: ❌ — agent has no semantic understanding of guest app structure
- **record**: still works for video logging
- **web**: ❌ — would need to ssh into guest, not the kinclaw way

**Net effect:** kinclaw becomes "vision-LLM driving a screenshot +
coordinate clicker". This is exactly the agent profile that scored
~12-15% on OSWorld (GPT-4o + Set-of-Mark).

## Why run it anyway

Three reasons:

1. **Public-facing leaderboard parity** — OSWorld is the desktop
   computer-use benchmark people actually cite. Having a kinclaw
   number on it (even with caveats) puts kinclaw in the conversation.
2. **Honesty floor** — if kinclaw's vision-only mode hits ~10%, that's
   a useful lower bound on its capability without AX. We claim AX
   gets us 30+% on macbench; OSWorld measures the version *without*
   AX. The two together make a richer story than either alone.
3. **Cross-OS validation that screen + input claws actually work** —
   if kinclaw on Ubuntu via VMware can complete *some* tasks, our
   primitives are not macOS-only by accident; they generalize when
   the LLM does the heavy lifting.

## What we will NOT claim

- ❌ "kinclaw scores X% on OSWorld" without the vision-only caveat
- ❌ "kinclaw is a general-purpose agent" based on this score alone
- ❌ "kinclaw on OSWorld is competitive with Anthropic Computer Use"
   if the gap is (e.g.) 12% vs 38% — that's a 3× gap that reflects
   architectural fit, not capability

## What we WILL claim

- ✅ "kinclaw's vision-only fallback scores X% on OSWorld" — the
  number is honest as long as the caveat is right next to it
- ✅ "kinclaw's strongest evidence of capability is on macOS native
  tasks (macbench), not Linux VM tasks (OSWorld)" — points readers
  at the right benchmark for the right question

## Setup (planned, not implemented)

OSWorld's [README](https://github.com/xlang-ai/OSWorld#installation)
on macOS:

```bash
# 1. VMware Fusion (free for personal use as of 2024-05)
brew install --cask vmware-fusion

# 2. OSWorld + its environment
cd benchmarks/osworld
./setup.sh   # ← TODO: implement
# - clones xlang-ai/OSWorld at a pinned commit
# - downloads OSWorld's Ubuntu VM image (~10 GB)
# - registers it with vmrun
# - installs Python deps for OSWorld's runner

# 3. Verify
vmrun -T fusion list  # should show the OSWorld Ubuntu vm

# 4. Run kinclaw on the first 10 OSWorld tasks
./run.sh AGENT=../../kinclaw TASKS=0-9
```

## Adapter design (planned, not implemented)

OSWorld's runner expects an Agent class with `predict(observation)`
returning an action. Bridging that to kinclaw means a small Python
shim:

```
benchmarks/osworld/adapter/
├── kinclaw_agent.py    Python class implementing OSWorld's Agent contract
│                       — calls out to `kinclaw -exec "$prompt"` per step
├── observation.py      Translate OSWorld's pyatspi a11y tree → screenshot
│                       (we only feed kinclaw the screenshot since it can't
│                        use Linux a11y meaningfully)
├── action.py           Translate kinclaw tool calls → OSWorld pyautogui actions
└── window_capture.py   Find + crop VMware Fusion window from macOS screen
                        (fed to kinclaw as the screenshot it operates on)
```

The trick is `window_capture.py`. kinclaw on macOS sees its own
desktop, including the VMware Fusion window. We need to:

1. Use sckit-go to capture only the VMware window (sckit supports
   window-targeted capture)
2. Tell kinclaw "this is your screen" — synthesizing input goes through
   VMware Fusion's CGEvent → guest input forwarder
3. Translate kinclaw's "click at (x, y)" to coordinates relative to
   the VMware window's content area

This is fragile (VMware window can be repositioned mid-run, scaled
differently per Retina, etc.). v1 will pin VMware Fusion to a known
position + size and refuse to run if the window moves.

## Soul

`souls/osworld.soul.md` (planned):
- Disable AX/ui claws entirely (only screen + input)
- Force "look at screenshot, decide one action, click/type, look again"
  loop
- Tight circuit breaker — Linux desktop tasks can spiral if the
  agent gets confused
- Hard step limit per task (OSWorld evaluates at end + during)

## Roadmap

```
v0.1   Adapter scaffolded (this README, no code)              ← here
v0.2   setup.sh + VMware Fusion automation; OSWorld VM boots cleanly
v0.3   Python shim — kinclaw_agent.py basic predict() loop
v0.4   window_capture + first task hand-tested end-to-end
v0.5   first run on 10 tasks; expect very low pass rate, debug
v0.6   first run on full 369 tasks; publish results.json with caveats
```

Each step roughly a day. The Python ↔ Go boundary is the messiest
part; we may bypass it by writing a Go shim that speaks OSWorld's
Python protocol via subprocess (fewer moving parts than embedding).

## Reference

- OSWorld GitHub: <https://github.com/xlang-ai/OSWorld>
- OSWorld leaderboard: <https://os-world.github.io>
- OSWorld-Verified blog (the cleaned-up 2025-07 release; report against this version):
  <https://xlang.ai/blog/osworld-verified>
- Anthropic CUA on OSWorld: search "Anthropic Computer Use OSWorld benchmark"
- This adapter intentionally targets OSWorld-Verified, not the
  original 2024-04 release. Numbers reported pre-cleanup are not
  directly comparable.
