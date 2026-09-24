// backend/internal/services/job_match.go
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

// JobMatcher is the seam job_search_handler.go and RunScrape call through,
// so tests can inject a fake instead of hitting the real AI service.
type JobMatcher interface {
	MatchJob(ctx context.Context, cvText, skills, preferences, jobTitle, company, location, jobDescription string) (score int, reasoning string, err error)
}

var _ JobMatcher = (*JobMatchService)(nil)

// JobMatchService calls the AI service's /job-match endpoint. Follows the
// same AI_SERVICE_URL/30s-timeout convention as DraftReplyService.
type JobMatchService struct {
	httpClient *http.Client
	baseURL    string
}

func NewJobMatchService() *JobMatchService {
	baseURL := os.Getenv("AI_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://ai:8000"
	}
	return &JobMatchService{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
	}
}

type jobMatchRequest struct {
	CVText         string `json:"cv_text"`
	Skills         string `json:"skills"`
	Preferences    string `json:"preferences"`
	JobTitle       string `json:"job_title"`
	Company        string `json:"company"`
	Location       string `json:"location"`
	JobDescription string `json:"job_description"`
}

type jobMatchResponse struct {
	Score     int    `json:"score"`
	Reasoning string `json:"reasoning"`
}

func (s *JobMatchService) MatchJob(ctx context.Context, cvText, skills, preferences, jobTitle, company, location, jobDescription string) (int, string, error) {
	payload, err := json.Marshal(jobMatchRequest{
		CVText:         cvText,
		Skills:         skills,
		Preferences:    preferences,
		JobTitle:       jobTitle,
		Company:        company,
		Location:       location,
		JobDescription: jobDescription,
	})
	if err != nil {
		return 0, "", fmt.Errorf("encoding job-match request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/job-match", bytes.NewReader(payload))
	if err != nil {
		return 0, "", fmt.Errorf("building job-match request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("calling ai service: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("reading job-match response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, "", fmt.Errorf("ai service returned %d: %s", resp.StatusCode, string(body))
	}

	var parsed jobMatchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, "", fmt.Errorf("parsing job-match response: %w", err)
	}

	return parsed.Score, parsed.Reasoning, nil
}
