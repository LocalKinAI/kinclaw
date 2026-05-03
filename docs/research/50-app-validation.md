# 50-App Validation: "5 claws cover any computer" hypothesis

**Generated**: 2026-04-27 00:37:29 -0700
**Probe**: `cmd/probe-ax/main.go` — walks AX tree to depth 8 via kinax-go, no LLM in loop
**Sample**: 50 apps from /Applications + /System/Applications, balanced across 6 categories

---

## TL;DR

| Tier | Count | % | Meaning for 5-claw control |
|---|---|---|---|
| 🟢 **AX-rich** | 44 | 88% | `ui` claw alone drives it |
| 🟡 **AX-shallow** | 3 | 6% | `ui` + `input` (cmd-keys / type-text) hybrid |
| 🟠 **AX-blank** | 3 | 6% | needs `record` + `screen` + vision |
| 🔴 **dead** | 0 | 0% | process didn't start (TCC / sandbox) |

**94% controllable today** (47 / 50 via `ui` ± `input`).
**88% pure-AX**, no input fallback needed (44 / 50).
**6% require vision** as primary path (3 / 50).

✅ **5-claw bet validated**. >90% of macOS apps yield to the kernel's primitives. The remaining tail is a vision/input combo away from full coverage.

## Per-category breakdown

### Apple Native Everyday (n=15)

🟢 rich=14 · 🟡 shallow=1 · 🟠 blank=0 · 🔴 dead=0

| Status | App | Nodes | Btn | TxtFld | MenuItm | Depth | Note |
|---|---|---:|---:|---:|---:|---:|---|
| 🟢 | `Notes` | 3831 | 23 | 13 | 190 | 8 | probe_timeout(5000ms) |
| 🟡 | `AddressBook` | 1964 | 0 | 4 | 0 | 8 | probe_timeout(5000ms) |
| 🟢 | `mail` | 623 | 16 | 2 | 495 | 8 |  |
| 🟢 | `Music` | 427 | 16 | 0 | 289 | 7 |  |
| 🟢 | `Photos` | 393 | 5 | 0 | 332 | 8 |  |
| 🟢 | `iCal` | 383 | 8 | 0 | 239 | 7 |  |
| 🟢 | `TV` | 372 | 10 | 0 | 245 | 8 |  |
| 🟢 | `reminders` | 351 | 9 | 2 | 270 | 8 |  |
| 🟢 | `MobileSMS` | 339 | 8 | 2 | 265 | 8 |  |
| 🟢 | `iBooksX` | 309 | 6 | 0 | 254 | 7 |  |
| 🟢 | `stocks` | 296 | 3 | 0 | 259 | 6 |  |
| 🟢 | `Maps` | 294 | 6 | 1 | 230 | 8 |  |
| 🟢 | `findmy` | 279 | 12 | 0 | 221 | 7 |  |
| 🟢 | `Home` | 256 | 4 | 0 | 210 | 8 |  |
| 🟢 | `weather` | 218 | 4 | 1 | 169 | 8 |  |

### Apple System / Utility (n=12)

🟢 rich=12 · 🟡 shallow=0 · 🟠 blank=0 · 🔴 dead=0

| Status | App | Nodes | Btn | TxtFld | MenuItm | Depth | Note |
|---|---|---:|---:|---:|---:|---:|---|
| 🟢 | `Safari` | 1186 | 51 | 2 | 1013 | 8 |  |
| 🟢 | `Preview` | 894 | 55 | 41 | 325 | 8 |  |
| 🟢 | `freeform` | 459 | 0 | 0 | 410 | 8 |  |
| 🟢 | `TextEdit` | 352 | 0 | 0 | 311 | 8 |  |
| 🟢 | `shortcuts` | 339 | 13 | 3 | 219 | 8 |  |
| 🟢 | `Stickies` | 298 | 3 | 2 | 256 | 6 |  |
| 🟢 | `QuickTimePlayerX` | 292 | 0 | 0 | 261 | 6 |  |
| 🟢 | `VoiceMemos` | 261 | 6 | 1 | 210 | 6 |  |
| 🟢 | `AppStore` | 256 | 14 | 1 | 187 | 7 |  |
| 🟢 | `clock` | 253 | 3 | 0 | 201 | 8 |  |
| 🟢 | `calculator` | 250 | 27 | 0 | 194 | 7 |  |
| 🟢 | `FaceTime` | 222 | 7 | 0 | 178 | 8 |  |

### Utilities folder (n=8)

🟢 rich=6 · 🟡 shallow=1 · 🟠 blank=1 · 🔴 dead=0

| Status | App | Nodes | Btn | TxtFld | MenuItm | Depth | Note |
|---|---|---:|---:|---:|---:|---:|---|
| 🟡 | `ActivityMonitor` | 4172 | 0 | 0 | 0 | 7 | probe_timeout(5000ms) |
| 🟢 | `ScriptEditor2` | 3864 | 8 | 366 | 0 | 8 | probe_timeout(5000ms) |
| 🟢 | `Terminal` | 420 | 21 | 4 | 338 | 6 |  |
| 🟢 | `DiskUtility` | 355 | 14 | 1 | 239 | 7 |  |
| 🟢 | `Console` | 348 | 8 | 1 | 255 | 7 |  |
| 🟢 | `SystemProfiler` | 333 | 7 | 51 | 183 | 7 |  |
| 🟢 | `ColorSyncUtility` | 234 | 15 | 1 | 188 | 6 |  |
| 🟠 | `screenshot.launcher` | 1 | 0 | 0 | 0 | 0 |  |

### Apple Pro / iWork (n=5)

🟢 rich=5 · 🟡 shallow=0 · 🟠 blank=0 · 🔴 dead=0

| Status | App | Nodes | Btn | TxtFld | MenuItm | Depth | Note |
|---|---|---:|---:|---:|---:|---:|---|
| 🟢 | `iWork.Numbers` | 1318 | 56 | 41 | 714 | 8 |  |
| 🟢 | `iWork.Pages` | 796 | 0 | 0 | 721 | 8 |  |
| 🟢 | `dt.Xcode` | 795 | 5 | 0 | 704 | 8 |  |
| 🟢 | `iWork.Keynote` | 786 | 1 | 0 | 704 | 8 |  |
| 🟢 | `iMovieApp` | 402 | 7 | 1 | 330 | 8 |  |

### Third-party Electron / cross-platform (n=6)

🟢 rich=4 · 🟡 shallow=1 · 🟠 blank=1 · 🔴 dead=0

| Status | App | Nodes | Btn | TxtFld | MenuItm | Depth | Note |
|---|---|---:|---:|---:|---:|---:|---|
| 🟢 | `todesktop.230313mzl4w4u92` | 494 | 3 | 0 | 437 | 8 |  |
| 🟢 | `microsoft.VSCode` | 494 | 0 | 0 | 449 | 8 |  |
| 🟢 | `google.Chrome` | 340 | 3 | 0 | 289 | 8 |  |
| 🟢 | `anthropic.claudefordesktop` | 220 | 0 | 0 | 195 | 6 |  |
| 🟡 | `hnc.Discord` | 34 | 0 | 0 | 22 | 5 |  |
| 🟠 | `us.zoom.xos` | 1 | 0 | 0 | 0 | 0 |  |

### Heavyweight / specialty (n=4)

🟢 rich=3 · 🟡 shallow=0 · 🟠 blank=1 · 🔴 dead=0

| Status | App | Nodes | Btn | TxtFld | MenuItm | Depth | Note |
|---|---|---:|---:|---:|---:|---:|---|
| 🟢 | `tencent.xinWeChat` | 253 | 3 | 0 | 218 | 6 |  |
| 🟢 | `io.tailscale.ipn.macos` | 156 | 1 | 0 | 134 | 8 |  |
| 🟢 | `baidu.BaiduNetdisk-mac` | 150 | 3 | 0 | 122 | 6 |  |
| 🟠 | `docker.docker` | 1 | 0 | 0 | 0 | 0 |  |

## Apps below the AX threshold

3 apps fell into 🟠 blank or 🔴 dead — these are the cases the 5-claw bet has to address with vision + input fallback.

- 🟠 **`docker.docker`** (Heavyweight / specialty): nodes=1, buttons=0
- 🟠 **`us.zoom.xos`** (Third-party Electron / cross-platform): nodes=1, buttons=0
- 🟠 **`screenshot.launcher`** (Utilities folder): nodes=1, buttons=0


## Methodology

**Probe** (`cmd/probe-ax/main.go`):
1. `open -gb <bundle_id>` — start process in background, no focus steal
2. Poll `kinax.ApplicationByBundleID` until reachable (8s timeout)
3. `osascript activate` to bring the app to foreground (REQUIRED — background-launched apps return AXApplication root with zero children)
4. Wait 1.5s for AppKit/Catalyst/Electron to draw windows
5. Walk AX tree depth-first to depth 8, count role distribution
6. Probe-walk capped at 5s (apps with >4000 nodes hit timeout — partial counts still reliable for category)

**Categorization rules**:
- `actionable = AXButton + AXTextField + AXMenuItem`
- 🟢 **rich**: `processOK AND nodes >= 50 AND actionable >= 5`
- 🟡 **shallow**: `processOK AND nodes >= 10` but below rich
- 🟠 **blank**: `processOK AND nodes < 10` (basically a menubar app or AX-hostile shell)
- 🔴 **dead**: process didn't start within 8s

**Why this categorization**: original "buttons >= 5" rule was wrong — Electron apps (VSCode, Cursor, Claude desktop) consistently have 200-500 nodes but 0-3 AXButtons because they render via HTML divs, not AppKit NSButton. Driving them means: keyboard shortcuts (input claw), AXStaticText readback, AXMenuItem navigation. So `actionable` counts buttons + text-fields + menu-items, and the absence of AXButton is a routing signal, not a failure.

**Sample selection**: 50 apps from /Applications, /System/Applications, /System/Applications/Utilities. Skewed toward Apple stock (35/50) — that's what's installed on this dev Mac. Third-party + heavyweight covers 10/50 — enough to detect Electron/Catalyst/native signal differences, not enough to claim "average Electron app".

**Limits**:
- Apps behind sign-in / TCC / first-launch modals show partial trees.
- "Buttons" doesn't include AXLink, AXCheckBox, AXPopUpButton, AXSlider, AXMenuButton.
- Single point-in-time probe; collapsed sidebars / closed sheets would expand richness.

## Raw data

- CSV: `/tmp/kinclaw-50app-probe/results.csv`
- Classified: `/tmp/kinclaw-50app-probe/classified.txt`
- Run log: `/tmp/kinclaw-50app-probe/run.log`
