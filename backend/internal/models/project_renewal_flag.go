// backend/internal/models/project_renewal_flag.go
package models

import "time"

// ProjectRenewalFlag flags a project whose contract_end_date has come
// within the renewal lead time (or already passed). Created by
// services.RunProjectRenewalCheck, surfaced to super_admin users only (see
// routes/project_renewal_routes.go). Dismissing a flag just snoozes it -
// RunProjectRenewalCheck re-flags the project once
// projectRenewalRepeatInterval has elapsed and the contract still hasn't
// been renewed (contract_end_date pushed back out beyond the lead time).
type ProjectRenewalFlag struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	ProjectID       uint       `json:"project_id" gorm:"not null"`
	ContractEndDate time.Time  `json:"contract_end_date" gorm:"not null"`
	Status          string     `json:"status" gorm:"size:20;not null;default:'pending'"`
	CreatedAt       time.Time  `json:"created_at"`
	DismissedBy     *uint      `json:"dismissed_by"`
	DismissedAt     *time.Time `json:"dismissed_at"`
}

func (ProjectRenewalFlag) TableName() string { return "project_renewal_flags" }

// ProjectRenewalFlagWithNames adds the display name ListProjectRenewalFlags
// joins in, for the dashboard's renewal widget.
type ProjectRenewalFlagWithNames struct {
	ProjectRenewalFlag
	ProjectName string `json:"project_name"`
}

type ProjectRenewalFlagListResponse struct {
	Success bool                          `json:"success"`
	Message string                        `json:"message,omitempty"`
	Flags   []ProjectRenewalFlagWithNames `json:"flags,omitempty"`
}

type ProjectRenewalFlagActionResponse struct {
	Success bool                `json:"success"`
	Message string              `json:"message,omitempty"`
	Flag    *ProjectRenewalFlag `json:"flag,omitempty"`
}
