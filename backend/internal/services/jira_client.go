// backend/internal/services/jira_client.go
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"dev-bridge-manager/internal/models"
)

// JiraClient is the seam RunJiraSync and the connect handler call through,
// so tests can inject a fake instead of hitting the real Jira Cloud API.
type JiraClient interface {
	TestConnection(ctx context.Context, baseURL, email, apiToken string) error
	SearchAssignedIssues(ctx context.Context, integration models.JiraBoardIntegration) ([]JiraIssue, error)
}

var _ JiraClient = (*RealJiraClient)(nil)

// JiraIssue is the subset of a Jira Cloud issue this integration mirrors.
type JiraIssue struct {
	Key         string
	Summary     string
	Description string
	StatusName  string
}

// RealJiraClient calls the real Jira Cloud REST API v3. Unlike other HTTP
// clients in this codebase, it takes no fixed base URL at construction time -
// each board supplies its own Jira base URL/credentials per call.
type RealJiraClient struct {
	httpClient *http.Client
}

func NewRealJiraClient() *RealJiraClient {
	return &RealJiraClient{httpClient: &http.Client{Timeout: 30 * time.Second}}
}

func (c *RealJiraClient) TestConnection(ctx context.Context, baseURL, email, apiToken string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/rest/api/3/myself", nil)
	if err != nil {
		return fmt.Errorf("building myself request: %w", err)
	}
	req.SetBasicAuth(email, apiToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling jira: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

type jiraSearchResponse struct {
	Issues        []jiraIssueJSON `json:"issues"`
	NextPageToken string          `json:"nextPageToken"`
	IsLast        bool            `json:"isLast"`
}

type jiraIssueJSON struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"`
		Status      struct {
			Name string `json:"name"`
		} `json:"status"`
	} `json:"fields"`
}

func (c *RealJiraClient) SearchAssignedIssues(ctx context.Context, integration models.JiraBoardIntegration) ([]JiraIssue, error) {
	jql := fmt.Sprintf("project = %s AND assignee = currentUser() AND statusCategory != Done", integration.ProjectKey)

	var issues []JiraIssue
	pageToken := ""
	for {
		endpoint := fmt.Sprintf(
			"%s/rest/api/3/search/jql?jql=%s&fields=summary,description,status&maxResults=100",
			strings.TrimRight(integration.BaseURL, "/"),
			url.QueryEscape(jql),
		)
		if pageToken != "" {
			endpoint += "&nextPageToken=" + url.QueryEscape(pageToken)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("building search request: %w", err)
		}
		req.SetBasicAuth(integration.Email, integration.APIToken)
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("calling jira search: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading jira search response: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("jira search returned %d: %s", resp.StatusCode, string(body))
		}

		var parsed jiraSearchResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return nil, fmt.Errorf("parsing jira search response: %w", err)
		}

		for _, raw := range parsed.Issues {
			issues = append(issues, JiraIssue{
				Key:         raw.Key,
				Summary:     raw.Fields.Summary,
				Description: adfToPlainText(raw.Fields.Description),
				StatusName:  raw.Fields.Status.Name,
			})
		}

		if parsed.IsLast || parsed.NextPageToken == "" {
			break
		}
		pageToken = parsed.NextPageToken
	}

	return issues, nil
}

// adfToPlainText extracts plain text from a Jira issue's description field,
// which the API returns as either an Atlassian Document Format (ADF) object,
// a legacy plain string, or omitted/null.
func adfToPlainText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}

	var doc struct {
		Content []adfNode `json:"content"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}

	var b strings.Builder
	for _, node := range doc.Content {
		writeADFNode(&b, node)
	}
	return strings.TrimSpace(b.String())
}

type adfNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text"`
	Content []adfNode `json:"content"`
}

func writeADFNode(b *strings.Builder, node adfNode) {
	if node.Type == "text" {
		b.WriteString(node.Text)
	}
	for _, child := range node.Content {
		writeADFNode(b, child)
	}
	if node.Type == "paragraph" {
		b.WriteString("\n")
	}
}
