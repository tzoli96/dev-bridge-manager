// backend/internal/models/gmail_account.go
package models

import "time"

type GmailAccount struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	UserID        uint       `json:"user_id" gorm:"not null;uniqueIndex"`
	EmailAddress  string     `json:"email_address" gorm:"size:255;not null"`
	AccessToken   string     `json:"-" gorm:"type:text;not null"`
	RefreshToken  string     `json:"-" gorm:"type:text;not null"`
	TokenExpiry   time.Time  `json:"-"`
	LastHistoryID string     `json:"-" gorm:"column:last_history_id;size:50"`
	NeedsReauth   bool       `json:"needs_reauth" gorm:"default:false"`
	LastSyncedAt  *time.Time `json:"last_synced_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (GmailAccount) TableName() string { return "gmail_accounts" }

type GmailStatusResponse struct {
	Success      bool       `json:"success"`
	Connected    bool       `json:"connected"`
	EmailAddress string     `json:"email_address,omitempty"`
	LastSyncedAt *time.Time `json:"last_synced_at,omitempty"`
	NeedsReauth  bool       `json:"needs_reauth,omitempty"`
}
