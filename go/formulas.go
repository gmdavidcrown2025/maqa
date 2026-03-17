package maqa

import (
	"math"
	"math/rand"
)

// ScoreCalculator centralizes MAQA formulas so the engine stays focused on orchestration.
type ScoreCalculator struct {
	Config Config
}

// NewScoreCalculator creates a calculator and validates the configuration.
func NewScoreCalculator(config Config) (*ScoreCalculator, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &ScoreCalculator{Config: config}, nil
}

func clampUnit(value float64) float64 {
	return math.Max(0.0, math.Min(1.0, value))
}

func targetCumulative(quotaQ float64, dayIndex int, daysInMonth int) float64 {
	return quotaQ * float64(dayIndex) / float64(daysInMonth)
}

// CalcFit consumes the precomputed upstream fit_score directly.
func (c ScoreCalculator) CalcFit(broker Broker, _ Lead) float64 {
	return clampUnit(broker.FitScore)
}

// CalcQuotaGap compares the current allocated count against the ideal monthly pace.
func (c ScoreCalculator) CalcQuotaGap(quotaQ float64, allocatedCount float64, dayIndex int, daysInMonth int) float64 {
	targetValue := targetCumulative(quotaQ, dayIndex, daysInMonth)
	denominator := math.Max(targetValue, c.Config.EpsilonQ*quotaQ)
	zScore := (targetValue - allocatedCount) / denominator
	return math.Tanh(c.Config.AlphaQ * zScore)
}

// CalcBurst detects short-term spikes relative to the recent seven-day baseline.
func (c ScoreCalculator) CalcBurst(last24hCount float64, last7dCount float64) float64 {
	baseline := last7dCount / 7.0
	zScore := (last24hCount - baseline - c.Config.DeltaB) / math.Max(baseline, c.Config.EpsilonB)
	return math.Min(c.Config.BMax, math.Max(0.0, zScore))
}

// CalcService applies a light adjustment based on availability and current load.
func (c ScoreCalculator) CalcService(broker Broker) float64 {
	if !broker.IsEligible || !broker.SLAOK {
		return 0.0
	}
	responseScore := clampUnit(broker.ResponseScore)
	loadPenalty := clampUnit(broker.CurrentLoad)
	return clampUnit(responseScore * (1.0 - 0.5*loadPenalty))
}

// CalcOverQuotaDecay applies tail decay after a broker exceeds quota instead of hard-filtering the broker out.
func (c ScoreCalculator) CalcOverQuotaDecay(quotaQ float64, allocatedCount float64, dayIndex int, daysInMonth int) float64 {
	overQuota := math.Max(0.0, allocatedCount-quotaQ)
	if overQuota <= 0.0 {
		return 1.0
	}
	monthProgress := float64(dayIndex) / float64(daysInMonth)
	return math.Exp(-c.Config.Beta*overQuota) * math.Pow(monthProgress, c.Config.Eta)
}

// CalcRawScore combines the base score terms.
func (c ScoreCalculator) CalcRawScore(fit float64, quotaGap float64, burst float64, service float64) float64 {
	return c.Config.WFit*fit + c.Config.WQ*quotaGap - c.Config.WB*burst + c.Config.WSrv*service
}

// AddNoise adds a tiny perturbation only to break near-ties.
func (c ScoreCalculator) AddNoise(score float64, rng *rand.Rand) float64 {
	randomValue := rand.Float64()
	if rng != nil {
		randomValue = rng.Float64()
	}
	return score + randomValue*c.Config.NoiseEps
}

// CalcScoreBreakdown computes the full score breakdown for a single broker.
func (c ScoreCalculator) CalcScoreBreakdown(broker Broker, lead Lead, context RankingContext, rng *rand.Rand) ScoreBreakdown {
	fit := c.CalcFit(broker, lead)
	quotaGap := c.CalcQuotaGap(broker.QuotaQ, broker.AllocatedCount, context.DayIndex, context.DaysInMonth)
	burst := c.CalcBurst(broker.Last24hCount, broker.Last7dCount)
	service := c.CalcService(broker)
	decay := c.CalcOverQuotaDecay(broker.QuotaQ, broker.AllocatedCount, context.DayIndex, context.DaysInMonth)
	rawScore := c.CalcRawScore(fit, quotaGap, burst, service)
	finalScore := rawScore * decay
	noisyScore := c.AddNoise(finalScore, rng)
	return ScoreBreakdown{
		Fit:            fit,
		QuotaGap:       quotaGap,
		Burst:          burst,
		Service:        service,
		OverQuotaDecay: decay,
		RawScore:       rawScore,
		FinalScore:     finalScore,
		NoisyScore:     noisyScore,
	}
}
