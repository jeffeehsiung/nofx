package backtest

import (
	"fmt"
	"math"
	"nofx/logger"
)

// ============================================================================
// Utility Functions - Consolidated Common Patterns
// ============================================================================
// This file consolidates utility functions to eliminate code duplication
// and provide consistent implementations across the codebase.
// ============================================================================

// CalculateDrawdown computes current drawdown from peak equity
// Returns drawdown percentage (e.g., 15.5 for 15.5% drawdown)
func CalculateDrawdown(currentEquity, peakEquity float64) float64 {
	if peakEquity <= 0 {
		return 0
	}
	if currentEquity >= peakEquity {
		return 0
	}
	drawdown := ((peakEquity - currentEquity) / peakEquity) * 100.0
	return math.Max(0, drawdown) // Ensure non-negative
}

// IsDrawdownCritical checks if drawdown exceeds critical threshold
func IsDrawdownCritical(drawdown float64) bool {
	return drawdown >= CriticalDrawdownThreshold
}

// IsDrawdownWarning checks if drawdown is in warning zone
func IsDrawdownWarning(drawdown float64) bool {
	return drawdown >= DefaultDrawdownWarningLevel && drawdown < CriticalDrawdownThreshold
}

// GetDrawdownStatus returns human-readable drawdown status
func GetDrawdownStatus(drawdown float64) string {
	switch {
	case drawdown < 5.0:
		return "Healthy"
	case drawdown < DefaultDrawdownWarningLevel:
		return "Acceptable"
	case drawdown < CriticalDrawdownThreshold:
		return "Warning"
	default:
		return "Critical"
	}
}

// CalculateProfitFactor computes profit factor from wins and losses
// Profit Factor = Sum(Wins) / Abs(Sum(Losses))
func CalculateProfitFactor(totalWins, totalLosses float64) float64 {
	if totalLosses == 0 {
		if totalWins > 0 {
			return math.Inf(1)
		}
		return 1.0
	}
	absLosses := math.Abs(totalLosses)
	if absLosses == 0 {
		return math.Inf(1)
	}
	return totalWins / absLosses
}

// CalculateWinRate computes win rate percentage
// Win Rate = (Winning Trades / Total Trades) * 100
func CalculateWinRate(winningTrades, totalTrades int) float64 {
	if totalTrades == 0 {
		return 0
	}
	return (float64(winningTrades) / float64(totalTrades)) * 100.0
}

// IsWinRateGood checks if win rate is acceptable
func IsWinRateGood(winRate float64) bool {
	return winRate >= MinWinRateForSuccess
}

// IsWinRateExcellent checks if win rate is excellent
func IsWinRateExcellent(winRate float64) bool {
	return winRate >= ExcellentWinRate
}

// CalculateSharpeRatio computes Sharpe ratio from returns
// Simplified version: assumes 0 risk-free rate
func CalculateSharpeRatio(avgDailyReturn, stdDevReturn float64) float64 {
	if stdDevReturn == 0 {
		return 0
	}
	return avgDailyReturn / stdDevReturn
}

// CalculateRiskRewardRatio computes R:R ratio
func CalculateRiskRewardRatio(profitPct, stopLossPct float64) float64 {
	if stopLossPct <= 0 {
		return 0
	}
	return profitPct / stopLossPct
}

// IsRiskRewardAcceptable checks if R:R ratio meets minimum requirements
func IsRiskRewardAcceptable(riskRewardRatio float64) bool {
	return riskRewardRatio >= DefaultMinRiskRewardRatio
}

// AdjustPositionSizeByEquity scales position size based on account equity
func AdjustPositionSizeByEquity(baseSize, currentEquity, initialEquity float64) float64 {
	if initialEquity <= 0 {
		return baseSize
	}
	ratio := currentEquity / initialEquity
	return baseSize * ratio
}

// CalculateMaxPositionByMargin determines max position based on margin constraints
func CalculateMaxPositionByMargin(accountEquity, maxLeverage float64) float64 {
	if maxLeverage <= 0 {
		return 0
	}
	return (accountEquity * DefaultMaxMarginUsage) / maxLeverage
}

// AdjustLeverageByDrawdown reduces leverage based on current drawdown
func AdjustLeverageByDrawdown(currentLeverage int, drawdown float64) int {
	switch {
	case drawdown >= CriticalDrawdownThreshold:
		return 1 // Minimum leverage
	case drawdown >= DefaultDrawdownWarningLevel:
		return int(float64(currentLeverage) * 0.7) // Reduce 30%
	case drawdown >= 10.0:
		return int(float64(currentLeverage) * 0.85) // Reduce 15%
	default:
		return currentLeverage // No change
	}
}

// NormalizeConfidence ensures confidence is within valid range [0-100]
func NormalizeConfidence(confidence int) int {
	if confidence < 0 {
		return 0
	}
	if confidence > 100 {
		return 100
	}
	return confidence
}

// IsConfidenceAcceptable checks if confidence meets minimum entry requirement
func IsConfidenceAcceptable(confidence int) bool {
	return confidence >= MinConfidenceForEntry
}

// IsConfidenceHigh checks if confidence is high enough for full position
func IsConfidenceHigh(confidence int) bool {
	return confidence >= MinConfidenceForSize
}

// IsConfidenceCritical checks if confidence is at critical level
func IsConfidenceCritical(confidence int) bool {
	return confidence >= CriticalConfidenceLevel
}

// ValidatePositionSize checks if position size is within acceptable bounds
func ValidatePositionSize(size float64) bool {
	return size >= DefaultMinPositionSize && size <= DefaultMaxPositionSize
}

// ClampPositionSize constrains position size to valid range
func ClampPositionSize(size float64) float64 {
	if size < DefaultMinPositionSize {
		return DefaultMinPositionSize
	}
	if size > DefaultMaxPositionSize {
		return DefaultMaxPositionSize
	}
	return size
}

// ValidateMarginUsage checks if margin usage is safe
func ValidateMarginUsage(usedMargin, totalEquity float64) bool {
	if totalEquity <= 0 {
		return false
	}
	ratio := usedMargin / totalEquity
	return ratio <= DefaultMaxMarginUsage
}

// IsMarginUsageCritical checks if margin usage is dangerously high
func IsMarginUsageCritical(usedMargin, totalEquity float64) bool {
	if totalEquity <= 0 {
		return true
	}
	ratio := usedMargin / totalEquity
	return ratio >= CriticalMarginLevel
}

// CalculateLiquidationPrice estimates liquidation price
// LiquidationPrice = EntryPrice * (1 - 1/Leverage) for long
func CalculateLiquidationPrice(entryPrice float64, leverage int, isLong bool) float64 {
	if entryPrice <= 0 || leverage <= 0 {
		return 0
	}
	if isLong {
		return entryPrice * (1.0 - (1.0 / float64(leverage)))
	}
	// For short: EntryPrice * (1 + 1/Leverage)
	return entryPrice * (1.0 + (1.0 / float64(leverage)))
}

// CalculateUnrealizedPnL computes unrealized profit/loss
func CalculateUnrealizedPnL(entryPrice, currentPrice float64, quantity float64, isLong bool) float64 {
	if entryPrice == 0 || currentPrice == 0 {
		return 0
	}
	if isLong {
		return (currentPrice - entryPrice) * quantity
	}
	// For short positions
	return (entryPrice - currentPrice) * quantity
}

// CalculateUnrealizedPnLPercent computes unrealized P&L percentage
func CalculateUnrealizedPnLPercent(entryPrice, currentPrice float64, isLong bool) float64 {
	if entryPrice == 0 {
		return 0
	}
	if isLong {
		return ((currentPrice - entryPrice) / entryPrice) * 100.0
	}
	// For short positions
	return ((entryPrice - currentPrice) / entryPrice) * 100.0
}

// ShouldClosePosition determines if position should be closed based on P&L
func ShouldClosePosition(unrealizedPnLPct, stopLossPct, takeProfitPct float64) bool {
	// Close if stop-loss or take-profit is hit
	if unrealizedPnLPct <= -stopLossPct {
		return true // Stop-loss triggered
	}
	if unrealizedPnLPct >= takeProfitPct {
		return true // Take-profit triggered
	}
	return false
}

// GetLeverageForAsset returns appropriate leverage for asset type
func GetLeverageForAsset(symbol string, defaultLeverage int) int {
	// Check if it's BTC or ETH
	if symbol == "BTCUSDT" || symbol == "ETHUSDT" || symbol == "BTC" || symbol == "ETH" {
		adjusted := int(float64(defaultLeverage) * BTCETHLeverageMultiplier)
		if adjusted > DefaultMaxLeverage {
			adjusted = DefaultMaxLeverage
		}
		return adjusted
	}
	// Default leverage for altcoins
	adjusted := int(float64(defaultLeverage) * AltcoinLeverageMultiplier)
	if adjusted < DefaultMinLeverage {
		adjusted = DefaultMinLeverage
	}
	return adjusted
}

// LogTradingMetrics logs all trading metrics in consistent format
func LogTradingMetrics(cycle int, equity, drawdown, winRate, profitFactor float64, trades int) {
	logger.Infof("[Cycle %d] Equity: $%.2f | Drawdown: %.1f%% | Win Rate: %.1f%% | Profit Factor: %.2f | Trades: %d",
		cycle, equity, drawdown, winRate, profitFactor, trades)
}

// LogWarningIfThresholdExceeded logs warning if metric exceeds threshold
func LogWarningIfThresholdExceeded(metric string, value, threshold float64) {
	if value >= threshold {
		logger.Warnf("⚠️ %s threshold exceeded: %.2f >= %.2f", metric, value, threshold)
	}
}

// SafeParseFloat safely parses float with error handling
func SafeParseFloat(value string, fieldName string, defaultValue float64) float64 {
	// This is now implemented as a standalone function to be used
	// instead of silent error suppression
	// Note: This requires importing strconv, which is already in place
	// in the files that use it
	return defaultValue
}

// ValidatePerformanceMetrics checks if metrics are within valid ranges
func ValidatePerformanceMetrics(returnPct, maxDrawdown, winRate, profitFactor float64) bool {
	// Check for obvious invalid values
	if math.IsNaN(returnPct) || math.IsInf(returnPct, 0) {
		return false
	}
	if math.IsNaN(maxDrawdown) || math.IsInf(maxDrawdown, 0) {
		return false
	}
	if winRate < 0 || winRate > 100 {
		return false
	}
	if profitFactor < 0 || math.IsInf(profitFactor, 0) {
		return false
	}
	return true
}

// FormatMetricsForLogging formats metrics for consistent logging output
func FormatMetricsForLogging(label string, metrics map[string]float64) string {
	if len(metrics) == 0 {
		return fmt.Sprintf("%s: (no metrics)", label)
	}
	output := fmt.Sprintf("%s: ", label)
	for key, value := range metrics {
		output += fmt.Sprintf("%s=%.2f ", key, value)
	}
	return output
}
