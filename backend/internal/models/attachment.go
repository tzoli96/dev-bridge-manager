package models

import "time"

type Attachment struct {
	ID           uint  `gorm:"primaryKey"`
	TaskID       uint  `gorm:"not null;index"`
	CommentID    *uint `gorm:"index"`
	Filename     string
	OriginalName string
	MimeType     string
	Size         int64
	UploadedBy   uint
	CreatedAt    time.Time
}

func (Attachment) TableName() string { return "attachments" }

type AttachmentDTO struct {
	ID           string          `json:"id"`
	TaskID       string          `json:"taskId"`
	CommentID    string          `json:"commentId,omitempty"`
	OriginalName string          `json:"originalName"`
	MimeType     string          `json:"mimeType"`
	Size         int64           `json:"size"`
	UploadedByID string          `json:"uploadedById"`
	UploadedBy   *TaskUserRefDTO `json:"uploadedBy,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	DownloadUrl  string          `json:"downloadUrl"`
}
