// backend/internal/services/email_client_matching.go
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

// ClientCandidate is the subset of a client's fields the AI service needs
// to decide whether an email is from that client.
type ClientCandidate struct {
	ID    uint   `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// EmailClientMatcher is the seam gmail_sync.go calls through, so tests can
// inject a fake instead of hitting the real AI service.
type EmailClientMatcher interface {
	MatchClient(ctx context.Context, subject, snippet, fromAddress, fromName string, candidates []ClientCandidate) (*uint, error)
}

var _ EmailClientMatcher = (*EmailClientMatchingService)(nil)

// EmailClientMatchingService calls the AI service's /match-email-client
// endpoint. Same baseURL resolution as EmailCategorizationService.
type EmailClientMatchingService struct {
	httpClient *http.Client
	baseURL    string
}

func NewEmailClientMatchingService() *EmailClientMatchingService {
	baseURL := os.Getenv("AI_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://ai:8000"
	}
	return &EmailClientMatchingService{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		baseURL:    baseURL,
	}
}

type matchEmailClientRequest struct {
	Subject     string            `json:"subject"`
	Snippet     string            `json:"snippet"`
	FromAddress string            `json:"from_address"`
	FromName    string            `json:"from_name"`
	Clients     []ClientCandidate `json:"clients"`
}

type matchEmailClientResponse struct {
	ClientID *uint `json:"client_id"`
}

func (s *EmailClientMatchingService) MatchClient(ctx context.Context, subject, snippet, fromAddress, fromName string, candidates []ClientCandidate) (*uint, error) {
	payload, err := json.Marshal(matchEmailClientRequest{
		Subject:     subject,
		Snippet:     snippet,
		FromAddress: fromAddress,
		FromName:    fromName,
		Clients:     candidates,
	})
	if err != nil {
		return nil, fmt.Errorf("encoding match-email-client request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/match-email-client", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("building match-email-client request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling ai service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading match-email-client response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai service returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed matchEmailClientResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parsing match-email-client response: %w", err)
	}

	return parsed.ClientID, nil
}
