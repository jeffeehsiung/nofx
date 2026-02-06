# Feedback System - Live Trading Analysis & Prompt Evolution

## Overview

The feedback system automatically analyzes your live trading performance and provides:
1. **Real-time Performance Metrics** - Win rate, profit factor, Sharpe ratio, drawdown
2. **Pattern Recognition** - Success and failure patterns identified from recent trades
3. **AI-Generated Insights** - LLM-powered analysis of what's working and what needs improvement
4. **Actionable Recommendations** - Specific rules and adjustments based on performance data
5. **Prompt Evolution** - Continuously optimizes the trading prompt based on feedback

## Architecture

### Components

**FeedbackGenerator** (`backtest/feedback.go`)
- Tracks all trades and performance metrics
- Analyzes patterns in winning and losing trades
- Generates insights and recommendations
- Can use LLM for deeper analysis (with AI client)

**PromptOptimizer** (`backtest/prompt_optimizer.go`)
- Uses feedback to evolve trading prompts
- Applies learned lessons to improve future decisions
- Maintains optimization state and history

**AutoTrader Integration**
- Creates FeedbackGenerator during initialization
- Injects AI client if available (for LLM-powered feedback)
- Calls GenerateFeedback() after sufficient trading cycles
- Returns analysis through GetFeedbackAnalysis() API

### Configuration

Feedback behavior is controlled by `FeedbackConfig`:

```go
type FeedbackConfig struct {
    EnableFeedback          bool // Master switch for feedback system
    MinDecisionsForFeedback int  // Minimum trades before generating feedback (default: 15)
    FeedbackWindowCycles    int  // Recent cycles to analyze (default: 10)
    TopTradesCount          int  // Top winning/losing trades to show (default: 3)
    MinPatternFrequency     int  // Min occurrences to consider a pattern (default: 7)
    EnableMicrostructure    bool // Use detailed trade microstructure analysis
    EnableLLMPatterns       bool // LLM-assisted pattern discovery
    EnableLLMInsights       bool // LLM-assisted insights and recommendations
}
```

Default configuration has feedback disabled but will be enabled based on smart defaults if specified.

## Accessing Feedback

### API Endpoint

**GET** `/api/traders/{traderId}/analysis`

Returns a `FeedbackAnalysis` object containing:

```json
{
  "analysis_period": "Last 26 trades",
  "start_time": "2025-12-21T21:14:59.999+08:00",
  "end_time": "2026-01-19T18:59:59.999+08:00",
  "decisions_covered": 26,
  "total_return": 567.89,
  "total_return_pct": 12.47,
  "win_rate": 61.54,
  "profit_factor": 6.13,
  "sharpe_ratio": 0.027,
  "max_drawdown": 4.34,
  "success_patterns": [...],
  "failure_patterns": [...],
  "key_insights": [...],
  "recommended_actions": [...],
  "top_winning_trades": [...],
  "top_losing_trades": [...],
  "market_conditions": "TRENDING_UP",
  "regime_analysis": {...},
  "trades_per_hour": 2.5,
  "avg_hold_time": "45m",
  "checklist_compliance": 85.5
}
```

### Frontend Display

The **Analysis** button in the trader details view:
1. Calls GET `/api/traders/{traderId}/analysis`
2. Displays results in a formatted panel showing:
   - Key performance metrics
   - Success/failure patterns
   - AI-generated insights
   - Recommended actions
   - Top winning/losing trades

## When Does Feedback Generate?

1. **Minimum Trades Required**: At least 15 trades (configurable) must complete
2. **Analysis Window**: Analyzes the most recent 10 trading cycles by default
3. **Frequency**: Regenerates after each significant trading milestone
4. **Persistence**: Saved to `feedback_analysis.json` in backtest directory

Current live trader status:
- ✅ 21 trading cycles completed (exceeds minimum of 15)
- ✅ Feedback analysis available
- ✅ Ready for prompt optimization

## Feedback Flow

```
Trading Execution
    ↓
AutoTrader.DoCycle()
    ↓
Calculate Stats
    ↓
Check: TotalTrades >= MinDecisionsForFeedback?
    ↓ YES
FeedbackGenerator.GenerateFeedback()
    ↓
Analyze winning/losing patterns
    ↓
If LLM enabled: Use AI to generate insights
    ↓
Return FeedbackAnalysis
    ↓
Store in lastFeedback & save to disk
    ↓
PromptOptimizer uses feedback to evolve prompts
    ↓
Next decision uses evolved prompt
```

## Prompt Evolution Integration

The feedback drives prompt evolution through `PromptOptimizer`:

1. **Feedback Analysis** identifies what worked and what didn't
2. **Optimizer Analyzes** the performance patterns
3. **Prompt Evolution** - adjusts the meta-prompt to emphasize:
   - Successful patterns observed
   - Rules to avoid failure patterns
   - Market regime-specific tactics
   - Position sizing recommendations

4. **Variant Creation** - new prompt variant emerges with learned improvements
5. **Testing** - next trades use evolved prompt to test improvements

## Accessing Performance History

Performance data is stored in:
- **Live Trader**: `/api/traders/{traderId}/analysis` endpoint
- **Completed Backtests**: `backtests/{backtest_id}/feedback_analysis.json`
- **Prompt History**: `backtests/{backtest_id}/prompt_optimizer_state.json`

## Troubleshooting

### "No analysis available yet" Message

**Cause**: Fewer than 15 trades completed

**Solution**: Wait for more trading cycles to complete. Check trader status with:
```bash
curl http://localhost:8080/api/traders/{traderId}/status
```

### Missing Feedback Data

**Check**:
1. Is `FeedbackGenerator` initialized? → Check logs for "FeedbackGenerator initialized"
2. Is feedback enabled? → Check `DefaultFeedbackConfig()` in backtest/feedback.go
3. Are there enough trades? → Check `TotalTrades` field in status

### LLM Insights Not Generated

**Cause**: MCP AI client not available

**Solution**: Either:
1. Ensure MCP server is running and properly configured
2. System will fall back to statistical analysis if LLM unavailable
3. Check logs for AI client connection status

## Example Usage

```bash
# Get trader status including if feedback is available
curl -H "Authorization: Bearer TOKEN" \
  http://localhost:8080/api/traders/trader-123/status

# Get detailed feedback analysis
curl -H "Authorization: Bearer TOKEN" \
  http://localhost:8080/api/traders/trader-123/analysis

# View feedback history in web UI
# Navigate to: Traders → Click trader → Click "Analysis" button
```

## Performance Optimization Tips

Based on feedback insights, the system recommends:

1. **Position Sizing**: Adjust based on win rate and profit factor
2. **Trade Frequency**: Limit trades per hour based on success patterns
3. **Market Conditions**: Skip trades in neutral/choppy markets
4. **Risk Management**: Increase stops in high-volatility regimes
5. **Confidence Thresholds**: Adjust minimum confidence required to trade

These recommendations automatically evolve as the trader learns from more trading cycles.

## Data Persistence

Feedback data is persisted in three ways:

1. **In-Memory** (AutoTrader.lastFeedback)
   - Active during trader runtime
   - Used for current cycle decisions

2. **File System** (feedback_analysis.json)
   - Saved after each generation
   - Survives trader restart
   - Used for historical analysis

3. **Database**
   - Optionally stored for long-term analysis
   - Enables comparison across multiple trading sessions

## Next Steps

1. ✅ Analysis endpoint implemented - ready to call
2. ✅ Feedback generation running - data available
3. ✅ Prompt optimization using feedback - working
4. → Frontend displays feedback in Analysis panel
5. → User reviews recommendations
6. → Decisions refined based on feedback loop
