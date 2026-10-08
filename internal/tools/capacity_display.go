package tools

import (
	"bytes"
	"encoding/json"
	"strings"
)

// Keep the MCP sizing response operational. The backend API retains calibration
// diagnostics; exposing them here encourages verbose, misleading sizing prose.
func capacityDisplay(data []byte) []byte {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var result map[string]any
	if err := decoder.Decode(&result); err != nil || result == nil {
		return data
	}
	removeCalibration := func(row map[string]any) {
		for _, key := range []string{"calibration", "calibration_profile", "fit_amdahl_serial_fraction", "infer_amdahl_serial_fraction"} {
			delete(row, key)
		}
	}
	removeCalibration(result)
	if rows, ok := result["workloads"].([]any); ok {
		for _, value := range rows {
			if row, ok := value.(map[string]any); ok {
				removeCalibration(row)
			}
		}
	}
	if row, ok := result["recommended"].(map[string]any); ok {
		delete(row, "amdahl_efficiency")
	}
	if rows, ok := result["candidates"].([]any); ok {
		for _, value := range rows {
			if row, ok := value.(map[string]any); ok {
				delete(row, "amdahl_efficiency")
			}
		}
	}
	for _, key := range []string{"warnings", "assumptions"} {
		if rows, ok := result[key].([]any); ok {
			kept := make([]any, 0, len(rows))
			for _, value := range rows {
				note, ok := value.(string)
				if ok {
					lower := strings.ToLower(note)
					if strings.Contains(lower, "extrapolat") || strings.Contains(lower, "amdahl") || strings.Contains(lower, "benchmark model work") || strings.Contains(lower, "cache misses run bounded calibration") {
						continue
					}
				}
				kept = append(kept, value)
			}
			result[key] = kept
		}
	}
	result["estimate_notice"] = "Experimental estimate; validate with a representative workload."
	output, err := json.Marshal(result)
	if err != nil {
		return data
	}
	return output
}
