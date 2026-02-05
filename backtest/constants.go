package backtest

// ============================================================================
// Trading Constants - Consolidated Magic Numbers
// ============================================================================
// This file consolidates all hard-coded constants used throughout the backtest
// system for easier maintenance and configuration.
// ============================================================================

// Leverage Configuration
const (
	DefaultMinLeverage        = 1
	DefaultMaxLeverage        = 10
	DefaultOptimalLeverage    = 3
	BTCETHLeverageMultiplier  = 2.0
	AltcoinLeverageMultiplier = 1.0
)

// Risk Management Thresholds
const (
	DefaultMaxDrawdownPct       = 20.0 // Maximum acceptable drawdown percentage
	DefaultDrawdownWarningLevel = 15.0 // Start warning at this drawdown
	DefaultStopLossPct          = 3.0  // Default stop-loss percentage
	DefaultTakeProfitPct        = 6.0  // Default take-profit percentage
	MaxDrawdownLimit            = 40.0 // Hard limit for drawdown
	CriticalDrawdownThreshold   = 30.0 // Critical drawdown level
)

// Position Sizing
const (
	DefaultMinPositionSize   = 50.0   // Minimum position size in USD
	DefaultMaxPositionSize   = 1000.0 // Maximum position size in USD
	DefaultBasePositionSize  = 200.0  // Base position size
	MinimumPositionValue     = 10.0   // Absolute minimum in USD
	MaxPositionsPerAccount   = 5      // Maximum concurrent positions
	DefaultPositionSizeScale = 1.0    // Scaling factor for position sizing
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

// Risk Reward Ratios
const (
	DefaultMinRiskRewardRatio = 1.5 // Minimum R:R ratio
	OptimalRiskRewardRatio    = 2.0 // Ideal R:R ratio
	MaxRiskRewardRatio        = 5.0 // Maximum acceptable R:R ratio
)

// Performance Thresholds for Optimization
const (
	MinWinRateForSuccess     = 50.0 // Win rate percentage to be profitable
	GoodWinRate              = 55.0 // Good win rate threshold
	ExcellentWinRate         = 65.0 // Excellent win rate threshold
	MinProfitFactorThreshold = 1.0  // Break-even profit factor
	GoodProfitFactor         = 1.5  // Good profit factor
	ExcellentProfitFactor    = 2.0  // Excellent profit factor
)

// Margin and Leverage Safety
const (
	DefaultMaxMarginUsage = 0.9  // 90% max margin usage
	SafeMarginLevel       = 0.7  // Safe margin usage level
	WarningMarginLevel    = 0.8  // Start warning at this level
	CriticalMarginLevel   = 0.95 // Critical margin usage
	MinMarginBuffer       = 0.05 // Minimum margin buffer (5%)
)

// Feedback and Pattern Detection
const (
	MinTradesForFeedback   = 10 // Minimum trades before generating feedback
	MinPatternsToReport    = 2  // Minimum pattern occurrences to report
	PatternFrequencyMin    = 3  // Minimum frequency to be significant
	FeedbackUpdateInterval = 5  // Update feedback every N cycles
)

// Optimization Cycles and Frequencies
const (
	DefaultFactorOptimizationCycles = 15 // Optimize parameters every N cycles
	DefaultPromptEvolutionCycles    = 20 // Evolve prompts every N cycles
	MinCyclesBeforeOptimization     = 10 // Need at least N cycles
	OptimizationCompletionThreshold = 20 // Consider optimization complete after N cycles
)

// Compliance and Tracking
const (
	DefaultComplianceRewardPoints  = 1.0  // Points for following recommendation
	DefaultCompliancePenaltyPoints = -0.5 // Points for violating recommendation
	ComplianceRateGood             = 0.75 // 75%+ is good
	ComplianceRateExcellent        = 0.85 // 85%+ is excellent
	ComplianceRatePoor             = 0.40 // <40% is poor
)

// Time-based Constants
const (
	DefaultCheckpointIntervalSeconds = 2  // Checkpoint every N seconds
	DefaultCheckpointIntervalBars    = 20 // Checkpoint every N bars
	DefaultDecisionCadenceNBars      = 20 // Make decisions every N bars
	WebsocketHeartbeatInterval       = 30 // Heartbeat every N seconds
	WebsocketReconnectDelay          = 5  // Reconnect delay in seconds
)

// Data Quality and Validation
const (
	MinimumKlineCount        = 10    // Minimum candles needed for analysis
	MaximumKlineAge          = 3600  // Maximum age of kline data in seconds
	MinimumVolumeThreshold   = 0.0   // Minimum volume requirement
	PriceValidationTolerance = 0.001 // Allow 0.1% price variance
)

// Drawer and Profit Thresholds
const (
	MinProfitThresholdForMonitoring = 5.0  // Start monitoring drawdown after N% profit
	DrawdownCloseThreshold          = 30.0 // Close position if drawdown exceeds N%
	QuickProfitTarget               = 3.0  // Quick profit target percentage
	SwingTradeTarget                = 10.0 // Swing trade target percentage
)

// Sentiment and Pattern Scoring
const (
	PositiveSentimentThreshold = 0.6 // Positive sentiment above this
	NegativeSentimentThreshold = 0.4 // Negative sentiment below this
	NeutralSentimentRange      = 0.1 // Range around 0.5 for neutral
	PatternConfidenceWeight    = 0.3 // Weight for pattern confidence
	TechnicalConfidenceWeight  = 0.4 // Weight for technical analysis
	SentimentConfidenceWeight  = 0.3 // Weight for sentiment
)

// Data Source Configuration
const (
	DefaultTimeframe           = "15m" // Default analysis timeframe
	ConfirmationTimeframe      = "5m"  // Confirmation timeframe
	LongerTimeframe            = "1h"  // Longer timeframe for context
	MaxTimeframesPerStrategy   = 3     // Maximum timeframes to track
	MinCandlesForValidAnalysis = 50    // Minimum candles for valid analysis
)

// HTTP and API Configuration
const (
	DefaultHTTPTimeout    = 30 // HTTP timeout in seconds
	DefaultRetryAttempts  = 3  // Number of retry attempts
	DefaultRetryDelay     = 1  // Retry delay in seconds
	MaxConcurrentRequests = 10 // Maximum concurrent API requests
	RateLimitWaitTime     = 1  // Wait time for rate limit in seconds
)

// Error and Exception Handling
const (
	DefaultErrorWaitTime = 5   // Wait time before retry after error (seconds)
	MaxConsecutiveErrors = 5   // Max consecutive errors before shutdown
	ErrorRecoveryTimeout = 300 // Error recovery timeout in seconds
)

// Performance Monitoring
const (
	PerformanceCheckInterval = 60  // Check performance every N seconds
	PerformanceHistorySize   = 100 // Keep N performance records
	MetricsUpdateInterval    = 5   // Update metrics every N cycles
)
