# Job Search & AI Matching Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Automatically monitor profession.hu for new job listings, let the operator manually add listings from other sites (nofluffjobs.com, LinkedIn, etc.), score every listing against the operator's CV/skills/preferences with AI, and generate an editable AI cover-letter draft that gets sent through the existing Gmail-send flow.

**Architecture:** A new `jobscraper` Go package does the HTTP fetching/HTML parsing (goquery) and robots.txt compliance, feeding a `job_listings` table. A `RunScrape` orchestrator (mirroring `RunGmailSync`) saves new listings and calls a new AI-service endpoint (`/job-match`, a `pydantic_ai` agent) to score every unscored listing into `job_matches`. A super_admin-only `/admin/job-search/*` API and a standalone Next.js page expose profile editing, on-demand scanning, manual URL-add, the ranked match list, and an AI-drafted application (`/job-application-draft`) that reuses the existing `/emails/send` endpoint to actually send.

**Tech Stack:** Go (Fiber, GORM, golang-migrate), goquery (new direct dependency for HTML parsing), Python (FastAPI, pydantic_ai/Gemini), Next.js/React/TypeScript.

**Spec:** `docs/superpowers/specs/2026-09-24-job-search-matching-design.md`

## Global Constraints

- No GORM `AutoMigrate` - all schema changes go through `golang-migrate` files under `backend/migrations/`. Re-verify `000032` is still free before creating the migration (`ls backend/migrations | sort -V | tail -5`).
- `goquery` (`github.com/PuerkitoBio/goquery`) is a deliberate new direct dependency - no HTML-parsing mechanism exists in this codebase and manual regex parsing would be worse. Add it with `go get`, not by hand-editing `go.mod`.
- No separate robots.txt library - the rule format needed (`User-agent: *` / `Disallow:`) is simple enough to hand-parse.
- Every `pydantic_ai.Agent` reads its own `GEMINI_MODEL` env var independently (`os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")`) rather than importing it from another module, to avoid a circular import - same reasoning as `categorize_email.py`/`draft_reply.py`/`task_breakdown.py`.
- Every Go backend command runs via `docker exec devbridge_backend ...`, every AI-service Python command via `docker exec devbridge_ai ...`, every frontend command via `docker exec devbridge_frontend ...` - never on host.
- `/admin/job-search/*` routes are `super_admin`-only, applied per-route via `middleware.RequireRole("super_admin")` - never via `admin.Use(...)`, so the restriction never leaks onto other `/admin/*` routes.
- This is a single-operator personal tool (not multi-tenant): `job_search_profiles` is a singleton row with `id = 1`, same convention as `profiles`.
- No queue/worker infrastructure - scoring runs synchronously in the same request/scheduler tick that discovers new listings, matching the spec's stated low volume.
- Pure functions (HTML parsing, robots.txt parsing, JSON-LD/OG extraction, AI prompt-building) get unit tests with fixtures. DB-touching orchestration and HTTP handlers do not get unit tests - this matches the existing `gmail_sync.go` (no test file) / `gmail_sync_helpers_test.go` (pure helpers only) and `email_handler.go` (no test file) split.

---

### Task 1: Database schema and Go models

**Files:**
- Create: `backend/migrations/000032_add_job_search.up.sql`
- Create: `backend/migrations/000032_add_job_search.down.sql`
- Create: `backend/internal/models/job_search.go`

**Interfaces:**
- Produces: `models.JobSearchProfile{ID uint, CVText string, Skills string, Preferences string, UpdatedBy uint, UpdatedAt time.Time}`, `models.JobListing{ID uint, Site string, ExternalURL string, Title string, Company string, Location string, Description string, PostedAt *time.Time, ScrapedAt time.Time}`, `models.JobMatch{ID uint, JobListingID uint, JobListing JobListing, Score int, Reasoning string, Status string, AppliedAt *time.Time, ApplicationText *string, CreatedAt time.Time, UpdatedAt time.Time}` - every later Go task depends on these three types and their exact field names.

This task has no automated test (schema/model definitions have nothing to unit-test against) - verification is a container build plus a manual schema check, matching how `000030_add_profile` was verified.

- [ ] **Step 1: Verify migration number 000032 is still free**

Run: `ls backend/migrations | sort -V | tail -5`
Expected: highest existing number is `000031_add_draft_feedback`. If `000032` already exists, use the next free number and adjust every reference below accordingly.

- [ ] **Step 2: Write the migration**

`backend/migrations/000032_add_job_search.up.sql`:
```sql
-- backend/migrations/000032_add_job_search.up.sql

CREATE TABLE job_search_profiles (
    id SERIAL PRIMARY KEY,
    cv_text TEXT NOT NULL DEFAULT '',
    skills TEXT NOT NULL DEFAULT '',
    preferences TEXT NOT NULL DEFAULT '',
    updated_by INTEGER NOT NULL REFERENCES users(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO job_search_profiles (id, updated_by) VALUES (1, 1);

CREATE TABLE job_listings (
    id SERIAL PRIMARY KEY,
    site VARCHAR(50) NOT NULL,
    external_url TEXT NOT NULL,
    title VARCHAR(255) NOT NULL,
    company VARCHAR(255) NOT NULL,
    location VARCHAR(255) NOT NULL DEFAULT '',
    description TEXT NOT NULL,
    posted_at TIMESTAMPTZ,
    scraped_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (site, external_url)
);

CREATE TABLE job_matches (
    id SERIAL PRIMARY KEY,
    job_listing_id INTEGER NOT NULL UNIQUE REFERENCES job_listings(id) ON DELETE CASCADE,
    score INTEGER NOT NULL,
    reasoning TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'new',
    applied_at TIMESTAMPTZ,
    application_text TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_job_matches_score ON job_matches(score DESC);
```

`backend/migrations/000032_add_job_search.down.sql`:
```sql
-- backend/migrations/000032_add_job_search.down.sql

DROP TABLE job_matches;
DROP TABLE job_listings;
DROP TABLE job_search_profiles;
```

- [ ] **Step 3: Write the Go models**

`backend/internal/models/job_search.go`:
```go
// backend/internal/models/job_search.go
package models

import "time"

type JobSearchProfile struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	CVText      string    `json:"cv_text" gorm:"column:cv_text;type:text"`
	Skills      string    `json:"skills" gorm:"type:text"`
	Preferences string    `json:"preferences" gorm:"type:text"`
	UpdatedBy   uint      `json:"updated_by"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (JobSearchProfile) TableName() string { return "job_search_profiles" }

type JobListing struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	Site        string     `json:"site"`
	ExternalURL string     `json:"external_url" gorm:"column:external_url"`
	Title       string     `json:"title"`
	Company     string     `json:"company"`
	Location    string     `json:"location"`
	Description string     `json:"description" gorm:"type:text"`
	PostedAt    *time.Time `json:"posted_at"`
	ScrapedAt   time.Time  `json:"scraped_at"`
}

func (JobListing) TableName() string { return "job_listings" }

type JobMatch struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	JobListingID    uint       `json:"job_listing_id" gorm:"column:job_listing_id;uniqueIndex"`
	JobListing      JobListing `json:"job_listing" gorm:"foreignKey:JobListingID"`
	Score           int        `json:"score"`
	Reasoning       string     `json:"reasoning" gorm:"type:text"`
	Status          string     `json:"status"`
	AppliedAt       *time.Time `json:"applied_at"`
	ApplicationText *string    `json:"application_text" gorm:"type:text"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (JobMatch) TableName() string { return "job_matches" }
```

- [ ] **Step 4: Verify the backend builds**

Run: `docker exec devbridge_backend go build ./...`
Expected: no errors.

- [ ] **Step 5: Apply the migration and verify the schema**

Run: `docker restart devbridge_backend`
Then check the logs for a successful migration: `docker logs devbridge_backend --tail 50 | grep -i migrat`
Then verify the tables exist: `docker exec devbridge_postgres psql -U devbridge_user -d devbridge -c "\d job_listings" -c "\d job_matches" -c "\d job_search_profiles"`
Expected: all three tables listed with the columns from Step 2, and `SELECT * FROM job_search_profiles;` returns exactly one row with `id = 1`.

- [ ] **Step 6: Commit**

```bash
git add backend/migrations/000032_add_job_search.up.sql backend/migrations/000032_add_job_search.down.sql backend/internal/models/job_search.go
git commit -m "feat(job-search): add job_search_profiles/job_listings/job_matches schema and models"
```

---

### Task 2: jobscraper package core + robots.txt parser

**Files:**
- Create: `backend/internal/services/jobscraper/scraper.go`
- Create: `backend/internal/services/jobscraper/robots.go`
- Create: `backend/internal/services/jobscraper/robots_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `jobscraper.ScrapedJob{ExternalURL, Title, Company, Location, Description string, PostedAt *time.Time}`, `jobscraper.Scraper` interface (`Site() string`, `Scrape(ctx context.Context) ([]ScrapedJob, error)`), `jobscraper.userAgent` constant, `jobscraper.robotsAllowed(ctx context.Context, client *http.Client, rawURL string) (bool, error)` - Tasks 3 and 4 both call `robotsAllowed` and implement `Scraper`/return `ScrapedJob`.

- [ ] **Step 1: Write scraper.go (shared types)**

`backend/internal/services/jobscraper/scraper.go`:
```go
// backend/internal/services/jobscraper/scraper.go
package jobscraper

import (
	"context"
	"time"
)

// ScrapedJob is the common shape both the automated profession.hu scraper
// and the manual URL-add flow produce, so RunScrape and the manual-add
// handler can save either one the same way.
type ScrapedJob struct {
	ExternalURL string
	Title       string
	Company     string
	Location    string
	Description string
	PostedAt    *time.Time
}

// Scraper is implemented once per auto-scraped site. Only profession.hu
// implements it today (see the design doc's Scope section for why
// nofluffjobs.com/LinkedIn are manual-add only).
type Scraper interface {
	Site() string
	Scrape(ctx context.Context) ([]ScrapedJob, error)
}

// userAgent identifies this scraper to site operators, per the design doc's
// robots.txt-compliance requirement - an explicit, contactable identity
// rather than pretending to be a browser.
const userAgent = "DevBridgeManager-JobSearch/1.0 (personal use, contact: zoltan@oktatron.com)"
```

- [ ] **Step 2: Write the failing robots.txt parser tests**

`backend/internal/services/jobscraper/robots_test.go`:
```go
// backend/internal/services/jobscraper/robots_test.go
package jobscraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseDisallowedPathsOnlyWildcardBlock(t *testing.T) {
	body := strings.NewReader(`User-agent: Googlebot
Disallow: /only-for-google

User-agent: *
Disallow: /api/
Disallow: /admin/
`)
	got := parseDisallowedPaths(body)
	want := []string{"/api/", "/admin/"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestParseDisallowedPathsIgnoresComments(t *testing.T) {
	body := strings.NewReader(`# comment
User-agent: *
# another comment
Disallow: /private/
`)
	got := parseDisallowedPaths(body)
	if len(got) != 1 || got[0] != "/private/" {
		t.Fatalf("expected [/private/], got %v", got)
	}
}

func TestRobotsAllowedRespectsDisallow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Write([]byte("User-agent: *\nDisallow: /private/\n"))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()

	client := server.Client()

	allowed, err := robotsAllowed(context.Background(), client, server.URL+"/private/page")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatal("expected disallowed path to be rejected")
	}

	allowed, err = robotsAllowed(context.Background(), client, server.URL+"/public/page")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected non-disallowed path to be allowed")
	}
}

func TestRobotsAllowedTreatsMissingRobotsTxtAsAllowed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer server.Close()

	allowed, err := robotsAllowed(context.Background(), server.Client(), server.URL+"/anything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatal("expected missing robots.txt to be treated as allowed")
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `docker exec devbridge_backend go test ./internal/services/jobscraper/...`
Expected: FAIL - `undefined: parseDisallowedPaths`, `undefined: robotsAllowed`.

- [ ] **Step 4: Implement robots.go**

`backend/internal/services/jobscraper/robots.go`:
```go
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
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `docker exec devbridge_backend go test ./internal/services/jobscraper/... -run 'Robots|ParseDisallowed' -v`
Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/jobscraper/scraper.go backend/internal/services/jobscraper/robots.go backend/internal/services/jobscraper/robots_test.go
git commit -m "feat(job-search): add jobscraper package with ScrapedJob/Scraper types and robots.txt parser"
```

---

### Task 3: ProfessionHuScraper and parseListingPage

**Files:**
- Create: `backend/internal/services/jobscraper/professionhu.go`
- Create: `backend/internal/services/jobscraper/professionhu_test.go`
- Create: `backend/internal/services/jobscraper/testdata/professionhu_sample.html`

**Interfaces:**
- Consumes: `jobscraper.ScrapedJob`, `jobscraper.Scraper`, `jobscraper.userAgent`, `jobscraper.robotsAllowed` (Task 2).
- Produces: `jobscraper.NewProfessionHuScraper(knownURLs map[string]bool) *ProfessionHuScraper` (implements `Scraper`, `Site()` returns `"profession.hu"`) - Task 9's `RunScrape` constructs this and passes it into the shared `[]jobscraper.Scraper` slice.

- [ ] **Step 1: Add the goquery dependency**

Run: `docker exec devbridge_backend go get github.com/PuerkitoBio/goquery`
Expected: `go.mod`/`go.sum` updated with `github.com/PuerkitoBio/goquery` and its transitive dependencies (`golang.org/x/net`, `andybalholm/cascadia`).

- [ ] **Step 2: Create the fixture**

`backend/internal/services/jobscraper/testdata/professionhu_sample.html`:
```html
<!--
  Trimmed excerpt of a real profession.hu unfiltered listing page
  (https://www.profession.hu/allasok/1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1),
  saved 2026-09-24. Contains two consecutive, real job cards, kept verbatim
  so parseListingPage's unit test asserts against real site markup instead
  of a hand-written approximation. Everything outside the two
  dsx-job-card-basic divs (page chrome, scripts, other cards) was removed.
-->
<!DOCTYPE html>
<html lang="hu">
<head><meta charset="utf-8"><title>Állások, munkák és állásajánlatok - Profession.hu</title></head>
<body>
<ul class="dsx-job-cards-list">
<li class="advertisement-result-list-item" data-prof-id="3010682">
<div class="dsx-card dsx-job-card dsx-job-card-basic " data-theme="pds" data-theme-color="pds-light" data-card-id="detailed-job-card-3010682">
<div class="dsx-card__body">
<div class="dsx-card__content">
<div class="dsx-job-card-job-top-content">
<div class="ds-flex-column ds-gap-md ds-flex-1">
<div class="ds-flex ds-gap-8 ds-flex-wrap">
<div id="detailed-job-card-3010682-is-new-badge-badge" class="ds-badge ds-badge-blue-highlighted ds-badge-sm">
<div class="ds-badge-content">Új</div>
</div>
<div id="detailed-job-card-3010682-is-urgent-badge-badge" class="ds-badge ds-badge-neutral ds-badge-sm">
<div class="ds-badge-content">Tipp</div>
</div>
</div>
<h2 id="detailed-job-card-3010682-title-position" aria-label="Munkakör neve" class="ds-display- ds-heading-3 ds-pb-xs">
<a href="https://www.profession.hu/allas/motorkerekpar-szerelo-meteor-motortech-kft-budapest-3010682" title="Meteor Motortech Kft. Motorkerékpár szerelő állás, munka 1091 Budapest" class="ga-enhanced-event-click" target="_blank" data-item-id="3010682">
Motorkerékpár szerelő
</a>
</h2>
</div>
<div id="detailed-job-card-3010682-details-logo-wrapper" class="dsx-card-logo-wrapper">
<img id="detailed-job-card-3010682-details-logo-image" src="/static/assets/images/logos/default-logos/M.png" alt="Meteor Motortech Kft. karrier, állás és munka" title="Meteor Motortech Kft. karrier, állás és munka">
</div>
</div>
<ul class="dsx-job-card-job-details dsx-job-card-job-details-primary" aria-label="Állás részletei">
<div class="ds-flex ds-gap-sm ds-flex-wrap">
<li class="icon-width-text">
<a id="detailed-job-card-3010682-details-company-name" target="_blank" class="icon-width-text icon-width-text-flex-start" href="https://www.profession.hu/allasok/meteor-motortech-kft/1,0,0,0,0,0,0,0,0,0,106195" title="Meteor Motortech Kft. karrier, állás és munka">
<svg color="var(--icon-semantic-neutral)" aria-hidden="true" class="ds-icon"><use href="https://www.profession.hu/static/assets/images/untitled-ui-icons/untitled-ui-sprites/line-icons/building-04.svg#icon"></use></svg> <span class="details-text" aria-label="Hirdető cég">
Meteor Motortech Kft.
</span>
</a>
</li>
<li id="detailed-job-card-3010682-details-location" class="icon-width-text icon-width-text-flex-start" title="Budapest IX.kerület">
<svg color="var(--icon-semantic-neutral)" aria-hidden="true" class="ds-icon"><use href="https://www.profession.hu/static/assets/images/untitled-ui-icons/untitled-ui-sprites/line-icons/marker-pin-01.svg#icon"></use></svg> <span class="details-text" aria-label='Munkavégzés helye'><strong class="fw-inherit primary-details-location comma-break">Budapest IX.kerület</strong></span>
</li>
</div>
</ul>
<div id="detailed-job-card-3010682-more-details-container" class="collapse ds-w100">
<hr class="ds-hr-0">
<div id="detailed-job-card-3010682-details-task-list" class="details-text dsx-job-card-job-details dsx-job-card-job-details-task-list">
<div>
<div class="ds-body-2-bold">Főbb feladatok</div>
<ul>
<li>Brit motorkerékpárok szakszerű karbantartása, javítása és
szervizelése a gyári előírások szerint.</li>
<li>Új motorok összeszerelése, beüzemelése,</li>
<li>Időszakos átvizsgálások, olajcserék, futómű- és fékrendszer
ellenőrzések, elektromos diagnosztikák elvégzése.</li>
</ul>
</div>
</div>
<div class="ds-pt-md ds-flex ds-flex-justify-between ds-gap-8 ds-w100 ds-flex-align-end md:ds-flex-align-center">
<div class="ds-flex ds-gap-8 ds-w100 ds-flex-wrap">
<div id="detailed-job-card-3010682-details-start-date" class="dsx-job-card__date ds-mr-16 " aria-label="feladva">
<span>feladva: </span><strong class="fw-inherit">Friss</strong>
</div>
</div>
</div>
</div>
</div>
</div>
</div>
</li>
<li class="advertisement-result-list-item" data-prof-id="3008887">
<div class="dsx-card dsx-job-card dsx-job-card-basic " data-theme="pds" data-theme-color="pds-light" data-card-id="detailed-job-card-3008887">
<div class="dsx-card__body">
<div class="dsx-card__content">
<div class="dsx-job-card-job-top-content">
<div class="ds-flex-column ds-gap-md ds-flex-1">
<div class="ds-flex ds-gap-8 ds-flex-wrap">
<div id="detailed-job-card-3008887-is-new-badge-badge" class="ds-badge ds-badge-blue-highlighted ds-badge-sm">
<div class="ds-badge-content">Új</div>
</div>
</div>
<h2 id="detailed-job-card-3008887-title-position" aria-label="Munkakör neve" class="ds-display- ds-heading-3 ds-pb-xs">
<a href="https://www.profession.hu/allas/anyagkezelo-alapanyag-komissiozo-cellcomp-kft-celldomolk-3008887" title="CELLCOMP KFT. ANYAGKEZELŐ / ALAPANYAG KOMISSIÓZÓ állás, munka 9500 Celldömölk" class="ga-enhanced-event-click" target="_blank" data-item-id="3008887">
ANYAGKEZELŐ / ALAPANYAG KOMISSIÓZÓ
</a>
</h2>
</div>
<div id="detailed-job-card-3008887-details-logo-wrapper" class="dsx-card-logo-wrapper">
<img id="detailed-job-card-3008887-details-logo-image" src="/images/logos/thumb_list/2/3/23475_1484308168.jpg" alt="CELLCOMP KFT. karrier, állás és munka" title="CELLCOMP KFT. karrier, állás és munka">
</div>
</div>
<ul class="dsx-job-card-job-details dsx-job-card-job-details-primary" aria-label="Állás részletei">
<div class="ds-flex ds-gap-sm ds-flex-wrap">
<li class="icon-width-text">
<a id="detailed-job-card-3008887-details-company-name" target="_blank" class="icon-width-text icon-width-text-flex-start" href="https://www.profession.hu/allasok/cellcomp-kft/1,0,0,0,0,0,0,0,0,0,23475" title="CELLCOMP KFT. karrier, állás és munka">
<svg color="var(--icon-semantic-neutral)" aria-hidden="true" class="ds-icon"><use href="https://www.profession.hu/static/assets/images/untitled-ui-icons/untitled-ui-sprites/line-icons/building-04.svg#icon"></use></svg> <span class="details-text" aria-label="Hirdető cég">
CELLCOMP KFT.
</span>
</a>
</li>
<li id="detailed-job-card-3008887-details-location" class="icon-width-text icon-width-text-flex-start" title="Celldömölk">
<svg color="var(--icon-semantic-neutral)" aria-hidden="true" class="ds-icon"><use href="https://www.profession.hu/static/assets/images/untitled-ui-icons/untitled-ui-sprites/line-icons/marker-pin-01.svg#icon"></use></svg> <span class="details-text" aria-label='Munkavégzés helye'><strong class="fw-inherit primary-details-location comma-break">Celldömölk</strong></span>
</li>
</div>
</ul>
<div id="detailed-job-card-3008887-more-details-container" class="collapse ds-w100">
<hr class="ds-hr-0">
<div id="detailed-job-card-3008887-details-task-list" class="details-text dsx-job-card-job-details dsx-job-card-job-details-task-list">
<div>
<div class="ds-body-2-bold">Főbb feladatok</div>
<ul>
<li>Alapanyagok beérkeztetése, be- és kitárolása, komissiózás</li>
<li>Termelés alapanyaggal való kiszolgálása</li>
<li>Raktári automaták kezelése</li>
<li>Napi feladatok elvégzéséhez kapcsolódó adminisztráció</li>
<li>Gépi és Kézi anyagmozgatás</li>
</ul>
</div>
</div>
<div class="ds-pt-md ds-flex ds-flex-justify-between ds-gap-8 ds-w100 ds-flex-align-end md:ds-flex-align-center">
<div class="ds-flex ds-gap-8 ds-w100 ds-flex-wrap">
<div id="detailed-job-card-3008887-details-start-date" class="dsx-job-card__date ds-mr-16 " aria-label="feladva">
<span>feladva: </span><strong class="fw-inherit">Friss</strong>
</div>
</div>
</div>
</div>
</div>
</div>
</div>
</li>
</ul>
</body>
</html>
```

- [ ] **Step 3: Write the failing test**

`backend/internal/services/jobscraper/professionhu_test.go`:
```go
// backend/internal/services/jobscraper/professionhu_test.go
package jobscraper

import (
	"os"
	"strings"
	"testing"
)

func TestParseListingPageExtractsRealJobs(t *testing.T) {
	html, err := os.ReadFile("testdata/professionhu_sample.html")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	jobs, err := parseListingPage(string(html))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}

	first := jobs[0]
	if first.Title != "Motorkerékpár szerelő" {
		t.Errorf("unexpected title: %q", first.Title)
	}
	if first.ExternalURL != "https://www.profession.hu/allas/motorkerekpar-szerelo-meteor-motortech-kft-budapest-3010682" {
		t.Errorf("unexpected url: %q", first.ExternalURL)
	}
	if first.Company != "Meteor Motortech Kft." {
		t.Errorf("unexpected company: %q", first.Company)
	}
	if first.Location != "Budapest IX.kerület" {
		t.Errorf("unexpected location: %q", first.Location)
	}
	if !strings.Contains(first.Description, "Brit motorkerékpárok szakszerű karbantartása") {
		t.Errorf("expected description to include task bullet, got: %q", first.Description)
	}

	second := jobs[1]
	if second.Title != "ANYAGKEZELŐ / ALAPANYAG KOMISSIÓZÓ" {
		t.Errorf("unexpected title: %q", second.Title)
	}
	if second.Company != "CELLCOMP KFT." {
		t.Errorf("unexpected company: %q", second.Company)
	}
	if second.Location != "Celldömölk" {
		t.Errorf("unexpected location: %q", second.Location)
	}
}

func TestParseListingPageReturnsEmptyForNoCards(t *testing.T) {
	jobs, err := parseListingPage("<html><body>no cards here</body></html>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected no jobs, got %d", len(jobs))
	}
}
```

- [ ] **Step 4: Run the test to verify it fails**

Run: `docker exec devbridge_backend go test ./internal/services/jobscraper/... -run ParseListingPage`
Expected: FAIL - `undefined: parseListingPage`.

- [ ] **Step 5: Implement professionhu.go**

`backend/internal/services/jobscraper/professionhu.go`:
```go
// backend/internal/services/jobscraper/professionhu.go
package jobscraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// professionHuListingURLPrefix reproduces the exact query-string shape
// profession.hu uses for its unfiltered, all-category listing: "1," then
// 36 "0," category slots, then the page number. Verified by hand against
// https://www.profession.hu/allasok/1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1
var professionHuListingURLPrefix = "1," + strings.Repeat("0,", 36)

const professionHuMaxPages = 10
const professionHuRequestDelay = 1500 * time.Millisecond

type ProfessionHuScraper struct {
	httpClient *http.Client
	knownURLs  map[string]bool
}

// NewProfessionHuScraper takes the (site, external_url) pairs already
// stored for this site so Scrape can stop paginating once it stops seeing
// new ads - a pagination optimization only. True dedup happens at the
// (site, external_url) DB unique constraint regardless of this map.
func NewProfessionHuScraper(knownURLs map[string]bool) *ProfessionHuScraper {
	return &ProfessionHuScraper{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		knownURLs:  knownURLs,
	}
}

func (s *ProfessionHuScraper) Site() string { return "profession.hu" }

func (s *ProfessionHuScraper) Scrape(ctx context.Context) ([]ScrapedJob, error) {
	allowed, err := robotsAllowed(ctx, s.httpClient, "https://www.profession.hu/allasok")
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, fmt.Errorf("profession.hu robots.txt disallows /allasok")
	}

	var jobs []ScrapedJob
	for page := 1; page <= professionHuMaxPages; page++ {
		if page > 1 {
			time.Sleep(professionHuRequestDelay)
		}

		html, err := fetchListingPage(ctx, s.httpClient, page)
		if err != nil {
			return jobs, fmt.Errorf("fetching page %d: %w", page, err)
		}

		pageJobs, err := parseListingPage(html)
		if err != nil {
			return jobs, fmt.Errorf("parsing page %d: %w", page, err)
		}
		if len(pageJobs) == 0 {
			break
		}

		allKnown := true
		for _, job := range pageJobs {
			jobs = append(jobs, job)
			if !s.knownURLs[job.ExternalURL] {
				allKnown = false
			}
		}
		if allKnown {
			break
		}
	}
	return jobs, nil
}

func fetchListingPage(ctx context.Context, client *http.Client, page int) (string, error) {
	trimmedPrefix := strings.TrimSuffix(professionHuListingURLPrefix, ",")
	listingURL := fmt.Sprintf("https://www.profession.hu/allasok/%s,%d", trimmedPrefix, page)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listingURL, nil)
	if err != nil {
		return "", fmt.Errorf("building listing request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching listing page: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading listing page: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("listing page returned %d", resp.StatusCode)
	}
	return string(body), nil
}

// parseListingPage is pure and network-free so it can be unit-tested
// against the saved fixture instead of a live fetch.
func parseListingPage(html string) ([]ScrapedJob, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parsing listing html: %w", err)
	}

	var jobs []ScrapedJob
	doc.Find("div.dsx-job-card-basic").Each(func(_ int, card *goquery.Selection) {
		titleLink := card.Find(`h2[id$="-title-position"] a`).First()
		title := strings.TrimSpace(titleLink.Text())
		url, _ := titleLink.Attr("href")
		if title == "" || url == "" {
			return
		}

		company := strings.TrimSpace(card.Find(`a[id$="-details-company-name"] span.details-text`).First().Text())
		location := strings.TrimSpace(card.Find(`li[id$="-details-location"] strong.primary-details-location`).First().Text())

		var descParts []string
		card.Find(`div[id$="-details-task-list"] li`).Each(func(_ int, li *goquery.Selection) {
			text := strings.TrimSpace(li.Text())
			if text != "" {
				descParts = append(descParts, text)
			}
		})

		jobs = append(jobs, ScrapedJob{
			ExternalURL: url,
			Title:       title,
			Company:     company,
			Location:    location,
			Description: strings.Join(descParts, "\n"),
		})
	})
	return jobs, nil
}
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `docker exec devbridge_backend go test ./internal/services/jobscraper/... -v`
Expected: PASS (all tests in the package, including Task 2's).

- [ ] **Step 7: Commit**

```bash
git add backend/go.mod backend/go.sum backend/internal/services/jobscraper/professionhu.go backend/internal/services/jobscraper/professionhu_test.go backend/internal/services/jobscraper/testdata/professionhu_sample.html
git commit -m "feat(job-search): add ProfessionHuScraper with paginated, robots.txt-respecting scraping"
```

---

### Task 4: Manual URL-add fetch (JSON-LD + Open Graph extraction)

**Files:**
- Create: `backend/internal/services/jobscraper/manual_fetch.go`
- Create: `backend/internal/services/jobscraper/manual_fetch_test.go`
- Create: `backend/internal/services/jobscraper/testdata/manual_jobposting_ldjson.html`
- Create: `backend/internal/services/jobscraper/testdata/manual_og_only.html`
- Create: `backend/internal/services/jobscraper/testdata/manual_no_data.html`

**Interfaces:**
- Consumes: `jobscraper.ScrapedJob`, `jobscraper.userAgent`, `jobscraper.robotsAllowed` (Task 2).
- Produces: `jobscraper.ManualFetchResult{Job ScrapedJob, Extracted bool}`, `jobscraper.FetchJobFromURL(ctx context.Context, rawURL string) (ManualFetchResult, error)` - Task 11's manual-add handler calls this directly.

- [ ] **Step 1: Create the fixtures**

`backend/internal/services/jobscraper/testdata/manual_jobposting_ldjson.html`:
```html
<!DOCTYPE html>
<html><head>
<meta property="og:title" content="Should not be used - JSON-LD takes priority">
<script type="application/ld+json">
{
  "@context": "https://schema.org/",
  "@type": "JobPosting",
  "title": "Senior Backend Engineer",
  "description": "Build and maintain our core Go services.",
  "hiringOrganization": { "@type": "Organization", "name": "Acme Corp" },
  "jobLocation": {
    "@type": "Place",
    "address": { "@type": "PostalAddress", "addressLocality": "Budapest", "addressRegion": "Hungary" }
  }
}
</script>
</head><body></body></html>
```

`backend/internal/services/jobscraper/testdata/manual_og_only.html`:
```html
<!DOCTYPE html>
<html><head>
<meta property="og:title" content="Azure Cloud Infrastructure Engineer">
<meta property="og:description" content="Remote role managing Azure infrastructure for a distributed team.">
<meta property="og:site_name" content="Square One Resources">
</head><body></body></html>
```

`backend/internal/services/jobscraper/testdata/manual_no_data.html`:
```html
<!DOCTYPE html>
<html><head><title>Just a plain page</title></head><body><p>No structured job data here.</p></body></html>
```

- [ ] **Step 2: Write the failing tests**

`backend/internal/services/jobscraper/manual_fetch_test.go`:
```go
// backend/internal/services/jobscraper/manual_fetch_test.go
package jobscraper

import (
	"os"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return string(data)
}

func TestExtractJobPostingPrefersJSONLD(t *testing.T) {
	html := readFixture(t, "manual_jobposting_ldjson.html")
	job, ok := extractJobPosting(html, "https://example.com/job/123")
	if !ok {
		t.Fatal("expected extraction to succeed")
	}
	if job.Title != "Senior Backend Engineer" {
		t.Errorf("unexpected title: %q", job.Title)
	}
	if job.Company != "Acme Corp" {
		t.Errorf("unexpected company: %q", job.Company)
	}
	if job.Location != "Budapest, Hungary" {
		t.Errorf("unexpected location: %q", job.Location)
	}
	if job.Description != "Build and maintain our core Go services." {
		t.Errorf("unexpected description: %q", job.Description)
	}
	if job.ExternalURL != "https://example.com/job/123" {
		t.Errorf("unexpected url: %q", job.ExternalURL)
	}
}

func TestExtractJobPostingFallsBackToOpenGraph(t *testing.T) {
	html := readFixture(t, "manual_og_only.html")
	job, ok := extractJobPosting(html, "https://nofluffjobs.com/job/example")
	if !ok {
		t.Fatal("expected extraction to succeed via OG fallback")
	}
	if job.Title != "Azure Cloud Infrastructure Engineer" {
		t.Errorf("unexpected title: %q", job.Title)
	}
	if job.Company != "Square One Resources" {
		t.Errorf("unexpected company: %q", job.Company)
	}
	if job.Description == "" {
		t.Error("expected non-empty description")
	}
}

func TestExtractJobPostingReturnsNotExtractedWhenNoData(t *testing.T) {
	html := readFixture(t, "manual_no_data.html")
	_, ok := extractJobPosting(html, "https://example.com/nothing")
	if ok {
		t.Fatal("expected extraction to fail (Extracted == false), not an error")
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `docker exec devbridge_backend go test ./internal/services/jobscraper/... -run ExtractJobPosting`
Expected: FAIL - `undefined: extractJobPosting`.

- [ ] **Step 4: Implement manual_fetch.go**

`backend/internal/services/jobscraper/manual_fetch.go`:
```go
// backend/internal/services/jobscraper/manual_fetch.go
package jobscraper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ManualFetchResult is FetchJobFromURL's return value. Extracted is false
// (not an error) when neither JSON-LD nor Open Graph tags gave enough data -
// the caller then falls back to asking the operator for the fields by hand.
type ManualFetchResult struct {
	Job       ScrapedJob
	Extracted bool
}

type jsonLDJobPosting struct {
	Type               string `json:"@type"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	HiringOrganization struct {
		Name string `json:"name"`
	} `json:"hiringOrganization"`
	JobLocation struct {
		Address struct {
			AddressLocality string `json:"addressLocality"`
			AddressRegion   string `json:"addressRegion"`
		} `json:"address"`
	} `json:"jobLocation"`
}

// FetchJobFromURL is a single, explicit, one-off fetch triggered by an
// admin action - no pagination, no repeated scheduling, unlike Scrape.
func FetchJobFromURL(ctx context.Context, rawURL string) (ManualFetchResult, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	allowed, err := robotsAllowed(ctx, client, rawURL)
	if err != nil {
		return ManualFetchResult{}, err
	}
	if !allowed {
		return ManualFetchResult{}, fmt.Errorf("robots.txt disallows fetching %s", rawURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ManualFetchResult{}, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return ManualFetchResult{}, fmt.Errorf("fetching url: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ManualFetchResult{}, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return ManualFetchResult{}, fmt.Errorf("url returned %d", resp.StatusCode)
	}

	job, extracted := extractJobPosting(string(body), rawURL)
	return ManualFetchResult{Job: job, Extracted: extracted}, nil
}

// extractJobPosting is pure so it can be tested against saved fixtures
// instead of a live fetch - JSON-LD/Open Graph are stable public standards,
// so synthetic fixtures are as reliable as live-fetched ones here.
func extractJobPosting(html, sourceURL string) (ScrapedJob, bool) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return ScrapedJob{}, false
	}

	if job, ok := extractFromJSONLD(doc, sourceURL); ok {
		return job, true
	}
	return extractFromOpenGraph(doc, sourceURL)
}

func extractFromJSONLD(doc *goquery.Document, sourceURL string) (ScrapedJob, bool) {
	var found ScrapedJob
	var ok bool
	doc.Find(`script[type="application/ld+json"]`).EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		var posting jsonLDJobPosting
		if err := json.Unmarshal([]byte(sel.Text()), &posting); err != nil {
			return true // keep looking at other script blocks
		}
		if posting.Type != "JobPosting" || posting.Title == "" {
			return true
		}
		location := strings.Trim(strings.TrimSpace(strings.Join([]string{
			posting.JobLocation.Address.AddressLocality,
			posting.JobLocation.Address.AddressRegion,
		}, ", ")), ", ")
		found = ScrapedJob{
			ExternalURL: sourceURL,
			Title:       posting.Title,
			Company:     posting.HiringOrganization.Name,
			Location:    location,
			Description: posting.Description,
		}
		ok = true
		return false // stop: found a usable JobPosting
	})
	return found, ok
}

func extractFromOpenGraph(doc *goquery.Document, sourceURL string) (ScrapedJob, bool) {
	title := ogContent(doc, "og:title")
	description := ogContent(doc, "og:description")
	if title == "" || description == "" {
		return ScrapedJob{}, false
	}
	return ScrapedJob{
		ExternalURL: sourceURL,
		Title:       title,
		Company:     ogContent(doc, "og:site_name"),
		Description: description,
	}, true
}

func ogContent(doc *goquery.Document, property string) string {
	content, _ := doc.Find(fmt.Sprintf(`meta[property="%s"]`, property)).First().Attr("content")
	return strings.TrimSpace(content)
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `docker exec devbridge_backend go test ./internal/services/jobscraper/... -v`
Expected: PASS (all tests in the package).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/jobscraper/manual_fetch.go backend/internal/services/jobscraper/manual_fetch_test.go backend/internal/services/jobscraper/testdata/manual_jobposting_ldjson.html backend/internal/services/jobscraper/testdata/manual_og_only.html backend/internal/services/jobscraper/testdata/manual_no_data.html
git commit -m "feat(job-search): add manual URL-add fetch with JSON-LD/Open Graph extraction"
```

---

### Task 5: AI service - job_match.py

**Files:**
- Create: `ai/app/job_match.py`
- Create: `ai/tests/test_job_match.py`
- Modify: `ai/app/main.py`

**Interfaces:**
- Produces: `JobMatchRequest(cv_text, skills, preferences="", job_title, company, location, job_description)`, `JobMatchResult(score: int, reasoning: str)`, `async def job_match(req: JobMatchRequest) -> JobMatchResult`, endpoint `POST /job-match` - Task 7's Go `JobMatchService` calls this endpoint with this exact request/response shape.

- [ ] **Step 1: Write the failing test**

`ai/tests/test_job_match.py`:
```python
from fastapi.testclient import TestClient
from pydantic_ai.models.test import TestModel

from app.main import app
from app.job_match import JobMatchRequest, _build_prompt, job_match_agent

client = TestClient(app)


def test_job_match_returns_score_and_reasoning():
    with job_match_agent.override(model=TestModel(custom_output_args={"score": 85, "reasoning": "Erős egyezés a Go tapasztalat miatt."})):
        response = client.post(
            "/job-match",
            json={
                "cv_text": "5 év Go fejlesztői tapasztalat.",
                "skills": "Go, PostgreSQL, Docker",
                "job_title": "Senior Backend Engineer",
                "company": "Acme Corp",
                "location": "Budapest",
                "job_description": "Go szolgáltatások fejlesztése és karbantartása.",
            },
        )
    assert response.status_code == 200
    body = response.json()
    assert body["score"] == 85
    assert body["reasoning"] == "Erős egyezés a Go tapasztalat miatt."


def test_job_match_requires_job_fields():
    response = client.post("/job-match", json={"cv_text": "x", "skills": "y"})
    assert response.status_code == 422


def test_build_prompt_includes_preferences_when_present():
    req = JobMatchRequest(
        cv_text="CV szöveg",
        skills="Go",
        preferences="Csak távmunka",
        job_title="Fejlesztő",
        company="Acme",
        location="Budapest",
        job_description="Leírás",
    )
    prompt = _build_prompt(req)
    assert "Csak távmunka" in prompt


def test_build_prompt_omits_preferences_section_when_empty():
    req = JobMatchRequest(
        cv_text="CV szöveg",
        skills="Go",
        job_title="Fejlesztő",
        company="Acme",
        location="Budapest",
        job_description="Leírás",
    )
    prompt = _build_prompt(req)
    assert "Preferenciák:" not in prompt
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `docker exec devbridge_ai pytest tests/test_job_match.py -v`
Expected: FAIL - `ModuleNotFoundError: No module named 'app.job_match'`.

- [ ] **Step 3: Implement job_match.py**

`ai/app/job_match.py`:
```python
import os

from pydantic import BaseModel
from pydantic_ai import Agent

# Read GEMINI_MODEL directly in this module rather than importing it from
# another agent module, to avoid a circular import - same reasoning as
# categorize_email.py/draft_reply.py/task_breakdown.py.
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")


class JobMatchRequest(BaseModel):
    cv_text: str
    skills: str
    preferences: str = ""
    job_title: str
    company: str
    location: str
    job_description: str


class JobMatchResult(BaseModel):
    score: int
    reasoning: str


job_match_agent = Agent(
    f"google:{GEMINI_MODEL}",
    output_type=JobMatchResult,
    defer_model_check=True,
    instructions=(
        "Egy álláskereső CV-je, készségei és (opcionálisan) preferenciái "
        "alapján pontozd 0-100 skálán, mennyire illik rá az adott állás. "
        "0-20: nem releváns. 40-60: részleges egyezés. 80-100: erős egyezés. "
        "Az indoklás legyen 1-2 mondat, magyar nyelven."
    ),
)


def _build_prompt(req: JobMatchRequest) -> str:
    preferences_section = f"\nPreferenciák: {req.preferences}" if req.preferences.strip() else ""
    return (
        f"CV: {req.cv_text}\n"
        f"Készségek: {req.skills}"
        f"{preferences_section}\n\n"
        f"Állás: {req.job_title} - {req.company} ({req.location})\n"
        f"Leírás: {req.job_description}"
    )


async def job_match(req: JobMatchRequest) -> JobMatchResult:
    result = await job_match_agent.run(_build_prompt(req))
    return result.output
```

- [ ] **Step 4: Register the endpoint in main.py**

In `ai/app/main.py`, add the import alongside the existing ones:
```python
from app.job_match import JobMatchRequest, JobMatchResult, job_match
```

And add the endpoint after `task_breakdown_endpoint`:
```python
@app.post("/job-match", response_model=JobMatchResult)
async def job_match_endpoint(payload: JobMatchRequest) -> JobMatchResult:
    return await job_match(payload)
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `docker exec devbridge_ai pytest tests/test_job_match.py -v`
Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add ai/app/job_match.py ai/tests/test_job_match.py ai/app/main.py
git commit -m "feat(job-search): add /job-match AI endpoint"
```

---

### Task 6: AI service - job_application_draft.py

**Files:**
- Create: `ai/app/job_application_draft.py`
- Create: `ai/tests/test_job_application_draft.py`
- Modify: `ai/app/main.py`

**Interfaces:**
- Produces: `JobApplicationDraftRequest(cv_text, skills, job_title, company, job_description, instruction="")`, `JobApplicationDraftResult(draft: str)`, `async def job_application_draft(req) -> str`, endpoint `POST /job-application-draft` - Task 8's Go `JobApplicationDraftService` calls this endpoint with this exact request/response shape.

- [ ] **Step 1: Write the failing test**

`ai/tests/test_job_application_draft.py`:
```python
from fastapi.testclient import TestClient
from pydantic_ai.models.test import TestModel

from app.main import app
from app.job_application_draft import JobApplicationDraftRequest, _build_prompt, job_application_draft_agent

client = TestClient(app)


def test_job_application_draft_returns_generated_draft():
    with job_application_draft_agent.override(model=TestModel(custom_output_args={"draft": "Tisztelt Cím! ..."})):
        response = client.post(
            "/job-application-draft",
            json={
                "cv_text": "5 év Go fejlesztői tapasztalat.",
                "skills": "Go, PostgreSQL, Docker",
                "job_title": "Senior Backend Engineer",
                "company": "Acme Corp",
                "job_description": "Go szolgáltatások fejlesztése és karbantartása.",
            },
        )
    assert response.status_code == 200
    assert response.json()["draft"] == "Tisztelt Cím! ..."


def test_job_application_draft_requires_job_fields():
    response = client.post("/job-application-draft", json={"cv_text": "x", "skills": "y"})
    assert response.status_code == 422


def test_build_prompt_includes_instruction_when_present():
    req = JobApplicationDraftRequest(
        cv_text="CV szöveg",
        skills="Go",
        job_title="Fejlesztő",
        company="Acme",
        job_description="Leírás",
        instruction="Emeld ki a Go tapasztalatot",
    )
    prompt = _build_prompt(req)
    assert "Emeld ki a Go tapasztalatot" in prompt


def test_build_prompt_omits_instruction_section_when_empty():
    req = JobApplicationDraftRequest(
        cv_text="CV szöveg",
        skills="Go",
        job_title="Fejlesztő",
        company="Acme",
        job_description="Leírás",
    )
    prompt = _build_prompt(req)
    assert "Kiemelendő szempont" not in prompt
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `docker exec devbridge_ai pytest tests/test_job_application_draft.py -v`
Expected: FAIL - `ModuleNotFoundError: No module named 'app.job_application_draft'`.

- [ ] **Step 3: Implement job_application_draft.py**

`ai/app/job_application_draft.py`:
```python
import os

from pydantic import BaseModel
from pydantic_ai import Agent

# Read GEMINI_MODEL directly in this module rather than importing it from
# another agent module, to avoid a circular import - same reasoning as
# categorize_email.py/draft_reply.py/task_breakdown.py/job_match.py.
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")


class JobApplicationDraftRequest(BaseModel):
    cv_text: str
    skills: str
    job_title: str
    company: str
    job_description: str
    instruction: str = ""


class JobApplicationDraftResult(BaseModel):
    draft: str


job_application_draft_agent = Agent(
    f"google:{GEMINI_MODEL}",
    output_type=JobApplicationDraftResult,
    defer_model_check=True,
    instructions=(
        "Írj egy udvarias, magyar nyelvű, kész (nem placeholder) motivációs "
        "levelet egy állásjelentkezéshez, a megadott CV, készségek és "
        "álláshirdetés alapján. Emeld ki a hirdetés szempontjából releváns "
        "tapasztalatot. Ha kifejezett kiemelendő szempont van megadva, azt "
        "kezeld explicit hangsúlyként a levélben."
    ),
)


def _build_prompt(req: JobApplicationDraftRequest) -> str:
    instruction_section = f"\nKiemelendő szempont: {req.instruction}" if req.instruction.strip() else ""
    return (
        f"CV: {req.cv_text}\n"
        f"Készségek: {req.skills}"
        f"{instruction_section}\n\n"
        f"Állás: {req.job_title} - {req.company}\n"
        f"Leírás: {req.job_description}"
    )


async def job_application_draft(req: JobApplicationDraftRequest) -> str:
    result = await job_application_draft_agent.run(_build_prompt(req))
    return result.output.draft
```

- [ ] **Step 4: Register the endpoint in main.py**

In `ai/app/main.py`, add the import:
```python
from app.job_application_draft import JobApplicationDraftRequest, JobApplicationDraftResult, job_application_draft
```

And add the endpoint after `job_match_endpoint`:
```python
@app.post("/job-application-draft", response_model=JobApplicationDraftResult)
async def job_application_draft_endpoint(payload: JobApplicationDraftRequest) -> JobApplicationDraftResult:
    draft = await job_application_draft(payload)
    return JobApplicationDraftResult(draft=draft)
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `docker exec devbridge_ai pytest tests/test_job_application_draft.py -v`
Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add ai/app/job_application_draft.py ai/tests/test_job_application_draft.py ai/app/main.py
git commit -m "feat(job-search): add /job-application-draft AI endpoint"
```

---

### Task 7: Go JobMatchService (AI-service client)

**Files:**
- Create: `backend/internal/services/job_match.go`
- Create: `backend/internal/services/job_match_test.go`

**Interfaces:**
- Consumes: AI service's `POST /job-match` (Task 5) - `{cv_text, skills, preferences, job_title, company, location, job_description}` -> `{score, reasoning}`.
- Produces: `services.JobMatcher` interface (`MatchJob(ctx, cvText, skills, preferences, jobTitle, company, location, jobDescription string) (score int, reasoning string, err error)`), `services.NewJobMatchService() *JobMatchService` - Task 9's `RunScrape` and Task 12's handler both take a `JobMatcher`.

- [ ] **Step 1: Write the failing test**

`backend/internal/services/job_match_test.go`:
```go
// backend/internal/services/job_match_test.go
package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJobMatchSendsFieldsAndReturnsScore(t *testing.T) {
	var gotBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		if r.URL.Path != "/job-match" {
			t.Errorf("expected path /job-match, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"score": 85, "reasoning": "Erős egyezés."})
	}))
	defer server.Close()

	svc := &JobMatchService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	score, reasoning, err := svc.MatchJob(context.Background(), "cv", "skills", "prefs", "title", "company", "location", "description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 85 || reasoning != "Erős egyezés." {
		t.Fatalf("unexpected result: score=%d reasoning=%q", score, reasoning)
	}
	if gotBody["job_title"] != "title" || gotBody["company"] != "company" || gotBody["location"] != "location" {
		t.Fatalf("request body missing expected fields: %+v", gotBody)
	}
}

func TestJobMatchReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"detail":"boom"}`))
	}))
	defer server.Close()

	svc := &JobMatchService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	_, _, err := svc.MatchJob(context.Background(), "cv", "skills", "", "title", "company", "location", "description")
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestJobMatch`
Expected: FAIL - `undefined: JobMatchService`.

- [ ] **Step 3: Implement job_match.go**

`backend/internal/services/job_match.go`:
```go
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
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestJobMatch -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/job_match.go backend/internal/services/job_match_test.go
git commit -m "feat(job-search): add Go JobMatchService AI-service client"
```

---

### Task 8: Go JobApplicationDraftService (AI-service client)

**Files:**
- Create: `backend/internal/services/job_application_draft.go`
- Create: `backend/internal/services/job_application_draft_test.go`

**Interfaces:**
- Consumes: AI service's `POST /job-application-draft` (Task 6) - `{cv_text, skills, job_title, company, job_description, instruction}` -> `{draft}`.
- Produces: `services.JobApplicationDrafter` interface (`DraftApplication(ctx, cvText, skills, jobTitle, company, jobDescription, instruction string) (string, error)`), `services.NewJobApplicationDraftService() *JobApplicationDraftService` - Task 12's handler takes a `JobApplicationDrafter`.

- [ ] **Step 1: Write the failing test**

`backend/internal/services/job_application_draft_test.go`:
```go
// backend/internal/services/job_application_draft_test.go
package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJobApplicationDraftSendsFieldsAndReturnsDraft(t *testing.T) {
	var gotBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		if r.URL.Path != "/job-application-draft" {
			t.Errorf("expected path /job-application-draft, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"draft": "Tisztelt Cím!"})
	}))
	defer server.Close()

	svc := &JobApplicationDraftService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	draft, err := svc.DraftApplication(context.Background(), "cv", "skills", "title", "company", "description", "emeld ki a Go tapasztalatot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if draft != "Tisztelt Cím!" {
		t.Fatalf("unexpected draft: %q", draft)
	}
	if gotBody["instruction"] != "emeld ki a Go tapasztalatot" {
		t.Fatalf("request body missing expected instruction: %+v", gotBody)
	}
}

func TestJobApplicationDraftReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"detail":"boom"}`))
	}))
	defer server.Close()

	svc := &JobApplicationDraftService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	_, err := svc.DraftApplication(context.Background(), "cv", "skills", "title", "company", "description", "")
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestJobApplicationDraft`
Expected: FAIL - `undefined: JobApplicationDraftService`.

- [ ] **Step 3: Implement job_application_draft.go**

`backend/internal/services/job_application_draft.go`:
```go
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
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestJobApplicationDraft -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/job_application_draft.go backend/internal/services/job_application_draft_test.go
git commit -m "feat(job-search): add Go JobApplicationDraftService AI-service client"
```

---

### Task 9: RunScrape orchestrator, scheduler, and main.go wiring

**Files:**
- Create: `backend/internal/services/job_scraping.go`
- Create: `backend/internal/services/job_scraping_scheduler.go`
- Modify: `backend/cmd/server/main.go`

**Interfaces:**
- Consumes: `jobscraper.Scraper`, `jobscraper.NewProfessionHuScraper` (Task 3), `services.JobMatcher`/`NewJobMatchService` (Task 7), `models.JobListing`/`JobSearchProfile`/`JobMatch` (Task 1).
- Produces: `services.RunScrape(ctx context.Context, matcher JobMatcher) (newListings int, newMatches int)` - Task 10's `ScanNow` handler and this task's own scheduler both call it.

This is DB-touching orchestration with no automated test, matching `gmail_sync.go`'s `RunGmailSync` (no test file). Verification is a container build plus a manual scan-now-equivalent check once Task 10 exposes it over HTTP - for this task alone, a build check and a manual `go vet` are sufficient.

- [ ] **Step 1: Implement job_scraping.go**

`backend/internal/services/job_scraping.go`:
```go
// backend/internal/services/job_scraping.go
package services

import (
	"context"
	"log"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services/jobscraper"

	"gorm.io/gorm"
)

// RunScrape runs every registered scraper, saves any new listings (dedup
// via the (site, external_url) unique constraint), then scores every
// listing that doesn't have a match yet. Mirrors RunGmailSync's per-item
// error tolerance: one scraper (or one match) failing is logged and
// skipped, the rest of the run continues.
func RunScrape(ctx context.Context, matcher JobMatcher) (newListings int, newMatches int) {
	db := database.GetDB()
	scrapers := registeredScrapers(db)

	for _, scraper := range scrapers {
		jobs, err := scraper.Scrape(ctx)
		if err != nil {
			log.Printf("job scraping: %s failed: %v", scraper.Site(), err)
			continue
		}
		for _, job := range jobs {
			result := db.Exec(`
				INSERT INTO job_listings (site, external_url, title, company, location, description, posted_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (site, external_url) DO NOTHING
			`, scraper.Site(), job.ExternalURL, job.Title, job.Company, job.Location, job.Description, job.PostedAt)
			if result.Error != nil {
				log.Printf("job scraping: failed to save listing %s: %v", job.ExternalURL, result.Error)
				continue
			}
			if result.RowsAffected > 0 {
				newListings++
			}
		}
	}

	newMatches = scoreUnmatchedListings(ctx, db, matcher)
	return newListings, newMatches
}

// registeredScrapers builds the one scraper this feature currently
// supports, pre-loaded with the (site, external_url) pairs already in
// job_listings so ProfessionHuScraper.Scrape can stop paging once it stops
// seeing new ads.
func registeredScrapers(db *gorm.DB) []jobscraper.Scraper {
	var professionHuURLs []string
	db.Model(&models.JobListing{}).Where("site = ?", "profession.hu").Pluck("external_url", &professionHuURLs)
	known := make(map[string]bool, len(professionHuURLs))
	for _, u := range professionHuURLs {
		known[u] = true
	}
	return []jobscraper.Scraper{jobscraper.NewProfessionHuScraper(known)}
}

func scoreUnmatchedListings(ctx context.Context, db *gorm.DB, matcher JobMatcher) int {
	var profile models.JobSearchProfile
	if err := db.First(&profile, 1).Error; err != nil {
		log.Printf("job scraping: failed to load job search profile: %v", err)
		return 0
	}

	var listings []models.JobListing
	if err := db.Joins("LEFT JOIN job_matches ON job_matches.job_listing_id = job_listings.id").
		Where("job_matches.id IS NULL").Find(&listings).Error; err != nil {
		log.Printf("job scraping: failed to load unmatched listings: %v", err)
		return 0
	}

	created := 0
	for _, listing := range listings {
		score, reasoning, err := matcher.MatchJob(ctx, profile.CVText, profile.Skills, profile.Preferences, listing.Title, listing.Company, listing.Location, listing.Description)
		if err != nil {
			log.Printf("job scraping: failed to score listing %d: %v", listing.ID, err)
			continue
		}
		if err := db.Create(&models.JobMatch{
			JobListingID: listing.ID,
			Score:        score,
			Reasoning:    reasoning,
			Status:       "new",
		}).Error; err != nil {
			log.Printf("job scraping: failed to save match for listing %d: %v", listing.ID, err)
			continue
		}
		created++
	}
	return created
}
```

- [ ] **Step 2: Implement job_scraping_scheduler.go**

`backend/internal/services/job_scraping_scheduler.go`:
```go
// backend/internal/services/job_scraping_scheduler.go
package services

import (
	"context"
	"time"
)

const jobScrapingInterval = 24 * time.Hour

// StartJobScrapingScheduler mirrors StartGmailSyncScheduler's plain
// time.Ticker pattern: no cron dependency exists in this codebase, and a
// daily cadence matches the design doc's stated scan frequency.
func StartJobScrapingScheduler() {
	matcher := NewJobMatchService()
	RunScrape(context.Background(), matcher)

	ticker := time.NewTicker(jobScrapingInterval)
	for range ticker.C {
		RunScrape(context.Background(), matcher)
	}
}
```

- [ ] **Step 3: Wire the scheduler into main.go**

In `backend/cmd/server/main.go`, add after the existing `go services.StartGmailSyncScheduler()` line:
```go
	// Job search scraping/scoring, once at startup then every 24h (see
	// services.RunScrape)
	go services.StartJobScrapingScheduler()
```

- [ ] **Step 4: Verify the backend builds**

Run: `docker exec devbridge_backend go build ./...`
Expected: no errors.

- [ ] **Step 5: Verify all existing tests still pass**

Run: `docker exec devbridge_backend go test ./...`
Expected: PASS (no regressions in any package).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/job_scraping.go backend/internal/services/job_scraping_scheduler.go backend/cmd/server/main.go
git commit -m "feat(job-search): add RunScrape orchestrator and daily scheduler"
```

---

### Task 10: Job search handler - profile and scan-now

**Files:**
- Create: `backend/internal/handlers/job_search_handler.go`

**Interfaces:**
- Consumes: `models.JobSearchProfile` (Task 1), `services.RunScrape`/`JobMatcher`/`NewJobMatchService` (Tasks 7, 9), `services.JobApplicationDrafter`/`NewJobApplicationDraftService` (Task 8).
- Produces: `handlers.JobSearchHandler` struct with `NewJobSearchHandler() *JobSearchHandler`, `GetJobSearchProfile`, `UpdateJobSearchProfile`, `ScanNow` methods - Task 12 adds the remaining methods to the same struct and Task 12's routes file registers all of them together (registering routes for only some handler methods before the others exist would still compile fine since these are just unused exported methods, but routes are wired in one pass in Task 12 for a single reviewable diff).

This is a DB-touching HTTP handler with no automated test, matching `email_handler.go` (no test file). Verification is a container build; end-to-end verification happens once Task 12 registers the routes.

- [ ] **Step 1: Implement job_search_handler.go (profile + scan-now)**

`backend/internal/handlers/job_search_handler.go`:
```go
// backend/internal/handlers/job_search_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type JobSearchHandler struct {
	matcher services.JobMatcher
	drafter services.JobApplicationDrafter
}

func NewJobSearchHandler() *JobSearchHandler {
	return &JobSearchHandler{
		matcher: services.NewJobMatchService(),
		drafter: services.NewJobApplicationDraftService(),
	}
}

// GetJobSearchProfile - GET /api/v1/admin/job-search/profile (super_admin only, see routes/job_search_routes.go)
func (h *JobSearchHandler) GetJobSearchProfile(c *fiber.Ctx) error {
	var profile models.JobSearchProfile
	if err := database.GetDB().First(&profile, 1).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load job search profile"})
	}
	return c.JSON(fiber.Map{"success": true, "profile": profile})
}

// UpdateJobSearchProfile - PUT /api/v1/admin/job-search/profile (super_admin only, see routes/job_search_routes.go)
func (h *JobSearchHandler) UpdateJobSearchProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)

	var req struct {
		CVText      string `json:"cv_text"`
		Skills      string `json:"skills"`
		Preferences string `json:"preferences"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	db := database.GetDB()
	if err := db.Model(&models.JobSearchProfile{}).Where("id = ?", 1).Updates(map[string]interface{}{
		"cv_text":     req.CVText,
		"skills":      req.Skills,
		"preferences": req.Preferences,
		"updated_by":  userID,
	}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to update job search profile"})
	}

	var profile models.JobSearchProfile
	if err := db.First(&profile, 1).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load updated job search profile"})
	}
	return c.JSON(fiber.Map{"success": true, "profile": profile})
}

// ScanNow - POST /api/v1/admin/job-search/scan-now (super_admin only, see routes/job_search_routes.go)
// Runs RunScrape synchronously and reports how many new listings/matches
// were created - no queue, matching the design doc's stated low volume.
func (h *JobSearchHandler) ScanNow(c *fiber.Ctx) error {
	newListings, newMatches := services.RunScrape(c.Context(), h.matcher)
	return c.JSON(fiber.Map{"success": true, "new_listings": newListings, "new_matches": newMatches})
}
```

- [ ] **Step 2: Verify the backend builds**

Run: `docker exec devbridge_backend go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handlers/job_search_handler.go
git commit -m "feat(job-search): add job search handler profile and scan-now endpoints"
```

---

### Task 11: Job search handler - manual listing add

**Files:**
- Modify: `backend/internal/handlers/job_search_handler.go`

**Interfaces:**
- Consumes: `jobscraper.FetchJobFromURL`/`ScrapedJob` (Task 4), `services.JobMatcher` (Task 7), `models.JobListing`/`JobMatch`/`JobSearchProfile` (Task 1).
- Produces: `JobSearchHandler.AddManualListing` method - Task 12's routes file registers this at `POST /admin/job-search/listings/manual`.

- [ ] **Step 1: Add AddManualListing to job_search_handler.go**

Add this import to the existing import block in `backend/internal/handlers/job_search_handler.go`:
```go
	"net/url"
	"strings"

	"dev-bridge-manager/internal/services/jobscraper"
```

Add this method to the file:
```go
// AddManualListing - POST /api/v1/admin/job-search/listings/manual (super_admin only, see routes/job_search_routes.go)
func (h *JobSearchHandler) AddManualListing(c *fiber.Ctx) error {
	var req struct {
		URL         string `json:"url"`
		Title       string `json:"title"`
		Company     string `json:"company"`
		Location    string `json:"location"`
		Description string `json:"description"`
	}
	if err := c.BodyParser(&req); err != nil || strings.TrimSpace(req.URL) == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "A hirdetés URL-je kötelező"})
	}

	fetchResult, err := jobscraper.FetchJobFromURL(c.Context(), req.URL)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Nem sikerült lekérni az URL-t: " + err.Error()})
	}

	job := jobscraper.ScrapedJob{ExternalURL: req.URL}
	if fetchResult.Extracted {
		job = fetchResult.Job
	} else {
		if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Description) == "" {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"message": "Az adatok automatikus kinyerése nem sikerült - add meg kézzel a hirdetés adatait",
			})
		}
		job.Title = req.Title
		job.Company = req.Company
		job.Location = req.Location
		job.Description = req.Description
	}

	site, err := siteFromURL(req.URL)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Érvénytelen URL"})
	}

	db := database.GetDB()
	var listing models.JobListing
	err = db.Raw(`
		INSERT INTO job_listings (site, external_url, title, company, location, description)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (site, external_url) DO NOTHING
		RETURNING id, site, external_url, title, company, location, description, posted_at, scraped_at
	`, site, job.ExternalURL, job.Title, job.Company, job.Location, job.Description).Scan(&listing).Error
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to save listing"})
	}
	if listing.ID == 0 {
		// ON CONFLICT DO NOTHING left listing.ID unset - the row already
		// existed, so load it back by its unique key.
		if err := db.Where("site = ? AND external_url = ?", site, job.ExternalURL).First(&listing).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load existing listing"})
		}
	}

	var match models.JobMatch
	if err := db.Where("job_listing_id = ?", listing.ID).First(&match).Error; err != nil {
		var profile models.JobSearchProfile
		if err := db.First(&profile, 1).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load job search profile"})
		}
		score, reasoning, err := h.matcher.MatchJob(c.Context(), profile.CVText, profile.Skills, profile.Preferences, listing.Title, listing.Company, listing.Location, listing.Description)
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"success": false, "message": "Failed to score listing: " + err.Error()})
		}
		match = models.JobMatch{JobListingID: listing.ID, Score: score, Reasoning: reasoning, Status: "new"}
		if err := db.Create(&match).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to save match"})
		}
	}
	match.JobListing = listing

	return c.JSON(fiber.Map{"success": true, "match": match})
}

func siteFromURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid url")
	}
	return strings.TrimPrefix(u.Host, "www."), nil
}
```

Also add `"fmt"` to the import block (used by `siteFromURL`).

- [ ] **Step 2: Verify the backend builds**

Run: `docker exec devbridge_backend go build ./...`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handlers/job_search_handler.go
git commit -m "feat(job-search): add manual listing add endpoint"
```

---

### Task 12: Job search handler (matches) + routes wiring

**Files:**
- Modify: `backend/internal/handlers/job_search_handler.go`
- Create: `backend/internal/routes/job_search_routes.go`
- Modify: `backend/internal/routes/routes.go`

**Interfaces:**
- Consumes: `models.JobMatch` (Task 1), `services.JobApplicationDrafter` (Task 8), every `JobSearchHandler` method from Tasks 10-11.
- Produces: `JobSearchHandler.ListJobMatches`, `UpdateJobMatchStatus`, `DraftApplication`, `MarkApplied` methods; `routes.SetupJobSearchRoutes(api fiber.Router)` - Task 13's frontend service calls the full set of endpoints this task registers.

- [ ] **Step 1: Add the remaining handler methods**

Add this import to the existing import block in `backend/internal/handlers/job_search_handler.go`:
```go
	"strconv"
	"time"
```

Add these methods to the file:
```go
// ListJobMatches - GET /api/v1/admin/job-search/matches?status= (super_admin only, see routes/job_search_routes.go)
func (h *JobSearchHandler) ListJobMatches(c *fiber.Ctx) error {
	query := database.GetDB().Preload("JobListing").Order("score DESC")
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	var matches []models.JobMatch
	if err := query.Find(&matches).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load matches"})
	}
	return c.JSON(fiber.Map{"success": true, "matches": matches})
}

var validJobMatchStatuses = map[string]bool{"reviewed": true, "dismissed": true}

// UpdateJobMatchStatus - PATCH /api/v1/admin/job-search/matches/:id/status (super_admin only, see routes/job_search_routes.go)
func (h *JobSearchHandler) UpdateJobMatchStatus(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid match id"})
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil || !validJobMatchStatuses[req.Status] {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Status must be 'reviewed' or 'dismissed'"})
	}
	if err := database.GetDB().Model(&models.JobMatch{}).Where("id = ?", id).Update("status", req.Status).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to update status"})
	}
	return c.JSON(fiber.Map{"success": true})
}

// DraftApplication - POST /api/v1/admin/job-search/matches/:id/draft-application (super_admin only, see routes/job_search_routes.go)
// Generates a draft without persisting it, same "generate only" pattern as
// email_handler.go's BreakdownEmailIntoTasks.
func (h *JobSearchHandler) DraftApplication(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid match id"})
	}
	var req struct {
		Instruction string `json:"instruction"`
	}
	c.BodyParser(&req)

	var match models.JobMatch
	if err := database.GetDB().Preload("JobListing").First(&match, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Match not found"})
	}

	var profile models.JobSearchProfile
	if err := database.GetDB().First(&profile, 1).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to load job search profile"})
	}

	draft, err := h.drafter.DraftApplication(c.Context(), profile.CVText, profile.Skills, match.JobListing.Title, match.JobListing.Company, match.JobListing.Description, req.Instruction)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"success": false, "message": "Failed to generate application draft: " + err.Error()})
	}
	return c.JSON(fiber.Map{"success": true, "draft": draft})
}

// MarkApplied - POST /api/v1/admin/job-search/matches/:id/mark-applied (super_admin only, see routes/job_search_routes.go)
// Called after either the "sent via email" or "applied manually on site"
// path completes on the frontend.
func (h *JobSearchHandler) MarkApplied(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid match id"})
	}
	var req struct {
		ApplicationText string `json:"application_text"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	now := time.Now()
	if err := database.GetDB().Model(&models.JobMatch{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":           "applied",
		"applied_at":       now,
		"application_text": req.ApplicationText,
	}).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Failed to mark as applied"})
	}
	return c.JSON(fiber.Map{"success": true})
}
```

- [ ] **Step 2: Create the routes file**

`backend/internal/routes/job_search_routes.go`:
```go
// backend/internal/routes/job_search_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupJobSearchRoutes(api fiber.Router) {
	h := handlers.NewJobSearchHandler()

	admin := api.Group("/admin/job-search")
	admin.Use(middleware.JWTMiddleware())

	// super_admin-only, same reasoning as profile_routes.go: this is a
	// personal job-search tool for the operator, not a client-facing
	// feature - passed per-route, not via admin.Use(), so it never leaks
	// onto other /admin/* routes registered by other route files.
	admin.Get("/profile", middleware.RequireRole("super_admin"), h.GetJobSearchProfile)
	admin.Put("/profile", middleware.RequireRole("super_admin"), h.UpdateJobSearchProfile)
	admin.Post("/scan-now", middleware.RequireRole("super_admin"), h.ScanNow)
	admin.Post("/listings/manual", middleware.RequireRole("super_admin"), h.AddManualListing)
	admin.Get("/matches", middleware.RequireRole("super_admin"), h.ListJobMatches)
	admin.Patch("/matches/:id/status", middleware.RequireRole("super_admin"), h.UpdateJobMatchStatus)
	admin.Post("/matches/:id/draft-application", middleware.RequireRole("super_admin"), h.DraftApplication)
	admin.Post("/matches/:id/mark-applied", middleware.RequireRole("super_admin"), h.MarkApplied)
}
```

- [ ] **Step 3: Register the routes in routes.go**

In `backend/internal/routes/routes.go`, add after `SetupProfileRoutes(v1)`:
```go
	SetupJobSearchRoutes(v1)         // Job search: profile, scan-now, manual add, matches - super_admin only
```

- [ ] **Step 4: Verify the backend builds and existing tests still pass**

Run: `docker exec devbridge_backend go build ./... && docker exec devbridge_backend go test ./...`
Expected: no errors, all tests PASS.

- [ ] **Step 5: Manual smoke test**

Run: `docker exec devbridge_backend curl -s -X GET http://localhost:8080/api/v1/admin/job-search/profile -H "Authorization: Bearer <a super_admin JWT from the running app>"`
Expected: `{"success":true,"profile":{"id":1,"cv_text":"","skills":"","preferences":"",...}}`

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handlers/job_search_handler.go backend/internal/routes/job_search_routes.go backend/internal/routes/routes.go
git commit -m "feat(job-search): add match list/status/draft/mark-applied endpoints and route registration"
```

---

### Task 13: Frontend jobSearchService.ts

**Files:**
- Create: `frontend/src/services/jobSearchService.ts`

**Interfaces:**
- Consumes: every `/admin/job-search/*` endpoint from Task 12.
- Produces: `JobSearchProfile`, `JobListing`, `JobMatch` TypeScript interfaces, `ManualListingInput`, and `JobSearchService` with `getProfile`, `updateProfile`, `scanNow`, `addManualListing`, `listMatches`, `updateMatchStatus`, `draftApplication`, `markApplied` - Tasks 14 and 15 both import from this file.

This is a thin HTTP client with no business logic to unit-test, matching `profileService.ts`/`emailsService.ts` (no test files). Verification is `tsc --noEmit`.

- [ ] **Step 1: Implement jobSearchService.ts**

`frontend/src/services/jobSearchService.ts`:
```typescript
// frontend/src/services/jobSearchService.ts
import { apiClient } from '@/lib/api'

export interface JobSearchProfile {
    id: number
    cv_text: string
    skills: string
    preferences: string
    updated_by: number
    updated_at: string
}

export interface JobListing {
    id: number
    site: string
    external_url: string
    title: string
    company: string
    location: string
    description: string
    posted_at: string | null
    scraped_at: string
}

export interface JobMatch {
    id: number
    job_listing_id: number
    job_listing: JobListing
    score: number
    reasoning: string
    status: 'new' | 'reviewed' | 'dismissed' | 'applied'
    applied_at: string | null
    application_text: string | null
    created_at: string
    updated_at: string
}

export interface ManualListingInput {
    url: string
    title?: string
    company?: string
    location?: string
    description?: string
}

interface ProfileApiResponse {
    success: boolean
    message?: string
    profile?: JobSearchProfile
}

interface MatchesApiResponse {
    success: boolean
    message?: string
    matches?: JobMatch[]
}

interface ScanNowApiResponse {
    success: boolean
    message?: string
    new_listings?: number
    new_matches?: number
}

interface ManualAddApiResponse {
    success: boolean
    message?: string
    match?: JobMatch
}

interface DraftApiResponse {
    success: boolean
    message?: string
    draft?: string
}

export const JobSearchService = {
    async getProfile(): Promise<JobSearchProfile> {
        const response = await apiClient.get<ProfileApiResponse>('/admin/job-search/profile')
        if (response.success && response.profile) return response.profile
        throw new Error(response.message || 'Failed to fetch job search profile')
    },

    async updateProfile(data: { cv_text: string; skills: string; preferences: string }): Promise<JobSearchProfile> {
        const response = await apiClient.put<ProfileApiResponse>('/admin/job-search/profile', data)
        if (response.success && response.profile) return response.profile
        throw new Error(response.message || 'Failed to update job search profile')
    },

    async scanNow(): Promise<{ newListings: number; newMatches: number }> {
        // Scan-now runs the scraper and scores every new listing
        // synchronously - same reasoning as draftReply's longer timeout,
        // just larger since this can score many listings in one call.
        const response = await apiClient.post<ScanNowApiResponse>('/admin/job-search/scan-now', {}, 40000)
        if (response.success) {
            return { newListings: response.new_listings || 0, newMatches: response.new_matches || 0 }
        }
        throw new Error(response.message || 'Failed to run scan')
    },

    async addManualListing(input: ManualListingInput): Promise<JobMatch> {
        const response = await apiClient.post<ManualAddApiResponse>('/admin/job-search/listings/manual', input, 40000)
        if (response.success && response.match) return response.match
        throw new Error(response.message || 'Failed to add listing')
    },

    async listMatches(status?: string): Promise<JobMatch[]> {
        const response = await apiClient.get<MatchesApiResponse>('/admin/job-search/matches', status ? { status } : undefined)
        if (response.success && response.matches) return response.matches
        throw new Error(response.message || 'Failed to fetch matches')
    },

    async updateMatchStatus(id: number, status: 'reviewed' | 'dismissed'): Promise<void> {
        const response = await apiClient.patch<{ success: boolean; message?: string }>(`/admin/job-search/matches/${id}/status`, { status })
        if (!response.success) throw new Error(response.message || 'Failed to update match status')
    },

    async draftApplication(id: number, instruction?: string): Promise<string> {
        const response = await apiClient.post<DraftApiResponse>(`/admin/job-search/matches/${id}/draft-application`, { instruction }, 40000)
        if (response.success && response.draft !== undefined) return response.draft
        throw new Error(response.message || 'Failed to draft application')
    },

    async markApplied(id: number, applicationText: string): Promise<void> {
        const response = await apiClient.post<{ success: boolean; message?: string }>(`/admin/job-search/matches/${id}/mark-applied`, { application_text: applicationText })
        if (!response.success) throw new Error(response.message || 'Failed to mark as applied')
    },
}
```

- [ ] **Step 2: Verify it type-checks**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/services/jobSearchService.ts
git commit -m "feat(job-search): add jobSearchService frontend API client"
```

---

### Task 14: Frontend job-search admin page

**Files:**
- Create: `frontend/src/app/dashboard/admin/job-search/page.tsx`

**Interfaces:**
- Consumes: `JobSearchService`, `JobSearchProfile`, `JobMatch` (Task 13). Renders a `JobApplicationModal` from Task 15 (created next, but this task can be written first and Task 15 wires the import - both tasks touch this file's import line, so this task adds a temporary `applicationMatch` state without the modal import to keep this task's build green on its own, and Task 15 adds the import plus the rendered modal).

- [ ] **Step 1: Implement page.tsx (without the application modal, wired in Task 15)**

`frontend/src/app/dashboard/admin/job-search/page.tsx`:
```tsx
// frontend/src/app/dashboard/admin/job-search/page.tsx
'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { ArrowLeft } from 'lucide-react'
import { ProtectedRoute } from '@/components/auth/ProtectedRoute'
import { Button } from '@/components/ui/button'
import LoadingState from '@/components/ui/LoadingState'
import ErrorState from '@/components/ui/ErrorState'
import { JobSearchService, JobSearchProfile, JobMatch } from '@/services/jobSearchService'

function scoreBadgeClass(score: number): string {
    if (score >= 70) return 'bg-success/10 text-success'
    if (score >= 40) return 'bg-amber-500/10 text-amber-700'
    return 'bg-muted text-muted-foreground'
}

function statusLabel(status: JobMatch['status']): string {
    switch (status) {
        case 'new': return 'Új'
        case 'reviewed': return 'Átnézve'
        case 'dismissed': return 'Elutasítva'
        case 'applied': return 'Jelentkezve'
    }
}

export default function JobSearchPage() {
    return (
        <ProtectedRoute role="super_admin">
            <JobSearchPageContent />
        </ProtectedRoute>
    )
}

function JobSearchPageContent() {
    const [, setProfile] = useState<JobSearchProfile | null>(null)
    const [cvText, setCvText] = useState('')
    const [skills, setSkills] = useState('')
    const [preferences, setPreferences] = useState('')
    const [profileLoading, setProfileLoading] = useState(true)
    const [profileError, setProfileError] = useState<string | null>(null)
    const [savingProfile, setSavingProfile] = useState(false)
    const [profileSaved, setProfileSaved] = useState(false)

    const [matches, setMatches] = useState<JobMatch[]>([])
    const [matchesLoading, setMatchesLoading] = useState(true)
    const [matchesError, setMatchesError] = useState<string | null>(null)

    const [scanning, setScanning] = useState(false)
    const [scanMessage, setScanMessage] = useState<string | null>(null)

    const [manualUrl, setManualUrl] = useState('')
    const [manualFields, setManualFields] = useState<{ title: string; company: string; location: string; description: string } | null>(null)
    const [manualError, setManualError] = useState<string | null>(null)
    const [addingManual, setAddingManual] = useState(false)

    const loadProfile = () => {
        setProfileLoading(true)
        setProfileError(null)
        JobSearchService.getProfile()
            .then((p) => {
                setProfile(p)
                setCvText(p.cv_text)
                setSkills(p.skills)
                setPreferences(p.preferences)
            })
            .catch((err) => setProfileError(err.message))
            .finally(() => setProfileLoading(false))
    }

    const loadMatches = () => {
        setMatchesLoading(true)
        setMatchesError(null)
        JobSearchService.listMatches()
            .then(setMatches)
            .catch((err) => setMatchesError(err.message))
            .finally(() => setMatchesLoading(false))
    }

    useEffect(() => {
        loadProfile()
        loadMatches()
    }, [])

    const handleSaveProfile = async (e: React.FormEvent) => {
        e.preventDefault()
        setSavingProfile(true)
        setProfileError(null)
        setProfileSaved(false)
        try {
            const updated = await JobSearchService.updateProfile({ cv_text: cvText, skills, preferences })
            setProfile(updated)
            setProfileSaved(true)
        } catch (err) {
            setProfileError(err instanceof Error ? err.message : 'Hiba történt a profil mentése közben.')
        } finally {
            setSavingProfile(false)
        }
    }

    const handleScanNow = async () => {
        setScanning(true)
        setScanMessage(null)
        try {
            const { newListings, newMatches } = await JobSearchService.scanNow()
            setScanMessage(`${newListings} új hirdetés, ${newMatches} új értékelés.`)
            loadMatches()
        } catch (err) {
            setScanMessage(err instanceof Error ? err.message : 'Hiba történt a keresés közben.')
        } finally {
            setScanning(false)
        }
    }

    const handleAddManual = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!manualUrl.trim()) return
        setAddingManual(true)
        setManualError(null)
        try {
            await JobSearchService.addManualListing(
                manualFields ? { url: manualUrl.trim(), ...manualFields } : { url: manualUrl.trim() }
            )
            setManualUrl('')
            setManualFields(null)
            loadMatches()
        } catch (err) {
            const message = err instanceof Error ? err.message : 'Hiba történt a hirdetés hozzáadása közben.'
            setManualError(message)
            if (!manualFields) {
                setManualFields({ title: '', company: '', location: '', description: '' })
            }
        } finally {
            setAddingManual(false)
        }
    }

    const handleUpdateStatus = async (id: number, status: 'reviewed' | 'dismissed') => {
        try {
            await JobSearchService.updateMatchStatus(id, status)
            setMatches((prev) => prev.map((m) => (m.id === id ? { ...m, status } : m)))
        } catch {
            // Best-effort - the list keeps showing the stale status if this
            // fails, and the user can retry the action.
        }
    }

    return (
        <div className="p-6 space-y-6 max-w-4xl mx-auto">
            <Link href="/dashboard/admin" className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground">
                <ArrowLeft size={14} /> Vissza az adminisztrációhoz
            </Link>

            <div>
                <h1 className="text-lg font-semibold text-foreground">Álláskeresés</h1>
                <p className="text-muted-foreground text-sm">profession.hu automatikus figyelése és AI-alapú illeszkedés-értékelés</p>
            </div>

            <section className="bg-card rounded-xl shadow-sm border border-border p-6">
                <h2 className="font-medium text-foreground mb-4">CV és preferenciák</h2>
                {profileLoading ? (
                    <LoadingState message="Betöltés..." />
                ) : (
                    <form onSubmit={handleSaveProfile} className="space-y-4">
                        {profileError && (
                            <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                                {profileError}
                            </div>
                        )}
                        {profileSaved && (
                            <div className="bg-success/10 border border-success/20 text-success px-3 py-2 rounded text-sm">
                                Profil elmentve
                            </div>
                        )}
                        <div>
                            <label htmlFor="cv_text" className="block text-sm font-medium text-foreground mb-1">CV szövege</label>
                            <textarea
                                id="cv_text"
                                value={cvText}
                                onChange={(e) => setCvText(e.target.value)}
                                rows={6}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={savingProfile}
                            />
                        </div>
                        <div>
                            <label htmlFor="skills" className="block text-sm font-medium text-foreground mb-1">Készségek</label>
                            <textarea
                                id="skills"
                                value={skills}
                                onChange={(e) => setSkills(e.target.value)}
                                rows={3}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={savingProfile}
                            />
                        </div>
                        <div>
                            <label htmlFor="preferences" className="block text-sm font-medium text-foreground mb-1">Preferenciák</label>
                            <textarea
                                id="preferences"
                                value={preferences}
                                onChange={(e) => setPreferences(e.target.value)}
                                rows={3}
                                className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={savingProfile}
                            />
                        </div>
                        <Button type="submit" disabled={savingProfile}>
                            {savingProfile ? 'Mentés...' : 'Mentés'}
                        </Button>
                    </form>
                )}
            </section>

            <section className="bg-card rounded-xl shadow-sm border border-border p-6 space-y-3">
                <h2 className="font-medium text-foreground">Keresés</h2>
                <Button onClick={handleScanNow} loading={scanning}>
                    Keresés most
                </Button>
                {scanMessage && <p className="text-sm text-muted-foreground">{scanMessage}</p>}
            </section>

            <section className="bg-card rounded-xl shadow-sm border border-border p-6 space-y-3">
                <h2 className="font-medium text-foreground">Hirdetés hozzáadása linkkel</h2>
                <form onSubmit={handleAddManual} className="space-y-3">
                    {manualError && (
                        <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                            {manualError}
                        </div>
                    )}
                    <input
                        type="url"
                        value={manualUrl}
                        onChange={(e) => setManualUrl(e.target.value)}
                        placeholder="https://..."
                        className="w-full px-3 py-2 border border-input rounded-lg focus:outline-none focus:ring-2 focus:ring-ring"
                        disabled={addingManual}
                        required
                    />
                    {manualFields && (
                        <div className="space-y-2 border border-border rounded-lg p-3">
                            <p className="text-xs text-muted-foreground">Az adatok automatikus kinyerése nem sikerült - add meg kézzel:</p>
                            <input
                                type="text"
                                value={manualFields.title}
                                onChange={(e) => setManualFields({ ...manualFields, title: e.target.value })}
                                placeholder="Munkakör"
                                className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={addingManual}
                            />
                            <input
                                type="text"
                                value={manualFields.company}
                                onChange={(e) => setManualFields({ ...manualFields, company: e.target.value })}
                                placeholder="Cég"
                                className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={addingManual}
                            />
                            <input
                                type="text"
                                value={manualFields.location}
                                onChange={(e) => setManualFields({ ...manualFields, location: e.target.value })}
                                placeholder="Helyszín"
                                className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={addingManual}
                            />
                            <textarea
                                value={manualFields.description}
                                onChange={(e) => setManualFields({ ...manualFields, description: e.target.value })}
                                placeholder="Leírás"
                                rows={4}
                                className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                                disabled={addingManual}
                            />
                        </div>
                    )}
                    <Button type="submit" loading={addingManual}>
                        Hirdetés hozzáadása
                    </Button>
                </form>
            </section>

            <section className="space-y-3">
                <h2 className="font-medium text-foreground">Találatok</h2>
                {matchesLoading ? (
                    <LoadingState message="Betöltés..." />
                ) : matchesError ? (
                    <ErrorState error={matchesError} onRetry={loadMatches} />
                ) : matches.length === 0 ? (
                    <p className="text-sm text-muted-foreground">Nincs még egyetlen hirdetés sem.</p>
                ) : (
                    <div className="space-y-3">
                        {matches.map((match) => (
                            <div key={match.id} className="bg-card rounded-xl shadow-sm border border-border p-4 space-y-2">
                                <div className="flex items-start justify-between gap-3">
                                    <div>
                                        <h3 className="font-medium text-foreground">{match.job_listing.title}</h3>
                                        <p className="text-sm text-muted-foreground">{match.job_listing.company} - {match.job_listing.location}</p>
                                    </div>
                                    <div className="flex items-center gap-2 shrink-0">
                                        <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${scoreBadgeClass(match.score)}`}>
                                            {match.score}
                                        </span>
                                        <span className="text-xs font-medium px-2 py-0.5 rounded-full bg-muted text-muted-foreground">
                                            {statusLabel(match.status)}
                                        </span>
                                    </div>
                                </div>
                                <p className="text-sm text-foreground">{match.reasoning}</p>
                                <div className="flex flex-wrap gap-2 pt-1">
                                    <a href={match.job_listing.external_url} target="_blank" rel="noopener noreferrer">
                                        <Button type="button" variant="outline" size="sm">Hirdetés megnyitása</Button>
                                    </a>
                                    {match.status !== 'dismissed' && match.status !== 'applied' && (
                                        <Button type="button" variant="danger" size="sm" onClick={() => handleUpdateStatus(match.id, 'dismissed')}>
                                            Elutasítás
                                        </Button>
                                    )}
                                </div>
                            </div>
                        ))}
                    </div>
                )}
            </section>
        </div>
    )
}
```

- [ ] **Step 2: Verify it type-checks**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/app/dashboard/admin/job-search/page.tsx
git commit -m "feat(job-search): add job search admin page (profile, scan, manual add, match list)"
```

---

### Task 15: Frontend JobApplicationModal + wiring into the page

**Files:**
- Create: `frontend/src/components/jobSearch/JobApplicationModal.tsx`
- Modify: `frontend/src/app/dashboard/admin/job-search/page.tsx`

**Interfaces:**
- Consumes: `JobSearchService.draftApplication`/`markApplied` (Task 13), `EmailsService.send` (existing `frontend/src/services/emailsService.ts`), `JobMatch` (Task 13), `Modal`/`Button` (existing shared UI components).
- Produces: `JobApplicationModal({ match: JobMatch, onClose: () => void, onApplied: (matchId: number) => void })` - rendered from Task 14's page.

- [ ] **Step 1: Implement JobApplicationModal.tsx**

`frontend/src/components/jobSearch/JobApplicationModal.tsx`:
```tsx
// frontend/src/components/jobSearch/JobApplicationModal.tsx
'use client'

import { useEffect, useState } from 'react'
import { Modal } from '@/components/ui/modal'
import { Button } from '@/components/ui/button'
import { JobSearchService, JobMatch } from '@/services/jobSearchService'
import { EmailsService } from '@/services/emailsService'

interface JobApplicationModalProps {
    match: JobMatch
    onClose: () => void
    onApplied: (matchId: number) => void
}

export function JobApplicationModal({ match, onClose, onApplied }: JobApplicationModalProps) {
    const [draft, setDraft] = useState('')
    const [draftLoading, setDraftLoading] = useState(true)
    const [draftError, setDraftError] = useState<string | null>(null)

    const [email, setEmail] = useState('')
    const [sending, setSending] = useState(false)
    const [sendError, setSendError] = useState<string | null>(null)

    const [markingApplied, setMarkingApplied] = useState(false)

    useEffect(() => {
        setDraftLoading(true)
        setDraftError(null)
        JobSearchService.draftApplication(match.id)
            .then(setDraft)
            .catch((err) => setDraftError(err instanceof Error ? err.message : 'Hiba történt a jelentkezés-tervezet készítése közben.'))
            .finally(() => setDraftLoading(false))
    }, [match.id])

    const handleSendEmail = async () => {
        if (!email.trim()) return
        setSending(true)
        setSendError(null)
        try {
            const sendResult = await EmailsService.send({
                to: email.trim(),
                subject: `Jelentkezés: ${match.job_listing.title}`,
                body: draft,
            })
            if (!sendResult.success) {
                setSendError(sendResult.message || 'Hiba történt az e-mail küldése közben.')
                return
            }
            await JobSearchService.markApplied(match.id, draft)
            onApplied(match.id)
        } catch (err) {
            setSendError(err instanceof Error ? err.message : 'Hiba történt az e-mail küldése közben.')
        } finally {
            setSending(false)
        }
    }

    const handleMarkApplied = async () => {
        setMarkingApplied(true)
        setSendError(null)
        try {
            await JobSearchService.markApplied(match.id, draft)
            onApplied(match.id)
        } catch (err) {
            setSendError(err instanceof Error ? err.message : 'Hiba történt a megjelölés közben.')
        } finally {
            setMarkingApplied(false)
        }
    }

    return (
        <Modal isOpen onClose={onClose} title={`Jelentkezés: ${match.job_listing.title}`} size="lg">
            <div className="p-6 pt-0 space-y-4">
                {draftError && (
                    <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded-lg text-sm">
                        {draftError}
                    </div>
                )}
                {sendError && (
                    <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded-lg text-sm">
                        {sendError}
                    </div>
                )}

                {draftLoading ? (
                    <div className="text-sm text-muted-foreground py-2">AI tervezet készítése...</div>
                ) : (
                    <textarea
                        value={draft}
                        onChange={(e) => setDraft(e.target.value)}
                        rows={12}
                        className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                    />
                )}

                <div>
                    <label htmlFor="apply_email" className="block text-sm font-medium text-foreground mb-1">Címzett e-mail címe</label>
                    <input
                        id="apply_email"
                        type="email"
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                        placeholder="hr@ceg.hu"
                        className="w-full px-3 py-2 border border-input rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                    />
                </div>

                <div className="flex flex-wrap gap-3 pt-2">
                    <Button type="button" onClick={handleSendEmail} loading={sending} disabled={!email.trim() || draftLoading}>
                        Küldés emailben
                    </Button>
                    <a href={match.job_listing.external_url} target="_blank" rel="noopener noreferrer">
                        <Button type="button" variant="outline">Hirdetés megnyitása</Button>
                    </a>
                    <Button type="button" variant="secondary" onClick={handleMarkApplied} loading={markingApplied}>
                        Megjelölés jelentkezettként
                    </Button>
                </div>
            </div>
        </Modal>
    )
}
```

- [ ] **Step 2: Wire the modal into the page**

In `frontend/src/app/dashboard/admin/job-search/page.tsx`, add the import:
```tsx
import { JobApplicationModal } from '@/components/jobSearch/JobApplicationModal'
```

Change the profile state line from:
```tsx
    const [, setProfile] = useState<JobSearchProfile | null>(null)
```
to:
```tsx
    const [applicationMatch, setApplicationMatch] = useState<JobMatch | null>(null)
    const [, setProfile] = useState<JobSearchProfile | null>(null)
```

In the match card's button row, add a "Jelentkezés" button before the "Elutasítás" button:
```tsx
                                    <Button type="button" size="sm" onClick={() => setApplicationMatch(match)}>
                                        Jelentkezés
                                    </Button>
```

At the end of the returned JSX, just before the final closing `</div>` of the page, add:
```tsx
            {applicationMatch && (
                <JobApplicationModal
                    match={applicationMatch}
                    onClose={() => setApplicationMatch(null)}
                    onApplied={(id) => {
                        setMatches((prev) => prev.map((m) => (m.id === id ? { ...m, status: 'applied' } : m)))
                        setApplicationMatch(null)
                    }}
                />
            )}
```

- [ ] **Step 3: Verify it type-checks**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 4: Manual browser check**

Log in as a super_admin user, navigate to `/dashboard/admin/job-search`, click "Jelentkezés" on any match, confirm the AI draft loads into the textarea, and confirm "Küldés emailben" and "Megjelölés jelentkezettként" are both clickable (full send requires a configured Gmail account and a real recipient - at minimum confirm no console errors and the modal opens/closes correctly).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/jobSearch/JobApplicationModal.tsx frontend/src/app/dashboard/admin/job-search/page.tsx
git commit -m "feat(job-search): add application modal with AI draft, email send, and mark-applied"
```

---

### Task 16: AdminTab "Álláskeresés" card

**Files:**
- Modify: `frontend/src/components/dashboard/tabs/AdminTab.tsx`

**Interfaces:**
- Consumes: nothing new - navigates to Task 14's page via `next/navigation`'s `useRouter`.

- [ ] **Step 1: Add the router import and hook**

In `frontend/src/components/dashboard/tabs/AdminTab.tsx`, change:
```tsx
'use client'

import { useState } from 'react'
import { User } from '@/types/user'
```
to:
```tsx
'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { User } from '@/types/user'
```

Inside the `AdminTab` function body, add the router hook right after the component signature:
```tsx
export default function AdminTab({ user }: AdminTabProps) {
    const router = useRouter()
    const [isBillingoModalOpen, setIsBillingoModalOpen] = useState(false)
```

- [ ] **Step 2: Add the card entry**

In the `adminCards` array, change the last entry ("AI Profil / Perszóna") from:
```tsx
        {
            title: "AI Profil / Perszóna",
            description: "Háttér, szakterület és írásminták beállítása, amit az AI-alapú funkciók a te stílusodban való válaszadáshoz használnak",
            buttonLabel: "Szerkesztés",
            permission: null,
            requireSuperAdmin: true,
            action: () => setIsProfileModalOpen(true),
            comingSoon: false
        }
    ]
```
to:
```tsx
        {
            title: "AI Profil / Perszóna",
            description: "Háttér, szakterület és írásminták beállítása, amit az AI-alapú funkciók a te stílusodban való válaszadáshoz használnak",
            buttonLabel: "Szerkesztés",
            permission: null,
            requireSuperAdmin: true,
            action: () => setIsProfileModalOpen(true),
            comingSoon: false
        },
        {
            title: "Álláskeresés",
            description: "profession.hu automatikus figyelése, AI-alapú illeszkedés-értékelés és jelentkezési tervezet",
            buttonLabel: "Megnyitás",
            permission: null,
            requireSuperAdmin: true,
            action: () => router.push('/dashboard/admin/job-search'),
            comingSoon: false
        }
    ]
```

- [ ] **Step 3: Verify it type-checks**

Run: `docker exec devbridge_frontend npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 4: Manual browser check**

Log in as a super_admin user, open the Admin tab, confirm the "Álláskeresés" card is visible, and clicking "Megnyitás" navigates to `/dashboard/admin/job-search`. Log in as a non-super_admin admin (if one exists) and confirm the card is not visible.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/dashboard/tabs/AdminTab.tsx
git commit -m "feat(job-search): add Álláskeresés card to the admin tab"
```
