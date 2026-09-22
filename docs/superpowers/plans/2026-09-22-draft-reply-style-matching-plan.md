# Draft-Reply Style Matching (RAG-lite + Feedback Logging) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the AI "draft-reply" suggestion sound more like the user by (1) automatically retrieving 0-2 previously sent replies that are stylistically relevant (same thread or same recipient/domain, ranked by lexical overlap) as few-shot examples, and (2) logging, on every send, how much the user changed the AI draft before sending it, as a lightweight future quality signal.

**Architecture:** A new pure scoring layer (`Tokenize`/`JaccardSimilarity`, domain/recipient matching, candidate scoring) backs a DB-touching orchestrator (`FindSimilarReplies`) that queries the already-synced `emails` table for candidate `sent` rows, ranks them, and fetches only the winning 0-2 candidates' full bodies live via the existing `GmailAPI.GetFullMessage`. The result flows through the existing `DraftReplier`/`DraftReplyService` seam into a new `similar_replies` field on the AI service's `/draft-reply` request, added to the prompt as a new section alongside the existing static profile context. Separately, the `SendEmail` handler gains a `generateSnippet` fix (sent-mirror rows currently persist an empty snippet, starving the ranking's most important signal) and, when the compose session started from an AI draft, computes a Jaccard similarity between that draft and the actually-sent body and logs it to a new `draft_feedback` table. No new infrastructure, dependencies, or vector/embedding search - purely additive to existing flows.

**Tech Stack:** Go (Fiber, GORM, golang-migrate), Python (FastAPI, pydantic-ai, Gemini via `google:<model>`), Next.js/React (TypeScript), PostgreSQL.

**Spec:** `docs/superpowers/specs/2026-09-22-draft-reply-style-matching-design.md`

## Global Constraints

- No GORM `AutoMigrate` anywhere - all schema changes go through golang-migrate files under `backend/migrations/`.
- Before creating `backend/migrations/000031_add_draft_feedback.up/down.sql`, run `ls backend/migrations | sort -V | tail -5` to confirm `000031` is still free - other in-flight work may have claimed it since this plan was written.
- No semantic/embedding search, no vector database, no model fine-tuning - lexical (word-overlap) Jaccard similarity is the only similarity mechanism, for both retrieval ranking and feedback measurement.
- The retrieval candidate query is capped to the most recent 200 `sent` rows (`ORDER BY received_at DESC LIMIT 200`) so the ranking step never runs against an unbounded set.
- Top-k for injected style examples is **2** (`similarReplyTopK = 2`), not more.
- If a per-candidate live fetch (`GmailAPI.GetFullMessage`) fails, drop that candidate only - never fail the whole draft-reply request because of it.
- If zero qualifying candidates remain after filtering, `similar_replies` is empty and behavior is identical to today (static profile only).
- No admin review/approval UI for `draft_feedback` in this round - measurement and storage only. No automatic `ProfileSample` creation from feedback.
- The AI never sends anything automatically - the draft is always a suggestion, and `draft_feedback` logging is purely for future analysis; it never triggers any action.
- No new DB-integration test infrastructure and no frontend `*.test.tsx` files - matching the existing convention, pure functions get unit tests, DB-touching Fiber handlers and DB-touching orchestrators do not (see `draft_reply_test.go`, `email_handler_test.go`).
- Run all backend Go commands via `docker exec devbridge_backend ...`, all AI-service Python commands via `docker exec devbridge_ai ...`, and all frontend commands via `docker exec devbridge_frontend ...`. Never run them on the host.

---

### Task 1: Text similarity helpers (`Tokenize`, `JaccardSimilarity`)

**Files:**
- Create: `backend/internal/services/text_similarity.go`
- Test: `backend/internal/services/text_similarity_test.go`

**Interfaces:**
- Produces: `services.Tokenize(s string) []string` and `services.JaccardSimilarity(a, b string) float64`. Consumed by Task 3 (`ScoreCandidate`) and Task 8 (feedback similarity computation).

- [ ] **Step 1: Write the failing tests**

`backend/internal/services/text_similarity_test.go`:

```go
// backend/internal/services/text_similarity_test.go
package services

import "testing"

func TestTokenize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"lowercases and splits on punctuation", "Szia, mikor kezdünk?", []string{"szia", "mikor", "kezdünk"}},
		{"drops tokens shorter than 3 characters", "az és mi alma", []string{"alma"}},
		{"empty input", "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Tokenize(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestJaccardSimilarity(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want float64
	}{
		{"identical text", "Szia, mikor kezdünk?", "Szia, mikor kezdünk?", 1.0},
		{"no common words", "kék labda gurul", "piros autó száguld", 0.0},
		{"partial overlap", "kék labda gurul gyorsan", "kék labda áll csendben", 2.0 / 6.0},
		{"both empty", "", "", 0.0},
		{"one empty", "kék labda", "", 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := JaccardSimilarity(tc.a, tc.b)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `docker exec devbridge_backend go test ./internal/services/... -run 'TestTokenize|TestJaccardSimilarity' -v`
Expected: FAIL with `undefined: Tokenize` / `undefined: JaccardSimilarity`.

- [ ] **Step 3: Write the implementation**

`backend/internal/services/text_similarity.go`:

```go
// backend/internal/services/text_similarity.go
package services

import (
	"strings"
	"unicode"
)

// Tokenize lowercases s, splits on runs of non-letter/non-digit characters,
// and drops tokens shorter than 3 characters - short tokens (particles,
// prepositions) contribute noise rather than signal to Jaccard similarity.
func Tokenize(s string) []string {
	var tokens []string
	var cur []rune
	flush := func() {
		if len(cur) >= 3 {
			tokens = append(tokens, string(cur))
		}
		cur = nil
	}
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur = append(cur, r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}

// JaccardSimilarity returns |intersection| / |union| of the token sets of a
// and b - 1.0 means the same words, 0.0 means no shared words (or both
// inputs tokenize to nothing).
func JaccardSimilarity(a, b string) float64 {
	setA := toSet(Tokenize(a))
	setB := toSet(Tokenize(b))

	union := make(map[string]struct{}, len(setA)+len(setB))
	intersection := 0
	for t := range setA {
		union[t] = struct{}{}
		if _, ok := setB[t]; ok {
			intersection++
		}
	}
	for t := range setB {
		union[t] = struct{}{}
	}
	if len(union) == 0 {
		return 0
	}
	return float64(intersection) / float64(len(union))
}

func toSet(tokens []string) map[string]struct{} {
	set := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		set[t] = struct{}{}
	}
	return set
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `docker exec devbridge_backend go test ./internal/services/... -run 'TestTokenize|TestJaccardSimilarity' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/text_similarity.go backend/internal/services/text_similarity_test.go
git commit -m "$(cat <<'EOF'
feat(services): add Tokenize/JaccardSimilarity text similarity helpers

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 2: Fix empty sent-mail snippet on send

**Files:**
- Modify: `backend/internal/handlers/email_handler.go:334-347` (the `localEmail` construction in `SendEmail`)
- Test: `backend/internal/handlers/email_handler_test.go`

**Interfaces:**
- Produces: `generateSnippet(body string) string` (unexported, package `handlers`). Consumed only within `SendEmail` in this same file.

- [ ] **Step 1: Write the failing test**

Append to `backend/internal/handlers/email_handler_test.go`:

```go
func TestGenerateSnippet(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"empty body", "", ""},
		{"short body left unchanged", "Szia, köszönöm az emailt.", "Szia, köszönöm az emailt."},
		{
			"long body cut at a word boundary around 200 chars",
			strings.Repeat("alma ", 60), // 300 chars, well past the 200 cutoff
			strings.TrimSpace(strings.Repeat("alma ", 60))[:199],
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := generateSnippet(tc.body)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			if len(got) > 200 {
				t.Fatalf("snippet longer than 200 chars: %d", len(got))
			}
		})
	}
}
```

Add `"strings"` to this test file's imports (it currently has none beyond `testing` and the models package):

```go
import (
	"strings"
	"testing"

	"dev-bridge-manager/internal/models"
)
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `docker exec devbridge_backend go test ./internal/handlers/... -run TestGenerateSnippet -v`
Expected: FAIL with `undefined: generateSnippet`.

- [ ] **Step 3: Write the implementation**

Add to `backend/internal/handlers/email_handler.go` (near the top-level helpers, after `isValidEmailCategory`):

```go
const snippetMaxLen = 200

// generateSnippet derives a Gmail-style preview snippet from a plain-text
// body: whitespace-collapsed, cut to roughly the first 200 characters at a
// word boundary. Used to backfill the sent-mirror row's snippet, which the
// incremental Gmail sync (gmail_sync.go) never revisits once a
// gmail_message_id is known.
func generateSnippet(body string) string {
	joined := strings.Join(strings.Fields(body), " ")
	if len(joined) <= snippetMaxLen {
		return joined
	}
	cut := joined[:snippetMaxLen]
	if idx := strings.LastIndex(cut, " "); idx > 0 {
		cut = cut[:idx]
	}
	return cut
}
```

Then change the `localEmail` construction in `SendEmail` (currently `Snippet: ""`):

```go
	localEmail := models.Email{
		GmailAccountID: account.ID,
		GmailMessageID: gmailMessageID,
		Folder:         "sent",
		FromAddress:    account.EmailAddress,
		ToAddresses:    to,
		Subject:        subject,
		Snippet:        generateSnippet(body),
		HasAttachments: len(attachments) > 0,
		AttachmentMeta: models.AttachmentMetaToJSON(attachmentMetas),
		IsRead:         true,
		ReceivedAt:     now,
		SyncedAt:       now,
	}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `docker exec devbridge_backend go test ./internal/handlers/... -run TestGenerateSnippet -v`
Expected: PASS

- [ ] **Step 5: Build the whole backend to confirm nothing else broke**

Run: `docker exec devbridge_backend go build ./...`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handlers/email_handler.go backend/internal/handlers/email_handler_test.go
git commit -m "$(cat <<'EOF'
fix(emails): backfill sent-mirror snippet instead of leaving it empty

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Candidate domain-matching and scoring (pure functions)

**Files:**
- Create: `backend/internal/services/similar_replies_scoring.go`
- Test: `backend/internal/services/similar_replies_scoring_test.go`

**Interfaces:**
- Consumes: `services.JaccardSimilarity(a, b string) float64` (Task 1).
- Produces: `services.ExtractDomain(emailAddress string) (string, error)`, `services.RecipientMatches(toAddresses, fromAddress string) bool`, `services.CandidateQualifies(candidate models.Email, incomingThreadID, incomingFromAddress string) bool`, `services.ScoreCandidate(candidate models.Email, incomingThreadID, incomingContent string) float64`. All consumed by Task 4 (`FindSimilarReplies`).

- [ ] **Step 1: Write the failing tests**

`backend/internal/services/similar_replies_scoring_test.go`:

```go
// backend/internal/services/similar_replies_scoring_test.go
package services

import (
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestExtractDomain(t *testing.T) {
	cases := []struct {
		name    string
		addr    string
		want    string
		wantErr bool
	}{
		{"plain address", "jane@example.com", "example.com", false},
		{"display name plus address", `"Jane Doe" <jane@example.com>`, "example.com", false},
		{"uppercase domain lowercased", "jane@EXAMPLE.COM", "example.com", false},
		{"malformed address", "not-an-address", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractDomain(tc.addr)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q", tc.addr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRecipientMatches(t *testing.T) {
	cases := []struct {
		name        string
		toAddresses string
		fromAddress string
		want        bool
	}{
		{"exact address match", "Jane Doe <jane@example.com>", "jane@example.com", true},
		{"domain match, different mailbox", "Bob <bob@example.com>", "jane@example.com", true},
		{"multi-recipient header, one matches", "Alice <alice@other.com>, Bob <bob@example.com>", "jane@example.com", true},
		{"case-insensitive match", "JANE@EXAMPLE.COM", "jane@example.com", true},
		{"no match", "Alice <alice@other.com>", "jane@example.com", false},
		{"empty from address", "Alice <alice@other.com>", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RecipientMatches(tc.toAddresses, tc.fromAddress)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCandidateQualifies(t *testing.T) {
	cases := []struct {
		name                string
		candidate           models.Email
		incomingThreadID    string
		incomingFromAddress string
		want                bool
	}{
		{
			"same thread qualifies regardless of recipient",
			models.Email{ThreadID: "t1", ToAddresses: "someone-else@other.com"},
			"t1", "jane@example.com",
			true,
		},
		{
			"different thread, recipient domain matches",
			models.Email{ThreadID: "t2", ToAddresses: "jane@example.com"},
			"t1", "jane@example.com",
			true,
		},
		{
			"different thread, no recipient match",
			models.Email{ThreadID: "t2", ToAddresses: "someone-else@other.com"},
			"t1", "jane@example.com",
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CandidateQualifies(tc.candidate, tc.incomingThreadID, tc.incomingFromAddress)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestScoreCandidate(t *testing.T) {
	t.Run("thread match always outscores lexical overlap", func(t *testing.T) {
		threadMatch := models.Email{ThreadID: "t1", Subject: "unrelated", Snippet: "totally different words"}
		score := ScoreCandidate(threadMatch, "t1", "some incoming content")
		if score != threadMatchScore {
			t.Fatalf("got %v, want %v", score, threadMatchScore)
		}
	})

	t.Run("cross-thread match uses Jaccard of subject+snippet vs content", func(t *testing.T) {
		candidate := models.Email{ThreadID: "t2", Subject: "Ajánlat", Snippet: "kék labda gurul"}
		score := ScoreCandidate(candidate, "t1", "kék labda áll")
		want := JaccardSimilarity("Ajánlat kék labda gurul", "kék labda áll")
		if score != want {
			t.Fatalf("got %v, want %v", score, want)
		}
	})

	t.Run("no thread match and no overlap scores zero", func(t *testing.T) {
		candidate := models.Email{ThreadID: "t2", Subject: "xyz", Snippet: "qrs"}
		score := ScoreCandidate(candidate, "t1", "completely unrelated content here")
		if score != 0 {
			t.Fatalf("got %v, want 0", score)
		}
	})
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `docker exec devbridge_backend go test ./internal/services/... -run 'TestExtractDomain|TestRecipientMatches|TestCandidateQualifies|TestScoreCandidate' -v`
Expected: FAIL with `undefined: ExtractDomain` (and the other new names).

- [ ] **Step 3: Write the implementation**

`backend/internal/services/similar_replies_scoring.go`:

```go
// backend/internal/services/similar_replies_scoring.go
package services

import (
	"fmt"
	"net/mail"
	"strings"

	"dev-bridge-manager/internal/models"
)

// threadMatchScore is a fixed score for same-thread candidates, guaranteed
// higher than any JaccardSimilarity result (whose max is 1.0) so a thread
// match always outranks a cross-thread lexical match.
const threadMatchScore = 2.0

// ExtractDomain returns the lowercase domain part of an email address, e.g.
// "jane@example.com" -> "example.com". Accepts either a bare address or a
// "Display Name <addr>" form, since it's built on net/mail.ParseAddress.
func ExtractDomain(emailAddress string) (string, error) {
	addr, err := mail.ParseAddress(emailAddress)
	if err != nil {
		return "", fmt.Errorf("parsing address %q: %w", emailAddress, err)
	}
	parts := strings.SplitN(addr.Address, "@", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", fmt.Errorf("no domain in address %q", emailAddress)
	}
	return strings.ToLower(parts[1]), nil
}

// RecipientMatches reports whether toAddresses - a raw, possibly
// multi-recipient To header value stored on models.Email - contains
// fromAddress's exact email address or its domain. Case-insensitive.
func RecipientMatches(toAddresses, fromAddress string) bool {
	fromLower := strings.ToLower(strings.TrimSpace(fromAddress))
	if fromLower == "" {
		return false
	}
	toLower := strings.ToLower(toAddresses)
	if strings.Contains(toLower, fromLower) {
		return true
	}
	domain, err := ExtractDomain(fromAddress)
	if err != nil || domain == "" {
		return false
	}
	return strings.Contains(toLower, "@"+domain)
}

// CandidateQualifies reports whether a sent-folder candidate is eligible as
// a style example for a reply to incomingFromAddress in thread
// incomingThreadID: either the same thread, or a shared recipient
// address/domain.
func CandidateQualifies(candidate models.Email, incomingThreadID, incomingFromAddress string) bool {
	if incomingThreadID != "" && candidate.ThreadID == incomingThreadID {
		return true
	}
	return RecipientMatches(candidate.ToAddresses, incomingFromAddress)
}

// ScoreCandidate ranks a qualifying candidate. A same-thread candidate is
// the strongest possible signal (literally the same conversation) and
// always outranks a cross-thread lexical match, which falls back to
// JaccardSimilarity between the candidate's subject+snippet and the
// incoming email's content.
func ScoreCandidate(candidate models.Email, incomingThreadID, incomingContent string) float64 {
	if incomingThreadID != "" && candidate.ThreadID == incomingThreadID {
		return threadMatchScore
	}
	return JaccardSimilarity(candidate.Subject+" "+candidate.Snippet, incomingContent)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `docker exec devbridge_backend go test ./internal/services/... -run 'TestExtractDomain|TestRecipientMatches|TestCandidateQualifies|TestScoreCandidate' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/similar_replies_scoring.go backend/internal/services/similar_replies_scoring_test.go
git commit -m "$(cat <<'EOF'
feat(services): add candidate qualification/scoring for similar-reply retrieval

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 4: `FindSimilarReplies` DB orchestrator

**Files:**
- Create: `backend/internal/services/similar_replies.go`

**Interfaces:**
- Consumes: `services.CandidateQualifies`, `services.ScoreCandidate` (Task 3); `services.GmailAPI.GetFullMessage(ctx, account *models.GmailAccount, messageID string) (*GmailFullMessage, error)` (existing, `backend/internal/services/gmail_api.go`); `models.Email{ID, GmailAccountID, ThreadID, FromAddress, ToAddresses, Subject, Snippet, GmailMessageID, Folder, ReceivedAt}` (existing).
- Produces: `services.FindSimilarReplies(ctx context.Context, db *gorm.DB, account *models.GmailAccount, incoming *models.Email, content string, gmailAPI GmailAPI) []string`. Consumed by Task 5's wiring into `email_handler.go`'s `DraftReply` handler.

No test for this task: it's a DB-touching orchestrator, matching the existing convention that DB-touching code (Fiber handlers, and by the same reasoning this GORM-querying function) doesn't get unit tests in this codebase - see the spec's Testing section and `email_handler_test.go`, which only tests the one pure helper it contains.

- [ ] **Step 1: Write the implementation**

`backend/internal/services/similar_replies.go`:

```go
// backend/internal/services/similar_replies.go
package services

import (
	"context"
	"sort"

	"dev-bridge-manager/internal/models"

	"gorm.io/gorm"
)

const (
	similarReplyCandidateLimit = 200
	similarReplyTopK           = 2
)

// FindSimilarReplies returns up to similarReplyTopK bodies of previously
// sent replies that are stylistically relevant to the email being answered
// (incoming) - the RAG-lite retrieval step for the draft-reply prompt.
// Candidates are drawn from the most recent similarReplyCandidateLimit
// "sent" rows for this account, filtered by CandidateQualifies and ranked
// by ScoreCandidate. Returns nil if no qualifying candidate remains. A
// failed live fetch for an individual winning candidate just drops that
// candidate - it never fails the whole call.
func FindSimilarReplies(ctx context.Context, db *gorm.DB, account *models.GmailAccount, incoming *models.Email, content string, gmailAPI GmailAPI) []string {
	var candidates []models.Email
	if err := db.WithContext(ctx).
		Where("gmail_account_id = ? AND folder = 'sent'", account.ID).
		Order("received_at DESC").
		Limit(similarReplyCandidateLimit).
		Find(&candidates).Error; err != nil {
		return nil
	}

	type scoredCandidate struct {
		email models.Email
		score float64
	}
	var qualifying []scoredCandidate
	for _, c := range candidates {
		if !CandidateQualifies(c, incoming.ThreadID, incoming.FromAddress) {
			continue
		}
		score := ScoreCandidate(c, incoming.ThreadID, content)
		if score <= 0 {
			continue
		}
		qualifying = append(qualifying, scoredCandidate{email: c, score: score})
	}
	sort.Slice(qualifying, func(i, j int) bool { return qualifying[i].score > qualifying[j].score })

	var replies []string
	for _, q := range qualifying {
		if len(replies) >= similarReplyTopK {
			break
		}
		full, err := gmailAPI.GetFullMessage(ctx, account, q.email.GmailMessageID)
		if err != nil {
			continue
		}
		replies = append(replies, full.BodyText)
	}
	return replies
}
```

- [ ] **Step 2: Build to confirm it compiles**

Run: `docker exec devbridge_backend go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add backend/internal/services/similar_replies.go
git commit -m "$(cat <<'EOF'
feat(services): add FindSimilarReplies retrieval orchestrator

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Extend `DraftReplier`/`DraftReplyService` with `similarReplies`, wire into the `DraftReply` handler

**Files:**
- Modify: `backend/internal/services/draft_reply.go`
- Modify: `backend/internal/services/draft_reply_test.go`
- Modify: `backend/internal/handlers/email_handler.go:162-207` (the `DraftReply` handler)

**Interfaces:**
- Consumes: `services.FindSimilarReplies` (Task 4).
- Produces: `DraftReplier.DraftReply(ctx context.Context, emailContent, profileContext string, similarReplies []string) (string, error)` - the interface signature every caller and every fake in future tests must match from here on.

- [ ] **Step 1: Write the failing test changes**

Replace the full contents of `backend/internal/services/draft_reply_test.go`:

```go
// backend/internal/services/draft_reply_test.go
package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDraftReplySendsFieldsAndReturnsDraft(t *testing.T) {
	var gotBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		if r.URL.Path != "/draft-reply" {
			t.Errorf("expected path /draft-reply, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"draft": "Szia! Köszönöm a megkeresésed."})
	}))
	defer server.Close()

	svc := &DraftReplyService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	got, err := svc.DraftReply(context.Background(), "email body", "profile context", []string{"prior reply one", "prior reply two"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Szia! Köszönöm a megkeresésed." {
		t.Fatalf("expected draft text, got %q", got)
	}
	if gotBody["email_content"] != "email body" || gotBody["profile_context"] != "profile context" {
		t.Fatalf("request body missing expected fields: %+v", gotBody)
	}
	gotSimilar, ok := gotBody["similar_replies"].([]interface{})
	if !ok || len(gotSimilar) != 2 || gotSimilar[0] != "prior reply one" || gotSimilar[1] != "prior reply two" {
		t.Fatalf("request body missing expected similar_replies: %+v", gotBody)
	}
}

func TestDraftReplyReturnsErrorOnNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"detail":"boom"}`))
	}))
	defer server.Close()

	svc := &DraftReplyService{httpClient: &http.Client{Timeout: 5 * time.Second}, baseURL: server.URL}
	_, err := svc.DraftReply(context.Background(), "s", "p", nil)
	if err == nil {
		t.Fatal("expected an error for a 500 response, got nil")
	}
}

func TestDraftReplyReturnsErrorOnTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"draft": "d"})
	}))
	defer server.Close()

	svc := &DraftReplyService{httpClient: &http.Client{Timeout: 5 * time.Millisecond}, baseURL: server.URL}
	_, err := svc.DraftReply(context.Background(), "s", "p", nil)
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestDraftReply -v`
Expected: FAIL with a compile error (`not enough arguments in call to svc.DraftReply`).

- [ ] **Step 3: Update the implementation**

In `backend/internal/services/draft_reply.go`, change the interface, request struct, and method:

```go
// DraftReplier is the seam email_handler.go calls through, so tests can
// inject a fake instead of hitting the real AI service.
type DraftReplier interface {
	DraftReply(ctx context.Context, emailContent, profileContext string, similarReplies []string) (string, error)
}
```

```go
type draftReplyRequest struct {
	EmailContent   string   `json:"email_content"`
	ProfileContext string   `json:"profile_context"`
	SimilarReplies []string `json:"similar_replies"`
}
```

```go
func (s *DraftReplyService) DraftReply(ctx context.Context, emailContent, profileContext string, similarReplies []string) (string, error) {
	payload, err := json.Marshal(draftReplyRequest{
		EmailContent:   emailContent,
		ProfileContext: profileContext,
		SimilarReplies: similarReplies,
	})
	if err != nil {
		return "", fmt.Errorf("encoding draft-reply request: %w", err)
	}
	// ... rest of the function body is unchanged from here on
```

(Everything after the `json.Marshal` call - building the request, doing the HTTP call, parsing the response - stays exactly as it is today.)

- [ ] **Step 4: Run the tests to verify they pass**

Run: `docker exec devbridge_backend go test ./internal/services/... -run TestDraftReply -v`
Expected: PASS

- [ ] **Step 5: Wire retrieval into the `DraftReply` handler**

In `backend/internal/handlers/email_handler.go`, change the `DraftReply` handler's call site (currently `draft, err := h.draftReplier.DraftReply(c.Context(), content, profileContext)`):

```go
	similarReplies := services.FindSimilarReplies(c.Context(), database.GetDB(), account, &email, content, h.gmailAPI)

	draft, err := h.draftReplier.DraftReply(c.Context(), content, profileContext, similarReplies)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"success": false, "message": "Failed to generate draft: " + err.Error()})
	}
```

This goes right before the existing `draft, err := ...` line, using the `account` and `email` variables already in scope earlier in the same handler.

- [ ] **Step 6: Build the whole backend**

Run: `docker exec devbridge_backend go build ./...`
Expected: no errors

- [ ] **Step 7: Run the full backend test suite**

Run: `docker exec devbridge_backend go test ./...`
Expected: all PASS

- [ ] **Step 8: Commit**

```bash
git add backend/internal/services/draft_reply.go backend/internal/services/draft_reply_test.go backend/internal/handlers/email_handler.go
git commit -m "$(cat <<'EOF'
feat(emails): pass retrieved similar replies into the draft-reply request

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: AI service - `similar_replies` field and prompt section

**Files:**
- Modify: `ai/app/draft_reply.py`
- Modify: `ai/tests/test_draft_reply.py`

**Interfaces:**
- Consumes: nothing new (mirrors the request shape `similar_replies: list[str]` produced by Task 5's `draftReplyRequest`).
- Produces: `DraftReplyRequest.similar_replies: list[str] = []`; `_build_prompt(req: DraftReplyRequest) -> str` (new, pure, unexported-by-convention helper extracted from `draft_reply`).

- [ ] **Step 1: Write the failing tests**

In `ai/tests/test_draft_reply.py`, change the existing import line (currently `from app.draft_reply import draft_reply_agent`) to also pull in the two new names:

```python
from app.draft_reply import DraftReplyRequest, _build_prompt, draft_reply_agent
```

Then append these two tests to the file:

```python
def test_build_prompt_includes_similar_replies_section_when_present():
    req = DraftReplyRequest(
        email_content="Szia, mikor kezdünk?",
        profile_context="",
        similar_replies=["Szia! Jövő héten kezdünk.", "Köszönöm a türelmed."],
    )
    prompt = _build_prompt(req)
    assert "korábban általad írt" in prompt
    assert "Szia! Jövő héten kezdünk." in prompt
    assert "Köszönöm a türelmed." in prompt


def test_build_prompt_omits_similar_replies_section_when_empty():
    req = DraftReplyRequest(email_content="Csak egy teszt üzenet.")
    prompt = _build_prompt(req)
    assert "korábban általad írt" not in prompt
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `docker exec devbridge_ai pytest tests/test_draft_reply.py -v`
Expected: FAIL with `ImportError: cannot import name '_build_prompt'` (and `similar_replies` being an unexpected keyword argument).

- [ ] **Step 3: Write the implementation**

Replace the full contents of `ai/app/draft_reply.py`:

```python
import os

from pydantic import BaseModel
from pydantic_ai import Agent

# Each agent module reads its own GEMINI_MODEL rather than importing it from
# categorize_email.py, to avoid a circular import - same reasoning as the
# comment in categorize_email.py.
GEMINI_MODEL = os.environ.get("GEMINI_MODEL", "gemini-2.5-flash")


class DraftReplyRequest(BaseModel):
    email_content: str
    profile_context: str = ""
    similar_replies: list[str] = []


class DraftReplyResult(BaseModel):
    draft: str


draft_reply_agent = Agent(
    f"google:{GEMINI_MODEL}",
    output_type=DraftReplyResult,
    defer_model_check=True,
    instructions=(
        "Draft a reply to a business email on behalf of the user. Write in "
        "Hungarian unless the original email is in another language. Keep "
        "the reply focused and only as long as the original email warrants. "
        "This is a draft suggestion only - a human will review and edit it "
        "before sending, so prefer a complete, ready-to-edit draft over "
        "placeholders."
    ),
)


def _build_prompt(req: DraftReplyRequest) -> str:
    prompt_parts = []
    if req.profile_context.strip():
        prompt_parts.append(
            "Az alábbi profil alapján fogalmazz választ a felhasználó nevében. "
            "Vedd figyelembe a hátterét, szakterületét és kommunikációs "
            "stílusát, az írásmintákat pedig hangnem-referenciaként "
            "használd.\n\n" + req.profile_context
        )
    if req.similar_replies:
        examples = "\n---\n".join(req.similar_replies)
        prompt_parts.append(
            "Az alábbi, korábban általad írt, hasonló témájú/címzettnek "
            "szóló válaszok stílusát is vedd figyelembe:\n\n---\n" + examples
        )
    prompt_parts.append(f"Beérkező email:\n{req.email_content}")
    return "\n\n".join(prompt_parts)


async def draft_reply(req: DraftReplyRequest) -> str:
    result = await draft_reply_agent.run(_build_prompt(req))
    return result.output.draft
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `docker exec devbridge_ai pytest tests/test_draft_reply.py -v`
Expected: PASS

- [ ] **Step 5: Run the full AI service test suite**

Run: `docker exec devbridge_ai pytest -q`
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add ai/app/draft_reply.py ai/tests/test_draft_reply.py
git commit -m "$(cat <<'EOF'
feat(ai): add similar_replies prompt section to draft-reply

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: `draft_feedback` migration + model

**Files:**
- Create: `backend/migrations/000031_add_draft_feedback.up.sql`
- Create: `backend/migrations/000031_add_draft_feedback.down.sql`
- Create: `backend/internal/models/draft_feedback.go`

**Interfaces:**
- Produces: `models.DraftFeedback{ID uint, EmailID uint, AIDraftText string, SentText string, Similarity float64, CreatedAt time.Time}` (table `draft_feedback`, FK `email_id` -> `emails(id)`). Consumed by Task 8's feedback-logging wiring in `SendEmail`.

There is no existing test file for any model in this codebase (`find backend/internal/models -iname "*_test.go"` returns nothing) - models are plain structs with no testable logic, so this task has no unit test. Verification is: the migration runs cleanly and the code compiles.

- [ ] **Step 1: Confirm the next-free migration number**

Run: `ls backend/migrations | sort -V | tail -5`
Expected: highest existing pair is `000030_add_profile.up/down.sql`. If a `000031` pair already exists from other work, use the next free number instead and adjust the filenames in the remaining steps accordingly.

- [ ] **Step 2: Write the migration**

`backend/migrations/000031_add_draft_feedback.up.sql`:

```sql
-- backend/migrations/000031_add_draft_feedback.up.sql

CREATE TABLE draft_feedback (
    id SERIAL PRIMARY KEY,
    email_id INTEGER NOT NULL REFERENCES emails(id) ON DELETE CASCADE,
    ai_draft_text TEXT NOT NULL,
    sent_text TEXT NOT NULL,
    similarity DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_draft_feedback_email_id ON draft_feedback(email_id);
```

`backend/migrations/000031_add_draft_feedback.down.sql`:

```sql
-- backend/migrations/000031_add_draft_feedback.down.sql

DROP TABLE draft_feedback;
```

- [ ] **Step 3: Write the model**

`backend/internal/models/draft_feedback.go`:

```go
// backend/internal/models/draft_feedback.go
package models

import "time"

// DraftFeedback logs how much a user changed an AI-generated draft reply
// before sending it, as a lightweight future quality signal. Purely a
// measurement record - nothing reads it to take automatic action.
type DraftFeedback struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	EmailID     uint      `json:"email_id" gorm:"not null;index"`
	AIDraftText string    `json:"ai_draft_text" gorm:"type:text;not null"`
	SentText    string    `json:"sent_text" gorm:"type:text;not null"`
	Similarity  float64   `json:"similarity" gorm:"not null"`
	CreatedAt   time.Time `json:"created_at"`
}

func (DraftFeedback) TableName() string { return "draft_feedback" }
```

- [ ] **Step 4: Build to confirm it compiles**

Run: `docker exec devbridge_backend go build ./...`
Expected: no errors

- [ ] **Step 5: Run the migration against the dev database**

Confirm however this project runs migrations in dev (check for a `migrate` invocation in the backend's startup or a documented make target) actually applies `000031` cleanly, then confirm `\d draft_feedback` in `docker exec devbridge_postgres psql -U devbridge_user -d devbridge` shows the expected columns and the `idx_draft_feedback_email_id` index.

- [ ] **Step 6: Commit**

```bash
git add backend/migrations/000031_add_draft_feedback.up.sql backend/migrations/000031_add_draft_feedback.down.sql backend/internal/models/draft_feedback.go
git commit -m "$(cat <<'EOF'
feat(emails): add draft_feedback table and model

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 8: Wire feedback-similarity logging into `SendEmail`

**Files:**
- Modify: `backend/internal/handlers/email_handler.go:254-353` (the `SendEmail` handler's doc comment and body)

**Interfaces:**
- Consumes: `services.JaccardSimilarity(a, b string) float64` (Task 1); `models.DraftFeedback` (Task 7).
- Produces: nothing new for other tasks - this is the last piece of the feedback-logging flow before the frontend wiring in Task 9, which supplies the new `ai_draft_text` form field this task reads.

No test for this task: it's entirely inside a DB-touching Fiber handler, matching the existing convention (see Task 4's rationale). The spec's "visszacsatolás hasonlóság-számítás" test requirement is satisfied by Task 1's `TestJaccardSimilarity` table - this task calls that same, already-tested function with different (feedback-shaped) inputs, and doesn't introduce any new computation of its own.

- [ ] **Step 1: Update the handler doc comment**

Change the `SendEmail` doc comment to mention the new optional field:

```go
// SendEmail - POST /api/v1/emails/send - új levél vagy válasz küldése
// multipart/form-data: mezők "to", "subject", "body", opcionális
// "in_reply_to_email_id", opcionális "ai_draft_text" (ha a compose egy AI
// válasz-javaslatból indult, visszacsatolás-méréshez), és opcionális
// "files" (max maxAttachmentCount db, max maxAttachmentSize/fájl, csak
// allowedAttachmentMimeTypes típusok).
```

- [ ] **Step 2: Read the new form field**

Add alongside the existing `bodyHTML := c.FormValue("body_html")` line:

```go
	aiDraftText := c.FormValue("ai_draft_text")
```

- [ ] **Step 3: Log feedback after the local mirror row is created**

Change the end of `SendEmail`, after `database.GetDB().Create(&localEmail)` (currently the function ends right after that `if err != nil { log.Printf(...) }` block and the final `return`):

```go
	if err := database.GetDB().Create(&localEmail).Error; err != nil {
		log.Printf("email send: failed to mirror sent message %s locally: %v", gmailMessageID, err)
	} else if strings.TrimSpace(aiDraftText) != "" {
		similarity := services.JaccardSimilarity(aiDraftText, body)
		feedback := models.DraftFeedback{
			EmailID:     localEmail.ID,
			AIDraftText: aiDraftText,
			SentText:    body,
			Similarity:  similarity,
		}
		if err := database.GetDB().Create(&feedback).Error; err != nil {
			log.Printf("email send: failed to log draft feedback for email %d: %v", localEmail.ID, err)
		}
	}

	return c.JSON(models.EmailSendResponse{Success: true, GmailMessageID: gmailMessageID})
```

This only runs `Create(&feedback)` when the local mirror succeeded (so `localEmail.ID` is populated) and `ai_draft_text` was actually sent - matching the spec's "no napló sor if compose didn't start from an AI draft" rule. A feedback-logging failure only logs; it never changes the response, matching the existing local-mirror error-handling pattern immediately above it.

- [ ] **Step 4: Build the whole backend**

Run: `docker exec devbridge_backend go build ./...`
Expected: no errors

- [ ] **Step 5: Run the full backend test suite**

Run: `docker exec devbridge_backend go test ./...`
Expected: all PASS (no new tests in this task; this just confirms nothing broke)

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handlers/email_handler.go
git commit -m "$(cat <<'EOF'
feat(emails): log draft-vs-sent similarity when compose started from an AI draft

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```

---

### Task 9: Frontend - carry the AI draft text through to send for feedback logging

**Files:**
- Modify: `frontend/src/services/emailsService.ts`
- Modify: `frontend/src/app/dashboard/emails/page.tsx`

**Interfaces:**
- Consumes: `ai_draft_text` optional form field (Task 8).
- Produces: `SendEmailRequest.ai_draft_text?: string`. No further consumers - this is the last task.

No test for this task: this codebase has zero `*.test.tsx` files and this plan doesn't introduce that infrastructure, matching the spec's Testing section.

- [ ] **Step 1: Add the field to `SendEmailRequest` and send it when present**

In `frontend/src/services/emailsService.ts`, change the interface:

```ts
export interface SendEmailRequest {
    to: string
    subject: string
    body: string
    body_html?: string
    in_reply_to_email_id?: number
    ai_draft_text?: string
    files?: File[]
}
```

And in `send()`, add alongside the existing `in_reply_to_email_id` handling:

```ts
        if (payload.ai_draft_text) {
            extraFields.ai_draft_text = payload.ai_draft_text
        }
```

- [ ] **Step 2: Track the AI draft's original text for the whole compose session**

In `frontend/src/app/dashboard/emails/page.tsx`, add a new state near `pendingDraftText`:

```ts
    const [pendingDraftText, setPendingDraftText] = useState<string | null>(null)
    const [aiDraftOriginalText, setAiDraftOriginalText] = useState<string | null>(null)
```

- [ ] **Step 3: Reset it at the start of every compose session**

In `openCompose`, reset the new state alongside the existing resets (`setSendError(null)`, `setDraftError(null)`, `setComposeFiles([])`) - `openCompose` is the single entry point for "new email", "reply", and the reply flow inside `handleDraftReply`, so resetting here covers both "compose closed" and "reply target changed":

```ts
    const openCompose = (reply?: EmailDetail) => {
        setSendError(null)
        setDraftError(null)
        setComposeFiles([])
        setAiDraftOriginalText(null)
        if (reply) {
```

- [ ] **Step 4: Capture the draft text when a draft is successfully generated**

In `handleDraftReply`, alongside `setPendingDraftText(res.draft)`:

```ts
            setPendingDraftText(res.draft)
            setAiDraftOriginalText(res.draft)
```

- [ ] **Step 5: Send it along with the rest of the compose payload**

In `handleSend`, add it to the `EmailsService.send` call:

```ts
            const res = await EmailsService.send({
                to: composeTo.trim(),
                subject: composeSubject.trim(),
                body: isEmpty ? '' : (composeEditorRef.current?.getText() ?? ''),
                body_html: isEmpty ? '' : (composeEditorRef.current?.getHTML() ?? ''),
                in_reply_to_email_id: replyToId,
                ai_draft_text: aiDraftOriginalText ?? undefined,
                files: composeFiles,
            })
```

- [ ] **Step 6: Build to confirm no type errors**

Run: `docker exec devbridge_frontend npm run build` (or confirm the dev server, already running under `npm run dev`, recompiles without errors)
Expected: no errors

- [ ] **Step 7: Manual verification in the browser**

Open an inbox email, click the AI reply-suggestion button, let it fill the editor, edit the text a little, and send. Then check `docker exec devbridge_postgres psql -U devbridge_user -d devbridge -c "SELECT email_id, similarity FROM draft_feedback ORDER BY id DESC LIMIT 1;"` and confirm a row was logged with a similarity below 1.0 (since the text was edited). Separately, compose a brand-new email (not from a draft) and send it, then confirm no new `draft_feedback` row appears for it.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/services/emailsService.ts frontend/src/app/dashboard/emails/page.tsx
git commit -m "$(cat <<'EOF'
feat(emails): send the original AI draft text for feedback logging on send

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>
EOF
)"
```
