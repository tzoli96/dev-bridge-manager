// backend/internal/services/job_application_draft.go
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

// JobApplicationDrafter is the seam job_search_handler.go calls through, so
// tests can inject a fake instead of hitting the real AI service.
type JobApplicationDrafter interface {
	DraftApplication(ctx context.Context, cvText, skills, jobTitle, company, jobDescription, instruction string) (string, error)
}

var _ JobApplicationDrafter = (*JobApplicationDraftService)(nil)

// JobApplicationDraftService calls the AI service's /job-application-draft
// endpoint. Follows the same AI_SERVICE_URL/30s-timeout convention as
// DraftReplyService.
type JobApplicationDraftService struct {
	httpClient *http.Client
	baseURL    string
}

func NewJobApplicationDraftService() *JobApplicationDraftService {
	baseURL := os.Getenv("AI_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://ai:8000"
	}
	return &JobApplicationDraftService{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
	}
}

type jobApplicationDraftRequest struct {
	CVText         string `json:"cv_text"`
	Skills         string `json:"skills"`
	JobTitle       string `json:"job_title"`
	Company        string `json:"company"`
	JobDescription string `json:"job_description"`
	Instruction    string `json:"instruction"`
}

type jobApplicationDraftResponse struct {
	Draft string `json:"draft"`
}

func (s *JobApplicationDraftService) DraftApplication(ctx context.Context, cvText, skills, jobTitle, company, jobDescription, instruction string) (string, error) {
	payload, err := json.Marshal(jobApplicationDraftRequest{
		CVText:         cvText,
		Skills:         skills,
		JobTitle:       jobTitle,
		Company:        company,
		JobDescription: jobDescription,
		Instruction:    instruction,
	})
	if err != nil {
		return "", fmt.Errorf("encoding job-application-draft request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/job-application-draft", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("building job-application-draft request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling ai service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading job-application-draft response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ai service returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed jobApplicationDraftResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("parsing job-application-draft response: %w", err)
	}

	return parsed.Draft, nil
}
