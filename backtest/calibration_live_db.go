//go:build calibration_live_db

package backtest

import (
	"fmt"
	"time"

	"nofx/decision"
	"nofx/logger"
	"nofx/store"
)

// CalibrateFromLiveDB loads recent trades for a specific trader and calibrates thresholds.
// Requires a PositionStore instance (caller is responsible for wiring DB access).
func CalibrateFromLiveDB(posStore *store.PositionStore, traderID string, maxTrades int) (decision.FailureThresholds, int, string, error) {
	defaults := decision.DefaultFailureThresholds()

	if traderID == "" {
		return defaults, 0, "", fmt.Errorf("traderID required for live DB calibration")
	}

	if maxTrades <= 0 {
		maxTrades = 500
	}

	if posStore == nil {
		return defaults, 0, "", fmt.Errorf("position store not provided")
	}

	recentTrades, err := posStore.GetRecentTrades(traderID, maxTrades)
	if err != nil {
		return defaults, 0, "", fmt.Errorf("failed to fetch recent trades for trader %s: %w", traderID, err)
	}

	if len(recentTrades) == 0 {
		return defaults, 0, "no recent trades found for trader " + traderID, nil
	}

	logger.Infof("Fetched %d recent trades from DB for trader %s", len(recentTrades), traderID)

	outcomes := make([]decision.TradeOutcome, 0, len(recentTrades))
	for _, trade := range recentTrades {
		holdingMinutes := 0
		if trade.ExitTime > 0 && trade.EntryTime > 0 && trade.ExitTime > trade.EntryTime {
			holdingMinutes = int(time.Unix(trade.ExitTime, 0).Sub(time.Unix(trade.EntryTime, 0)).Minutes())
		}

		outcomes = append(outcomes, decision.TradeOutcome{
			Symbol:            trade.Symbol,
			Profitable:        trade.RealizedPnL > 0,
			VolumeAtEntry:     1.0,
			OIAtEntry:         0.0,
			VolumeDuringTrade: 0.0,
			OIDuringTrade:     0.0,
			EntrySpread:       0.0,
			ExitSpread:        0.0,
			EntryDepth:        0.0,
			ExitDepth:         0.0,
			HoldingMinutes:    holdingMinutes,
			PnLPct:            trade.PnLPct,
		})
	}

	calibrator := decision.NewThresholdCalibrator()
	if err := calibrator.CalibrateFromHistory(outcomes); err != nil {
		return defaults, len(outcomes), "", err
	}

	thresholds := calibrator.ApplyToAnalyzer()
	summary := calibrator.GetCalibrationSummary()
	sampleCount := len(outcomes)
	fullSummary := fmt.Sprintf("Live DB calibration (trader=%s, samples=%d): %s", traderID, sampleCount, summary)

	logger.Info(fullSummary)

	return thresholds, sampleCount, fullSummary, nil
}

// CalibrateFromOfflineRuns loads historical runs from disk and calibrates
// This is the standard monthly recalibration path
func CalibrateFromOfflineRuns(maxTrades int) (decision.FailureThresholds, int, string, error) {
	fg := NewFeedbackGenerator("current_run", 0.0, DefaultFeedbackConfig())
	return calibrateFailureThresholds("", fg, maxTrades)
}
