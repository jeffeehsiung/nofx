package backtest

import (
	"encoding/json"
	"fmt"
	"nofx/decision"
	"nofx/logger"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================================
// Reinforcement Learning Compliance Tracker
// ============================================================================
// Tracks whether the LLM follows feedback recommendations
// Provides rewards/penalties to reinforce good behavior
// ============================================================================

// ComplianceRecord tracks if a recommendation was followed
type ComplianceRecord struct {
	Timestamp      time.Time `json:"timestamp"`
	Cycle          int       `json:"cycle"`
	Recommendation string    `json:"recommendation"`
	Expected       string    `json:"expected"`      // What should have been done
	Actual         string    `json:"actual"`        // What was actually done
	Compliant      bool      `json:"compliant"`     // Did it follow recommendation?
	RewardPoints   float64   `json:"reward_points"` // Reward (positive) or penalty (negative)
}

// ComplianceTracker tracks LLM compliance with recommendations
type ComplianceTracker struct {
	activeRecommendations []string // Current active recommendations
	complianceHistory     []*ComplianceRecord

	// Aggregated metrics
	totalRecommendations int
	compliantActions     int
	complianceRate       float64
	totalRewardPoints    float64

	// Configuration
	config *ComplianceConfig
}

// ComplianceConfig controls compliance tracking
type ComplianceConfig struct {
	EnableTracking        bool    `json:"enable_tracking"`
	RewardForCompliance   float64 `json:"reward_for_compliance"`    // Points for following
	PenaltyForViolation   float64 `json:"penalty_for_violation"`    // Points for not following
	ComplianceDecayRate   float64 `json:"compliance_decay_rate"`    // How fast old records decay
	MinComplianceForBoost float64 `json:"min_compliance_for_boost"` // Min rate for positive boost
}

// DefaultComplianceConfig returns default configuration
func DefaultComplianceConfig() *ComplianceConfig {
	return &ComplianceConfig{
		EnableTracking:        true,
		RewardForCompliance:   1.0,
		PenaltyForViolation:   -0.5,
		ComplianceDecayRate:   0.95,
		MinComplianceForBoost: 0.7,
	}
}

// NewComplianceTracker creates a new compliance tracker
func NewComplianceTracker(config *ComplianceConfig) *ComplianceTracker {
	if config == nil {
		config = DefaultComplianceConfig()
	}

	ct := &ComplianceTracker{
		activeRecommendations: make([]string, 0),
		complianceHistory:     make([]*ComplianceRecord, 0),
		config:                config,
	}

	logger.Infof("[ComplianceTracker] Initialized reinforcement learning tracker")

	return ct
}

// SetRecommendations updates the active recommendations from feedback
func (ct *ComplianceTracker) SetRecommendations(recommendations []string) {
	ct.activeRecommendations = recommendations
	logger.Infof("[ComplianceTracker] 📋 Updated %d active recommendations", len(recommendations))
}

// CheckCompliance evaluates if a decision followed recommendations
func (ct *ComplianceTracker) CheckCompliance(cycle int, decision *decision.Decision, feedback *FeedbackAnalysis) {
	if !ct.config.EnableTracking || feedback == nil {
		return
	}

	// Check compliance with each active recommendation
	for _, rec := range ct.activeRecommendations {
		compliant, rewardPoints := ct.evaluateRecommendation(rec, decision, feedback)

		record := &ComplianceRecord{
			Timestamp:      time.Now(),
			Cycle:          cycle,
			Recommendation: rec,
			Expected:       ct.extractExpectation(rec),
			Actual:         ct.extractActual(decision),
			Compliant:      compliant,
			RewardPoints:   rewardPoints,
		}

		ct.complianceHistory = append(ct.complianceHistory, record)
		ct.totalRecommendations++
		if compliant {
			ct.compliantActions++
		}
		ct.totalRewardPoints += rewardPoints

		// Log important violations
		if !compliant && strings.Contains(rec, "CRITICAL") {
			logger.Warnf("[ComplianceTracker] ⚠️ CRITICAL recommendation violated at cycle %d: %s", cycle, rec)
		}
	}

	// Update compliance rate
	if ct.totalRecommendations > 0 {
		ct.complianceRate = float64(ct.compliantActions) / float64(ct.totalRecommendations)
	}
}

// evaluateRecommendation checks if a specific recommendation was followed
func (ct *ComplianceTracker) evaluateRecommendation(rec string, dec *decision.Decision, feedback *FeedbackAnalysis) (bool, float64) {
	recLower := strings.ToLower(rec)

	// Use feedback patterns to adjust reward scaling
	// If following patterns that led to success, increase rewards
	rewardMultiplier := 1.0
	if feedback != nil {
		// Check if current decision aligns with success patterns
		for _, pattern := range feedback.SuccessPatterns {
			if pattern.AvgPnLPct > 0 && ct.decisionMatchesPattern(dec, pattern) {
				rewardMultiplier = 1.5 // Boost reward for following winning patterns
				break
			}
		}
		// Check if current decision matches failure patterns (reduce reward)
		for _, pattern := range feedback.FailurePatterns {
			if pattern.AvgPnLPct < 0 && ct.decisionMatchesPattern(dec, pattern) {
				rewardMultiplier = 0.7 // Reduce reward if still following losing patterns
				break
			}
		}
	}

	// 1. Check leverage recommendations
	if strings.Contains(recLower, "lower leverage") || strings.Contains(recLower, "reduce leverage") ||
		(strings.Contains(recLower, "leverage") && (strings.Contains(recLower, "max") || strings.Contains(recLower, "<=") || strings.Contains(recLower, "≤") || strings.Contains(recLower, "2-3x") || strings.Contains(recLower, "3x"))) {
		if dec.Leverage <= 3 {
			return true, ct.config.RewardForCompliance * rewardMultiplier
		}
		return false, ct.config.PenaltyForViolation
	}

	// 2. Check position size recommendations
	if strings.Contains(recLower, "reduce position size") || strings.Contains(recLower, "smaller position") ||
		(strings.Contains(recLower, "position size") && (strings.Contains(recLower, "<=") || strings.Contains(recLower, "≤") || strings.Contains(recLower, "max") || strings.Contains(recLower, "$"))) {
		// Position size is stored as PositionSizeUSD in Decision struct
		if dec.PositionSizeUSD <= 200 {
			return true, ct.config.RewardForCompliance * rewardMultiplier
		}
		return false, ct.config.PenaltyForViolation
	}

	// 3. Check confidence recommendations
	if strings.Contains(recLower, "70%+ confidence") || strings.Contains(recLower, "high-confidence") {
		if dec.Confidence >= 70 {
			return true, ct.config.RewardForCompliance * rewardMultiplier
		}
		return false, ct.config.PenaltyForViolation
	}

	// 4. Check hold action recommendations (from overtrading pattern)
	if strings.Contains(recLower, "reduce trading frequency") || strings.Contains(recLower, "reduce frequency") ||
		strings.Contains(recLower, "cooldown") || strings.Contains(recLower, "cool-down") ||
		strings.Contains(recLower, "cool down") || strings.Contains(recLower, "wait") ||
		strings.Contains(recLower, "pause") || strings.Contains(recLower, "break") {
		if dec.Action == "hold" {
			return true, ct.config.RewardForCompliance * rewardMultiplier
		}
		return false, ct.config.PenaltyForViolation
	}

	// 5. Check stop-loss discipline
	if strings.Contains(recLower, "discipline on stop-losses") || strings.Contains(recLower, "respect stop") {
		// This would require tracking if stops were hit and followed
		// For now, give partial credit if stop-loss is set
		return true, ct.config.RewardForCompliance * 0.5 * rewardMultiplier
	}

	// 6. Check selective trading
	if strings.Contains(recLower, "selective trading") || strings.Contains(recLower, "only take trades") {
		if dec.Action == "hold" || dec.Confidence >= 70 {
			return true, ct.config.RewardForCompliance * rewardMultiplier
		}
		return false, ct.config.PenaltyForViolation
	}

	// 7. Check time-based recommendations (optimal hours)
	if strings.Contains(recLower, "focus trading activity") || strings.Contains(recLower, "avoid trading during") || strings.Contains(recLower, "time selection") {
		// Extract hour from recommendation and compare
		// For now, assume compliant
		return true, ct.config.RewardForCompliance * 0.5 * rewardMultiplier
	}

	// Default: assume compliance if no specific violation detected
	return true, 0.0
}

// decisionMatchesPattern checks if a decision aligns with a trading pattern
func (ct *ComplianceTracker) decisionMatchesPattern(dec *decision.Decision, pattern TradingPattern) bool {
	patternLower := strings.ToLower(pattern.PatternType)
	patternText := strings.ToLower(pattern.PatternType + " " + pattern.Description + " " + pattern.Recommendation)

	// Match high leverage patterns
	if strings.Contains(patternLower, "high_leverage") || strings.Contains(patternLower, "leverage") || strings.Contains(patternText, "leverage") {
		return dec.Leverage >= 5
	}

	// Match position size patterns
	if strings.Contains(patternLower, "large_position") || strings.Contains(patternLower, "position_size") || strings.Contains(patternText, "position size") {
		return dec.PositionSizeUSD >= 300
	}

	// Match confidence patterns
	if strings.Contains(patternLower, "low_confidence") || strings.Contains(patternText, "low confidence") {
		return dec.Confidence < 60
	}
	if strings.Contains(patternLower, "high_confidence") || strings.Contains(patternText, "high confidence") {
		return dec.Confidence >= 70
	}

	// Match overtrading patterns
	if strings.Contains(patternLower, "overtrading") || strings.Contains(patternLower, "frequency") || strings.Contains(patternText, "frequency") {
		return dec.Action != "hold" && dec.Action != "wait"
	}

	return false
}

// extractExpectation extracts what was expected from a recommendation
func (ct *ComplianceTracker) extractExpectation(rec string) string {
	recLower := strings.ToLower(rec)

	if strings.Contains(recLower, "reduce leverage") {
		return "leverage ≤ 3x"
	}
	if strings.Contains(recLower, "reduce position size") {
		return "position ≤ $200"
	}
	if strings.Contains(recLower, "70%+ confidence") {
		return "confidence ≥ 70%"
	}
	if strings.Contains(recLower, "reduce frequency") {
		return "hold or wait"
	}
	if strings.Contains(recLower, "selective trading") {
		return "hold or high confidence entry"
	}

	return "follow recommendation"
}

// extractActual extracts what actually happened from a decision
func (ct *ComplianceTracker) extractActual(dec *decision.Decision) string {
	return fmt.Sprintf("action=%s, leverage=%dx, size=$%.0f, confidence=%d%%",
		dec.Action, dec.Leverage, dec.PositionSizeUSD, dec.Confidence)
}

// GetComplianceRate returns the current compliance rate (0.0-1.0)
func (ct *ComplianceTracker) GetComplianceRate() float64 {
	return ct.complianceRate
}

// GetRewardPoints returns total accumulated reward points
func (ct *ComplianceTracker) GetRewardPoints() float64 {
	return ct.totalRewardPoints
}

// GetComplianceFeedback generates feedback message about compliance
func (ct *ComplianceTracker) GetComplianceFeedback(lang string) string {
	if ct.totalRecommendations == 0 {
		return ""
	}

	rate := ct.complianceRate * 100

	if lang == "zh" {
		var message string
		if rate >= 80 {
			message = fmt.Sprintf("✅ **优秀的纪律性**: 您遵循了 %.0f%% 的建议 (+%.1f 奖励分)", rate, ct.totalRewardPoints)
		} else if rate >= 60 {
			message = fmt.Sprintf("⚠️ **良好但可改进**: 遵循了 %.0f%% 的建议 (%.1f 分)", rate, ct.totalRewardPoints)
		} else {
			message = fmt.Sprintf("🔴 **纪律性差**: 仅遵循了 %.0f%% 的建议 (%.1f 分). 请严格遵循反馈建议!", rate, ct.totalRewardPoints)
		}

		// Add recent violations
		violations := ct.getRecentViolations(3)
		if len(violations) > 0 {
			message += "\n\n**最近的违规行为**:\n"
			for _, v := range violations {
				message += fmt.Sprintf("- %s (周期 %d)\n", v.Recommendation, v.Cycle)
			}
		}

		return message
	}

	// English
	var message string
	if rate >= 80 {
		message = fmt.Sprintf("✅ **Excellent Discipline**: You followed %.0f%% of recommendations (+%.1f reward points)", rate, ct.totalRewardPoints)
	} else if rate >= 60 {
		message = fmt.Sprintf("⚠️ **Good But Improvable**: Followed %.0f%% of recommendations (%.1f points)", rate, ct.totalRewardPoints)
	} else {
		message = fmt.Sprintf("🔴 **Poor Discipline**: Only followed %.0f%% of recommendations (%.1f points). Please strictly adhere to feedback!", rate, ct.totalRewardPoints)
	}

	// Add recent violations
	violations := ct.getRecentViolations(3)
	if len(violations) > 0 {
		message += "\n\n**Recent Violations**:\n"
		for _, v := range violations {
			message += fmt.Sprintf("- %s (cycle %d)\n", v.Recommendation, v.Cycle)
		}
	}

	return message
}

// getRecentViolations returns the N most recent compliance violations
func (ct *ComplianceTracker) getRecentViolations(n int) []*ComplianceRecord {
	violations := make([]*ComplianceRecord, 0)

	// Iterate backwards through history
	for i := len(ct.complianceHistory) - 1; i >= 0 && len(violations) < n; i-- {
		if !ct.complianceHistory[i].Compliant {
			violations = append(violations, ct.complianceHistory[i])
		}
	}

	return violations
}

// AdjustFeedbackWeight adjusts feedback importance based on compliance
// Returns a multiplier (0.5-2.0) to apply to feedback signals
func (ct *ComplianceTracker) AdjustFeedbackWeight() float64 {
	if ct.totalRecommendations == 0 {
		return 1.0
	}

	// Good compliance -> amplify feedback
	// Poor compliance -> maintain or reduce feedback weight
	if ct.complianceRate >= ct.config.MinComplianceForBoost {
		// Boost feedback for compliant behavior
		return 1.0 + (ct.complianceRate-ct.config.MinComplianceForBoost)*2.0
	} else if ct.complianceRate < 0.4 {
		// Reduce feedback weight if not following
		return 0.5 + ct.complianceRate
	}

	return 1.0
}

// SaveState saves the tracker state to disk
func (ct *ComplianceTracker) SaveState(runID string) error {
	dir := filepath.Join("backtests", runID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	filename := filepath.Join(dir, "compliance_tracker_state.json")

	data, err := json.MarshalIndent(ct, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tracker state: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write tracker state: %w", err)
	}

	logger.Infof("[ComplianceTracker] 💾 Saved state to %s", filename)
	logger.Infof("  Compliance Rate: %.1f%% (%d/%d)", ct.complianceRate*100, ct.compliantActions, ct.totalRecommendations)
	logger.Infof("  Reward Points: %.1f", ct.totalRewardPoints)

	return nil
}

// GetDetailedReport generates a detailed compliance report
func (ct *ComplianceTracker) GetDetailedReport() map[string]interface{} {
	report := make(map[string]interface{})

	report["total_recommendations"] = ct.totalRecommendations
	report["compliant_actions"] = ct.compliantActions
	report["compliance_rate"] = ct.complianceRate
	report["total_reward_points"] = ct.totalRewardPoints
	report["feedback_weight_multiplier"] = ct.AdjustFeedbackWeight()

	// Breakdown by recommendation type
	recTypes := make(map[string]map[string]int)
	for _, record := range ct.complianceHistory {
		recType := ct.categorizeRecommendation(record.Recommendation)
		if _, exists := recTypes[recType]; !exists {
			recTypes[recType] = map[string]int{"total": 0, "compliant": 0}
		}
		recTypes[recType]["total"]++
		if record.Compliant {
			recTypes[recType]["compliant"]++
		}
	}
	report["compliance_by_type"] = recTypes

	return report
}

// categorizeRecommendation categorizes a recommendation for reporting
func (ct *ComplianceTracker) categorizeRecommendation(rec string) string {
	recLower := strings.ToLower(rec)

	if strings.Contains(recLower, "leverage") {
		return "leverage_management"
	}
	if strings.Contains(recLower, "position size") {
		return "position_sizing"
	}
	if strings.Contains(recLower, "confidence") {
		return "trade_selection"
	}
	if strings.Contains(recLower, "frequency") || strings.Contains(recLower, "selective") {
		return "trading_frequency"
	}
	if strings.Contains(recLower, "stop") {
		return "risk_management"
	}

	return "other"
}
