# OSWorld Benchmark — 调研存档

**日期**: 2026-05-02
**调研者**: KinClaw / Pilot
**目的**: 理解 computer-use agent 圈子的标准 benchmark,判断 KinClaw 何时跑、为什么跑
**关联文件**: `turix-cua-2026-05.md` (竞品 TuriX 在该 benchmark 上 64.2% 全球第 3)

---

## TL;DR (修订 2026-05-02 晚)

OSWorld 是 computer-use agent 空间**唯一**有学术地位 + 跨平台 + execution-based 的 benchmark。NeurIPS 2024 paper,HKU Xlang AI 团队。**所有头部 agent 都报这个分**。两年从 12% 干到 64%,人类基线 72.36%。

**战略立场: OSWorld 就是标准,正面打,慢慢打。** 不绕道、不立自己 benchmark、不等 MLX 才跑。多季度路线 — Q3 2026 写 adapter 跑 baseline (30-40%),Q4 优化 scaffold (45-50%),Q1 2027 训 `kinclaw-actor` 自家模型 (55%+),Q2 跨平台奇袭,Q3-Q4 2027 冲 60%+ 进 SOTA top 5。

**核心洞察**: 30 视频不只是 marketing,**是 actor 模型的训练数据采集** — 真人 + KinClaw 操作的轨迹,正是训 GUI 模型的好原料。Marketing × 数据复利。

---

## 基本信息

| 项 | 数据 |
|---|---|
| Paper | arXiv 2404.07972 (NeurIPS 2024) |
| 团队 | HKU Xlang AI |
| 仓库 | https://github.com/xlang-ai/OSWorld |
| 官网 | https://os-world.github.io/ |
| 任务数 | 369 (+ 43 Windows 分析任务) |
| 平台 | Ubuntu / Windows / macOS |
| License | (开源,research-friendly) |
| 最新版本 | OSWorld-Verified (2025-07-28) |

---

## 任务结构

- **单 app 任务**: 268 个 (72.6%)
- **跨 app 工作流**: 101 个 (27.4%)
- **覆盖应用**: LibreOffice、GIMP、VS Code、Chrome、Terminal、Thunderbird、文件管理器、Calculator…
- **评估函数数量**: 134 个自定义脚本

## 评估方式 — execution-based ⚠️

这是 OSWorld 设计上最重要的一点。**不是 LLM 当裁判**,而是任务做完后**直接看 OS 真实状态**。

例子:
- task: "把 ~/Downloads/file.txt 重命名为 doc.md"
- 验证: `os.path.exists("/home/user/Downloads/doc.md") and not os.path.exists("/home/user/Downloads/file.txt")`
- 二值化结果

**为什么重要**:
- 避免 "LLM 评 LLM" 的循环 偏差
- 避免任务说服力 (agent 编出听起来对的步骤但实际没做对) 
- 数字可信,可复现,跨研究组对比

这是它成为 standard 的根本原因。

---

## SOTA 历史曲线 (戏剧性)

| 时间 | 最佳分数 | 说明 |
|---|---|---|
| 2024-04 (paper 发布) | 12.24% | 当时最强 VLM 也跌跌撞撞 |
| 2024-12 | ~30% | Claude Computer Use 发布 |
| 2025 中 | ~50% | UI-TARS / Aria-UI 等专门模型出现 |
| 2026-05 (现在) | **64.2%** (TuriX-CUA) | 当前 SOTA |
| **人类基线** | **72.36%** | 上限 |

**两年从 12% 干到 64%** — AI 进步最快子领域之一。

驱动力:
1. **VLM 本体变强** — Gemini 3、Claude 4、GPT-5 看截图能力质变
2. **专门 GUI 模型** — turix-actor、UI-TARS、Aria-UI 这种特化在点击精度上的训练

---

## 报过分的玩家 (圈内身份证)

- Anthropic Claude Computer Use
- OpenAI Operator
- **TuriX-CUA: 64.2% (第 3)** ← 我们直接对手
- 阿里 GUI-Owl
- ByteDance UI-TARS
- 各种学术论文

**没报 OSWorld 数字 = 在这个圈子没说话权**。这是硬通货。

---

## 基础设施

**不能在 host 机跑** — 任务会改文件、装东西、点 UI。必须沙箱。

支持的 provider:
- Docker (本地)
- VMware
- VirtualBox  
- **AWS** (并行,2025-07 后 1 小时跑完)

默认凭证:
- 本地 VM: `user` / `password`
- AWS: `osworld-public-evaluation`

跑步流程:
```bash
git clone https://github.com/xlang-ai/OSWorld
# 装 Docker / 准备 AWS 账号
# 写一个 agent adapter (吃 OSWorld task 格式,出 OSWorld action 格式)
# 跑 369 任务,可并行
```

**成本估算**:
- 单 task 平均 30-50 步,每步一次 LLM 推理
- 用 Claude Sonnet: 369 × 40 步 × ~5K tokens = 约 70M tokens ≈ **$200-400 USD 一次跑**
- 用 GPT-4o: 类似数量级
- 用 Gemini 3 Flash: 可能 $50-100
- 本地模型 (Ollama / MLX): 时间换钱,**几乎免费但可能跑两天**

---

## KinClaw 跑 OSWorld 需要做什么

技术上需要:

1. **Adapter 层**: 把 KinClaw 的 soul/skill 接口包成 OSWorld 期望的 agent interface
   - OSWorld agent 接口: 接收 task 描述 + 当前 screenshot,返回 action (mouse/keyboard/finished)
   - KinClaw 等价: soul 收 task + screen claw 看屏幕 + input claw 出动作
   - **半天到一天工作量**

2. **Linux VM 支持**: OSWorld Mac 任务很少 (主要 Ubuntu)。要拿好分,得让 KinClaw 在 Linux 跑
   - 我们目前 macOS-only
   - sckit-go / kinax-go / input-go 都是 mac-specific
   - **Linux 适配 = 巨大工作量**,不是为了跑 benchmark 单独做的

3. **跑分**:
   - 第一轮估计 30-40% (用 Claude Sonnet 当 brain,无专门 actor)
   - 优化 prompt + skills 后能到 50%
   - 要破 60% 大概率需要专门 actor 模型

---

## ~~战略判断 — 现在不跑的理由~~ (作废 2026-05-02 晚)

> 此节原结论"暂不跑"已被修订。理由如下:
> - "立自己 benchmark" 是回避主战场,圈子不认就没意义。
> - "等 MLX 本地推理才跑"错失 baseline 数据复利期 — 早跑早知道差距在哪。
> - "Mac-only 等待"违反"要赢必做 Linux/Windows"的事实。
>
> **新立场: 正面打,慢慢打,OSWorld 就是标准。** 见下文路线图。

---

## 🎯 攻顶路线图 (今晚定稿 2026-05-02)

### 核心洞察 — 怎么打

1. **Actor 模型是护城河,不是 Brain** — TuriX 强不在 Brain (Gemini 3 / 通用 VLM 谁都能换),强在自训 `turix-actor` 做精确点击。**这是必须自己训的零件**。Brain 谁强用谁。

2. **Skills/scaffold 是 leverage** — soul + super-skill + Go 单二进制是我们独有"框架优势"。同样的 brain + actor,我们的脚手架能多挤 5-10 分。

3. **跨平台是奇袭** — TuriX Windows 是"screenshot only"降级版 (无 UIA)。我们做 `sckit-win + kinax-win`,Windows 子集分数能直接打过他们。Linux 同理。

4. **30 视频 = 训练数据采集** — 真人 + KinClaw 操作的轨迹,正是 GUI 模型训练原料。Marketing × 数据复利。**这是之前没看出来的关键**。

### 6 阶段路线 (multi-quarter)

| 阶段 | 时间窗 | 目标 | 前置 | 故事 |
|---|---|---|---|---|
| **0. 视频** | Q2 2026 (现在) | 30 个 mac demo | — | 品类教育 + 数据采集 |
| **1. Adapter** | Q3 2026 早 | 写 OSWorld adapter,接通 KinClaw | 30 视频开始 | "KinClaw 上 OSWorld 跑道" |
| **2. Baseline** | Q3 2026 中 | 用 Claude 4 / Gemini 3 当 brain,首轮 **30-40%** | Adapter 通 | "首个 Go-native CUA 的 OSWorld 数字" |
| **3. Scaffold 优化** | Q4 2026 | 调 soul + skills + few-shot,挤到 **45-50%** | Baseline 数据 | "soul 系统的杠杆效应" |
| **4. 自训 kinclaw-actor v1** | Q1 2027 | M 系列 + MLX,SFT 7B GUI 模型,挤到 **55%+** | MLX 通,30 视频数据 | "本地推理 + 自训 actor + 55%" |
| **5. 跨平台奇袭** | Q2 2027 | `sckit-win` + `kinax-win` (UIA),Windows 子集打透 | KinClaw Windows 移植 | "Windows 不是 screenshot,要 UIA" |
| **6. 决战** | Q3-Q4 2027 | 全 369 task,**60%+** 进 SOTA top 5 | 上面全到位 | "Go-native + local + cross-platform 第一名" |

### 立即可做 (不用等 30 视频跑完)

**Adapter 是冷启动** — 1-2 天工作量,可以现在就写。先跑通 hello world (跑 1 个 task 看 KinClaw 能不能吃 OSWorld 接口),哪怕只跑通不调分。

技术上:
- OSWorld agent interface: 接 task 描述 + screenshot,出 action (mouse/keyboard/finished)
- KinClaw 等价: soul 收 task + screen claw 看屏幕 + input claw 出动作
- 中间薄壳一层翻译就行

存档位置建议: `kinclaw/cmd/kinclaw-osworld/` 或独立 repo `kinclaw-osworld-adapter`

### 不变的部分

- $99 Apple Dev cert 下来后能 distribute .app,跟 benchmark 数字形成产品闭环
- MLX 通了之后训 `kinclaw-actor` 就有路径 (M 系列本地训 7B 慢但可行)
- LocalKin 平台叙事 ("local-first AI runtime") 跟 OSWorld 数字配合是杀手组合

---

## 周边 benchmark (顺便了解)

| Benchmark | 焦点 | 何时考虑 |
|---|---|---|
| **OSWorld** | 通用桌面 (跨平台) | **现在,主战场** |
| OSWorld-Human | 效率 (步数比人类多多少) | OSWorld baseline 后 |
| OSUNIVERSE | OSWorld 升级版 (2025 出) | OSWorld 站稳后跟进 |
| WebArena | 纯 Web 任务 | 跟 browser_session 配合时 |
| WindowsAgentArena | Win 专属 | KinClaw Windows 出来后 |
| ~~Mac-specific bench~~ | ~~机会自立标准?~~ | **作废** — OSWorld 就是标准,不绕道 |

**~~深度想法: 立自己 benchmark~~** (作废 2026-05-02 晚): 自立标准是回避主战场 — 圈子不认就没意义,永远主场不等于永远第一。**OSWorld 是标准,正面打**。

---

## 监控清单

定期 (每月) 看一次:
- [ ] OSWorld 排行榜更新 (os-world.github.io)
- [ ] 新 SOTA 模型 (UI-TARS-2、turix-actor 升级…)
- [ ] OSWorld-Verified 是否再次更新
- [ ] 出新 benchmark? (OSUNIVERSE 普及程度)

---

## 总结一句话 (修订 2026-05-02 晚)

**OSWorld 是空间硬通货 = 标准 = 不绕道。我们打不过,要慢慢可以打过 — 6 阶段路线 (adapter → baseline → scaffold → kinclaw-actor → 跨平台 → 决战),18 个月时间窗口。30 视频不只是 marketing,是训 actor 模型的数据采集。Q3 2026 跑出第一个 30-40% baseline,Q4 2027 冲 60%+ 进 SOTA top 5。正面打,慢慢打,OSWorld 就是标准。**
