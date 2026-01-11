package decision

import (
	"fmt"
	"math"
	"time"
)

// ============================================================================
// Production-Grade Trade Failure Analysis V2
// ============================================================================
// Deterministic, evidence-based failure categorization using market microstructure,
// volatility, regime, and execution data.
// ============================================================================

// TradeFailureReasonV2 production-grade failure reasons with evidence
type TradeFailureReasonV2 string

const (
	// Pre-Trade / Signal Quality
	ReasonSignalQualityLow  TradeFailureReasonV2 = "signal_quality_low"  // Weak edge, low expectancy
	ReasonRegimeMismatch    TradeFailureReasonV2 = "regime_mismatch"     // Strategy ↔ regime mismatch
	ReasonLiquidityRiskHigh TradeFailureReasonV2 = "liquidity_risk_high" // Spread/depth/slippage too high
	ReasonStackedRisk       TradeFailureReasonV2 = "stacked_risk"        // Overexposed to same factor

	// Entry Execution
	ReasonChasingEntry     TradeFailureReasonV2 = "chasing_entry"     // Late entry, adverse slippage
	ReasonFalseBreakoutV2  TradeFailureReasonV2 = "false_breakout_v2" // No follow-through, no confirm
	ReasonPrematureEntry   TradeFailureReasonV2 = "premature_entry"   // Before confirmation criteria
	ReasonSizingError      TradeFailureReasonV2 = "sizing_error"      // Too big for liquidity
	ReasonSlippageExceeded TradeFailureReasonV2 = "slippage_exceeded" // Impact > budget

	// During Trade
	ReasonStopTooTight        TradeFailureReasonV2 = "stop_too_tight"         // Stop < 1.5x ATR
	ReasonMomentumDecay       TradeFailureReasonV2 = "momentum_decay"         // Volume/OI collapse
	ReasonLiquidityDried      TradeFailureReasonV2 = "liquidity_dried"        // Spread widened, depth fell
	ReasonStopHitRegimeChange TradeFailureReasonV2 = "stop_hit_regime_change" // Trend reversed, market shifted

	// Exit / Exit Timing
	ReasonLateExitGiveBack     TradeFailureReasonV2 = "late_exit_giveback"     // Large give-back from peak
	ReasonTrendReversalIgnored TradeFailureReasonV2 = "trend_reversal_ignored" // Missed reversal signal
	ReasonHighSlippageExit     TradeFailureReasonV2 = "high_slippage_exit"     // Poor exit execution

	// Process / Control
	ReasonFundingDrag       TradeFailureReasonV2 = "funding_drag"        // Funding cost ate profit
	ReasonBorrowingCostHigh TradeFailureReasonV2 = "borrowing_cost_high" // Borrow cost significant
	ReasonTechnicalFault    TradeFailureReasonV2 = "technical_fault"     // Execution error, system issue
)

// RecentOrderV2 extended order struct with comprehensive microstructure, volatility, regime, and flow data
type RecentOrderV2 struct {
	// Basic Execution
	Symbol       string        `json:"symbol"`
	Side         string        `json:"side"`
	EntryPrice   float64       `json:"entry_price"`
	ExitPrice    float64       `json:"exit_price"`
	RealizedPnL  float64       `json:"realized_pnl"`
	PnLPct       float64       `json:"pnl_pct"`
	EntryTime    time.Time     `json:"entry_time"`
	ExitTime     time.Time     `json:"exit_time"`
	HoldDuration time.Duration `json:"hold_duration"`

	// Microstructure (Entry)
	EntrySpread         float64 `json:"entry_spread"`          // Bid-ask spread %
	EntryDepth          float64 `json:"entry_depth"`           // USD liquidity at price
	EntrySlippage       float64 `json:"entry_slippage"`        // Arrival price → fill price %
	EntrySlippageBudget float64 `json:"entry_slippage_budget"` // Expected tolerance
	EntryArrivalPrice   float64 `json:"entry_arrival_price"`   // Signal-time price
	EntryFillPrice      float64 `json:"entry_fill_price"`      // Actual execution price
	EntryFillTime       int64   `json:"entry_fill_time"`       // Fill time (ms)
	SignalTime          int64   `json:"signal_time"`           // Signal generation (ms)

	// Microstructure (Exit)
	ExitSpread   float64 `json:"exit_spread"`   // Spread at exit
	ExitDepth    float64 `json:"exit_depth"`    // Depth at exit
	ExitSlippage float64 `json:"exit_slippage"` // Exit fill slippage %

	// Volatility & Risk
	ATRAtEntry         float64 `json:"atr_at_entry"`         // ATR (14)
	RealizedVolatility float64 `json:"realized_volatility"`  // σ during hold
	StopDistance       float64 `json:"stop_distance"`        // Stop distance %
	StopDistanceVsATR  float64 `json:"stop_distance_vs_atr"` // Stop / ATR
	RiskPerTrade       float64 `json:"risk_per_trade"`       // USD
	RiskBudget         float64 `json:"risk_budget"`          // Account risk allocation

	// Regime (Entry & Exit for comparison)
	TrendStrengthAtEntry float64 `json:"trend_strength_at_entry"` // -1 (strong down) to +1 (strong up)
	TrendStrengthAtExit  float64 `json:"trend_strength_at_exit"`
	ChopScoreAtEntry     float64 `json:"chop_score_at_entry"` // 0 (trending) to 1 (choppy)
	ChopScoreAtExit      float64 `json:"chop_score_at_exit"`
	VolatilityRegime     string  `json:"volatility_regime"`      // "low", "normal", "high"
	MarketRegimeAtEntry  string  `json:"market_regime_at_entry"` // "trending", "sideways", "volatile"
	MarketRegimeAtExit   string  `json:"market_regime_at_exit"`

	// Flow & Participation
	VolumeAtEntry           float64 `json:"volume_at_entry"`            // % of 24h baseline
	VolumeBaselinePercent   float64 `json:"volume_baseline_percent"`    // 100 = baseline
	OIDeltaAtEntry          float64 `json:"oi_delta_at_entry"`          // % change from 1h ago
	OIBaselineChangePercent float64 `json:"oi_baseline_change_percent"` // 100 = baseline
	VolumeDeltaDuringTrade  float64 `json:"volume_delta_during_trade"`  // % change during hold
	OIDeltaDuringTrade      float64 `json:"oi_delta_during_trade"`      // % change during hold

	// Excursion Metrics
	MaxFavorableExcursion float64 `json:"max_favorable_excursion"` // Best % profit
	MaxAdverseExcursion   float64 `json:"max_adverse_excursion"`   // Worst % loss
	GiveBackFromPeak      float64 `json:"giveback_from_peak"`      // % retracement from MFE

	// Carry & Funding
	FundingAccrued    float64 `json:"funding_accrued"`     // Cumulative funding cost
	BorrowCostAccrued float64 `json:"borrow_cost_accrued"` // Cumulative borrow cost

	// Exit Details
	ExitReason string `json:"exit_reason"` // "stop", "profit_target", "signal", "manual"
}

// FailedTradeAnalysisV2 comprehensive result with evidence tracking
type FailedTradeAnalysisV2 struct {
	PrimaryReason    TradeFailureReasonV2   `json:"primary_reason"`
	ConfidenceScore  float64                `json:"confidence_score"`  // 0-1
	SecondaryReasons []TradeFailureReasonV2 `json:"secondary_reasons"` // Related issues
	Evidence         map[string]interface{} `json:"evidence"`          // Supporting metrics
	DetailedNotes    string                 `json:"detailed_notes"`    // Plain language
	Recommendation   string                 `json:"recommendation"`    // Actionable fix
}

// ============================================================================
// Main Analysis Function
// ============================================================================

// AnalyzeFailedTradeV2 deterministically categorizes failed trades with evidence
func AnalyzeFailedTradeV2(order *RecentOrderV2) *FailedTradeAnalysisV2 {
	if order == nil {
		return nil
	}

	analysis := &FailedTradeAnalysisV2{
		Evidence: make(map[string]interface{}),
	}

	// Evaluate rules in priority order, pick FIRST matching rule with highest specificity
	// Earlier rules are more specific and take precedence
	ruleSequence := []struct {
		check      func(*RecentOrderV2) bool
		reason     TradeFailureReasonV2
		confidence float64
	}{
		// High confidence, highly specific rules (check first)
		{isChasing, ReasonChasingEntry, 0.90},
		{isLateExitGiveBack, ReasonLateExitGiveBack, 0.88},
		{isStopTooTight, ReasonStopTooTight, 0.88},
		{isMomentumDecay, ReasonMomentumDecay, 0.85},
		{isFalseBreakoutV2, ReasonFalseBreakoutV2, 0.85},
		{isLiquidityDried, ReasonLiquidityDried, 0.83},
		{isPrematureEntry, ReasonPrematureEntry, 0.82},
		{isStopHitRegimeChange, ReasonStopHitRegimeChange, 0.80},
		{isHighSlippageRegime, ReasonSlippageExceeded, 0.78},
		{isFundingDrag, ReasonFundingDrag, 0.79},
	}

	// Find FIRST matching rule (highest priority match wins)
	var bestMatch *struct {
		check      func(*RecentOrderV2) bool
		reason     TradeFailureReasonV2
		confidence float64
	}

	for i := range ruleSequence {
		rule := &ruleSequence[i]
		if rule.check(order) {
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
	analysis.Recommendation = generateRecommendationV2(analysis.PrimaryReason, order, analysis.Evidence)

	// Generate detailed notes
	analysis.DetailedNotes = generateDetailedNotesV2(analysis.PrimaryReason, order, analysis.Evidence)

	return analysis
}

// ============================================================================
// Deterministic Rule Functions
// ============================================================================

// isChasing detects late entry with adverse slippage
func isChasing(order *RecentOrderV2) bool {
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
func isFalseBreakoutV2(order *RecentOrderV2) bool {
	if order == nil {
		return false
	}
	// Normalize volume/OI to baseline to handle inputs expressed as raw percent or ratio.
	volBase := order.VolumeBaselinePercent
	if volBase == 0 {
		volBase = 1
	}
	if volBase > 10 {
		volBase = volBase / 100.0 // convert 100-based percent to ratio
	}
	volumeRatio := order.VolumeAtEntry / volBase

	oiBase := order.OIBaselineChangePercent
	if oiBase == 0 {
		oiBase = 1
	}
	if oiBase > 10 {
		oiBase = oiBase / 100.0
	}
	oiRatio := order.OIDeltaAtEntry / oiBase

	// Rule: Both volume AND OI must be weak
	weakVolume := volumeRatio < 0.90 // < 90%
	weakOI := oiRatio < 0.30         // < 30% increase
	return weakVolume && weakOI
}

// isPrematureEntry detects entries before confirmation criteria
func isPrematureEntry(order *RecentOrderV2) bool {
	if order == nil {
		return false
	}
	volBase := order.VolumeBaselinePercent
	if volBase == 0 {
		volBase = 1
	}
	if volBase > 10 {
		volBase = volBase / 100.0
	}
	volumeRatio := order.VolumeAtEntry / volBase

	oiBase := order.OIBaselineChangePercent
	if oiBase == 0 {
		oiBase = 1
	}
	if oiBase > 10 {
		oiBase = oiBase / 100.0
	}
	oiRatio := order.OIDeltaAtEntry / oiBase

	// Rule: Both volume and OI below thresholds
	lowVolume := volumeRatio < 0.90 // < 90%
	lowOI := oiRatio < 0.50         // < 50%
	return lowVolume && lowOI
}

// isStopTooTight detects stops closer than risk management threshold
func isStopTooTight(order *RecentOrderV2) bool {
	if order == nil {
		return false
	}
	// Rule: Stop distance < 1.5x ATR (industry standard)
	return order.StopDistanceVsATR < 1.5
}

// isMomentumDecay detects volume/OI collapse during trade
func isMomentumDecay(order *RecentOrderV2) bool {
	if order == nil {
		return false
	}
	// Rule: Both volume AND OI decline significantly
	volumeDropped := order.VolumeDeltaDuringTrade < -0.30 // > 30% drop
	oiDropped := order.OIDeltaDuringTrade < -0.20         // > 20% drop
	return volumeDropped && oiDropped
}

// isLiquidityDried detects spread widening and depth reduction
func isLiquidityDried(order *RecentOrderV2) bool {
	if order == nil {
		return false
	}
	// Rule: Both spread widened AND depth shrunk significantly
	if order.EntrySpread > 0 && order.ExitSpread > 0 {
		spreadWorsened := order.ExitSpread > (order.EntrySpread * 2.0) // 2x worse
		depthShrunk := order.ExitDepth < (order.EntryDepth * 0.50)     // 50% reduction
		return spreadWorsened && depthShrunk
	}
	return false
}

// isStopHitRegimeChange detects trend reversal or market regime shift
func isStopHitRegimeChange(order *RecentOrderV2) bool {
	if order == nil {
		return false
	}
	// Rule 1: Trend flip (strong → opposite direction)
	if math.Abs(order.TrendStrengthAtEntry) > 0.60 {
		trendFlipped := (order.TrendStrengthAtEntry > 0 && order.TrendStrengthAtExit < -0.60) ||
			(order.TrendStrengthAtEntry < 0 && order.TrendStrengthAtExit > 0.60)
		if trendFlipped {
			return true
		}
	}
	// Rule 2: Market regime shift (trending → choppy)
	if order.MarketRegimeAtEntry == "trending" && order.MarketRegimeAtExit != "trending" {
		chopIncreased := order.ChopScoreAtExit > order.ChopScoreAtEntry+0.20
		return chopIncreased
	}
	return false
}

// isLateExitGiveBack detects poor exit timing with large give-back
func isLateExitGiveBack(order *RecentOrderV2) bool {
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
func isHighSlippageRegime(order *RecentOrderV2) bool {
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
func isFundingDrag(order *RecentOrderV2) bool {
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
func populateEvidence(analysis *FailedTradeAnalysisV2, order *RecentOrderV2) {
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
		analysis.Evidence["volume_baseline"] = order.VolumeBaselinePercent
		analysis.Evidence["oi_delta_at_entry"] = order.OIDeltaAtEntry
		analysis.Evidence["oi_baseline"] = order.OIBaselineChangePercent
		analysis.Evidence["volume_strength"] = fmt.Sprintf("%.0f%%", (order.VolumeAtEntry/order.VolumeBaselinePercent)*100)
		analysis.Evidence["oi_strength"] = fmt.Sprintf("%.0f%%", (order.OIDeltaAtEntry/order.OIBaselineChangePercent)*100)

	case ReasonPrematureEntry:
		analysis.Evidence["volume_check"] = fmt.Sprintf("%.0f%% (need 90%%)", (order.VolumeAtEntry/order.VolumeBaselinePercent)*100)
		analysis.Evidence["oi_check"] = fmt.Sprintf("%.0f%% (need 50%%)", (order.OIDeltaAtEntry/order.OIBaselineChangePercent)*100)

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
		analysis.Evidence["trend_at_entry"] = order.TrendStrengthAtEntry
		analysis.Evidence["trend_at_exit"] = order.TrendStrengthAtExit
		analysis.Evidence["chop_at_entry"] = order.ChopScoreAtEntry
		analysis.Evidence["chop_at_exit"] = order.ChopScoreAtExit

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
func generateRecommendationV2(reason TradeFailureReasonV2, order *RecentOrderV2, evidence map[string]interface{}) string {
	switch reason {
	case ReasonChasingEntry:
		return "Use limit orders with tighter parameters. Wait for confirmed signal before entering. Reduce position size when spreads/depth deteriorate."

	case ReasonFalseBreakoutV2:
		return "Require BOTH volume >110% baseline AND OI >50% increase. Use breakout filters: check 5m/15m candles for true breakout patterns. Add pullback entry after confirmation."

	case ReasonPrematureEntry:
		return "Enforce confirmation criteria: wait for volume >90% AND OI increase >50%. Add time filter: only enter after X candles of confirmation. Use order book depth as confirmation."

	case ReasonStopTooTight:
		return fmt.Sprintf("Increase stop to at least %.2fx ATR (currently %.2fx). For this trade, stop should be ≥%.2f away from entry.", 1.5, order.StopDistanceVsATR, order.ATRAtEntry*1.5)

	case ReasonMomentumDecay:
		return "Monitor volume/OI during trade. Exit with trailing stop when volume falls >20% and OI declines >15%. Consider tighter profit targets in momentum-dependent strategies."

	case ReasonLiquidityDried:
		return "Check order book depth before entry. Avoid positions when spreads >0.15% or depth <$1M. Use limit orders. Consider smaller position size in illiquid markets."

	case ReasonStopHitRegimeChange:
		return "Add regime filters: reduce position size or skip trades when chop score high. Use multi-timeframe confirmation: ensure 4h trend aligns with 1h entry. Tighten stops pre-economic events."

	case ReasonLateExitGiveBack:
		return "Use trailing stops (e.g., 2% trail). Set profit targets at 60% of MFE. Exit when momentum indicators diverge. Consider partial exits to lock in profits early."

	case ReasonSlippageExceeded:
		return "Use limit orders with reasonable spread tolerance. Size down when spreads widen. Schedule entries for peak liquidity hours. Use VWAP-based execution for larger orders."

	case ReasonFundingDrag:
		return "Check funding rates before entry. Avoid long positions in positive funding environments. Use spot-hedged positions. Monitor accumulated cost: exit if >20% of position value."

	case ReasonBorrowingCostHigh:
		return "Check borrow availability and rates. Only short when rates <0.01% daily. Prefer spot lending to direct borrowing. Size accordingly to manage carry costs."

	case ReasonRegimeMismatch:
		return "Restrict trading to compatible market regimes. Strategy works in trending → skip sideways/choppy periods. Wait for market to return to favorable regime before re-entering."

	default:
		return "Analyze trade context and market conditions. Verify signal quality and execution. Review recent win rate and adjust position sizing if needed."
	}
}

// generateDetailedNotesV2 generates plain language explanation
func generateDetailedNotesV2(reason TradeFailureReasonV2, order *RecentOrderV2, evidence map[string]interface{}) string {
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
		return fmt.Sprintf("Entry was too early, before setup confirmation. Volume was only %.0f%% of baseline (need 90%%) and OI delta was only %.0f%% of baseline (need 50%%). Entered before others confirmed.", order.VolumeAtEntry/order.VolumeBaselinePercent*100, order.OIDeltaAtEntry/order.OIBaselineChangePercent*100)

	case ReasonStopTooTight:
		return fmt.Sprintf("Stop loss was set at %.2fx ATR, below the 1.5x threshold. ATR was %.2f, but stop was only %.2f away. This is too tight and vulnerable to normal volatility noise.", order.StopDistanceVsATR, order.ATRAtEntry, order.StopDistance)

	case ReasonMomentumDecay:
		return fmt.Sprintf("Trade lost momentum during execution. Volume declined %.1f%% and OI fell %.1f%%, indicating lack of follow-through and weakening conviction from other traders.", order.VolumeDeltaDuringTrade*100, order.OIDeltaDuringTrade*100)

	case ReasonLiquidityDried:
		return fmt.Sprintf("Liquidity evaporated during the trade. Bid-ask spread widened %.1fx (from %.3f to %.3f) and available depth fell %.1f%% (from $%.0f to $%.0f). Execution became difficult.", order.ExitSpread/order.EntrySpread, order.EntrySpread, order.ExitSpread, (1-order.ExitDepth/order.EntryDepth)*100, order.EntryDepth, order.ExitDepth)

	case ReasonStopHitRegimeChange:
		return fmt.Sprintf("Market regime shifted during trade. Trend strength went from %.2f to %.2f, and market moved from '%s' to '%s'. Stop was hit during this adverse regime shift, not due to tight positioning.", order.TrendStrengthAtEntry, order.TrendStrengthAtExit, order.MarketRegimeAtEntry, order.MarketRegimeAtExit)

	case ReasonLateExitGiveBack:
		mfeStr := evidence["max_favorable_excursion"].(string)
		givebackStr := evidence["giveback"].(string)
		percentStr := evidence["percent_of_profit_given_back"].(string)
		return fmt.Sprintf("Poor exit timing cost significant profit. Trade reached %s gain at its peak but gave back %s (%s of the peak profit). Should have exited earlier with trailing stop or partial takes.", mfeStr, givebackStr, percentStr)

	case ReasonSlippageExceeded:
		return fmt.Sprintf("Execution slippage was excessive (%.2f%%) for market conditions. Volatility was '%s', volume was %.0f%% of baseline, and spread was %.3f. Either order was too large or timed poorly.", order.EntrySlippage*100, order.VolatilityRegime, order.VolumeAtEntry/order.VolumeBaselinePercent*100, order.EntrySpread)

	case ReasonFundingDrag:
		totalCost := evidence["total_cost"].(float64)
		pnl := evidence["realized_pnl"].(float64)
		return fmt.Sprintf("Carry costs eroded the profit. Funding accrued $%.2f and borrow cost $%.2f (total $%.2f). This represented %.1f%% of the realized loss. Trade was hurt by duration and funding environment.", order.FundingAccrued, order.BorrowCostAccrued, totalCost, (totalCost/math.Abs(pnl))*100)

	case ReasonRegimeMismatch:
		return fmt.Sprintf("Strategy doesn't work in '%s' market regime. Entry was made when market was '%s' (chop: %.2f). This strategy prefers trending conditions. Avoid trading in choppy/sideways markets.", order.MarketRegimeAtEntry, order.MarketRegimeAtExit, order.ChopScoreAtExit)

	default:
		return "Trade failed due to combination of execution, timing, or market condition factors. Review signal quality, entry/exit mechanics, and position sizing."
	}
}
