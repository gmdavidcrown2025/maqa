package maqa

import (
	"math"
	"math/rand"
	"testing"
)

func TestConfigValidateRejectsInvalidWeightSum(t *testing.T) {
	config := DefaultConfig()
	config.WFit = 0.6
	if err := config.Validate(); err == nil {
		t.Fatal("expected invalid weight sum to fail validation")
	}
}

func TestCalcFitClampsToUnitRange(t *testing.T) {
	calculator, err := NewScoreCalculator(DefaultConfig())
	if err != nil {
		t.Fatalf("unexpected config error: %v", err)
	}
	lead := Lead{LeadID: "lead-fit"}
	if score := calculator.CalcFit(Broker{BrokerID: "b1", QuotaQ: 30, FitScore: -0.3}, lead); score != 0.0 {
		t.Fatalf("expected 0.0, got %v", score)
	}
	if score := calculator.CalcFit(Broker{BrokerID: "b1", QuotaQ: 30, FitScore: 1.4}, lead); score != 1.0 {
		t.Fatalf("expected 1.0, got %v", score)
	}
}

func TestCalcQuotaGapMatchesGoldenValue(t *testing.T) {
	calculator, _ := NewScoreCalculator(DefaultConfig())
	expected := math.Tanh(0.8)
	score := calculator.CalcQuotaGap(30, 9, 15, 30)
	assertAlmostEqual(t, score, expected, 1e-9)
}

func TestCalcBurstMatchesGoldenValue(t *testing.T) {
	calculator, _ := NewScoreCalculator(DefaultConfig())
	score := calculator.CalcBurst(5, 14)
	assertAlmostEqual(t, score, 1.25, 1e-9)
}

func TestCalcBurstIsCappedInExtremeButValidCase(t *testing.T) {
	calculator, _ := NewScoreCalculator(DefaultConfig())
	score := calculator.CalcBurst(7, 7)
	assertAlmostEqual(t, score, 2.0, 1e-9)
}

func TestCalcServiceMatchesGoldenValue(t *testing.T) {
	calculator, _ := NewScoreCalculator(DefaultConfig())
	broker := Broker{BrokerID: "b1", QuotaQ: 30, IsEligible: true, SLAOK: true, ResponseScore: 0.8, CurrentLoad: 0.4}
	expected := 0.8 * (1 - 0.5*0.4)
	assertAlmostEqual(t, calculator.CalcService(broker), expected, 1e-9)
}

func TestCalcServiceReturnsZeroWhenSLAIsNotOK(t *testing.T) {
	calculator, _ := NewScoreCalculator(DefaultConfig())
	broker := Broker{BrokerID: "b1", QuotaQ: 30, IsEligible: true, SLAOK: false, ResponseScore: 0.9, CurrentLoad: 0.2}
	assertAlmostEqual(t, calculator.CalcService(broker), 0.0, 1e-9)
}

func TestCalcOverQuotaDecayMatchesGoldenValue(t *testing.T) {
	calculator, _ := NewScoreCalculator(DefaultConfig())
	expected := math.Exp(-1.6) * math.Pow(20.0/30.0, 2.0)
	score := calculator.CalcOverQuotaDecay(10, 12, 20, 30)
	assertAlmostEqual(t, score, expected, 1e-9)
}

func TestCalcRawScoreMatchesGoldenValue(t *testing.T) {
	calculator, _ := NewScoreCalculator(DefaultConfig())
	expected := 0.50*0.8 + 0.25*0.2 - 0.15*0.5 + 0.10*0.9
	score := calculator.CalcRawScore(0.8, 0.2, 0.5, 0.9)
	assertAlmostEqual(t, score, expected, 1e-9)
}

func TestAddNoiseStaysWithinExpectedRange(t *testing.T) {
	config := DefaultConfig()
	calculator, _ := NewScoreCalculator(config)
	baseScore := 0.42
	noisyScore := calculator.AddNoise(baseScore, rand.New(rand.NewSource(0)))
	if noisyScore < baseScore {
		t.Fatalf("noise should not decrease score: %v < %v", noisyScore, baseScore)
	}
	if noisyScore > baseScore+config.NoiseEps {
		t.Fatalf("noise exceeded expected upper bound: %v", noisyScore)
	}
}
