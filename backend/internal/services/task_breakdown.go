// backend/internal/services/task_breakdown.go
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// TaskBreakdownItem mirrors a single proposed subtask.
type TaskBreakdownItem struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// TaskBreakdownGroup mirrors one proposed top-level task, with its related
// items (if any) nested as subtasks.
type TaskBreakdownGroup struct {
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Subtasks    []TaskBreakdownItem `json:"subtasks"`
}

// TaskBreakdowner is the seam email_handler.go calls through, so tests can
// inject a fake instead of hitting the real AI service.
type TaskBreakdowner interface {
	BreakdownIntoTasks(ctx context.Context, subject, emailContent string) ([]TaskBreakdownGroup, error)
}

var _ TaskBreakdowner = (*TaskBreakdownService)(nil)

// TaskBreakdownService calls the AI service's /task-breakdown endpoint.
// baseURL follows the same AI_SERVICE_URL convention as DraftReplyService;
// timeout matches it too, since this is a similarly heavy generation task.
type TaskBreakdownService struct {
	httpClient *http.Client
	baseURL    string
}

func NewTaskBreakdownService() *TaskBreakdownService {
	baseURL := os.Getenv("AI_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://ai:8000"
	}
	return &TaskBreakdownService{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
	}
}

type taskBreakdownRequest struct {
	EmailContent string `json:"email_content"`
	Subject      string `json:"subject"`
}

type taskBreakdownResponse struct {
	Groups []TaskBreakdownGroup `json:"groups"`
}

func (s *TaskBreakdownService) BreakdownIntoTasks(ctx context.Context, subject, emailContent string) ([]TaskBreakdownGroup, error) {
	payload, err := json.Marshal(taskBreakdownRequest{
		EmailContent: emailContent,
		Subject:      subject,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding task-breakdown request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/task-breakdown", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("building task-breakdown request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling ai service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading task-breakdown response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai service returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed taskBreakdownResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parsing task-breakdown response: %w", err)
	}

	return parsed.Groups, nil
}
