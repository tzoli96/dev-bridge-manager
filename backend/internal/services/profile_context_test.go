// backend/internal/services/profile_context_test.go
package services

import (
	"dev-bridge-manager/internal/models"
	"strings"
	"testing"
)

func TestBuildProfileContextEmptyProfileReturnsEmptyString(t *testing.T) {
	profile := &models.Profile{}
	if got := BuildProfileContext(profile); got != "" {
		t.Fatalf("expected empty string for empty profile, got %q", got)
	}
}

func TestBuildProfileContextNilProfileReturnsEmptyString(t *testing.T) {
	if got := BuildProfileContext(nil); got != "" {
		t.Fatalf("expected empty string for nil profile, got %q", got)
	}
}

func TestBuildProfileContextIncludesAllFilledSections(t *testing.T) {
	profile := &models.Profile{
		Background: "Fejlesztő és vállalkozó",
		Expertise:  "Backend rendszerek, Go, Python",
		ToneRules:  "Közvetlen, tegeződő, rövid mondatok",
		Samples: []models.ProfileSample{
			{Label: "Ügyfélnek írt email", Content: "Szia! Köszönöm a megkeresést."},
		},
	}
	got := BuildProfileContext(profile)
	for _, want := range []string{
		"Fejlesztő és vállalkozó", "Backend rendszerek", "tegeződő",
		"Ügyfélnek írt email", "Köszönöm a megkeresést",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected context to contain %q, got: %s", want, got)
		}
	}
}

func TestBuildProfileContextSkipsSamplesWithEmptyContent(t *testing.T) {
	profile := &models.Profile{
		Samples: []models.ProfileSample{{Label: "Üres", Content: ""}},
	}
	if got := BuildProfileContext(profile); got != "" {
		t.Fatalf("expected empty string when only sample is blank, got %q", got)
	}
}
