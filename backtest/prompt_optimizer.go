package backtest

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/logger"
	"nofx/store"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ============================================================================
// Prompt Optimization System (Outer Loop)
// ============================================================================
// Automatically evolves system prompts based on performance feedback
// Implements A/B testing and evolutionary algorithms to improve prompt quality
// IMPLEMENTED: Smart prompt rewriting with batch LLM requests
// - evolvePromptsWithLLM() now uses batch request for all sections (vs iterative)
// - Only rewrite underperforming prompts (fitness < threshold)
// - Parses results back into sections using extractPromptSections()
// - Integrates with store/strategy.go PromptVariant configuration
// See: evolvePromptsWithLLMBatch(), extractPromptSections(), isUnderperforming()
// ============================================================================

// PromptVariant represents a specific version of a system prompt
type PromptVariant struct {
	ID                     string    `json:"id"`
	PromptRoleDefinition   string    `json:"prompt_role_definition"`
	PromptTradingFrequency string    `json:"prompt_trading_frequency"`
	PromptEntryStandards   string    `json:"prompt_entry_standards"`
	PromptDecisionProcess  string    `json:"prompt_decision_process"`
	Version                int       `json:"version"`
	CreatedAt              time.Time `json:"created_at"`

	// Performance metrics
	TotalDecisions int     `json:"total_decisions"`
	TotalReturn    float64 `json:"total_return"`
	WinRate        float64 `json:"win_rate"`
	ProfitFactor   float64 `json:"profit_factor"`
	SharpeRatio    float64 `json:"sharpe_ratio"`
	MaxDrawdown    float64 `json:"max_drawdown"`

	// Fitness score for evolution
	FitnessScore float64 `json:"fitness_score"`
	Generation   int     `json:"generation"`
	IsActive     bool    `json:"is_active"`
}

// PromptOptimizer manages prompt evolution and A/B testing
type PromptOptimizer struct {
	RunID                  string // Backtest run ID for database persistence
	BasePrompt             *store.PromptSectionsConfig
	Variants               []*PromptVariant
	CurrentVariant         *PromptVariant
	Generation             int
	PopulationSize         int
	FirstShouldEvolveCycle int

	// Configuration
	Config *PromptOptimizerConfig

	// Performance tracking
	DecisionCounts  map[string]int      // variant ID -> decision count
	PerformanceData map[string]*Metrics // variant ID -> metrics

	// AI client for LLM-based evolution
	AIClient interface {
		CallWithMessages(systemPrompt, userPrompt string) (string, error)
	}

	// Storage for persistence
	Storage *store.BacktestStore
}

// PromptOptimizerConfig controls prompt optimization behavior
type PromptOptimizerConfig struct {
	EnableOptimization  bool `json:"enable_optimization"`
	PopulationSize      int  `json:"population_size"`        // Number of prompt variants to test
	EvaluationCycles    int  `json:"evaluation_cycles"`      // Cycles before evaluating variants
	TopVariantsToKeep   int  `json:"top_variants_to_keep"`   // Best variants to preserve
	MinDecisionsPerTest int  `json:"min_decisions_per_test"` // Min decisions before evaluation
}

// DefaultPromptOptimizerConfig returns default configuration
func DefaultPromptOptimizerConfig() *PromptOptimizerConfig {
	return &PromptOptimizerConfig{
		EnableOptimization:  false,
		PopulationSize:      5,
		EvaluationCycles:    30,
		TopVariantsToKeep:   2,
		MinDecisionsPerTest: 15,
	}
}

// NewPromptOptimizer creates a new prompt optimizer
func NewPromptOptimizer(basePrompt *store.PromptSectionsConfig, config *PromptOptimizerConfig) *PromptOptimizer {
	return NewPromptOptimizerWithAI(basePrompt, config, nil, "", nil)
}

// NewPromptOptimizerWithAI creates a new prompt optimizer with AI client for LLM-based evolution
func NewPromptOptimizerWithAI(basePrompt *store.PromptSectionsConfig, config *PromptOptimizerConfig, aiClient interface {
	CallWithMessages(systemPrompt, userPrompt string) (string, error)
}, runID string, storage *store.BacktestStore) *PromptOptimizer {
	if config == nil {
		config = DefaultPromptOptimizerConfig()
	}

	po := &PromptOptimizer{
		RunID:                  runID,
		BasePrompt:             basePrompt,
		Variants:               make([]*PromptVariant, 0),
		Generation:             1,
		PopulationSize:         config.PopulationSize,
		Config:                 config,
		DecisionCounts:         make(map[string]int),
		PerformanceData:        make(map[string]*Metrics),
		AIClient:               aiClient,
		Storage:                storage,
		FirstShouldEvolveCycle: -1,
	}

	// Create initial variant (base prompt) with consistent naming: gen1
	baseVariant := &PromptVariant{
		ID:                     "gen1",
		PromptRoleDefinition:   basePrompt.RoleDefinition,
		PromptTradingFrequency: basePrompt.TradingFrequency,
		PromptEntryStandards:   basePrompt.EntryStandards,
		PromptDecisionProcess:  basePrompt.DecisionProcess,
		Version:                1,
		CreatedAt:              time.Now(),
		Generation:             1,
		IsActive:               true,
		FitnessScore:           0.0,
	}

	po.Variants = append(po.Variants, baseVariant)
	po.CurrentVariant = baseVariant

	// Save initial variant to database
	if err := po.SaveVariantToDB(baseVariant); err != nil {
		logger.Errorf("[PromptOptimizer] Failed to save base variant: %v", err)
	}

	logger.Infof("[PromptOptimizer] Initialized with base prompt: gen1 (generation: 1)")

	return po
}

// GetCurrentPrompt returns the currently active system prompt variant
func (pv *PromptVariant) toStoreData(runID string) *store.PromptVariantData {
	return &store.PromptVariantData{
		ID:                     pv.ID,
		RunID:                  runID,
		VariantID:              pv.ID,
		Generation:             pv.Generation,
		IsActive:               pv.IsActive,
		PromptRoleDefinition:   pv.PromptRoleDefinition,
		PromptTradingFrequency: pv.PromptTradingFrequency,
		PromptEntryStandards:   pv.PromptEntryStandards,
		PromptDecisionProcess:  pv.PromptDecisionProcess,
		TotalDecisions:         pv.TotalDecisions,
		TotalReturn:            pv.TotalReturn,
		WinRate:                pv.WinRate,
		ProfitFactor:           pv.ProfitFactor,
		SharpeRatio:            pv.SharpeRatio,
		MaxDrawdown:            pv.MaxDrawdown,
		FitnessScore:           pv.FitnessScore,
		CreatedAt:              pv.CreatedAt.Format(time.RFC3339),
		UpdatedAt:              time.Now().Format(time.RFC3339),
	}
}

// GetCurrentPrompt returns the currently active system prompt variant data
func (po *PromptOptimizer) GetCurrentPrompt() *store.PromptVariantData {
	if po.CurrentVariant != nil {
		return po.CurrentVariant.toStoreData(po.RunID)
	}
	return nil
}

// Get Variant Prompt by ID
func (po *PromptOptimizer) GetVariantPromptByID(variantID string) *store.PromptVariantData {
	variant := po.GetVariantByID(variantID)
	if variant != nil {
		return variant.toStoreData(po.RunID)
	}
	return nil
}

// GetAllVariants returns a copy of all prompt variants for inspection
func (po *PromptOptimizer) GetAllVariants() []*PromptVariant {
	result := make([]*PromptVariant, len(po.Variants))
	copy(result, po.Variants)
	return result
}

// GetCurrentVariant returns the currently active variant
func (po *PromptOptimizer) GetCurrentVariant() *PromptVariant {
	return po.CurrentVariant
}

// GetCurrentVariant by ID
func (po *PromptOptimizer) GetVariantByID(variantID string) *PromptVariant {
	for _, v := range po.Variants {
		if v.ID == variantID {
			return v
		}
	}
	return nil
}

// Get ID by Variant
func (po *PromptOptimizer) GetVariantID(variant *PromptVariant) string {
	return variant.ID
}

// SaveVariantToDB persists a prompt variant to the database
func (po *PromptOptimizer) SaveVariantToDB(variant *PromptVariant) error {
	if po.Storage == nil || po.RunID == "" {
		return nil // Skip if storage not configured
	}
	if err := po.Storage.EnsureRunExists(po.RunID); err != nil {
		return err
	}

	metrics := po.PerformanceData[variant.ID]
	if metrics == nil {
		metrics = &Metrics{}
	}
	variantData := &store.PromptVariantData{
		ID:                     variant.ID,
		RunID:                  po.RunID,
		VariantID:              variant.ID,
		Generation:             variant.Generation,
		IsActive:               variant.IsActive,
		PromptRoleDefinition:   variant.PromptRoleDefinition,
		PromptTradingFrequency: variant.PromptTradingFrequency,
		PromptEntryStandards:   variant.PromptEntryStandards,
		PromptDecisionProcess:  variant.PromptDecisionProcess,
		TotalDecisions:         po.DecisionCounts[variant.ID],
		TotalReturn:            metrics.TotalReturnPct,
		WinRate:                metrics.WinRate,
		ProfitFactor:           metrics.ProfitFactor,
		SharpeRatio:            metrics.SharpeRatio,
		MaxDrawdown:            metrics.MaxDrawdownPct,
		FitnessScore:           variant.FitnessScore,
		CreatedAt:              variant.CreatedAt.Format(time.RFC3339),
	}

	return po.Storage.SavePromptVariant(variantData)
}

// SaveAllVariantsToDB persists all variants to the database
func (po *PromptOptimizer) SaveAllVariantsToDB() error {
	if po.Storage == nil || po.RunID == "" {
		return nil
	}

	for _, variant := range po.Variants {
		if err := po.SaveVariantToDB(variant); err != nil {
			logger.Errorf("[PromptOptimizer] Failed to save variant %s: %v", variant.ID, err)
			return err
		}
	}
	return nil
}

// GetGeneration returns the current generation number
func (po *PromptOptimizer) GetGeneration() int {
	return po.Generation
}

// ActivateVariant switches to a specific variant by ID
func (po *PromptOptimizer) ActivateVariant(variantID string) error {
	for _, v := range po.Variants {
		if v.ID == variantID {
			po.CurrentVariant = v
			v.IsActive = true
			// Mark others as inactive
			for _, other := range po.Variants {
				if other.ID != variantID {
					other.IsActive = false
				}
			}
			// Persist the activation state change to database
			if err := po.SaveAllVariantsToDB(); err != nil {
				return fmt.Errorf("failed to save all variants to DB: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("variant %s not found", variantID)
}

// RecordDecisionOutcome records the outcome of a decision made with a specific prompt
func (po *PromptOptimizer) RecordDecisionOutcome(variantID string, metrics *Metrics) {
	if !po.Config.EnableOptimization {
		return
	}

	po.DecisionCounts[variantID]++
	po.PerformanceData[variantID] = metrics

	// Update variant metrics
	for _, variant := range po.Variants {
		if variant.ID == variantID {
			variant.TotalDecisions = po.DecisionCounts[variantID]
			variant.TotalReturn = metrics.TotalReturnPct
			variant.WinRate = metrics.WinRate
			variant.ProfitFactor = metrics.ProfitFactor
			variant.SharpeRatio = metrics.SharpeRatio
			variant.MaxDrawdown = metrics.MaxDrawdownPct
			variant.FitnessScore = po.calculateFitness(metrics)

			// Save updated variant to database periodically (every 5 decisions)
			if po.DecisionCounts[variantID]%5 == 0 {
				if err := po.SaveVariantToDB(variant); err != nil {
					logger.Errorf("[PromptOptimizer] Failed to save variant %s: %v", variant.ID, err)
				}
			}
			break
		}
	}
}

// ShouldEvolve determines if it's time to evolve prompts
func (po *PromptOptimizer) ShouldEvolve(currentCycle int) bool {
	if po.FirstShouldEvolveCycle == -1 {
		po.FirstShouldEvolveCycle = currentCycle
		logger.Infof("[PromptOptimizer] First ShouldEvolve call at cycle %d", currentCycle)
	}
	if !po.Config.EnableOptimization {
		logger.Infof("[PromptOptimizer] Optimization disabled, skipping evolution: ID=%s, Cycle=%d", po.RunID, currentCycle)
		return false
	}
	if po == nil || po.Config == nil {
		logger.Warnf("[PromptOptimizer] ShouldEvolve called with nil optimizer or config")
		return false
	}
	if po.PerformanceData == nil {
		logger.Warnf("[PromptOptimizer] ShouldEvolve called with nil PerformanceData")
		return false
	}
	// Evolve every EvaluationCycles
	if ((currentCycle - po.FirstShouldEvolveCycle) % po.Config.EvaluationCycles) != 0 {
		logger.Infof("[PromptOptimizer] Not evaluation cycle yet (offset logic), skipping evolution: ID=%s, Cycle=%d", po.RunID, currentCycle)
		return false
	}

	// Check if we have enough data
	totalDecisions := 0
	for _, count := range po.DecisionCounts {
		totalDecisions += count
	}

	// Performance check: only evolve if current variant is underperforming
	if po.CurrentVariant != nil {
		currentMetrics := po.PerformanceData[po.CurrentVariant.ID]
		if currentMetrics != nil {
			if currentMetrics.WinRate >= 60.0 && currentMetrics.TotalReturnPct >= 10.0 {
				logger.Infof("[PromptOptimizer] Current variant %s is performing well (WinRate=%.1f%%, Return=%.2f%%), skipping evolution",
					po.CurrentVariant.ID, currentMetrics.WinRate, currentMetrics.TotalReturnPct)
				return false
			} else {
				logger.Infof("[PromptOptimizer] Current variant %s is underperforming (WinRate=%.1f%%, Return=%.2f%%), considering evolution",
					po.CurrentVariant.ID, currentMetrics.WinRate, currentMetrics.TotalReturnPct)
			}
		} else {
			logger.Errorf("[PromptOptimizer] CurrentMetrics is nil for variant %s", po.CurrentVariant.ID)
		}
	}
	return totalDecisions >= po.Config.MinDecisionsPerTest
}

// EvolvePrompts creates new generation of prompts using LLM-based evolution
// The LLM analyzes performance and rewrites the system prompt to address weaknesses
func (po *PromptOptimizer) EvolvePrompts(variantID string) error {
	if !po.Config.EnableOptimization {
		return nil
	}

	logger.Infof("[PromptOptimizer] 🧬 Evolving prompts (generation %d → %d)", po.Generation, po.Generation+1)

	// Sort variants by fitness
	sort.Slice(po.Variants, func(i, j int) bool {
		return po.Variants[i].FitnessScore > po.Variants[j].FitnessScore
	})

	// Log current performance
	for i, variant := range po.Variants {
		if variant.TotalDecisions > 0 {
			logger.Infof("  Variant %s (gen %d): Fitness=%.3f, Return=%.2f%%, WinRate=%.1f%%, Decisions=%d",
				variant.ID, variant.Generation, variant.FitnessScore,
				variant.TotalReturn, variant.WinRate, variant.TotalDecisions)
		}
		if i >= 2 { // Only log top 3
			break
		}
	}

	// Use LLM-based evolution if AI client is available
	if po.AIClient != nil {
		return po.evolvePromptsWithLLM(variantID)
	}

	// Fallback: Keep top performers only (no genetic algorithm)
	logger.Infof("[PromptOptimizer] ⚠️ No AI client available, keeping top variant only")
	topVariants := po.Variants[:1] // Keep only the best

	// Update generation
	po.Generation++
	po.Variants = topVariants
	po.CurrentVariant = po.Variants[0]

	// Reset tracking
	po.DecisionCounts = make(map[string]int)
	po.PerformanceData = make(map[string]*Metrics)

	logger.Infof("[PromptOptimizer] ✅ Evolution complete: %d variants in generation %d", len(po.Variants), po.Generation)

	return nil
}

// evolvePromptsWithLLM uses LLM to evolve system prompts based on performance
// Optimized to use batch LLM request instead of iterative calls
func (po *PromptOptimizer) evolvePromptsWithLLM(variantID string) error {
	currentVariant := po.GetVariantByID(variantID)
	if currentVariant == nil {
		logger.Fatal("[PromptOptimizer] Variant not found, skipping LLM evolution")
		return nil
	}
	currentMetrics := po.PerformanceData[currentVariant.ID]

	if currentMetrics == nil {
		logger.Infof("[PromptOptimizer] No performance data, skipping LLM evolution")
		return nil
	}

	// Check if prompt is underperforming before evolution
	if po.isUnderperforming(currentMetrics) {
		// Use optimized batch LLM evolution
		return po.evolvePromptsWithLLMBatch(variantID, currentVariant, currentMetrics)
	}

	logger.Infof("[PromptOptimizer] Prompt variant %s performing adequately (fitness: %.3f), keeping current", variantID, currentVariant.FitnessScore)
	return nil
}

// isUnderperforming determines if a prompt variant should be evolved
func (po *PromptOptimizer) isUnderperforming(metrics *Metrics) bool {
	// Evolve if any of these conditions are met
	return metrics.WinRate < 50 || // Low win rate
		metrics.ProfitFactor < 1.3 || // Low profit factor
		metrics.TotalReturnPct < 5 || // Low returns
		metrics.SharpeRatio < 0.5 // Low risk-adjusted returns
}

// evolvePromptsWithLLMBatch uses a single batch LLM request to rewrite all prompt sections
// More efficient than iterative calls, ensures consistency, and parses results back into sections
func (po *PromptOptimizer) evolvePromptsWithLLMBatch(variantID string, currentVariant *PromptVariant, currentMetrics *Metrics) error {

	// Detect language from current prompt
	lang := "en"
	if strings.Contains(currentVariant.PromptRoleDefinition, "专业") || strings.Contains(currentVariant.PromptRoleDefinition, "量化") {
		lang = "zh"
	}

	logger.Infof("[PromptOptimizer] 🤖 Batch LLM evolution for underperforming variant %s (%s)...", variantID, lang)

	// Build single meta-prompt for all sections
	var metaPrompt string
	var userPrompt string

	if lang == "zh" {
		metaPrompt = po.buildBatchEvolutionPromptZH(currentVariant, currentMetrics)
		userPrompt = "请一次性改写以下所有系统提示词部分，并按照指定格式输出。"
	} else {
		metaPrompt = po.buildBatchEvolutionPromptEN(currentVariant, currentMetrics)
		userPrompt = "Please rewrite all the following system prompt sections at once and output in the specified format."
	}

	// Single batch call to LLM
	evolvedText, err := po.AIClient.CallWithMessages(metaPrompt, userPrompt)
	if err != nil {
		logger.Infof("[PromptOptimizer] ❌ Batch LLM evolution failed: %v, keeping current variant", err)
		po.Generation++
		return nil // Non-fatal, continue with current
	}

	// Parse evolved sections from batch response
	sections := po.extractPromptSections(evolvedText)

	// Create new evolved variant
	evolvedVariant := &PromptVariant{
		ID:                     fmt.Sprintf("gen%d", po.Generation+1),
		PromptRoleDefinition:   sections["role"],
		PromptTradingFrequency: sections["frequency"],
		PromptEntryStandards:   sections["entry"],
		PromptDecisionProcess:  sections["decision"],
		Version:                po.Generation + 1,
		CreatedAt:              time.Now(),
		Generation:             po.Generation + 1,
		IsActive:               true,
		FitnessScore:           0.0, // Will be evaluated in next cycle
	}

	// Update generation
	po.Generation++
	po.Variants = []*PromptVariant{evolvedVariant}
	po.CurrentVariant = evolvedVariant

	// Save new variant to database
	if err := po.SaveVariantToDB(evolvedVariant); err != nil {
		logger.Errorf("[PromptOptimizer] Failed to save evolved variant %s: %v", evolvedVariant.ID, err)
	}

	// Reset tracking
	po.DecisionCounts = make(map[string]int)
	po.PerformanceData = make(map[string]*Metrics)

	logger.Infof("[PromptOptimizer] ✅ Batch LLM evolution complete: new variant %s (generation %d)", evolvedVariant.ID, po.Generation)
	logger.Infof("[PromptOptimizer] 📝 Role definition preview: %s...", evolvedVariant.PromptRoleDefinition[:minInt(100, len(evolvedVariant.PromptRoleDefinition))])
	logger.Infof("[PromptOptimizer] 📝 Trading frequency preview: %s...", evolvedVariant.PromptTradingFrequency[:minInt(100, len(evolvedVariant.PromptTradingFrequency))])
	logger.Infof("[PromptOptimizer] 📝 Entry standards preview: %s...", evolvedVariant.PromptEntryStandards[:minInt(100, len(evolvedVariant.PromptEntryStandards))])
	logger.Infof("[PromptOptimizer] 📝 Decision process preview: %s...", evolvedVariant.PromptDecisionProcess[:minInt(100, len(evolvedVariant.PromptDecisionProcess))])

	return nil
}

// buildBatchEvolutionPromptEN creates a batch meta-prompt for all sections (English)
func (po *PromptOptimizer) buildBatchEvolutionPromptEN(variant *PromptVariant, metrics *Metrics) string {
	var sb strings.Builder
	sb.WriteString("You are an expert prompt engineer for trading systems. Evolve ALL four sections below simultaneously.\n\n")
	sb.WriteString("# Performance Issues Identified\n")
	if metrics.WinRate < 50 {
		sb.WriteString(fmt.Sprintf("- Low win rate: %.1f%%\n", metrics.WinRate))
	}
	if metrics.ProfitFactor < 1.3 {
		sb.WriteString(fmt.Sprintf("- Low profit factor: %.2f\n", metrics.ProfitFactor))
	}
	if metrics.TotalReturnPct < 5 {
		sb.WriteString(fmt.Sprintf("- Insufficient returns: %.2f%%\n", metrics.TotalReturnPct))
	}
	if metrics.SharpeRatio < 0.5 {
		sb.WriteString(fmt.Sprintf("- Low risk-adjusted returns: %.2f\n", metrics.SharpeRatio))
	}
	sb.WriteString("\n# Current Sections\n\n")
	sb.WriteString("## ROLE_DEFINITION\n")
	sb.WriteString(variant.PromptRoleDefinition)
	sb.WriteString("\n\n## TRADING_FREQUENCY\n")
	sb.WriteString(variant.PromptTradingFrequency)
	sb.WriteString("\n\n## ENTRY_STANDARDS\n")
	sb.WriteString(variant.PromptEntryStandards)
	sb.WriteString("\n\n## DECISION_PROCESS\n")
	sb.WriteString(variant.PromptDecisionProcess)
	sb.WriteString("\n\n# Task\n")
	sb.WriteString("Rewrite ALL four sections to address the identified issues. Output format MUST be:\n\n")
	sb.WriteString("[ROLE_DEFINITION]\n<rewritten role definition>\n[END_ROLE_DEFINITION]\n\n")
	sb.WriteString("[TRADING_FREQUENCY]\n<rewritten trading frequency>\n[END_TRADING_FREQUENCY]\n\n")
	sb.WriteString("[ENTRY_STANDARDS]\n<rewritten entry standards>\n[END_ENTRY_STANDARDS]\n\n")
	sb.WriteString("[DECISION_PROCESS]\n<rewritten decision process>\n[END_DECISION_PROCESS]\n\n")
	sb.WriteString("Ensure consistency across all sections and preserve high-quality logic.\n")
	return sb.String()
}

// buildBatchEvolutionPromptZH creates a batch meta-prompt for all sections (Chinese)
func (po *PromptOptimizer) buildBatchEvolutionPromptZH(variant *PromptVariant, metrics *Metrics) string {
	var sb strings.Builder
	sb.WriteString("你是交易系统提示词工程专家。请同时改写以下四个部分。\n\n")
	sb.WriteString("# 识别的表现问题\n")
	if metrics.WinRate < 50 {
		sb.WriteString(fmt.Sprintf("- 低胜率: %.1f%%\n", metrics.WinRate))
	}
	if metrics.ProfitFactor < 1.3 {
		sb.WriteString(fmt.Sprintf("- 低利润因子: %.2f\n", metrics.ProfitFactor))
	}
	if metrics.TotalReturnPct < 5 {
		sb.WriteString(fmt.Sprintf("- 收益不足: %.2f%%\n", metrics.TotalReturnPct))
	}
	if metrics.SharpeRatio < 0.5 {
		sb.WriteString(fmt.Sprintf("- 风险调整回报低: %.2f\n", metrics.SharpeRatio))
	}
	sb.WriteString("\n# 当前部分\n\n")
	sb.WriteString("## ROLE_DEFINITION\n")
	sb.WriteString(variant.PromptRoleDefinition)
	sb.WriteString("\n\n## TRADING_FREQUENCY\n")
	sb.WriteString(variant.PromptTradingFrequency)
	sb.WriteString("\n\n## ENTRY_STANDARDS\n")
	sb.WriteString(variant.PromptEntryStandards)
	sb.WriteString("\n\n## DECISION_PROCESS\n")
	sb.WriteString(variant.PromptDecisionProcess)
	sb.WriteString("\n\n# 任务\n")
	sb.WriteString("改写所有四个部分以解决识别的问题。输出格式必须为：\n\n")
	sb.WriteString("[ROLE_DEFINITION]\n<改写的角色定义>\n[END_ROLE_DEFINITION]\n\n")
	sb.WriteString("[TRADING_FREQUENCY]\n<改写的交易频率>\n[END_TRADING_FREQUENCY]\n\n")
	sb.WriteString("[ENTRY_STANDARDS]\n<改写的入场标准>\n[END_ENTRY_STANDARDS]\n\n")
	sb.WriteString("[DECISION_PROCESS]\n<改写的决策流程>\n[END_DECISION_PROCESS]\n\n")
	sb.WriteString("确保所有部分之间的一致性，并保留高质量的逻辑。\n")
	return sb.String()
}

// extractPromptSections parses batch LLM output into individual prompt sections
func (po *PromptOptimizer) extractPromptSections(evolvedText string) map[string]string {
	sections := map[string]string{
		"role":      "",
		"frequency": "",
		"entry":     "",
		"decision":  "",
	}

	// Extract using markers
	text := evolvedText

	// Extract ROLE_DEFINITION
	if start := strings.Index(text, "[ROLE_DEFINITION]"); start != -1 {
		if end := strings.Index(text[start:], "[END_ROLE_DEFINITION]"); end != -1 {
			content := text[start+len("[ROLE_DEFINITION]") : start+end]
			sections["role"] = strings.TrimSpace(content)
		}
	}

	// Extract TRADING_FREQUENCY
	if start := strings.Index(text, "[TRADING_FREQUENCY]"); start != -1 {
		if end := strings.Index(text[start:], "[END_TRADING_FREQUENCY]"); end != -1 {
			content := text[start+len("[TRADING_FREQUENCY]") : start+end]
			sections["frequency"] = strings.TrimSpace(content)
		}
	}

	// Extract ENTRY_STANDARDS
	if start := strings.Index(text, "[ENTRY_STANDARDS]"); start != -1 {
		if end := strings.Index(text[start:], "[END_ENTRY_STANDARDS]"); end != -1 {
			content := text[start+len("[ENTRY_STANDARDS]") : start+end]
			sections["entry"] = strings.TrimSpace(content)
		}
	}

	// Extract DECISION_PROCESS
	if start := strings.Index(text, "[DECISION_PROCESS]"); start != -1 {
		if end := strings.Index(text[start:], "[END_DECISION_PROCESS]"); end != -1 {
			content := text[start+len("[DECISION_PROCESS]") : start+end]
			sections["decision"] = strings.TrimSpace(content)
		}
	}

	logger.Infof("[PromptOptimizer] Extracted sections: role=%d chars, frequency=%d chars, entry=%d chars, decision=%d chars",
		len(sections["role"]), len(sections["frequency"]), len(sections["entry"]), len(sections["decision"]))

	return sections
}

// Helper function for min
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// calculateFitness computes fitness score for a prompt variant
func (po *PromptOptimizer) calculateFitness(metrics *Metrics) float64 {
	// Multi-objective fitness function
	// Weights: Return (40%), Win Rate (20%), Profit Factor (20%), Sharpe (10%), Drawdown (10%)

	returnScore := metrics.TotalReturnPct / 100.0 // Normalize to 0-1 range (assuming -100% to +100%)
	if returnScore < -1 {
		returnScore = -1
	}
	if returnScore > 1 {
		returnScore = 1
	}

	winRateScore := metrics.WinRate / 100.0 // Already 0-100

	profitFactorScore := math.Min(metrics.ProfitFactor/2.0, 1.0) // Normalize (2.0 = perfect)

	sharpeScore := math.Min(metrics.SharpeRatio/2.0, 1.0) // Normalize (2.0 = excellent)
	if sharpeScore < 0 {
		sharpeScore = 0
	}

	drawdownScore := 1.0 - (metrics.MaxDrawdownPct / 100.0) // Lower is better
	if drawdownScore < 0 {
		drawdownScore = 0
	}

	fitness := (returnScore * 0.4) +
		(winRateScore * 0.2) +
		(profitFactorScore * 0.2) +
		(sharpeScore * 0.1) +
		(drawdownScore * 0.1)

	return fitness
}

// SaveState saves the optimizer state to disk
func (po *PromptOptimizer) SaveState(runID string) error {
	dir := filepath.Join("backtests", runID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	filename := filepath.Join(dir, "prompt_optimizer_state.json")

	data, err := json.MarshalIndent(po, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal optimizer state: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write optimizer state: %w", err)
	}

	logger.Infof("[PromptOptimizer] 💾 Saved state to %s", filename)
	return nil
}

// LoadState loads the optimizer state from disk
func (po *PromptOptimizer) LoadState(runID string) error {
	filename := filepath.Join("backtests", runID, "prompt_optimizer_state.json")

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read optimizer state: %w", err)
	}

	if err := json.Unmarshal(data, po); err != nil {
		return fmt.Errorf("failed to unmarshal optimizer state: %w", err)
	}

	logger.Infof("[PromptOptimizer] 📂 Loaded state from %s (generation %d)", filename, po.Generation)
	return nil
}

// GetEvolutionSummary returns a summary of prompt evolution for LLM feedback
func (po *PromptOptimizer) GetEvolutionSummary(lang string) string {
	if po.Generation <= 1 || len(po.Variants) == 0 {
		return "" // No evolution yet
	}

	// Sort variants by fitness to get best performers
	sortedVariants := make([]*PromptVariant, len(po.Variants))
	copy(sortedVariants, po.Variants)
	sort.Slice(sortedVariants, func(i, j int) bool {
		return sortedVariants[i].FitnessScore > sortedVariants[j].FitnessScore
	})

	if lang == "zh" {
		var sb strings.Builder
		sb.WriteString("## 🧬 提示词进化历史\n")
		sb.WriteString(fmt.Sprintf("当前代数: %d | 总变体: %d\n\n", po.Generation, len(po.Variants)))

		// Show top 3 performing variants
		sb.WriteString("**表现最佳的提示词策略**:\n")
		for i := 0; i < 3 && i < len(sortedVariants); i++ {
			v := sortedVariants[i]
			if v.TotalDecisions > 0 {
				active := ""
				if v.IsActive {
					active = " [当前使用]"
				}
				sb.WriteString(fmt.Sprintf("%d. 变体 %s (第%d代)%s\n", i+1, v.ID, v.Generation, active))
				sb.WriteString(fmt.Sprintf("   - 适应度: %.3f | 收益: %.2f%% | 胜率: %.1f%% | 交易数: %d\n",
					v.FitnessScore, v.TotalReturn, v.WinRate, v.TotalDecisions))
			}
		}
		return sb.String()
	}

	// English
	var sb strings.Builder
	sb.WriteString("## 🧬 Prompt Evolution History\n")
	sb.WriteString(fmt.Sprintf("Current Generation: %d | Total Variants: %d\n\n", po.Generation, len(po.Variants)))

	// Show top 3 performing variants
	sb.WriteString("**Best Performing Prompt Strategies**:\n")
	for i := 0; i < 3 && i < len(sortedVariants); i++ {
		v := sortedVariants[i]
		if v.TotalDecisions > 0 {
			active := ""
			if v.IsActive {
				active = " [CURRENT]"
			}
			sb.WriteString(fmt.Sprintf("%d. Variant %s (Gen %d)%s\n", i+1, v.ID, v.Generation, active))
			sb.WriteString(fmt.Sprintf("   - Fitness: %.3f | Return: %.2f%% | Win Rate: %.1f%% | Trades: %d\n",
				v.FitnessScore, v.TotalReturn, v.WinRate, v.TotalDecisions))
		}
	}
	return sb.String()
}
