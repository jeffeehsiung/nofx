package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	"nofx/backtest"
	"nofx/config"
	"nofx/decision"
)

type outputPayload struct {
	GeneratedAt   time.Time                  `json:"generated_at"`
	SampleSize    int                        `json:"sample_size"`
	Thresholds    decision.FailureThresholds `json:"thresholds"`
	CalibrationOK bool                       `json:"calibration_ok"`
	Summary       string                     `json:"summary,omitempty"`
}

func main() {
	var (
		maxTrades         int
		output            string
		exclude           string
		driftReportPath   string
		currentThreshPath string
	)
	flag.IntVar(&maxTrades, "max-trades", 1000, "maximum number of closed trades to use")
	flag.StringVar(&output, "output", "./config/calibrated_thresholds.json", "output JSON filepath")
	flag.StringVar(&exclude, "exclude-run", "", "run ID to exclude (e.g., current run)")
	flag.StringVar(&driftReportPath, "drift-report", "", "path to save drift detection report (optional)")
	flag.StringVar(&currentThreshPath, "current-thresholds", "", "current thresholds JSON for drift comparison (optional)")
	flag.Parse()

	thresholds := decision.DefaultFailureThresholds()
	sampleSize := 0
	summary := ""
	ok := false

	var err error
	outcomes, err := collectOutcomes(exclude, maxTrades)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error collecting outcomes: %v\n", err)
	} else if len(outcomes) >= 30 {
		calibrator := decision.NewThresholdCalibrator()
		if err := calibrator.CalibrateFromHistory(outcomes); err != nil {
			fmt.Fprintf(os.Stderr, "calibration failed: %v\n", err)
		} else {
			thresholds = calibrator.ApplyToAnalyzer()
			sampleSize = len(outcomes)
			summary = calibrator.GetCalibrationSummary()
			ok = true
		}
	}

	// Check for threshold drift if comparing with existing config
	if currentThreshPath != "" {
		oldThresholdsJSON, _, err := config.LoadCalibratedThresholdsJSON(currentThreshPath)
		newThresholdsJSON := config.FailureThresholdsJSON{
			WeakVolumeThreshold:      thresholds.WeakVolumeThreshold,
			WeakOIThreshold:          thresholds.WeakOIThreshold,
			PrematureVolumeThreshold: thresholds.PrematureVolumeThreshold,
			PrematureOIThreshold:     thresholds.PrematureOIThreshold,
			VolumeDecayThreshold:     thresholds.VolumeDecayThreshold,
			OIDecayThreshold:         thresholds.OIDecayThreshold,
			SpreadWorseningMultiple:  thresholds.SpreadWorseningMultiple,
			DepthReductionThreshold:  thresholds.DepthReductionThreshold,
		}
		if err == nil {
			alerts := config.CheckThresholdDriftJSON(oldThresholdsJSON, newThresholdsJSON, config.Features().DriftAlertThresholdPct)
			config.NotifyThresholdDrift(alerts, config.Features().VerboseDriftLogging)

			// Save drift report if requested
			if driftReportPath != "" {
				if err := config.SaveThresholdDriftReport(driftReportPath, oldThresholdsJSON, newThresholdsJSON, alerts); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to save drift report: %v\n", err)
				}
			}
		}
	}

	payload := outputPayload{
		GeneratedAt:   time.Now().UTC(),
		SampleSize:    sampleSize,
		Thresholds:    thresholds,
		CalibrationOK: ok,
		Summary:       summary,
	}

	if err := writeJSON(output, payload); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write output: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote %s (sample=%d, ok=%v)\n", output, sampleSize, ok)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(dirOf(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func dirOf(path string) string {
	if i := len(path) - 1; i >= 0 {
		for ; i >= 0; i-- {
			if path[i] == '/' {
				return path[:i]
			}
		}
	}
	return "."
}

// collectOutcomes gathers closed positions from recent runs and converts them into
// decision.TradeOutcome samples for calibration.
func collectOutcomes(excludeRun string, maxTrades int) ([]decision.TradeOutcome, error) {
	runIDs, err := backtest.LoadRunIDs()
	if err != nil {
		return nil, err
	}

	type runWithTime struct {
		id string
		ts int64
	}
	runs := make([]runWithTime, 0, len(runIDs))
	for _, id := range runIDs {
		if id == excludeRun {
			continue
		}
		if meta, err := backtest.LoadRunMetadata(id); err == nil {
			runs = append(runs, runWithTime{id: id, ts: meta.UpdatedAt.Unix()})
		}
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].ts > runs[j].ts })

	outcomes := make([]decision.TradeOutcome, 0, maxTrades)
	fg := backtest.NewFeedbackGenerator("recalibrate", backtest.DefaultFeedbackConfig())

	for _, r := range runs {
		if len(outcomes) >= maxTrades {
			break
		}
		events, err := backtest.LoadTradeEvents(r.id)
		if err != nil || len(events) == 0 {
			continue
		}
		closed := extractClosedPositions(events)
		for _, pos := range closed {
			outcomes = append(outcomes, tradeOutcomeFromClosedPosition(pos))
			if len(outcomes) >= maxTrades {
				break
			}
		}
	}
	_ = fg // reserved for future enrichment
	return outcomes, nil
}

// extractClosedPositions matches open and close events to create complete trade records
func extractClosedPositions(events []backtest.TradeEvent) []backtest.ClosedPosition {
	var closed []backtest.ClosedPosition
	open := make(map[string]backtest.TradeEvent) // key: symbol:side
	for _, e := range events {
		if e.LiquidationFlag {
			continue
		}
		key := fmt.Sprintf("%s:%s", e.Symbol, e.Side)
		switch e.Action {
		case "open":
			open[key] = e
		case "close":
			if o, ok := open[key]; ok {
				closed = append(closed, backtest.ClosedPosition{
					Symbol:      e.Symbol,
					Side:        e.Side,
					EntryTime:   time.UnixMilli(o.Timestamp),
					ExitTime:    time.UnixMilli(e.Timestamp),
					EntryPrice:  o.Price,
					ExitPrice:   e.Price,
					Quantity:    e.Quantity,
					Leverage:    o.Leverage,
					RealizedPnL: e.RealizedPnL,
					EntryEvent:  o,
					ExitEvent:   e,
				})
				delete(open, key)
			}
		}
	}
	return closed
}

// tradeOutcomeFromClosedPosition converts a ClosedPosition into TradeOutcome metrics
func tradeOutcomeFromClosedPosition(pos backtest.ClosedPosition) decision.TradeOutcome {
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
		VolumeAtEntry:     1.0, // unknown here; neutral baseline
		OIAtEntry:         0.0, // unknown here
		VolumeDuringTrade: 0.0, // unknown here
		OIDuringTrade:     0.0, // unknown here
		EntrySpread:       pos.EntryEvent.Spread,
		ExitSpread:        pos.ExitEvent.Spread,
		EntryDepth:        pos.EntryEvent.Depth,
		ExitDepth:         pos.ExitEvent.Depth,
		HoldingMinutes:    holdingMinutes,
		PnLPct:            pnlPct,
	}
}
