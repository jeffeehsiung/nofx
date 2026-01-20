package decision

import (
	"fmt"
	"math"
	"strings"
)

// buildContinuousLearningFeedback creates a holistic learning section that enables the LLM to:
// 1. See what strategies are working vs failing
// 2. Identify patterns in wins and losses
// 3. Receive actionable improvement suggestions
// 4. Self-correct and adapt in real-time
// This is the core of the continuous learning system - every decision makes the next one smarter.
func (e *StrategyEngine) buildContinuousLearningFeedback(sb *strings.Builder, ctx *Context, lang Language) {
	if lang == LangChinese {
		sb.WriteString("\n## 🧠 持续学习系统 - 实时反馈与自我改进\n\n")
		sb.WriteString("系统正在分析你的交易模式。以下是关键洞察和改进建议：\n\n")
	} else {
		sb.WriteString("\n## 🧠 Continuous Learning System - Real-Time Feedback & Self-Improvement\n\n")
		sb.WriteString("The system is analyzing your trading patterns. Here are key insights and improvement suggestions:\n\n")
	}

	// === 1. PERFORMANCE SNAPSHOT ===
	if lang == LangChinese {
		sb.WriteString("### 📊 当前表现快照\n")
		sb.WriteString(fmt.Sprintf("- **总交易:** %d笔 | **胜率:** %.1f%% | **利润因子:** %.2f | **夏普比率:** %.2f\n",
			ctx.TradingStats.TotalTrades,
			ctx.TradingStats.WinRate,
			ctx.TradingStats.ProfitFactor,
			ctx.TradingStats.SharpeRatio))
		sb.WriteString(fmt.Sprintf("- **总盈亏:** %+.2f USDT | **最大回撤:** %.1f%% | **盈亏比:** %.2f\n",
			ctx.TradingStats.TotalPnL,
			ctx.TradingStats.MaxDrawdownPct,
			e.calculateWinLossRatio(ctx.TradingStats)))
	} else {
		sb.WriteString("### 📊 Current Performance Snapshot\n")
		sb.WriteString(fmt.Sprintf("- **Total Trades:** %d | **Win Rate:** %.1f%% | **Profit Factor:** %.2f | **Sharpe:** %.2f\n",
			ctx.TradingStats.TotalTrades,
			ctx.TradingStats.WinRate,
			ctx.TradingStats.ProfitFactor,
			ctx.TradingStats.SharpeRatio))
		sb.WriteString(fmt.Sprintf("- **Total PnL:** %+.2f USDT | **Max Drawdown:** %.1f%% | **Win/Loss Ratio:** %.2f\n",
			ctx.TradingStats.TotalPnL,
			ctx.TradingStats.MaxDrawdownPct,
			e.calculateWinLossRatio(ctx.TradingStats)))
	}
	sb.WriteString("\n")

	// === 2. WHAT'S WORKING & WHAT'S NOT ===
	e.analyzeWinningVsLosingPatterns(sb, ctx, lang)

	// === 3. ACTIONABLE IMPROVEMENT SUGGESTIONS ===
	e.provideActionableImprovements(sb, ctx, lang)

	// === 4. ADAPTIVE LEARNING INSIGHTS ===
	// Show how optimization systems are learning
	e.showOptimizationSystemsLearning(sb, ctx, lang)

	sb.WriteString("\n")
}

// analyzeWinningVsLosingPatterns identifies patterns in successful vs failed trades
func (e *StrategyEngine) analyzeWinningVsLosingPatterns(sb *strings.Builder, ctx *Context, lang Language) {
	if len(ctx.RecentOrders) < 2 {
		return
	}

	// Separate recent wins and losses
	type SimpleOrder struct {
		Symbol      string
		RealizedPnL float64
		EntryPrice  float64
	}

	var recentWins, recentLosses []SimpleOrder
	for i := len(ctx.RecentOrders) - 1; i >= 0 && len(recentWins)+len(recentLosses) < 10; i-- {
		order := ctx.RecentOrders[i]
		simple := SimpleOrder{
			Symbol:      order.Symbol,
			RealizedPnL: order.RealizedPnL,
			EntryPrice:  order.EntryPrice,
		}
		if order.RealizedPnL > 0 {
			recentWins = append(recentWins, simple)
		} else if order.RealizedPnL < 0 {
			recentLosses = append(recentLosses, simple)
		}
	}

	if len(recentWins) == 0 && len(recentLosses) == 0 {
		return
	}

	if lang == LangChinese {
		sb.WriteString("### ✅❌ 盈亏模式分析\n")
	} else {
		sb.WriteString("### ✅❌ Win/Loss Pattern Analysis\n")
	}

	// Show win/loss distribution
	if lang == LangChinese {
		sb.WriteString(fmt.Sprintf("**最近交易:** %d胜 / %d负\n\n", len(recentWins), len(recentLosses)))
	} else {
		sb.WriteString(fmt.Sprintf("**Recent Trades:** %d wins / %d losses\n\n", len(recentWins), len(recentLosses)))
	}

	// Winning patterns
	if len(recentWins) > 0 {
		if lang == LangChinese {
			sb.WriteString("**✓ 成功模式 (保持这些做法):**\n")
		} else {
			sb.WriteString("**✓ Winning Patterns (Keep Doing):**\n")
		}

		// Group by symbol
		winSymbols := make(map[string]float64)
		for _, win := range recentWins {
			winSymbols[win.Symbol] += win.RealizedPnL
		}

		if lang == LangChinese {
			sb.WriteString("- **盈利币种:** ")
		} else {
			sb.WriteString("- **Profitable Symbols:** ")
		}
		for symbol, pnl := range winSymbols {
			sb.WriteString(fmt.Sprintf("%s (+%.2f) ", symbol, pnl))
		}
		sb.WriteString("\n")

		// Show best trade as example
		bestWin := recentWins[0]
		for _, win := range recentWins {
			if win.RealizedPnL > bestWin.RealizedPnL {
				bestWin = win
			}
		}

		if lang == LangChinese {
			sb.WriteString(fmt.Sprintf("- **最佳交易:** %s, 收益: +%.2f USDT (%.1f%%)\n",
				bestWin.Symbol, bestWin.RealizedPnL, bestWin.RealizedPnL/bestWin.EntryPrice*100))
			sb.WriteString("  → 分析成功因素：是否有相似的市场条件或入场信号？\n\n")
		} else {
			sb.WriteString(fmt.Sprintf("- **Best Trade:** %s, Profit: +%.2f USDT (%.1f%%)\n",
				bestWin.Symbol, bestWin.RealizedPnL, bestWin.RealizedPnL/bestWin.EntryPrice*100))
			sb.WriteString("  → Analyze success factors: Any similar market conditions or entry signals?\n\n")
		}
	}

	// Losing patterns
	if len(recentLosses) > 0 {
		if lang == LangChinese {
			sb.WriteString("**⚠ 失败模式 (需要改进):**\n")
		} else {
			sb.WriteString("**⚠ Losing Patterns (Need Improvement):**\n")
		}

		// Group by symbol
		lossSymbols := make(map[string]float64)
		for _, loss := range recentLosses {
			lossSymbols[loss.Symbol] += loss.RealizedPnL
		}

		if lang == LangChinese {
			sb.WriteString("- **亏损币种:** ")
		} else {
			sb.WriteString("- **Losing Symbols:** ")
		}
		for symbol, pnl := range lossSymbols {
			sb.WriteString(fmt.Sprintf("%s (%.2f) ", symbol, pnl))
		}
		sb.WriteString("\n")

		// Show worst trade as warning
		worstLoss := recentLosses[0]
		for _, loss := range recentLosses {
			if loss.RealizedPnL < worstLoss.RealizedPnL {
				worstLoss = loss
			}
		}

		if lang == LangChinese {
			sb.WriteString(fmt.Sprintf("- **最大亏损:** %s, 损失: %.2f USDT (%.1f%%)\n",
				worstLoss.Symbol, worstLoss.RealizedPnL, worstLoss.RealizedPnL/worstLoss.EntryPrice*100))
			sb.WriteString("  → 避免重复错误：为什么这笔交易失败？入场时机、止损设置还是市场环境？\n\n")
		} else {
			sb.WriteString(fmt.Sprintf("- **Worst Loss:** %s, Loss: %.2f USDT (%.1f%%)\n",
				worstLoss.Symbol, worstLoss.RealizedPnL, worstLoss.RealizedPnL/worstLoss.EntryPrice*100))
			sb.WriteString("  → Avoid repeating mistakes: Why did this trade fail? Entry timing, stop loss, or market regime?\n\n")
		}
	}
}

// provideActionableImprovements gives specific, actionable suggestions based on performance
func (e *StrategyEngine) provideActionableImprovements(sb *strings.Builder, ctx *Context, lang Language) {
	if lang == LangChinese {
		sb.WriteString("### 💡 可执行的改进建议\n")
	} else {
		sb.WriteString("### 💡 Actionable Improvement Suggestions\n")
	}

	suggestions := []string{}

	// Win rate improvement
	if ctx.TradingStats.WinRate < 45 {
		if lang == LangChinese {
			suggestions = append(suggestions, "**提高胜率:** 你的胜率低于45%。增加交易选择性 - 只在最高确定性的设置下交易。等待更强的确认信号。")
		} else {
			suggestions = append(suggestions, "**Improve Win Rate:** Your win rate is below 45%. Increase trade selectivity - only trade highest-confidence setups. Wait for stronger confirmation signals.")
		}
	}

	// Profit factor improvement
	if ctx.TradingStats.ProfitFactor < 1.5 {
		if lang == LangChinese {
			suggestions = append(suggestions, "**优化利润因子:** 利润因子低于1.5。让盈利交易运行更长时间（移动止损以锁定利润），更早止损来限制损失。")
		} else {
			suggestions = append(suggestions, "**Optimize Profit Factor:** Profit factor below 1.5. Let winners run longer (trail stops to lock profit), cut losses earlier to limit damage.")
		}
	}

	// Risk management
	if ctx.TradingStats.MaxDrawdownPct > 20 {
		if lang == LangChinese {
			suggestions = append(suggestions, "**控制风险:** 最大回撤超过20%。立即减少仓位规模或使用更严格的止损。保护本金是第一优先级。")
		} else {
			suggestions = append(suggestions, "**Risk Management:** Max drawdown over 20%. Reduce position sizes immediately or use tighter stops. Capital preservation is priority #1.")
		}
	}

	// Sharpe ratio improvement
	if ctx.TradingStats.SharpeRatio < 0.5 && ctx.TradingStats.TotalTrades >= 10 {
		if lang == LangChinese {
			suggestions = append(suggestions, "**提升风险调整收益:** 夏普比率低于0.5。减少交易频率，专注于最佳机会，提高每笔交易的质量。")
		} else {
			suggestions = append(suggestions, "**Improve Risk-Adjusted Returns:** Sharpe ratio below 0.5. Reduce trade frequency, focus on best opportunities, improve quality per trade.")
		}
	}

	// Win/loss ratio
	winLossRatio := e.calculateWinLossRatio(ctx.TradingStats)
	if winLossRatio < 1.5 {
		if lang == LangChinese {
			suggestions = append(suggestions, fmt.Sprintf("**优化盈亏比:** 当前盈亏比%.2f。目标是2:1或更高。考虑在遇到阻力时获利，在支撑位止损。", winLossRatio))
		} else {
			suggestions = append(suggestions, fmt.Sprintf("**Optimize Win/Loss Ratio:** Current ratio %.2f. Target 2:1 or higher. Consider taking profit at resistance, stopping at support.", winLossRatio))
		}
	}

	// Show all suggestions
	if len(suggestions) > 0 {
		for i, suggestion := range suggestions {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, suggestion))
		}
		sb.WriteString("\n")
	} else {
		if lang == LangChinese {
			sb.WriteString("✓ 表现优秀！继续保持当前方法，但保持警惕市场变化。\n\n")
		} else {
			sb.WriteString("✓ Performance is strong! Continue current approach but stay alert to market shifts.\n\n")
		}
	}
}

// showOptimizationSystemsLearning shows how the adaptive systems are evolving
func (e *StrategyEngine) showOptimizationSystemsLearning(sb *strings.Builder, ctx *Context, lang Language) {
	if lang == LangChinese {
		sb.WriteString("### 🔧 自适应系统学习状态\n")
	} else {
		sb.WriteString("### 🔧 Adaptive Systems Learning Status\n")
	}

	learningInsights := []string{}

	// Weight optimization
	if ctx.OptimizedWeights != "" {
		if lang == LangChinese {
			learningInsights = append(learningInsights, "✓ **权重优化:** 系统已根据市场表现调整指标权重")
		} else {
			learningInsights = append(learningInsights, "✓ **Weight Optimization:** System has adjusted indicator weights based on market performance")
		}
	}

	// Threshold calibration
	if ctx.CalibratedThresholds != "" {
		if lang == LangChinese {
			learningInsights = append(learningInsights, "✓ **阈值校准:** 风险检测阈值已根据历史数据优化")
		} else {
			learningInsights = append(learningInsights, "✓ **Threshold Calibration:** Risk detection thresholds optimized from historical data")
		}
	}

	// Compliance tracking
	if ctx.ComplianceFeedback != "" {
		if lang == LangChinese {
			learningInsights = append(learningInsights, "✓ **合规跟踪:** 系统正在识别遵守vs违反策略规则的模式")
		} else {
			learningInsights = append(learningInsights, "✓ **Compliance Tracking:** System is identifying patterns of strategy rule adherence vs violation")
		}
	}

	// Prompt evolution
	if ctx.PromptEvolutionSummary != "" {
		if lang == LangChinese {
			learningInsights = append(learningInsights, "✓ **策略进化:** 决策框架正在通过遗传算法或元学习演化")
		} else {
			learningInsights = append(learningInsights, "✓ **Strategy Evolution:** Decision framework is evolving through genetic algorithm or meta-learning")
		}
	}

	if len(learningInsights) > 0 {
		for _, insight := range learningInsights {
			sb.WriteString(fmt.Sprintf("- %s\n", insight))
		}
		sb.WriteString("\n")

		if lang == LangChinese {
			sb.WriteString("**重要:** 这些优化系统在后台持续学习。你看到的建议会随着系统收集更多数据而改进。\n")
			sb.WriteString("**你的任务:** 基于上述洞察，现在做出最优交易决策。每次决策都在训练系统变得更智能。\n")
		} else {
			sb.WriteString("**Important:** These optimization systems are continuously learning in the background. The suggestions you see will improve as the system gathers more data.\n")
			sb.WriteString("**Your Task:** Based on the above insights, now make the optimal trading decision. Every decision trains the system to be smarter.\n")
		}
	}
}

// calculateWinLossRatio returns the average win to average loss ratio
func (e *StrategyEngine) calculateWinLossRatio(stats *TradingStats) float64 {
	if stats.AvgLoss == 0 {
		return 0
	}
	return stats.AvgWin / math.Abs(stats.AvgLoss)
}
