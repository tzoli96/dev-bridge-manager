// backend/internal/models/project_environment.go
package models

import "time"

type ProjectEnvironment struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	ProjectID  uint      `json:"project_id" gorm:"not null;index"`
	Name       string    `json:"name" gorm:"not null" validate:"required,min=1,max=255"`
	URL        string    `json:"url"`
	GitRepoURL string    `json:"git_repo_url" gorm:"column:git_repo_url"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ProjectEnvironmentRequest is used for both create and update: the
// two payloads are identical, unlike passwords where the secret handling
// could diverge.
type ProjectEnvironmentRequest struct {
	Name       string `json:"name" validate:"required,min=1,max=255"`
	URL        string `json:"url"`
	GitRepoURL string `json:"git_repo_url"`
}
