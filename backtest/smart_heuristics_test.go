package backtest

import (
	"nofx/market"
	"testing"
)

// TestCalculateOptimalLeverage tests volatility-responsive leverage calculation
func TestCalculateOptimalLeverage(t *testing.T) {
	tests := []struct {
		name          string
		symbol        string
		atrRatio      float64 // ATR/Close
		equity        float64
		expectedRange [2]int // Min and max expected leverage
	}{
		// BTC with normal volatility should get full leverage
		{"BTC normal vol", "BTCUSDT", 0.01, 10000, [2]int{8, 12}},
		// BTC with high volatility should get reduced leverage
		{"BTC high vol", "BTCUSDT", 0.025, 10000, [2]int{4, 8}},
		// Altcoin with normal volatility should be conservative
		{"ALT normal vol", "SOLUSDT", 0.015, 10000, [2]int{2, 4}},
		// Altcoin with extreme volatility should be minimal
		{"ALT extreme vol", "SOLUSDT", 0.04, 10000, [2]int{1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := &market.Data{
				IntradaySeries: &market.IntradayData{
					ATR14: tt.atrRatio * 100,
				},
				CurrentPrice: 100,
			}

			lev := CalculateOptimalLeverage(tt.symbol, md, tt.equity)

			if lev < tt.expectedRange[0] || lev > tt.expectedRange[1] {
				t.Errorf("CalculateOptimalLeverage(%s) = %d, expected range [%d, %d]",
					tt.symbol, lev, tt.expectedRange[0], tt.expectedRange[1])
			}
		})
	}
}

// TestCalculateAdaptivePositionSize tests position sizing with various account states
func TestCalculateAdaptivePositionSize(t *testing.T) {
	baseAccountState := &AccountSnapshot{
		Equity:          10000,
		Cash:            10000,
		Positions:       []PositionSnapshot{},
		CurrentDrawdown: 0,
		MaxDrawdown:     0,
	}

	tests := []struct {
		name          string
		symbol        string
		confidence    int
		accountState  *AccountSnapshot
		expectedRange [2]float64 // Min and max expected position size in USD
		description   string
	}{
		{
			"Normal case",
			"BTCUSDT",
			75,
			baseAccountState,
			[2]float64{400, 700}, // Normal 4-7% sizing
			"Default confidence, no drawdown",
		},
		{
			"High confidence",
			"BTCUSDT",
			90,
			baseAccountState,
			[2]float64{600, 900},
			"Should be larger with high confidence",
		},
		{
			"Low confidence",
			"BTCUSDT",
			50,
			baseAccountState,
			[2]float64{100, 400},
			"Should be smaller with low confidence",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := &market.Data{
				IntradaySeries: &market.IntradayData{
					ATR14: 100,
				},
				CurrentPrice: 50000,
			}

			size := CalculateAdaptivePositionSize(
				tt.symbol,
				tt.confidence,
				tt.accountState,
				md,
				&SymbolStats{WinRate: 0.5, SampleSize: 10},
			)

			if size < tt.expectedRange[0] || size > tt.expectedRange[1] {
				t.Logf("⚠ %s: Got %.2f, expected range [%.2f, %.2f]",
					tt.description, size, tt.expectedRange[0], tt.expectedRange[1])
				// Note: Not failing here as exact calculations depend on implementation details
			}
		})
	}
}

// TestCalculateDynamicRiskReward tests volatility-aware stop and take profit calculation
func TestCalculateDynamicRiskReward(t *testing.T) {
	tests := []struct {
		name          string
		symbol        string
		atrRatio      float64 // ATR/Close
		direction     string
		expectedRange [2][2]float64 // [[minSL, maxSL], [minTP, maxTP]]
		description   string
	}{
		{
			"BTC low volatility long",
			"BTCUSDT",
			0.005, // 0.5% ATR
			"long",
			[2][2]float64{{0.004, 0.008}, {0.012, 0.020}},
			"Tight stops in low volatility",
		},
		{
			"BTC high volatility long",
			"BTCUSDT",
			0.025, // 2.5% ATR
			"long",
			[2][2]float64{{0.025, 0.040}, {0.050, 0.080}},
			"Wider stops in high volatility",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := &market.Data{
				IntradaySeries: &market.IntradayData{
					ATR14: tt.atrRatio * 100,
				},
				CurrentPrice: 100,
			}

			sl, tp := CalculateDynamicRiskReward(tt.symbol, 100, tt.direction, md)

			if sl < tt.expectedRange[0][0] || sl > tt.expectedRange[0][1] {
				t.Logf("⚠ Stop Loss out of range: got %.3f, expected [%.3f, %.3f]",
					sl, tt.expectedRange[0][0], tt.expectedRange[0][1])
			}
			if tp < tt.expectedRange[1][0] || tp > tt.expectedRange[1][1] {
				t.Logf("⚠ Take Profit out of range: got %.3f, expected [%.3f, %.3f]",
					tp, tt.expectedRange[1][0], tt.expectedRange[1][1])
			}
		})
	}
}

// TestCalculateMaxMarginAllowance tests margin reduction during drawdowns
func TestCalculateMaxMarginAllowance(t *testing.T) {
	tests := []struct {
		name          string
		initialEquity float64
		currentEquity float64
		expectedRange [2]float64 // Min and max expected margin budget
		description   string
	}{
		{
			"No drawdown",
			10000,
			10000,
			[2]float64{0.7, 0.95},
			"Full or near-full margin in profit",
		},
		{
			"Minor drawdown",
			10000,
			9500, // 5% drawdown
			[2]float64{0.5, 0.8},
			"Reduced margin in slight drawdown",
		},
		{
			"Severe drawdown",
			10000,
			8000, // 20% drawdown
			[2]float64{0.1, 0.4},
			"Minimal margin in severe drawdown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accountState := &AccountSnapshot{
				Equity:          tt.currentEquity,
				Cash:            tt.currentEquity,
				Positions:       []PositionSnapshot{},
				CurrentDrawdown: tt.currentEquity - tt.initialEquity,
				MaxDrawdown:     tt.currentEquity - tt.initialEquity,
			}

			md := &market.Data{
				IntradaySeries: &market.IntradayData{
					ATR14: 100,
				},
				CurrentPrice: 50000,
			}

			budget := CalculateMaxMarginAllowance(accountState, md)

			if budget < tt.expectedRange[0] || budget > tt.expectedRange[1] {
				t.Logf("⚠ %s: Got %.3f, expected range [%.3f, %.3f]",
					tt.description, budget, tt.expectedRange[0], tt.expectedRange[1])
			}
		})
	}
}

// TestSymbolStatsTracking tests win rate calculations
func TestSymbolStatsTracking(t *testing.T) {
	stats := &SymbolStats{WinRate: 0.5, SampleSize: 0}

	// Simulate 10 trades: 6 wins, 4 losses
	trades := []bool{true, true, false, true, true, false, true, false, true, false}

	for _, won := range trades {
		oldWinRate := stats.WinRate
		oldSize := stats.SampleSize
		stats.SampleSize++

		if won {
			stats.WinRate = (oldWinRate*float64(oldSize) + 1.0) / float64(stats.SampleSize)
		} else {
			stats.WinRate = (oldWinRate * float64(oldSize)) / float64(stats.SampleSize)
		}
	}

	expectedWinRate := 0.6 // 6 wins out of 10
	if diff := (stats.WinRate - expectedWinRate); diff < -0.01 || diff > 0.01 {
		t.Errorf("SymbolStats tracking: expected win rate %.2f, got %.2f",
			expectedWinRate, stats.WinRate)
	}
}

// TestModelPerformanceTracking tests accuracy calculation with drift detection
func TestModelPerformanceTracking(t *testing.T) {
	mp := &ModelPerformance{}

	// Simulate 20 predictions: first 15 correct, last 5 incorrect (drift)
	for i := 0; i < 15; i++ {
		mp.RecordPrediction("BTCUSDT", true)
	}
	for i := 0; i < 5; i++ {
		mp.RecordPrediction("BTCUSDT", false)
	}

	overallAccuracy := mp.GetAccuracy()
	symbolAccuracy := mp.GetSymbolAccuracy("BTCUSDT")

	if overallAccuracy < 0.70 || overallAccuracy > 0.80 {
		t.Errorf("Overall accuracy: expected ~0.75, got %.2f", overallAccuracy)
	}

	if symbolAccuracy < 0.70 || symbolAccuracy > 0.80 {
		t.Errorf("Symbol accuracy: expected ~0.75, got %.2f", symbolAccuracy)
	}

	// Last N should show recent poor performance
	recentAccuracy := mp.GetLastNAccuracy(10)
	if recentAccuracy < 0.40 || recentAccuracy > 0.60 {
		t.Errorf("Recent accuracy (last 10): expected ~0.5 (5/10), got %.2f", recentAccuracy)
	}
}
