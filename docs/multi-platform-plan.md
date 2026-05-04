# Multi-platform integration plan

**日期**: 2026-05-03 (深夜)
**状态**: strategy 已锁,phase 0/1 落地中
**作者**: Jacky + Pilot 对话过程中
**关联**:
- [`docs/roadmap.md`](roadmap.md) — KinClaw 主线 roadmap
- [`docs/spotlight-shell-plan.md`](spotlight-shell-plan.md) — KinClaw Mac 子项目计划
- `kinclaw-mac` repo — Swift 原生 macOS dock
- `~/.localkin/research/perplexity-personal-computer-2026-05.md` — 同空间 $40b 对手已经多平台

---

## TL;DR

**两层架构, 三个仓 max, mac premium + linux/win 用 web UI**。

```
       kinclaw (Go kernel) — 一个仓 + build tags 跨 mac/linux/win
              │
              ├── 5 claws (per-platform impl)
              ├── HTTP API :5001 (跨平台一致)
              └── bundled web UI (kinclaw serve)

       Two shells:
       ├── kinclaw-mac     Swift 原生 (旗舰 macOS, Proprietary)
       └── kinclaw-pal     Tauri 跨平台 (Linux/Win, Apache-2.0)
                            ⚠️ 只在 Phase 4 才做,需求驱动
```

**核心战略选择**:
- **kernel 单 codebase + build tags**, 不要 fork-per-platform (TuriX 的 multi-agent-windows / multi-agent-linux 分支会变维护噩梦)
- **Mac 优先 native** (Swift 已投入), Linux/Win 优先用 web UI (zero 投入)
- **Tauri shell 作为 Phase 4 选项**, 不是 Phase 1 必须
- **3 个仓 max**, 不要 4 (kinclaw + kinclaw-mac + future kinclaw-pal)

---

## 当前状态 (2026-05-03)

| 件 | 状态 | 仓 | License |
|---|---|---|---|
| `kinclaw` Go kernel | ✅ macOS 工作 | github.com/LocalKinAI/kinclaw (public) | Apache-2.0 |
| `kinclaw-mac` Swift dock | ✅ M0-M10 + r1-r10 polish 全 ship | github.com/LocalKinAI/kinclaw-mac (private) | Proprietary |
| `kinclaw serve` web UI | ✅ v1.8 起内置, 跨浏览器工作 | bundled in kinclaw | (跟随 kinclaw) |
| Linux kernel | ❌ 5 claws 没 port | — | — |
| Windows kernel | ❌ 5 claws 没 port | — | — |

---

## 架构 — 两层

```
┌──────────────────────────────────────────────────────────┐
│   Layer 1: kinclaw kernel (Go)                            │
│   单 binary, 所有平台的引擎                                │
│                                                           │
│   ├─ Souls        (.soul.md, 跨平台同款)                   │
│   ├─ Skills       (SKILL.md, 跨平台同款)                   │
│   ├─ Memory       (~/.localkin/memory.db, 跨平台共享)      │
│   ├─ Sessions     (~/.kinclaw/sessions/, 跨平台一致)       │
│   ├─ HTTP API     (:5001, 跨平台一致, kinclaw-mac/web 都接) │
│   ├─ web UI       (pkg/server/index.html, 跨浏览器工作)     │
│   └─ 5 claws      (per-platform Go impl 用 build tags)     │
│                                                           │
│   编译: GOOS=darwin/linux/windows go build → 3 个 binary   │
└────────────────────┬──────────────────────────────────────┘
                     │
       ┌─────────────┼─────────────────────┐
       ▼             ▼                     ▼
   ┌────────┐   ┌──────────┐         ┌──────────────┐
   │ macOS  │   │ Linux    │         │ Windows      │
   │ claws  │   │ claws    │         │  claws       │
   │ ✅ done │   │ Q3 2026  │         │  Q3 2026     │
   └───┬────┘   └─────┬────┘         └─────┬────────┘
       │              │                    │
       ▼              ▼                    ▼
┌─────────────┐  ┌────────────────────────────────┐
│ Layer 2a:   │  │ Layer 2b:                       │
│ kinclaw-mac │  │ kinclaw serve 自带 web UI       │
│ Swift native│  │ http://localhost:5001 浏览器打开 │
│ 旗舰 + 付费   │  │ 免费 + 即时跨平台              │
│ Proprietary │  │ Apache-2.0 (kernel 一部分)     │
└─────────────┘  └────────────────────────────────┘
                            │
                            ▼ (Phase 4 if 需求够)
                  ┌────────────────────┐
                  │ kinclaw-pal (Tauri) │
                  │ Linux + Windows     │
                  │ 跟 web UI 同款体验    │
                  │ Apache-2.0          │
                  └────────────────────┘
```

---

## Layer 1: kernel — 单 codebase, build tags 跨平台

**绝对不要 fork-per-platform**。TuriX 那种 mac/linux/windows 分支在他们 7 个 branch 上已经看出维护噩梦 (功能漂移, bug 难追, release 乱套)。

正确做法 — Go build tags:

```
kinclaw/
├── pkg/sckit/                  ← screen capture
│   ├── screen_darwin.go         //go:build darwin
│   ├── screen_linux.go          //go:build linux
│   ├── screen_windows.go        //go:build windows
│   └── screen_common.go         // shared types
│
├── pkg/kinax/                  ← UI tree
│   ├── ax_darwin.go             // macOS Accessibility framework
│   ├── ax_linux.go              // AT-SPI / atspi-rs
│   └── ax_windows.go            // Windows UI Automation (UIA)
│
├── pkg/input/                  ← mouse/keyboard
│   ├── input_darwin.go          // CGEventCreate*
│   ├── input_linux.go           // xdotool / wlroots
│   └── input_windows.go         // SendInput
│
├── pkg/kinrec/                 ← screen recording
│   ├── kinrec_darwin.go         // ScreenCaptureKit
│   ├── kinrec_linux.go          // ffmpeg-x11grab
│   └── kinrec_windows.go        // ffmpeg-gdigrab / D3D11
│
└── pkg/web/                    ← browser automation (跨平台已 work)
    └── (Playwright Go,跨平台原生)
```

每个 claw 的 platform-specific 文件有 build tag, Go 自动选。`go build ./cmd/kinclaw/` 在 mac 上输出 mac binary, 在 Linux 上输出 Linux binary。**接口一致** — soul / skill 看到的 `screen.capture()` 行为一样, 底下不同。

### Linux 5-claw 选型 (建议)

| Claw | 选 | 备注 |
|---|---|---|
| sckit | scrot 或 `gnome-screenshot` 命令 / x11 lib | Wayland 单独考虑 |
| kinax | atspi-rs (Rust) via cgo or 直 D-Bus | accessibility 主流方案 |
| input | xdotool 命令 / 直 X11 lib | Wayland 受限 |
| kinrec | ffmpeg + x11grab / wf-recorder | |
| web | Playwright | 已跨平台 |

### Windows 5-claw 选型

| Claw | 选 | 备注 |
|---|---|---|
| sckit | Windows Graphics Capture API / GDI BitBlt | |
| kinax | Windows UI Automation (UIA) via interop | 标准 |
| input | SendInput / Win32 API | |
| kinrec | ffmpeg gdigrab 或 Media Foundation | |
| web | Playwright | 已跨平台 |

---

## Layer 2: shells

### Layer 2a: kinclaw-mac (Swift 原生 — 旗舰 + 付费)

- **已经做了**: M0-M10 + 10 轮 polish, 31+ commits
- **定位**: macOS 旗舰体验, future $5-10/月 订阅候选
- **License**: Proprietary (跟 localkin-ios 一致)
- **特点**: NSPanel + 玻璃 blur + ⌘⌥K 全局热键 + 拖拽 attachment + 多 session
- **30 视频 marketing 直接对它**

### Layer 2b: kinclaw serve web UI (跨浏览器, 免费, 跨平台)

- **已经存在**: kinclaw 主仓 v1.8+ 自带 (`pkg/server/index.html` ~1700 行)
- **定位**: Linux/Windows 用户的**主要 UI**, mac 用户的备选
- **License**: Apache-2.0 (跟 kernel 一起)
- **特点**: 任何浏览器打开 `http://localhost:5001` 即用, 已经支持 voice / SSE / soul switcher / replay
- **足够好** — Manus / Operator 也是 web UI 起家

### Layer 2c (Phase 4, 可选): kinclaw-pal Tauri shell

**只有当**满足下述触发条件再做:

- Linux/Windows 月活用户 > 1000
- 有 issue / discussion 明确要求 native window experience (不是 web UI)
- 有社区 PR 主动来贡献 Linux/Win 改进

**满足时**才开 `kinclaw-pal`:
- Tauri (Rust core + Web frontend) — 一份代码同时输出 Linux + Windows binary
- 复用 `kinclaw serve` 的 web UI 直接包进 native window
- 加 Linux/Win 平台特定: 全局热键, system tray icon, autostart
- 不用 Electron — 200MB Chromium bundle 跟 KinClaw 单二进制气质相反, Tauri ≈ 5MB

---

## 仓库布局 — **3 个 max**

```
~/Documents/Workspace/
├── kinclaw                ✅ Go kernel (mac), Q3 2026 + linux/win
│                          public, Apache-2.0
│                          → github.com/LocalKinAI/kinclaw
│
├── kinclaw-mac            ✅ Swift macOS dock
│                          private, Proprietary
│                          → github.com/LocalKinAI/kinclaw-mac
│
└── kinclaw-pal            🔜 Phase 4 if 需求够
                            public, Apache-2.0 (建议)
                            → github.com/LocalKinAI/kinclaw-pal
```

**关键: 不要再开 kinclaw-linux / kinclaw-windows / kinclaw-cli 之类的额外仓**。
- kernel 单仓搞定 build tags
- shell 只有 2 个 (mac native + tauri 跨平台)

---

## 协议层 — 跨 shell 必须一致的 contract

所有 shell (kinclaw-mac, web UI, future kinclaw-pal) **依赖同一组协议**。
变协议 = 跨产品 breaking change, 谨慎。

| 协议 | 在哪定义 | 跨 shell 一致? | doc 状态 |
|---|---|---|---|
| HTTP API `:5001/api/*` | `kinclaw/pkg/server/server.go : Event 类型 + handlers` | ✅ 已经 | 散在代码注释里, 该出独立 spec |
| Soul 格式 (.soul.md YAML+MD) | `kinclaw/souls/*.soul.md` | ✅ 已经 | **🚨 没正式 spec** |
| Skill 格式 (SKILL.md) | `kinclaw/skills/*/SKILL.md` | ✅ 已经 | **🚨 没正式 spec** |
| `~/.kinclaw/` 数据布局 | 散在 supervisor/server 代码 | ✅ 已经 | **🚨 没正式 doc** (上面有简单 README 段) |
| Memory schema (SQLite) | `pkg/memory/memory.go` | ✅ 已经 | 代码即 doc |
| Session JSON 格式 | `kinclaw-mac/Models/ChatSession.swift` | **❌ 还在 mac 仓** | **🚨 该挪到 kernel** |
| Cloud agent catalog | `kinclaw-mac/Models/CloudAgentCatalog.swift` | **❌ 只 mac 有** | **🚨 该挪到 kernel** |
| Tool / Event 类型 | `kinclaw/pkg/server/server.go` | ✅ 已经 | 已有 |

### 优先该 doc 化的 4 件 (短期 todo)

1. **`docs/spec/soul.md`** — soul 格式正式定义 (frontmatter 字段, `permissions`, `boot`, `model`, etc.)
2. **`docs/spec/skill.md`** — SKILL.md 格式正式定义 (frontmatter, command 字段, schema, env)
3. **`docs/spec/data-layout.md`** — `~/.kinclaw/` 完整目录树 + 每个文件用途
4. **`docs/spec/api.md`** — HTTP API 完整 spec (endpoint / payload / event types)

这 4 个文档**让 Linux/Windows shell 实现者 (包括未来社区贡献者) 知道契约边界**。

### Session 格式该挪到 kernel

当前 `kinclaw-mac/Models/ChatSession.swift` 定义了 session JSON shape。这是 product-specific 状态, 但 future Tauri shell / 跨设备同步 / kinclaw kernel 自己都该读得懂。

**建议**: 把 session JSON 的 spec 加到 `kinclaw/docs/spec/sessions.md`, kernel 提供一组可选的 helpers (`kinclaw sessions list / show / clear`) 让任何 shell 复用。

---

## 命名 — 主品牌 KinClaw 不变, 后缀打 platform

| 内部代号 | 用户看见 | 跟谁对标 |
|---|---|---|
| `kinclaw` (Go binary) | "kinclaw" CLI | docker / git |
| `kinclaw-mac` (Swift app) | **KinClaw Mac** | VS Code (Mac) |
| `kinclaw serve` web UI | **KinClaw Web** 或 "the dashboard" | localhost:8080 类 |
| `kinclaw-pal` Tauri (future) | **KinClaw Linux** / **KinClaw Windows** | Slack Linux / Spotify Windows |

跟 VS Code 同款思路: 主品牌不分, 平台后缀只在不得不区分时打出来。**用户搜 "KinClaw" 看到产品页, 然后选自己的 OS**。

---

## License 分层

| 层 | License | 为啥 |
|---|---|---|
| kernel + web UI | **Apache-2.0** | 开源吸纳贡献, 被任何 LocalKin 产品复用 |
| kinclaw-mac | **Proprietary** | 旗舰 + 未来付费层候选 |
| kinclaw-pal (future) | **Apache-2.0** | Linux 群体很少付费, 开源吸纳社区 |

跟 TuriX 现在 (开源核心 + 付费 SuperPower app) 对齐, 但**Mac 也 native + premium** 比 TuriX 更有差异化。

---

## Phase 排序

| Phase | 时间窗 | 做啥 | 依赖 |
|---|---|---|---|
| **0** | 2026-04 ~ 5 月初 | KinClaw Mac M0-M10 ship, 31 commits | ✅ 已完成 |
| **1** | 2026 5月底 | doc spec layer (soul / skill / data-layout / api / sessions) 进 `kinclaw/docs/spec/` | 跟 30 视频并行 |
| **2** | 2026 Q3 | Linux 5 爪 ports + 在 Linux 验证 web UI | 30 视频跑完 |
| **3** | 2026 Q4 | Windows 5 爪 ports | Phase 2 通了 |
| **4 (可选)** | 2027 看市场 | Tauri shell `kinclaw-pal` for Linux+Win | 月活 > 1000 + 社区呼声 |

**不在排序里**: 加额外 platform shell 仓 (违背"3 仓 max"), 自训 model (osworld 路线那个), windows native shell (违背 Tauri 选择)。

---

## 决策点 — 为啥这条路

### 为啥不每平台一个仓?

**TuriX 的反例**: 7 个分支 (main / mac_legacy / mac_mcp / multi-agent-linux / multi-agent-windows / windows_mcp / windows-legacy)。从外面看就是混乱, 内部维护一定灾难。Go build tags 是工业标准做法 (k8s / docker / cilium 全部这么干), 没必要重新发明。

### 为啥 Mac 不用 Tauri?

- kinclaw-mac 已经投入 31 commits + 10 轮 polish + native NSPanel 玻璃 blur + ⌘⌥K 全局热键 + 红绿灯 traffic lights, 这些 Tauri 给不了或者很别扭
- macOS 用户会比较 kinclaw-mac vs Spotlight / Raycast / Alfred — 这些都是原生, native bar 是入场券
- 30 视频拍 native 才有"产品感"

### 为啥 Linux/Win 不立刻 native?

- solo founder, 时间是最稀缺资源
- web UI 已经有了 + 跨平台自带工作
- Tauri shell 是"再做一个产品", 至少 1 个月工作量
- 等到真的有 Linux/Win 用户社区 + 反馈 + 贡献再投入, 不是 over-engineer

### 为啥 web UI 够用?

- Manus / OpenAI Operator / TuriX SuperPower 都是 web UI 起步
- KinClaw Mac 跟它们的差异化是 Mac native + 不是 web UI 必须
- Linux/Win 用户基础低, 短期不值得为他们做 native
- 真做了, 就是 Tauri 包一层, 不是 from scratch

---

## 监控清单

每月看:
- [ ] Linux/Windows 用户 issue / discussion 数量 (Phase 4 触发条件)
- [ ] kinclaw kernel 跨平台 build 是否过 (CI matrix)
- [ ] Soul/skill spec doc 是否被外部参考过 (gitsearch / community PR)
- [ ] kinclaw-mac vs Tauri 路线如果有人质疑, 回头看这份文档

每季度看:
- [ ] Phase 排序是否要调整 (osworld 路线 vs 多平台 vs 都做)
- [ ] License 分层是否要变 (kinclaw-mac 是否 open)
- [ ] 仓库数是否还稳定在 ≤ 3

---

## 一句话总结

**两层架构 (kernel + shells), 单 codebase 跨平台 (Go build tags), 三个仓 max (kinclaw + kinclaw-mac + future kinclaw-pal), Mac premium native + Linux/Win 用 web UI (够用) + Tauri 是 Phase 4 选项 (需求驱动)。比 TuriX 的 fork-per-platform 干净一档, 比 Manus / Perplexity 的"全 web 不分平台"多一档 native 旗舰。Solo founder 唯一可执行的路径 — 拒绝 over-engineer, 拒绝 4 仓, 拒绝立刻投 Linux/Win native, 但骨架今天就立清楚, 让 Q3 2026 做 kernel ports 时不必重新设计。**
