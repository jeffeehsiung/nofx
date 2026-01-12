package backtest

import (
	"fmt"
	"nofx/decision"
	"nofx/logger"
	"sort"
)

// calibrateFailureThresholds builds calibrated thresholds from historical trades.
// It loads recent backtest runs (excluding the current run) and converts closed
// positions into decision.TradeOutcome samples for the ThresholdCalibrator.
func calibrateFailureThresholds(currentRunID string, fg *FeedbackGenerator, maxTrades int) (decision.FailureThresholds, int, string, error) {
	thresholds := decision.DefaultFailureThresholds()

	outcomes, err := collectCalibrationOutcomes(currentRunID, fg, maxTrades)
	if err != nil {
		return thresholds, len(outcomes), "", err
	}

	if len(outcomes) < 30 {
		return thresholds, len(outcomes), "", fmt.Errorf("insufficient data for calibration: need at least 30 trades, got %d", len(outcomes))
	}

	calibrator := decision.NewThresholdCalibrator()
	if err := calibrator.CalibrateFromHistory(outcomes); err != nil {
		return thresholds, len(outcomes), "", err
	}

	thresholds = calibrator.ApplyToAnalyzer()
	summary := calibrator.GetCalibrationSummary()
	return thresholds, len(outcomes), summary, nil
}

// collectCalibrationOutcomes gathers closed trades across recent runs (excluding
// the current run) until maxTrades is reached. Newest runs are processed first
// using their UpdatedAt metadata.
func collectCalibrationOutcomes(currentRunID string, fg *FeedbackGenerator, maxTrades int) ([]decision.TradeOutcome, error) {
	runIDs, err := LoadRunIDs()
	if err != nil {
		return nil, fmt.Errorf("load run IDs: %w", err)
	}

	type runWithTime struct {
		runID   string
		updateT int64
	}

	runs := make([]runWithTime, 0, len(runIDs))
	for _, id := range runIDs {
		if id == currentRunID {
			continue
		}

		meta, err := LoadRunMetadata(id)
		if err != nil {
			logger.Infof("skip calibration source %s: %v", id, err)
			continue
		}

		runs = append(runs, runWithTime{runID: id, updateT: meta.UpdatedAt.Unix()})
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].updateT > runs[j].updateT
	})

	outcomes := make([]decision.TradeOutcome, 0, maxTrades)
	for _, run := range runs {
		if len(outcomes) >= maxTrades {
			break
		}

		events, err := LoadTradeEvents(run.runID)
		if err != nil || len(events) == 0 {
			if err != nil {
				logger.Infof("skip calibration run %s: %v", run.runID, err)
			}
			continue
		}

		closed := fg.extractClosedPositions(events)
		for _, pos := range closed {
			outcomes = append(outcomes, tradeOutcomeFromClosedPosition(pos))
			if len(outcomes) >= maxTrades {
				break
			}
		}
	}

	return outcomes, nil
}

// tradeOutcomeFromClosedPosition converts a ClosedPosition into the metrics
// required by the ThresholdCalibrator, handling missing market data gracefully.
func tradeOutcomeFromClosedPosition(pos ClosedPosition) decision.TradeOutcome {
	volumeAtEntry := 1.0
	oiAtEntry := 0.0
	volumeDuring := 0.0
	oiDuring := 0.0

	if pos.EntryMarketData != nil {
		volumeAtEntry = extractVolumeRatio(pos.EntryMarketData)
		oiAtEntry = extractOIDelta(pos.EntryMarketData)
	}

	if pos.EntryMarketData != nil && pos.ExitMarketData != nil {
		volumeDuring = extractVolumeRatio(pos.ExitMarketData) - extractVolumeRatio(pos.EntryMarketData)
		oiDuring = extractOIDelta(pos.ExitMarketData) - extractOIDelta(pos.EntryMarketData)
	}

	pnlPct := 0.0
	if pos.EntryPrice > 0 {
		if pos.Side == "long" {
			pnlPct = ((pos.ExitPrice - pos.EntryPrice) / pos.EntryPrice) * 100 * float64(pos.Leverage)
		} else {
			pnlPct = ((pos.EntryPrice - pos.ExitPrice) / pos.EntryPrice) * 100 * float64(pos.Leverage)
		}
	}

	holdingMinutes := int(pos.ExitTime.Sub(pos.EntryTime).Minutes())
	if holdingMinutes < 0 {
		holdingMinutes = 0
	}

	return decision.TradeOutcome{
		Symbol:            pos.Symbol,
		Profitable:        pos.RealizedPnL > 0,
		VolumeAtEntry:     volumeAtEntry,
		OIAtEntry:         oiAtEntry,
		VolumeDuringTrade: volumeDuring,
		OIDuringTrade:     oiDuring,
		EntrySpread:       pos.EntryEvent.Spread,
		ExitSpread:        pos.ExitEvent.Spread,
		EntryDepth:        pos.EntryEvent.Depth,
		ExitDepth:         pos.ExitEvent.Depth,
		HoldingMinutes:    holdingMinutes,
		PnLPct:            pnlPct,
	}
}
