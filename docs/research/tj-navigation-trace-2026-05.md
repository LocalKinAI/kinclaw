# Task Trace: "Navigate to nearest Trader Joe's" — KinClaw Pilot

**日期**: 2026-05-03 (深夜)
**任务**: 用户对 KinClaw Pilot 说 "导航最近的 TJ"
**结果**: ✅ 最终成功 — 但 18 步 / 110 秒 / 6 种 skill / 6 次 error
**研究价值**: 这是黄金训练数据 — *真实失败-恢复-成功* 的完整轨迹
**关联**:
- `perplexity-personal-computer-2026-05.md` — 这种 trace 是我们 vs Perplexity 的差异化 (开源可见)
- `osworld-benchmark-2026-05.md` — 训 `kinclaw-actor` 的数据采集策略,这是模板

---

## TL;DR

KinClaw Pilot 完成了"导航到最近 Trader Joe's"任务,但效率极差 (18 步 vs 最优 1 步)。重要的是 **agent 没有放弃** — 在 6 种 skill 之间切换,从 AX 失败 → app_open → AX 树 → vision + 坐标 → web → 鼠标点击,最终把 Apple Maps 打开 + 输入地址 + 启动导航。

**Kernel 的 circuit breaker** (`Skill "input" 被调用 8 次`) 在中途触发但 agent 自我调整后仍完成任务。

**对训 actor 模型来说,这条 trace 比 100 条直道成功 demo 更值钱**。

---

## 完整 18 步 Trace

按时间顺序,每条标注 (skill, status, hint):

| # | Skill | Status | Detail | 解读 |
|---|---|---|---|---|
| 1 | ui | ❌ | `kinax: not found` | 想直接 AX 找元素,Maps 还没打开 |
| 2 | **app_open_clean** | ✅ | `opened: Maps` | **关键转折**: 切策略,先打开 app |
| 3 | ui | ❌ | `no element matching` | Maps 启动中 / 没那个元素 |
| 4 | ui | ✅ | `AXApplication "Maps" AXW…` | 拿整棵 AX 树看结构 |
| 5 | input | ✅ | `pressed cmd+f` | 触发搜索框 |
| 6 | input | ❌ | `missing required` | 参数缺,可能 `text=` 没填 |
| 7 | input | ✅ | `typed 12 chars` | 改正后输入地址 |
| 8 | screen | ✅ | screenshot | 看搜索建议 |
| 9 | input | ✅ | `left-click at (600,300)` | 点搜索建议 (vision-driven 坐标) |
| 10 | screen | ✅ | screenshot | 看路线选项 |
| 11 | ui | ✅ | `pid=44207 role= title=""` | 检查活动窗口 |
| 12 | input | ✅ | `left-click at (400,230)` | 选路线 |
| 13 | shell | running | command in flight | 可能 `osascript` 调 Maps |
| 14 | screen | ✅ | screenshot | 验证 |
| 15 | web | ❌ | `goto failed: Page.goto: …` | 试 web 路径,失败 |
| 16 | input | ✅ | `left-click at (1590,60)` | 切回鼠标点击 |
| 17 | input | ❌ | `hotkey: bad mods` | 想发快捷键,modifier 错 |
| 18 | input | ⏳ | running | 最后一击 → 完成 |

**计数**:
- 18 tool calls 总共
- 6 种不同 skill (ui / app_open_clean / input / screen / shell / web)
- 6 次 ❌ error / 11 次 ✅ done / 1 个 running
- 平均 4-8 秒/call → **总耗时 ~110 秒** (估计)

---

## 该走的路 (最优解)

```bash
# 1 个 shell call,~2 秒,完成。
shell command="open 'maps://?daddr=550+Showers+Dr,+Mountain+View'"
```

或者更稳:
```bash
shell command="open -a Maps && open 'maps://?daddr=550+Showers+Dr,+Mountain+View'"
```

Apple Maps 接受 `maps://` URL scheme,带 `?daddr=` 直接进入导航模式。

**对比**:
- 最优: 1 call, ~2s
- 实际: 18 calls, ~110s
- **效率差: 18x 调用 / 55x 时间**

---

## 为什么 agent 没走最优解

诊断:

1. **soul 没教过 URL scheme 这个 trick** — pilot.soul.md 里没写"先试 URL scheme,再退回 GUI"
2. **`app_open_clean` skill 把 Maps 打开了,但没注入"导航 to X" 的意图** — 它只 launches app,不传 deep link
3. **训练数据偏倚**: pilot 见过的 demo 都是"GUI 点点点"型,deep-link 模式罕见

每一条都是可修的,加 doctrine + skill 升级 → 下次 1 步搞定。

---

## 但 agent 表现的**亮点** ⚠️ 不能忽视

虽然慢,但这条 trace 展示了 4 个关键能力:

### 1. Skill 切换 (Strategy Pivot)

AX 找不到元素 → **切到 `app_open_clean`**。
ui 路径不行 → **切到 vision + 坐标点击**。
单路径 web 不行 → **切回 input 鼠标**。

通用 VLM agent 会卡在第 1 个失败的策略上重试到死。**KinClaw 的 5-claw 架构允许 agent 在不同观察/操作模态间漂移**。

### 2. 应对 Circuit Breaker

中途触发 `Skill "input" called 8 times` 的 kernel 安全机制 — **agent 没放弃**,而是看了消息后调整策略,继续推进。说明 kinclaw 的 `report what you have, finish the user's task` 提示比硬性 termination 更有用。

### 3. 视觉 + 结构混用

第 4 步拿 AX 树 (结构化),第 8 / 10 / 14 步穿插截屏 (视觉),第 9 / 12 / 16 步用 vision-driven 坐标点击 — **不死磕一种感知**。

### 4. 真完成

18 步走完,Apple Maps **真的开了导航**。换成 Perplexity Personal Computer:agent 跑在他们云端,你 Mac 屏幕变化它得通过截图回传,这种 18 步混合策略走下去,云端 brain 会因为延迟堆积或 timeout 直接放弃。**本地架构的优势在压力测试下暴露出来**。

---

## 训练数据视角:这条 trace 的价值

如果我们要训 `kinclaw-actor` (见 `osworld-benchmark-2026-05.md` Q1 2027 路线),这种 trace 是黄金:

```
任务: "导航到最近 TJ"
状态序列: 18 个 (screenshot + AX_tree) 快照
动作序列: 18 个 (skill, params)
结果序列: 18 个 (success/error + output)
最终状态: 任务完成 (label = 1)
```

每一步都标注好了,可直接喂 SFT (supervised fine-tuning) 数据集。

**关键**: 包含 6 个 error → recovery 对。模型学到 "GUI 点不通时不要硬试,切策略"。这是通用 VLM 学不到的,因为他们的训练数据基本是直道成功 demo。

**复利**: 30 视频每条都生成这种 trace。30 × 18 平均 = 540 条 (state, action, result) 标注。再加上日常使用,几个月攒到几千条 → 够训第一版 actor。

---

## 立即可做的改进 (不等训模型)

### A. Soul doctrine 加 URL scheme 偏好

在 `kinclaw/souls/pilot.soul.md` 加一段:

```markdown
## 应用 deep-link 优先 (URL scheme over GUI clicking)

Mac 上有许多 task 用 URL scheme 一行 shell 就能完成。**优先试 deep-link**,
失败再退回 GUI 点击。

| 场景 | 优先做 | 别做 |
|---|---|---|
| 导航到地址 | `shell open "maps://?daddr=ADDR"` | 打开 Maps → 点搜索 → 输入 → 点导航 |
| 发邮件 | `shell open "mailto:X@Y?subject=Z"` | 打开 Mail → 新建 → 填… |
| 拨电话 | `shell open "tel:+1234567890"` | 打开电话 → 拨号盘 |
| 打开网页 | `shell open "https://..."` | 浏览器 → 地址栏 → … |
| 备忘录新建 | `shell open "x-applemobilesafari://"` 类似套路 | Notes → 点新建 |
| 打开 Music 播放 | `shell open "music://"` | Music → 播放 |
| 邮件搜索 | `shell open "message:"` 加查询 | Mail → 搜索框 |

URL scheme 1-2 步,GUI 平均 5-7 步且容易卡。

判断流程:
1. 用户要求"打开 X 到 Y 状态"型任务?→ **先想 X 有没有 URL scheme**
2. 没听过的 app → `shell defaults read /Applications/X.app/Contents/Info.plist` 看 CFBundleURLSchemes
3. 实在没有 → 才走 GUI 点击
```

### B. 升级 `app_open_clean` skill

让它支持 `deep_link=` 参数:

```
app_open_clean app=Maps deep_link="maps://?daddr=550+Showers+Dr"
```

skill 内部:
```bash
open -a "$app"
[ -n "$deep_link" ] && open "$deep_link"
```

### C. 加新 skill `app_deep_link`

专门做 deep-link 调用,跟 `app_open_clean` 区分职责:

```
app_deep_link url=maps://?daddr=...
```

简单 wrap `open URL`,但是有专门的 skill name 让 brain 在 doctrine 里更容易选中。

---

## Demo 视频脚本草稿 — "18 步的勇气"

把这条 trace 当 60 秒戏剧弧:

```
[0:00]  ⌘⌥K 召唤 KinClaw Pilot
[0:03]  你 (打字): "导航到最近 Trader Joe's"
[0:05]  Pilot 开始,浮窗里 tool widget 一个个冒出来
[0:15]  第 1 个 ❌ 出现 (kinax not found)
[0:20]  第 2 个 ✅ (app_open_clean — Maps 打开了)
[0:30]  又 2 个 ❌ + 4 个 ✅
[0:40]  字幕: "Skill 'input' 被调用 8 次 — kernel 提示"
[0:42]  旁白: "8 次没进展就拉手刹。Manus 不会拉这个手刹。"
[0:50]  Pilot 切策略,web → 失败 → 回 input
[1:00]  Apple Maps 真启动导航 ✅
[1:05]  字幕浮现: 18 calls · 110 秒 · 真完成
[1:10]  旁白:
        "URL scheme 1 步搞定,Pilot 走了 18 步 —
         这不是为了赢,是为了证明它在有阻力时能找到路。
         下版本我会教它 URL scheme,数字从 18 → 1。
         这就是 soul 系统 — 你看到失误,加一行 doctrine,
         下次它就会了。Perplexity / Manus 闭源你改不了,
         KinClaw 你能。"
[1:25]  KinClaw Mac · github.com/LocalKinAI/kinclaw
```

**为什么这比纯成功 demo 强**:
- 真实 (有 error,没造假)
- 教育 (展示 agent 怎么工作)
- 反差 (Manus 不会这样,Perplexity 闭源你不知道里面发生啥)
- 调用观众参与感 ("我也能改 soul")

---

## 长期意义:可观测性是 KinClaw 独家护城河

**Perplexity Personal Computer / Manus / Operator** 的 agent loop 跑在他们云端。你看到的是最终结果,**中间 18 步发生啥你不知道**。出错时只能等他们工程师改。

**KinClaw 的 18 步全在你 Mac 上,全部可见,每一步可以暂停 / 检查 / 改 doctrine**。

这个 trace 本身就是论据 — *KinClaw 卖的不是 agent,是 agent 透明性*。

---

## 监控清单

| 项 | 频率 |
|---|---|
| 这种 trace 在日常使用中出现频率 | 每周 |
| 同类任务的步数趋势 (加 URL scheme doctrine 后是否降低) | 每月 |
| Trace 数据集累积 | 持续 — 等 5 月底先攒 100 条评估能否训 actor |
| 哪些 task 类型最容易 18 步死循环 | 持续 — 这些是 doctrine 优先要补的 |

---

## 一句话总结

**18 步 / 6 种 skill / 6 个 error / 但成功 — 这条 trace 是 KinClaw 三个独家价值的同时证明: (1) 5-claw 架构允许 skill 切换 recovery, (2) kernel circuit breaker 防死循环但不硬终止, (3) 全过程可见可改 (不像 Perplexity / Manus 闭源云 agent)。同时是训 kinclaw-actor 模型的黄金 SFT 数据。短期立刻补 URL scheme doctrine 让 18 步降到 1 步;长期累积这种 trace 当训练语料。30 视频第 1 支用这条 task 的戏剧弧,比纯成功 demo 强 10 倍。**
