package decision

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"gonum.org/v1/gonum/stat/distuv"
)

type BayesianThreshold struct {
	PriorAlpha       float64
	PriorBeta        float64
	PosteriorAlpha   float64
	PosteriorBeta    float64
	CurrentThreshold float64
	Confidence       float64
}

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

	// Baysian learning
	BayesianModels map[string]*BayesianThreshold
	LearningRate   float64
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
		BayesianModels:           make(map[string]*BayesianThreshold),
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

	// NEW: Validate data quality
	validTrades := c.filterValidTrades(trades)
	if len(validTrades) < len(trades)/2 {
		return fmt.Errorf("insufficient valid data: %d/%d trades have complete metrics",
			len(validTrades), len(trades))
	}

	// Ensure we have variance in metrics (not all zeros)
	if !c.hasSufficientVariance(validTrades) {
		return fmt.Errorf("insufficient variance in metrics for meaningful calibration")
	}

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

// CalibrateFromHistoryWithBayesian - enhanced calibration using Bayesian methods
func (c *ThresholdCalibrator) CalibrateFromHistoryWithBayesian(trades []TradeOutcome) error {
	if len(trades) < 30 {
		return fmt.Errorf("insufficient data: need at least 30 trades, got %d", len(trades))
	}

	// First, do regular calibration to get initial thresholds
	if err := c.CalibrateFromHistory(trades); err != nil {
		return err
	}

	// Get initial thresholds
	initialThresholds := map[string]float64{
		"VolumeAtEntry":     c.WeakVolumeThreshold,
		"OIAtEntry":         c.WeakOIThreshold,
		"VolumeDuringTrade": c.VolumeDecayThreshold,
		"OIDuringTrade":     c.OIDecayThreshold,
		"EntrySpread":       c.getDefaultThreshold("EntrySpread"),
		"ExitSpread":        c.getDefaultThreshold("ExitSpread"),
	}

	// Initialize Bayesian models with initial thresholds
	c.BayesianModels = make(map[string]*BayesianThreshold)
	for name, threshold := range initialThresholds {
		c.BayesianModels[name] = &BayesianThreshold{
			PriorAlpha:       1.0,
			PriorBeta:        1.0,
			PosteriorAlpha:   1.0,
			PosteriorBeta:    1.0,
			CurrentThreshold: threshold,
			Confidence:       0.5,
		}
	}

	// Update Bayesian models with all historical trades
	for _, trade := range trades {
		c.UpdateBayesian(trade, initialThresholds)
	}

	// Update thresholds with Bayesian-optimized values
	for metricName, model := range c.BayesianModels {
		if model.Confidence > 0.6 { // Only use if confident
			switch metricName {
			case "VolumeAtEntry":
				c.WeakVolumeThreshold = model.CurrentThreshold
			case "OIAtEntry":
				c.WeakOIThreshold = model.CurrentThreshold
			case "VolumeDuringTrade":
				c.VolumeDecayThreshold = model.CurrentThreshold
			case "OIDuringTrade":
				c.OIDecayThreshold = model.CurrentThreshold
			}
		}
	}

	return nil
}

// filterValidTrades removes trades with invalid or missing metrics
func (c *ThresholdCalibrator) filterValidTrades(trades []TradeOutcome) []TradeOutcome {
	var valid []TradeOutcome
	for _, t := range trades {
		// Skip trades with missing or invalid metrics
		if t.VolumeAtEntry <= 0 || t.EntrySpread <= 0 ||
			math.IsNaN(t.VolumeDuringTrade) || math.IsInf(t.VolumeDuringTrade, 0) {
			continue
		}
		valid = append(valid, t)
	}
	return valid
}

// hasSufficientVariance checks if metrics have enough variation for meaningful calibration
func (c *ThresholdCalibrator) hasSufficientVariance(trades []TradeOutcome) bool {
	if len(trades) < 5 {
		return false
	}

	// Check variance for key metrics
	metrics := []string{"VolumeAtEntry", "OIAtEntry", "EntrySpread"}
	for _, metric := range metrics {
		values := make([]float64, 0, len(trades))
		for _, t := range trades {
			var value float64
			switch metric {
			case "VolumeAtEntry":
				value = t.VolumeAtEntry
			case "OIAtEntry":
				value = t.OIAtEntry
			case "EntrySpread":
				value = t.EntrySpread
			}
			values = append(values, value)
		}

		if c.coefficientOfVariation(values) < 0.1 {
			// Less than 10% variation - insufficient for meaningful thresholds
			return false
		}
	}

	return true
}

// coefficientOfVariation calculates CV = std dev / mean
func (c *ThresholdCalibrator) coefficientOfVariation(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Calculate mean
	var sum float64
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	if mean == 0 {
		return 0
	}

	// Calculate standard deviation
	var variance float64
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values))
	stdDev := math.Sqrt(variance)

	return stdDev / mean
}

// extractMetrics extracts all relevant metrics from a trade for Bayesian updating
func (c *ThresholdCalibrator) extractMetrics(trade TradeOutcome) map[string]float64 {
	return map[string]float64{
		"VolumeAtEntry":     trade.VolumeAtEntry,
		"OIAtEntry":         trade.OIAtEntry,
		"VolumeDuringTrade": trade.VolumeDuringTrade,
		"OIDuringTrade":     trade.OIDuringTrade,
		"EntrySpread":       trade.EntrySpread,
		"ExitSpread":        trade.ExitSpread,
		"EntryDepth":        trade.EntryDepth,
		"ExitDepth":         trade.ExitDepth,
	}
}

// getDefaultThreshold returns a conservative default threshold for a metric
func (c *ThresholdCalibrator) getDefaultThreshold(metricName string) float64 {
	switch metricName {
	case "VolumeAtEntry":
		return 0.90
	case "OIAtEntry":
		return 0.30
	case "VolumeDuringTrade":
		return -0.30
	case "OIDuringTrade":
		return -0.20
	case "EntrySpread":
		return 0.002 // 0.2%
	case "ExitSpread":
		return 0.002
	case "EntryDepth":
		return 1000.0 // Arbitrary depth value
	case "ExitDepth":
		return 1000.0
	default:
		return 0.5
	}
}

// UpdateBayesian updates Bayesian thresholds with new trade data
func (c *ThresholdCalibrator) UpdateBayesian(trade TradeOutcome, currentThresholds map[string]float64) {
	if c.BayesianModels == nil {
		c.BayesianModels = make(map[string]*BayesianThreshold)
		c.LearningRate = 0.1 // Default learning rate
	}

	// For each threshold, update based on whether trade was successful
	metrics := c.extractMetrics(trade)

	for metricName, threshold := range currentThresholds {
		// Initialize Bayesian model if needed
		model, exists := c.BayesianModels[metricName]
		if !exists {
			model = &BayesianThreshold{
				PriorAlpha:       1.0, // Weak Jeffreys prior
				PriorBeta:        1.0,
				PosteriorAlpha:   1.0,
				PosteriorBeta:    1.0,
				CurrentThreshold: threshold,
				Confidence:       0.5,
			}
			c.BayesianModels[metricName] = model
		}

		// Get the actual metric value
		metricValue, hasValue := metrics[metricName]
		if !hasValue {
			continue
		}

		// Determine if this metric value relative to threshold predicted success
		var predictedSuccess bool

		switch metricName {
		case "VolumeAtEntry", "OIAtEntry", "EntryDepth", "ExitDepth":
			// Higher is better
			predictedSuccess = metricValue > threshold
		case "EntrySpread", "ExitSpread":
			// Lower is better
			predictedSuccess = metricValue < threshold
		case "VolumeDuringTrade", "OIDuringTrade":
			// Positive change is better (these are ratios)
			predictedSuccess = metricValue > threshold // threshold is usually negative
		default:
			predictedSuccess = true
		}

		// Update posterior
		if predictedSuccess && trade.Profitable {
			// True positive: threshold correctly predicted success
			model.PosteriorAlpha += 1.0
		} else if !predictedSuccess && !trade.Profitable {
			// True negative: threshold correctly predicted failure
			model.PosteriorAlpha += 1.0
		} else {
			// False positive or false negative
			model.PosteriorBeta += 1.0
		}

		// Calculate new optimal threshold using Bayesian optimization
		c.updateThresholdWithBayesianOptimization(model, metricName, metrics)
	}
}

// updateThresholdWithBayesianOptimization finds new threshold using Thompson sampling
func (c *ThresholdCalibrator) updateThresholdWithBayesianOptimization(
	model *BayesianThreshold,
	metricName string,
	metrics map[string]float64,
) {
	// Sample from posterior Beta distribution
	betaDist := distuv.Beta{
		Alpha: model.PosteriorAlpha,
		Beta:  model.PosteriorBeta,
	}

	// Thompson sampling: sample success probability
	sampledProb := betaDist.Rand()

	// Adjust threshold based on sampled probability
	metricValue := metrics[metricName]

	// Move threshold toward metric value if sampled probability is high
	// This is a simplified gradient ascent
	delta := (metricValue - model.CurrentThreshold) * c.LearningRate * sampledProb
	model.CurrentThreshold += delta

	// Update confidence (inverse of variance)
	total := model.PosteriorAlpha + model.PosteriorBeta
	if total > 2.0 {
		variance := (model.PosteriorAlpha * model.PosteriorBeta) /
			(total * total * (total + 1))
		model.Confidence = math.Max(0, 1.0-math.Sqrt(variance)*10)
	}
}

// GetDynamicThreshold returns threshold with uncertainty consideration
func (c *ThresholdCalibrator) GetDynamicThreshold(metricName string, confidenceLevel float64) float64 {
	model, exists := c.BayesianModels[metricName]
	if !exists {
		return c.getDefaultThreshold(metricName)
	}

	// If confidence is low, use more conservative threshold
	if model.Confidence < 0.3 {
		// Use lower quantile for safety
		betaDist := distuv.Beta{
			Alpha: model.PosteriorAlpha,
			Beta:  model.PosteriorBeta,
		}
		conservativeLevel := confidenceLevel * model.Confidence
		adjustment := betaDist.Quantile(conservativeLevel) - 0.5

		// Adjust threshold conservatively
		switch metricName {
		case "VolumeAtEntry", "OIAtEntry", "EntryDepth", "ExitDepth":
			return model.CurrentThreshold * (1.0 + adjustment) // Increase threshold
		case "EntrySpread", "ExitSpread":
			return model.CurrentThreshold * (1.0 - adjustment) // Decrease threshold
		case "VolumeDuringTrade", "OIDuringTrade":
			return model.CurrentThreshold * (1.0 + adjustment) // More conservative
		}
	}

	return model.CurrentThreshold
}

// GetBayesianSummary returns summary of Bayesian learning
func (c *ThresholdCalibrator) GetBayesianSummary() string {
	if len(c.BayesianModels) == 0 {
		return "No Bayesian models trained yet."
	}

	var sb strings.Builder
	sb.WriteString("Bayesian Threshold Learning Summary:\n")

	for name, model := range c.BayesianModels {
		sb.WriteString(fmt.Sprintf("  %s:\n", name))
		sb.WriteString(fmt.Sprintf("    Current: %.4f\n", model.CurrentThreshold))
		sb.WriteString(fmt.Sprintf("    Confidence: %.1f%%\n", model.Confidence*100))
		sb.WriteString(fmt.Sprintf("    Posterior: α=%.1f, β=%.1f\n",
			model.PosteriorAlpha, model.PosteriorBeta))
		successProb := model.PosteriorAlpha / (model.PosteriorAlpha + model.PosteriorBeta)
		sb.WriteString(fmt.Sprintf("    Success Probability: %.1f%%\n\n", successProb*100))
	}

	return sb.String()
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

// GetQualityScore returns a quality score for the calibration (0.0-1.0)
func (c *ThresholdCalibrator) GetQualityScore() float64 {
	if c.SampleSize == 0 {
		return 0.0
	}

	score := 0.0
	score += math.Min(float64(c.SampleSize)/100.0, 1.0) * 0.3 // Sample size weight

	// TODO: Add actual separation score and variance score
	// For now, use a basic heuristic
	if c.SampleSize >= 50 {
		score += 0.4
	} else if c.SampleSize >= 30 {
		score += 0.3
	} else {
		score += 0.1
	}

	return math.Max(0.0, math.Min(1.0, score))
}

// IsReliable checks if calibration is reliable enough to use
func (c *ThresholdCalibrator) IsReliable() bool {
	return c.SampleSize >= 30 && c.GetQualityScore() > 0.5
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
// ToCalibratedThresholds converts calibrator state to persistable format with metadata
func (c *ThresholdCalibrator) ToCalibratedThresholds() *CalibratedThresholds {
	ct := &CalibratedThresholds{
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
		ConfidenceScores:         make(map[string]float64),
		BayesianModels:           make(map[string]*SerializedBayesianModel),
		Metadata: map[string]string{
			"quality_score":      fmt.Sprintf("%.2f", c.GetQualityScore()),
			"reliable":           fmt.Sprintf("%v", c.IsReliable()),
			"calibration_method": "ROC_analysis",
		},
	}

	// Add Bayesian models if available
	if len(c.BayesianModels) > 0 {
		for name, model := range c.BayesianModels {
			// Store the full Bayesian model
			ct.BayesianModels[name] = &SerializedBayesianModel{
				PriorAlpha:       model.PriorAlpha,
				PriorBeta:        model.PriorBeta,
				PosteriorAlpha:   model.PosteriorAlpha,
				PosteriorBeta:    model.PosteriorBeta,
				CurrentThreshold: model.CurrentThreshold,
				Confidence:       model.Confidence,
			}

			// Calculate and store confidence score
			total := model.PosteriorAlpha + model.PosteriorBeta
			if total > 0 {
				// Calculate success probability
				successProb := model.PosteriorAlpha / total

				// Calculate variance of Beta distribution
				variance := (model.PosteriorAlpha * model.PosteriorBeta) /
					(total * total * (total + 1))

				// Confidence is based on low variance and high sample size
				confidence := model.Confidence
				ct.ConfidenceScores[name] = confidence

				// Add to metadata
				ct.Metadata[fmt.Sprintf("%s_success_prob", name)] = fmt.Sprintf("%.3f", successProb)
				ct.Metadata[fmt.Sprintf("%s_variance", name)] = fmt.Sprintf("%.6f", variance)
			}
		}

		ct.Metadata["has_bayesian_models"] = "true"
		ct.Metadata["bayesian_model_count"] = fmt.Sprintf("%d", len(c.BayesianModels))
	} else {
		ct.Metadata["has_bayesian_models"] = "false"
	}

	return ct
}

// FormatThresholdsForPromptWithUncertainty - enhanced prompt with confidence
// FormatThresholdsForPrompt returns formatted thresholds for LLM with appropriate detail level
func (c *ThresholdCalibrator) FormatThresholdsForPrompt(lang string, concise bool) string {
	if c.SampleSize == 0 {
		return "" // Not calibrated yet
	}

	if concise {
		return c.formatThresholdsConcise(lang)
	}
	return c.formatThresholdsDetailed(lang)
}

// formatThresholdsConcise returns very brief thresholds (for limited context windows)
func (c *ThresholdCalibrator) formatThresholdsConcise(lang string) string {
	if lang == "zh" {
		return fmt.Sprintf(`📊 风险阈值 (基于%d笔交易):
• 弱成交量: %.2f | 弱持仓量: %.2f
• 动量衰减: 成交量%.2f | 持仓量%.2f
• 流动性: 价差>%.1fx | 深度<%.2f`,
			c.SampleSize,
			c.WeakVolumeThreshold, c.WeakOIThreshold,
			c.VolumeDecayThreshold, c.OIDecayThreshold,
			c.SpreadWorseningMultiple, c.DepthReductionThreshold,
		)
	}

	return fmt.Sprintf(`📊 Risk Thresholds (from %d trades):
• Weak Volume: %.2f | Weak OI: %.2f
• Momentum Decay: Volume%.2f | OI%.2f
• Liquidity: Spread>%.1fx | Depth<%.2f`,
		c.SampleSize,
		c.WeakVolumeThreshold, c.WeakOIThreshold,
		c.VolumeDecayThreshold, c.OIDecayThreshold,
		c.SpreadWorseningMultiple, c.DepthReductionThreshold,
	)
}

// formatThresholdsDetailed returns thresholds with confidence indicators (recommended for most use)
func (c *ThresholdCalibrator) formatThresholdsDetailed(lang string) string {
	var sb strings.Builder

	if lang == "zh" {
		sb.WriteString(fmt.Sprintf("## 📊 学习到的风险阈值 (基于 %d 笔交易)\n\n", c.SampleSize))

		// Entry quality with confidence
		sb.WriteString("**🎯 入场质量检测**:\n")
		c.addThresholdWithConfidence(&sb, "成交量警戒", "VolumeAtEntry", c.WeakVolumeThreshold, lang)
		c.addThresholdWithConfidence(&sb, "持仓量警戒", "OIAtEntry", c.WeakOIThreshold, lang)

		// Momentum decay with confidence
		sb.WriteString("\n**📉 持仓期间监控**:\n")
		c.addThresholdWithConfidence(&sb, "成交量衰减", "VolumeDuringTrade", c.VolumeDecayThreshold, lang)
		c.addThresholdWithConfidence(&sb, "持仓量衰减", "OIDuringTrade", c.OIDecayThreshold, lang)

		// Liquidity (no Bayesian confidence yet)
		sb.WriteString("\n**💧 流动性监控**:\n")
		sb.WriteString(fmt.Sprintf("- 价差恶化倍数: %.1fx (价差扩大超过此倍数则流动性恶化)\n", c.SpreadWorseningMultiple))
		sb.WriteString(fmt.Sprintf("- 深度缩减阈值: %.2f (深度缩减低于此比例则流动性枯竭)\n", c.DepthReductionThreshold))

		// Quality indicator
		sb.WriteString(fmt.Sprintf("\n💡 校准质量: %s\n", c.getQualityEmoji()))

	} else {
		sb.WriteString(fmt.Sprintf("## 📊 Learned Risk Thresholds (from %d trades)\n\n", c.SampleSize))

		// Entry quality with confidence
		sb.WriteString("**🎯 Entry Quality Detection**:\n")
		c.addThresholdWithConfidence(&sb, "Volume Alert", "VolumeAtEntry", c.WeakVolumeThreshold, lang)
		c.addThresholdWithConfidence(&sb, "OI Alert", "OIAtEntry", c.WeakOIThreshold, lang)

		// Momentum decay with confidence
		sb.WriteString("\n**📉 During-Trade Monitoring**:\n")
		c.addThresholdWithConfidence(&sb, "Volume Decay", "VolumeDuringTrade", c.VolumeDecayThreshold, lang)
		c.addThresholdWithConfidence(&sb, "OI Decay", "OIDuringTrade", c.OIDecayThreshold, lang)

		// Liquidity (no Bayesian confidence yet)
		sb.WriteString("\n**💧 Liquidity Monitoring**:\n")
		sb.WriteString(fmt.Sprintf("- Spread Worsening: >%.1fx (spread widens beyond this = liquidity deteriorating)\n", c.SpreadWorseningMultiple))
		sb.WriteString(fmt.Sprintf("- Depth Reduction: <%.2f (depth falls below this = liquidity dried)\n", c.DepthReductionThreshold))

		// Quality indicator
		sb.WriteString(fmt.Sprintf("\n💡 Calibration Quality: %s\n", c.getQualityEmoji()))
	}

	return sb.String()
}

// addThresholdWithConfidence adds a threshold line with confidence indicator
func (c *ThresholdCalibrator) addThresholdWithConfidence(sb *strings.Builder, displayName, modelKey string, threshold float64, lang string) {
	confidence := 0.5
	if model, exists := c.BayesianModels[modelKey]; exists {
		confidence = model.Confidence
	}

	confidenceEmoji := "🟢" // High confidence
	if confidence < 0.3 {
		confidenceEmoji = "🔴" // Low confidence
	} else if confidence < 0.6 {
		confidenceEmoji = "🟡" // Medium confidence
	}

	var formatString string
	if lang == "zh" {
		if modelKey == "VolumeDuringTrade" || modelKey == "OIDuringTrade" {
			// These are negative thresholds (decay thresholds)
			formatString = "- %s %s: %.2f (置信度: %.0f%%)\n"
		} else {
			formatString = "- %s %s: %.2f (置信度: %.0f%%)\n"
		}
	} else {
		if modelKey == "VolumeDuringTrade" || modelKey == "OIDuringTrade" {
			formatString = "- %s %s: %.2f (confidence: %.0f%%)\n"
		} else {
			formatString = "- %s %s: %.2f (confidence: %.0f%%)\n"
		}
	}

	sb.WriteString(fmt.Sprintf(formatString,
		confidenceEmoji, displayName, threshold, confidence*100))
}

// getQualityEmoji returns emoji based on calibration quality
func (c *ThresholdCalibrator) getQualityEmoji() string {
	if !c.IsReliable() {
		return "🔴 LOW (needs more data)"
	}

	score := c.GetQualityScore()
	if score > 0.8 {
		return "🟢 EXCELLENT"
	} else if score > 0.6 {
		return "🟡 GOOD"
	} else {
		return "🟠 FAIR"
	}
}

// GetThresholdsForLLM returns appropriately formatted thresholds based on context window size
func (c *ThresholdCalibrator) GetThresholdsForLLM(lang string, availableTokens int) string {
	if c.SampleSize == 0 {
		if lang == "zh" {
			return "⚠️ 尚未校准阈值 - 使用默认值"
		}
		return "⚠️ Thresholds not calibrated yet - using defaults"
	}

	// Estimate token counts
	detailedTokens := 80 // Rough estimate

	if availableTokens < detailedTokens {
		// Limited context window - use concise version
		return c.formatThresholdsConcise(lang)
	} else if availableTokens < 200 {
		// Medium context window - use detailed but skip explanations
		return c.formatThresholdsDetailed(lang)
	} else {
		// Plenty of space - use full version with explanations
		return c.formatThresholdsFull(lang)
	}
}

// formatThresholdsFull returns complete thresholds with all explanations (for large context windows)
func (c *ThresholdCalibrator) formatThresholdsFull(lang string) string {
	var sb strings.Builder

	if lang == "zh" {
		sb.WriteString("## 📊 学习到的风险阈值分析报告\n\n")
		sb.WriteString(fmt.Sprintf("**数据基础**: %d 笔历史交易\n\n", c.SampleSize))

		sb.WriteString("### 🎯 入场质量检测阈值\n")
		sb.WriteString("这些阈值帮助判断是否在合适的市场条件下入场：\n")
		c.addThresholdWithExplanation(&sb, "成交量警戒线", "VolumeAtEntry", c.WeakVolumeThreshold,
			"低于此值表示市场成交量不足，信号可能不可靠", lang)
		c.addThresholdWithExplanation(&sb, "持仓量警戒线", "OIAtEntry", c.WeakOIThreshold,
			"低于此值表示持仓兴趣不足，趋势可能缺乏持续性", lang)

		sb.WriteString("\n### 📉 持仓期间监控阈值\n")
		sb.WriteString("这些阈值帮助判断持仓期间市场条件是否恶化：\n")
		c.addThresholdWithExplanation(&sb, "成交量衰减警戒", "VolumeDuringTrade", c.VolumeDecayThreshold,
			"成交量下降超过此比例表示动量正在衰减，应考虑提前退出", lang)
		c.addThresholdWithExplanation(&sb, "持仓量衰减警戒", "OIDuringTrade", c.OIDecayThreshold,
			"持仓量下降超过此比例表示市场兴趣正在减弱", lang)

		sb.WriteString("\n### 💧 流动性监控阈值\n")
		sb.WriteString("这些阈值帮助判断执行条件是否恶化：\n")
		sb.WriteString(fmt.Sprintf("- **价差恶化倍数**: %.1fx - 价差扩大超过此倍数表示流动性正在恶化，执行成本增加\n", c.SpreadWorseningMultiple))
		sb.WriteString(fmt.Sprintf("- **深度缩减阈值**: %.2f - 市场深度缩减低于此比例表示流动性枯竭，退出困难\n", c.DepthReductionThreshold))

		sb.WriteString("\n### 📈 校准质量评估\n")
		sb.WriteString(fmt.Sprintf("- 样本数量: %d 笔交易\n", c.SampleSize))
		sb.WriteString(fmt.Sprintf("- 置信度水平: %s\n", c.getQualityEmoji()))
		sb.WriteString(fmt.Sprintf("- 可靠性: %v\n", c.IsReliable()))

		sb.WriteString("\n💡 **使用建议**: 当市场条件接近这些阈值时，应更加谨慎。低置信度阈值需要更多验证。\n")

	} else {
		sb.WriteString("## 📊 Learned Risk Thresholds Analysis Report\n\n")
		sb.WriteString(fmt.Sprintf("**Data Basis**: %d historical trades\n\n", c.SampleSize))

		sb.WriteString("### 🎯 Entry Quality Detection Thresholds\n")
		sb.WriteString("These thresholds help determine if market conditions are suitable for entry:\n")
		c.addThresholdWithExplanation(&sb, "Volume Alert", "VolumeAtEntry", c.WeakVolumeThreshold,
			"Below this value indicates insufficient market volume, signals may be unreliable", lang)
		c.addThresholdWithExplanation(&sb, "OI Alert", "OIAtEntry", c.WeakOIThreshold,
			"Below this value indicates insufficient position interest, trend may lack persistence", lang)

		sb.WriteString("\n### 📉 During-Trade Monitoring Thresholds\n")
		sb.WriteString("These thresholds help determine if market conditions are deteriorating during holding:\n")
		c.addThresholdWithExplanation(&sb, "Volume Decay", "VolumeDuringTrade", c.VolumeDecayThreshold,
			"Volume decline beyond this indicates momentum decay, consider early exit", lang)
		c.addThresholdWithExplanation(&sb, "OI Decay", "OIDuringTrade", c.OIDecayThreshold,
			"OI decline beyond this indicates weakening market interest", lang)

		sb.WriteString("\n### 💧 Liquidity Monitoring Thresholds\n")
		sb.WriteString("These thresholds help determine if execution conditions are deteriorating:\n")
		sb.WriteString(fmt.Sprintf("- **Spread Worsening Multiple**: %.1fx - Spread widening beyond this indicates liquidity deterioration, increased execution costs\n", c.SpreadWorseningMultiple))
		sb.WriteString(fmt.Sprintf("- **Depth Reduction Threshold**: %.2f - Market depth falling below this indicates liquidity drying up, difficult exit\n", c.DepthReductionThreshold))

		sb.WriteString("\n### 📈 Calibration Quality Assessment\n")
		sb.WriteString(fmt.Sprintf("- Sample Size: %d trades\n", c.SampleSize))
		sb.WriteString(fmt.Sprintf("- Confidence Level: %s\n", c.getQualityEmoji()))
		sb.WriteString(fmt.Sprintf("- Reliable: %v\n", c.IsReliable()))

		sb.WriteString("\n💡 **Usage Advice**: Be more cautious when market conditions approach these thresholds. Low-confidence thresholds require more verification.\n")
	}

	return sb.String()
}

// addThresholdWithExplanation adds a threshold with full explanation
func (c *ThresholdCalibrator) addThresholdWithExplanation(sb *strings.Builder, displayName, modelKey string, threshold float64, explanation, lang string) {
	confidence := 0.5
	if model, exists := c.BayesianModels[modelKey]; exists {
		confidence = model.Confidence
	}

	confidenceEmoji := "🟢"
	if confidence < 0.3 {
		confidenceEmoji = "🔴"
	} else if confidence < 0.6 {
		confidenceEmoji = "🟡"
	}

	if lang == "zh" {
		sb.WriteString(fmt.Sprintf("- %s **%s**: %.2f\n", confidenceEmoji, displayName, threshold))
		sb.WriteString(fmt.Sprintf("  → %s (置信度: %.0f%%)\n", explanation, confidence*100))
	} else {
		sb.WriteString(fmt.Sprintf("- %s **%s**: %.2f\n", confidenceEmoji, displayName, threshold))
		sb.WriteString(fmt.Sprintf("  → %s (confidence: %.0f%%)\n", explanation, confidence*100))
	}
}
