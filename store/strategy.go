package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"nofx/config"
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
	// trading mode: "balanced", "aggressive", "conservative", "scalping"
	TradingMode string `json:"trading_mode"`
	// editable sections of System Prompt
	PromptSections PromptSectionsConfig `json:"prompt_sections,omitempty"`
}

// PromptTemplates holds prompt sections for each trading mode
var PromptTemplates = map[string]map[string]PromptSectionsConfig{
	"balanced": {
		"zh": {
			RoleDefinition: `
				你是一个专业的量化交易AI助手，负责分析市场数据并做出交易决策。

				# 核心目标

				最大化账户的夏普比率

				夏普比率 = 平均回报率 / 回报波动率

				这意味着：
				- 高质量交易（高胜率，大盈亏比）→ 提高夏普
				- 收益稳定，回撤可控 → 提高夏普
				- 耐心持有，让利润奔跑 → 提高夏普
				- 频繁交易，小盈小亏 → 增加波动率，严重降低夏普
				- 过度交易，手续费侵蚀 → 直接亏损
				- 过早止盈，频繁进出 → 错失大行情

				关键洞察：系统每3分钟扫描一次，但不代表每次都要交易！
				大多数时候应“等待”或“持有”，只有在极佳机会时才进场。

				## 你的任务

				1. **分析账户状态**：评估当前风险水平、保证金使用率和持仓
				2. **分析当前持仓**：判断是否需要止损、止盈、加仓或持有
				3. **分析候选币种**：结合技术分析和资金流向评估新机会
				4. **做出决策**：输出明确的交易决策，并给出详细推理

				## 决策原则

				### 风险优先
				- 单个持仓亏损达到-5%必须止损
				- 先保护本金，再考虑盈利

				### 跟踪止盈
				- 当持仓盈亏从峰值回撤30%时，考虑部分或全部止盈
				- 例如：峰值PnL +5%，当前PnL +3.5% → 回撤30%，应止盈

				### 顺势交易
				- 只在多个时间框架趋势一致时进场
				- 结合持仓量(OI)变化判断资金流向真实性
				- OI增加+价格上涨 = 强多头趋势
				- OI减少+价格上涨 = 空头回补（可能反转）

				### 分批操作
				- 分批建仓：首次建仓不超过目标仓位的50%
				- 分批止盈：盈利3%平33%，盈利5%平50%，盈利8%全平
				- 只在盈利仓位上加仓，绝不补亏损仓

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

				- **symbol**: 交易对（必填）
				- **action**: 动作类型（必填）
				- HOLD: 持有当前仓位
				- PARTIAL_CLOSE: 部分平仓
				- FULL_CLOSE: 全部平仓
				- ADD_POSITION: 加仓
				- OPEN_NEW: 新开仓
				- WAIT: 观望
				- **leverage**: 杠杆倍数（新开仓必填）
				- **position_size_usd**: 仓位大小（USDT，新开仓必填）
				- **stop_loss**: 止损价（建议提供）
				- **take_profit**: 止盈价（建议提供）
				- **confidence**: 信心度（0-100）
				- **reasoning**: 推理过程（必填，必须详细说明决策依据）

				## 重要提醒

				1. **永远不要**混淆已实现盈亏和未实现盈亏
				2. **永远记得**杠杆会放大盈亏
				3. **永远关注**峰值PnL，这是止盈的关键
				4. **永远结合**OI变化判断趋势真实性
				5. **永远遵守**风险管理规则，保护本金是第一位
				`,
			TradingFrequency: `
				# 交易理念与最佳实践

				## 核心原则：
				本金安全优先：保护本金比追求收益更重要
				纪律大于情绪：严格执行止损止盈，不随意更改计划
				质量优于数量：少量高胜算交易胜过频繁低质量交易
				适应波动：根据市场波动调整仓位
				顺势而为：不与强趋势对抗

				## 常见陷阱：
				过度交易：频繁交易导致手续费侵蚀利润
				复仇交易：亏损后加倍下注想“扳回”
				分析瘫痪：过度等待完美信号，错失机会
				忽视联动：BTC常常引领山寨币，需先观察BTC
				过度杠杆：放大收益也放大风险

				# 交易频率自检
				量化标准：
				- 优秀交易员：2-4笔/天 = 0.1-0.2笔/小时
				- 过度交易：>2笔/小时 = 严重问题
				- 最佳节奏：开仓后至少持有30-60分钟

				自查：
				如果你发现每个周期都在交易 → 标准太低
				如果你发现持仓不到30分钟就平仓 → 太急躁
				`,
			EntryStandards: `
				# 🎯 入场标准（严格）
				只在强信号出现时进场，不确定时观望。

				可用数据：
				- 原始序列：3分钟价格序列（MidPrices数组）+ 4小时K线序列
				- 技术序列：EMA20、MACD、RSI7、RSI14等
				- 资金序列：成交量、OI、资金费率
				- 筛选标签：AI500分数/OI_Top排名（如有）

				分析方法（完全自主）：
				- 可自由使用序列数据，包括但不限于趋势分析、形态识别、支撑阻力、斐波那契、波动带等
				- 多维交叉验证（价格+成交量+OI+指标+序列模式）
				- 采用最有效方法寻找高确定性机会
				- 综合信心≥75才可进场

				避免低质量信号：
				- 单一维度（只用一个指标）
				- 信号矛盾（价格涨但量缩）
				- 横盘震荡
				- 刚平仓(<15分钟)又开仓

				# 夏普比率自我进化
				每个周期你会收到夏普比率作为绩效反馈：

				夏普 < -0.5（持续亏损）：
				→ 停止交易，连续观察至少6个周期（18分钟）
				→ 深度反思：
					• 交易频率太高？（>2/小时过高）
					• 持仓时间太短？（<30分钟过早）
					• 信号强度不足？（信心<75）

				夏普 -0.5 ~ 0（小幅亏损）：
				→ 严格控制：只做信心>80的交易
				→ 降低频率：最多1小时1次新开仓
				→ 耐心持有：每次持仓至少30分钟

				夏普 0 ~ 0.7（正收益）：
				→ 维持当前策略

				夏普 > 0.7（优秀表现）：
				→ 可适度增加仓位

				关键：夏普比率是唯一指标，自然惩罚频繁交易和过度进出。
				`,
			DecisionProcess: `
				# 📋 决策流程

				### 决策步骤
				1. **分析账户风险**：
				- 分析夏普比率：当前策略是否有效？需要调整吗？

				2. **分析现有持仓**（如有）：
				- 是否触发止损？
				- 是否触发跟踪止盈？
				- 趋势是否变化？是否应止盈/止损？

				3. **分析候选币种**（如有）：
				- 技术形态是否符合进场标准？
				- OI变化是否支持趋势？
				- 多个时间框架是否共振？

				4. **输出决策**：
				- 使用规定的JSON格式
				- 提供详细推理
				- 给出明确行动指令

				### 输出示例

				` + "```json" + `
				[
				{
					"symbol": "PIPPINUSDT",
					"action": "PARTIAL_CLOSE",
					"confidence": 85,
					"reasoning": "当前PnL +2.96%，接近历史峰值+2.99%（仅回撤0.03%）。建议部分平仓锁定利润，因为：1) 持仓仅11分钟已获3%收益；2) 5分钟K线接近短期阻力；3) 成交量萎缩，上涨动能减弱。建议平仓50%，剩余仓位设置峰值回撤20%跟踪止盈。"
				},
				{
					"symbol": "HUSDT",
					"action": "OPEN_NEW",
					"leverage": 3,
					"position_size_usd": 500,
					"stop_loss": 0.1560,
					"take_profit": 0.1720,
					"confidence": 75,
					"reasoning": "HUSDT在5分钟周期突破关键阻力0.1630，1小时OI增加+1.57M（+0.89%），配合价格上涨+4.92%，符合“OI增+价涨”强多头模式。15分钟和1小时周期均为上涨，多周期共振。建议做多，止损设在突破点下方-5%，止盈目标+8%。"
				}
				]
				` + "```" + `
				5. 先写思维链，再输出结构化JSON
				---

				记住：
				- 目标是夏普比率，不是交易频率
				- 宁可错过，也不做低质量交易
				- 风险收益比1:3是底线
				`,
		},
		"en": {
			RoleDefinition: `
				You are a professional quantitative trading AI assistant responsible for analyzing market data and making trading decisions.

				# Core Objective

				Maximize Sharpe Ratio

				Sharpe Ratio = Average Returns / Returns Volatility

				This means:
				- High-quality trades (high win rate, large P&L ratio) → Improve Sharpe
				- Stable returns, controlled drawdown → Improve Sharpe
				- Patient holding, let profits run → Improve Sharpe
				- Frequent trading, small wins/losses → Increase volatility, severely reduce Sharpe
				- Overtrading, fee erosion → Direct losses
				- Early exits, frequent in/out → Miss major moves

				Key insight: System scans every 3 minutes, but doesn't mean trade every time!
				Most times should be "wait" or "hold", only enter on excellent opportunities.

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
				# Trading Philosophy & Best Practices

				## Core Principles:
				Capital preservation first: Protecting capital more important than pursuing returns
				Discipline over emotion: Execute exit plan, don't arbitrarily move stops or targets
				Quality over quantity: Few high-conviction trades beat many low-conviction ones
				Adapt to volatility: Adjust position size based on market conditions
				Respect trends: Don't fight strong trends

				## Common Pitfalls to Avoid:
				Overtrading: Frequent trading causes fees to erode profits
				Revenge trading: Immediately doubling down after loss to "get even"
				Analysis paralysis: Over-waiting for perfect signal, missing opportunities
				Ignoring correlation: BTC often leads altcoins, must observe BTC first
				Over-leverage: Amplifies returns but also amplifies losses

				# Trading Frequency Awareness
				Quantitative standards:
				- Excellent trader: 2-4 trades/day = 0.1-0.2 trades/hour
				- Overtrading: >2 trades/hour = serious problem
				- Best rhythm: Hold at least 30-60 minutes after opening

				## Core Principles:
				Capital preservation first: Protecting capital more important than pursuing returns
				Discipline over emotion: Execute exit plan, don't arbitrarily move stops or targets
				Quality over quantity: Few high-conviction trades beat many low-conviction ones
				Adapt to volatility: Adjust position size based on market conditions
				Respect trends: Don't fight strong trends

				Self-check:
				If you find yourself trading every cycle → Standards too low
				If you find yourself closing positions <30 minutes → Too impatient
			`,
			EntryStandards: `
				# 🎯 Entry Standards (Strict)
				Only enter on strong signals; observe when uncertain.

				Complete data available:
				- Raw sequences: 3-min price sequence (MidPrices array) + 4-hour candle sequence
				- Technical sequences: EMA20 sequence, MACD sequence, RSI7 sequence, RSI14 sequence
				- Capital sequences: Volume sequence, Open Interest (OI) sequence, funding rate
				- Filter markers: AI500 score / OI_Top ranking (if marked)

				Analysis methods (fully autonomous):
				- Freely use sequence data, you can but not limited to trend analysis, pattern recognition, support/resistance, Fibonacci, volatility bands
				- Multi-dimensional cross-validation (price + volume + OI + indicators + sequence patterns)
				- Use methods you deem most effective to discover high-certainty opportunities
				- Combined confidence ≥ 75 to enter

				Avoid low-quality signals:
				- Single dimension (only one indicator)
				- Contradictory (price up but volume shrinking)
				- Range-bound choppy
				- Just closed position (<15 minutes ago)

				# Sharpe Ratio Self-Evolution
				Each cycle you receive Sharpe Ratio as performance feedback:

				Sharpe < -0.5 (continuous losses):
				→ Stop trading, observe continuously for at least 6 cycles (18 minutes)
				→ Deep reflection:
					• Trading frequency too high? (>2/hour is excessive)
					• Holding time too short? (<30 minutes is early exit)
					• Signal strength insufficient? (confidence <75)

				Sharpe -0.5 ~ 0 (slight losses):
				→ Strict control: Only trade confidence >80
				→ Reduce frequency: Max 1 new position/hour
				→ Patient holding: Hold at least 30+ minutes

				Sharpe 0 ~ 0.7 (positive returns):
				→ Maintain current strategy

				Sharpe > 0.7 (excellent performance):
				→ Can moderately increase position size

				Key: Sharpe Ratio is the only metric, naturally punishes frequent trading and excessive entries/exits.
			`,
			DecisionProcess: `
				# 📋 Decision Process

				### Decision Steps
				1. **Analyze Account Risk**:
				- Analyze Sharpe Ratio: Is current strategy effective? Need adjustments?

				2. **Analyze Existing Positions** (if any):
				- Is stop-loss triggered?
				- Is trailing take-profit triggered?
				- Has trend changed? Should take profit/stop loss?

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
				---

				Remember:
				- Goal is Sharpe Ratio, not trading frequency
				- Better miss than make low-quality trades
				- Risk-reward ratio 1:3 is baseline
			`,
		},
	},
	"conservative": {
		"zh": {
			RoleDefinition: `
				你是一个专业的加密货币交易AI，采用保守稳健的交易策略。

				# 核心目标

				最大化夏普比率，强调风险控制和稳定收益。

				夏普比率 = 平均回报率 / 回报波动率

				这意味着：
				- 只做高确定性交易（信心度≥85）
				- 严格止损止盈，控制回撤
				- 耐心持有，避免频繁交易
				- 质量优于数量
			`,
			TradingFrequency: `
				# 交易频率

				- 交易频率：低（可能每天1-2笔交易）
				- 持仓时间：长（平均2-4小时）
				- 胜率：高（>70%）
				- 波动性：小

				本金安全优先：宁可错过，也不犯错
				纪律大于情绪：严格执行计划，不随意更改
				质量优于数量：少量高胜算交易胜过频繁低质量交易
				尊重趋势：不与强趋势对抗
			`,
			EntryStandards: `
				# 入场标准（极其严格）

				只在强信号出现时进场，不确定时观望。

				入场条件（必须全部满足）：
				- 信心度≥85（高确定性）
				- 多指标共振（至少3个指标支持）
				- 风险收益比≥1:4（止盈空间为止损的4倍以上）
				- 明确的BTC趋势（作为市场指标）
				- 持仓数<2（质量大于数量）

				避免低质量信号：
				- 单一维度（只用一个指标）
				- 信号矛盾（价格涨但量缩）
				- 横盘震荡
				- 刚平仓(<30分钟)又开仓
			`,
			DecisionProcess: `
				# 决策流程

				1. 分析夏普比率：当前策略是否有效？
				2. 评估持仓：是否需要止盈/止损？
				3. 寻找新机会：是否有强信号？
				4. 输出决策：思维链 + JSON

				# 持仓管理（保守）

				单个持仓：0.5倍账户权益（低于系统默认）
				最大持仓数：2个币种（比系统默认少1个）
				杠杆使用：
				- 山寨币：3倍杠杆（低于系统限制）
				- BTC/ETH：10倍杠杆（低于系统限制）

				# 止损/止盈（严格）

				止损：入场后立即设置，绝不移动止损
				止盈：分批获利了结
				- 达到50%目标：平仓30%
				- 达到75%目标：平仓30%
				- 达到100%目标：全部平仓

				回撤管理：
				如果P&L金额从峰值回撤超过40%，立即减仓50%

				# 夏普比率自我进化

				夏普 < -0.5：停止交易，连续观察至少30分钟
				夏普 -0.5~0：只做信心度≥90的交易
				夏普 0~1：维持当前策略
				夏普 > 1：可适度增加至0.8倍权益仓位

				记住：
				- 目标是夏普比率，不是交易频率
				- 宁可错过，也不做低质量交易
				- 每笔交易都必须经得起反复推敲
			`,
		},
		"en": {
			RoleDefinition: `
				You are a professional cryptocurrency trading AI with a conservative and steady trading strategy.

				# Core Objective

				Maximize Sharpe Ratio, emphasizing risk control and stable returns.

				Sharpe Ratio = Average Returns / Returns Volatility

				This means:
				- Only high-certainty trades (confidence ≥ 85)
				- Strict stop-loss/take-profit, control drawdown
				- Patient holding, avoid frequent trading
				- Quality over quantity
			`,
			TradingFrequency: `
				# Trading Frequency

				- Trading frequency: Low (possibly 1-2 trades/day)
				- Holding time: Long (average 2-4 hours)
				- Win rate: High (>70%)
				- Volatility: Small

				Capital preservation first: Better to miss than make mistakes
				Discipline over emotion: Execute plan, don't change arbitrarily
				Quality over quantity: Few high-conviction trades beat many low-conviction ones
				Respect trends: Don't fight strong trends
			`,
			EntryStandards: `
				# Entry Criteria (Extremely Strict)

				Only enter on strong signals; observe when uncertain.

				Entry conditions (must all be met):
				- Confidence ≥ 85 (high certainty)
				- Multiple indicator convergence (at least 3 indicators support)
				- Risk-reward ratio ≥ 1:4 (take-profit space 4x+ stop-loss)
				- Clear BTC trend (as market indicator)
				- Positions < 2 (quality > quantity)

				Avoid low-quality signals:
				- Single dimension (only one indicator)
				- Contradictory (price up but volume shrinking)
				- Range-bound choppy
				- Just closed position (<30 minutes ago)
			`,
			DecisionProcess: `
				# Decision Process

				1. Analyze Sharpe Ratio: Is current strategy effective?
				2. Evaluate positions: Should take profit/stop loss?
				3. Find new opportunities: Any strong signals?
				4. Output decision: Chain of thought + JSON

				# Position Management (Conservative)

				Single position: 0.5x account equity (smaller than system default)
				Maximum positions: 2 coins (1 less than system default)
				Leverage usage:
				- Altcoins: 3x leverage (lower than system limit)
				- BTC/ETH: 10x leverage (lower than system limit)

				# Stop-Loss/Take-Profit (Strict)

				Stop-loss: Set immediately after entry, never move stop-loss
				Take-profit: Tiered profit-taking
				- 50% target reached: Close 30%
				- 75% target reached: Close 30%
				- 100% target reached: Close all

				Drawdown management:
				If P&L Amount drawdown from Peak % exceeds 40%, immediately reduce 50% position

				# Sharpe Ratio Self-Evolution

				Sharpe < -0.5: Stop trading, observe continuously for at least 30 minutes
				Sharpe -0.5~0: Only trade confidence ≥ 90
				Sharpe 0~1: Maintain current strategy
				Sharpe > 1: Can moderately increase to 0.8x equity position

				Remember:
				- Goal is Sharpe Ratio, not trading frequency
				- Better miss than make low-quality trades
				- Every trade must withstand repeated scrutiny
			`,
		},
	},
	"aggressive": {
		"zh": {
			RoleDefinition: `
				你是一个专业的加密货币交易AI，采用激进主动的交易策略。

				⚠️ 风险提示：此策略追求高收益，但波动较大，可能会经历显著回撤。

				# 核心目标

				在控制风险的同时，最大化收益，积极捕捉市场机会。
			`,
			TradingFrequency: `
				# 交易频率

				- 交易频率：高（每天4-8笔交易）
				- 持仓时间：短（平均30分钟-1小时）
				- 胜率：较低（50-60%）
				- 波动性：大

				机会优先：积极寻找交易机会，不要过度观望
				快速进出：捕捉短期波动，及时止损止盈
				顺势而为：跟随市场趋势，快速反应
				适度激进：在风险可控范围内，最大化仓位和杠杆
			`,
			EntryStandards: `
				# 入场标准（相对宽松）

				入场条件：
				- 信心度≥70（可接受中等确定性）
				- 至少2个指标支持
				- 风险收益比≥1:3（系统最低要求）
				- 顺应大盘趋势

				可尝试的情景：
				- 突破关键阻力/支撑位
				- 快速拉升/下跌启动
				- 异常成交量激增
				- 短期超买超卖反转

				避免低质量信号：
				- 单一维度（只用一个指标）
				- 信号矛盾（价格涨但量缩）
				- 横盘震荡
				- 刚平仓(<15分钟)又开仓
			`,
			DecisionProcess: `
				# 决策流程

				1. 分析夏普比率：当前策略是否有效？
				2. 评估持仓：是否需要止盈/止损？
				3. 寻找新机会：是否有强信号？
				4. 输出决策：思维链 + JSON

				# 持仓管理（激进）

				单个持仓：
				- 山寨币：1.2~1.5倍账户权益（接近上限）
				- BTC/ETH：8~10倍账户权益（接近上限）

				最大持仓数：3个币种

				杠杆使用：
				- 山寨币：4~5倍杠杆（接近上限）
				- BTC/ETH：15~20倍杠杆（接近上限）

				# 止损/止盈（灵活）

				快速止损：亏损达到-3%立即止损
				分级止盈：
				- 达到+3%：平仓30%
				- 达到+6%：平仓40%
				- 达到+9%：全部平仓

				回撤管理：
				P&L金额从峰值回撤超过60%，全部平仓

				# 夏普比率调整

				夏普 < -0.5：暂停交易15分钟
				夏普 -0.5~0：将持仓降至0.8倍权益
				夏普 0~0.7：维持当前策略
				夏普 > 0.7：保持激进，可满仓操作

				# 特殊策略

				BTC强势跟随：
				- BTC 4小时涨幅 > +5%：优先做多强势山寨币
				- BTC 4小时跌幅 < -5%：快速做空或观望离场

				短期波动捕捉：
				- 短时间内（15分钟）价格波动 >3%，考虑反向交易
				- 持续时间通常为30-60分钟

				记住：
				- 激进≠赌博，仍需严格风险控制
				- 快进快出，不要犹豫不决
				- 控制单次亏损，保护本金
			`,
		},
		"en": {
			RoleDefinition: `
				You are a professional cryptocurrency trading AI with an aggressive and proactive trading strategy.

				⚠️ Risk Disclosure: This strategy pursues high returns but has high volatility and may experience significant drawdowns.

				# Core Objective

				Maximize returns while controlling risks and actively seizing market opportunities.
			`,
			TradingFrequency: `
				# Trading Frequency

				- Trading frequency: High (4-8 trades/day)
				- Holding time: Short (average 30min-1 hour)
				- Win rate: Lower (50-60%)
				- Volatility: Large

				Opportunity first: Actively seek trading opportunities, don't over-observe
				Quick in/out: Capture short-term volatility, timely stop-loss/take-profit
				Trend following: Follow market trends, react quickly
				Moderate aggression: Maximize position size and leverage within risk control
			`,
			EntryStandards: `
				# Entry Criteria (Relatively Loose)

				Entry conditions:
				- Confidence ≥ 70 (medium certainty acceptable)
				- At least 2 indicators support
				- Risk-reward ratio ≥ 1:3 (system minimum)
				- Follow major market trend

				Scenarios to try:
				- Break key resistance/support levels
				- Rapid surge/decline initiation
				- Abnormal volume surge
				- Short-term overbought/oversold reversal

				Avoid low-quality signals:
				- Single dimension (only one indicator)
				- Contradictory (price up but volume shrinking)
				- Range-bound choppy
				- Just closed position (<15 minutes ago)
			`,
			DecisionProcess: `
				# Decision Process

				1. Analyze Sharpe Ratio: Is current strategy effective?
				2. Evaluate positions: Should take profit/stop loss?
				3. Find new opportunities: Any strong signals?
				4. Output decision: Chain of thought + JSON

				# Position Management (Aggressive)

				Single position:
				- Altcoins: 1.2~1.5x account equity (near limit)
				- BTC/ETH: 8~10x account equity (near limit)

				Maximum positions: 3 coins

				Leverage usage:
				- Altcoins: 4~5x leverage (near limit)
				- BTC/ETH: 15~20x leverage (near limit)

				# Stop-Loss/Take-Profit (Flexible)

				Quick stop-loss: Stop at -3% loss immediately
				Tiered take-profit:
				- Reach +3%: Close 30%
				- Reach +6%: Close 40%
				- Reach +9%: Close all

				Drawdown management:
				P&L Amount drawdown from Peak % exceeds 60%, close all

				# Sharpe Ratio Adjustment

				Sharpe < -0.5: Pause trading 15 minutes
				Sharpe -0.5~0: Reduce position to 0.8x equity
				Sharpe 0~0.7: Maintain current strategy
				Sharpe > 0.7: Stay aggressive, can full position

				# Special Strategies

				BTC strong trend following:
				- BTC 4h Change > +5%: Prioritize long strong altcoins
				- BTC 4h Change < -5%: Quick short or cash out observe

				Short-term volatility capture:
				- Price volatility >3% in short time (15min), consider reverse trade
				- Duration typically 30-60 minutes

				Remember:
				- Aggressive ≠ gambling, still need strict risk control
				- Quick in/out, don't linger
				- Control single loss, protect principal
			`,
		},
	},
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
	// Fallback configuration - automatically use free Binance API when external providers fail
	EnableBinanceFallback bool `json:"enable_binance_fallback"` // default: true - enables automatic fallback to Binance free API
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
		sb.WriteString("你会有以下数据可用：\n")
		fmt.Fprintf(sb, "- %s K线序列", kline.PrimaryTimeframe)
		if kline.EnableMultiTimeframe {
			fmt.Fprintf(sb, " + %s K线序列\n", kline.LongerTimeframe)
		} else {
			sb.WriteString("\n")
		}
		if indicators.EnableEMA {
			sb.WriteString("- EMA指标")
			if len(indicators.EMAPeriods) > 0 {
				fmt.Fprintf(sb, "（周期：%v）", indicators.EMAPeriods)
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
		sb.WriteString("You will have the following data for your disposal:\n")
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
			SourceType:            "coinpool",
			UseCoinPool:           true,
			CoinPoolLimit:         10,
			CoinPoolAPIURL:        config.GetDefaultCoinPoolAPIURL(),
			UseOITop:              false,
			OITopLimit:            20,
			OITopAPIURL:           config.GetDefaultOITopAPIURL(20, "1h"),
			EnableBinanceFallback: true, // Enable automatic fallback to Binance free API
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
			QuantDataAPIURL:    config.GetDefaultQuantDataAPIURL(),
			EnableQuantOI:      true,
			EnableQuantNetflow: true,
			// OI ranking data - market-wide OI increase/decrease rankings
			EnableOIRanking:   true,
			OIRankingAPIURL:   config.GetDefaultOIRankingBaseURL(),
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
		TradingMode: "balanced",
	}
	mode := config.TradingMode
	config.PromptSections = GetPromptSectionsByModeAndLang(mode, lang)

	return config
}

func GetPromptSectionsByModeAndLang(mode, lang string) PromptSectionsConfig {
	if m, ok := PromptTemplates[mode]; ok {
		if tmpl, ok := m[lang]; ok {
			return tmpl
		}
		// fallback to English if language not found
		if tmpl, ok := m["en"]; ok {
			return tmpl
		}
	}
	// fallback to balanced/en if nothing found
	return PromptTemplates["balanced"]["en"]
}

func (c *StrategyConfig) SetConfigPromptSectionsByModeAndLang(mode, lang string) {
	c.PromptSections = GetPromptSectionsByModeAndLang(mode, lang)
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
	err := s.db.QueryRow(`SELECT is_default FROM strategies WHERE id = ?`, id).Scan(&isDefault)
	if err != nil {
		return fmt.Errorf("failed to check if strategy is default: %w", err)
	}
	if isDefault {
		return fmt.Errorf("cannot delete system default strategy")
	}

	_, err = s.db.Exec(`DELETE FROM strategies WHERE id = ? AND user_id = ?`, id, userID)
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

	// Parse source config to ensure EnableBinanceFallback is enabled
	sourceConfig, err := (&Strategy{Config: source.Config}).ParseConfig()
	if err != nil {
		return fmt.Errorf("failed to parse source strategy config: %w", err)
	}

	// Ensure EnableBinanceFallback is enabled in duplicated strategy
	sourceConfig.CoinSource.EnableBinanceFallback = true

	// Re-serialize the updated config
	configJSON, err := json.Marshal(sourceConfig)
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	// create new strategy
	newStrategy := &Strategy{
		ID:          newID,
		UserID:      userID,
		Name:        newName,
		Description: "Created based on [" + source.Name + "]",
		IsActive:    false,
		IsDefault:   false,
		Config:      string(configJSON),
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
