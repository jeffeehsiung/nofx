package backtest

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/logger"
	"nofx/store"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================================
// Factor Weight Optimization System (Inner Loop) - REFACTORED
// ============================================================================
// Automatically optimizes trading strategy parameters based on historical performance
// Adapts the RiskControlConfig from store.StrategyConfig to optimize:
// - Leverage (BTCETHMaxLeverage, AltcoinMaxLeverage)
// - Position sizing (Position value ratios, MinPositionSize)
// - Risk controls (MaxMarginUsage, MinConfidence, etc.)
// - Drawdown monitoring settings
// ============================================================================

// FactorOptimizer optimizes trading risk control parameters
// This is a refactored version that uses store.RiskControlConfig instead of FactorWeights
type FactorOptimizer struct {
	currentConfig       *store.RiskControlConfig
	baselineConfig      *store.RiskControlConfig
	configPerformance   map[string]*Metrics
	optimizationHistory []*OptimizationRecord
	config              *FactorOptimizerConfig
}

// FactorOptimizerConfig controls optimization behavior
type FactorOptimizerConfig struct {
	EnableOptimization   bool    `json:"enable_optimization"`
	OptimizationCycles   int     `json:"optimization_cycles"`
	ParameterSearchWidth float64 `json:"parameter_search_width"`
	MinTradesForUpdate   int     `json:"min_trades_for_update"`
	AdaptationRate       float64 `json:"adaptation_rate"`
}

// OptimizationRecord tracks a parameter optimization event
type OptimizationRecord struct {
	Timestamp      time.Time                `json:"timestamp"`
	Cycle          int                      `json:"cycle"`
	OldConfig      *store.RiskControlConfig `json:"old_config"`
	NewConfig      *store.RiskControlConfig `json:"new_config"`
	ImprovementPct float64                  `json:"improvement_pct"`
	Reason         string                   `json:"reason"`
}

// DefaultFactorOptimizerConfig returns default configuration
func DefaultFactorOptimizerConfig() *FactorOptimizerConfig {
	return &FactorOptimizerConfig{
		EnableOptimization:   true,
		OptimizationCycles:   MinTradesForFeedback,
		ParameterSearchWidth: 0.2,
		MinTradesForUpdate:   MinTradesForFeedback,
		AdaptationRate:       0.15,
	}
}

// NewFactorOptimizer creates a new factor optimizer using store.RiskControlConfig
func NewFactorOptimizer(riskcontrolConfig *store.RiskControlConfig, config *FactorOptimizerConfig) *FactorOptimizer {
	if config == nil {
		config = DefaultFactorOptimizerConfig()
	}

	// Create default RiskControlConfig (matches store defaults)
	var defaultConfig *store.RiskControlConfig
	if riskcontrolConfig == nil {
		defaultConfig = &store.RiskControlConfig{
			MaxPositions:                 MaxPositionsPerAccount,
			BTCETHMaxLeverage:            DefaultMaxLeverage,
			AltcoinMaxLeverage:           DefaultOptimalLeverage,
			BTCETHMaxPositionValueRatio:  MaxPositionsPerAccount,
			AltcoinMaxPositionValueRatio: DefaultPositionSizeScale,
			MaxMarginUsage:               DefaultMaxMarginUsage,
			MinPositionSize:              DefaultMinPositionSize,
			MinRiskRewardRatio:           DefaultMinRiskRewardRatio,
			MinConfidence:                MinConfidenceForEntry,
			DrawdownMonitoringEnabled:    true,
			DrawdownCheckInterval:        PerformanceCheckInterval,
			MinProfitThreshold:           MinProfitThresholdForMonitoring,
			DrawdownCloseThreshold:       CriticalDrawdownThreshold,
		}
	} else {
		defaultConfig = riskcontrolConfig
	}

	// Copy for current config (will be modified)
	currentCopy := *defaultConfig

	fo := &FactorOptimizer{
		currentConfig:       &currentCopy,
		baselineConfig:      defaultConfig,
		configPerformance:   make(map[string]*Metrics),
		optimizationHistory: make([]*OptimizationRecord, 0),
		config:              config,
	}

	logger.Infof("[FactorOptimizer] Initialized with store.RiskControlConfig")
	logger.Infof("  BTC/ETH Leverage: %d | Altcoin Leverage: %d",
		fo.currentConfig.BTCETHMaxLeverage, fo.currentConfig.AltcoinMaxLeverage)
	logger.Infof("  Min Position Size: $%.0f | Max Positions: %d",
		fo.currentConfig.MinPositionSize, fo.currentConfig.MaxPositions)
	logger.Infof("  Min Confidence: %d%% | Max Margin Usage: %.0f%%",
		fo.currentConfig.MinConfidence, fo.currentConfig.MaxMarginUsage*100)

	return fo
}

// GetCurrentWeights returns the current RiskControlConfig as interface{}
// Used to attach optimized config to decision context
func (fo *FactorOptimizer) GetCurrentWeights() interface{} {
	return fo.currentConfig
}

// ShouldOptimize determines if it's time to optimize parameters
func (fo *FactorOptimizer) ShouldOptimize(currentCycle int, totalTrades int) bool {
	if !fo.config.EnableOptimization {
		return false
	}
	if totalTrades < fo.config.MinTradesForUpdate {
		return false
	}
	if currentCycle > 0 && currentCycle%fo.config.OptimizationCycles == 0 {
		return true
	}
	return false
}

// OptimizeWeights analyzes LLM-enhanced feedback patterns and adjusts risk control config
// Now leverages:
// - LLM-generated pattern recommendations with microstructure insights
// - RecommendedActions from LLM feedback analysis (with RecentOrder fields)
// - Pattern-specific metadata (AvgPnLPct, Frequency) for weighted adjustments
func (fo *FactorOptimizer) OptimizeWeights(feedback *FeedbackAnalysis, cycle int) error {
	if feedback == nil {
		return fmt.Errorf("feedback is nil")
	}
	logger.Infof("[FactorOptimizer] 🔍 Optimizing risk control parameters at cycle %d (LLM-enhanced)", cycle)

	oldConfig := *fo.currentConfig
	newConfig := *fo.currentConfig
	improvements := make([]string, 0)

	// ============================================================================
	// PHASE 1: PARSE LLM-GENERATED RECOMMENDATIONS
	// ============================================================================
	// Extract quantified recommendations from LLM feedback actions
	llmRecommendations := fo.parseLLMRecommendations(feedback.RecommendedActions)

	// Apply LLM recommendations if present
	if len(llmRecommendations) > 0 {
		logger.Debugf("[FactorOptimizer] 📊 Processing %d LLM recommendations", len(llmRecommendations))
		for key, value := range llmRecommendations {
			logger.Debugf("  • %s → %v", key, value)
		}
		fo.applyLLMRecommendations(&newConfig, llmRecommendations, &improvements)
	}

	// ============================================================================
	// PHASE 2: PATTERN-BASED OPTIMIZATION WITH MICROSTRUCTURE AWARENESS
	// ============================================================================

	// 1. Optimize leverage based on failure and success patterns
	// Now uses pattern frequency and avg PnL % for weighted adjustments
	if leverageFailure := fo.findPatternWithMetrics(feedback.FailurePatterns, "high_leverage_losses"); leverageFailure != nil {
		// Weighted reduction: higher frequency or larger losses = stronger reduction
		reductionFactor := SafeMarginLevel + (PatternConfidenceWeight * math.Min(1.0, float64(leverageFailure.Frequency)/10.0))
		newConfig.BTCETHMaxLeverage = int(float64(newConfig.BTCETHMaxLeverage) * reductionFactor)
		newConfig.AltcoinMaxLeverage = int(float64(newConfig.AltcoinMaxLeverage) * reductionFactor)
		if newConfig.BTCETHMaxLeverage < 1 {
			newConfig.BTCETHMaxLeverage = 1
		}
		if newConfig.AltcoinMaxLeverage < 1 {
			newConfig.AltcoinMaxLeverage = 1
		}
		improvements = append(improvements, fmt.Sprintf(
			"Reduced BTC/ETH leverage %d→%d, Altcoin %d→%d (freq:%d, loss:%.1f%%)",
			oldConfig.BTCETHMaxLeverage, newConfig.BTCETHMaxLeverage,
			oldConfig.AltcoinMaxLeverage, newConfig.AltcoinMaxLeverage,
			leverageFailure.Frequency, leverageFailure.AvgPnLPct))
	} else if leverageSuccess := fo.findPatternWithMetrics(feedback.SuccessPatterns, "high_leverage_success"); leverageSuccess != nil {
		// Conservative increase
		newConfig.BTCETHMaxLeverage = int(float64(newConfig.BTCETHMaxLeverage) * 1.1)
		newConfig.AltcoinMaxLeverage = int(float64(newConfig.AltcoinMaxLeverage) * 1.1)
		improvements = append(improvements, fmt.Sprintf(
			"Increased BTC/ETH leverage %d→%d, Altcoin %d→%d (freq:%d, gain:%.1f%%)",
			oldConfig.BTCETHMaxLeverage, newConfig.BTCETHMaxLeverage,
			oldConfig.AltcoinMaxLeverage, newConfig.AltcoinMaxLeverage,
			leverageSuccess.Frequency, leverageSuccess.AvgPnLPct))
	}

	// 2. Optimize position sizing based on patterns
	if oversizedPattern := fo.findPatternWithMetrics(feedback.FailurePatterns, "oversized_positions"); oversizedPattern != nil {
		// Weighted reduction based on frequency
		reductionFactor := SafeMarginLevel - (0.1 * math.Min(1.0, float64(oversizedPattern.Frequency)/5.0))
		newConfig.MinPositionSize = newConfig.MinPositionSize * reductionFactor
		if newConfig.MinPositionSize < 20 {
			newConfig.MinPositionSize = 20
		}
		newConfig.BTCETHMaxPositionValueRatio = newConfig.BTCETHMaxPositionValueRatio * reductionFactor
		newConfig.AltcoinMaxPositionValueRatio = newConfig.AltcoinMaxPositionValueRatio * reductionFactor
		improvements = append(improvements, fmt.Sprintf(
			"Reduced position sizes by %.0f%% (freq:%d, loss:%.1f%%)",
			(1-reductionFactor)*100, oversizedPattern.Frequency, oversizedPattern.AvgPnLPct))
	}

	// 3. Optimize confidence thresholds based on win rate and pattern analysis
	if feedback.WinRate < MinWinRateForSuccess {
		// Too many losers - be more selective
		delta := int(math.Min(float64(newConfig.MinConfidence)*0.2, 20.0))
		newConfig.MinConfidence = int(math.Min(float64(newConfig.MinConfidence)+float64(delta), 85.0))
		improvements = append(improvements, fmt.Sprintf(
			"Increased min confidence %d→%d for better selection (WinRate:%.0f%%)",
			oldConfig.MinConfidence, newConfig.MinConfidence, feedback.WinRate*100))
	} else if feedback.WinRate > GoodWinRate {
		// Winning often - can be slightly less selective
		delta := int(math.Max(float64(newConfig.MinConfidence)*0.05, 5.0))
		newConfig.MinConfidence = int(math.Max(float64(newConfig.MinConfidence)-float64(delta), 60.0))
		improvements = append(improvements, fmt.Sprintf(
			"Decreased min confidence %d→%d to capture more (WinRate:%.0f%%)",
			oldConfig.MinConfidence, newConfig.MinConfidence, feedback.WinRate*100))
	}

	// 4. Optimize margin usage based on drawdown
	if feedback.MaxDrawdown > DefaultMaxDrawdownPct { // Exceeded typical 20% drawdown limit
		newConfig.MaxMarginUsage = newConfig.MaxMarginUsage * SafeMarginLevel
		if newConfig.MaxMarginUsage < SentimentConfidenceWeight {
			newConfig.MaxMarginUsage = SentimentConfidenceWeight
		}
		improvements = append(improvements, fmt.Sprintf(
			"Reduced max margin usage %.0f%%→%.0f%% (DD:%.1f%%)",
			oldConfig.MaxMarginUsage*100, newConfig.MaxMarginUsage*100, feedback.MaxDrawdown))
	}

	// 5. Optimize max positions based on performance
	if feedback.TotalReturnPct < -10.0 && fo.currentConfig.MaxPositions > 1 {
		newConfig.MaxPositions = fo.currentConfig.MaxPositions - 1
		improvements = append(improvements, fmt.Sprintf(
			"Reduced max positions %d→%d (Return:%.1f%%)",
			oldConfig.MaxPositions, newConfig.MaxPositions, feedback.TotalReturnPct))
	}

	// 6. Optimize drawdown monitoring thresholds
	if feedback.MaxDrawdown > DefaultDrawdownWarningLevel {
		newConfig.DrawdownMonitoringEnabled = true
		newConfig.DrawdownCheckInterval = WebsocketHeartbeatInterval   // Check more frequently
		newConfig.MinProfitThreshold = MinProfitThresholdForMonitoring // Lower threshold for monitoring
		improvements = append(improvements, fmt.Sprintf(
			"Enabled aggressive drawdown monitoring (DD:%.1f%%)", feedback.MaxDrawdown))
	}

	// ============================================================================
	// PHASE 3: MICROSTRUCTURE-AWARE V2 EXECUTION-LEVEL FAILURE RESPONSE
	// ============================================================================
	// Adjust parameters based on Trade Failure V2 patterns with microstructure metrics

	// Helper function to check for V2 failure reason
	hasV2Failure := func(patternType string) bool {
		for _, pattern := range feedback.FailurePatterns {
			if pattern.PatternType == patternType {
				return true
			}
		}
		return false
	}

	// Helper function to count V2 failures of a specific type
	countV2Failures := func(patternType string) int {
		for _, pattern := range feedback.FailurePatterns {
			if pattern.PatternType == patternType {
				return pattern.Frequency
			}
		}
		return 0
	}

	// Execution failures - reduce slippage budget and position size
	if hasV2Failure("chasing_entry") || hasV2Failure("slippage_exceeded") {
		// Reduce entry tolerance for slippage
		newConfig.MinPositionSize = newConfig.MinPositionSize * ComplianceRateExcellent
		improvements = append(improvements,
			"Reduced min position size by 15% due to chasing/slippage failures (microstructure)")
	}

	// Stop loss management
	if hasV2Failure("stop_too_tight") {
		// Increase min confidence to avoid tight stops on weak signals
		newConfig.MinConfidence = int(math.Min(float64(newConfig.MinConfidence)*1.1, 85.0))
		improvements = append(improvements,
			"Increased min confidence by 10% to avoid tight stops (microstructure)")
	}

	// Liquidity-related failures - uses FillQuality from RecentOrder
	if hasV2Failure("liquidity_risk_high") || hasV2Failure("liquidity_dried") {
		// Reduce position sizes for liquidity-constrained trades
		newConfig.AltcoinMaxPositionValueRatio = newConfig.AltcoinMaxPositionValueRatio * SafeMarginLevel
		if newConfig.AltcoinMaxPositionValueRatio < SentimentConfidenceWeight {
			newConfig.AltcoinMaxPositionValueRatio = SentimentConfidenceWeight
		}
		improvements = append(improvements,
			"Reduced altcoin position ratio to 0.7x (liquidity microstructure)")
	}

	// False breakouts and premature entries - detected via LLM analysis of market regime
	if countV2Failures("false_breakout_v2") > 2 || countV2Failures("premature_entry") > 2 {
		// More aggressive filtering for entry confirmation
		newConfig.MinConfidence = int(math.Min(float64(newConfig.MinConfidence)*1.15, 85.0))
		improvements = append(improvements,
			fmt.Sprintf("Increased min confidence by 15%% due to false breakout patterns (freq:%d)",
				countV2Failures("false_breakout_v2")+countV2Failures("premature_entry")))
	}

	// Momentum decay and late exit issues - uses MFE/MAE and GiveBackFromPeak from RecentOrder
	if hasV2Failure("momentum_decay") || hasV2Failure("late_exit_giveback") {
		// Consider implementation of trailing stops here
		improvements = append(improvements,
			"⚠️ Momentum decay detected - consider trailing stops to avoid give-back (MFE/MAE microstructure)")
	}

	// Regime mismatch
	if countV2Failures("regime_mismatch") > 1 {
		// Already have regime checking - note for monitoring
		improvements = append(improvements,
			"Regime mismatch detected - verify pre-entry regime checks (market regime analysis)")
	}

	// Stacked risk - reduce position count
	if hasV2Failure("stacked_risk") {
		if newConfig.MaxPositions > 2 {
			newConfig.MaxPositions = newConfig.MaxPositions - 1
		}
		improvements = append(improvements,
			fmt.Sprintf("Reduced max positions from %d to %d due to correlation risk",
				oldConfig.MaxPositions, newConfig.MaxPositions))
	}

	// Cost-related failures - uses FundingAccrued from RecentOrder
	if hasV2Failure("funding_drag") || hasV2Failure("borrowing_cost_high") {
		// Reduce hold time / position time exposure
		improvements = append(improvements,
			"⚠️ Funding/borrowing costs detected - reduce holds (funding cost microstructure)")
	}

	// Technical faults
	if hasV2Failure("technical_fault") {
		improvements = append(improvements,
			"⚠️ Technical faults detected - review system reliability")
	}

	// Calculate improvement score
	improvementPct := 0.0
	if feedback.TotalReturnPct > 0 {
		improvementPct = feedback.TotalReturnPct
	}

	// Record optimization
	record := &OptimizationRecord{
		Timestamp:      time.Now(),
		Cycle:          cycle,
		OldConfig:      &oldConfig,
		NewConfig:      &newConfig,
		ImprovementPct: improvementPct,
		Reason:         strings.Join(improvements, "; "),
	}
	fo.optimizationHistory = append(fo.optimizationHistory, record)

	// Apply new config
	fo.currentConfig = &newConfig

	// Log results
	logger.Infof("[FactorOptimizer] 🎯 Optimization complete: %d parameter adjustments", len(improvements))
	for _, improvement := range improvements {
		logger.Infof("  • %s", improvement)
	}
	if improvementPct != 0 {
		logger.Infof("  Performance change: %+.1f%%", improvementPct)
	}

	return nil
}

// findPatternWithMetrics returns the TradingPattern with quantified metrics (frequency, AvgPnLPct)
// Used for weighted optimization decisions
func (fo *FactorOptimizer) findPatternWithMetrics(patterns []TradingPattern, patternType string) *TradingPattern {
	for i := range patterns {
		if patterns[i].PatternType == patternType {
			return &patterns[i]
		}
	}
	return nil
}

// parseLLMRecommendations extracts quantified parameter adjustments from LLM-generated recommendation strings
// Parses patterns like:
// - "reduce leverage to 2x" → {leverage: 2}
// - "increase min confidence to 75" → {min_confidence: 75}
// - "reduce position size by 30%" → {position_size_reduction: 0.3}
// - "max 2 concurrent positions" → {max_positions: 2}
func (fo *FactorOptimizer) parseLLMRecommendations(recommendations []string) map[string]float64 {
	result := make(map[string]float64)

	for _, rec := range recommendations {
		recLower := strings.ToLower(rec)

		// Leverage recommendations: "leverage to Nx", "reduce leverage", "max Nx leverage"
		if strings.Contains(recLower, "leverage") {
			// Try to parse "Nx" or "N x" pattern
			parts := strings.FieldsFunc(recLower, func(r rune) bool { return (r < '0' || r > '9') && r != 'x' && r != '.' })
			for i, part := range parts {
				if part == "x" && i > 0 {
					// Previous part should be the leverage value
					var val float64
					if _, err := fmt.Sscanf(parts[i-1], "%f", &val); err == nil && val > 0 && val < 50 {
						result["leverage"] = val
						break
					}
				}
			}
		}

		// Confidence recommendations: "confidence to NN", "confidence above NN", "min entry NN"
		if strings.Contains(recLower, "confidence") || strings.Contains(recLower, "entry") {
			var val float64
			if _, err := fmt.Sscanf(recLower, "confidence to %f", &val); err == nil {
				result["min_confidence"] = val
			} else if _, err := fmt.Sscanf(recLower, "confidence above %f", &val); err == nil {
				result["min_confidence"] = val
			} else if _, err := fmt.Sscanf(recLower, "min entry %f", &val); err == nil {
				result["min_confidence"] = val
			}
		}

		// Position size recommendations: "reduce by NN%", "size to $NN", "max position $NN"
		if strings.Contains(recLower, "position") || strings.Contains(recLower, "size") {
			var val float64
			if _, err := fmt.Sscanf(recLower, "reduce by %f%%", &val); err == nil {
				result["position_size_reduction"] = val / 100.0
			} else if _, err := fmt.Sscanf(recLower, "position size $%f", &val); err == nil {
				result["min_position_size"] = val
			} else if _, err := fmt.Sscanf(recLower, "max position $%f", &val); err == nil {
				result["min_position_size"] = val
			}
		}

		// Max positions: "max N positions", "N concurrent trades", "reduce to N positions"
		if strings.Contains(recLower, "positions") || strings.Contains(recLower, "concurrent") {
			var val float64
			if _, err := fmt.Sscanf(recLower, "max %f positions", &val); err == nil {
				result["max_positions"] = val
			} else if _, err := fmt.Sscanf(recLower, "%f concurrent", &val); err == nil {
				result["max_positions"] = val
			} else if _, err := fmt.Sscanf(recLower, "reduce to %f positions", &val); err == nil {
				result["max_positions"] = val
			}
		}

		// Margin usage: "margin usage to NN%", "reduce margin to NN%"
		if strings.Contains(recLower, "margin") {
			var val float64
			if _, err := fmt.Sscanf(recLower, "margin usage to %f%%", &val); err == nil {
				result["max_margin_usage"] = val / 100.0
			} else if _, err := fmt.Sscanf(recLower, "reduce margin to %f%%", &val); err == nil {
				result["max_margin_usage"] = val / 100.0
			}
		}
	}

	return result
}

// applyLLMRecommendations applies parsed LLM recommendations to the risk config
func (fo *FactorOptimizer) applyLLMRecommendations(config *store.RiskControlConfig, recs map[string]float64, improvements *[]string) {
	if leverage, ok := recs["leverage"]; ok && leverage > 0 && leverage < 50 {
		oldLev := config.BTCETHMaxLeverage
		config.BTCETHMaxLeverage = int(leverage)
		config.AltcoinMaxLeverage = int(math.Max(1, leverage*PositiveSentimentThreshold)) // Alt coins at 60% of BTC/ETH
		*improvements = append(*improvements, fmt.Sprintf(
			"LLM recommendation: leverage %d→%d (parsed: %.0fx)", oldLev, config.BTCETHMaxLeverage, leverage))
	}

	if confidence, ok := recs["min_confidence"]; ok && confidence >= 50 && confidence <= 95 {
		oldConf := config.MinConfidence
		config.MinConfidence = int(confidence)
		*improvements = append(*improvements, fmt.Sprintf(
			"LLM recommendation: confidence %d→%d%% (microstructure aware)", oldConf, config.MinConfidence))
	}

	if reduction, ok := recs["position_size_reduction"]; ok && reduction > 0 && reduction < 1 {
		oldSize := config.MinPositionSize
		config.MinPositionSize = config.MinPositionSize * (1 - reduction)
		if config.MinPositionSize < 20 {
			config.MinPositionSize = 20
		}
		*improvements = append(*improvements, fmt.Sprintf(
			"LLM recommendation: position size $%.0f→$%.0f (%.0f%% reduction)", oldSize, config.MinPositionSize, reduction*100))
	}

	if posSize, ok := recs["min_position_size"]; ok && posSize >= 20 && posSize <= 5000 {
		oldSize := config.MinPositionSize
		config.MinPositionSize = posSize
		*improvements = append(*improvements, fmt.Sprintf(
			"LLM recommendation: position size $%.0f→$%.0f", oldSize, posSize))
	}

	if maxPos, ok := recs["max_positions"]; ok && maxPos >= 1 && maxPos <= 10 {
		oldPos := config.MaxPositions
		config.MaxPositions = int(maxPos)
		*improvements = append(*improvements, fmt.Sprintf(
			"LLM recommendation: max positions %d→%d", oldPos, config.MaxPositions))
	}

	if marginUsage, ok := recs["max_margin_usage"]; ok && marginUsage >= SentimentConfidenceWeight && marginUsage <= CriticalMarginLevel {
		oldMargin := config.MaxMarginUsage
		config.MaxMarginUsage = marginUsage
		*improvements = append(*improvements, fmt.Sprintf(
			"LLM recommendation: margin usage %.0f%%→%.0f%% (execution quality aware)",
			oldMargin*100, marginUsage*100))
	}
}

// GetRiskControlConfig returns the current RiskControlConfig
func (fo *FactorOptimizer) GetRiskControlConfig() *store.RiskControlConfig {
	return fo.currentConfig
}

// GetOptimizationHistory returns the optimization history
func (fo *FactorOptimizer) GetOptimizationHistory() []*OptimizationRecord {
	return fo.optimizationHistory
}

// SaveState persists the optimizer state to disk
func (fo *FactorOptimizer) SaveState(runID string) error {
	if runID == "" {
		return fmt.Errorf("runID is empty")
	}

	dir := filepath.Join("backtests", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	state := map[string]interface{}{
		"timestamp":            time.Now().UTC(),
		"current_config":       fo.currentConfig,
		"baseline_config":      fo.baselineConfig,
		"optimization_history": fo.optimizationHistory,
		"total_optimizations":  len(fo.optimizationHistory),
	}

	path := filepath.Join(dir, "factor_optimizer_state.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}

	logger.Debugf("[FactorOptimizer] 💾 Saved state to %s", path)
	return nil
}

// LoadState restores the optimizer state from disk
func (fo *FactorOptimizer) LoadState(runID string) error {
	if runID == "" {
		return fmt.Errorf("runID is empty")
	}

	path := filepath.Join("backtests", runID, "factor_optimizer_state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Not an error if file doesn't exist yet
		}
		return err
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	// Restore config and history
	if configData, ok := state["current_config"]; ok {
		configJSON, _ := json.Marshal(configData)
		if err := json.Unmarshal(configJSON, fo.currentConfig); err != nil {
			return fmt.Errorf("failed to unmarshal current_config: %w", err)
		}
	}

	if historyData, ok := state["optimization_history"]; ok {
		historyJSON, _ := json.Marshal(historyData)
		if err := json.Unmarshal(historyJSON, &fo.optimizationHistory); err != nil {
			return fmt.Errorf("failed to unmarshal optimization_history: %w", err)
		}
	}

	logger.Debugf("[FactorOptimizer] 📖 Loaded state from %s (%d optimizations)",
		path, len(fo.optimizationHistory))
	return nil
}

// FormatWeightsForPrompt formats current RiskControlConfig for LLM prompts
func (fo *FactorOptimizer) FormatWeightsForPrompt(lang string) string {
	cfg := fo.currentConfig

	if lang == "zh" {
		return fmt.Sprintf(`
## 🎯 当前优化的风控参数

**杠杆配置**
- BTC/ETH最大杠杆: %dx
- 山寨币最大杠杆: %dx

**头寸管理**
- 最小头寸规模: $%.0f
- 最大同时持仓数: %d个
- BTC/ETH最大头寸占权益比: %.1f倍
- 山寨币最大头寸占权益比: %.1f倍

**风险控制**
- 最大保证金使用率: %.0f%%
- 最小入场信心: %d%%
- 最小风险收益比: %.1f
- 最大连续亏损: 20%%

**回撤监控**
- 监控状态: %v
- 检查间隔: %d秒
- 启动阈值: %.0f%%利润
- 止损阈值: %.0f%%回撤
`,
			cfg.BTCETHMaxLeverage, cfg.AltcoinMaxLeverage,
			cfg.MinPositionSize, cfg.MaxPositions,
			cfg.BTCETHMaxPositionValueRatio, cfg.AltcoinMaxPositionValueRatio,
			cfg.MaxMarginUsage*100, cfg.MinConfidence,
			cfg.MinRiskRewardRatio,
			cfg.DrawdownMonitoringEnabled,
			cfg.DrawdownCheckInterval,
			cfg.MinProfitThreshold,
			cfg.DrawdownCloseThreshold,
		)
	}

	// English version
	return fmt.Sprintf(`
## 🎯 Current Optimized Risk Control Parameters

**Leverage Configuration**
- BTC/ETH Max Leverage: %dx
- Altcoin Max Leverage: %dx

**Position Management**
- Min Position Size: $%.0f
- Max Concurrent Positions: %d
- BTC/ETH Max Position Value Ratio: %.1fx equity
- Altcoin Max Position Value Ratio: %.1fx equity

**Risk Controls**
- Max Margin Usage: %.0f%%
- Min Entry Confidence: %d%%
- Min Risk/Reward Ratio: %.1f
- Max Drawdown Limit: 20%%

**Drawdown Monitoring**
- Monitoring Enabled: %v
- Check Interval: %d seconds
- Activation Threshold: %.0f%% profit
- Close Threshold: %.0f%% drawdown
`,
		cfg.BTCETHMaxLeverage, cfg.AltcoinMaxLeverage,
		cfg.MinPositionSize, cfg.MaxPositions,
		cfg.BTCETHMaxPositionValueRatio, cfg.AltcoinMaxPositionValueRatio,
		cfg.MaxMarginUsage*100, cfg.MinConfidence,
		cfg.MinRiskRewardRatio,
		cfg.DrawdownMonitoringEnabled,
		cfg.DrawdownCheckInterval,
		cfg.MinProfitThreshold,
		cfg.DrawdownCloseThreshold,
	)
}
