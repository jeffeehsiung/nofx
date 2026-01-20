package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// StrategyStore strategy storage
type StrategyStore struct {
	db *sql.DB
}

// Strategy strategy configuration
type Strategy struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`  // whether it is active (a user can only have one active strategy)
	IsDefault   bool      `json:"is_default"` // whether it is a system default strategy
	Config      string    `json:"config"`     // strategy configuration in JSON format
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// StrategyConfig strategy configuration details (JSON structure)
type StrategyConfig struct {
	// coin source configuration
	CoinSource CoinSourceConfig `json:"coin_source"`
	// quantitative data configuration
	Indicators IndicatorConfig `json:"indicators"`
	// custom prompt (appended at the end)
	CustomPrompt string `json:"custom_prompt,omitempty"`
	// risk control configuration
	RiskControl RiskControlConfig `json:"risk_control"`
	// editable sections of System Prompt
	PromptSections PromptSectionsConfig `json:"prompt_sections,omitempty"`
}

// PromptSectionsConfig editable sections of System Prompt
type PromptSectionsConfig struct {
	// role definition (title + description)
	RoleDefinition string `json:"role_definition,omitempty"`
	// trading frequency awareness
	TradingFrequency string `json:"trading_frequency,omitempty"`
	// entry standards
	EntryStandards string `json:"entry_standards,omitempty"`
	// decision process
	DecisionProcess string `json:"decision_process,omitempty"`
}

// CoinSourceConfig coin source configuration
type CoinSourceConfig struct {
	// source type: "static" | "coinpool" | "oi_top" | "mixed"
	SourceType string `json:"source_type"`
	// static coin list (used when source_type = "static")
	StaticCoins []string `json:"static_coins,omitempty"`
	// whether to use AI500 coin pool
	UseCoinPool bool `json:"use_coin_pool"`
	// AI500 coin pool maximum count
	CoinPoolLimit int `json:"coin_pool_limit,omitempty"`
	// AI500 coin pool API URL (strategy-level configuration)
	CoinPoolAPIURL string `json:"coin_pool_api_url,omitempty"`
	// whether to use OI Top
	UseOITop bool `json:"use_oi_top"`
	// OI Top maximum count
	OITopLimit int `json:"oi_top_limit,omitempty"`
	// OI Top API URL (strategy-level configuration)
	OITopAPIURL string `json:"oi_top_api_url,omitempty"`
}

// IndicatorConfig indicator configuration
type IndicatorConfig struct {
	// K-line configuration
	Klines KlineConfig `json:"klines"`
	// raw kline data (OHLCV) - always enabled, required for AI analysis
	EnableRawKlines bool `json:"enable_raw_klines"`
	// technical indicator switches
	EnableEMA         bool `json:"enable_ema"`
	EnableMACD        bool `json:"enable_macd"`
	EnableRSI         bool `json:"enable_rsi"`
	EnableATR         bool `json:"enable_atr"`
	EnableBOLL        bool `json:"enable_boll"` // Bollinger Bands
	EnableVolume      bool `json:"enable_volume"`
	EnableOI          bool `json:"enable_oi"`           // open interest
	EnableFundingRate bool `json:"enable_funding_rate"` // funding rate
	// EMA period configuration
	EMAPeriods []int `json:"ema_periods,omitempty"` // default [20, 50]
	// RSI period configuration
	RSIPeriods []int `json:"rsi_periods,omitempty"` // default [7, 14]
	// ATR period configuration
	ATRPeriods []int `json:"atr_periods,omitempty"` // default [14]
	// MACD period configuration (fast, slow)
	MACDFastPeriod int `json:"macd_fast_period,omitempty"` // default 12
	MACDSlowPeriod int `json:"macd_slow_period,omitempty"` // default 26
	// BOLL period configuration (period, standard deviation multiplier is fixed at 2)
	BOLLPeriods []int `json:"boll_periods,omitempty"` // default [20] - can select multiple timeframes
	// external data sources
	ExternalDataSources []ExternalDataSource `json:"external_data_sources,omitempty"`
	// quantitative data sources (capital flow, position changes, price changes)
	EnableQuantData    bool   `json:"enable_quant_data"`            // whether to enable quantitative data
	QuantDataAPIURL    string `json:"quant_data_api_url,omitempty"` // quantitative data API address
	EnableQuantOI      bool   `json:"enable_quant_oi"`              // whether to show OI data
	EnableQuantNetflow bool   `json:"enable_quant_netflow"`         // whether to show Netflow data
	// OI ranking data (market-wide open interest increase/decrease rankings)
	EnableOIRanking   bool   `json:"enable_oi_ranking"`             // whether to enable OI ranking data
	OIRankingAPIURL   string `json:"oi_ranking_api_url,omitempty"`  // OI ranking API base URL
	OIRankingDuration string `json:"oi_ranking_duration,omitempty"` // duration: 1h, 4h, 24h
	OIRankingLimit    int    `json:"oi_ranking_limit,omitempty"`    // number of entries (default 10)
}

// KlineConfig K-line configuration
type KlineConfig struct {
	// primary timeframe: "1m", "3m", "5m", "15m", "1h", "4h"
	PrimaryTimeframe string `json:"primary_timeframe"`
	// primary timeframe K-line count
	PrimaryCount int `json:"primary_count"`
	// longer timeframe
	LongerTimeframe string `json:"longer_timeframe,omitempty"`
	// longer timeframe K-line count
	LongerCount int `json:"longer_count,omitempty"`
	// whether to enable multi-timeframe analysis
	EnableMultiTimeframe bool `json:"enable_multi_timeframe"`
	// selected timeframe list (new: supports multi-timeframe selection)
	SelectedTimeframes []string `json:"selected_timeframes,omitempty"`
}

// ExternalDataSource external data source configuration
type ExternalDataSource struct {
	Name        string            `json:"name"`   // data source name
	Type        string            `json:"type"`   // type: "api" | "webhook"
	URL         string            `json:"url"`    // API URL
	Method      string            `json:"method"` // HTTP method
	Headers     map[string]string `json:"headers,omitempty"`
	DataPath    string            `json:"data_path,omitempty"`    // JSON data path
	RefreshSecs int               `json:"refresh_secs,omitempty"` // refresh interval (seconds)
}

// RiskControlConfig risk control configuration
// All parameters are clearly defined without ambiguity:
//
// Position Limits:
//   - MaxPositions: max number of coins held simultaneously (CODE ENFORCED)
//
// Trading Leverage (exchange leverage for opening positions):
//   - BTCETHMaxLeverage: BTC/ETH max exchange leverage (AI guided)
//   - AltcoinMaxLeverage: Altcoin max exchange leverage (AI guided)
//
// Position Value Limits (single position notional value / account equity):
//   - BTCETHMaxPositionValueRatio: BTC/ETH max = equity × ratio (CODE ENFORCED)
//   - AltcoinMaxPositionValueRatio: Altcoin max = equity × ratio (CODE ENFORCED)
//
// Risk Controls:
//   - MaxMarginUsage: max margin utilization percentage (CODE ENFORCED)
//   - MinPositionSize: minimum position size in USDT (CODE ENFORCED)
//   - MinRiskRewardRatio: min take_profit / stop_loss ratio (AI guided)
//   - MinConfidence: min AI confidence to open position (AI guided)
type RiskControlConfig struct {
	// Max number of coins held simultaneously (CODE ENFORCED)
	MaxPositions int `json:"max_positions"`

	// BTC/ETH exchange leverage for opening positions (AI guided)
	BTCETHMaxLeverage int `json:"btc_eth_max_leverage"`
	// Altcoin exchange leverage for opening positions (AI guided)
	AltcoinMaxLeverage int `json:"altcoin_max_leverage"`

	// BTC/ETH single position max value = equity × this ratio (CODE ENFORCED, default: 5)
	BTCETHMaxPositionValueRatio float64 `json:"btc_eth_max_position_value_ratio"`
	// Altcoin single position max value = equity × this ratio (CODE ENFORCED, default: 1)
	AltcoinMaxPositionValueRatio float64 `json:"altcoin_max_position_value_ratio"`

	// Max margin utilization (e.g. 0.9 = 90%) (CODE ENFORCED)
	MaxMarginUsage float64 `json:"max_margin_usage"`
	// Min position size in USDT (CODE ENFORCED)
	MinPositionSize float64 `json:"min_position_size"`

	// Min take_profit / stop_loss ratio (AI guided)
	MinRiskRewardRatio float64 `json:"min_risk_reward_ratio"`
	// Min AI confidence to open position (AI guided)
	MinConfidence int `json:"min_confidence"`

	// Drawdown monitoring configuration (CODE ENFORCED - trailing stop for profit protection)
	DrawdownMonitoringEnabled bool    `json:"drawdown_monitoring_enabled"` // Enable/disable drawdown monitoring (default: true)
	DrawdownCheckInterval     int     `json:"drawdown_check_interval"`     // Check interval in seconds (default: 60, min: 15, max: 300)
	MinProfitThreshold        float64 `json:"min_profit_threshold"`        // Minimum profit % to start monitoring (default: 5.0)
	DrawdownCloseThreshold    float64 `json:"drawdown_close_threshold"`    // Drawdown % from peak to trigger close (default: 40.0, e.g. peak 10% -> 6% triggers close)
}

func (s *StrategyStore) initTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS strategies (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			is_active BOOLEAN DEFAULT 0,
			is_default BOOLEAN DEFAULT 0,
			config TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// create indexes
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_strategies_user_id ON strategies(user_id)`)
	_, _ = s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_strategies_is_active ON strategies(is_active)`)

	// trigger: automatically update updated_at on update
	_, err = s.db.Exec(`
		CREATE TRIGGER IF NOT EXISTS update_strategies_updated_at
		AFTER UPDATE ON strategies
		BEGIN
			UPDATE strategies SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
		END
	`)

	return err
}

func (s *StrategyStore) initDefaultData() error {
	// No longer pre-populate strategies - create on demand when user configures
	return nil
}

// AvailableIndicatorsString returns a summary of enabled indicators and their periods
func (c *StrategyConfig) AvailableIndicatorsString(sb *strings.Builder, lang string) string {
	indicators := c.Indicators
	kline := indicators.Klines
	if lang == "zh" {
		sb.WriteString(fmt.Sprintf("- %s K线序列", kline.PrimaryTimeframe))
		if kline.EnableMultiTimeframe {
			sb.WriteString(fmt.Sprintf(" + %s K线序列\n", kline.LongerTimeframe))
		} else {
			sb.WriteString("\n")
		}
		if indicators.EnableEMA {
			sb.WriteString("- EMA指标")
			if len(indicators.EMAPeriods) > 0 {
				sb.WriteString(fmt.Sprintf("（周期：%v）", indicators.EMAPeriods))
			}
			sb.WriteString("\n")
		}
		if indicators.EnableMACD {
			sb.WriteString("- MACD指标\n")
		}
		if indicators.EnableRSI {
			sb.WriteString("- RSI指标")
			if len(indicators.RSIPeriods) > 0 {
				sb.WriteString(fmt.Sprintf("（周期：%v）", indicators.RSIPeriods))
			}
			sb.WriteString("\n")
		}
		if indicators.EnableATR {
			sb.WriteString("- ATR指标")
			if len(indicators.ATRPeriods) > 0 {
				sb.WriteString(fmt.Sprintf("（周期：%v）", indicators.ATRPeriods))
			}
			sb.WriteString("\n")
		}
		if indicators.EnableBOLL {
			sb.WriteString("- 布林带（BOLL）- 上轨/中轨/下轨")
			if len(indicators.BOLLPeriods) > 0 {
				sb.WriteString(fmt.Sprintf("（周期：%v）", indicators.BOLLPeriods))
			}
			sb.WriteString("\n")
		}
		if indicators.EnableVolume {
			sb.WriteString("- 成交量数据\n")
		}
		if indicators.EnableOI {
			sb.WriteString("- 持仓量（OI）数据\n")
		}
		if indicators.EnableFundingRate {
			sb.WriteString("- 资金费率\n")
		}
		if len(c.CoinSource.StaticCoins) > 0 || c.CoinSource.UseCoinPool || c.CoinSource.UseOITop {
			sb.WriteString("- AI500 / OI_Top 筛选标签（如有）\n")
		}
		if indicators.EnableQuantData {
			sb.WriteString("- 量化数据（机构/散户资金流向、持仓变化、多周期价格变化）\n")
		}
	} else {
		sb.WriteString(fmt.Sprintf("- %s price series", kline.PrimaryTimeframe))
		if kline.EnableMultiTimeframe {
			sb.WriteString(fmt.Sprintf(" + %s K-line series\n", kline.LongerTimeframe))
		} else {
			sb.WriteString("\n")
		}
		if indicators.EnableEMA {
			sb.WriteString("- EMA indicators")
			if len(indicators.EMAPeriods) > 0 {
				sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.EMAPeriods))
			}
			sb.WriteString("\n")
		}
		if indicators.EnableMACD {
			sb.WriteString("- MACD indicators\n")
		}
		if indicators.EnableRSI {
			sb.WriteString("- RSI indicators")
			if len(indicators.RSIPeriods) > 0 {
				sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.RSIPeriods))
			}
			sb.WriteString("\n")
		}
		if indicators.EnableATR {
			sb.WriteString("- ATR indicators")
			if len(indicators.ATRPeriods) > 0 {
				sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.ATRPeriods))
			}
			sb.WriteString("\n")
		}
		if indicators.EnableBOLL {
			sb.WriteString("- Bollinger Bands (BOLL) - Upper/Middle/Lower bands")
			if len(indicators.BOLLPeriods) > 0 {
				sb.WriteString(fmt.Sprintf(" (periods: %v)", indicators.BOLLPeriods))
			}
			sb.WriteString("\n")
		}
		if indicators.EnableVolume {
			sb.WriteString("- Volume data\n")
		}
		if indicators.EnableOI {
			sb.WriteString("- Open Interest (OI) data\n")
		}
		if indicators.EnableFundingRate {
			sb.WriteString("- Funding rate\n")
		}
		if len(c.CoinSource.StaticCoins) > 0 || c.CoinSource.UseCoinPool || c.CoinSource.UseOITop {
			sb.WriteString("- AI500 / OI_Top filter tags (if available)\n")
		}
		if indicators.EnableQuantData {
			sb.WriteString("- Quantitative data (institutional/retail fund flow, position changes, multi-period price changes)\n")
		}
	}
	return sb.String()
}

// GetDefaultStrategyConfig returns the default strategy configuration for the given language
func GetDefaultStrategyConfig(lang string) StrategyConfig {
	config := StrategyConfig{
		CoinSource: CoinSourceConfig{
			SourceType:     "coinpool",
			UseCoinPool:    true,
			CoinPoolLimit:  10,
			CoinPoolAPIURL: "http://nofxaios.com:30006/api/ai500/list?auth=cm_568c67eae410d912c54c",
			UseOITop:       false,
			OITopLimit:     20,
			OITopAPIURL:    "http://nofxaios.com:30006/api/oi/top-ranking?limit=20&duration=1h&auth=cm_568c67eae410d912c54c",
		},
		Indicators: IndicatorConfig{
			Klines: KlineConfig{
				PrimaryTimeframe:     "5m",
				PrimaryCount:         30,
				LongerTimeframe:      "4h",
				LongerCount:          10,
				EnableMultiTimeframe: true,
				SelectedTimeframes:   []string{"5m", "15m", "1h", "4h"},
			},
			EnableRawKlines:    true, // Required - raw OHLCV data for AI analysis
			EnableEMA:          false,
			EnableMACD:         false,
			EnableRSI:          false,
			EnableATR:          false,
			EnableBOLL:         false,
			EnableVolume:       true,
			EnableOI:           true,
			EnableFundingRate:  true,
			EMAPeriods:         []int{20, 50},
			RSIPeriods:         []int{7, 14},
			ATRPeriods:         []int{14},
			MACDFastPeriod:     12, // default MACD fast period
			MACDSlowPeriod:     26, // default MACD slow period
			BOLLPeriods:        []int{20},
			EnableQuantData:    true,
			QuantDataAPIURL:    "http://nofxaios.com:30006/api/coin/{symbol}?include=netflow,oi,price&auth=cm_568c67eae410d912c54c",
			EnableQuantOI:      true,
			EnableQuantNetflow: true,
			// OI ranking data - market-wide OI increase/decrease rankings
			EnableOIRanking:   true,
			OIRankingAPIURL:   "http://nofxaios.com:30006",
			OIRankingDuration: "1h",
			OIRankingLimit:    10,
		},
		RiskControl: RiskControlConfig{
			MaxPositions:                 3,   // Max 3 coins simultaneously (CODE ENFORCED)
			BTCETHMaxLeverage:            5,   // BTC/ETH exchange leverage (AI guided)
			AltcoinMaxLeverage:           5,   // Altcoin exchange leverage (AI guided)
			BTCETHMaxPositionValueRatio:  5.0, // BTC/ETH: max position = 5x equity (CODE ENFORCED)
			AltcoinMaxPositionValueRatio: 1.0, // Altcoin: max position = 1x equity (CODE ENFORCED)
			MaxMarginUsage:               0.9, // Max 90% margin usage (CODE ENFORCED)
			MinPositionSize:              12,  // Min 12 USDT per position (CODE ENFORCED)
			MinRiskRewardRatio:           3.0, // Min 3:1 profit/loss ratio (AI guided)
			MinConfidence:                75,  // Min 75% confidence (AI guided)
			// Drawdown monitoring defaults (CODE ENFORCED - automatic profit protection)
			DrawdownMonitoringEnabled: true, // Enable drawdown monitoring by default
			DrawdownCheckInterval:     60,   // Check every 60 seconds (1 minute)
			MinProfitThreshold:        5.0,  // Start monitoring when profit > 5%
			DrawdownCloseThreshold:    40.0, // Close when profit drops 40% from peak (e.g. 10% -> 6%)
		},
	}

	if lang == "zh" {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `
			# 你是一个专业的量化交易AI助手，负责分析市场数据并做出交易决策。

			## 你的任务
			1. **分析账户状态**: 评估当前风险水平、保证金使用率、持仓情况
			2. **分析当前持仓**: 判断是否需要止盈、止损、加仓或持有
			3. **分析候选币种**: 评估新的交易机会，结合技术分析和资金流向
			4. **做出决策**: 输出明确的交易决策，包含详细的推理过程

			## 决策原则
			### 风险优先
			- 单个持仓亏损达到-5%必须止损
			- 优先保护资本，再考虑盈利
			### 跟踪止盈
			- 当持仓盈亏从峰值回撤30%时，考虑部分或全部止盈
			- 例如：Peak PnL +5%，Current PnL +3.5% → 回撤了30%，应该止盈

			### 顺势交易
			- 只在多个时间框架趋势一致时进场
			- 结合持仓量(OI)变化判断资金流向真实性
			- OI增加+价格上涨 = 强多头趋势
			- OI减少+价格上涨 = 空头平仓（可能反转）

			### 分批操作
			- 分批建仓：第一次开仓不超过目标仓位的50%
			- 分批止盈：盈利3%平33%，盈利5%平50%，盈利8%全平
			- 只在盈利仓位上加仓，永远不要追亏损

			## 输出格式要求
			**必须**使用以下JSON格式输出决策：
			` + "```json" + `
			[
			{
				"symbol": "BTCUSDT",
				"action": "HOLD|PARTIAL_CLOSE|FULL_CLOSE|ADD_POSITION|OPEN_NEW|WAIT",
				"leverage": 3,
				"position_size_usd": 1000,
				"stop_loss": 42000,
				"take_profit": 48000,
				"confidence": 85,
				"reasoning": "详细的推理过程，说明为什么做出这个决策"
			}
			]
			` + "```" + `

			### 字段说明
			- **symbol**: 交易对（必需）
			- **action**: 动作类型（必需）
			- HOLD: 持有当前仓位
			- PARTIAL_CLOSE: 部分平仓
			- FULL_CLOSE: 全部平仓
			- ADD_POSITION: 在现有仓位上加仓
			- OPEN_NEW: 开设新仓位
			- WAIT: 等待，不采取任何行动
			- **leverage**: 杠杆倍数（开新仓时必需）
			- **position_size_usd**: 仓位大小（USDT，开新仓时必需）
			- **stop_loss**: 止损价格（开新仓时建议提供）
			- **take_profit**: 止盈价格（开新仓时建议提供）
			- **confidence**: 信心度（0-100）
			- **reasoning**: 推理过程（必需，必须详细说明决策依据）

			## 重要提醒
			1. **永远不要**混淆已实现盈亏和未实现盈亏
			2. **永远记得**考虑杠杆对盈亏的放大作用
			3. **永远关注**Peak PnL，这是判断止盈的关键指标
			4. **永远结合**持仓量(OI)变化来判断趋势真实性
			5. **永远遵守**风险管理规则，保护资本是第一位的
			`,
			TradingFrequency: `
			# ⏱️ 交易频率意识
			- 优秀交易员：每天2-12笔 ≈ 每小时0.1-0.5笔
			- 单笔持仓时间 ≥ 7-180分钟
			如果你发现自己每个周期都在交易 → 标准太低；如果持仓不到7分钟就平仓 → 太冲动。
			`,
			EntryStandards: `# 🎯 入场标准（严格）
			只在多个信号共振时入场。自由使用任何有效的分析方法，避免单一指标、信号矛盾、横盘震荡、或平仓后立即重新开仓等低质量行为。
			`,
			DecisionProcess: `
			# 📋 决策流程
			1. **分析账户风险**:
			- 当前保证金使用率是否在安全范围？
			- 是否有足够资金开新仓？

			2. **分析现有持仓**（如果有）:
			- 是否触发止损条件？
			- 是否触发跟踪止盈条件？
			- 是否适合加仓？

			3. **分析候选币种**（如果有）:
			- 技术形态是否符合进场条件？
			- 持仓量变化是否支持趋势？
			- 多个时间框架是否共振？

			4. **输出决策**:
			- 使用规定的JSON格式
			- 提供详细的推理过程
			- 给出明确的行动指令

			### 输出示例

			` + "```json" + `
			[
			{
				"symbol": "PIPPINUSDT",
				"action": "PARTIAL_CLOSE",
				"confidence": 85,
				"reasoning": "当前PnL +2.96%，接近历史峰值+2.99%（回撤仅0.03%）。建议部分平仓锁定利润，因为：1) 持仓时间仅11分钟，已获得3%收益；2) 5分钟K线显示价格接近短期阻力位；3) 成交量开始萎缩，上涨动能减弱。建议平仓50%，剩余仓位设置跟踪止盈在峰值回撤20%处。"
			},
			{
				"symbol": "HUSDT",
				"action": "OPEN_NEW",
				"leverage": 3,
				"position_size_usd": 500,
				"stop_loss": 0.1560,
				"take_profit": 0.1720,
				"confidence": 75,
				"reasoning": "HUSDT在5分钟时间框架突破关键阻力位0.1630，持仓量1小时内增加+1.57M (+0.89%)，配合价格上涨+4.92%，符合'OI增加+价格上涨'的强多头模式。15分钟和1小时时间框架均呈现上涨趋势，多周期共振。建议开仓做多，止损设在突破点下方-5%，止盈目标+8%。"
			}
			]
			` + "```" + `
			5. 先写思维链，再输出结构化JSON`,
		}
	} else {
		config.PromptSections = PromptSectionsConfig{
			RoleDefinition: `
			# You are a professional quantitative trading AI assistant responsible for analyzing market data and making trading decisions.

			## Your Mission

			1. **Analyze Account Status**: Evaluate current risk level, margin usage, and positions
			2. **Analyze Current Positions**: Determine if stop-loss, take-profit, scaling, or holding is needed
			3. **Analyze Candidate Coins**: Assess new trading opportunities using technical analysis and capital flows
			4. **Make Decisions**: Output clear trading decisions with detailed reasoning

			## Decision Principles

			### Risk First
			- Must stop-loss when single position loss reaches -5%
			- Capital protection first, profit second

			### Trailing Take-Profit
			- Consider partial/full profit-taking when PnL pulls back 30% from peak
			- Example: Peak PnL +5%, Current PnL +3.5% → 30% drawdown, should take profit

			### Trend Following
			- Only enter when trends align across multiple timeframes
			- Use Open Interest (OI) changes to validate capital flow authenticity
			- OI up + Price up = Strong bullish trend
			- OI down + Price up = Shorts covering (potential reversal)

			### Scale Operations
			- Scale-in: First entry max 50% of target position
			- Scale-out: Close 33% at +3%, 50% at +5%, 100% at +8%
			- Only add to winning positions, never average down losers

			## Output Format Requirements

			**Must** use the following JSON format:

			` + "```json" + `
			[
			{
				"symbol": "BTCUSDT",
				"action": "HOLD|PARTIAL_CLOSE|FULL_CLOSE|ADD_POSITION|OPEN_NEW|WAIT",
				"leverage": 3,
				"position_size_usd": 1000,
				"stop_loss": 42000,
				"take_profit": 48000,
				"confidence": 85,
				"reasoning": "Detailed reasoning explaining why this decision was made"
			}
			]
			` + "```" + `

			### Field Descriptions

			- **symbol**: Trading pair (required)
			- **action**: Action type (required)
			- HOLD: Hold current position
			- PARTIAL_CLOSE: Partially close position
			- FULL_CLOSE: Fully close position
			- ADD_POSITION: Add to existing position
			- OPEN_NEW: Open new position
			- WAIT: Wait, take no action
			- **leverage**: Leverage multiplier (required for new positions)
			- **position_size_usd**: Position size in USDT (required for new positions)
			- **stop_loss**: Stop-loss price (recommended for new positions)
			- **take_profit**: Take-profit price (recommended for new positions)
			- **confidence**: Confidence level (0-100)
			- **reasoning**: Detailed reasoning (required, must explain decision basis)

			## Critical Reminders

			1. **Never** confuse realized and unrealized P&L
			2. **Always remember** leverage amplifies both gains and losses
			3. **Always watch** Peak PnL - it's key for take-profit decisions
			4. **Always combine** OI changes to validate trend authenticity
			5. **Always follow** risk management rules - capital protection is priority #1
			`,
			TradingFrequency: `
			# ⏱️ Trading Frequency Awareness
			- Excellent trader: 2-12 trades per day ≈ 0.1-0.5 trades per hour
			- Single position holding time ≥ 7-180 minutes
			If you find yourself trading every cycle → standards are too low; if closing positions in <7 minutes → too impulsive.
			`,
			EntryStandards: `
			# 🎯 Entry Standards (Strict)
			Only enter positions when multiple signals resonate. Freely use any effective analysis methods, avoid low-quality behaviors such as single indicators, contradictory signals, sideways oscillation, or immediately restarting after closing positions.
			`,
			DecisionProcess: `
			# 📋 Decision Process

			### Decision Steps
			1. **Analyze Account Risk**:
			- Is margin usage within safe range?
			- Is there enough capital for new positions?

			2. **Analyze Existing Positions** (if any):
			- Is stop-loss triggered?
			- Is trailing take-profit triggered?
			- Is it suitable to scale-in?

			3. **Analyze Candidate Coins** (if any):
			- Does technical pattern meet entry criteria?
			- Do OI changes support the trend?
			- Do multiple timeframes align?

			4. **Output Decision**:
			- Use the specified JSON format
			- Provide detailed reasoning
			- Give clear action instructions

			### Output Example

			` + "```json" + `
			[
			{
				"symbol": "PIPPINUSDT",
				"action": "PARTIAL_CLOSE",
				"confidence": 85,
				"reasoning": "Current PnL +2.96%, near historical peak +2.99% (only 0.03% pullback). Suggest partial close to lock profits because: 1) Only 11 minutes holding time with 3% gain; 2) 5M chart shows price approaching short-term resistance; 3) Volume declining, upward momentum weakening. Recommend closing 50%, set trailing stop at 20% pullback from peak for remainder."
			},
			{
				"symbol": "HUSDT",
				"action": "OPEN_NEW",
				"leverage": 3,
				"position_size_usd": 500,
				"stop_loss": 0.1560,
				"take_profit": 0.1720,
				"confidence": 75,
				"reasoning": "HUSDT broke key resistance 0.1630 on 5M timeframe. OI increased +1.57M (+0.89%) in 1H paired with price +4.92%, matching 'OI up + price up' strong bullish pattern. Both 15M and 1H timeframes show uptrend, multi-timeframe resonance confirmed. Recommend long entry, stop-loss -5% below breakout, target +8% profit."
			}
			]
			` + "```" + `
			5. Write chain of thought first, then output structured JSON
			`,
		}
	}

	return config
}

// Create create a strategy
func (s *StrategyStore) Create(strategy *Strategy) error {
	_, err := s.db.Exec(`
		INSERT INTO strategies (id, user_id, name, description, is_active, is_default, config)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, strategy.ID, strategy.UserID, strategy.Name, strategy.Description, strategy.IsActive, strategy.IsDefault, strategy.Config)
	return err
}

// Update update a strategy
func (s *StrategyStore) Update(strategy *Strategy) error {
	_, err := s.db.Exec(`
		UPDATE strategies SET
			name = ?, description = ?, config = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ?
	`, strategy.Name, strategy.Description, strategy.Config, strategy.ID, strategy.UserID)
	return err
}

// Delete delete a strategy
func (s *StrategyStore) Delete(userID, id string) error {
	// do not allow deleting system default strategy
	var isDefault bool
	s.db.QueryRow(`SELECT is_default FROM strategies WHERE id = ?`, id).Scan(&isDefault)
	if isDefault {
		return fmt.Errorf("cannot delete system default strategy")
	}

	_, err := s.db.Exec(`DELETE FROM strategies WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// List get user's strategy list
func (s *StrategyStore) List(userID string) ([]*Strategy, error) {
	// get user's own strategies + system default strategy
	rows, err := s.db.Query(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE user_id = ? OR is_default = 1
		ORDER BY is_default DESC, created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var strategies []*Strategy
	for rows.Next() {
		var st Strategy
		var createdAt, updatedAt string
		err := rows.Scan(
			&st.ID, &st.UserID, &st.Name, &st.Description,
			&st.IsActive, &st.IsDefault, &st.Config,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}
		st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		strategies = append(strategies, &st)
	}
	return strategies, nil
}

// Get get a single strategy
func (s *StrategyStore) Get(userID, id string) (*Strategy, error) {
	var st Strategy
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE id = ? AND (user_id = ? OR is_default = 1)
	`, id, userID).Scan(
		&st.ID, &st.UserID, &st.Name, &st.Description,
		&st.IsActive, &st.IsDefault, &st.Config,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &st, nil
}

// GetActive get user's currently active strategy
func (s *StrategyStore) GetActive(userID string) (*Strategy, error) {
	var st Strategy
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE user_id = ? AND is_active = 1
	`, userID).Scan(
		&st.ID, &st.UserID, &st.Name, &st.Description,
		&st.IsActive, &st.IsDefault, &st.Config,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		// no active strategy, return system default strategy
		return s.GetDefault()
	}
	if err != nil {
		return nil, err
	}
	st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &st, nil
}

// GetDefault get system default strategy
func (s *StrategyStore) GetDefault() (*Strategy, error) {
	var st Strategy
	var createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT id, user_id, name, description, is_active, is_default, config, created_at, updated_at
		FROM strategies
		WHERE is_default = 1
		LIMIT 1
	`).Scan(
		&st.ID, &st.UserID, &st.Name, &st.Description,
		&st.IsActive, &st.IsDefault, &st.Config,
		&createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	st.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	st.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &st, nil
}

// SetActive set active strategy (will first deactivate other strategies)
func (s *StrategyStore) SetActive(userID, strategyID string) error {
	// begin transaction
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// first deactivate all strategies for the user
	_, err = tx.Exec(`UPDATE strategies SET is_active = 0 WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}

	// activate specified strategy
	_, err = tx.Exec(`UPDATE strategies SET is_active = 1 WHERE id = ? AND (user_id = ? OR is_default = 1)`, strategyID, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Duplicate duplicate a strategy (used to create custom strategy based on default strategy)
func (s *StrategyStore) Duplicate(userID, sourceID, newID, newName string) error {
	// get source strategy
	source, err := s.Get(userID, sourceID)
	if err != nil {
		return fmt.Errorf("failed to get source strategy: %w", err)
	}

	// create new strategy
	newStrategy := &Strategy{
		ID:          newID,
		UserID:      userID,
		Name:        newName,
		Description: "Created based on [" + source.Name + "]",
		IsActive:    false,
		IsDefault:   false,
		Config:      source.Config,
	}

	return s.Create(newStrategy)
}

// ParseConfig parse strategy configuration JSON
func (s *Strategy) ParseConfig() (*StrategyConfig, error) {
	var config StrategyConfig
	if err := json.Unmarshal([]byte(s.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to parse strategy configuration: %w", err)
	}
	return &config, nil
}

// SetConfig set strategy configuration
func (s *Strategy) SetConfig(config *StrategyConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to serialize strategy configuration: %w", err)
	}
	s.Config = string(data)
	return nil
}
