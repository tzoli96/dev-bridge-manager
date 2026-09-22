// backend/internal/services/similar_replies_scoring.go
package services

import (
	"fmt"
	"net/mail"
	"strings"

	"dev-bridge-manager/internal/models"
)

// threadMatchScore is a fixed score for same-thread candidates, guaranteed
// higher than any JaccardSimilarity result (whose max is 1.0) so a thread
// match always outranks a cross-thread lexical match.
const threadMatchScore = 2.0

// ExtractDomain returns the lowercase domain part of an email address, e.g.
// "jane@example.com" -> "example.com". Accepts either a bare address or a
// "Display Name <addr>" form, since it's built on net/mail.ParseAddress.
func ExtractDomain(emailAddress string) (string, error) {
	addr, err := mail.ParseAddress(emailAddress)
	if err != nil {
		return "", fmt.Errorf("parsing address %q: %w", emailAddress, err)
	}
	parts := strings.SplitN(addr.Address, "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", fmt.Errorf("no domain in address %q", emailAddress)
	}
	return strings.ToLower(parts[1]), nil
}

// RecipientMatches reports whether toAddresses - a raw, possibly
// multi-recipient To header value stored on models.Email - contains
// fromAddress's exact email address or its domain. Case-insensitive.
func RecipientMatches(toAddresses, fromAddress string) bool {
	fromLower := strings.ToLower(strings.TrimSpace(fromAddress))
	if fromLower == "" {
		return false
	}
	toLower := strings.ToLower(toAddresses)
	if strings.Contains(toLower, fromLower) {
		return true
	}
	domain, err := ExtractDomain(fromAddress)
	if err != nil || domain == "" {
		return false
	}
	return strings.Contains(toLower, "@"+domain)
}

// CandidateQualifies reports whether a sent-folder candidate is eligible as
// a style example for a reply to incomingFromAddress in thread
// incomingThreadID: either the same thread, or a shared recipient
// address/domain.
func CandidateQualifies(candidate models.Email, incomingThreadID, incomingFromAddress string) bool {
	if incomingThreadID != "" && candidate.ThreadID == incomingThreadID {
		return true
	}
	return RecipientMatches(candidate.ToAddresses, incomingFromAddress)
}

// ScoreCandidate ranks a qualifying candidate. A same-thread candidate is
// the strongest possible signal (literally the same conversation) and
// always outranks a cross-thread lexical match, which falls back to
// JaccardSimilarity between the candidate's subject+snippet and the
// incoming email's content.
func ScoreCandidate(candidate models.Email, incomingThreadID, incomingContent string) float64 {
	if incomingThreadID != "" && candidate.ThreadID == incomingThreadID {
		return threadMatchScore
	}
	return JaccardSimilarity(candidate.Subject+" "+candidate.Snippet, incomingContent)
}
