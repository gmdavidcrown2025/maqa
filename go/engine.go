package maqa

import (
	"math/rand"
	"sort"
)

// MAQAEngine filters candidate brokers and returns the final descending ranking.
type MAQAEngine struct {
	Config     Config
	Calculator *ScoreCalculator
}

// NewEngine creates a ranking engine.
func NewEngine(config Config) (*MAQAEngine, error) {
	calculator, err := NewScoreCalculator(config)
	if err != nil {
		return nil, err
	}
	return &MAQAEngine{Config: config, Calculator: calculator}, nil
}

// NewDefaultEngine creates a ranking engine with default parameters.
func NewDefaultEngine() (*MAQAEngine, error) {
	return NewEngine(DefaultConfig())
}

// IsEligible checks whether a broker is allowed to participate in ranking.
func (e MAQAEngine) IsEligible(broker Broker) bool {
	return broker.IsEligible && broker.SLAOK
}

// Rank is the main entry point of the Go MAQA implementation: filter, score, and sort by noisy_score descending.
func (e MAQAEngine) Rank(brokers []Broker, lead Lead, context RankingContext, rng *rand.Rand) RankingResult {
	ranked := make([]RankedBroker, 0, len(brokers))
	for _, broker := range brokers {
		if !e.IsEligible(broker) {
			continue
		}
		ranked = append(ranked, RankedBroker{
			Broker: broker,
			Score:  e.Calculator.CalcScoreBreakdown(broker, lead, context, rng),
		})
	}
	sort.Slice(ranked, func(i int, j int) bool {
		return ranked[i].Score.NoisyScore > ranked[j].Score.NoisyScore
	})
	return RankingResult{RankedBrokers: ranked}
}
