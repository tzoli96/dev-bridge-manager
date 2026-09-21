// backend/internal/services/draft_reply.go
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

// DraftReplier is the seam email_handler.go calls through, so tests can
// inject a fake instead of hitting the real AI service.
type DraftReplier interface {
	DraftReply(ctx context.Context, emailContent, profileContext string) (string, error)
}

var _ DraftReplier = (*DraftReplyService)(nil)

// DraftReplyService calls the AI service's /draft-reply endpoint. baseURL
// follows the same AI_SERVICE_URL convention as EmailCategorizationService.
// Timeout is longer than EmailCategorizationService's 10s: generating a full
// reply draft is a heavier generation task than a one-word categorization.
type DraftReplyService struct {
	httpClient *http.Client
	baseURL    string
}

func NewDraftReplyService() *DraftReplyService {
	baseURL := os.Getenv("AI_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://ai:8000"
	}
	return &DraftReplyService{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
	}
}

type draftReplyRequest struct {
	EmailContent   string `json:"email_content"`
	ProfileContext string `json:"profile_context"`
}

type draftReplyResponse struct {
	Draft string `json:"draft"`
}

func (s *DraftReplyService) DraftReply(ctx context.Context, emailContent, profileContext string) (string, error) {
	payload, err := json.Marshal(draftReplyRequest{EmailContent: emailContent, ProfileContext: profileContext})
	if err != nil {
		return "", fmt.Errorf("encoding draft-reply request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/draft-reply", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("building draft-reply request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling ai service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading draft-reply response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ai service returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed draftReplyResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parsing draft-reply response: %w", err)
	}

	return parsed.Draft, nil
}
