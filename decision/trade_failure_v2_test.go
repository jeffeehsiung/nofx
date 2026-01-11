package decision

import (
	"testing"
	"time"
)

// TestIsChasing tests detection of chasing behavior
func TestIsChasing(t *testing.T) {
	tests := []struct {
		name         string
		order        *RecentOrderV2
		shouldDetect bool
	}{
		{
			name: "High slippage exceeds budget",
			order: &RecentOrderV2{
				EntrySlippage:       0.08,
				EntrySlippageBudget: 0.03,
				EntryFillTime:       1000,
				SignalTime:          100,
			},
			shouldDetect: true,
		},
		{
			name: "Normal execution",
			order: &RecentOrderV2{
				EntrySlippage:       0.01,
				EntrySlippageBudget: 0.03,
				EntryFillTime:       200,
				SignalTime:          100,
			},
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isChasing(tt.order)
			if result != tt.shouldDetect {
				t.Errorf("expected %v, got %v", tt.shouldDetect, result)
			}
		})
	}
}

// TestIsFalseBreakoutV2 tests detection of false breakouts
func TestIsFalseBreakoutV2(t *testing.T) {
	tests := []struct {
		name         string
		order        *RecentOrderV2
		shouldDetect bool
	}{
		{
			name: "No volume confirmation",
			order: &RecentOrderV2{
				VolumeAtEntry:           0.85,
				VolumeBaselinePercent:   100.0,
				OIDeltaAtEntry:          0.02,
				OIBaselineChangePercent: 100.0,
			},
			shouldDetect: true,
		},
		{
			name: "Strong confirmation",
			order: &RecentOrderV2{
				VolumeAtEntry:           1.35,
				VolumeBaselinePercent:   100.0,
				OIDeltaAtEntry:          0.25,
				OIBaselineChangePercent: 100.0,
			},
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isFalseBreakoutV2(tt.order)
			if result != tt.shouldDetect {
				t.Errorf("expected %v, got %v", tt.shouldDetect, result)
			}
		})
	}
}

// TestIsStopTooTight tests stop distance validation
func TestIsStopTooTight(t *testing.T) {
	tests := []struct {
		name         string
		order        *RecentOrderV2
		shouldDetect bool
	}{
		{
			name: "Stop way too tight",
			order: &RecentOrderV2{
				StopDistanceVsATR: 0.8,
			},
			shouldDetect: true,
		},
		{
			name: "Stop at minimum",
			order: &RecentOrderV2{
				StopDistanceVsATR: 1.5,
			},
			shouldDetect: false,
		},
		{
			name: "Comfortable stop",
			order: &RecentOrderV2{
				StopDistanceVsATR: 2.5,
			},
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isStopTooTight(tt.order)
			if result != tt.shouldDetect {
				t.Errorf("expected %v, got %v", tt.shouldDetect, result)
			}
		})
	}
}

// TestIsMomentumDecay tests momentum fade detection
func TestIsMomentumDecay(t *testing.T) {
	tests := []struct {
		name         string
		order        *RecentOrderV2
		shouldDetect bool
	}{
		{
			name: "Both volume and OI collapse",
			order: &RecentOrderV2{
				VolumeDeltaDuringTrade: -0.40,
				OIDeltaDuringTrade:     -0.25,
			},
			shouldDetect: true,
		},
		{
			name: "Strong momentum sustained",
			order: &RecentOrderV2{
				VolumeDeltaDuringTrade: 0.10,
				OIDeltaDuringTrade:     0.18,
			},
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMomentumDecay(tt.order)
			if result != tt.shouldDetect {
				t.Errorf("expected %v, got %v", tt.shouldDetect, result)
			}
		})
	}
}

// TestIsLiquidityDried tests spread/depth deterioration
func TestIsLiquidityDried(t *testing.T) {
	tests := []struct {
		name         string
		order        *RecentOrderV2
		shouldDetect bool
	}{
		{
			name: "Severe spread and depth deterioration",
			order: &RecentOrderV2{
				EntrySpread: 0.01,
				ExitSpread:  0.04,
				EntryDepth:  1000000,
				ExitDepth:   200000,
			},
			shouldDetect: true,
		},
		{
			name: "Stable conditions",
			order: &RecentOrderV2{
				EntrySpread: 0.02,
				ExitSpread:  0.022,
				EntryDepth:  250000,
				ExitDepth:   240000,
			},
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLiquidityDried(tt.order)
			if result != tt.shouldDetect {
				t.Errorf("expected %v, got %v", tt.shouldDetect, result)
			}
		})
	}
}

// TestIsStopHitRegimeChange tests stop hit due to regime change
func TestIsStopHitRegimeChange(t *testing.T) {
	tests := []struct {
		name         string
		order        *RecentOrderV2
		shouldDetect bool
	}{
		{
			name: "Strong trend reversal",
			order: &RecentOrderV2{
				TrendStrengthAtEntry: 0.85,
				TrendStrengthAtExit:  -0.75,
			},
			shouldDetect: true,
		},
		{
			name: "Trend maintained",
			order: &RecentOrderV2{
				TrendStrengthAtEntry: 0.70,
				TrendStrengthAtExit:  0.65,
			},
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isStopHitRegimeChange(tt.order)
			if result != tt.shouldDetect {
				t.Errorf("expected %v, got %v", tt.shouldDetect, result)
			}
		})
	}
}

// TestIsLateExitGiveBack tests exit timing issues
func TestIsLateExitGiveBack(t *testing.T) {
	tests := []struct {
		name         string
		order        *RecentOrderV2
		shouldDetect bool
	}{
		{
			name: "Large give-back from peak",
			order: &RecentOrderV2{
				MaxFavorableExcursion: 0.08,
				GiveBackFromPeak:      0.06,
			},
			shouldDetect: true,
		},
		{
			name: "Controlled exit",
			order: &RecentOrderV2{
				MaxFavorableExcursion: 0.05,
				GiveBackFromPeak:      0.01,
			},
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLateExitGiveBack(tt.order)
			if result != tt.shouldDetect {
				t.Errorf("expected %v, got %v", tt.shouldDetect, result)
			}
		})
	}
}

// TestAnalyzeFailedTradeV2 tests end-to-end analysis
func TestAnalyzeFailedTradeV2(t *testing.T) {
	tests := []struct {
		name          string
		order         *RecentOrderV2
		expectReason  TradeFailureReasonV2
		minConfidence float64
	}{
		{
			name: "Clear chasing case",
			order: &RecentOrderV2{
				EntrySlippage:       0.09,
				EntrySlippageBudget: 0.02,
				EntryFillTime:       3000,
				SignalTime:          100,
				RealizedPnL:         -100,
			},
			expectReason:  ReasonChasingEntry,
			minConfidence: 0.7,
		},
		{
			name: "False breakout case",
			order: &RecentOrderV2{
				VolumeAtEntry:           0.70,
				VolumeBaselinePercent:   100.0,
				OIDeltaAtEntry:          0.05,
				OIBaselineChangePercent: 100.0,
				RealizedPnL:             -50,
				StopDistanceVsATR:       2.0, // avoid being flagged as stop too tight
			},
			expectReason:  ReasonFalseBreakoutV2,
			minConfidence: 0.7,
		},
		{
			name: "Stop too tight case",
			order: &RecentOrderV2{
				StopDistanceVsATR: 0.7,
				RealizedPnL:       -30,
			},
			expectReason:  ReasonStopTooTight,
			minConfidence: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnalyzeFailedTradeV2(tt.order)

			if result == nil {
				t.Errorf("analysis returned nil")
				return
			}

			if result.PrimaryReason != tt.expectReason {
				t.Errorf("expected reason %s, got %s", tt.expectReason, result.PrimaryReason)
			}

			if result.ConfidenceScore < tt.minConfidence {
				t.Errorf("confidence %.2f < minimum %.2f", result.ConfidenceScore, tt.minConfidence)
			}

			if result.Evidence == nil || len(result.Evidence) == 0 {
				t.Errorf("no evidence provided")
			}

			if result.DetailedNotes == "" {
				t.Errorf("no detailed notes provided")
			}

			if result.Recommendation == "" {
				t.Errorf("no recommendation provided")
			}
		})
	}
}

// TestAnalyzeFailedTradeV2_NilOrder tests nil order handling
func TestAnalyzeFailedTradeV2_NilOrder(t *testing.T) {
	result := AnalyzeFailedTradeV2(nil)
	if result != nil {
		t.Error("expected nil result for nil order")
	}
}

// BenchmarkAnalyzeFailedTradeV2 benchmarks the main analysis function
func BenchmarkAnalyzeFailedTradeV2(b *testing.B) {
	order := &RecentOrderV2{
		Symbol:                "BTC/USDT",
		EntrySlippage:         0.05,
		EntrySlippageBudget:   0.02,
		StopDistanceVsATR:     1.2,
		ATRAtEntry:            100,
		VolumeAtEntry:         0.8,
		OIDeltaAtEntry:        0.1,
		MaxFavorableExcursion: 0.03,
		GiveBackFromPeak:      0.02,
		RealizedPnL:           -50,
		EntryTime:             time.Now(),
		ExitTime:              time.Now().Add(time.Hour),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AnalyzeFailedTradeV2(order)
	}
}
