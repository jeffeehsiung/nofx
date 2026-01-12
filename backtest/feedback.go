package backtest

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/decision"
	"nofx/logger"
	"nofx/market"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ============================================================================
// Feedback Loop System - Unsupervised Learning from Historical Performance
// ============================================================================
// This module implements a feedback loop that analyzes past trading decisions
// and their outcomes to help the LLM learn from mistakes and improve profitability.
// ============================================================================

// FeedbackAnalysis contains comprehensive analysis of historical trading performance
type FeedbackAnalysis struct {
	// Period analyzed
	AnalysisPeriod   string    `json:"analysis_period"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	DecisionsCovered int       `json:"decisions_covered"`

	// Overall performance
	TotalReturn    float64 `json:"total_return"`
	TotalReturnPct float64 `json:"total_return_pct"`
	WinRate        float64 `json:"win_rate"`
	ProfitFactor   float64 `json:"profit_factor"`
	SharpeRatio    float64 `json:"sharpe_ratio"`
	MaxDrawdown    float64 `json:"max_drawdown"`

	// Pattern analysis
	SuccessPatterns []TradingPattern `json:"success_patterns"`
	FailurePatterns []TradingPattern `json:"failure_patterns"`

	// Actionable insights
	KeyInsights        []string `json:"key_insights"`
	RecommendedActions []string `json:"recommended_actions"`

	// Detailed decision analysis
	TopWinningTrades []DecisionOutcome `json:"top_winning_trades"`
	TopLosingTrades  []DecisionOutcome `json:"top_losing_trades"`

	// Market regime analysis
	MarketConditions string             `json:"market_conditions"`
	RegimeAnalysis   map[string]float64 `json:"regime_analysis"`
}

// TradingPattern represents a discovered pattern in trading behavior
type TradingPattern struct {
	PatternType    string   `json:"pattern_type"`   // e.g., "high_leverage_losses", "quick_exits_profitable"
	Frequency      int      `json:"frequency"`      // how often this pattern occurred
	AvgPnL         float64  `json:"avg_pnl"`        // average P&L for this pattern
	AvgPnLPct      float64  `json:"avg_pnl_pct"`    // average P&L percentage
	Description    string   `json:"description"`    // human-readable description
	Evidence       []string `json:"evidence"`       // specific examples
	Recommendation string   `json:"recommendation"` // what to do about it
}

// DecisionOutcome links a decision to its actual outcome
type DecisionOutcome struct {
	Timestamp  time.Time `json:"timestamp"`
	Symbol     string    `json:"symbol"`
	Action     string    `json:"action"`
	Reasoning  string    `json:"reasoning"`
	Confidence int       `json:"confidence"`

	// Entry details
	EntryPrice   float64 `json:"entry_price"`
	PositionSize float64 `json:"position_size"`
	Leverage     int     `json:"leverage"`

	// Exit details
	ExitPrice    float64 `json:"exit_price"`
	HoldDuration string  `json:"hold_duration"`

	// Outcome
	RealizedPnL    float64 `json:"realized_pnl"`
	RealizedPnLPct float64 `json:"realized_pnl_pct"`
	Success        bool    `json:"success"`

	// What went right/wrong
	Analysis string `json:"analysis"`

	// NEW: Detailed microstructure data for Trade Failure V2 analysis
	RecentOrder *decision.RecentOrder `json:"recent_order,omitempty"`
}

// FeedbackConfig controls feedback loop behavior
type FeedbackConfig struct {
	EnableFeedback          bool `json:"enable_feedback"`
	MinDecisionsForFeedback int  `json:"min_decisions_for_feedback"` // minimum decisions before generating feedback
	FeedbackWindowCycles    int  `json:"feedback_window_cycles"`     // how many recent cycles to analyze
	TopTradesCount          int  `json:"top_trades_count"`           // number of top winning/losing trades to show
	MinPatternFrequency     int  `json:"min_pattern_frequency"`      // minimum occurrences to consider a pattern
}

// DefaultFeedbackConfig returns sensible defaults
func DefaultFeedbackConfig() FeedbackConfig {
	return FeedbackConfig{
		EnableFeedback:          true,
		MinDecisionsForFeedback: 10,
		FeedbackWindowCycles:    20,
		TopTradesCount:          3,
		MinPatternFrequency:     2,
	}
}

// FeedbackGenerator generates feedback from historical performance
type FeedbackGenerator struct {
	runID  string
	config FeedbackConfig

	// Calibrated thresholds for Trade Failure V2 (defaults if calibration unavailable)
	failureThresholds decision.FailureThresholds
}

// NewFeedbackGenerator creates a new feedback generator
func NewFeedbackGenerator(runID string, config FeedbackConfig) *FeedbackGenerator {
	return &FeedbackGenerator{
		runID:             runID,
		config:            config,
		failureThresholds: decision.DefaultFailureThresholds(),
	}
}

// SetFailureThresholds injects calibrated thresholds (idempotent fallback-safe)
func (fg *FeedbackGenerator) SetFailureThresholds(thresholds decision.FailureThresholds) {
	fg.failureThresholds = thresholds
}

// GenerateFeedback analyzes recent performance and generates actionable insights
func (fg *FeedbackGenerator) GenerateFeedback() (*FeedbackAnalysis, error) {
	// Load trade events and decision records
	events, err := LoadTradeEvents(fg.runID)
	if err != nil {
		return nil, fmt.Errorf("failed to load trade events: %w", err)
	}

	if len(events) < fg.config.MinDecisionsForFeedback {
		logger.Infof("Not enough trades (%d) for feedback analysis (need %d)", len(events), fg.config.MinDecisionsForFeedback)
		return nil, nil
	}

	// Get backtest metrics
	ckpt, err := LoadCheckpoint(fg.runID)
	if err != nil {
		return nil, fmt.Errorf("failed to load checkpoint: %w", err)
	}
	cfg := &BacktestConfig{
		InitialBalance: ckpt.Equity + ckpt.RealizedPnL - ckpt.UnrealizedPnL,
	}

	state, err := fg.getCurrentState()
	if err != nil {
		logger.Infof("Warning: could not load current state: %v", err)
		state = nil
	}

	metrics, err := CalculateMetrics(fg.runID, cfg, state)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate metrics: %w", err)
	}

	// Create analysis
	analysis := &FeedbackAnalysis{
		AnalysisPeriod:   fmt.Sprintf("Last %d trades", len(events)),
		DecisionsCovered: len(events),
		TotalReturnPct:   metrics.TotalReturnPct,
		WinRate:          metrics.WinRate,
		ProfitFactor:     metrics.ProfitFactor,
		SharpeRatio:      metrics.SharpeRatio,
		MaxDrawdown:      metrics.MaxDrawdownPct,
		RegimeAnalysis:   make(map[string]float64),
	}

	if len(events) > 0 {
		analysis.StartTime = time.UnixMilli(events[0].Timestamp)
		analysis.EndTime = time.UnixMilli(events[len(events)-1].Timestamp)
	}

	// Analyze closed positions (match open and close events)
	closedPositions := fg.extractClosedPositions(events)

	// Generate decision outcomes
	outcomes := fg.createDecisionOutcomes(closedPositions)

	// Identify patterns
	analysis.SuccessPatterns = fg.identifySuccessPatterns(outcomes, metrics)
	analysis.FailurePatterns = fg.identifyFailurePatterns(outcomes, metrics)

	// Extract top trades
	analysis.TopWinningTrades = fg.getTopTrades(outcomes, true, fg.config.TopTradesCount)
	analysis.TopLosingTrades = fg.getTopTrades(outcomes, false, fg.config.TopTradesCount)

	// Generate insights
	analysis.KeyInsights = fg.generateKeyInsights(metrics, outcomes, analysis)
	analysis.RecommendedActions = fg.generateRecommendedActions(analysis)

	// Market regime analysis
	analysis.MarketConditions = fg.analyzeMarketConditions(metrics, outcomes)

	return analysis, nil
}

func (fg *FeedbackGenerator) getCurrentState() (*BacktestState, error) {
	ckpt, err := LoadCheckpoint(fg.runID)
	if err != nil {
		return nil, err
	}

	state := &BacktestState{
		Cash:           ckpt.Cash,
		Equity:         ckpt.Equity,
		UnrealizedPnL:  ckpt.UnrealizedPnL,
		RealizedPnL:    ckpt.RealizedPnL,
		MaxEquity:      ckpt.MaxEquity,
		MinEquity:      ckpt.MinEquity,
		MaxDrawdownPct: ckpt.MaxDrawdownPct,
		Liquidated:     ckpt.Liquidated,
	}

	return state, nil
}

// ClosedPosition represents a completed trade with entry and exit details
type ClosedPosition struct {
	Symbol      string
	Side        string
	EntryTime   time.Time
	ExitTime    time.Time
	EntryPrice  float64
	ExitPrice   float64
	Quantity    float64
	Leverage    int
	RealizedPnL float64
	EntryEvent  TradeEvent
	ExitEvent   TradeEvent

	// Market data snapshots for microstructure analysis
	EntryMarketData *market.Data
	ExitMarketData  *market.Data
}

// extractClosedPositions matches open and close events to create complete trade records
func (fg *FeedbackGenerator) extractClosedPositions(events []TradeEvent) []ClosedPosition {
	var closed []ClosedPosition
	openPositions := make(map[string]TradeEvent) // key: symbol:side

	for _, event := range events {
		if event.LiquidationFlag {
			continue // Skip liquidation events
		}

		key := fmt.Sprintf("%s:%s", event.Symbol, event.Side)

		if event.Action == "open" {
			openPositions[key] = event
		} else if event.Action == "close" {
			if openEvent, exists := openPositions[key]; exists {
				closed = append(closed, ClosedPosition{
					Symbol:      event.Symbol,
					Side:        event.Side,
					EntryTime:   time.UnixMilli(openEvent.Timestamp),
					ExitTime:    time.UnixMilli(event.Timestamp),
					EntryPrice:  openEvent.Price,
					ExitPrice:   event.Price,
					Quantity:    event.Quantity,
					Leverage:    openEvent.Leverage,
					RealizedPnL: event.RealizedPnL,
					EntryEvent:  openEvent,
					ExitEvent:   event,
				})
				delete(openPositions, key)
			}
		}
	}

	return closed
}

// buildRecentOrderFromPosition converts a ClosedPosition into decision.RecentOrder with microstructure data
// This is the bridge between backtest execution and Trade Failure V2 analysis
func buildRecentOrderFromPosition(pos ClosedPosition) *decision.RecentOrder {
	holdDuration := pos.ExitTime.Sub(pos.EntryTime)

	order := &decision.RecentOrder{
		Symbol:       pos.Symbol,
		Side:         pos.Side,
		EntryPrice:   pos.EntryPrice,
		ExitPrice:    pos.ExitPrice,
		RealizedPnL:  pos.RealizedPnL,
		EntryTime:    pos.EntryTime.Format(time.RFC3339),
		ExitTime:     pos.ExitTime.Format(time.RFC3339),
		HoldDuration: formatDuration(holdDuration),
		Leverage:     pos.Leverage,
	}

	// Calculate PnL percentage
	if pos.EntryPrice > 0 {
		if pos.Side == "long" {
			order.PnLPct = ((pos.ExitPrice - pos.EntryPrice) / pos.EntryPrice) * 100 * float64(pos.Leverage)
		} else {
			order.PnLPct = ((pos.EntryPrice - pos.ExitPrice) / pos.EntryPrice) * 100 * float64(pos.Leverage)
		}
	}

	// Populate microstructure data from entry event (use actual captured values)
	order.EntrySpread = pos.EntryEvent.Spread
	order.EntryDepth = pos.EntryEvent.Depth
	if pos.EntryPrice > 0 {
		order.EntrySlippage = math.Abs(pos.EntryEvent.Slippage / pos.EntryPrice)
	}
	order.EntrySlippageBudget = pos.EntryEvent.SlippageBudget
	order.SignalTime = pos.EntryEvent.SignalTime
	order.EntryFillTime = pos.EntryEvent.FillTime

	// Populate microstructure data from exit event (use actual captured values)
	order.ExitSpread = pos.ExitEvent.Spread
	order.ExitDepth = pos.ExitEvent.Depth
	if pos.ExitPrice > 0 {
		order.ExitSlippage = math.Abs(pos.ExitEvent.Slippage / pos.ExitPrice)
	}

	// Populate market data from entry snapshot
	if pos.EntryMarketData != nil {
		order.ATRAtEntry = calculateATRFromSeries(pos.EntryMarketData)
		order.TrendStrength = extractTrendStrength(pos.EntryMarketData)
		order.ChopScore = extractChopScore(pos.EntryMarketData)
		order.MarketRegime = extractMarketRegime(pos.EntryMarketData)
		order.VolatilityRegime = extractVolatilityRegime(pos.EntryMarketData)
		order.VolumeAtEntry = extractVolumeRatio(pos.EntryMarketData)
		order.OIDeltaAtEntry = extractOIDelta(pos.EntryMarketData)
	}

	// Calculate deltas during trade (entry vs exit market data)
	if pos.EntryMarketData != nil && pos.ExitMarketData != nil {
		order.VolumeDeltaDuringTrade = extractVolumeRatio(pos.ExitMarketData) - extractVolumeRatio(pos.EntryMarketData)
		order.OIDeltaDuringTrade = extractOIDelta(pos.ExitMarketData) - extractOIDelta(pos.EntryMarketData)
	}

	// Calculate stop distance vs ATR if we have ATR and entry spread
	// Use entry spread + 2*ATR as reasonable stop estimate (ATR-based risk management)
	if order.ATRAtEntry > 0 && pos.EntryPrice > 0 {
		atrPct := order.ATRAtEntry / pos.EntryPrice
		// Stop distance = spread + 2*ATR (standard risk management)
		stopDistance := order.EntrySpread + (atrPct * 2.0)
		order.StopDistance = stopDistance
		order.StopDistanceVsATR = stopDistance / atrPct
	}

	// Populate excursion metrics from TradeEvent
	order.MaxFavorableExcursion = pos.EntryEvent.MaxFavorableExcursion
	order.MaxAdverseExcursion = pos.ExitEvent.MaxAdverseExcursion

	// Calculate giveback: how much profit was left on the table after peak
	// GiveBack = MaxFavorableExcursion - RealizedPnL
	if order.MaxFavorableExcursion > 0 && pos.RealizedPnL > 0 {
		order.GiveBackFromPeak = order.MaxFavorableExcursion - pos.RealizedPnL
	}

	return order
}

// Helper functions for market data extraction

func calculateATRFromSeries(data *market.Data) float64 {
	// Try from intraday data first
	if data.IntradaySeries != nil && data.IntradaySeries.ATR14 > 0 {
		return data.IntradaySeries.ATR14
	}
	// Try from longer-term data
	if data.LongerTermContext != nil && data.LongerTermContext.ATR14 > 0 {
		return data.LongerTermContext.ATR14
	}
	// Try from timeframe data (1h or 4h)
	if data.TimeframeData != nil {
		if tfData, ok := data.TimeframeData["1h"]; ok && tfData.ATR14 > 0 {
			return tfData.ATR14
		}
		if tfData, ok := data.TimeframeData["4h"]; ok && tfData.ATR14 > 0 {
			return tfData.ATR14
		}
	}
	return 0.0
}

func extractTrendStrength(data *market.Data) float64 {
	// Calculate trend strength from EMA20 vs EMA50
	if data.LongerTermContext != nil && data.LongerTermContext.EMA20 > 0 && data.LongerTermContext.EMA50 > 0 {
		return (data.LongerTermContext.EMA20 - data.LongerTermContext.EMA50) / data.LongerTermContext.EMA50
	}
	return 0.0
}

func extractChopScore(data *market.Data) float64 {
	// Estimate choppiness from ATR vs price range
	atr := calculateATRFromSeries(data)
	if atr > 0 && data.CurrentPrice > 0 {
		// ATR as % of price - lower means less choppy
		return math.Min(atr/data.CurrentPrice*10, 1.0) // scale to 0-1
	}
	return 0.5 // neutral default
}

func extractMarketRegime(data *market.Data) string {
	// Determine regime from trend strength and chop
	trendStrength := extractTrendStrength(data)
	chopScore := extractChopScore(data)

	if chopScore > 0.6 {
		return "sideways"
	} else if math.Abs(trendStrength) > 0.3 {
		return "trending"
	}
	return "volatile"
}

func extractVolatilityRegime(data *market.Data) string {
	atr := calculateATRFromSeries(data)
	if atr == 0 {
		return "normal"
	}

	// Classify based on ATR relative to price
	atrPct := atr / data.CurrentPrice
	if atrPct < 0.02 {
		return "low"
	} else if atrPct > 0.05 {
		return "high"
	}
	return "normal"
}

func extractVolumeRatio(data *market.Data) float64 {
	// Get latest volume as ratio of baseline (assume 1.0 = baseline)
	if data.IntradaySeries != nil && len(data.IntradaySeries.Volume) > 0 {
		latestVol := data.IntradaySeries.Volume[len(data.IntradaySeries.Volume)-1]
		// Estimate baseline as average of recent volumes
		if len(data.IntradaySeries.Volume) > 10 {
			var sum float64
			start := len(data.IntradaySeries.Volume) - 10
			for i := start; i < len(data.IntradaySeries.Volume)-1; i++ {
				sum += data.IntradaySeries.Volume[i]
			}
			baseline := sum / 9.0
			if baseline > 0 {
				return latestVol / baseline
			}
		}
		return 1.0
	}
	return 1.0
}

func extractOIDelta(data *market.Data) float64 {
	// Get OI delta if available
	if data.OpenInterest != nil && data.OpenInterest.Latest > 0 && data.OpenInterest.Average > 0 {
		return (data.OpenInterest.Latest - data.OpenInterest.Average) / data.OpenInterest.Average
	}
	return 0.0
}

// createDecisionOutcomes converts closed positions into decision outcomes with analysis
func (fg *FeedbackGenerator) createDecisionOutcomes(closedPositions []ClosedPosition) []DecisionOutcome {
	outcomes := make([]DecisionOutcome, 0, len(closedPositions))

	for _, pos := range closedPositions {
		pnlPct := 0.0
		if pos.EntryPrice > 0 {
			if pos.Side == "long" {
				pnlPct = ((pos.ExitPrice - pos.EntryPrice) / pos.EntryPrice) * 100 * float64(pos.Leverage)
			} else {
				pnlPct = ((pos.EntryPrice - pos.ExitPrice) / pos.EntryPrice) * 100 * float64(pos.Leverage)
			}
		}

		holdDuration := pos.ExitTime.Sub(pos.EntryTime)

		outcome := DecisionOutcome{
			Timestamp:      pos.EntryTime,
			Symbol:         pos.Symbol,
			Action:         fmt.Sprintf("open_%s", pos.Side),
			Reasoning:      "", // Could be populated from decision logs if available
			Confidence:     0,  // Could be populated from decision logs
			EntryPrice:     pos.EntryPrice,
			PositionSize:   pos.EntryPrice * pos.Quantity,
			Leverage:       pos.Leverage,
			ExitPrice:      pos.ExitPrice,
			HoldDuration:   formatDuration(holdDuration),
			RealizedPnL:    pos.RealizedPnL,
			RealizedPnLPct: pnlPct,
			Success:        pos.RealizedPnL > 0,
			Analysis:       fg.analyzeDecisionOutcome(pos, pnlPct, holdDuration),
			RecentOrder:    buildRecentOrderFromPosition(pos), // Bridge to Trade Failure V2
		}

		outcomes = append(outcomes, outcome)
	}

	return outcomes
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func (fg *FeedbackGenerator) analyzeDecisionOutcome(pos ClosedPosition, pnlPct float64, holdDuration time.Duration) string {
	var analysis []string

	if pos.RealizedPnL > 0 {
		analysis = append(analysis, fmt.Sprintf("✓ Profitable trade: +%.2f%% (%.2f USDT)", pnlPct, pos.RealizedPnL))

		if holdDuration < 30*time.Minute {
			analysis = append(analysis, "Quick profit capture - good execution")
		} else if holdDuration > 4*time.Hour {
			analysis = append(analysis, "Patient hold paid off - good discipline")
		}

		if pos.Leverage >= 5 {
			analysis = append(analysis, "High leverage used effectively")
		}
	} else {
		analysis = append(analysis, fmt.Sprintf("✗ Loss: %.2f%% (%.2f USDT)", pnlPct, pos.RealizedPnL))

		if math.Abs(pnlPct) > 5 {
			analysis = append(analysis, "⚠️ Large loss - stop-loss may have been too wide")
		}

		if holdDuration < 15*time.Minute {
			analysis = append(analysis, "Premature exit - might have lacked conviction")
		} else if holdDuration > 6*time.Hour {
			analysis = append(analysis, "Held losing position too long - hesitant to cut losses")
		}

		if pos.Leverage >= 5 {
			analysis = append(analysis, "⚠️ High leverage amplified losses")
		}
	}

	return strings.Join(analysis, "; ")
}

// identifySuccessPatterns finds patterns in winning trades
func (fg *FeedbackGenerator) identifySuccessPatterns(outcomes []DecisionOutcome, metrics *Metrics) []TradingPattern {
	var patterns []TradingPattern

	// Pattern 1: Quick profit-taking
	quickWins := 0
	quickWinsPnL := 0.0
	for _, outcome := range outcomes {
		if outcome.Success && strings.Contains(outcome.HoldDuration, "m") && !strings.Contains(outcome.HoldDuration, "h") {
			duration := parseDurationMinutes(outcome.HoldDuration)
			if duration < 30 {
				quickWins++
				quickWinsPnL += outcome.RealizedPnLPct
			}
		}
	}

	if quickWins >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "quick_profit_taking",
			Frequency:      quickWins,
			AvgPnL:         0,
			AvgPnLPct:      quickWinsPnL / float64(quickWins),
			Description:    "Profitable trades closed within 30 minutes",
			Evidence:       []string{fmt.Sprintf("%d trades with avg %.2f%% profit", quickWins, quickWinsPnL/float64(quickWins))},
			Recommendation: "Continue quick profit-taking strategy for scalping opportunities",
		})
	}

	// Pattern 2: Optimal leverage usage
	lowLevWins := 0
	lowLevPnL := 0.0
	highLevWins := 0
	highLevPnL := 0.0

	for _, outcome := range outcomes {
		if outcome.Success {
			if outcome.Leverage <= 3 {
				lowLevWins++
				lowLevPnL += outcome.RealizedPnLPct
			} else if outcome.Leverage >= 5 {
				highLevWins++
				highLevPnL += outcome.RealizedPnLPct
			}
		}
	}

	if lowLevWins >= fg.config.MinPatternFrequency && highLevWins >= fg.config.MinPatternFrequency {
		lowAvg := lowLevPnL / float64(lowLevWins)
		highAvg := highLevPnL / float64(highLevWins)

		if lowAvg > highAvg {
			patterns = append(patterns, TradingPattern{
				PatternType:    "low_leverage_success",
				Frequency:      lowLevWins,
				AvgPnL:         0,
				AvgPnLPct:      lowAvg,
				Description:    "Lower leverage (≤3x) produces better risk-adjusted returns",
				Evidence:       []string{fmt.Sprintf("Low lev: %.2f%% avg, High lev: %.2f%% avg", lowAvg, highAvg)},
				Recommendation: "Favor lower leverage positions for more consistent profits",
			})
		} else {
			patterns = append(patterns, TradingPattern{
				PatternType:    "high_leverage_success",
				Frequency:      highLevWins,
				AvgPnL:         0,
				AvgPnLPct:      highAvg,
				Description:    "Higher leverage (≥5x) produces better returns when correct",
				Evidence:       []string{fmt.Sprintf("High lev: %.2f%% avg vs Low lev: %.2f%% avg", highAvg, lowAvg)},
				Recommendation: "High leverage is working - ensure tight stop-losses are maintained",
			})
		}
	}

	// Pattern 3: Symbol-specific success
	symbolWins := make(map[string]int)
	symbolPnL := make(map[string]float64)

	for _, outcome := range outcomes {
		if outcome.Success {
			symbolWins[outcome.Symbol]++
			symbolPnL[outcome.Symbol] += outcome.RealizedPnLPct
		}
	}

	for symbol, wins := range symbolWins {
		if wins >= fg.config.MinPatternFrequency {
			avgPnL := symbolPnL[symbol] / float64(wins)
			if avgPnL > 3.0 { // At least 3% average profit
				patterns = append(patterns, TradingPattern{
					PatternType:    "symbol_affinity",
					Frequency:      wins,
					AvgPnL:         0,
					AvgPnLPct:      avgPnL,
					Description:    fmt.Sprintf("Strong performance on %s", symbol),
					Evidence:       []string{fmt.Sprintf("%d wins, %.2f%% avg profit", wins, avgPnL)},
					Recommendation: fmt.Sprintf("Consider prioritizing %s for future trades", symbol),
				})
			}
		}
	}

	// Pattern 4: Consecutive wins (momentum trading)
	consecutiveWins := 0
	maxWinStreak := 0
	currentStreak := 0
	winStreakPnL := 0.0

	for i, outcome := range outcomes {
		if outcome.Success {
			currentStreak++
			if currentStreak > maxWinStreak {
				maxWinStreak = currentStreak
			}
			if currentStreak >= 3 { // 3+ wins in a row
				consecutiveWins++
				winStreakPnL += outcome.RealizedPnLPct
			}
		} else {
			currentStreak = 0
		}
		// Check if win followed by immediate loss (profit giveaway)
		if i > 0 && outcomes[i-1].Success && !outcome.Success {
			// This is handled in failure patterns
		}
	}

	if consecutiveWins >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "momentum_trading",
			Frequency:      consecutiveWins,
			AvgPnL:         0,
			AvgPnLPct:      winStreakPnL / float64(consecutiveWins),
			Description:    fmt.Sprintf("Win streaks detected (max %d consecutive wins)", maxWinStreak),
			Evidence:       []string{fmt.Sprintf("%d occurrences of 3+ wins in a row", consecutiveWins)},
			Recommendation: "Capitalize on momentum - when on a streak, maintain the same approach",
		})
	}

	// Pattern 5: Optimal trading hours
	hourlyWins := make(map[int]int)
	hourlyPnL := make(map[int]float64)
	hourlyCount := make(map[int]int)

	for _, outcome := range outcomes {
		hour := outcome.Timestamp.Hour()
		hourlyCount[hour]++
		if outcome.Success {
			hourlyWins[hour]++
			hourlyPnL[hour] += outcome.RealizedPnLPct
		}
	}

	bestHour := -1
	bestWinRate := 0.0
	for hour, wins := range hourlyWins {
		if hourlyCount[hour] >= fg.config.MinPatternFrequency {
			winRate := float64(wins) / float64(hourlyCount[hour]) * 100
			if winRate > bestWinRate && winRate > 60 { // >60% win rate
				bestWinRate = winRate
				bestHour = hour
			}
		}
	}

	if bestHour >= 0 {
		avgPnL := hourlyPnL[bestHour] / float64(hourlyWins[bestHour])
		patterns = append(patterns, TradingPattern{
			PatternType:    "optimal_trading_hours",
			Frequency:      hourlyWins[bestHour],
			AvgPnL:         0,
			AvgPnLPct:      avgPnL,
			Description:    fmt.Sprintf("Best performance during hour %02d:00--%02d:59 UTC", bestHour, bestHour),
			Evidence:       []string{fmt.Sprintf("%.1f%% win rate, %.2f%% avg profit", bestWinRate, avgPnL)},
			Recommendation: fmt.Sprintf("Focus trading activity around %02d:00 UTC when market conditions are most favorable", bestHour),
		})
	}

	// Pattern 6: Proper position sizing on wins
	smallPosWins := 0
	smallPosPnL := 0.0
	largePosWins := 0
	largePosPnL := 0.0

	for _, outcome := range outcomes {
		if outcome.Success {
			if outcome.PositionSize < 100 { // Small positions
				smallPosWins++
				smallPosPnL += outcome.RealizedPnLPct
			} else if outcome.PositionSize > 500 { // Large positions
				largePosWins++
				largePosPnL += outcome.RealizedPnLPct
			}
		}
	}

	if smallPosWins >= fg.config.MinPatternFrequency && largePosWins >= fg.config.MinPatternFrequency {
		smallAvg := smallPosPnL / float64(smallPosWins)
		largeAvg := largePosPnL / float64(largePosWins)

		if smallAvg > largeAvg && smallAvg > 3.0 {
			patterns = append(patterns, TradingPattern{
				PatternType:    "optimal_position_sizing",
				Frequency:      smallPosWins,
				AvgPnL:         0,
				AvgPnLPct:      smallAvg,
				Description:    "Smaller positions (<$100) producing better risk-adjusted returns",
				Evidence:       []string{fmt.Sprintf("Small: %.2f%% vs Large: %.2f%%", smallAvg, largeAvg)},
				Recommendation: "Start with smaller position sizes to reduce risk and improve consistency",
			})
		}
	}

	return patterns
}

// identifyFailurePatterns finds patterns in losing trades
func (fg *FeedbackGenerator) identifyFailurePatterns(outcomes []DecisionOutcome, metrics *Metrics) []TradingPattern {
	var patterns []TradingPattern

	// ============================================================================
	// TIER 1: Execution-Level Failure Analysis (Trade Failure V2)
	// ============================================================================

	// Analyze failures using microstructure-based Trade Failure V2
	v2FailureReasons := make(map[string]struct {
		count    int
		pnlSum   float64
		evidence []string
		examples []*decision.RecentOrder
	})

	for _, outcome := range outcomes {
		// Only analyze failed trades that have microstructure data
		if !outcome.Success && outcome.RecentOrder != nil {
			thresholds := fg.failureThresholds
			analysis := decision.AnalyzeFailedTradeWithThresholds(outcome.RecentOrder, &thresholds)
			if analysis != nil {
				reason := string(analysis.PrimaryReason)
				confidence := analysis.ConfidenceScore

				entry := v2FailureReasons[reason]
				entry.count++
				entry.pnlSum += outcome.RealizedPnLPct

				// Store evidence (top 3 examples per reason)
				if len(entry.examples) < 3 {
					entry.examples = append(entry.examples, outcome.RecentOrder)
				}

				// Store detailed notes from V2 analysis
				evidence := fmt.Sprintf("%s (confidence: %.0f%%)",
					analysis.DetailedNotes, confidence*100)
				if len(evidence) > 100 {
					evidence = evidence[:100] + "..."
				}
				entry.evidence = append(entry.evidence, evidence)

				v2FailureReasons[reason] = entry
			}
		}
	}

	// Convert V2 failure reasons to trading patterns
	for reason, data := range v2FailureReasons {
		if data.count >= fg.config.MinPatternFrequency {
			avgPnL := data.pnlSum / float64(data.count)

			// Get V2 recommendation
			v2Reason := decision.TradeFailureReason(reason)
			recommendation := getV2Recommendation(v2Reason)

			patterns = append(patterns, TradingPattern{
				PatternType:    reason, // e.g., "chasing_entry", "stop_too_tight"
				Frequency:      data.count,
				AvgPnL:         0,
				AvgPnLPct:      avgPnL,
				Description:    fmt.Sprintf("Execution-level failure: %s", humanizeV2Reason(v2Reason)),
				Evidence:       data.evidence,
				Recommendation: recommendation,
			})
		}
	}

	// ============================================================================
	// TIER 2: Behavioral Pattern Detection (Original Feedback System)
	// ============================================================================

	// Pattern 1: Holding losers too long
	longLosses := 0
	longLossesPnL := 0.0

	for _, outcome := range outcomes {
		if !outcome.Success {
			duration := parseDurationMinutes(outcome.HoldDuration)
			if duration > 240 { // > 4 hours
				longLosses++
				longLossesPnL += outcome.RealizedPnLPct
			}
		}
	}

	if longLosses >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "holding_losers",
			Frequency:      longLosses,
			AvgPnL:         0,
			AvgPnLPct:      longLossesPnL / float64(longLosses),
			Description:    "Holding losing positions for too long (>4h)",
			Evidence:       []string{fmt.Sprintf("%d trades, avg loss %.2f%%", longLosses, longLossesPnL/float64(longLosses))},
			Recommendation: "⚠️ CRITICAL: Cut losses faster. Set tighter stop-losses and respect them",
		})
	}

	// Pattern 2: Leverage amplifying losses
	highLevLosses := 0
	highLevLossPnL := 0.0
	lowLevLosses := 0
	lowLevLossPnL := 0.0

	for _, outcome := range outcomes {
		if !outcome.Success {
			if outcome.Leverage >= 5 {
				highLevLosses++
				highLevLossPnL += outcome.RealizedPnLPct
			} else if outcome.Leverage <= 3 {
				lowLevLosses++
				lowLevLossPnL += outcome.RealizedPnLPct
			}
		}
	}

	if highLevLosses >= fg.config.MinPatternFrequency && lowLevLosses >= fg.config.MinPatternFrequency {
		highAvgLoss := highLevLossPnL / float64(highLevLosses)
		lowAvgLoss := lowLevLossPnL / float64(lowLevLosses)

		if highAvgLoss < lowAvgLoss { // More negative = worse
			patterns = append(patterns, TradingPattern{
				PatternType:    "high_leverage_losses",
				Frequency:      highLevLosses,
				AvgPnL:         0,
				AvgPnLPct:      highAvgLoss,
				Description:    "High leverage (≥5x) amplifying losses significantly",
				Evidence:       []string{fmt.Sprintf("High lev losses: %.2f%% avg vs Low lev: %.2f%%", highAvgLoss, lowAvgLoss)},
				Recommendation: "⚠️ CRITICAL: Reduce leverage. High leverage is destroying capital",
			})
		}
	}

	// Pattern 3: Premature entries (quick losses)
	quickLosses := 0
	quickLossPnL := 0.0

	for _, outcome := range outcomes {
		if !outcome.Success {
			duration := parseDurationMinutes(outcome.HoldDuration)
			if duration < 15 { // < 15 minutes
				quickLosses++
				quickLossPnL += outcome.RealizedPnLPct
			}
		}
	}

	if quickLosses >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "premature_entries",
			Frequency:      quickLosses,
			AvgPnL:         0,
			AvgPnLPct:      quickLossPnL / float64(quickLosses),
			Description:    "Getting stopped out quickly (<15min) suggests poor entry timing",
			Evidence:       []string{fmt.Sprintf("%d quick losses, avg %.2f%%", quickLosses, quickLossPnL/float64(quickLosses))},
			Recommendation: "⚠️ Improve entry timing: wait for better confirmation before entering",
		})
	}

	// Pattern 4: Symbol-specific failures
	symbolLosses := make(map[string]int)
	symbolLossPnL := make(map[string]float64)

	for _, outcome := range outcomes {
		if !outcome.Success {
			symbolLosses[outcome.Symbol]++
			symbolLossPnL[outcome.Symbol] += outcome.RealizedPnLPct
		}
	}

	for symbol, losses := range symbolLosses {
		if losses >= fg.config.MinPatternFrequency {
			avgLoss := symbolLossPnL[symbol] / float64(losses)
			if avgLoss < -3.0 { // At least -3% average loss
				patterns = append(patterns, TradingPattern{
					PatternType:    "symbol_weakness",
					Frequency:      losses,
					AvgPnL:         0,
					AvgPnLPct:      avgLoss,
					Description:    fmt.Sprintf("Consistent losses on %s", symbol),
					Evidence:       []string{fmt.Sprintf("%d losses, %.2f%% avg loss", losses, avgLoss)},
					Recommendation: fmt.Sprintf("⚠️ Avoid or reduce exposure to %s until market conditions improve", symbol),
				})
			}
		}
	}

	// Pattern 5: Overtrading (multiple trades in short period)
	tradesByHour := make(map[string]int) // hour key: "2026-01-11-14"
	overtradeInstances := 0
	overtradeLoss := 0.0

	for i, outcome := range outcomes {
		hourKey := outcome.Timestamp.Format("2006-01-02-15")
		tradesByHour[hourKey]++

		// If 3+ trades in same hour and this one lost
		if tradesByHour[hourKey] >= 3 && !outcome.Success {
			// Check if this is part of rapid sequence
			if i > 0 {
				timeDiff := outcome.Timestamp.Sub(outcomes[i-1].Timestamp).Minutes()
				if timeDiff < 30 { // Less than 30 min between trades
					overtradeInstances++
					overtradeLoss += outcome.RealizedPnLPct
				}
			}
		}
	}

	if overtradeInstances >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "overtrading",
			Frequency:      overtradeInstances,
			AvgPnL:         0,
			AvgPnLPct:      overtradeLoss / float64(overtradeInstances),
			Description:    "Taking too many trades in short periods (3+ per hour) leading to losses",
			Evidence:       []string{fmt.Sprintf("%d instances, avg loss %.2f%%", overtradeInstances, overtradeLoss/float64(overtradeInstances))},
			Recommendation: "⚠️ Reduce trading frequency. Wait at least 1 hour between trades unless strong setup",
		})
	}

	// Pattern 6: Win streak breakage (giving back profits)
	winStreakGivebacks := 0
	givebackLoss := 0.0

	for i := 2; i < len(outcomes); i++ {
		// If previous 2 were wins and current is loss
		if outcomes[i-1].Success && outcomes[i-2].Success && !outcomes[i].Success {
			// Check if loss is significant
			if outcomes[i].RealizedPnLPct < -3.0 {
				winStreakGivebacks++
				givebackLoss += outcomes[i].RealizedPnLPct
			}
		}
	}

	if winStreakGivebacks >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "profit_giveback",
			Frequency:      winStreakGivebacks,
			AvgPnL:         0,
			AvgPnLPct:      givebackLoss / float64(winStreakGivebacks),
			Description:    "Giving back profits after win streaks with large losses",
			Evidence:       []string{fmt.Sprintf("%d occurrences, avg loss %.2f%%", winStreakGivebacks, givebackLoss/float64(winStreakGivebacks))},
			Recommendation: "⚠️ After 2+ wins, take a break or reduce position size on next trade",
		})
	}

	// Pattern 7: Revenge trading (quick trade after loss)
	revengeTradeCount := 0
	revengeTradeLoss := 0.0

	for i := 1; i < len(outcomes); i++ {
		// If previous was a loss
		if !outcomes[i-1].Success {
			// And current trade came within 15 minutes
			timeDiff := outcomes[i].Timestamp.Sub(outcomes[i-1].Timestamp).Minutes()
			if timeDiff < 15 && !outcomes[i].Success {
				revengeTradeCount++
				revengeTradeLoss += outcomes[i].RealizedPnLPct
			}
		}
	}

	if revengeTradeCount >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "revenge_trading",
			Frequency:      revengeTradeCount,
			AvgPnL:         0,
			AvgPnLPct:      revengeTradeLoss / float64(revengeTradeCount),
			Description:    "Entering new trades too quickly after losses (<15min), often resulting in more losses",
			Evidence:       []string{fmt.Sprintf("%d revenge trades, avg loss %.2f%%", revengeTradeCount, revengeTradeLoss/float64(revengeTradeCount))},
			Recommendation: "⚠️ CRITICAL: After a loss, wait at least 30 minutes before next trade to avoid emotional decisions",
		})
	}

	// Pattern 8: Poor trading hours
	hourlyLosses := make(map[int]int)
	hourlyLossPnL := make(map[int]float64)
	hourlyTotalTrades := make(map[int]int)

	for _, outcome := range outcomes {
		hour := outcome.Timestamp.Hour()
		hourlyTotalTrades[hour]++
		if !outcome.Success {
			hourlyLosses[hour]++
			hourlyLossPnL[hour] += outcome.RealizedPnLPct
		}
	}

	worstHour := -1
	worstLossRate := 0.0
	for hour, losses := range hourlyLosses {
		if hourlyTotalTrades[hour] >= fg.config.MinPatternFrequency {
			lossRate := float64(losses) / float64(hourlyTotalTrades[hour]) * 100
			if lossRate > worstLossRate && lossRate > 60 { // >60% loss rate
				worstLossRate = lossRate
				worstHour = hour
			}
		}
	}

	if worstHour >= 0 {
		avgLoss := hourlyLossPnL[worstHour] / float64(hourlyLosses[worstHour])
		patterns = append(patterns, TradingPattern{
			PatternType:    "poor_trading_hours",
			Frequency:      hourlyLosses[worstHour],
			AvgPnL:         0,
			AvgPnLPct:      avgLoss,
			Description:    fmt.Sprintf("Poor performance during hour %02d:00--%02d:59 UTC", worstHour, worstHour),
			Evidence:       []string{fmt.Sprintf("%.1f%% loss rate, %.2f%% avg loss", worstLossRate, avgLoss)},
			Recommendation: fmt.Sprintf("⚠️ AVOID trading during %02d:00 UTC when conditions are unfavorable", worstHour),
		})
	}

	// Pattern 9: Oversized positions on losses
	largePosLosses := 0
	largePosLossPnL := 0.0
	smallPosLosses := 0
	smallPosLossPnL := 0.0

	for _, outcome := range outcomes {
		if !outcome.Success {
			if outcome.PositionSize > 500 {
				largePosLosses++
				largePosLossPnL += outcome.RealizedPnLPct
			} else if outcome.PositionSize < 100 {
				smallPosLosses++
				smallPosLossPnL += outcome.RealizedPnLPct
			}
		}
	}

	if largePosLosses >= fg.config.MinPatternFrequency && smallPosLosses >= fg.config.MinPatternFrequency {
		largeAvgLoss := largePosLossPnL / float64(largePosLosses)
		smallAvgLoss := smallPosLossPnL / float64(smallPosLosses)

		if largeAvgLoss < smallAvgLoss { // More negative = worse
			patterns = append(patterns, TradingPattern{
				PatternType:    "oversized_positions",
				Frequency:      largePosLosses,
				AvgPnL:         0,
				AvgPnLPct:      largeAvgLoss,
				Description:    "Large positions (>$500) amplifying losses significantly",
				Evidence:       []string{fmt.Sprintf("Large: %.2f%% vs Small: %.2f%%", largeAvgLoss, smallAvgLoss)},
				Recommendation: "⚠️ CRITICAL: Reduce position sizes. Large positions are destroying capital faster",
			})
		}
	}

	return patterns
}

func parseDurationMinutes(duration string) int {
	// Parse duration like "2h30m" or "45m"
	var hours, minutes int
	if strings.Contains(duration, "h") {
		fmt.Sscanf(duration, "%dh%dm", &hours, &minutes)
	} else {
		fmt.Sscanf(duration, "%dm", &minutes)
	}
	return hours*60 + minutes
}

// getTopTrades returns the top winning or losing trades
func (fg *FeedbackGenerator) getTopTrades(outcomes []DecisionOutcome, winning bool, count int) []DecisionOutcome {
	filtered := make([]DecisionOutcome, 0)

	for _, outcome := range outcomes {
		if outcome.Success == winning {
			filtered = append(filtered, outcome)
		}
	}

	// Sort by absolute PnL percentage
	sort.Slice(filtered, func(i, j int) bool {
		return math.Abs(filtered[i].RealizedPnLPct) > math.Abs(filtered[j].RealizedPnLPct)
	})

	if len(filtered) > count {
		filtered = filtered[:count]
	}

	return filtered
}

// generateKeyInsights creates actionable insights from the analysis
func (fg *FeedbackGenerator) generateKeyInsights(metrics *Metrics, outcomes []DecisionOutcome, analysis *FeedbackAnalysis) []string {
	insights := make([]string, 0)

	// Performance assessment
	if metrics.TotalReturnPct < -5 {
		insights = append(insights, fmt.Sprintf("🔴 CRITICAL: Portfolio down %.2f%%. Immediate strategy revision needed", metrics.TotalReturnPct))
	} else if metrics.TotalReturnPct < 0 {
		insights = append(insights, fmt.Sprintf("⚠️ Portfolio slightly negative (%.2f%%). Minor adjustments recommended", metrics.TotalReturnPct))
	} else if metrics.TotalReturnPct > 10 {
		insights = append(insights, fmt.Sprintf("✅ Strong performance: +%.2f%%. Current strategy is working well", metrics.TotalReturnPct))
	}

	// Win rate analysis
	if metrics.WinRate < 40 {
		insights = append(insights, fmt.Sprintf("📉 Low win rate (%.1f%%). Focus on better entry timing and trade selection", metrics.WinRate))
	} else if metrics.WinRate > 60 {
		insights = append(insights, fmt.Sprintf("📈 Excellent win rate (%.1f%%). Good trade selection", metrics.WinRate))
	}

	// Profit factor analysis
	if metrics.ProfitFactor < 1.0 {
		insights = append(insights, fmt.Sprintf("⚠️ Profit factor %.2f < 1.0: Losses exceed profits. Let winners run longer", metrics.ProfitFactor))
	} else if metrics.ProfitFactor < 1.5 {
		insights = append(insights, fmt.Sprintf("Profit factor %.2f is acceptable but can be improved. Focus on risk/reward ratio", metrics.ProfitFactor))
	} else {
		insights = append(insights, fmt.Sprintf("✅ Good profit factor (%.2f). Average wins sufficiently larger than losses", metrics.ProfitFactor))
	}

	// Drawdown analysis
	if metrics.MaxDrawdownPct > 30 {
		insights = append(insights, fmt.Sprintf("🔴 CRITICAL: %.1f%% max drawdown is excessive. Reduce position sizes and leverage", metrics.MaxDrawdownPct))
	} else if metrics.MaxDrawdownPct > 20 {
		insights = append(insights, fmt.Sprintf("⚠️ Drawdown %.1f%% is high. Improve risk management", metrics.MaxDrawdownPct))
	} else {
		insights = append(insights, fmt.Sprintf("✅ Drawdown well-controlled at %.1f%%", metrics.MaxDrawdownPct))
	}

	// Pattern-specific insights
	for _, pattern := range analysis.FailurePatterns {
		switch pattern.PatternType {
		case "holding_losers":
			insights = append(insights, "⚠️ Tendency to hold losing positions. Set and respect stop-losses")
		case "high_leverage_losses":
			insights = append(insights, "⚠️ High leverage causing outsized losses. Reduce leverage immediately")
		case "overtrading":
			insights = append(insights, "⚠️ Overtrading detected. Quality over quantity - be more selective")
		case "revenge_trading":
			insights = append(insights, "🔴 CRITICAL: Revenge trading after losses. Take breaks to avoid emotional decisions")
		case "profit_giveback":
			insights = append(insights, "⚠️ Giving back profits after wins. Consider taking breaks after 2+ consecutive wins")
		case "oversized_positions":
			insights = append(insights, "⚠️ Position sizing too aggressive. Reduce to preserve capital")
		}
	}

	return insights
}

// generateRecommendedActions creates specific actions to improve performance
func (fg *FeedbackGenerator) generateRecommendedActions(analysis *FeedbackAnalysis) []string {
	actions := make([]string, 0)

	// Based on overall performance
	if analysis.TotalReturnPct < 0 {
		actions = append(actions, "1. REDUCE POSITION SIZES: Start with 50% of current position sizes until profitability improves")
		actions = append(actions, "2. TIGHTER STOP-LOSSES: Set stop-losses at 2-3% maximum loss per trade")
		actions = append(actions, "3. SELECTIVE TRADING: Only take trades with 70%+ confidence and clear technical setup")
	}

	// Based on win rate
	if analysis.WinRate < 45 {
		actions = append(actions, "4. IMPROVE ENTRY TIMING: Wait for stronger confirmations (multiple timeframe alignment + volume)")
		actions = append(actions, "5. SKIP LOW-CONFIDENCE TRADES: Avoid trades with confidence < 70%")
	}

	// Based on failure patterns
	hasHoldingLosers := false
	hasHighLevLosses := false
	hasPrematureEntries := false
	hasOvertrading := false
	hasRevengeTrading := false
	hasProfitGiveback := false
	hasOversizedPos := false
	hasPoorHours := false

	for _, pattern := range analysis.FailurePatterns {
		switch pattern.PatternType {
		case "holding_losers":
			hasHoldingLosers = true
		case "high_leverage_losses":
			hasHighLevLosses = true
		case "premature_entries":
			hasPrematureEntries = true
		case "overtrading":
			hasOvertrading = true
		case "revenge_trading":
			hasRevengeTrading = true
		case "profit_giveback":
			hasProfitGiveback = true
		case "oversized_positions":
			hasOversizedPos = true
		case "poor_trading_hours":
			hasPoorHours = true
		}
	}

	if hasHoldingLosers {
		actions = append(actions, "6. DISCIPLINE ON STOP-LOSSES: When stop-loss is hit, exit immediately without hesitation")
	}

	if hasHighLevLosses {
		actions = append(actions, "7. LOWER LEVERAGE: Reduce leverage to 2-3x maximum until consistent profitability achieved")
	}

	if hasPrematureEntries {
		actions = append(actions, "8. BE PATIENT: Wait for price to confirm direction before entering (avoid FOMO)")
	}

	if hasOvertrading {
		actions = append(actions, "9. REDUCE FREQUENCY: Limit to maximum 2-3 high-quality trades per day")
	}

	if hasRevengeTrading {
		actions = append(actions, "10. MANDATORY COOLDOWN: Wait 30+ minutes after any loss before next trade")
	}

	if hasProfitGiveback {
		actions = append(actions, "11. PROTECT PROFITS: After 2+ wins, take a break or reduce position size by 50%")
	}

	if hasOversizedPos {
		actions = append(actions, "12. SIZE DOWN: Reduce position sizes to $100-200 range until consistency improves")
	}

	if hasPoorHours {
		actions = append(actions, "13. TIME SELECTION: Avoid trading during identified poor-performance hours")
	}

	// Based on success patterns
	actionNum := 14 // Continue from failure pattern actions
	for _, pattern := range analysis.SuccessPatterns {
		switch pattern.PatternType {
		case "symbol_affinity":
			actions = append(actions, fmt.Sprintf("%d. FOCUS ON WINNERS: %s", actionNum, pattern.Recommendation))
			actionNum++
		case "quick_profit_taking":
			actions = append(actions, fmt.Sprintf("%d. CONTINUE SCALPING: Quick profit-taking has been effective", actionNum))
			actionNum++
		case "momentum_trading":
			actions = append(actions, fmt.Sprintf("%d. RIDE MOMENTUM: When on a win streak, maintain the same approach", actionNum))
			actionNum++
		case "optimal_trading_hours":
			actions = append(actions, fmt.Sprintf("%d. TIME FOCUS: %s", actionNum, pattern.Recommendation))
			actionNum++
		case "optimal_position_sizing":
			actions = append(actions, fmt.Sprintf("%d. POSITION SIZING: %s", actionNum, pattern.Recommendation))
			actionNum++
		}
	}

	// Based on profit factor
	if analysis.ProfitFactor < 1.5 {
		actions = append(actions, "11. LET WINNERS RUN: Move stop-loss to breakeven and let profitable trades reach larger targets")
		actions = append(actions, "12. IMPROVE RISK/REWARD: Target at least 2:1 reward-to-risk ratio on all trades")
	}

	return actions
}

// analyzeMarketConditions determines the current market regime
func (fg *FeedbackGenerator) analyzeMarketConditions(metrics *Metrics, outcomes []DecisionOutcome) string {
	if metrics.WinRate < 40 && metrics.TotalReturnPct < -5 {
		return "DIFFICULT MARKET: High volatility or ranging conditions making trend-following difficult. Consider reducing activity"
	}

	if metrics.WinRate > 55 && metrics.TotalReturnPct > 5 {
		return "FAVORABLE MARKET: Clear trends present, strategy is well-aligned with current conditions"
	}

	if metrics.WinRate > 50 && metrics.ProfitFactor < 1.2 {
		return "MIXED MARKET: Winning often but profits are small. Need to let winners run longer"
	}

	return "NEUTRAL MARKET: Standard trading conditions, continue with current approach"
}

// FormatFeedbackForPrompt formats the feedback analysis for inclusion in AI prompts
func (fg *FeedbackGenerator) FormatFeedbackForPrompt(analysis *FeedbackAnalysis, lang string) string {
	if analysis == nil {
		return ""
	}

	return analysis.FormatForPrompt(lang)
}

// FormatForPrompt formats the feedback analysis for inclusion in AI prompts (method on FeedbackAnalysis)
func (analysis *FeedbackAnalysis) FormatForPrompt(lang string) string {
	if analysis == nil {
		return ""
	}

	var sb strings.Builder

	if lang == "zh" {
		sb.WriteString("## 📊 历史表现反馈\n\n")
		sb.WriteString(fmt.Sprintf("**分析周期**: %s (覆盖 %d 个决策)\n", analysis.AnalysisPeriod, analysis.DecisionsCovered))
		sb.WriteString(fmt.Sprintf("**总回报**: %.2f%% | **胜率**: %.1f%% | **盈利因子**: %.2f | **最大回撤**: %.1f%%\n\n",
			analysis.TotalReturnPct, analysis.WinRate, analysis.ProfitFactor, analysis.MaxDrawdown))

		if len(analysis.KeyInsights) > 0 {
			sb.WriteString("### 关键洞察\n\n")
			for _, insight := range analysis.KeyInsights {
				sb.WriteString(fmt.Sprintf("- %s\n", insight))
			}
			sb.WriteString("\n")
		}

		// ============================================================================
		// EXECUTION-LEVEL FAILURE ANALYSIS (Trade Failure V2 Microstructure)
		// ============================================================================
		if len(analysis.FailurePatterns) > 0 {
			// Separate V2 execution failures from other patterns
			var v2Failures []TradingPattern
			var otherFailures []TradingPattern

			for _, pattern := range analysis.FailurePatterns {
				// V2 failure reasons contain microstructure keywords
				if strings.Contains(pattern.PatternType, "_") ||
					strings.Contains(pattern.Description, "Execution-level") {
					v2Failures = append(v2Failures, pattern)
				} else {
					otherFailures = append(otherFailures, pattern)
				}
			}

			// Display V2 execution-level diagnostics
			if len(v2Failures) > 0 {
				sb.WriteString("### 📋 执行级失败诊断 (微观结构分析)\n\n")
				sb.WriteString("**这些失败根植于市场执行条件和入场/出场时机：**\n\n")

				for i, pattern := range v2Failures {
					if i >= 5 { // Limit to top 5
						break
					}
					sb.WriteString(fmt.Sprintf("**%s**\n", pattern.Description))
					sb.WriteString(fmt.Sprintf("   发生: %d 次 | 平均亏损: %.2f%%\n", pattern.Frequency, pattern.AvgPnLPct))

					// Display evidence (first 2 pieces)
					if len(pattern.Evidence) > 0 {
						for j, evidence := range pattern.Evidence {
							if j >= 2 {
								break
							}
							sb.WriteString(fmt.Sprintf("   证据: %s\n", evidence))
						}
					}

					sb.WriteString(fmt.Sprintf("   **行动**: %s\n\n", pattern.Recommendation))
				}
			}

			// Display other failure patterns
			if len(otherFailures) > 0 {
				sb.WriteString("### ⚠️ 其他发现的失败模式\n\n")
				for i, pattern := range otherFailures {
					if i >= 3 {
						break
					}
					sb.WriteString(fmt.Sprintf("**%s** (发生 %d 次, 平均亏损 %.2f%%)\n",
						pattern.Description, pattern.Frequency, pattern.AvgPnLPct))
					sb.WriteString(fmt.Sprintf("   → %s\n\n", pattern.Recommendation))
				}
			}
		} else if len(analysis.FailurePatterns) > 0 {
			sb.WriteString("### ⚠️ 发现的失败模式\n\n")
			for i, pattern := range analysis.FailurePatterns {
				if i >= 3 {
					break // Limit to top 3
				}
				sb.WriteString(fmt.Sprintf("**%s** (发生 %d 次, 平均亏损 %.2f%%)\n",
					pattern.Description, pattern.Frequency, pattern.AvgPnLPct))
				sb.WriteString(fmt.Sprintf("   → %s\n\n", pattern.Recommendation))
			}
		}

		if len(analysis.SuccessPatterns) > 0 {
			sb.WriteString("### ✅ 发现的成功模式\n\n")
			for i, pattern := range analysis.SuccessPatterns {
				if i >= 3 {
					break
				}
				sb.WriteString(fmt.Sprintf("**%s** (发生 %d 次, 平均盈利 %.2f%%)\n",
					pattern.Description, pattern.Frequency, pattern.AvgPnLPct))
				sb.WriteString(fmt.Sprintf("   → %s\n\n", pattern.Recommendation))
			}
		}

		if len(analysis.RecommendedActions) > 0 {
			sb.WriteString("### 🎯 建议行动\n\n")
			for _, action := range analysis.RecommendedActions {
				if strings.HasPrefix(action, "⚠️") || strings.HasPrefix(action, "🔴") {
					sb.WriteString(fmt.Sprintf("**%s**\n", action))
				} else {
					sb.WriteString(fmt.Sprintf("%s\n", action))
				}
			}
			sb.WriteString("\n")
		}

		sb.WriteString(fmt.Sprintf("**市场条件评估**: %s\n\n", analysis.MarketConditions))

		sb.WriteString("---\n\n")
		sb.WriteString("**💡 基于以上反馈进行决策**: 学习失败教训，复制成功模式，严格执行建议行动\n\n")

	} else {
		sb.WriteString("## 📊 Historical Performance Feedback\n\n")
		sb.WriteString(fmt.Sprintf("**Analysis Period**: %s (%d decisions covered)\n", analysis.AnalysisPeriod, analysis.DecisionsCovered))
		sb.WriteString(fmt.Sprintf("**Total Return**: %.2f%% | **Win Rate**: %.1f%% | **Profit Factor**: %.2f | **Max Drawdown**: %.1f%%\n\n",
			analysis.TotalReturnPct, analysis.WinRate, analysis.ProfitFactor, analysis.MaxDrawdown))

		if len(analysis.KeyInsights) > 0 {
			sb.WriteString("### Key Insights\n\n")
			for _, insight := range analysis.KeyInsights {
				sb.WriteString(fmt.Sprintf("- %s\n", insight))
			}
			sb.WriteString("\n")
		}

		// ============================================================================
		// EXECUTION-LEVEL FAILURE ANALYSIS (Trade Failure V2 Microstructure)
		// ============================================================================
		if len(analysis.FailurePatterns) > 0 {
			// Separate V2 execution failures from other patterns
			var v2Failures []TradingPattern
			var otherFailures []TradingPattern

			for _, pattern := range analysis.FailurePatterns {
				// V2 failure reasons contain microstructure keywords
				if strings.Contains(pattern.PatternType, "_") ||
					strings.Contains(pattern.Description, "Execution-level") {
					v2Failures = append(v2Failures, pattern)
				} else {
					otherFailures = append(otherFailures, pattern)
				}
			}

			// Display V2 execution-level diagnostics
			if len(v2Failures) > 0 {
				sb.WriteString("### 📋 Execution-Level Failure Diagnostics (Microstructure)\n\n")
				sb.WriteString("**These failures are rooted in actual market execution conditions and entry/exit timing:**\n\n")

				for i, pattern := range v2Failures {
					if i >= 5 { // Limit to top 5
						break
					}
					sb.WriteString(fmt.Sprintf("**%s**\n", pattern.Description))
					sb.WriteString(fmt.Sprintf("   Occurred: %d times | Avg Loss: %.2f%%\n", pattern.Frequency, pattern.AvgPnLPct))

					// Display evidence (first 2 pieces)
					if len(pattern.Evidence) > 0 {
						for j, evidence := range pattern.Evidence {
							if j >= 2 {
								break
							}
							sb.WriteString(fmt.Sprintf("   Evidence: %s\n", evidence))
						}
					}

					sb.WriteString(fmt.Sprintf("   **Action**: %s\n\n", pattern.Recommendation))
				}
			}

			// Display other failure patterns
			if len(otherFailures) > 0 {
				sb.WriteString("### ⚠️ Other Identified Failure Patterns\n\n")
				for i, pattern := range otherFailures {
					if i >= 3 {
						break
					}
					sb.WriteString(fmt.Sprintf("**%s** (occurred %d times, avg loss %.2f%%)\n",
						pattern.Description, pattern.Frequency, pattern.AvgPnLPct))
					sb.WriteString(fmt.Sprintf("   → %s\n\n", pattern.Recommendation))
				}
			}
		} else if len(analysis.FailurePatterns) > 0 {
			sb.WriteString("### ⚠️ Identified Failure Patterns\n\n")
			for i, pattern := range analysis.FailurePatterns {
				if i >= 3 {
					break
				}
				sb.WriteString(fmt.Sprintf("**%s** (occurred %d times, avg loss %.2f%%)\n",
					pattern.Description, pattern.Frequency, pattern.AvgPnLPct))
				sb.WriteString(fmt.Sprintf("   → %s\n\n", pattern.Recommendation))
			}
		}

		if len(analysis.SuccessPatterns) > 0 {
			sb.WriteString("### ✅ Identified Success Patterns\n\n")
			for i, pattern := range analysis.SuccessPatterns {
				if i >= 3 {
					break
				}
				sb.WriteString(fmt.Sprintf("**%s** (occurred %d times, avg profit %.2f%%)\n",
					pattern.Description, pattern.Frequency, pattern.AvgPnLPct))
				sb.WriteString(fmt.Sprintf("   → %s\n\n", pattern.Recommendation))
			}
		}

		if len(analysis.RecommendedActions) > 0 {
			sb.WriteString("### 🎯 Recommended Actions\n\n")
			for _, action := range analysis.RecommendedActions {
				if strings.HasPrefix(action, "⚠️") || strings.HasPrefix(action, "🔴") {
					sb.WriteString(fmt.Sprintf("**%s**\n", action))
				} else {
					sb.WriteString(fmt.Sprintf("%s\n", action))
				}
			}
			sb.WriteString("\n")
		}

		sb.WriteString(fmt.Sprintf("**Market Conditions Assessment**: %s\n\n", analysis.MarketConditions))

		sb.WriteString("---\n\n")
		sb.WriteString("**💡 Make decisions based on this feedback**: Learn from failures, replicate successes, and strictly follow recommended actions\n\n")
	}

	return sb.String()
}

// SaveFeedbackAnalysis saves the feedback analysis to disk for later reference
func (fg *FeedbackGenerator) SaveFeedbackAnalysis(analysis *FeedbackAnalysis) error {
	if analysis == nil {
		return nil
	}

	feedbackPath := filepath.Join(runDir(fg.runID), "feedback_analysis.json")
	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal feedback: %w", err)
	}

	if err := os.WriteFile(feedbackPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write feedback file: %w", err)
	}

	logger.Infof("💾 Feedback analysis saved to %s", feedbackPath)
	return nil
}

// LoadFeedbackAnalysis loads a previously saved feedback analysis
func LoadFeedbackAnalysis(runID string) (*FeedbackAnalysis, error) {
	feedbackPath := filepath.Join(runDir(runID), "feedback_analysis.json")
	data, err := os.ReadFile(feedbackPath)
	if err != nil {
		return nil, err
	}

	var analysis FeedbackAnalysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		return nil, err
	}

	return &analysis, nil
}

// ============================================================================
// Trade Failure V2 Integration Helpers
// ============================================================================

// humanizeV2Reason converts Trade Failure V2 reason code to human-readable text
func humanizeV2Reason(reason decision.TradeFailureReason) string {
	switch reason {
	case decision.ReasonSignalQualityLow:
		return "Signal quality too low - weak edge"
	case decision.ReasonRegimeMismatch:
		return "Trade conflicted with market regime"
	case decision.ReasonLiquidityRiskHigh:
		return "Liquidity risk too high at entry"
	case decision.ReasonStackedRisk:
		return "Overexposed to same market factor"
	case decision.ReasonChasingEntry:
		return "Chased entry with excessive slippage"
	case decision.ReasonFalseBreakoutV2:
		return "False breakout - no follow-through"
	case decision.ReasonPrematureEntry:
		return "Entered before confirmation criteria met"
	case decision.ReasonSizingError:
		return "Position size too large for available liquidity"
	case decision.ReasonSlippageExceeded:
		return "Actual slippage exceeded budget"
	case decision.ReasonStopTooTight:
		return "Stop loss too tight relative to volatility"
	case decision.ReasonMomentumDecay:
		return "Momentum decayed - volume/OI collapsed"
	case decision.ReasonLiquidityDried:
		return "Market liquidity dried up during hold"
	case decision.ReasonStopHitRegimeChange:
		return "Stop hit due to regime change"
	case decision.ReasonLateExitGiveBack:
		return "Exited late with large give-back from peak"
	case decision.ReasonTrendReversalIgnored:
		return "Missed trend reversal signal"
	case decision.ReasonHighSlippageExit:
		return "Poor exit execution with high slippage"
	case decision.ReasonFundingDrag:
		return "Funding costs ate into profit"
	case decision.ReasonBorrowingCostHigh:
		return "Borrowing costs were significant"
	case decision.ReasonTechnicalFault:
		return "Technical or execution system error"
	default:
		return string(reason)
	}
}

// getV2Recommendation provides actionable recommendations for V2 failure reasons
func getV2Recommendation(reason decision.TradeFailureReason) string {
	switch reason {
	case decision.ReasonSignalQualityLow:
		return "⚠️ SIGNALS: Only enter trades with strong, multi-confirmation signal. Model confidence alone is insufficient."
	case decision.ReasonRegimeMismatch:
		return "⚠️ REGIME: Check market regime before entry. Skip trades in choppy/sideways markets (ChopScore > 50)."
	case decision.ReasonLiquidityRiskHigh:
		return "⚠️ LIQUIDITY: Reduce position size for illiquid symbols. Monitor spread and depth at entry."
	case decision.ReasonStackedRisk:
		return "⚠️ CORRELATION: Don't stack positions in same sector/factor. Reduce correlation exposure."
	case decision.ReasonChasingEntry:
		return "⚠️ EXECUTION: Don't chase entries. Set hard limit on entry slippage. Cancel if slippage exceeds budget."
	case decision.ReasonFalseBreakoutV2:
		return "⚠️ CONFIRMATION: Require volume confirmation (+50% baseline) and OI increase before entering breakouts."
	case decision.ReasonPrematureEntry:
		return "⚠️ TIMING: Wait for all confirmation criteria. Don't enter before price pattern validates."
	case decision.ReasonSizingError:
		return "⚠️ SIZE: Reduce position size based on available liquidity. Use ATR-based sizing with depth checks."
	case decision.ReasonSlippageExceeded:
		return "⚠️ EXECUTION: Monitor actual slippage vs budget. Reduce order size or use limit orders for large positions."
	case decision.ReasonStopTooTight:
		return "⚠️ RISK: Widen stops to minimum 1.5x ATR. Use volatility-adjusted stops, not arbitrary percentages."
	case decision.ReasonMomentumDecay:
		return "⚠️ EXITS: Track momentum during hold. Use trailing stops that move with momentum, not just price."
	case decision.ReasonLiquidityDried:
		return "⚠️ EXITS: Monitor spread during hold. Exit immediately if spread widens >3x entry level."
	case decision.ReasonStopHitRegimeChange:
		return "⚠️ REGIME: Monitor regime throughout hold. Reduce exposure if regime shifts. Consider dynamic stops."
	case decision.ReasonLateExitGiveBack:
		return "⚠️ EXITS: Set profit targets based on momentum. Don't hold hoping for bigger moves. Exit on confirmation loss."
	case decision.ReasonTrendReversalIgnored:
		return "⚠️ EXITS: Monitor trend indicators. Exit if trend reversal signals appear. Don't force profits."
	case decision.ReasonHighSlippageExit:
		return "⚠️ EXECUTION: Use limit orders for exits. Avoid market orders in low-liquidity conditions."
	case decision.ReasonFundingDrag:
		return "⚠️ COSTS: Monitor funding rates. Reduce holding time if funding becomes expensive."
	case decision.ReasonBorrowingCostHigh:
		return "⚠️ COSTS: Check borrowing costs before entry. Skip trades where costs exceed expected profit."
	case decision.ReasonTechnicalFault:
		return "⚠️ SYSTEMS: Log technical faults. Improve system reliability. Skip trades until systems stabilize."
	default:
		return "Review execution quality and market conditions for this failure mode."
	}
}

// FormatForDebate formats the feedback for multi-agent debate context
// Emphasizes areas of contention and decision points tailored to agent roles
func (analysis *FeedbackAnalysis) FormatForDebate(lang string, agentRole string) string {
	if analysis == nil {
		return ""
	}

	var sb strings.Builder

	if lang == "zh" {
		sb.WriteString("## 🎯 辩论依据: 历史表现分析\n\n")

		// Tailor feedback based on agent role
		if agentRole == "bull" || agentRole == "optimistic" {
			sb.WriteString("**作为乐观方，您应该关注**:\n")
			if len(analysis.SuccessPatterns) > 0 {
				sb.WriteString("✅ **成功模式** (支持您的观点):\n")
				for i, pattern := range analysis.SuccessPatterns {
					if i >= 2 {
						break
					}
					sb.WriteString(fmt.Sprintf("- %s (%.2f%% 平均盈利)\n", pattern.Description, pattern.AvgPnLPct))
				}
			}
			sb.WriteString(fmt.Sprintf("\n当前总回报: %.2f%%, 胜率: %.1f%%\n", analysis.TotalReturnPct, analysis.WinRate))
		} else if agentRole == "bear" || agentRole == "skeptical" {
			sb.WriteString("**作为谨慎方，您应该关注**:\n")
			if len(analysis.FailurePatterns) > 0 {
				sb.WriteString("⚠️ **失败模式** (支持您的警告):\n")
				for i, pattern := range analysis.FailurePatterns {
					if i >= 2 {
						break
					}
					sb.WriteString(fmt.Sprintf("- %s (%.2f%% 平均亏损)\n", pattern.Description, pattern.AvgPnLPct))
				}
			}
			sb.WriteString(fmt.Sprintf("\n最大回撤: %.1f%%, 盈利因子: %.2f\n", analysis.MaxDrawdown, analysis.ProfitFactor))
		} else {
			// Neutral/synthesis agent - sees both sides
			sb.WriteString("**综合双方观点，平衡考虑**:\n")
			sb.WriteString(fmt.Sprintf("表现: 回报 %.2f%%, 胜率 %.1f%%, 回撤 %.1f%%\n\n",
				analysis.TotalReturnPct, analysis.WinRate, analysis.MaxDrawdown))
		}
	} else {
		sb.WriteString("## 🎯 Debate Evidence: Historical Performance Analysis\n\n")

		// Tailor feedback based on agent role
		if agentRole == "bull" || agentRole == "optimistic" {
			sb.WriteString("**As the optimistic side, focus on**:\n")
			if len(analysis.SuccessPatterns) > 0 {
				sb.WriteString("✅ **Success Patterns** (support your position):\n")
				for i, pattern := range analysis.SuccessPatterns {
					if i >= 2 {
						break
					}
					sb.WriteString(fmt.Sprintf("- %s (%.2f%% avg profit)\n", pattern.Description, pattern.AvgPnLPct))
				}
			}
			sb.WriteString(fmt.Sprintf("\nCurrent total return: %.2f%%, win rate: %.1f%%\n", analysis.TotalReturnPct, analysis.WinRate))
		} else if agentRole == "bear" || agentRole == "skeptical" {
			sb.WriteString("**As the cautious side, focus on**:\n")
			if len(analysis.FailurePatterns) > 0 {
				sb.WriteString("⚠️ **Failure Patterns** (support your warnings):\n")
				for i, pattern := range analysis.FailurePatterns {
					if i >= 2 {
						break
					}
					sb.WriteString(fmt.Sprintf("- %s (%.2f%% avg loss)\n", pattern.Description, pattern.AvgPnLPct))
				}
			}
			sb.WriteString(fmt.Sprintf("\nMax drawdown: %.1f%%, profit factor: %.2f\n", analysis.MaxDrawdown, analysis.ProfitFactor))
		} else {
			// Neutral/synthesis agent - sees both sides
			sb.WriteString("**Synthesize both perspectives, balanced view**:\n")
			sb.WriteString(fmt.Sprintf("Performance: %.2f%% return, %.1f%% win rate, %.1f%% drawdown\n\n",
				analysis.TotalReturnPct, analysis.WinRate, analysis.MaxDrawdown))
		}
	}

	return sb.String()
}
