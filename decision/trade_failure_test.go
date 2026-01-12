package decision

import (
	"testing"
	"time"
)

// TestIsChasing tests detection of chasing behavior
func TestIsChasing(t *testing.T) {
	tests := []struct {
		name         string
		order        *RecentOrder
		shouldDetect bool
	}{
		{
			name: "High slippage exceeds budget",
			order: &RecentOrder{
				EntrySlippage:       0.08,
				EntrySlippageBudget: 0.03,
				EntryFillTime:       1000,
				SignalTime:          100,
			},
			shouldDetect: true,
		},
		{
			name: "Normal execution",
			order: &RecentOrder{
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
			result := isChasing(tt.order, nil)
			if result != tt.shouldDetect {
				t.Errorf("expected %v, got %v", tt.shouldDetect, result)
			}
		})
	}
}

// TestIsFalseBreakout tests detection of false breakouts
func TestIsFalseBreakout(t *testing.T) {
	tests := []struct {
		name         string
		order        *RecentOrder
		shouldDetect bool
	}{
		{
			name: "No volume confirmation",
			order: &RecentOrder{
				VolumeAtEntry:  0.85,
				OIDeltaAtEntry: 0.02,
			},
			shouldDetect: true,
		},
		{
			name: "Strong confirmation",
			order: &RecentOrder{
				VolumeAtEntry:  1.35,
				OIDeltaAtEntry: 0.25,
			},
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isFalseBreakout(tt.order, nil)
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
		order        *RecentOrder
		shouldDetect bool
	}{
		{
			name: "Stop way too tight",
			order: &RecentOrder{
				StopDistanceVsATR: 0.8,
			},
			shouldDetect: true,
		},
		{
			name: "Stop at minimum",
			order: &RecentOrder{
				StopDistanceVsATR: 1.5,
			},
			shouldDetect: false,
		},
		{
			name: "Comfortable stop",
			order: &RecentOrder{
				StopDistanceVsATR: 2.5,
			},
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isStopTooTight(tt.order, nil)
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
		order        *RecentOrder
		shouldDetect bool
	}{
		{
			name: "Both volume and OI collapse",
			order: &RecentOrder{
				VolumeDeltaDuringTrade: -0.40,
				OIDeltaDuringTrade:     -0.25,
			},
			shouldDetect: true,
		},
		{
			name: "Strong momentum sustained",
			order: &RecentOrder{
				VolumeDeltaDuringTrade: 0.10,
				OIDeltaDuringTrade:     0.18,
			},
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMomentumDecay(tt.order, nil)
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
		order        *RecentOrder
		shouldDetect bool
	}{
		{
			name: "Severe spread and depth deterioration",
			order: &RecentOrder{
				EntrySpread: 0.01,
				ExitSpread:  0.04,
				EntryDepth:  1000000,
				ExitDepth:   200000,
			},
			shouldDetect: true,
		},
		{
			name: "Stable conditions",
			order: &RecentOrder{
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
			result := isLiquidityDried(tt.order, nil)
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
		order        *RecentOrder
		shouldDetect bool
	}{
		{
			name: "Unfavorable regime (sideways, choppy)",
			order: &RecentOrder{
				MarketRegime: "sideways",
				ChopScore:    0.70,
			},
			shouldDetect: true,
		},
		{
			name: "Favorable regime (trending)",
			order: &RecentOrder{
				MarketRegime:  "trending",
				ChopScore:     0.20,
				TrendStrength: 0.50,
			},
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isStopHitRegimeChange(tt.order, nil)
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
		order        *RecentOrder
		shouldDetect bool
	}{
		{
			name: "Large give-back from peak",
			order: &RecentOrder{
				MaxFavorableExcursion: 0.08,
				GiveBackFromPeak:      0.06,
			},
			shouldDetect: true,
		},
		{
			name: "Controlled exit",
			order: &RecentOrder{
				MaxFavorableExcursion: 0.05,
				GiveBackFromPeak:      0.01,
			},
			shouldDetect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLateExitGiveBack(tt.order, nil)
			if result != tt.shouldDetect {
				t.Errorf("expected %v, got %v", tt.shouldDetect, result)
			}
		})
	}
}

// TestAnalyzeFailedTrade tests end-to-end analysis
func TestAnalyzeFailedTrade(t *testing.T) {
	tests := []struct {
		name          string
		order         *RecentOrder
		expectReason  TradeFailureReason
		minConfidence float64
	}{
		{
			name: "Clear chasing case",
			order: &RecentOrder{
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
			order: &RecentOrder{
				VolumeAtEntry:     0.70,
				OIDeltaAtEntry:    0.05,
				RealizedPnL:       -50,
				StopDistanceVsATR: 2.0, // avoid being flagged as stop too tight
			},
			expectReason:  ReasonFalseBreakoutV2,
			minConfidence: 0.7,
		},
		{
			name: "Stop too tight case",
			order: &RecentOrder{
				StopDistanceVsATR: 0.7,
				RealizedPnL:       -30,
			},
			expectReason:  ReasonStopTooTight,
			minConfidence: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnalyzeFailedTrade(tt.order)

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

			if len(result.Evidence) == 0 {
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

// TestAnalyzeFailedTrade_NilOrder tests nil order handling
func TestAnalyzeFailedTrade_NilOrder(t *testing.T) {
	result := AnalyzeFailedTrade(nil)
	if result != nil {
		t.Error("expected nil result for nil order")
	}
}

// BenchmarkAnalyzeFailedTrade benchmarks the main analysis function
func BenchmarkAnalyzeFailedTrade(b *testing.B) {
	order := &RecentOrder{
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
		EntryTime:             time.Now().Format(time.RFC3339),
		ExitTime:              time.Now().Add(time.Hour).Format(time.RFC3339),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		AnalyzeFailedTrade(order)
	}
}
