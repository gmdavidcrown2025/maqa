package maqa

import (
	"math/rand"
	"sort"
)

// MAQAEngine 负责过滤候选 broker 并输出降序排序结果。
type MAQAEngine struct {
	Config     Config
	Calculator *ScoreCalculator
}

// NewEngine 创建排序引擎。
func NewEngine(config Config) (*MAQAEngine, error) {
	calculator, err := NewScoreCalculator(config)
	if err != nil {
		return nil, err
	}
	return &MAQAEngine{Config: config, Calculator: calculator}, nil
}

// NewDefaultEngine 使用默认参数创建排序引擎。
func NewDefaultEngine() (*MAQAEngine, error) {
	return NewEngine(DefaultConfig())
}

// IsEligible 判断 broker 是否允许参与排序。
func (e MAQAEngine) IsEligible(broker Broker) bool {
	return broker.IsEligible && broker.SLAOK
}

// Rank 是 Go 版本 MAQA 的主入口：过滤、打分并按 noisy_score 降序返回。
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
