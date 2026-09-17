# Gmail Integration — Design

Date: 2026-09-17
Status: Approved by user, ready for implementation planning.

## Goal

Let the (single) super admin connect their own Gmail account to the app and
browse received/sent mail from a dedicated "Emails" menu section, and send an
invoice-notice email to a client before issuing an invoice on the project's
invoice page.

## Context and non-goals

This is a single-user feature: only the super admin will connect and use
Gmail through this app. The data model still scopes everything by
`user_id` for straightforward future extension, but no per-team-member
connection UI or access control beyond "admin-only" is built now.

**Non-goals (explicitly out of scope for this spec):**
- Multiple Gmail accounts / per-team-member connections.
- Storing email bodies or attachment contents locally — both are fetched
  live from Gmail on demand (see "Data model").
- Rich compose (HTML editor, drafts, scheduling, threading UI beyond a
  simple reply). Compose/reply is a plain-text textarea only.
- Real-time push (Gmail Pub/Sub webhooks) — a 3-hourly poll is sufficient
  for a single-user internal tool.
- Sending the invoice-notice email is a prerequisite gate for issuing the
  invoice — it explicitly is **not**; the two actions are independent
  buttons so a failed email never blocks billing.

## Existing foundation (verified, reused as-is)

- `time.Ticker`-based background job pattern, already used for monthly
  auto-invoicing (`backend/internal/services/scheduler.go`) — the Gmail
  sync job follows the same shape (its own ticker, its own goroutine
  started from `main.go`).
- `billingo_settings` table stores an operational secret (API key) as a
  plain column with no encryption layer — the Gmail OAuth tokens follow
  the same existing security posture (see "Security").
- Admin-only nav gating pattern in `DashboardNav.tsx`
  (`hasAnyPermission`/`isAdmin`) — reused for the new "Emails" link.
- The project's invoice page
  (`frontend/src/app/dashboard/board/[projectId]/invoice/page.tsx`) already
  has the client list and period selection this feature needs for the
  notice email; no new data is required there beyond what's already loaded.

## Scope

1. Google OAuth2 connect/disconnect flow for one Gmail account.
2. A background sync job (3-hourly) that mirrors message **metadata only**
   (not bodies, not attachment contents) into a local table, using the
   Gmail History API for incremental updates after an initial one-time
   30-day backfill.
3. An "Emails" page: Inbox/Sent tabs backed by the local table, a detail
   view that fetches the full message body live from Gmail when opened,
   and a simple compose/reply flow that sends via the Gmail API.
4. An "Értesítő küldése" (send notice) action on the invoice page that
   emails the selected client via Gmail and records that it was sent for
   the selected project/period.

## Data model (new tables, one migration)

### `gmail_accounts`
- `id`, `user_id` (FK → users, unique — one connection per user, though
  only one row will exist in practice)
- `email_address` (string)
- `access_token`, `refresh_token` (string, plaintext columns — see
  "Security")
- `token_expiry` (timestamp)
- `last_history_id` (string, nullable — Gmail's opaque history cursor;
  null until the initial backfill completes)
- `last_synced_at` (timestamp, nullable)
- `created_at`, `updated_at`

### `emails`
- `id`, `gmail_account_id` (FK → gmail_accounts)
- `gmail_message_id` (string, unique per account — dedup key for sync
  upserts)
- `thread_id` (string)
- `folder` (`inbox`|`sent` — derived from the message's Gmail label ids at
  sync time; a message with neither label, e.g. drafts/spam, is skipped)
- `from_address`, `from_name`, `to_addresses` (comma-joined string is
  enough; no need to normalize into a join table for a read-mostly mailbox
  view)
- `subject`, `snippet`
- `has_attachments` (bool), `attachment_meta` (JSON array of
  `{filename, size, attachment_id}` — no file content, just enough to
  render an icon and drive the live-download endpoint)
- `is_read` (bool, from the `UNREAD` label's absence)
- `received_at` (timestamp — Gmail's internal date)
- `synced_at` (timestamp)
- Index on `(gmail_account_id, folder, received_at desc)` for the list
  queries.

### `invoice_notices`
- `id`, `project_id` (FK → projects), `client_id` (FK → clients)
- `period_start`, `period_end` (date, nullable — mirrors `invoices`'
  period fields; null for fixed-price projects)
- `gmail_message_id` (string — the sent notice, for reference/debugging)
- `sent_by` (FK → users), `sent_at`

Both `.up.sql` and `.down.sql` files, matching the convention used by the
most recent migrations in this codebase (000019–000023).

## Backend

### OAuth connect flow

- `GET /api/v1/gmail/auth-url` — builds the Google consent URL
  (`golang.org/x/oauth2/google` config) with scopes
  `gmail.readonly` + `gmail.send`, `access_type=offline` (to receive a
  refresh token), `prompt=consent`. Admin-gated.
- `GET /api/v1/gmail/callback` — Google redirects here with a `code`;
  exchanged for tokens via the oauth2 client, upserted into
  `gmail_accounts` for the current user. Redirects the browser back to
  `/dashboard/emails`.
- `GET /api/v1/gmail/status` — `{ connected: bool, email_address?: string,
  last_synced_at?: string }`.
- `POST /api/v1/gmail/disconnect` — deletes the `gmail_accounts` row (the
  associated `emails` rows are deleted too via `ON DELETE CASCADE`).

### Sync job (`backend/internal/services/gmail_sync.go`)

Same shape as `scheduler.go`: a ticker firing every 3 hours, plus one run
immediately on startup. For each row in `gmail_accounts`:

1. **No `last_history_id` yet (first sync):** list messages via
   `users.messages.list` with query `after:<unix timestamp 30 days ago>`,
   paging through results. For each message id not already present in
   `emails`, fetch it with `format=metadata` (cheap — headers + label ids
   + snippet, no body) and insert a row. When the list is exhausted, call
   `users.getProfile` to read the account's current `historyId` and store
   it as `last_history_id`.
2. **Has `last_history_id`:** call `users.history.list(startHistoryId=...)`,
   walking pages, collecting `messagesAdded` (insert, same metadata fetch
   as above) and `messagesDeleted` (delete the matching row if present —
   keeps the local mirror honest if the user deletes mail in Gmail).
   Advance `last_history_id` to the response's new `historyId` once fully
   paged through.
3. **History expired (Gmail returns 404 on `history.list`):** this means
   too much time passed since the last successful sync for Gmail to have
   kept the history around (documented as roughly a week; irrelevant at a
   3-hour cadence but handled defensively). Falls back to step 1's
   date-query approach using `last_synced_at` as the cutoff instead of a
   fixed 30 days, then re-establishes `last_history_id` via
   `getProfile` exactly as in step 1.
4. Update `last_synced_at` regardless of which path ran.

A sync failure for one account (e.g. revoked token) is logged and does not
crash the process; `gmail/status` will keep reporting the last known
`last_synced_at` until a sync succeeds again, and the frontend shows a
"reconnect" prompt when Gmail confirms the token is invalid (surfaced via
a `needs_reauth: bool` field on the status response, set when the sync job
gets an `invalid_grant`/401 from Google).

### Email endpoints

- `GET /api/v1/emails?folder=inbox|sent&page=` — paginated list from the
  local `emails` table, ordered by `received_at desc`. No live Gmail call.
- `GET /api/v1/emails/:id` — loads the local row for headers, then calls
  `users.messages.get(format=full)` live against Gmail for the body
  (`text/plain` and `text/html` parts), returned together. Marks the
  local row `is_read=true` (mirrors Gmail's own read state locally only;
  does not push a read-state change back to Gmail, since this is a
  metadata cache, not a two-way sync).
- `GET /api/v1/emails/:id/attachments/:attachmentId` — proxies
  `users.messages.attachments.get` and streams the decoded bytes back with
  the original filename/content-type; nothing is written to disk.
- `POST /api/v1/emails/send` — body `{ to, subject, body, in_reply_to?:
  gmail_message_id }`. Builds a RFC 2822 message (reusing the `in_reply_to`
  message's `Message-ID`/`References` headers when replying, fetched live
  from Gmail), sends via `users.messages.send`, and inserts the resulting
  message into the local `emails` table as `folder='sent'` immediately
  (rather than waiting for the next sync) so it shows up right away.

### Invoice-notice endpoint

`POST /api/v1/projects/:id/invoice-notice`

Request body: `{ client_id, period_start?, period_end? }` (period fields
only for hourly projects, same convention as the existing
`POST /api/v1/projects/:id/invoices`).

Flow: loads the project and client (validating the client is attached to
the project, same check `invoices.go` already does), builds a plain-text
notice email from a fixed template (project name, period if hourly,
amount due if easily computed the same way `invoice_calc.go` already
does, otherwise omitted), sends it via the same Gmail send path as
`/emails/send`, and inserts an `invoice_notices` row. Returns 502 with the
Gmail error message on send failure — no `invoice_notices` row is written
in that case, so the invoice page won't show a false "sent" state.

`GET /api/v1/projects/:id/invoice-notices?period_start=&period_end=` —
used by the invoice page to check whether a notice was already sent for
the currently-selected period, driving the "Elküldve: {date}" label.

### New dependencies

`golang.org/x/oauth2`, `golang.org/x/oauth2/google`,
`google.golang.org/api/gmail/v1`, `google.golang.org/api/option` — the
official Google API client libraries; unavoidable for a real OAuth2 +
Gmail API integration, and standard/well-maintained enough that hand-
rolling the equivalent REST calls would be strictly worse (more code, more
places to get token refresh or pagination wrong).

### New environment variables

`GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET`,
`GOOGLE_OAUTH_REDIRECT_URL` — created by the user in Google Cloud Console
(OAuth consent screen + credentials), added to `backend/.env`. Exact setup
steps are provided during implementation, not part of this spec.

### Permissions

New permission `gmail.manage`, granted only to `super_admin` (matching
"only I will use this"). All `/api/v1/gmail/*` and `/api/v1/emails/*`
routes require it. The invoice-notice endpoint reuses the existing
`invoices.create` permission (it's part of the invoicing flow, not the
mailbox feature).

## Frontend

- **Nav:** new "Emails" link in `DashboardNav.tsx`, gated on `gmail.manage`
  (same `hasAnyPermission` pattern as the other links).
- **`/dashboard/emails` page:**
  - Not connected: a "Gmail csatlakoztatása" card/button that redirects to
    `GET /api/v1/gmail/auth-url`'s returned URL.
  - Connected: Inbox/Sent tabs, a list (sender, subject, snippet, date,
    unread styling, attachment icon), and a detail panel/modal that lazy-
    loads the full body + attachments on open. A "Válasz" button on an
    open message and an "Új levél" button open the same simple compose
    modal (to, subject, body textarea).
  - Visual polish (the "extra UI" ask — card-based inbox layout, unread
    emphasis, transitions) is worked out during the `frontend-design`
    implementation phase; this spec only fixes the functional shape above.
- **Invoice page
  (`/dashboard/board/[projectId]/invoice/page.tsx`):** a new "Értesítő
  küldése" button next to the existing "Számla kiállítása" button,
  independent of it. Shows "Elküldve: {sent_at}" once a notice exists for
  the selected client/period (fetched via the new
  `GET .../invoice-notices` endpoint alongside the page's existing data
  loading).

## Security

- OAuth tokens are stored as plaintext columns in `gmail_accounts`,
  matching this codebase's existing posture for the Billingo API key
  (`billingo_settings.api_key`) — no secrets-manager or column encryption
  exists anywhere in this app, and introducing one solely for this
  feature would be new infrastructure disproportionate to a single-user
  internal tool. If this changes later, both tables would migrate
  together.
- All Gmail/email routes require the `gmail.manage` permission
  (super_admin only).
- The Gmail access/refresh tokens never reach the browser; all Gmail API
  calls happen server-side, including attachment downloads (proxied, not
  redirected).

## Testing plan

- Backend: unit tests for the sync job's upsert/dedup logic and the
  history-expired fallback path, using a fake Gmail client (interface
  around the subset of methods used, so the real
  `google.golang.org/api/gmail/v1` client isn't needed in tests).
  `go build`/`go vet`/`gofmt` in the documented backend container, as with
  prior work in this codebase.
- The actual OAuth consent screen cannot be driven non-interactively; the
  user performs the real Google sign-in/consent in a browser once
  implementation is ready, and confirms the connect/status/disconnect
  flow and that sync populates `emails`.
- Any real email send (compose, reply, or invoice notice) during
  development/testing is only ever sent to an address the user provides
  (e.g. their own) — never to a real client — per this project's standing
  rule against touching external providers/production-adjacent data
  without explicit approval for each such action.
