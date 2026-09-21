// backend/internal/models/profile.go
package models

import "time"

type Profile struct {
	ID         uint            `json:"id" gorm:"primaryKey"`
	Background string          `json:"background" gorm:"type:text"`
	Expertise  string          `json:"expertise" gorm:"type:text"`
	ToneRules  string          `json:"tone_rules" gorm:"column:tone_rules;type:text"`
	Samples    []ProfileSample `json:"samples" gorm:"foreignKey:ProfileID"`
	UpdatedBy  uint            `json:"updated_by"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

func (Profile) TableName() string { return "profiles" }

type ProfileSample struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	ProfileID uint   `json:"profile_id" gorm:"not null;index"`
	Label     string `json:"label" gorm:"size:100"`
	Content   string `json:"content" gorm:"type:text"`
}

func (ProfileSample) TableName() string { return "profile_samples" }

type ProfileSampleInput struct {
	Label   string `json:"label"`
	Content string `json:"content"`
}

type ProfileUpdateRequest struct {
	Background string               `json:"background"`
	Expertise  string               `json:"expertise"`
	ToneRules  string               `json:"tone_rules"`
	Samples    []ProfileSampleInput `json:"samples"`
}

type ProfileResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"message,omitempty"`
	Profile *Profile `json:"profile,omitempty"`
}
