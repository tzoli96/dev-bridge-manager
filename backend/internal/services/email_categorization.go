// backend/internal/services/email_categorization.go
package services

import (
	"bytes"
	"context"
	"dev-bridge-manager/internal/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// EmailCategorizer is the seam gmail_sync.go calls through, so tests can
// inject a fake instead of hitting the real AI service.
type EmailCategorizer interface {
	Categorize(ctx context.Context, subject, snippet, fromAddress, fromName string) (*string, error)
}

var _ EmailCategorizer = (*EmailCategorizationService)(nil)

// EmailCategorizationService calls the AI service's /categorize-email
// endpoint. baseURL defaults to the AI service's docker-network address but
// is overridable via AI_SERVICE_URL for other environments, and is set
// directly (bypassing the env var) in tests to point at an httptest.Server.
type EmailCategorizationService struct {
	httpClient *http.Client
	baseURL    string
}

func NewEmailCategorizationService() *EmailCategorizationService {
	baseURL := os.Getenv("AI_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://ai:8000"
	}
	return &EmailCategorizationService{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    baseURL,
	}
}

type categorizeEmailRequest struct {
	Subject     string `json:"subject"`
	Snippet     string `json:"snippet"`
	FromAddress string `json:"from_address"`
	FromName    string `json:"from_name"`
}

type categorizeEmailResponse struct {
	Category string `json:"category"`
}

func (s *EmailCategorizationService) Categorize(ctx context.Context, subject, snippet, fromAddress, fromName string) (*string, error) {
	payload, err := json.Marshal(categorizeEmailRequest{
		Subject:     subject,
		Snippet:     snippet,
		FromAddress: fromAddress,
		FromName:    fromName,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding categorize-email request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/categorize-email", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("building categorize-email request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling ai service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading categorize-email response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai service returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed categorizeEmailResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parsing categorize-email response: %w", err)
	}

	if !models.ValidEmailCategories[parsed.Category] {
		return nil, fmt.Errorf("ai service returned unknown category %q", parsed.Category)
	}

	return &parsed.Category, nil
}
