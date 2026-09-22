// backend/internal/services/text_similarity.go
package services

import (
	"strings"
	"unicode"
)

// Tokenize lowercases s, splits on runs of non-letter/non-digit characters,
// and drops tokens shorter than 3 characters - short tokens (particles,
// prepositions) contribute noise rather than signal to Jaccard similarity.
func Tokenize(s string) []string {
	var tokens []string
	var cur []rune
	flush := func() {
		if len(cur) >= 3 {
			tokens = append(tokens, string(cur))
		}
		cur = nil
	}
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur = append(cur, r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

// JaccardSimilarity returns |intersection| / |union| of the token sets of a
// and b - 1.0 means the same words, 0.0 means no shared words (or both
// inputs tokenize to nothing).
func JaccardSimilarity(a, b string) float64 {
	setA := toSet(Tokenize(a))
	setB := toSet(Tokenize(b))

	union := make(map[string]struct{}, len(setA)+len(setB))
	intersection := 0
	for t := range setA {
		union[t] = struct{}{}
		if _, ok := setB[t]; ok {
			intersection++
		}
	}
	for t := range setB {
		union[t] = struct{}{}
	}
	if len(union) == 0 {
		return 0
	}
	return float64(intersection) / float64(len(union))
}

func toSet(tokens []string) map[string]struct{} {
	set := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		set[t] = struct{}{}
	}
	return set
}
