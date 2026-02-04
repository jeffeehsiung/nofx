package decision

import (
	"math/rand"
	"testing"
)

func TestThresholdCalibrator_Basic(t *testing.T) {
	calibrator := NewThresholdCalibrator()

	// Generate synthetic trade data with clear patterns
	trades := generateSyntheticTrades(100)

	err := calibrator.CalibrateFromHistory(trades)
	if err != nil {
		t.Fatalf("Calibration failed: %v", err)
	}

	// Verify thresholds are within reasonable ranges
	if calibrator.WeakVolumeThreshold < 0.5 || calibrator.WeakVolumeThreshold > 1.0 {
		t.Errorf("WeakVolumeThreshold out of range: %.2f", calibrator.WeakVolumeThreshold)
	}

	if calibrator.WeakOIThreshold < 0.1 || calibrator.WeakOIThreshold > 0.8 {
		t.Errorf("WeakOIThreshold out of range: %.2f", calibrator.WeakOIThreshold)
	}

	if calibrator.VolumeDecayThreshold > -0.10 || calibrator.VolumeDecayThreshold < -0.50 {
		t.Errorf("VolumeDecayThreshold out of range: %.2f", calibrator.VolumeDecayThreshold)
	}

	summary := calibrator.GetCalibrationSummary()
	if len(summary) < 100 {
		t.Error("Calibration summary too short")
	}
}

func TestThresholdCalibrator_InsufficientData(t *testing.T) {
	calibrator := NewThresholdCalibrator()

	// Only 10 trades - should fail
	trades := generateSyntheticTrades(10)

	err := calibrator.CalibrateFromHistory(trades)
	if err == nil {
		t.Error("Expected error with insufficient data, got nil")
	}
}

func TestThresholdCalibrator_Percentile(t *testing.T) {
	calibrator := NewThresholdCalibrator()

	values := []float64{1.0, 2.0, 3.0, 4.0, 5.0, 6.0, 7.0, 8.0, 9.0, 10.0}

	p25 := calibrator.percentile(values, 0.25)
	p50 := calibrator.percentile(values, 0.50)
	p75 := calibrator.percentile(values, 0.75)

	if p25 < 2.0 || p25 > 4.0 {
		t.Errorf("25th percentile unexpected: %.2f", p25)
	}

	if p50 < 5.0 || p50 > 6.0 {
		t.Errorf("50th percentile unexpected: %.2f", p50)
	}

	if p75 < 7.0 || p75 > 8.5 {
		t.Errorf("75th percentile unexpected: %.2f", p75)
	}
}

func TestThresholdCalibrator_ApplyToAnalyzer(t *testing.T) {
	calibrator := NewThresholdCalibrator()
	trades := generateSyntheticTrades(100)
	err := calibrator.CalibrateFromHistory(trades)
	if err != nil {
		t.Fatalf("Calibration failed: %v", err)
	}

	thresholds := calibrator.ApplyToAnalyzer()

	// Verify all threshold fields are populated
	if thresholds.WeakVolumeThreshold == 0 {
		t.Error("WeakVolumeThreshold not set")
	}
	if thresholds.SpreadWorseningMultiple == 0 {
		t.Error("SpreadWorseningMultiple not set")
	}
}

// generateSyntheticTrades creates realistic synthetic trade data for testing
func generateSyntheticTrades(count int) []TradeOutcome {
	rng := rand.New(rand.NewSource(42)) // Deterministic for testing

	trades := make([]TradeOutcome, count)

	for i := 0; i < count; i++ {
		isProfitable := rng.Float64() > 0.40 // 60% win rate

		// Winning trades tend to have better entry conditions
		volumeBase := 0.95
		oiBase := 0.40
		if !isProfitable {
			volumeBase = 0.75 // Lower volume for losers
			oiBase = 0.25     // Lower OI for losers
		}

		// Add noise
		volume := volumeBase + (rng.Float64()-0.5)*0.2
		oi := oiBase + (rng.Float64()-0.5)*0.15

		// During-trade metrics
		volumeDuring := 0.0
		oiDuring := 0.0
		if !isProfitable {
			volumeDuring = -0.35 + (rng.Float64()-0.5)*0.1 // Losers see decay
			oiDuring = -0.25 + (rng.Float64()-0.5)*0.1
		} else {
			volumeDuring = -0.10 + (rng.Float64()-0.5)*0.1 // Winners more stable
			oiDuring = -0.05 + (rng.Float64()-0.5)*0.1
		}

		// Liquidity metrics
		entrySpread := 0.001 + rng.Float64()*0.002
		exitSpread := entrySpread * (1.0 + rng.Float64()*0.5)
		if !isProfitable {
			exitSpread = entrySpread * (2.0 + rng.Float64()*1.0) // Losers see worse spread
		}

		entryDepth := 100000 + rng.Float64()*50000
		exitDepth := entryDepth * (0.8 + rng.Float64()*0.3)
		if !isProfitable {
			exitDepth = entryDepth * (0.4 + rng.Float64()*0.2) // Losers see depth shrinkage
		}

		pnlPct := 0.0
		if isProfitable {
			pnlPct = 2.0 + rng.Float64()*8.0 // 2-10% profit
		} else {
			pnlPct = -(1.0 + rng.Float64()*5.0) // -1 to -6% loss
		}

		trades[i] = TradeOutcome{
			Symbol:            "TESTUSDT",
			Profitable:        isProfitable,
			VolumeAtEntry:     volume,
			OIAtEntry:         oi,
			VolumeDuringTrade: volumeDuring,
			OIDuringTrade:     oiDuring,
			EntrySpread:       entrySpread,
			ExitSpread:        exitSpread,
			EntryDepth:        entryDepth,
			ExitDepth:         exitDepth,
			HoldingMinutes:    30 + rng.Intn(120),
			PnLPct:            pnlPct,
		}
	}

	return trades
}

func BenchmarkThresholdCalibration(b *testing.B) {
	trades := generateSyntheticTrades(200)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calibrator := NewThresholdCalibrator()
		if err := calibrator.CalibrateFromHistory(trades); err != nil {
			b.Fatalf("Calibration failed: %v", err)
		}
	}
}
