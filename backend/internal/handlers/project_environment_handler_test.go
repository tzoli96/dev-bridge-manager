// backend/internal/handlers/project_environment_handler_test.go
package handlers

import (
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestValidateEnvironmentRequest(t *testing.T) {
	cases := []struct {
		name    string
		req     models.ProjectEnvironmentRequest
		wantErr bool
	}{
		{"name only", models.ProjectEnvironmentRequest{Name: "prod"}, false},
		{"full https", models.ProjectEnvironmentRequest{Name: "prod", URL: "https://app.example.com", GitRepoURL: "https://github.com/org/repo.git"}, false},
		{"scp-style repo", models.ProjectEnvironmentRequest{Name: "dev", GitRepoURL: "git@github.com:org/repo.git"}, false},
		{"ssh repo", models.ProjectEnvironmentRequest{Name: "dev", GitRepoURL: "ssh://git@host/org/repo.git"}, false},
		{"blank name", models.ProjectEnvironmentRequest{Name: "   "}, true},
		{"url without scheme", models.ProjectEnvironmentRequest{Name: "prod", URL: "app.example.com"}, true},
		{"javascript url rejected", models.ProjectEnvironmentRequest{Name: "prod", URL: "javascript:alert(1)"}, true},
		{"repo without scheme", models.ProjectEnvironmentRequest{Name: "prod", GitRepoURL: "github.com/org/repo"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.req
			msg := validateEnvironmentRequest(&req)
			if (msg != "") != tc.wantErr {
				t.Fatalf("validateEnvironmentRequest(%+v) = %q, wantErr %v", tc.req, msg, tc.wantErr)
			}
		})
	}
}

func TestValidateEnvironmentRequestTrims(t *testing.T) {
	req := models.ProjectEnvironmentRequest{Name: "  prod  ", URL: " https://a.io ", GitRepoURL: " git@h:o/r.git "}
	if msg := validateEnvironmentRequest(&req); msg != "" {
		t.Fatalf("unexpected error: %s", msg)
	}
	if req.Name != "prod" || req.URL != "https://a.io" || req.GitRepoURL != "git@h:o/r.git" {
		t.Fatalf("fields not trimmed: %+v", req)
	}
}
