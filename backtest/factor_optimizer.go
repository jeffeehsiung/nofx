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
		OptimizationCycles:   10,
		ParameterSearchWidth: 0.2,
		MinTradesForUpdate:   10,
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
			MaxPositions:                 5,
			BTCETHMaxLeverage:            5,
			AltcoinMaxLeverage:           3,
			BTCETHMaxPositionValueRatio:  5.0,
			AltcoinMaxPositionValueRatio: 1.0,
			MaxMarginUsage:               0.9,
			MinPositionSize:              50.0,
			MinRiskRewardRatio:           1.5,
			MinConfidence:                65,
			DrawdownMonitoringEnabled:    true,
			DrawdownCheckInterval:        60,
			MinProfitThreshold:           5.0,
			DrawdownCloseThreshold:       40.0,
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

// OptimizeWeights analyzes feedback patterns and adjusts risk control config
func (fo *FactorOptimizer) OptimizeWeights(feedback *FeedbackAnalysis, cycle int) error {
	if feedback == nil {
		return fmt.Errorf("feedback is nil")
	}
	logger.Infof("[FactorOptimizer] 🔍 Optimizing risk control parameters at cycle %d", cycle)

	oldConfig := *fo.currentConfig
	newConfig := *fo.currentConfig
	improvements := make([]string, 0)

	// 1. Optimize leverage based on failure and success patterns
	if fo.hasPattern(feedback.FailurePatterns, "high_leverage_losses") {
		// Reduce leverage by 30%
		newConfig.BTCETHMaxLeverage = int(float64(newConfig.BTCETHMaxLeverage) * 0.7)
		newConfig.AltcoinMaxLeverage = int(float64(newConfig.AltcoinMaxLeverage) * 0.7)
		if newConfig.BTCETHMaxLeverage < 1 {
			newConfig.BTCETHMaxLeverage = 1
		}
		if newConfig.AltcoinMaxLeverage < 1 {
			newConfig.AltcoinMaxLeverage = 1
		}
		improvements = append(improvements, fmt.Sprintf("Reduced BTC/ETH leverage %d→%d, Altcoin %d→%d due to losses",
			oldConfig.BTCETHMaxLeverage, newConfig.BTCETHMaxLeverage,
			oldConfig.AltcoinMaxLeverage, newConfig.AltcoinMaxLeverage))
	} else if fo.hasPattern(feedback.SuccessPatterns, "high_leverage_success") {
		// Increase leverage by 15%
		newConfig.BTCETHMaxLeverage = int(float64(newConfig.BTCETHMaxLeverage) * 1.15)
		newConfig.AltcoinMaxLeverage = int(float64(newConfig.AltcoinMaxLeverage) * 1.15)
		improvements = append(improvements, fmt.Sprintf("Increased BTC/ETH leverage %d→%d, Altcoin %d→%d due to success",
			oldConfig.BTCETHMaxLeverage, newConfig.BTCETHMaxLeverage,
			oldConfig.AltcoinMaxLeverage, newConfig.AltcoinMaxLeverage))
	}

	// 2. Optimize position sizing based on patterns
	if fo.hasPattern(feedback.FailurePatterns, "oversized_positions") {
		newConfig.MinPositionSize = newConfig.MinPositionSize * 0.6
		if newConfig.MinPositionSize < 20 {
			newConfig.MinPositionSize = 20
		}
		newConfig.BTCETHMaxPositionValueRatio = newConfig.BTCETHMaxPositionValueRatio * 0.6
		newConfig.AltcoinMaxPositionValueRatio = newConfig.AltcoinMaxPositionValueRatio * 0.6
		improvements = append(improvements, "Reduced position sizes due to oversizing pattern")
	}

	// 3. Optimize confidence thresholds based on win rate
	if feedback.WinRate < 40 {
		// Too many losers - be more selective
		newConfig.MinConfidence = int(math.Min(float64(newConfig.MinConfidence)*1.2, 85.0))
		improvements = append(improvements, fmt.Sprintf("Increased min confidence %d→%d for better trade selection",
			oldConfig.MinConfidence, newConfig.MinConfidence))
	} else if feedback.WinRate > 65 {
		// Winning often - can be slightly less selective
		newConfig.MinConfidence = int(math.Max(float64(newConfig.MinConfidence)*0.95, 60.0))
		improvements = append(improvements, fmt.Sprintf("Decreased min confidence %d→%d to capture more opportunities",
			oldConfig.MinConfidence, newConfig.MinConfidence))
	}

	// 4. Optimize margin usage based on drawdown
	if feedback.MaxDrawdown > 20.0 { // Exceeded typical 20% drawdown limit
		newConfig.MaxMarginUsage = newConfig.MaxMarginUsage * 0.7
		if newConfig.MaxMarginUsage < 0.3 {
			newConfig.MaxMarginUsage = 0.3
		}
		improvements = append(improvements, fmt.Sprintf("Reduced max margin usage %.0f%%→%.0f%% due to high drawdown",
			oldConfig.MaxMarginUsage*100, newConfig.MaxMarginUsage*100))
	}

	// 5. Optimize max positions based on performance
	if feedback.TotalReturnPct < -10.0 && fo.currentConfig.MaxPositions > 1 {
		newConfig.MaxPositions = fo.currentConfig.MaxPositions - 1
		improvements = append(improvements, fmt.Sprintf("Reduced max positions %d→%d due to poor returns",
			oldConfig.MaxPositions, newConfig.MaxPositions))
	}

	// 6. Optimize drawdown monitoring thresholds
	if feedback.MaxDrawdown > 15.0 {
		newConfig.DrawdownMonitoringEnabled = true
		newConfig.DrawdownCheckInterval = 30 // Check more frequently
		newConfig.MinProfitThreshold = 3.0   // Lower threshold for monitoring
		improvements = append(improvements, "Enabled aggressive drawdown monitoring due to high drawdown")
	}

	// ============================================================================
	// V2 EXECUTION-LEVEL FAILURE RESPONSE
	// ============================================================================
	// Adjust parameters based on microstructure failure patterns from Trade Failure V2

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
		newConfig.MinPositionSize = newConfig.MinPositionSize * 0.85
		improvements = append(improvements,
			fmt.Sprintf("Reduced min position size by 15%% due to chasing/slippage failures"))
	}

	// Stop loss management
	if hasV2Failure("stop_too_tight") {
		// Increase min confidence to avoid tight stops on weak signals
		newConfig.MinConfidence = int(math.Min(float64(newConfig.MinConfidence)*1.1, 85.0))
		improvements = append(improvements,
			fmt.Sprintf("Increased min confidence by 10%% to avoid tight stops on weak signals"))
	}

	// Liquidity-related failures
	if hasV2Failure("liquidity_risk_high") || hasV2Failure("liquidity_dried") {
		// Reduce position sizes for liquidity-constrained trades
		newConfig.AltcoinMaxPositionValueRatio = newConfig.AltcoinMaxPositionValueRatio * 0.7
		if newConfig.AltcoinMaxPositionValueRatio < 0.3 {
			newConfig.AltcoinMaxPositionValueRatio = 0.3
		}
		improvements = append(improvements,
			"Reduced altcoin position size ratio to 0.7x due to liquidity issues")
	}

	// False breakouts and premature entries
	if countV2Failures("false_breakout_v2") > 2 || countV2Failures("premature_entry") > 2 {
		// More aggressive filtering for entry confirmation
		newConfig.MinConfidence = int(math.Min(float64(newConfig.MinConfidence)*1.15, 85.0))
		improvements = append(improvements,
			fmt.Sprintf("Increased min confidence by 15%% due to false breakout/premature entry patterns"))
	}

	// Momentum decay and late exit issues
	if hasV2Failure("momentum_decay") || hasV2Failure("late_exit_giveback") {
		// Tighten profit targets - exit earlier to avoid give-back
		improvements = append(improvements,
			"⚠️ Monitor momentum during holds - implement trailing stops to avoid give-back")
	}

	// Regime mismatch
	if countV2Failures("regime_mismatch") > 1 {
		// Already have regime checking - note for monitoring
		improvements = append(improvements,
			"Regime mismatch detected - ensure pre-entry regime checks are active")
	}

	// Stacked risk - reduce position count
	if hasV2Failure("stacked_risk") {
		if newConfig.MaxPositions > 2 {
			newConfig.MaxPositions = newConfig.MaxPositions - 1
		}
		improvements = append(improvements,
			fmt.Sprintf("Reduced max concurrent positions from %d to %d due to correlation risk",
				oldConfig.MaxPositions, newConfig.MaxPositions))
	}

	// Cost-related failures
	if hasV2Failure("funding_drag") || hasV2Failure("borrowing_cost_high") {
		// Reduce hold time / position time exposure
		improvements = append(improvements,
			"⚠️ Funding/borrowing costs detected - reduce hold time for cost-sensitive trades")
	}

	// Technical faults
	if hasV2Failure("technical_fault") {
		improvements = append(improvements,
			"⚠️ Technical faults detected - review system reliability before next trading cycle")
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

// hasPattern checks if a pattern type exists in a list of TradingPattern structs
func (fo *FactorOptimizer) hasPattern(patterns []TradingPattern, patternType string) bool {
	for _, p := range patterns {
		if p.PatternType == patternType {
			return true
		}
	}
	return false
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
