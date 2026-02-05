package config

// Trading Configuration Constants
// This package contains all magic numbers and configuration values
// used across the application to avoid circular imports

// Leverage Configuration
const (
	MinLeverage        = 1.0  // Minimum leverage allowed
	DefaultMaxLeverage = 10.0 // Default maximum leverage
	OptimalLeverage    = 3.0  // Optimal leverage for most strategies
	MaxAllowedLeverage = 20.0 // Absolute maximum leverage
	SafeLeverage       = 2.0  // Conservative leverage
	AggressiveLeverage = 5.0  // Aggressive leverage
)

// Risk Management Configuration
const (
	DefaultMaxDrawdownPct       = 20.0 // Maximum portfolio drawdown %
	DefaultDrawdownWarningLevel = 15.0 // Alert when drawdown reaches this %
	CriticalDrawdownThreshold   = 25.0 // Stop trading threshold
	StopLossPct                 = 3.0  // Stop loss percentage
	TakeProfitPct               = 10.0 // Take profit percentage
	BreakevenPct                = 1.0  // Point at which to move SL to breakeven
	DefaultRiskPerTrade         = 2.0  // Risk per trade as % of equity
	MaxRiskPerTrade             = 5.0  // Maximum risk per single trade
)

// Position Sizing Configuration
const (
	MinPositionSize        = 50.0   // Minimum position size in USD
	MaxPositionSize        = 1000.0 // Maximum position size in USD
	MaxPositionsPerAccount = 5      // Maximum concurrent positions
	DefaultPositionSize    = 100.0  // Default position size
)

// Confidence Thresholds
const (
	MinConfidenceForEntry   = 65 // Minimum confidence to enter trade
	MinConfidenceForSize    = 75 // Minimum confidence for full position
	LowConfidenceThreshold  = 40 // Below this is too risky
	HighConfidenceThreshold = 75 // Above this is strong signal
	CriticalConfidenceLevel = 90 // Very high confidence

	// Confidence levels for position sizing guidance
	ConfidenceHigh         = 85  // ≥85% confidence level
	ConfidenceHighMin      = 80  // Min position % for high confidence
	ConfidenceHighMax      = 100 // Max position % for high confidence
	ConfidenceMediumMin    = 70  // Medium confidence range start
	ConfidenceMediumMax    = 84  // Medium confidence range end
	ConfidenceMediumPosMin = 50  // Min position % for medium confidence
	ConfidenceMediumPosMax = 80  // Max position % for medium confidence
	ConfidenceLow          = 60  // Low confidence minimum
	ConfidenceLowMax       = 69  // Low confidence range end
	ConfidenceLowPosMin    = 30  // Min position % for low confidence
	ConfidenceLowPosMax    = 50  // Max position % for low confidence
)

// Performance Thresholds
const (
	MinWinRate            = 50.0 // Minimum acceptable win rate %
	GoodWinRate           = 55.0 // Good win rate
	ExcellentWinRate      = 65.0 // Excellent win rate
	MinProfitFactor       = 1.2  // Minimum profit factor
	GoodProfitFactor      = 1.5  // Good profit factor
	ExcellentProfitFactor = 2.0  // Excellent profit factor
)

// Portfolio Configuration
const (
	DefaultEquity        = 10000.0 // Default starting equity
	MinEquity            = 100.0   // Minimum equity for trading
	MaxEquityPerPosition = 0.2     // Max 20% of equity per position
	EquityCheckInterval  = 60      // Check equity every 60 seconds
)

// Market Data Configuration
const (
	DefaultKlineInterval       = 5    // Default kline interval (5 minutes)
	MinKlineInterval           = 1    // Minimum kline interval (1 minute)
	MaxKlineInterval           = 1440 // Maximum kline interval (1 day = 1440 min)
	HistoricalDataDays         = 30   // Days of historical data to fetch
	WebsocketReconnectWait     = 5    // Seconds to wait before reconnecting
	WebsocketMaxRetries        = 10   // Max reconnection attempts
	OrderBookDepth             = 20   // Order book depth to maintain
	OrderBookUpdateInterval    = 100  // ms - update frequency
	MaxPriceDeviationThreshold = 0.02 // 2% - max deviation between ticker and kline
)

// Default Position Ratios
const (
	DefaultBTCETHPosRatio  = 5.0 // BTC/ETH position ratio
	DefaultAltcoinPosRatio = 1.0 // Altcoin position ratio
	MaxBTCRatio            = 0.4 // Max 40% in BTC
	MaxETHRatio            = 0.3 // Max 30% in ETH
	MaxAltcoinRatio        = 0.3 // Max 30% in altcoins
)

// Backtest Configuration
const (
	DefaultBacktestDays      = 90 // Default backtest period
	BacktestWarmupDays       = 7  // Warmup period before trading
	MinBacktestDays          = 1  // Minimum backtest period
	BacktestProgressInterval = 10 // Report progress every N days
)

// Monitoring & Alerts
const (
	PositionMonitorInterval = 30   // Check positions every 30 seconds
	MetricsUpdateInterval   = 300  // Update metrics every 5 minutes
	AlertCheckInterval      = 60   // Check alerts every 60 seconds
	LogRotationSize         = 1024 // MB - rotate logs at this size
	MetricsRetentionDays    = 90   // Keep metrics for 90 days
)

// Timeframe Configuration
const (
	Timeframe1Min  = 1
	Timeframe5Min  = 5
	Timeframe15Min = 15
	Timeframe30Min = 30
	Timeframe1Hour = 60
	Timeframe4Hour = 240
	Timeframe1Day  = 1440
)

// API Configuration
const (
	DefaultAPITimeout      = 30   // Seconds
	MaxAPIRetries          = 3    // Maximum retry attempts
	APIRetryWaitTime       = 2    // Seconds between retries
	RequestRateLimit       = 1200 // Requests per minute
	WebsocketMessageBuffer = 1000 // Message buffer size
	WebsocketPingInterval  = 30   // Seconds
	WebsocketReadTimeout   = 60   // Seconds
)

// Exchange Fee Defaults
const (
	DefaultBinanceTakerFeeRate = 0.0004 // 0.04% taker fee
)

// Validation Constants
const (
	MinOrderQuantity      = 0.001   // Minimum order quantity
	MaxOrderQuantity      = 1000000 // Maximum order quantity
	MinOrderPrice         = 0.00001 // Minimum order price
	MaxOrderPrice         = 1000000 // Maximum order price
	PriceDecimalPlaces    = 8       // Decimal places for prices
	QuantityDecimalPlaces = 8       // Decimal places for quantities
)

// AI/Strategy Configuration
const (
	MaxPromptTokens        = 4000 // Max tokens in prompt
	MaxResponseTokens      = 2000 // Max tokens in response
	DefaultTemperature     = 0.7  // LLM temperature
	DefaultTopP            = 0.9  // LLM top-p sampling
	DebateRoundCount       = 3    // Number of debate rounds
	MaxDebateAgents        = 5    // Maximum agents in debate
	AnalysisTimeout        = 30   // Seconds for analysis
	OptimizationIterations = 5    // Parameter optimization iterations
)

// Compliance & Risk Controls
const (
	MaxDailyLossPct            = 5.0  // Stop trading if daily loss > 5%
	MaxMonthlyLossPct          = 10.0 // Stop trading if monthly loss > 10%
	MinRequiredWinRateForTrade = 0.45 // Minimum win rate to continue
	MaxConsecutiveLosingTrades = 5    // Alert after 5 losing trades
	DailyTradeLimit            = 100  // Max trades per day
	WeeklyTradeLimit           = 500  // Max trades per week
	MaxOpenOrdersPerSymbol     = 3    // Max concurrent orders per symbol
)

// Time Configuration
const (
	MarketOpenHour        = 0  // Market open (UTC, 0 = 00:00)
	MarketCloseHour       = 24 // Market close (UTC, 24 = 00:00 next day)
	DefaultTimeZone       = "UTC"
	TickerUpdateFrequency = 1000 // ms
)
