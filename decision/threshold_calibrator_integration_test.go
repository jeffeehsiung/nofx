package decision

/*
// illustrateThresholdCalibration demonstrates complete end-to-end calibration workflow
func illustrateThresholdCalibration() {
	// Step 1: Prepare historical trade data
	// In production, this would come from your database:
	//   SELECT symbol, profitable, volume_at_entry, oi_at_entry, ...
	//   FROM closed_trades WHERE close_time > NOW() - INTERVAL 90 DAY

	historicalTrades := []TradeOutcome{
		// Losing trade with weak entry signals
		{
			Symbol:            "BTCUSDT",
			Profitable:        false,
			VolumeAtEntry:     0.75,  // Only 75% of average volume
			OIAtEntry:         0.20,  // Only 20% OI increase
			VolumeDuringTrade: -0.35, // Volume dried up 35%
			OIDuringTrade:     -0.25, // OI declined 25%
			EntrySpread:       0.0010,
			ExitSpread:        0.0032, // Spread widened 3.2x
			EntryDepth:        150000,
			ExitDepth:         50000, // Depth collapsed to 33%
			HoldingMinutes:    45,
			PnLPct:            -2.8,
		},

		// Winning trade with strong confirmation
		{
			Symbol:            "BTCUSDT",
			Profitable:        true,
			VolumeAtEntry:     1.25,  // 125% of average (strong)
			OIAtEntry:         0.45,  // 45% OI increase (strong)
			VolumeDuringTrade: -0.05, // Minor volume decline
			OIDuringTrade:     0.10,  // OI actually increased
			EntrySpread:       0.0010,
			ExitSpread:        0.0012, // Spread stable
			EntryDepth:        200000,
			ExitDepth:         190000, // Depth stable
			HoldingMinutes:    90,
			PnLPct:            4.2,
		},

		// Add 200+ more real trades for proper calibration...
	}

	// Step 2: Create calibrator
	calibrator := NewThresholdCalibrator()

	// Step 3: Calibrate from historical data
	err := calibrator.CalibrateFromHistory(historicalTrades)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Step 4: Review calibration results
	summary := calibrator.GetCalibrationSummary()
	fmt.Println(summary)

	// Step 5: Get calibrated thresholds
	thresholds := calibrator.ApplyToAnalyzer()

	// Step 6: Analyze a new trade with calibrated thresholds
	newTrade := &RecentOrder{
		Symbol:         "BTCUSDT",
		VolumeAtEntry:  0.78, // Borderline weak volume
		OIDeltaAtEntry: 0.22, // Borderline weak OI
		Side:           "LONG",
		EntryPrice:     50000,
		// ... other fields
	}

	// Use calibrated thresholds
	analysis := AnalyzeFailedTradeWithThresholds(newTrade, &thresholds)

	if analysis != nil {
		fmt.Printf("\nTrade Analysis:\n")
		fmt.Printf("  Primary Reason: %s\n", analysis.PrimaryReason)
		fmt.Printf("  Confidence: %.0f%%\n", analysis.ConfidenceScore*100)
		fmt.Printf("  Recommendation: %s\n", analysis.Recommendation)
	}

	// For comparison: analyze without calibration (uses defaults)
	defaultAnalysis := AnalyzeFailedTrade(newTrade)
	if defaultAnalysis != nil {
		fmt.Printf("\nDefault Analysis (for comparison):\n")
		fmt.Printf("  Primary Reason: %s\n", defaultAnalysis.PrimaryReason)
		fmt.Printf("  Confidence: %.0f%%\n", defaultAnalysis.ConfidenceScore*100)
	}
}

// illustrateBacktestIntegration shows how to integrate calibration with backtest system
func illustrateBacktestIntegration() {
	// Pseudo-code for backtest integration
	fmt.Println(`
Backtest Integration Example
============================

1. At Backtest Startup:
   ----------------------
   
   // Load recent trades from database
   trades := db.Query("SELECT * FROM closed_trades WHERE close_time > ?", 
                      time.Now().Add(-90*24*time.Hour))
   
   // Calibrate thresholds
   calibrator := decision.NewThresholdCalibrator()
   calibrator.CalibrateFromHistory(convertToTradeOutcomes(trades))
   
   // Store in backtest context
   bt.FailureThresholds = calibrator.ApplyToAnalyzer()
   
   log.Printf("Calibrated from %d historical trades", len(trades))
   log.Print(calibrator.GetCalibrationSummary())


2. During Backtest:
   -----------------
   
   // When a trade closes
   func OnTradeClose(order *decision.RecentOrder) {
       // Analyze with calibrated thresholds
       analysis := decision.AnalyzeFailedTradeWithThresholds(
           order, 
           &bt.FailureThresholds,
       )
       
       if analysis != nil && !order.Profitable {
           // Log failure reason
           log.Printf("Trade failed: %s (%.0f%% confidence)", 
                      analysis.PrimaryReason, 
                      analysis.ConfidenceScore*100)
           
           // Update statistics
           bt.Stats.FailureReasons[analysis.PrimaryReason]++
       }
   }


3. After Backtest:
   ----------------
   
   // Generate failure analysis report
   func GenerateReport(bt *Backtest) {
       fmt.Println("\nTrade Failure Analysis")
       fmt.Println("======================")
       
       for reason, count := range bt.Stats.FailureReasons {
           pct := float64(count) / float64(bt.Stats.TotalLosses) * 100
           fmt.Printf("  %s: %d (%.1f%%)\n", reason, count, pct)
       }
       
       // Actionable insights
       if bt.Stats.FailureReasons["false_breakout_v2"] > bt.Stats.TotalLosses/3 {
           fmt.Println("\n⚠️  Recommendation: Improve entry confirmation criteria")
           fmt.Println("   - Require higher volume confirmation")
           fmt.Println("   - Add OI divergence filter")
       }
   }


4. Periodic Recalibration:
   ------------------------
   
   // Run monthly (cron job or scheduler)
   func MonthlyRecalibration() {
       // Get last 60 days or 500 trades
       trades := db.GetRecentTrades(max(500, last60Days))
       
       if len(trades) < 100 {
           log.Warn("Insufficient data for recalibration")
           return
       }
       
       // Calibrate
       calibrator := decision.NewThresholdCalibrator()
       err := calibrator.CalibrateFromHistory(trades)
       if err != nil {
           log.Error("Recalibration failed: %v", err)
           return
       }
       
       // Compare to current thresholds
       newThresholds := calibrator.ApplyToAnalyzer()
       oldThresholds := config.Current().FailureThresholds
       
       drift := calculateDrift(oldThresholds, newThresholds)
       log.Printf("Threshold drift: %.1f%%", drift)
       
       // Update config
       config.Update(newThresholds)
       
       // Notify team
       sendNotification("Thresholds recalibrated", 
                       calibrator.GetCalibrationSummary())
   }
`)
}

// illustratePerSymbolCalibration demonstrates symbol-specific threshold calibration
func illustratePerSymbolCalibration() {
	fmt.Println(`
Per-Symbol Calibration Example
==============================

Different assets have different characteristics:
- BTC: High liquidity, tight spreads, stable OI
- Altcoins: Lower liquidity, wider spreads, volatile OI

Solution: Calibrate separately for each symbol
`)

	// Simulate per-symbol calibration
	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
	symbolThresholds := make(map[string]FailureThresholds)

	for _, symbol := range symbols {
		fmt.Printf("\nCalibrating %s:\n", symbol)
		fmt.Println("-------------------")

		// In production: filter trades by symbol
		// symbolTrades := filterTradesBySymbol(allTrades, symbol)

		// For demo: simulate different threshold characteristics

		// Simulate calibration (in production: use real data)
		switch symbol {
		case "BTCUSDT":
			// BTC: Stricter thresholds (high liquidity expected)
			symbolThresholds[symbol] = FailureThresholds{
				WeakVolumeThreshold:      0.92,
				WeakOIThreshold:          0.35,
				PrematureVolumeThreshold: 0.92,
				PrematureOIThreshold:     0.55,
				VolumeDecayThreshold:     -0.25,
				OIDecayThreshold:         -0.18,
				SpreadWorseningMultiple:  1.8,
				DepthReductionThreshold:  0.55,
			}

		case "ETHUSDT":
			// ETH: Moderate thresholds
			symbolThresholds[symbol] = DefaultFailureThresholds()

		case "SOLUSDT":
			// SOL: More permissive (lower liquidity)
			symbolThresholds[symbol] = FailureThresholds{
				WeakVolumeThreshold:      0.85,
				WeakOIThreshold:          0.25,
				PrematureVolumeThreshold: 0.85,
				PrematureOIThreshold:     0.45,
				VolumeDecayThreshold:     -0.35,
				OIDecayThreshold:         -0.25,
				SpreadWorseningMultiple:  2.5,
				DepthReductionThreshold:  0.40,
			}
		}

		fmt.Printf("  Weak Volume Threshold: %.2f\n", symbolThresholds[symbol].WeakVolumeThreshold)
		fmt.Printf("  Weak OI Threshold: %.2f\n", symbolThresholds[symbol].WeakOIThreshold)
		fmt.Printf("  Spread Worsening: %.1fx\n", symbolThresholds[symbol].SpreadWorseningMultiple)
	}

	fmt.Println("\n✅ Per-symbol calibration complete!")
	fmt.Println("   Store in config: config.SymbolThresholds[symbol]")
}

// Helper to show calibration metrics
func illustrateValidationMetrics() {
	fmt.Println(`
Validation Metrics
==================

After calibration, validate on holdout test set:

True Positives (TP):  Losers correctly identified as risky
False Positives (FP): Winners incorrectly flagged as risky
True Negatives (TN):  Winners correctly identified as safe
False Negatives (FN): Losers incorrectly identified as safe

Key Metrics:
-----------
Sensitivity (Recall):  TP / (TP + FN)  →  % of losers caught
Specificity:           TN / (TN + FP)  →  % of false alarms avoided
Accuracy:              (TP + TN) / Total
Youden's J:            Sensitivity + Specificity - 1  (maximized)

Target Performance:
-------------------
✅ Sensitivity: 75-85%  (catch most losers)
✅ Specificity: 70-80%  (low false alarm rate)
✅ Youden's J: 0.45-0.65

Example Results:
----------------
Calibration from 547 trades:
  True Positives:  89 (81% sensitivity)
  False Positives: 18 (76% specificity)
  True Negatives:  74
  False Negatives: 21
  Youden's J: 0.57 ✅ GOOD

Interpretation:
  - Catches 81% of losing trades
  - Only 24% false alarm rate
  - Well-balanced threshold selection
`)
}
*/
