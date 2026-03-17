from __future__ import annotations

from random import Random

from .config import MAQAConfig
from .formulas import ScoreCalculator
from .models import Broker, Lead, RankedBroker, RankingContext, RankingResult


class MAQAEngine:
    # 负责过滤候选人并完成排序，默认第一名即为建议人选。
    def __init__(self, config: MAQAConfig | None = None, calculator: ScoreCalculator | None = None) -> None:
        self.config = config or MAQAConfig()
        self.calculator = calculator or ScoreCalculator(self.config)

    @staticmethod
    def is_eligible(broker: Broker) -> bool:
        return broker.is_eligible and broker.sla_ok

    def rank(
        self,
        brokers: tuple[Broker, ...],
        lead: Lead,
        context: RankingContext,
        rng: Random | None = None,
    ) -> RankingResult:
        # 主入口：过滤候选人、打分并按分数降序返回。
        ranked_brokers = tuple(
            RankedBroker(
                broker=broker,
                score=self.calculator.calc_score_breakdown(broker, lead, context, rng),
            )
            for broker in brokers
            if self.is_eligible(broker)
        )
        return RankingResult(
            ranked_brokers=tuple(
                sorted(ranked_brokers, key=lambda item: item.score.noisy_score, reverse=True)
            )
        )
