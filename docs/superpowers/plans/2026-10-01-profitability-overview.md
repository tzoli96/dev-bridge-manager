# Jövedelmezőség – 1. szakasz: valódi óradíj (Overview) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ügyfelenként és projektenként megmutatni a névleges és a valódi óradíjat (a levelezés és megbeszélés becsült idejével együtt), az "áremelés-jelölt" jelzéssel, szerkeszthető becslési beállításokkal.

**Architecture:** A számítás tiszta Go függvényekben él (`BuildOverview`), amit egy vékony SQL-betöltő táplál a meglévő táblákból, igény szerint (a `client_health` mintájára, nincs pillanatkép és nincs ütemező). Négy végpont a `/api/v1/profitability` alatt, két új jogosultsággal. A felület egy új `Jövedelmezőség` oldal egy táblázatos áttekintéssel és egy beállítás-panellel.

**Tech Stack:** Go 1.x + Fiber + GORM + golang-migrate (backend konténer), Next.js + TypeScript (frontend konténer), PostgreSQL.

**Spec:** `docs/superpowers/specs/2026-10-01-profitability-design.md` (ez a terv annak **1. szállítási szakaszát** valósítja meg; a 2–4. szakasz külön tervet kap az 1. szakasz után.)

## Global Constraints

- **Nincs beépített adó- vagy járulékszabály, nincs alapértelmezett kulcs.** (Ebben a szakaszban adó egyáltalán nincs.)
- A számla `amount` **nettónak** számít; ez a Task 0 előfeltétele, nem feltevés.
- Bevétel = `invoices` sorok `status = 'created'` és `billingo_invoice_id <> ''`, a `COALESCE(period_end, created_at)` hónapjához rendelve (fix áras számlának nem mindig van `period_end`-je).
- Csak a levelezés `ugyfel` és `szamla` kategóriája számít; irány a `folder` (`inbox` / `sent`).
- Jogok: `profitability.read` és `profitability.manage`; alapból csak `super_admin` és `admin`. A projekttagság **nem** elég.
- Minden ellenőrzés (go test, gofmt, go vet, tsc, eslint, psql) **a projekt konténereiben** fut. Parancsok a repo gyökeréből: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c '…'` (hasonlóan `frontend`, `postgres`). Host-oldali futtatás tilos.
- Adatbázis-ellenőrzés csak a lokális dev adatbázison, visszagörgetett tranzakcióban, ideiglenes tesztadattal. Valós vagy megosztott adat nem érintett.
- **Commit, merge és push csak a felhasználó kifejezett kérésére.** A terv commit-lépései az ő jóváhagyásáig nem futnak le. Mindig csak a felsorolt fájlokat `git add`-eld.
- A working tree-ben **nem commitolt, idegen munka** van (Jira: `activity_digest_handler.go`, `jira_*`, `task_comment_handler.go`, `models/kanban.go`, `services/jira_*`, `migrations/000040_*`, `docs/.../marketing-contacts.md`). Ezekhez nem nyúlunk, és nem kerülhetnek commitba.
- Munkabranch: `feat/profitability-overview`.

## File Structure

| Fájl | Felelősség |
|---|---|
| `backend/migrations/000044_add_profitability.{up,down}.sql` | Jogok, `profit_settings`, `client_meeting_allowances` |
| `backend/internal/models/profitability.go` | `ProfitSettings`, `ClientMeetingAllowance`, kérés-típusok |
| `backend/internal/services/profitability_calc.go` | Tiszta számítás: `MonthWindow`, `AllocateHours`, `BuildOverview` |
| `backend/internal/services/profitability_calc_test.go` | A fentiek tesztjei |
| `backend/internal/services/profitability.go` | SQL-betöltő: `LoadOverview(months)` |
| `backend/internal/handlers/profitability_handler.go` | Végpontok és validáció |
| `backend/internal/handlers/profitability_handler_test.go` | Validáció tesztjei |
| `backend/internal/routes/profitability_routes.go` | Útvonalak és jogosultságok |
| `backend/internal/routes/routes.go` | Bekötés (1 sor) |
| `frontend/src/services/profitabilityService.ts` | API kliens és típusok |
| `frontend/src/app/dashboard/profitability/page.tsx` | Áttekintő oldal |
| `frontend/src/components/dashboard/DashboardNav.tsx` | Menüpont (1 elem) |

---

### Task 0: Előfeltétel – nettó vagy bruttó a számla összege?

Ez **kézi ellenőrzés**, kód nem készül. A többi task nem indulhat el addig, amíg az eredmény nincs rögzítve.

- [ ] **Step 1: Kérd el a felhasználótól egy valós, kiállított Billingo-számla nettó és bruttó összegét, és a hozzá tartozó `invoices.amount` értéket.**

A kérdés: az `invoices.amount` a nettóval egyezik-e? (A kódban az összeg `óra × díj`, ÁFA nem látszik: `backend/internal/services/invoice_calc.go`.)

- [ ] **Step 2: Rögzítsd az eredményt a spec "Nyitott pontok" részében.**

Ha **nettó**: a terv változtatás nélkül megy tovább.
Ha **bruttó**: állj meg, és jelezd a felhasználónak; a bevétel-számítást (Task 3) ÁFA-levonással kell kiegészíteni, ami új döntés.

- [ ] **Step 3: Hozd létre a munkabranchet.**

Run: `git switch -c feat/profitability-overview`
Expected: `Switched to a new branch 'feat/profitability-overview'` (a nem commitolt idegen változtatások a working tree-ben maradnak).

---

### Task 1: Migráció – jogok, beállítások, megbeszélés-átalány

**Files:**
- Create: `backend/migrations/000044_add_profitability.up.sql`
- Create: `backend/migrations/000044_add_profitability.down.sql`

**Interfaces:**
- Produces: tábla `profit_settings(id=1, minutes_per_inbound_email, minutes_per_outbound_email, default_capacity_hours_per_month, underpriced_ratio_threshold, updated_at)`; tábla `client_meeting_allowances(id, client_id UNIQUE, hours_per_month, updated_at)`; jogok `profitability.read`, `profitability.manage`.

- [ ] **Step 1: Írd meg az up migrációt**

```sql
CREATE TABLE profit_settings (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    minutes_per_inbound_email NUMERIC(6,2) NOT NULL DEFAULT 5,
    minutes_per_outbound_email NUMERIC(6,2) NOT NULL DEFAULT 10,
    default_capacity_hours_per_month NUMERIC(6,2) NOT NULL DEFAULT 120,
    underpriced_ratio_threshold NUMERIC(4,2) NOT NULL DEFAULT 0.60,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO profit_settings (id) VALUES (1);

CREATE TABLE client_meeting_allowances (
    id SERIAL PRIMARY KEY,
    client_id INTEGER NOT NULL UNIQUE REFERENCES clients(id) ON DELETE CASCADE,
    hours_per_month NUMERIC(6,2) NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO permissions (name, display_name, description, resource, action) VALUES
    ('profitability.read', 'View Profitability', 'Can view client/project profitability', 'profitability', 'read'),
    ('profitability.manage', 'Manage Profitability', 'Can edit profitability settings and meeting allowances', 'profitability', 'manage');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name IN ('super_admin', 'admin')
  AND p.name IN ('profitability.read', 'profitability.manage');
```

A `0.60` küszöb és az `5` / `10` perc **szerkeszthető kezdőérték**, nem tény: a küszöböt a Task 7 valós adaton felülvizsgálja.

- [ ] **Step 2: Írd meg a down migrációt**

```sql
DELETE FROM role_permissions
WHERE permission_id IN (SELECT id FROM permissions WHERE name IN ('profitability.read', 'profitability.manage'));
DELETE FROM permissions WHERE name IN ('profitability.read', 'profitability.manage');
DROP TABLE IF EXISTS client_meeting_allowances;
DROP TABLE IF EXISTS profit_settings;
```

- [ ] **Step 3: Próbáld ki visszagörgetett tranzakcióban: up, down, újra up**

Run:
```bash
{ echo "BEGIN;"; cat backend/migrations/000044_add_profitability.up.sql; echo "SELECT count(*) AS perms FROM permissions WHERE name LIKE 'profitability.%';"; cat backend/migrations/000044_add_profitability.down.sql; echo "SELECT to_regclass('profit_settings') IS NULL AS down_ok;"; cat backend/migrations/000044_add_profitability.up.sql; echo "SELECT count(*) AS settings_rows FROM profit_settings;"; echo "ROLLBACK;"; } | docker compose -f .docker/docker-compose.yml exec -T postgres psql -U devbridge_user -d devbridge -v ON_ERROR_STOP=1 2>&1 | grep -vE "^(INSERT|DELETE|DROP|CREATE|BEGIN|ROLLBACK)"
```
Expected: `perms = 2`, `down_ok = t`, `settings_rows = 1`, hiba nélkül.

- [ ] **Step 4: Commit** (csak a felhasználó kérésére)

```bash
git add backend/migrations/000044_add_profitability.up.sql backend/migrations/000044_add_profitability.down.sql
git commit -m "feat(profitability): add settings, meeting allowances and permissions"
```

---

### Task 2: Modellek

**Files:**
- Create: `backend/internal/models/profitability.go`

**Interfaces:**
- Produces: `models.ProfitSettings`, `models.ClientMeetingAllowance`, `models.ProfitSettingsRequest`, `models.MeetingAllowanceRequest`.

- [ ] **Step 1: Írd meg a modelleket**

```go
// backend/internal/models/profitability.go
package models

import "time"

type ProfitSettings struct {
	ID                           uint      `json:"-" gorm:"primaryKey"`
	MinutesPerInboundEmail       float64   `json:"minutes_per_inbound_email"`
	MinutesPerOutboundEmail      float64   `json:"minutes_per_outbound_email"`
	DefaultCapacityHoursPerMonth float64   `json:"default_capacity_hours_per_month"`
	UnderpricedRatioThreshold    float64   `json:"underpriced_ratio_threshold"`
	UpdatedAt                    time.Time `json:"updated_at"`
}

func (ProfitSettings) TableName() string { return "profit_settings" }

type ClientMeetingAllowance struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	ClientID      uint      `json:"client_id" gorm:"uniqueIndex;not null"`
	HoursPerMonth float64   `json:"hours_per_month"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (ClientMeetingAllowance) TableName() string { return "client_meeting_allowances" }

type ProfitSettingsRequest struct {
	MinutesPerInboundEmail       float64 `json:"minutes_per_inbound_email"`
	MinutesPerOutboundEmail      float64 `json:"minutes_per_outbound_email"`
	DefaultCapacityHoursPerMonth float64 `json:"default_capacity_hours_per_month"`
	UnderpricedRatioThreshold    float64 `json:"underpriced_ratio_threshold"`
}

type MeetingAllowanceRequest struct {
	HoursPerMonth float64 `json:"hours_per_month"`
}
```

- [ ] **Step 2: Fordítás ellenőrzése**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go build ./internal/models/'`
Expected: nincs kimenet, kilépési kód 0.

- [ ] **Step 3: Commit** (csak kérésre)

```bash
git add backend/internal/models/profitability.go
git commit -m "feat(profitability): add settings and meeting allowance models"
```

---

### Task 3: Tiszta számítás (TDD)

**Files:**
- Create: `backend/internal/services/profitability_calc.go`
- Test: `backend/internal/services/profitability_calc_test.go`

**Interfaces:**
- Consumes: `models.ProfitSettings` (Task 2).
- Produces:
  - `func MonthWindow(now time.Time, n int) (months []string, from, to time.Time)`
  - `func AllocateHours(hours float64, weights map[uint]float64, fallbackIDs []uint) map[uint]float64`
  - `func BuildOverview(in OverviewInput) Overview`
  - típusok: `InvoiceMonth`, `ProjectHoursMonth`, `EmailMonth`, `ProjectClientLink`, `OverviewInput`, `RateRow`, `Overview`.

- [ ] **Step 1: Írd meg a bukó teszteket**

```go
// backend/internal/services/profitability_calc_test.go
package services

import (
	"math"
	"testing"
	"time"

	"dev-bridge-manager/internal/models"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 0.01 }

func TestMonthWindow(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	months, from, to := MonthWindow(now, 3)
	want := []string{"2026-07", "2026-08", "2026-09"}
	for i, m := range want {
		if months[i] != m {
			t.Fatalf("months[%d] = %s, want %s", i, months[i], m)
		}
	}
	if !from.Equal(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("from = %v", from)
	}
	if !to.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("to = %v", to)
	}
}

func TestAllocateHoursProportionalToWeights(t *testing.T) {
	got := AllocateHours(10, map[uint]float64{1: 300, 2: 100}, []uint{1, 2})
	if !approx(got[1], 7.5) || !approx(got[2], 2.5) {
		t.Fatalf("got %v", got)
	}
}

func TestAllocateHoursFallsBackToEqualSplit(t *testing.T) {
	got := AllocateHours(10, nil, []uint{1, 2})
	if !approx(got[1], 5) || !approx(got[2], 5) {
		t.Fatalf("got %v", got)
	}
}

func TestAllocateHoursNothingToAllocateTo(t *testing.T) {
	if got := AllocateHours(10, nil, nil); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func baseSettings() models.ProfitSettings {
	return models.ProfitSettings{
		MinutesPerInboundEmail:    5,
		MinutesPerOutboundEmail:   10,
		UnderpricedRatioThreshold: 0.9,
	}
}

func TestBuildOverviewRealRateIncludesEmailAndMeetingTime(t *testing.T) {
	pid := uint(7)
	in := OverviewInput{
		Months:       []string{"2026-08", "2026-09"},
		Invoices:     []InvoiceMonth{{ProjectID: 7, ClientID: 1, Month: "2026-08", Amount: 100000}, {ProjectID: 7, ClientID: 1, Month: "2026-09", Amount: 100000}},
		ProjectHours: []ProjectHoursMonth{{ProjectID: 7, Month: "2026-08", Hours: 10}, {ProjectID: 7, Month: "2026-09", Hours: 10}},
		Emails: []EmailMonth{
			{ClientID: 1, ProjectID: &pid, Month: "2026-08", Folder: "inbox", Count: 12}, // 12*5min = 1h
			{ClientID: 1, ProjectID: &pid, Month: "2026-08", Folder: "sent", Count: 6},   // 6*10min = 1h
		},
		Links:        []ProjectClientLink{{ProjectID: 7, ClientID: 1}},
		ClientNames:  map[uint]string{1: "PIXEL"},
		ProjectNames: map[uint]string{7: "Mentorfy"},
		MeetingHours: map[uint]float64{1: 1}, // 1h/month * 2 months
		Settings:     baseSettings(),
	}

	out := BuildOverview(in)
	if len(out.Clients) != 1 {
		t.Fatalf("clients = %d", len(out.Clients))
	}
	c := out.Clients[0]
	if !approx(c.Revenue, 200000) || !approx(c.LoggedHours, 20) || !approx(c.EmailHours, 2) || !approx(c.MeetingHours, 2) {
		t.Fatalf("client row = %+v", c)
	}
	if c.NominalRate == nil || !approx(*c.NominalRate, 10000) {
		t.Fatalf("nominal = %v", c.NominalRate)
	}
	if c.RealRate == nil || !approx(*c.RealRate, 200000.0/24.0) {
		t.Fatalf("real = %v", c.RealRate)
	}
	if c.Ratio == nil || !approx(*c.Ratio, (200000.0/24.0)/10000.0) {
		t.Fatalf("ratio = %v", c.Ratio)
	}
	if !c.UnderpricedCandidate {
		t.Fatalf("ratio %.3f under threshold 0.9 should flag", *c.Ratio)
	}
	if !c.LowData {
		t.Fatalf("2 months of data should be flagged low-data")
	}

	// Project rows carry email time but no meeting flat-rate (it is per client).
	p := out.Projects[0]
	if !approx(p.EmailHours, 2) || !approx(p.MeetingHours, 0) {
		t.Fatalf("project row = %+v", p)
	}
}

func TestBuildOverviewNoHoursMeansNoRates(t *testing.T) {
	in := OverviewInput{
		Months:      []string{"2026-09"},
		Invoices:    []InvoiceMonth{{ProjectID: 7, ClientID: 1, Month: "2026-09", Amount: 50000}},
		ClientNames: map[uint]string{1: "A"}, ProjectNames: map[uint]string{7: "P"},
		Settings: baseSettings(),
	}
	c := BuildOverview(in).Clients[0]
	if c.NominalRate != nil || c.RealRate != nil || c.Ratio != nil || c.UnderpricedCandidate {
		t.Fatalf("expected no rates without hours, got %+v", c)
	}
}

func TestBuildOverviewAllocatesHoursByInvoicedShare(t *testing.T) {
	in := OverviewInput{
		Months: []string{"2026-09"},
		Invoices: []InvoiceMonth{
			{ProjectID: 7, ClientID: 1, Month: "2026-09", Amount: 300},
			{ProjectID: 7, ClientID: 2, Month: "2026-09", Amount: 100},
		},
		ProjectHours: []ProjectHoursMonth{{ProjectID: 7, Month: "2026-09", Hours: 8}},
		Links:        []ProjectClientLink{{ProjectID: 7, ClientID: 1}, {ProjectID: 7, ClientID: 2}},
		ClientNames:  map[uint]string{1: "A", 2: "B"}, ProjectNames: map[uint]string{7: "P"},
		Settings: baseSettings(),
	}
	out := BuildOverview(in)
	hours := map[uint]float64{}
	for _, c := range out.Clients {
		hours[c.ID] = c.LoggedHours
	}
	if !approx(hours[1], 6) || !approx(hours[2], 2) {
		t.Fatalf("hours = %v", hours)
	}
}

func TestBuildOverviewWarnsAboutUnattributedHours(t *testing.T) {
	in := OverviewInput{
		Months:       []string{"2026-09"},
		ProjectHours: []ProjectHoursMonth{{ProjectID: 7, Month: "2026-09", Hours: 4}},
		ProjectNames: map[uint]string{7: "P"},
		Settings:     baseSettings(),
	}
	out := BuildOverview(in)
	if len(out.Warnings) != 1 {
		t.Fatalf("warnings = %v", out.Warnings)
	}
}

func TestBuildOverviewSkipsClientsWithoutActivity(t *testing.T) {
	in := OverviewInput{
		Months:       []string{"2026-09"},
		ClientNames:  map[uint]string{1: "Idle"},
		MeetingHours: map[uint]float64{1: 2},
		Settings:     baseSettings(),
	}
	if out := BuildOverview(in); len(out.Clients) != 0 {
		t.Fatalf("idle client should not appear, got %+v", out.Clients)
	}
}
```

- [ ] **Step 2: Futtasd, és ellenőrizd, hogy elbukik**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/services/ -run "MonthWindow|AllocateHours|BuildOverview" 2>&1 | tail -15'`
Expected: FAIL (`undefined: MonthWindow`, `undefined: BuildOverview` …).

- [ ] **Step 3: Írd meg a minimális implementációt**

```go
// backend/internal/services/profitability_calc.go
package services

import (
	"fmt"
	"sort"
	"time"

	"dev-bridge-manager/internal/models"
)

// lowDataMonths is the minimum number of months with activity before a row's
// rates are presented without a "low data" warning.
const lowDataMonths = 3

type InvoiceMonth struct {
	ProjectID uint
	ClientID  uint
	Month     string // "YYYY-MM"
	Amount    float64
}

type ProjectHoursMonth struct {
	ProjectID uint
	Month     string
	Hours     float64
}

type EmailMonth struct {
	ClientID  uint
	ProjectID *uint
	Month     string
	Folder    string // "inbox" | "sent"
	Count     int
}

type ProjectClientLink struct {
	ProjectID uint
	ClientID  uint
}

type OverviewInput struct {
	Months       []string
	Invoices     []InvoiceMonth
	ProjectHours []ProjectHoursMonth
	Emails       []EmailMonth
	Links        []ProjectClientLink
	ClientNames  map[uint]string
	ProjectNames map[uint]string
	MeetingHours map[uint]float64 // per client, hours per month
	Settings     models.ProfitSettings
}

// RateRow is one client or project line of the overview. Rates are pointers
// so "no hours logged" serialises as null instead of a misleading 0 or Inf.
type RateRow struct {
	ID                   uint     `json:"id"`
	Name                 string   `json:"name"`
	Revenue              float64  `json:"revenue"`
	LoggedHours          float64  `json:"logged_hours"`
	EmailHours           float64  `json:"email_hours"`
	MeetingHours         float64  `json:"meeting_hours"`
	MeetingHoursPerMonth float64  `json:"meeting_hours_per_month"`
	NominalRate          *float64 `json:"nominal_rate"`
	RealRate             *float64 `json:"real_rate"`
	Ratio                *float64 `json:"ratio"`
	UnderpricedCandidate bool     `json:"underpriced_candidate"`
	MonthsWithData       int      `json:"months_with_data"`
	LowData              bool     `json:"low_data"`
}

type Overview struct {
	Months   []string              `json:"months"`
	Clients  []RateRow             `json:"clients"`
	Projects []RateRow             `json:"projects"`
	Warnings []string              `json:"warnings"`
	Settings models.ProfitSettings `json:"settings"`
}

// MonthWindow returns the n complete calendar months before now's month, as
// "YYYY-MM" labels, plus the [from, to) instants covering them.
func MonthWindow(now time.Time, n int) (months []string, from, to time.Time) {
	to = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	from = to.AddDate(0, -n, 0)
	for i := 0; i < n; i++ {
		months = append(months, from.AddDate(0, i, 0).Format("2006-01"))
	}
	return months, from, to
}

// AllocateHours splits hours across ids proportionally to weights; when all
// weights are zero it splits equally across fallbackIDs; with neither it
// returns an empty map (the caller reports the hours as unattributed).
func AllocateHours(hours float64, weights map[uint]float64, fallbackIDs []uint) map[uint]float64 {
	out := map[uint]float64{}
	var total float64
	for _, w := range weights {
		total += w
	}
	if total > 0 {
		for id, w := range weights {
			out[id] = hours * w / total
		}
		return out
	}
	if len(fallbackIDs) == 0 {
		return out
	}
	share := hours / float64(len(fallbackIDs))
	for _, id := range fallbackIDs {
		out[id] = share
	}
	return out
}

func BuildOverview(in OverviewInput) Overview {
	clients := map[uint]*RateRow{}
	projects := map[uint]*RateRow{}
	clientMonths := map[uint]map[string]bool{}
	projectMonths := map[uint]map[string]bool{}

	rowFor := func(rows map[uint]*RateRow, names map[uint]string, id uint) *RateRow {
		r, ok := rows[id]
		if !ok {
			name := names[id]
			if name == "" {
				name = fmt.Sprintf("#%d", id)
			}
			r = &RateRow{ID: id, Name: name}
			rows[id] = r
		}
		return r
	}
	mark := func(m map[uint]map[string]bool, id uint, month string) {
		if m[id] == nil {
			m[id] = map[string]bool{}
		}
		m[id][month] = true
	}

	linksByProject := map[uint][]uint{}
	for _, l := range in.Links {
		linksByProject[l.ProjectID] = append(linksByProject[l.ProjectID], l.ClientID)
	}

	// project -> month -> client -> invoiced amount, used as allocation weights.
	weights := map[uint]map[string]map[uint]float64{}
	for _, inv := range in.Invoices {
		rowFor(clients, in.ClientNames, inv.ClientID).Revenue += inv.Amount
		rowFor(projects, in.ProjectNames, inv.ProjectID).Revenue += inv.Amount
		mark(clientMonths, inv.ClientID, inv.Month)
		mark(projectMonths, inv.ProjectID, inv.Month)
		if weights[inv.ProjectID] == nil {
			weights[inv.ProjectID] = map[string]map[uint]float64{}
		}
		if weights[inv.ProjectID][inv.Month] == nil {
			weights[inv.ProjectID][inv.Month] = map[uint]float64{}
		}
		weights[inv.ProjectID][inv.Month][inv.ClientID] += inv.Amount
	}

	var unattributed float64
	for _, ph := range in.ProjectHours {
		rowFor(projects, in.ProjectNames, ph.ProjectID).LoggedHours += ph.Hours
		mark(projectMonths, ph.ProjectID, ph.Month)
		alloc := AllocateHours(ph.Hours, weights[ph.ProjectID][ph.Month], linksByProject[ph.ProjectID])
		var allocated float64
		for clientID, h := range alloc {
			rowFor(clients, in.ClientNames, clientID).LoggedHours += h
			mark(clientMonths, clientID, ph.Month)
			allocated += h
		}
		unattributed += ph.Hours - allocated
	}

	for _, e := range in.Emails {
		minutes := in.Settings.MinutesPerInboundEmail
		if e.Folder == "sent" {
			minutes = in.Settings.MinutesPerOutboundEmail
		}
		h := float64(e.Count) * minutes / 60
		rowFor(clients, in.ClientNames, e.ClientID).EmailHours += h
		if e.ProjectID != nil {
			rowFor(projects, in.ProjectNames, *e.ProjectID).EmailHours += h
		}
	}

	finalize := func(rows map[uint]*RateRow, months map[uint]map[string]bool, withMeetings bool) []RateRow {
		out := make([]RateRow, 0, len(rows))
		for id, r := range rows {
			if withMeetings {
				r.MeetingHoursPerMonth = in.MeetingHours[id]
				r.MeetingHours = in.MeetingHours[id] * float64(len(in.Months))
			}
			if r.LoggedHours > 0 {
				v := r.Revenue / r.LoggedHours
				r.NominalRate = &v
			}
			if total := r.LoggedHours + r.EmailHours + r.MeetingHours; total > 0 {
				v := r.Revenue / total
				r.RealRate = &v
			}
			if r.NominalRate != nil && r.RealRate != nil && *r.NominalRate > 0 {
				v := *r.RealRate / *r.NominalRate
				r.Ratio = &v
				r.UnderpricedCandidate = v < in.Settings.UnderpricedRatioThreshold
			}
			r.MonthsWithData = len(months[id])
			r.LowData = r.MonthsWithData < lowDataMonths
			out = append(out, *r)
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Revenue != out[j].Revenue {
				return out[i].Revenue > out[j].Revenue
			}
			return out[i].ID < out[j].ID
		})
		return out
	}

	warnings := []string{}
	if unattributed > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"%.1f naplózott óra nem rendelhető ügyfélhez (nincs számla vagy projekt–ügyfél kapcsolat)", unattributed))
	}

	return Overview{
		Months:   in.Months,
		Clients:  finalize(clients, clientMonths, true),
		Projects: finalize(projects, projectMonths, false),
		Warnings: warnings,
		Settings: in.Settings,
	}
}
```

- [ ] **Step 4: Futtasd, és ellenőrizd, hogy átmegy**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -l internal/services/profitability_calc*.go; go vet ./internal/services/ && go test ./internal/services/ -run "MonthWindow|AllocateHours|BuildOverview" -v 2>&1 | grep -E "^(--- |PASS|FAIL|ok)"'`
Expected: a `gofmt -l` nem listáz fájlt; minden teszt `--- PASS`, a végén `ok`.

- [ ] **Step 5: Commit** (csak kérésre)

```bash
git add backend/internal/services/profitability_calc.go backend/internal/services/profitability_calc_test.go
git commit -m "feat(profitability): add pure overview calculation"
```

---

### Task 4: SQL-betöltő

**Files:**
- Create: `backend/internal/services/profitability.go`

**Interfaces:**
- Consumes: `MonthWindow`, `BuildOverview`, `OverviewInput`, `Overview` (Task 3); `models.ProfitSettings`, `models.ClientMeetingAllowance` (Task 2).
- Produces: `func LoadOverview(months int) (Overview, error)`.

- [ ] **Step 1: Írd meg a betöltőt**

```go
// backend/internal/services/profitability.go
package services

import (
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"gorm.io/gorm"
)

// LoadOverview gathers the monthly aggregates for the last `months` complete
// months from the existing tables and hands them to the pure BuildOverview.
func LoadOverview(months int) (Overview, error) {
	db := database.GetDB()
	window, from, to := MonthWindow(time.Now(), months)

	var settings models.ProfitSettings
	if err := db.First(&settings, 1).Error; err != nil {
		return Overview{}, err
	}

	invoices, err := loadInvoiceMonths(db, from, to)
	if err != nil {
		return Overview{}, err
	}
	hours, err := loadProjectHoursMonths(db, from, to)
	if err != nil {
		return Overview{}, err
	}
	emails, err := loadEmailMonths(db, from, to)
	if err != nil {
		return Overview{}, err
	}

	var links []ProjectClientLink
	if err := db.Table("project_clients").Select("project_id, client_id").Scan(&links).Error; err != nil {
		return Overview{}, err
	}

	clientNames, err := loadNames(db, "clients")
	if err != nil {
		return Overview{}, err
	}
	projectNames, err := loadNames(db, "projects")
	if err != nil {
		return Overview{}, err
	}

	var allowances []models.ClientMeetingAllowance
	if err := db.Find(&allowances).Error; err != nil {
		return Overview{}, err
	}
	meetingHours := make(map[uint]float64, len(allowances))
	for _, a := range allowances {
		meetingHours[a.ClientID] = a.HoursPerMonth
	}

	return BuildOverview(OverviewInput{
		Months:       window,
		Invoices:     invoices,
		ProjectHours: hours,
		Emails:       emails,
		Links:        links,
		ClientNames:  clientNames,
		ProjectNames: projectNames,
		MeetingHours: meetingHours,
		Settings:     settings,
	}), nil
}

// Same "real invoice" definition as the overdue check (client_health.go):
// status created and an actual Billingo document. Fixed-price invoices may
// have no period_end, so they fall back to the creation date.
func loadInvoiceMonths(db *gorm.DB, from, to time.Time) ([]InvoiceMonth, error) {
	var rows []InvoiceMonth
	err := db.Table("invoices").
		Select("project_id, client_id, to_char(COALESCE(period_end, created_at), 'YYYY-MM') AS month, SUM(amount) AS amount").
		Where("status = ? AND billingo_invoice_id <> ? AND COALESCE(period_end, created_at) >= ? AND COALESCE(period_end, created_at) < ?",
			"created", "", from, to).
		Group("project_id, client_id, to_char(COALESCE(period_end, created_at), 'YYYY-MM')").
		Scan(&rows).Error
	return rows, err
}

func loadProjectHoursMonths(db *gorm.DB, from, to time.Time) ([]ProjectHoursMonth, error) {
	var rows []ProjectHoursMonth
	err := db.Table("task_time_entries").
		Select("tasks.project_id AS project_id, to_char(task_time_entries.date, 'YYYY-MM') AS month, SUM(task_time_entries.hours) AS hours").
		Joins("JOIN tasks ON tasks.id = task_time_entries.task_id").
		Where("task_time_entries.date >= ? AND task_time_entries.date < ?", from, to).
		Group("tasks.project_id, to_char(task_time_entries.date, 'YYYY-MM')").
		Scan(&rows).Error
	return rows, err
}

func loadEmailMonths(db *gorm.DB, from, to time.Time) ([]EmailMonth, error) {
	var rows []EmailMonth
	err := db.Table("emails").
		Select("client_id, project_id, to_char(received_at, 'YYYY-MM') AS month, folder, COUNT(*) AS count").
		Where("client_id IS NOT NULL AND category IN ? AND received_at >= ? AND received_at < ?",
			[]string{models.EmailCategoryClient, models.EmailCategoryBilling}, from, to).
		Group("client_id, project_id, to_char(received_at, 'YYYY-MM'), folder").
		Scan(&rows).Error
	return rows, err
}

func loadNames(db *gorm.DB, table string) (map[uint]string, error) {
	var rows []struct {
		ID   uint
		Name string
	}
	if err := db.Table(table).Select("id, name").Scan(&rows).Error; err != nil {
		return nil, err
	}
	names := make(map[uint]string, len(rows))
	for _, r := range rows {
		names[r.ID] = r.Name
	}
	return names, nil
}
```

- [ ] **Step 2: Fordítás és vet**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -l internal/services/profitability.go; go vet ./internal/services/ && echo vet-ok'`
Expected: `vet-ok`, a `gofmt -l` nem listáz fájlt.

- [ ] **Step 3: Ellenőrizd az SQL-t a dev adatbázison, visszagörgetett tranzakcióban**

A repóban nincs adatbázisos tesztinfrastruktúra, ezért ugyanazt az SQL-t futtatjuk ideiglenes tesztadattal. Feltételezi, hogy van legalább egy `users` sor és a Task 1 migráció lefutott (a backend újraindul és lefuttatja; ellenőrizd: `SELECT version FROM schema_migrations` → `44`).

Run:
```bash
cat <<'SQL' | docker compose -f .docker/docker-compose.yml exec -T postgres psql -U devbridge_user -d devbridge -v ON_ERROR_STOP=1 2>&1 | grep -vE "^(INSERT|BEGIN|ROLLBACK)"
BEGIN;
INSERT INTO clients (name) VALUES ('__tmp_client');
INSERT INTO projects (name, created_by) SELECT '__tmp_project', id FROM users ORDER BY id LIMIT 1;
INSERT INTO project_clients (project_id, client_id)
  SELECT p.id, c.id FROM projects p, clients c WHERE p.name='__tmp_project' AND c.name='__tmp_client';
INSERT INTO invoices (project_id, client_id, status, billingo_invoice_id, amount, period_end, created_by)
  SELECT p.id, c.id, 'created', 'B-1', 100000, date_trunc('month', now()) - interval '1 day', p.created_by
  FROM projects p, clients c WHERE p.name='__tmp_project' AND c.name='__tmp_client';
INSERT INTO invoices (project_id, client_id, status, billingo_invoice_id, amount, period_end, created_by)
  SELECT p.id, c.id, 'failed', '', 999999, date_trunc('month', now()) - interval '1 day', p.created_by
  FROM projects p, clients c WHERE p.name='__tmp_project' AND c.name='__tmp_client';
SELECT 'invoices (varhato: 100000, a failed kimarad)' AS ellenorzes, project_id, client_id,
       to_char(COALESCE(period_end, created_at), 'YYYY-MM') AS month, SUM(amount) AS amount
FROM invoices
WHERE status = 'created' AND billingo_invoice_id <> ''
  AND COALESCE(period_end, created_at) >= date_trunc('month', now()) - interval '1 month'
  AND COALESCE(period_end, created_at) <  date_trunc('month', now())
  AND project_id = (SELECT id FROM projects WHERE name='__tmp_project')
GROUP BY project_id, client_id, to_char(COALESCE(period_end, created_at), 'YYYY-MM');
ROLLBACK;
SQL
```
Expected: egyetlen sor, `amount = 100000`; a `failed` számla nem szerepel.

- [ ] **Step 4: Commit** (csak kérésre)

```bash
git add backend/internal/services/profitability.go
git commit -m "feat(profitability): add overview SQL loader"
```

---

### Task 5: Handler, útvonalak, bekötés (TDD a validációra)

**Files:**
- Create: `backend/internal/handlers/profitability_handler.go`
- Test: `backend/internal/handlers/profitability_handler_test.go`
- Create: `backend/internal/routes/profitability_routes.go`
- Modify: `backend/internal/routes/routes.go` (a `SetupProjectEnvironmentRoutes(v1)` sor mellé)

**Interfaces:**
- Consumes: `services.LoadOverview` (Task 4); `models.ProfitSettings`, `ProfitSettingsRequest`, `MeetingAllowanceRequest`, `ClientMeetingAllowance` (Task 2).
- Produces: `GET /profitability/overview?months=`, `GET /profitability/settings`, `PUT /profitability/settings`, `PUT /profitability/clients/:id/meeting-allowance`; függvények `validateProfitSettings(models.ProfitSettingsRequest) string`, `validateMeetingAllowance(models.MeetingAllowanceRequest) string`, `parseOverviewMonths(string) (int, bool)`.

- [ ] **Step 1: Írd meg a bukó teszteket**

```go
// backend/internal/handlers/profitability_handler_test.go
package handlers

import (
	"testing"

	"dev-bridge-manager/internal/models"
)

func TestParseOverviewMonths(t *testing.T) {
	cases := []struct {
		in     string
		want   int
		wantOK bool
	}{
		{"", 3, true},
		{"6", 6, true},
		{"24", 24, true},
		{"0", 0, false},
		{"25", 0, false},
		{"abc", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseOverviewMonths(tc.in)
		if ok != tc.wantOK || (ok && got != tc.want) {
			t.Fatalf("parseOverviewMonths(%q) = %d,%v want %d,%v", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestValidateProfitSettings(t *testing.T) {
	good := models.ProfitSettingsRequest{
		MinutesPerInboundEmail: 5, MinutesPerOutboundEmail: 10,
		DefaultCapacityHoursPerMonth: 120, UnderpricedRatioThreshold: 0.6,
	}
	if msg := validateProfitSettings(good); msg != "" {
		t.Fatalf("good settings rejected: %s", msg)
	}
	bad := []models.ProfitSettingsRequest{
		{MinutesPerInboundEmail: -1, MinutesPerOutboundEmail: 10, DefaultCapacityHoursPerMonth: 120, UnderpricedRatioThreshold: 0.6},
		{MinutesPerInboundEmail: 5, MinutesPerOutboundEmail: 241, DefaultCapacityHoursPerMonth: 120, UnderpricedRatioThreshold: 0.6},
		{MinutesPerInboundEmail: 5, MinutesPerOutboundEmail: 10, DefaultCapacityHoursPerMonth: 745, UnderpricedRatioThreshold: 0.6},
		{MinutesPerInboundEmail: 5, MinutesPerOutboundEmail: 10, DefaultCapacityHoursPerMonth: 120, UnderpricedRatioThreshold: 0},
		{MinutesPerInboundEmail: 5, MinutesPerOutboundEmail: 10, DefaultCapacityHoursPerMonth: 120, UnderpricedRatioThreshold: 1.6},
	}
	for i, b := range bad {
		if msg := validateProfitSettings(b); msg == "" {
			t.Fatalf("bad settings #%d accepted: %+v", i, b)
		}
	}
}

func TestValidateMeetingAllowance(t *testing.T) {
	if msg := validateMeetingAllowance(models.MeetingAllowanceRequest{HoursPerMonth: 0}); msg != "" {
		t.Fatalf("zero rejected: %s", msg)
	}
	if msg := validateMeetingAllowance(models.MeetingAllowanceRequest{HoursPerMonth: 744}); msg != "" {
		t.Fatalf("744 rejected: %s", msg)
	}
	if msg := validateMeetingAllowance(models.MeetingAllowanceRequest{HoursPerMonth: -1}); msg == "" {
		t.Fatal("negative accepted")
	}
	if msg := validateMeetingAllowance(models.MeetingAllowanceRequest{HoursPerMonth: 745}); msg == "" {
		t.Fatal("745 accepted")
	}
}
```

- [ ] **Step 2: Futtasd, és ellenőrizd, hogy elbukik**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/handlers/ -run "ParseOverviewMonths|ValidateProfitSettings|ValidateMeetingAllowance" 2>&1 | tail -8'`
Expected: FAIL (`undefined: parseOverviewMonths` …).

- [ ] **Step 3: Írd meg a handlert**

```go
// backend/internal/handlers/profitability_handler.go
package handlers

import (
	"strconv"
	"time"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm/clause"
)

type ProfitabilityHandler struct{}

func NewProfitabilityHandler() *ProfitabilityHandler {
	return &ProfitabilityHandler{}
}

// parseOverviewMonths returns the requested window length; empty means the
// default of 3 months, anything outside 1..24 is rejected.
func parseOverviewMonths(raw string) (int, bool) {
	if raw == "" {
		return 3, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 24 {
		return 0, false
	}
	return n, true
}

func validateProfitSettings(req models.ProfitSettingsRequest) string {
	if req.MinutesPerInboundEmail < 0 || req.MinutesPerInboundEmail > 240 ||
		req.MinutesPerOutboundEmail < 0 || req.MinutesPerOutboundEmail > 240 {
		return "Email minutes must be between 0 and 240"
	}
	if req.DefaultCapacityHoursPerMonth < 0 || req.DefaultCapacityHoursPerMonth > 744 {
		return "Capacity must be between 0 and 744 hours per month"
	}
	if req.UnderpricedRatioThreshold <= 0 || req.UnderpricedRatioThreshold > 1.5 {
		return "Threshold must be greater than 0 and at most 1.5"
	}
	return ""
}

func validateMeetingAllowance(req models.MeetingAllowanceRequest) string {
	if req.HoursPerMonth < 0 || req.HoursPerMonth > 744 {
		return "Hours per month must be between 0 and 744"
	}
	return ""
}

// GetOverview - GET /api/v1/profitability/overview?months=
func (h *ProfitabilityHandler) GetOverview(c *fiber.Ctx) error {
	months, ok := parseOverviewMonths(c.Query("months"))
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "months must be between 1 and 24"})
	}
	overview, err := services.LoadOverview(months)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading profitability overview"})
	}
	return c.JSON(overview)
}

// GetSettings - GET /api/v1/profitability/settings
func (h *ProfitabilityHandler) GetSettings(c *fiber.Ctx) error {
	var settings models.ProfitSettings
	if err := database.GetDB().First(&settings, 1).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading settings"})
	}
	return c.JSON(settings)
}

// UpdateSettings - PUT /api/v1/profitability/settings
func (h *ProfitabilityHandler) UpdateSettings(c *fiber.Ctx) error {
	var req models.ProfitSettingsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateProfitSettings(req); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}

	var settings models.ProfitSettings
	if err := database.GetDB().First(&settings, 1).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading settings"})
	}
	settings.MinutesPerInboundEmail = req.MinutesPerInboundEmail
	settings.MinutesPerOutboundEmail = req.MinutesPerOutboundEmail
	settings.DefaultCapacityHoursPerMonth = req.DefaultCapacityHoursPerMonth
	settings.UnderpricedRatioThreshold = req.UnderpricedRatioThreshold
	settings.UpdatedAt = time.Now()
	if err := database.GetDB().Save(&settings).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating settings"})
	}
	return c.JSON(settings)
}

// PutMeetingAllowance - PUT /api/v1/profitability/clients/:id/meeting-allowance
func (h *ProfitabilityHandler) PutMeetingAllowance(c *fiber.Ctx) error {
	clientID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid client ID"})
	}
	var req models.MeetingAllowanceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateMeetingAllowance(req); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}

	var client models.Client
	if err := database.GetDB().Select("id").First(&client, clientID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Client not found"})
	}

	allowance := models.ClientMeetingAllowance{
		ClientID:      uint(clientID),
		HoursPerMonth: req.HoursPerMonth,
		UpdatedAt:     time.Now(),
	}
	err = database.GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "client_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"hours_per_month", "updated_at"}),
	}).Create(&allowance).Error
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error saving meeting allowance"})
	}
	return c.JSON(allowance)
}
```

- [ ] **Step 4: Írd meg az útvonalakat és kösd be**

```go
// backend/internal/routes/profitability_routes.go
package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

func SetupProfitabilityRoutes(api fiber.Router) {
	h := handlers.NewProfitabilityHandler()

	g := api.Group("/profitability")
	g.Use(middleware.JWTMiddleware())
	g.Use(middleware.RequirePermission("profitability.read"))

	// GET /api/v1/profitability/overview - Ügyfél- és projekt-óradíjak (névleges és valódi)
	g.Get("/overview", h.GetOverview)

	// GET /api/v1/profitability/settings - Levelezés-becslés beállításai
	g.Get("/settings", h.GetSettings)

	// PUT /api/v1/profitability/settings - Beállítások mentése
	g.Put("/settings", middleware.RequirePermission("profitability.manage"), h.UpdateSettings)

	// PUT /api/v1/profitability/clients/:id/meeting-allowance - Havi megbeszélés-átalány
	g.Put("/clients/:id/meeting-allowance", middleware.RequirePermission("profitability.manage"), h.PutMeetingAllowance)
}
```

Módosítás a `backend/internal/routes/routes.go`-ban, a `SetupProjectEnvironmentRoutes(v1)` sor után:

```go
	SetupProfitabilityRoutes(v1)         // Client/project profitability - profitability.read/manage permissions
```

- [ ] **Step 5: Futtasd a teszteket, a gofmt-et és a vet-et**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -l internal/handlers/profitability_handler*.go internal/routes/profitability_routes.go internal/routes/routes.go; go vet ./internal/handlers/ ./internal/routes/ && go test ./internal/handlers/ -run "ParseOverviewMonths|ValidateProfitSettings|ValidateMeetingAllowance" -v 2>&1 | grep -E "^(--- |PASS|FAIL|ok)"'`
Expected: `gofmt -l` nem listáz fájlt, a tesztek `--- PASS`, a végén `ok`.

- [ ] **Step 6: Ellenőrizd, hogy az útvonalak be vannak kötve és védettek**

(A hot reload újraépíti a backendet; várj ~10 másodpercet.)

Run: `for p in overview settings; do curl -s -o /dev/null -w "GET /profitability/$p token nélkül: HTTP %{http_code}\n" http://localhost:8080/api/v1/profitability/$p; done`
Expected: mindkettő `HTTP 401`.

- [ ] **Step 7: Commit** (csak kérésre)

```bash
git add backend/internal/handlers/profitability_handler.go backend/internal/handlers/profitability_handler_test.go backend/internal/routes/profitability_routes.go backend/internal/routes/routes.go
git commit -m "feat(profitability): add overview, settings and meeting allowance endpoints"
```

---

### Task 6: Frontend – service, oldal, menüpont

**Files:**
- Create: `frontend/src/services/profitabilityService.ts`
- Create: `frontend/src/app/dashboard/profitability/page.tsx`
- Modify: `frontend/src/components/dashboard/DashboardNav.tsx` (új elem a `Marketing lista` elem után; az import sorba `TrendingUp`)

**Interfaces:**
- Consumes: a Task 5 végpontjai; `useAuth` (`@/hooks/auth/use-auth`), `hasPermission` (`@/utils/permissions`).
- Produces: `profitabilityService.overview(months)`, `.getSettings()`, `.updateSettings(data)`, `.setMeetingAllowance(clientId, hours)`.

- [ ] **Step 1: Írd meg a service-t**

```ts
// frontend/src/services/profitabilityService.ts
import { apiClient } from '@/lib/api';

export interface RateRow {
    id: number;
    name: string;
    revenue: number;
    logged_hours: number;
    email_hours: number;
    meeting_hours: number;
    meeting_hours_per_month: number;
    nominal_rate: number | null;
    real_rate: number | null;
    ratio: number | null;
    underpriced_candidate: boolean;
    months_with_data: number;
    low_data: boolean;
}

export interface ProfitSettings {
    minutes_per_inbound_email: number;
    minutes_per_outbound_email: number;
    default_capacity_hours_per_month: number;
    underpriced_ratio_threshold: number;
    updated_at: string;
}

export type ProfitSettingsInput = Omit<ProfitSettings, 'updated_at'>;

export interface ProfitabilityOverview {
    months: string[];
    clients: RateRow[];
    projects: RateRow[];
    warnings: string[];
    settings: ProfitSettings;
}

export const profitabilityService = {
    async overview(months: number): Promise<ProfitabilityOverview> {
        return apiClient.get(`/profitability/overview?months=${months}`);
    },

    async getSettings(): Promise<ProfitSettings> {
        return apiClient.get('/profitability/settings');
    },

    async updateSettings(data: ProfitSettingsInput): Promise<ProfitSettings> {
        return apiClient.put('/profitability/settings', data);
    },

    async setMeetingAllowance(clientId: number, hoursPerMonth: number): Promise<void> {
        return apiClient.put(`/profitability/clients/${clientId}/meeting-allowance`, {
            hours_per_month: hoursPerMonth,
        });
    },
};
```

- [ ] **Step 2: Írd meg az oldalt**

```tsx
// frontend/src/app/dashboard/profitability/page.tsx
'use client';

import React from 'react';
import { useAuth } from '@/hooks/auth/use-auth';
import { hasPermission } from '@/utils/permissions';
import {
    profitabilityService,
    ProfitabilityOverview,
    ProfitSettingsInput,
    RateRow,
} from '@/services/profitabilityService';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Modal } from '@/components/ui/modal';
import EmptyState from '@/components/ui/EmptyState';
import LoadingState from '@/components/ui/LoadingState';
import ErrorState from '@/components/ui/ErrorState';
import { TrendingUp, Settings2, Pencil } from 'lucide-react';

const errorMessage = (err: unknown) => (err instanceof Error ? err.message : String(err));

const formatHuf = (value: number | null) =>
    value === null ? '—' : `${Math.round(value).toLocaleString('hu-HU')} Ft`;
const formatHours = (value: number) => `${value.toFixed(1)} ó`;
const formatRatio = (value: number | null) => (value === null ? '—' : `${Math.round(value * 100)}%`);

export default function ProfitabilityPage() {
    const { user } = useAuth();
    const canManage = hasPermission(user, 'profitability.manage');

    const [months, setMonths] = React.useState(3);
    const [data, setData] = React.useState<ProfitabilityOverview | null>(null);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);
    const [actionError, setActionError] = React.useState<string | null>(null);

    const [showSettings, setShowSettings] = React.useState(false);
    const [settingsForm, setSettingsForm] = React.useState<ProfitSettingsInput | null>(null);
    const [isSaving, setIsSaving] = React.useState(false);

    const [meetingClient, setMeetingClient] = React.useState<RateRow | null>(null);
    const [meetingHours, setMeetingHours] = React.useState('0');

    const load = React.useCallback(async () => {
        try {
            setLoading(true);
            setError(null);
            const overview = await profitabilityService.overview(months);
            setData(overview);
            const { updated_at: _updatedAt, ...input } = overview.settings;
            setSettingsForm(input);
        } catch (err: unknown) {
            setError(errorMessage(err));
        } finally {
            setLoading(false);
        }
    }, [months]);

    React.useEffect(() => {
        load();
    }, [load]);

    const saveSettings = async () => {
        if (!settingsForm) return;
        setIsSaving(true);
        try {
            await profitabilityService.updateSettings(settingsForm);
            setActionError(null);
            setShowSettings(false);
            await load();
        } catch (err: unknown) {
            setActionError(errorMessage(err));
        } finally {
            setIsSaving(false);
        }
    };

    const saveMeeting = async () => {
        if (!meetingClient) return;
        setIsSaving(true);
        try {
            await profitabilityService.setMeetingAllowance(meetingClient.id, Number(meetingHours));
            setActionError(null);
            setMeetingClient(null);
            await load();
        } catch (err: unknown) {
            setActionError(errorMessage(err));
        } finally {
            setIsSaving(false);
        }
    };

    const setField = (key: keyof ProfitSettingsInput) => (v: string) =>
        setSettingsForm((f) => (f ? { ...f, [key]: Number(v) } : f));

    const renderTable = (title: string, rows: RateRow[], clientTable: boolean) => (
        <section className="mb-8">
            <h2 className="text-lg font-semibold text-foreground mb-3">{title}</h2>
            <div className="overflow-x-auto bg-card border border-border rounded-lg">
                <table className="w-full text-sm">
                    <thead className="text-left text-muted-foreground border-b border-border">
                        <tr>
                            <th className="p-3">Név</th>
                            <th className="p-3 text-right">Bevétel</th>
                            <th className="p-3 text-right">Naplózott</th>
                            <th className="p-3 text-right">Levelezés</th>
                            {clientTable && <th className="p-3 text-right">Megbeszélés</th>}
                            <th className="p-3 text-right">Névleges óradíj</th>
                            <th className="p-3 text-right">Valódi óradíj</th>
                            <th className="p-3 text-right">Arány</th>
                        </tr>
                    </thead>
                    <tbody>
                        {rows.map((row) => (
                            <tr key={row.id} className="border-b border-border last:border-0">
                                <td className="p-3 text-foreground">
                                    {row.name}
                                    {row.underpriced_candidate && (
                                        <span className="ml-2 text-xs bg-destructive/10 text-destructive rounded-full px-2 py-0.5">
                                            áremelés-jelölt
                                        </span>
                                    )}
                                    {row.low_data && (
                                        <span
                                            className="ml-2 text-xs bg-muted text-muted-foreground rounded-full px-2 py-0.5"
                                            title={`Csak ${row.months_with_data} hónapnyi adat van, a szám tájékoztató jellegű`}
                                        >
                                            kevés adat
                                        </span>
                                    )}
                                </td>
                                <td className="p-3 text-right">{formatHuf(row.revenue)}</td>
                                <td className="p-3 text-right">{formatHours(row.logged_hours)}</td>
                                <td className="p-3 text-right">{formatHours(row.email_hours)}</td>
                                {clientTable && (
                                    <td className="p-3 text-right">
                                        {formatHours(row.meeting_hours)}
                                        {canManage && (
                                            <Button
                                                variant="ghost"
                                                size="icon-sm"
                                                icon={Pencil}
                                                onClick={() => {
                                                    setMeetingClient(row);
                                                    setMeetingHours(String(row.meeting_hours_per_month));
                                                    setActionError(null);
                                                }}
                                            />
                                        )}
                                    </td>
                                )}
                                <td className="p-3 text-right">{formatHuf(row.nominal_rate)}</td>
                                <td className="p-3 text-right font-medium">{formatHuf(row.real_rate)}</td>
                                <td className="p-3 text-right">{formatRatio(row.ratio)}</td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>
        </section>
    );

    return (
        <div className="p-6 max-w-6xl mx-auto">
            <div className="flex items-center justify-between mb-6">
                <h1 className="text-2xl font-semibold text-foreground flex items-center gap-2">
                    <TrendingUp size={22} /> Jövedelmezőség
                </h1>
                <div className="flex items-center gap-2">
                    <select
                        value={months}
                        onChange={(e) => setMonths(Number(e.target.value))}
                        className="border border-border rounded-md bg-background text-sm px-2 py-1.5"
                    >
                        {[3, 6, 12].map((m) => (
                            <option key={m} value={m}>
                                Utolsó {m} hónap
                            </option>
                        ))}
                    </select>
                    {canManage && (
                        <Button variant="secondary" icon={Settings2} onClick={() => setShowSettings(true)}>
                            Beállítások
                        </Button>
                    )}
                </div>
            </div>

            {actionError && (
                <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm mb-4">
                    {actionError}
                </div>
            )}

            {loading && <LoadingState message="Jövedelmezőség betöltése..." />}
            {!loading && error && <ErrorState error={error} onRetry={load} />}

            {!loading && !error && data && (
                <>
                    <p className="text-xs text-muted-foreground mb-4">
                        A valódi óradíj a bevételt a naplózott órákkal, a levelezés becsült idejével
                        ({data.settings.minutes_per_inbound_email} perc/bejövő, {data.settings.minutes_per_outbound_email}{' '}
                        perc/kimenő levél) és a megbeszélés-átalánnyal osztja. Az áremelés-jelölt küszöb:{' '}
                        {Math.round(data.settings.underpriced_ratio_threshold * 100)}%.
                    </p>

                    {data.warnings.map((w) => (
                        <div
                            key={w}
                            className="bg-muted border border-border text-foreground px-3 py-2 rounded text-sm mb-4"
                        >
                            {w}
                        </div>
                    ))}

                    {data.clients.length === 0 && data.projects.length === 0 ? (
                        <EmptyState
                            icon="files"
                            title="Nincs adat az időszakban"
                            description="Nincs kiállított számla, naplózott óra vagy ügyfélhez rendelt levél az utolsó teljes hónapokban."
                        />
                    ) : (
                        <>
                            {renderTable('Ügyfelek', data.clients, true)}
                            {renderTable('Projektek', data.projects, false)}
                            <p className="text-xs text-muted-foreground">
                                A projekt-sorok nem tartalmazzák a megbeszélés-átalányt, mert az ügyfélenként adható meg.
                            </p>
                        </>
                    )}
                </>
            )}

            <Modal isOpen={showSettings} onClose={() => setShowSettings(false)} title="Számítási beállítások" size="sm">
                {settingsForm && (
                    <div className="space-y-3 mt-2">
                        <Input
                            label="Perc / bejövő levél"
                            type="number"
                            value={settingsForm.minutes_per_inbound_email}
                            onChange={setField('minutes_per_inbound_email')}
                        />
                        <Input
                            label="Perc / kimenő levél"
                            type="number"
                            value={settingsForm.minutes_per_outbound_email}
                            onChange={setField('minutes_per_outbound_email')}
                        />
                        <Input
                            label="Kapacitás (óra / hó)"
                            type="number"
                            value={settingsForm.default_capacity_hours_per_month}
                            onChange={setField('default_capacity_hours_per_month')}
                        />
                        <Input
                            label="Áremelés-jelölt küszöb (0–1.5, pl. 0.6 = 60%)"
                            type="number"
                            value={settingsForm.underpriced_ratio_threshold}
                            onChange={setField('underpriced_ratio_threshold')}
                        />
                        <div className="flex justify-end gap-2 pt-2">
                            <Button variant="secondary" onClick={() => setShowSettings(false)}>
                                Mégse
                            </Button>
                            <Button onClick={saveSettings} loading={isSaving}>
                                Mentés
                            </Button>
                        </div>
                    </div>
                )}
            </Modal>

            <Modal
                isOpen={meetingClient !== null}
                onClose={() => setMeetingClient(null)}
                title={`Megbeszélés-átalány — ${meetingClient?.name ?? ''}`}
                size="sm"
            >
                <div className="space-y-3 mt-2">
                    <Input
                        label="Óra / hónap"
                        type="number"
                        value={meetingHours}
                        onChange={setMeetingHours}
                    />
                    <div className="flex justify-end gap-2 pt-2">
                        <Button variant="secondary" onClick={() => setMeetingClient(null)}>
                            Mégse
                        </Button>
                        <Button onClick={saveMeeting} loading={isSaving}>
                            Mentés
                        </Button>
                    </div>
                </div>
            </Modal>
        </div>
    );
}
```

- [ ] **Step 3: Add hozzá a menüpontot**

A `DashboardNav.tsx` import sorában bővítsd a `lucide-react` importot `TrendingUp`-pal, és a `Marketing lista` elem **után** add hozzá:

```tsx
        {
            href: '/dashboard/profitability',
            label: 'Jövedelmezőség',
            icon: TrendingUp,
            show: hasAnyPermission(user, ['profitability.read']),
        },
```

- [ ] **Step 4: Típusellenőrzés és lint (csak az új/módosított fájlokra értelmezve)**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cd /app && npx tsc --noEmit -p . 2>&1 | grep -E "profitability|DashboardNav" || echo "tsc: nincs hiba az érintett fájlokban"; npx eslint src/services/profitabilityService.ts src/app/dashboard/profitability/page.tsx src/components/dashboard/DashboardNav.tsx 2>&1 | tail -15'`
Expected: `tsc: nincs hiba az érintett fájlokban`, az eslint kimenete üres. (Az 23 már meglévő `tsc` hiba más fájlokban nem ennek a tervnek a része.) Ha az `_updatedAt` változó eslintet dob (`no-unused-vars`), cseréld az objektum-destrukturálást egy kifejezett mezőkiosztásra.

- [ ] **Step 5: Commit** (csak kérésre)

```bash
git add frontend/src/services/profitabilityService.ts frontend/src/app/dashboard/profitability/page.tsx frontend/src/components/dashboard/DashboardNav.tsx
git commit -m "feat(profitability): add profitability overview page"
```

---

### Task 7: Végellenőrzés valós adaton

- [ ] **Step 1: Futtasd a teljes szakasz tesztjeit konténerben**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/services/ ./internal/handlers/ ./internal/middleware/ 2>&1 | tail -8'`
Expected: mindhárom csomag `ok`. Ha egy **régi** teszt bukik, külön jelöld pre-existing-ként, ne keverd az újakkal.

- [ ] **Step 2: Nézd meg az oldalt böngészőben (Playwright vagy kézzel)**

Jelentkezz be egy `admin` / `super_admin` felhasználóval a `http://localhost:3010`-en, nyisd meg a **Jövedelmezőség** menüpontot. Ellenőrizd: betölt; a táblák és a figyelmeztetések megjelennek; a Beállítások és a megbeszélés-átalány mentése működik; a menüpont egy `user` szerepkörű felhasználónál nem látszik.

- [ ] **Step 3: Vesd össze egy ügyfél számát kézzel**

Válassz ki egy ügyfelet, számold ki kézzel: bevétel ÷ (naplózott óra + levelek × perc ÷ 60 + átalány × hónapok). Ha eltér az oldal számától, a hiba a számításban van, ne a bizalomban.

- [ ] **Step 4: Javasolj küszöböt valós adat alapján**

Nézd meg az arányok eloszlását, és javasold a felhasználónak az `underpriced_ratio_threshold` értékét (a kezdő `0.60` csak szerkeszthető alapérték). Ha a rendszerben továbbra is kevés adat van (jelenleg egy projekt), ezt mondd ki, és ne állíts pontosságot.

- [ ] **Step 5: Rögzítsd a nyitott pontokat a specben**

Frissítsd a spec "Nyitott pontok" részét: a nettó/bruttó eredménye (Task 0), a több ügyfeles óraelosztás tapasztalata, a küszöb javasolt értéke.

---

## Self-Review

**Spec-lefedettség (csak az 1. szakasz):**
- Migrációk (jogok, beállítások, átalány) → Task 1 ✔
- `profit_settings` / `client_meeting_allowances` modellek → Task 2 ✔
- Valódi és névleges óradíj, arány, áremelés-jelölt, kevés adat jelzés → Task 3 ✔
- Órák szétosztása több ügyfél között a számlázott összegek arányában → Task 3 (`AllocateHours`) ✔
- Csak `ugyfel` / `szamla` kategória, irány a `folder` szerint → Task 4 ✔
- `GET /overview`, `GET/PUT /settings`, `PUT /clients/:id/meeting-allowance` → Task 5 ✔
- Jogok (`profitability.read` / `manage`, csak admin) → Task 1 + 5 ✔
- Áttekintés felület, "mivel számolt" szöveg, menüpont → Task 6 ✔
- Előfeltétel (nettó/bruttó) → Task 0 ✔
- Nem ennek a tervnek a része (a spec 2–4. szakasza): `forecast`, paraméterkészletek, forgatókönyvek, összehasonlítás, sablonok.

**Eltérések a spectől (a felhasználónak jelezni):**
1. A **projekt-sorok nem tartalmazzák a megbeszélés-átalányt**, mert az ügyfélenként adható meg. A spec "projektenként és ügyfelenként" képlete ennyiben szűkül.
2. A bevétel hónapja `COALESCE(period_end, created_at)`, mert fix áras számlának nem mindig van `period_end`-je. A spec csak a `period_end`-et írta.
3. A `GET /clients/:id/meeting-allowance` nincs: az érték az `overview` ügyfélsoraiban (`meeting_hours_per_month`) jön, így nincs külön olvasó végpont. Csak olyan ügyfélnél állítható az átalány a felületről, akinek van tevékenysége az időszakban.
4. A `profit_settings` `default_capacity_hours_per_month` oszlopa itt még nem hat a számításra (a 3. szakasz használja); a Beállítások űrlapon már szerkeszthető.

**Placeholder-ellenőrzés:** nincs TBD/TODO; minden kódlépésnél teljes kód van. A `0.60` küszöb és az `5`/`10` perc szerkeszthető kezdőérték, a Task 7 felülvizsgálja.

**Típus-konzisztencia:** `RateRow` mezői (`nominal_rate`, `real_rate`, `ratio`, `underpriced_candidate`, `months_with_data`, `low_data`, `meeting_hours_per_month`) megegyeznek a Go JSON-címkéivel és a TypeScript interfésszel. A függvénynevek (`MonthWindow`, `AllocateHours`, `BuildOverview`, `LoadOverview`, `parseOverviewMonths`, `validateProfitSettings`, `validateMeetingAllowance`) minden taskban azonosak.
