// backend/internal/services/draft_edit_examples.go
package services

import (
	"context"
	"log"

	"dev-bridge-manager/internal/models"

	"gorm.io/gorm"
)

const (
	editExampleTopK          = 2
	editExampleMinSimilarity = 0.3
	editExampleMaxSimilarity = 0.8
)

// DraftEditExample pairs an AI-generated draft with the text the user
// actually sent, so the draft-reply prompt can show the model concrete
// examples of how this user tends to edit its drafts - not just the final
// result (see FindSimilarReplies for that half of the picture).
type DraftEditExample struct {
	AIDraft string
	Sent    string
}

// FindDraftEditExamples returns up to editExampleTopK past drafts this
// account edited moderately (similarity between editExampleMinSimilarity and
// editExampleMaxSimilarity) before sending, most recent first. Very high
// similarity means barely any edit happened - nothing to learn from. Very
// low similarity usually means the draft was replaced for content/context
// reasons unrelated to style, which would be a misleading example. A failed
// query just logs and returns nil, matching FindSimilarReplies - a
// suggestion feature degrading gracefully beats failing the whole draft.
func FindDraftEditExamples(ctx context.Context, db *gorm.DB, account *models.GmailAccount) []DraftEditExample {
	var feedback []models.DraftFeedback
	err := db.WithContext(ctx).
		Joins("JOIN emails ON emails.id = draft_feedback.email_id").
		Where("emails.gmail_account_id = ? AND draft_feedback.similarity BETWEEN ? AND ?", account.ID, editExampleMinSimilarity, editExampleMaxSimilarity).
		Order("draft_feedback.created_at DESC").
		Limit(editExampleTopK).
		Find(&feedback).Error
	if err != nil {
		log.Printf("draft reply: edit-example query failed for account %d: %v", account.ID, err)
		return nil
	}

	examples := make([]DraftEditExample, 0, len(feedback))
	for _, f := range feedback {
		examples = append(examples, DraftEditExample{AIDraft: f.AIDraftText, Sent: f.SentText})
	}
	return examples
}
