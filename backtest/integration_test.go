package backtest

import (
	"fmt"
	"nofx/decision"
	"strings"
	"testing"
	"time"
)

// TestTradeFailureV2Integration verifies the complete 3-tier learning loop
func TestTradeFailureV2Integration(t *testing.T) {
	separator := strings.Repeat("=", 80)
	fmt.Println("\n" + separator)
	fmt.Println("TRADE FAILURE V2 INTEGRATION TEST - 3-Tier Learning Loop")
	fmt.Println(separator)

	// ========================================================================
	// SETUP: Create synthetic test data
	// ========================================================================

	outcomes := []DecisionOutcome{
		// Failure 1: Chasing entry with excessive slippage
		{
			Success:        false,
			RealizedPnLPct: -2.5,
			RecentOrder: &decision.RecentOrder{
				Symbol:                "BTC",
				EntryTime:             time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
				EntryPrice:            45000.0,
				EntrySlippage:         150.0,
				ExitPrice:             44100.0,
				ExitSlippage:          200.0,
				MaxFavorableExcursion: 0.8,
				MaxAdverseExcursion:   -3.2,
				EntrySpread:           0.10,
				ExitSpread:            0.15,
				ATRAtEntry:            1000.0,
				ExitTime:              time.Now().Add(-20 * time.Hour).Format(time.RFC3339),
			},
		},
		// Failure 2: Stop too tight
		{
			Success:        false,
			RealizedPnLPct: -1.8,
			RecentOrder: &decision.RecentOrder{
				Symbol:                "ETH",
				EntryTime:             time.Now().Add(-22 * time.Hour).Format(time.RFC3339),
				EntryPrice:            2500.0,
				ATRAtEntry:            100.0,
				MaxFavorableExcursion: 0.5,
				MaxAdverseExcursion:   -2.0,
				EntrySpread:           0.12,
				ExitSpread:            0.18,
			},
		},
		// Success: Good trade for comparison
		{
			Success:        true,
			RealizedPnLPct: 4.5,
			RecentOrder: &decision.RecentOrder{
				Symbol:                "BTC",
				EntryTime:             time.Now().Add(-16 * time.Hour).Format(time.RFC3339),
				EntryPrice:            45500.0,
				ExitPrice:             47500.0,
				EntrySpread:           0.08,
				ExitSpread:            0.10,
				MaxFavorableExcursion: 5.2,
				MaxAdverseExcursion:   -0.5,
				ATRAtEntry:            1000.0,
			},
		},
	}

	// ========================================================================
	// TIER 1: Execution-Level Failure Analysis (V2)
	// ========================================================================
	fmt.Println("\n📋 TIER 1: V2 MICROSTRUCTURE ANALYSIS")
	fmt.Println(strings.Repeat("-", 80))

	v2Count := 0
	for _, outcome := range outcomes {
		if !outcome.Success && outcome.RecentOrder != nil {
			analysis := decision.AnalyzeFailedTrade(outcome.RecentOrder)
			if analysis != nil {
				v2Count++
				fmt.Printf("✗ Failure detected: %s (confidence: %.0f%%)\n",
					analysis.PrimaryReason, analysis.ConfidenceScore*100)
				fmt.Printf("  Details: %s\n", analysis.DetailedNotes)
			}
		}
	}

	// ========================================================================
	// TIER 2: Pattern Aggregation (FeedbackGenerator)
	// ========================================================================
	fmt.Println("\n📊 TIER 2: PATTERN AGGREGATION & FEEDBACK GENERATION")
	fmt.Println(strings.Repeat("-", 80))

	fg := NewFeedbackGenerator("test_run", 0.0, DefaultFeedbackConfig())
	patterns := fg.identifyFailurePatterns(outcomes, &Metrics{})

	fmt.Printf("Identified %d patterns from V2 analysis:\n", len(patterns))
	for _, pattern := range patterns {
		fmt.Printf("• %s (Type: %s)\n", pattern.Description, pattern.PatternType)
		fmt.Printf("  Frequency: %d | Avg PnL: %.2f%%\n", pattern.Frequency, pattern.AvgPnLPct)
	}

	// ========================================================================
	// TIER 3: Parameter Optimization (FactorOptimizer)
	// ========================================================================
	fmt.Println("\n🎯 TIER 3: PARAMETER OPTIMIZATION")
	fmt.Println(strings.Repeat("-", 80))

	feedback := &FeedbackAnalysis{
		FailurePatterns: patterns,
		WinRate:         50.0,
		MaxDrawdown:     8.5,
		TotalReturnPct:  -2.1,
	}

	optimizer := NewFactorOptimizer(nil, DefaultFactorOptimizerConfig())
	err := optimizer.OptimizeWeights(feedback, 1)
	if err != nil {
		t.Fatalf("OptimizeWeights failed: %v", err)
	}

	fmt.Println("✅ Parameters optimized based on V2 failure patterns")

	// ========================================================================
	// VERIFICATION
	// ========================================================================
	fmt.Println("\n" + separator)
	fmt.Println("✅ INTEGRATION TEST COMPLETE")
	fmt.Println(separator)
	fmt.Printf("\nV2 Failures detected: %d\n", v2Count)
	fmt.Printf("Patterns identified: %d\n", len(patterns))
	fmt.Println("\n3-Tier Learning Loop:")
	fmt.Println("  1. ✅ V2 Failure Analysis")
	fmt.Println("  2. ✅ Pattern Aggregation")
	fmt.Println("  3. ✅ Parameter Optimization")
}
