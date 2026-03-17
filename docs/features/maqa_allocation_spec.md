# MAQA Ranking Algorithm Specification

[English](maqa_allocation_spec.md) | [简体中文](maqa_allocation_spec.zh-CN.md)

## 1. Purpose

This document describes the actual MAQA algorithm implemented in this repository. It focuses on:

- how the ranking problem is modeled
- what business problem each score component solves
- how the total score is composed
- what each parameter controls
- what complexity is intentionally left out of the current version

The goal of this document is to explain algorithmic logic and business rationale rather than code-level implementation details.

## 2. Positioning

The current MAQA implementation is a broker ranking algorithm, not a full allocation execution system.

Given:

- one lead
- a set of broker state snapshots
- a time context

The algorithm outputs:

- a descending ranking of brokers

If the business only needs one default recommendation, the first broker in the ranking is typically used. But the primary responsibility of MAQA is ranking, not executing the final allocation workflow.

## 3. What MAQA Tries to Balance

MAQA is not a single-factor fit-based ranker. It tries to balance four dimensions at the same time:

1. Matching quality
   Whether the lead and broker are a good fit.

2. Monthly pacing
   Whether the broker is behind the expected monthly pace and should be compensated.

3. Short-term smoothing
   Whether the broker has already received too many leads in the last 24 hours.

4. Service readiness
   Whether the broker is still in a good operational state to take more leads.

In addition, MAQA handles a special case:

- when a broker has exceeded the monthly target, the broker is not cut off immediately, but continues to receive a decaying tail opportunity

So MAQA is neither a round-robin scheduler nor a fit-only ranking rule. It is a multi-factor ranking model.

## 4. Input Abstraction

### 4.1 Broker Snapshot

Each broker is represented as a state snapshot. The current core inputs are:

- monthly target `quota_q`
- current monthly allocated count `allocated_count`
- allocated count in the last 24 hours `last_24h_count`
- allocated count in the last 7 days `last_7d_count`
- precomputed fit score `fit_score`
- aggregated eligibility flag `is_eligible`
- response readiness score `response_score`
- current load `current_load`
- SLA status `sla_ok`

These values are expected to be prepared by upstream systems. MAQA does not currently derive these features from databases, CRM systems, or service monitoring systems.

### 4.2 Lead

In the current version, lead exists mainly as the ranking target itself. `Fit` is not computed inside MAQA. The algorithm assumes upstream systems have already computed the broker-to-lead fit and stored it in `fit_score`.

### 4.3 Time Context

The time context currently includes at least:

- day of month `day_index`
- total number of days in month `days_in_month`

This lets the algorithm transform absolute monthly counts into pace-relative deviations.

## 5. Overall Scoring Structure

The current algorithm first computes a base composite score, then applies over-quota decay, and finally adds a very small perturbation to break near-ties.

### 5.1 Base Composite Score

```math
RawScore_i = w_{fit} \cdot Fit_i + w_q \cdot QuotaGap_i - w_b \cdot Burst_i + w_{srv} \cdot Service_i
```

Where:

- `Fit` is better when larger
- `QuotaGap` is better when larger
- `Burst` is worse when larger, so it is subtracted
- `Service` is better when larger

### 5.2 Score After Over-Quota Decay

```math
FinalScore_i = RawScore_i \cdot OverQuotaDecay_i
```

This is the tail-decay step for brokers who have exceeded quota.

### 5.3 Final Sorting Value

```math
NoisyScore_i = FinalScore_i + U(0, noise\_eps)
```

Here `U(0, noise_eps)` is a very small uniform perturbation used to break near-ties. The default amplitude has already been reduced to `0.001`. Its job is to avoid mechanically fixed ordering caused by input order, not to override meaningful score differences.

Ranking is performed in descending order of `NoisyScore`.

## 6. Score Components and Formulas

### 6.1 Fit: The Primary Matching Signal

`Fit` represents how suitable the broker is for the current lead.

In the current implementation, `Fit` is not computed inside MAQA. MAQA directly reads upstream `fit_score` and clamps it to the range `[0, 1]`.

Its role is:

- the most important positive signal in the ranking
- the main representation of suitability
- not the only factor, but the most heavily weighted one

The default weights prioritize matching quality first.

### 6.2 QuotaGap: Monthly Pacing Adjustment

`QuotaGap` solves the following problem:

- it is not enough to look at how far a broker is from the monthly target in absolute terms
- instead, the algorithm should look at whether the broker is behind or ahead relative to the expected pace at the current day of the month

First define the ideal cumulative target under linear monthly pacing:

```math
Target = quota_q \cdot \frac{day\_index}{days\_in\_month}
```

Then compute normalized deviation:

```math
z = \frac{Target - allocated\_count}{\max(Target, \epsilon_q \cdot quota_q)}
```

Then compress the value using hyperbolic tangent:

```math
QuotaGap = tanh(\alpha_q \cdot z)
```

Business interpretation:

- if the broker is behind pace, `QuotaGap > 0`
- if the broker is near pace, `QuotaGap \approx 0`
- if the broker is clearly ahead, `QuotaGap < 0`

`tanh` is used to keep the term bounded and numerically stable even when a broker is extremely behind or ahead.

### 6.3 Burst: Short-Term Anti-Spike Term

`Burst` solves the following problem:

- a broker may still be behind on monthly pace
- but may already have received too many leads in the last 24 hours
- in that case, the algorithm should still suppress short-term concentration

First define the recent daily baseline from the last 7 days:

```math
Baseline = \frac{last\_7d\_count}{7}
```

Then compare the last 24 hours against this baseline:

```math
z = \frac{last\_24h\_count - Baseline - \delta_b}{\max(Baseline, \epsilon_b)}
```

Then keep only the positive half-axis and cap it:

```math
Burst = \min(b_{max}, \max(0, z))
```

Business interpretation:

- if the last 24 hours are close to normal, `Burst = 0` or very small
- if the last 24 hours are significantly above the baseline, `Burst` increases
- larger `Burst` produces a larger deduction in ranking

So `Burst` is a short-cycle anti-congestion term. It does not answer whether the broker should get leads in general. It answers whether the broker has already been given too many leads in the immediate recent window.

### 6.4 Service: Service Readiness Adjustment

`Service` represents the broker's current operational readiness.

It answers the question:

- is this broker still in a good state to continue taking leads right now?

In the current implementation, `Service` is based mainly on:

- `response_score`: service quality or response capability
- `current_load`: current workload level

If the broker fails the basic participation condition, then `Service = 0`. Otherwise:

```math
Service = clamp(response\_score, 0, 1) \cdot \left(1 - 0.5 \cdot clamp(current\_load, 0, 1)\right)
```

Then it is clamped to `[0, 1]`.

Business interpretation:

- stronger response capability increases `Service`
- higher current load decreases `Service`
- it is a light adjustment term rather than a primary driver

So `Service` is not about lead-to-broker suitability. It is about whether the broker is currently in a good operational state to take more work.

### 6.5 OverQuotaDecay: Tail Decay After Exceeding Quota

`OverQuotaDecay` governs ranking behavior after a broker has already reached or exceeded monthly quota.

It solves the following problem:

- a broker should not be cut off immediately after reaching quota
- but a broker who is already over quota should not keep competing with under-target brokers on exactly equal footing

First define the exceeded amount:

```math
over = \max(0, allocated\_count - quota_q)
```

If there is no over-quota amount:

```math
OverQuotaDecay = 1
```

If the broker is already over quota:

```math
OverQuotaDecay = e^{-\beta \cdot over} \cdot \left(\frac{day\_index}{days\_in\_month}\right)^{\eta}
```

This means:

- the more over quota, the stronger the decay
- the closer to month end, the weaker the decay

Intuition:

- if a broker exceeds quota significantly in the middle of the month, the broker should not keep winning aggressively
- but near the end of the month, a softer tail opportunity is acceptable even for over-quota brokers

This is an important characteristic of MAQA: over-quota brokers are not hard-filtered out. They stay in the ranking with multiplicative tail decay.

## 7. Current Eligibility Handling

Eligibility is intentionally reduced to a minimal form in the current version.

A broker can participate in ranking only if:

- `is_eligible = True`
- `sla_ok = True`

This means:

- complex eligibility rules are not modeled inside MAQA
- city, region, business type, schedule, risk control, and other upstream rules should already be resolved before MAQA is called
- MAQA currently consumes only an aggregated participation result

So Eligibility is treated as an upstream precondition rather than a major modeling dimension inside the ranking algorithm.

## 8. Why the Current Design Uses Weighted Addition Instead of Multiplicative Fit Gating

The current implementation combines `Fit`, `QuotaGap`, `Burst`, and `Service` as a weighted sum instead of using a structure such as “compute an adjustment term and multiply everything by `Fit`”.

Reasons:

1. Better interpretability
   Each term’s contribution can be explained directly.

2. Better stability
   If `Fit` becomes a global gate, low-fit brokers become much less likely to recover ranking even when they are behind monthly pace, calm in the short term, or operationally stronger.

3. Easier parameter tuning
   The current phase aims to establish a runnable, auditable, evolvable baseline. Linear combination is more suitable than deeply nested multiplicative structures for a first version.

So the current MAQA choice is:

- `Fit` is the primary signal
- the other terms are peer adjustment signals
- `OverQuotaDecay` is kept as a special multiplicative tail adjustment

## 9. Parameters and Their Roles

Current default parameters are:

### 9.1 Weight Parameters

- `w_fit = 0.50`
- `w_q = 0.25`
- `w_b = 0.15`
- `w_srv = 0.10`

The current implementation requires the sum of these four weights to be exactly `1.0`:

```math
w_{fit} + w_q + w_b + w_{srv} = 1.0
```

Reasons:

- to keep the scale of `RawScore` stable
- to keep each weight interpretable as relative importance
- to avoid accidentally changing the total score scale during tuning

This default set expresses the current preference:

- prioritize matching quality first
- then account for monthly pacing
- explicitly penalize short-term lead spikes
- lightly adjust by service readiness

### 9.2 QuotaGap Parameters

- `alpha_q = 2.0`
- `epsilon_q = 0.2`

Roles:

- `alpha_q` controls how aggressively monthly pacing deviations are amplified
- `epsilon_q` prevents numerical instability when the month is still early or the target is small

### 9.3 Burst Parameters

- `delta_b = 0.5`
- `epsilon_b = 0.5`
- `b_max = 2.0`

Roles:

- `delta_b` defines a tolerated amount of short-term fluctuation
- `epsilon_b` prevents instability when the recent baseline is very low
- `b_max` caps the maximum influence of extreme spikes

### 9.4 OverQuotaDecay Parameters

- `beta = 0.8`
- `eta = 2.0`

Roles:

- `beta` controls how strongly over-quota amount drives decay
- `eta` controls how much decay relaxes near month end

### 9.5 Noise Parameter

- `noise_eps = 0.001`

Role:

- used only to break near-ties
- intentionally conservative so it does not override meaningful score differences

## 10. Parameter Tuning Guidance

Parameter tuning should not be treated as blind trial-and-error. It should be treated as systematic adjustment of ranking preferences. A practical approach is to define the business objective first, then tune parameters layer by layer.

### 10.1 Start by Defining the Optimization Goal

Before tuning, decide what you want to optimize more strongly.

Common goals include:

- prioritizing matching quality more strongly
- pulling monthly pacing closer to target
- suppressing short-term lead spikes
- improving service readiness stability
- relaxing or tightening the tail opportunity after brokers go over quota

Different goals correspond to different groups of parameters. Do not aggressively tune many directions at once, or attribution becomes unclear.

### 10.2 Recommended Tuning Order

A practical order is:

1. Tune the weight parameters first.
   They directly control the relative importance of score components.

2. Then tune the sensitivity parameters inside each formula.
   Examples: `alpha_q`, `delta_b`, `beta`.

3. Tune the noise parameter last.
   `noise_eps` is only a tie-break tool and should not be used as a primary optimization lever.

### 10.3 How to Tune Weight Parameters

#### `w_fit`

Represents how important matching quality is in the total score.

- Increase it: ranking becomes more dominated by brokers with high `fit_score`.
- Decrease it: monthly pacing, short-term smoothing, and service state more easily affect the ranking.

Use it when:

- you strongly trust fit quality and want matching quality to dominate
- or you want fit to be less dominant because the upstream fit model is still weak

#### `w_q`

Represents how strongly monthly pacing affects ranking.

- Increase it: brokers behind target pace are more strongly compensated.
- Decrease it: ranking becomes closer to fit-plus-service ordering.

Use it when:

- you want smoother monthly progress toward quota
- or you want quota to have less influence on immediate ranking

#### `w_b`

Represents the penalty strength for short-term spikes.

- Increase it: brokers who recently received many leads are suppressed more quickly.
- Decrease it: the system tolerates more short-cycle concentration.

Use it when:

- short-term lead flooding is a real operational issue
- or you care more about immediate conversion and less about short-term concentration

#### `w_srv`

Represents how strongly service readiness affects ranking.

- Increase it: response quality and current load matter more.
- Decrease it: service remains only a light correction.

Use it when:

- service state is operationally important and the upstream features are reliable
- or keep it small when `response_score` and `current_load` are still noisy

### 10.4 How to Tune QuotaGap Parameters

#### `alpha_q`

Controls sensitivity to monthly pace deviation.

- Increase it: even small deviations quickly produce stronger `QuotaGap` values.
- Decrease it: the effect changes more gently.

Use it when:

- you want the system to react more aggressively to monthly underperformance
- or more gently if monthly pacing should be secondary

#### `epsilon_q`

Controls the lower bound of the `QuotaGap` denominator.

- Increase it: early-month behavior becomes more stable.
- Decrease it: early-month behavior becomes more sensitive.

Use it when:

- early-month ranking feels too jumpy
- or you want the system to start differentiating brokers earlier

### 10.5 How to Tune Burst Parameters

#### `delta_b`

Represents the tolerated short-term deviation before penalty starts.

- Increase it: the system becomes more tolerant of mild spikes.
- Decrease it: the system penalizes earlier.

#### `epsilon_b`

Controls the lower bound of the `Burst` denominator.

- Increase it: low-baseline brokers are treated more conservatively.
- Decrease it: low-baseline spikes become more sensitive.

#### `b_max`

Controls the maximum burst penalty.

- Increase it: extreme concentration can hurt more.
- Decrease it: the penalty saturates earlier.

Practical guidance:

- if the problem is short-term flooding, first consider increasing `w_b`
- if the problem is that mild spikes are not noticed early enough, first consider reducing `delta_b`

### 10.6 How to Tune OverQuotaDecay Parameters

#### `beta`

Controls how fast the score decays once a broker is over quota.

- Increase it: over-quota brokers step aside more quickly.
- Decrease it: tail opportunity remains looser.

#### `eta`

Controls how much the decay relaxes near month end.

- Increase it: the difference between mid-month and end-of-month decay becomes more obvious.
- Decrease it: the difference becomes smaller.

Practical guidance:

- if you want over-quota brokers to back off quickly, increase `beta`
- if you still want a softer tail near month end, increase `eta`

### 10.7 How to Tune Noise

#### `noise_eps`

Controls the amplitude of the tie-breaking perturbation.

- Increase it: near-ties are broken more aggressively, but very close rankings become less stable.
- Decrease it: rankings become more stable and easier to reproduce.

Guidance:

- do not increase `noise_eps` unless you actually observe too many ties
- the default `0.001` is intentionally conservative and is usually enough as a tie-breaker

### 10.8 Practical Tuning Rules

1. Tune one group of parameters at a time.
2. Keep the four weight parameters summing to `1.0`.
3. Prefer tuning weights before low-level formula parameters.
4. Prefer small steps rather than large jumps.
5. Use golden-case replay or historical replay rather than intuition alone.
6. Judge the final ranking outcome, not just intermediate score movement.
7. Do not use `noise_eps` as the primary tuning tool.

## 11. Intentional Simplifications in the Current Version

To keep the algorithm core clear, the current version intentionally simplifies several aspects:

1. `Fit` is precomputed upstream instead of inside MAQA
2. Eligibility is reduced to an aggregated boolean result
3. the algorithm ranks only; it does not perform state write-back after allocation
4. monthly pacing uses a linear target curve rather than a more complex pacing curve
5. tie-breaking noise stays simple and minimal

These simplifications intentionally keep MAQA as a clean core ranking library.

## 12. How to Interpret the Result

The output ranking should be interpreted as follows:

- a higher rank means the broker is more favorable under the current rules, parameters, and state snapshot
- the top-ranked broker is usually the default recommendation
- but semantically the algorithm outputs an ordered candidate list rather than a uniquely mandated winner

This means upper-layer systems can still choose to:

- take the first broker directly
- inspect only the top N brokers
- apply manual review
- add further business rules on top of the ranking

## 13. Testing and Verifiability

The current implementation uses two types of tests:

1. Formula tests
   They verify directionality and golden numeric values for terms such as `QuotaGap`, `Burst`, and `OverQuotaDecay`.

2. Golden ranking tests
   They build a fixed broker set and verify the final ranking order and key intermediate values.

This means the current version is not yet a globally optimized, history-calibrated production parameter system, but it already has:

- interpretable formula behavior
- reproducible numeric outputs
- regression-safe ranking logic

## 14. Conclusion

Current MAQA is a core ranking model for broker ordering. Its purpose is not to cover every bit of production complexity. Its purpose is to stably encode a small set of important principles:

- matching quality comes first
- monthly pacing still matters
- short-term flooding should be controlled
- operational service readiness should be respected
- over-quota brokers should receive decaying tail opportunities instead of hard cutoff

On top of these principles, MAQA outputs a ranking that is explainable, auditable, and testable, providing stable ranking capability for upper-layer allocation systems.
