package decision

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// ThresholdCalibrator learns optimal thresholds from historical trade data
type ThresholdCalibrator struct {
	// Volume/OI thresholds for entry quality
	WeakVolumeThreshold      float64
	WeakOIThreshold          float64
	PrematureVolumeThreshold float64
	PrematureOIThreshold     float64

	// Momentum decay thresholds
	VolumeDecayThreshold float64
	OIDecayThreshold     float64

	// Liquidity deterioration thresholds
	SpreadWorseningMultiple float64
	DepthReductionThreshold float64

	// Sample size for calibration
	SampleSize int
}

// NewThresholdCalibrator creates a calibrator with default values
func NewThresholdCalibrator() *ThresholdCalibrator {
	return &ThresholdCalibrator{
		// Conservative defaults (will be overridden by calibration)
		WeakVolumeThreshold:      0.90,
		WeakOIThreshold:          0.30,
		PrematureVolumeThreshold: 0.90,
		PrematureOIThreshold:     0.50,
		VolumeDecayThreshold:     -0.30,
		OIDecayThreshold:         -0.20,
		SpreadWorseningMultiple:  2.0,
		DepthReductionThreshold:  0.50,
		SampleSize:               0,
	}
}

// TradeOutcome represents a completed trade with all relevant metrics
type TradeOutcome struct {
	Symbol     string
	Profitable bool // True if PnL > 0

	// Entry metrics
	VolumeAtEntry float64
	OIAtEntry     float64

	// During-trade metrics
	VolumeDuringTrade float64
	OIDuringTrade     float64

	// Liquidity metrics
	EntrySpread float64
	ExitSpread  float64
	EntryDepth  float64
	ExitDepth   float64

	// Additional context
	HoldingMinutes int
	PnLPct         float64
}

// CalibrateFromHistory learns optimal thresholds from historical trades
// Uses ROC (Receiver Operating Characteristic) analysis to find thresholds
// that maximize separation between winning and losing trades
func (c *ThresholdCalibrator) CalibrateFromHistory(trades []TradeOutcome) error {
	if len(trades) < 30 {
		return fmt.Errorf("insufficient data: need at least 30 trades, got %d", len(trades))
	}

	c.SampleSize = len(trades)

	// Separate winning and losing trades
	var winners, losers []TradeOutcome
	for _, t := range trades {
		if t.Profitable {
			winners = append(winners, t)
		} else {
			losers = append(losers, t)
		}
	}

	if len(winners) == 0 || len(losers) == 0 {
		return fmt.Errorf("need both winning and losing trades for calibration")
	}

	// Calibrate each threshold using statistical separation
	c.WeakVolumeThreshold = c.findOptimalThreshold(trades, func(t TradeOutcome) float64 {
		return t.VolumeAtEntry
	}, true) // Lower volume = worse

	c.WeakOIThreshold = c.findOptimalThreshold(trades, func(t TradeOutcome) float64 {
		return t.OIAtEntry
	}, true)

	c.PrematureVolumeThreshold = c.WeakVolumeThreshold // Same logic

	c.PrematureOIThreshold = c.findOptimalThreshold(trades, func(t TradeOutcome) float64 {
		return t.OIAtEntry
	}, true) * 1.5 // Slightly more permissive

	c.VolumeDecayThreshold = c.findOptimalThreshold(trades, func(t TradeOutcome) float64 {
		return t.VolumeDuringTrade
	}, true)

	c.OIDecayThreshold = c.findOptimalThreshold(trades, func(t TradeOutcome) float64 {
		return t.OIDuringTrade
	}, true)

	// For spread worsening, find the multiple that best separates outcomes
	spreadRatios := make([]float64, 0)
	for _, t := range losers {
		if t.EntrySpread > 0 && t.ExitSpread > t.EntrySpread {
			spreadRatios = append(spreadRatios, t.ExitSpread/t.EntrySpread)
		}
	}
	if len(spreadRatios) > 0 {
		c.SpreadWorseningMultiple = c.percentile(spreadRatios, 0.25) // 25th percentile of losers
	}

	// For depth shrinkage, find the ratio that best separates outcomes
	depthRatios := make([]float64, 0)
	for _, t := range losers {
		if t.EntryDepth > 0 && t.ExitDepth < t.EntryDepth {
			depthRatios = append(depthRatios, t.ExitDepth/t.EntryDepth)
		}
	}
	if len(depthRatios) > 0 {
		c.DepthReductionThreshold = c.percentile(depthRatios, 0.75) // 75th percentile of losers
	}

	return nil
}

// findOptimalThreshold finds the threshold value that maximizes predictive power
// using Youden's J statistic (sensitivity + specificity - 1)
func (c *ThresholdCalibrator) findOptimalThreshold(
	trades []TradeOutcome,
	extractMetric func(TradeOutcome) float64,
	lowerIsBetter bool,
) float64 {
	// Extract all metric values
	values := make([]float64, len(trades))
	for i, t := range trades {
		values[i] = extractMetric(t)
	}

	// Sort values
	sort.Float64s(values)

	// Try different threshold candidates (percentiles)
	bestThreshold := values[len(values)/2] // Default: median
	bestScore := 0.0

	percentiles := []float64{0.1, 0.15, 0.2, 0.25, 0.3, 0.35, 0.4, 0.45, 0.5, 0.55, 0.6, 0.65, 0.7, 0.75, 0.8}
	for _, p := range percentiles {
		threshold := c.percentile(values, p)

		// Calculate true positive rate (sensitivity) and true negative rate (specificity)
		truePos, falsePos, trueNeg, falseNeg := 0, 0, 0, 0

		for _, t := range trades {
			value := extractMetric(t)
			var positive bool
			if lowerIsBetter {
				positive = value < threshold // Metric below threshold = problem detected
			} else {
				positive = value > threshold // Metric above threshold = problem detected
			}

			if positive && !t.Profitable {
				truePos++ // Correctly predicted loser
			} else if positive && t.Profitable {
				falsePos++ // Incorrectly flagged winner
			} else if !positive && t.Profitable {
				trueNeg++ // Correctly predicted winner
			} else {
				falseNeg++ // Missed loser
			}
		}

		// Youden's J statistic
		sensitivity := 0.0
		specificity := 0.0
		if truePos+falseNeg > 0 {
			sensitivity = float64(truePos) / float64(truePos+falseNeg)
		}
		if trueNeg+falsePos > 0 {
			specificity = float64(trueNeg) / float64(trueNeg+falsePos)
		}

		score := sensitivity + specificity - 1.0

		if score > bestScore {
			bestScore = score
			bestThreshold = threshold
		}
	}

	return bestThreshold
}

// percentile calculates the p-th percentile of sorted values
func (c *ThresholdCalibrator) percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	index := p * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return sorted[lower]
	}

	// Linear interpolation
	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// GetCalibrationSummary returns a human-readable summary of calibrated thresholds
func (c *ThresholdCalibrator) GetCalibrationSummary() string {
	return fmt.Sprintf(
		"Threshold Calibration (from %d trades):\n"+
			"  Entry Quality:\n"+
			"    - Weak Volume: < %.2f (%.0f%%)\n"+
			"    - Weak OI: < %.2f (%.0f%%)\n"+
			"    - Low Volume: < %.2f (%.0f%%)\n"+
			"    - Low OI: < %.2f (%.0f%%)\n"+
			"  Momentum Decay:\n"+
			"    - Volume Drop: < %.2f (%.0f%% decline)\n"+
			"    - OI Drop: < %.2f (%.0f%% decline)\n"+
			"  Liquidity:\n"+
			"    - Spread Worsening: > %.1fx\n"+
			"    - Depth Shrinkage: < %.2f (%.0f%% remaining)\n",
		c.SampleSize,
		c.WeakVolumeThreshold, c.WeakVolumeThreshold*100,
		c.WeakOIThreshold, c.WeakOIThreshold*100,
		c.PrematureVolumeThreshold, c.PrematureVolumeThreshold*100,
		c.PrematureOIThreshold, c.PrematureOIThreshold*100,
		c.VolumeDecayThreshold, c.VolumeDecayThreshold*100,
		c.OIDecayThreshold, c.OIDecayThreshold*100,
		c.SpreadWorseningMultiple,
		c.DepthReductionThreshold, c.DepthReductionThreshold*100,
	)
}

// ApplyToAnalyzer updates a failure analyzer with calibrated thresholds
func (c *ThresholdCalibrator) ApplyToAnalyzer() FailureThresholds {
	return FailureThresholds{
		WeakVolumeThreshold:      c.WeakVolumeThreshold,
		WeakOIThreshold:          c.WeakOIThreshold,
		PrematureVolumeThreshold: c.PrematureVolumeThreshold,
		PrematureOIThreshold:     c.PrematureOIThreshold,
		VolumeDecayThreshold:     c.VolumeDecayThreshold,
		OIDecayThreshold:         c.OIDecayThreshold,
		SpreadWorseningMultiple:  c.SpreadWorseningMultiple,
		DepthReductionThreshold:  c.DepthReductionThreshold,
	}
}

// FailureThresholds holds all calibrated thresholds for failure analysis
type FailureThresholds struct {
	WeakVolumeThreshold      float64
	WeakOIThreshold          float64
	PrematureVolumeThreshold float64
	PrematureOIThreshold     float64
	VolumeDecayThreshold     float64
	OIDecayThreshold         float64
	SpreadWorseningMultiple  float64
	DepthReductionThreshold  float64
}

// DefaultFailureThresholds returns conservative default thresholds
func DefaultFailureThresholds() FailureThresholds {
	return FailureThresholds{
		WeakVolumeThreshold:      0.90,
		WeakOIThreshold:          0.30,
		PrematureVolumeThreshold: 0.90,
		PrematureOIThreshold:     0.50,
		VolumeDecayThreshold:     -0.30,
		OIDecayThreshold:         -0.20,
		SpreadWorseningMultiple:  2.0,
		DepthReductionThreshold:  0.50,
	}
}

// ToCalibratedThresholds converts calibrator state to persistable format with metadata
func (c *ThresholdCalibrator) ToCalibratedThresholds() *CalibratedThresholds {
	return &CalibratedThresholds{
		WeakVolumeThreshold:      c.WeakVolumeThreshold,
		WeakOIThreshold:          c.WeakOIThreshold,
		PrematureVolumeThreshold: c.PrematureVolumeThreshold,
		PrematureOIThreshold:     c.PrematureOIThreshold,
		VolumeDecayThreshold:     c.VolumeDecayThreshold,
		OIDecayThreshold:         c.OIDecayThreshold,
		SpreadWorseningMultiple:  c.SpreadWorseningMultiple,
		DepthReductionThreshold:  c.DepthReductionThreshold,
		CalibratedAt:             time.Now().Format(time.RFC3339),
		SampleSize:               c.SampleSize,
	}
}

// FormatThresholdsForPrompt returns a formatted string of learned thresholds for LLM
func (c *ThresholdCalibrator) FormatThresholdsForPrompt(lang string) string {
	if c.SampleSize == 0 {
		return "" // Not calibrated yet
	}

	if lang == "zh" {
		return fmt.Sprintf(`## 📊 学习到的风险阈值 (基于 %d 笔交易)

**入场质量检测**:
- 弱成交量警戒线: %.2f (低于此值表示成交量不足)
- 弱持仓量警戒线: %.2f (低于此值表示持仓兴趣不足)
- 过早入场成交量: %.2f (确认前的最小成交量)
- 过早入场持仓量: %.2f (确认前的最小持仓量)

**持仓期间监控**:
- 成交量衰减警戒: %.2f (成交量下降超过此比例则动量衰减)
- 持仓量衰减警戒: %.2f (持仓量下降超过此比例则兴趣减弱)

**流动性监控**:
- 价差恶化倍数: %.2fx (价差扩大超过此倍数则流动性恶化)
- 深度缩减阈值: %.2f (深度缩减低于此比例则流动性枯竭)

💡 这些阈值是从历史交易数据中学习得出，帮助识别潜在的失败交易。
`,
			c.SampleSize,
			c.WeakVolumeThreshold, c.WeakOIThreshold,
			c.PrematureVolumeThreshold, c.PrematureOIThreshold,
			c.VolumeDecayThreshold, c.OIDecayThreshold,
			c.SpreadWorseningMultiple, c.DepthReductionThreshold,
		)
	}

	// English
	return fmt.Sprintf(`## 📊 Learned Risk Thresholds (from %d trades)

**Entry Quality Detection**:
- Weak Volume Alert: %.2f (below this = insufficient volume)
- Weak OI Alert: %.2f (below this = insufficient position interest)
- Premature Entry Volume: %.2f (minimum volume before confirmation)
- Premature Entry OI: %.2f (minimum OI before confirmation)

**During-Trade Monitoring**:
- Volume Decay Alert: %.2f (decline beyond this = momentum decay)
- OI Decay Alert: %.2f (decline beyond this = interest weakening)

**Liquidity Monitoring**:
- Spread Worsening Multiple: %.2fx (spread widens beyond this = liquidity deteriorating)
- Depth Reduction Threshold: %.2f (depth falls below this = liquidity dried)

💡 These thresholds were learned from historical trade data to help identify potential failing trades.
`,
		c.SampleSize,
		c.WeakVolumeThreshold, c.WeakOIThreshold,
		c.PrematureVolumeThreshold, c.PrematureOIThreshold,
		c.VolumeDecayThreshold, c.OIDecayThreshold,
		c.SpreadWorseningMultiple, c.DepthReductionThreshold,
	)
}
