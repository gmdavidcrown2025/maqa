from __future__ import annotations

from math import exp, tanh
from random import Random

from .config import MAQAConfig
from .models import Broker, Lead, RankingContext, ScoreBreakdown


class ScoreCalculator:
    # 集中管理所有评分公式，避免引擎层混入过多数学细节。
    def __init__(self, config: MAQAConfig | None = None) -> None:
        self.config = config or MAQAConfig()

    @staticmethod
    def clamp_unit(value: float) -> float:
        return max(0.0, min(1.0, value))

    @staticmethod
    def target_cumulative(quota_q: float, day_index: int, days_in_month: int) -> float:
        return quota_q * day_index / days_in_month

    def calc_fit(self, broker: Broker, lead: Lead) -> float:
        # 当前简化版中，Fit 由上游直接预计算后传入。
        del lead
        return self.clamp_unit(broker.fit_score)

    def calc_quota_gap(
        self,
        quota_q: float,
        allocated_count: float,
        day_index: int,
        days_in_month: int,
    ) -> float:
        # 比较当前累计分配量与本月理想节奏之间的偏差。
        target_value = self.target_cumulative(quota_q, day_index, days_in_month)
        denominator = max(target_value, self.config.epsilon_q * quota_q)
        zscore = (target_value - allocated_count) / denominator
        return tanh(self.config.alpha_q * zscore)

    def calc_burst(self, last_24h_count: float, last_7d_count: float) -> float:
        # 用短期分配量相对近期基线的偏离程度来抑制灌单。
        baseline = last_7d_count / 7.0
        zscore = (last_24h_count - baseline - self.config.delta_b) / max(baseline, self.config.epsilon_b)
        return min(self.config.b_max, max(0.0, zscore))

    def calc_service(self, broker: Broker) -> float:
        # Service 只做轻量的可接单性和负载修正。
        if not broker.is_eligible or not broker.sla_ok:
            return 0.0
        response_score = self.clamp_unit(broker.response_score)
        load_penalty = self.clamp_unit(broker.current_load)
        return self.clamp_unit(response_score * (1.0 - 0.5 * load_penalty))

    def calc_over_quota_decay(
        self,
        quota_q: float,
        allocated_count: float,
        day_index: int,
        days_in_month: int,
    ) -> float:
        # 超额后不硬停，而是随超额量和月进度逐步衰减。
        over_quota = max(0.0, allocated_count - quota_q)
        if over_quota <= 0:
            return 1.0
        month_progress = day_index / days_in_month
        return exp(-self.config.beta * over_quota) * month_progress ** self.config.eta

    def calc_raw_score(self, fit: float, quota_gap: float, burst: float, service: float) -> float:
        return (
            self.config.w_fit * fit
            + self.config.w_q * quota_gap
            - self.config.w_b * burst
            + self.config.w_srv * service
        )

    def add_noise(self, score: float, rng: Random | None = None) -> float:
        # 扰动仅用于打破近似平分，不应覆盖明显分差。
        random_source = rng or Random()
        return score + random_source.random() * self.config.noise_eps

    def calc_score_breakdown(
        self,
        broker: Broker,
        lead: Lead,
        context: RankingContext,
        rng: Random | None = None,
    ) -> ScoreBreakdown:
        # 组装单个候选经纪人的完整可审计评分记录。
        fit = self.calc_fit(broker, lead)
        quota_gap = self.calc_quota_gap(
            quota_q=broker.quota_q,
            allocated_count=broker.allocated_count,
            day_index=context.day_index,
            days_in_month=context.days_in_month,
        )
        burst = self.calc_burst(broker.last_24h_count, broker.last_7d_count)
        service = self.calc_service(broker)
        decay = self.calc_over_quota_decay(
            quota_q=broker.quota_q,
            allocated_count=broker.allocated_count,
            day_index=context.day_index,
            days_in_month=context.days_in_month,
        )
        raw_score = self.calc_raw_score(fit, quota_gap, burst, service)
        final_score = raw_score * decay
        noisy_score = self.add_noise(final_score, rng)
        return ScoreBreakdown(
            fit=fit,
            quota_gap=quota_gap,
            burst=burst,
            service=service,
            over_quota_decay=decay,
            raw_score=raw_score,
            final_score=final_score,
            noisy_score=noisy_score,
        )
