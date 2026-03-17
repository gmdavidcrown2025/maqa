# 多经纪人线索分配系统设计文档（可上线版）

## 1. 文档目标

本文档给出一版**可上线实现**的多经纪人线索分配方案，目标是让 Codex 能直接基于此文档实现第一版生产系统。

该方案解决以下问题：

1. 每个经纪人在月内尽量按合同量平滑拿量。
2. 避免某一天或某几个小时集中灌单。
3. 到达合同量后不硬停，而是进入递减尾流。
4. 多经纪人之间分配不能只看平滑，还要兼顾线索匹配质量。
5. 方案具备较好的可解释性、可调参性和工程落地性。

---

## 2. 方案概览

采用一个两层结构：

### 第一层：候选过滤（Eligibility Layer）
先过滤掉不能接当前线索的经纪人。

典型过滤条件：
- 不在线
- 已请假/休息
- 当前并发或容量已满
- 城市不匹配
- 区域/楼盘/品类不匹配
- 风控不允许
- SLA 状态异常

输出候选集合：

```text
C(lead, t) = {agent_i | agent_i 满足所有硬约束}
```

### 第二层：候选排序（Ranking Layer）
对候选经纪人计算实时评分，选出最适合的人。

排序逻辑遵循：

> 先匹配，再平衡；先达标，再衰减；先防突刺，再谈绝对均匀。

---

## 3. 评分总公式（最终版）

对每条新线索 `lead`，对每个候选经纪人 `i` 计算：

```math
RawScore_i(lead, t) =
w_fit   * Fit_i(lead)
+ w_q   * QuotaGap_i(t)
- w_b   * Burst_i(t)
+ w_srv * Service_i(t)
```

最终评分：

```math
FinalScore_i(lead, t) = RawScore_i(lead, t) * OverQuotaDecay_i(t)
```

其中：
- `Fit_i(lead)`：线索与经纪人的匹配分
- `QuotaGap_i(t)`：月度目标进度缺口分
- `Burst_i(t)`：短期突刺惩罚
- `Service_i(t)`：服务状态/在线状态/响应能力加分
- `OverQuotaDecay_i(t)`：达到合同量后的尾流衰减因子

**推荐默认权重：**

```text
w_fit = 0.50
w_q   = 0.25
w_b   = 0.15
w_srv = 0.10
```

说明：
- 匹配质量优先级最高。
- 平滑与配额是强约束，但不应压过业务匹配。
- Burst 是惩罚项。
- OverQuotaDecay 不进入线性加和，而是作为乘性门控，更符合语义。

---

## 4. QuotaGap 标准定义

### 4.1 目标

QuotaGap 用于表达：

> 当前时点，相对本月理想进度，该经纪人是落后还是超前。

它不是“离合同总量还差多少”，而是“相对当前日期应有节奏的偏离”。

### 4.2 目标累计曲线

对经纪人 `i`：
- 月合同量：`Q_i`
- 月总天数：`T`
- 当前日期对应的月内天序：`t`

定义目标累计曲线：

```math
G_i(t) = Q_i * g(t / T)
```

第一版建议直接使用**线性目标曲线**：

```math
g(z) = z
```

因此：

```math
G_i(t) = Q_i * t / T
```

### 4.3 标准化缺口定义

设当前累计分配量为 `A_i(t)`，则原始缺口为：

```math
\Delta_i^{quota}(t) = G_i(t) - A_i(t)
```

标准化后：

```math
z_i^{quota}(t) =
\frac{G_i(t) - A_i(t)}{\max(G_i(t), \epsilon_q * Q_i)}
```

其中：
- `epsilon_q` 用于防止月初分母过小
- 推荐：`epsilon_q = 0.2`

最后做有界压缩：

```math
QuotaGap_i(t) = tanh(alpha_q * z_i^{quota}(t))
```

### 4.4 最终公式

```math
QuotaGap_i(t) =
\tanh\left(
alpha_q *
\frac{G_i(t) - A_i(t)}{\max(G_i(t), epsilon_q * Q_i)}
\right)
```

### 4.5 推荐默认参数

```text
alpha_q  = 2.0
epsilon_q = 0.2
```

### 4.6 数值范围与含义

`QuotaGap_i(t)` 取值范围近似在 `(-1, 1)`：

- 接近 `1`：明显落后，应该补量
- 接近 `0`：基本跟上节奏
- 接近 `-1`：明显超前，应降优先级

---

## 5. Burst 标准定义

### 5.1 目标

Burst 用于表达：

> 最近短时间内，这个经纪人是否被明显集中灌单。

它只做惩罚，不做奖励。

### 5.2 时间窗口定义

建议采用两个窗口：

- `W_short`：短窗口，反映最近是否突增
- `W_long`：长窗口，反映该经纪人的近期常态水平

第一版推荐：

```text
W_short = 最近 24 小时
W_long  = 最近 7 天
```

定义：

- `X_i_short(t)`：最近 24 小时累计分配条数
- `mu_i_base(t)`：最近 7 天的日均分配量

即：

```math
mu_i^{base}(t) = X_i^{long}(t) / 7
```

### 5.3 标准化突刺定义

先计算超发量：

```math
\Delta_i^{burst}(t) = X_i^{short}(t) - mu_i^{base}(t) - delta_b
```

其中：
- `delta_b` 为自然波动缓冲区
- 推荐：`delta_b = 0.5`

再标准化：

```math
z_i^{burst}(t) =
\frac{X_i^{short}(t) - mu_i^{base}(t) - delta_b}{\max(mu_i^{base}(t), epsilon_b)}
```

其中：
- `epsilon_b` 防止基线过小导致爆炸
- 推荐：`epsilon_b = 0.5`

只保留惩罚部分，并做上限截断：

```math
Burst_i(t) = min(B_max, max(0, z_i^{burst}(t)))
```

### 5.4 最终公式

```math
Burst_i(t) =
min(
B_max,
max(
0,
\frac{X_i^{short}(t) - mu_i^{base}(t) - delta_b}{\max(mu_i^{base}(t), epsilon_b)}
)
)
```

### 5.5 推荐默认参数

```text
W_short  = 24h
W_long   = 7d
delta_b  = 0.5
epsilon_b = 0.5
B_max    = 2.0
```

### 5.6 数值范围与含义

- `0`：最近没有明显突刺
- `0 ~ 1`：轻微突刺
- `1 ~ 2`：明显突刺
- `2`：达到惩罚上限

---

## 6. OverQuotaDecay 标准定义

### 6.1 目标

OverQuotaDecay 用于表达：

> 经纪人达到月合同量后，仍可继续拿单，但优先级要递减。

该项不应作为普通加减分项，而应作为一个 `(0, 1]` 的乘性门控项。

### 6.2 定义

设：
- 当前累计：`A_i(t)`
- 月合同量：`Q_i`
- 月内天序：`t`
- 月总天数：`T`

当未达到合同量时：

```math
OverQuotaDecay_i(t) = 1
```

当超过合同量后：

```math
OverQuotaDecay_i(t) = exp(-beta * (A_i(t) - Q_i)) * (t / T)^eta
```

### 6.3 最终公式

```math
OverQuotaDecay_i(t) =
\begin{cases}
1, & A_i(t) <= Q_i \\
exp(-beta * (A_i(t) - Q_i)) * (t / T)^eta, & A_i(t) > Q_i
\end{cases}
```

### 6.4 推荐默认参数

```text
beta = 0.8
eta  = 2.0
```

### 6.5 业务含义

- 未达标：不衰减
- 超额后：超得越多，衰减越快
- 越接近月底，衰减相对放松
- 越早超额，越要克制

---

## 7. Service 标准定义（推荐简化版）

第一版建议将 `Service_i(t)` 设计为一个 `0~1` 的归一化值，反映经纪人的当前服务可用性和履约能力。

可组合的输入包括：
- 当前在线状态
- 最近响应速度
- 最近接通率/响应率
- SLA 状态
- 当前工作负荷

第一版可以先简化为：

```text
Service_i(t) = 1.0  if 在线且可接单
Service_i(t) = 0.5  if 在线但繁忙
Service_i(t) = 0.0  if 不在线或不可接单
```

如果第一层 Eligibility 已经严格过滤，`Service_i(t)` 也可以弱化，只保留轻微排序作用。

---

## 8. Fit 标准定义（推荐落地版）

`Fit_i(lead)` 建议归一化到 `0~1`。第一版可以采用加权规则分，后续再切模型分。

推荐输入特征：
- 城市匹配
- 区域/商圈匹配
- 楼盘/品类匹配
- 客户预算带匹配
- 客户意图阶段匹配
- 历史转化率
- 历史该类线索处理经验

第一版示例：

```text
Fit_i(lead) =
0.30 * city_match
+ 0.25 * area_match
+ 0.20 * inventory_match
+ 0.15 * intent_match
+ 0.10 * conversion_score
```

所有子项归一化到 `0~1`。

---

## 9. 最终评分函数（可上线默认版）

```math
RawScore_i(lead, t) =
0.50 * Fit_i(lead)
+ 0.25 * QuotaGap_i(t)
- 0.15 * Burst_i(t)
+ 0.10 * Service_i(t)
```

```math
FinalScore_i(lead, t) = RawScore_i(lead, t) * OverQuotaDecay_i(t)
```

### 9.1 说明

1. `Fit` 是主导项。
2. `QuotaGap` 让系统在月内维持合同节奏。
3. `Burst` 防止短期集中灌单。
4. `OverQuotaDecay` 在达到合同量后保留尾流，但逐步收缩。
5. 该结构非常适合后续 A/B test。

---

## 10. 分配执行策略

### 10.1 候选选择

当线索到达时：

1. 基于 Eligibility 规则得到候选经纪人集合 `C(lead, t)`。
2. 对每个候选经纪人计算 `FinalScore_i(lead, t)`。
3. 选择得分最高的经纪人。

即：

```text
winner = argmax_i FinalScore_i(lead, t)
```

### 10.2 是否需要随机扰动

建议第一版加一个很小的随机扰动，避免分数极接近时永远分给固定的人：

```text
FinalScore'_i = FinalScore_i + Uniform(0, noise_eps)
```

推荐：

```text
noise_eps = 0.01 ~ 0.03
```

然后：

```text
winner = argmax_i FinalScore'_i
```

### 10.3 不建议第一版直接用 softmax 抽样

原因：
- 业务解释成本高
- 排查问题难
- 可能造成“明明这个人更合适却没拿到”的争议

所以第一版建议：

> Top-1 贪心 + 小幅随机扰动

---

## 11. 数据结构设计建议

### 11.1 agent_month_state 表

用于记录经纪人在当前月的节奏状态。

```text
agent_id
month_key                # 例如 2026-03
quota_q                  # 月合同量 Q_i
allocated_count          # 当前月累计已分配 A_i(t)
target_curve_type        # linear / custom
updated_at
```

### 11.2 agent_distribution_stats 表

用于记录近期分配统计。

```text
agent_id
last_24h_count           # X_i_short(t)
last_7d_count            # X_i_long(t)
last_7d_daily_avg        # mu_i_base(t)
last_assigned_at
updated_at
```

### 11.3 agent_service_state 表

```text
agent_id
is_online
is_available
current_load
sla_status
response_score
updated_at
```

### 11.4 agent_profile_features 表

```text
agent_id
city_list
area_list
inventory_tags
intent_tags
conversion_score
level
```

### 11.5 lead_features 表

```text
lead_id
city
area
inventory_type
budget_range
intent_stage
created_at
```

---

## 12. 实时计算流程

### 12.1 输入

- 一条新线索 `lead`
- 当前时间 `now`
- 所有候选经纪人的实时状态与统计

### 12.2 流程

```text
Step 1. 基于硬约束过滤出 candidates
Step 2. 对每个 candidate 计算 Fit
Step 3. 读取其当月状态，计算 G_i(t)、QuotaGap
Step 4. 读取短期统计，计算 Burst
Step 5. 根据 A_i(t) 与 Q_i 计算 OverQuotaDecay
Step 6. 读取服务状态，计算 Service
Step 7. 合成 RawScore 与 FinalScore
Step 8. 加微小随机扰动
Step 9. 选出 winner
Step 10. 写入分配结果并更新统计
```

---

## 13. Python 伪代码

```python
from dataclasses import dataclass
from math import tanh, exp
from random import random


@dataclass
class AgentState:
    agent_id: str
    quota_q: float
    allocated_count: float
    last_24h_count: float
    last_7d_count: float
    is_online: bool
    is_available: bool
    response_score: float   # 0~1
    current_load: float     # 可选


@dataclass
class Lead:
    lead_id: str
    city: str
    area: str
    inventory_type: str
    budget_range: str
    intent_stage: str


def target_cumulative(quota_q: float, day_index: int, days_in_month: int) -> float:
    return quota_q * day_index / days_in_month


def calc_quota_gap(quota_q: float, allocated_count: float, day_index: int, days_in_month: int,
                   alpha_q: float = 2.0, epsilon_q: float = 0.2) -> float:
    g_t = target_cumulative(quota_q, day_index, days_in_month)
    denom = max(g_t, epsilon_q * quota_q)
    z = (g_t - allocated_count) / denom
    return tanh(alpha_q * z)


def calc_burst(last_24h_count: float, last_7d_count: float,
               delta_b: float = 0.5, epsilon_b: float = 0.5, b_max: float = 2.0) -> float:
    mu_base = last_7d_count / 7.0
    z = (last_24h_count - mu_base - delta_b) / max(mu_base, epsilon_b)
    return min(b_max, max(0.0, z))


def calc_over_quota_decay(quota_q: float, allocated_count: float, day_index: int, days_in_month: int,
                          beta: float = 0.8, eta: float = 2.0) -> float:
    if allocated_count <= quota_q:
        return 1.0
    over = allocated_count - quota_q
    return exp(-beta * over) * (day_index / days_in_month) ** eta


def calc_service(is_online: bool, is_available: bool, response_score: float) -> float:
    if not is_online or not is_available:
        return 0.0
    return max(0.0, min(1.0, response_score))


def calc_fit(agent: AgentState, lead: Lead) -> float:
    # TODO: 这里替换成真实规则或模型分
    # 暂时返回一个 0~1 的占位值
    return 0.8


def eligible(agent: AgentState, lead: Lead) -> bool:
    # TODO: 替换成真实过滤逻辑
    return agent.is_online and agent.is_available


def calc_final_score(agent: AgentState, lead: Lead, day_index: int, days_in_month: int) -> float:
    fit = calc_fit(agent, lead)
    quota_gap = calc_quota_gap(agent.quota_q, agent.allocated_count, day_index, days_in_month)
    burst = calc_burst(agent.last_24h_count, agent.last_7d_count)
    service = calc_service(agent.is_online, agent.is_available, agent.response_score)
    decay = calc_over_quota_decay(agent.quota_q, agent.allocated_count, day_index, days_in_month)

    raw_score = 0.50 * fit + 0.25 * quota_gap - 0.15 * burst + 0.10 * service
    return raw_score * decay


def choose_agent(candidates: list[AgentState], lead: Lead, day_index: int, days_in_month: int) -> AgentState | None:
    if not candidates:
        return None

    best_agent = None
    best_score = float("-inf")

    for agent in candidates:
        score = calc_final_score(agent, lead, day_index, days_in_month)
        score += random() * 0.02  # small noise
        if score > best_score:
            best_score = score
            best_agent = agent

    return best_agent
```

---

## 14. 更新逻辑

线索分配成功后，至少更新以下数据：

1. `allocated_count += 1`
2. `last_24h_count += 1`
3. `last_7d_count += 1`
4. `last_assigned_at = now`
5. 记录分配日志，用于后续审计与回放

建议同时写一张分配明细表：

```text
lead_assignment_log
-------------------
assignment_id
lead_id
agent_id
assigned_at
fit_score
quota_gap_score
burst_score
service_score
over_quota_decay
raw_score
final_score
strategy_version
```

这张表很重要，后续排查和 A/B test 都靠它。

---

## 15. 监控指标

上线后建议重点监控：

### 15.1 配额达成类
- 经纪人月度合同量达成率
- 超发率
- 月末未达标率

### 15.2 平滑体验类
- 单经纪人最近 24h 分配峰值
- 单经纪人最近 7d 分配标准差
- 分配集中度（例如 top 10% 经纪人吃单占比）

### 15.3 业务效果类
- 线索响应率
- 线索转化率
- 不同等级经纪人的接单质量差异
- 平台整体成交/收益指标

### 15.4 算法诊断类
- QuotaGap 分布
- Burst 分布
- OverQuotaDecay 分布
- FinalScore 排名前几名的差值分布

---

## 16. 默认参数汇总

```text
# 权重
w_fit = 0.50
w_q   = 0.25
w_b   = 0.15
w_srv = 0.10

# QuotaGap
alpha_q   = 2.0
epsilon_q = 0.2

# Burst
W_short   = 24h
W_long    = 7d
delta_b   = 0.5
epsilon_b = 0.5
B_max     = 2.0

# OverQuotaDecay
beta = 0.8
eta  = 2.0

# 扰动
noise_eps = 0.02
```

---

## 17. 第一版上线边界

第一版建议有意识地收敛范围，不要一次性做得过重。

### 第一版建议做
- 线性目标累计曲线
- 规则式 Fit
- 规则式 Eligibility
- Top-1 贪心 + 微扰
- 标准化的 QuotaGap / Burst / OverQuotaDecay
- 完整分配日志

### 第一版不建议做
- 强化学习
- 多目标最优化求解器
- 复杂 softmax 采样
- 过于复杂的非线性目标曲线
- 自动在线调参

原因很简单：

> 第一版目标不是“最先进”，而是“可解释、可验证、可迭代”。

---

## 18. 后续迭代方向

等第一版跑稳后，可以按以下顺序升级：

1. `Fit` 从规则分切到模型分。
2. `Service` 引入更细的 SLA、接通率、响应速度。
3. 目标累计曲线从线性升级到轻微 S 曲线。
4. 分配策略从 Top-1 + noise 升级到 Top-k 重排。
5. 引入预估收益项 `ExpectedRevenue_i(lead)`。

届时评分公式可升级为：

```math
FinalScore_i(lead, t) =
[w_fit * Fit_i(lead)
+ w_rev * Revenue_i(lead)
+ w_q * QuotaGap_i(t)
- w_b * Burst_i(t)
+ w_srv * Service_i(t)]
* OverQuotaDecay_i(t)
```

---

## 19. 给 Codex 的实现要求

Codex 实现时，建议按以下模块拆分：

```text
allocation/
  eligibility.py
  fit_score.py
  quota_gap.py
  burst.py
  over_quota_decay.py
  service_score.py
  ranker.py
  assignment_service.py
  models.py
  repositories.py
  tests/
```

### 最低测试要求

1. QuotaGap 单元测试
   - 月初、月中、月末都要覆盖
   - 未达标、达标、超额都要覆盖

2. Burst 单元测试
   - 正常情况不惩罚
   - 明显突刺要惩罚
   - 低基线场景不能爆炸

3. OverQuotaDecay 单元测试
   - 未达标应为 1
   - 超 1、2、3 条应递减
   - 月底衰减应弱于月中

4. Ranker 单元测试
   - 分数高的候选人应被选中
   - noise 不应改变明显优势样本的结果

5. Integration Test
   - 分配后统计值更新正确
   - 日志记录完整

---

## 20. 最终结论

这版方案的核心不是“某个单独函数”，而是一个可上线的多经纪人动态分配框架：

1. 通过 `Fit` 保证业务匹配优先。
2. 通过 `QuotaGap` 保证月度节奏平衡。
3. 通过 `Burst` 抑制短期集中灌单。
4. 通过 `OverQuotaDecay` 保证达标后不断流但递减。
5. 通过 `Top-1 + noise` 实现稳定、可解释的实时分配。

对于第一版生产系统，这已经足够强，而且最重要的是：**可落地**。
