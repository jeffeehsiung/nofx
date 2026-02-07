package backtest

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/decision"
	"nofx/logger"
	"nofx/market"
	"nofx/store"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ============================================================================
// Feedback Loop System - Unsupervised Learning from Historical Performance
// ============================================================================
// This module implements a feedback loop that analyzes past trading decisions
// and their outcomes to help the LLM learn from mistakes and improve profitability.
// IMPLEMENTED: LLM-based feedback generation for AI-driven analysis evolution
// - FeedbackGenerator now supports AIClient for LLM-powered feedback analysis
// - GenerateFeedbackWithLLM() method evolves feedback based on recorded trading outcomes
// - Feedback includes quantified metrics instead of qualitative phrases
// - Results are parsed into rules enforced by ComplianceTracker (backtest/compliance_tracker.go)
// See: GenerateFeedbackWithLLM(), SetAIClient(), buildFeedbackAnalysisPrompt()
// IMPLEMENTED: Pattern identifiers now leverage microstructure data for deeper insights (Trade Failure V2)
// IMPLEMENTED: Optional LLM-assisted pattern detection for unsupervised, complex pattern discovery
// IMPLEMENTED: Optional LLM-assisted insights and recommendations for nuanced analysis beyond heuristics
// ============================================================================

// Add to FeedbackGenerator struct
type TradingFrequencyMetrics struct {
	TradesPerHour           float64
	AvgTimeBetweenTrades    time.Duration
	ConsecutiveTradingHours int
	MaxTradesInHour         int
}

// FeedbackAnalysis contains comprehensive analysis of historical trading performance
type FeedbackAnalysis struct {
	// Period analyzed
	AnalysisPeriod   string    `json:"analysis_period"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	DecisionsCovered int       `json:"decisions_covered"`

	// Overall performance
	TotalReturn    float64 `json:"total_return"`
	TotalReturnPct float64 `json:"total_return_pct"`
	WinRate        float64 `json:"win_rate"`
	ProfitFactor   float64 `json:"profit_factor"`
	SharpeRatio    float64 `json:"sharpe_ratio"`
	MaxDrawdown    float64 `json:"max_drawdown"`

	// Pattern analysis
	SuccessPatterns []TradingPattern `json:"success_patterns"`
	FailurePatterns []TradingPattern `json:"failure_patterns"`

	// Actionable insights
	KeyInsights        []string `json:"key_insights"`
	RecommendedActions []string `json:"recommended_actions"`

	// Detailed decision analysis
	AllOutcomes      []DecisionOutcome `json:"all_outcomes"` // NEW: All outcomes for calculations
	TopWinningTrades []DecisionOutcome `json:"top_winning_trades"`
	TopLosingTrades  []DecisionOutcome `json:"top_losing_trades"`

	// Market regime analysis
	MarketConditions string             `json:"market_conditions"`
	RegimeAnalysis   map[string]float64 `json:"regime_analysis"`

	// Calculated metrics for display
	TradesPerHour       float64 `json:"trades_per_hour"`      // NEW
	AvgHoldTime         string  `json:"avg_hold_time"`        // NEW
	ChecklistCompliance float64 `json:"checklist_compliance"` // NEW
}

// TradingPattern represents a discovered pattern in trading behavior
type TradingPattern struct {
	PatternType    string   `json:"pattern_type"`   // e.g., "high_leverage_losses", "quick_exits_profitable"
	Frequency      int      `json:"frequency"`      // how often this pattern occurred
	AvgPnL         float64  `json:"avg_pnl"`        // average P&L for this pattern
	AvgPnLPct      float64  `json:"avg_pnl_pct"`    // average P&L percentage
	Description    string   `json:"description"`    // human-readable description
	Evidence       []string `json:"evidence"`       // specific examples
	Recommendation string   `json:"recommendation"` // what to do about it
}

// DecisionOutcome links a decision to its actual outcome
type DecisionOutcome struct {
	Timestamp  time.Time `json:"timestamp"`
	Symbol     string    `json:"symbol"`
	Action     string    `json:"action"`
	Reasoning  string    `json:"reasoning"`
	Confidence int       `json:"confidence"`

	// Entry details
	EntryPrice   float64 `json:"entry_price"`
	PositionSize float64 `json:"position_size"`
	Leverage     int     `json:"leverage"`

	// Exit details
	ExitPrice    float64 `json:"exit_price"`
	HoldDuration string  `json:"hold_duration"`

	// Outcome
	RealizedPnL    float64 `json:"realized_pnl"`
	RealizedPnLPct float64 `json:"realized_pnl_pct"`
	Success        bool    `json:"success"`

	// What went right/wrong
	Analysis string `json:"analysis"`

	// NEW: Detailed microstructure data for Trade Failure V2 analysis
	RecentOrder *decision.RecentOrder `json:"recent_order,omitempty"`
}

// FeedbackConfig controls feedback loop behavior
type FeedbackConfig struct {
	EnableFeedback          bool `json:"enable_feedback"`
	MinDecisionsForFeedback int  `json:"min_decisions_for_feedback"` // minimum decisions before generating feedback
	FeedbackWindowCycles    int  `json:"feedback_window_cycles"`     // how many recent cycles to analyze
	TopTradesCount          int  `json:"top_trades_count"`           // number of top winning/losing trades to show
	MinPatternFrequency     int  `json:"min_pattern_frequency"`      // minimum occurrences to consider a pattern
	EnableMicrostructure    bool `json:"enable_microstructure"`      // use microstructure for pattern detection
	EnableLLMPatterns       bool `json:"enable_llm_patterns"`        // LLM-assisted pattern discovery
	EnableLLMInsights       bool `json:"enable_llm_insights"`        // LLM-assisted insights/recommendations
}

// DefaultFeedbackConfig returns sensible defaults (all disabled)
func DefaultFeedbackConfig() FeedbackConfig {
	return FeedbackConfig{
		EnableFeedback:          false,
		MinDecisionsForFeedback: 15,
		FeedbackWindowCycles:    10,
		TopTradesCount:          3,
		MinPatternFrequency:     7,
		EnableMicrostructure:    false,
		EnableLLMPatterns:       false,
		EnableLLMInsights:       false,
	}
}

// SmartFeedbackConfig creates intelligent defaults based on decision cadence and user preferences
// decisionCadenceNBars: how many bars between each decision (higher = slower decisions)
// enableFeedback: whether to enable feedback analysis
// enableLLM: whether to enable LLM-assisted patterns and insights
func SmartFeedbackConfig(decisionCadenceNBars int, enableFeedback, enableLLM bool) FeedbackConfig {
	cfg := DefaultFeedbackConfig()
	cfg.EnableFeedback = enableFeedback

	if !enableFeedback {
		return cfg // Return defaults if feedback disabled
	}

	// Always enable microstructure when feedback is enabled
	cfg.EnableMicrostructure = true
	cfg.EnableLLMPatterns = enableLLM
	cfg.EnableLLMInsights = enableLLM

	// Scale feedback thresholds based on decision cadence
	// Aim for feedback to start after ~150-200 bars of trading
	if decisionCadenceNBars > 0 {
		targetBarsBeforeFeedback := 150
		cfg.MinDecisionsForFeedback = max(3, targetBarsBeforeFeedback/decisionCadenceNBars)

		// Regenerate feedback every ~100-150 bars
		targetBarsPerFeedback := 100
		cfg.FeedbackWindowCycles = max(5, targetBarsPerFeedback/decisionCadenceNBars)
	}

	// Scale top trades count based on expected decision volume
	// If we make many decisions quickly, show more trades
	if decisionCadenceNBars < 10 {
		cfg.TopTradesCount = 5
	} else if decisionCadenceNBars > 30 {
		cfg.TopTradesCount = 2
	} else {
		cfg.TopTradesCount = 3
	}

	// Scale pattern frequency based on expected sample size
	if decisionCadenceNBars < 10 {
		cfg.MinPatternFrequency = 10
	} else if decisionCadenceNBars > 30 {
		cfg.MinPatternFrequency = 5
	} else {
		cfg.MinPatternFrequency = 7
	}

	return cfg
}

func (fg *FeedbackGenerator) DisableFeedback() {
	fg.config.EnableFeedback = false
}

// FeedbackGenerator generates feedback from historical performance
type FeedbackGenerator struct {
	runID  string
	config FeedbackConfig

	// Calibrated thresholds for Trade Failure V2 (defaults if calibration unavailable)
	initialBalance    float64
	failureThresholds decision.FailureThresholds

	// AI client for LLM-based feedback evolution
	AIClient interface {
		CallWithMessages(systemPrompt, userPrompt string) (string, error)
	}
}

// NewFeedbackGenerator creates a new feedback generator
func NewFeedbackGenerator(runID string, initialBalance float64, config FeedbackConfig) *FeedbackGenerator {
	return &FeedbackGenerator{
		runID:             runID,
		config:            config,
		initialBalance:    initialBalance,
		failureThresholds: decision.DefaultFailureThresholds(),
		AIClient:          nil, // Optional: set via SetAIClient()
	}
}

// SetAIClient injects an AI client for LLM-based feedback evolution
func (fg *FeedbackGenerator) SetAIClient(client interface {
	CallWithMessages(systemPrompt, userPrompt string) (string, error)
}) {
	fg.AIClient = client
}

// SetFailureThresholds injects calibrated thresholds (idempotent fallback-safe)
func (fg *FeedbackGenerator) SetFailureThresholds(thresholds decision.FailureThresholds) {
	fg.failureThresholds = thresholds
}

// GenerateFeedbackWithLLM generates feedback evolved by LLM based on trading outcomes
// Uses AIClient to analyze trading performance and suggest quantified improvements
// Returns enhanced FeedbackAnalysis with LLM-validated patterns and quantified rules
func (fg *FeedbackGenerator) GenerateFeedbackWithLLM() (*FeedbackAnalysis, error) {
	if fg.AIClient == nil {
		// Fallback to standard generation if no AI client available
		return fg.GenerateFeedback()
	}

	// Generate base analysis then replace hardcoded components using LLM
	analysis, err := fg.GenerateFeedback()
	if err != nil || analysis == nil {
		return analysis, err
	}

	if fg.applyLLMFullAnalysis(analysis, analysis.AllOutcomes, true) {
		logger.Infof("[FeedbackGenerator] ✅ LLM full analysis applied")
	} else if fg.applyLLMEnhancements(analysis) {
		logger.Infof("[FeedbackGenerator] ✅ LLM enhancements applied to patterns/insights")
	}

	// Keep quantified rules pass if LLM full analysis didn't provide actions
	if len(analysis.RecommendedActions) == 0 {
		metaPrompt := fg.buildFeedbackAnalysisPrompt(analysis)
		lang := "en"
		if len(analysis.FailurePatterns) > 0 && strings.Contains(analysis.FailurePatterns[0].Description, "日") {
			lang = "zh"
		}

		userPrompt := "Please analyze the following trading performance and provide quantified rules for improvement:\n"
		if lang == "zh" {
			userPrompt = "请分析以下交易表现，并提供量化的改进规则：\n"
		}

		evolvedRules, err := fg.AIClient.CallWithMessages(metaPrompt, userPrompt)
		if err == nil {
			updatedActions := fg.parseEvolvedRules(evolvedRules)
			if len(updatedActions) > 0 {
				analysis.RecommendedActions = updatedActions
				logger.Infof("[FeedbackGenerator] ✅ LLM-enhanced feedback generated with %d quantified rules", len(updatedActions))
			}
		}
	}

	return analysis, nil
}

// buildFeedbackAnalysisPrompt creates a meta-prompt for LLM to analyze trading performance
func (fg *FeedbackGenerator) buildFeedbackAnalysisPrompt(analysis *FeedbackAnalysis) string {
	var sb strings.Builder
	if strings.Contains(analysis.AnalysisPeriod, "日") {
		// Chinese version
		sb.WriteString("你是交易数据分析专家。你的任务是分析交易表现，并提供量化规则。\n\n")
		sb.WriteString("# 交易表现分析\n\n")
		sb.WriteString(fmt.Sprintf("- **总收益:** %.2f%%\n", analysis.TotalReturnPct))
		sb.WriteString(fmt.Sprintf("- **胜率:** %.1f%%\n", analysis.WinRate))
		sb.WriteString(fmt.Sprintf("- **利润因子:** %.2f\n", analysis.ProfitFactor))
		sb.WriteString(fmt.Sprintf("- **每小时交易:** %.2f\n", analysis.TradesPerHour))
		sb.WriteString(fmt.Sprintf("- **平均持仓时间:** %s\n", analysis.AvgHoldTime))
		sb.WriteString(fmt.Sprintf("- **最大回撤:** %.1f%%\n", analysis.MaxDrawdown))
		sb.WriteString("\n# 识别的失败模式\n")
		for _, pattern := range analysis.FailurePatterns {
			sb.WriteString(fmt.Sprintf("- %s (出现%d次，平均亏损: %.2f, 概率: %.1f%%)\n",
				pattern.Description, pattern.Frequency, pattern.AvgPnL, pattern.AvgPnLPct*100))
		}
		sb.WriteString("\n# 成功模式\n")
		for _, pattern := range analysis.SuccessPatterns {
			sb.WriteString(fmt.Sprintf("- %s (出现%d次，平均收益: %.2f, 概率: %.1f%%)\n",
				pattern.Description, pattern.Frequency, pattern.AvgPnL, pattern.AvgPnLPct*100))
		}
		sb.WriteString("\n# 任务\n")
		sb.WriteString("基于上述分析，提供5-7个具体的、量化的改进规则。每条规则应该包括：\n")
		sb.WriteString("1. 问题描述\n")
		sb.WriteString("2. 量化指标（具体数值）\n")
		sb.WriteString("3. 执行方法\n")
		sb.WriteString("4. 预期改进（百分比）\n\n")
		sb.WriteString("格式: 规则[N]: [描述] | 指标: [具体数值] | 方法: [如何执行] | 预期改进: [百分比]\n")
	} else {
		// English version
		sb.WriteString("You are a trading data analyst expert. Your task is to analyze trading performance and provide quantified rules.\n\n")
		sb.WriteString("# Trading Performance Analysis\n\n")
		sb.WriteString(fmt.Sprintf("- **Total Return:** %.2f%%\n", analysis.TotalReturnPct))
		sb.WriteString(fmt.Sprintf("- **Win Rate:** %.1f%%\n", analysis.WinRate))
		sb.WriteString(fmt.Sprintf("- **Profit Factor:** %.2f\n", analysis.ProfitFactor))
		sb.WriteString(fmt.Sprintf("- **Trades Per Hour:** %.2f\n", analysis.TradesPerHour))
		sb.WriteString(fmt.Sprintf("- **Average Hold Time:** %s\n", analysis.AvgHoldTime))
		sb.WriteString(fmt.Sprintf("- **Max Drawdown:** %.1f%%\n", analysis.MaxDrawdown))
		sb.WriteString("\n# Identified Failure Patterns\n")
		for _, pattern := range analysis.FailurePatterns {
			sb.WriteString(fmt.Sprintf("- %s (occurred %d times, avg loss: %.2f, avg loss %%: %.1f%%)\n",
				pattern.Description, pattern.Frequency, pattern.AvgPnL, pattern.AvgPnLPct*100))
		}
		sb.WriteString("\n# Success Patterns\n")
		for _, pattern := range analysis.SuccessPatterns {
			sb.WriteString(fmt.Sprintf("- %s (occurred %d times, avg gain: %.2f, avg gain %%: %.1f%%)\n",
				pattern.Description, pattern.Frequency, pattern.AvgPnL, pattern.AvgPnLPct*100))
		}
		sb.WriteString("\n# Task\n")
		sb.WriteString("Based on the analysis above, provide 5-7 specific, quantified improvement rules. Each rule should include:\n")
		sb.WriteString("1. Problem description\n")
		sb.WriteString("2. Quantified metric (specific numeric value)\n")
		sb.WriteString("3. Execution method\n")
		sb.WriteString("4. Expected improvement (percentage)\n\n")
		sb.WriteString("Format: Rule [N]: [Description] | Metric: [Specific Value] | Method: [How to Execute] | Expected Improvement: [Percentage]\n")
	}

	return sb.String()
}

// parseEvolvedRules extracts quantified rules from LLM output
func (fg *FeedbackGenerator) parseEvolvedRules(evolvedText string) []string {
	var rules []string

	// Parse lines matching the rule format
	lines := strings.Split(evolvedText, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for lines starting with "Rule" or "规则"
		if strings.HasPrefix(strings.ToLower(line), "rule ") || strings.HasPrefix(line, "规则 ") {
			// Extract the rule content
			if parts := strings.Split(line, ":"); len(parts) > 1 {
				rule := strings.TrimSpace(parts[1])
				if rule != "" {
					rules = append(rules, rule)
				}
			}
		} else if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "•") {
			// Also capture bullet points as rules
			rule := strings.TrimLeft(line, "- •")
			rule = strings.TrimSpace(rule)
			if len(rule) > 20 { // Only significant rules
				rules = append(rules, rule)
			}
		}
	}

	// If parsing found rules, use them; otherwise return empty
	if len(rules) == 0 {
		logger.Infof("[FeedbackGenerator] Failed to parse evolved rules from LLM output, using baseline rules")
	}

	return rules
}

type llmFeedbackResponse struct {
	SuccessPatterns    []TradingPattern `json:"success_patterns"`
	FailurePatterns    []TradingPattern `json:"failure_patterns"`
	KeyInsights        []string         `json:"key_insights"`
	RecommendedActions []string         `json:"recommended_actions"`
	MarketConditions   string           `json:"market_conditions"`
}

// UnmarshalJSON handles flexible JSON unmarshaling for LLM responses that may serialize patterns as strings
func (lr *llmFeedbackResponse) UnmarshalJSON(data []byte) error {
	aux := &struct {
		SuccessPatterns    interface{} `json:"success_patterns"`
		FailurePatterns    interface{} `json:"failure_patterns"`
		KeyInsights        []string    `json:"key_insights"`
		RecommendedActions []string    `json:"recommended_actions"`
		MarketConditions   interface{} `json:"market_conditions"` // Can be string or array
	}{
		KeyInsights:        []string{},
		RecommendedActions: []string{},
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// Handle success_patterns - could be array of objects or string
	if aux.SuccessPatterns != nil {
		switch v := aux.SuccessPatterns.(type) {
		case []interface{}:
			// Already array - try to unmarshal
			if patBytes, err := json.Marshal(v); err == nil {
				_ = json.Unmarshal(patBytes, &lr.SuccessPatterns)
			}
		case string:
			// String case - try to parse as JSON array
			if v != "" && (v[0] == '[' || v[0] == '{') {
				_ = json.Unmarshal([]byte(v), &lr.SuccessPatterns)
			}
			// If it's plain text, create a pattern from it
			if len(lr.SuccessPatterns) == 0 && v != "" {
				lr.SuccessPatterns = []TradingPattern{{
					PatternType:    "llm_success",
					Description:    v,
					Recommendation: "Review this identified pattern",
					Evidence:       []string{},
				}}
			}
		}
	}

	// Handle failure_patterns - could be array of objects or string
	if aux.FailurePatterns != nil {
		switch v := aux.FailurePatterns.(type) {
		case []interface{}:
			// Already array - try to unmarshal
			if patBytes, err := json.Marshal(v); err == nil {
				_ = json.Unmarshal(patBytes, &lr.FailurePatterns)
			}
		case string:
			// String case - try to parse as JSON array
			if v != "" && (v[0] == '[' || v[0] == '{') {
				_ = json.Unmarshal([]byte(v), &lr.FailurePatterns)
			}
			// If it's plain text, create a pattern from it
			if len(lr.FailurePatterns) == 0 && v != "" {
				lr.FailurePatterns = []TradingPattern{{
					PatternType:    "llm_failure",
					Description:    v,
					Recommendation: "Address this failure pattern",
					Evidence:       []string{},
				}}
			}
		}
	}

	// Handle key_insights
	if len(aux.KeyInsights) > 0 {
		lr.KeyInsights = aux.KeyInsights
	}

	// Handle recommended_actions
	if len(aux.RecommendedActions) > 0 {
		lr.RecommendedActions = aux.RecommendedActions
	}

	// Handle market_conditions - can be string or array
	if aux.MarketConditions != nil {
		switch v := aux.MarketConditions.(type) {
		case string:
			lr.MarketConditions = v
		case []interface{}:
			// Join array elements into comma-separated string
			conditions := make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					conditions = append(conditions, str)
				}
			}
			lr.MarketConditions = strings.Join(conditions, ", ")
		default:
			lr.MarketConditions = ""
		}
	}

	return nil
}

func (fg *FeedbackGenerator) shouldUseLLMAnalysis(force bool) bool {
	if fg.AIClient == nil {
		return false
	}
	if force {
		return true
	}
	return fg.config.EnableLLMPatterns || fg.config.EnableLLMInsights
}

func (fg *FeedbackGenerator) applyLLMFullAnalysis(analysis *FeedbackAnalysis, outcomes []DecisionOutcome, force bool) bool {
	if !fg.shouldUseLLMAnalysis(force) {
		return false
	}
	prompt := fg.buildLLMFullAnalysisPrompt(analysis, outcomes)
	systemPrompt := "You are a trading performance analyst. Return ONLY valid JSON without markdown fences."
	userPrompt := "Analyze historical decisions, outcomes, and microstructure data. Output JSON with success_patterns, failure_patterns, key_insights, recommended_actions, market_conditions."

	respText, err := fg.AIClient.CallWithMessages(systemPrompt, userPrompt+"\n\n"+prompt)
	if err != nil {
		logger.Infof("[FeedbackGenerator] LLM full-analysis failed: %v", err)
		return false
	}

	parsed, err := fg.parseLLMEnhancementResponse(respText)
	if err != nil {
		logger.Infof("[FeedbackGenerator] LLM full-analysis parse failed: %v", err)
		return false
	}

	analysis.SuccessPatterns = sanitizeLLMPatterns(parsed.SuccessPatterns, "llm_success")
	analysis.FailurePatterns = sanitizeLLMPatterns(parsed.FailurePatterns, "llm_failure")
	analysis.KeyInsights = append([]string(nil), parsed.KeyInsights...)
	analysis.RecommendedActions = append([]string(nil), parsed.RecommendedActions...)
	if parsed.MarketConditions != "" {
		analysis.MarketConditions = parsed.MarketConditions
	}

	return true
}

func (fg *FeedbackGenerator) applyLLMEnhancements(analysis *FeedbackAnalysis) bool {
	if fg.AIClient == nil {
		return false
	}
	if !fg.config.EnableLLMPatterns && !fg.config.EnableLLMInsights {
		return false
	}

	prompt := fg.buildLLMEnhancementPrompt(analysis)
	systemPrompt := "You are a trading performance analyst. Return ONLY valid JSON without markdown fences."
	userPrompt := "Analyze the data and return JSON with success_patterns, failure_patterns, key_insights, recommended_actions, market_conditions."

	respText, err := fg.AIClient.CallWithMessages(systemPrompt, userPrompt+"\n\n"+prompt)
	if err != nil {
		logger.Infof("[FeedbackGenerator] LLM enhancement failed: %v", err)
		return false
	}

	parsed, err := fg.parseLLMEnhancementResponse(respText)
	if err != nil {
		logger.Infof("[FeedbackGenerator] LLM enhancement parse failed: %v", err)
		return false
	}

	if fg.config.EnableLLMPatterns {
		analysis.SuccessPatterns = append(analysis.SuccessPatterns, sanitizeLLMPatterns(parsed.SuccessPatterns, "llm_success")...)
		analysis.FailurePatterns = append(analysis.FailurePatterns, sanitizeLLMPatterns(parsed.FailurePatterns, "llm_failure")...)
	}

	if fg.config.EnableLLMInsights {
		if len(parsed.KeyInsights) > 0 {
			analysis.KeyInsights = append(analysis.KeyInsights, parsed.KeyInsights...)
		}
		if len(parsed.RecommendedActions) > 0 {
			analysis.RecommendedActions = append(analysis.RecommendedActions, parsed.RecommendedActions...)
		}
		if parsed.MarketConditions != "" {
			analysis.MarketConditions = parsed.MarketConditions
		}
	}

	return true
}

func sanitizeLLMPatterns(patterns []TradingPattern, fallbackType string) []TradingPattern {
	result := make([]TradingPattern, 0, len(patterns))
	for _, p := range patterns {
		if strings.TrimSpace(p.Description) == "" {
			continue
		}
		if strings.TrimSpace(p.PatternType) == "" {
			p.PatternType = fallbackType
		}
		result = append(result, p)
	}
	return result
}

func (fg *FeedbackGenerator) buildLLMEnhancementPrompt(analysis *FeedbackAnalysis) string {
	var sb strings.Builder
	sb.WriteString("# Metrics\n")
	sb.WriteString(fmt.Sprintf("TotalReturnPct: %.2f\n", analysis.TotalReturnPct))
	sb.WriteString(fmt.Sprintf("WinRate: %.2f\n", analysis.WinRate))
	sb.WriteString(fmt.Sprintf("ProfitFactor: %.2f\n", analysis.ProfitFactor))
	sb.WriteString(fmt.Sprintf("SharpeRatio: %.2f\n", analysis.SharpeRatio))
	sb.WriteString(fmt.Sprintf("MaxDrawdown: %.2f\n", analysis.MaxDrawdown))
	sb.WriteString(fmt.Sprintf("TradesPerHour: %.2f\n", analysis.TradesPerHour))
	sb.WriteString(fmt.Sprintf("AvgHoldTime: %s\n", analysis.AvgHoldTime))

	sb.WriteString("\n# Sample Outcomes\n")
	maxSamples := 20
	count := 0
	for _, o := range analysis.AllOutcomes {
		if count >= maxSamples {
			break
		}
		if o.RecentOrder == nil {
			continue
		}
		order := o.RecentOrder
		sb.WriteString(fmt.Sprintf("%s %s pnlPct=%.2f hold=%s spread=%.4f slippage=%.4f budget=%.4f depth=%.0f fillTime=%d chop=%.2f trend=%.2f regime=%s\n",
			order.Symbol, order.Side, o.RealizedPnLPct, o.HoldDuration, order.EntrySpread, order.EntrySlippage, order.EntrySlippageBudget, order.EntryDepth, order.EntryFillTime, order.ChopScore, order.TrendStrength, order.MarketRegime))
		appendLLMExtraOrderFields(&sb, order)
		count++
	}

	return sb.String()
}

func (fg *FeedbackGenerator) buildLLMFullAnalysisPrompt(analysis *FeedbackAnalysis, outcomes []DecisionOutcome) string {
	var sb strings.Builder
	sb.WriteString("# Metrics\n")
	sb.WriteString(fmt.Sprintf("TotalReturnPct: %.2f\n", analysis.TotalReturnPct))
	sb.WriteString(fmt.Sprintf("WinRate: %.2f\n", analysis.WinRate))
	sb.WriteString(fmt.Sprintf("ProfitFactor: %.2f\n", analysis.ProfitFactor))
	sb.WriteString(fmt.Sprintf("SharpeRatio: %.2f\n", analysis.SharpeRatio))
	sb.WriteString(fmt.Sprintf("MaxDrawdown: %.2f\n", analysis.MaxDrawdown))
	sb.WriteString(fmt.Sprintf("TradesPerHour: %.2f\n", analysis.TradesPerHour))
	sb.WriteString(fmt.Sprintf("AvgHoldTime: %s\n", analysis.AvgHoldTime))

	sb.WriteString("\n# Decisions & Outcomes (sample)\n")
	maxSamples := 40
	count := 0
	for _, o := range outcomes {
		if count >= maxSamples {
			break
		}
		order := o.RecentOrder
		sb.WriteString(fmt.Sprintf("%s %s pnlPct=%.2f hold=%s lev=%d size=%.2f conf=%d reasoning=%q\n",
			o.Symbol, o.Action, o.RealizedPnLPct, o.HoldDuration, o.Leverage, o.PositionSize, o.Confidence, truncate(o.Reasoning, 120)))
		if order != nil {
			sb.WriteString(fmt.Sprintf("  micro: spread=%.4f depth=%.0f slip=%.4f budget=%.4f fill=%dms trend=%.2f chop=%.2f regime=%s vol=%s\n",
				order.EntrySpread, order.EntryDepth, order.EntrySlippage, order.EntrySlippageBudget, order.EntryFillTime, order.TrendStrength, order.ChopScore, order.MarketRegime, order.VolatilityRegime))
			appendLLMExtraOrderFields(&sb, order)
		}
		count++
	}

	sb.WriteString("\n# Task\n")
	sb.WriteString("Derive success and failure patterns from outcomes and microstructure. Provide insights and recommendations for next trades.\n")
	sb.WriteString("For market_conditions: Analyze the trading data and describe the market regime characteristics (trend, volatility, liquidity) in 30-80 chars.\n")
	sb.WriteString("Even with 10-20 trades, you have enough data to identify patterns. Do NOT return 'insufficient data' messages.\n")
	sb.WriteString("Only return empty string if literally no trades/outcomes are provided above.\n")
	sb.WriteString("Output JSON only with fields: success_patterns, failure_patterns, key_insights, recommended_actions, market_conditions.\n")

	return sb.String()
}

func (fg *FeedbackGenerator) parseLLMEnhancementResponse(text string) (*llmFeedbackResponse, error) {
	clean := stripJSONFences(text)

	// Try to fix common JSON issues
	clean = fixCommonJSONIssues(clean)

	var resp llmFeedbackResponse
	if err := json.Unmarshal([]byte(clean), &resp); err != nil {
		// Log the cleaned JSON for debugging
		logger.Infof("[FeedbackGenerator] Failed to parse LLM response. Cleaned JSON: %s", clean)
		return nil, err
	}
	return &resp, nil
}

func stripJSONFences(text string) string {
	clean := strings.TrimSpace(text)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	return strings.TrimSpace(clean)
}

func fixCommonJSONIssues(text string) string {
	// Remove trailing commas before closing braces/brackets
	// Pattern: ,\s*} or ,\s*]
	text = regexp.MustCompile(`,(\s*[}\]])`).ReplaceAllString(text, "$1")

	// Remove thousands separators in numbers (e.g., "1,000" -> "1000", "69,500-69,800" -> "69500-69800")
	// Apply repeatedly to handle all comma groups (e.g., "1,000,000" needs 2 passes)
	for {
		before := text
		text = regexp.MustCompile(`(\d),(\d{3})`).ReplaceAllString(text, "${1}${2}")
		if before == text {
			break
		}
	}

	// Fix double colons (e.g., "key":: "value" -> "key": "value")
	text = regexp.MustCompile(`:+`).ReplaceAllString(text, ":")

	// Remove colons after closing brackets/braces (e.g., ]: -> ])
	text = regexp.MustCompile(`([}\]]):`).ReplaceAllString(text, "$1")

	// Remove any control characters that might break JSON parsing
	text = strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, text)

	return text
}

func appendLLMExtraOrderFields(sb *strings.Builder, order *decision.RecentOrder) {
	if order == nil || sb == nil {
		return
	}

	// Correlation & Risk Book
	if order.CorrelationToBTC != 0 {
		fmt.Fprintf(sb, "    corr_btc=%.2f", order.CorrelationToBTC)
	}
	if order.PortfolioCorrelation != 0 {
		fmt.Fprintf(sb, " port_corr=%.2f", order.PortfolioCorrelation)
	}
	if order.TimeOfDay >= 0 {
		fmt.Fprintf(sb, " tod=%d", order.TimeOfDay)
	}
	if order.EventProximity != "" && order.EventProximity != "none" {
		sb.WriteString(fmt.Sprintf(" event=%s", order.EventProximity))
	}

	// Excursion Metrics
	if order.MaxFavorableExcursion != 0 {
		sb.WriteString(fmt.Sprintf(" mfe=%.2f", order.MaxFavorableExcursion))
	}
	if order.MaxAdverseExcursion != 0 {
		sb.WriteString(fmt.Sprintf(" mae=%.2f", order.MaxAdverseExcursion))
	}
	if order.GiveBackFromPeak != 0 {
		sb.WriteString(fmt.Sprintf(" giveback=%.2f", order.GiveBackFromPeak))
	}

	// Carry & Funding
	if order.FundingAccrued != 0 {
		sb.WriteString(fmt.Sprintf(" funding=%.4f", order.FundingAccrued))
	}
	if order.BorrowCostAccrued != 0 {
		sb.WriteString(fmt.Sprintf(" borrow=%.4f", order.BorrowCostAccrued))
	}

	// Execution Quality
	if order.FillQuality != 0 {
		sb.WriteString(fmt.Sprintf(" fillq=%.2f", order.FillQuality))
	}
	if order.SlippageVsVWAP != 0 {
		sb.WriteString(fmt.Sprintf(" slip_vwap=%.2f", order.SlippageVsVWAP))
	}
	if order.OrderReject {
		sb.WriteString(" reject=true")
	}
	if order.PartialFill {
		sb.WriteString(" partial=true")
	}

	sb.WriteString("\n")
}

func (fg *FeedbackGenerator) detectMicrostructureSuccessPatterns(outcomes []DecisionOutcome) []TradingPattern {
	patterns := make([]TradingPattern, 0)
	var spreads []float64
	var fillTimes []int64
	for _, o := range outcomes {
		if o.RecentOrder == nil || o.RecentOrder.EntrySpread <= 0 {
			continue
		}
		spreads = append(spreads, o.RecentOrder.EntrySpread)
		if o.RecentOrder.EntryFillTime > 0 {
			fillTimes = append(fillTimes, o.RecentOrder.EntryFillTime)
		}
	}
	medianSpread := medianFloat(spreads)
	medianFill := medianInt64(fillTimes)

	tightSpreadWins := 0
	lowSlippageWins := 0
	fastFillWins := 0
	evidence := make([]string, 0)
	for _, o := range outcomes {
		if !o.Success || o.RecentOrder == nil {
			continue
		}
		order := o.RecentOrder
		if medianSpread > 0 && order.EntrySpread > 0 && order.EntrySpread <= medianSpread*0.7 {
			tightSpreadWins++
			evidence = append(evidence, fmt.Sprintf("%s tight spread %.4f", order.Symbol, order.EntrySpread))
		}
		if order.EntrySlippageBudget > 0 && order.EntrySlippage > 0 && order.EntrySlippage <= order.EntrySlippageBudget*0.6 {
			lowSlippageWins++
			if len(evidence) < 5 {
				evidence = append(evidence, fmt.Sprintf("%s low slippage %.4f", order.Symbol, order.EntrySlippage))
			}
		}
		if medianFill > 0 && order.EntryFillTime > 0 && order.EntryFillTime <= int64(float64(medianFill)*0.7) {
			fastFillWins++
			if len(evidence) < 5 {
				evidence = append(evidence, fmt.Sprintf("%s fast fill %dms", order.Symbol, order.EntryFillTime))
			}
		}
	}

	if tightSpreadWins >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "micro_tight_spread_wins",
			Frequency:      tightSpreadWins,
			AvgPnL:         0,
			AvgPnLPct:      0,
			Description:    "Wins cluster around tight entry spreads",
			Evidence:       evidence,
			Recommendation: "Prioritize entries with tight spreads and adequate liquidity",
		})
	}
	if lowSlippageWins >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "micro_low_slippage_wins",
			Frequency:      lowSlippageWins,
			AvgPnL:         0,
			AvgPnLPct:      0,
			Description:    "Wins occur when slippage stays well below budget",
			Evidence:       evidence,
			Recommendation: "Avoid entries when expected slippage exceeds budget",
		})
	}
	if fastFillWins >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "micro_fast_fill_wins",
			Frequency:      fastFillWins,
			AvgPnL:         0,
			AvgPnLPct:      0,
			Description:    "Faster fills correlate with winning trades",
			Evidence:       evidence,
			Recommendation: "Favor venues/conditions with faster fills",
		})
	}

	return patterns
}

func (fg *FeedbackGenerator) detectMicrostructureFailurePatterns(outcomes []DecisionOutcome) []TradingPattern {
	patterns := make([]TradingPattern, 0)
	var spreads []float64
	var fillTimes []int64
	for _, o := range outcomes {
		if o.RecentOrder == nil || o.RecentOrder.EntrySpread <= 0 {
			continue
		}
		spreads = append(spreads, o.RecentOrder.EntrySpread)
		if o.RecentOrder.EntryFillTime > 0 {
			fillTimes = append(fillTimes, o.RecentOrder.EntryFillTime)
		}
	}
	medianSpread := medianFloat(spreads)
	medianFill := medianInt64(fillTimes)

	highSlippageLosses := 0
	wideSpreadLosses := 0
	thinDepthLosses := 0
	slowFillLosses := 0
	trendMismatchLosses := 0
	evidence := make([]string, 0)
	for _, o := range outcomes {
		if o.Success || o.RecentOrder == nil {
			continue
		}
		order := o.RecentOrder
		if order.EntrySlippageBudget > 0 && order.EntrySlippage > order.EntrySlippageBudget*1.2 {
			highSlippageLosses++
			if len(evidence) < 5 {
				evidence = append(evidence, fmt.Sprintf("%s slippage %.4f", order.Symbol, order.EntrySlippage))
			}
		}
		if medianSpread > 0 && order.EntrySpread > medianSpread*1.5 {
			wideSpreadLosses++
			if len(evidence) < 5 {
				evidence = append(evidence, fmt.Sprintf("%s wide spread %.4f", order.Symbol, order.EntrySpread))
			}
		}
		if order.EntryDepth > 0 && o.PositionSize > 0 && order.EntryDepth < o.PositionSize*3 {
			thinDepthLosses++
			if len(evidence) < 5 {
				evidence = append(evidence, fmt.Sprintf("%s thin depth %.0f", order.Symbol, order.EntryDepth))
			}
		}
		if medianFill > 0 && order.EntryFillTime > int64(float64(medianFill)*1.5) {
			slowFillLosses++
			if len(evidence) < 5 {
				evidence = append(evidence, fmt.Sprintf("%s slow fill %dms", order.Symbol, order.EntryFillTime))
			}
		}
		if order.TrendStrength != 0 {
			if (order.Side == "long" && order.TrendStrength < -0.2) || (order.Side == "short" && order.TrendStrength > 0.2) {
				trendMismatchLosses++
				if len(evidence) < 5 {
					evidence = append(evidence, fmt.Sprintf("%s trend mismatch %.2f", order.Symbol, order.TrendStrength))
				}
			}
		}
	}

	if highSlippageLosses >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "micro_high_slippage_losses",
			Frequency:      highSlippageLosses,
			Description:    "Losses cluster when slippage exceeds budget",
			Evidence:       evidence,
			Recommendation: "Avoid entries when expected slippage is above budget",
		})
	}
	if wideSpreadLosses >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "micro_wide_spread_losses",
			Frequency:      wideSpreadLosses,
			Description:    "Losses occur with wide entry spreads",
			Evidence:       evidence,
			Recommendation: "Filter trades with wide spreads or wait for liquidity",
		})
	}
	if thinDepthLosses >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "micro_thin_depth_losses",
			Frequency:      thinDepthLosses,
			Description:    "Losses when depth is thin relative to position size",
			Evidence:       evidence,
			Recommendation: "Reduce size or avoid trades in thin order books",
		})
	}
	if slowFillLosses >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "micro_slow_fill_losses",
			Frequency:      slowFillLosses,
			Description:    "Losses correlate with slow fills",
			Evidence:       evidence,
			Recommendation: "Avoid orders during low liquidity windows",
		})
	}
	if trendMismatchLosses >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "micro_trend_mismatch_losses",
			Frequency:      trendMismatchLosses,
			Description:    "Losses when trading against trend strength",
			Evidence:       evidence,
			Recommendation: "Align trades with dominant trend direction",
		})
	}

	return patterns
}

func medianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

func medianInt64(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

// GenerateFeedback analyzes recent performance and generates actionable insights
func (fg *FeedbackGenerator) GenerateFeedback() (*FeedbackAnalysis, error) {
	// Load trade events and decision records
	events, err := LoadTradeEvents(fg.runID)
	if err != nil {
		return nil, fmt.Errorf("failed to load trade events: %w", err)
	}

	if len(events) < fg.config.MinDecisionsForFeedback {
		logger.Infof("Not enough trades (%d) for feedback analysis (need %d)", len(events), fg.config.MinDecisionsForFeedback)
		return nil, nil
	}

	// Get backtest metrics
	cfg := &BacktestConfig{
		InitialBalance: fg.initialBalance,
	}

	state, err := fg.getCurrentState()
	if err != nil {
		logger.Infof("Warning: could not load current state: %v", err)
		state = nil
	}

	metrics, err := CalculateMetrics(fg.runID, cfg, state)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate metrics: %w", err)
	}

	// Analyze closed positions (match open and close events)
	closedPositions := fg.extractClosedPositions(events)

	// Generate decision outcomes
	outcomes := fg.createDecisionOutcomes(closedPositions)

	// Create analysis
	analysis := &FeedbackAnalysis{
		AnalysisPeriod:   fmt.Sprintf("Last %d trades", len(events)),
		DecisionsCovered: len(outcomes), // Use outcomes count
		AllOutcomes:      outcomes,      // Store all outcomes
		TotalReturnPct:   metrics.TotalReturnPct,
		WinRate:          metrics.WinRate,
		ProfitFactor:     metrics.ProfitFactor,
		SharpeRatio:      metrics.SharpeRatio,
		MaxDrawdown:      metrics.MaxDrawdownPct,
		RegimeAnalysis:   make(map[string]float64),
	}

	if len(events) > 0 {
		analysis.StartTime = time.UnixMilli(events[0].Timestamp)
		analysis.EndTime = time.UnixMilli(events[len(events)-1].Timestamp)
	}

	// Calculate and store metrics
	analysis.TradesPerHour = fg.calculateTradesPerHour(outcomes)
	analysis.AvgHoldTime = fg.calculateAvgHoldTime(outcomes)
	analysis.ChecklistCompliance = fg.calculateChecklistCompliance(outcomes)

	if len(outcomes) > 0 {
		analysis.StartTime = outcomes[0].Timestamp
		analysis.EndTime = outcomes[len(outcomes)-1].Timestamp
	}

	// Identify patterns
	llmApplied := fg.applyLLMFullAnalysis(analysis, outcomes, false)
	if !llmApplied {
		analysis.SuccessPatterns = fg.identifySuccessPatterns(outcomes, metrics)
		analysis.FailurePatterns = fg.identifyFailurePatterns(outcomes, metrics)
	} else {
		if len(analysis.SuccessPatterns) == 0 {
			analysis.SuccessPatterns = fg.identifySuccessPatterns(outcomes, metrics)
		}
		if len(analysis.FailurePatterns) == 0 {
			analysis.FailurePatterns = fg.identifyFailurePatterns(outcomes, metrics)
		}
	}

	// Extract top trades
	analysis.TopWinningTrades = fg.getTopTrades(outcomes, true, fg.config.TopTradesCount)
	analysis.TopLosingTrades = fg.getTopTrades(outcomes, false, fg.config.TopTradesCount)

	// Generate insights
	if !llmApplied || len(analysis.KeyInsights) == 0 {
		analysis.KeyInsights = fg.generateKeyInsights(metrics, outcomes, analysis)
	}
	if !llmApplied || len(analysis.RecommendedActions) == 0 {
		analysis.RecommendedActions = fg.generateRecommendedActions(analysis)
	}

	// Market regime analysis
	// Filter out unhelpful LLM responses like "Insufficient data..."
	if !llmApplied || analysis.MarketConditions == "" ||
		strings.Contains(strings.ToLower(analysis.MarketConditions), "insufficient") ||
		strings.Contains(strings.ToLower(analysis.MarketConditions), "zero") ||
		len(analysis.MarketConditions) < 20 {
		analysis.MarketConditions = fg.analyzeMarketConditions(metrics, outcomes)
	}

	return analysis, nil
}

func (fg *FeedbackGenerator) getCurrentState() (*BacktestState, error) {
	ckpt, err := LoadCheckpoint(fg.runID)
	if err != nil {
		return nil, err
	}

	state := &BacktestState{
		Cash:           ckpt.Cash,
		Equity:         ckpt.Equity,
		UnrealizedPnL:  ckpt.UnrealizedPnL,
		RealizedPnL:    ckpt.RealizedPnL,
		MaxEquity:      ckpt.MaxEquity,
		MinEquity:      ckpt.MinEquity,
		MaxDrawdownPct: ckpt.MaxDrawdownPct,
		Liquidated:     ckpt.Liquidated,
	}

	return state, nil
}

// ClosedPosition represents a completed trade with entry and exit details
type ClosedPosition struct {
	Symbol      string
	Side        string
	EntryTime   time.Time
	ExitTime    time.Time
	EntryPrice  float64
	ExitPrice   float64
	Quantity    float64
	Leverage    int
	RealizedPnL float64
	EntryEvent  TradeEvent
	ExitEvent   TradeEvent

	// Market data snapshots for microstructure analysis
	EntryMarketData *market.Data
	ExitMarketData  *market.Data
}

// extractClosedPositions matches open and close events to create complete trade records
func (fg *FeedbackGenerator) extractClosedPositions(events []TradeEvent) []ClosedPosition {
	var closed []ClosedPosition
	openPositions := make(map[string]TradeEvent) // key: symbol:side

	for _, event := range events {
		if event.LiquidationFlag {
			continue // Skip liquidation events
		}

		key := fmt.Sprintf("%s:%s", event.Symbol, event.Side)

		switch event.Action {
		case "open":
			openPositions[key] = event
		case "close":
			if openEvent, exists := openPositions[key]; exists {
				closed = append(closed, ClosedPosition{
					Symbol:      event.Symbol,
					Side:        event.Side,
					EntryTime:   time.UnixMilli(openEvent.Timestamp),
					ExitTime:    time.UnixMilli(event.Timestamp),
					EntryPrice:  openEvent.Price,
					ExitPrice:   event.Price,
					Quantity:    event.Quantity,
					Leverage:    openEvent.Leverage,
					RealizedPnL: event.RealizedPnL,
					EntryEvent:  openEvent,
					ExitEvent:   event,
				})
				delete(openPositions, key)
			}
		}
	}

	return closed
}

// buildRecentOrderFromPosition converts a ClosedPosition into decision.RecentOrder with microstructure data
// This is the bridge between backtest execution and Trade Failure V2 analysis
func buildRecentOrderFromPosition(pos ClosedPosition) *decision.RecentOrder {
	holdDuration := pos.ExitTime.Sub(pos.EntryTime)
	positionValue := pos.EntryPrice * pos.Quantity

	order := &decision.RecentOrder{
		Symbol:         pos.Symbol,
		Side:           pos.Side,
		EntryPrice:     pos.EntryPrice,
		ExitPrice:      pos.ExitPrice,
		RealizedPnL:    pos.RealizedPnL,
		EntryTime:      pos.EntryTime.Format(time.RFC3339),
		ExitTime:       pos.ExitTime.Format(time.RFC3339),
		HoldDuration:   formatDuration(holdDuration),
		Leverage:       pos.Leverage,
		TimeOfDay:      pos.EntryTime.UTC().Hour(),
		EventProximity: "none",
	}

	// Calculate PnL percentage
	if pos.EntryPrice > 0 {
		if pos.Side == "long" {
			order.PnLPct = ((pos.ExitPrice - pos.EntryPrice) / pos.EntryPrice) * 100 * float64(pos.Leverage)
		} else {
			order.PnLPct = ((pos.EntryPrice - pos.ExitPrice) / pos.EntryPrice) * 100 * float64(pos.Leverage)
		}
	}

	// Populate microstructure data from entry event (use actual captured values)
	order.EntrySpread = pos.EntryEvent.Spread
	order.EntryDepth = pos.EntryEvent.Depth
	if pos.EntryPrice > 0 {
		order.EntrySlippage = math.Abs(pos.EntryEvent.Slippage / pos.EntryPrice)
	}
	order.EntryArrivalPrice = pos.EntryPrice - pos.EntryEvent.Slippage
	order.EntryFillPrice = pos.EntryPrice
	order.EntrySlippageBudget = pos.EntryEvent.SlippageBudget
	order.SignalTime = pos.EntryEvent.SignalTime
	order.EntryFillTime = pos.EntryEvent.FillTime

	// Populate microstructure data from exit event (use actual captured values)
	order.ExitSpread = pos.ExitEvent.Spread
	order.ExitDepth = pos.ExitEvent.Depth
	if pos.ExitPrice > 0 {
		order.ExitSlippage = math.Abs(pos.ExitEvent.Slippage / pos.ExitPrice)
	}

	// Populate market data from entry snapshot
	if pos.EntryMarketData != nil {
		order.ATRAtEntry = calculateATRFromSeries(pos.EntryMarketData)
		order.TrendStrength = extractTrendStrength(pos.EntryMarketData)
		order.ChopScore = extractChopScore(pos.EntryMarketData)
		order.MarketRegime = extractMarketRegime(pos.EntryMarketData)
		order.VolatilityRegime = extractVolatilityRegime(pos.EntryMarketData)
		order.VolumeAtEntry = extractVolumeRatio(pos.EntryMarketData)
		order.OIDeltaAtEntry = extractOIDelta(pos.EntryMarketData)
		order.SlippageVsVWAP = computeSlippageVsVWAP(pos.EntryMarketData, order.EntryFillPrice)
		order.FundingAccrued = estimateFundingAccrued(pos.EntryMarketData.FundingRate, positionValue, holdDuration, pos.Side)
	}

	// Calculate deltas during trade (entry vs exit market data)
	if pos.EntryMarketData != nil && pos.ExitMarketData != nil {
		order.VolumeDeltaDuringTrade = extractVolumeRatio(pos.ExitMarketData) - extractVolumeRatio(pos.EntryMarketData)
		order.OIDeltaDuringTrade = extractOIDelta(pos.ExitMarketData) - extractOIDelta(pos.EntryMarketData)
	}

	// Calculate stop distance vs ATR if we have ATR and entry spread
	// Use entry spread + 2*ATR as reasonable stop estimate (ATR-based risk management)
	if order.ATRAtEntry > 0 && pos.EntryPrice > 0 {
		atrPct := order.ATRAtEntry / pos.EntryPrice
		// Stop distance = spread + 2*ATR (standard risk management)
		stopDistance := order.EntrySpread + (atrPct * 2.0)
		order.StopDistance = stopDistance
		order.StopDistanceVsATR = stopDistance / atrPct
	}

	// Populate excursion metrics from TradeEvent (convert USD to % of position value)
	if positionValue > 0 {
		order.MaxFavorableExcursion = (pos.EntryEvent.MaxFavorableExcursion / positionValue) * 100
		order.MaxAdverseExcursion = (pos.ExitEvent.MaxAdverseExcursion / positionValue) * 100
	}

	// Calculate giveback: how much profit was left on the table after peak
	// GiveBack = MFE% - RealizedPnL%
	if order.MaxFavorableExcursion > 0 && pos.EntryPrice > 0 {
		realizedPct := (pos.RealizedPnL / positionValue) * 100
		order.GiveBackFromPeak = order.MaxFavorableExcursion - realizedPct
	}

	// Execution quality
	order.FillQuality = estimateFillQuality(order)

	return order
}

// Helper functions for market data extraction

func calculateATRFromSeries(data *market.Data) float64 {
	// Try from intraday data first
	if data.IntradaySeries != nil && data.IntradaySeries.ATR14 > 0 {
		return data.IntradaySeries.ATR14
	}
	// Try from longer-term data
	if data.LongerTermContext != nil && data.LongerTermContext.ATR14 > 0 {
		return data.LongerTermContext.ATR14
	}
	// Try from timeframe data (1h or 4h)
	if data.TimeframeData != nil {
		if tfData, ok := data.TimeframeData["1h"]; ok && tfData.ATR14 > 0 {
			return tfData.ATR14
		}
		if tfData, ok := data.TimeframeData["4h"]; ok && tfData.ATR14 > 0 {
			return tfData.ATR14
		}
	}
	return 0.0
}

func extractTrendStrength(data *market.Data) float64 {
	// Calculate trend strength from EMA20 vs EMA50
	if data.LongerTermContext != nil && data.LongerTermContext.EMA20 > 0 && data.LongerTermContext.EMA50 > 0 {
		return (data.LongerTermContext.EMA20 - data.LongerTermContext.EMA50) / data.LongerTermContext.EMA50
	}
	return 0.0
}

func extractChopScore(data *market.Data) float64 {
	// Estimate choppiness from ATR vs price range
	atr := calculateATRFromSeries(data)
	if atr > 0 && data.CurrentPrice > 0 {
		// ATR as % of price - lower means less choppy
		return math.Min(atr/data.CurrentPrice*10, 1.0) // scale to 0-1
	}
	return 0.5 // neutral default
}

func extractMarketRegime(data *market.Data) string {
	// Determine regime from trend strength and chop
	trendStrength := extractTrendStrength(data)
	chopScore := extractChopScore(data)

	if chopScore > 0.6 {
		return "sideways"
	} else if math.Abs(trendStrength) > 0.3 {
		return "trending"
	}
	return "volatile"
}

func extractVolatilityRegime(data *market.Data) string {
	atr := calculateATRFromSeries(data)
	if atr == 0 {
		return "normal"
	}

	// Classify based on ATR relative to price
	atrPct := atr / data.CurrentPrice
	if atrPct < 0.02 {
		return "low"
	} else if atrPct > 0.05 {
		return "high"
	}
	return "normal"
}

func extractVolumeRatio(data *market.Data) float64 {
	// Get latest volume as ratio of baseline (assume 1.0 = baseline)
	if data.IntradaySeries != nil && len(data.IntradaySeries.Volume) > 0 {
		latestVol := data.IntradaySeries.Volume[len(data.IntradaySeries.Volume)-1]
		// Estimate baseline as average of recent volumes
		if len(data.IntradaySeries.Volume) > 10 {
			var sum float64
			start := len(data.IntradaySeries.Volume) - 10
			for i := start; i < len(data.IntradaySeries.Volume)-1; i++ {
				sum += data.IntradaySeries.Volume[i]
			}
			baseline := sum / 9.0
			if baseline > 0 {
				return latestVol / baseline
			}
		}
		return 1.0
	}
	return 1.0
}

func extractOIDelta(data *market.Data) float64 {
	// Get OI delta if available
	if data.OpenInterest != nil && data.OpenInterest.Latest > 0 && data.OpenInterest.Average > 0 {
		return (data.OpenInterest.Latest - data.OpenInterest.Average) / data.OpenInterest.Average
	}
	return 0.0
}

func computeSlippageVsVWAP(data *market.Data, fillPrice float64) float64 {
	if data == nil || fillPrice <= 0 {
		return 0
	}
	if vwap := estimateEntryVWAP(data); vwap > 0 {
		return (fillPrice - vwap) / vwap * 100
	}
	return 0
}

func estimateEntryVWAP(data *market.Data) float64 {
	if data == nil || len(data.TimeframeData) == 0 {
		return 0
	}
	for _, tfData := range data.TimeframeData {
		if tfData == nil || len(tfData.Klines) == 0 {
			continue
		}
		last := tfData.Klines[len(tfData.Klines)-1]
		return barVWAPFromKlineBar(last)
	}
	return 0
}

// Add this helper function to handle market.KlineBar input
func barVWAPFromKlineBar(bar market.KlineBar) float64 {
	// VWAP = (High + Low + Close) / 3
	return (bar.High + bar.Low + bar.Close) / 3
}

func estimateFundingAccrued(fundingRate float64, positionValue float64, holdDuration time.Duration, side string) float64 {
	if fundingRate == 0 || positionValue <= 0 || holdDuration <= 0 {
		return 0
	}
	// fundingRate is typically per 8h; scale by holding time
	hours := holdDuration.Hours()
	accrued := positionValue * fundingRate * (hours / 8.0)
	if strings.EqualFold(side, "short") {
		accrued = -accrued
	}
	return accrued
}

func estimateFillQuality(order *decision.RecentOrder) float64 {
	if order == nil {
		return 0
	}
	quality := 1.0
	if order.EntrySlippageBudget > 0 && order.EntrySlippage > 0 {
		ratio := order.EntrySlippage / order.EntrySlippageBudget
		quality -= math.Min(1, ratio) * 0.6
	}
	if order.EntryFillTime > 0 {
		penalty := math.Min(1, float64(order.EntryFillTime)/1500.0) * 0.2
		quality -= penalty
	}
	if order.EntrySpread > 0 {
		penalty := math.Min(1, order.EntrySpread/0.003) * 0.2
		quality -= penalty
	}
	if quality < 0 {
		return 0
	}
	if quality > 1 {
		return 1
	}
	return quality
}

// createDecisionOutcomes converts closed positions into decision outcomes with analysis
func (fg *FeedbackGenerator) createDecisionOutcomes(closedPositions []ClosedPosition) []DecisionOutcome {
	outcomes := make([]DecisionOutcome, 0, len(closedPositions))

	// Load decision records to fetch original reasoning and confidence
	decisionMap := fg.loadDecisionRecordsMap()

	for _, pos := range closedPositions {
		pnlPct := 0.0
		if pos.EntryPrice > 0 {
			if pos.Side == "long" {
				pnlPct = ((pos.ExitPrice - pos.EntryPrice) / pos.EntryPrice) * 100 * float64(pos.Leverage)
			} else {
				pnlPct = ((pos.EntryPrice - pos.ExitPrice) / pos.EntryPrice) * 100 * float64(pos.Leverage)
			}
		}
		holdDuration := pos.ExitTime.Sub(pos.EntryTime)

		isTradeSuccess := pnlPct >= (1.0 * float64(pos.Leverage))
		// Try to find the original decision record for this trade
		reasoning := ""
		confidence := 0
		if decRecord := fg.findDecisionForTrade(decisionMap, pos); decRecord != nil {
			reasoning = fg.extractReasoningFromDecision(decRecord, pos.Symbol, pos.Side)
			confidence = fg.extractConfidenceFromDecision(decRecord, pos.Symbol)
		}

		outcome := DecisionOutcome{
			Timestamp:      pos.EntryTime,
			Symbol:         pos.Symbol,
			Action:         fmt.Sprintf("open_%s", pos.Side),
			Reasoning:      reasoning,
			Confidence:     confidence,
			EntryPrice:     pos.EntryPrice,
			PositionSize:   pos.EntryPrice * pos.Quantity,
			Leverage:       pos.Leverage,
			ExitPrice:      pos.ExitPrice,
			HoldDuration:   formatDuration(holdDuration),
			RealizedPnL:    pos.RealizedPnL,
			RealizedPnLPct: pnlPct,
			Success:        isTradeSuccess,
			Analysis:       fg.analyzeDecisionOutcome(pos, pnlPct, holdDuration),
			RecentOrder:    buildRecentOrderFromPosition(pos), // Bridge to Trade Failure V2
		}

		outcomes = append(outcomes, outcome)
	}

	return outcomes
}

// loadDecisionRecordsMap loads all decision records into a map keyed by timestamp
func (fg *FeedbackGenerator) loadDecisionRecordsMap() map[int64]*store.DecisionRecord {
	records, err := LoadDecisionRecords(fg.runID, 0, 0) // Load all records (limit=0 means no limit)
	if err != nil {
		logger.Infof("Warning: could not load decision records for feedback: %v", err)
		return make(map[int64]*store.DecisionRecord)
	}

	recordMap := make(map[int64]*store.DecisionRecord)
	for i := range records {
		ts := records[i].Timestamp.UnixMilli()
		recordMap[ts] = records[i] // records[i] is already a pointer
	}
	return recordMap
}

// findDecisionForTrade finds the decision record that led to opening this position
func (fg *FeedbackGenerator) findDecisionForTrade(decisionMap map[int64]*store.DecisionRecord, pos ClosedPosition) *store.DecisionRecord {
	entryTs := pos.EntryTime.UnixMilli()

	// Try exact match first
	if record, ok := decisionMap[entryTs]; ok {
		return record
	}

	// Try within 5 minutes window (decisions might be slightly before trade execution)
	for ts, record := range decisionMap {
		if math.Abs(float64(ts-entryTs)) < 5*60*1000 { // 5 minutes in milliseconds
			return record
		}
	}

	return nil
}

// New detection function
func (fg *FeedbackGenerator) detectOvertrading(outcomes []DecisionOutcome) *TradingPattern {
	if len(outcomes) < 2 {
		return nil
	}

	// Sort by timestamp
	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].Timestamp.Before(outcomes[j].Timestamp)
	})

	tradesByHour := make(map[string]int)
	tradesByMinute := make(map[string]int)

	var totalDuration time.Duration
	var lastTime time.Time

	for i, outcome := range outcomes {
		hourKey := outcome.Timestamp.Format("2006-01-02-15")
		minuteKey := outcome.Timestamp.Format("2006-01-02-15:04")

		tradesByHour[hourKey]++
		tradesByMinute[minuteKey]++

		if i > 0 {
			timeDiff := outcome.Timestamp.Sub(lastTime)
			totalDuration += timeDiff
		}
		lastTime = outcome.Timestamp
	}

	// Calculate metrics
	tradesPerHour := float64(len(outcomes)) / (totalDuration.Hours() + 1)
	avgTimeBetween := totalDuration / time.Duration(len(outcomes)-1)

	// Find problematic patterns
	problematicHours := 0
	maxTradesInHour := 0

	for _, count := range tradesByHour {
		if count > maxTradesInHour {
			maxTradesInHour = count
		}
		if count >= 4 { // 4+ trades per hour is excessive for swing trading
			problematicHours++
		}
	}

	if problematicHours >= 2 || tradesPerHour > 2.0 {
		evidence := []string{
			fmt.Sprintf("Trading frequency: %.1f trades/hour (recommended: 0.2-0.5)", tradesPerHour),
			fmt.Sprintf("Average time between trades: %v", avgTimeBetween.Round(time.Minute)),
			fmt.Sprintf("%d instances of 4+ trades in one hour", problematicHours),
		}

		return &TradingPattern{
			PatternType:    "overtrading_frequency",
			Frequency:      problematicHours,
			AvgPnLPct:      -1.5, // Overtrading typically results in -1% to -3% per trade
			Description:    fmt.Sprintf("Excessive trading frequency: %.1fx higher than optimal", tradesPerHour/0.3),
			Evidence:       evidence,
			Recommendation: "ENFORCE: Max 2 trades/hour, 8 trades/day. Wait minimum 30 minutes between trades unless high-conviction setup.",
		}
	}

	return nil
}

// Add to identifyFailurePatterns
func (fg *FeedbackGenerator) detectContradictoryAnalysis(outcomes []DecisionOutcome) *TradingPattern {
	if len(outcomes) < 3 {
		return nil
	}

	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].Timestamp.Before(outcomes[j].Timestamp)
	})

	contradictoryMoves := 0
	evidence := []string{}

	for i := 1; i < len(outcomes)-1; i++ {
		current := outcomes[i]
		previous := outcomes[i-1]

		// Check for same symbol trades with conflicting directions in short time
		timeDiffCurrentPrev := current.Timestamp.Sub(previous.Timestamp)

		if current.Symbol == previous.Symbol && timeDiffCurrentPrev < 60*time.Minute {
			// Get directions (simplified)
			currentDirection := "unknown"
			previousDirection := "unknown"

			if strings.Contains(strings.ToLower(current.Action), "long") {
				currentDirection = "long"
			} else if strings.Contains(strings.ToLower(current.Action), "short") {
				currentDirection = "short"
			}

			if strings.Contains(strings.ToLower(previous.Action), "long") {
				previousDirection = "long"
			} else if strings.Contains(strings.ToLower(previous.Action), "short") {
				previousDirection = "short"
			}

			// If opposite directions on same symbol within short time
			if currentDirection != "unknown" && previousDirection != "unknown" &&
				currentDirection != previousDirection && timeDiffCurrentPrev < 30*time.Minute {

				contradictoryMoves++
				evidence = append(evidence,
					fmt.Sprintf("%s: %s at %s, then %s at %s (gap: %v)",
						current.Symbol, previousDirection, previous.Timestamp.Format("15:04"),
						currentDirection, current.Timestamp.Format("15:04"),
						timeDiffCurrentPrev.Round(time.Minute)))
			}
		}
	}

	if contradictoryMoves >= 2 {
		return &TradingPattern{
			PatternType:    "contradictory_analysis",
			Frequency:      contradictoryMoves,
			AvgPnLPct:      -2.0,
			Description:    "Frequent directional flip-flopping on same symbols within short timeframes",
			Evidence:       evidence[:min(3, len(evidence))],
			Recommendation: "ENFORCE: Once a position is closed on a symbol, wait minimum 2 hours before re-entering same symbol. Maintain conviction in original analysis.",
		}
	}

	return nil
}

// extractReasoningFromDecision extracts the reasoning for a specific symbol/side from decision record
func (fg *FeedbackGenerator) extractReasoningFromDecision(record *store.DecisionRecord, symbol, side string) string {
	if record == nil {
		return ""
	}

	// Check if there's a decision for this symbol and side
	for _, decision := range record.Decisions {
		if decision.Symbol == symbol {
			// Check if the action matches the side (e.g., "long" matches "open_long" or "close_long")
			actionMatchesSide := false
			if side == "long" && (strings.Contains(strings.ToLower(decision.Action), "long")) {
				actionMatchesSide = true
			} else if side == "short" && (strings.Contains(strings.ToLower(decision.Action), "short")) {
				actionMatchesSide = true
			} else if side == "" {
				actionMatchesSide = true // If no side specified, match any
			}

			if actionMatchesSide && decision.Reasoning != "" {
				return decision.Reasoning
			}
		}
	}

	// Fallback: try to extract from CoT trace
	if record.CoTTrace != "" {
		// Look for reasoning related to this symbol in the CoT trace
		lines := strings.Split(record.CoTTrace, "\n")
		for _, line := range lines {
			if strings.Contains(strings.ToUpper(line), symbol) {
				// Found a line mentioning this symbol, return it (truncated)
				if len(line) > 200 {
					return line[:200] + "..."
				}
				return line
			}
		}
	}

	return ""
}

// extractConfidenceFromDecision extracts the confidence level for a specific symbol from decision record
func (fg *FeedbackGenerator) extractConfidenceFromDecision(record *store.DecisionRecord, symbol string) int {
	if record == nil {
		return 0
	}

	for _, decision := range record.Decisions {
		if decision.Symbol == symbol && decision.Confidence > 0 {
			return decision.Confidence
		}
	}

	return 0
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func (fg *FeedbackGenerator) analyzeDecisionOutcome(pos ClosedPosition, pnlPct float64, holdDuration time.Duration) string {
	var analysis []string

	if pos.RealizedPnL > 0 {
		analysis = append(analysis, fmt.Sprintf("✓ Profitable trade: +%.2f%% (%.2f USDT)", pnlPct, pos.RealizedPnL))

		if holdDuration < 30*time.Minute {
			analysis = append(analysis, "Quick profit capture - good execution")
		} else if holdDuration > 4*time.Hour {
			analysis = append(analysis, "Patient hold paid off - good discipline")
		}

		if pos.Leverage >= 5 {
			analysis = append(analysis, "High leverage used effectively")
		}
	} else {
		analysis = append(analysis, fmt.Sprintf("✗ Loss: %.2f%% (%.2f USDT)", pnlPct, pos.RealizedPnL))

		if math.Abs(pnlPct) > 5 {
			analysis = append(analysis, "⚠️ Large loss - stop-loss may have been too wide")
		}

		if holdDuration < 15*time.Minute {
			analysis = append(analysis, "Premature exit - might have lacked conviction")
		} else if holdDuration > 6*time.Hour {
			analysis = append(analysis, "Held losing position too long - hesitant to cut losses")
		}

		if pos.Leverage >= 5 {
			analysis = append(analysis, "⚠️ High leverage amplified losses")
		}
	}

	return strings.Join(analysis, "; ")
}

// identifySuccessPatterns finds patterns in winning trades
func (fg *FeedbackGenerator) identifySuccessPatterns(outcomes []DecisionOutcome, metrics *Metrics) []TradingPattern {
	var patterns []TradingPattern

	// Validate using metrics context - only identify patterns if we have adequate data
	if len(outcomes) < fg.config.MinDecisionsForFeedback || metrics == nil {
		return patterns
	}

	// Pattern 1: Quick profit-taking
	// Use metrics to determine if this is a reliable strategy in current market
	quickWins := 0
	quickWinsPnL := 0.0
	for _, outcome := range outcomes {
		if outcome.Success && strings.Contains(outcome.HoldDuration, "m") && !strings.Contains(outcome.HoldDuration, "h") {
			duration := parseDurationMinutes(outcome.HoldDuration)
			if duration < 30 {
				quickWins++
				quickWinsPnL += outcome.RealizedPnLPct
			}
		}
	}

	if quickWins >= fg.config.MinPatternFrequency {
		// Use metrics to validate pattern is consistent with overall strategy performance
		if metrics.TotalReturnPct > 0 {
			avgQuickProfit := quickWinsPnL / float64(quickWins)
			patterns = append(patterns, TradingPattern{
				PatternType:    "quick_profit_taking",
				Frequency:      quickWins,
				AvgPnL:         0,
				AvgPnLPct:      avgQuickProfit,
				Description:    fmt.Sprintf("Profitable trades closed within 30 minutes (%.1f%% of winning trades)", float64(quickWins)/float64(len(outcomes))*100),
				Evidence:       []string{fmt.Sprintf("%d trades with avg %.2f%% profit | Overall win rate: %.1f%%", quickWins, avgQuickProfit, metrics.WinRate)},
				Recommendation: "Continue quick profit-taking strategy for scalping opportunities - effective in current market conditions",
			})
		}
	}

	// Pattern 2: Optimal leverage usage
	lowLevWins := 0
	lowLevPnL := 0.0
	highLevWins := 0
	highLevPnL := 0.0

	for _, outcome := range outcomes {
		if outcome.Success {
			if outcome.Leverage <= 3 {
				lowLevWins++
				lowLevPnL += outcome.RealizedPnLPct
			} else if outcome.Leverage >= 5 {
				highLevWins++
				highLevPnL += outcome.RealizedPnLPct
			}
		}
	}

	if lowLevWins >= fg.config.MinPatternFrequency && highLevWins >= fg.config.MinPatternFrequency {
		lowAvg := lowLevPnL / float64(lowLevWins)
		highAvg := highLevPnL / float64(highLevWins)

		if lowAvg > highAvg {
			patterns = append(patterns, TradingPattern{
				PatternType:    "low_leverage_success",
				Frequency:      lowLevWins,
				AvgPnL:         0,
				AvgPnLPct:      lowAvg,
				Description:    "Lower leverage (≤3x) produces better risk-adjusted returns",
				Evidence:       []string{fmt.Sprintf("Low lev: %.2f%% avg, High lev: %.2f%% avg", lowAvg, highAvg)},
				Recommendation: "Favor lower leverage positions for more consistent profits",
			})
		} else {
			patterns = append(patterns, TradingPattern{
				PatternType:    "high_leverage_success",
				Frequency:      highLevWins,
				AvgPnL:         0,
				AvgPnLPct:      highAvg,
				Description:    "Higher leverage (≥5x) produces better returns when correct",
				Evidence:       []string{fmt.Sprintf("High lev: %.2f%% avg vs Low lev: %.2f%% avg", highAvg, lowAvg)},
				Recommendation: "High leverage is working - ensure tight stop-losses are maintained",
			})
		}
	}

	// Pattern 3: Symbol-specific success
	symbolWins := make(map[string]int)
	symbolPnL := make(map[string]float64)

	for _, outcome := range outcomes {
		if outcome.Success {
			symbolWins[outcome.Symbol]++
			symbolPnL[outcome.Symbol] += outcome.RealizedPnLPct
		}
	}

	for symbol, wins := range symbolWins {
		if wins >= fg.config.MinPatternFrequency {
			avgPnL := symbolPnL[symbol] / float64(wins)
			if avgPnL > 3.0 { // At least 3% average profit
				patterns = append(patterns, TradingPattern{
					PatternType:    "symbol_affinity",
					Frequency:      wins,
					AvgPnL:         0,
					AvgPnLPct:      avgPnL,
					Description:    fmt.Sprintf("Strong performance on %s", symbol),
					Evidence:       []string{fmt.Sprintf("%d wins, %.2f%% avg profit", wins, avgPnL)},
					Recommendation: fmt.Sprintf("Consider prioritizing %s for future trades", symbol),
				})
			}
		}
	}

	// Pattern 4: Consecutive wins (momentum trading)
	consecutiveWins := 0
	maxWinStreak := 0
	currentStreak := 0
	winStreakPnL := 0.0

	for _, outcome := range outcomes {
		if outcome.Success {
			currentStreak++
			if currentStreak > maxWinStreak {
				maxWinStreak = currentStreak
			}
			if currentStreak >= 3 { // 3+ wins in a row
				consecutiveWins++
				winStreakPnL += outcome.RealizedPnLPct
			}
		} else {
			currentStreak = 0
		}
	}

	if consecutiveWins >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "momentum_trading",
			Frequency:      consecutiveWins,
			AvgPnL:         0,
			AvgPnLPct:      winStreakPnL / float64(consecutiveWins),
			Description:    fmt.Sprintf("Win streaks detected (max %d consecutive wins)", maxWinStreak),
			Evidence:       []string{fmt.Sprintf("%d occurrences of 3+ wins in a row", consecutiveWins)},
			Recommendation: "Capitalize on momentum - when on a streak, maintain the same approach",
		})
	}

	// Pattern 5: Optimal trading hours
	hourlyWins := make(map[int]int)
	hourlyPnL := make(map[int]float64)
	hourlyCount := make(map[int]int)

	for _, outcome := range outcomes {
		hour := outcome.Timestamp.Hour()
		hourlyCount[hour]++
		if outcome.Success {
			hourlyWins[hour]++
			hourlyPnL[hour] += outcome.RealizedPnLPct
		}
	}

	bestHour := -1
	bestWinRate := 0.0
	for hour, wins := range hourlyWins {
		if hourlyCount[hour] >= fg.config.MinPatternFrequency {
			winRate := float64(wins) / float64(hourlyCount[hour]) * 100
			if winRate > bestWinRate && winRate > 60 { // >60% win rate
				bestWinRate = winRate
				bestHour = hour
			}
		}
	}

	if bestHour >= 0 {
		avgPnL := hourlyPnL[bestHour] / float64(hourlyWins[bestHour])
		patterns = append(patterns, TradingPattern{
			PatternType:    "optimal_trading_hours",
			Frequency:      hourlyWins[bestHour],
			AvgPnL:         0,
			AvgPnLPct:      avgPnL,
			Description:    fmt.Sprintf("Best performance during hour %02d:00--%02d:59 UTC", bestHour, bestHour),
			Evidence:       []string{fmt.Sprintf("%.1f%% win rate, %.2f%% avg profit", bestWinRate, avgPnL)},
			Recommendation: fmt.Sprintf("Focus trading activity around %02d:00 UTC when market conditions are most favorable", bestHour),
		})
	}

	// Pattern 6: Proper position sizing on wins
	smallPosWins := 0
	smallPosPnL := 0.0
	largePosWins := 0
	largePosPnL := 0.0

	for _, outcome := range outcomes {
		if outcome.Success {
			if outcome.PositionSize < 100 { // Small positions
				smallPosWins++
				smallPosPnL += outcome.RealizedPnLPct
			} else if outcome.PositionSize > 500 { // Large positions
				largePosWins++
				largePosPnL += outcome.RealizedPnLPct
			}
		}
	}

	if smallPosWins >= fg.config.MinPatternFrequency && largePosWins >= fg.config.MinPatternFrequency {
		smallAvg := smallPosPnL / float64(smallPosWins)
		largeAvg := largePosPnL / float64(largePosWins)

		if smallAvg > largeAvg && smallAvg > 3.0 {
			patterns = append(patterns, TradingPattern{
				PatternType:    "optimal_position_sizing",
				Frequency:      smallPosWins,
				AvgPnL:         0,
				AvgPnLPct:      smallAvg,
				Description:    "Smaller positions (<$100) producing better risk-adjusted returns",
				Evidence:       []string{fmt.Sprintf("Small: %.2f%% vs Large: %.2f%%", smallAvg, largeAvg)},
				Recommendation: "Start with smaller position sizes to reduce risk and improve consistency",
			})
		}
	}

	if fg.config.EnableMicrostructure {
		patterns = append(patterns, fg.detectMicrostructureSuccessPatterns(outcomes)...)
	}

	return patterns
}

// identifyFailurePatterns finds patterns in losing trades
func (fg *FeedbackGenerator) identifyFailurePatterns(outcomes []DecisionOutcome, metrics *Metrics) []TradingPattern {
	var patterns []TradingPattern

	// Use metrics context to understand failure patterns within overall performance
	if len(outcomes) == 0 || metrics == nil {
		return patterns
	}

	// ============================================================================
	// NEW: Call the specialized detection functions
	// ============================================================================

	// 1. Detect overtrading patterns
	if overtradingPattern := fg.detectOvertrading(outcomes); overtradingPattern != nil {
		patterns = append(patterns, *overtradingPattern)
	}

	// 2. Detect contradictory analysis patterns
	if contradictoryPattern := fg.detectContradictoryAnalysis(outcomes); contradictoryPattern != nil {
		patterns = append(patterns, *contradictoryPattern)
	}

	// 3. Detect "scared money" patterns
	if scaredMoneyPattern := fg.detectScaredMoneyPattern(outcomes); scaredMoneyPattern != nil {
		patterns = append(patterns, *scaredMoneyPattern)
	}

	// ============================================================================
	// TIER 1: Execution-Level Failure Analysis (Trade Failure V2)
	// ============================================================================

	// Analyze failures using microstructure-based Trade Failure V2
	v2FailureReasons := make(map[string]struct {
		count    int
		pnlSum   float64
		evidence []string
		examples []*decision.RecentOrder
	})

	for _, outcome := range outcomes {
		// Only analyze failed trades that have microstructure data
		if !outcome.Success && outcome.RecentOrder != nil {
			thresholds := fg.failureThresholds
			analysis := decision.AnalyzeFailedTradeWithThresholds(outcome.RecentOrder, &thresholds)
			if analysis != nil {
				reason := string(analysis.PrimaryReason)
				confidence := analysis.ConfidenceScore

				entry := v2FailureReasons[reason]
				entry.count++
				entry.pnlSum += outcome.RealizedPnLPct

				// Store evidence (top 3 examples per reason)
				if len(entry.examples) < 3 {
					entry.examples = append(entry.examples, outcome.RecentOrder)
				}

				// Store detailed notes from V2 analysis
				evidence := fmt.Sprintf("%s (confidence: %.0f%%)",
					analysis.DetailedNotes, confidence*100)
				if len(evidence) > 100 {
					evidence = evidence[:100] + "..."
				}
				entry.evidence = append(entry.evidence, evidence)

				v2FailureReasons[reason] = entry
			}
		}
	}

	// Convert V2 failure reasons to trading patterns
	// Use metrics to contextualize failure severity (e.g., if overall return is negative, failures are critical)
	for reason, data := range v2FailureReasons {
		if data.count >= fg.config.MinPatternFrequency {
			avgPnL := data.pnlSum / float64(data.count)

			// Get V2 recommendation
			v2Reason := decision.TradeFailureReason(reason)
			recommendation := getV2Recommendation(v2Reason)

			// Add metrics context to make patterns more actionable
			frequencyPct := float64(data.count) / float64(len(outcomes)) * 100
			description := fmt.Sprintf("Execution-level failure: %s | Frequency: %.1f%% of trades", humanizeV2ReasonEN(v2Reason), frequencyPct)
			if metrics.TotalReturnPct < -5 {
				description += " (CRITICAL - impacts overall returns)"
			} else if metrics.TotalReturnPct < 0 {
				description += " (HIGH - contributing to negative returns)"
			}

			patterns = append(patterns, TradingPattern{
				PatternType:    reason, // e.g., "chasing_entry", "stop_too_tight"
				Frequency:      data.count,
				AvgPnL:         0,
				AvgPnLPct:      avgPnL,
				Description:    description,
				Evidence:       data.evidence,
				Recommendation: recommendation,
			})
		}
	}

	// ============================================================================
	// TIER 2: Behavioral Pattern Detection (Original Feedback System)
	// ============================================================================

	// Pattern 1: Holding losers too long
	longLosses := 0
	longLossesPnL := 0.0

	for _, outcome := range outcomes {
		if !outcome.Success {
			duration := parseDurationMinutes(outcome.HoldDuration)
			if duration > 240 { // > 4 hours
				longLosses++
				longLossesPnL += outcome.RealizedPnLPct
			}
		}
	}

	if longLosses >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "holding_losers",
			Frequency:      longLosses,
			AvgPnL:         0,
			AvgPnLPct:      longLossesPnL / float64(longLosses),
			Description:    "Holding losing positions for too long (>4h)",
			Evidence:       []string{fmt.Sprintf("%d trades, avg loss %.2f%%", longLosses, longLossesPnL/float64(longLosses))},
			Recommendation: "⚠️ CRITICAL: Cut losses faster. Set tighter stop-losses and respect them",
		})
	}

	// Pattern 2: Leverage amplifying losses
	highLevLosses := 0
	highLevLossPnL := 0.0
	lowLevLosses := 0
	lowLevLossPnL := 0.0

	for _, outcome := range outcomes {
		if !outcome.Success {
			if outcome.Leverage >= 5 {
				highLevLosses++
				highLevLossPnL += outcome.RealizedPnLPct
			} else if outcome.Leverage <= 3 {
				lowLevLosses++
				lowLevLossPnL += outcome.RealizedPnLPct
			}
		}
	}

	if highLevLosses >= fg.config.MinPatternFrequency && lowLevLosses >= fg.config.MinPatternFrequency {
		highAvgLoss := highLevLossPnL / float64(highLevLosses)
		lowAvgLoss := lowLevLossPnL / float64(lowLevLosses)

		if highAvgLoss < lowAvgLoss { // More negative = worse
			patterns = append(patterns, TradingPattern{
				PatternType:    "high_leverage_losses",
				Frequency:      highLevLosses,
				AvgPnL:         0,
				AvgPnLPct:      highAvgLoss,
				Description:    "High leverage (≥5x) amplifying losses significantly",
				Evidence:       []string{fmt.Sprintf("High lev losses: %.2f%% avg vs Low lev: %.2f%%", highAvgLoss, lowAvgLoss)},
				Recommendation: "⚠️ CRITICAL: Reduce leverage. High leverage is destroying capital",
			})
		}
	}

	// Pattern 3: Premature entries (quick losses)
	quickLosses := 0
	quickLossPnL := 0.0

	for _, outcome := range outcomes {
		if !outcome.Success {
			duration := parseDurationMinutes(outcome.HoldDuration)
			if duration < 15 { // < 15 minutes
				quickLosses++
				quickLossPnL += outcome.RealizedPnLPct
			}
		}
	}

	if quickLosses >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "premature_entries",
			Frequency:      quickLosses,
			AvgPnL:         0,
			AvgPnLPct:      quickLossPnL / float64(quickLosses),
			Description:    "Getting stopped out quickly (<15min) suggests poor entry timing",
			Evidence:       []string{fmt.Sprintf("%d quick losses, avg %.2f%%", quickLosses, quickLossPnL/float64(quickLosses))},
			Recommendation: "⚠️ Improve entry timing: wait for better confirmation before entering",
		})
	}

	// Pattern 4: Symbol-specific failures
	symbolLosses := make(map[string]int)
	symbolLossPnL := make(map[string]float64)

	for _, outcome := range outcomes {
		if !outcome.Success {
			symbolLosses[outcome.Symbol]++
			symbolLossPnL[outcome.Symbol] += outcome.RealizedPnLPct
		}
	}

	for symbol, losses := range symbolLosses {
		if losses >= fg.config.MinPatternFrequency {
			avgLoss := symbolLossPnL[symbol] / float64(losses)
			if avgLoss < -3.0 { // At least -3% average loss
				patterns = append(patterns, TradingPattern{
					PatternType:    "symbol_weakness",
					Frequency:      losses,
					AvgPnL:         0,
					AvgPnLPct:      avgLoss,
					Description:    fmt.Sprintf("Consistent losses on %s", symbol),
					Evidence:       []string{fmt.Sprintf("%d losses, %.2f%% avg loss", losses, avgLoss)},
					Recommendation: fmt.Sprintf("⚠️ Avoid or reduce exposure to %s until market conditions improve", symbol),
				})
			}
		}
	}

	// Pattern 5: Overtrading (multiple trades in short period)
	tradesByHour := make(map[string]int) // hour key: "2026-01-11-14"
	overtradeInstances := 0
	overtradeLoss := 0.0

	for i, outcome := range outcomes {
		hourKey := outcome.Timestamp.Format("2006-01-02-15")
		tradesByHour[hourKey]++

		// If 3+ trades in same hour and this one lost
		if tradesByHour[hourKey] >= 3 && !outcome.Success {
			// Check if this is part of rapid sequence
			if i > 0 {
				timeDiff := outcome.Timestamp.Sub(outcomes[i-1].Timestamp).Minutes()
				if timeDiff < 30 { // Less than 30 min between trades
					overtradeInstances++
					overtradeLoss += outcome.RealizedPnLPct
				}
			}
		}
	}

	if overtradeInstances >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "overtrading",
			Frequency:      overtradeInstances,
			AvgPnL:         0,
			AvgPnLPct:      overtradeLoss / float64(overtradeInstances),
			Description:    "Taking too many trades in short periods (3+ per hour) leading to losses",
			Evidence:       []string{fmt.Sprintf("%d instances, avg loss %.2f%%", overtradeInstances, overtradeLoss/float64(overtradeInstances))},
			Recommendation: "⚠️ Reduce trading frequency. Wait at least 1 hour between trades unless strong setup",
		})
	}

	// Pattern 6: Win streak breakage (giving back profits)
	winStreakGivebacks := 0
	givebackLoss := 0.0

	for i := 2; i < len(outcomes); i++ {
		// If previous 2 were wins and current is loss
		if outcomes[i-1].Success && outcomes[i-2].Success && !outcomes[i].Success {
			// Check if loss is significant
			if outcomes[i].RealizedPnLPct < -3.0 {
				winStreakGivebacks++
				givebackLoss += outcomes[i].RealizedPnLPct
			}
		}
	}

	if winStreakGivebacks >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "profit_giveback",
			Frequency:      winStreakGivebacks,
			AvgPnL:         0,
			AvgPnLPct:      givebackLoss / float64(winStreakGivebacks),
			Description:    "Giving back profits after win streaks with large losses",
			Evidence:       []string{fmt.Sprintf("%d occurrences, avg loss %.2f%%", winStreakGivebacks, givebackLoss/float64(winStreakGivebacks))},
			Recommendation: "⚠️ After 2+ wins, take a break or reduce position size on next trade",
		})
	}

	// Pattern 7: Revenge trading (quick trade after loss)
	revengeTradeCount := 0
	revengeTradeLoss := 0.0

	for i := 1; i < len(outcomes); i++ {
		// If previous was a loss
		if !outcomes[i-1].Success {
			// And current trade came within 15 minutes
			timeDiff := outcomes[i].Timestamp.Sub(outcomes[i-1].Timestamp).Minutes()
			if timeDiff < 15 && !outcomes[i].Success {
				revengeTradeCount++
				revengeTradeLoss += outcomes[i].RealizedPnLPct
			}
		}
	}

	if revengeTradeCount >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "revenge_trading",
			Frequency:      revengeTradeCount,
			AvgPnL:         0,
			AvgPnLPct:      revengeTradeLoss / float64(revengeTradeCount),
			Description:    "Entering new trades too quickly after losses (<15min), often resulting in more losses",
			Evidence:       []string{fmt.Sprintf("%d revenge trades, avg loss %.2f%%", revengeTradeCount, revengeTradeLoss/float64(revengeTradeCount))},
			Recommendation: "⚠️ CRITICAL: After a loss, wait at least 30 minutes before next trade to avoid emotional decisions",
		})
	}

	// Pattern 8: Poor trading hours
	hourlyLosses := make(map[int]int)
	hourlyLossPnL := make(map[int]float64)
	hourlyTotalTrades := make(map[int]int)

	for _, outcome := range outcomes {
		hour := outcome.Timestamp.Hour()
		hourlyTotalTrades[hour]++
		if !outcome.Success {
			hourlyLosses[hour]++
			hourlyLossPnL[hour] += outcome.RealizedPnLPct
		}
	}

	worstHour := -1
	worstLossRate := 0.0
	for hour, losses := range hourlyLosses {
		if hourlyTotalTrades[hour] >= fg.config.MinPatternFrequency {
			lossRate := float64(losses) / float64(hourlyTotalTrades[hour]) * 100
			if lossRate > worstLossRate && lossRate > 60 { // >60% loss rate
				worstLossRate = lossRate
				worstHour = hour
			}
		}
	}

	if worstHour >= 0 {
		avgLoss := hourlyLossPnL[worstHour] / float64(hourlyLosses[worstHour])
		patterns = append(patterns, TradingPattern{
			PatternType:    "poor_trading_hours",
			Frequency:      hourlyLosses[worstHour],
			AvgPnL:         0,
			AvgPnLPct:      avgLoss,
			Description:    fmt.Sprintf("Poor performance during hour %02d:00--%02d:59 UTC", worstHour, worstHour),
			Evidence:       []string{fmt.Sprintf("%.1f%% loss rate, %.2f%% avg loss", worstLossRate, avgLoss)},
			Recommendation: fmt.Sprintf("⚠️ AVOID trading during %02d:00 UTC when conditions are unfavorable", worstHour),
		})
	}

	// Pattern 9: Oversized positions on losses
	largePosLosses := 0
	largePosLossPnL := 0.0
	smallPosLosses := 0
	smallPosLossPnL := 0.0

	for _, outcome := range outcomes {
		if !outcome.Success {
			if outcome.PositionSize > 500 {
				largePosLosses++
				largePosLossPnL += outcome.RealizedPnLPct
			} else if outcome.PositionSize < 100 {
				smallPosLosses++
				smallPosLossPnL += outcome.RealizedPnLPct
			}
		}
	}

	if largePosLosses >= fg.config.MinPatternFrequency && smallPosLosses >= fg.config.MinPatternFrequency {
		largeAvgLoss := largePosLossPnL / float64(largePosLosses)
		smallAvgLoss := smallPosLossPnL / float64(smallPosLosses)

		if largeAvgLoss < smallAvgLoss { // More negative = worse
			patterns = append(patterns, TradingPattern{
				PatternType:    "oversized_positions",
				Frequency:      largePosLosses,
				AvgPnL:         0,
				AvgPnLPct:      largeAvgLoss,
				Description:    "Large positions (>$500) amplifying losses significantly",
				Evidence:       []string{fmt.Sprintf("Large: %.2f%% vs Small: %.2f%%", largeAvgLoss, smallAvgLoss)},
				Recommendation: "⚠️ CRITICAL: Reduce position sizes. Large positions are destroying capital faster",
			})
		}
	}

	// NEW: Specific pattern detection from your NOFX example

	// Pattern: Trading During Consolidation
	consolidationTrades := 0
	consolidationLoss := 0.0
	for _, outcome := range outcomes {
		// This would need market data - simplified check
		if outcome.HoldDuration < "45m" && !outcome.Success {
			// Quick losses often happen in consolidation
			consolidationTrades++
			consolidationLoss += outcome.RealizedPnLPct
		}
	}

	if consolidationTrades >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType:    "trading_consolidation",
			Frequency:      consolidationTrades,
			AvgPnLPct:      consolidationLoss / float64(consolidationTrades),
			Description:    "Trading during sideways/consolidation markets",
			Evidence:       []string{fmt.Sprintf("%d trades during low-volatility periods", consolidationTrades)},
			Recommendation: "AVOID trading when: 1) Price between EMAs, 2) Bollinger Bands < 50% width, 3) Volume < average",
		})
	}

	// Pattern: Poor R/R Execution
	poorRRCount := 0
	poorRRLoss := 0.0
	for _, outcome := range outcomes {
		// Simplified: Winners should be significantly larger than losers
		if outcome.Success && outcome.RealizedPnLPct < 2.0 {
			// Small winners suggest poor R/R
			poorRRCount++
		} else if !outcome.Success && math.Abs(outcome.RealizedPnLPct) > 3.0 {
			// Large losers suggest poor risk management
			poorRRCount++
			poorRRLoss += outcome.RealizedPnLPct
		}
	}

	if poorRRCount >= fg.config.MinPatternFrequency {
		patterns = append(patterns, TradingPattern{
			PatternType: "poor_risk_reward",
			Frequency:   poorRRCount,
			AvgPnLPct:   poorRRLoss / float64(poorRRCount),
			Description: "Inadequate risk/reward ratios on trades",
			Evidence: []string{
				fmt.Sprintf("%d trades with suboptimal R/R", poorRRCount),
				"Winners too small (<2%) or losers too large (>3%)",
			},
			Recommendation: "ENFORCE: Minimum 2:1 R/R. Set stop-loss at 2%, target at 4%+. Use ATR-based stops (1.5x ATR).",
		})
	}

	if fg.config.EnableMicrostructure {
		patterns = append(patterns, fg.detectMicrostructureFailurePatterns(outcomes)...)
	}

	return patterns
}

func parseDurationMinutes(duration string) int {
	// Parse duration like "2h30m" or "45m"
	var hours, minutes int
	if strings.Contains(duration, "h") {
		_, _ = fmt.Sscanf(duration, "%dh%dm", &hours, &minutes)
	} else {
		_, _ = fmt.Sscanf(duration, "%dm", &minutes)
	}
	return hours*60 + minutes
}

// getTopTrades returns the top winning or losing trades
func (fg *FeedbackGenerator) getTopTrades(outcomes []DecisionOutcome, winning bool, count int) []DecisionOutcome {
	filtered := make([]DecisionOutcome, 0)

	for _, outcome := range outcomes {
		if outcome.Success == winning {
			filtered = append(filtered, outcome)
		}
	}

	// Sort by absolute PnL percentage
	sort.Slice(filtered, func(i, j int) bool {
		return math.Abs(filtered[i].RealizedPnLPct) > math.Abs(filtered[j].RealizedPnLPct)
	})

	if len(filtered) > count {
		filtered = filtered[:count]
	}

	return filtered
}

// generateKeyInsights creates actionable insights from the analysis
func (fg *FeedbackGenerator) generateKeyInsights(metrics *Metrics, outcomes []DecisionOutcome, analysis *FeedbackAnalysis) []string {
	insights := make([]string, 0)

	// Use outcomes to get trade count for context
	totalTrades := len(outcomes)
	winningTrades := 0
	for _, outcome := range outcomes {
		if outcome.Success {
			winningTrades++
		}
	}

	// Performance assessment
	if metrics.TotalReturnPct < -5 {
		insights = append(insights, fmt.Sprintf("🔴 CRITICAL: Portfolio down %.2f%%. Immediate strategy revision needed", metrics.TotalReturnPct))
	} else if metrics.TotalReturnPct < 0 {
		insights = append(insights, fmt.Sprintf("⚠️ Portfolio slightly negative (%.2f%%). Minor adjustments recommended", metrics.TotalReturnPct))
	} else if metrics.TotalReturnPct > 10 {
		insights = append(insights, fmt.Sprintf("✅ Strong performance: +%.2f%%. Current strategy is working well", metrics.TotalReturnPct))
	}

	// Win rate analysis (with actual trade count)
	if metrics.WinRate < 40 {
		insights = append(insights, fmt.Sprintf("📉 Low win rate (%.1f%%, %d of %d trades). Focus on better entry timing and trade selection", metrics.WinRate, winningTrades, totalTrades))
	} else if metrics.WinRate > 60 {
		insights = append(insights, fmt.Sprintf("📈 Excellent win rate (%.1f%%, %d of %d trades). Good trade selection", metrics.WinRate, winningTrades, totalTrades))
	}

	// Profit factor analysis
	if metrics.ProfitFactor < 1.0 {
		insights = append(insights, fmt.Sprintf("⚠️ Okay profit factor %.2f < 1.0: Losses exceed profits. Strategy is likely losing money over time."+
			"Every single trade** must have a **minimum 1.5:1 reward-to-risk ratio** before entry."+
			"Until you have a winning strategy, you must enforce discipline. **Aim for a minimum 2:1 Reward-to-Risk ratio on every planned trade.**"+
			"This means if your stop-loss is 2%%, your target should be at least 4%% away."+
			"**Define Riding-the-Trend as a Trade Management Rule**: Once a trade is in profit (e.g., +1.5R), move your stop-loss to breakeven.\n\n", metrics.ProfitFactor))
	} else if metrics.ProfitFactor < 1.5 {
		insights = append(insights, fmt.Sprintf("Profit factor %.2f is acceptable but can be improved. Focus on better closing time, and ride the market trend.", metrics.ProfitFactor))
	} else {
		insights = append(insights, fmt.Sprintf("✅ Good profit factor (%.2f). Average wins sufficiently larger than losses", metrics.ProfitFactor))
	}

	// Drawdown analysis
	if metrics.MaxDrawdownPct > 30 {
		insights = append(insights, fmt.Sprintf("🔴 CRITICAL: %.1f%% max drawdown is excessive. Reduce position sizes and leverage", metrics.MaxDrawdownPct))
	} else if metrics.MaxDrawdownPct > 20 {
		insights = append(insights, fmt.Sprintf("⚠️ Drawdown %.1f%% is high. Improve risk management", metrics.MaxDrawdownPct))
	} else {
		insights = append(insights, fmt.Sprintf("✅ Drawdown well-controlled at %.1f%%", metrics.MaxDrawdownPct))
	}

	// Pattern-specific insights from the identified failure patterns
	for _, pattern := range analysis.FailurePatterns {
		// Add insights for the new patterns we're detecting
		switch pattern.PatternType {
		case "overtrading_frequency":
			insights = append(insights, fmt.Sprintf("🚨 OVERTRADING: %s", pattern.Description))
		case "contradictory_analysis":
			insights = append(insights, fmt.Sprintf("🔄 CONTRADICTORY ANALYSIS: %s", pattern.Description))
		case "scared_money_pattern":
			insights = append(insights, fmt.Sprintf("💸 SCARED MONEY: %s", pattern.Description))
		case "holding_losers":
			insights = append(insights, "⚠️ Tendency to hold losing positions. Set and respect stop-losses")
		case "high_leverage_losses":
			insights = append(insights, "⚠️ High leverage causing outsized losses. Reduce leverage immediately")
		case "overtrading":
			insights = append(insights, "⚠️ Overtrading detected. Quality over quantity - be more selective")
		case "revenge_trading":
			insights = append(insights, "🔴 CRITICAL: Revenge trading after losses. Take breaks to avoid emotional decisions")
		case "profit_giveback":
			insights = append(insights, "⚠️ Giving back profits after wins. Consider taking breaks after 2+ consecutive wins")
		case "oversized_positions":
			insights = append(insights, "⚠️ Position sizing too aggressive. Reduce to preserve capital")
		}
	}

	// Remove the duplicate detection logic that's now in the pattern detection functions

	return insights
}

// Add the "scared money" pattern detection function (was in generateKeyInsights but should be a pattern detector)
func (fg *FeedbackGenerator) detectScaredMoneyPattern(outcomes []DecisionOutcome) *TradingPattern {
	if len(outcomes) < 2 {
		return nil
	}

	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].Timestamp.Before(outcomes[j].Timestamp)
	})

	scaredMoneyInstances := 0
	scaredMoneyLoss := 0.0
	evidence := []string{}

	for i := 0; i < len(outcomes)-1; i++ {
		current := outcomes[i]
		next := outcomes[i+1]

		// Check for pattern: small loss followed by quick re-entry on same symbol
		if !current.Success &&
			math.Abs(current.RealizedPnLPct) < 0.5 && // Small loss (<0.5%)
			next.Symbol == current.Symbol && // Same symbol
			next.Timestamp.Sub(current.Timestamp) < 30*time.Minute { // Within 30 minutes

			scaredMoneyInstances++
			scaredMoneyLoss += current.RealizedPnLPct

			evidence = append(evidence,
				fmt.Sprintf("%s: Small loss (%.2f%%) at %s, re-entered at %s (gap: %v)",
					current.Symbol, current.RealizedPnLPct,
					current.Timestamp.Format("15:04"),
					next.Timestamp.Format("15:04"),
					next.Timestamp.Sub(current.Timestamp).Round(time.Minute)))
		}
	}

	if scaredMoneyInstances >= fg.config.MinPatternFrequency {
		return &TradingPattern{
			PatternType:    "scared_money_pattern",
			Frequency:      scaredMoneyInstances,
			AvgPnLPct:      scaredMoneyLoss / float64(scaredMoneyInstances),
			Description:    "Scared Money Pattern: Cutting positions early after small losses, then re-entering",
			Evidence:       evidence[:min(3, len(evidence))],
			Recommendation: "ENFORCE: After closing a position on ANY symbol, wait minimum 60 minutes before re-entering same symbol. Small losses are acceptable - don't chase trades.",
		}
	}

	return nil
}

// generateRecommendedActions creates specific actions to improve performance
func (fg *FeedbackGenerator) generateRecommendedActions(analysis *FeedbackAnalysis) []string {
	actions := make([]string, 0)

	// Based on overall performance
	if analysis.TotalReturnPct < 0 {
		actions = append(actions, "1. REDUCE POSITION SIZES: Start with 50% of current position sizes until profitability improves")
		actions = append(actions, "2. TIGHTER STOP-LOSSES: Set stop-losses at 2-3% maximum loss per trade")
		actions = append(actions, "3. SELECTIVE TRADING: Only take trades with 70%+ confidence and clear technical setup")
	}

	// Based on win rate
	if analysis.WinRate < 45 {
		actions = append(actions, "4. IMPROVE ENTRY TIMING: Wait for stronger confirmations (multiple timeframe alignment + volume)")
		actions = append(actions, "5. SKIP LOW-CONFIDENCE TRADES: Avoid trades with confidence < 70%")
	}

	// Based on failure patterns
	hasHoldingLosers := false
	hasHighLevLosses := false
	hasPrematureEntries := false
	hasOvertrading := false
	hasRevengeTrading := false
	hasProfitGiveback := false
	hasOversizedPos := false
	hasPoorHours := false

	for _, pattern := range analysis.FailurePatterns {
		switch pattern.PatternType {
		case "holding_losers":
			hasHoldingLosers = true
		case "high_leverage_losses":
			hasHighLevLosses = true
		case "premature_entries":
			hasPrematureEntries = true
		case "overtrading":
			hasOvertrading = true
		case "revenge_trading":
			hasRevengeTrading = true
		case "profit_giveback":
			hasProfitGiveback = true
		case "oversized_positions":
			hasOversizedPos = true
		case "poor_trading_hours":
			hasPoorHours = true
		}
	}

	if hasHoldingLosers {
		actions = append(actions, "6. DISCIPLINE ON STOP-LOSSES: When stop-loss is hit, exit immediately without hesitation")
	}

	if hasHighLevLosses {
		actions = append(actions, "7. LOWER LEVERAGE: Reduce leverage to 2-3x maximum until consistent profitability achieved")
	}

	if hasPrematureEntries {
		actions = append(actions, "8. BE PATIENT: Wait for price to confirm direction before entering (avoid FOMO)")
	}

	if hasOvertrading {
		actions = append(actions, "9. REDUCE FREQUENCY: Limit to maximum 2-3 high-quality trades per day")
	}

	if hasRevengeTrading {
		actions = append(actions, "10. MANDATORY COOLDOWN: Wait 30+ minutes after any loss before next trade")
	}

	if hasProfitGiveback {
		actions = append(actions, "11. PROTECT PROFITS: After 2+ wins, take a break or reduce position size by 50%")
	}

	if hasOversizedPos {
		actions = append(actions, "12. SIZE DOWN: Reduce position sizes to $100-200 range until consistency improves")
	}

	if hasPoorHours {
		actions = append(actions, "13. TIME SELECTION: Avoid trading during identified poor-performance hours")
	}

	// Based on success patterns
	actionNum := 14 // Continue from failure pattern actions
	for _, pattern := range analysis.SuccessPatterns {
		switch pattern.PatternType {
		case "symbol_affinity":
			actions = append(actions, fmt.Sprintf("%d. FOCUS ON WINNERS: %s", actionNum, pattern.Recommendation))
			actionNum++
		case "quick_profit_taking":
			actions = append(actions, fmt.Sprintf("%d. CONTINUE SCALPING: Quick profit-taking has been effective", actionNum))
			actionNum++
		case "momentum_trading":
			actions = append(actions, fmt.Sprintf("%d. RIDE MOMENTUM: When on a win streak, maintain the same approach", actionNum))
			actionNum++
		case "optimal_trading_hours":
			actions = append(actions, fmt.Sprintf("%d. TIME FOCUS: %s", actionNum, pattern.Recommendation))
			actionNum++
		case "optimal_position_sizing":
			actions = append(actions, fmt.Sprintf("%d. POSITION SIZING: %s", actionNum, pattern.Recommendation))
			actionNum++
		}
	}

	// Based on profit factor
	if analysis.ProfitFactor < 1.5 {
		actions = append(actions, "11. LET WINNERS RUN: Move stop-loss to breakeven and let profitable trades reach larger targets")
		actions = append(actions, "12. IMPROVE RISK/REWARD: Target at least 2:1 reward-to-risk ratio on all trades")
	}

	// NEW: Stricter actions based on specific phenomenons
	// 1. Overtrading countermeasures
	actions = append(actions, "🚫 **TRADING FREQUENCY LIMITS**:")
	actions = append(actions, "   • MAX 2 trades per hour")
	actions = append(actions, "   • MAX 8 trades per day")
	actions = append(actions, "   • Minimum 45 minutes between trades")

	// 2. Checklist enforcement
	actions = append(actions, "✅ **MANDATORY PRE-TRADE CHECKLIST**:")
	actions = append(actions, "   • Confidence ≥ 80% (not 70%)")
	actions = append(actions, "   • Position size ≤ 40% of usual (not 50%)")
	actions = append(actions, "   • Clear 2:1 R/R BEFORE entry")
	actions = append(actions, "   • Market in trending regime (ChopScore < 40)")
	actions = append(actions, "   • Multi-timeframe alignment (5m, 15m, 1H)")

	// 3. Psychological safeguards
	actions = append(actions, "🧘 **PSYCHOLOGICAL SAFEGUARDS**:")
	actions = append(actions, "   • After ANY loss: 60-minute mandatory break")
	actions = append(actions, "   • After 2 consecutive wins: Reduce position size by 30%")
	actions = append(actions, "   • Never re-enter same symbol within 90 minutes")

	// 4. Market context emphasis
	actions = append(actions, "📊 **MARKET CONTEXT RULES**:")
	actions = append(actions, "   • SKIP trades when: EMA20 between EMA50 and EMA100")
	actions = append(actions, "   • SKIP trades when: RSI 40-60 (neutral)")
	actions = append(actions, "   • ONLY enter when: Volume > 150% of 20-period average")
	actions = append(actions, "   • ONLY enter when: Institutional OI trending same direction")

	return actions
}

// analyzeMarketConditions determines the current market regime
func (fg *FeedbackGenerator) analyzeMarketConditions(metrics *Metrics, outcomes []DecisionOutcome) string {
	// Use outcomes to assess trade behavior in current market
	if len(outcomes) == 0 {
		return "INSUFFICIENT DATA: Need more trades to determine market conditions"
	}

	// Count consecutive wins/losses for trend assessment
	consecutiveWins := 0
	consecutiveLosses := 0
	maxConsecutiveWins := 0
	maxConsecutiveLosses := 0

	for _, outcome := range outcomes {
		if outcome.Success {
			consecutiveWins++
			if consecutiveWins > maxConsecutiveWins {
				maxConsecutiveWins = consecutiveWins
			}
			consecutiveLosses = 0
		} else {
			consecutiveLosses++
			if consecutiveLosses > maxConsecutiveLosses {
				maxConsecutiveLosses = consecutiveLosses
			}
			consecutiveWins = 0
		}
	}

	// Analyze market conditions using both metrics and trade patterns
	if metrics.WinRate < 40 && metrics.TotalReturnPct < -5 && maxConsecutiveLosses > 3 {
		return "DIFFICULT MARKET: High volatility or ranging conditions. Multiple consecutive losses detected. Consider reducing activity"
	}

	if metrics.WinRate > 55 && metrics.TotalReturnPct > 5 && maxConsecutiveWins > 2 {
		return "FAVORABLE MARKET: Clear trends present with consecutive wins. Strategy is well-aligned with current conditions"
	}

	if metrics.WinRate > 50 && metrics.ProfitFactor < 1.2 && maxConsecutiveWins > 0 {
		return "MIXED MARKET: Winning often but profits are small. Need to let winners run longer and reduce stop losses"
	}

	return fmt.Sprintf("NEUTRAL MARKET: Standard trading conditions with %d trades analyzed. Continue with current approach", len(outcomes))
}

// FormatFeedbackForPrompt formats the feedback analysis for inclusion in AI prompts
func (fg *FeedbackGenerator) FormatForPrompt(analysis *FeedbackAnalysis, lang string, detailed bool) string {
	if analysis == nil {
		return ""
	}

	return fg.FormatFeedbackForPrompt(analysis, lang, detailed)
}

// FormatForPrompt formats feedback for LLM consumption - CONCISE VERSION
func (fg *FeedbackGenerator) FormatFeedbackForPrompt(analysis *FeedbackAnalysis, lang string, detailed bool) string {
	if analysis == nil {
		return "NO_FEEDBACK_AVAILABLE"
	}

	if detailed {
		return fg.FormatForDetailedPrompt(analysis, lang)
	}
	// Get only the MOST important insights (top 3 of each category)
	prioritized := fg.prioritizeFeedback(analysis)

	var sb strings.Builder

	if lang == "zh" {
		fg.formatZHConcise(&sb, prioritized, analysis)
	} else {
		fg.formatENConcise(&sb, prioritized, analysis)
	}

	return sb.String()
}

// FormatForPrompt formats the feedback analysis for inclusion in AI prompts (method on FeedbackAnalysis)
func (fg *FeedbackGenerator) FormatForDetailedPrompt(analysis *FeedbackAnalysis, lang string) string {
	if analysis == nil {
		return ""
	}

	var sb strings.Builder

	if lang == "zh" {
		sb.WriteString("## 🎯 战略级问题诊断\n\n")
		sb.WriteString("### 🔍 核心问题识别\n\n")

		// Categorize problems by severity
		sb.WriteString("**🔴 严重问题 (需要立即解决):**\n")
		// Add detected severe problems

		sb.WriteString("\n**⚠️ 中等问题 (需要改进):**\n")
		// Add detected medium problems

		sb.WriteString("\n**📊 性能数据:**\n")
		sb.WriteString(fmt.Sprintf("- 交易频率: %.1f 笔/小时\n", analysis.TradesPerHour))
		sb.WriteString(fmt.Sprintf("- 平均持仓时间: %s\n", analysis.AvgHoldTime))
		sb.WriteString(fmt.Sprintf("- 检查表遵从率: %.1f%%\n", analysis.ChecklistCompliance))

		sb.WriteString("\n### 🛡️ 防护机制激活\n")
		sb.WriteString("基于上述问题，以下防护机制已激活:\n")
		sb.WriteString("1. **频率限制器**: 最大2笔/小时\n")
		sb.WriteString("2. **情绪冷却器**: 亏损后60分钟暂停\n")
		sb.WriteString("3. **市场过滤器**: 仅趋势市场交易\n")
		sb.WriteString("4. **规模控制器**: 仓位≤40%正常规模\n\n")

		sb.WriteString("## 📊 历史表现反馈\n\n")
		sb.WriteString(fmt.Sprintf("**分析周期**: %s (覆盖 %d 个决策)\n", analysis.AnalysisPeriod, analysis.DecisionsCovered))
		sb.WriteString(fmt.Sprintf("**总回报**: %.2f%% | **胜率**: %.1f%% | **盈利因子**: %.2f | **最大回撤**: %.1f%%\n\n",
			analysis.TotalReturnPct, analysis.WinRate, analysis.ProfitFactor, analysis.MaxDrawdown))

		if len(analysis.KeyInsights) > 0 {
			sb.WriteString("### 关键洞察\n\n")
			for _, insight := range analysis.KeyInsights {
				sb.WriteString(fmt.Sprintf("- %s\n", insight))
			}
			sb.WriteString("\n")
		}

		// ============================================================================
		// EXECUTION-LEVEL FAILURE ANALYSIS (Trade Failure V2 Microstructure)
		// ============================================================================
		if len(analysis.FailurePatterns) > 0 {
			// Separate V2 execution failures from other patterns
			var v2Failures []TradingPattern
			var otherFailures []TradingPattern

			for _, pattern := range analysis.FailurePatterns {
				// V2 failure reasons contain microstructure keywords
				if strings.Contains(pattern.PatternType, "_") ||
					strings.Contains(pattern.Description, "Execution-level") {
					v2Failures = append(v2Failures, pattern)
				} else {
					otherFailures = append(otherFailures, pattern)
				}
			}

			// Display V2 execution-level diagnostics
			if len(v2Failures) > 0 {
				sb.WriteString("### 📋 执行级失败诊断 (微观结构分析)\n\n")
				sb.WriteString("**这些失败根植于市场执行条件和入场/出场时机：**\n\n")

				for i, pattern := range v2Failures {
					if i >= 5 { // Limit to top 5
						break
					}
					sb.WriteString(fmt.Sprintf("**%s**\n", pattern.Description))
					sb.WriteString(fmt.Sprintf("   发生: %d 次 | 平均亏损: %.2f%%\n", pattern.Frequency, pattern.AvgPnLPct))

					// Display evidence (first 2 pieces)
					if len(pattern.Evidence) > 0 {
						for j, evidence := range pattern.Evidence {
							if j >= 2 {
								break
							}
							sb.WriteString(fmt.Sprintf("   证据: %s\n", evidence))
						}
					}

					sb.WriteString(fmt.Sprintf("   **行动**: %s\n\n", pattern.Recommendation))
				}
			}

			// Display other failure patterns
			if len(otherFailures) > 0 {
				sb.WriteString("### ⚠️ 其他发现的失败模式\n\n")
				for i, pattern := range otherFailures {
					if i >= 3 {
						break
					}
					sb.WriteString(fmt.Sprintf("**%s** (发生 %d 次, 平均亏损 %.2f%%)\n",
						pattern.Description, pattern.Frequency, pattern.AvgPnLPct))
					sb.WriteString(fmt.Sprintf("   → %s\n\n", pattern.Recommendation))
				}
			}
		} else if len(analysis.FailurePatterns) > 0 {
			sb.WriteString("### ⚠️ 发现的失败模式\n\n")
			for i, pattern := range analysis.FailurePatterns {
				if i >= 3 {
					break // Limit to top 3
				}
				sb.WriteString(fmt.Sprintf("**%s** (发生 %d 次, 平均亏损 %.2f%%)\n",
					pattern.Description, pattern.Frequency, pattern.AvgPnLPct))
				sb.WriteString(fmt.Sprintf("   → %s\n\n", pattern.Recommendation))
			}
		}

		if len(analysis.SuccessPatterns) > 0 {
			sb.WriteString("### ✅ 发现的成功模式\n\n")
			for i, pattern := range analysis.SuccessPatterns {
				if i >= 3 {
					break
				}
				sb.WriteString(fmt.Sprintf("**%s** (发生 %d 次, 平均盈利 %.2f%%)\n",
					pattern.Description, pattern.Frequency, pattern.AvgPnLPct))
				sb.WriteString(fmt.Sprintf("   → %s\n\n", pattern.Recommendation))
			}
		}

		if len(analysis.RecommendedActions) > 0 {
			sb.WriteString("### 🎯 建议行动\n\n")
			for _, action := range analysis.RecommendedActions {
				if strings.HasPrefix(action, "⚠️") || strings.HasPrefix(action, "🔴") {
					sb.WriteString(fmt.Sprintf("**%s**\n", action))
				} else {
					sb.WriteString(fmt.Sprintf("%s\n", action))
				}
			}
			sb.WriteString("\n")
		}

		// ============================================================================
		// FEW-SHOT LEARNING: Concrete Trade Examples
		// ============================================================================
		if len(analysis.TopWinningTrades) > 0 || len(analysis.TopLosingTrades) > 0 {
			sb.WriteString("### 📚 从过去的交易中学习 (Few-Shot Examples)\n\n")
			sb.WriteString("**研究这些具体案例来理解什么有效，什么无效：**\n\n")

			// Show top winning trades as positive examples
			if len(analysis.TopWinningTrades) > 0 {
				sb.WriteString("#### ✅ 成功案例 (模仿这些交易):\n\n")
				for i, trade := range analysis.TopWinningTrades {
					if i >= 3 { // Limit to top 3
						break
					}
					sb.WriteString(fmt.Sprintf("**案例 %d: %s %s**\n", i+1, trade.Symbol, trade.Action))
					sb.WriteString(fmt.Sprintf("- 时间: %s | 持仓: %s\n", trade.Timestamp.Format("01-02 15:04"), trade.HoldDuration))
					sb.WriteString(fmt.Sprintf("- 入场: %.4f | 出场: %.4f | 杠杆: %dx\n", trade.EntryPrice, trade.ExitPrice, trade.Leverage))
					sb.WriteString(fmt.Sprintf("- **结果**: +%.2f%% (%.2f USDT)\n", trade.RealizedPnLPct, trade.RealizedPnL))
					if trade.Analysis != "" {
						sb.WriteString(fmt.Sprintf("- 分析: %s\n", trade.Analysis))
					}
					if trade.Reasoning != "" {
						sb.WriteString(fmt.Sprintf("- 原始理由: %s\n", trade.Reasoning))
					}
					sb.WriteString("\n")
				}
			}

			// Show top losing trades as negative examples
			if len(analysis.TopLosingTrades) > 0 {
				sb.WriteString("#### ❌ 失败案例 (避免重复这些错误):\n\n")
				for i, trade := range analysis.TopLosingTrades {
					if i >= 3 { // Limit to top 3
						break
					}
					sb.WriteString(fmt.Sprintf("**案例 %d: %s %s**\n", i+1, trade.Symbol, trade.Action))
					sb.WriteString(fmt.Sprintf("- 时间: %s | 持仓: %s\n", trade.Timestamp.Format("01-02 15:04"), trade.HoldDuration))
					sb.WriteString(fmt.Sprintf("- 入场: %.4f | 出场: %.4f | 杠杆: %dx\n", trade.EntryPrice, trade.ExitPrice, trade.Leverage))
					sb.WriteString(fmt.Sprintf("- **结果**: %.2f%% (%.2f USDT)\n", trade.RealizedPnLPct, trade.RealizedPnL))

					// Add Trade Failure V2 execution-level diagnosis
					if trade.RecentOrder != nil {
						failureAnalysis := decision.AnalyzeFailedTrade(trade.RecentOrder)
						if failureAnalysis != nil {
							sb.WriteString(fmt.Sprintf("- **执行诊断**: %s (置信度: %.0f%%)\n",
								humanizeV2ReasonZH(failureAnalysis.PrimaryReason),
								failureAnalysis.ConfidenceScore*100))
							if failureAnalysis.DetailedNotes != "" {
								sb.WriteString(fmt.Sprintf("- 详细说明: %s\n", failureAnalysis.DetailedNotes))
							}
							if failureAnalysis.Recommendation != "" {
								sb.WriteString(fmt.Sprintf("- ⚠️ 如何避免: %s\n", failureAnalysis.Recommendation))
							}
						}
					}

					if trade.Analysis != "" {
						sb.WriteString(fmt.Sprintf("- 分析: %s\n", trade.Analysis))
					}
					if trade.Reasoning != "" {
						sb.WriteString(fmt.Sprintf("- 原始理由: %s\n", trade.Reasoning))
					}
					sb.WriteString("\n")
				}
			}

			sb.WriteString("**💡 学习要点**: 分析这些真实案例，理解决策背景和结果之间的因果关系\n\n")
		}

		sb.WriteString(fmt.Sprintf("**市场条件评估**: %s\n\n", analysis.MarketConditions))

		sb.WriteString("---\n\n")
		sb.WriteString("**💡 基于以上反馈进行决策**: 学习失败教训，复制成功模式，严格执行建议行动\n\n")

	} else {
		sb.WriteString("## 🎯 Strategic Problem Diagnosis\n\n")
		sb.WriteString("### 🔍 Core Issue Identification\n\n")

		sb.WriteString("**🔴 Critical Issues (Immediate Action Required):**\n")

		sb.WriteString("\n**⚠️ Moderate Issues (Needs Improvement):**\n")

		sb.WriteString("\n**📊 Performance Metrics:**\n")
		sb.WriteString("\n**📊 Performance Metrics:**\n")
		sb.WriteString(fmt.Sprintf("- Trading Frequency: %.1f trades/hour\n", analysis.TradesPerHour))
		sb.WriteString(fmt.Sprintf("- Average Hold Time: %s\n", analysis.AvgHoldTime))
		sb.WriteString(fmt.Sprintf("- Checklist Compliance: %.1f%%\n", analysis.ChecklistCompliance))

		sb.WriteString("\n### 🛡️ Protection Mechanisms Activated\n")
		sb.WriteString("Based on above issues, the following protections are activated:\n")
		sb.WriteString("1. **Frequency Limiter**: Max 2 trades/hour\n")
		sb.WriteString("2. **Emotional Cooldown**: 60-min pause after any loss\n")
		sb.WriteString("3. **Market Filter**: Trade only in trending regimes\n")
		sb.WriteString("4. **Size Controller**: Position size ≤40% of normal\n\n")

		sb.WriteString("## 📊 Historical Performance Feedback\n\n")
		sb.WriteString(fmt.Sprintf("**Analysis Period**: %s (%d decisions covered)\n", analysis.AnalysisPeriod, analysis.DecisionsCovered))
		sb.WriteString(fmt.Sprintf("**Total Return**: %.2f%% | **Win Rate**: %.1f%% | **Profit Factor**: %.2f | **Max Drawdown**: %.1f%%\n\n",
			analysis.TotalReturnPct, analysis.WinRate, analysis.ProfitFactor, analysis.MaxDrawdown))

		if len(analysis.KeyInsights) > 0 {
			sb.WriteString("### Key Insights\n\n")
			for _, insight := range analysis.KeyInsights {
				sb.WriteString(fmt.Sprintf("- %s\n", insight))
			}
			sb.WriteString("\n")
		}

		// ============================================================================
		// EXECUTION-LEVEL FAILURE ANALYSIS (Trade Failure V2 Microstructure)
		// ============================================================================
		if len(analysis.FailurePatterns) > 0 {
			// Separate V2 execution failures from other patterns
			var v2Failures []TradingPattern
			var otherFailures []TradingPattern

			for _, pattern := range analysis.FailurePatterns {
				// V2 failure reasons contain microstructure keywords
				if strings.Contains(pattern.PatternType, "_") ||
					strings.Contains(pattern.Description, "Execution-level") {
					v2Failures = append(v2Failures, pattern)
				} else {
					otherFailures = append(otherFailures, pattern)
				}
			}

			// Display V2 execution-level diagnostics
			if len(v2Failures) > 0 {
				sb.WriteString("### 📋 Execution-Level Failure Diagnostics (Microstructure)\n\n")
				sb.WriteString("**These failures are rooted in actual market execution conditions and entry/exit timing:**\n\n")

				for i, pattern := range v2Failures {
					if i >= 5 { // Limit to top 5
						break
					}
					sb.WriteString(fmt.Sprintf("**%s**\n", pattern.Description))
					sb.WriteString(fmt.Sprintf("   Occurred: %d times | Avg Loss: %.2f%%\n", pattern.Frequency, pattern.AvgPnLPct))

					// Display evidence (first 2 pieces)
					if len(pattern.Evidence) > 0 {
						for j, evidence := range pattern.Evidence {
							if j >= 2 {
								break
							}
							sb.WriteString(fmt.Sprintf("   Evidence: %s\n", evidence))
						}
					}

					sb.WriteString(fmt.Sprintf("   **Action**: %s\n\n", pattern.Recommendation))
				}
			}

			// Display other failure patterns
			if len(otherFailures) > 0 {
				sb.WriteString("### ⚠️ Other Identified Failure Patterns\n\n")
				for i, pattern := range otherFailures {
					if i >= 3 {
						break
					}
					sb.WriteString(fmt.Sprintf("**%s** (occurred %d times, avg loss %.2f%%)\n",
						pattern.Description, pattern.Frequency, pattern.AvgPnLPct))
					sb.WriteString(fmt.Sprintf("   → %s\n\n", pattern.Recommendation))
				}
			}
		} else if len(analysis.FailurePatterns) > 0 {
			sb.WriteString("### ⚠️ Identified Failure Patterns\n\n")
			for i, pattern := range analysis.FailurePatterns {
				if i >= 3 {
					break
				}
				sb.WriteString(fmt.Sprintf("**%s** (occurred %d times, avg loss %.2f%%)\n",
					pattern.Description, pattern.Frequency, pattern.AvgPnLPct))
				sb.WriteString(fmt.Sprintf("   → %s\n\n", pattern.Recommendation))
			}
		}

		if len(analysis.SuccessPatterns) > 0 {
			sb.WriteString("### ✅ Identified Success Patterns\n\n")
			for i, pattern := range analysis.SuccessPatterns {
				if i >= 3 {
					break
				}
				sb.WriteString(fmt.Sprintf("**%s** (occurred %d times, avg profit %.2f%%)\n",
					pattern.Description, pattern.Frequency, pattern.AvgPnLPct))
				sb.WriteString(fmt.Sprintf("   → %s\n\n", pattern.Recommendation))
			}
		}

		if len(analysis.RecommendedActions) > 0 {
			sb.WriteString("### 🎯 Recommended Actions\n\n")
			for _, action := range analysis.RecommendedActions {
				if strings.HasPrefix(action, "⚠️") || strings.HasPrefix(action, "🔴") {
					sb.WriteString(fmt.Sprintf("**%s**\n", action))
				} else {
					sb.WriteString(fmt.Sprintf("%s\n", action))
				}
			}
			sb.WriteString("\n")
		}

		// ============================================================================
		// FEW-SHOT LEARNING: Concrete Trade Examples
		// ============================================================================
		if len(analysis.TopWinningTrades) > 0 || len(analysis.TopLosingTrades) > 0 {
			sb.WriteString("### 📚 Learn from Past Trades (Few-Shot Examples)\n\n")
			sb.WriteString("**Study these concrete examples to understand what works and what doesn't:**\n\n")

			// Show top winning trades as positive examples
			if len(analysis.TopWinningTrades) > 0 {
				sb.WriteString("#### ✅ Success Examples (Replicate These Trades):\n\n")
				for i, trade := range analysis.TopWinningTrades {
					if i >= 3 { // Limit to top 3
						break
					}
					sb.WriteString(fmt.Sprintf("**Example %d: %s %s**\n", i+1, trade.Symbol, trade.Action))
					sb.WriteString(fmt.Sprintf("- Time: %s | Duration: %s\n", trade.Timestamp.Format("01-02 15:04"), trade.HoldDuration))
					sb.WriteString(fmt.Sprintf("- Entry: %.4f | Exit: %.4f | Leverage: %dx\n", trade.EntryPrice, trade.ExitPrice, trade.Leverage))
					sb.WriteString(fmt.Sprintf("- **Outcome**: +%.2f%% (%.2f USDT)\n", trade.RealizedPnLPct, trade.RealizedPnL))
					if trade.Analysis != "" {
						sb.WriteString(fmt.Sprintf("- Analysis: %s\n", trade.Analysis))
					}
					if trade.Reasoning != "" {
						sb.WriteString(fmt.Sprintf("- Original Reasoning: %s\n", trade.Reasoning))
					}
					sb.WriteString("\n")
				}
			}

			// Show top losing trades as negative examples
			if len(analysis.TopLosingTrades) > 0 {
				sb.WriteString("#### ❌ Failure Examples (Avoid Repeating These Mistakes):\n\n")
				for i, trade := range analysis.TopLosingTrades {
					if i >= 3 { // Limit to top 3
						break
					}
					sb.WriteString(fmt.Sprintf("**Example %d: %s %s**\n", i+1, trade.Symbol, trade.Action))
					sb.WriteString(fmt.Sprintf("- Time: %s | Duration: %s\n", trade.Timestamp.Format("01-02 15:04"), trade.HoldDuration))
					sb.WriteString(fmt.Sprintf("- Entry: %.4f | Exit: %.4f | Leverage: %dx\n", trade.EntryPrice, trade.ExitPrice, trade.Leverage))
					sb.WriteString(fmt.Sprintf("- **Outcome**: %.2f%% (%.2f USDT)\n", trade.RealizedPnLPct, trade.RealizedPnL))

					// Add Trade Failure V2 execution-level diagnosis
					if trade.RecentOrder != nil {
						failureAnalysis := decision.AnalyzeFailedTrade(trade.RecentOrder)
						if failureAnalysis != nil {
							sb.WriteString(fmt.Sprintf("- **Execution Diagnosis**: %s (confidence: %.0f%%)\n",
								humanizeV2ReasonEN(failureAnalysis.PrimaryReason),
								failureAnalysis.ConfidenceScore*100))
							if failureAnalysis.DetailedNotes != "" {
								sb.WriteString(fmt.Sprintf("- Root Cause: %s\n", failureAnalysis.DetailedNotes))
							}
							if failureAnalysis.Recommendation != "" {
								sb.WriteString(fmt.Sprintf("- ⚠️ How to Avoid: %s\n", failureAnalysis.Recommendation))
							}
						}
					}

					if trade.Analysis != "" {
						sb.WriteString(fmt.Sprintf("- Analysis: %s\n", trade.Analysis))
					}
					if trade.Reasoning != "" {
						sb.WriteString(fmt.Sprintf("- Original Reasoning: %s\n", trade.Reasoning))
					}
					sb.WriteString("\n")
				}
			}

			sb.WriteString("**💡 Learning Point**: Analyze these real examples to understand the causal relationship between decision context and outcomes\n\n")
		}

		sb.WriteString(fmt.Sprintf("**Market Conditions Assessment**: %s\n\n", analysis.MarketConditions))

		sb.WriteString("---\n\n")
		sb.WriteString("**💡 Make decisions based on this feedback**: Learn from failures, replicate successes, and strictly follow recommended actions\n\n")
	}

	return sb.String()
}

// Helper functions for FormatForPrompt - ACTUAL IMPLEMENTATIONS
func (fg *FeedbackGenerator) calculateTradesPerHour(outcomes []DecisionOutcome) float64 {
	if len(outcomes) < 2 {
		return 0
	}

	// Sort by timestamp
	sort.Slice(outcomes, func(i, j int) bool {
		return outcomes[i].Timestamp.Before(outcomes[j].Timestamp)
	})

	// Calculate time window
	startTime := outcomes[0].Timestamp
	endTime := outcomes[len(outcomes)-1].Timestamp
	timeWindow := endTime.Sub(startTime)

	if timeWindow <= 0 {
		return 0
	}

	return float64(len(outcomes)) / timeWindow.Hours()
}

func (fg *FeedbackGenerator) calculateAvgHoldTime(outcomes []DecisionOutcome) string {
	if len(outcomes) == 0 {
		return "0m"
	}

	var totalDuration time.Duration
	count := 0

	for _, outcome := range outcomes {
		if outcome.HoldDuration != "" {
			duration, err := parseDurationFromString(outcome.HoldDuration)
			if err == nil {
				totalDuration += duration
				count++
			}
		}
	}

	if count == 0 {
		return "0m"
	}

	avgDuration := totalDuration / time.Duration(count)
	return formatDurationShort(avgDuration)
}

func (fg *FeedbackGenerator) calculateChecklistCompliance(outcomes []DecisionOutcome) float64 {
	if len(outcomes) == 0 {
		return 0.0
	}

	compliancePoints := 0
	totalPossiblePoints := 0

	for _, outcome := range outcomes {
		// Checklist items to check:
		// 1. Confidence >= 70%
		if outcome.Confidence >= 70 {
			compliancePoints++
		}
		totalPossiblePoints++

		// 2. Position size not excessive (simplified check)
		// In real implementation, you'd check against account size
		if outcome.PositionSize < 1000 { // Assuming $1000 is reasonable max
			compliancePoints++
		}
		totalPossiblePoints++

		// 3. Hold duration reasonable (not too short, not too long)
		duration, err := parseDurationFromString(outcome.HoldDuration)
		if err == nil && duration >= 15*time.Minute && duration <= 24*time.Hour {
			compliancePoints++
		}
		totalPossiblePoints++

		// 4. Leverage not excessive
		if outcome.Leverage <= 5 {
			compliancePoints++
		}
		totalPossiblePoints++
	}

	if totalPossiblePoints == 0 {
		return 0.0
	}

	return float64(compliancePoints) / float64(totalPossiblePoints) * 100.0
}

// Helper function to parse duration from string like "2h30m" or "45m"
func parseDurationFromString(durationStr string) (time.Duration, error) {
	// Remove any spaces
	durationStr = strings.TrimSpace(durationStr)

	// Try parsing with time.ParseDuration
	dur, err := time.ParseDuration(durationStr)
	if err == nil {
		return dur, nil
	}

	// Try custom parsing for format like "2h30m"
	var hours, minutes int
	if strings.Contains(durationStr, "h") && strings.Contains(durationStr, "m") {
		_, err := fmt.Sscanf(durationStr, "%dh%dm", &hours, &minutes)
		if err == nil {
			return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute, nil
		}
	} else if strings.Contains(durationStr, "h") {
		_, err := fmt.Sscanf(durationStr, "%dh", &hours)
		if err == nil {
			return time.Duration(hours) * time.Hour, nil
		}
	} else if strings.Contains(durationStr, "m") {
		_, err := fmt.Sscanf(durationStr, "%dm", &minutes)
		if err == nil {
			return time.Duration(minutes) * time.Minute, nil
		}
	}

	return 0, fmt.Errorf("invalid duration format: %s", durationStr)
}

// Helper function for short duration formatting
func formatDurationShort(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// SaveFeedbackAnalysis saves the feedback analysis to disk for later reference
func (fg *FeedbackGenerator) SaveFeedbackAnalysis(analysis *FeedbackAnalysis) error {
	if analysis == nil {
		return nil
	}

	feedbackPath := filepath.Join(runDir(fg.runID), "feedback_analysis.json")
	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal feedback: %w", err)
	}

	if err := os.WriteFile(feedbackPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write feedback file: %w", err)
	}

	logger.Infof("💾 Feedback analysis saved to %s", feedbackPath)
	return nil
}

// LoadFeedbackAnalysis loads a previously saved feedback analysis
func LoadFeedbackAnalysis(runID string) (*FeedbackAnalysis, error) {
	feedbackPath := filepath.Join(runDir(runID), "feedback_analysis.json")
	data, err := os.ReadFile(feedbackPath)
	if err != nil {
		return nil, err
	}

	var analysis FeedbackAnalysis
	if err := json.Unmarshal(data, &analysis); err != nil {
		return nil, err
	}

	return &analysis, nil
}

// ============================================================================
// Trade Failure V2 Integration Helpers
// ============================================================================

// humanizeV2Reason converts Trade Failure V2 reason code to human-readable text
func humanizeV2ReasonEN(reason decision.TradeFailureReason) string {
	switch reason {
	case decision.ReasonSignalQualityLow:
		return "Signal quality too low - weak edge"
	case decision.ReasonRegimeMismatch:
		return "Trade conflicted with market regime"
	case decision.ReasonLiquidityRiskHigh:
		return "Liquidity risk too high at entry"
	case decision.ReasonStackedRisk:
		return "Overexposed to same market factor"
	case decision.ReasonChasingEntry:
		return "Chased entry with excessive slippage"
	case decision.ReasonFalseBreakoutV2:
		return "False breakout - no follow-through"
	case decision.ReasonPrematureEntry:
		return "Entered before confirmation criteria met"
	case decision.ReasonSizingError:
		return "Position size too large for available liquidity"
	case decision.ReasonSlippageExceeded:
		return "Actual slippage exceeded budget"
	case decision.ReasonStopTooTight:
		return "Stop loss too tight relative to volatility"
	case decision.ReasonMomentumDecay:
		return "Momentum decayed - volume/OI collapsed"
	case decision.ReasonLiquidityDried:
		return "Market liquidity dried up during hold"
	case decision.ReasonStopHitRegimeChange:
		return "Stop hit due to regime change"
	case decision.ReasonLateExitGiveBack:
		return "Exited late with large give-back from peak"
	case decision.ReasonTrendReversalIgnored:
		return "Missed trend reversal signal"
	case decision.ReasonHighSlippageExit:
		return "Poor exit execution with high slippage"
	case decision.ReasonFundingDrag:
		return "Funding costs ate into profit"
	case decision.ReasonBorrowingCostHigh:
		return "Borrowing costs were significant"
	case decision.ReasonTechnicalFault:
		return "Technical or execution system error"
	default:
		return string(reason)
	}
}

func humanizeV2ReasonZH(reason decision.TradeFailureReason) string {
	switch reason {
	case decision.ReasonSignalQualityLow:
		return "信号质量过低 - 优势微弱"
	case decision.ReasonRegimeMismatch:
		return "交易与市场状态冲突"
	case decision.ReasonLiquidityRiskHigh:
		return "入场时流动性风险过高"
	case decision.ReasonStackedRisk:
		return "对同一市场因子过度暴露"
	case decision.ReasonChasingEntry:
		return "追逐入场导致滑点过高"
	case decision.ReasonFalseBreakoutV2:
		return "假突破 - 缺乏跟进"
	case decision.ReasonPrematureEntry:
		return "确认条件未满足前过早入场"
	case decision.ReasonSizingError:
		return "仓位过大超出可用流动性"
	case decision.ReasonSlippageExceeded:
		return "实际滑点超出预算"
	case decision.ReasonStopTooTight:
		return "止损相对于波动率设置过紧"
	case decision.ReasonMomentumDecay:
		return "动量衰减 - 成交量/OI崩溃"
	case decision.ReasonLiquidityDried:
		return "持仓期间市场流动性枯竭"
	case decision.ReasonStopHitRegimeChange:
		return "止损被触发因市场状态变化"
	case decision.ReasonLateExitGiveBack:
		return "出场过晚导致利润大幅回吐"
	case decision.ReasonTrendReversalIgnored:
		return "忽略趋势反转信号"
	case decision.ReasonHighSlippageExit:
		return "出场执行不佳导致高滑点"
	case decision.ReasonFundingDrag:
		return "资金费用侵蚀利润"
	case decision.ReasonBorrowingCostHigh:
		return "借贷成本显著"
	case decision.ReasonTechnicalFault:
		return "技术或执行系统错误"
	default:
		return string(reason)
	}
}

// getV2Recommendation provides actionable recommendations for V2 failure reasons
func getV2Recommendation(reason decision.TradeFailureReason) string {
	switch reason {
	case decision.ReasonSignalQualityLow:
		return "⚠️ SIGNALS: Only enter trades with strong, multi-confirmation signal. Model confidence alone is insufficient."
	case decision.ReasonRegimeMismatch:
		return "⚠️ REGIME: Check market regime before entry. Skip trades in choppy/sideways markets (ChopScore > 50)."
	case decision.ReasonLiquidityRiskHigh:
		return "⚠️ LIQUIDITY: Reduce position size for illiquid symbols. Monitor spread and depth at entry."
	case decision.ReasonStackedRisk:
		return "⚠️ CORRELATION: Don't stack positions in same sector/factor. Reduce correlation exposure."
	case decision.ReasonChasingEntry:
		return "⚠️ EXECUTION: Don't chase entries. Set hard limit on entry slippage. Cancel if slippage exceeds budget."
	case decision.ReasonFalseBreakoutV2:
		return "⚠️ CONFIRMATION: Require volume confirmation (+50% baseline) and OI increase before entering breakouts."
	case decision.ReasonPrematureEntry:
		return "⚠️ TIMING: Wait for all confirmation criteria. Don't enter before price pattern validates."
	case decision.ReasonSizingError:
		return "⚠️ SIZE: Reduce position size based on available liquidity. Use ATR-based sizing with depth checks."
	case decision.ReasonSlippageExceeded:
		return "⚠️ EXECUTION: Monitor actual slippage vs budget. Reduce order size or use limit orders for large positions."
	case decision.ReasonStopTooTight:
		return "⚠️ RISK: Widen stops to minimum 1.5x ATR. Use volatility-adjusted stops, not arbitrary percentages."
	case decision.ReasonMomentumDecay:
		return "⚠️ EXITS: Track momentum during hold. Use trailing stops that move with momentum, not just price."
	case decision.ReasonLiquidityDried:
		return "⚠️ EXITS: Monitor spread during hold. Exit immediately if spread widens >3x entry level."
	case decision.ReasonStopHitRegimeChange:
		return "⚠️ REGIME: Monitor regime throughout hold. Reduce exposure if regime shifts. Consider dynamic stops."
	case decision.ReasonLateExitGiveBack:
		return "⚠️ EXITS: Set profit targets based on momentum. Don't hold hoping for bigger moves. Exit on confirmation loss."
	case decision.ReasonTrendReversalIgnored:
		return "⚠️ EXITS: Monitor trend indicators. Exit if trend reversal signals appear. Don't force profits."
	case decision.ReasonHighSlippageExit:
		return "⚠️ EXECUTION: Use limit orders for exits. Avoid market orders in low-liquidity conditions."
	case decision.ReasonFundingDrag:
		return "⚠️ COSTS: Monitor funding rates. Reduce holding time if funding becomes expensive."
	case decision.ReasonBorrowingCostHigh:
		return "⚠️ COSTS: Check borrowing costs before entry. Skip trades where costs exceed expected profit."
	case decision.ReasonTechnicalFault:
		return "⚠️ SYSTEMS: Log technical faults. Improve system reliability. Skip trades until systems stabilize."
	default:
		return "Review execution quality and market conditions for this failure mode."
	}
}

// FormatForDebate formats the feedback for multi-agent debate context
// Emphasizes areas of contention and decision points tailored to agent roles
func (analysis *FeedbackAnalysis) FormatForDebate(lang string, agentRole string) string {
	if analysis == nil {
		return ""
	}

	var sb strings.Builder

	if lang == "zh" {
		sb.WriteString("## 🎯 辩论依据: 历史表现分析\n\n")

		// Tailor feedback based on agent role
		if agentRole == "bull" || agentRole == "optimistic" {
			sb.WriteString("**作为乐观方，您应该关注**:\n")
			if len(analysis.SuccessPatterns) > 0 {
				sb.WriteString("✅ **成功模式** (支持您的观点):\n")
				for i, pattern := range analysis.SuccessPatterns {
					if i >= 2 {
						break
					}
					sb.WriteString(fmt.Sprintf("- %s (%.2f%% 平均盈利)\n", pattern.Description, pattern.AvgPnLPct))
				}
			}
			sb.WriteString(fmt.Sprintf("\n当前总回报: %.2f%%, 胜率: %.1f%%\n", analysis.TotalReturnPct, analysis.WinRate))
		} else if agentRole == "bear" || agentRole == "skeptical" {
			sb.WriteString("**作为谨慎方，您应该关注**:\n")
			if len(analysis.FailurePatterns) > 0 {
				sb.WriteString("⚠️ **失败模式** (支持您的警告):\n")
				for i, pattern := range analysis.FailurePatterns {
					if i >= 2 {
						break
					}
					sb.WriteString(fmt.Sprintf("- %s (%.2f%% 平均亏损)\n", pattern.Description, pattern.AvgPnLPct))
				}
			}
			sb.WriteString(fmt.Sprintf("\n最大回撤: %.1f%%, 盈利因子: %.2f\n", analysis.MaxDrawdown, analysis.ProfitFactor))
		} else {
			// Neutral/synthesis agent - sees both sides
			sb.WriteString("**综合双方观点，平衡考虑**:\n")
			sb.WriteString(fmt.Sprintf("表现: 回报 %.2f%%, 胜率 %.1f%%, 回撤 %.1f%%\n\n",
				analysis.TotalReturnPct, analysis.WinRate, analysis.MaxDrawdown))
		}
	} else {
		sb.WriteString("## 🎯 Debate Evidence: Historical Performance Analysis\n\n")

		// Tailor feedback based on agent role
		if agentRole == "bull" || agentRole == "optimistic" {
			sb.WriteString("**As the optimistic side, focus on**:\n")
			if len(analysis.SuccessPatterns) > 0 {
				sb.WriteString("✅ **Success Patterns** (support your position):\n")
				for i, pattern := range analysis.SuccessPatterns {
					if i >= 2 {
						break
					}
					sb.WriteString(fmt.Sprintf("- %s (%.2f%% avg profit)\n", pattern.Description, pattern.AvgPnLPct))
				}
			}
			sb.WriteString(fmt.Sprintf("\nCurrent total return: %.2f%%, win rate: %.1f%%\n", analysis.TotalReturnPct, analysis.WinRate))
		} else if agentRole == "bear" || agentRole == "skeptical" {
			sb.WriteString("**As the cautious side, focus on**:\n")
			if len(analysis.FailurePatterns) > 0 {
				sb.WriteString("⚠️ **Failure Patterns** (support your warnings):\n")
				for i, pattern := range analysis.FailurePatterns {
					if i >= 2 {
						break
					}
					sb.WriteString(fmt.Sprintf("- %s (%.2f%% avg loss)\n", pattern.Description, pattern.AvgPnLPct))
				}
			}
			sb.WriteString(fmt.Sprintf("\nMax drawdown: %.1f%%, profit factor: %.2f\n", analysis.MaxDrawdown, analysis.ProfitFactor))
		} else {
			// Neutral/synthesis agent - sees both sides
			sb.WriteString("**Synthesize both perspectives, balanced view**:\n")
			sb.WriteString(fmt.Sprintf("Performance: %.2f%% return, %.1f%% win rate, %.1f%% drawdown\n\n",
				analysis.TotalReturnPct, analysis.WinRate, analysis.MaxDrawdown))
		}
	}

	return sb.String()
}

// ============================================================================
// Concise prompt Helpers
// ============================================================================

// prioritizeFeedback extracts only the most critical feedback items
func (fg *FeedbackGenerator) prioritizeFeedback(analysis *FeedbackAnalysis) *PrioritizedFeedback {
	pf := &PrioritizedFeedback{}

	// CRITICAL: Issues causing direct losses
	for _, pattern := range analysis.FailurePatterns {
		if pattern.AvgPnLPct < -3.0 && pattern.Frequency >= 3 {
			pf.CriticalFailures = append(pf.CriticalFailures, pattern)
			if len(pf.CriticalFailures) >= 2 {
				break
			}
		}
	}

	// HIGH: Issues hurting performance but not catastrophic
	for _, pattern := range analysis.FailurePatterns {
		if pattern.AvgPnLPct < -1.5 && pattern.Frequency >= 2 {
			pf.HighFailures = append(pf.HighFailures, pattern)
			if len(pf.HighFailures) >= 3 {
				break
			}
		}
	}

	// SUCCESS: Replicable winning patterns
	for _, pattern := range analysis.SuccessPatterns {
		if pattern.AvgPnLPct > 2.0 && pattern.Frequency >= 3 {
			pf.SuccessPatterns = append(pf.SuccessPatterns, pattern)
			if len(pf.SuccessPatterns) >= 2 {
				break
			}
		}
	}

	// Get key metrics summary
	pf.Metrics = fg.summarizeMetrics(analysis)

	// Get top 2 actionable recommendations
	pf.Actions = fg.getTopActions(analysis)

	return pf
}

type PrioritizedFeedback struct {
	CriticalFailures []TradingPattern // Must fix NOW
	HighFailures     []TradingPattern // Important to fix
	SuccessPatterns  []TradingPattern // Replicate these
	Metrics          MetricsSummary
	Actions          []string
}

type MetricsSummary struct {
	ReturnStatus       string // "CRITICAL", "POOR", "OK", "GOOD", "EXCELLENT"
	WinRateStatus      string
	ProfitFactorStatus string
	DrawdownStatus     string
	TradesPerHour      float64
}

func (fg *FeedbackGenerator) formatENConcise(sb *strings.Builder, pf *PrioritizedFeedback, analysis *FeedbackAnalysis) {
	// === URGENT SECTION (MAX 3 LINES) ===
	sb.WriteString("🚨 **PERFORMANCE FEEDBACK (CRITICAL)**\n\n")

	// 1-line performance summary
	sb.WriteString(fg.oneLineSummary(analysis))
	sb.WriteString("\n\n")

	// === CRITICAL FAILURES (MAX 2) ===
	if len(pf.CriticalFailures) > 0 {
		sb.WriteString("**🔥 MUST FIX IMMEDIATELY:**\n")
		for i, pattern := range pf.CriticalFailures {
			sb.WriteString(fmt.Sprintf("%d. %s (Avg loss: %.1f%%, Occurred: %dx)\n",
				i+1, pattern.Description, pattern.AvgPnLPct, pattern.Frequency))
			sb.WriteString(fmt.Sprintf("   → %s\n", pattern.Recommendation))
			if i >= 1 {
				break
			} // Max 2
		}
		sb.WriteString("\n")
	}

	// === EXECUTION DIAGNOSTICS (MAX 3) ===
	// Focus on microstructure failures if present
	v2Failures := fg.extractV2Failures(pf.HighFailures)
	if len(v2Failures) > 0 {
		sb.WriteString("**🔍 EXECUTION ISSUES:**\n")
		for i, pattern := range v2Failures {
			sb.WriteString(fmt.Sprintf("• %s\n", pattern.Description))
			if i >= 2 {
				break
			} // Max 3
		}
		sb.WriteString("\n")
	}

	// === SUCCESS PATTERNS TO REPLICATE (MAX 2) ===
	if len(pf.SuccessPatterns) > 0 {
		sb.WriteString("**✅ REPLICATE THESE WINNERS:**\n")
		for i, pattern := range pf.SuccessPatterns {
			sb.WriteString(fmt.Sprintf("%d. %s (Avg gain: %.1f%%)\n",
				i+1, pattern.Description, pattern.AvgPnLPct))
			sb.WriteString(fmt.Sprintf("   → %s\n", pattern.Recommendation))
			if i >= 1 {
				break
			} // Max 2
		}
		sb.WriteString("\n")
	}

	// === QUANTITATIVE RULES (CONDENSE TO 5 MAX) ===
	sb.WriteString("**📊 ENFORCE THESE RULES:**\n")
	rules := fg.getEssentialRules(analysis)
	for i, rule := range rules {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, rule))
		if i >= 4 {
			break
		} // Max 5 rules
	}
	sb.WriteString("\n")

	// === FEW-SHOT EXAMPLES (1 WIN, 1 LOSS ONLY) ===
	sb.WriteString("**📚 LEARN FROM THESE 2 TRADES:**\n")

	// One winning example
	if len(analysis.TopWinningTrades) > 0 {
		trade := analysis.TopWinningTrades[0]
		sb.WriteString(fmt.Sprintf("✅ WIN: %s %s (+%.1f%% in %s)\n",
			trade.Symbol, trade.Action, trade.RealizedPnLPct, trade.HoldDuration))
		if trade.Reasoning != "" && len(trade.Reasoning) < 100 {
			sb.WriteString(fmt.Sprintf("   Reason: %s\n", truncate(trade.Reasoning, 80)))
		}
	}

	// One losing example with root cause
	if len(analysis.TopLosingTrades) > 0 {
		trade := analysis.TopLosingTrades[0]
		sb.WriteString(fmt.Sprintf("❌ LOSS: %s %s (%.1f%% in %s)\n",
			trade.Symbol, trade.Action, trade.RealizedPnLPct, trade.HoldDuration))

		// Add root cause if available from V2 analysis
		if trade.RecentOrder != nil {
			failure := decision.AnalyzeFailedTrade(trade.RecentOrder)
			if failure != nil && failure.ConfidenceScore > 0.7 {
				sb.WriteString(fmt.Sprintf("   Root cause: %s\n",
					truncate(failure.DetailedNotes, 60)))
			}
		}
	}
	sb.WriteString("\n")

	// === MARKET CONTEXT ===
	sb.WriteString(fmt.Sprintf("**📈 MARKET CONDITION:** %s\n",
		truncate(analysis.MarketConditions, 80)))

	// === MEMORY AID (CRITICAL ONLY) ===
	sb.WriteString("\n**💡 CRITICAL TAKEAWAY:** ")
	sb.WriteString(fg.getCriticalTakeaway(pf, analysis))
}

func (fg *FeedbackGenerator) formatZHConcise(sb *strings.Builder, pf *PrioritizedFeedback, analysis *FeedbackAnalysis) {
	sb.WriteString("🚨 **表现反馈 (核心摘要)**\n\n")

	// 1-line summary
	sb.WriteString(fg.oneLineSummaryZH(analysis))
	sb.WriteString("\n\n")

	if len(pf.CriticalFailures) > 0 {
		sb.WriteString("**🔥 必须立即修复:**\n")
		for i, pattern := range pf.CriticalFailures {
			sb.WriteString(fmt.Sprintf("%d. %s (平均亏损: %.1f%%, 出现: %d次)\n",
				i+1, pattern.Description, pattern.AvgPnLPct, pattern.Frequency))
			sb.WriteString(fmt.Sprintf("   → %s\n", pattern.Recommendation))
			if i >= 1 {
				break
			}
		}
		sb.WriteString("\n")
	}

	// Success patterns
	if len(pf.SuccessPatterns) > 0 {
		sb.WriteString("**✅ 复制这些成功模式:**\n")
		for i, pattern := range pf.SuccessPatterns {
			sb.WriteString(fmt.Sprintf("%d. %s (平均盈利: %.1f%%)\n",
				i+1, pattern.Description, pattern.AvgPnLPct))
			sb.WriteString(fmt.Sprintf("   → %s\n", pattern.Recommendation))
			if i >= 1 {
				break
			}
		}
		sb.WriteString("\n")
	}

	// Essential rules
	sb.WriteString("**📊 强制执行这些规则:**\n")
	rules := fg.getEssentialRulesZH(analysis)
	for i, rule := range rules {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, rule))
		if i >= 4 {
			break
		}
	}

	sb.WriteString(fmt.Sprintf("\n**📈 市场状况:** %s\n",
		truncate(analysis.MarketConditions, 60)))
}

// Helper functions for concise formatting
func (fg *FeedbackGenerator) oneLineSummary(analysis *FeedbackAnalysis) string {
	var status string
	if analysis.TotalReturnPct < -10 {
		status = "🔴 CRITICAL LOSS"
	} else if analysis.TotalReturnPct < 0 {
		status = "⚠️ NEGATIVE"
	} else if analysis.TotalReturnPct < 5 {
		status = "🟡 NEUTRAL"
	} else {
		status = "✅ PROFITABLE"
	}

	return fmt.Sprintf("%s | Return: %.1f%% | Win Rate: %.0f%% | Trades/Hour: %.1f",
		status, analysis.TotalReturnPct, analysis.WinRate, analysis.TradesPerHour)
}

func (fg *FeedbackGenerator) oneLineSummaryZH(analysis *FeedbackAnalysis) string {
	var status string
	if analysis.TotalReturnPct < -10 {
		status = "🔴 严重亏损"
	} else if analysis.TotalReturnPct < 0 {
		status = "⚠️ 负收益"
	} else if analysis.TotalReturnPct < 5 {
		status = "🟡 中性"
	} else {
		status = "✅ 盈利"
	}

	return fmt.Sprintf("%s | 回报: %.1f%% | 胜率: %.0f%% | 交易频率: %.1f笔/小时",
		status, analysis.TotalReturnPct, analysis.WinRate, analysis.TradesPerHour)
}

func (fg *FeedbackGenerator) extractV2Failures(patterns []TradingPattern) []TradingPattern {
	var v2 []TradingPattern
	for _, p := range patterns {
		// Check if it's a V2 microstructure failure
		if strings.Contains(p.PatternType, "_") ||
			strings.Contains(p.Description, "Execution-level") ||
			strings.Contains(p.Description, "slippage") ||
			strings.Contains(p.Description, "spread") ||
			strings.Contains(p.Description, "ATR") {
			v2 = append(v2, p)
		}
	}
	return v2
}

func (fg *FeedbackGenerator) getEssentialRules(analysis *FeedbackAnalysis) []string {
	rules := []string{}

	// Always include these core rules
	rules = append(rules, "Max 2 trades/hour, 8 trades/day")
	rules = append(rules, "Minimum 30min between trades")

	// Add context-specific rules
	if analysis.TradesPerHour > 1.5 {
		rules = append(rules, "⚠️ You are overtrading - reduce frequency by 50%")
	}

	if analysis.WinRate < 40 {
		rules = append(rules, "Only take trades with ≥70% confidence")
	}

	if analysis.TotalReturnPct < 0 {
		rules = append(rules, "Reduce position size by 30% until profitable")
	}

	// Add rule from most critical failure
	if len(analysis.FailurePatterns) > 0 {
		topFailure := analysis.FailurePatterns[0]
		// Extract the core rule from recommendation
		if strings.Contains(topFailure.Recommendation, "ENFORCE:") {
			rule := strings.Replace(topFailure.Recommendation, "ENFORCE:", "", 1)
			rules = append(rules, strings.TrimSpace(rule))
		}
	}

	return rules[:min(5, len(rules))]
}

func (fg *FeedbackGenerator) getEssentialRulesZH(analysis *FeedbackAnalysis) []string {
	rules := []string{}

	rules = append(rules, "每小时最多2笔交易, 每天最多8笔")
	rules = append(rules, "交易间隔至少30分钟")

	if analysis.TradesPerHour > 1.5 {
		rules = append(rules, "⚠️ 交易过度 - 频率减少50%")
	}

	if analysis.WinRate < 40 {
		rules = append(rules, "只做信心度≥70%的交易")
	}

	if analysis.TotalReturnPct < 0 {
		rules = append(rules, "仓位减少30%直到盈利")
	}

	return rules[:min(5, len(rules))]
}

func (fg *FeedbackGenerator) getCriticalTakeaway(pf *PrioritizedFeedback, analysis *FeedbackAnalysis) string {
	if len(pf.CriticalFailures) > 0 {
		return fmt.Sprintf("Fix '%s' first - it's causing %.1f%% average losses",
			pf.CriticalFailures[0].Description, pf.CriticalFailures[0].AvgPnLPct)
	}

	if analysis.TotalReturnPct < -5 {
		return "Strategy is losing money. Reduce trading frequency and position sizes immediately."
	}

	if analysis.WinRate > 55 && analysis.ProfitFactor > 1.5 {
		return "Strategy is working. Continue but stay disciplined with position sizing."
	}

	return "Focus on quality over quantity. Wait for high-conviction setups only."
}

func (fg *FeedbackGenerator) summarizeMetrics(analysis *FeedbackAnalysis) MetricsSummary {
	ms := MetricsSummary{
		TradesPerHour: analysis.TradesPerHour,
	}

	// Return status
	if analysis.TotalReturnPct < -10 {
		ms.ReturnStatus = "CRITICAL"
	} else if analysis.TotalReturnPct < 0 {
		ms.ReturnStatus = "POOR"
	} else if analysis.TotalReturnPct < 5 {
		ms.ReturnStatus = "OK"
	} else if analysis.TotalReturnPct < 15 {
		ms.ReturnStatus = "GOOD"
	} else {
		ms.ReturnStatus = "EXCELLENT"
	}

	// Win rate status
	if analysis.WinRate < 40 {
		ms.WinRateStatus = "POOR"
	} else if analysis.WinRate < 50 {
		ms.WinRateStatus = "OK"
	} else if analysis.WinRate < 60 {
		ms.WinRateStatus = "GOOD"
	} else {
		ms.WinRateStatus = "EXCELLENT"
	}

	// Profit factor status
	if analysis.ProfitFactor < 1.0 {
		ms.ProfitFactorStatus = "CRITICAL"
	} else if analysis.ProfitFactor < 1.2 {
		ms.ProfitFactorStatus = "POOR"
	} else if analysis.ProfitFactor < 1.5 {
		ms.ProfitFactorStatus = "OK"
	} else if analysis.ProfitFactor < 2.0 {
		ms.ProfitFactorStatus = "GOOD"
	} else {
		ms.ProfitFactorStatus = "EXCELLENT"
	}

	// Drawdown status
	if analysis.MaxDrawdown > CriticalDrawdownThreshold {
		ms.DrawdownStatus = "CRITICAL"
	} else if analysis.MaxDrawdown > DefaultMaxDrawdownPct {
		ms.DrawdownStatus = "HIGH"
	} else if analysis.MaxDrawdown > DefaultDrawdownWarningLevel {
		ms.DrawdownStatus = "MODERATE"
	} else {
		ms.DrawdownStatus = "LOW"
	}

	return ms
}

func (fg *FeedbackGenerator) getTopActions(analysis *FeedbackAnalysis) []string {
	actions := []string{}

	// Always include if losing money
	if analysis.TotalReturnPct < 0 {
		actions = append(actions, "Reduce position sizes by 30%")
	}

	// Include from failure patterns
	for _, pattern := range analysis.FailurePatterns {
		if pattern.AvgPnLPct < -2.0 {
			// Extract action from recommendation
			if strings.Contains(pattern.Recommendation, "ENFORCE:") {
				action := strings.Replace(pattern.Recommendation, "ENFORCE:", "Implement:", 1)
				actions = append(actions, action)
			}
			if len(actions) >= 2 {
				break
			}
		}
	}

	return actions[:min(2, len(actions))]
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ============================================================
// ADDITIONAL: Real-time feedback for immediate use
// ============================================================

// GetImmediateFeedback returns feedback for the MOST RECENT trade only
func (fg *FeedbackGenerator) GetImmediateFeedback(lastTrade DecisionOutcome, lang string) string {
	var sb strings.Builder

	if lang == "zh" {
		fg.formatImmediateFeedbackZH(&sb, lastTrade)
	} else {
		fg.formatImmediateFeedbackEN(&sb, lastTrade)
	}

	return sb.String()
}

func (fg *FeedbackGenerator) formatImmediateFeedbackEN(sb *strings.Builder, lastTrade DecisionOutcome) {
	sb.WriteString("🔍 **LAST TRADE DIAGNOSIS:**\n\n")

	if lastTrade.Success {
		sb.WriteString(fmt.Sprintf("✅ WIN: +%.1f%% in %s\n",
			lastTrade.RealizedPnLPct, lastTrade.HoldDuration))

		// What worked well
		if lastTrade.HoldDuration < "30m" {
			sb.WriteString("   Quick profit capture - good execution\n")
		}
		if lastTrade.Leverage <= 3 {
			sb.WriteString("   Appropriate leverage used\n")
		}
		if lastTrade.RealizedPnLPct > 3.0 {
			sb.WriteString("   Strong profit - good trade management\n")
		}
	} else {
		sb.WriteString(fmt.Sprintf("❌ LOSS: %.1f%% in %s\n",
			lastTrade.RealizedPnLPct, lastTrade.HoldDuration))

		// Root cause analysis using V2
		if lastTrade.RecentOrder != nil {
			failure := decision.AnalyzeFailedTrade(lastTrade.RecentOrder)
			if failure != nil && failure.ConfidenceScore > 0.6 {
				sb.WriteString(fmt.Sprintf("   Root cause: %s\n",
					humanizeV2ReasonEN(failure.PrimaryReason)))

				// Add V2-specific details
				if failure.DetailedNotes != "" {
					note := truncate(failure.DetailedNotes, 100)
					sb.WriteString(fmt.Sprintf("   Details: %s\n", note))
				}
			}
		}

		// Behavioral warnings
		if lastTrade.HoldDuration < "15m" {
			sb.WriteString("   ⚠️ Stopped out quickly - poor entry timing\n")
		} else if lastTrade.HoldDuration > "4h" {
			sb.WriteString("   ⚠️ Held loser too long - cut losses faster\n")
		}

		if math.Abs(lastTrade.RealizedPnLPct) > 5 {
			sb.WriteString("   ⚠️ Large loss - stop loss too wide\n")
		}

		if lastTrade.Leverage >= 5 && math.Abs(lastTrade.RealizedPnLPct) > 2 {
			sb.WriteString("   ⚠️ High leverage amplified loss\n")
		}
	}

	// Add execution quality assessment
	if lastTrade.RecentOrder != nil {
		sb.WriteString(fg.assessExecutionQuality(lastTrade.RecentOrder))
	}

	// One specific action for next trade
	sb.WriteString("\n**NEXT TRADE ACTION:** ")
	sb.WriteString(fg.getNextTradeAction(lastTrade))
}

func (fg *FeedbackGenerator) formatImmediateFeedbackZH(sb *strings.Builder, lastTrade DecisionOutcome) {
	sb.WriteString("🔍 **最近交易诊断:**\n\n")

	if lastTrade.Success {
		sb.WriteString(fmt.Sprintf("✅ 盈利: +%.1f%% (持仓: %s)\n",
			lastTrade.RealizedPnLPct, lastTrade.HoldDuration))

		// 成功因素分析
		if lastTrade.HoldDuration < "30m" {
			sb.WriteString("   快速获利了结 - 执行良好\n")
		}
		if lastTrade.Leverage <= 3 {
			sb.WriteString("   杠杆使用恰当\n")
		}
		if lastTrade.RealizedPnLPct > 3.0 {
			sb.WriteString("   强劲利润 - 交易管理优秀\n")
		}

		// 检查是否有改进空间
		if lastTrade.RecentOrder != nil && lastTrade.RecentOrder.GiveBackFromPeak > 0 {
			sb.WriteString(fmt.Sprintf("   ⚠️ 利润回吐: %.2f USDT\n",
				lastTrade.RecentOrder.GiveBackFromPeak))
		}
	} else {
		sb.WriteString(fmt.Sprintf("❌ 亏损: %.1f%% (持仓: %s)\n",
			lastTrade.RealizedPnLPct, lastTrade.HoldDuration))

		// V2根因分析
		if lastTrade.RecentOrder != nil {
			failure := decision.AnalyzeFailedTrade(lastTrade.RecentOrder)
			if failure != nil && failure.ConfidenceScore > 0.6 {
				sb.WriteString(fmt.Sprintf("   失败原因: %s\n",
					humanizeV2ReasonZH(failure.PrimaryReason)))

				// 添加详细说明
				if failure.DetailedNotes != "" {
					note := truncate(failure.DetailedNotes, 80)
					sb.WriteString(fmt.Sprintf("   详细说明: %s\n", note))
				}
			}
		}

		// 行为警告
		if lastTrade.HoldDuration < "15m" {
			sb.WriteString("   ⚠️ 过早被止损 - 入场时机不佳\n")
		} else if lastTrade.HoldDuration > "4h" {
			sb.WriteString("   ⚠️ 持仓亏损过久 - 应更快止损\n")
		}

		if math.Abs(lastTrade.RealizedPnLPct) > 5 {
			sb.WriteString("   ⚠️ 亏损过大 - 止损设置太宽\n")
		}

		if lastTrade.Leverage >= 5 && math.Abs(lastTrade.RealizedPnLPct) > 2 {
			sb.WriteString("   ⚠️ 高杠杆放大了亏损\n")
		}
	}

	// 执行质量评估
	if lastTrade.RecentOrder != nil {
		sb.WriteString(fg.assessExecutionQualityZH(lastTrade.RecentOrder))
	}

	// 下一交易具体行动
	sb.WriteString("\n**下一交易行动:** ")
	sb.WriteString(fg.getNextTradeActionZH(lastTrade))
}

func (fg *FeedbackGenerator) assessExecutionQuality(order *decision.RecentOrder) string {
	if order == nil {
		return ""
	}

	var issues []string

	// Check slippage
	if order.EntrySlippage > 0.001 { // 0.1%
		issues = append(issues, fmt.Sprintf("High entry slippage: %.3f%%", order.EntrySlippage*100))
	}

	if order.ExitSlippage > 0.001 {
		issues = append(issues, fmt.Sprintf("High exit slippage: %.3f%%", order.ExitSlippage*100))
	}

	// Check spread
	if order.EntrySpread > 0.002 { // 0.2%
		issues = append(issues, fmt.Sprintf("Wide entry spread: %.3f%%", order.EntrySpread*100))
	}

	// Check fill time
	if order.EntryFillTime > 2000 { // 2 seconds
		issues = append(issues, fmt.Sprintf("Slow fill: %dms", order.EntryFillTime))
	}

	if len(issues) == 0 {
		return "   Execution quality: Good ✓\n"
	}

	return fmt.Sprintf("   ⚠️ Execution issues: %s\n", strings.Join(issues, ", "))
}

func (fg *FeedbackGenerator) assessExecutionQualityZH(order *decision.RecentOrder) string {
	if order == nil {
		return ""
	}

	var issues []string

	// 检查滑点
	if order.EntrySlippage > 0.001 { // 0.1%
		issues = append(issues, fmt.Sprintf("入场滑点过高: %.3f%%", order.EntrySlippage*100))
	}

	if order.ExitSlippage > 0.001 {
		issues = append(issues, fmt.Sprintf("出场滑点过高: %.3f%%", order.ExitSlippage*100))
	}

	// 检查点差
	if order.EntrySpread > 0.002 { // 0.2%
		issues = append(issues, fmt.Sprintf("入场点差过大: %.3f%%", order.EntrySpread*100))
	}

	// 检查成交时间
	if order.EntryFillTime > 2000 { // 2秒
		issues = append(issues, fmt.Sprintf("成交缓慢: %dms", order.EntryFillTime))
	}

	if len(issues) == 0 {
		return "   执行质量: 良好 ✓\n"
	}

	return fmt.Sprintf("   ⚠️ 执行问题: %s\n", strings.Join(issues, ", "))
}

func (fg *FeedbackGenerator) getNextTradeAction(lastTrade DecisionOutcome) string {
	actions := []string{}

	if !lastTrade.Success {
		// Loss-specific actions
		if lastTrade.HoldDuration < "15m" {
			actions = append(actions, "Wait for stronger confirmation before entering")
		}
		if math.Abs(lastTrade.RealizedPnLPct) > 5 {
			actions = append(actions, "Set tighter stop-loss (max 3%)")
		}
		if lastTrade.Leverage >= 5 {
			actions = append(actions, "Reduce leverage to 3x maximum")
		}

		// Always include after loss
		if len(actions) == 0 {
			actions = append(actions, "Wait 30+ minutes before next trade to avoid revenge trading")
		}
	} else {
		// Win-specific actions
		if lastTrade.RealizedPnLPct < 2.0 && lastTrade.HoldDuration > "1h" {
			actions = append(actions, "Consider taking profits earlier on similar setups")
		}
		if lastTrade.Leverage >= 5 {
			actions = append(actions, "Consider reducing leverage to lock in profits")
		}

		// Always include after win
		if len(actions) == 0 {
			actions = append(actions, "Maintain discipline - don't increase position size")
		}
	}

	// Get context-specific action from V2 analysis
	if lastTrade.RecentOrder != nil {
		v2Action := fg.getV2SpecificAction(lastTrade.RecentOrder)
		if v2Action != "" {
			actions = append([]string{v2Action}, actions...)
		}
	}

	return actions[0] // Return the most specific action
}

func (fg *FeedbackGenerator) getNextTradeActionZH(lastTrade DecisionOutcome) string {
	actions := []string{}

	if !lastTrade.Success {
		// 亏损后特定行动
		if lastTrade.HoldDuration < "15m" {
			actions = append(actions, "等待更强的确认信号再入场")
		}
		if math.Abs(lastTrade.RealizedPnLPct) > 5 {
			actions = append(actions, "设置更紧的止损（最大3%）")
		}
		if lastTrade.Leverage >= 5 {
			actions = append(actions, "降低杠杆至最高3倍")
		}

		// 亏损后总是包含的
		if len(actions) == 0 {
			actions = append(actions, "等待30分钟以上再进行下一笔交易，避免报复性交易")
		}
	} else {
		// 盈利后特定行动
		if lastTrade.RealizedPnLPct < 2.0 && lastTrade.HoldDuration > "1h" {
			actions = append(actions, "考虑在类似设置中更早获利了结")
		}
		if lastTrade.Leverage >= 5 {
			actions = append(actions, "考虑降低杠杆以锁定利润")
		}

		// 盈利后总是包含的
		if len(actions) == 0 {
			actions = append(actions, "保持纪律 - 不要增加仓位大小")
		}
	}

	// 从V2分析获取特定行动
	if lastTrade.RecentOrder != nil {
		v2Action := fg.getV2SpecificActionZH(lastTrade.RecentOrder)
		if v2Action != "" {
			actions = append([]string{v2Action}, actions...)
		}
	}

	return actions[0]
}

func (fg *FeedbackGenerator) getV2SpecificAction(order *decision.RecentOrder) string {
	if order == nil {
		return ""
	}

	failure := decision.AnalyzeFailedTrade(order)
	if failure == nil {
		return ""
	}

	switch failure.PrimaryReason {
	case decision.ReasonChasingEntry:
		return "Use limit orders instead of market orders for entries"
	case decision.ReasonStopTooTight:
		return "Set stop-loss at least 1.5x ATR from entry"
	case decision.ReasonSlippageExceeded:
		return "Reduce position size or trade during higher liquidity"
	case decision.ReasonRegimeMismatch:
		return "Check market regime before entering (avoid chop)"
	case decision.ReasonLiquidityRiskHigh:
		return "Avoid trading during low-volume periods"
	case decision.ReasonLateExitGiveBack:
		return "Set profit targets and stick to them"
	default:
		return ""
	}
}

func (fg *FeedbackGenerator) getV2SpecificActionZH(order *decision.RecentOrder) string {
	if order == nil {
		return ""
	}

	failure := decision.AnalyzeFailedTrade(order)
	if failure == nil {
		return ""
	}

	switch failure.PrimaryReason {
	case decision.ReasonChasingEntry:
		return "使用限价单而非市价单入场"
	case decision.ReasonStopTooTight:
		return "设置止损至少距离入场价1.5倍ATR"
	case decision.ReasonSlippageExceeded:
		return "减小仓位或在高流动性时段交易"
	case decision.ReasonRegimeMismatch:
		return "入场前检查市场状态（避免震荡市）"
	case decision.ReasonLiquidityRiskHigh:
		return "避免在低成交量时段交易"
	case decision.ReasonLateExitGiveBack:
		return "设置盈利目标并严格执行"
	default:
		return ""
	}
}
