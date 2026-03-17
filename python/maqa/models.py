from __future__ import annotations

from datetime import datetime

from pydantic import BaseModel, ConfigDict


class MAQABaseModel(BaseModel):
    # 所有领域对象都按不可变快照处理，便于推理和测试。
    model_config = ConfigDict(frozen=True)


class Broker(MAQABaseModel):
    # 排序引擎所需的最小经纪人状态。
    broker_id: str
    quota_q: float
    allocated_count: float = 0.0
    last_24h_count: float = 0.0
    last_7d_count: float = 0.0
    last_assigned_at: datetime | None = None
    fit_score: float = 0.8
    is_eligible: bool = True
    response_score: float = 1.0
    current_load: float = 0.0
    sla_ok: bool = True


class Lead(MAQABaseModel):
    # 当前版本刻意保持极简，只保留排序链路需要的线索信息。
    lead_id: str
    created_at: datetime | None = None


class RankingContext(MAQABaseModel):
    # 月度进度、衰减等公式依赖的时间上下文。
    now: datetime
    day_index: int
    days_in_month: int


class ScoreBreakdown(MAQABaseModel):
    # 单个经纪人在当前线索和时间点下的完整打分明细。
    fit: float
    quota_gap: float
    burst: float
    service: float
    over_quota_decay: float
    raw_score: float
    final_score: float
    noisy_score: float


class RankedBroker(MAQABaseModel):
    # 将经纪人与其得分绑定，便于排序和回溯。
    broker: Broker
    score: ScoreBreakdown


class RankingResult(MAQABaseModel):
    # 排序引擎的主输出：按分数降序排列的经纪人列表。
    ranked_brokers: tuple[RankedBroker, ...]

    @property
    def top_broker(self) -> Broker | None:
        # 如需默认选一个人，取排序第一名。
        if not self.ranked_brokers:
            return None
        return self.ranked_brokers[0].broker

    @property
    def top_score(self) -> ScoreBreakdown | None:
        # 返回第一名对应的完整打分明细。
        if not self.ranked_brokers:
            return None
        return self.ranked_brokers[0].score
