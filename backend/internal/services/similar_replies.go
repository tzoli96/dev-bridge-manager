// backend/internal/services/similar_replies.go
package services

import (
	"context"
	"sort"

	"dev-bridge-manager/internal/models"

	"gorm.io/gorm"
)

const (
	similarReplyCandidateLimit = 200
	similarReplyTopK           = 2
)

// FindSimilarReplies returns up to similarReplyTopK bodies of previously
// sent replies that are stylistically relevant to the email being answered
// (incoming) - the RAG-lite retrieval step for the draft-reply prompt.
// Candidates are drawn from the most recent similarReplyCandidateLimit
// "sent" rows for this account, filtered by CandidateQualifies and ranked
// by ScoreCandidate. Returns nil if no qualifying candidate remains. A
// failed live fetch for an individual winning candidate just drops that
// candidate - it never fails the whole call.
func FindSimilarReplies(ctx context.Context, db *gorm.DB, account *models.GmailAccount, incoming *models.Email, content string, gmailAPI GmailAPI) []string {
	var candidates []models.Email
	if err := db.WithContext(ctx).
		Where("gmail_account_id = ? AND folder = 'sent'", account.ID).
		Order("received_at DESC").
		Limit(similarReplyCandidateLimit).
		Find(&candidates).Error; err != nil {
		return nil
	}

	type scoredCandidate struct {
		email models.Email
		score float64
	}
	var qualifying []scoredCandidate
	for _, c := range candidates {
		if !CandidateQualifies(c, incoming.ThreadID, incoming.FromAddress) {
			continue
		}
		score := ScoreCandidate(c, incoming.ThreadID, content)
		if score <= 0 {
			continue
		}
		qualifying = append(qualifying, scoredCandidate{email: c, score: score})
	}
	sort.Slice(qualifying, func(i, j int) bool { return qualifying[i].score > qualifying[j].score })

	var replies []string
	for _, q := range qualifying {
		if len(replies) >= similarReplyTopK {
			break
		}
		full, err := gmailAPI.GetFullMessage(ctx, account, q.email.GmailMessageID)
		if err != nil {
			continue
		}
		replies = append(replies, full.BodyText)
	}
	return replies
}
