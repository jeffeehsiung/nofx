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
	RunID          string // Backtest run ID for database persistence
	BasePrompt     *store.PromptSectionsConfig
	Variants       []*PromptVariant
	CurrentVariant *PromptVariant
	Generation     int
	PopulationSize int
	MutationRate   float64

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
	EnableOptimization  bool    `json:"enable_optimization"`
	PopulationSize      int     `json:"population_size"`        // Number of prompt variants to test
	MutationRate        float64 `json:"mutation_rate"`          // Probability of mutation (0.0-1.0)
	EvaluationCycles    int     `json:"evaluation_cycles"`      // Cycles before evaluating variants
	TopVariantsToKeep   int     `json:"top_variants_to_keep"`   // Best variants to preserve
	MinDecisionsPerTest int     `json:"min_decisions_per_test"` // Min decisions before evaluation
}

// DefaultPromptOptimizerConfig returns default configuration
func DefaultPromptOptimizerConfig() *PromptOptimizerConfig {
	return &PromptOptimizerConfig{
		EnableOptimization:  true,
		PopulationSize:      5,
		MutationRate:        0.3,
		EvaluationCycles:    20,
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
		RunID:           runID,
		BasePrompt:      basePrompt,
		Variants:        make([]*PromptVariant, 0),
		Generation:      1,
		PopulationSize:  config.PopulationSize,
		MutationRate:    config.MutationRate,
		Config:          config,
		DecisionCounts:  make(map[string]int),
		PerformanceData: make(map[string]*Metrics),
		AIClient:        aiClient,
		Storage:         storage,
	}

	// Create initial variant (base prompt) with consistent naming: gen1-v1
	baseVariant := &PromptVariant{
		ID:                     "gen1-v1",
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

	logger.Infof("[PromptOptimizer] Initialized with base prompt: gen1-v1 (generation: 1)")

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
	if !po.Config.EnableOptimization {
		return false
	}

	// Evolve every EvaluationCycles
	if currentCycle%po.Config.EvaluationCycles != 0 {
		return false
	}

	// Check if we have enough data
	totalDecisions := 0
	for _, count := range po.DecisionCounts {
		totalDecisions += count
	}

	return totalDecisions >= po.Config.MinDecisionsPerTest
}

// EvolvePrompts creates new generation of prompts using LLM-based evolution
// The LLM analyzes performance and rewrites the system prompt to address weaknesses
func (po *PromptOptimizer) EvolvePrompts(variantID string, strategy_prompt *store.PromptSectionsConfig) error {
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
		return po.evolvePromptsWithLLM(variantID, strategy_prompt)
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
func (po *PromptOptimizer) evolvePromptsWithLLM(variantID string, strategy_prompt *store.PromptSectionsConfig) error {
	currentVariant := po.GetVariantByID(variantID)
	if currentVariant == nil {
		logger.Fatal("[PromptOptimizer] Variant not found, skipping LLM evolution")
		return nil
	}
	currentMetrics := po.PerformanceData[currentVariant.ID]

	if currentMetrics == nil {
		logger.Infof("[PromptOptimizer] No performance data, skipping LLM evolution")
		po.Generation++
		return nil
	}

	// Detect language from current prompt
	lang := "en"
	if strings.Contains(currentVariant.PromptRoleDefinition, "专业") || strings.Contains(currentVariant.PromptRoleDefinition, "量化") {
		lang = "zh"
	}

	// Build meta-learning prompt for LLM
	var role_definition string
	var trading_frequency string
	var entry_standards string
	var decision_process string

	role_definition = strategy_prompt.RoleDefinition
	trading_frequency = strategy_prompt.TradingFrequency
	entry_standards = strategy_prompt.EntryStandards
	decision_process = strategy_prompt.DecisionProcess

	logger.Infof("[PromptOptimizer] 🤖 Asking LLM to evolve system prompt (%s)...", lang)

	// Ask LLM to improve the prompt
	for prompt_section, content := range map[string]string{
		"Role Definition":   role_definition,
		"Trading Frequency": trading_frequency,
		"Entry Standards":   entry_standards,
		"Decision Process":  decision_process,
	} {
		var metaPrompt string
		var evolvedText string
		var err error
		if lang == "zh" {
			metaPrompt = po.buildEvolutionMetaPromptZH(content, currentVariant, currentMetrics)
			evolvedText, err = po.AIClient.CallWithMessages(metaPrompt, fmt.Sprintf("当前的%s系统提示词\n是:\n%s。\n请改写\n", prompt_section, content))
		} else {
			metaPrompt = po.buildEvolutionMetaPromptEN(content, currentVariant, currentMetrics)
			evolvedText, err = po.AIClient.CallWithMessages(metaPrompt, fmt.Sprintf("The current %s system prompt is:\n%s.\nPlease rewrite it.\n", prompt_section, content))
		}
		if err != nil {
			logger.Infof("[PromptOptimizer] ❌ LLM evolution failed for section %s: %v", prompt_section, err)
			// Fallback: keep current variant
		} else {
			switch prompt_section {
			case "Role Definition":
				role_definition = evolvedText
			case "Trading Frequency":
				trading_frequency = evolvedText
			case "Entry Standards":
				entry_standards = evolvedText
			case "Decision Process":
				decision_process = evolvedText
			}
		}
	}

	// Create new evolved variant
	evolvedVariant := &PromptVariant{
		ID:                     fmt.Sprintf("gen%d-v1", po.Generation+1),
		PromptRoleDefinition:   role_definition,
		PromptTradingFrequency: trading_frequency,
		PromptEntryStandards:   entry_standards,
		PromptDecisionProcess:  decision_process,
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

	logger.Infof("[PromptOptimizer] ✅ LLM evolution complete: new variant %s (generation %d)", evolvedVariant.ID, po.Generation)
	logger.Infof("[PromptOptimizer] 📝 Role definition preview: %s...", evolvedVariant.PromptRoleDefinition[:100])
	logger.Infof("[PromptOptimizer] 📝 Trading frequency preview: %s...", evolvedVariant.PromptTradingFrequency[:100])
	logger.Infof("[PromptOptimizer] 📝 Entry standards preview: %s...", evolvedVariant.PromptEntryStandards[:100])
	logger.Infof("[PromptOptimizer] 📝 Decision process preview: %s...", evolvedVariant.PromptDecisionProcess[:100])

	return nil
}

// buildEvolutionMetaPrompt creates a prompt for the LLM to evolve the system prompt
func (po *PromptOptimizer) buildEvolutionMetaPromptEN(prompt string, variant *PromptVariant, metrics *Metrics) string {
	var sb strings.Builder
	sb.WriteString("You are an expert in prompt engineering for trading systems. Your task is to improve trading prompts based on performance analysis.")
	sb.WriteString("# System Prompt Evolution Task\n\n")
	sb.WriteString("## Current System Prompt\n```\n")
	sb.WriteString(prompt)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Performance Analysis\n")
	sb.WriteString(fmt.Sprintf("- **Total Return:** %.2f%%\n", metrics.TotalReturnPct))
	sb.WriteString(fmt.Sprintf("- **Win Rate:** %.1f%%\n", metrics.WinRate))
	sb.WriteString(fmt.Sprintf("- **Profit Factor:** %.2f\n", metrics.ProfitFactor))
	sb.WriteString(fmt.Sprintf("- **Sharpe Ratio:** %.2f\n", metrics.SharpeRatio))
	sb.WriteString(fmt.Sprintf("- **Max Drawdown:** %.1f%%\n", metrics.MaxDrawdownPct))
	sb.WriteString(fmt.Sprintf("- **Fitness Score:** %.3f\n\n", variant.FitnessScore))

	// Identify specific issues
	sb.WriteString("## Identified Issues\n")
	if metrics.WinRate < 45 {
		sb.WriteString("- ⚠️ **Low Win Rate (<45%):** The strategy is too aggressive or lacks proper entry criteria.\n")
	}
	if metrics.ProfitFactor < 1.5 {
		sb.WriteString("- ⚠️ **Low Profit Factor (<1.5):** Losses are too large relative to wins. Need better risk management.\n")
	}
	if metrics.MaxDrawdownPct > 20 {
		sb.WriteString("- ⚠️ **High Drawdown (>20%):** Position sizing is too aggressive or stop losses are too wide.\n")
	}
	if metrics.SharpeRatio < 0.5 {
		sb.WriteString("- ⚠️ **Low Sharpe Ratio (<0.5):** Returns don't justify the risk. Need higher quality trades.\n")
	}
	if metrics.TotalReturnPct < 0 {
		sb.WriteString("- ⚠️ **Negative Returns:** The strategy is losing money. Fundamental approach needs revision.\n")
	}
	sb.WriteString("\n")

	// Learning from market
	sb.WriteString("## Learning Requirements\n")
	sb.WriteString("The evolved prompt should:\n")
	sb.WriteString("1. **Learn from mistakes:** Address the specific issues identified above\n")
	sb.WriteString("2. **Market adaptation:** Consider market sentiment, volatility regimes, and trending vs ranging conditions\n")
	sb.WriteString("3. **Risk awareness:** Emphasize capital preservation and proper position sizing\n")
	sb.WriteString("4. **Pattern recognition:** Encourage identifying high-probability setups based on market structure\n")
	sb.WriteString("5. **Continuous improvement:** Build in self-reflection and adaptation mindset\n\n")

	// Skillset enhancement
	sb.WriteString("## Skillset Enhancement\n")
	sb.WriteString("Equip the role with missing or under-developed skills:\n\n")

	sb.WriteString("**Technical Analysis Skills:**\n")
	sb.WriteString("- Multi-timeframe analysis (1h, 4h, daily) for trend confirmation\n")
	sb.WriteString("- Support/resistance identification and price action reading\n")
	sb.WriteString("- Volume analysis for confirming breakouts/breakdowns\n")
	sb.WriteString("- Market microstructure interpretation (order book, tape reading)\n\n")

	sb.WriteString("**Signal Processing & Quantitative Analysis:**\n")
	sb.WriteString("- Fourier analysis to decompose price into frequency components (identify dominant cycles)\n")
	sb.WriteString("- Digital filtering (low-pass to remove noise, high-pass to detect regime changes, band-pass for cycle extraction)\n")
	sb.WriteString("- Spectral analysis to measure market periodicity and hidden rhythms\n")
	sb.WriteString("- Wavelet transforms for multi-scale time-frequency analysis\n")
	sb.WriteString("- Signal-to-noise ratio assessment to distinguish patterns from randomness\n")
	sb.WriteString("- Autocorrelation and cross-correlation for lead-lag relationships\n")
	sb.WriteString("- **Extended Kalman Filter (EKF)** with Fourier-based observations:\n")
	sb.WriteString("  * Use frequency domain features (dominant frequencies, spectral power) as observations\n")
	sb.WriteString("  * Spectrograms for spatial-temporal evolution tracking\n")
	sb.WriteString("  * State estimation: predict next market state (trend, volatility, regime) from noisy observations\n")
	sb.WriteString("  * Non-linear dynamics modeling: EKF handles non-Gaussian, non-linear market behavior\n")
	sb.WriteString("  * Adaptive filtering: continuously update state estimates as new data arrives\n\n")

	sb.WriteString("**Risk Management Expertise:**\n")
	sb.WriteString("- Dynamic position sizing based on volatility and account risk\n")
	sb.WriteString("- Stop-loss placement using ATR, structure, or percentage-based methods\n")
	sb.WriteString("- Portfolio heat management (total risk across all positions)\n")
	sb.WriteString("- Correlation awareness to avoid overconcentration\n\n")

	sb.WriteString("**Market Psychology & Sentiment:**\n")
	sb.WriteString("- Recognizing fear/greed extremes from funding rates, open interest, social sentiment\n")
	sb.WriteString("- Contrarian thinking when crowd is overly positioned\n")
	sb.WriteString("- Identifying market regime changes (trending → ranging, risk-on → risk-off)\n")
	sb.WriteString("- BTC dominance and altcoin rotation cycle awareness\n\n")

	sb.WriteString("**Execution & Trade Management:**\n")
	sb.WriteString("- Entry timing optimization (avoid FOMO, wait for pullbacks)\n")
	sb.WriteString("- Scaling in/out strategies for better average prices\n")
	sb.WriteString("- Trailing stop techniques to capture trends while protecting profits\n")
	sb.WriteString("- Knowing when NOT to trade (low liquidity, high uncertainty, choppy conditions)\n\n")

	sb.WriteString("**Self-Awareness & Metacognition:**\n")
	sb.WriteString("- Recognizing own biases (recency bias, confirmation bias, overconfidence)\n")
	sb.WriteString("- Learning from both wins and losses (what was luck vs skill?)\n")
	sb.WriteString("- Adapting strategy based on changing market conditions\n")
	sb.WriteString("- Keeping detailed mental models of why trades work or fail\n\n")

	sb.WriteString("## Self-Diagnosis: What Skills Are Missing?\n")
	sb.WriteString("**Critical Task:** Analyze the current prompt and identify what capabilities it lacks.\n\n")
	sb.WriteString("Ask yourself:\n")
	sb.WriteString("1. What analytical frameworks or methodologies are absent?\n")
	sb.WriteString("2. What market dynamics or phenomena does it fail to consider?\n")
	sb.WriteString("3. What decision-making processes or heuristics could improve outcomes?\n")
	sb.WriteString("4. What domain knowledge (crypto-specific, macro, derivatives) is missing?\n")
	sb.WriteString("5. What statistical, mathematical, or computational techniques would be valuable?\n")
	sb.WriteString("6. What psychological or behavioral finance concepts should be integrated?\n\n")
	sb.WriteString("**Be creative and comprehensive.** Don't just address the issues above - think about what a world-class trader would know that this prompt doesn't capture.\n\n")

	sb.WriteString("## Your Task\n")
	sb.WriteString("Rewrite the system prompt to:\n")
	sb.WriteString("1. Address the performance issues identified\n")
	sb.WriteString("2. Integrate the missing/weak skills listed above\n")
	sb.WriteString("3. **Add capabilities you identified as missing through self-diagnosis**\n")
	sb.WriteString("4. Make the role more sophisticated and market-aware\n")
	sb.WriteString("5. Preserve what's currently working well\n\n")
	sb.WriteString("Output ONLY the improved system prompt, no explanations or commentary.\n")

	return sb.String()
}

// buildEvolutionMetaPromptZH creates a Chinese prompt for the LLM to evolve the system prompt
func (po *PromptOptimizer) buildEvolutionMetaPromptZH(prompt string, variant *PromptVariant, metrics *Metrics) string {
	var sb strings.Builder
	sb.WriteString("你是交易系统提示词工程专家。你的任务是基于表现分析改进交易提示词。")
	sb.WriteString("# 系统提示词进化任务\n\n")
	sb.WriteString("## 当前系统提示词\n```\n")
	sb.WriteString(prompt)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## 表现分析\n")
	sb.WriteString(fmt.Sprintf("- **总收益:** %.2f%%\n", metrics.TotalReturnPct))
	sb.WriteString(fmt.Sprintf("- **胜率:** %.1f%%\n", metrics.WinRate))
	sb.WriteString(fmt.Sprintf("- **利润因子:** %.2f\n", metrics.ProfitFactor))
	sb.WriteString(fmt.Sprintf("- **夏普比率:** %.2f\n", metrics.SharpeRatio))
	sb.WriteString(fmt.Sprintf("- **最大回撤:** %.1f%%\n", metrics.MaxDrawdownPct))
	sb.WriteString(fmt.Sprintf("- **适应度得分:** %.3f\n\n", variant.FitnessScore))

	// Identify specific issues
	sb.WriteString("## 识别的问题\n")
	if metrics.WinRate < 45 {
		sb.WriteString("- ⚠️ **低胜率 (<45%):** 策略过于激进或缺乏合适的入场标准。\n")
	}
	if metrics.ProfitFactor < 1.5 {
		sb.WriteString("- ⚠️ **低利润因子 (<1.5):** 相对于盈利，亏损过大。需要更好的风险管理。\n")
	}
	if metrics.MaxDrawdownPct > 20 {
		sb.WriteString("- ⚠️ **高回撤 (>20%):** 仓位规模过于激进或止损过宽。\n")
	}
	if metrics.SharpeRatio < 0.5 {
		sb.WriteString("- ⚠️ **低夏普比率 (<0.5):** 收益无法证明风险。需要更高质量的交易。\n")
	}
	if metrics.TotalReturnPct < 0 {
		sb.WriteString("- ⚠️ **负收益:** 策略正在亏损。基本方法需要修订。\n")
	}
	sb.WriteString("\n")

	// Learning from market
	sb.WriteString("## 学习要求\n")
	sb.WriteString("进化后的提示词应该:\n")
	sb.WriteString("1. **从错误中学习:** 解决上述识别的具体问题\n")
	sb.WriteString("2. **市场适应:** 考虑市场情绪、波动率状态、趋势vs震荡条件\n")
	sb.WriteString("3. **风险意识:** 强调资本保护和合理的仓位规模\n")
	sb.WriteString("4. **模式识别:** 鼓励基于市场结构识别高概率设置\n")
	sb.WriteString("5. **持续改进:** 建立自我反思和适应心态\n\n")

	// Skillset enhancement
	sb.WriteString("## 技能增强\n")
	sb.WriteString("为角色配备缺失或未充分发展的技能:\n\n")

	sb.WriteString("**技术分析技能:**\n")
	sb.WriteString("- 多时间框架分析 (1h, 4h, daily) 用于趋势确认\n")
	sb.WriteString("- 支撑/阻力识别和价格行为解读\n")
	sb.WriteString("- 成交量分析用于确认突破/击穿\n")
	sb.WriteString("- 市场微观结构解读 (订单簿、盘口)\n\n")

	sb.WriteString("**信号处理与量化分析:**\n")
	sb.WriteString("- 傅里叶分析将价格分解为频率成分 (识别主导周期)\n")
	sb.WriteString("- 数字滤波 (低通去除噪声、高通检测状态变化、带通提取周期)\n")
	sb.WriteString("- 频谱分析测量市场周期性和隐藏节奏\n")
	sb.WriteString("- 小波变换用于多尺度时频分析\n")
	sb.WriteString("- 信噪比评估以区分模式与随机性\n")
	sb.WriteString("- 自相关和互相关用于领先-滞后关系\n")
	sb.WriteString("- **扩展卡尔曼滤波器 (EKF)** 与傅里叶观测:\n")
	sb.WriteString("  * 使用频域特征 (主导频率、谱功率) 作为观测值\n")
	sb.WriteString("  * 频谱图用于时空演化跟踪\n")
	sb.WriteString("  * 状态估计: 从噪声观测预测下一个市场状态 (趋势、波动率、状态)\n")
	sb.WriteString("  * 非线性动力学建模: EKF 处理非高斯、非线性市场行为\n")
	sb.WriteString("  * 自适应滤波: 随着新数据到达持续更新状态估计\n\n")

	sb.WriteString("**风险管理专业知识:**\n")
	sb.WriteString("- 基于波动率和账户风险的动态仓位规模\n")
	sb.WriteString("- 使用 ATR、结构或百分比方法放置止损\n")
	sb.WriteString("- 投资组合热度管理 (所有仓位的总风险)\n")
	sb.WriteString("- 相关性意识以避免过度集中\n\n")

	sb.WriteString("**市场心理与情绪:**\n")
	sb.WriteString("- 从资金费率、持仓量、社交情绪识别恐惧/贪婪极端\n")
	sb.WriteString("- 当群众过度定位时的逆向思维\n")
	sb.WriteString("- 识别市场状态变化 (趋势 → 震荡、风险偏好 → 风险规避)\n")
	sb.WriteString("- BTC 主导地位和山寨币轮动周期意识\n\n")

	sb.WriteString("**执行与交易管理:**\n")
	sb.WriteString("- 入场时机优化 (避免 FOMO，等待回调)\n")
	sb.WriteString("- 分批进出策略以获得更好的平均价格\n")
	sb.WriteString("- 移动止损技术以捕捉趋势同时保护利润\n")
	sb.WriteString("- 知道何时不交易 (低流动性、高不确定性、震荡条件)\n\n")

	sb.WriteString("**自我意识与元认知:**\n")
	sb.WriteString("- 识别自身偏见 (近期偏见、确认偏见、过度自信)\n")
	sb.WriteString("- 从盈利和亏损中学习 (什么是运气 vs 技能?)\n")
	sb.WriteString("- 基于变化的市场条件调整策略\n")
	sb.WriteString("- 保持关于交易成功或失败原因的详细心智模型\n\n")

	sb.WriteString("## 自我诊断: 缺少什么技能?\n")
	sb.WriteString("**关键任务:** 分析当前提示词并识别它缺乏什么能力。\n\n")
	sb.WriteString("问自己:\n")
	sb.WriteString("1. 缺少哪些分析框架或方法论?\n")
	sb.WriteString("2. 未能考虑哪些市场动态或现象?\n")
	sb.WriteString("3. 哪些决策过程或启发式方法可以改善结果?\n")
	sb.WriteString("4. 缺少哪些领域知识 (加密货币特定、宏观、衍生品)?\n")
	sb.WriteString("5. 哪些统计、数学或计算技术会有价值?\n")
	sb.WriteString("6. 应该整合哪些心理或行为金融概念?\n\n")
	sb.WriteString("**要有创意和全面性。** 不要只解决上述问题 - 思考世界级交易员知道而此提示词未捕捉的内容。\n\n")

	sb.WriteString("## 你的任务\n")
	sb.WriteString("重写系统提示词以:\n")
	sb.WriteString("1. 解决识别的表现问题\n")
	sb.WriteString("2. 整合上述列出的缺失/弱技能\n")
	sb.WriteString("3. **添加你通过自我诊断识别的缺失能力**\n")
	sb.WriteString("4. 使角色更加复杂和具有市场意识\n")
	sb.WriteString("5. 保留当前运作良好的部分\n\n")
	sb.WriteString("只输出改进的系统提示词，不要解释或评论。\n")

	return sb.String()
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
		sb.WriteString("\n💡 系统正在通过遗传算法不断优化提示词策略，以提高交易表现。\n")
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
	sb.WriteString("\n💡 The system is continuously optimizing prompt strategies through genetic algorithms to improve trading performance.\n")
	return sb.String()
}
