package decision

import (
	"fmt"
	"log"
)

// ExampleUsage demonstrates how to use the threshold calibration system
func ExampleUsage() {
	// 1. Collect historical trade outcomes from your backtest or live trading database
	historicalTrades := []TradeOutcome{
		{
			Symbol:            "BTCUSDT",
			Profitable:        false,
			VolumeAtEntry:     0.75,  // Low volume = red flag
			OIAtEntry:         0.20,  // Low OI = red flag
			VolumeDuringTrade: -0.35, // Volume dried up
			OIDuringTrade:     -0.25, // OI declined
			EntrySpread:       0.001,
			ExitSpread:        0.003, // 3x spread widening
			EntryDepth:        100000,
			ExitDepth:         40000, // Depth collapsed
			HoldingMinutes:    45,
			PnLPct:            -2.5,
		},
		{
			Symbol:            "ETHUSDT",
			Profitable:        true,
			VolumeAtEntry:     1.15, // Strong volume
			OIAtEntry:         0.40, // Strong OI
			VolumeDuringTrade: -0.10,
			OIDuringTrade:     -0.05,
			EntrySpread:       0.001,
			ExitSpread:        0.0012,
			EntryDepth:        200000,
			ExitDepth:         180000,
			HoldingMinutes:    60,
			PnLPct:            3.5,
		},
		// Add hundreds more...
	}

	// 2. Create calibrator and learn from history
	calibrator := NewThresholdCalibrator()
	err := calibrator.CalibrateFromHistory(historicalTrades)
	if err != nil {
		log.Fatalf("Calibration failed: %v", err)
	}

	// 3. Review calibration results
	fmt.Println(calibrator.GetCalibrationSummary())

	// 4. Apply to analyzer
	thresholds := calibrator.ApplyToAnalyzer()

	// 5. Use calibrated thresholds for new trade analysis
	newTrade := &RecentOrder{
		Symbol:         "SOLUSDT",
		VolumeAtEntry:  0.80, // Is this weak? Use calibrated threshold to decide
		OIDeltaAtEntry: 0.25,
		// ... other fields
	}

	analysis := AnalyzeFailedTradeWithThresholds(newTrade, &thresholds)
	if analysis != nil {
		fmt.Printf("Failure reason: %s (confidence: %.0f%%)\n",
			analysis.PrimaryReason, analysis.ConfidenceScore*100)
		fmt.Printf("Recommendation: %s\n", analysis.Recommendation)
	}

	// 6. Periodically re-calibrate as market conditions evolve
	// Run calibration monthly or after significant regime changes
}

// IntegrationWithBacktest shows how to integrate calibration with backtesting
func IntegrationWithBacktest() {
	// Pseudo-code for integration:
	//
	// 1. Run initial backtest with default thresholds
	// 2. Collect all trade outcomes from backtest
	// 3. Calibrate thresholds from outcomes
	// 4. Re-run backtest with calibrated thresholds
	// 5. Compare results (should improve failure detection accuracy)
	// 6. If improvement detected, persist calibrated thresholds to config
	// 7. Use calibrated thresholds in live trading

	// Example calibration pipeline:
	fmt.Println(`
	Calibration Pipeline:
	=====================

	Step 1: Initial Backtest
	  - Run with DefaultFailureThresholds()
	  - Log all trade outcomes to database

	Step 2: Data Collection
	  - Query last 500+ closed trades
	  - Extract entry/exit metrics (volume, OI, spread, depth)
	  - Label trades as profitable/unprofitable

	Step 3: Threshold Calibration
	  - calibrator := NewThresholdCalibrator()
	  - calibrator.CalibrateFromHistory(trades)
	  - thresholds := calibrator.ApplyToAnalyzer()

	Step 4: Validation Backtest
	  - Run backtest with calibrated thresholds
	  - Measure false positive/false negative rates
	  - Compare to baseline (default thresholds)

	Step 5: Deployment
	  - If metrics improved: save thresholds to config
	  - Use calibrated thresholds in live trading
	  - Monitor and re-calibrate monthly

	Key Metrics to Track:
	  - True Positive Rate: % of losers correctly identified
	  - False Positive Rate: % of winners incorrectly flagged
	  - Youden's J: TPR + TNR - 1 (maximized during calibration)
	`)
}
