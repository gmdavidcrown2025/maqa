from __future__ import annotations

from datetime import datetime

from pydantic import BaseModel, ConfigDict


class MAQABaseModel(BaseModel):
    # Treat all domain objects as immutable snapshots for safer reasoning and testing.
    model_config = ConfigDict(frozen=True)


class Broker(MAQABaseModel):
    # Minimal broker state required by the ranking engine.
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
    # Keep the current version intentionally minimal and only store lead data needed by ranking.
    lead_id: str
    created_at: datetime | None = None


class RankingContext(MAQABaseModel):
    # Time context required by pacing and decay formulas.
    now: datetime
    day_index: int
    days_in_month: int


class ScoreBreakdown(MAQABaseModel):
    # Full score breakdown for a single broker at the current lead and time context.
    fit: float
    quota_gap: float
    burst: float
    service: float
    over_quota_decay: float
    raw_score: float
    final_score: float
    noisy_score: float


class RankedBroker(MAQABaseModel):
    # Bind a broker to its score for sorting and traceability.
    broker: Broker
    score: ScoreBreakdown


class RankingResult(MAQABaseModel):
    # Main engine output: brokers sorted in descending score order.
    ranked_brokers: tuple[RankedBroker, ...]

    @property
    def top_broker(self) -> Broker | None:
        # Return the first ranked broker when a default single choice is needed.
        if not self.ranked_brokers:
            return None
        return self.ranked_brokers[0].broker

    @property
    def top_score(self) -> ScoreBreakdown | None:
        # Return the complete score breakdown of the first-ranked broker.
        if not self.ranked_brokers:
            return None
        return self.ranked_brokers[0].score
