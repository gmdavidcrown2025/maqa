package maqa

import "time"

// Broker 表示排序引擎消费的经纪人状态快照。
type Broker struct {
	BrokerID       string
	QuotaQ         float64
	AllocatedCount float64
	Last24hCount   float64
	Last7dCount    float64
	LastAssignedAt *time.Time
	FitScore       float64
	IsEligible     bool
	ResponseScore  float64
	CurrentLoad    float64
	SLAOK          bool
}

// Lead 表示当前待排序的线索。
type Lead struct {
	LeadID    string
	CreatedAt *time.Time
}

// RankingContext 提供月度进度计算所需的时间上下文。
type RankingContext struct {
	Now         time.Time
	DayIndex    int
	DaysInMonth int
}

// ScoreBreakdown 保存单个 broker 的完整评分明细。
type ScoreBreakdown struct {
	Fit            float64
	QuotaGap       float64
	Burst          float64
	Service        float64
	OverQuotaDecay float64
	RawScore       float64
	FinalScore     float64
	NoisyScore     float64
}

// RankedBroker 将 broker 与其得分绑定。
type RankedBroker struct {
	Broker Broker
	Score  ScoreBreakdown
}

// RankingResult 是排序引擎的主输出。
type RankingResult struct {
	RankedBrokers []RankedBroker
}

// TopBroker 返回排序第一名；若列表为空则返回 nil。
func (r RankingResult) TopBroker() *Broker {
	if len(r.RankedBrokers) == 0 {
		return nil
	}
	return &r.RankedBrokers[0].Broker
}

// TopScore 返回第一名对应的打分明细；若列表为空则返回 nil。
func (r RankingResult) TopScore() *ScoreBreakdown {
	if len(r.RankedBrokers) == 0 {
		return nil
	}
	return &r.RankedBrokers[0].Score
}
