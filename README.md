# MAQA

[English](README.md) | [简体中文](README.zh-CN.md)

MAQA is a broker ranking library for real-estate lead distribution.

It ranks a set of brokers for a given lead by combining matching quality, monthly pacing, short-term burst control, service readiness, and over-quota tail decay.

## Why This Project

Most lead allocation systems fail in one of two ways:

- they over-optimize for matching and ignore pacing or operational fairness
- they chase fairness and hurt lead quality

MAQA is a pragmatic middle ground. It is designed to be:

- explainable: every score component has clear business meaning
- testable: key formulas and ranking cases are covered by golden tests
- portable: Python and Go implementations share the same algorithm spec and golden fixtures

## What It Does

For a given:

- `lead`
- set of `broker` snapshots
- `ranking context` such as current day in month

MAQA returns:

- brokers sorted in descending order
- full score breakdown for each broker

The current score model is:

```text
RawScore   = w_fit*Fit + w_q*QuotaGap - w_b*Burst + w_srv*Service
FinalScore = RawScore * OverQuotaDecay
NoisyScore = FinalScore + U(0, noise_eps)
```

The algorithm specification is documented in [docs/features/maqa_allocation_spec.md](docs/features/maqa_allocation_spec.md).

## Repository Layout

```text
.
├── docs/       # algorithm specification
├── go/         # Go implementation
├── python/     # Python implementation
└── testdata/   # shared golden cases
```

## Quick Start

### Python

Install:

```bash
cd python
pip install -e .
```

Run tests:

```bash
cd python
python -m unittest discover -s tests -v
```

Minimal usage:

```python
from datetime import datetime
from maqa import Broker, Lead, MAQAEngine, RankingContext

engine = MAQAEngine()
result = engine.rank(
    brokers=(
        Broker(broker_id="b1", quota_q=30, allocated_count=14, fit_score=0.8),
        Broker(broker_id="b2", quota_q=30, allocated_count=18, fit_score=0.7),
    ),
    lead=Lead(lead_id="lead-1"),
    context=RankingContext(
        now=datetime(2026, 3, 17, 10, 0, 0),
        day_index=17,
        days_in_month=31,
    ),
)

print(result.top_broker)
print(result.ranked_brokers)
```

### Go

Run tests:

```bash
cd go
go test -v ./...
```

Compile:

```bash
cd go
go build ./...
```

Minimal usage:

```go
engine, _ := maqa.NewDefaultEngine()
result := engine.Rank(
    []maqa.Broker{
        {BrokerID: "b1", QuotaQ: 30, AllocatedCount: 14, FitScore: 0.8, IsEligible: true, SLAOK: true, ResponseScore: 1.0},
        {BrokerID: "b2", QuotaQ: 30, AllocatedCount: 18, FitScore: 0.7, IsEligible: true, SLAOK: true, ResponseScore: 1.0},
    },
    maqa.Lead{LeadID: "lead-1"},
    maqa.RankingContext{Now: time.Now(), DayIndex: 17, DaysInMonth: 31},
    nil,
)

fmt.Println(result.TopBroker())
```

## Shared Golden Cases

Cross-language golden fixtures live in [testdata/golden_cases](testdata/golden_cases).

They are used to keep Python and Go aligned on:

- ranking order
- deterministic score components
- raw and final score outputs

`noisy_score` is intentionally not shared as a golden value, because Python and Go use different random number implementations.

## License

This project is licensed under the Apache License 2.0.

See [LICENSE](LICENSE) for the full license text.

## Status

Current repository scope is intentionally focused:

- ranking only, not full allocation workflow
- simple eligibility handling
- precomputed `fit_score` supplied by upstream systems
- no database coupling

That keeps MAQA as a clean core ranking library rather than a full application.
