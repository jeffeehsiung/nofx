package config

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

// FailureThresholdsJSON represents persisted failure thresholds (mirrors decision.FailureThresholds)
// Defined here to avoid circular imports between config and decision
type FailureThresholdsJSON struct {
	WeakVolumeThreshold      float64 `json:"weakVolumeThreshold"`
	WeakOIThreshold          float64 `json:"weakOIThreshold"`
	PrematureVolumeThreshold float64 `json:"prematureVolumeThreshold"`
	PrematureOIThreshold     float64 `json:"prematureOIThreshold"`
	VolumeDecayThreshold     float64 `json:"volumeDecayThreshold"`
	OIDecayThreshold         float64 `json:"oIDecayThreshold"`
	SpreadWorseningMultiple  float64 `json:"spreadWorseningMultiple"`
	DepthReductionThreshold  float64 `json:"depthReductionThreshold"`
}

// CalibratedThresholdsConfig represents persisted calibrated thresholds from monthly runs
type CalibratedThresholdsConfig struct {
	GeneratedAt    time.Time             `json:"generatedAt"`
	SampleSize     int                   `json:"sampleSize"`
	Thresholds     FailureThresholdsJSON `json:"thresholds"`
	CalibrationOK  bool                  `json:"calibrationOk"`
	Summary        string                `json:"summary"`
	SourceTraderID string                `json:"sourceTraderID,omitempty"` // If calibrated from specific trader
}

// LoadCalibratedThresholdsJSON loads thresholds from disk if available
// Returns (thresholds JSON, isValid, error)
// Caller is responsible for converting JSON to decision.FailureThresholds
func LoadCalibratedThresholdsJSON(configPath string) (FailureThresholdsJSON, bool, error) {
	defaults := FailureThresholdsJSON{
		WeakVolumeThreshold:      0.90,
		WeakOIThreshold:          0.30,
		PrematureVolumeThreshold: 0.90,
		PrematureOIThreshold:     0.50,
		VolumeDecayThreshold:     -0.30,
		OIDecayThreshold:         -0.20,
		SpreadWorseningMultiple:  2.0,
		DepthReductionThreshold:  0.50,
	}

	// Default path if not specified
	if configPath == "" {
		configPath = "./config/calibrated_thresholds.json"
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// File doesn't exist - not an error, just use defaults
		return defaults, false, nil
	}

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return defaults, false, fmt.Errorf("failed to read calibrated thresholds: %w", err)
	}

	// Parse JSON
	var config CalibratedThresholdsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return defaults, false, fmt.Errorf("failed to parse calibrated thresholds JSON: %w", err)
	}

	// Validate
	if !config.CalibrationOK || config.SampleSize < 30 {
		return defaults, false, nil
	}

	return config.Thresholds, true, nil
}

// DriftAlert represents a threshold that has drifted significantly
type DriftAlert struct {
	ThresholdName    string  `json:"thresholdName"`
	OldValue         float64 `json:"oldValue"`
	NewValue         float64 `json:"newValue"`
	DriftPercentage  float64 `json:"driftPercentage"`
	AlertTriggered   bool    `json:"alertTriggered"`
	AlertDescription string  `json:"alertDescription"`
}

// CheckThresholdDriftJSON compares old and new thresholds (as JSON), returns alerts for significant changes
// thresholdPct is the percentage change threshold to trigger an alert (e.g., 10 for 10%)
func CheckThresholdDriftJSON(oldThresholds, newThresholds FailureThresholdsJSON, thresholdPct float64) []DriftAlert {
	alerts := []DriftAlert{}

	type thresholdPair struct {
		name string
		old  float64
		new  float64
	}

	pairs := []thresholdPair{
		{"WeakVolumeThreshold", oldThresholds.WeakVolumeThreshold, newThresholds.WeakVolumeThreshold},
		{"WeakOIThreshold", oldThresholds.WeakOIThreshold, newThresholds.WeakOIThreshold},
		{"PrematureVolumeThreshold", oldThresholds.PrematureVolumeThreshold, newThresholds.PrematureVolumeThreshold},
		{"PrematureOIThreshold", oldThresholds.PrematureOIThreshold, newThresholds.PrematureOIThreshold},
		{"VolumeDecayThreshold", oldThresholds.VolumeDecayThreshold, newThresholds.VolumeDecayThreshold},
		{"OIDecayThreshold", oldThresholds.OIDecayThreshold, newThresholds.OIDecayThreshold},
		{"SpreadWorseningMultiple", oldThresholds.SpreadWorseningMultiple, newThresholds.SpreadWorseningMultiple},
		{"DepthReductionThreshold", oldThresholds.DepthReductionThreshold, newThresholds.DepthReductionThreshold},
	}

	for _, p := range pairs {
		if p.old == 0 {
			continue // Skip division by zero
		}

		drift := math.Abs((p.new-p.old)/p.old) * 100
		alert := DriftAlert{
			ThresholdName:   p.name,
			OldValue:        p.old,
			NewValue:        p.new,
			DriftPercentage: drift,
			AlertTriggered:  drift > thresholdPct,
		}

		if alert.AlertTriggered {
			direction := "increased"
			if p.new < p.old {
				direction = "decreased"
			}
			alert.AlertDescription = fmt.Sprintf("%s %s by %.1f%% (%.4f → %.4f)",
				p.name, direction, drift, p.old, p.new)
		}

		alerts = append(alerts, alert)
	}

	return alerts
}

// SaveThresholdDriftReport saves drift detection report to disk
func SaveThresholdDriftReport(reportPath string, oldThresholds, newThresholds FailureThresholdsJSON, alerts []DriftAlert) error {
	report := map[string]interface{}{
		"generatedAt":   time.Now().Format(time.RFC3339),
		"oldThresholds": oldThresholds,
		"newThresholds": newThresholds,
		"driftAlerts":   alerts,
		"triggeredCount": func() int {
			count := 0
			for _, a := range alerts {
				if a.AlertTriggered {
					count++
				}
			}
			return count
		}(),
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal drift report: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(reportPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	if err := os.WriteFile(reportPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write drift report: %w", err)
	}

	return nil
}

// NotifyThresholdDrift logs significant threshold changes
// if verboseLogging is true, logs all changes; otherwise only alerts
func NotifyThresholdDrift(alerts []DriftAlert, verboseLogging bool) {
	triggeredAlerts := []DriftAlert{}
	for _, a := range alerts {
		if a.AlertTriggered {
			triggeredAlerts = append(triggeredAlerts, a)
		}
	}

	if len(triggeredAlerts) > 0 {
		// Use standard library fmt instead of logger to avoid circular imports
		fmt.Printf("⚠️  Threshold drift detected: %d thresholds changed significantly\n", len(triggeredAlerts))
		for _, alert := range triggeredAlerts {
			fmt.Printf("   %s\n", alert.AlertDescription)
		}
	}

	if verboseLogging {
		for _, a := range alerts {
			if !a.AlertTriggered {
				fmt.Printf("   %s %s by %.1f%% (%.4f → %.4f)\n",
					a.ThresholdName,
					func() string {
						if a.NewValue > a.OldValue {
							return "↑"
						}
						return "↓"
					}(),
					a.DriftPercentage, a.OldValue, a.NewValue)
			}
		}
	}
}
