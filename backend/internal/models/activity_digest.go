// backend/internal/models/activity_digest.go
package models

type ActivityDigestTask struct {
	ID          uint   `json:"id"`
	Title       string `json:"title"`
	ProjectName string `json:"project_name"`
}

type ActivityDigestResponse struct {
	Success bool `json:"success"`

	NewTasksCount int                  `json:"new_tasks_count"`
	NewTasks      []ActivityDigestTask `json:"new_tasks"`

	CompletedTasksCount int                  `json:"completed_tasks_count"`
	CompletedTasks      []ActivityDigestTask `json:"completed_tasks"`

	NewInvoicesCount int     `json:"new_invoices_count"`
	NewInvoicesTotal float64 `json:"new_invoices_total"`

	NewClientEmailsCount int `json:"new_client_emails_count"`

	NewStallFlagsCount int `json:"new_stall_flags_count"`
}
