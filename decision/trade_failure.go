package decision

import (
	"fmt"
	"math"
)

// ============================================================================
// Production-Grade Trade Failure Analysis V2
// ============================================================================
// Deterministic, evidence-based failure categorization using market microstructure,
// volatility, regime, and execution data.
// ============================================================================

// TradeFailureReason production-grade failure reasons with evidence (unified)
type TradeFailureReason string

const (
	// Pre-Trade / Signal Quality
	ReasonSignalQualityLow  TradeFailureReason = "signal_quality_low"  // Weak edge, low expectancy
	ReasonRegimeMismatch    TradeFailureReason = "regime_mismatch"     // Strategy ↔ regime mismatch
	ReasonLiquidityRiskHigh TradeFailureReason = "liquidity_risk_high" // Spread/depth/slippage too high
	ReasonStackedRisk       TradeFailureReason = "stacked_risk"        // Overexposed to same factor

	// Entry Execution
	ReasonChasingEntry     TradeFailureReason = "chasing_entry"     // Late entry, adverse slippage
	ReasonFalseBreakoutV2  TradeFailureReason = "false_breakout_v2" // No follow-through, no confirm
	ReasonPrematureEntry   TradeFailureReason = "premature_entry"   // Before confirmation criteria
	ReasonSizingError      TradeFailureReason = "sizing_error"      // Too big for liquidity
	ReasonSlippageExceeded TradeFailureReason = "slippage_exceeded" // Impact > budget

	// During Trade
	ReasonStopTooTight        TradeFailureReason = "stop_too_tight"         // Stop < 1.5x ATR
	ReasonMomentumDecay       TradeFailureReason = "momentum_decay"         // Volume/OI collapse
	ReasonLiquidityDried      TradeFailureReason = "liquidity_dried"        // Spread widened, depth fell
	ReasonStopHitRegimeChange TradeFailureReason = "stop_hit_regime_change" // Trend reversed, market shifted

	// Exit / Exit Timing
	ReasonLateExitGiveBack     TradeFailureReason = "late_exit_giveback"     // Large give-back from peak
	ReasonTrendReversalIgnored TradeFailureReason = "trend_reversal_ignored" // Missed reversal signal
	ReasonHighSlippageExit     TradeFailureReason = "high_slippage_exit"     // Poor exit execution

	// Process / Control
	ReasonFundingDrag       TradeFailureReason = "funding_drag"        // Funding cost ate profit
	ReasonBorrowingCostHigh TradeFailureReason = "borrowing_cost_high" // Borrow cost significant
	ReasonTechnicalFault    TradeFailureReason = "technical_fault"     // Execution error, system issue
)

// RecentOrderV2 removed; using unified `RecentOrder` from engine.go

// FailedTradeAnalysis comprehensive result with evidence tracking
type FailedTradeAnalysis struct {
	PrimaryReason    TradeFailureReason     `json:"primary_reason"`
	ConfidenceScore  float64                `json:"confidence_score"`  // 0-1
	SecondaryReasons []TradeFailureReason   `json:"secondary_reasons"` // Related issues
	Evidence         map[string]interface{} `json:"evidence"`          // Supporting metrics
	DetailedNotes    string                 `json:"detailed_notes"`    // Plain language
	Recommendation   string                 `json:"recommendation"`    // Actionable fix
}

// ============================================================================
// Main Analysis Function
// ============================================================================

// AnalyzeFailedTrade deterministically categorizes failed trades with evidence
// Uses default thresholds if thresholds parameter is nil
func AnalyzeFailedTrade(order *RecentOrder) *FailedTradeAnalysis {
	return AnalyzeFailedTradeWithThresholds(order, nil)
}

// AnalyzeFailedTradeWithThresholds allows using calibrated thresholds from historical data
func AnalyzeFailedTradeWithThresholds(order *RecentOrder, thresholds *FailureThresholds) *FailedTradeAnalysis {
	if order == nil {
		return nil
	}

	// Use defaults if no thresholds provided
	if thresholds == nil {
		defaultThresholds := DefaultFailureThresholds()
		thresholds = &defaultThresholds
	}

	analysis := &FailedTradeAnalysis{
		Evidence: make(map[string]interface{}),
	}

	// Evaluate rules in priority order, pick FIRST matching rule with highest specificity
	// Earlier rules are more specific and take precedence
	ruleSequence := []struct {
		check      func(*RecentOrder, *FailureThresholds) bool
		reason     TradeFailureReason
		confidence float64
	}{
		// High confidence, highly specific rules (check first)
		{isChasing, ReasonChasingEntry, 0.90},
		{isLateExitGiveBack, ReasonLateExitGiveBack, 0.88},
		{isStopTooTight, ReasonStopTooTight, 0.88},
		{isMomentumDecay, ReasonMomentumDecay, 0.85},
		{isFalseBreakout, ReasonFalseBreakoutV2, 0.85},
		{isLiquidityDried, ReasonLiquidityDried, 0.83},
		{isPrematureEntry, ReasonPrematureEntry, 0.82},
		{isStopHitRegimeChange, ReasonStopHitRegimeChange, 0.80},
		{isHighSlippageRegime, ReasonSlippageExceeded, 0.78},
		{isFundingDrag, ReasonFundingDrag, 0.79},
	}

	// Find FIRST matching rule (highest priority match wins)
	var bestMatch *struct {
		check      func(*RecentOrder, *FailureThresholds) bool
		reason     TradeFailureReason
		confidence float64
	}

	for i := range ruleSequence {
		rule := &ruleSequence[i]
		if rule.check(order, thresholds) {
			bestMatch = rule
			break // Take the first match (highest priority)
		}
	}

	// Assign primary reason and confidence
	if bestMatch != nil {
		analysis.PrimaryReason = bestMatch.reason
		analysis.ConfidenceScore = bestMatch.confidence
	} else {
		analysis.PrimaryReason = ReasonSignalQualityLow // Fallback
		analysis.ConfidenceScore = 0.60
	}

	// Populate evidence based on primary reason
	populateEvidence(analysis, order)

	// Generate recommendation
	analysis.Recommendation = generateRecommendation(analysis.PrimaryReason, order, analysis.Evidence)

	// Generate detailed notes
	analysis.DetailedNotes = generateDetailedNotes(analysis.PrimaryReason, order, analysis.Evidence)

	return analysis
}

// ============================================================================
// Deterministic Rule Functions
// ============================================================================

// isChasing detects late entry with adverse slippage
func isChasing(order *RecentOrder, _ *FailureThresholds) bool {
	if order == nil {
		return false
	}
	// Rule 1: Slippage exceeds budget by significant margin
	if order.EntrySlippage > 0 && order.EntrySlippageBudget > 0 {
		if order.EntrySlippage > order.EntrySlippageBudget*1.5 {
			return true
		}
	}
	// Rule 2: Fill time far from signal (delayed execution = late entry)
	if order.EntryFillTime > 0 && order.SignalTime > 0 {
		timeToFill := order.EntryFillTime - order.SignalTime
		if timeToFill > 2000 { // 2+ seconds = chasing in fast market
			return true
		}
	}
	return false
}

// isFalseBreakoutV2 detects breakouts without volume or OI confirmation
func isFalseBreakout(order *RecentOrder, thresholds *FailureThresholds) bool {
	if order == nil {
		return false
	}

	// Use defaults if no thresholds provided
	if thresholds == nil {
		defaults := DefaultFailureThresholds()
		thresholds = &defaults
	}

	// Use provided values; if percent-like (>10), convert to ratio.
	volumeRatio := order.VolumeAtEntry
	if volumeRatio > 10 {
		volumeRatio = volumeRatio / 100.0
	}
	oiRatio := order.OIDeltaAtEntry
	if oiRatio > 10 {
		oiRatio = oiRatio / 100.0
	}

	// Rule: Both volume AND OI must be weak (use calibrated thresholds)
	weakVolume := volumeRatio < thresholds.WeakVolumeThreshold
	weakOI := oiRatio < thresholds.WeakOIThreshold
	return weakVolume && weakOI
}

// isPrematureEntry detects entries before confirmation criteria
func isPrematureEntry(order *RecentOrder, thresholds *FailureThresholds) bool {
	if order == nil {
		return false
	}

	// Use defaults if no thresholds provided
	if thresholds == nil {
		defaults := DefaultFailureThresholds()
		thresholds = &defaults
	}

	volumeRatio := order.VolumeAtEntry
	if volumeRatio > 10 {
		volumeRatio = volumeRatio / 100.0
	}
	oiRatio := order.OIDeltaAtEntry
	if oiRatio > 10 {
		oiRatio = oiRatio / 100.0
	}

	// Rule: Both volume and OI below thresholds (use calibrated values)
	lowVolume := volumeRatio < thresholds.PrematureVolumeThreshold
	lowOI := oiRatio < thresholds.PrematureOIThreshold
	return lowVolume && lowOI
}

// isStopTooTight detects stops closer than risk management threshold
func isStopTooTight(order *RecentOrder, _ *FailureThresholds) bool {
	if order == nil {
		return false
	}
	// Rule: Stop distance < 1.5x ATR (industry standard)
	return order.StopDistanceVsATR < 1.5
}

// isMomentumDecay detects volume/OI collapse during trade
func isMomentumDecay(order *RecentOrder, thresholds *FailureThresholds) bool {
	if order == nil {
		return false
	}

	// Use defaults if no thresholds provided
	if thresholds == nil {
		defaults := DefaultFailureThresholds()
		thresholds = &defaults
	}

	// Rule: Both volume AND OI decline significantly (use calibrated thresholds)
	volumeDropped := order.VolumeDeltaDuringTrade < thresholds.VolumeDecayThreshold
	oiDropped := order.OIDeltaDuringTrade < thresholds.OIDecayThreshold
	return volumeDropped && oiDropped
}

// isLiquidityDried detects spread widening and depth reduction
func isLiquidityDried(order *RecentOrder, thresholds *FailureThresholds) bool {
	if order == nil {
		return false
	}

	// Use defaults if no thresholds provided
	if thresholds == nil {
		defaults := DefaultFailureThresholds()
		thresholds = &defaults
	}

	// Rule: Both spread widened AND depth shrunk significantly (use calibrated values)
	if order.EntrySpread > 0 && order.ExitSpread > 0 {
		spreadWorsened := order.ExitSpread > (order.EntrySpread * thresholds.SpreadWorseningMultiple)
		depthShrunk := order.ExitDepth < (order.EntryDepth * thresholds.DepthReductionThreshold)
		return spreadWorsened && depthShrunk
	}
	return false
}

// isStopHitRegimeChange detects trend reversal or market regime shift
func isStopHitRegimeChange(order *RecentOrder, _ *FailureThresholds) bool {
	if order == nil {
		return false
	}
	// Approximate regime risk using unified fields (no entry/exit split available)
	// If market is not trending and chop is high, consider regime change risk.
	if order.MarketRegime != "trending" {
		if order.ChopScore > 0.5 {
			return true
		}
	}
	// If trend strength is weak (|trend| < 0.2), treat as unfavorable regime
	if math.Abs(order.TrendStrength) < 0.2 {
		return true
	}
	return false
}

// isLateExitGiveBack detects poor exit timing with large give-back
func isLateExitGiveBack(order *RecentOrder, _ *FailureThresholds) bool {
	if order == nil {
		return false
	}
	// Rule: Had 3%+ profit at peak but gave back >30% of gains
	if order.MaxFavorableExcursion > 0.03 {
		percentGiveBack := order.GiveBackFromPeak / order.MaxFavorableExcursion
		return percentGiveBack > 0.30
	}
	return false
}

// isHighSlippageRegime detects slippage that's excessive for market conditions
func isHighSlippageRegime(order *RecentOrder, _ *FailureThresholds) bool {
	if order == nil {
		return false
	}
	// In normal/low vol with good liquidity, slippage should be low
	if order.VolatilityRegime != "high" && order.VolumeAtEntry >= 1.0 && order.EntrySpread < 0.02 {
		// Normal conditions: slippage should be < 3%
		return order.EntrySlippage > 0.03
	}
	return false
}

// isFundingDrag detects when funding/borrow costs are significant
func isFundingDrag(order *RecentOrder, _ *FailureThresholds) bool {
	if order == nil {
		return false
	}
	totalCost := order.FundingAccrued + order.BorrowCostAccrued
	// If costs > 20% of realized loss, funding was the culprit
	if order.RealizedPnL < 0 {
		return totalCost > math.Abs(order.RealizedPnL)*0.20
	}
	return false
}

// ============================================================================
// Evidence Population
// ============================================================================

// populateEvidence builds evidence map for the primary reason
func populateEvidence(analysis *FailedTradeAnalysis, order *RecentOrder) {
	switch analysis.PrimaryReason {
	case ReasonChasingEntry:
		analysis.Evidence["entry_slippage"] = order.EntrySlippage
		analysis.Evidence["slippage_budget"] = order.EntrySlippageBudget
		analysis.Evidence["slippage_ratio"] = order.EntrySlippage / order.EntrySlippageBudget
		if order.EntryFillTime > 0 && order.SignalTime > 0 {
			analysis.Evidence["fill_delay_ms"] = order.EntryFillTime - order.SignalTime
		}

	case ReasonFalseBreakoutV2:
		analysis.Evidence["volume_at_entry"] = order.VolumeAtEntry
		analysis.Evidence["oi_delta_at_entry"] = order.OIDeltaAtEntry
		v := order.VolumeAtEntry
		if v > 10 {
			v = v / 100.0
		}
		oi := order.OIDeltaAtEntry
		if oi > 10 {
			oi = oi / 100.0
		}
		analysis.Evidence["volume_strength"] = fmt.Sprintf("%.0f%%", v*100)
		analysis.Evidence["oi_strength"] = fmt.Sprintf("%.0f%%", oi*100)

	case ReasonPrematureEntry:
		v := order.VolumeAtEntry
		if v > 10 {
			v = v / 100.0
		}
		oi := order.OIDeltaAtEntry
		if oi > 10 {
			oi = oi / 100.0
		}
		analysis.Evidence["volume_check"] = fmt.Sprintf("%.0f%% (need 90%%)", v*100)
		analysis.Evidence["oi_check"] = fmt.Sprintf("%.0f%% (need 50%%)", oi*100)

	case ReasonStopTooTight:
		analysis.Evidence["stop_distance_vs_atr"] = order.StopDistanceVsATR
		analysis.Evidence["atr_at_entry"] = order.ATRAtEntry
		analysis.Evidence["stop_distance"] = order.StopDistance
		analysis.Evidence["recommended_minimum"] = order.ATRAtEntry * 1.5

	case ReasonMomentumDecay:
		analysis.Evidence["volume_delta"] = fmt.Sprintf("%.1f%%", order.VolumeDeltaDuringTrade*100)
		analysis.Evidence["oi_delta"] = fmt.Sprintf("%.1f%%", order.OIDeltaDuringTrade*100)

	case ReasonLiquidityDried:
		analysis.Evidence["entry_spread"] = order.EntrySpread
		analysis.Evidence["exit_spread"] = order.ExitSpread
		analysis.Evidence["spread_ratio"] = order.ExitSpread / order.EntrySpread
		analysis.Evidence["entry_depth"] = order.EntryDepth
		analysis.Evidence["exit_depth"] = order.ExitDepth
		analysis.Evidence["depth_ratio"] = order.ExitDepth / order.EntryDepth

	case ReasonStopHitRegimeChange:
		analysis.Evidence["trend_strength"] = order.TrendStrength
		analysis.Evidence["market_regime"] = order.MarketRegime
		analysis.Evidence["chop_score"] = order.ChopScore

	case ReasonLateExitGiveBack:
		analysis.Evidence["max_favorable_excursion"] = fmt.Sprintf("%.2f%%", order.MaxFavorableExcursion*100)
		analysis.Evidence["giveback"] = fmt.Sprintf("%.2f%%", order.GiveBackFromPeak*100)
		percentGiveBack := (order.GiveBackFromPeak / order.MaxFavorableExcursion) * 100
		analysis.Evidence["percent_of_profit_given_back"] = fmt.Sprintf("%.0f%%", percentGiveBack)

	case ReasonSlippageExceeded:
		analysis.Evidence["entry_slippage"] = fmt.Sprintf("%.2f%%", order.EntrySlippage*100)
		analysis.Evidence["volatility_regime"] = order.VolatilityRegime
		analysis.Evidence["volume_level"] = order.VolumeAtEntry
		analysis.Evidence["spread_at_entry"] = order.EntrySpread

	case ReasonFundingDrag:
		analysis.Evidence["funding_accrued"] = order.FundingAccrued
		analysis.Evidence["borrow_cost"] = order.BorrowCostAccrued
		analysis.Evidence["total_cost"] = order.FundingAccrued + order.BorrowCostAccrued
		analysis.Evidence["realized_pnl"] = order.RealizedPnL
	}
}

// ============================================================================
// Recommendation Generation
// ============================================================================

// generateRecommendationV2 generates actionable fix for each failure reason
func generateRecommendation(reason TradeFailureReason, order *RecentOrder, evidence map[string]interface{}) string {
	switch reason {
	case ReasonChasingEntry:
		if slippageRatio, ok := evidence["slippage_ratio"].(float64); ok {
			return fmt.Sprintf("Use limit orders and wait for confirmed signal. Slippage was %.2fx budget (%.1f%%). Reduce position size or wait for better liquidity.",
				slippageRatio, order.EntrySlippage*100)
		}
		return "Use limit orders with tighter parameters. Wait for confirmed signal before entering. Reduce position size when spreads/depth deteriorate."

	case ReasonFalseBreakoutV2:
		if volStr, ok := evidence["volume_strength"].(string); ok {
			if oiStr, ok := evidence["oi_strength"].(string); ok {
				return fmt.Sprintf("Require BOTH volume >110%% AND OI >50%% increase. Current: volume %s, OI %s. Add pullback entry after full confirmation.", volStr, oiStr)
			}
		}
		return "Require BOTH volume >110% baseline AND OI >50% increase. Use breakout filters: check 5m/15m candles for true breakout patterns."

	case ReasonPrematureEntry:
		if volCheck, ok := evidence["volume_check"].(string); ok {
			if oiCheck, ok := evidence["oi_check"].(string); ok {
				return fmt.Sprintf("Enforce confirmation: volume %s, OI %s. Wait until both criteria met before entering.", volCheck, oiCheck)
			}
		}
		return "Enforce confirmation criteria: wait for volume >90%% AND OI increase >50%%. Add time filter: only enter after X candles of confirmation."

	case ReasonStopTooTight:
		if recommended, ok := evidence["recommended_minimum"].(float64); ok {
			if stopDist, ok := evidence["stop_distance"].(float64); ok {
				return fmt.Sprintf("Stop is too tight (%.2f away). Increase to minimum %.2f (%.2fx ATR). This prevents normal volatility stops.",
					stopDist, recommended, recommended/order.ATRAtEntry)
			}
		}
		return fmt.Sprintf("Increase stop to at least %.2fx ATR (currently %.2fx). For this trade, stop should be ≥%.2f away from entry.",
			1.5, order.StopDistanceVsATR, order.ATRAtEntry*1.5)

	case ReasonMomentumDecay:
		if volDelta, ok := evidence["volume_delta"].(string); ok {
			if oiDelta, ok := evidence["oi_delta"].(string); ok {
				return fmt.Sprintf("Monitor momentum: volume %s, OI %s. Exit with trailing stop when both decline >15%%. Consider tighter profit targets.", volDelta, oiDelta)
			}
		}
		return "Monitor volume/OI during trade. Exit with trailing stop when volume falls >20% and OI declines >15%%. Consider tighter profit targets."

	case ReasonLiquidityDried:
		if spreadRatio, ok := evidence["spread_ratio"].(float64); ok {
			if depthRatio, ok := evidence["depth_ratio"].(float64); ok {
				return fmt.Sprintf("Liquidity deteriorated: spread widened %.1fx, depth fell to %.0f%%. Check depth before entry. Use smaller sizes in illiquid markets.",
					spreadRatio, depthRatio*100)
			}
		}
		return "Check order book depth before entry. Avoid positions when spreads >0.15% or depth <$1M. Use limit orders. Consider smaller position size."

	case ReasonStopHitRegimeChange:
		if regime, ok := evidence["market_regime"].(string); ok {
			if chopScore, ok := evidence["chop_score"].(float64); ok {
				return fmt.Sprintf("Regime risk in '%s' market (chop: %.2f). Add filters: skip trades when chop high. Use multi-timeframe confirmation.",
					regime, chopScore)
			}
		}
		return "Add regime filters: reduce position size or skip trades when chop score high. Use multi-timeframe confirmation: ensure 4h trend aligns with 1h entry."

	case ReasonLateExitGiveBack:
		if percentGiven, ok := evidence["percent_of_profit_given_back"].(string); ok {
			return fmt.Sprintf("Gave back %s of peak profit. Use trailing stops (2%% trail) and take profits at 60%% of MFE to avoid late exits.",
				percentGiven)
		}
		return "Use trailing stops (e.g., 2% trail). Set profit targets at 60% of MFE. Exit when momentum indicators diverge. Consider partial exits to lock in profits."

	case ReasonSlippageExceeded:
		if slipStr, ok := evidence["entry_slippage"].(string); ok {
			if spread, ok := evidence["spread_at_entry"].(float64); ok {
				return fmt.Sprintf("Slippage was excessive (%s) with spread %.4f. Use limit orders, size down when spreads widen, or schedule entries during peak liquidity.",
					slipStr, spread)
			}
		}
		return "Use limit orders with reasonable spread tolerance. Size down when spreads widen. Schedule entries for peak liquidity hours."

	case ReasonFundingDrag:
		if totalCost, ok := evidence["total_cost"].(float64); ok {
			if pnl, ok := evidence["realized_pnl"].(float64); ok {
				costPct := (totalCost / math.Abs(pnl)) * 100
				return fmt.Sprintf("Funding cost $%.2f was %.0f%% of loss. Check rates before entry, avoid positive funding longs, use spot hedges, exit if cost >20%% position.",
					totalCost, costPct)
			}
		}
		return "Check funding rates before entry. Avoid long positions in positive funding. Use spot-hedged positions. Monitor cost: exit if >20% of position value."

	case ReasonBorrowingCostHigh:
		return "Check borrow availability and rates. Only short when rates <0.01% daily. Prefer spot lending. Size accordingly to manage carry costs."

	case ReasonRegimeMismatch:
		if regime, ok := evidence["market_regime"].(string); ok {
			return fmt.Sprintf("Strategy doesn't work in '%s' regime. Avoid trading in choppy conditions. Wait for favorable trending environment.",
				regime)
		}
		return "Restrict trading to compatible market regimes. Strategy works in trending → skip sideways/choppy periods. Wait for market to return to favorable regime."

	default:
		return "Analyze trade context and market conditions. Verify signal quality and execution. Review recent win rate and adjust position sizing if needed."
	}
}

// generateDetailedNotesV2 generates plain language explanation
func generateDetailedNotes(reason TradeFailureReason, order *RecentOrder, evidence map[string]interface{}) string {
	switch reason {
	case ReasonChasingEntry:
		slippageRatio := evidence["slippage_ratio"].(float64)
		return fmt.Sprintf("Entry was too aggressive. Slippage was %.1f%% but budget was only %.1f%% (ratio: %.2f). Fill was delayed by %dms from signal, indicating late pursuit of moved price.",
			order.EntrySlippage*100, order.EntrySlippageBudget*100, slippageRatio, order.EntryFillTime-order.SignalTime)

	case ReasonFalseBreakoutV2:
		volStr := evidence["volume_strength"].(string)
		oiStr := evidence["oi_strength"].(string)
		return fmt.Sprintf("Breakout lacked confirmation. Volume was only %s of baseline (need 90%%) and OI was only %s of baseline (need 30%%). Classic false breakout pattern with weak participation.", volStr, oiStr)

	case ReasonPrematureEntry:
		v := order.VolumeAtEntry
		if v > 10 {
			v = v / 100.0
		}
		oi := order.OIDeltaAtEntry
		if oi > 10 {
			oi = oi / 100.0
		}
		return fmt.Sprintf("Entry was too early, before setup confirmation. Volume was only %.0f%% of baseline (need 90%%) and OI delta was only %.0f%% of baseline (need 50%%). Entered before others confirmed.", v*100, oi*100)

	case ReasonStopTooTight:
		return fmt.Sprintf("Stop loss was set at %.2fx ATR, below the 1.5x threshold. ATR was %.2f, but stop was only %.2f away. This is too tight and vulnerable to normal volatility noise.", order.StopDistanceVsATR, order.ATRAtEntry, order.StopDistance)

	case ReasonMomentumDecay:
		return fmt.Sprintf("Trade lost momentum during execution. Volume declined %.1f%% and OI fell %.1f%%, indicating lack of follow-through and weakening conviction from other traders.", order.VolumeDeltaDuringTrade*100, order.OIDeltaDuringTrade*100)

	case ReasonLiquidityDried:
		return fmt.Sprintf("Liquidity evaporated during the trade. Bid-ask spread widened %.1fx (from %.3f to %.3f) and available depth fell %.1f%% (from $%.0f to $%.0f). Execution became difficult.", order.ExitSpread/order.EntrySpread, order.EntrySpread, order.ExitSpread, (1-order.ExitDepth/order.EntryDepth)*100, order.EntryDepth, order.ExitDepth)

	case ReasonStopHitRegimeChange:
		return fmt.Sprintf("Unfavorable regime during trade. Trend strength was %.2f and market regime '%s' with chop score %.2f. Stop likely hit due to regime risk, not just tight positioning.", order.TrendStrength, order.MarketRegime, order.ChopScore)

	case ReasonLateExitGiveBack:
		mfeStr := evidence["max_favorable_excursion"].(string)
		givebackStr := evidence["giveback"].(string)
		percentStr := evidence["percent_of_profit_given_back"].(string)
		return fmt.Sprintf("Poor exit timing cost significant profit. Trade reached %s gain at its peak but gave back %s (%s of the peak profit). Should have exited earlier with trailing stop or partial takes.", mfeStr, givebackStr, percentStr)

	case ReasonSlippageExceeded:
		v := order.VolumeAtEntry
		if v > 10 {
			v = v / 100.0
		}
		return fmt.Sprintf("Execution slippage was excessive (%.2f%%) for market conditions. Volatility was '%s', volume was %.0f%% of baseline, and spread was %.3f. Either order was too large or timed poorly.", order.EntrySlippage*100, order.VolatilityRegime, v*100, order.EntrySpread)

	case ReasonFundingDrag:
		totalCost := evidence["total_cost"].(float64)
		pnl := evidence["realized_pnl"].(float64)
		return fmt.Sprintf("Carry costs eroded the profit. Funding accrued $%.2f and borrow cost $%.2f (total $%.2f). This represented %.1f%% of the realized loss. Trade was hurt by duration and funding environment.", order.FundingAccrued, order.BorrowCostAccrued, totalCost, (totalCost/math.Abs(pnl))*100)

	case ReasonRegimeMismatch:
		return fmt.Sprintf("Strategy doesn't work in '%s' market regime. Current chop score is %.2f. This strategy prefers trending conditions. Avoid trading in choppy/sideways markets.", order.MarketRegime, order.ChopScore)

	default:
		return "Trade failed due to combination of execution, timing, or market condition factors. Review signal quality, entry/exit mechanics, and position sizing."
	}
}
