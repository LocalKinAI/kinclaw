# Perplexity Personal Computer — 调研存档

**日期**: 2026-05-03
**调研者**: KinClaw Mac / Pilot
**目的**: 4-17 上线的 Perplexity Personal Computer 是 KinClaw Mac 最直接的(且最危险的)竞品 — 完整复盘
**关联文件**:
- `turix-cua-2026-05.md` — 同空间另一对手 (开源 macOS CUA, 64.2% OSWorld)
- `osworld-benchmark-2026-05.md` — 攻顶路线图,Perplexity 没报 OSWorld

---

## TL;DR

**Perplexity (估值 $40b) 4-17 发了 Mac 桌面 agent — `Personal Computer`**。形态跟今晚我们做的 KinClaw Mac 几乎一样:本地 macOS app,操作 Mac 文件 + apps + web,聊天驱动,语音支持,Mac mini 24/7 后台模式 + iPhone 远程。**Apple 在 keynote 给了 shoutout**。

**他们关键的"假本地"**: 操作层本地,**LLM brain 跑在他们的云端服务器**。订阅付费 + waitlist。

**这其实是好消息**:
- 形态被 $40b 公司用产品验证了 — 我们做的事不是 niche
- 他们关上了"本地 brain"的口袋 — KinClaw Mac + MLX 路径是唯一真本地选项
- 他们订阅收费 — 我们免费 + 开源 = 即时采纳故事

**紧迫度上升**: MLX 集成从 Q1 2027 提前。`拔网线 demo` 是杀手对比。

---

## 完整产品状况

### 上线时间线

| 日期 | 事件 |
|---|---|
| 2026-03-11 | AppleInsider 首次报道 Personal Computer 项目 |
| 2026-04-17 | 正式上线 Mac 平台 (subscriber-only) |
| 2026-05-01 | Apple keynote shoutout |
| 2026-05-03 | 调研此件 |

**节奏**: 1 个半月从首报到 Apple 背书。$40b 公司全速推。

### 平台支持

| 平台 | 状态 |
|---|---|
| Mac (桌面 app) | ✅ |
| Mac mini 24/7 模式 | ✅ marquee feature |
| iOS (Comet iOS) | ✅ — 远程操控 Mac mini |
| Windows | Comet 浏览器 ✅;Personal Computer ❌ |

**Mac mini 路线特别值得注意** — 把 Mac mini 当成"个人 AI 服务器" 24/7 跑。从手机或别的设备远程派任务。

### 架构 — 真正的"本地"在哪

```
[你的 Mac (Personal Computer app)]
   │
   ├─ 文件访问 ✅ 本地
   ├─ App 调用 ✅ 本地
   ├─ Comet 浏览器 ✅ 本地
   │
   ▼ (网络请求)
   │
[Perplexity 云服务器]
   ├─ LLM 推理 ⚠️ 全在这
   ├─ Agent 决策循环 ⚠️ 全在这
   └─ "Computer" platform (云端 agent runtime)
```

**操作层本地,大脑层云端**。营销话术叫 "across both local and cloud environments" — 听起来美,实质是"本地拿数据,送云端思考,云端给指令"。

你的所有屏幕截图、文件内容、操作意图,**都得过 Perplexity 服务器**。

### 安全模型 (按他们 CEO 公开说法)

- 敏感动作需要用户**批准** (per-action prompt)
- 活动**日志** (record everything)
- **Kill switch** (一键停)

中规中矩,但前提是数据已经过云。

### 商业模型

| 项 | 数据 |
|---|---|
| 价格 | 付费订阅 (具体没公布) |
| 准入 | **Waitlist** |
| 已包含 Pro 订阅?| 应该是 (Comet Browser 包含在 Pro) |
| 费用 | 推测 $20-50/mo (跟 ChatGPT Pro / Claude Max 同档) |

**Waitlist** 是关键信号 — 服务器 capacity 跟不上 / 想造稀缺感。两者都好,都不利于 KinClaw Mac 的潜在用户(他们等不到就找替代)。

---

## 完整对比矩阵 (含 Perplexity)

| | Claude CU | OpenAI Operator | Manus | TuriX-CUA | **Perplexity PC** | **KinClaw Mac** |
|---|---|---|---|---|---|---|
| 形态 | 云沙箱 | 云沙箱 | 云沙箱 | 本地 Mac | 本地 Mac | 本地 Mac |
| LLM 跑在 | Anthropic 云 | OpenAI 云 | 他们云 | 你选 (cloud/local) | **他们云** | **你选 (cloud/local)** |
| 价格 | API 按量 | 订阅 | 订阅 | 免费 | **付费订阅** | **免费** |
| 开源 | ❌ | ❌ | ❌ | ✅ MIT | ❌ | ✅ Apache-2.0 |
| 数据上云?| ✅ 必 | ✅ 必 | ✅ 必 | 可纯本地 | **✅ 必** | **可纯本地** |
| 多 agent | 1 (Claude) | 1 (Operator) | 1 (Manus) | 多 (souls) | 1 (Perplexity) | **105 (98 云 + 7 本地)** |
| 真操作 Mac | ❌ 沙箱 | ❌ 沙箱 | ❌ 沙箱 | ✅ | ✅ | ✅ |
| Mac mini 24/7 | ❌ | ❌ | N/A | ❌ (可加) | ✅ marquee | ❌ (架构通,可加) |
| 远程 (iPhone) | ❌ | ❌ | ❌ | ❌ | ✅ Comet iOS | ❌ (v2.1 relay 计划) |
| OSWorld 数字 | 报过 | 报过 | 没报 | 64.2% | **没报** | 没报 (Q3 2026 跑) |
| 上线 | 2024-10 | 2025 | 2024 | 2025-08 | 2026-04 | 2026-05 (今晚) |

**KinClaw Mac 跟 TuriX-CUA 是表里只有的** **完全本地** 选项。Perplexity 的"本地"加引号 — 大脑还在他们手里。

---

## 我们仍领先的 4 个差异化

### 1. 真本地选项 (LLM brain pluggable)

KinClaw Mac soul 配置可以指定 brain provider:
```yaml
brain:
  provider: ollama   # 本地
  model: kimi-k2.5
```

→ 完全 air-gapped 模式可行 (MLX 通了之后体验更好)。

Perplexity 永远做不到 — 他们的产品定位就是"我们的云模型 + 你的 Mac"。

### 2. 免费 + 开源

KinClaw 内核 Apache-2.0,kinclaw-mac shell 即使闭源也是免费 .app。Perplexity 收订阅 + waitlist。

普通用户决策门槛: $0 vs $20/mo + 申请等待。**100x 转化优势**。

### 3. 105 个 agents,你选

Perplexity Personal Computer 是**单一 agent** — Perplexity AI 助手。所有任务都是"问 Perplexity"。

KinClaw Mac dock 形态:
- 7 个本地 KinClaw souls (Pilot/Coder/Critic/Curator/Eye/Marketer/Researcher)
- 17 个 Core 云 agents (英语导师/中医/猎房/万事通...)
- 42 个 Selah 灵修大师
- 39 个岐黄 中医名家

每个 agent 都有专长,⌘⌥K 一键召唤任意一位。

### 4. 开发者友好 + 可定制

KinClaw 用户可以:
- 自己写 soul (Markdown YAML 即可)
- 自己加 skill (super-skill 模式)
- fork kinclaw-mac 改 UI
- 接入 LocalKin 平台 swarm

Perplexity 是 SaaS,你只能用他们给的。

---

## 我们落后的 2 个地方

### 1. Apple 背书

Perplexity 在 Apple keynote 被点名,**WWDC 26 之前最高级别背书**。我们零曝光。

补救方式:
- 跑 OSWorld 拿数字 → 学术圈认账 → 间接进 Apple 视野
- $99 cert 下来后 .app 通过 App Store 审核 → 间接 Apple 渠道
- 30 视频做 viral → social proof 自下而上

### 2. Mac mini 24/7 + iPhone 远程

他们这条产品故事很强 — "你晚上把任务丢给 Mac mini,睡醒在 iPhone 看结果"。

我们当前形态需要 Mac 醒着 + 浮窗在前。

补救:
- KinClaw 主线 v2.1 relay 计划做的就是这个 — 但需要 $99 cert 才能签 .app 才能做 OAuth 认证
- 时间窗口: Perplexity 已经在做了,我们大约慢 6-12 个月

---

## 战略影响 + 紧迫度调整

### 之前 OSWorld 6 阶段路线 (Q4 2027 冲 60%) → 现在需要重排

| 阶段 | 之前计划 | Perplexity 影响下的修订 |
|---|---|---|
| Q2 2026 (now) | 30 视频 | 30 视频 + **首支拔网线对比 Perplexity** |
| Q3 2026 | OSWorld adapter + baseline | **MLX 集成提前** + OSWorld baseline |
| Q4 2026 | scaffold 优化 | scaffold 优化 + **Mac mini 24/7 模式** |
| Q1 2027 | 训 kinclaw-actor | 训 actor + **iOS shortcut 接入** |
| Q2 2027 | Windows/Linux | (推迟) |
| Q3-Q4 2027 | 决战 OSWorld | 决战 OSWorld |

**关键调整**: MLX 本地推理从 Q1 2027 → **Q3 2026**。这是我们 vs Perplexity 唯一的护城河护墙;不能让他们先做出本地 LLM 选项(虽然不像他们会做,但保险起见提前)。

### 30 视频内容方向调整

之前的杀手 demo:
- 朋友找房子 (browser_session 真验证)
- 安装速度对比 TuriX

新增的:
- **拔网线 demo**: KinClaw Mac (本地 brain) 继续运行 vs Perplexity Personal Computer (无网络) 死掉
- **隐私 demo**: 截屏一份敏感文件,KinClaw 处理 vs Perplexity 上传到服务器
- **多 agent 矩阵 demo**: ⌘⌥K 切 8 个不同 agent 完成不同任务,Perplexity 只能问 Perplexity

### 定位文案调整

之前:
> "KinClaw Mac — Manus on your Mac"

加强后:
> **"KinClaw Mac — Perplexity Personal Computer if it were free, open-source, and your data never left your Mac."**

或更短:
> **"Your Mac's AI dock. Free. Open. Your data stays."**

---

## 监控清单

| 项 | 频率 | 触发动作 |
|---|---|---|
| Perplexity Personal Computer 价格公布 | 周看 | 调整 KinClaw Mac 营销文案 |
| waitlist 开放程度 | 月看 | 趁封闭期推 KinClaw |
| 加 Linux/Windows 支持?| 季看 | 重新评估我们 Win/Linux 路线 |
| Open source 化?| 季看 | 不太可能但盯着 |
| OSWorld 数字公布?| 月看 | 我们有数字时对比 |
| Apple 是否进一步深合作 | 持续 | 大动作影响整盘棋 |

---

## 一句话总结

**Perplexity 4-17 上线 Personal Computer,把我们今晚做的形态用 $40b 资源做了商业化版本 — 但 LLM brain 在他们云端 + 付费订阅 + 闭源 + waitlist。我们今晚做的 KinClaw Mac 在形态上跟他们打平,在差异化上靠"真本地 LLM 选项 + 免费 + 开源 + 多 agent"赢 4 项,落后 2 项 (Apple 背书 + Mac mini 24/7)。MLX 提前是必须的,30 视频第一支应是拔网线对比。Perplexity 不是威胁,是放大器 — 他们做品类教育,我们抢免费 + 开源 + 本地的细分市场。**
