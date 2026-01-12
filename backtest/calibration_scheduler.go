package backtest

import (
	"nofx/decision"
	"nofx/logger"
	"time"
)

// CalibrationScheduler handles periodic recalibration of failure thresholds
type CalibrationScheduler struct {
	ticker   *time.Ticker
	stopCh   chan struct{}
	interval time.Duration
	manager  *Manager
}

// NewCalibrationScheduler creates a new calibration scheduler
func NewCalibrationScheduler(manager *Manager, interval time.Duration) *CalibrationScheduler {
	if interval == 0 {
		interval = 30 * 24 * time.Hour // Default: monthly
	}

	return &CalibrationScheduler{
		ticker:   time.NewTicker(interval),
		stopCh:   make(chan struct{}),
		interval: interval,
		manager:  manager,
	}
}

// Start begins the periodic calibration scheduler
func (cs *CalibrationScheduler) Start() {
	logger.Infof("📅 Calibration scheduler started (interval: %v)", cs.interval)

	go func() {
		for {
			select {
			case <-cs.ticker.C:
				cs.runCalibration()
			case <-cs.stopCh:
				cs.ticker.Stop()
				logger.Info("📅 Calibration scheduler stopped")
				return
			}
		}
	}()
}

// Stop stops the calibration scheduler
func (cs *CalibrationScheduler) Stop() {
	close(cs.stopCh)
}

// runCalibration performs a full calibration cycle
func (cs *CalibrationScheduler) runCalibration() {
	logger.Info("🔧 Starting periodic threshold recalibration...")

	// Get all completed backtest runs from the last 90 days
	runs, err := cs.manager.ListRuns()
	if err != nil {
		logger.Errorf("❌ Failed to list backtest runs: %v", err)
		return
	}

	// Collect all trade outcomes from recent runs
	var allOutcomes []DecisionOutcome
	cutoffTime := time.Now().Add(-90 * 24 * time.Hour)

	for _, run := range runs {
		// Skip if too old or not completed
		if run.CreatedAt.Before(cutoffTime) || run.State != RunStateCompleted {
			continue
		}

		// Load feedback analysis for this run
		analysis, err := LoadFeedbackAnalysis(run.RunID)
		if err != nil {
			continue // Skip runs without analysis
		}

		// Collect losing trades
		if analysis.TopLosingTrades != nil {
			allOutcomes = append(allOutcomes, analysis.TopLosingTrades...)
		}
	}

	if len(allOutcomes) < 100 {
		logger.Warnf("⚠️ Insufficient data for recalibration (%d trades, need 100+)", len(allOutcomes))
		logger.Info("💡 Run more backtests or wait for more historical data")
		return
	}

	// Convert to TradeOutcome format for calibrator
	tradeOutcomes := make([]decision.TradeOutcome, 0, len(allOutcomes))
	for _, outcome := range allOutcomes {
		if outcome.RecentOrder == nil {
			continue
		}

		// Parse hold duration from string (e.g., "2h 30m" or "45m 30s")
		holdMinutes := 0
		if outcome.RecentOrder.HoldDuration != "" {
			if duration, err := parseHoldDuration(outcome.RecentOrder.HoldDuration); err == nil {
				holdMinutes = int(duration.Minutes())
			}
		}

		tradeOutcomes = append(tradeOutcomes, decision.TradeOutcome{
			Symbol:            outcome.Symbol,
			Profitable:        outcome.Success,
			VolumeAtEntry:     outcome.RecentOrder.VolumeAtEntry,
			OIAtEntry:         outcome.RecentOrder.OIDeltaAtEntry,
			VolumeDuringTrade: outcome.RecentOrder.VolumeDeltaDuringTrade,
			OIDuringTrade:     outcome.RecentOrder.OIDeltaDuringTrade,
			EntrySpread:       outcome.RecentOrder.EntrySpread,
			ExitSpread:        outcome.RecentOrder.ExitSpread,
			EntryDepth:        outcome.RecentOrder.EntryDepth,
			ExitDepth:         outcome.RecentOrder.ExitDepth,
			HoldingMinutes:    holdMinutes,
			PnLPct:            outcome.RealizedPnLPct,
		})
	}

	if len(tradeOutcomes) < 100 {
		logger.Warnf("⚠️ Insufficient valid trades for recalibration (%d trades)", len(tradeOutcomes))
		return
	}

	// Run calibration
	calibrator := decision.NewThresholdCalibrator()
	if err := calibrator.CalibrateFromHistory(tradeOutcomes); err != nil {
		logger.Errorf("❌ Calibration failed: %v", err)
		return
	}

	// Convert to persistable format with metadata
	thresholds := calibrator.ToCalibratedThresholds()

	// Save to disk for persistence
	if err := decision.SaveCalibratedThresholds(thresholds); err != nil {
		logger.Warnf("⚠️ Failed to save calibrated thresholds: %v", err)
	}

	// Log summary
	summary := calibrator.GetCalibrationSummary()
	logger.Info("✅ Thresholds recalibrated successfully")
	logger.Info(summary)

	// TODO: Broadcast to active traders to reload thresholds
	// This would require integration with trader manager
}

// RunManualCalibration allows manual triggering of calibration (for testing)
func (cs *CalibrationScheduler) RunManualCalibration() {
	logger.Info("🔧 Manual calibration triggered")
	cs.runCalibration()
}

// parseHoldDuration converts formatted duration strings back to time.Duration
// Handles formats like "2h 30m", "45m 30s", "30s"
func parseHoldDuration(s string) (time.Duration, error) {
	// Try standard Go duration format first
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// Fallback: try parsing from formatted string with spaces
	// Example: "2h 30m 15s" -> "2h30m15s"
	normalized := ""
	for _, r := range s {
		if r != ' ' {
			normalized += string(r)
		}
	}

	return time.ParseDuration(normalized)
}
