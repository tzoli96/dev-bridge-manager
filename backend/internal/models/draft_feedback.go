// backend/internal/models/draft_feedback.go
package models

import "time"

// DraftFeedback logs how much a user changed an AI-generated draft reply
// before sending it, as a lightweight future quality signal. Purely a
// measurement record - nothing reads it to take automatic action.
type DraftFeedback struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	EmailID     uint      `json:"email_id" gorm:"not null;index"`
	AIDraftText string    `json:"ai_draft_text" gorm:"column:ai_draft_text;type:text;not null"`
	SentText    string    `json:"sent_text" gorm:"type:text;not null"`
	Similarity  float64   `json:"similarity" gorm:"not null"`
	CreatedAt   time.Time `json:"created_at"`
}

func (DraftFeedback) TableName() string { return "draft_feedback" }
