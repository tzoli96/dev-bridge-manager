// backend/internal/services/client_status_ai.go
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

// ClientStatusDrafter is the seam RunClientStatusEmailDrafts calls through,
// so tests can inject a fake instead of hitting the real AI service.
type ClientStatusDrafter interface {
	DraftClientStatusEmail(ctx context.Context, facts ClientStatusFacts) (subject, body string, err error)
}

var _ ClientStatusDrafter = (*ClientStatusAIService)(nil)

// ClientStatusFacts is the weekly, per-client data handed to the AI service
// to draft a status email from - see ClientWeeklySummary for how it's
// computed.
type ClientStatusFacts struct {
	ClientName           string
	PeriodStart          string
	PeriodEnd            string
	CompletedTasks       []string
	HoursLogged          float64
	InvoicesCreated      int
	InvoicesCreatedTotal float64
	InvoicesPaid         int
}

// ClientStatusAIService calls the AI service's /client-status-email
// endpoint, following the same AI_SERVICE_URL/timeout convention as
// DraftReplyService.
type ClientStatusAIService struct {
	httpClient *http.Client
	baseURL    string
}

func NewClientStatusAIService() *ClientStatusAIService {
	baseURL := os.Getenv("AI_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://ai:8000"
	}
	return &ClientStatusAIService{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
	}
}

type clientStatusEmailRequest struct {
	ClientName           string   `json:"client_name"`
	PeriodStart          string   `json:"period_start"`
	PeriodEnd            string   `json:"period_end"`
	CompletedTasks       []string `json:"completed_tasks"`
	HoursLogged          float64  `json:"hours_logged"`
	InvoicesCreated      int      `json:"invoices_created"`
	InvoicesCreatedTotal float64  `json:"invoices_created_total"`
	InvoicesPaid         int      `json:"invoices_paid"`
}

type clientStatusEmailResponse struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

func (s *ClientStatusAIService) DraftClientStatusEmail(ctx context.Context, facts ClientStatusFacts) (string, string, error) {
	completedTasks := facts.CompletedTasks
	if completedTasks == nil {
		completedTasks = []string{}
	}
	payload, err := json.Marshal(clientStatusEmailRequest{
		ClientName:           facts.ClientName,
		PeriodStart:          facts.PeriodStart,
		PeriodEnd:            facts.PeriodEnd,
		CompletedTasks:       completedTasks,
		HoursLogged:          facts.HoursLogged,
		InvoicesCreated:      facts.InvoicesCreated,
		InvoicesCreatedTotal: facts.InvoicesCreatedTotal,
		InvoicesPaid:         facts.InvoicesPaid,
	})
	if err != nil {
		return "", "", fmt.Errorf("encoding client-status-email request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/client-status-email", bytes.NewReader(payload))
	if err != nil {
		return "", "", fmt.Errorf("building client-status-email request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("calling ai service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("reading client-status-email response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("ai service returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed clientStatusEmailResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", fmt.Errorf("parsing client-status-email response: %w", err)
	}

	return parsed.Subject, parsed.Body, nil
}
