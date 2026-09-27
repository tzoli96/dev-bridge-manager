// backend/internal/models/search.go
package models

type SearchClientResult struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type SearchProjectResult struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type SearchTaskResult struct {
	ID          uint   `json:"id"`
	Title       string `json:"title"`
	ProjectID   uint   `json:"project_id"`
	ProjectName string `json:"project_name"`
}

type SearchResponse struct {
	Success  bool                  `json:"success"`
	Message  string                `json:"message,omitempty"`
	Clients  []SearchClientResult  `json:"clients"`
	Projects []SearchProjectResult `json:"projects"`
	Tasks    []SearchTaskResult    `json:"tasks"`
}
