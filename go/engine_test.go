package maqa

import (
	"encoding/json"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type goldenCase struct {
	CaseID      string         `json:"case_id"`
	Description string         `json:"description"`
	Config      goldenConfig   `json:"config"`
	Context     goldenContext  `json:"context"`
	Lead        goldenLead     `json:"lead"`
	Brokers     []goldenBroker `json:"brokers"`
	Expected    goldenExpected `json:"expected"`
}

type goldenConfig struct {
	WFit     float64 `json:"w_fit"`
	WQ       float64 `json:"w_q"`
	WB       float64 `json:"w_b"`
	WSrv     float64 `json:"w_srv"`
	AlphaQ   float64 `json:"alpha_q"`
	EpsilonQ float64 `json:"epsilon_q"`
	DeltaB   float64 `json:"delta_b"`
	EpsilonB float64 `json:"epsilon_b"`
	BMax     float64 `json:"b_max"`
	Beta     float64 `json:"beta"`
	Eta      float64 `json:"eta"`
	NoiseEps float64 `json:"noise_eps"`
}

type goldenContext struct {
	Now         string `json:"now"`
	DayIndex    int    `json:"day_index"`
	DaysInMonth int    `json:"days_in_month"`
}

type goldenLead struct {
	LeadID string `json:"lead_id"`
}

type goldenBroker struct {
	BrokerID       string  `json:"broker_id"`
	QuotaQ         float64 `json:"quota_q"`
	AllocatedCount float64 `json:"allocated_count"`
	Last24hCount   float64 `json:"last_24h_count"`
	Last7dCount    float64 `json:"last_7d_count"`
	FitScore       float64 `json:"fit_score"`
	IsEligible     bool    `json:"is_eligible"`
	ResponseScore  float64 `json:"response_score"`
	CurrentLoad    float64 `json:"current_load"`
	SLAOK          bool    `json:"sla_ok"`
}

type goldenExpected struct {
	RankedBrokerIDs []string                       `json:"ranked_broker_ids"`
	Scores          map[string]goldenExpectedScore `json:"scores"`
}

type goldenExpectedScore struct {
	QuotaGap       float64 `json:"quota_gap"`
	Burst          float64 `json:"burst"`
	Service        float64 `json:"service"`
	OverQuotaDecay float64 `json:"over_quota_decay"`
	RawScore       float64 `json:"raw_score"`
	FinalScore     float64 `json:"final_score"`
}

func TestRankReturnsRankedBrokersInDescendingOrder(t *testing.T) {
	engine, err := NewDefaultEngine()
	if err != nil {
		t.Fatalf("unexpected engine error: %v", err)
	}
	context := RankingContext{Now: time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC), DayIndex: 17, DaysInMonth: 31}
	lead := Lead{LeadID: "lead-1"}
	brokers := []Broker{
		{BrokerID: "b1", QuotaQ: 30, AllocatedCount: 10, FitScore: 0.9, IsEligible: true, SLAOK: true, ResponseScore: 1.0},
		{BrokerID: "b2", QuotaQ: 30, AllocatedCount: 25, FitScore: 0.4, IsEligible: true, SLAOK: true, ResponseScore: 1.0, Last24hCount: 5, Last7dCount: 7},
	}

	result := engine.Rank(brokers, lead, context, rand.New(rand.NewSource(0)))
	if len(result.RankedBrokers) != 2 {
		t.Fatalf("expected 2 ranked brokers, got %d", len(result.RankedBrokers))
	}
	if result.RankedBrokers[0].Broker.BrokerID != "b1" || result.RankedBrokers[1].Broker.BrokerID != "b2" {
		t.Fatalf("unexpected ranking order: %#v", brokerIDs(result))
	}
	if result.TopBroker() == nil || result.TopBroker().BrokerID != "b1" {
		t.Fatal("expected top broker to be b1")
	}
	if result.TopScore() == nil {
		t.Fatal("expected top score to be present")
	}
}

func TestRankReturnsEmptyRankingWhenNoBrokerIsEligible(t *testing.T) {
	engine, _ := NewDefaultEngine()
	context := RankingContext{Now: time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC), DayIndex: 17, DaysInMonth: 31}
	lead := Lead{LeadID: "lead-2"}
	brokers := []Broker{{BrokerID: "b1", QuotaQ: 30, AllocatedCount: 10, IsEligible: false, SLAOK: true}}

	result := engine.Rank(brokers, lead, context, rand.New(rand.NewSource(0)))
	if len(result.RankedBrokers) != 0 {
		t.Fatalf("expected empty ranking, got %d", len(result.RankedBrokers))
	}
	if result.TopBroker() != nil || result.TopScore() != nil {
		t.Fatal("expected top broker and top score to be nil")
	}
}

func TestRankFiltersOutBrokerWhenSLAIsNotOK(t *testing.T) {
	engine, _ := NewDefaultEngine()
	context := RankingContext{Now: time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC), DayIndex: 17, DaysInMonth: 31}
	lead := Lead{LeadID: "lead-3"}
	brokers := []Broker{
		{BrokerID: "b1", QuotaQ: 30, AllocatedCount: 12, FitScore: 0.8, IsEligible: true, SLAOK: true, ResponseScore: 1.0},
		{BrokerID: "b2", QuotaQ: 30, AllocatedCount: 8, FitScore: 0.95, IsEligible: true, SLAOK: false, ResponseScore: 1.0},
	}

	result := engine.Rank(brokers, lead, context, rand.New(rand.NewSource(0)))
	if len(result.RankedBrokers) != 1 || result.RankedBrokers[0].Broker.BrokerID != "b1" {
		t.Fatalf("unexpected ranking result: %#v", brokerIDs(result))
	}
}

func TestOverQuotaBrokerIsStillRankedInsteadOfBeingFilteredOut(t *testing.T) {
	engine, _ := NewDefaultEngine()
	context := RankingContext{Now: time.Date(2026, 3, 17, 10, 0, 0, 0, time.UTC), DayIndex: 17, DaysInMonth: 31}
	lead := Lead{LeadID: "lead-4"}
	brokers := []Broker{
		{BrokerID: "b1", QuotaQ: 30, AllocatedCount: 14, FitScore: 0.75, IsEligible: true, SLAOK: true, ResponseScore: 1.0},
		{BrokerID: "b2", QuotaQ: 30, AllocatedCount: 31, FitScore: 0.9, IsEligible: true, SLAOK: true, ResponseScore: 1.0, Last24hCount: 1, Last7dCount: 7},
	}

	result := engine.Rank(brokers, lead, context, rand.New(rand.NewSource(0)))
	if len(result.RankedBrokers) != 2 {
		t.Fatalf("expected 2 ranked brokers, got %d", len(result.RankedBrokers))
	}
	found := false
	for _, item := range result.RankedBrokers {
		if item.Broker.BrokerID == "b2" {
			found = true
			if item.Score.OverQuotaDecay >= 1.0 {
				t.Fatalf("expected over quota broker decay < 1.0, got %v", item.Score.OverQuotaDecay)
			}
		}
	}
	if !found {
		t.Fatal("expected over quota broker to remain in ranking")
	}
}

func TestRankingMatchesGoldenCase(t *testing.T) {
	fixture, err := loadGoldenCase(filepath.Join("..", "testdata", "golden_cases", "ranking_case_001.json"))
	if err != nil {
		t.Fatalf("failed to load golden case: %v", err)
	}
	engine, err := NewEngine(fixture.Config.toConfig())
	if err != nil {
		t.Fatalf("failed to build engine: %v", err)
	}
	context, err := fixture.Context.toRankingContext()
	if err != nil {
		t.Fatalf("failed to parse context: %v", err)
	}
	brokers := make([]Broker, 0, len(fixture.Brokers))
	for _, broker := range fixture.Brokers {
		brokers = append(brokers, broker.toBroker())
	}
	result := engine.Rank(brokers, fixture.Lead.toLead(), context, rand.New(rand.NewSource(0)))

	if got, want := brokerIDs(result), fixture.Expected.RankedBrokerIDs; !equalStringSlices(got, want) {
		t.Fatalf("unexpected ranking order: got %v want %v", got, want)
	}

	for _, item := range result.RankedBrokers {
		expected := fixture.Expected.Scores[item.Broker.BrokerID]
		assertAlmostEqual(t, item.Score.QuotaGap, expected.QuotaGap, 1e-9)
		assertAlmostEqual(t, item.Score.Burst, expected.Burst, 1e-9)
		assertAlmostEqual(t, item.Score.Service, expected.Service, 1e-9)
		assertAlmostEqual(t, item.Score.OverQuotaDecay, expected.OverQuotaDecay, 1e-9)
		assertAlmostEqual(t, item.Score.RawScore, expected.RawScore, 1e-9)
		assertAlmostEqual(t, item.Score.FinalScore, expected.FinalScore, 1e-9)
	}
}

func loadGoldenCase(path string) (goldenCase, error) {
	var fixture goldenCase
	content, err := os.ReadFile(path)
	if err != nil {
		return fixture, err
	}
	if err := json.Unmarshal(content, &fixture); err != nil {
		return fixture, err
	}
	return fixture, nil
}

func (c goldenConfig) toConfig() Config {
	return Config{
		WFit:     c.WFit,
		WQ:       c.WQ,
		WB:       c.WB,
		WSrv:     c.WSrv,
		AlphaQ:   c.AlphaQ,
		EpsilonQ: c.EpsilonQ,
		DeltaB:   c.DeltaB,
		EpsilonB: c.EpsilonB,
		BMax:     c.BMax,
		Beta:     c.Beta,
		Eta:      c.Eta,
		NoiseEps: c.NoiseEps,
	}
}

func (c goldenContext) toRankingContext() (RankingContext, error) {
	layouts := []string{time.RFC3339, "2006-01-02T15:04:05"}
	for _, layout := range layouts {
		now, err := time.Parse(layout, c.Now)
		if err == nil {
			return RankingContext{Now: now, DayIndex: c.DayIndex, DaysInMonth: c.DaysInMonth}, nil
		}
	}
	return RankingContext{}, os.ErrInvalid
}

func (l goldenLead) toLead() Lead {
	return Lead{LeadID: l.LeadID}
}

func (b goldenBroker) toBroker() Broker {
	return Broker{
		BrokerID:       b.BrokerID,
		QuotaQ:         b.QuotaQ,
		AllocatedCount: b.AllocatedCount,
		Last24hCount:   b.Last24hCount,
		Last7dCount:    b.Last7dCount,
		FitScore:       b.FitScore,
		IsEligible:     b.IsEligible,
		ResponseScore:  b.ResponseScore,
		CurrentLoad:    b.CurrentLoad,
		SLAOK:          b.SLAOK,
	}
}

func brokerIDs(result RankingResult) []string {
	ids := make([]string, 0, len(result.RankedBrokers))
	for _, item := range result.RankedBrokers {
		ids = append(ids, item.Broker.BrokerID)
	}
	return ids
}

func equalStringSlices(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
