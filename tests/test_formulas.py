import unittest
from pydantic import ValidationError
from math import exp, tanh
from random import Random

from maqa import Broker, MAQAConfig, Lead, ScoreCalculator


class ConfigValidationTests(unittest.TestCase):
    def test_weight_sum_must_equal_one(self) -> None:
        with self.assertRaises(ValidationError):
            MAQAConfig(w_fit=0.6, w_q=0.25, w_b=0.15, w_srv=0.10)


class FitTests(unittest.TestCase):
    def setUp(self) -> None:
        self.calculator = ScoreCalculator(MAQAConfig())
        self.lead = Lead(lead_id="lead-fit")

    def test_fit_is_clamped_to_zero_when_input_is_negative(self) -> None:
        broker = Broker(broker_id="b1", quota_q=30, allocated_count=0, fit_score=-0.3)
        self.assertEqual(self.calculator.calc_fit(broker, self.lead), 0.0)

    def test_fit_is_clamped_to_one_when_input_exceeds_upper_bound(self) -> None:
        broker = Broker(broker_id="b1", quota_q=30, allocated_count=0, fit_score=1.4)
        self.assertEqual(self.calculator.calc_fit(broker, self.lead), 1.0)


class QuotaGapTests(unittest.TestCase):
    def setUp(self) -> None:
        self.calculator = ScoreCalculator(MAQAConfig())

    def test_quota_gap_positive_when_behind_target(self) -> None:
        score = self.calculator.calc_quota_gap(quota_q=30, allocated_count=5, day_index=15, days_in_month=30)
        self.assertGreater(score, 0.0)

    def test_quota_gap_near_zero_when_on_target(self) -> None:
        score = self.calculator.calc_quota_gap(quota_q=30, allocated_count=15, day_index=15, days_in_month=30)
        self.assertAlmostEqual(score, 0.0, places=6)

    def test_quota_gap_negative_when_ahead_of_target(self) -> None:
        score = self.calculator.calc_quota_gap(quota_q=30, allocated_count=20, day_index=15, days_in_month=30)
        self.assertLess(score, 0.0)

    def test_quota_gap_matches_golden_value(self) -> None:
        # 手算：target=15, denom=15, z=(15-9)/15=0.4, tanh(2*0.4)=tanh(0.8)
        expected = tanh(0.8)
        score = self.calculator.calc_quota_gap(quota_q=30, allocated_count=9, day_index=15, days_in_month=30)
        self.assertAlmostEqual(score, expected, places=6)


class BurstTests(unittest.TestCase):
    def setUp(self) -> None:
        self.calculator = ScoreCalculator(MAQAConfig())

    def test_burst_zero_when_recent_load_is_normal(self) -> None:
        score = self.calculator.calc_burst(last_24h_count=1, last_7d_count=7)
        self.assertEqual(score, 0.0)

    def test_burst_positive_when_recent_load_spikes(self) -> None:
        score = self.calculator.calc_burst(last_24h_count=5, last_7d_count=7)
        self.assertGreater(score, 0.0)

    def test_burst_is_capped_in_extreme_but_valid_case(self) -> None:
        # 最近 24h 已经等于最近 7 天总量，代表几乎全部线索集中在一天内。
        score = self.calculator.calc_burst(last_24h_count=7, last_7d_count=7)
        self.assertLessEqual(score, 2.0)
        self.assertEqual(score, 2.0)

    def test_burst_matches_golden_value(self) -> None:
        # 手算：baseline=14/7=2, z=(5-2-0.5)/2=1.25, capped=1.25
        score = self.calculator.calc_burst(last_24h_count=5, last_7d_count=14)
        self.assertAlmostEqual(score, 1.25, places=6)


class ServiceTests(unittest.TestCase):
    def setUp(self) -> None:
        self.calculator = ScoreCalculator(MAQAConfig())

    def test_service_is_zero_when_sla_is_not_ok(self) -> None:
        broker = Broker(
            broker_id="b1",
            quota_q=30,
            allocated_count=0,
            response_score=0.9,
            current_load=0.2,
            sla_ok=False,
        )
        self.assertEqual(self.calculator.calc_service(broker), 0.0)

    def test_service_matches_golden_value(self) -> None:
        broker = Broker(
            broker_id="b1",
            quota_q=30,
            allocated_count=0,
            response_score=0.8,
            current_load=0.4,
        )
        expected = 0.8 * (1 - 0.5 * 0.4)
        self.assertAlmostEqual(self.calculator.calc_service(broker), expected, places=6)


class OverQuotaDecayTests(unittest.TestCase):
    def setUp(self) -> None:
        self.calculator = ScoreCalculator(MAQAConfig())

    def test_decay_is_one_before_quota(self) -> None:
        score = self.calculator.calc_over_quota_decay(quota_q=10, allocated_count=10, day_index=10, days_in_month=30)
        self.assertEqual(score, 1.0)

    def test_decay_shrinks_as_over_quota_grows(self) -> None:
        score_one_over = self.calculator.calc_over_quota_decay(quota_q=10, allocated_count=11, day_index=20, days_in_month=30)
        score_three_over = self.calculator.calc_over_quota_decay(quota_q=10, allocated_count=13, day_index=20, days_in_month=30)
        self.assertGreater(score_one_over, score_three_over)

    def test_decay_is_weaker_near_month_end(self) -> None:
        middle = self.calculator.calc_over_quota_decay(quota_q=10, allocated_count=11, day_index=15, days_in_month=30)
        end = self.calculator.calc_over_quota_decay(quota_q=10, allocated_count=11, day_index=30, days_in_month=30)
        self.assertGreater(end, middle)

    def test_decay_matches_golden_value(self) -> None:
        # 手算：over=2, exp(-0.8*2)*(20/30)^2
        expected = exp(-1.6) * (20 / 30) ** 2
        score = self.calculator.calc_over_quota_decay(quota_q=10, allocated_count=12, day_index=20, days_in_month=30)
        self.assertAlmostEqual(score, expected, places=6)


class RawScoreAndNoiseTests(unittest.TestCase):
    def setUp(self) -> None:
        self.config = MAQAConfig()
        self.calculator = ScoreCalculator(self.config)

    def test_raw_score_matches_golden_value(self) -> None:
        score = self.calculator.calc_raw_score(fit=0.8, quota_gap=0.2, burst=0.5, service=0.9)
        expected = 0.50 * 0.8 + 0.25 * 0.2 - 0.15 * 0.5 + 0.10 * 0.9
        self.assertAlmostEqual(score, expected, places=6)

    def test_noise_stays_within_expected_range(self) -> None:
        base_score = 0.42
        noisy_score = self.calculator.add_noise(base_score, Random(0))
        self.assertGreaterEqual(noisy_score, base_score)
        self.assertLessEqual(noisy_score, base_score + self.config.noise_eps)


if __name__ == "__main__":
    unittest.main()
