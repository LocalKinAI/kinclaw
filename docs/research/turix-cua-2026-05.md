# TuriX-CUA 调研 — 同空间最强对手分析

**日期**: 2026-05-02
**调研者**: KinClaw / Pilot
**目的**: 复盘 macOS computer-use agent 空间最像 KinClaw 的开源项目,提炼可学习点
**仓库**: https://github.com/TurixAI/TuriX-CUA
**官网**: https://turix.ai

---

## TL;DR

TuriX-CUA 是 KinClaw 在公开开源世界的**最近邻居**。9 个月内 2.8k 星,刷到 OSWorld 64.2% 全球第 3。架构选择跟我们大量收敛 (markdown skills、看屏幕点鼠标),但他们已经**训了自家模型** (`turix-actor`) 并且 **Linux/Windows 分支已经在开发**。

意外发现:他们也叫 "claw" — 上层 orchestrator 名为 **Clawdbot / OpenClaw**,TuriX 是它的执行 skill。命名正交但收敛,说明这条路确实是行业共识。

**判断**: 他们不是威胁是地图。我们的差异化护城河在 (1) Go 单二进制 (2) super-skill 吸收速度 (3) Soul 系统优雅度。他们的护城河在 (1) 自训模型 (2) OSWorld 公开数据 (3) 已经动 Linux/Windows。

---

## 仓库数据

| 维度 | 数据 |
|---|---|
| Stars | 2,816 |
| Forks | 288 |
| 创建日期 | 2025-08-04 |
| 最近更新 | 2026-05-02 (今天活跃) |
| 主语言 | Python 3.12 (97.8%) |
| License | MIT |
| Repo size | ~146 MB |
| 219 commits | main 分支 |
| 分支数 | **7 个** |
| 当前最新 release | v0.4 (2026-03-06) |

**分支清单**:
- `main` (Mac 主线)
- `mac_legacy`, `mac_mcp`
- `multi-agent-windows` ← **关键**
- `multi-agent-linux` ← **关键**
- `windows_mcp`, `windows-legacy`

**Release 节奏**:
- v0.1 (2025-08-04) - 首版
- v0.2 (2025-12-01) - 模型升级
- v0.3 (2026-01-27) - DuckDuckGo + Ollama + memory compression + Skills
- v0.4 (2026-03-06) - brain-actor multi-agent 模式
- v3.0.0-alpha (2026-04-08) - SuperPower 桌面 app

平均 2-3 个月一个 minor。我们 v1.x 节奏更快 (周级),但 release 力度小。

---

## 公司/产品全貌

不是单一开源项目,是**完整产品矩阵**:

```
turix.ai (官网)
 ├─ TuriX-CUA (开源核心,Apache MIT)
 ├─ TuriX SuperPower 3.0 (闭源桌面 app, all-in-one)
 │   ├─ TuriX CUA (这个开源仓库)
 │   ├─ TuriX CLI
 │   ├─ TuriX-work (办公自动化)
 │   └─ TuriX-code (代码自动化)
 ├─ turix-actor 模型 (闭源,turixapi.io 提供)
 ├─ turix-brain 模型 (闭源,turixapi.io 提供)
 └─ TuriX Parallelum (multi-role 框架,turix.ai/research/)

OpenClaw / Clawdbot (上层 orchestrator)
 └─ TuriX 作为 skill 接入 (your_dir/clawd/skills/local/turix-mac/)
```

**商业模式推测**:
- 开源 CUA 引流
- 闭源模型 API (turixapi.io) 收钱
- SuperPower 桌面 app 可能免费 + 付费层

跟 LocalKin 矩阵高度对称:
- KinClaw / browser_session = TuriX-CUA
- LocalKin Platform = Clawdbot
- (我们没有 turix-actor 等价物 ← **缺口**)
- (我们没有 SuperPower 等价物 ← **缺口**)

---

## 技术栈

```
Python 3.12
LangChain (ChatOpenAI / ChatGoogleGenerativeAI / ChatAnthropic / ChatOllama)
Safari WebDriver (浏览器自动化)
macOS Accessibility framework (AX)
Conda env (推荐)
```

**部署体验对比**:

| | TuriX | KinClaw |
|---|---|---|
| 安装 | `conda create + pip install -r` | `brew install kinclaw` |
| 二进制 | 无 (Python 脚本 + 依赖) | 单二进制 (~30MB) |
| 冷启动 | 秒级 (LangChain 初始化重) | 毫秒级 (Go) |
| 配置 | `examples/config.json` (4 个 LLM 角色) | `.soul.md` (单一文件) |
| 入口 | `python examples/main.py` | `kinclaw` |
| 用户门槛 | 需要懂 Python + conda | 装上就能跑 |

**这是我们最大的体验优势**。普通用户不会折腾 conda env。

---

## 架构 — Multi-Model 设计

```
Brain LLM       ← 看屏幕、推理下一步、维护上下文
Actor LLM       ← 把"点这里"翻译成精确坐标 (turix-actor 强烈推荐)
Planner LLM     ← (可选) 高层任务分解
Memory LLM      ← 跨步上下文压缩
```

每个角色可以独立换模型。`config.json`:

```json
{
  "brain_llm":   { "provider": "...", "model_name": "...", "api_key": "...", "base_url": "..." },
  "actor_llm":   { ... },
  "memory_llm":  { ... },
  "planner_llm": { ... },
  "agent": {
    "use_plan": true,
    "use_skills": true,
    "skills_dir": "skills",
    "skills_max_chars": 4000,
    "task": "open system settings, switch to Dark Mode"
  }
}
```

**对比 KinClaw 的单 soul 设计**:

KinClaw 把所有职责压在一个 soul 里 (一个 LLM 看屏幕 + 决策 + 执行)。TuriX 拆成 4 个独立 LLM 协作。

**优劣**:
- 多模型: 每角色可优化、可换便宜小模型,但调度开销 + 推理路径长
- 单模型 (我们): 简单、低延迟、但要求模型本身够全能

**学习点**: 当我们能跑本地模型时 (MLX 通了之后),可以考虑 Brain (大) + Actor (小,座标专用) 拆分。**先标记,不立即做**。

---

## 推荐模型清单

| 角色 | 推荐 |
|---|---|
| Brain | Gemini-3-pro (smartest) 或 turix-brain |
| Actor | **turix-actor (强烈推荐)** ← 自家训的护城河 |
| Memory | turix-brain |
| Planner | Gemini-3-flash (快够用) |

也支持 Ollama 本地 (llama3.2-vision 测过)。OpenAI-compatible endpoint 通用。

**关键洞察**: 他们公开推荐自家 turix-actor 当 actor — 说明这个特化模型是产品差异化核心。Brain 可以是任意 VLM,但精确点击需要专门模型。

---

## OSWorld 公开成绩

- **OSWorld (50 步任务): 64.2% 成功率 — 全球第 3**
- **macOS 内部 benchmark: 80%+**

我们的内部数据 (Genesis loop):
- 94% AX 准确率
- 80% task 完成率

**问题**: 我们的数字没有第三方 benchmark 背书。要可比,得自己跑 OSWorld。

---

## Skills 系统 — 设计完全收敛 ⚠️

TuriX skill 格式:

```markdown
---
name: skill-identifier
description: When to use this skill
---
# Skill Title
[Execution guidance for the Brain model]
```

KinClaw SKILL.md 格式 (本质相同):

```markdown
---
name: skill-name
description: ...
command: ...
---
# 说明
...
```

**两队独立得到同一答案** = markdown-as-spec 是正确的设计选择。这一点是我们方向上最大的验证。

差异:
- TuriX skills 是给 Brain LLM 读的 *指导文档* (LLM 自己执行)
- KinClaw skills 既能是 *指导文档* 也能是 *可执行命令* (`command:` 字段)
- KinClaw 的 super-skill 模式 (薄壳 + 厚肉) 让我们能半天吸收 91K 星 OSS,这是他们没有的

---

## OpenClaw / Clawdbot 发现 (重要 🚨)

仓库里有 `OpenCLaw_TuriX_skill/` 目录,内含 `SKILL.md` (12.9 KB) 描述把 TuriX 接入 Clawdbot 的方法:

> "TuriX acts as the 'eyes and hands' for Clawdbot"

文件路径约定: `your_dir/clawd/skills/local/turix-mac/`

**OpenClaw / Clawdbot 是什么**:
- 看起来是个上层 agent orchestrator,类似 Claude Desktop 但带 skill 市场
- TurixAI 把 TuriX 注册成 Clawdbot 的一个 macOS 执行 skill
- Lobehub 上有 "openclaw-skills-turix-cua" 条目 (访问 403,可能要登录)

**命名碰撞**:
- 他们: Clawdbot (Claude + claw 的合成)、OpenClaw (skill 市场)、turix-actor
- 我们: KinClaw (Kin + claw)、KinKit、Soul 系统

**两队都选了 "claw" 隐喻**。computer-use agent → 抓取/点击 → claw,这个 metaphor 收敛是不可避免的。

**应对**:
- 不改名,KinClaw 已经在我们生态里 ($99 cert / GH releases / 域名)
- 在 README 显眼处加 differentiation 一句话:"KinClaw 不是 Clawdbot,KinClaw 是 LocalKin 家的 Go-native 单二进制 macOS agent"
- 战略上 LocalKin 平台对应他们 Clawdbot,我们的位置是平行的

---

## 训练披露 — 不公开 ❌

`turix.ai/research/` 上提到:
- TuriX Parallelum (multi-role 框架,长 horizon 任务)
- TuriX VLM (action planning + click precision)
- 链接 "Read Technical Report" (没爬到内容)

**没有公开**:
- 训练数据 (来源、规模)
- 训练方法 (RL? SFT? DPO?)
- 模型尺寸
- 架构细节

**意味着**: 他们把训练当核心商业秘密。我们如果将来要训自家模型,得自己摸索 — 没有现成 paper 抄。但他们公开 OSWorld 64.2% 这个数字,说明他们的方法 work。

**建议**: 把这个仓库的 commit 历史定期扫一下,看他们模型迭代节奏 (v0.2 "compatible with our latest model" 这种 changelog 是信号)。

---

## Linux / Windows 进度 (我们的紧迫信号)

他们已经有 4 条非 Mac 分支:
- `multi-agent-windows`
- `multi-agent-linux`
- `windows_mcp`
- `windows-legacy`

`multi-agent-windows` 分支特征:
- 有 `OpenCLaw_TuriX_skill/scripts/run_turix.ps1` (PowerShell)
- "There is no use_ui parameter in the windows version, the state is only a screenshot" — Windows 版降级到纯截图 (没 AX 树等价物用)

**意味着**:
- 他们的 Windows 实现是"截图 only",没用 Windows UIA。这是个**降级**实现,精度会比 Mac 差。
- Linux 分支没读到细节,但存在意味着在做。

**对我们的启示**:
1. Windows 真要做的话,基础设施层 = 截图 + Windows UIA (类似 sckit-go + kinax-go on Windows)
2. 如果只做截图版,我们不会比他们好
3. 5 爪架构在 Windows 上需要 win 等价物:`sckit-win` `kinax-win` `input-win` `kinrec-win`
4. 他们已经动手 = 我们做 Windows 的窗口期变小

---

## SuperPower 3.0.0-alpha (April 2026)

闭源桌面 app,Apple Silicon only:
- 内嵌 TuriX CUA
- TuriX CLI
- TuriX-work (办公场景)
- TuriX-code (代码场景)

**对应我们**:
- KinClaw serve UI = 雏形
- LocalKin 平台 = 等价层
- 我们还没把 KinClaw 包成 .app

**不急做**: 我们的差异化是单二进制 + brew 安装,做成 .app 反而模糊定位。但要监控他们 SuperPower 用户量 — 如果他们靠 app 起飞,说明命令行用户面太窄,得跟。

---

## 我们能学到的

### 立即可做 (本周内)

1. **跑 OSWorld benchmark** — 哪怕只跑 50 个任务,有第三方数字。可信度对齐 64.2%。
2. **README 加 differentiation 段** — "KinClaw vs TuriX-CUA: Go 单二进制 vs Python conda、super-skill 吸收 vs 多 LLM 协作"
3. **存档 turix.ai/research/ Technical Report** — 找 PDF 抓下来,放 `~/.localkin/research/turix-vlm-paper.pdf`

### 中期 (1-3 个月)

4. **Windows 探路** — 至少把 `sckit-win` 跟 `input-win` 的 spike 试一下。他们已经动了。
5. **OpenClaw 模式研究** — 他们怎么把 TuriX 注册成 Clawdbot 的 skill?LocalKin 平台用类似机制接入 KinClaw 时可以参考。
6. **多模型可选拆分** — soul.md 里加可选字段:`brain_model:` `actor_model:` 默认同一个,允许拆分。先开口子,实际拆等 MLX 通了再说。

### 长期 (取决于 $99 cert + 资源)

7. **训自家 actor 模型** — 这是他们最大护城河。我们没有就永远是"通用 VLM 套壳",有了才是产品。
8. **SuperPower 等价品** — 等 30 个视频证明品类,再考虑包 .app。

### 不学的 (主动选择不做)

- ❌ **Python + LangChain 重栈** — 这是我们最大体验优势,绝不动
- ❌ **多 LLM 强制协作架构** — 单 soul 简单赢面更大,先做加法不做减法
- ❌ **Conda env 教程** — 用户体验毒药
- ❌ **改名躲 "claw" 碰撞** — 我们已经在生态里,改名沉没成本太大

---

## 监控清单

定期 (每月) 看一次:
- [ ] TurixAI/TuriX-CUA stars / forks / commits 节奏
- [ ] release notes 里的模型升级信号
- [ ] multi-agent-linux/windows 分支进度
- [ ] turix.ai/research/ 是否发新 paper
- [ ] 有没有 OSWorld 排行更新

---

## 总结一句话

**TuriX-CUA 是同空间最像 KinClaw 的项目,他们做 Python 重栈 + 自训模型 + 多 LLM 协作 + 已动 Win/Linux,我们做 Go 单二进制 + super-skill + 单 soul + Mac 专精。两队独立收敛在 markdown skills 和 "claw" 隐喻上,验证方向正确。短期他们领先在 OSWorld 公开数据 + 自训模型,我们领先在部署体验 + 吸收速度。中期决战在 Windows/Linux 谁先做透 + 谁先训出自家模型。**
