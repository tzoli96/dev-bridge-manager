// backend/internal/services/similar_replies_scoring_test.go
package services

import (
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestExtractDomain(t *testing.T) {
	cases := []struct {
		name    string
		addr    string
		want    string
		wantErr bool
	}{
		{"plain address", "jane@example.com", "example.com", false},
		{"display name plus address", `"Jane Doe" <jane@example.com>`, "example.com", false},
		{"uppercase domain lowercased", "jane@EXAMPLE.COM", "example.com", false},
		{"malformed address", "not-an-address", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractDomain(tc.addr)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q", tc.addr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRecipientMatches(t *testing.T) {
	cases := []struct {
		name        string
		toAddresses string
		fromAddress string
		want        bool
	}{
		{"exact address match", "Jane Doe <jane@example.com>", "jane@example.com", true},
		{"domain match, different mailbox", "Bob <bob@example.com>", "jane@example.com", true},
		{"multi-recipient header, one matches", "Alice <alice@other.com>, Bob <bob@example.com>", "jane@example.com", true},
		{"case-insensitive match", "JANE@EXAMPLE.COM", "jane@example.com", true},
		{"no match", "Alice <alice@other.com>", "jane@example.com", false},
		{"empty from address", "Alice <alice@other.com>", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RecipientMatches(tc.toAddresses, tc.fromAddress)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCandidateQualifies(t *testing.T) {
	cases := []struct {
		name                string
		candidate           models.Email
		incomingThreadID    string
		incomingFromAddress string
		want                bool
	}{
		{
			"same thread qualifies regardless of recipient",
			models.Email{ThreadID: "t1", ToAddresses: "someone-else@other.com"},
			"t1", "jane@example.com",
			true,
		},
		{
			"different thread, recipient domain matches",
			models.Email{ThreadID: "t2", ToAddresses: "jane@example.com"},
			"t1", "jane@example.com",
			true,
		},
		{
			"different thread, no recipient match",
			models.Email{ThreadID: "t2", ToAddresses: "someone-else@other.com"},
			"t1", "jane@example.com",
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CandidateQualifies(tc.candidate, tc.incomingThreadID, tc.incomingFromAddress)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestScoreCandidate(t *testing.T) {
	t.Run("thread match always outscores lexical overlap", func(t *testing.T) {
		threadMatch := models.Email{ThreadID: "t1", Subject: "unrelated", Snippet: "totally different words"}
		score := ScoreCandidate(threadMatch, "t1", "some incoming content")
		if score != threadMatchScore {
			t.Fatalf("got %v, want %v", score, threadMatchScore)
		}
	})

	t.Run("cross-thread match uses Jaccard of subject+snippet vs content", func(t *testing.T) {
		candidate := models.Email{ThreadID: "t2", Subject: "Ajánlat", Snippet: "kék labda gurul"}
		score := ScoreCandidate(candidate, "t1", "kék labda áll")
		want := JaccardSimilarity("Ajánlat kék labda gurul", "kék labda áll")
		if score != want {
			t.Fatalf("got %v, want %v", score, want)
		}
	})

	t.Run("no thread match and no overlap scores zero", func(t *testing.T) {
		candidate := models.Email{ThreadID: "t2", Subject: "xyz", Snippet: "qrs"}
		score := ScoreCandidate(candidate, "t1", "completely unrelated content here")
		if score != 0 {
			t.Fatalf("got %v, want 0", score)
		}
	})
}
