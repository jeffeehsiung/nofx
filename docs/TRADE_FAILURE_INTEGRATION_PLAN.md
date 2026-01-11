# Trade Failure Analysis & Feedback System - Integration Plan

## Executive Summary

Your project already has **three overlapping systems** for learning from trading mistakes:

1. **decision/trade_failure_v2.go** - Microstructure-based trade failure analysis (NEW)
2. **backtest/feedback.go** - Pattern-based performance feedback (EXISTING)
3. **backtest/factor_optimizer.go** - Parameter optimization based on feedback (EXISTING)

These systems were developed independently and are **not yet integrated**. This document provides a plan to unify them into a single, powerful learning loop.

---

## Current State Analysis

**STATUS UPDATE (January 12, 2026)**: Phase 1 (Data Plumbing) is now **✅ COMPLETE**

### System 1: Trade Failure Analysis V2 (`decision/trade_failure_v2.go`)

**Purpose**: Deterministic, evidence-based failure categorization using market microstructure

**Key Features**:
- Uses `RecentOrder` with detailed microstructure data (spread, depth, slippage, OI, volume)
- 10 deterministic failure reason categories (chasing, false breakout, stop too tight, etc.)
- Evidence-based confidence scoring (0.78-0.90)
- Generates specific recommendations per failure type

**Current Status**: ✅ **Implemented and tested** | ✅ **RecentOrder now fully populated with all microstructure data**

**Latest Changes** (January 2026):
- ✅ Real-time MFE/MAE tracking implemented
- ✅ All microstructure fields populated in TradeEvent
- ✅ Spread/depth calculations from actual market data (no magic numbers)
- ✅ GiveBack metric calculated (profit left on table)
- ✅ ATR-based risk metrics (stop distance vs volatility)

**Capabilities**:
```go
analysis := decision.AnalyzeFailedTrade(order)
// Returns:
// - PrimaryReason (e.g., "chasing_entry", "stop_too_tight")
// - ConfidenceScore (0.78-0.90)
// - Evidence map with specific metrics
// - DetailedNotes (plain language explanation)
// - Recommendation (actionable fix)
```

**Data Ready**: `RecentOrder` now contains:
- ✅ EntryPrice, ExitPrice, RealizedPnL, PnLPct
- ✅ EntrySpread, ExitSpread (from real candle ranges)
- ✅ EntryDepth, ExitDepth (from volume statistics)
- ✅ EntrySlippage, EntrySlippageBudget (volatility-adaptive)
- ✅ ATRAtEntry, TrendStrength, ChopScore, MarketRegime, VolatilityRegime
- ✅ MaxFavorableExcursion (MFE), MaxAdverseExcursion (MAE)
- ✅ GiveBackFromPeak, StopDistanceVsATR
- ✅ VolumeAtEntry, OIDeltaAtEntry, volume/OI deltas during trade

### System 2: Feedback Generator (`backtest/feedback.go`)

**Purpose**: Autonomous learning from historical performance patterns

**Key Features**:
- Analyzes closed positions from trade events
- Identifies 9 failure patterns (holding_losers, high_leverage_losses, revenge_trading, etc.)
- Identifies 6 success patterns (quick_exits, optimal_leverage, symbol_affinity, etc.)
- Generates actionable recommendations
- Formats feedback for LLM prompt injection

**Current Status**: ✅ **Fully integrated** into backtest runner, fires every 10 decisions

**Integration Points**:
- `backtest/runner.go:buildDecisionContext()` generates feedback
- Feedback injected into `decision.Context.PerformanceFeedback`
- LLM receives formatted feedback in system prompt

### System 3: Factor Optimizer (`backtest/factor_optimizer.go`)

**Purpose**: Adjusts risk control parameters based on failure patterns

**Key Features**:
- Modifies `RiskControlConfig` (leverage, position size, margins, confidence thresholds)
- Responds to 9 failure patterns from feedback system
- Tracks optimization history
- Provides 30-70% parameter adjustments based on severity

**Current Status**: ✅ **Fully integrated**, runs after feedback generation

**Integration Points**:
- Called from `runner.go` after feedback generation
- Optimized weights attached to `decision.Context.OptimizedWeights`
- Parameters influence next decision cycle

---

## The Gap: Trade Failure Analysis V2 is Isolated

### Problem

`decision/trade_failure_v2.go` provides **granular, microstructure-based** failure analysis but:
- ❌ Not called from `backtest/runner.go`
- ❌ Not integrated with `FeedbackGenerator`
- ❌ `RecentOrder` microstructure fields not populated from backtest positions
- ❌ LLM never sees this detailed failure analysis

### Why This Matters

Trade Failure V2 offers **superior diagnostics**:
- **Execution quality**: Detects slippage budget violations, delayed fills, spread deterioration
- **Market microstructure**: Identifies liquidity drying up, OI/volume decay
- **Risk management**: Flags stops too tight vs ATR, regime mismatches
- **Precise evidence**: Quantified metrics (slippage ratio, depth shrinkage, chop score)

Current feedback system uses **high-level patterns** (e.g., "holding losers > 4h") but misses **why** the trade failed at the execution level.

---

## Unified Architecture Proposal

### Goal

Create a **three-tier learning loop**:

```
┌─────────────────────────────────────────────────────────────┐
│  Tier 1: Execution-Level Diagnosis (Trade Failure V2)      │
│  ↓ Per-trade microstructure analysis                        │
│  ↓ Evidence: slippage, spread, depth, OI delta, ATR        │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  Tier 2: Pattern Detection (Feedback Generator)            │
│  ↓ Aggregates Tier 1 failures into behavioral patterns     │
│  ↓ Evidence: frequency, avg PnL, timing clusters           │
└─────────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────────┐
│  Tier 3: Parameter Optimization (Factor Optimizer)         │
│  ↓ Adjusts leverage, position size, confidence thresholds  │
│  ↓ Tracks optimization history and performance delta       │
└─────────────────────────────────────────────────────────────┘
                          ↓
                 LLM System Prompt
        (receives all 3 tiers of feedback)
```

### Integration Flow

#### Phase 1: Populate Microstructure Data

**File**: `backtest/runner.go`

When closing a position:
1. Build `decision.RecentOrder` from `ClosedPosition`
2. Populate microstructure fields:
   - Entry/exit spread, depth, slippage from execution log
   - ATR, trend strength, chop score from `market.Data`
   - Volume/OI metrics from `market.Data` at entry/exit
   - MFE/MAE from position tracking

**Implementation**:
```go
// In backtest/runner.go after position close
func (r *Runner) buildRecentOrderFromPosition(pos ClosedPosition, mktData *market.Data) *decision.RecentOrder {
    order := &decision.RecentOrder{
        Symbol:       pos.Symbol,
        Side:         pos.Side,
        EntryPrice:   pos.EntryPrice,
        ExitPrice:    pos.ExitPrice,
        RealizedPnL:  pos.RealizedPnL,
        PnLPct:       (pos.RealizedPnL / pos.Value) * 100,
        EntryTime:    time.UnixMilli(pos.EntryTime).Format(time.RFC3339),
        ExitTime:     time.UnixMilli(pos.ExitTime).Format(time.RFC3339),
        HoldDuration: formatDuration(time.Duration(pos.ExitTime - pos.EntryTime) * time.Millisecond),
        Leverage:     pos.Leverage,
    }
    
    // Populate microstructure from market data and execution log
    if mktData != nil {
        order.ATRAtEntry = calculateATR(mktData.IntradaySeries)
        order.TrendStrength = mktData.TrendStrength
        order.ChopScore = mktData.ChopScore
        order.MarketRegime = mktData.MarketRegime
        order.VolumeAtEntry = mktData.VolumeRatio
        order.OIDeltaAtEntry = mktData.OIDelta1h
        // ... more fields
    }
    
    // Populate from execution log (slippage, spread, depth)
    if execData, ok := r.executionLog[pos.ID]; ok {
        order.EntrySlippage = execData.EntrySlippage
        order.EntrySlippageBudget = r.cfg.SlippageBps / 10000.0
        order.EntrySpread = execData.EntrySpread
        order.EntryDepth = execData.EntryDepth
        // ... more fields
    }
    
    return order
}
```

#### Phase 2: Call Trade Failure Analysis

**File**: `backtest/feedback.go`

Extend `FeedbackGenerator` to call `decision.AnalyzeFailedTrade()` for each failed trade:

```go
// Add to FeedbackGenerator struct
type FeedbackGenerator struct {
    // ... existing fields
    microFailureAnalyses []*decision.FailedTradeAnalysis // NEW
}

// Modify identifyFailurePatterns to use V2 analysis
func (fg *FeedbackGenerator) identifyFailurePatterns(outcomes []DecisionOutcome, metrics *Metrics) []TradingPattern {
    patterns := make([]TradingPattern, 0)
    
    // NEW: Aggregate V2 failure analyses by reason
    failureReasonCounts := make(map[decision.TradeFailureReason]int)
    failureReasonPnL := make(map[decision.TradeFailureReason]float64)
    failureReasonEvidence := make(map[decision.TradeFailureReason][]string)
    
    for _, outcome := range outcomes {
        if !outcome.Success && outcome.RecentOrder != nil {
            analysis := decision.AnalyzeFailedTrade(outcome.RecentOrder)
            if analysis != nil {
                fg.microFailureAnalyses = append(fg.microFailureAnalyses, analysis)
                
                reason := analysis.PrimaryReason
                failureReasonCounts[reason]++
                failureReasonPnL[reason] += outcome.RealizedPnLPct
                
                // Collect evidence snippets
                evidenceSnippet := analysis.DetailedNotes
                if len(evidenceSnippet) > 100 {
                    evidenceSnippet = evidenceSnippet[:100] + "..."
                }
                failureReasonEvidence[reason] = append(failureReasonEvidence[reason], evidenceSnippet)
            }
        }
    }
    
    // Convert V2 failure reasons to patterns
    for reason, count := range failureReasonCounts {
        if count >= fg.config.MinPatternFrequency {
            avgPnL := failureReasonPnL[reason] / float64(count)
            patterns = append(patterns, TradingPattern{
                PatternType:    string(reason), // e.g., "chasing_entry", "stop_too_tight"
                Frequency:      count,
                AvgPnLPct:      avgPnL,
                Description:    getDescriptionForReason(reason),
                Evidence:       failureReasonEvidence[reason],
                Recommendation: getRecommendationForReason(reason),
            })
        }
    }
    
    // ... continue with existing high-level patterns
    // (holding_losers, revenge_trading, etc.)
    
    return patterns
}
```

#### Phase 3: Enhanced Feedback Format

**File**: `backtest/feedback.go`

Update `FormatForPrompt()` to include microstructure diagnostics:

```go
func (analysis *FeedbackAnalysis) FormatForPrompt(lang string) string {
    var sb strings.Builder
    
    // ... existing overall metrics
    
    // NEW: Microstructure failure breakdown
    sb.WriteString("### 🔬 Execution-Level Failure Analysis\n\n")
    
    microFailureCounts := make(map[decision.TradeFailureReason]int)
    for _, pattern := range analysis.FailurePatterns {
        if reason := decision.TradeFailureReason(pattern.PatternType); isV2Reason(reason) {
            microFailureCounts[reason] = pattern.Frequency
        }
    }
    
    if len(microFailureCounts) > 0 {
        sb.WriteString("**Root causes identified from market microstructure:**\n")
        for reason, count := range microFailureCounts {
            sb.WriteString(fmt.Sprintf("- **%s** (%d occurrences): %s\n", 
                humanizeReason(reason), 
                count,
                getShortRecommendation(reason)))
        }
        sb.WriteString("\n")
    }
    
    // ... existing patterns and recommendations
    
    return sb.String()
}
```

#### Phase 4: Factor Optimizer Extension

**File**: `backtest/factor_optimizer.go`

Extend optimizer to respond to V2 failure reasons:

```go
func (fo *FactorOptimizer) OptimizeWeights(feedback *FeedbackAnalysis, cycle int) error {
    // ... existing code
    
    // NEW: Respond to microstructure failures
    for _, pattern := range feedback.FailurePatterns {
        reason := decision.TradeFailureReason(pattern.PatternType)
        
        switch reason {
        case decision.ReasonStopTooTight:
            // Increase stop distance multiplier
            newConfig.StopATRMultiplier *= 1.3
            improvements = append(improvements, "Widened stops (1.3x) due to premature stop-outs")
            
        case decision.ReasonChasingEntry:
            // Tighten slippage budget and add execution delay
            newConfig.MaxSlippageBps = int(float64(newConfig.MaxSlippageBps) * 0.7)
            newConfig.MinConfidence += 5 // Require higher confidence for entries
            improvements = append(improvements, "Tightened slippage tolerance and raised confidence threshold")
            
        case decision.ReasonFalseBreakoutV2:
            // Require stronger confirmation signals
            newConfig.MinVolumeConfirmation = 1.1 // 110% of baseline
            newConfig.MinOIConfirmation = 0.50    // 50% OI increase
            improvements = append(improvements, "Strengthened breakout confirmation criteria")
            
        case decision.ReasonMomentumDecay:
            // Add momentum monitoring and earlier exits
            newConfig.EnableMomentumMonitoring = true
            newConfig.TrailingStopMultiplier *= 0.9 // Tighter trailing stops
            improvements = append(improvements, "Enabled momentum monitoring with tighter trailing stops")
            
        case decision.ReasonRegimeMismatch:
            // Add regime filters
            newConfig.MaxChopScoreThreshold = 0.5 // Avoid choppy markets
            newConfig.MinTrendStrength = 0.3      // Require trending conditions
            improvements = append(improvements, "Added regime filters (trending markets only)")
        }
    }
    
    // ... existing leverage/position size optimizations
    
    return nil
}
```

---

## Implementation Checklist

### Phase 1: Data Plumbing (Week 1)

**STATUS: ✅ COMPLETE**

- [x] Add execution microstructure capture to `backtest/runner.go`
  - [x] Capture real spreads from candle high-low ranges
  - [x] Calculate depth from volume statistics
  - [x] Volatility-adaptive slippage budgets
  - [x] ATR-based stop distance sizing
- [x] Implement `buildRecentOrderFromPosition()` function
  - [x] Map `ClosedPosition` → `decision.RecentOrder`
  - [x] Populate all 30+ microstructure fields from market data
  - [x] Build verified with clean compile
- [x] Add MFE/MAE excursion tracking
  - [x] Real-time tracking of max favorable/adverse excursion
  - [x] Capture on close and populate TradeEvent
  - [x] Calculate GiveBack metric
- [x] Extend `DecisionOutcome` in `backtest/feedback.go`
  - [x] Add `RecentOrder *decision.RecentOrder` field
  - [x] Populate during `createDecisionOutcomes()`

**Completed Documentation**:
- [docs/PHASE_1_IMPLEMENTATION_SUMMARY.md](PHASE_1_IMPLEMENTATION_SUMMARY.md)
- [docs/MICROSTRUCTURE_REFACTORING.md](MICROSTRUCTURE_REFACTORING.md)
- [docs/MFE_MAE_IMPLEMENTATION.md](MFE_MAE_IMPLEMENTATION.md)

### Phase 2: Analysis Integration (Week 2)

**STATUS: 🔄 IN PROGRESS**

- [ ] Modify `FeedbackGenerator.identifyFailurePatterns()`
  - [ ] Call `decision.AnalyzeFailedTrade()` for each failed trade
  - [ ] Aggregate V2 reasons into patterns
  - [ ] Preserve evidence and recommendations
- [ ] Add helper functions to `backtest/feedback.go`
  - [ ] `isV2Reason()` - check if pattern is from Trade Failure V2
  - [ ] `humanizeReason()` - format reason for prompt display
  - [ ] `getShortRecommendation()` - extract recommendation text
- [ ] Update `FeedbackAnalysis` struct
  - [ ] Add `MicroFailureAnalyses []*decision.FailedTradeAnalysis`
  - [ ] Persist to JSON for debugging

### Phase 3: Prompt Enhancement (Week 3)

- [ ] Update `FormatForPrompt()` in `backtest/feedback.go`
  - [ ] Add "Execution-Level Failure Analysis" section
  - [ ] Display V2 failure reasons with counts and evidence
  - [ ] Integrate into existing prompt structure
- [ ] Test prompt injection in backtest runs
  - [ ] Verify LLM receives microstructure feedback
  - [ ] Validate prompt length stays within token limits
  - [ ] A/B test with/without V2 feedback

### Phase 4: Optimizer Extension (Week 4)

- [ ] Extend `FactorOptimizer.OptimizeWeights()`
  - [ ] Add switch cases for all V2 failure reasons
  - [ ] Define parameter adjustments per reason
  - [ ] Track V2-triggered optimizations separately
- [ ] Add new `RiskControlConfig` fields as needed
  - [ ] `StopATRMultiplier` (for ReasonStopTooTight)
  - [ ] `MinVolumeConfirmation`, `MinOIConfirmation` (for ReasonFalseBreakoutV2)
  - [ ] `MaxChopScoreThreshold`, `MinTrendStrength` (for ReasonRegimeMismatch)
- [ ] Persist optimizer state with V2 parameters

### Phase 5: Testing & Validation (Week 5)

- [ ] Unit tests for all new integration points
- [ ] Integration tests with sample backtest runs
- [ ] Compare win rate before/after V2 integration
- [ ] Validate feedback format in LLM prompts
- [ ] Performance profiling (ensure no latency regression)

### Phase 6: Documentation (Week 6)

- [ ] Update `FEEDBACK_LOOP_GUIDE.md` with V2 integration
- [ ] Add architecture diagrams for 3-tier loop
- [ ] Document new `RiskControlConfig` fields
- [ ] Create examples of V2 feedback in prompts
- [ ] Write troubleshooting guide for common issues

---

## Benefits of Integration

### 1. Superior Diagnostics

**Before** (Pattern-based only):
> ⚠️ Holding losing positions for too long (>4h) (8 times, -5.20% avg)
> → Set and respect stop-losses

**After** (With V2 microstructure):
> ⚠️ Stop distance too tight vs ATR (8 times, -5.20% avg)
> → Evidence: Stop was 0.8x ATR (need 1.5x minimum)
> → Recommendation: Increase stop to 1.5x ATR for this volatility regime

### 2. Actionable Parameter Adjustments

V2 enables **precise** optimizer responses:
- Instead of "reduce leverage 30%", can do "tighten slippage budget 30% due to chasing entries"
- Instead of "trade less", can do "require 110% volume confirmation due to false breakouts"
- Instead of "cut losses faster", can do "widen stops from 0.8x ATR to 1.5x ATR"

### 3. Faster Learning Loop

With execution-level diagnostics, the LLM can:
- Identify **why** a trade failed (not just **that** it failed)
- Learn from 1-2 examples (vs 5-10 needed for pattern detection)
- Adjust behavior mid-backtest (vs waiting for statistical significance)

### 4. Market Microstructure Awareness

V2 teaches the LLM about:
- Liquidity regimes (tight spreads = safe, widening = danger)
- Volume confirmation (weak volume = false signal)
- Regime matching (trending vs choppy, high vs low volatility)
- Execution quality (slippage, fill delays, order book depth)

---

## Migration Strategy

### Current Status & Timeline Adjustment

**Phase 1 Complete** (Jan 12, 2026): All data plumbing done
- Microstructure data flowing: execution → TradeEvent → ClosedPosition → RecentOrder
- MFE/MAE tracking active
- Zero magic numbers in calculations
- Ready for Phase 2

**Updated Timeline**:
- ✅ **Phase 1** (1 week): COMPLETE - Data plumbing and excursion tracking
- 🔄 **Phase 2** (1-2 weeks): THIS WEEK - Wire V2 analysis into feedback loop
- ⏳ **Phase 3** (1 week): Next week - Prompt enhancement with V2 diagnostics
- ⏳ **Phase 4** (1 week): Following week - Optimizer extension for V2 failures
- ⏳ **Phase 5** (1 week): Validation and testing
- ⏳ **Phase 6** (1 week): Documentation and deployment

**Total Estimated Time**: 6 weeks from now (completed in early February 2026)

### Option A: Phased Rollout (Recommended - CURRENT PLAN)

1. **THIS WEEK**: Complete Phase 2 (wire AnalyzeFailedTrade calls)
2. **NEXT WEEK**: Phase 3 (prompt injection with V2 feedback)
3. **WEEK 3**: Phase 4 (optimizer parameter adjustments)
4. **WEEK 4**: Phase 5 (A/B testing and validation)
5. **WEEKS 5-6**: Phase 6 (documentation and full rollout)

### Option B: Parallel Systems

- Keep existing feedback system as-is
- Add V2 as **supplementary** analysis (separate prompt section)
- Let both systems run for 1 month, compare insights
- Gradually phase out redundant pattern detection

### Option C: Full Replacement

- Replace high-level patterns with V2-only analysis
- Simplify `feedback.go` to just aggregate V2 results
- Riskier: loses existing behavioral patterns (revenge trading, etc.)
- **Not recommended**: both systems provide value

---

## Expected Outcomes

### Metrics to Track

1. **Win Rate Improvement**
   - Baseline: Current backtest win rate (e.g., 35%)
   - Target: +5-10% absolute improvement (40-45%)
   - Timeframe: 30-day backtest runs

2. **Drawdown Reduction**
   - Baseline: Current max drawdown (e.g., 18%)
   - Target: -20% relative reduction (14.4%)
   - Mechanism: Better stop management, regime filtering

3. **Profit Factor**
   - Baseline: <1.0 (losses exceed profits)
   - Target: >1.2 (sustainable profitability)
   - Mechanism: Avoid false breakouts, timing improvements

4. **Learning Speed**
   - Baseline: 50+ decisions before pattern detection
   - Target: 10-20 decisions before microstructure feedback
   - Measurement: Time to first meaningful recommendation

### Success Criteria

✅ **Phase 1 Success**: `RecentOrder` populated with all microstructure fields for 100% of closed positions

✅ **Phase 2 Success**: V2 failure reasons appear in feedback analysis with correct evidence

✅ **Phase 3 Success**: LLM prompt includes execution-level diagnostics with no token overflow

✅ **Phase 4 Success**: Optimizer makes at least 3 V2-triggered parameter adjustments per 100 decisions

✅ **Phase 5 Success**: A/B test shows +5% win rate or +0.2 profit factor improvement

---

## Risk Mitigation

### Risk 1: Token Limit Overflow

**Problem**: Adding V2 feedback may exceed LLM context limits

**Mitigation**:
- Summarize V2 evidence (max 1-2 sentences per failure)
- Top 3-5 failure reasons only (by frequency)
- Feature flag to disable V2 feedback if needed

### Risk 2: Optimizer Over-Tuning

**Problem**: Too many parameter changes cause instability

**Mitigation**:
- Limit V2-triggered changes to 1-2 per optimization cycle
- Track optimization history and revert if performance degrades
- Require 10+ occurrences before triggering V2 parameter change

### Risk 3: Data Quality Issues

**Problem**: Missing/incorrect microstructure data leads to wrong diagnoses

**Mitigation**:
- Add validation checks in `buildRecentOrderFromPosition()`
- Log warnings when fields are missing/zero
- Fall back to high-level patterns if microstructure incomplete
- Unit tests for all field mappings

### Risk 4: Integration Bugs

**Problem**: Breaking changes to existing feedback system

**Mitigation**:
- Keep V2 integration behind feature flag initially
- Extensive unit/integration testing before rollout
- Monitor error rates in production backtests
- Quick rollback plan if issues arise

---

## Next Steps

1. **Review this document** with the team and prioritize phases
2. **Create feature flag** (`enable_v2_feedback`) in `BacktestConfig`
3. **Start with Phase 1** (data plumbing) - lowest risk, highest value
4. **Run baseline backtests** to establish comparison metrics
5. **Implement incrementally** with A/B testing at each phase

---

## Questions for Consideration

1. **Prompt Length**: What is your LLM's context limit? (Need to budget tokens for V2 feedback)
2. **Update Frequency**: Should V2 feedback be injected every decision or only after 10 decisions (like current feedback)?
3. **Prioritization**: Which failure reasons are most critical to fix first? (Start with highest-impact reasons)
4. **Parameter Budget**: What's the acceptable rate of parameter changes per cycle? (Too many = instability)
5. **Validation Criteria**: What win rate/profit factor improvement would justify full rollout?

---

## Conclusion

You have **three powerful components** that, when integrated, will create a state-of-the-art learning loop:

- **Trade Failure V2** = Precise execution diagnostics
- **Feedback Generator** = Behavioral pattern detection
- **Factor Optimizer** = Autonomous parameter tuning

The integration is **straightforward** (mostly wiring existing functions together) and **low-risk** (can be feature-flagged and A/B tested).

**Expected impact**: +5-10% win rate improvement, -20% drawdown reduction, faster learning convergence.

**Recommended timeline**: 6 weeks phased rollout with A/B testing.

Let me know when you're ready to start implementation! 🚀
