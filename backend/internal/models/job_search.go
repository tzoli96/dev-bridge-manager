// backend/internal/models/job_search.go
package models

import "time"

type JobSearchProfile struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	CVText      string    `json:"cv_text" gorm:"column:cv_text;type:text"`
	Skills      string    `json:"skills" gorm:"type:text"`
	Preferences string    `json:"preferences" gorm:"type:text"`
	UpdatedBy   uint      `json:"updated_by"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (JobSearchProfile) TableName() string { return "job_search_profiles" }

type JobListing struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	Site        string     `json:"site"`
	ExternalURL string     `json:"external_url" gorm:"column:external_url"`
	Title       string     `json:"title"`
	Company     string     `json:"company"`
	Location    string     `json:"location"`
	Description string     `json:"description" gorm:"type:text"`
	PostedAt    *time.Time `json:"posted_at"`
	ScrapedAt   time.Time  `json:"scraped_at"`
}

func (JobListing) TableName() string { return "job_listings" }

type JobMatch struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	JobListingID    uint       `json:"job_listing_id" gorm:"column:job_listing_id;uniqueIndex"`
	JobListing      JobListing `json:"job_listing" gorm:"foreignKey:JobListingID"`
	Score           int        `json:"score"`
	Reasoning       string     `json:"reasoning" gorm:"type:text"`
	Status          string     `json:"status"`
	AppliedAt       *time.Time `json:"applied_at"`
	ApplicationText *string    `json:"application_text" gorm:"type:text"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (JobMatch) TableName() string { return "job_matches" }
