package decision

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"nofx/config"
	"nofx/logger"
	"nofx/market"
	"nofx/mcp"
	"nofx/provider"
	"nofx/security"
	"nofx/store"
	"regexp"
	"strings"
	"time"
)

// ============================================================================
// Pre-compiled regular expressions (performance optimization)
// ============================================================================

var (
	// Safe regex: precisely match ```json code blocks
	reJSONFence      = regexp.MustCompile(`(?is)` + "```json\\s*(\\[\\s*\\{.*?\\}\\s*\\])\\s*```")
	reJSONArray      = regexp.MustCompile(`(?is)\[\s*\{.*?\}\s*\]`)
	reArrayHead      = regexp.MustCompile(`^\[\s*\{`)
	reArrayOpenSpace = regexp.MustCompile(`^\[\s+\{`)
	reInvisibleRunes = regexp.MustCompile("[\u200B\u200C\u200D\uFEFF]")

	// XML tag extraction (supports any characters in reasoning chain)
	reReasoningTag = regexp.MustCompile(`(?s)<reasoning>(.*?)</reasoning>`)
	reDecisionTag  = regexp.MustCompile(`(?s)<decision>(.*?)</decision>`)
)

// ============================================================================
// Type Definitions
// ============================================================================

// PositionInfo position information
type PositionInfo struct {
	Symbol           string  `json:"symbol"`
	Side             string  `json:"side"` // "long" or "short"
	EntryPrice       float64 `json:"entry_price"`
	MarkPrice        float64 `json:"mark_price"`
	Quantity         float64 `json:"quantity"`
	Leverage         int     `json:"leverage"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	UnrealizedPnLPct float64 `json:"unrealized_pnl_pct"`
	PeakPnLPct       float64 `json:"peak_pnl_pct"` // Historical peak profit percentage
	LiquidationPrice float64 `json:"liquidation_price"`
	MarginUsed       float64 `json:"margin_used"`
	UpdateTime       int64   `json:"update_time"` // Position update timestamp (milliseconds)
}

// AccountInfo account information
type AccountInfo struct {
	TotalEquity      float64 `json:"total_equity"`      // Account equity
	AvailableBalance float64 `json:"available_balance"` // Available balance
	UnrealizedPnL    float64 `json:"unrealized_pnl"`    // Unrealized profit/loss
	TotalPnL         float64 `json:"total_pnl"`         // Total profit/loss
	TotalPnLPct      float64 `json:"total_pnl_pct"`     // Total profit/loss percentage
	MarginUsed       float64 `json:"margin_used"`       // Used margin
	MarginUsedPct    float64 `json:"margin_used_pct"`   // Margin usage rate
	PositionCount    int     `json:"position_count"`    // Number of positions
}

// CandidateCoin candidate coin (from coin pool)
type CandidateCoin struct {
	Symbol  string   `json:"symbol"`
	Sources []string `json:"sources"` // Sources: "ai500" and/or "oi_top"
}

// OITopData open interest growth top data (for AI decision reference)
type OITopData struct {
	Rank              int     // OI Top ranking
	OIDeltaPercent    float64 // Open interest change percentage (1 hour)
	OIDeltaValue      float64 // Open interest change value
	PriceDeltaPercent float64 // Price change percentage
}

// TradingStats trading statistics (for AI input)
type TradingStats struct {
	TotalTrades    int     `json:"total_trades"`     // Total number of trades (closed)
	WinRate        float64 `json:"win_rate"`         // Win rate (%)
	ProfitFactor   float64 `json:"profit_factor"`    // Profit factor
	SharpeRatio    float64 `json:"sharpe_ratio"`     // Sharpe ratio
	TotalPnL       float64 `json:"total_pnl"`        // Total profit/loss
	AvgWin         float64 `json:"avg_win"`          // Average win
	AvgLoss        float64 `json:"avg_loss"`         // Average loss
	MaxDrawdownPct float64 `json:"max_drawdown_pct"` // Maximum drawdown (%)
}

// RecentOrder recently completed order (for AI input)
type RecentOrder struct {
	// Basic execution data
	Symbol       string  `json:"symbol"`        // Trading pair
	Side         string  `json:"side"`          // long/short
	EntryPrice   float64 `json:"entry_price"`   // Entry price
	ExitPrice    float64 `json:"exit_price"`    // Exit price
	RealizedPnL  float64 `json:"realized_pnl"`  // Realized profit/loss
	PnLPct       float64 `json:"pnl_pct"`       // Profit/loss percentage
	EntryTime    string  `json:"entry_time"`    // Entry time
	ExitTime     string  `json:"exit_time"`     // Exit time
	HoldDuration string  `json:"hold_duration"` // Hold duration, e.g. "2h30m"
	Leverage     int     `json:"leverage,omitempty"`

	// Market Microstructure (Entry)
	EntrySpread         float64 `json:"entry_spread,omitempty"`          // Bid-ask spread % at entry
	EntryDepth          float64 `json:"entry_depth,omitempty"`           // Available depth USD at entry
	EntryArrivalPrice   float64 `json:"entry_arrival_price,omitempty"`   // Signal price
	EntryFillPrice      float64 `json:"entry_fill_price,omitempty"`      // Actual execution price
	EntrySlippage       float64 `json:"entry_slippage,omitempty"`        // Arrival → fill slippage %
	EntrySlippageBudget float64 `json:"entry_slippage_budget,omitempty"` // Expected slippage tolerance %
	EntryFillTime       int64   `json:"entry_fill_time,omitempty"`       // Fill time (ms)
	SignalTime          int64   `json:"signal_time,omitempty"`           // Signal generation (ms)

	// Market Microstructure (Exit)
	ExitSpread   float64 `json:"exit_spread,omitempty"`   // Spread at exit
	ExitDepth    float64 `json:"exit_depth,omitempty"`    // Depth at exit
	ExitSlippage float64 `json:"exit_slippage,omitempty"` // Exit execution slippage %

	// Volatility & Risk
	ATRAtEntry           float64 `json:"atr_at_entry,omitempty"`             // ATR (14) at entry
	RealizedVolatility   float64 `json:"realized_volatility,omitempty"`      // σ during trade hold
	StopDistance         float64 `json:"stop_distance,omitempty"`            // Stop distance in %
	StopDistanceVsATR    float64 `json:"stop_distance_vs_atr,omitempty"`     // Stop / ATR ratio
	RiskPerTrade         float64 `json:"risk_per_trade,omitempty"`           // Absolute risk USD
	RiskPerTradeVsBudget float64 `json:"risk_per_trade_vs_budget,omitempty"` // Risk / account % of risk budget

	// Regime Tags
	TrendStrength    float64 `json:"trend_strength,omitempty"`    // -1 (strong down) to +1 (strong up)
	ChopScore        float64 `json:"chop_score,omitempty"`        // 0 (trending) to 1 (choppy)
	VolatilityRegime string  `json:"volatility_regime,omitempty"` // "low", "normal", "high"
	MarketRegime     string  `json:"market_regime,omitempty"`     // "trending", "sideways", "volatile"

	// Flow & Participation
	VolumeAtEntry          float64 `json:"volume_at_entry,omitempty"`           // Volume vs 24h baseline %
	OIDeltaAtEntry         float64 `json:"oi_delta_at_entry,omitempty"`         // OI 1h change %
	VolumeDeltaDuringTrade float64 `json:"volume_delta_during_trade,omitempty"` // Volume fade during hold
	OIDeltaDuringTrade     float64 `json:"oi_delta_during_trade,omitempty"`     // OI change during hold

	// Correlation & Risk Book
	CorrelationToBTC     float64 `json:"correlation_to_btc,omitempty"`    // Position correlation to BTC
	PortfolioCorrelation float64 `json:"portfolio_correlation,omitempty"` // Correlation to current book
	TimeOfDay            int     `json:"time_of_day,omitempty"`           // 0-23 hour UTC
	EventProximity       string  `json:"event_proximity,omitempty"`       // "pre_event", "post_event", "none"

	// Excursion Metrics
	MaxFavorableExcursion float64 `json:"max_favorable_excursion,omitempty"` // Best % profit during trade
	MaxAdverseExcursion   float64 `json:"max_adverse_excursion,omitempty"`   // Worst % loss during trade
	GiveBackFromPeak      float64 `json:"giveback_from_peak,omitempty"`      // % give-back from MFE

	// Carry & Funding
	FundingAccrued    float64 `json:"funding_accrued,omitempty"`     // Cumulative funding cost
	BorrowCostAccrued float64 `json:"borrow_cost_accrued,omitempty"` // Cumulative borrow cost

	// Execution Quality
	FillQuality    float64 `json:"fill_quality,omitempty"`     // 0-1 score (1=perfect)
	SlippageVsVWAP float64 `json:"slippage_vs_vwap,omitempty"` // Entry fill vs VWAP %
	OrderReject    bool    `json:"order_reject,omitempty"`     // Was order rejected?
	PartialFill    bool    `json:"partial_fill,omitempty"`     // Was fill partial?
}

// Context trading context (complete information passed to AI)
type Context struct {
	CurrentTime            string                                  `json:"current_time"`
	RuntimeMinutes         int                                     `json:"runtime_minutes"`
	CallCount              int                                     `json:"call_count"`
	Account                AccountInfo                             `json:"account"`
	Positions              []PositionInfo                          `json:"positions"`
	CandidateCoins         []CandidateCoin                         `json:"candidate_coins"`
	PromptVariant          string                                  `json:"prompt_variant,omitempty"` // Evolved prompt variant ID
	TradingStats           *TradingStats                           `json:"trading_stats,omitempty"`
	RecentOrders           []RecentOrder                           `json:"recent_orders,omitempty"`
	PerformanceFeedback    string                                  `json:"-"` // *backtest.FeedbackAnalysis - avoiding circular dependency
	OptimizedWeights       interface{}                             `json:"-"` // *store.RiskControlConfig - optimized parameters
	ComplianceFeedback     string                                  `json:"-"` // Reinforcement learning: compliance with recommendations
	PromptEvolutionSummary string                                  `json:"-"` // Prompt optimizer evolution history and learnings
	EvolvedRoleDefinition  string                                  `json:"-"` // Evolved role definition based on performance
	CalibratedThresholds   string                                  `json:"-"` // Learned thresholds for failure detection
	MarketDataMap          map[string]*market.Data                 `json:"-"`
	MultiTFMarket          map[string]map[string]*market.Data      `json:"-"`
	OITopDataMap           map[string]*OITopData                   `json:"-"`
	QuantDataMap           map[string]*QuantData                   `json:"-"`
	OIRankingData          *provider.OIRankingData                 `json:"-"` // Market-wide OI ranking data
	MicrostructureDataMap  map[string]*market.MarketMicrostructure `json:"-"` // Market microstructure data per symbol
	BTCETHLeverage         int                                     `json:"-"`
	AltcoinLeverage        int                                     `json:"-"`
	Timeframes             []string                                `json:"-"`
}

// Decision AI trading decision
type Decision struct {
	Symbol string `json:"symbol"`
	Action string `json:"action"` // "open_long", "open_short", "close_long", "close_short", "hold", "wait"

	// Opening position parameters
	Leverage        int     `json:"leverage,omitempty"`
	PositionSizeUSD float64 `json:"position_size_usd,omitempty"`
	StopLoss        float64 `json:"stop_loss,omitempty"`
	TakeProfit      float64 `json:"take_profit,omitempty"`

	// Common parameters
	Confidence int     `json:"confidence,omitempty"` // Confidence level (0-100)
	RiskUSD    float64 `json:"risk_usd,omitempty"`   // Maximum USD risk
	Reasoning  string  `json:"reasoning"`
}

// FullDecision AI's complete decision (including chain of thought)
type FullDecision struct {
	SystemPrompt        string     `json:"system_prompt"`
	UserPrompt          string     `json:"user_prompt"`
	CoTTrace            string     `json:"cot_trace"`
	Decisions           []Decision `json:"decisions"`
	RawResponse         string     `json:"raw_response"`
	Timestamp           time.Time  `json:"timestamp"`
	AIRequestDurationMs int64      `json:"ai_request_duration_ms,omitempty"`
}

// QuantData quantitative data structure (fund flow, position changes, price changes)
type QuantData struct {
	Symbol      string             `json:"symbol"`
	Price       float64            `json:"price"`
	Netflow     *NetflowData       `json:"netflow,omitempty"`
	OI          map[string]*OIData `json:"oi,omitempty"`
	PriceChange map[string]float64 `json:"price_change,omitempty"`
}

type NetflowData struct {
	Institution *FlowTypeData `json:"institution,omitempty"`
	Personal    *FlowTypeData `json:"personal,omitempty"`
}

type FlowTypeData struct {
	Future map[string]float64 `json:"future,omitempty"`
	Spot   map[string]float64 `json:"spot,omitempty"`
}

type OIData struct {
	CurrentOI float64                 `json:"current_oi"`
	Delta     map[string]*OIDeltaData `json:"delta,omitempty"`
}

type OIDeltaData struct {
	OIDelta        float64 `json:"oi_delta"`
	OIDeltaValue   float64 `json:"oi_delta_value"`
	OIDeltaPercent float64 `json:"oi_delta_percent"`
}

// ============================================================================
// StrategyEngine - Core Strategy Execution Engine
// ============================================================================

// StrategyEngine strategy execution engine
type StrategyEngine struct {
	config *store.StrategyConfig
}

// NewStrategyEngine creates strategy execution engine
func NewStrategyEngine(config *store.StrategyConfig) *StrategyEngine {
	return &StrategyEngine{config: config}
}

// GetRiskControlConfig gets risk control configuration
func (e *StrategyEngine) GetRiskControlConfig() store.RiskControlConfig {
	return e.config.RiskControl
}

// GetConfig gets complete strategy configuration
func (e *StrategyEngine) GetConfig() *store.StrategyConfig {
	return e.config
}

// SetStrategyPrompt updates the strategy prompt sections in the strategy configuration
func (e *StrategyEngine) SetStrategyPrompt(variant *store.PromptVariantData) {
	e.config.PromptSections.RoleDefinition = variant.PromptRoleDefinition
	e.config.PromptSections.TradingFrequency = variant.PromptTradingFrequency
	e.config.PromptSections.EntryStandards = variant.PromptEntryStandards
	e.config.PromptSections.DecisionProcess = variant.PromptDecisionProcess
	logger.Infof("✅ StrategyEngine updated strategy prompts for variant: %s", variant.VariantID)
}

// SetCustomPrompt updates the custom prompt in the strategy configuration
// Used by prompt optimizer to apply evolved prompts to future decisions
func (e *StrategyEngine) SetCustomPrompt(customPrompt string) {
	e.config.CustomPrompt = customPrompt
}

// ============================================================================
// Entry Functions - Main API
// ============================================================================

// GetFullDecision gets AI's complete trading decision (batch analysis of all coins and positions)
// Uses default strategy configuration - for production use GetFullDecisionWithStrategy with explicit config
func GetFullDecision(ctx *Context, mcpClient mcp.AIClient) (*FullDecision, error) {
	defaultConfig := store.GetDefaultStrategyConfig("en")
	engine := NewStrategyEngine(&defaultConfig)
	return GetFullDecisionWithStrategy(ctx, mcpClient, engine)
}

// GetFullDecisionWithStrategy uses StrategyEngine to get AI decision (unified prompt generation)
func GetFullDecisionWithStrategy(ctx *Context, mcpClient mcp.AIClient, engine *StrategyEngine) (*FullDecision, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is nil")
	}
	if engine == nil {
		defaultConfig := store.GetDefaultStrategyConfig("en")
		engine = NewStrategyEngine(&defaultConfig)
	}

	// 1. Fetch market data using strategy config
	if len(ctx.MarketDataMap) == 0 {
		if err := fetchMarketDataWithStrategy(ctx, engine); err != nil {
			return nil, fmt.Errorf("failed to fetch market data: %w", err)
		}
	}

	// Ensure OITopDataMap is initialized
	if ctx.OITopDataMap == nil {
		ctx.OITopDataMap = make(map[string]*OITopData)
		oiPositions, err := provider.GetOITopPositions()
		if err == nil {
			for _, pos := range oiPositions {
				ctx.OITopDataMap[pos.Symbol] = &OITopData{
					Rank:              pos.Rank,
					OIDeltaPercent:    pos.OIDeltaPercent,
					OIDeltaValue:      pos.OIDeltaValue,
					PriceDeltaPercent: pos.PriceDeltaPercent,
				}
			}
		}
	}

	// 2. Build System Prompt using strategy engine
	riskConfig := engine.GetRiskControlConfig()
	systemPrompt := engine.BuildSystemPromptWithContext(ctx.Account.TotalEquity, ctx)

	// 3. Build User Prompt using strategy engine
	userPrompt := engine.BuildUserPrompt(ctx)

	// 4. Call AI API
	aiCallStart := time.Now()
	aiResponse, err := mcpClient.CallWithMessages(systemPrompt, userPrompt)
	aiCallDuration := time.Since(aiCallStart)
	if err != nil {
		return nil, fmt.Errorf("AI API call failed: %w", err)
	}

	// 5. Parse AI response
	decision, err := parseFullDecisionResponse(
		aiResponse,
		ctx.Account.TotalEquity,
		riskConfig.BTCETHMaxLeverage,
		riskConfig.AltcoinMaxLeverage,
		riskConfig.BTCETHMaxPositionValueRatio,
		riskConfig.AltcoinMaxPositionValueRatio,
	)

	if decision != nil {
		decision.Timestamp = time.Now()
		decision.SystemPrompt = systemPrompt
		decision.UserPrompt = userPrompt
		decision.AIRequestDurationMs = aiCallDuration.Milliseconds()
		decision.RawResponse = aiResponse
	}

	if err != nil {
		return decision, fmt.Errorf("failed to parse AI response: %w", err)
	}

	return decision, nil
}

// ============================================================================
// Market Data Fetching
// ============================================================================

// fetchMarketDataWithStrategy fetches market data using strategy config (multiple timeframes)
func fetchMarketDataWithStrategy(ctx *Context, engine *StrategyEngine) error {
	config := engine.GetConfig()
	ctx.MarketDataMap = make(map[string]*market.Data)
	ctx.MicrostructureDataMap = make(map[string]*market.MarketMicrostructure)

	timeframes := config.Indicators.Klines.SelectedTimeframes
	primaryTimeframe := config.Indicators.Klines.PrimaryTimeframe
	klineCount := config.Indicators.Klines.PrimaryCount

	// Compatible with old configuration
	if len(timeframes) == 0 {
		if primaryTimeframe != "" {
			timeframes = append(timeframes, primaryTimeframe)
		} else {
			timeframes = append(timeframes, "3m")
		}
		if config.Indicators.Klines.LongerTimeframe != "" {
			timeframes = append(timeframes, config.Indicators.Klines.LongerTimeframe)
		}
	}
	if primaryTimeframe == "" {
		primaryTimeframe = timeframes[0]
	}
	if klineCount <= 0 {
		klineCount = 30
	}

	logger.Infof("📊 Strategy timeframes: %v, Primary: %s, Kline count: %d", timeframes, primaryTimeframe, klineCount)

	// Store timeframes in context for formatter to use
	ctx.Timeframes = timeframes

	// 1. First fetch data for position coins (must fetch)
	for _, pos := range ctx.Positions {
		data, err := market.GetWithTimeframes(pos.Symbol, timeframes, primaryTimeframe, klineCount)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch market data for position %s: %v", pos.Symbol, err)
			continue
		}
		ctx.MarketDataMap[pos.Symbol] = data

		// Fetch microstructure data for positions
		microstructure := engine.FetchMicrostructureData(pos.Symbol)
		if microstructure != nil {
			ctx.MicrostructureDataMap[pos.Symbol] = microstructure
		}
	}

	// 2. Fetch data for all candidate coins
	positionSymbols := make(map[string]bool)
	for _, pos := range ctx.Positions {
		positionSymbols[pos.Symbol] = true
	}

	const minOIThresholdMillions = 15.0 // 15M USD minimum open interest value

	for _, coin := range ctx.CandidateCoins {
		if _, exists := ctx.MarketDataMap[coin.Symbol]; exists {
			continue
		}

		data, err := market.GetWithTimeframes(coin.Symbol, timeframes, primaryTimeframe, klineCount)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch market data for %s: %v", coin.Symbol, err)
			continue
		}

		// Liquidity filter (skip for xyz dex assets - they don't have OI data from Binance)
		isExistingPosition := positionSymbols[coin.Symbol]
		isXyzAsset := market.IsXyzDexAsset(coin.Symbol)
		if !isExistingPosition && !isXyzAsset && data.OpenInterest != nil && data.CurrentPrice > 0 {
			oiValue := data.OpenInterest.Latest * data.CurrentPrice
			oiValueInMillions := oiValue / 1_000_000
			if oiValueInMillions < minOIThresholdMillions {
				logger.Infof("⚠️  %s OI value too low (%.2fM USD < %.1fM), skipping coin",
					coin.Symbol, oiValueInMillions, minOIThresholdMillions)
				continue
			}
		}

		ctx.MarketDataMap[coin.Symbol] = data

		// Fetch microstructure data for candidate coins
		microstructure := engine.FetchMicrostructureData(coin.Symbol)
		if microstructure != nil {
			ctx.MicrostructureDataMap[coin.Symbol] = microstructure
		}
	}

	logger.Infof("📊 Successfully fetched multi-timeframe market data for %d coins", len(ctx.MarketDataMap))
	logger.Infof("📊 Fetched microstructure data for %d coins", len(ctx.MicrostructureDataMap))
	return nil
}

// ============================================================================
// Candidate Coins
// ============================================================================

// GetCandidateCoins gets candidate coins based on strategy configuration
func (e *StrategyEngine) GetCandidateCoins() ([]CandidateCoin, error) {
	var candidates []CandidateCoin
	symbolSources := make(map[string][]string)

	coinSource := e.config.CoinSource

	if coinSource.CoinPoolAPIURL != "" {
		provider.SetCoinPoolAPI(coinSource.CoinPoolAPIURL)
	}
	if coinSource.OITopAPIURL != "" {
		provider.SetOITopAPI(coinSource.OITopAPIURL)
	}

	switch coinSource.SourceType {
	case "static":
		for _, symbol := range coinSource.StaticCoins {
			symbol = market.Normalize(symbol)
			candidates = append(candidates, CandidateCoin{
				Symbol:  symbol,
				Sources: []string{"static"},
			})
		}
		return candidates, nil

	case "coinpool":
		// 检查 use_coin_pool 标志，如果为 false 则回退到静态币种
		if !coinSource.UseCoinPool {
			logger.Infof("⚠️  source_type is 'coinpool' but use_coin_pool is false, falling back to static coins")
			for _, symbol := range coinSource.StaticCoins {
				symbol = market.Normalize(symbol)
				candidates = append(candidates, CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"static"},
				})
			}
			return candidates, nil
		}
		return e.getCoinPoolCoins(coinSource.CoinPoolLimit)

	case "oi_top":
		// 检查 use_oi_top 标志，如果为 false 则回退到静态币种
		if !coinSource.UseOITop {
			logger.Infof("⚠️  source_type is 'oi_top' but use_oi_top is false, falling back to static coins")
			for _, symbol := range coinSource.StaticCoins {
				symbol = market.Normalize(symbol)
				candidates = append(candidates, CandidateCoin{
					Symbol:  symbol,
					Sources: []string{"static"},
				})
			}
			return candidates, nil
		}
		return e.getOITopCoins(coinSource.OITopLimit)

	case "mixed":
		if coinSource.UseCoinPool {
			poolCoins, err := e.getCoinPoolCoins(coinSource.CoinPoolLimit)
			if err != nil {
				logger.Infof("⚠️  Failed to get AI500 coin pool: %v", err)
			} else {
				for _, coin := range poolCoins {
					symbolSources[coin.Symbol] = append(symbolSources[coin.Symbol], "ai500")
				}
			}
		}

		if coinSource.UseOITop {
			oiCoins, err := e.getOITopCoins(coinSource.OITopLimit)
			if err != nil {
				logger.Infof("⚠️  Failed to get OI Top: %v", err)
			} else {
				for _, coin := range oiCoins {
					symbolSources[coin.Symbol] = append(symbolSources[coin.Symbol], "oi_top")
				}
			}
		}

		for _, symbol := range coinSource.StaticCoins {
			symbol = market.Normalize(symbol)
			if _, exists := symbolSources[symbol]; !exists {
				symbolSources[symbol] = []string{"static"}
			} else {
				symbolSources[symbol] = append(symbolSources[symbol], "static")
			}
		}

		for symbol, sources := range symbolSources {
			candidates = append(candidates, CandidateCoin{
				Symbol:  symbol,
				Sources: sources,
			})
		}
		return candidates, nil

	default:
		return nil, fmt.Errorf("unknown coin source type: %s", coinSource.SourceType)
	}
}

func (e *StrategyEngine) getCoinPoolCoins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 30
	}

	// Check if Binance fallback is enabled (default: true)
	useFallback := true
	if !e.config.CoinSource.EnableBinanceFallback {
		useFallback = false
	}

	// Use fallback system: external API → Binance volume ranking
	symbols, source, err := provider.GetTopCoinsWithFallback(limit, useFallback)
	if err != nil {
		return nil, err
	}

	var candidates []CandidateCoin
	for _, symbol := range symbols {
		sourceLabel := "ai500"
		if source == "binance_volume" {
			sourceLabel = "binance_volume"
		}
		candidates = append(candidates, CandidateCoin{
			Symbol:  symbol,
			Sources: []string{sourceLabel},
		})
	}

	logger.Infof("✓ Got %d coin pool candidates from source: %s", len(candidates), source)
	return candidates, nil
}

func (e *StrategyEngine) getOITopCoins(limit int) ([]CandidateCoin, error) {
	if limit <= 0 {
		limit = 20
	}

	// Check if Binance fallback is enabled (default: true)
	useFallback := true
	if !e.config.CoinSource.EnableBinanceFallback {
		useFallback = false
	}

	// Use fallback system: external OI API → Binance momentum ranking
	symbols, source, err := provider.GetOITopSymbolsWithFallback(limit, useFallback)
	if err != nil {
		return nil, err
	}

	var candidates []CandidateCoin
	for _, symbol := range symbols {
		sourceLabel := "oi_top"
		if source == "binance_momentum" {
			sourceLabel = "binance_momentum"
		}
		candidates = append(candidates, CandidateCoin{
			Symbol:  symbol,
			Sources: []string{sourceLabel},
		})
	}

	logger.Infof("✓ Got %d OI top candidates from source: %s", len(candidates), source)
	return candidates, nil
}

// ============================================================================
// External & Quant Data
// ============================================================================

// FetchMarketData fetches market data based on strategy configuration
func (e *StrategyEngine) FetchMarketData(symbol string) (*market.Data, error) {
	return market.Get(symbol)
}

// FetchMicrostructureData fetches market microstructure data (order book analysis)
func (e *StrategyEngine) FetchMicrostructureData(symbol string) *market.MarketMicrostructure {
	analyzer := market.NewMarketMicrostructureAnalyzer()

	// Fetch order book depth
	depth, err := analyzer.FetchOrderBookDepth(symbol, config.OrderBookDepth)
	if err != nil {
		logger.Infof("⚠️  Failed to fetch order book depth for %s: %v", symbol, err)
		return nil
	}

	// Get current price from market data
	marketData, err := e.FetchMarketData(symbol)
	if err != nil {
		logger.Infof("⚠️  Failed to fetch market data for %s: %v", symbol, err)
		return nil
	}

	// Get K-lines for VWAP calculation (get last 100 candles, 1h timeframe)
	// apiClient := &market.APIClient{}
	apiClient := market.NewAPIClient()
	klines, err := apiClient.GetKlines(symbol, "1h", 100)
	if err != nil {
		logger.Infof("⚠️  Failed to fetch K-lines for %s: %v", symbol, err)
		// Continue anyway - VWAP is optional
		klines = nil
	}

	// Analyze market microstructure
	microstructure, err := analyzer.AnalyzeMarketMicrostructure(symbol, depth, marketData.CurrentPrice, klines)
	if err != nil {
		logger.Infof("⚠️  Failed to analyze microstructure for %s: %v", symbol, err)
		return nil
	}

	return microstructure
}

// FetchExternalData fetches external data sources
func (e *StrategyEngine) FetchExternalData() (map[string]interface{}, error) {
	externalData := make(map[string]interface{})

	for _, source := range e.config.Indicators.ExternalDataSources {
		data, err := e.fetchSingleExternalSource(source)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch external data source [%s]: %v", source.Name, err)
			continue
		}
		externalData[source.Name] = data
	}

	return externalData, nil
}

func (e *StrategyEngine) fetchSingleExternalSource(source store.ExternalDataSource) (interface{}, error) {
	// SSRF Protection: Validate URL before making request
	if err := security.ValidateURL(source.URL); err != nil {
		return nil, fmt.Errorf("external source URL validation failed: %w", err)
	}

	timeout := time.Duration(source.RefreshSecs) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Use SSRF-safe HTTP client
	client := security.SafeHTTPClient(timeout)

	req, err := http.NewRequest(source.Method, source.URL, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range source.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	if source.DataPath != "" {
		result = extractJSONPath(result, source.DataPath)
	}

	return result, nil
}

func extractJSONPath(data interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return nil
		}
	}

	return current
}

// FetchQuantData fetches quantitative data for a single coin
func (e *StrategyEngine) FetchQuantData(symbol string) (*QuantData, error) {
	if !e.config.Indicators.EnableQuantData {
		return nil, nil
	}

	// Try external API first if configured
	if e.config.Indicators.QuantDataAPIURL != "" {
		apiURL := e.config.Indicators.QuantDataAPIURL
		url := strings.ReplaceAll(apiURL, "{symbol}", symbol)

		// SSRF Protection: Validate URL before making request
		resp, err := security.SafeGet(url, 10*time.Second)
		if err == nil {
			defer func() {
				_ = resp.Body.Close()
			}()

			if resp.StatusCode == http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err == nil {
					// Try new format first (https://nofxos.ai uses "success": true)
					var newApiResp struct {
						Success bool       `json:"success"`
						Data    *QuantData `json:"data"`
					}
					if err := json.Unmarshal(body, &newApiResp); err == nil && newApiResp.Success && newApiResp.Data != nil {
						// Success - return external API data (new format)
						return newApiResp.Data, nil
					}
				}
			}
		}
		// External API failed, log and try fallback
		logger.Infof("⚠️  External quant API failed for %s, trying DIY fallback", symbol)
	}

	// Fallback to DIY calculation if enabled
	if e.config.CoinSource.EnableBinanceFallback {
		diyData, err := provider.GetDIYQuantData(symbol)
		if err != nil {
			return nil, fmt.Errorf("DIY fallback failed: %w", err)
		}

		// Convert DIY data to QuantData format
		quantData := convertDIYToQuantData(diyData)
		logger.Infof("✓ Using DIY quant data for %s (free fallback)", symbol)
		return quantData, nil
	}

	return nil, fmt.Errorf("no quant data source available")
}

// FetchQuantDataBatch batch fetches quantitative data
func (e *StrategyEngine) FetchQuantDataBatch(symbols []string) map[string]*QuantData {
	result := make(map[string]*QuantData)

	if !e.config.Indicators.EnableQuantData || e.config.Indicators.QuantDataAPIURL == "" {
		return result
	}

	for _, symbol := range symbols {
		data, err := e.FetchQuantData(symbol)
		if err != nil {
			logger.Infof("⚠️  Failed to fetch quantitative data for %s: %v", symbol, err)
			continue
		}
		if data != nil {
			result[symbol] = data
		}
	}

	return result
}

// FetchOIRankingData fetches market-wide OI ranking data
func (e *StrategyEngine) FetchOIRankingData() *provider.OIRankingData {
	indicators := e.config.Indicators
	if !indicators.EnableOIRanking {
		return nil
	}

	baseURL := indicators.OIRankingAPIURL
	if baseURL == "" {
		baseURL = config.DefaultBaseURL
	}

	// Get auth key from existing API URL or use default
	authKey := "cm_568c67eae410d912c54c"
	if indicators.QuantDataAPIURL != "" {
		if idx := strings.Index(indicators.QuantDataAPIURL, "auth="); idx != -1 {
			authKey = indicators.QuantDataAPIURL[idx+5:]
			if ampIdx := strings.Index(authKey, "&"); ampIdx != -1 {
				authKey = authKey[:ampIdx]
			}
		}
	}

	duration := indicators.OIRankingDuration
	if duration == "" {
		duration = "1h"
	}

	limit := indicators.OIRankingLimit
	if limit <= 0 {
		limit = 10
	}

	logger.Infof("📊 Fetching OI ranking data (duration: %s, limit: %d)", duration, limit)

	data, err := provider.GetOIRankingData(baseURL, authKey, duration, limit)
	if err != nil {
		logger.Warnf("⚠️  Failed to fetch OI ranking data: %v", err)

		// Fallback tier 2: CoinGlass (requires COINGLASS_API_KEY to be set)
		if e.config.CoinSource.EnableBinanceFallback {
			fallbackData, fallbackErr := provider.GetOIRankingFromCoinGlass(duration, limit)
			if fallbackErr == nil {
				logger.Infof("✓ Using CoinGlass OI ranking fallback (%d positions)", len(fallbackData.TopPositions))
				return fallbackData
			}
			logger.Warnf("⚠️  CoinGlass fallback failed: %v, trying Binance momentum fallback", fallbackErr)

			// Fallback tier 3: Binance momentum-based ranking (always available)
			symbolsData, _, binanceErr := provider.GetOITopSymbolsWithFallback(limit, false)
			if binanceErr == nil && len(symbolsData) > 0 {
				// Convert symbols to OIRankingData format
				topCount := limit
				if topCount > len(symbolsData) {
					topCount = len(symbolsData)
				}

				oiPositions := make([]provider.OIPosition, 0, topCount)
				for i := 0; i < topCount && i < len(symbolsData); i++ {
					oiPositions = append(oiPositions, provider.OIPosition{
						Symbol: symbolsData[i],
						Rank:   i + 1,
					})
				}

				binanceData := &provider.OIRankingData{
					Duration:     duration,
					TopPositions: oiPositions,
					LowPositions: []provider.OIPosition{}, // Empty low positions for Binance fallback
					FetchedAt:    time.Now(),
				}
				logger.Infof("✓ Using Binance momentum OI ranking fallback (%d top positions)", len(binanceData.TopPositions))
				return binanceData
			}
			if binanceErr != nil {
				logger.Warnf("⚠️  Binance fallback also failed: %v", binanceErr)
			}
		}

		return nil
	}

	logger.Infof("✓ OI ranking data ready: %d top, %d low positions",
		len(data.TopPositions), len(data.LowPositions))

	return data
}

// ============================================================================
// Prompt Building - System Prompt
// ============================================================================

// BuildSystemPromptWithContext builds System Prompt with optional context for evolved role
func (e *StrategyEngine) BuildSystemPromptWithContext(accountEquity float64, ctx *Context) string {
	var sb strings.Builder
	riskControl := e.config.RiskControl
	promptSections := e.config.PromptSections
	lang := detectLanguage(promptSections.RoleDefinition)
	defaultConfig := store.GetDefaultStrategyConfig(string(lang))

	// 1. Role definition (editable) - USE EVOLVED ROLE IF AVAILABLE
	roleToUse := promptSections.RoleDefinition
	if ctx != nil && ctx.EvolvedRoleDefinition != "" {
		roleToUse = ctx.EvolvedRoleDefinition
	}

	if roleToUse != "" {
		sb.WriteString(roleToUse)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString(defaultConfig.PromptSections.RoleDefinition)
		sb.WriteString("\n\n")
	}

	// 2. Trading frequency (editable)
	if promptSections.TradingFrequency != "" {
		sb.WriteString(promptSections.TradingFrequency)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString(defaultConfig.PromptSections.TradingFrequency)
		sb.WriteString("\n\n")
	}

	// 3. Entry standards (editable)
	if promptSections.EntryStandards != "" {
		sb.WriteString(promptSections.EntryStandards)
	} else {
		sb.WriteString(defaultConfig.PromptSections.EntryStandards)
	}

	// 4. Decision process (editable)
	if promptSections.DecisionProcess != "" {
		sb.WriteString(promptSections.DecisionProcess)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString(defaultConfig.PromptSections.DecisionProcess)
		sb.WriteString("\n\n")
	}

	// 5. Available indicators
	e.config.AvailableIndicatorsString(&sb, string(lang))

	// 6. Hard constraints (risk control)
	btcEthPosValueRatio := riskControl.BTCETHMaxPositionValueRatio
	if btcEthPosValueRatio <= 0 {
		btcEthPosValueRatio = config.DefaultBTCETHPosRatio
	}
	altcoinPosValueRatio := riskControl.AltcoinMaxPositionValueRatio
	if altcoinPosValueRatio <= 0 {
		altcoinPosValueRatio = config.DefaultAltcoinPosRatio
	}

	sb.WriteString("# Hard Constraints (Risk Control)\n\n")
	sb.WriteString("## CODE ENFORCED (Backend validation, cannot be bypassed):\n")
	sb.WriteString(fmt.Sprintf("- Max Positions: %d coins simultaneously\n", riskControl.MaxPositions))
	sb.WriteString(fmt.Sprintf("- Position Value Limit (Altcoins): max %.0f USDT (= equity %.0f × %.1fx)\n",
		accountEquity*altcoinPosValueRatio, accountEquity, altcoinPosValueRatio))
	sb.WriteString(fmt.Sprintf("- Position Value Limit (BTC/ETH): max %.0f USDT (= equity %.0f × %.1fx)\n",
		accountEquity*btcEthPosValueRatio, accountEquity, btcEthPosValueRatio))
	sb.WriteString(fmt.Sprintf("- Max Margin Usage: ≤%.0f%%\n", riskControl.MaxMarginUsage*100))
	sb.WriteString(fmt.Sprintf("- Min Position Size: ≥%.0f USDT\n\n", riskControl.MinPositionSize))

	sb.WriteString("## AI GUIDED (Recommended, you should follow):\n")
	sb.WriteString(fmt.Sprintf("- Trading Leverage: Altcoins max %dx | BTC/ETH max %dx\n",
		riskControl.AltcoinMaxLeverage, riskControl.BTCETHMaxLeverage))
	sb.WriteString(fmt.Sprintf("- Risk-Reward Ratio: ≥1:%.1f (take_profit / stop_loss)\n", riskControl.MinRiskRewardRatio))
	sb.WriteString(fmt.Sprintf("- Min Confidence: ≥%d to open position\n\n", riskControl.MinConfidence))

	// Position sizing guidance
	sb.WriteString("## Position Sizing Guidance\n")
	sb.WriteString("Calculate `position_size_usd` based on your confidence and the Position Value Limits above:\n")
	sb.WriteString(fmt.Sprintf("- High confidence (≥%d): Use %d-%d%%%% of max position value limit\n", config.ConfidenceHigh, config.ConfidenceHighMin, config.ConfidenceHighMax))
	sb.WriteString(fmt.Sprintf("- Medium confidence (%d-%d): Use %d-%d%%%% of max position value limit\n", config.ConfidenceMediumMin, config.ConfidenceMediumMax, config.ConfidenceMediumPosMin, config.ConfidenceMediumPosMax))
	sb.WriteString(fmt.Sprintf("- Low confidence (%d-%d): Use %d-%d%%%% of max position value limit\n", config.ConfidenceLow, config.ConfidenceLowMax, config.ConfidenceLowPosMin, config.ConfidenceLowPosMax))
	sb.WriteString(fmt.Sprintf("- Example: With equity %.0f and BTC/ETH ratio %.1fx, max is %.0f USDT\n",
		accountEquity, btcEthPosValueRatio, accountEquity*btcEthPosValueRatio))
	sb.WriteString("- **DO NOT** just use available_balance as position_size_usd. Use the Position Value Limits!\n\n")

	// 7. Schema prompt (Explain the fields defined in the schema)
	schemaPrompt := GetSchemaPrompt(lang)
	sb.WriteString(schemaPrompt)
	sb.WriteString("\n\n")

	// 7. Output format
	sb.WriteString("# Additional Output Format (Strictly Follow)\n\n")
	sb.WriteString("**Must use XML tags <reasoning> and <decision> to separate chain of thought and decision JSON, " +
		"Do not use the ~ (tilde) symbol or any range/approximate notation in your JSON output. All numbers must be precise values. " +
		"avoiding parsing errors**\n\n")
	sb.WriteString("## Format Requirements\n\n")
	sb.WriteString("<reasoning>\n")
	sb.WriteString("Your chain of thought analysis...\n")
	sb.WriteString("- Briefly analyze your thinking process \n")
	sb.WriteString("</reasoning>\n\n")
	sb.WriteString("<decision>\n")
	sb.WriteString("Step 2: JSON decision array\n\n")
	sb.WriteString("```json\n[\n")
	// Use the actual configured position value ratio for BTC/ETH in the example
	examplePositionSize := accountEquity * btcEthPosValueRatio
	sb.WriteString(fmt.Sprintf("  {\"symbol\": \"BTCUSDT\", \"action\": \"open_short\", \"leverage\": %d, \"position_size_usd\": %.0f, \"stop_loss\": 97000, \"take_profit\": 91000, \"confidence\": 85, \"risk_usd\": 300},\n",
		riskControl.BTCETHMaxLeverage, examplePositionSize))
	sb.WriteString("  {\"symbol\": \"ETHUSDT\", \"action\": \"close_long\"}\n")
	sb.WriteString("]\n```\n")
	sb.WriteString("</decision>\n\n")
	sb.WriteString("## Field Description\n\n")
	sb.WriteString("- `action`: open_long | open_short | close_long | close_short | hold | wait\n")
	sb.WriteString(fmt.Sprintf("- `confidence`: 0-100 (opening recommended ≥ %d)\n", riskControl.MinConfidence))
	sb.WriteString("- Required when opening: leverage, position_size_usd, stop_loss, take_profit, confidence, risk_usd\n")
	sb.WriteString("- **IMPORTANT**: All numeric values must be calculated numbers, NOT formulas/expressions (e.g., use `27.76` not `3000 * 0.01`)\n\n")

	// 8. Custom Prompt
	if e.config.CustomPrompt != "" {
		sb.WriteString("# 📌 Personalized Trading Strategy\n\n")
		sb.WriteString(e.config.CustomPrompt)
		sb.WriteString("\n\n")
		sb.WriteString("Note: The above personalized strategy is a supplement to the basic rules and cannot violate the basic risk control principles.\n")
	}

	return sb.String()
}

// ============================================================================
// Prompt Building - User Prompt
// ============================================================================

// BuildUserPrompt builds User Prompt based on strategy configuration
func (e *StrategyEngine) BuildUserPrompt(ctx *Context) string {
	var sb strings.Builder
	promptSections := e.config.PromptSections
	// 0. Detect language from role definition
	lang := detectLanguage(promptSections.RoleDefinition)

	// 1. 账户信息
	if lang == LangChinese {
		sb.WriteString(formatAccountZH(ctx))
	} else {
		sb.WriteString(formatAccountEN(ctx))
	}
	// 2. 性能反馈
	if ctx.PerformanceFeedback != "" {
		sb.WriteString(ctx.PerformanceFeedback)
		sb.WriteString("\n")
	}
	// 3. 合规反馈（强化学习：展示LLM遵循建议的程度）
	if ctx.ComplianceFeedback != "" {
		sb.WriteString(ctx.ComplianceFeedback)
		sb.WriteString("\n")
	}
	// 4. 优化后的权重参数
	if ctx.OptimizedWeights != nil {
		if lang == LangChinese {
			sb.WriteString(formatOptimizedWeightsZH(ctx.OptimizedWeights))
		} else {
			sb.WriteString(formatOptimizedWeightsEN(ctx.OptimizedWeights))
		}
	}
	// 5. 校准阈值（强化学习：展示LLM调整的关键阈值）
	if ctx.CalibratedThresholds != "" {
		sb.WriteString(ctx.CalibratedThresholds)
		sb.WriteString("\n")
	}
	// 6. 当前持仓
	if len(ctx.Positions) > 0 {
		if lang == LangChinese {
			sb.WriteString(formatCurrentPositionsZH(e.config, ctx))
		} else {
			sb.WriteString(formatCurrentPositionsEN(e.config, ctx))
		}
	}
	// 7. 最近交易记录
	if len(ctx.RecentOrders) > 0 {
		if lang == LangChinese {
			sb.WriteString(formatRecentTradesZH(ctx.RecentOrders))
		} else {
			sb.WriteString(formatRecentTradesEN(ctx.RecentOrders))
		}
	}
	// 8. 历史交易统计（强化学习：基于最近交易表现的总结）
	if ctx.TradingStats != nil && len(ctx.RecentOrders) > 0 {
		var wins, losses []interface{}
		for _, order := range ctx.RecentOrders {
			if order.RealizedPnL >= 0 {
				wins = append(wins, order)
			} else {
				losses = append(losses, order)
			}
		}
		var metaPrompt string
		if lang == LangChinese {
			metaPrompt = buildMetaPromptZH(*ctx.TradingStats, wins, losses)
		} else {
			metaPrompt = buildMetaPromptEN(*ctx.TradingStats, wins, losses)
		}
		sb.WriteString(metaPrompt)
		sb.WriteString("\n")
	}
	// 9. 候选币种（带市场数据）
	if len(ctx.CandidateCoins) > 0 {
		if lang == LangChinese {
			sb.WriteString(formatCandidateCoinsZH(ctx))
		} else {
			sb.WriteString(formatCandidateCoinsEN(ctx))
		}
	}
	// 10. OI排名数据（如果有）
	if ctx.OIRankingData != nil {
		if lang == LangChinese {
			sb.WriteString(formatOIRankingZH(ctx.OIRankingData))
		} else {
			sb.WriteString(formatOIRankingEN(ctx.OIRankingData))
		}
	}
	sb.WriteString("---\n\n")
	if lang == LangChinese {
		sb.WriteString("- 你的JSON输出中的所有数字必须是纯数字，不能有千分位分隔符或逗号。\n")
		sb.WriteString("现在请分析并输出你的决策（思维链 + JSON）\n")
	} else {
		sb.WriteString("- All numbers in your JSON output must be plain numbers, without any thousand separators or commas.\n")
		sb.WriteString("Now please analyze and output your decision (Chain of Thought + JSON)\n")
	}

	return sb.String()
}

// ============================================================================
// AI Response Parsing
// ============================================================================

func parseFullDecisionResponse(aiResponse string, accountEquity float64, btcEthLeverage, altcoinLeverage int, btcEthPosRatio, altcoinPosRatio float64) (*FullDecision, error) {
	cotTrace := extractCoTTrace(aiResponse)

	decisions, err := extractDecisions(aiResponse)
	if err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: []Decision{},
		}, fmt.Errorf("failed to extract decisions: %w", err)
	}

	if err := validateDecisions(decisions, accountEquity, btcEthLeverage, altcoinLeverage, btcEthPosRatio, altcoinPosRatio); err != nil {
		return &FullDecision{
			CoTTrace:  cotTrace,
			Decisions: decisions,
		}, fmt.Errorf("decision validation failed: %w", err)
	}

	return &FullDecision{
		CoTTrace:  cotTrace,
		Decisions: decisions,
	}, nil
}

func extractCoTTrace(response string) string {
	if match := reReasoningTag.FindStringSubmatch(response); len(match) > 1 {
		logger.Infof("✓ Extracted reasoning chain using <reasoning> tag")
		return strings.TrimSpace(match[1])
	}

	if decisionIdx := strings.Index(response, "<decision>"); decisionIdx > 0 {
		logger.Infof("✓ Extracted content before <decision> tag as reasoning chain")
		return strings.TrimSpace(response[:decisionIdx])
	}

	jsonStart := strings.Index(response, "[")
	if jsonStart > 0 {
		logger.Infof("⚠️  Extracted reasoning chain using old format ([ character separator)")
		return strings.TrimSpace(response[:jsonStart])
	}

	return strings.TrimSpace(response)
}

func extractDecisions(response string) ([]Decision, error) {
	s := removeInvisibleRunes(response)
	s = strings.TrimSpace(s)
	s = fixMissingQuotes(s)

	var jsonPart string
	if match := reDecisionTag.FindStringSubmatch(s); len(match) > 1 {
		jsonPart = strings.TrimSpace(match[1])
		logger.Infof("✓ Extracted JSON using <decision> tag")
	} else {
		jsonPart = s
		logger.Infof("⚠️  <decision> tag not found, searching JSON in full text")
	}

	jsonPart = fixMissingQuotes(jsonPart)

	if m := reJSONFence.FindStringSubmatch(jsonPart); len(m) > 1 {
		jsonContent := strings.TrimSpace(m[1])
		jsonContent = compactArrayOpen(jsonContent)
		jsonContent = fixMissingQuotes(jsonContent)
		jsonContent = strings.ReplaceAll(jsonContent, "~", "")
		if err := validateJSONFormat(jsonContent); err != nil {
			return nil, fmt.Errorf("JSON format validation failed: %w\nJSON content: %s\nFull response:\n%s", err, jsonContent, response)
		}
		var decisions []Decision
		if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
			return nil, fmt.Errorf("JSON parsing failed: %w\nJSON content: %s", err, jsonContent)
		}
		return decisions, nil
	}

	jsonContent := strings.TrimSpace(reJSONArray.FindString(jsonPart))
	if jsonContent == "" {
		logger.Infof("⚠️  [SafeFallback] AI didn't output JSON decision, entering safe wait mode")

		cotSummary := jsonPart
		if len(cotSummary) > 240 {
			cotSummary = cotSummary[:240] + "..."
		}

		fallbackDecision := Decision{
			Symbol:    "ALL",
			Action:    "wait",
			Reasoning: fmt.Sprintf("Model didn't output structured JSON decision, entering safe wait; summary: %s", cotSummary),
		}

		return []Decision{fallbackDecision}, nil
	}

	jsonContent = compactArrayOpen(jsonContent)
	jsonContent = fixMissingQuotes(jsonContent)
	jsonContent = strings.ReplaceAll(jsonContent, "~", "")
	jsonContent = removeNumberThousandSeparators(jsonContent)

	if err := validateJSONFormat(jsonContent); err != nil {
		return nil, fmt.Errorf("JSON format validation failed: %w\nJSON content: %s\nFull response:\n%s", err, jsonContent, response)
	}

	var decisions []Decision
	if err := json.Unmarshal([]byte(jsonContent), &decisions); err != nil {
		return nil, fmt.Errorf("JSON parsing failed: %w\nJSON content: %s", err, jsonContent)
	}

	return decisions, nil
}

func fixMissingQuotes(jsonStr string) string {
	jsonStr = strings.ReplaceAll(jsonStr, "\u201c", "\"")
	jsonStr = strings.ReplaceAll(jsonStr, "\u201d", "\"")
	jsonStr = strings.ReplaceAll(jsonStr, "\u2018", "'")
	jsonStr = strings.ReplaceAll(jsonStr, "\u2019", "'")

	jsonStr = strings.ReplaceAll(jsonStr, "［", "[")
	jsonStr = strings.ReplaceAll(jsonStr, "］", "]")
	jsonStr = strings.ReplaceAll(jsonStr, "｛", "{")
	jsonStr = strings.ReplaceAll(jsonStr, "｝", "}")
	jsonStr = strings.ReplaceAll(jsonStr, "：", ":")
	jsonStr = strings.ReplaceAll(jsonStr, "，", ",")

	jsonStr = strings.ReplaceAll(jsonStr, "【", "[")
	jsonStr = strings.ReplaceAll(jsonStr, "】", "]")
	jsonStr = strings.ReplaceAll(jsonStr, "〔", "[")
	jsonStr = strings.ReplaceAll(jsonStr, "〕", "]")
	jsonStr = strings.ReplaceAll(jsonStr, "、", ",")

	jsonStr = strings.ReplaceAll(jsonStr, "　", " ")

	return jsonStr
}

func removeNumberThousandSeparators(jsonStr string) string {
	// Remove thousand separators from JSON numbers: "price": 7,787 -> "price": 7787
	// Only remove commas between digits when they're in numeric JSON value contexts
	// Skip commas inside quoted strings - they're legitimate text separators

	var result strings.Builder
	inQuotes := false
	escape := false

	for i := 0; i < len(jsonStr); i++ {
		ch := jsonStr[i]

		// Track if we're inside a quoted string
		if ch == '"' && !escape {
			inQuotes = !inQuotes
		}
		escape = (ch == '\\' && !escape)

		// Only remove comma if it's a thousand separator outside quotes
		if !inQuotes && ch == ',' &&
			i > 0 && i < len(jsonStr)-1 &&
			jsonStr[i-1] >= '0' && jsonStr[i-1] <= '9' &&
			jsonStr[i+1] >= '0' && jsonStr[i+1] <= '9' {
			// Check if this looks like a number context (preceded by : or [, with only whitespace/digits/commas in between)
			foundNumberContext := false
			for j := i - 1; j >= 0; j-- {
				ch := jsonStr[j]
				if ch == ':' || ch == '[' {
					foundNumberContext = true
					break
				}
				// If we hit a quote or closing brace, we're not in number context
				if ch == '"' || ch == '}' || ch == ']' {
					break
				}
				// Only allow digits, commas, whitespace, and minus sign in number context
				if !(ch >= '0' && ch <= '9') && ch != ',' && ch != ' ' && ch != '\t' && ch != '-' && ch != '+' && ch != '.' {
					break
				}
			}

			if foundNumberContext {
				continue // Skip this thousand separator comma
			}
		}

		result.WriteByte(ch)
	}
	return result.String()
}

func validateJSONFormat(jsonStr string) error {
	trimmed := strings.TrimSpace(jsonStr)

	if !reArrayHead.MatchString(trimmed) {
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed[:min(20, len(trimmed))], "{") {
			return fmt.Errorf("not a valid decision array (must contain objects {}), actual content: %s", trimmed[:min(50, len(trimmed))])
		}
		return fmt.Errorf("JSON must start with [{ (whitespace allowed), actual: %s", trimmed[:min(20, len(trimmed))])
	}

	if strings.Contains(jsonStr, "~") {
		return fmt.Errorf("JSON cannot contain range symbol ~, all numbers must be precise single values")
	}

	for i := 0; i < len(jsonStr)-4; i++ {
		if jsonStr[i] >= '0' && jsonStr[i] <= '9' &&
			jsonStr[i+1] == ',' &&
			jsonStr[i+2] >= '0' && jsonStr[i+2] <= '9' &&
			jsonStr[i+3] >= '0' && jsonStr[i+3] <= '9' &&
			jsonStr[i+4] >= '0' && jsonStr[i+4] <= '9' {
			return fmt.Errorf("JSON numbers cannot contain thousand separator comma, found: %s", jsonStr[i:min(i+10, len(jsonStr))])
		}
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func removeInvisibleRunes(s string) string {
	return reInvisibleRunes.ReplaceAllString(s, "")
}

func compactArrayOpen(s string) string {
	return reArrayOpenSpace.ReplaceAllString(strings.TrimSpace(s), "[{")
}

// ============================================================================
// Decision Validation
// ============================================================================

func validateDecisions(decisions []Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, btcEthPosRatio, altcoinPosRatio float64) error {
	for i, decision := range decisions {
		if err := validateDecision(&decision, accountEquity, btcEthLeverage, altcoinLeverage, btcEthPosRatio, altcoinPosRatio); err != nil {
			return fmt.Errorf("decision #%d validation failed: %w", i+1, err)
		}
	}
	return nil
}

func validateDecision(d *Decision, accountEquity float64, btcEthLeverage, altcoinLeverage int, btcEthPosRatio, altcoinPosRatio float64) error {
	validActions := map[string]bool{
		"open_long":   true,
		"open_short":  true,
		"close_long":  true,
		"close_short": true,
		"hold":        true,
		"wait":        true,
	}

	if !validActions[d.Action] {
		return fmt.Errorf("invalid action: %s", d.Action)
	}

	if d.Action == "open_long" || d.Action == "open_short" {
		maxLeverage := altcoinLeverage
		posRatio := altcoinPosRatio
		maxPositionValue := accountEquity * posRatio
		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			maxLeverage = btcEthLeverage
			posRatio = btcEthPosRatio
			maxPositionValue = accountEquity * posRatio
		}
		d.Action = strings.ToLower(d.Action)

		if !validActions[d.Action] {
			return fmt.Errorf("invalid action: %s", d.Action)
		}
		if d.Leverage <= 0 {
			return fmt.Errorf("leverage must be greater than 0: %d", d.Leverage)
		}
		if d.Leverage > maxLeverage {
			logger.Infof("⚠️  [Leverage Fallback] %s leverage exceeded (%dx > %dx), auto-adjusting to limit %dx",
				d.Symbol, d.Leverage, maxLeverage, maxLeverage)
			d.Leverage = maxLeverage
		}
		if d.PositionSizeUSD <= 0 {
			return fmt.Errorf("position size must be greater than 0: %.2f", d.PositionSizeUSD)
		}

		const minPositionSizeGeneral = 12.0
		const minPositionSizeBTCETH = 60.0

		if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
			if d.PositionSizeUSD < minPositionSizeBTCETH {
				return fmt.Errorf("%s opening amount too small (%.2f USDT), must be ≥%.2f USDT", d.Symbol, d.PositionSizeUSD, minPositionSizeBTCETH)
			}
		} else {
			if d.PositionSizeUSD < minPositionSizeGeneral {
				return fmt.Errorf("opening amount too small (%.2f USDT), must be ≥%.2f USDT", d.PositionSizeUSD, minPositionSizeGeneral)
			}
		}

		tolerance := maxPositionValue * 0.01
		if d.PositionSizeUSD > maxPositionValue+tolerance {
			if d.Symbol == "BTCUSDT" || d.Symbol == "ETHUSDT" {
				return fmt.Errorf("BTC/ETH single coin position value cannot exceed %.0f USDT (%.1fx account equity), actual: %.0f", maxPositionValue, posRatio, d.PositionSizeUSD)
			} else {
				return fmt.Errorf("altcoin single coin position value cannot exceed %.0f USDT (%.1fx account equity), actual: %.0f", maxPositionValue, posRatio, d.PositionSizeUSD)
			}
		}
		if d.StopLoss <= 0 || d.TakeProfit <= 0 {
			return fmt.Errorf("stop loss and take profit must be greater than 0")
		}

		if d.Action == "open_long" {
			if d.StopLoss >= d.TakeProfit {
				return fmt.Errorf("for long positions, stop loss price must be less than take profit price")
			}
		} else {
			if d.StopLoss <= d.TakeProfit {
				return fmt.Errorf("for short positions, stop loss price must be greater than take profit price")
			}
		}

		var entryPrice float64
		if d.Action == "open_long" {
			entryPrice = d.StopLoss + (d.TakeProfit-d.StopLoss)*0.2
		} else {
			entryPrice = d.StopLoss - (d.StopLoss-d.TakeProfit)*0.2
		}

		var riskPercent, rewardPercent, riskRewardRatio float64
		if d.Action == "open_long" {
			riskPercent = (entryPrice - d.StopLoss) / entryPrice * 100
			rewardPercent = (d.TakeProfit - entryPrice) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		} else {
			riskPercent = (d.StopLoss - entryPrice) / entryPrice * 100
			rewardPercent = (entryPrice - d.TakeProfit) / entryPrice * 100
			if riskPercent > 0 {
				riskRewardRatio = rewardPercent / riskPercent
			}
		}

		if riskRewardRatio < 3.0 {
			return fmt.Errorf("risk/reward ratio too low (%.2f:1), must be ≥3.0:1 [risk: %.2f%% reward: %.2f%%] [stop loss: %.2f take profit: %.2f]",
				riskRewardRatio, riskPercent, rewardPercent, d.StopLoss, d.TakeProfit)
		}
	}

	return nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// detectLanguage detects language from text content
// Returns LangChinese if text contains Chinese characters, otherwise LangEnglish
func detectLanguage(text string) Language {
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			return LangChinese
		}
	}
	return LangEnglish
}

// buildMetaPromptEN builds a meta-prompt for LLM self-improvement in English
func buildMetaPromptEN(stats TradingStats, wins, losses []interface{}) string {
	var sb strings.Builder
	sb.WriteString("## 📊 Strategy Self-Improvement Opportunity\n\n")
	sb.WriteString("Based on recent performance, consider these improvements to your strategy:\n\n")

	sb.WriteString("**Current Performance:**\n")
	sb.WriteString(fmt.Sprintf("- Win Rate: %.1f%% | Profit Factor: %.2f | Sharpe: %.2f\n", stats.WinRate, stats.ProfitFactor, stats.SharpeRatio))
	sb.WriteString(fmt.Sprintf("- Avg Win: $%.2f | Avg Loss: $%.2f | Max Drawdown: %.1f%%\n\n", stats.AvgWin, stats.AvgLoss, stats.MaxDrawdownPct))

	sb.WriteString(fmt.Sprintf("**Recent Winning Trades (%d):**\n", len(wins)))
	for i, trade := range wins {
		if i >= 3 {
			break
		}
		// Use type assertion to get trade fields
		if ro, ok := trade.(RecentOrder); ok {
			sb.WriteString(fmt.Sprintf("- %s %s: Entry $%.2f → Exit $%.2f, +$%.2f (%.1f%%) in %s\n",
				ro.Symbol, ro.Side, ro.EntryPrice, ro.ExitPrice, ro.RealizedPnL, ro.PnLPct, ro.HoldDuration))
		}
	}

	sb.WriteString(fmt.Sprintf("\n**Recent Losing Trades (%d):**\n", len(losses)))
	for i, trade := range losses {
		if i >= 3 {
			break
		}
		// Use type assertion to get trade fields
		if ro, ok := trade.(RecentOrder); ok {
			sb.WriteString(fmt.Sprintf("- %s %s: Entry $%.2f → Exit $%.2f, -$%.2f (%.1f%%) in %s\n",
				ro.Symbol, ro.Side, ro.EntryPrice, ro.ExitPrice, ro.RealizedPnL, ro.PnLPct, ro.HoldDuration))
		}
	}

	sb.WriteString("\n**Questions for Self-Improvement:**\n")
	sb.WriteString("1. What patterns differentiate the winning trades from the losing ones?\n")
	sb.WriteString("2. Can you identify specific entry/exit rules that would eliminate the recent losses?\n")
	sb.WriteString("3. Are there conditions (market regime, time, volatility) where your approach breaks down?\n")
	sb.WriteString("4. How could you adjust your strategy to maintain wins while reducing losses?\n")
	sb.WriteString("Please analyze rationally step by step and think carefully before placing the next 5 trades, and review carefully after trading.\n\n")

	return sb.String()
}

// buildMetaPromptZH builds a meta-prompt for LLM self-improvement in Chinese
func buildMetaPromptZH(stats TradingStats, wins, losses []interface{}) string {
	var sb strings.Builder
	sb.WriteString("## 📊 策略自我改进机会\n\n")
	sb.WriteString("基于最近的表现，考虑对你的策略进行以下改进：\n\n")

	sb.WriteString("**当前表现：**\n")
	sb.WriteString(fmt.Sprintf("- 胜率: %.1f%% | 利润因子: %.2f | 夏普比: %.2f\n", stats.WinRate, stats.ProfitFactor, stats.SharpeRatio))
	sb.WriteString(fmt.Sprintf("- 平均赢: $%.2f | 平均亏: $%.2f | 最大回撤: %.1f%%\n\n", stats.AvgWin, stats.AvgLoss, stats.MaxDrawdownPct))

	sb.WriteString(fmt.Sprintf("**最近的盈利交易 (%d)：**\n", len(wins)))
	for i, trade := range wins {
		if i >= 3 {
			break
		}
		// Use type assertion to get trade fields
		if ro, ok := trade.(RecentOrder); ok {
			sb.WriteString(fmt.Sprintf("- %s %s: 入场 $%.2f → 出场 $%.2f, +$%.2f (%.1f%%) 持仓 %s\n",
				ro.Symbol, ro.Side, ro.EntryPrice, ro.ExitPrice, ro.RealizedPnL, ro.PnLPct, ro.HoldDuration))
		}
	}

	sb.WriteString(fmt.Sprintf("\n**最近的亏损交易 (%d)：**\n", len(losses)))
	for i, trade := range losses {
		if i >= 3 {
			break
		}
		// Use type assertion to get trade fields
		if ro, ok := trade.(RecentOrder); ok {
			sb.WriteString(fmt.Sprintf("- %s %s: 入场 $%.2f → 出场 $%.2f, -$%.2f (%.1f%%) 持仓 %s\n",
				ro.Symbol, ro.Side, ro.EntryPrice, ro.ExitPrice, ro.RealizedPnL, ro.PnLPct, ro.HoldDuration))
		}
	}

	sb.WriteString("\n**自我改进问题：**\n")
	sb.WriteString("1. 盈利交易和亏损交易之间有什么关键差异？ \n")
	sb.WriteString("2. 你能识别哪些具体的进出场规则可以避免最近的亏损吗？\n")
	sb.WriteString("3. 有没有某些条件（市场态势、时间、波动率）会导致你的方法失效？\n")
	sb.WriteString("4. 你如何调整策略来保持赢利同时减少亏损？\n")
	sb.WriteString("请理性一步步分析且再下5个交易前仔细思考，交易后仔细复盘。\n\n")

	return sb.String()
}

// convertDIYToQuantData converts DIY quant data to QuantData format
func convertDIYToQuantData(diy *provider.DIYQuantData) *QuantData {
	if diy == nil {
		return nil
	}

	qd := &QuantData{
		Symbol: diy.Symbol,
		Price:  diy.Price.Current,
	}

	// Convert netflow data
	if diy.Netflow != nil {
		// Map taker buy/sell to institutional/personal approximation
		// High taker buy ratio suggests institutional long positions
		inst := make(map[string]float64)
		pers := make(map[string]float64)

		// Estimate institutional vs personal based on taker ratio
		if diy.Netflow.TakerBuyRatio > 55 {
			// More institutional buying
			inst["future"] = diy.Netflow.TakerBuyVolume * 0.7 // 70% attributed to institutions
			pers["future"] = diy.Netflow.TakerBuyVolume * 0.3 // 30% retail
		} else if diy.Netflow.TakerBuyRatio < 45 {
			// More institutional selling
			inst["future"] = -diy.Netflow.TakerSellVolume * 0.7
			pers["future"] = -diy.Netflow.TakerSellVolume * 0.3
		} else {
			// Balanced - split evenly
			inst["future"] = (diy.Netflow.TakerBuyVolume - diy.Netflow.TakerSellVolume) * 0.5
			pers["future"] = (diy.Netflow.TakerBuyVolume - diy.Netflow.TakerSellVolume) * 0.5
		}

		qd.Netflow = &NetflowData{
			Institution: &FlowTypeData{Future: inst},
			Personal:    &FlowTypeData{Future: pers},
		}
	}

	// Convert OI data
	if diy.OI != nil {
		oiMap := make(map[string]*OIData)
		oiMap["24h"] = &OIData{
			CurrentOI: diy.OI.Current,
			Delta: map[string]*OIDeltaData{
				"24h": {
					OIDelta:        diy.OI.Change24h,
					OIDeltaValue:   diy.OI.Change24h * diy.Price.Current,
					OIDeltaPercent: diy.OI.ChangePercent24h,
				},
			},
		}
		qd.OI = oiMap
	}

	// Convert price data
	if diy.Price != nil {
		priceChange := make(map[string]float64)
		// Normalize from percentage (-1.5 for -1.5%) to decimal form (-0.015 for -1.5%) to match external API format
		priceChange["24h"] = diy.Price.ChangePercent24h / 100.0
		qd.PriceChange = priceChange
	}

	return qd
}
