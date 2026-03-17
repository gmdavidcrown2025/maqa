import unittest
from datetime import datetime
from random import Random

from maqa import Broker, Lead, MAQAEngine, RankingContext


def build_broker(
    broker_id: str,
    quota_q: float,
    allocated_count: float,
    fit_score: float = 0.8,
    response_score: float = 1.0,
    current_load: float = 0.0,
    last_24h_count: float = 0.0,
    last_7d_count: float = 0.0,
    is_eligible: bool = True,
) -> Broker:
    return Broker(
        broker_id=broker_id,
        quota_q=quota_q,
        allocated_count=allocated_count,
        last_24h_count=last_24h_count,
        last_7d_count=last_7d_count,
        fit_score=fit_score,
        is_eligible=is_eligible,
        response_score=response_score,
        current_load=current_load,
    )


class RankingTests(unittest.TestCase):
    def setUp(self) -> None:
        self.engine = MAQAEngine()
        self.context = RankingContext(
            now=datetime(2026, 3, 17, 10, 0, 0),
            day_index=17,
            days_in_month=31,
        )

    def test_returns_ranked_brokers_in_descending_order(self) -> None:
        lead = Lead(lead_id="lead-1")
        strong_broker = build_broker("b1", quota_q=30, allocated_count=10, fit_score=0.9)
        weak_broker = build_broker("b2", quota_q=30, allocated_count=25, fit_score=0.4, last_24h_count=5, last_7d_count=7)

        result = self.engine.rank((strong_broker, weak_broker), lead, self.context, rng=Random(0))

        self.assertEqual(len(result.ranked_brokers), 2)
        self.assertEqual(result.ranked_brokers[0].broker.broker_id, "b1")
        self.assertEqual(result.ranked_brokers[1].broker.broker_id, "b2")
        self.assertGreaterEqual(
            result.ranked_brokers[0].score.noisy_score,
            result.ranked_brokers[1].score.noisy_score,
        )
        self.assertEqual(result.top_broker.broker_id, "b1")
        self.assertEqual(result.top_score, result.ranked_brokers[0].score)

    def test_top_broker_is_first_ranked_broker(self) -> None:
        lead = Lead(lead_id="lead-2")
        strong_broker = build_broker("b1", quota_q=30, allocated_count=10, fit_score=0.9)
        weak_broker = build_broker("b2", quota_q=30, allocated_count=25, fit_score=0.4)

        result = self.engine.rank((strong_broker, weak_broker), lead, self.context, rng=Random(0))

        self.assertIsNotNone(result.top_broker)
        self.assertEqual(result.top_broker.broker_id, "b1")

    def test_returns_empty_ranking_when_no_broker_is_eligible(self) -> None:
        lead = Lead(lead_id="lead-3")
        blocked_broker = build_broker("b1", quota_q=30, allocated_count=10, is_eligible=False)

        result = self.engine.rank((blocked_broker,), lead, self.context, rng=Random(0))

        self.assertEqual(result.ranked_brokers, ())
        self.assertIsNone(result.top_broker)
        self.assertIsNone(result.top_score)

    def test_filters_out_broker_when_sla_is_not_ok(self) -> None:
        lead = Lead(lead_id="lead-4")
        healthy_broker = build_broker("b1", quota_q=30, allocated_count=12, fit_score=0.8)
        sla_blocked_broker = Broker(
            broker_id="b2",
            quota_q=30,
            allocated_count=8,
            fit_score=0.95,
            response_score=1.0,
            current_load=0.0,
            sla_ok=False,
        )

        result = self.engine.rank((healthy_broker, sla_blocked_broker), lead, self.context, rng=Random(0))

        self.assertEqual([item.broker.broker_id for item in result.ranked_brokers], ["b1"])

    def test_over_quota_broker_is_still_ranked_instead_of_being_filtered_out(self) -> None:
        lead = Lead(lead_id="lead-5")
        normal_broker = build_broker("b1", quota_q=30, allocated_count=14, fit_score=0.75)
        over_quota_broker = build_broker(
            "b2",
            quota_q=30,
            allocated_count=31,
            fit_score=0.9,
            response_score=1.0,
            current_load=0.0,
            last_24h_count=1,
            last_7d_count=7,
        )

        result = self.engine.rank((normal_broker, over_quota_broker), lead, self.context, rng=Random(0))

        self.assertEqual(len(result.ranked_brokers), 2)
        self.assertIn("b2", [item.broker.broker_id for item in result.ranked_brokers])
        over_quota_item = next(item for item in result.ranked_brokers if item.broker.broker_id == "b2")
        self.assertLess(over_quota_item.score.over_quota_decay, 1.0)

    def test_ranking_matches_golden_case(self) -> None:
        lead = Lead(lead_id="lead-golden")
        broker_1 = build_broker(
            "b1",
            quota_q=30,
            allocated_count=14,
            fit_score=0.8,
            response_score=0.9,
            current_load=0.2,
            last_24h_count=2,
            last_7d_count=14,
        )
        broker_2 = build_broker(
            "b2",
            quota_q=30,
            allocated_count=15,
            fit_score=0.9,
            response_score=1.0,
            current_load=0.0,
            last_24h_count=6,
            last_7d_count=14,
        )
        broker_3 = build_broker(
            "b3",
            quota_q=30,
            allocated_count=33,
            fit_score=0.95,
            response_score=1.0,
            current_load=0.0,
            last_24h_count=1,
            last_7d_count=14,
        )
        broker_4 = build_broker(
            "b4",
            quota_q=30,
            allocated_count=31,
            fit_score=0.7,
            response_score=0.8,
            current_load=0.1,
            last_24h_count=3,
            last_7d_count=14,
        )

        result = self.engine.rank((broker_1, broker_2, broker_3, broker_4), lead, self.context, rng=Random(0))

        self.assertEqual([item.broker.broker_id for item in result.ranked_brokers], ["b1", "b2", "b4", "b3"])
        self.assertAlmostEqual(result.ranked_brokers[0].score.quota_gap, 0.289517, places=5)
        self.assertAlmostEqual(result.ranked_brokers[0].score.burst, 0.0, places=6)
        self.assertAlmostEqual(result.ranked_brokers[0].score.service, 0.81, places=6)
        self.assertAlmostEqual(result.ranked_brokers[0].score.raw_score, 0.553379, places=5)
        self.assertAlmostEqual(result.ranked_brokers[0].score.final_score, 0.553379, places=5)
        self.assertAlmostEqual(result.ranked_brokers[1].score.quota_gap, 0.174661, places=5)
        self.assertAlmostEqual(result.ranked_brokers[1].score.burst, 1.75, places=6)
        self.assertAlmostEqual(result.ranked_brokers[1].score.final_score, 0.331165, places=5)
        self.assertAlmostEqual(result.ranked_brokers[2].score.over_quota_decay, 0.135126, places=5)
        self.assertAlmostEqual(result.ranked_brokers[2].score.final_score, 0.020625, places=5)
        self.assertAlmostEqual(result.ranked_brokers[3].score.over_quota_decay, 0.027281, places=5)
        self.assertAlmostEqual(result.ranked_brokers[3].score.final_score, 0.009106, places=5)


if __name__ == "__main__":
    unittest.main()
