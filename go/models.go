package maqa

import "time"

// Broker represents the broker state snapshot consumed by the ranking engine.
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

// Lead represents the lead currently being ranked.
type Lead struct {
	LeadID    string
	CreatedAt *time.Time
}

// RankingContext provides the time context required by monthly pacing formulas.
type RankingContext struct {
	Now         time.Time
	DayIndex    int
	DaysInMonth int
}

// ScoreBreakdown stores the full score details for a single broker.
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

// RankedBroker binds a broker to its score.
type RankedBroker struct {
	Broker Broker
	Score  ScoreBreakdown
}

// RankingResult is the main output of the ranking engine.
type RankingResult struct {
	RankedBrokers []RankedBroker
}

// TopBroker returns the first-ranked broker, or nil when the ranking is empty.
func (r RankingResult) TopBroker() *Broker {
	if len(r.RankedBrokers) == 0 {
		return nil
	}
	return &r.RankedBrokers[0].Broker
}

// TopScore returns the score breakdown of the first-ranked broker, or nil when the ranking is empty.
func (r RankingResult) TopScore() *ScoreBreakdown {
	if len(r.RankedBrokers) == 0 {
		return nil
	}
	return &r.RankedBrokers[0].Score
}
