package tools

import "context"

// Soil returns a soil analysis for a location. It is a static placeholder for a
// future sensor/lab service; the ctx and location are accepted so the signature
// matches the other tools and a real implementation can slot in later.
func (r *Registry) Soil(_ context.Context, location string) map[string]any {
	return map[string]any{
		"loc":    location,
		"pH":     6.5,
		"N":      "medium",
		"P":      "low",
		"K":      "high",
		"advice": []string{"Add phosphorus-rich fertiliser"},
	}
}
