// backend/internal/services/profile_context.go
package services

import (
	"dev-bridge-manager/internal/models"
	"fmt"
	"strings"
)

// BuildProfileContext renders the profile into a single prompt-ready text
// block so every consumer (draft-reply today, future agents later) gets the
// same formatting instead of re-assembling it from the raw fields themselves.
func BuildProfileContext(profile *models.Profile) string {
	if profile == nil {
		return ""
	}

	var sections []string
	if strings.TrimSpace(profile.Background) != "" {
		sections = append(sections, "Háttér: "+profile.Background)
	}
	if strings.TrimSpace(profile.Expertise) != "" {
		sections = append(sections, "Szakterület: "+profile.Expertise)
	}
	if strings.TrimSpace(profile.ToneRules) != "" {
		sections = append(sections, "Kommunikációs stílus: "+profile.ToneRules)
	}

	var sampleLines []string
	for _, s := range profile.Samples {
		if strings.TrimSpace(s.Content) == "" {
			continue
		}
		label := s.Label
		if label == "" {
			label = "Minta"
		}
		sampleLines = append(sampleLines, fmt.Sprintf("- [%s]: %s", label, s.Content))
	}
	if len(sampleLines) > 0 {
		sections = append(sections, "Írásminták:\n"+strings.Join(sampleLines, "\n"))
	}

	return strings.Join(sections, "\n\n")
}
