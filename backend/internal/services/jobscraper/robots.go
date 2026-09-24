// backend/internal/services/jobscraper/robots.go
package jobscraper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// robotsAllowed reports whether rawURL's path is allowed by that host's
// robots.txt for a "User-agent: *" block. A missing or unreachable
// robots.txt is treated as allowed - the common convention: no declared
// rules means no restrictions.
func robotsAllowed(ctx context.Context, client *http.Client, rawURL string) (bool, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false, fmt.Errorf("parsing url: %w", err)
	}

	robotsURL := fmt.Sprintf("%s://%s/robots.txt", u.Scheme, u.Host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, nil)
	if err != nil {
		return false, fmt.Errorf("building robots.txt request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return true, nil // unreachable robots.txt: treat as allowed
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return true, nil // missing robots.txt: treat as allowed
	}

	disallowed := parseDisallowedPaths(resp.Body)
	for _, prefix := range disallowed {
		if strings.HasPrefix(u.Path, prefix) {
			return false, nil
		}
	}
	return true, nil
}

// parseDisallowedPaths does a minimal, hand-rolled robots.txt parse: it
// only tracks the "User-agent: *" block's "Disallow:" lines, since that is
// the only rule shape this package needs to respect (see the design doc's
// Global Constraints for why no library is used here).
func parseDisallowedPaths(body io.Reader) []string {
	var disallowed []string
	inWildcardBlock := false
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "user-agent:"):
			agent := strings.TrimSpace(line[len("user-agent:"):])
			inWildcardBlock = agent == "*"
		case inWildcardBlock && strings.HasPrefix(lower, "disallow:"):
			path := strings.TrimSpace(line[len("disallow:"):])
			if path != "" {
				disallowed = append(disallowed, path)
			}
		}
	}
	return disallowed
}
