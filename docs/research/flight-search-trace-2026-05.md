# Task Trace: "Find SFO→Beijing flight prices for July 8" — KinClaw Pilot

**日期**: 2026-05-03 (深夜)
**任务**: 用户请 KinClaw Pilot 查 SFO 飞北京 单程 7 月 8 日的实时机票
**结果**: ❌ **失败** — 拿到参考价格,没拿到实时报价
**研究价值**: 第二条真实失败 trace,跟 `tj-navigation-trace-2026-05.md`
对比能看出 *agent 在 GUI 复杂度面前的死局*
**关联**:
- `tj-navigation-trace-2026-05.md` — 同样 18 步绕路但**最终成功**的对比
- `perplexity-personal-computer-2026-05.md` — Perplexity 这种闭源 agent
  可能也卡同样 GUI,但用户看不见 trace
- `osworld-benchmark-2026-05.md` — 这种失败案例是训 actor 模型的负样本

---

## TL;DR

Pilot 试图通过 browser_session 在 Google Flights 上查实时票价。**卡在
日期选择器** —— 默认 2026,要回退到 2025-07,需要至少 6 次"上一月"点击 +
6 次截屏验证状态 + 30+ tool calls。**最后超时被 kernel 杀掉**。

回落到先前 web_search 拿到的参考价 ($445-$900+),向用户**诚实说明**
没拿到实时数据 + 给手动操作建议。

**关键学习**: GUI 日期选择器是 **agent 必死局**。所有票务 / 订房 /
租车 / 酒店任务都该走 URL 参数,**绝不进 GUI 翻日历**。

---

## 已知步骤序列(用户分享的 agent 自述)

| # | 步 | 状态 | 备注 |
|---|---|---|---|
| 1 | 拿用户位置 | ✅ | Mountain View, CA (37.398, -122.063) |
| 2 | 搜索附近机场 | ✅ | SFO / SJC / OAK 三个 + 距离 |
| 3 | web_search 价格参考 | ✅ | $445 起 (Expedia 广告价),$550-900 7 月旺季 |
| 4 | browser_session: 打开 Google Flights | ✅ | URL: flights.google.com 主页 |
| 5 | 设置 oneway + SFO + PEK | ✅ | 表单填好了 |
| 6 | **设置出发日期 = 2025-07-08** | ❌ **死局开始** | 默认日历显示 2026 |
| 7 | 点 "上一月" → 截屏读 → 再点 → 再读 … | ❌ 多次循环 | LLM 每次得从截屏读"现在哪月" |
| 8 | Kernel timeout / circuit breaker 触发 | ❌ | browser_session 被杀 |
| 9 | Fallback: 试直接 page scrape | ❌ timeout | Google Flights React 树太重 |
| 10 | 报告失败 + 给参考价格 | ✅ 收尾诚实 | 没编造数字,给手动操作建议 |

预估总 tool calls: **30+** (大部分在第 7 步循环里)
预估时间: **2-3 分钟** 直到 timeout

---

## 该走的捷径 — Google Flights 直接 URL 参数

**Google Flights 早就支持 URL 透传日期 + 起止 + oneway**:

```
https://www.google.com/travel/flights?
  hl=en
  &curr=USD
  &q=Flights%20from%20SFO%20to%20PEK
  &date=2025-07-08
  &type=oneway
```

**1 次 `shell open URL` 或 `web fetch URL`,0 次日期点击,直接到
结果页**。然后从页面 HTML 抓 `aria-label="Flight"` 的元素就拿到所有
航班 + 价格。

或者更直接 — Kayak 的 URL 格式更稳:

```
https://www.kayak.com/flights/SFO-PEK/2025-07-08?sort=price_a
```

格式 `/flights/<起>-<止>/<日期>` 就行,sort=price_a 按价格升序。结果
页 HTML 干净,scrape 抓 `data-testid="flight-card"` 即可。

### 步数对比

| 路径 | 步数 | 时间 | 成功率 |
|---|---|---|---|
| **GUI 翻日历** (Pilot 走的路) | 30+ | 2-3 min | ~30% (超时易死) |
| **Google Flights URL 参数** | 3-4 | <30 sec | ~85% |
| **Kayak URL 参数** | 3-4 | <30 sec | ~90% (HTML 更稳) |
| **Skyscanner URL** | 3-4 | <30 sec | ~85% |
| **第三方 API** (Amadeus, Duffel) | 1 | <5 sec | ~99% (但需 API key) |

**至少 10 倍效率差,3 倍可靠性差**。

---

## 为什么 Pilot 没走捷径

跟 TJ 案例同一个根因 —— **soul 没明确教过 URL 参数优先**。

加之这次比 TJ 更糟糕的是:**browser_session 这条路 *看起来* 在工作**:
- 打开 ✅
- 选 oneway ✅
- 填起止 ✅
- 然后日期 ❌ 死

Agent 投入了 5+ 步成本后才发现卡死,沉没成本心态让它继续点了 25+ 次
"上一月" 才被 kernel 杀。**没有早期 abort 机制**。

---

## 该往 pilot.soul.md 加的 doctrine

详见今晚同步加的 `kinclaw/souls/pilot.soul.md` 修订 (commit pending),
核心:

```markdown
## App deep-link / URL 参数优先（不要硬点 GUI）

GUI 路径在以下场景必死或极慢:
- 日期选择器
- 多筛选器
- 弹窗 / cookie banner / "are you sure" 链

判断:
1. 任务涉及 *日期 / 数量 / 起止 / 价格* 任一?→ URL 参数,绝不 GUI
2. URL 能直达结果页?→ shell open / web fetch 一步
3. 都不行?→ 才走 GUI;但 >5 步必停;有日期翻页直接放弃
```

加了 14 个常用站点的 URL 参数表 (Google Flights / Kayak / Skyscanner /
Booking / Airbnb / Zillow / Amazon / GitHub / ArXiv / Maps 等)。

---

## 训练数据视角

跟 TJ trace 互补 —— TJ 是 *绕路成功* 的样本,这条是 *绕路失败* 的样本。
两类都需要,actor 模型才学到:

1. **何时坚持** (TJ 18 步切策略最终通)
2. **何时放弃** (这条 Flight 30+ 步早该 abort)

**判别标准** (要训进 actor 的): 对同一个状态(日历显示错误月份)
**重复 click "previous_month" >3 次没明显进展**,就该 abort 切策略
(URL params / API / fall back to web search)。

---

## Kernel 改进建议

KinClaw kernel 已有 `Skill "input" called 8 times` 的 circuit breaker,
但这个 case 里失败因为:
- browser_session 是**单一 skill 内部**的 N 次操作
- Kernel 看到的是 "browser_session" 一次调用,内部点了 30 次
- 没法在 sub-skill 层面拉手刹

**改进**:在 `skills/browser_session/runner.py` 内加 max-actions
budget,默认 20 次 action 不进展就 abort 并报告。这样 Pilot 会更早
拿到 "browser_session: 20 actions, no progress, here's last screenshot"
失败信号,可以更快切策略。

记入 `kinclaw` 主仓 issue list (M-something todo)。

---

## 30 视频脚本草稿

跟 TJ "成功的勇气" 配对 —— "失败也透明":

```
[0:00] ⌘⌥K 召唤
[0:03] 你: "查一下 7 月 8 号 SFO 飞北京的实时机票"
[0:08] Pilot 开始,tool widgets 一个个冒
[0:15] location ✅, airports ✅, web_search 价格 ✅
[0:25] browser_session 启动,Google Flights 打开
[0:35] 表单填好,卡在日期选择器
[0:50] "上一月" "上一月" "上一月"...
[1:30] kernel timeout,browser_session 被杀
[1:35] Pilot 切策略,试直接 scrape...也 timeout
[1:50] Pilot 收尾:"我没拿到实时,这是参考价格"

旁白: "看到了吗?它失败了,但失败得**诚实**。
      没编造一个 $678 假装查到。
      Manus 给你一个数字,你不知道它怎么来的;
      Pilot 给你一个数字,trace 里你能看见每一步。
      这是 transparency 不是 perfection。
      
      下个版本我会加 URL 参数 doctrine,
      30 步降到 3 步。今晚就加。
      
      这是 soul 系统 — 你看到失误,加一行 doctrine,
      下次它就会了。"

[2:15] 切到 doctrine 修改截图
[2:20] git commit -m "..."
[2:30] 再问一次同样问题,Pilot 1 步开 Google Flights URL,3 秒拿到价格
[2:40] 字幕: "30 步 → 3 步 · 2 分钟 → 3 秒"
```

**这是 30 视频里的 #2** —— 跟 #1 (TJ 18 步勇气) 形成"成功 + 失败"的
完整 narrative arc。

---

## 一句话总结

**第二条真实失败 trace —— 跟 TJ 互补,共同证明 KinClaw 三个独家价值的
*同时* 反例: (1) 5-claw 切策略会 *耗费太多步骤*, (2) circuit breaker
*在 sub-skill 内部失效*, (3) 全过程可见所以失败也透明 (Manus / Perplexity
你看不见这种死法)。直接修法是给 pilot.soul.md 加 URL-参数优先 doctrine
(已加,见 kinclaw 主仓 commit) + browser_session 内部加 max-actions budget
(M-something issue)。每条这种 trace 同时是 actor 模型训练的负样本 —
学到的是 *何时放弃 GUI 切 URL 参数*。**
