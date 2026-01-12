package backtest

import (
	"encoding/json"
	"fmt"
	"math"
	"nofx/logger"
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
	ID         string    `json:"id"`
	PromptText string    `json:"prompt_text"`
	Version    int       `json:"version"`
	CreatedAt  time.Time `json:"created_at"`

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
	basePrompt     string
	variants       []*PromptVariant
	currentVariant *PromptVariant
	generation     int
	populationSize int
	mutationRate   float64

	// Configuration
	config *PromptOptimizerConfig

	// Performance tracking
	decisionCounts  map[string]int      // variant ID -> decision count
	performanceData map[string]*Metrics // variant ID -> metrics
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
func NewPromptOptimizer(basePrompt string, config *PromptOptimizerConfig) *PromptOptimizer {
	if config == nil {
		config = DefaultPromptOptimizerConfig()
	}

	po := &PromptOptimizer{
		basePrompt:      basePrompt,
		variants:        make([]*PromptVariant, 0),
		generation:      1,
		populationSize:  config.PopulationSize,
		mutationRate:    config.MutationRate,
		config:          config,
		decisionCounts:  make(map[string]int),
		performanceData: make(map[string]*Metrics),
	}

	// Create initial variant (base prompt)
	baseVariant := &PromptVariant{
		ID:           "base",
		PromptText:   basePrompt,
		Version:      1,
		CreatedAt:    time.Now(),
		Generation:   1,
		IsActive:     true,
		FitnessScore: 0.0,
	}

	po.variants = append(po.variants, baseVariant)
	po.currentVariant = baseVariant

	logger.Infof("[PromptOptimizer] Initialized with base prompt (generation: 1)")

	return po
}

// GetCurrentPrompt returns the currently active prompt variant
func (po *PromptOptimizer) GetCurrentPrompt() string {
	if po.currentVariant != nil {
		return po.currentVariant.PromptText
	}
	return po.basePrompt
}

// GetAllVariants returns a copy of all prompt variants for inspection
func (po *PromptOptimizer) GetAllVariants() []*PromptVariant {
	result := make([]*PromptVariant, len(po.variants))
	copy(result, po.variants)
	return result
}

// GetCurrentVariant returns the currently active variant
func (po *PromptOptimizer) GetCurrentVariant() *PromptVariant {
	return po.currentVariant
}

// GetGeneration returns the current generation number
func (po *PromptOptimizer) GetGeneration() int {
	return po.generation
}

// ActivateVariant switches to a specific variant by ID
func (po *PromptOptimizer) ActivateVariant(variantID string) error {
	for _, v := range po.variants {
		if v.ID == variantID {
			po.currentVariant = v
			v.IsActive = true
			// Mark others as inactive
			for _, other := range po.variants {
				if other.ID != variantID {
					other.IsActive = false
				}
			}
			return nil
		}
	}
	return fmt.Errorf("variant %s not found", variantID)
}

// RecordDecisionOutcome records the outcome of a decision made with a specific prompt
func (po *PromptOptimizer) RecordDecisionOutcome(variantID string, metrics *Metrics) {
	if !po.config.EnableOptimization {
		return
	}

	po.decisionCounts[variantID]++
	po.performanceData[variantID] = metrics

	// Update variant metrics
	for _, variant := range po.variants {
		if variant.ID == variantID {
			variant.TotalDecisions = po.decisionCounts[variantID]
			variant.TotalReturn = metrics.TotalReturnPct
			variant.WinRate = metrics.WinRate
			variant.ProfitFactor = metrics.ProfitFactor
			variant.SharpeRatio = metrics.SharpeRatio
			variant.MaxDrawdown = metrics.MaxDrawdownPct
			variant.FitnessScore = po.calculateFitness(metrics)
			break
		}
	}
}

// ShouldEvolve determines if it's time to evolve prompts
func (po *PromptOptimizer) ShouldEvolve(currentCycle int) bool {
	if !po.config.EnableOptimization {
		return false
	}

	// Evolve every EvaluationCycles
	if currentCycle%po.config.EvaluationCycles != 0 {
		return false
	}

	// Check if we have enough data
	totalDecisions := 0
	for _, count := range po.decisionCounts {
		totalDecisions += count
	}

	return totalDecisions >= po.config.MinDecisionsPerTest
}

// EvolvePrompts creates new generation of prompts based on performance
func (po *PromptOptimizer) EvolvePrompts() error {
	if !po.config.EnableOptimization {
		return nil
	}

	logger.Infof("[PromptOptimizer] 🧬 Evolving prompts (generation %d → %d)", po.generation, po.generation+1)

	// Sort variants by fitness
	sort.Slice(po.variants, func(i, j int) bool {
		return po.variants[i].FitnessScore > po.variants[j].FitnessScore
	})

	// Log current performance
	for i, variant := range po.variants {
		if variant.TotalDecisions > 0 {
			logger.Infof("  Variant %s (gen %d): Fitness=%.3f, Return=%.2f%%, WinRate=%.1f%%, Decisions=%d",
				variant.ID, variant.Generation, variant.FitnessScore,
				variant.TotalReturn, variant.WinRate, variant.TotalDecisions)
		}
		if i >= 2 { // Only log top 3
			break
		}
	}

	// Keep top performers
	topVariants := po.variants[:po.config.TopVariantsToKeep]

	// Generate new variants
	newVariants := make([]*PromptVariant, 0)
	newVariants = append(newVariants, topVariants...) // Keep elite

	// Create children through crossover and mutation
	for len(newVariants) < po.populationSize {
		// Select two parents (tournament selection)
		parent1 := po.tournamentSelect()
		parent2 := po.tournamentSelect()

		// Create child through crossover
		child := po.crossover(parent1, parent2)

		// Apply mutation
		child = po.mutate(child)

		newVariants = append(newVariants, child)
	}

	// Update generation
	po.generation++
	po.variants = newVariants

	// Set new current variant (best from new generation)
	po.currentVariant = po.variants[0]

	// Reset tracking
	po.decisionCounts = make(map[string]int)
	po.performanceData = make(map[string]*Metrics)

	logger.Infof("[PromptOptimizer] ✅ Evolution complete: %d variants in generation %d", len(po.variants), po.generation)
	logger.Infof("[PromptOptimizer] New champion: %s (fitness: %.3f)", po.currentVariant.ID, po.currentVariant.FitnessScore)

	return nil
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

// tournamentSelect selects a variant using tournament selection
func (po *PromptOptimizer) tournamentSelect() *PromptVariant {
	// Simple tournament: pick 2 random, return best
	if len(po.variants) < 2 {
		return po.variants[0]
	}

	idx1 := 0
	idx2 := 1
	if len(po.variants) > 2 {
		// In production, use proper random selection
		idx2 = len(po.variants) / 2
	}

	if po.variants[idx1].FitnessScore > po.variants[idx2].FitnessScore {
		return po.variants[idx1]
	}
	return po.variants[idx2]
}

// crossover creates a child prompt by combining two parent prompts
func (po *PromptOptimizer) crossover(parent1, parent2 *PromptVariant) *PromptVariant {
	// Simple crossover: take sections from each parent
	lines1 := strings.Split(parent1.PromptText, "\n")
	lines2 := strings.Split(parent2.PromptText, "\n")

	childLines := make([]string, 0)

	// Alternate taking lines from each parent
	maxLen := len(lines1)
	if len(lines2) > maxLen {
		maxLen = len(lines2)
	}

	for i := 0; i < maxLen; i++ {
		if i%2 == 0 && i < len(lines1) {
			childLines = append(childLines, lines1[i])
		} else if i < len(lines2) {
			childLines = append(childLines, lines2[i])
		}
	}

	childPrompt := strings.Join(childLines, "\n")

	return &PromptVariant{
		ID:         fmt.Sprintf("gen%d-v%d", po.generation+1, len(po.variants)+1),
		PromptText: childPrompt,
		Version:    po.generation + 1,
		CreatedAt:  time.Now(),
		Generation: po.generation + 1,
		IsActive:   true,
	}
}

// mutate applies random mutations to a prompt variant
func (po *PromptOptimizer) mutate(variant *PromptVariant) *PromptVariant {
	// Apply mutation with probability
	if po.mutationRate == 0 {
		return variant
	}

	// Simple mutation strategies (in production, use more sophisticated NLP)
	mutations := []string{
		"Be more aggressive in taking profits",
		"Focus on risk management and capital preservation",
		"Prioritize high-confidence setups only",
		"Consider market regime when making decisions",
		"Use tighter stop-losses to limit downside",
		"Let winning positions run longer",
		"Reduce position sizes during uncertainty",
		"Pay attention to volume and momentum",
	}

	// Add a random mutation phrase
	mutationText := mutations[0] // In production, pick random
	variant.PromptText = variant.PromptText + "\n\nADDITIONAL GUIDANCE: " + mutationText

	return variant
}

// SaveState saves the optimizer state to disk
func (po *PromptOptimizer) SaveState(runID string) error {
	dir := filepath.Join("backtests", runID)
	os.MkdirAll(dir, 0755)

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

	logger.Infof("[PromptOptimizer] 📂 Loaded state from %s (generation %d)", filename, po.generation)
	return nil
}
