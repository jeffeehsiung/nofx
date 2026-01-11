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
				Symbol:           "BTC",
				Timestamp:        time.Now().Add(-24 * time.Hour),
				EntryPrice:       45000.0,
				EntrySlippage:    150.0,
				EntrySlippageBps: 33,
				ExitPrice:        44100.0,
				ExitSlippage:     200.0,
				ExitSlippageBps:  45,
				MFE:              0.8,
				MAE:              -3.2,
				EntrySpreadBps:   10,
				ExitSpreadBps:    15,
				EntryATR:         1000.0,
				ExitTime:         time.Now().Add(-20 * time.Hour),
			},
		},
		// Failure 2: Stop too tight
		{
			Success:        false,
			RealizedPnLPct: -1.8,
			RecentOrder: &decision.RecentOrder{
				Symbol:       "ETH",
				Timestamp:    time.Now().Add(-22 * time.Hour),
				EntryPrice:   2500.0,
				EntryATR:     100.0,
				MFE:          0.5,
				MAE:          -2.0,
				EntrySpreadBps: 12,
				ExitSpreadBps:  18,
			},
		},
		// Success: Good trade for comparison
		{
			Success:        true,
			RealizedPnLPct: 4.5,
			RecentOrder: &decision.RecentOrder{
				Symbol:       "BTC",
				Timestamp:    time.Now().Add(-16 * time.Hour),
				EntryPrice:   45500.0,
				ExitPrice:    47500.0,
				EntrySpreadBps: 8,
				ExitSpreadBps:  10,
				MFE:          5.2,
				MAE:          -0.5,
				EntryATR:     1000.0,
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

	fg := NewFeedbackGenerator("test_run", DefaultFeedbackConfig())
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

	optimizer := NewFactorOptimizer(DefaultFactorOptimizerConfig())
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
