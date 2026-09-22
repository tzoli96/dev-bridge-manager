// backend/internal/services/text_similarity_test.go
package services

import "testing"

func TestTokenize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"lowercases and splits on punctuation", "Szia, mikor kezdünk?", []string{"szia", "mikor", "kezdünk"}},
		{"drops tokens shorter than 3 characters", "az és mi alma", []string{"alma"}},
		{"empty input", "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Tokenize(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestJaccardSimilarity(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want float64
	}{
		{"identical text", "Szia, mikor kezdünk?", "Szia, mikor kezdünk?", 1.0},
		{"no common words", "kék labda gurul", "piros autó száguld", 0.0},
		{"partial overlap", "kék labda gurul gyorsan", "kék labda áll csendben", 2.0 / 6.0},
		{"both empty", "", "", 0.0},
		{"one empty", "kék labda", "", 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := JaccardSimilarity(tc.a, tc.b)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
