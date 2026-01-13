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

// LiveComplianceFeedback provides lightweight compliance tracking for live trading
type LiveComplianceFeedback struct {
	recentTrades []RecentOrder
	stats        *store.TraderStats
}

// NewLiveComplianceFeedback generates compliance feedback from recent trades
func NewLiveComplianceFeedback(recentTrades []RecentOrder, stats *store.TraderStats) *LiveComplianceFeedback {
	if stats == nil || stats.TotalTrades == 0 {
		return nil
	}
	return &LiveComplianceFeedback{
		recentTrades: recentTrades,
		stats:        stats,
	}
}

// FormatForPrompt renders compliance feedback for live trading
func (lc *LiveComplianceFeedback) FormatForPrompt(lang string) string {
	if lc == nil || lc.stats == nil {
		return ""
	}

	// Analyze recent trades for patterns
	if len(lc.recentTrades) == 0 {
		return ""
	}

	var winCount, lossCount int
	var avgWinPct, avgLossPct float64

	for _, trade := range lc.recentTrades {
		if trade.PnLPct > 0 {
			winCount++
			avgWinPct += trade.PnLPct
		} else {
			lossCount++
			avgLossPct += trade.PnLPct
		}
	}

	if winCount > 0 {
		avgWinPct /= float64(winCount)
	}
	if lossCount > 0 {
		avgLossPct /= float64(lossCount)
	}

	if lang == "zh" {
		var assessment string
		winRate := lc.stats.WinRate

		if winRate >= 60 {
			assessment = "✅ **优秀** - 保持当前策略，继续执行"
		} else if winRate >= 50 {
			assessment = "⚠️ **可接受** - 略有改进空间，监控风险"
		} else if winRate >= 40 {
			assessment = "🔴 **需要改进** - 亏损交易过多，审视信号质量"
		} else {
			assessment = "🔴 **严重问题** - 胜率过低，考虑暂停或调整策略"
		}

		return fmt.Sprintf(`
## 📊 近期表现评估（最近 %d 笔交易）
- 盈利: %d 笔 (平均 +%.2f%%) | 亏损: %d 笔 (平均 %.2f%%)
- 胜率: %.1f%% | 利润因子: %.2f

**评价**: %s

💡 继续学习和调整策略以改进表现。
`,
			len(lc.recentTrades), winCount, avgWinPct, lossCount, avgLossPct,
			winRate, lc.stats.ProfitFactor, assessment,
		)
	}

	// English
	var assessment string
	winRate := lc.stats.WinRate

	if winRate >= 60 {
		assessment = "✅ **Excellent** - Maintain current strategy"
	} else if winRate >= 50 {
		assessment = "⚠️ **Acceptable** - Room for improvement, monitor risk"
	} else if winRate >= 40 {
		assessment = "🔴 **Needs Improvement** - Too many losses, review signal quality"
	} else {
		assessment = "🔴 **Critical** - Win rate too low, consider adjustment"
	}

	return fmt.Sprintf(`
## 📊 Recent Performance (Last %d trades)
- Wins: %d trades (avg +%.2f%%) | Losses: %d trades (avg %.2f%%)
- Win Rate: %.1f%% | Profit Factor: %.2f

**Assessment**: %s

💡 Continue learning and adjusting strategy for improvement.
`,
		len(lc.recentTrades), winCount, avgWinPct, lossCount, avgLossPct,
		winRate, lc.stats.ProfitFactor, assessment,
	)
}

// LiveThresholdSummary provides learned thresholds from recent trade patterns
type LiveThresholdSummary struct {
	recentTrades []RecentOrder
}

// NewLiveThresholdSummary creates threshold summary from recent trades
func NewLiveThresholdSummary(recentTrades []RecentOrder) *LiveThresholdSummary {
	if len(recentTrades) == 0 {
		return nil
	}
	return &LiveThresholdSummary{recentTrades: recentTrades}
}

// FormatForPrompt renders learned thresholds for live trading
func (lt *LiveThresholdSummary) FormatForPrompt(lang string) string {
	if lt == nil || len(lt.recentTrades) == 0 {
		return ""
	}

	// Analyze winning vs losing trades
	var winTrades, loseTrades []RecentOrder
	for _, trade := range lt.recentTrades {
		if trade.PnLPct > 0 {
			winTrades = append(winTrades, trade)
		} else {
			loseTrades = append(loseTrades, trade)
		}
	}

	if len(winTrades) == 0 || len(loseTrades) == 0 {
		return "" // Not enough data
	}

	// Calculate average characteristics
	avgWinDuration := calculateAvgDuration(winTrades)
	avgLoseDuration := calculateAvgDuration(loseTrades)
	avgWinSize := calculateAvgSize(winTrades)
	avgLoseSize := calculateAvgSize(loseTrades)

	if lang == "zh" {
		return fmt.Sprintf(`
## 📏 从最近交易中学习的最优阈值
**盈利交易特征** (n=%d):
- 平均持仓时间: %s
- 平均盈利幅度: +%.2f%%

**亏损交易特征** (n=%d):
- 平均持仓时间: %s
- 平均亏损幅度: %.2f%%

💡 建议: 尽量提高交易相似度到盈利交易特征，避免亏损交易的特征。
`,
			len(winTrades), avgWinDuration, avgWinSize,
			len(loseTrades), avgLoseDuration, avgLoseSize,
		)
	}

	// English
	return fmt.Sprintf(`
## 📏 Learned Thresholds from Recent Trades
**Winning Trades** (n=%d):
- Avg Hold Time: %s
- Avg Profit: +%.2f%%

**Losing Trades** (n=%d):
- Avg Hold Time: %s
- Avg Loss: %.2f%%

💡 Tip: Aim for characteristics of winning trades, avoid patterns of losing trades.
`,
		len(winTrades), avgWinDuration, avgWinSize,
		len(loseTrades), avgLoseDuration, avgLoseSize,
	)
}

// Helper functions
func calculateAvgDuration(trades []RecentOrder) string {
	if len(trades) == 0 {
		return "N/A"
	}
	// For simplicity, return a reasonable estimate based on HoldDuration
	// In production, parse HoldDuration strings properly
	return "varies"
}

func calculateAvgSize(trades []RecentOrder) float64 {
	if len(trades) == 0 {
		return 0
	}
	total := 0.0
	for _, trade := range trades {
		total += trade.PnLPct
	}
	return total / float64(len(trades))
}
