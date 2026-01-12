package decision

import (
	"fmt"
	"nofx/store"
)

// LivePerformanceFeedback provides a lightweight runtime performance summary for prompts.
type LivePerformanceFeedback struct {
	stats *store.TraderStats
}

// NewLivePerformanceFeedback builds a summary only when stats are present and non-empty.
func NewLivePerformanceFeedback(stats *store.TraderStats) *LivePerformanceFeedback {
	if stats == nil || stats.TotalTrades == 0 {
		return nil
	}
	return &LivePerformanceFeedback{stats: stats}
}

// FormatForPrompt renders a compact block for the LLM prompt.
func (l *LivePerformanceFeedback) FormatForPrompt(lang string) string {
	if l == nil || l.stats == nil || l.stats.TotalTrades == 0 {
		return ""
	}

	s := l.stats

	if lang == "zh" {
		return fmt.Sprintf(`
## 📈 实时表现摘要
- 交易数: %d | 胜率: %.1f%% | 利润因子: %.2f
- 总盈亏: %.2f USDT | 平均盈亏: +%.2f / -%.2f USDT
- 最大回撤: %.1f%%
`,
			s.TotalTrades, s.WinRate, s.ProfitFactor,
			s.TotalPnL, s.AvgWin, s.AvgLoss,
			s.MaxDrawdownPct,
		)
	}

	return fmt.Sprintf(`
## 📈 Live Performance Snapshot
- Trades: %d | Win Rate: %.1f%% | Profit Factor: %.2f
- Total PnL: %.2f USDT | Avg Win/Loss: +%.2f / -%.2f USDT
- Max Drawdown: %.1f%%
`,
		s.TotalTrades, s.WinRate, s.ProfitFactor,
		s.TotalPnL, s.AvgWin, s.AvgLoss,
		s.MaxDrawdownPct,
	)
}

// LiveOptimizedWeights renders the active risk-control parameters for prompts.
type LiveOptimizedWeights struct {
	config store.RiskControlConfig
}

// NewLiveOptimizedWeights wraps the current risk-control settings for prompt formatting.
func NewLiveOptimizedWeights(cfg store.RiskControlConfig) *LiveOptimizedWeights {
	return &LiveOptimizedWeights{config: cfg}
}

// FormatWeightsForPrompt satisfies the formatter contract used by the prompt builder.
func (l *LiveOptimizedWeights) FormatWeightsForPrompt(lang string) string {
	cfg := l.config

	if lang == "zh" {
		return fmt.Sprintf(`
## 🎯 当前风控参数（实时）
- 杠杆: BTC/ETH %dx | 山寨 %dx
- 头寸上限: BTC/ETH %.1fx | 山寨 %.1fx | 最大持仓数: %d
- 最小头寸: $%.0f | 最小信心: %d%% | 最小RRR: %.1f
- 最大保证金占用: %.0f%% | 回撤监控: %v | 检查间隔: %ds
`,
			cfg.BTCETHMaxLeverage, cfg.AltcoinMaxLeverage,
			cfg.BTCETHMaxPositionValueRatio, cfg.AltcoinMaxPositionValueRatio, cfg.MaxPositions,
			cfg.MinPositionSize, cfg.MinConfidence, cfg.MinRiskRewardRatio,
			cfg.MaxMarginUsage*100, cfg.DrawdownMonitoringEnabled, cfg.DrawdownCheckInterval,
		)
	}

	return fmt.Sprintf(`
## 🎯 Current Risk Controls (Live)
- Leverage: BTC/ETH %dx | Altcoin %dx
- Position Caps: BTC/ETH %.1fx | Altcoin %.1fx | Max Positions: %d
- Min Position: $%.0f | Min Confidence: %d%% | Min RRR: %.1f
- Max Margin Usage: %.0f%% | Drawdown Monitor: %v | Interval: %ds
`,
		cfg.BTCETHMaxLeverage, cfg.AltcoinMaxLeverage,
		cfg.BTCETHMaxPositionValueRatio, cfg.AltcoinMaxPositionValueRatio, cfg.MaxPositions,
		cfg.MinPositionSize, cfg.MinConfidence, cfg.MinRiskRewardRatio,
		cfg.MaxMarginUsage*100, cfg.DrawdownMonitoringEnabled, cfg.DrawdownCheckInterval,
	)
}
