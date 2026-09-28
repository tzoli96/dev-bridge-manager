# Jira Integration for Kanban Boards Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a super_admin optionally connect a kanban board to a Jira Cloud project so the board additionally shows a read-only, one-directional mirror of that account's assigned, not-yet-done Jira issues, alongside the board's existing local tasks/columns, without ever writing anything back to Jira.

**Architecture:** A new `jira_board_integrations` table (one row per connected board) stores connection credentials. A background sync job (`services.RunJiraSync`, ticking every 15 minutes, same `time.Ticker` pattern as `gmail_sync.go`) polls each connected board's Jira project via a small `JiraClient` interface, upserting mirrored `Task`/`KanbanColumn` rows tagged `source = "jira"` / `jira_status_name`. Existing task/column handlers gain a read-only guard (pure predicate functions `isJiraSourced`/`isJiraColumn`) that reject local mutation of anything Jira-sourced. A new `JiraIntegrationHandler` exposes connect/status/disconnect endpoints, gated `super_admin`-only. The frontend gets a connect/status/disconnect panel in the column settings modal, a Jira badge on mirrored cards, and read-only enforcement in the task detail view.

**Tech Stack:** Go/Fiber/GORM/PostgreSQL backend (existing), Next.js/React/TypeScript frontend (existing), Jira Cloud REST API v3 (new external dependency, already approved in the design spec).

**Spec:** `docs/superpowers/specs/2026-09-28-jira-integration-design.md`

## Global Constraints

- All backend commands run in `docker exec devbridge_backend ...`; all frontend commands run in `docker exec devbridge_frontend ...`. Never run Go/Node tooling on the host.
- Never touch production/shared data, cloud resources, or external providers without explicit per-action approval. The real-Jira-account connection test in Task 13 requires the user's fresh explicit approval at execution time — it must never proceed automatically.
- Extend established codebase flows first: this plan reuses the existing `time.Ticker` scheduler pattern (`gmail_sync.go`), the existing HTTP-client-with-interface pattern (`client_status_ai.go`), the existing plain-SQL migration style, and the existing per-route `middleware.RequireRole("super_admin")` pattern. No new queue, worker, cron dependency, or abstraction is introduced.
- **Testing-convention deviation (must be visible, not silently applied):** the spec's Testing plan section calls for DB-backed sync-job upsert/delete tests and handler-level 403 tests. An exhaustive `find . -name "*_test.go"` search of this codebase confirms **zero** DB-touching tests exist anywhere — every existing test (handler and service) tests only pure functions, and DB-touching orchestration (e.g. `gmail_sync.go`'s `RunGmailSync`) has no test file at all. This plan honors that established convention instead of introducing new DB-test infrastructure: the sync job's decision logic is factored into pure, DB-free helpers (`issueKeysToRemove`, `nextColumnPosition`) that get real unit tests, and the read-only guards are factored into pure predicates (`isJiraSourced`, `isJiraColumn`) that get real unit tests — while the DB-touching orchestration (`RunJiraSync`/`syncBoardIntegration`) and the HTTP handlers themselves remain untested, verified only by manual container `curl`/build checks, exactly matching `gmail_sync.go` and every existing admin CRUD handler.
- **Route-path adaptation (must be visible, not silently applied):** the spec's literal endpoint path is `POST/GET/DELETE /api/v1/admin/boards/:boardId/jira-integration`. This codebase has no standalone `/admin/boards/:boardId` route anywhere — every board route nests under `/api/v1/projects/:id/boards/:boardId/...` (see `kanban_routes.go`). This plan adapts the path to `/api/v1/projects/:id/boards/:boardId/jira-integration`, added directly into the existing `SetupKanbanRoutes` function, keeping `middleware.RequireRole("super_admin")` applied per-route.
- Keep diffs reviewable: no unrelated refactoring, no formatter-only churn.
- Commit after each task's steps pass; never commit failing code.

---

### Task 1: Database schema + Go models

**Files:**
- Create: `backend/migrations/000039_add_jira_integration.up.sql`
- Create: `backend/migrations/000039_add_jira_integration.down.sql`
- Create: `backend/internal/models/jira_board_integration.go`
- Modify: `backend/internal/models/kanban.go:12-22` (KanbanColumn struct)
- Modify: `backend/internal/models/kanban.go:49-69` (Task struct)
- Modify: `backend/internal/models/kanban.go:158-190` (TaskDTO struct)
- Modify: `backend/internal/models/kanban.go:192-202` (KanbanColumnDTO struct)

**Interfaces:**
- Produces: `models.JiraBoardIntegration` (GORM model, table `jira_board_integrations`), `models.JiraIntegrationStatusDTO`, `models.ConnectJiraIntegrationRequest`, `models.Task.Source string` / `models.Task.JiraIssueKey *string` / `models.Task.JiraSyncedAt *time.Time`, `models.KanbanColumn.JiraStatusName *string`, `models.TaskDTO.Source string`, `models.KanbanColumnDTO.JiraStatusName *string`.

- [ ] **Step 1: Write the migration**

Create `backend/migrations/000039_add_jira_integration.up.sql`:

```sql
CREATE TABLE jira_board_integrations (
    id SERIAL PRIMARY KEY,
    board_id INTEGER NOT NULL UNIQUE REFERENCES boards(id) ON DELETE CASCADE,
    base_url VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    api_token VARCHAR(255) NOT NULL,
    project_key VARCHAR(50) NOT NULL,
    connected_by INTEGER NOT NULL REFERENCES users(id),
    connected_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_sync_at TIMESTAMP,
    last_sync_error TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_jira_board_integrations_board ON jira_board_integrations (board_id);

ALTER TABLE tasks ADD COLUMN source VARCHAR(20) NOT NULL DEFAULT 'local';
ALTER TABLE tasks ADD COLUMN jira_issue_key VARCHAR(50);
ALTER TABLE tasks ADD COLUMN jira_synced_at TIMESTAMP;

ALTER TABLE kanban_columns ADD COLUMN jira_status_name VARCHAR(100);
```

Create `backend/migrations/000039_add_jira_integration.down.sql`:

```sql
ALTER TABLE kanban_columns DROP COLUMN jira_status_name;

ALTER TABLE tasks DROP COLUMN jira_synced_at;
ALTER TABLE tasks DROP COLUMN jira_issue_key;
ALTER TABLE tasks DROP COLUMN source;

DROP TABLE IF EXISTS jira_board_integrations;
```

- [ ] **Step 2: Create the JiraBoardIntegration model**

Create `backend/internal/models/jira_board_integration.go`:

```go
// backend/internal/models/jira_board_integration.go
package models

import "time"

// JiraBoardIntegration links a single kanban board to a Jira Cloud
// project/account. At most one row exists per board (BoardID is unique) -
// connecting a board that's already connected reconnects it in place rather
// than creating a second row.
type JiraBoardIntegration struct {
	ID            uint       `gorm:"primaryKey"`
	BoardID       uint       `gorm:"column:board_id;not null;unique"`
	BaseURL       string     `gorm:"column:base_url;size:255"`
	Email         string     `gorm:"column:email;size:255"`
	APIToken      string     `json:"-" gorm:"column:api_token;size:255"`
	ProjectKey    string     `gorm:"column:project_key;size:50"`
	ConnectedBy   uint       `gorm:"column:connected_by"`
	ConnectedAt   time.Time  `gorm:"column:connected_at"`
	LastSyncAt    *time.Time `gorm:"column:last_sync_at"`
	LastSyncError string     `gorm:"column:last_sync_error"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (JiraBoardIntegration) TableName() string { return "jira_board_integrations" }

// JiraIntegrationStatusDTO is the read-only shape returned by GET .../jira-integration.
type JiraIntegrationStatusDTO struct {
	Connected     bool   `json:"connected"`
	BaseURL       string `json:"baseUrl,omitempty"`
	Email         string `json:"email,omitempty"`
	ProjectKey    string `json:"projectKey,omitempty"`
	LastSyncAt    string `json:"lastSyncAt,omitempty"`
	LastSyncError string `json:"lastSyncError,omitempty"`
}

// ConnectJiraIntegrationRequest is the body of POST .../jira-integration.
type ConnectJiraIntegrationRequest struct {
	BaseURL    string `json:"baseUrl" validate:"required"`
	Email      string `json:"email" validate:"required"`
	APIToken   string `json:"apiToken" validate:"required"`
	ProjectKey string `json:"projectKey" validate:"required"`
}
```

- [ ] **Step 3: Add additive fields to kanban.go**

In `backend/internal/models/kanban.go`, modify the `KanbanColumn` struct (lines 12-22):

```go
type KanbanColumn struct {
	ID             uint `gorm:"primaryKey"`
	BoardID        uint `gorm:"not null;index" json:"boardId"`
	Title          string
	Color          string
	Position       int
	MaxTasks       *int
	IsDone         bool
	JiraStatusName *string `gorm:"column:jira_status_name"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
```

Modify the `Task` struct (lines 49-69):

```go
type Task struct {
	ID              uint `gorm:"primaryKey"`
	ProjectID       uint `gorm:"not null;index"`
	Title           string
	Description     string
	HTMLDescription string
	Priority        string
	Status          string
	AssigneeID      *uint
	ParentTaskID    *uint `gorm:"index"`
	EstimatedHours  float64
	Tags            string     `gorm:"type:jsonb"`
	DueDate         *time.Time `gorm:"type:date"`
	IsArchived      bool
	Source          string     `gorm:"column:source;default:local"`
	JiraIssueKey    *string    `gorm:"column:jira_issue_key"`
	JiraSyncedAt    *time.Time `gorm:"column:jira_synced_at"`
	CreatedBy       uint
	UpdatedBy       uint
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
```

Modify the `TaskDTO` struct (lines 158-190), adding `Source` right after `Status`:

```go
type TaskDTO struct {
	ID              string           `json:"id"`
	Title           string           `json:"title"`
	Description     string           `json:"description"`
	HTMLDescription string           `json:"htmlDescription,omitempty"`
	Priority        string           `json:"priority"`
	Status          string           `json:"status"`
	Source          string           `json:"source"`
	ColumnID        string           `json:"columnId"`
	IsDoneColumn    bool             `json:"isDoneColumn,omitempty"`
	BoardID         string           `json:"boardId,omitempty"`
	ProjectID       string           `json:"projectId"`
	AssigneeID      string           `json:"assigneeId,omitempty"`
	Assignee        *TaskAssigneeDTO `json:"assignee,omitempty"`
	EstimatedHours  float64          `json:"estimatedHours"`
	LoggedHours     float64          `json:"loggedHours"`
	// HasUninvoicedHours is true when this task has logged hours on an
	// hourly-priced project that no created invoice's period covers yet.
	// Always false for fixed-price/unpriced projects, where per-hour billing
	// status doesn't apply.
	HasUninvoicedHours bool                `json:"hasUninvoicedHours"`
	Tags               []TaskTagDTO        `json:"tags"`
	TimeEntries        []TimeEntryDTO      `json:"timeEntries"`
	Comments           []TaskCommentDTO    `json:"comments"`
	Attachments        []AttachmentDTO     `json:"attachments"`
	Position           int                 `json:"position"`
	DueDate            string              `json:"dueDate,omitempty"`
	SubtaskProgress    *SubtaskProgressDTO `json:"subtaskProgress,omitempty"`
	ParentTask         *TaskParentRefDTO   `json:"parentTask,omitempty"`
	CreatedAt          time.Time           `json:"createdAt"`
	UpdatedAt          time.Time           `json:"updatedAt"`
	CreatedBy          string              `json:"createdBy"`
	UpdatedBy          string              `json:"updatedBy"`
}
```

Modify the `KanbanColumnDTO` struct (lines 192-202), adding `JiraStatusName` right after `IsDone`:

```go
type KanbanColumnDTO struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Color          string    `json:"color"`
	Position       int       `json:"position"`
	MaxTasks       *int      `json:"maxTasks,omitempty"`
	IsDone         bool      `json:"isDone"`
	JiraStatusName *string   `json:"jiraStatusName,omitempty"`
	Tasks          []TaskDTO `json:"tasks"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}
```

- [ ] **Step 4: Run the migration and verify the schema**

Run: `docker exec devbridge_backend go build ./...`
Expected: builds cleanly (migrations run automatically on backend startup via `database.RunMigrations`, called from `main.go`).

Run: `docker restart devbridge_backend` then `docker exec devbridge_postgres psql -U postgres -d dev_bridge_manager -c '\d jira_board_integrations'`
Expected: shows the new table with all 12 columns.

Run: `docker exec devbridge_postgres psql -U postgres -d dev_bridge_manager -c '\d tasks' | grep -E "source|jira_issue_key|jira_synced_at"`
Expected: shows the 3 new columns.

Run: `docker exec devbridge_postgres psql -U postgres -d dev_bridge_manager -c '\d kanban_columns' | grep jira_status_name`
Expected: shows the new column.

- [ ] **Step 5: Commit**

```bash
git add backend/migrations/000039_add_jira_integration.up.sql backend/migrations/000039_add_jira_integration.down.sql backend/internal/models/jira_board_integration.go backend/internal/models/kanban.go
git commit -m "feat(jira): add jira_board_integrations table and mirrored-task/column columns"
```

---

### Task 2: Jira API client

**Files:**
- Create: `backend/internal/services/jira_client.go`
- Create: `backend/internal/services/jira_client_test.go`

**Interfaces:**
- Consumes: `models.JiraBoardIntegration` (Task 1).
- Produces: `services.JiraClient` interface (`TestConnection(ctx, baseURL, email, apiToken string) error`, `SearchAssignedIssues(ctx, integration models.JiraBoardIntegration) ([]JiraIssue, error)`), `services.RealJiraClient`, `services.NewRealJiraClient() *RealJiraClient`, `services.JiraIssue{Key, Summary, Description, StatusName string}`, `services.adfToPlainText(raw json.RawMessage) string`.

- [ ] **Step 1: Write the failing tests**

Create `backend/internal/services/jira_client_test.go`:

```go
// backend/internal/services/jira_client_test.go
package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestAdfToPlainText_LegacyString(t *testing.T) {
	raw := json.RawMessage(`"Plain legacy description"`)
	got := adfToPlainText(raw)
	if got != "Plain legacy description" {
		t.Errorf("got %q, want %q", got, "Plain legacy description")
	}
}

func TestAdfToPlainText_ADFDocument(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "doc",
		"content": [
			{"type": "paragraph", "content": [{"type": "text", "text": "First line"}]},
			{"type": "paragraph", "content": [{"type": "text", "text": "Second line"}]}
		]
	}`)
	got := adfToPlainText(raw)
	want := "First line\nSecond line"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAdfToPlainText_NilOrEmpty(t *testing.T) {
	if got := adfToPlainText(nil); got != "" {
		t.Errorf("nil: got %q, want empty", got)
	}
	if got := adfToPlainText(json.RawMessage("null")); got != "" {
		t.Errorf("null: got %q, want empty", got)
	}
	if got := adfToPlainText(json.RawMessage("")); got != "" {
		t.Errorf("empty: got %q, want empty", got)
	}
}

func TestRealJiraClient_TestConnection_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/myself" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		user, pass, ok := r.BasicAuth()
		if !ok || user != "me@example.com" || pass != "token123" {
			t.Errorf("unexpected basic auth: %s/%s (ok=%v)", user, pass, ok)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accountId":"abc"}`))
	}))
	defer server.Close()

	client := NewRealJiraClient()
	err := client.TestConnection(context.Background(), server.URL, "me@example.com", "token123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRealJiraClient_TestConnection_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"errorMessages":["Unauthorized"]}`))
	}))
	defer server.Close()

	client := NewRealJiraClient()
	err := client.TestConnection(context.Background(), server.URL, "me@example.com", "wrong-token")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRealJiraClient_SearchAssignedIssues_SinglePage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search/jql" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"issues": [
				{"key": "PROJ-1", "fields": {"summary": "First issue", "description": "Plain text", "status": {"name": "To Do"}}}
			],
			"isLast": true
		}`))
	}))
	defer server.Close()

	client := NewRealJiraClient()
	integration := models.JiraBoardIntegration{BaseURL: server.URL, Email: "me@example.com", APIToken: "token123", ProjectKey: "PROJ"}
	issues, err := client.SearchAssignedIssues(context.Background(), integration)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Key != "PROJ-1" || issues[0].Summary != "First issue" || issues[0].Description != "Plain text" || issues[0].StatusName != "To Do" {
		t.Errorf("unexpected issue: %+v", issues[0])
	}
}

func TestRealJiraClient_SearchAssignedIssues_Pagination(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		if r.URL.Query().Get("nextPageToken") == "" {
			w.Write([]byte(`{
				"issues": [{"key": "PROJ-1", "fields": {"summary": "First", "status": {"name": "To Do"}}}],
				"nextPageToken": "page2",
				"isLast": false
			}`))
		} else {
			w.Write([]byte(`{
				"issues": [{"key": "PROJ-2", "fields": {"summary": "Second", "status": {"name": "In Progress"}}}],
				"isLast": true
			}`))
		}
	}))
	defer server.Close()

	client := NewRealJiraClient()
	integration := models.JiraBoardIntegration{BaseURL: server.URL, Email: "me@example.com", APIToken: "token123", ProjectKey: "PROJ"}
	issues, err := client.SearchAssignedIssues(context.Background(), integration)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 requests, got %d", callCount)
	}
	if len(issues) != 2 || issues[0].Key != "PROJ-1" || issues[1].Key != "PROJ-2" {
		t.Errorf("unexpected issues: %+v", issues)
	}
}

func TestRealJiraClient_SearchAssignedIssues_ParsesADFDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"issues": [{
				"key": "PROJ-3",
				"fields": {
					"summary": "ADF issue",
					"description": {"type": "doc", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Rich description"}]}]},
					"status": {"name": "To Do"}
				}
			}],
			"isLast": true
		}`))
	}))
	defer server.Close()

	client := NewRealJiraClient()
	integration := models.JiraBoardIntegration{BaseURL: server.URL, Email: "me@example.com", APIToken: "token123", ProjectKey: "PROJ"}
	issues, err := client.SearchAssignedIssues(context.Background(), integration)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(issues) != 1 || issues[0].Description != "Rich description" {
		t.Errorf("unexpected issues: %+v", issues)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail (package doesn't compile yet)**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestRealJiraClient -v`
Expected: FAIL with `undefined: NewRealJiraClient` (or similar compile error).

- [ ] **Step 3: Implement jira_client.go**

Create `backend/internal/services/jira_client.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `docker exec devbridge_backend go test ./internal/services/... -run "TestAdfToPlainText|TestRealJiraClient" -v`
Expected: PASS (all 8 test functions).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/jira_client.go backend/internal/services/jira_client_test.go
git commit -m "feat(jira): add Jira Cloud REST API v3 client"
```

---

### Task 3: Jira sync orchestrator + scheduler

**Files:**
- Create: `backend/internal/services/jira_sync.go`
- Create: `backend/internal/services/jira_sync_test.go`
- Modify: `backend/cmd/server/main.go:86` (add scheduler wiring after the client-status-email scheduler, before the `// Start server` block)

**Interfaces:**
- Consumes: `services.JiraClient`, `services.JiraIssue` (Task 2); `models.JiraBoardIntegration`, `models.Task`, `models.KanbanColumn`, `models.Board`, `models.TaskPlacement` (Task 1 and existing).
- Produces: `services.RunJiraSync(client JiraClient)`, `services.StartJiraSyncScheduler()`, pure helpers `issueKeysToRemove(mirroredKeys, currentKeys []string) []string` and `nextColumnPosition(existing []models.KanbanColumn) int`.

- [ ] **Step 1: Write the failing tests for the pure helpers**

Create `backend/internal/services/jira_sync_test.go`:

```go
// backend/internal/services/jira_sync_test.go
package services

import (
	"reflect"
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestIssueKeysToRemove(t *testing.T) {
	mirrored := []string{"PROJ-1", "PROJ-2", "PROJ-3"}
	current := []string{"PROJ-2", "PROJ-4"}
	got := issueKeysToRemove(mirrored, current)
	want := []string{"PROJ-1", "PROJ-3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIssueKeysToRemove_NoneToRemove(t *testing.T) {
	got := issueKeysToRemove([]string{"PROJ-1"}, []string{"PROJ-1", "PROJ-2"})
	if len(got) != 0 {
		t.Errorf("expected no keys to remove, got %v", got)
	}
}

func TestNextColumnPosition_Empty(t *testing.T) {
	got := nextColumnPosition(nil)
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestNextColumnPosition_NonEmpty(t *testing.T) {
	cols := []models.KanbanColumn{{Position: 0}, {Position: 2}, {Position: 1}}
	got := nextColumnPosition(cols)
	if got != 3 {
		t.Errorf("got %d, want 3", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `docker exec devbridge_backend go test ./internal/services/... -run "TestIssueKeysToRemove|TestNextColumnPosition" -v`
Expected: FAIL with `undefined: issueKeysToRemove` (compile error).

- [ ] **Step 3: Implement the pure helpers and the full sync orchestrator**

Create `backend/internal/services/jira_sync.go`:

```go
// backend/internal/services/jira_sync.go
package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"gorm.io/gorm"
)

const jiraSyncInterval = 15 * time.Minute

// StartJiraSyncScheduler mirrors gmail_sync.go's StartGmailSyncScheduler
// pattern: run once immediately, then on a fixed ticker.
func StartJiraSyncScheduler() {
	client := NewRealJiraClient()
	RunJiraSync(client)

	ticker := time.NewTicker(jiraSyncInterval)
	for range ticker.C {
		RunJiraSync(client)
	}
}

// RunJiraSync syncs every connected board in turn, tolerating individual
// board failures the same way RunGmailSync tolerates individual account
// failures - one board's error doesn't stop the others from syncing.
func RunJiraSync(client JiraClient) {
	var integrations []models.JiraBoardIntegration
	if err := database.GetDB().Find(&integrations).Error; err != nil {
		log.Printf("jira sync: failed to load integrations: %v", err)
		return
	}
	for i := range integrations {
		if err := syncBoardIntegration(context.Background(), client, &integrations[i]); err != nil {
			log.Printf("jira sync: board %d failed: %v", integrations[i].BoardID, err)
		}
	}
}

func syncBoardIntegration(ctx context.Context, client JiraClient, integration *models.JiraBoardIntegration) error {
	db := database.GetDB()

	var board models.Board
	if err := db.First(&board, integration.BoardID).Error; err != nil {
		return fmt.Errorf("loading board: %w", err)
	}

	issues, err := client.SearchAssignedIssues(ctx, *integration)
	if err != nil {
		db.Model(integration).Updates(map[string]interface{}{"last_sync_at": time.Now(), "last_sync_error": err.Error()})
		return err
	}

	var columns []models.KanbanColumn
	db.Where("board_id = ?", integration.BoardID).Find(&columns)
	columnByStatus := make(map[string]models.KanbanColumn, len(columns))
	for _, c := range columns {
		if c.JiraStatusName != nil {
			columnByStatus[*c.JiraStatusName] = c
		}
	}

	currentKeys := make([]string, 0, len(issues))
	for _, issue := range issues {
		currentKeys = append(currentKeys, issue.Key)

		column, ok := columnByStatus[issue.StatusName]
		if !ok {
			statusName := issue.StatusName
			column = models.KanbanColumn{
				BoardID:        integration.BoardID,
				Title:          statusName,
				Color:          "bg-blue-500",
				Position:       nextColumnPosition(columns),
				JiraStatusName: &statusName,
			}
			if err := db.Create(&column).Error; err != nil {
				log.Printf("jira sync: board %d failed to create column for status %q: %v", integration.BoardID, statusName, err)
				continue
			}
			columns = append(columns, column)
			columnByStatus[statusName] = column
		}

		var task models.Task
		lookupErr := db.Where("project_id = ? AND jira_issue_key = ?", board.ProjectID, issue.Key).First(&task).Error
		now := time.Now()

		if lookupErr == gorm.ErrRecordNotFound {
			jiraKey := issue.Key
			task = models.Task{
				ProjectID:    board.ProjectID,
				Title:        issue.Summary,
				Description:  issue.Description,
				Priority:     "medium",
				Status:       "todo",
				Source:       "jira",
				JiraIssueKey: &jiraKey,
				JiraSyncedAt: &now,
				CreatedBy:    integration.ConnectedBy,
				UpdatedBy:    integration.ConnectedBy,
			}
			if err := db.Create(&task).Error; err != nil {
				log.Printf("jira sync: board %d failed to create task for issue %s: %v", integration.BoardID, issue.Key, err)
				continue
			}

			var maxPosition struct{ Max int }
			db.Model(&models.TaskPlacement{}).
				Select("COALESCE(MAX(position), -1) as max").
				Where("column_id = ?", column.ID).
				Scan(&maxPosition)
			placement := models.TaskPlacement{TaskID: task.ID, BoardID: integration.BoardID, ColumnID: column.ID, Position: maxPosition.Max + 1}
			if err := db.Create(&placement).Error; err != nil {
				log.Printf("jira sync: board %d failed to place task for issue %s: %v", integration.BoardID, issue.Key, err)
			}
		} else if lookupErr != nil {
			log.Printf("jira sync: board %d failed to look up task for issue %s: %v", integration.BoardID, issue.Key, lookupErr)
			continue
		} else {
			task.Title = issue.Summary
			task.Description = issue.Description
			task.JiraSyncedAt = &now
			db.Save(&task)

			var placement models.TaskPlacement
			if err := db.Where("task_id = ? AND board_id = ?", task.ID, integration.BoardID).First(&placement).Error; err == nil {
				if placement.ColumnID != column.ID {
					placement.ColumnID = column.ID
					db.Save(&placement)
				}
			}
		}
	}

	var mirroredKeys []string
	db.Model(&models.Task{}).
		Where("project_id = ? AND source = ? AND jira_issue_key IS NOT NULL", board.ProjectID, "jira").
		Pluck("jira_issue_key", &mirroredKeys)

	for _, key := range issueKeysToRemove(mirroredKeys, currentKeys) {
		db.Where("project_id = ? AND jira_issue_key = ?", board.ProjectID, key).Delete(&models.Task{})
	}

	db.Model(integration).Updates(map[string]interface{}{"last_sync_at": time.Now(), "last_sync_error": ""})
	return nil
}

// issueKeysToRemove returns the previously-mirrored issue keys that no longer
// appear in the current search results, meaning they've left the "assigned,
// not yet done" set (completed or reassigned) and should be un-mirrored.
func issueKeysToRemove(mirroredKeys []string, currentKeys []string) []string {
	current := make(map[string]bool, len(currentKeys))
	for _, k := range currentKeys {
		current[k] = true
	}
	var toRemove []string
	for _, k := range mirroredKeys {
		if !current[k] {
			toRemove = append(toRemove, k)
		}
	}
	return toRemove
}

// nextColumnPosition returns the position a newly auto-created Jira-status
// column should take: one past the highest existing position on the board.
func nextColumnPosition(existing []models.KanbanColumn) int {
	max := -1
	for _, c := range existing {
		if c.Position > max {
			max = c.Position
		}
	}
	return max + 1
}
```

Note: `RunJiraSync`/`syncBoardIntegration` are DB-touching orchestration with no automated test, matching `gmail_sync.go`'s `RunGmailSync` (no test file) - see the Global Constraints testing-convention note above.

- [ ] **Step 4: Run tests to verify they pass**

Run: `docker exec devbridge_backend go test ./internal/services/... -run "TestIssueKeysToRemove|TestNextColumnPosition" -v`
Expected: PASS (4 test functions).

- [ ] **Step 5: Wire the scheduler into main.go**

In `backend/cmd/server/main.go`, after line 86 (`go services.StartClientStatusEmailScheduler()`), before the blank line preceding `// Start server`:

```go
	// AI-drafted weekly client status emails, once at startup then hourly,
	// gated to only do work on Mondays (see services.RunClientStatusEmailDrafts)
	go services.StartClientStatusEmailScheduler()

	// Jira board sync, once at startup then every 15 minutes (see
	// services.RunJiraSync)
	go services.StartJiraSyncScheduler()

	// Start server
```

- [ ] **Step 6: Verify build**

Run: `docker exec devbridge_backend go build ./...`
Expected: builds cleanly.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/services/jira_sync.go backend/internal/services/jira_sync_test.go backend/cmd/server/main.go
git commit -m "feat(jira): add sync orchestrator and 15-minute scheduler"
```

---

### Task 4: Read-only guards for Jira-sourced tasks/columns

**Files:**
- Create: `backend/internal/handlers/jira_guard.go`
- Create: `backend/internal/handlers/jira_guard_test.go`
- Modify: `backend/internal/handlers/task_handler.go:194-251` (UpdateTask)
- Modify: `backend/internal/handlers/task_handler.go:254-265` (DeleteTask)
- Modify: `backend/internal/handlers/task_handler.go:267-342` (MoveTask)
- Modify: `backend/internal/handlers/task_handler.go:344-391` (PlaceTask)
- Modify: `backend/internal/handlers/task_handler.go:393-428` (RemovePlacement)
- Modify: `backend/internal/handlers/kanban_handler.go:158-210` (UpdateColumn)
- Modify: `backend/internal/handlers/kanban_handler.go:213-254` (DeleteColumn)
- Modify: `backend/internal/handlers/kanban_handler.go:256-273` (ReorderColumns)

**Interfaces:**
- Consumes: `models.Task.Source`, `models.KanbanColumn.JiraStatusName` (Task 1).
- Produces: `isJiraSourced(task models.Task) bool`, `isJiraColumn(column models.KanbanColumn) bool`.

- [ ] **Step 1: Write the failing tests**

Create `backend/internal/handlers/jira_guard_test.go`:

```go
// backend/internal/handlers/jira_guard_test.go
package handlers

import (
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestIsJiraSourced(t *testing.T) {
	if isJiraSourced(models.Task{Source: "local"}) {
		t.Error("expected local task to not be Jira-sourced")
	}
	if !isJiraSourced(models.Task{Source: "jira"}) {
		t.Error("expected jira task to be Jira-sourced")
	}
}

func TestIsJiraColumn(t *testing.T) {
	if isJiraColumn(models.KanbanColumn{}) {
		t.Error("expected column with nil JiraStatusName to not be a Jira column")
	}
	statusName := "To Do"
	if !isJiraColumn(models.KanbanColumn{JiraStatusName: &statusName}) {
		t.Error("expected column with JiraStatusName set to be a Jira column")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `docker exec devbridge_backend go test ./internal/handlers/... -run "TestIsJiraSourced|TestIsJiraColumn" -v`
Expected: FAIL with `undefined: isJiraSourced` (compile error).

- [ ] **Step 3: Implement the guard predicates**

Create `backend/internal/handlers/jira_guard.go`:

```go
// backend/internal/handlers/jira_guard.go
package handlers

import "dev-bridge-manager/internal/models"

// isJiraSourced reports whether a task is a read-only mirror of a Jira issue,
// meaning its content, placement, and lifecycle cannot be edited locally.
func isJiraSourced(task models.Task) bool {
	return task.Source == "jira"
}

// isJiraColumn reports whether a column was auto-created by the Jira sync job
// to mirror a Jira status, meaning it cannot be renamed, recolored, deleted,
// or reordered from the UI.
func isJiraColumn(column models.KanbanColumn) bool {
	return column.JiraStatusName != nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `docker exec devbridge_backend go test ./internal/handlers/... -run "TestIsJiraSourced|TestIsJiraColumn" -v`
Expected: PASS (2 test functions).

- [ ] **Step 5: Wire the guard into task_handler.go's UpdateTask**

In `backend/internal/handlers/task_handler.go`, modify `UpdateTask` (currently lines 194-251), inserting the guard right after loading the task:

```go
	var task models.Task
	if err := database.GetDB().First(&task, taskID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task not found"})
	}
	if isJiraSourced(task) {
		return c.Status(403).JSON(fiber.Map{"success": false, "message": "This task is mirrored from Jira and can't be edited locally"})
	}
	original := task
```

- [ ] **Step 6: Wire the guard into task_handler.go's DeleteTask (restructured to load first)**

Replace the current `DeleteTask` (lines 254-265), which deletes directly without loading the task first, with:

```go
// DeleteTask - DELETE /api/v1/projects/:id/tasks/:taskId
func (h *TaskHandler) DeleteTask(c *fiber.Ctx) error {
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	var task models.Task
	if err := database.GetDB().First(&task, taskID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task not found"})
	}
	if isJiraSourced(task) {
		return c.Status(403).JSON(fiber.Map{"success": false, "message": "This task is mirrored from Jira and can't be deleted locally"})
	}

	if err := database.GetDB().Delete(&models.Task{}, taskID).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting task"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Task deleted successfully"})
}
```

- [ ] **Step 7: Wire the guard into task_handler.go's MoveTask and PlaceTask**

In `MoveTask` (currently lines 267-342), right after loading the task (lines 278-281):

```go
	var task models.Task
	if err := database.GetDB().First(&task, taskID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task not found"})
	}
	if isJiraSourced(task) {
		return c.Status(403).JSON(fiber.Map{"success": false, "message": "This task is mirrored from Jira and can't be moved locally"})
	}
```

In `PlaceTask` (currently lines 344-391), right after loading the task (lines 355-358):

```go
	var task models.Task
	if err := database.GetDB().First(&task, taskID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task not found"})
	}
	if isJiraSourced(task) {
		return c.Status(403).JSON(fiber.Map{"success": false, "message": "This task is mirrored from Jira and can't be placed locally"})
	}
```

- [ ] **Step 8: Wire the guard into task_handler.go's RemovePlacement (restructured to load task first)**

Replace the current `RemovePlacement` (lines 393-428), which currently goes straight from `taskID` to the placement lookup without loading the task, with:

```go
// RemovePlacement - DELETE /api/v1/projects/:id/boards/:boardId/tasks/:taskId
// Removes the task's placement from this board only. If this was the
// task's last placement, the task itself is deleted (comments/time
// entries cascade via existing FKs); otherwise only the placement is
// removed and the task survives on its other boards.
func (h *TaskHandler) RemovePlacement(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	db := database.GetDB()

	var task models.Task
	if err := db.First(&task, taskID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task not found"})
	}
	if isJiraSourced(task) {
		return c.Status(403).JSON(fiber.Map{"success": false, "message": "This task is mirrored from Jira and can't be removed locally"})
	}

	var placement models.TaskPlacement
	if err := db.Where("task_id = ? AND board_id = ?", taskID, boardID).First(&placement).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task is not placed on this board"})
	}

	if err := db.Delete(&placement).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error removing task from board"})
	}

	var remaining int64
	db.Model(&models.TaskPlacement{}).Where("task_id = ?", taskID).Count(&remaining)
	if remaining == 0 {
		if err := db.Delete(&models.Task{}, taskID).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting task"})
		}
	}

	return c.JSON(fiber.Map{"success": true, "message": "Task removed from board successfully"})
}
```

- [ ] **Step 9: Wire the guard into kanban_handler.go's UpdateColumn and DeleteColumn**

In `UpdateColumn` (currently lines 158-210), right after loading the column (lines 165-168):

```go
	var column models.KanbanColumn
	if err := database.GetDB().First(&column, columnID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Column not found"})
	}
	if isJiraColumn(column) {
		return c.Status(403).JSON(fiber.Map{"success": false, "message": "This column mirrors a Jira status and can't be edited locally"})
	}
```

In `DeleteColumn` (currently lines 213-254), right after loading the column (lines 219-222):

```go
	var column models.KanbanColumn
	if err := database.GetDB().First(&column, columnID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Column not found"})
	}
	if isJiraColumn(column) {
		return c.Status(403).JSON(fiber.Map{"success": false, "message": "This column mirrors a Jira status and can't be deleted locally"})
	}
```

- [ ] **Step 10: Make ReorderColumns silently skip Jira columns**

Replace the current `ReorderColumns` (lines 256-273), which unconditionally updates every column's position, with a version that loads each column first and skips (rather than errors the whole batch) any Jira-derived column:

```go
// ReorderColumns - PUT /api/v1/projects/:id/boards/:boardId/kanban/columns/reorder
func (h *KanbanHandler) ReorderColumns(c *fiber.Ctx) error {
	var req models.ReorderColumnsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	db := database.GetDB()
	for _, order := range req.Orders {
		columnID, err := models.StrToID(order.ColumnID)
		if err != nil {
			continue
		}
		var column models.KanbanColumn
		if err := db.First(&column, columnID).Error; err != nil {
			continue
		}
		if isJiraColumn(column) {
			continue
		}
		db.Model(&models.KanbanColumn{}).Where("id = ?", columnID).Update("position", order.Position)
	}

	return c.JSON(fiber.Map{"success": true, "message": "Columns reordered successfully"})
}
```

- [ ] **Step 11: Verify build**

Run: `docker exec devbridge_backend go build ./...`
Expected: builds cleanly.

Run: `docker exec devbridge_backend go test ./internal/handlers/... -v`
Expected: PASS (no regressions in existing handler tests, plus the 2 new guard tests).

- [ ] **Step 12: Commit**

```bash
git add backend/internal/handlers/jira_guard.go backend/internal/handlers/jira_guard_test.go backend/internal/handlers/task_handler.go backend/internal/handlers/kanban_handler.go
git commit -m "feat(jira): enforce read-only guards on Jira-sourced tasks and columns"
```

---

### Task 5: Jira integration handler + routes

**Files:**
- Create: `backend/internal/handlers/jira_integration_handler.go`
- Modify: `backend/internal/routes/kanban_routes.go:10-27`

**Interfaces:**
- Consumes: `services.JiraClient`, `services.NewRealJiraClient()`, `services.RunJiraSync` (Task 2, 3); `models.JiraBoardIntegration`, `models.JiraIntegrationStatusDTO`, `models.ConnectJiraIntegrationRequest` (Task 1); `currentUserID(c *fiber.Ctx) uint` (existing, `task_handler.go:19-24`).
- Produces: `handlers.JiraIntegrationHandler` with `Connect`, `Status`, `Disconnect` fiber handlers; routes `POST/GET/DELETE /api/v1/projects/:id/boards/:boardId/jira-integration`.

- [ ] **Step 1: Implement the handler**

Create `backend/internal/handlers/jira_integration_handler.go`:

```go
// backend/internal/handlers/jira_integration_handler.go
package handlers

import (
	"strconv"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type JiraIntegrationHandler struct {
	client services.JiraClient
}

func NewJiraIntegrationHandler() *JiraIntegrationHandler {
	return &JiraIntegrationHandler{client: services.NewRealJiraClient()}
}

func jiraStatusDTO(integration models.JiraBoardIntegration) models.JiraIntegrationStatusDTO {
	lastSyncAt := ""
	if integration.LastSyncAt != nil {
		lastSyncAt = integration.LastSyncAt.Format(time.RFC3339)
	}
	return models.JiraIntegrationStatusDTO{
		Connected:     true,
		BaseURL:       integration.BaseURL,
		Email:         integration.Email,
		ProjectKey:    integration.ProjectKey,
		LastSyncAt:    lastSyncAt,
		LastSyncError: integration.LastSyncError,
	}
}

// Connect - POST /api/v1/projects/:id/boards/:boardId/jira-integration
func (h *JiraIntegrationHandler) Connect(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}

	var req models.ConnectJiraIntegrationRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.BaseURL == "" || req.Email == "" || req.APIToken == "" || req.ProjectKey == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "baseUrl, email, apiToken, and projectKey are all required"})
	}

	if err := h.client.TestConnection(c.Context(), req.BaseURL, req.Email, req.APIToken); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Could not connect to Jira with the given credentials: " + err.Error()})
	}

	integration := models.JiraBoardIntegration{BoardID: uint(boardID)}
	err = database.GetDB().Where("board_id = ?", boardID).Assign(models.JiraBoardIntegration{
		BaseURL:       req.BaseURL,
		Email:         req.Email,
		APIToken:      req.APIToken,
		ProjectKey:    req.ProjectKey,
		ConnectedBy:   currentUserID(c),
		ConnectedAt:   time.Now(),
		LastSyncError: "",
	}).FirstOrCreate(&integration).Error
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error connecting Jira integration"})
	}

	go services.RunJiraSync(services.NewRealJiraClient())

	return c.Status(201).JSON(jiraStatusDTO(integration))
}

// Status - GET /api/v1/projects/:id/boards/:boardId/jira-integration
func (h *JiraIntegrationHandler) Status(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}

	var integration models.JiraBoardIntegration
	if err := database.GetDB().Where("board_id = ?", boardID).First(&integration).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.JSON(models.JiraIntegrationStatusDTO{Connected: false})
		}
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading Jira integration"})
	}

	return c.JSON(jiraStatusDTO(integration))
}

// Disconnect - DELETE /api/v1/projects/:id/boards/:boardId/jira-integration
// Removes the integration row only; already-mirrored tasks/columns are left
// in place (frozen, no longer synced) rather than deleted.
func (h *JiraIntegrationHandler) Disconnect(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}

	if err := database.GetDB().Where("board_id = ?", boardID).Delete(&models.JiraBoardIntegration{}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error disconnecting Jira integration"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Jira integration disconnected"})
}
```

- [ ] **Step 2: Wire the routes**

In `backend/internal/routes/kanban_routes.go`, add the handler instantiation (after line 17, `activityLogHandler := handlers.NewActivityLogHandler()`):

```go
	jiraIntegrationHandler := handlers.NewJiraIntegrationHandler()
```

Then add the routes right after the "Board entity CRUD" block (after line 26):

```go
	// Jira board integration (super_admin-only, same reasoning as
	// client_status_email_routes.go - passed per-route so it never leaks
	// onto other board routes).
	projects.Post("/:id/boards/:boardId/jira-integration", middleware.RequireRole("super_admin"), jiraIntegrationHandler.Connect)
	projects.Get("/:id/boards/:boardId/jira-integration", middleware.RequireRole("super_admin"), jiraIntegrationHandler.Status)
	projects.Delete("/:id/boards/:boardId/jira-integration", middleware.RequireRole("super_admin"), jiraIntegrationHandler.Disconnect)
```

- [ ] **Step 3: Verify build**

Run: `docker exec devbridge_backend go build ./...`
Expected: builds cleanly.

Run: `docker exec devbridge_backend go vet ./...`
Expected: no issues.

- [ ] **Step 4: Manually verify the endpoints (no automated handler test, matching every existing admin CRUD handler in this codebase)**

Run: `docker exec devbridge_backend curl -s -X GET http://localhost:8080/api/v1/projects/1/boards/1/jira-integration -H "Authorization: Bearer <a super_admin JWT>"`
Expected: `{"connected":false}`.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/jira_integration_handler.go backend/internal/routes/kanban_routes.go
git commit -m "feat(jira): add connect/status/disconnect endpoints"
```

---

### Task 6: Surface Source/JiraStatusName through existing DTO builders

**Files:**
- Modify: `backend/internal/handlers/kanban_dto_builder.go:261-292` (buildTaskDTO's returned literal)
- Modify: `backend/internal/handlers/kanban_handler.go:60-78` (columnDTO)

**Interfaces:**
- Consumes: `models.Task.Source`, `models.KanbanColumn.JiraStatusName`, `models.TaskDTO.Source`, `models.KanbanColumnDTO.JiraStatusName` (Task 1).
- Produces: every `TaskDTO`/`KanbanColumnDTO` returned by the API now includes `source`/`jiraStatusName`.

- [ ] **Step 1: Add Source to buildTaskDTO's returned TaskDTO**

In `backend/internal/handlers/kanban_dto_builder.go`, modify the `models.TaskDTO{...}` literal inside `buildTaskDTO` (currently lines 261-292), adding `Source` right after `Status`:

```go
	return models.TaskDTO{
		ID:              models.IDToStr(t.ID),
		Title:           t.Title,
		Description:     t.Description,
		HTMLDescription: t.HTMLDescription,
		Priority:        t.Priority,
		Status:          t.Status,
		Source:          t.Source,
		ColumnID:        columnID,
		ProjectID:       models.IDToStr(t.ProjectID),
```
(the rest of the literal is unchanged)

- [ ] **Step 2: Add JiraStatusName to columnDTO**

In `backend/internal/handlers/kanban_handler.go`, modify `columnDTO` (currently lines 60-78):

```go
func columnDTO(c models.KanbanColumn, tasks []models.TaskDTO) models.KanbanColumnDTO {
	colTasks := make([]models.TaskDTO, 0)
	for _, t := range tasks {
		if t.ColumnID == models.IDToStr(c.ID) {
			colTasks = append(colTasks, t)
		}
	}
	return models.KanbanColumnDTO{
		ID:             models.IDToStr(c.ID),
		Title:          c.Title,
		Color:          c.Color,
		Position:       c.Position,
		MaxTasks:       c.MaxTasks,
		IsDone:         c.IsDone,
		JiraStatusName: c.JiraStatusName,
		Tasks:          colTasks,
		CreatedAt:      c.CreatedAt,
		UpdatedAt:      c.UpdatedAt,
	}
}
```

- [ ] **Step 3: Verify build and existing tests**

Run: `docker exec devbridge_backend go build ./...`
Expected: builds cleanly.

Run: `docker exec devbridge_backend go test ./internal/handlers/... -v`
Expected: PASS (no regressions).

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handlers/kanban_dto_builder.go backend/internal/handlers/kanban_handler.go
git commit -m "feat(jira): surface task/column Jira metadata through existing DTOs"
```

---

### Task 7: Frontend types + jiraIntegrationService

**Files:**
- Modify: `frontend/src/types/kanban/kanban.types.ts:3-13` (KanbanColumn)
- Modify: `frontend/src/types/kanban/task.types.ts:15-43` (Task)
- Create: `frontend/src/services/kanban/jiraIntegrationService.ts`

**Interfaces:**
- Consumes: `apiClient` (`frontend/src/lib/api.ts`), API responses from Task 5/6.
- Produces: `KanbanColumn.jiraStatusName?: string`, `Task.source: 'local' | 'jira'`, `JiraIntegrationStatus`, `JiraIntegrationConnectData`, `jiraIntegrationService.getStatus/connect/disconnect`.

- [ ] **Step 1: Add jiraStatusName to KanbanColumn and the Jira DTO types**

In `frontend/src/types/kanban/kanban.types.ts`, modify the `KanbanColumn` interface (lines 3-13):

```ts
export interface KanbanColumn {
    id: string;
    title: string;
    color: string;
    position: number;
    maxTasks?: number;
    isDone?: boolean;
    jiraStatusName?: string;
    tasks: Task[];
    createdAt: string;
    updatedAt: string;
}
```

Then add these new interfaces at the end of the file:

```ts
export interface JiraIntegrationStatus {
    connected: boolean;
    baseUrl?: string;
    email?: string;
    projectKey?: string;
    lastSyncAt?: string;
    lastSyncError?: string;
}

export interface JiraIntegrationConnectData {
    baseUrl: string;
    email: string;
    apiToken: string;
    projectKey: string;
}
```

- [ ] **Step 2: Add source to Task**

In `frontend/src/types/kanban/task.types.ts`, modify the `Task` interface (lines 15-43), adding `source` right after `status`:

```ts
export interface Task {
    id: string;
    title: string;
    description: string;
    htmlDescription?: string;
    priority: TaskPriority;
    status: TaskStatus;
    source: 'local' | 'jira';
    columnId: string;
    boardId?: string;
    projectId: string;
    assigneeId?: string;
    assignee?: TaskAssignee;
    estimatedHours: number;
    loggedHours: number;
    tags: TaskTag[];
    timeEntries: TimeEntry[];
    comments: TaskComment[];
    attachments: TaskAttachment[];
    position: number;
    dueDate?: string;
    subtaskProgress?: { total: number; done: number };
    isDoneColumn?: boolean;
    hasUninvoicedHours?: boolean;
    parentTask?: { id: string; title: string };
    createdAt: string;
    updatedAt: string;
    createdBy: string;
    updatedBy: string;
}
```

- [ ] **Step 3: Create the Jira integration service**

Create `frontend/src/services/kanban/jiraIntegrationService.ts`:

```ts
// frontend/src/services/kanban/jiraIntegrationService.ts
import { apiClient } from '@/lib/api';
import type { JiraIntegrationStatus, JiraIntegrationConnectData } from '@/types/kanban';

interface StatusApiResponse extends JiraIntegrationStatus {
    success?: boolean;
    message?: string;
}

interface ActionApiResponse {
    success: boolean;
    message?: string;
}

export const jiraIntegrationService = {
    async getStatus(projectId: string, boardId: string): Promise<JiraIntegrationStatus> {
        return apiClient.get(`/projects/${projectId}/boards/${boardId}/jira-integration`);
    },

    async connect(projectId: string, boardId: string, data: JiraIntegrationConnectData): Promise<JiraIntegrationStatus> {
        const response = await apiClient.post<StatusApiResponse>(`/projects/${projectId}/boards/${boardId}/jira-integration`, data);
        if (response.success === false) throw new Error(response.message || 'Failed to connect Jira integration');
        return response;
    },

    async disconnect(projectId: string, boardId: string): Promise<void> {
        const response = await apiClient.delete<ActionApiResponse>(`/projects/${projectId}/boards/${boardId}/jira-integration`);
        if (!response.success) throw new Error(response.message || 'Failed to disconnect Jira integration');
    },
};
```

- [ ] **Step 4: Verify types**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no new type errors (existing unrelated errors, if any, are pre-existing and out of scope).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/types/kanban/kanban.types.ts frontend/src/types/kanban/task.types.ts frontend/src/services/kanban/jiraIntegrationService.ts
git commit -m "feat(jira): add frontend types and integration service"
```

---

### Task 8: RichTextEditor read-only support

**Files:**
- Modify: `frontend/src/components/ui/rich-text-editor.tsx` (full file, 80 lines)

**Interfaces:**
- Produces: `RichTextEditorProps.readOnly?: boolean`, consumed by Task 9's `DescriptionTab.tsx`.

- [ ] **Step 1: Add the readOnly prop**

Replace the full contents of `frontend/src/components/ui/rich-text-editor.tsx`:

```tsx
'use client';

import React, { useRef, useEffect } from 'react';
import { Bold, Italic, List } from 'lucide-react';
import { cn } from '@/lib/utils';

interface RichTextEditorProps {
    content: string;
    onChange: (html: string, text: string) => void;
    onBlur?: () => void;
    placeholder?: string;
    minHeight?: string;
    readOnly?: boolean;
}

export const RichTextEditor: React.FC<RichTextEditorProps> = ({ content, onChange, onBlur, placeholder, minHeight = '100px', readOnly = false }) => {
    const editorRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        if (editorRef.current && editorRef.current.innerHTML !== content) {
            editorRef.current.innerHTML = content || '';
        }
        // Only sync when content is set from outside (e.g. loading an existing task);
        // typing already updates the DOM directly, so re-running this on every
        // keystroke would fight the browser's own caret position.
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const emitChange = () => {
        const el = editorRef.current;
        if (!el) return;
        onChange(el.innerHTML, el.textContent || '');
    };

    const exec = (command: string) => {
        document.execCommand(command);
        editorRef.current?.focus();
        emitChange();
    };

    return (
        <div className="border border-input rounded-lg overflow-hidden">
            {!readOnly && (
                <div className="flex items-center gap-1 border-b border-border bg-muted px-2 py-1">
                    <button
                        type="button"
                        onClick={() => exec('bold')}
                        className="p-1.5 rounded hover:bg-muted text-muted-foreground"
                    >
                        <Bold size={14} />
                    </button>
                    <button
                        type="button"
                        onClick={() => exec('italic')}
                        className="p-1.5 rounded hover:bg-muted text-muted-foreground"
                    >
                        <Italic size={14} />
                    </button>
                    <button
                        type="button"
                        onClick={() => exec('insertUnorderedList')}
                        className="p-1.5 rounded hover:bg-muted text-muted-foreground"
                    >
                        <List size={14} />
                    </button>
                </div>
            )}
            <div
                ref={editorRef}
                contentEditable={!readOnly}
                onInput={emitChange}
                onBlur={onBlur}
                data-placeholder={placeholder}
                className={cn(
                    'p-3 text-sm text-foreground focus:outline-none prose prose-sm max-w-none',
                    'empty:before:content-[attr(data-placeholder)] empty:before:text-muted-foreground'
                )}
                style={{ minHeight }}
            />
        </div>
    );
};
```

- [ ] **Step 2: Verify types**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no new type errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/ui/rich-text-editor.tsx
git commit -m "feat(jira): add readOnly support to RichTextEditor"
```

---

### Task 9: DescriptionTab read-only enforcement for Jira-sourced tasks

**Files:**
- Modify: `frontend/src/components/kanban/task-detail/DescriptionTab.tsx` (full file, 242 lines)

**Interfaces:**
- Consumes: `Task.source` (Task 7), `RichTextEditor.readOnly` (Task 8).
- Produces: a task detail view where every editable control is disabled and a banner is shown when `task.source === 'jira'`.

- [ ] **Step 1: Disable editable controls and add a banner when task.source is 'jira'**

Replace the full contents of `frontend/src/components/kanban/task-detail/DescriptionTab.tsx`:

```tsx
'use client';

import React, { useState, useEffect } from 'react';
import { useParams } from 'next/navigation';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import { RichTextEditor } from '@/components/ui/rich-text-editor';
import { useTasks } from '@/hooks/kanban';
import { useTaskAssignees } from '@/hooks/kanban/use-task-assignees';
import type { TagLevel, TaskPriority, UpdateTaskData } from '@/types/kanban';
import { Check, Loader2, AlertCircle, Trash2, X } from 'lucide-react';
import { cn } from '@/lib/utils';

type FieldStatus = 'idle' | 'saving' | 'saved' | 'error';

const LEVEL_ORDER: TagLevel[] = ['low', 'medium', 'high'];

interface DescriptionTabProps {
    taskId: string;
    onDeletePermanently?: () => Promise<void>;
}

export const DescriptionTab: React.FC<DescriptionTabProps> = ({ taskId, onDeletePermanently }) => {
    const { projectId } = useParams<{ projectId: string }>();
    const { getTask, updateTask } = useTasks(projectId);
    const { assignees, loadAssignees } = useTaskAssignees(projectId);
    const task = getTask(taskId);
    const isJira = task?.source === 'jira';

    const [title, setTitle] = useState(task?.title ?? '');
    const [description, setDescription] = useState(task?.description ?? '');
    const [htmlDescription, setHtmlDescription] = useState(task?.htmlDescription ?? '');
    const [priority, setPriority] = useState<TaskPriority>(task?.priority ?? ('medium' as TaskPriority));
    const [dueDate, setDueDate] = useState(task?.dueDate ?? '');
    const [estimatedHours, setEstimatedHours] = useState(task?.estimatedHours ?? 0);
    const [assigneeId, setAssigneeId] = useState(task?.assigneeId ?? '');
    const [tags, setTags] = useState(task?.tags ?? []);
    const [tagInput, setTagInput] = useState('');
    const [status, setStatus] = useState<Record<string, FieldStatus>>({});
    const [isDeleting, setIsDeleting] = useState(false);

    useEffect(() => { loadAssignees(); }, [loadAssignees]);

    const save = async (field: string, data: UpdateTaskData) => {
        setStatus((s) => ({ ...s, [field]: 'saving' }));
        try {
            await updateTask(taskId, data);
            setStatus((s) => ({ ...s, [field]: 'saved' }));
            setTimeout(() => setStatus((s) => ({ ...s, [field]: 'idle' })), 1500);
        } catch {
            setStatus((s) => ({ ...s, [field]: 'error' }));
        }
    };

    const Indicator = ({ field }: { field: string }) => {
        const s = status[field] ?? 'idle';
        const icon =
            s === 'saving' ? <Loader2 className="w-3.5 h-3.5 animate-spin text-muted-foreground" /> :
            s === 'saved' ? <Check className="w-3.5 h-3.5 text-success" /> :
            s === 'error' ? <AlertCircle className="w-3.5 h-3.5 text-destructive" /> :
            null;
        return (
            <div className={cn('w-3.5 h-3.5 shrink-0 transition-opacity duration-300', s === 'idle' ? 'opacity-0' : 'opacity-100')}>
                {icon}
            </div>
        );
    };

    const commitTags = (next: typeof tags) => {
        if (isJira) return;
        setTags(next);
        save('tags', { tags: next.map((t) => ({ name: t.name, level: t.level })) });
    };
    const handleAddTag = () => {
        if (!tagInput.trim()) return;
        commitTags([...tags, { id: tagInput, name: tagInput, color: 'blue', level: 'medium' as TagLevel }]);
        setTagInput('');
    };
    const handleRemoveTag = (name: string) => commitTags(tags.filter((t) => t.name !== name));
    const handleCycleTagLevel = (name: string) => commitTags(tags.map((t) =>
        t.name === name ? { ...t, level: LEVEL_ORDER[(LEVEL_ORDER.indexOf(t.level) + 1) % LEVEL_ORDER.length] } : t
    ));

    if (!task) return null;

    const handleTitleBlur = () => {
        if (isJira) return;
        if (!title.trim()) {
            setTitle(task.title);
            setStatus((s) => ({ ...s, title: 'error' }));
            return;
        }
        save('title', { title });
    };

    return (
        <div className="space-y-8">
            {isJira && (
                <div className="text-sm text-muted-foreground bg-muted border border-border rounded-lg px-3 py-2">
                    This task is mirrored from Jira and is read-only here. Time tracking and comments are still fully usable.
                </div>
            )}

            <section className="space-y-4">
                <div className="flex items-center gap-2">
                    <div className="flex-1">
                        <Input
                            label="Task Title"
                            value={title}
                            onChange={setTitle}
                            onBlur={handleTitleBlur}
                            disabled={isJira}
                            required
                        />
                    </div>
                    <Indicator field="title" />
                </div>

                <div className="space-y-2">
                    <label className="block text-sm font-medium text-foreground">Description</label>
                    <div className="flex items-start gap-2">
                        <div className="flex-1">
                            <RichTextEditor
                                content={htmlDescription}
                                onChange={(html, text) => { setHtmlDescription(html); setDescription(text); }}
                                onBlur={() => !isJira && save('description', { description, htmlDescription })}
                                minHeight="120px"
                                readOnly={isJira}
                            />
                        </div>
                        <Indicator field="description" />
                    </div>
                </div>
            </section>

            <section className="space-y-4 rounded-xl bg-muted p-4">
                <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Details</h3>

                <div className="grid grid-cols-2 gap-4">
                    <div className="flex items-center gap-2">
                        <div className="flex-1">
                            <Select
                                label="Priority"
                                value={priority}
                                disabled={isJira}
                                onChange={(value) => {
                                    if (isJira) return;
                                    const next = value as TaskPriority;
                                    setPriority(next);
                                    save('priority', { priority: next });
                                }}
                                options={[
                                    { value: 'low', label: 'Low' },
                                    { value: 'medium', label: 'Medium' },
                                    { value: 'high', label: 'High' },
                                    { value: 'urgent', label: 'Urgent' },
                                ]}
                            />
                        </div>
                        <Indicator field="priority" />
                    </div>
                    <div className="flex items-center gap-2">
                        <div className="flex-1">
                            <Input
                                label="Estimated Hours"
                                type="number"
                                min="0"
                                step="0.5"
                                value={String(estimatedHours)}
                                disabled={isJira}
                                onChange={(v) => setEstimatedHours(Number(v))}
                                onBlur={() => !isJira && save('estimatedHours', { estimatedHours })}
                            />
                        </div>
                        <Indicator field="estimatedHours" />
                    </div>
                </div>

                <div className="grid grid-cols-2 gap-4">
                    <div className="flex items-center gap-2">
                        <div className="flex-1">
                            <Input
                                label="Due Date"
                                type="date"
                                value={dueDate}
                                disabled={isJira}
                                onChange={(v) => { if (isJira) return; setDueDate(v); save('dueDate', { dueDate: v }); }}
                            />
                        </div>
                        <Indicator field="dueDate" />
                    </div>
                    <div className="flex items-center gap-2">
                        <div className="flex-1">
                            <Select
                                label="Assignee"
                                value={assigneeId}
                                disabled={isJira}
                                onChange={(value) => { if (isJira) return; setAssigneeId(value); save('assignee', { assigneeId: value }); }}
                                options={[
                                    { value: '', label: 'Unassigned' },
                                    ...assignees.map((a) => ({ value: String(a.user_id), label: a.user_name })),
                                ]}
                            />
                        </div>
                        <Indicator field="assignee" />
                    </div>
                </div>
            </section>

            <section className="space-y-2">
                <label className="block text-sm font-medium text-foreground">Tags</label>
                <div className="flex flex-wrap items-center gap-2 p-2 border rounded-lg border-border focus-within:ring-ring focus-within:border-ring">
                    {tags.map((tag) => (
                        <Badge key={tag.name} variant="secondary" className="gap-1">
                            <button type="button" disabled={isJira} onClick={() => handleCycleTagLevel(tag.name)}>{tag.level}</button>
                            {tag.name}
                            {!isJira && (
                                <button type="button" onClick={() => handleRemoveTag(tag.name)} className="ml-1 hover:text-destructive">
                                    <X size={12} />
                                </button>
                            )}
                        </Badge>
                    ))}
                    {!isJira && (
                        <input
                            className="flex-1 min-w-[120px] outline-none py-1"
                            value={tagInput}
                            onChange={(e) => setTagInput(e.target.value)}
                            onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); handleAddTag(); } }}
                            placeholder="Add tag…"
                        />
                    )}
                </div>
            </section>

            {onDeletePermanently && !isJira && (
                <div className="pt-4 border-t">
                    <Button
                        type="button"
                        variant="danger"
                        size="sm"
                        icon={Trash2}
                        loading={isDeleting}
                        onClick={async () => {
                            const subtaskWarning = task.subtaskProgress?.total
                                ? ` This will also permanently delete its ${task.subtaskProgress.total} subtask${task.subtaskProgress.total === 1 ? '' : 's'}.`
                                : '';
                            if (!confirm(`Delete this task permanently? It will be removed from every board it appears on.${subtaskWarning}`)) return;
                            setIsDeleting(true);
                            try { await onDeletePermanently(); } finally { setIsDeleting(false); }
                        }}
                    >
                        {isDeleting ? 'Deleting…' : 'Delete permanently'}
                    </Button>
                </div>
            )}
        </div>
    );
};
```

- [ ] **Step 2: Verify types**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no new type errors. (If `Input`/`Select` don't currently accept a `disabled` prop, they already forward `...props` from `React.ComponentProps<"input">`/similar per their existing definitions, so `disabled` passes through natively.)

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/kanban/task-detail/DescriptionTab.tsx
git commit -m "feat(jira): make task detail view read-only for Jira-sourced tasks"
```

---

### Task 10: TaskCard Jira badge and blocked drag

**Files:**
- Modify: `frontend/src/components/kanban/task-card.tsx:74-94` (handleDragStart)
- Modify: `frontend/src/components/kanban/task-card.tsx:117-134` (header area)

**Interfaces:**
- Consumes: `Task.source` (Task 7).

- [ ] **Step 1: Block drag for Jira-sourced tasks**

In `frontend/src/components/kanban/task-card.tsx`, modify `handleDragStart` (currently lines 74-94):

```tsx
    const handleDragStart = (e: React.DragEvent) => {
        if (!permissions.canMoveTasks || task.source === 'jira') {
            e.preventDefault();
            return;
        }

        console.log('🎯 TaskCard drag start:', task.title);

        // Adatok tárolása a drag event-ben
        e.dataTransfer.setData('application/json', JSON.stringify({
            taskId: task.id,
            fromColumn: task.columnId
        }));
        e.dataTransfer.effectAllowed = 'move';

        // Visual feedback
        e.currentTarget.style.opacity = '0.5';

        // Hook callback hívása
        onDragStart();
    };
```

Also update the `draggable` prop on the root `<div>` (currently line 108) and the cursor class (line 113):

```tsx
        <div
            draggable={permissions.canMoveTasks && task.source !== 'jira'}
            onDragStart={handleDragStart}
            onDragEnd={handleDragEnd}
            className={cn(
                "group bg-card rounded-lg border border-border hover:border-primary/30 hover:shadow-sm transition-all duration-150 p-3.5",
                permissions.canMoveTasks && task.source !== 'jira' ? "cursor-move" : "cursor-default"
            )}
        >
```

- [ ] **Step 2: Add a Jira badge to the card header**

Import `Braces` alongside the other lucide icons (modify the import block, currently lines 5-17):

```tsx
import {
    Edit2,
    MessageSquare,
    Timer,
    Trash2,
    FolderPlus,
    GripVertical,
    Clock,
    Calendar,
    AlertCircle,
    ListTree,
    Receipt,
    Braces
} from 'lucide-react';
```

Then, in the header block (currently lines 117-134), add the badge right after the title `<h4>`:

```tsx
            {/* Header */}
            <div className="flex items-start justify-between gap-2 mb-2">
                <div className="flex items-start gap-2 flex-1 min-w-0">
                    <PriorityIcon
                        size={14}
                        className={cn("mt-0.5 flex-shrink-0", priorityIconColors[task.priority])}
                    />
                    <h4 className="font-medium text-foreground line-clamp-2 text-sm leading-snug">
                        {task.title}
                    </h4>
                    {task.source === 'jira' && (
                        <span
                            title="Mirrored from Jira"
                            className="flex items-center gap-0.5 text-[10px] font-medium text-primary bg-primary/10 rounded px-1 py-0.5 flex-shrink-0"
                        >
                            <Braces size={10} />
                        </span>
                    )}
                </div>

                {permissions.canMoveTasks && task.source !== 'jira' && (
                    <GripVertical
                        size={14}
                        className="text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity flex-shrink-0"
                    />
                )}
            </div>
```

- [ ] **Step 3: Verify types**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no new type errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/kanban/task-card.tsx
git commit -m "feat(jira): show Jira badge and block drag on mirrored task cards"
```

---

### Task 11: Jira integration panel + read-only column rendering

**Files:**
- Create: `frontend/src/components/kanban/jira-integration-panel.tsx`
- Modify: `frontend/src/components/kanban/column-settings-modal.tsx` (full file)

**Interfaces:**
- Consumes: `jiraIntegrationService` (Task 7), `useAuth` (`@/hooks/auth/use-auth`), `isSuperAdmin` (`@/utils/permissions`), `KanbanColumn.jiraStatusName` (Task 7).
- Produces: `JiraIntegrationPanel` component, rendered inside `ColumnSettingsModal`; Jira-derived columns render as a simplified read-only row.

- [ ] **Step 1: Create the Jira integration panel**

Create `frontend/src/components/kanban/jira-integration-panel.tsx`:

```tsx
'use client';

import React from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useAuth } from '@/hooks/auth/use-auth';
import { isSuperAdmin } from '@/utils/permissions';
import { jiraIntegrationService } from '@/services/kanban/jiraIntegrationService';
import type { JiraIntegrationStatus } from '@/types/kanban';

interface JiraIntegrationPanelProps {
    projectId: string;
    boardId: string;
}

export const JiraIntegrationPanel: React.FC<JiraIntegrationPanelProps> = ({ projectId, boardId }) => {
    const { user } = useAuth();
    const [status, setStatus] = React.useState<JiraIntegrationStatus | null>(null);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);
    const [saving, setSaving] = React.useState(false);
    const [baseUrl, setBaseUrl] = React.useState('');
    const [email, setEmail] = React.useState('');
    const [apiToken, setApiToken] = React.useState('');
    const [projectKey, setProjectKey] = React.useState('');

    const loadStatus = React.useCallback(() => {
        setLoading(true);
        jiraIntegrationService.getStatus(projectId, boardId)
            .then(setStatus)
            .catch(() => setStatus({ connected: false }))
            .finally(() => setLoading(false));
    }, [projectId, boardId]);

    React.useEffect(() => {
        loadStatus();
    }, [loadStatus]);

    if (!isSuperAdmin(user)) return null;
    if (loading) return null;

    const handleConnect = async () => {
        setError(null);
        setSaving(true);
        try {
            await jiraIntegrationService.connect(projectId, boardId, { baseUrl, email, apiToken, projectKey });
            setBaseUrl('');
            setEmail('');
            setApiToken('');
            setProjectKey('');
            loadStatus();
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to connect to Jira');
        } finally {
            setSaving(false);
        }
    };

    const handleDisconnect = async () => {
        if (!window.confirm('Disconnect the Jira integration? Already-mirrored tasks and columns will stay in place but will no longer be synced.')) return;
        setError(null);
        setSaving(true);
        try {
            await jiraIntegrationService.disconnect(projectId, boardId);
            loadStatus();
        } catch (err) {
            setError(err instanceof Error ? err.message : 'Failed to disconnect the Jira integration');
        } finally {
            setSaving(false);
        }
    };

    return (
        <div className="border border-dashed border-input rounded-lg p-3 space-y-3">
            <h3 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Jira Integration</h3>

            {error && (
                <div className="text-sm text-destructive bg-destructive/10 border border-destructive/20 rounded-lg px-3 py-2">
                    {error}
                </div>
            )}

            {status?.connected ? (
                <div className="space-y-2">
                    <p className="text-sm text-foreground">
                        Connected: <span className="font-medium">{status.projectKey}</span> ({status.email})
                    </p>
                    <p className="text-xs text-muted-foreground">
                        {status.lastSyncAt ? `Last synced: ${new Date(status.lastSyncAt).toLocaleString()}` : 'Not synced yet'}
                    </p>
                    {status.lastSyncError && (
                        <p className="text-xs text-destructive">Last sync error: {status.lastSyncError}</p>
                    )}
                    <Button variant="outline" size="sm" loading={saving} onClick={handleDisconnect}>
                        Disconnect
                    </Button>
                </div>
            ) : (
                <div className="space-y-2">
                    <Input label="Base URL" value={baseUrl} onChange={setBaseUrl} placeholder="https://yourcompany.atlassian.net" />
                    <Input label="Email" value={email} onChange={setEmail} placeholder="me@example.com" />
                    <Input label="API Token" type="password" value={apiToken} onChange={setApiToken} />
                    <Input label="Project Key" value={projectKey} onChange={setProjectKey} placeholder="PROJ" />
                    <Button
                        size="sm"
                        loading={saving}
                        disabled={!baseUrl || !email || !apiToken || !projectKey}
                        onClick={handleConnect}
                    >
                        Connect
                    </Button>
                </div>
            )}
        </div>
    );
};
```

- [ ] **Step 2: Wire the panel into ColumnSettingsModal and render Jira-derived columns read-only**

Replace the full contents of `frontend/src/components/kanban/column-settings-modal.tsx`:

```tsx
'use client';

import React from 'react';
import { ArrowUp, ArrowDown, Trash2, Plus, Braces } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { cn } from '@/lib/utils';
import { useKanbanStore } from '@/stores/kanban';
import { kanbanService } from '@/services/kanban';
import { JiraIntegrationPanel } from '@/components/kanban/jira-integration-panel';
import type { KanbanColumn } from '@/types/kanban';

interface ColumnSettingsModalProps {
    projectId: string;
    boardId: string;
}

const COLUMN_COLORS = [
    'bg-primary',
    'bg-warning',
    'bg-primary',
    'bg-success',
    'bg-destructive',
    'bg-primary',
    'bg-primary',
    'bg-muted-foreground/30',
];

interface ColumnTitleInputProps {
    title: string;
    onCommit: (title: string) => void;
    className?: string;
}

const ColumnTitleInput: React.FC<ColumnTitleInputProps> = ({ title, onCommit, className }) => {
    const [draft, setDraft] = React.useState(title);

    React.useEffect(() => {
        setDraft(title);
    }, [title]);

    return (
        <Input
            value={draft}
            onChange={setDraft}
            onBlur={() => {
                const trimmed = draft.trim();
                if (trimmed && trimmed !== title) {
                    onCommit(trimmed);
                } else {
                    setDraft(title);
                }
            }}
            className={className}
        />
    );
};

const ColorPicker: React.FC<{ value: string; onChange: (color: string) => void }> = ({ value, onChange }) => (
    <div className="flex gap-1.5">
        {COLUMN_COLORS.map((color) => (
            <button
                key={color}
                type="button"
                aria-label={color}
                onClick={() => onChange(color)}
                className={cn(
                    'w-5 h-5 rounded-full transition-transform hover:scale-110',
                    color,
                    value === color && 'ring-2 ring-offset-1 ring-ring'
                )}
            />
        ))}
    </div>
);

export const ColumnSettingsModal: React.FC<ColumnSettingsModalProps> = ({ projectId, boardId }) => {
    const columns = useKanbanStore((state) => state.columns);
    const setColumns = useKanbanStore((state) => state.setColumns);
    const addColumnToStore = useKanbanStore((state) => state.addColumn);
    const updateColumnInStore = useKanbanStore((state) => state.updateColumn);
    const deleteColumnFromStore = useKanbanStore((state) => state.deleteColumn);

    const [error, setError] = React.useState<string | null>(null);
    const [savingId, setSavingId] = React.useState<string | null>(null);
    const [newTitle, setNewTitle] = React.useState('');
    const [newColor, setNewColor] = React.useState(COLUMN_COLORS[0]);

    const sortedColumns = [...columns].sort((a, b) => a.position - b.position);

    const persistColumn = async (columnId: string, updates: Partial<Pick<KanbanColumn, 'title' | 'color' | 'maxTasks' | 'isDone'>>): Promise<boolean> => {
        setError(null);
        setSavingId(columnId);
        try {
            await kanbanService.updateColumn(projectId, boardId, columnId, updates);
            updateColumnInStore(columnId, updates);
            return true;
        } catch {
            setError('Failed to update column.');
            return false;
        } finally {
            setSavingId(null);
        }
    };

    const handleMove = async (index: number, direction: -1 | 1) => {
        const targetIndex = index + direction;
        if (targetIndex < 0 || targetIndex >= sortedColumns.length) return;

        const reordered = [...sortedColumns];
        [reordered[index], reordered[targetIndex]] = [reordered[targetIndex], reordered[index]];
        const orders = reordered.map((col, i) => ({ columnId: col.id, position: i }));

        setError(null);
        try {
            await kanbanService.reorderColumns(projectId, boardId, orders);
            setColumns(reordered.map((col, i) => ({ ...col, position: i })));
        } catch {
            setError('Failed to reorder columns.');
        }
    };

    const handleDelete = async (column: KanbanColumn) => {
        const taskCount = column.tasks?.length ?? 0;
        const confirmMessage = taskCount > 0
            ? `Delete "${column.title}"? ${taskCount} task(s) in it will be permanently deleted if this is their only remaining placement; tasks placed on other boards will survive there.`
            : `Delete "${column.title}"?`;
        if (!window.confirm(confirmMessage)) return;

        setError(null);
        setSavingId(column.id);
        try {
            await kanbanService.deleteColumn(projectId, boardId, column.id);
            deleteColumnFromStore(column.id);
        } catch {
            setError('Failed to delete column.');
        } finally {
            setSavingId(null);
        }
    };

    const handleAdd = async () => {
        if (!newTitle.trim()) return;

        setError(null);
        try {
            const column = await kanbanService.createColumn(projectId, boardId, {
                title: newTitle.trim(),
                color: newColor,
                position: columns.length,
            }) as KanbanColumn;
            addColumnToStore(column);
            setNewTitle('');
            setNewColor(COLUMN_COLORS[0]);
        } catch {
            setError('Failed to create column.');
        }
    };

    return (
        <div className="space-y-4">
            <JiraIntegrationPanel projectId={projectId} boardId={boardId} />

            {error && (
                <div className="text-sm text-destructive bg-destructive/10 border border-destructive/20 rounded-lg px-3 py-2">
                    {error}
                </div>
            )}

            <div className="space-y-3">
                {sortedColumns.map((column, index) => (
                    column.jiraStatusName ? (
                        <div key={column.id} className="border border-border rounded-lg p-3 flex items-center gap-2">
                            <Braces size={14} className="text-muted-foreground flex-shrink-0" />
                            <span className="flex-1 text-sm text-foreground">{column.title}</span>
                            <span className="text-xs text-muted-foreground">Mirrored from Jira</span>
                        </div>
                    ) : (
                        <div key={column.id} className="border border-border rounded-lg p-3 space-y-3">
                            <div className="flex items-center gap-2">
                                <div className="flex flex-col">
                                    <button
                                        type="button"
                                        disabled={index === 0}
                                        onClick={() => handleMove(index, -1)}
                                        className="p-1 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted disabled:opacity-30 disabled:hover:bg-transparent transition-colors"
                                    >
                                        <ArrowUp size={14} />
                                    </button>
                                    <button
                                        type="button"
                                        disabled={index === sortedColumns.length - 1}
                                        onClick={() => handleMove(index, 1)}
                                        className="p-1 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted disabled:opacity-30 disabled:hover:bg-transparent transition-colors"
                                    >
                                        <ArrowDown size={14} />
                                    </button>
                                </div>

                                <ColumnTitleInput
                                    title={column.title}
                                    onCommit={(title) => persistColumn(column.id, { title })}
                                    className="flex-1"
                                />

                                <button
                                    type="button"
                                    disabled={savingId === column.id}
                                    onClick={() => handleDelete(column)}
                                    className="p-2 rounded-md text-muted-foreground hover:text-destructive hover:bg-destructive/10 disabled:opacity-30 disabled:hover:bg-transparent transition-colors"
                                >
                                    <Trash2 size={16} />
                                </button>
                            </div>

                            <div className="flex items-center gap-6 pl-8">
                                <div className="flex items-center gap-2">
                                    <label className="text-xs font-medium text-muted-foreground">WIP limit</label>
                                    <input
                                        type="number"
                                        min={0}
                                        placeholder="—"
                                        value={column.maxTasks ?? ''}
                                        onChange={(e) => updateColumnInStore(column.id, {
                                            maxTasks: e.target.value === '' ? undefined : Number(e.target.value),
                                        })}
                                        onBlur={(e) => persistColumn(column.id, {
                                            maxTasks: e.target.value === '' ? undefined : Number(e.target.value),
                                        })}
                                        className="w-16 rounded-lg border border-input px-2 py-1.5 text-sm shadow-sm focus:border-ring focus:outline-none focus:ring-1 focus:ring-ring"
                                    />
                                </div>

                                <div className="flex items-center gap-2">
                                    <label className="text-xs font-medium text-muted-foreground">Color</label>
                                    <ColorPicker
                                        value={column.color}
                                        onChange={(color) => persistColumn(column.id, { color })}
                                    />
                                </div>

                                <label className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
                                    <input
                                        type="checkbox"
                                        checked={column.isDone ?? false}
                                        onChange={(e) => {
                                            const isDone = e.target.checked;
                                            if (isDone) {
                                                const other = sortedColumns.find((c) => c.id !== column.id && c.isDone);
                                                if (other && !window.confirm(`"${other.title}" is currently the Done column. Mark "${column.title}" as Done instead?`)) {
                                                    return;
                                                }
                                            }
                                            persistColumn(column.id, { isDone }).then((ok) => {
                                                if (ok && isDone) {
                                                    sortedColumns.forEach((c) => {
                                                        if (c.id !== column.id && c.isDone) updateColumnInStore(c.id, { isDone: false });
                                                    });
                                                }
                                            });
                                        }}
                                        className="rounded border-input"
                                    />
                                    Done column
                                </label>
                            </div>
                        </div>
                    )
                ))}
            </div>

            <div className="border border-dashed border-input rounded-lg p-3 space-y-3">
                <div className="flex items-center gap-2">
                    <Input
                        value={newTitle}
                        onChange={setNewTitle}
                        placeholder="New column title..."
                        className="flex-1"
                    />
                    <Button icon={Plus} onClick={handleAdd} disabled={!newTitle.trim()}>
                        Add Column
                    </Button>
                </div>
                <div className="flex items-center gap-2 pl-1">
                    <label className="text-xs font-medium text-muted-foreground">Color</label>
                    <ColorPicker value={newColor} onChange={setNewColor} />
                </div>
            </div>
        </div>
    );
};
```

- [ ] **Step 3: Verify types**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no new type errors.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/kanban/jira-integration-panel.tsx frontend/src/components/kanban/column-settings-modal.tsx
git commit -m "feat(jira): add connect/status/disconnect panel and read-only column rendering"
```

---

### Task 12: Backend full verification

**Files:** none (verification only)

- [ ] **Step 1: Full build**

Run: `docker exec devbridge_backend go build ./...`
Expected: builds cleanly.

- [ ] **Step 2: Vet**

Run: `docker exec devbridge_backend go vet ./...`
Expected: no issues.

- [ ] **Step 3: Format check**

Run: `docker exec devbridge_backend gofmt -l .`
Expected: no output (no unformatted files). If any files are listed, run `docker exec devbridge_backend gofmt -w <file>` and re-check.

- [ ] **Step 4: Full test suite**

Run: `docker exec devbridge_backend go test ./... -v`
Expected: PASS for every package. Report the exact pass/fail count; do not infer full-suite success from any targeted subset run earlier in this plan. Separate any pre-existing failures (present before this feature's changes) from anything newly introduced.

- [ ] **Step 5: Commit (only if verification uncovered fixes)**

If Steps 1-4 required any code changes to pass, commit them:

```bash
git add -A
git commit -m "fix(jira): address backend verification findings"
```

---

### Task 13: Frontend full verification + gated manual Jira test

**Files:** none (verification only)

- [ ] **Step 1: Full type check**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no new type errors compared to the pre-feature baseline.

- [ ] **Step 2: Manual browser verification of connect/status/disconnect and read-only behavior**

With the dev server running, as a super_admin:
1. Open a board's column settings modal and confirm the Jira Integration panel appears with an empty connect form.
2. Attempt to connect with an invalid `baseUrl`/token combination and confirm a clear error message appears (no crash).
3. Confirm a non-super_admin user does not see the panel at all.

- [ ] **Step 3: User-gated manual real-Jira-account connection test**

This step connects to a real, external Jira Cloud account and must NEVER proceed without the user's fresh, explicit approval at execution time — do not treat prior approvals in this conversation as covering this step. Ask the user directly: "I'm about to test the connect flow against a real Jira Cloud account. Do you want to proceed, and if so, which account/credentials should I use?" Only continue once they explicitly confirm and supply (or approve using) real credentials.

Once approved:
1. Connect the board to the real Jira project.
2. Confirm the connect call succeeds and the status panel shows the connected project/email.
3. Wait for (or manually trigger) a sync pass and confirm mirrored issues appear as read-only task cards with the Jira badge, grouped into auto-created columns matching their Jira statuses.
4. Confirm editing/moving/deleting a mirrored task or column is rejected (403) both via the UI (controls disabled) and, if desired, via a direct `curl` to the guarded endpoints.
5. Disconnect and confirm the mirrored tasks/columns remain in place (frozen) rather than being deleted.

- [ ] **Step 4: Report results**

Report exactly which checks were run (Steps 1-3), their outcomes, and explicitly note that Step 3 only ran if and when the user gave fresh approval — do not claim it ran otherwise.

---

## Self-Review

**Spec coverage:**
- Board→Jira project connection (super_admin only, per-board): Tasks 1, 5, 11. ✓
- Read-only mirror of assigned, not-yet-done issues, coexisting with local tasks/columns: Tasks 1, 3. ✓
- Never writes back to Jira: `JiraClient` interface only exposes `TestConnection`/`SearchAssignedIssues` (read-only Jira API calls); no write/update call to Jira exists anywhere in Tasks 2-3. ✓
- Time tracking/comments on mirrored tasks stay fully usable locally: the guards (Task 4) only block `UpdateTask`/`DeleteTask`/`MoveTask`/`PlaceTask`/`RemovePlacement`/column mutation endpoints; the existing comment (`commentHandler`) and time-entry (`timeEntryHandler`) routes in `kanban_routes.go` are untouched and remain fully usable on Jira-sourced tasks. ✓
- Disconnecting freezes (does not delete) already-mirrored tasks/columns: `Disconnect` (Task 5) only deletes the `JiraBoardIntegration` row, never touches `Task`/`KanbanColumn` rows. ✓
- Jira Cloud REST API v3 (`/rest/api/3/myself`, `/rest/api/3/search/jql`, JQL string, pagination, ADF parsing): Task 2. ✓
- Testing plan intent (sync-job upsert/delete logic, read-only guard checks) reconciled with actual no-DB-test convention: called out explicitly in Global Constraints and again in Tasks 3-4. ✓
- Route path: adapted and called out explicitly in Global Constraints and Task 5. ✓

**Placeholder scan:** no "TBD"/"TODO"/"similar to Task N" patterns found; every step contains complete, runnable code or a fully-specified manual verification procedure.

**Type consistency:** `JiraIssue{Key, Summary, Description, StatusName}` (Task 2) is used identically in Task 3's `syncBoardIntegration`. `models.JiraBoardIntegration` fields (`BoardID`, `BaseURL`, `Email`, `APIToken`, `ProjectKey`, `ConnectedBy`, `ConnectedAt`, `LastSyncAt`, `LastSyncError`) are used identically across Tasks 2, 3, 5. `isJiraSourced`/`isJiraColumn` (Task 4) match their usage in Tasks 4 exactly. `Task.source`/`KanbanColumn.jiraStatusName` (Task 7, frontend) match the JSON field names emitted by `TaskDTO.Source`/`KanbanColumnDTO.JiraStatusName` (Task 1/6, backend) exactly (`source`, `jiraStatusName`). `JiraIntegrationStatus`/`JiraIntegrationConnectData` (Task 7) match `models.JiraIntegrationStatusDTO`/`models.ConnectJiraIntegrationRequest` (Task 1) field-for-field (`connected`/`baseUrl`/`email`/`projectKey`/`lastSyncAt`/`lastSyncError` and `baseUrl`/`email`/`apiToken`/`projectKey` respectively).

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-09-28-jira-integration.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration.

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints.

Which approach?
