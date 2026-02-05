package decision

import (
	"fmt"
	"nofx/market"
	"nofx/provider"
	"nofx/store"
	"sort"
	"strings"
	"time"
)

// ============================================================================
// AI Data Formatter - AI数据格式化器
// ============================================================================
// 将交易上下文转换为AI友好的格式，确保AI能够100%理解数据
// ============================================================================

// ========== 中文格式化函数 ==========

// formatAccountZH 格式化账户信息（中文）
func formatAccountZH(ctx *Context) string {
	acc := ctx.Account
	var sb strings.Builder

	sb.WriteString("## 账户状态\n\n")
	sb.WriteString(fmt.Sprintf("总权益: %.2f USDT | ", acc.TotalEquity))
	sb.WriteString(fmt.Sprintf("可用余额: %.2f USDT (%.1f%%) | ", acc.AvailableBalance, (acc.AvailableBalance/acc.TotalEquity)*100))
	sb.WriteString(fmt.Sprintf("总盈亏: %+.2f%% | ", acc.TotalPnLPct))
	sb.WriteString(fmt.Sprintf("保证金使用率: %.1f%% | ", acc.MarginUsedPct))
	sb.WriteString(fmt.Sprintf("持仓数: %d\n\n", acc.PositionCount))

	// 添加风险提示
	if acc.MarginUsedPct > 85 {
		sb.WriteString("⚠️ **风险警告**: 保证金使用率 > 85%，处于高风险状态！\n\n")
	} else if acc.MarginUsedPct > 50 {
		sb.WriteString("⚠️ **风险提示**: 保证金使用率 > 50%，建议谨慎开仓\n\n")
	}

	return sb.String()
}

// formatRecentTradesZH 格式化最近交易（中文）
func formatRecentTradesZH(orders []RecentOrder) string {
	var sb strings.Builder
	sb.WriteString("## 最近完成的交易\n\n")

	for i, order := range orders {
		// 判断盈亏
		profitOrLoss := "盈利"
		if order.RealizedPnL < 0 {
			profitOrLoss = "亏损"
		}

		sb.WriteString(fmt.Sprintf("%d. %s %s | 进场 %.3f 出场 %.3f | %s: %+.2f USDT (%+.2f%%) | %s → %s (%s)\n",
			i+1,
			order.Symbol,
			order.Side,
			order.EntryPrice,
			order.ExitPrice,
			profitOrLoss,
			order.RealizedPnL,
			order.PnLPct,
			order.EntryTime,
			order.ExitTime,
			order.HoldDuration,
		))
	}

	sb.WriteString("\n")
	return sb.String()
}

// formatCurrentPositionsZH 格式化当前持仓（中文）
func formatCurrentPositionsZH(strategy_config *store.StrategyConfig, ctx *Context) string {
	var sb strings.Builder
	sb.WriteString("## 当前持仓\n\n")

	for i, pos := range ctx.Positions {
		// 计算回撤
		drawdown := pos.UnrealizedPnLPct - pos.PeakPnLPct

		sb.WriteString(fmt.Sprintf("%d. %s %s | ", i+1, pos.Symbol, strings.ToUpper(pos.Side)))
		sb.WriteString(fmt.Sprintf("进场 %.3f 当前 %.3f | ", pos.EntryPrice, pos.MarkPrice))
		sb.WriteString(fmt.Sprintf("数量 %.3f | ", pos.Quantity))
		sb.WriteString(fmt.Sprintf("仓位价值 %.2f USDT | ", pos.Quantity*pos.MarkPrice))
		sb.WriteString(fmt.Sprintf("盈亏 %+.2f%% | ", pos.UnrealizedPnLPct))
		sb.WriteString(fmt.Sprintf("盈亏金额 %+.2f USDT | ", pos.UnrealizedPnL))
		sb.WriteString(fmt.Sprintf("峰值盈亏 %.2f%% | ", pos.PeakPnLPct))
		sb.WriteString(fmt.Sprintf("杠杆 %dx | ", pos.Leverage))
		sb.WriteString(fmt.Sprintf("保证金 %.0f USDT | ", pos.MarginUsed))
		sb.WriteString(fmt.Sprintf("强平价 %.3f\n", pos.LiquidationPrice))

		// 添加分析提示
		if drawdown < -0.30*pos.PeakPnLPct && pos.PeakPnLPct > 0.02 {
			sb.WriteString(fmt.Sprintf("   ⚠️ **止盈提示**: 当前盈亏从峰值 %.2f%% 回撤到 %.2f%%，回撤幅度 %.2f%%，建议考虑止盈\n",
				pos.PeakPnLPct, pos.UnrealizedPnLPct, (drawdown/pos.PeakPnLPct)*100))
		}

		if pos.UnrealizedPnLPct < -4.0 {
			sb.WriteString("   ⚠️ **止损提示**: 亏损接近-5%止损线，建议考虑止损\n")
		}

		// 显示当前价格（如果有市场数据）
		if ctx.MarketDataMap != nil {
			if mdata, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(formatMarketDataZH(strategy_config, mdata))
			}
		}

		// 市场微观结构数据（如果有）
		if ctx.MicrostructureDataMap != nil {
			if ms, ok := ctx.MicrostructureDataMap[pos.Symbol]; ok {
				sb.WriteString(formatMicrostructureZH(ms))
			}
		}

		// 量化数据分析提示 (如果有)
		if ctx.QuantDataMap != nil {
			if qdata, ok := ctx.QuantDataMap[pos.Symbol]; ok {
				sb.WriteString(formatQuantDataZH(qdata))
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// formatCandidateCoinsZH 格式化候选币种（中文）
func formatCandidateCoinsZH(ctx *Context) string {
	var sb strings.Builder
	sb.WriteString("## 候选币种\n\n")

	for i, coin := range ctx.CandidateCoins {
		sb.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, coin.Symbol))

		// 当前价格
		if ctx.MarketDataMap != nil {
			if mdata, ok := ctx.MarketDataMap[coin.Symbol]; ok {
				sb.WriteString(fmt.Sprintf("当前价格: %.3f\n\n", mdata.CurrentPrice))

				// K线数据（多时间框架）
				if mdata.TimeframeData != nil {
					sb.WriteString(formatKlineDataZH(coin.Symbol, mdata.TimeframeData, ctx.Timeframes))
				}
			}
		}

		// OI数据（如果有）
		if ctx.OITopDataMap != nil {
			if oiData, ok := ctx.OITopDataMap[coin.Symbol]; ok {
				sb.WriteString(fmt.Sprintf("**持仓量变化**: OI排名 #%d | 变化 %+.2f%% (%+.2fM USDT) | 价格变化 %+.2f%%\n\n",
					oiData.Rank,
					oiData.OIDeltaPercent,
					oiData.OIDeltaValue/1_000_000,
					oiData.PriceDeltaPercent,
				))

				// OI解读
				oiChange := "增加"
				if oiData.OIDeltaPercent < 0 {
					oiChange = "减少"
				}
				priceChange := "上涨"
				if oiData.PriceDeltaPercent < 0 {
					priceChange = "下跌"
				}

				interpretation := getOIInterpretationZH(oiChange, priceChange)
				sb.WriteString(fmt.Sprintf("**市场解读**: %s\n\n", interpretation))
			}
		}

		// 量化数据分析提示 (如果有)
		if ctx.QuantDataMap != nil {
			if qdata, ok := ctx.QuantDataMap[coin.Symbol]; ok {
				sb.WriteString(formatQuantDataZH(qdata))
			}
		}
	}

	return sb.String()
}

// formatKlineDataZH 格式化K线数据（中文）
func formatKlineDataZH(symbol string, tfData map[string]*market.TimeframeSeriesData, timeframes []string) string {
	var sb strings.Builder

	for _, tf := range timeframes {
		if data, ok := tfData[tf]; ok && len(data.Klines) > 0 {
			sb.WriteString(fmt.Sprintf("#### %s %s 时间框架 (从旧到新)\n\n", symbol, tf))
			sb.WriteString("```\n")
			sb.WriteString("时间(UTC)      开盘      最高      最低      收盘      成交量\n")

			// 只显示最近30根K线
			startIdx := 0
			if len(data.Klines) > 30 {
				startIdx = len(data.Klines) - 30
			}

			for i := startIdx; i < len(data.Klines); i++ {
				k := data.Klines[i]
				t := time.UnixMilli(k.Time).UTC()
				sb.WriteString(fmt.Sprintf("%s    %.3f    %.3f    %.3f    %.3f    %.2f\n",
					t.Format("01-02 15:04"),
					k.Open,
					k.High,
					k.Low,
					k.Close,
					k.Volume,
				))
			}

			// 标记最后一根K线
			if len(data.Klines) > 0 {
				sb.WriteString("    <- 当前\n")
			}

			sb.WriteString("```\n\n")
		}
	}

	return sb.String()
}

// formatOIRankingZH 格式化OI排名数据（中文）
func formatOIRankingZH(oiData interface{}) string {
	if oiData == nil {
		return "## 市场持仓量排名\n\n(暂无数据)\n\n"
	}

	// Try to format as OIRankingData structure
	if oiRanking, ok := oiData.(*provider.OIRankingData); ok {
		if oiRanking == nil || (len(oiRanking.TopPositions) == 0 && len(oiRanking.LowPositions) == 0) {
			return "## 市场持仓量排名\n\n(数据加载中...)\n\n"
		}

		var sb strings.Builder
		sb.WriteString("## 市场持仓量排名\n\n")

		if len(oiRanking.TopPositions) > 0 {
			sb.WriteString("### 持仓量TOP (最高杠杆长仓)\n\n")
			sb.WriteString("市场资金正在流入以下币种，可能表示趋势延续或新仓位建立:\n\n")
			sb.WriteString("| 排名 | 币种 | 持仓变化值(USDT) | 变化幅度 | 价格变化 |\n")
			sb.WriteString("|------|------|------------------|----------|----------|\n")
			for i, pos := range oiRanking.TopPositions {
				if i >= 5 {
					break // 只显示前5个
				}
				sb.WriteString(fmt.Sprintf("| #%d | %s | %s | %+.2f%% | %+.2f%% |\n",
					pos.Rank,
					pos.Symbol,
					formatOIValue(pos.OIDeltaValue),
					pos.OIDeltaPercent,
					pos.PriceDeltaPercent,
				))
			}
			sb.WriteString("\n")
			sb.WriteString("**解读**: 持仓增加 + 价格上涨 = 多头主导; 持仓增加 + 价格下跌 = 空头主导\n\n")
		}

		if len(oiRanking.LowPositions) > 0 {
			sb.WriteString("### 持仓量LOW (最高杠杆空仓)\n\n")
			sb.WriteString("市场资金正在流出以下币种，可能表示趋势反转或仓位平仓:\n\n")
			sb.WriteString("| 排名 | 币种 | 持仓变化值(USDT) | 变化幅度 | 价格变化 |\n")
			sb.WriteString("|------|------|------------------|----------|----------|\n")
			for i, pos := range oiRanking.LowPositions {
				if i >= 5 {
					break
				}
				sb.WriteString(fmt.Sprintf("| #%d | %s | %s | %+.2f%% | %+.2f%% |\n",
					pos.Rank,
					pos.Symbol,
					formatOIValue(pos.OIDeltaValue),
					pos.OIDeltaPercent,
					pos.PriceDeltaPercent,
				))
			}
			sb.WriteString("\n")
			sb.WriteString("**解读**: 持仓减少 + 价格上涨 = 空头平仓(反弹); 持仓减少 + 价格下跌 = 多头平仓(回调)\n\n")
		}

		return sb.String()
	}

	return "## 市场持仓量排名\n\n(数据加载中...)\n\n"
}

// getOIInterpretationZH 获取OI变化解读（中文）
func getOIInterpretationZH(oiChange, priceChange string) string {
	if oiChange == "增加" && priceChange == "上涨" {
		return OIInterpretation.OIUp_PriceUp.ZH
	} else if oiChange == "增加" && priceChange == "下跌" {
		return OIInterpretation.OIUp_PriceDown.ZH
	} else if oiChange == "减少" && priceChange == "上涨" {
		return OIInterpretation.OIDown_PriceUp.ZH
	} else {
		return OIInterpretation.OIDown_PriceDown.ZH
	}
}

// formatMarketDataZH 格式化市场数据（中文）
func formatMarketDataZH(strategy_config *store.StrategyConfig, data *market.Data) string {
	if data == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 📈 市场数据概览\n\n")
	sb.WriteString(fmt.Sprintf("### %s 市场数据\n\n", data.Symbol))
	sb.WriteString(fmt.Sprintf("   📈 当前价格: %.3f\n", data.CurrentPrice))
	sb.WriteString(fmt.Sprintf(",  📈 当前EMA20: %.3f\n", data.CurrentEMA20))
	sb.WriteString(fmt.Sprintf(",  📈 当前MACD: %.3f\n", data.CurrentMACD))
	sb.WriteString(fmt.Sprintf(",  📈 当前RSI7: %.3f\n", data.CurrentRSI7))
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf(",  📈 当前OI: %.2f & 平均OI: %.2f\n\n", data.OpenInterest.Latest, data.OpenInterest.Average))
	sb.WriteString(fmt.Sprintf(",  📈 当前资金费率: %.3f%%\n", data.FundingRate))
	if len(data.TimeframeData) > 0 {
		timeframeOrder := []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w"}
		for _, tf := range timeframeOrder {
			if tfData, ok := data.TimeframeData[tf]; ok {
				sb.WriteString(fmt.Sprintf("=== %s 时间框架 (从旧到新) ===\n\n", strings.ToUpper(tf)))
				formatTimeframeSeriesDataZH(&sb, tfData)
			}
		}
	} else {
		// 兼容旧数据格式
		if data.IntradaySeries != nil {
			sb.WriteString(fmt.Sprintf("日内序列 (%s 时间间隔，从旧到新):\n\n", strategy_config.Indicators.Klines.PrimaryTimeframe))
			if len(data.IntradaySeries.MidPrices) > 0 {
				sb.WriteString(fmt.Sprintf("中间价: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
			}
			if len(data.IntradaySeries.EMA20Values) > 0 {
				sb.WriteString(fmt.Sprintf("EMA指标 (20期): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
			}
			if len(data.IntradaySeries.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD指标: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
			}
			if len(data.IntradaySeries.RSI7Values) > 0 {
				sb.WriteString(fmt.Sprintf("RSI指标 (7期): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
			}
			if len(data.IntradaySeries.RSI14Values) > 0 {
				sb.WriteString(fmt.Sprintf("RSI指标 (14期): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
			}
			if len(data.IntradaySeries.Volume) > 0 {
				sb.WriteString(fmt.Sprintf("成交量: %s\n\n", formatFloatSlice(data.IntradaySeries.Volume)))
			}
			if data.IntradaySeries.ATR14 > 0 {
				sb.WriteString(fmt.Sprintf("3m ATR (14期): %.3f\n\n", data.IntradaySeries.ATR14))
			}
		}
		if data.LongerTermContext != nil {
			sb.WriteString("### 📊 更长期市场背景\n\n")
			sb.WriteString(fmt.Sprintf("更长期时间框架 (%s):\n\n", strategy_config.Indicators.Klines.LongerTimeframe))
			sb.WriteString((fmt.Sprintf("EMA20 : %3.f vs. EMA50: %3f\n\n", data.LongerTermContext.EMA20, data.LongerTermContext.EMA50)))
			sb.WriteString((fmt.Sprintf("3期ATR : %3.f vs. 14期ATR: %3f\n\n", data.LongerTermContext.ATR3, data.LongerTermContext.ATR14)))
			sb.WriteString((fmt.Sprintf("当前成交量 : %3.f vs. 平均成交量: %3f\n\n", data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume)))
			if len(data.LongerTermContext.MACDValues) > 0 {
				sb.WriteString((fmt.Sprintf("MACD指标 : %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues))))
			}
			if len(data.LongerTermContext.RSI14Values) > 0 {
				sb.WriteString((fmt.Sprintf("RSI指标 (14期) : %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values))))
			}
		}
	}
	return sb.String()
}

// formatMicrostructureZH 市场微观结构数据格式化（中文）
func formatMicrostructureZH(ms *market.MarketMicrostructure) string {
	if ms == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## 📊 市场微观结构分析\n\n")
	sb.WriteString("**说明**: 订单簿分析提供支撑/阻力位、流动性深度和买卖压力信息\n\n")
	sb.WriteString(fmt.Sprintf("### %s 微观结构\n\n", ms.Symbol))

	// Support levels
	if len(ms.SupportLevels) > 0 {
		sb.WriteString("**支撑位** (价格下方的买单聚集区):\n")
		for i, price := range ms.SupportLevels {
			if i >= 3 {
				break // Top 3 only
			}
			distance := (ms.CurrentPrice - price) / ms.CurrentPrice * 100
			sb.WriteString(fmt.Sprintf("- %.2f USDT (距离: -%.2f%%)\n", price, distance))
		}
		sb.WriteString("\n")
	}

	// Resistance levels
	if len(ms.ResistanceLevels) > 0 {
		sb.WriteString("**阻力位** (价格上方的卖单聚集区):\n")
		for i, price := range ms.ResistanceLevels {
			if i >= 3 {
				break // Top 3 only
			}
			distance := (price - ms.CurrentPrice) / ms.CurrentPrice * 100
			sb.WriteString(fmt.Sprintf("- %.2f USDT (距离: +%.2f%%)\n", price, distance))
		}
		sb.WriteString("\n")
	}

	// Order book metrics
	sb.WriteString("**订单簿指标**:\n")
	sb.WriteString(fmt.Sprintf("- 买卖压力: %.2f ", ms.OrderBookImbalance))
	if ms.OrderBookImbalance > 0.6 {
		sb.WriteString("(买盘占优 🟢)\n")
	} else if ms.OrderBookImbalance < 0.4 {
		sb.WriteString("(卖盘占优 🔴)\n")
	} else {
		sb.WriteString("(相对平衡 ⚪)\n")
	}
	sb.WriteString(fmt.Sprintf("- 买卖价差: %.3f%% ", ms.BidAskSpread))
	if ms.BidAskSpread < 0.05 {
		sb.WriteString("(流动性良好)\n")
	} else if ms.BidAskSpread > 0.2 {
		sb.WriteString("(流动性较差)\n")
	} else {
		sb.WriteString("(流动性正常)\n")
	}
	sb.WriteString(fmt.Sprintf("- 订单簿深度: 买%.0f | 卖%.0f USDT\n", ms.BidDepth, ms.AskDepth))
	sb.WriteString(fmt.Sprintf("- VWAP偏离: %.2f%%\n\n", ms.VWAPDeviation))
	sb.WriteString(fmt.Sprintf("- 大宗交易活动: %d 笔大单 (成交量≥ %.2f)\n\n", ms.LargeOrderCount, ms.LargeOrderVolume))

	return sb.String()
}

// formatTimeframeSeriesDataZH 时间框架序列数据格式化（中文）
func formatTimeframeSeriesDataZH(sb *strings.Builder, data *market.TimeframeSeriesData) {
	if len(data.Klines) > 0 {
		sb.WriteString("时间(UTC)      开盘价     最高价     最低价     收盘价     成交量\n")
		for i, k := range data.Klines {
			t := time.Unix(k.Time/1000, 0).UTC()
			timeStr := t.Format("01-02 15:04")
			marker := ""
			if i == len(data.Klines)-1 {
				marker = "  <- 当前"
			}
			sb.WriteString(fmt.Sprintf("%-14s %-9.3f %-9.3f %-9.3f %-9.3f %-12.2f%s\n",
				timeStr, k.Open, k.High, k.Low, k.Close, k.Volume, marker))
		}
		sb.WriteString("\n")
	} else if len(data.MidPrices) > 0 {
		sb.WriteString(fmt.Sprintf("中间价: %s\n\n", formatFloatSlice(data.MidPrices)))
		if len(data.Volume) > 0 {
			sb.WriteString(fmt.Sprintf("成交量: %s\n\n", formatFloatSlice(data.Volume)))
		}
	}
	if len(data.EMA20Values) > 0 {
		sb.WriteString(fmt.Sprintf("EMA20: %s\n", formatFloatSlice(data.EMA20Values)))
	}
	if len(data.EMA50Values) > 0 {
		sb.WriteString(fmt.Sprintf("EMA50: %s\n", formatFloatSlice(data.EMA50Values)))
	}
	if len(data.MACDValues) > 0 {
		sb.WriteString(fmt.Sprintf("MACD: %s\n", formatFloatSlice(data.MACDValues)))
	}
	if len(data.RSI7Values) > 0 {
		sb.WriteString(fmt.Sprintf("RSI7: %s\n", formatFloatSlice(data.RSI7Values)))
	}
	if len(data.RSI14Values) > 0 {
		sb.WriteString(fmt.Sprintf("RSI14: %s\n", formatFloatSlice(data.RSI14Values)))
	}
	if data.ATR14 > 0 {
		sb.WriteString(fmt.Sprintf("ATR14: %.3f\n", data.ATR14))
	}
	if len(data.BOLLUpper) > 0 {
		sb.WriteString(fmt.Sprintf("BOLL 上轨: %s\n", formatFloatSlice(data.BOLLUpper)))
		sb.WriteString(fmt.Sprintf("BOLL 中轨: %s\n", formatFloatSlice(data.BOLLMiddle)))
		sb.WriteString(fmt.Sprintf("BOLL 下轨: %s\n", formatFloatSlice(data.BOLLLower)))
	}
	sb.WriteString("\n")
}

// formatQuantDataZH 格式化量化数据（中文）
func formatQuantDataZH(data *QuantData) string {
	if data == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 %s 量化数据:\n", data.Symbol))

	if len(data.PriceChange) > 0 {
		sb.WriteString("价格变动: ")
		timeframes := []string{"5m", "15m", "1h", "4h", "12h", "24h"}
		parts := []string{}
		for _, tf := range timeframes {
			if v, ok := data.PriceChange[tf]; ok {
				parts = append(parts, fmt.Sprintf("%s: %+.3f%%", tf, v*100))
			}
		}
		sb.WriteString(strings.Join(parts, " | "))
		sb.WriteString("\n")
	}

	if data.Netflow != nil {
		sb.WriteString("资金流向 (Netflow):\n")
		timeframes := []string{"5m", "15m", "1h", "4h", "12h", "24h"}

		if data.Netflow.Institution != nil {
			if len(data.Netflow.Institution.Future) > 0 {
				sb.WriteString("  机构期货:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Institution.Future[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
			if len(data.Netflow.Institution.Spot) > 0 {
				sb.WriteString("  机构现货:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Institution.Spot[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
		}

		if data.Netflow.Personal != nil {
			if len(data.Netflow.Personal.Future) > 0 {
				sb.WriteString("  散户期货:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Personal.Future[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
			if len(data.Netflow.Personal.Spot) > 0 {
				sb.WriteString("  散户现货:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Personal.Spot[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
		}
	}

	if len(data.OI) > 0 {
		for exchange, oiData := range data.OI {
			if len(oiData.Delta) > 0 {
				sb.WriteString(fmt.Sprintf("持仓量变化 (%s):\n", exchange))
				for _, tf := range []string{"5m", "15m", "1h", "4h", "12h", "24h"} {
					if d, ok := oiData.Delta[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %+.3f%% (%s)\n", tf, d.OIDeltaPercent, formatFlowValue(d.OIDeltaValue)))
					}
				}
			}
		}
	}

	return sb.String()
}

// ========== 英文格式化函数 ==========

// formatAccountEN 格式化账户信息（英文）
func formatAccountEN(ctx *Context) string {
	acc := ctx.Account
	var sb strings.Builder

	sb.WriteString("## Account Status\n\n")
	sb.WriteString(fmt.Sprintf("Total Equity: %.2f USDT | ", acc.TotalEquity))
	sb.WriteString(fmt.Sprintf("Available Balance: %.2f USDT (%.1f%%) | ", acc.AvailableBalance, (acc.AvailableBalance/acc.TotalEquity)*100))
	sb.WriteString(fmt.Sprintf("Total PnL: %+.2f%% | ", acc.TotalPnLPct))
	sb.WriteString(fmt.Sprintf("Margin Usage: %.1f%% | ", acc.MarginUsedPct))
	sb.WriteString(fmt.Sprintf("Positions: %d\n\n", acc.PositionCount))

	// Risk warning
	if acc.MarginUsedPct > 85 {
		sb.WriteString("⚠️ **Risk Alert**: Margin usage > 85%, high risk!\n\n")
	} else if acc.MarginUsedPct > 50 {
		sb.WriteString("⚠️ **Risk Notice**: Margin usage > 50%, be cautious with new positions\n\n")
	}

	return sb.String()
}

// formatRecentTradesEN 格式化最近交易（英文）
func formatRecentTradesEN(orders []RecentOrder) string {
	var sb strings.Builder
	sb.WriteString("## Recent Completed Trades\n\n")

	for i, order := range orders {
		profitOrLoss := "Profit"
		if order.RealizedPnL < 0 {
			profitOrLoss = "Loss"
		}

		sb.WriteString(fmt.Sprintf("%d. %s %s | Entry %.3f Exit %.3f | %s: %+.2f USDT (%+.2f%%) | %s → %s (%s)\n",
			i+1,
			order.Symbol,
			order.Side,
			order.EntryPrice,
			order.ExitPrice,
			profitOrLoss,
			order.RealizedPnL,
			order.PnLPct,
			order.EntryTime,
			order.ExitTime,
			order.HoldDuration,
		))
	}

	sb.WriteString("\n")
	return sb.String()
}

// formatCurrentPositionsEN 格式化当前持仓（英文）
func formatCurrentPositionsEN(strategy_config *store.StrategyConfig, ctx *Context) string {
	var sb strings.Builder
	sb.WriteString("## Current Positions\n\n")

	for i, pos := range ctx.Positions {
		drawdown := pos.UnrealizedPnLPct - pos.PeakPnLPct

		sb.WriteString(fmt.Sprintf("%d. %s %s | ", i+1, pos.Symbol, strings.ToUpper(pos.Side)))
		sb.WriteString(fmt.Sprintf("Entry %.3f Current %.3f | ", pos.EntryPrice, pos.MarkPrice))
		sb.WriteString(fmt.Sprintf("Qty %.3f | ", pos.Quantity))
		sb.WriteString(fmt.Sprintf("Value %.2f USDT | ", pos.Quantity*pos.MarkPrice))
		sb.WriteString(fmt.Sprintf("PnL %+.2f%% | ", pos.UnrealizedPnLPct))
		sb.WriteString(fmt.Sprintf("PnL Amount %+.2f USDT | ", pos.UnrealizedPnL))
		sb.WriteString(fmt.Sprintf("Peak PnL %.2f%% | ", pos.PeakPnLPct))
		sb.WriteString(fmt.Sprintf("Leverage %dx | ", pos.Leverage))
		sb.WriteString(fmt.Sprintf("Margin %.0f USDT | ", pos.MarginUsed))
		sb.WriteString(fmt.Sprintf("Liq Price %.3f\n", pos.LiquidationPrice))

		// Analysis hints
		if drawdown < -0.30*pos.PeakPnLPct && pos.PeakPnLPct > 0.02 {
			sb.WriteString(fmt.Sprintf("   ⚠️ **Take Profit Alert**: PnL dropped from peak %.2f%% to %.2f%%, drawdown %.2f%%, consider taking profit\n",
				pos.PeakPnLPct, pos.UnrealizedPnLPct, (drawdown/pos.PeakPnLPct)*100))
		}

		if pos.UnrealizedPnLPct < -4.0 {
			sb.WriteString("   ⚠️ **Stop Loss Alert**: Loss approaching -5% threshold, consider cutting loss\n")
		}

		if ctx.MarketDataMap != nil {
			if mdata, ok := ctx.MarketDataMap[pos.Symbol]; ok {
				sb.WriteString(formatMarketDataEN(strategy_config, mdata))
			}
		}
		if ctx.MicrostructureDataMap != nil {
			if ms, ok := ctx.MicrostructureDataMap[pos.Symbol]; ok {
				sb.WriteString(formatMicrostructureEN(ms))
			}
		}
		if ctx.QuantDataMap != nil {
			if qdata, ok := ctx.QuantDataMap[pos.Symbol]; ok {
				sb.WriteString(formatQuantDataEN(qdata))
			}
		}
	}

	return sb.String()
}

// formatCandidateCoinsEN 格式化候选币种（英文）
func formatCandidateCoinsEN(ctx *Context) string {
	var sb strings.Builder
	sb.WriteString("## Candidate Coins\n\n")

	for i, coin := range ctx.CandidateCoins {
		sb.WriteString(fmt.Sprintf("### %d. %s\n\n", i+1, coin.Symbol))

		if ctx.MarketDataMap != nil {
			if mdata, ok := ctx.MarketDataMap[coin.Symbol]; ok {
				sb.WriteString(fmt.Sprintf("Current Price: %.3f\n\n", mdata.CurrentPrice))

				if mdata.TimeframeData != nil {
					sb.WriteString(formatKlineDataEN(coin.Symbol, mdata.TimeframeData, ctx.Timeframes))
				}
			}
		}

		if ctx.OITopDataMap != nil {
			if oiData, ok := ctx.OITopDataMap[coin.Symbol]; ok {
				sb.WriteString(fmt.Sprintf("**OI Change**: Rank #%d | Change %+.2f%% (%+.2fM USDT) | Price Change %+.2f%%\n\n",
					oiData.Rank,
					oiData.OIDeltaPercent,
					oiData.OIDeltaValue/1_000_000,
					oiData.PriceDeltaPercent,
				))

				oiChange := "increase"
				if oiData.OIDeltaPercent < 0 {
					oiChange = "decrease"
				}
				priceChange := "up"
				if oiData.PriceDeltaPercent < 0 {
					priceChange = "down"
				}

				interpretation := getOIInterpretationEN(oiChange, priceChange)
				sb.WriteString(fmt.Sprintf("**Market Interpretation**: %s\n\n", interpretation))
			}
		}

		if ctx.QuantDataMap != nil {
			if qdata, ok := ctx.QuantDataMap[coin.Symbol]; ok {
				sb.WriteString(formatQuantDataEN(qdata))
			}
		}
	}

	return sb.String()
}

// formatKlineDataEN 格式化K线数据（英文）
func formatKlineDataEN(symbol string, tfData map[string]*market.TimeframeSeriesData, timeframes []string) string {
	var sb strings.Builder

	// Sort timeframes for consistent output
	sortedTF := make([]string, len(timeframes))
	copy(sortedTF, timeframes)
	sort.Strings(sortedTF)

	for _, tf := range sortedTF {
		if data, ok := tfData[tf]; ok && len(data.Klines) > 0 {
			sb.WriteString(fmt.Sprintf("#### %s %s Timeframe (oldest → latest)\n\n", symbol, tf))
			sb.WriteString("```\n")
			sb.WriteString("Time(UTC)      Open      High      Low       Close     Volume\n")

			startIdx := 0
			if len(data.Klines) > 30 {
				startIdx = len(data.Klines) - 30
			}

			for i := startIdx; i < len(data.Klines); i++ {
				k := data.Klines[i]
				t := time.UnixMilli(k.Time).UTC()
				sb.WriteString(fmt.Sprintf("%s    %.3f    %.3f    %.3f    %.3f    %.2f\n",
					t.Format("01-02 15:04"),
					k.Open,
					k.High,
					k.Low,
					k.Close,
					k.Volume,
				))
			}

			if len(data.Klines) > 0 {
				sb.WriteString("    <- current\n")
			}

			sb.WriteString("```\n\n")
		}
	}

	return sb.String()
}

// formatOIRankingEN 格式化OI排名数据（英文）
func formatOIRankingEN(oiData interface{}) string {
	if oiData == nil {
		return "## Market-wide OI Ranking\n\n(No data available)\n\n"
	}

	// Try to format as OIRankingData structure
	if oiRanking, ok := oiData.(*provider.OIRankingData); ok {
		if oiRanking == nil || (len(oiRanking.TopPositions) == 0 && len(oiRanking.LowPositions) == 0) {
			return "## Market-wide OI Ranking\n\n(Loading data...)\n\n"
		}

		var sb strings.Builder
		sb.WriteString("## Market-wide OI Ranking\n\n")

		if len(oiRanking.TopPositions) > 0 {
			sb.WriteString("### Top OI Positions (Highest Leverage Long)\n\n")
			sb.WriteString("Market funds are flowing into the following coins, possibly indicating trend continuation or new position building:\n\n")
			sb.WriteString("| Rank | Symbol | OI Change Value (USDT) | Change Percent | Price Change |\n")
			sb.WriteString("|------|--------|------------------------|----------------|--------------|\n")
			for i, pos := range oiRanking.TopPositions {
				if i >= 5 {
					break // Show only top 5
				}
				sb.WriteString(fmt.Sprintf("| #%d | %s | %s | %+.2f%% | %+.2f%% |\n",
					pos.Rank,
					pos.Symbol,
					formatOIValue(pos.OIDeltaValue),
					pos.OIDeltaPercent,
					pos.PriceDeltaPercent,
				))
			}
			sb.WriteString("\n")
			sb.WriteString("**Interpretion**: OI Increase + Price Up = Bullish Dominance; OI Increase + Price Down = Bearish Dominance\n\n")
		}

		if len(oiRanking.LowPositions) > 0 {
			sb.WriteString("### Low OI Positions (Highest Leverage Short)\n\n")
			sb.WriteString("Market funds are flowing out of the following coins, possibly indicating trend reversal or position liquidation:\n\n")
			sb.WriteString("| Rank | Symbol | OI Change Value (USDT) | Change Percent | Price Change |\n")
			sb.WriteString("|------|--------|------------------------|----------------|--------------|\n")
			for i, pos := range oiRanking.LowPositions {
				if i >= 5 {
					break
				}
				sb.WriteString(fmt.Sprintf("| #%d | %s | %s | %+.2f%% | %+.2f%% |\n",
					pos.Rank,
					pos.Symbol,
					formatOIValue(pos.OIDeltaValue),
					pos.OIDeltaPercent,
					pos.PriceDeltaPercent,
				))
			}
			sb.WriteString("\n")
			sb.WriteString("**Interpretion**: OI Decrease + Price Up = Short Covering (Rebound); OI Decrease + Price Down = Long Liquidation (Pullback)\n\n")
		}

		return sb.String()
	}

	return "## Market-wide OI Ranking\n\n(Loading data...)\n\n"
}

// getOIInterpretationEN 获取OI变化解读（英文）
func getOIInterpretationEN(oiChange, priceChange string) string {
	if oiChange == "increase" && priceChange == "up" {
		return OIInterpretation.OIUp_PriceUp.EN
	} else if oiChange == "increase" && priceChange == "down" {
		return OIInterpretation.OIUp_PriceDown.EN
	} else if oiChange == "decrease" && priceChange == "up" {
		return OIInterpretation.OIDown_PriceUp.EN
	} else if oiChange == "decrease" && priceChange == "down" {
		return OIInterpretation.OIDown_PriceDown.EN
	}
	return ""
}

// formatOptimizedWeightsZH formats optimized trading parameters (Chinese)
func formatOptimizedWeightsZH(weights interface{}) string {
	if weights == nil {
		return ""
	}

	if formatter, ok := weights.(interface{ FormatWeightsForPrompt(lang string) string }); ok {
		return formatter.FormatWeightsForPrompt("zh")
	}

	return ""
}

// formatOptimizedWeightsEN formats optimized trading parameters (English)
func formatOptimizedWeightsEN(weights interface{}) string {
	if weights == nil {
		return ""
	}

	if formatter, ok := weights.(interface{ FormatWeightsForPrompt(lang string) string }); ok {
		return formatter.FormatWeightsForPrompt("en")
	}

	return ""
}

// formatMarketDataEN formats market data (English)
func formatMarketDataEN(strategy_config *store.StrategyConfig, data *market.Data) string {
	if data == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 📈 Market Data Overview\n\n")
	sb.WriteString(fmt.Sprintf("### %s Market Data\n\n", data.Symbol))
	sb.WriteString(fmt.Sprintf("current_price = %.3f", data.CurrentPrice))
	sb.WriteString(fmt.Sprintf(", current_ema20 = %.3f", data.CurrentEMA20))
	sb.WriteString(fmt.Sprintf(", current_macd = %.3f", data.CurrentMACD))
	sb.WriteString(fmt.Sprintf(", current_rsi7 = %.3f", data.CurrentRSI7))
	sb.WriteString(fmt.Sprintf("Open Interest: Latest: %.2f Average: %.2f\n\n", data.OpenInterest.Latest, data.OpenInterest.Average))
	sb.WriteString(fmt.Sprintf("Funding Rate: %.2e\n\n", data.FundingRate))
	if len(data.TimeframeData) > 0 {
		timeframeOrder := []string{"1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "6h", "8h", "12h", "1d", "3d", "1w"}
		for _, tf := range timeframeOrder {
			if tfData, ok := data.TimeframeData[tf]; ok {
				sb.WriteString(fmt.Sprintf("=== %s Timeframe (oldest → latest) ===\n\n", strings.ToUpper(tf)))
				formatTimeframeSeriesDataEN(&sb, tfData)
			}
		}
	} else {
		// Compatible with old data format
		if data.IntradaySeries != nil {
			sb.WriteString(fmt.Sprintf("Intraday series (%s intervals, oldest → latest):\n\n", strategy_config.Indicators.Klines.PrimaryTimeframe))
			if len(data.IntradaySeries.MidPrices) > 0 {
				sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.IntradaySeries.MidPrices)))
			}
			if len(data.IntradaySeries.EMA20Values) > 0 {
				sb.WriteString(fmt.Sprintf("EMA indicators (20-period): %s\n\n", formatFloatSlice(data.IntradaySeries.EMA20Values)))
			}
			if len(data.IntradaySeries.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.IntradaySeries.MACDValues)))
			}

			if len(data.IntradaySeries.RSI7Values) > 0 {
				sb.WriteString(fmt.Sprintf("RSI indicators (7-Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI7Values)))
			}
			if len(data.IntradaySeries.RSI14Values) > 0 {
				sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.IntradaySeries.RSI14Values)))
			}
			if len(data.IntradaySeries.Volume) > 0 {
				sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.IntradaySeries.Volume)))
			}
			if data.IntradaySeries.ATR14 > 0 {
				sb.WriteString(fmt.Sprintf("3m ATR (14-period): %.3f\n\n", data.IntradaySeries.ATR14))
			}
		}
		if data.LongerTermContext != nil {
			sb.WriteString(fmt.Sprintf("Longer-term context (%s timeframe):\n\n", strategy_config.Indicators.Klines.LongerTimeframe))
			sb.WriteString(fmt.Sprintf("20-Period EMA: %.3f vs. 50-Period EMA: %.3f\n\n",
				data.LongerTermContext.EMA20, data.LongerTermContext.EMA50))
			sb.WriteString(fmt.Sprintf("3-Period ATR: %.3f vs. 14-Period ATR: %.3f\n\n",
				data.LongerTermContext.ATR3, data.LongerTermContext.ATR14))

			sb.WriteString(fmt.Sprintf("Current Volume: %.3f vs. Average Volume: %.3f\n\n",
				data.LongerTermContext.CurrentVolume, data.LongerTermContext.AverageVolume))
			if len(data.LongerTermContext.MACDValues) > 0 {
				sb.WriteString(fmt.Sprintf("MACD indicators: %s\n\n", formatFloatSlice(data.LongerTermContext.MACDValues)))
			}
			if len(data.LongerTermContext.RSI14Values) > 0 {
				sb.WriteString(fmt.Sprintf("RSI indicators (14-Period): %s\n\n", formatFloatSlice(data.LongerTermContext.RSI14Values)))
			}
		}
	}
	return sb.String()
}

// formatMicrostructureEN formats market microstructure data (English)
func formatMicrostructureEN(ms *market.MarketMicrostructure) string {
	if ms == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## 📊 Market Microstructure Analysis\n\n")
	sb.WriteString("**Note**: Order book analysis provides support/resistance levels, liquidity depth, and buy/sell pressure\n\n")
	sb.WriteString(fmt.Sprintf("### %s Microstructure\n\n", ms.Symbol))

	// Support levels
	if len(ms.SupportLevels) > 0 {
		sb.WriteString("**Support Levels** (bid order clusters below price):\n")
		for i, price := range ms.SupportLevels {
			if i >= 3 {
				break // Top 3 only
			}
			distance := (ms.CurrentPrice - price) / ms.CurrentPrice * 100
			sb.WriteString(fmt.Sprintf("- $%.2f (distance: -%.2f%%)\n", price, distance))
		}
		sb.WriteString("\n")
	}

	// Resistance levels
	if len(ms.ResistanceLevels) > 0 {
		sb.WriteString("**Resistance Levels** (ask order clusters above price):\n")
		for i, price := range ms.ResistanceLevels {
			if i >= 3 {
				break // Top 3 only
			}
			distance := (price - ms.CurrentPrice) / ms.CurrentPrice * 100
			sb.WriteString(fmt.Sprintf("- $%.2f (distance: +%.2f%%)\n", price, distance))
		}
		sb.WriteString("\n")
	}

	// Order book metrics
	sb.WriteString("**Order Book Metrics**:\n")
	sb.WriteString(fmt.Sprintf("- Order Book Imbalance: %.2f ", ms.OrderBookImbalance))
	if ms.OrderBookImbalance > 0.6 {
		sb.WriteString("(buy pressure 🟢)\n")
	} else if ms.OrderBookImbalance < 0.4 {
		sb.WriteString("(sell pressure 🔴)\n")
	} else {
		sb.WriteString("(balanced ⚪)\n")
	}
	sb.WriteString(fmt.Sprintf("- Spread: %.3f%% ", ms.BidAskSpread))
	if ms.BidAskSpread < 0.05 {
		sb.WriteString("(good liquidity)\n")
	} else if ms.BidAskSpread > 0.2 {
		sb.WriteString("(poor liquidity)\n")
	} else {
		sb.WriteString("(normal liquidity)\n")
	}
	sb.WriteString(fmt.Sprintf("- Order Book Depth: Bid $%.0f | Ask $%.0f\n", ms.BidDepth, ms.AskDepth))
	sb.WriteString(fmt.Sprintf("- VWAP Deviation: %.2f%%\n\n", ms.VWAPDeviation))
	sb.WriteString(fmt.Sprintf("- Large Trade Activity: %d large trades (volume ≥ $%.2f)\n\n", ms.LargeOrderCount, ms.LargeOrderVolume))
	return sb.String()
}

// formatTimeframeSeriesDataEN formats timeframe series data (English)
func formatTimeframeSeriesDataEN(sb *strings.Builder, data *market.TimeframeSeriesData) {
	if len(data.Klines) > 0 {
		sb.WriteString("Time(UTC)      Open      High      Low       Close     Volume\n")
		for i, k := range data.Klines {
			t := time.Unix(k.Time/1000, 0).UTC()
			timeStr := t.Format("01-02 15:04")
			marker := ""
			if i == len(data.Klines)-1 {
				marker = "  <- current"
			}
			sb.WriteString(fmt.Sprintf("%-14s %-9.3f %-9.3f %-9.3f %-9.3f %-12.2f%s\n",
				timeStr, k.Open, k.High, k.Low, k.Close, k.Volume, marker))
		}
		sb.WriteString("\n")
	} else if len(data.MidPrices) > 0 {
		sb.WriteString(fmt.Sprintf("Mid prices: %s\n\n", formatFloatSlice(data.MidPrices)))
		if len(data.Volume) > 0 {
			sb.WriteString(fmt.Sprintf("Volume: %s\n\n", formatFloatSlice(data.Volume)))
		}
	}
	if len(data.EMA20Values) > 0 {
		sb.WriteString(fmt.Sprintf("EMA20: %s\n", formatFloatSlice(data.EMA20Values)))
	}
	if len(data.EMA50Values) > 0 {
		sb.WriteString(fmt.Sprintf("EMA50: %s\n", formatFloatSlice(data.EMA50Values)))
	}
	if len(data.MACDValues) > 0 {
		sb.WriteString(fmt.Sprintf("MACD: %s\n", formatFloatSlice(data.MACDValues)))
	}
	if len(data.RSI7Values) > 0 {
		sb.WriteString(fmt.Sprintf("RSI7: %s\n", formatFloatSlice(data.RSI7Values)))
	}
	if len(data.RSI14Values) > 0 {
		sb.WriteString(fmt.Sprintf("RSI14: %s\n", formatFloatSlice(data.RSI14Values)))
	}
	if data.ATR14 > 0 {
		sb.WriteString(fmt.Sprintf("ATR14: %.3f\n", data.ATR14))
	}
	if len(data.BOLLUpper) > 0 {
		sb.WriteString(fmt.Sprintf("BOLL Upper: %s\n", formatFloatSlice(data.BOLLUpper)))
		sb.WriteString(fmt.Sprintf("BOLL Middle: %s\n", formatFloatSlice(data.BOLLMiddle)))
		sb.WriteString(fmt.Sprintf("BOLL Lower: %s\n", formatFloatSlice(data.BOLLLower)))
	}
	sb.WriteString("\n")
}

// formatQuantDataEN 格式化量化数据（英文）
func formatQuantDataEN(data *QuantData) string {
	if data == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📊 %s Quantitative Data:\n", data.Symbol))

	if len(data.PriceChange) > 0 {
		sb.WriteString("Price Change: ")
		timeframes := []string{"5m", "15m", "1h", "4h", "12h", "24h"}
		parts := []string{}
		for _, tf := range timeframes {
			if v, ok := data.PriceChange[tf]; ok {
				parts = append(parts, fmt.Sprintf("%s: %+.3f%%", tf, v*100))
			}
		}
		sb.WriteString(strings.Join(parts, " | "))
		sb.WriteString("\n")
	}

	if data.Netflow != nil {
		sb.WriteString("Fund Flow (Netflow):\n")
		timeframes := []string{"5m", "15m", "1h", "4h", "12h", "24h"}

		if data.Netflow.Institution != nil {
			if len(data.Netflow.Institution.Future) > 0 {
				sb.WriteString("  Institutional Futures:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Institution.Future[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
			if len(data.Netflow.Institution.Spot) > 0 {
				sb.WriteString("  Institutional Spot:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Institution.Spot[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
		}

		if data.Netflow.Personal != nil {
			if len(data.Netflow.Personal.Future) > 0 {
				sb.WriteString("  Retail Futures:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Personal.Future[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
			if len(data.Netflow.Personal.Spot) > 0 {
				sb.WriteString("  Retail Spot:\n")
				for _, tf := range timeframes {
					if v, ok := data.Netflow.Personal.Spot[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %s\n", tf, formatFlowValue(v)))
					}
				}
			}
		}
	}

	if len(data.OI) > 0 {
		for exchange, oiData := range data.OI {
			if len(oiData.Delta) > 0 {
				sb.WriteString(fmt.Sprintf("Open Interest (%s):\n", exchange))
				for _, tf := range []string{"5m", "15m", "1h", "4h", "12h", "24h"} {
					if d, ok := oiData.Delta[tf]; ok {
						sb.WriteString(fmt.Sprintf("    %s: %+.3f%% (%s)\n", tf, d.OIDeltaPercent, formatFlowValue(d.OIDeltaValue)))
					}
				}
			}
		}
	}

	return sb.String()
}

// =============================================
//  辅助格式化函数
// =============================================

// formatOIValue 格式化持仓量数值，带单位和符号
func formatOIValue(v float64) string {
	sign := ""
	if v >= 0 {
		sign = "+"
	}
	absV := v
	if absV < 0 {
		absV = -absV
	}
	if absV >= 1e9 {
		return fmt.Sprintf("%s%.2fB", sign, v/1e9)
	} else if absV >= 1e6 {
		return fmt.Sprintf("%s%.2fM", sign, v/1e6)
	} else if absV >= 1e3 {
		return fmt.Sprintf("%s%.2fK", sign, v/1e3)
	}
	return fmt.Sprintf("%s%.2f", sign, v)
}

// formatFlowValue 格式化资金流向数值，带单位和符号
func formatFlowValue(v float64) string {
	sign := ""
	if v >= 0 {
		sign = "+"
	}
	absV := v
	if absV < 0 {
		absV = -absV
	}
	if absV >= 1e9 {
		return fmt.Sprintf("%s%.2fB", sign, v/1e9)
	} else if absV >= 1e6 {
		return fmt.Sprintf("%s%.2fM", sign, v/1e6)
	} else if absV >= 1e3 {
		return fmt.Sprintf("%s%.2fK", sign, v/1e3)
	}
	return fmt.Sprintf("%s%.2f", sign, v)
}

// formatFloatSlice 格式化浮点数切片为字符串
func formatFloatSlice(values []float64) string {
	strValues := make([]string, len(values))
	for i, v := range values {
		strValues[i] = fmt.Sprintf("%.3f", v)
	}
	return "[" + strings.Join(strValues, ", ") + "]"
}
