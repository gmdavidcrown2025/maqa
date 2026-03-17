# MAQA

[English](README.md) | [简体中文](README.zh-CN.md)

MAQA 是一个面向房产销售线索分配场景的 Broker 排序算法库。

它会针对一条 lead，对一组 broker 进行排序。排序时综合考虑匹配质量、月度节奏、短期突刺控制、服务承接能力，以及超额后的尾流衰减。

## 项目价值

很多线索分配系统会走向两个极端：

- 过度强调匹配，忽略节奏、公平性和运行状态
- 过度追求平均，损害线索质量和转化机会

MAQA 的目标是在两者之间取得平衡。当前仓库强调：

- 可解释：每个评分项都有明确业务含义
- 可验证：关键公式和黄金样例都有测试覆盖
- 可移植：Python 和 Go 共用同一份算法规格和黄金样例

## 能做什么

给定：

- 一条 `lead`
- 一组 `broker` 状态快照
- 当前排序上下文 `ranking context`

MAQA 输出：

- 按分数降序排列的 broker 列表
- 每个 broker 的完整打分明细

当前评分模型为：

```text
RawScore   = w_fit*Fit + w_q*QuotaGap - w_b*Burst + w_srv*Service
FinalScore = RawScore * OverQuotaDecay
NoisyScore = FinalScore + U(0, noise_eps)
```

完整算法说明见 [docs/features/maqa_allocation_spec.zh-CN.md](docs/features/maqa_allocation_spec.zh-CN.md)。

## 仓库结构

```text
.
├── docs/       # 算法文档
├── go/         # Go 实现
├── python/     # Python 实现
└── testdata/   # 跨语言共享黄金样例
```

## 快速开始

### Python

安装依赖：

```bash
cd python
pip install -e .
```

运行测试：

```bash
cd python
python -m unittest discover -s tests -v
```

最小示例：

```python
from datetime import datetime
from maqa import Broker, Lead, MAQAEngine, RankingContext

engine = MAQAEngine()
result = engine.rank(
    brokers=(
        Broker(broker_id="b1", quota_q=30, allocated_count=14, fit_score=0.8),
        Broker(broker_id="b2", quota_q=30, allocated_count=18, fit_score=0.7),
    ),
    lead=Lead(lead_id="lead-1"),
    context=RankingContext(
        now=datetime(2026, 3, 17, 10, 0, 0),
        day_index=17,
        days_in_month=31,
    ),
)

print(result.top_broker)
print(result.ranked_brokers)
```

### Go

运行测试：

```bash
cd go
go test -v ./...
```

编译检查：

```bash
cd go
go build ./...
```

最小示例：

```go
engine, _ := maqa.NewDefaultEngine()
result := engine.Rank(
    []maqa.Broker{
        {BrokerID: "b1", QuotaQ: 30, AllocatedCount: 14, FitScore: 0.8, IsEligible: true, SLAOK: true, ResponseScore: 1.0},
        {BrokerID: "b2", QuotaQ: 30, AllocatedCount: 18, FitScore: 0.7, IsEligible: true, SLAOK: true, ResponseScore: 1.0},
    },
    maqa.Lead{LeadID: "lead-1"},
    maqa.RankingContext{Now: time.Now(), DayIndex: 17, DaysInMonth: 31},
    nil,
)

fmt.Println(result.TopBroker())
```

## 共享黄金样例

跨语言黄金样例位于 [testdata/golden_cases](testdata/golden_cases)。

它们用于保证 Python 和 Go 在以下方面保持一致：

- 排序顺序
- 确定性评分项
- `raw_score` 与 `final_score`

`noisy_score` 没有作为跨语言黄金值共享，因为 Python 和 Go 使用的随机数实现不同。

## 许可

本项目采用 Apache License 2.0 许可证。

完整协议见 [LICENSE](LICENSE)。

## 当前边界

当前仓库的实现范围刻意收敛在核心排序能力上：

- 只做排序，不覆盖完整分配工作流
- Eligibility 保持简化
- `fit_score` 由上游系统预先计算
- 不与数据库 schema 耦合

这样可以把 MAQA 保持为一个清晰、稳定、可复用的核心排序库。
