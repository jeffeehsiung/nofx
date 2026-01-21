package decision

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const calibrationFilePath = "data/calibrated_thresholds.json"

// CalibratedThresholds stores the calibrated failure detection thresholds
type CalibratedThresholds struct {
	WeakVolumeThreshold      float64 `json:"weak_volume_threshold"`
	WeakOIThreshold          float64 `json:"weak_oi_threshold"`
	PrematureVolumeThreshold float64 `json:"premature_volume_threshold"`
	PrematureOIThreshold     float64 `json:"premature_oi_threshold"`
	VolumeDecayThreshold     float64 `json:"volume_decay_threshold"`
	OIDecayThreshold         float64 `json:"oi_decay_threshold"`
	SpreadWorseningMultiple  float64 `json:"spread_worsening_multiple"`
	DepthReductionThreshold  float64 `json:"depth_reduction_threshold"`

	// Bayesian models serialization
	BayesianModels map[string]*SerializedBayesianModel `json:"bayesian_models,omitempty"`

	// Metadata
	CalibratedAt     string             `json:"calibrated_at"`
	SampleSize       int                `json:"sample_size"`
	ConfidenceScores map[string]float64 `json:"confidence_scores,omitempty"`
	Metadata         map[string]string  `json:"metadata,omitempty"`
}

// SerializedBayesianModel is a serializable version of BayesianThreshold
type SerializedBayesianModel struct {
	PriorAlpha       float64 `json:"prior_alpha"`
	PriorBeta        float64 `json:"prior_beta"`
	PosteriorAlpha   float64 `json:"posterior_alpha"`
	PosteriorBeta    float64 `json:"posterior_beta"`
	CurrentThreshold float64 `json:"current_threshold"`
	Confidence       float64 `json:"confidence"`
}

// SaveCalibratedThresholds persists calibrated thresholds to disk
func SaveCalibratedThresholds(thresholds *CalibratedThresholds) error {
	// Ensure data directory exists
	dataDir := filepath.Dir(calibrationFilePath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(thresholds, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal thresholds: %w", err)
	}

	// Write to file
	if err := os.WriteFile(calibrationFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write calibration file: %w", err)
	}

	return nil
}

// LoadCalibratedThresholds reads persisted thresholds from disk
func LoadCalibratedThresholds() (*CalibratedThresholds, error) {
	// Check if file exists
	if _, err := os.Stat(calibrationFilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("calibration file not found: %s", calibrationFilePath)
	}

	// Read file
	data, err := os.ReadFile(calibrationFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read calibration file: %w", err)
	}

	// Unmarshal
	var thresholds CalibratedThresholds
	if err := json.Unmarshal(data, &thresholds); err != nil {
		return nil, fmt.Errorf("failed to unmarshal thresholds: %w", err)
	}

	return &thresholds, nil
}

// HasCalibratedThresholds checks if calibrated thresholds exist
func HasCalibratedThresholds() bool {
	_, err := os.Stat(calibrationFilePath)
	return err == nil
}

// GetDefaultCalibratedThresholds returns default thresholds for first-time use
func GetDefaultCalibratedThresholds() *CalibratedThresholds {
	return &CalibratedThresholds{
		WeakVolumeThreshold:      0.90,
		WeakOIThreshold:          0.30,
		PrematureVolumeThreshold: 0.90,
		PrematureOIThreshold:     0.50,
		VolumeDecayThreshold:     -0.30,
		OIDecayThreshold:         -0.20,
		SpreadWorseningMultiple:  2.0,
		DepthReductionThreshold:  0.50,
		CalibratedAt:             time.Now().Format(time.RFC3339),
		SampleSize:               0,
		ConfidenceScores:         make(map[string]float64),
		BayesianModels:           make(map[string]*SerializedBayesianModel),
		Metadata: map[string]string{
			"quality_score":      "0.0",
			"reliable":           "false",
			"calibration_method": "defaults",
			"source":             "hardcoded_defaults",
		},
	}
}
