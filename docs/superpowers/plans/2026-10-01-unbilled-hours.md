# Számlázatlan órák oldal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Egy oldal, ami projektenként és ügyfelenként megmutatja a még nem kiszámlázott órákat, külön kiemelve a most számlázandókat (kész feladatok és az elmúlt hónapok órái), becsült összeggel, és a meglévő számlakiállító oldalra visz.

**Architecture:** Új, önálló szolgáltatás-fájl: egy tiszta számító függvény (`BuildUnbilledHours`) és egy vékony SQL-betöltő (`LoadUnbilledHours`), a Jövedelmezőség modul mintájára. A „kiszámlázott" fogalmat a meglévő `InvoicedPeriodsByProject` és `IsDateInvoiced` adja, nem írjuk újra. Egy új végpont (`GET /api/v1/billing/unbilled-hours`) a meglévő `checkInvoiceAccess` szabállyal, és egy új oldal a Számlázás alatt. Nincs migráció, nincs új tábla, nincs új jog.

**Tech Stack:** Go + Fiber + GORM (backend konténer), Next.js + TypeScript (frontend konténer), PostgreSQL.

**Spec:** nincs külön spec-fájl; a követelményeket a brainstorming döntései rögzítik (lent, „Döntések").

## Global Constraints

- **„Kiszámlázott" óra:** az órabejegyzés dátuma beleesik a projekt egy `status = 'created'`, `pricing_type = 'hourly'` számlájának `[period_start, period_end]` időszakába. Ezt kizárólag a meglévő `services.InvoicedPeriodsByProject` és `services.IsDateInvoiced` határozza meg. Új definíciót nem írunk.
- **Kategóriák** (egy bejegyzés pontosan egybe kerül, csak óradíjas projekten):
  - kiszámlázott: lásd fent;
  - **számlázandó:** nincs kiszámlázva, **és** (a feladata egy `is_done` oszlopban van **vagy** a dátuma az aktuális naptári hónap első napja előtt van);
  - **folyamatban:** nincs kiszámlázva, a feladat nincs kész oszlopban, és a dátuma az aktuális hónapban (vagy később) van.
- **Összeg** = számlázandó óra × a projekt `hourly_rate`-je. Ha az óradíj hiányzik, az összeg `null`, és figyelmeztetés jelenik meg.
- **Fix áras és hobbi projekt:** szerepel a listában a rögzített órákkal, de számlázandó/folyamatban értéke és összege nincs (`hourly_based = false`), és nem számít bele az összesítőkbe.
- **Ügyfél-hozzárendelés projektenként:** a projekt `auto_invoice_client_id`-ja; ennek hiányában az egyetlen hozzárendelt ügyfél (`project_clients`); egyébként a projekt a „Nincs kijelölt számlázandó ügyfél" csoportba kerül, figyelmeztetéssel. Egy projekt egy csoportban szerepel, nincs dupla számolás.
- **Mely projektek szerepelnek:** amelyiknek van legalább egy pozitív óraszámú bejegyzése (az archivált feladatok órái is számítanak, mert a számla is beleszámítja őket; minden projektállapot). Az óradíjas projekt, aminek nincs számlázandó és nincs folyamatban lévő órája, kimarad a listából, és csak egy számlálóban jelenik meg („N projekt teljesen ki van számlázva").
- **Jogosultság:** a végpont a meglévő `checkInvoiceAccess(permissionService, userID, "invoices.read")` szabályt használja (jog **vagy** admin/super_admin/manager szerepkör), mint a többi számlázó végpont. A menüpont/oldal elérése a Számlázás oldal szabályát követi.
- **Nincs adatmódosítás:** az oldal csak olvas. Számlát nem állít ki.
- Minden ellenőrzés (go test, gofmt, go vet, tsc, eslint, psql) **a projekt konténereiben** fut, a repo gyökeréből: `docker compose -f .docker/docker-compose.yml exec -T backend|frontend|postgres sh -c '…'`. Host-oldali futtatás tilos.
- Adatbázis-ellenőrzés csak a lokális dev adatbázison; írás csak **visszagörgetett tranzakcióban**, ideiglenes tesztadattal. Valós vagy megosztott adat nem módosulhat.
- **Commit, merge és push csak a felhasználó kifejezett kérésére.** A terv commit-lépései addig nem futnak le. `git add` csak a felsorolt fájlokra.
- A working tree-ben **idegen, nem commitolt munka** van (Jira: `activity_digest_handler.go`, `jira_*`, `task_comment_handler.go`, `models/kanban.go`, `services/jira_*`, `migrations/000040_*`, `docs/.../marketing-contacts.md`). Ezekhez nem nyúlunk.
- Új függőség tilos. `gofmt -w` minden új vagy módosított Go fájlon kötelező, majd `gofmt -l` üres. Az ideiglenes tesztfájlok neve `zz_tmp_*`; jelentés előtt törlendők (`ls … | grep -c zz_tmp` → 0).
- Munkabranch: `feat/unbilled-hours` (a `main`-ről).

## Döntések (Rulings a tervben; a felhasználónak jelezni)

1. **„Elmúlt hónap" = az előző naptári hónap és minden korábbi** (a határ az aktuális hónap 1. napja, UTC dátumként). A havi automatikus számlázás is így értelmezi.
2. **A kiszámlázott állapot időszak-alapú,** nem bejegyzésenkénti: egy részben átfedő számla minden átfedő nap óráját kiszámláznak tekinti (ugyanaz a szabály, mint a kanban kártya ikonja és a számlakiállító oldal figyelmeztetése).
3. **A „kész" a kanban oszlop `is_done` jelzőjén múlik** (az 1. szakasz hibajavításával bevezetett fogalom), nem a feladat `status` mezőjén. Egy kézzel létrehozott, nem `is_done` jelölésű „Kész" oszlop nem számít késznek.
4. **A teljesen kiszámlázott óradíjas projekt kimarad** a listából (zaj), de számlálóban látszik.
5. **Az „ügyfél" a számlázandó ügyfél, nem a projekt összes ügyfele** (lásd a Global Constraints-ben).
6. **Az oldal a `/dashboard/billing/unbilled` címen** van; a Számlázás oldalról egy gombbal érhető el. Új menüpont nincs.
7. **A „Számlázás" gomb a sorokban** a projekt meglévő számlakiállító oldalára (`/dashboard/board/:projectId/invoice`) visz, és csak `invoices.create` joggal, számlázandó órával rendelkező óradíjas projektnél jelenik meg.
8. **Nincs cache:** minden megnyitás újraszámol (néhány lekérdezés). Az adatmennyiség ezt indokolja.

## File Structure

| Fájl | Felelősség |
|---|---|
| `backend/internal/services/unbilled_hours.go` | Tiszta számítás: `BuildUnbilledHours`, típusok |
| `backend/internal/services/unbilled_hours_test.go` | A fentiek táblázatos tesztjei |
| `backend/internal/services/unbilled_hours_load.go` | `LoadUnbilledHours`: SQL-betöltés a meglévő táblákból |
| `backend/internal/handlers/unbilled_hours_handler.go` | Végpont és jogosultság-ellenőrzés |
| `backend/internal/routes/invoice_routes.go` | **Módosul:** `/billing` csoport, 1 útvonal |
| `frontend/src/services/unbilledHoursService.ts` | Típusok és API-hívás |
| `frontend/src/app/dashboard/billing/unbilled/page.tsx` | Az oldal |
| `frontend/src/app/dashboard/billing/page.tsx` | **Módosul:** „Számlázatlan órák" gomb a fejlécben |

---

### Task 1: Tiszta számítás (TDD)

**Files:**
- Create: `backend/internal/services/unbilled_hours.go`
- Test: `backend/internal/services/unbilled_hours_test.go`

**Interfaces:**
- Consumes: `services.InvoicedPeriod{Start, End time.Time}` és `services.IsDateInvoiced(date time.Time, periods []InvoicedPeriod) bool` (`invoice_calc.go`, már létezik). A tesztek használják a `profitability_calc_test.go`-ban definiált `approx(a, b float64) bool` segédet; **ne definiáld újra**.
- Produces:
  - `type UnbilledEntry struct { ProjectID uint; Date time.Time; Hours float64; TaskDone bool }`
  - `type UnbilledProject struct { ID uint; Name, PricingType string; HourlyRate *float64; AutoInvoiceClientID *uint; ClientIDs []uint }`
  - `type UnbilledInput struct { Now time.Time; Projects []UnbilledProject; Entries []UnbilledEntry; Periods map[uint][]InvoicedPeriod; ClientNames map[uint]string }`
  - `type UnbilledProjectRow`, `type UnbilledClientGroup`, `type UnbilledTotals`, `type UnbilledHours`
  - `func BuildUnbilledHours(in UnbilledInput) UnbilledHours`
  - `func unbilledCutoff(now time.Time) time.Time`

- [ ] **Step 1: Írd meg a bukó teszteket**

```go
// backend/internal/services/unbilled_hours_test.go
package services

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

func rate(v float64) *float64 { return &v }
func cid(v uint) *uint        { return &v }

// "Now" is 2026-10-15, so the cutoff is 2026-10-01: anything before it is
// "last month or older".
var ubNow = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

func hourlyProject(id uint, name string, r *float64, auto *uint, clients ...uint) UnbilledProject {
	return UnbilledProject{ID: id, Name: name, PricingType: "hourly", HourlyRate: r, AutoInvoiceClientID: auto, ClientIDs: clients}
}

func ubInput(projects []UnbilledProject, entries []UnbilledEntry) UnbilledInput {
	return UnbilledInput{
		Now: ubNow, Projects: projects, Entries: entries,
		Periods:     map[uint][]InvoicedPeriod{},
		ClientNames: map[uint]string{1: "PIXEL", 2: "ACME"},
	}
}

func TestUnbilledCutoffIsFirstOfCurrentMonthUTC(t *testing.T) {
	got := unbilledCutoff(time.Date(2026, 10, 31, 23, 59, 0, 0, time.UTC))
	if !got.Equal(d(2026, 10, 1)) {
		t.Fatalf("cutoff = %v", got)
	}
	if got := unbilledCutoff(d(2026, 1, 1)); !got.Equal(d(2026, 1, 1)) {
		t.Fatalf("cutoff on the 1st = %v", got)
	}
}

func TestBuildUnbilledHoursCategorisesByDoneAndCutoff(t *testing.T) {
	out := BuildUnbilledHours(ubInput(
		[]UnbilledProject{hourlyProject(7, "Mentorfy", rate(10000), nil, 1)},
		[]UnbilledEntry{
			{ProjectID: 7, Date: d(2026, 9, 30), Hours: 2, TaskDone: false}, // last month, open task -> billable
			{ProjectID: 7, Date: d(2026, 10, 1), Hours: 3, TaskDone: false}, // cutoff day, open -> in progress
			{ProjectID: 7, Date: d(2026, 10, 5), Hours: 4, TaskDone: true},  // this month, done -> billable
			{ProjectID: 7, Date: d(2026, 10, 9), Hours: 1, TaskDone: false}, // this month, open -> in progress
		},
	))
	if len(out.Clients) != 1 || len(out.Clients[0].Projects) != 1 {
		t.Fatalf("groups = %+v", out.Clients)
	}
	row := out.Clients[0].Projects[0]
	if !approx(row.BillableHours, 6) || !approx(row.InProgressHours, 4) || !approx(row.LoggedHours, 10) {
		t.Fatalf("row = %+v", row)
	}
	if row.BillableAmount == nil || !approx(*row.BillableAmount, 60000) {
		t.Fatalf("amount = %v", row.BillableAmount)
	}
	if !approx(out.Totals.BillableHours, 6) || !approx(out.Totals.BillableAmount, 60000) || !approx(out.Totals.InProgressHours, 4) {
		t.Fatalf("totals = %+v", out.Totals)
	}
}

func TestBuildUnbilledHoursInvoicedDatesAreExcluded(t *testing.T) {
	in := ubInput(
		[]UnbilledProject{hourlyProject(7, "P", rate(10000), nil, 1)},
		[]UnbilledEntry{
			{ProjectID: 7, Date: d(2026, 9, 10), Hours: 5, TaskDone: true},  // inside the invoiced period
			{ProjectID: 7, Date: d(2026, 9, 15), Hours: 2, TaskDone: true},  // period end is inclusive
			{ProjectID: 7, Date: d(2026, 9, 16), Hours: 3, TaskDone: false}, // after the period, last month -> billable
		},
	)
	in.Periods[7] = []InvoicedPeriod{{Start: d(2026, 9, 1), End: d(2026, 9, 15)}}
	row := BuildUnbilledHours(in).Clients[0].Projects[0]
	if !approx(row.BillableHours, 3) || !approx(row.LoggedHours, 10) {
		t.Fatalf("row = %+v", row)
	}
}

func TestBuildUnbilledHoursFullyInvoicedHourlyProjectIsOmittedButCounted(t *testing.T) {
	in := ubInput(
		[]UnbilledProject{
			hourlyProject(7, "Paid", rate(10000), nil, 1),
			hourlyProject(8, "Open", rate(10000), nil, 1),
		},
		[]UnbilledEntry{
			{ProjectID: 7, Date: d(2026, 9, 10), Hours: 5, TaskDone: true},
			{ProjectID: 8, Date: d(2026, 9, 10), Hours: 1, TaskDone: true},
		},
	)
	in.Periods[7] = []InvoicedPeriod{{Start: d(2026, 9, 1), End: d(2026, 9, 30)}}
	out := BuildUnbilledHours(in)
	if len(out.Clients) != 1 || len(out.Clients[0].Projects) != 1 || out.Clients[0].Projects[0].ProjectID != 8 {
		t.Fatalf("groups = %+v", out.Clients)
	}
	if out.FullyInvoicedProjects != 1 {
		t.Fatalf("fully invoiced = %d", out.FullyInvoicedProjects)
	}
}

func TestBuildUnbilledHoursMissingRateGivesNilAmountAndWarning(t *testing.T) {
	out := BuildUnbilledHours(ubInput(
		[]UnbilledProject{hourlyProject(7, "NoRate", nil, nil, 1)},
		[]UnbilledEntry{{ProjectID: 7, Date: d(2026, 9, 10), Hours: 4, TaskDone: true}},
	))
	row := out.Clients[0].Projects[0]
	if row.BillableAmount != nil || !row.MissingRate || !approx(row.BillableHours, 4) {
		t.Fatalf("row = %+v", row)
	}
	if len(out.Warnings) != 1 {
		t.Fatalf("warnings = %v", out.Warnings)
	}
	if !approx(out.Totals.BillableHours, 4) || !approx(out.Totals.BillableAmount, 0) {
		t.Fatalf("totals = %+v", out.Totals)
	}
}

func TestBuildUnbilledHoursFixedAndHobbyAreListedWithoutBillableValues(t *testing.T) {
	in := ubInput(
		[]UnbilledProject{
			{ID: 7, Name: "Fix", PricingType: "fixed", ClientIDs: []uint{1}},
			{ID: 8, Name: "Hobbi", PricingType: "hobby", ClientIDs: []uint{1}},
		},
		[]UnbilledEntry{
			{ProjectID: 7, Date: d(2026, 9, 10), Hours: 6, TaskDone: true},
			{ProjectID: 8, Date: d(2026, 10, 3), Hours: 2, TaskDone: false},
		},
	)
	out := BuildUnbilledHours(in)
	if len(out.Clients) != 1 || len(out.Clients[0].Projects) != 2 {
		t.Fatalf("groups = %+v", out.Clients)
	}
	for _, r := range out.Clients[0].Projects {
		if r.HourlyBased || r.BillableAmount != nil || !approx(r.BillableHours, 0) || !approx(r.InProgressHours, 0) {
			t.Fatalf("non-hourly row must carry no billable values: %+v", r)
		}
	}
	if !approx(out.Clients[0].Projects[0].LoggedHours+out.Clients[0].Projects[1].LoggedHours, 8) {
		t.Fatalf("logged hours lost: %+v", out.Clients[0].Projects)
	}
	if !approx(out.Totals.BillableHours, 0) || !approx(out.Totals.InProgressHours, 0) {
		t.Fatalf("totals must ignore non-hourly projects: %+v", out.Totals)
	}
}

func TestBuildUnbilledHoursClientResolution(t *testing.T) {
	entry := func(p uint) UnbilledEntry { return UnbilledEntry{ProjectID: p, Date: d(2026, 9, 10), Hours: 1, TaskDone: true} }
	in := ubInput(
		[]UnbilledProject{
			hourlyProject(1, "AutoSet", rate(1000), cid(2), 1, 2), // auto client wins over the two linked ones
			hourlyProject(2, "Sole", rate(1000), nil, 1),          // single linked client
			hourlyProject(3, "Many", rate(1000), nil, 1, 2),       // several, none designated
			hourlyProject(4, "None", rate(1000), nil),             // no client at all
		},
		[]UnbilledEntry{entry(1), entry(2), entry(3), entry(4)},
	)
	out := BuildUnbilledHours(in)

	byName := map[string][]string{}
	var unassigned *UnbilledClientGroup
	for i := range out.Clients {
		g := out.Clients[i]
		names := []string{}
		for _, p := range g.Projects {
			names = append(names, p.ProjectName)
		}
		if g.Unassigned {
			unassigned = &out.Clients[i]
		}
		byName[g.ClientName] = names
	}
	if len(byName["ACME"]) != 1 || byName["ACME"][0] != "AutoSet" {
		t.Fatalf("ACME group = %v", byName["ACME"])
	}
	if len(byName["PIXEL"]) != 1 || byName["PIXEL"][0] != "Sole" {
		t.Fatalf("PIXEL group = %v", byName["PIXEL"])
	}
	if unassigned == nil || len(unassigned.Projects) != 2 {
		t.Fatalf("unassigned = %+v", unassigned)
	}
	if out.Clients[len(out.Clients)-1].Unassigned != true {
		t.Fatalf("unassigned group must be last: %+v", out.Clients)
	}
	if len(out.Warnings) != 2 {
		t.Fatalf("expected one warning per unassigned project, got %v", out.Warnings)
	}
}

func TestBuildUnbilledHoursEachProjectAppearsOnceSoTotalsAreNotDoubled(t *testing.T) {
	in := ubInput(
		[]UnbilledProject{hourlyProject(1, "Shared", rate(1000), cid(1), 1, 2)},
		[]UnbilledEntry{{ProjectID: 1, Date: d(2026, 9, 10), Hours: 3, TaskDone: true}},
	)
	out := BuildUnbilledHours(in)
	if !approx(out.Totals.BillableHours, 3) || !approx(out.Totals.BillableAmount, 3000) {
		t.Fatalf("totals = %+v", out.Totals)
	}
	count := 0
	for _, g := range out.Clients {
		count += len(g.Projects)
	}
	if count != 1 {
		t.Fatalf("project listed %d times", count)
	}
}

func TestBuildUnbilledHoursOldestBillableDate(t *testing.T) {
	out := BuildUnbilledHours(ubInput(
		[]UnbilledProject{hourlyProject(7, "P", rate(1000), nil, 1)},
		[]UnbilledEntry{
			{ProjectID: 7, Date: d(2026, 9, 20), Hours: 1, TaskDone: false},
			{ProjectID: 7, Date: d(2026, 8, 3), Hours: 1, TaskDone: false},
			{ProjectID: 7, Date: d(2026, 10, 2), Hours: 1, TaskDone: false}, // in progress, must not count
		},
	))
	row := out.Clients[0].Projects[0]
	if row.OldestBillableDate == nil || *row.OldestBillableDate != "2026-08-03" {
		t.Fatalf("oldest = %v", row.OldestBillableDate)
	}
	none := BuildUnbilledHours(ubInput(
		[]UnbilledProject{hourlyProject(7, "P", rate(1000), nil, 1)},
		[]UnbilledEntry{{ProjectID: 7, Date: d(2026, 10, 2), Hours: 1, TaskDone: false}},
	))
	if none.Clients[0].Projects[0].OldestBillableDate != nil {
		t.Fatalf("only in-progress hours must give no oldest date")
	}
}

func TestBuildUnbilledHoursSortsGroupsAndProjectsByAmountThenName(t *testing.T) {
	in := ubInput(
		[]UnbilledProject{
			hourlyProject(1, "Small", rate(1000), nil, 1),
			hourlyProject(2, "Big", rate(1000), nil, 2),
			hourlyProject(3, "Bigger", rate(1000), nil, 2),
		},
		[]UnbilledEntry{
			{ProjectID: 1, Date: d(2026, 9, 1), Hours: 1, TaskDone: true},
			{ProjectID: 2, Date: d(2026, 9, 1), Hours: 5, TaskDone: true},
			{ProjectID: 3, Date: d(2026, 9, 1), Hours: 9, TaskDone: true},
		},
	)
	out := BuildUnbilledHours(in)
	if out.Clients[0].ClientName != "ACME" || out.Clients[1].ClientName != "PIXEL" {
		t.Fatalf("group order = %s, %s", out.Clients[0].ClientName, out.Clients[1].ClientName)
	}
	if out.Clients[0].Projects[0].ProjectName != "Bigger" || out.Clients[0].Projects[1].ProjectName != "Big" {
		t.Fatalf("project order = %+v", out.Clients[0].Projects)
	}
}

func TestBuildUnbilledHoursUnknownClientNameFallsBackToID(t *testing.T) {
	out := BuildUnbilledHours(ubInput(
		[]UnbilledProject{hourlyProject(7, "P", rate(1000), cid(99))},
		[]UnbilledEntry{{ProjectID: 7, Date: d(2026, 9, 10), Hours: 1, TaskDone: true}},
	))
	if out.Clients[0].ClientName != "#99" || out.Clients[0].Unassigned {
		t.Fatalf("group = %+v", out.Clients[0])
	}
}

func TestBuildUnbilledHoursIgnoresEntriesOfUnknownProjectsAndZeroHours(t *testing.T) {
	out := BuildUnbilledHours(ubInput(
		[]UnbilledProject{hourlyProject(7, "P", rate(1000), nil, 1)},
		[]UnbilledEntry{
			{ProjectID: 99, Date: d(2026, 9, 10), Hours: 5, TaskDone: true}, // no such project
			{ProjectID: 7, Date: d(2026, 9, 10), Hours: 0, TaskDone: true},  // zero hours
		},
	))
	if len(out.Clients) != 0 {
		t.Fatalf("groups = %+v", out.Clients)
	}
}

func TestBuildUnbilledHoursEmptyInputHasNonNilSlices(t *testing.T) {
	out := BuildUnbilledHours(UnbilledInput{Now: ubNow})
	if out.Clients == nil || out.Warnings == nil {
		t.Fatalf("nil slice in %+v", out)
	}
	if out.CutoffDate != "2026-10-01" || out.AsOf != "2026-10-15" {
		t.Fatalf("dates = %s / %s", out.CutoffDate, out.AsOf)
	}
}

func TestBuildUnbilledHoursClientGroupTotals(t *testing.T) {
	out := BuildUnbilledHours(ubInput(
		[]UnbilledProject{
			hourlyProject(1, "A", rate(1000), nil, 1),
			hourlyProject(2, "B", rate(2000), nil, 1),
		},
		[]UnbilledEntry{
			{ProjectID: 1, Date: d(2026, 9, 1), Hours: 2, TaskDone: true},
			{ProjectID: 2, Date: d(2026, 9, 1), Hours: 3, TaskDone: true},
			{ProjectID: 2, Date: d(2026, 10, 4), Hours: 1, TaskDone: false},
		},
	))
	g := out.Clients[0]
	if !approx(g.BillableHours, 5) || !approx(g.BillableAmount, 8000) || !approx(g.InProgressHours, 1) {
		t.Fatalf("group = %+v", g)
	}
}
```

- [ ] **Step 2: Futtasd, és ellenőrizd, hogy elbukik**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/services/ -run "Unbilled" 2>&1 | tail -8'`
Expected: FAIL (`undefined: BuildUnbilledHours`, `undefined: UnbilledInput` …).

- [ ] **Step 3: Írd meg a minimális implementációt**

```go
// backend/internal/services/unbilled_hours.go
package services

import (
	"fmt"
	"sort"
	"time"
)

// UnbilledEntry is one logged time entry. TaskDone is true when the entry's
// task currently sits in a kanban column flagged is_done.
type UnbilledEntry struct {
	ProjectID uint
	Date      time.Time
	Hours     float64
	TaskDone  bool
}

type UnbilledProject struct {
	ID                  uint
	Name                string
	PricingType         string
	HourlyRate          *float64
	AutoInvoiceClientID *uint
	ClientIDs           []uint
}

type UnbilledInput struct {
	Now         time.Time
	Projects    []UnbilledProject
	Entries     []UnbilledEntry
	Periods     map[uint][]InvoicedPeriod // already-invoiced hourly periods per project
	ClientNames map[uint]string
}

type UnbilledProjectRow struct {
	ProjectID          uint     `json:"project_id"`
	ProjectName        string   `json:"project_name"`
	PricingType        string   `json:"pricing_type"`
	HourlyBased        bool     `json:"hourly_based"`
	HourlyRate         *float64 `json:"hourly_rate"`
	LoggedHours        float64  `json:"logged_hours"`
	BillableHours      float64  `json:"billable_hours"`
	BillableAmount     *float64 `json:"billable_amount"`
	InProgressHours    float64  `json:"in_progress_hours"`
	OldestBillableDate *string  `json:"oldest_billable_date"`
	MissingRate        bool     `json:"missing_rate"`
}

type UnbilledClientGroup struct {
	ClientID        *uint                `json:"client_id"`
	ClientName      string               `json:"client_name"`
	Unassigned      bool                 `json:"unassigned"`
	BillableHours   float64              `json:"billable_hours"`
	BillableAmount  float64              `json:"billable_amount"`
	InProgressHours float64              `json:"in_progress_hours"`
	Projects        []UnbilledProjectRow `json:"projects"`
}

type UnbilledTotals struct {
	BillableHours   float64 `json:"billable_hours"`
	BillableAmount  float64 `json:"billable_amount"`
	InProgressHours float64 `json:"in_progress_hours"`
}

type UnbilledHours struct {
	AsOf                  string                `json:"as_of"`
	CutoffDate            string                `json:"cutoff_date"`
	Totals                UnbilledTotals        `json:"totals"`
	Clients               []UnbilledClientGroup `json:"clients"`
	FullyInvoicedProjects int                   `json:"fully_invoiced_projects"`
	Warnings              []string              `json:"warnings"`
}

// unbilledCutoff returns the first day of now's calendar month (UTC date).
// Hours dated before it count as "last month or older".
func unbilledCutoff(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// BuildUnbilledHours splits every project's logged hours into invoiced,
// billable-now (unbilled AND (task done OR dated before the current month))
// and in-progress. "Invoiced" is IsDateInvoiced over the project's created
// hourly invoices, so it is the same rule the kanban card icon uses. Each
// project is placed under exactly one billing client so totals never double
// count. Only hourly projects carry billable values.
func BuildUnbilledHours(in UnbilledInput) UnbilledHours {
	cutoff := unbilledCutoff(in.Now)
	out := UnbilledHours{
		AsOf:       in.Now.UTC().Format("2006-01-02"),
		CutoffDate: cutoff.Format("2006-01-02"),
		Clients:    []UnbilledClientGroup{},
		Warnings:   []string{},
	}

	projects := make(map[uint]UnbilledProject, len(in.Projects))
	for _, p := range in.Projects {
		projects[p.ID] = p
	}

	type agg struct {
		logged, billable, inProgress float64
		oldest                       time.Time
		hasOldest                    bool
	}
	aggs := map[uint]*agg{}
	for _, e := range in.Entries {
		p, ok := projects[e.ProjectID]
		if !ok || e.Hours <= 0 {
			continue
		}
		a := aggs[e.ProjectID]
		if a == nil {
			a = &agg{}
			aggs[e.ProjectID] = a
		}
		a.logged += e.Hours
		if p.PricingType != "hourly" || IsDateInvoiced(e.Date, in.Periods[e.ProjectID]) {
			continue
		}
		if e.TaskDone || e.Date.Before(cutoff) {
			a.billable += e.Hours
			if !a.hasOldest || e.Date.Before(a.oldest) {
				a.oldest, a.hasOldest = e.Date, true
			}
		} else {
			a.inProgress += e.Hours
		}
	}

	ids := make([]uint, 0, len(aggs))
	for id := range aggs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] }) // deterministic float sums

	groups := map[uint]*UnbilledClientGroup{} // key 0 = unassigned
	for _, id := range ids {
		p, a := projects[id], aggs[id]
		hourly := p.PricingType == "hourly"

		if hourly && a.billable == 0 && a.inProgress == 0 {
			out.FullyInvoicedProjects++
			continue
		}

		row := UnbilledProjectRow{
			ProjectID: p.ID, ProjectName: p.Name, PricingType: p.PricingType,
			HourlyBased: hourly, HourlyRate: p.HourlyRate, LoggedHours: a.logged,
		}
		if hourly {
			row.BillableHours = a.billable
			row.InProgressHours = a.inProgress
			if a.hasOldest {
				s := a.oldest.Format("2006-01-02")
				row.OldestBillableDate = &s
			}
			if p.HourlyRate != nil {
				amount := a.billable * *p.HourlyRate
				row.BillableAmount = &amount
			} else if a.billable > 0 {
				row.MissingRate = true
				out.Warnings = append(out.Warnings, fmt.Sprintf(
					"A(z) %s projektnek nincs óradíja, ezért a számlázandó összeg nem számolható", p.Name))
			}
		}

		var key uint
		switch {
		case p.AutoInvoiceClientID != nil:
			key = *p.AutoInvoiceClientID
		case len(p.ClientIDs) == 1:
			key = p.ClientIDs[0]
		default:
			out.Warnings = append(out.Warnings, unassignedWarning(p))
		}

		g := groups[key]
		if g == nil {
			g = &UnbilledClientGroup{Projects: []UnbilledProjectRow{}}
			if key == 0 {
				g.Unassigned = true
				g.ClientName = "Nincs kijelölt számlázandó ügyfél"
			} else {
				k := key
				g.ClientID = &k
				g.ClientName = in.ClientNames[key]
				if g.ClientName == "" {
					g.ClientName = fmt.Sprintf("#%d", key)
				}
			}
			groups[key] = g
		}
		g.Projects = append(g.Projects, row)
		if hourly {
			g.BillableHours += row.BillableHours
			g.InProgressHours += row.InProgressHours
			if row.BillableAmount != nil {
				g.BillableAmount += *row.BillableAmount
			}
			out.Totals.BillableHours += row.BillableHours
			out.Totals.InProgressHours += row.InProgressHours
			if row.BillableAmount != nil {
				out.Totals.BillableAmount += *row.BillableAmount
			}
		}
	}

	for _, g := range groups {
		sort.SliceStable(g.Projects, func(i, j int) bool {
			ai, aj := amountOrZero(g.Projects[i]), amountOrZero(g.Projects[j])
			if ai != aj {
				return ai > aj
			}
			if g.Projects[i].BillableHours != g.Projects[j].BillableHours {
				return g.Projects[i].BillableHours > g.Projects[j].BillableHours
			}
			return g.Projects[i].ProjectName < g.Projects[j].ProjectName
		})
		out.Clients = append(out.Clients, *g)
	}
	sort.SliceStable(out.Clients, func(i, j int) bool {
		a, b := out.Clients[i], out.Clients[j]
		if a.Unassigned != b.Unassigned {
			return !a.Unassigned // the unassigned group goes last
		}
		if a.BillableAmount != b.BillableAmount {
			return a.BillableAmount > b.BillableAmount
		}
		return a.ClientName < b.ClientName
	})
	return out
}

func amountOrZero(r UnbilledProjectRow) float64 {
	if r.BillableAmount == nil {
		return 0
	}
	return *r.BillableAmount
}

func unassignedWarning(p UnbilledProject) string {
	if len(p.ClientIDs) == 0 {
		return fmt.Sprintf("A(z) %s projekthez nincs ügyfél rendelve, ezért nem derül ki, kinek kell számlázni", p.Name)
	}
	return fmt.Sprintf("A(z) %s projekt több ügyfélhez tartozik, de nincs kijelölt számlázandó ügyfél (automatikus számlázás ügyfele)", p.Name)
}
```

- [ ] **Step 4: Futtasd, és ellenőrizd, hogy átmegy**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -w internal/services/unbilled_hours.go internal/services/unbilled_hours_test.go; gofmt -l internal/services/unbilled_hours*.go; go vet ./internal/services/ && go test -count=1 ./internal/services/ -run "Unbilled" -v 2>&1 | grep -E "^(--- |PASS|FAIL|ok)"'`
Expected: `gofmt -l` üres; minden teszt `--- PASS`, `ok`. Ha egy teszt bukik, **ne** a tesztet lazítsd: állj meg, és jelentsd a pontos számokat.

- [ ] **Step 5: Futtasd a csomag összes tesztjét**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test -count=1 ./internal/services/ 2>&1 | tail -3'`
Expected: `ok`. (A `d`, `rate`, `cid` segédnevek ütközhetnek egy meglévő azonosítóval: ha a fordító ütközést jelez, nevezd át a **saját** segédeidet `ubDate`, `ubRate`, `ubCID`-ra az egész fájlban.)

- [ ] **Step 6: Commit** (csak kérésre)

```bash
git add backend/internal/services/unbilled_hours.go backend/internal/services/unbilled_hours_test.go
git commit -m "feat(billing): add pure unbilled hours calculation"
```

---

### Task 2: SQL-betöltő

**Files:**
- Create: `backend/internal/services/unbilled_hours_load.go`

**Interfaces:**
- Consumes: `BuildUnbilledHours`, `UnbilledInput`, `UnbilledEntry`, `UnbilledProject` (Task 1); `InvoicedPeriodsByProject` (`invoice_calc.go`); `loadNames(db, table)` (`profitability.go`, ugyanabban a csomagban); `database.GetDB()`.
- Produces: `func LoadUnbilledHours(now time.Time) (UnbilledHours, error)`.

- [ ] **Step 1: Írd meg a betöltőt**

```go
// backend/internal/services/unbilled_hours_load.go
package services

import (
	"time"

	"dev-bridge-manager/internal/database"

	"gorm.io/gorm"
)

// LoadUnbilledHours gathers every project's logged hours, the billed periods
// and the client links, and hands them to the pure BuildUnbilledHours.
func LoadUnbilledHours(now time.Time) (UnbilledHours, error) {
	db := database.GetDB()

	entries, err := loadUnbilledEntries(db)
	if err != nil {
		return UnbilledHours{}, err
	}
	projects, err := loadUnbilledProjects(db)
	if err != nil {
		return UnbilledHours{}, err
	}

	hourlyIDs := make([]uint, 0, len(projects))
	for _, p := range projects {
		if p.PricingType == "hourly" {
			hourlyIDs = append(hourlyIDs, p.ID)
		}
	}
	periods, err := InvoicedPeriodsByProject(hourlyIDs)
	if err != nil {
		return UnbilledHours{}, err
	}

	clientNames, err := loadNames(db, "clients")
	if err != nil {
		return UnbilledHours{}, err
	}

	return BuildUnbilledHours(UnbilledInput{
		Now: now, Projects: projects, Entries: entries, Periods: periods, ClientNames: clientNames,
	}), nil
}

// task_done is true when any of the task's placements is in an is_done
// column (the same notion tasksInDoneColumn uses for the kanban DTOs).
// Archived tasks are included on purpose: the invoice sums their hours too.
func loadUnbilledEntries(db *gorm.DB) ([]UnbilledEntry, error) {
	var rows []UnbilledEntry
	err := db.Table("task_time_entries").
		Select(`tasks.project_id AS project_id, task_time_entries.date AS date, task_time_entries.hours AS hours,
			EXISTS (SELECT 1 FROM task_placements tp JOIN kanban_columns kc ON kc.id = tp.column_id
				WHERE tp.task_id = tasks.id AND kc.is_done = true) AS task_done`).
		Joins("JOIN tasks ON tasks.id = task_time_entries.task_id").
		Where("task_time_entries.hours > 0").
		Scan(&rows).Error
	return rows, err
}

func loadUnbilledProjects(db *gorm.DB) ([]UnbilledProject, error) {
	var rows []struct {
		ID                  uint
		Name                string
		PricingType         string
		HourlyRate          *float64
		AutoInvoiceClientID *uint
	}
	if err := db.Table("projects").
		Select("id, name, pricing_type, hourly_rate, auto_invoice_client_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	var links []struct {
		ProjectID uint
		ClientID  uint
	}
	if err := db.Table("project_clients").Select("project_id, client_id").Order("client_id ASC").Scan(&links).Error; err != nil {
		return nil, err
	}
	clientsByProject := make(map[uint][]uint, len(rows))
	for _, l := range links {
		clientsByProject[l.ProjectID] = append(clientsByProject[l.ProjectID], l.ClientID)
	}

	projects := make([]UnbilledProject, 0, len(rows))
	for _, r := range rows {
		projects = append(projects, UnbilledProject{
			ID: r.ID, Name: r.Name, PricingType: r.PricingType, HourlyRate: r.HourlyRate,
			AutoInvoiceClientID: r.AutoInvoiceClientID, ClientIDs: clientsByProject[r.ID],
		})
	}
	return projects, nil
}
```

- [ ] **Step 2: gofmt, vet, build**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -w internal/services/unbilled_hours_load.go; gofmt -l internal/services/unbilled_hours_load.go; go vet ./internal/services/ && go build ./... && echo build-ok'`
Expected: `gofmt -l` üres, `build-ok`.

- [ ] **Step 3: Futtasd a valódi betöltőt a dev adatbázison egy ideiglenes, csak olvasó teszttel, majd töröld**

A dev adatbázisban a „Golfrange online" (13-as projekt, óradíjas, 11 000 Ft/óra, egy hozzárendelt ügyfél) egyetlen 5 órás bejegyzése 2026-09-27-i, egy `Done` oszlopban lévő feladaton, és nincs számla. A várt eredmény: egy ügyfélcsoport, 5 számlázandó óra, 55 000 Ft.

Run:
```bash
cat > backend/internal/services/zz_tmp_unbilled_test.go <<'EOF'
package services

import (
	"testing"
	"time"

	"dev-bridge-manager/internal/database"
)

// TEMPORARY, read-only: runs the real loader against the dev DB. Deleted right after.
func TestTmpLoadUnbilledHoursAgainstDevDB(t *testing.T) {
	database.Connect()
	out, err := LoadUnbilledHours(time.Now())
	if err != nil {
		t.Fatalf("LoadUnbilledHours: %v", err)
	}
	t.Logf("asOf=%s cutoff=%s totals=%+v fullyInvoiced=%d warnings=%v", out.AsOf, out.CutoffDate, out.Totals, out.FullyInvoicedProjects, out.Warnings)
	for _, g := range out.Clients {
		for _, p := range g.Projects {
			amount := -1.0
			if p.BillableAmount != nil {
				amount = *p.BillableAmount
			}
			t.Logf("group=%q unassigned=%v project=%q hourly=%v logged=%.1f billable=%.1f amount=%.0f inProgress=%.1f oldest=%v",
				g.ClientName, g.Unassigned, p.ProjectName, p.HourlyBased, p.LoggedHours, p.BillableHours, amount, p.InProgressHours, p.OldestBillableDate)
		}
	}
}
EOF
docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/services/ -run TestTmpLoadUnbilledHoursAgainstDevDB -v 2>&1 | grep -E "asOf=|group=|FAIL|PASS|panic|error"'
rm -f backend/internal/services/zz_tmp_unbilled_test.go; ls backend/internal/services | grep -c zz_tmp
```
Expected: `PASS`; a `group=` sor: a Golfrange online projekt, `hourly=true`, `logged=5.0 billable=5.0 amount=55000 inProgress=0.0`; a `totals` sorban `BillableHours:5 BillableAmount:55000`; hiba nélkül; az utolsó sor `0` (az ideiglenes fájl törölve). Ha az adatok időközben megváltoztak (más óraszám), az **elvárást** a tényleges adatból számold újra és írd le; a lényeg, hogy az eredmény egyezzen a kézi számítással.

- [ ] **Step 4: Ellenőrizd az `task_done` részlekérdezést és az összes bejegyzés lekérését visszagörgetett tranzakcióban**

Run:
```bash
cat <<'SQL' | docker compose -f .docker/docker-compose.yml exec -T postgres psql -U devbridge_user -d devbridge -v ON_ERROR_STOP=1 2>&1 | grep -vE "^(BEGIN|ROLLBACK)"
BEGIN;
SELECT tasks.project_id, task_time_entries.date, task_time_entries.hours,
  EXISTS (SELECT 1 FROM task_placements tp JOIN kanban_columns kc ON kc.id = tp.column_id
          WHERE tp.task_id = tasks.id AND kc.is_done = true) AS task_done
FROM task_time_entries JOIN tasks ON tasks.id = task_time_entries.task_id
WHERE task_time_entries.hours > 0;
ROLLBACK;
SQL
```
Expected: a lekérdezés hiba nélkül lefut, és a 13-as projekt 5 órás sorára `task_done = t`. (Írást nem végez; a tranzakció csak védelem.)

- [ ] **Step 5: Commit** (csak kérésre)

```bash
git add backend/internal/services/unbilled_hours_load.go
git commit -m "feat(billing): add unbilled hours loader"
```

---

### Task 3: Végpont és útvonal

**Files:**
- Create: `backend/internal/handlers/unbilled_hours_handler.go`
- Modify: `backend/internal/routes/invoice_routes.go` (új `/billing` csoport a `SetupInvoiceRoutes`-ban)

**Interfaces:**
- Consumes: `services.LoadUnbilledHours` (Task 2); `checkInvoiceAccess(ps *services.PermissionService, userID uint, action string) error` (`invoice_handler.go`, már létezik, csomagszintű); `services.NewPermissionService()`.
- Produces: `GET /api/v1/billing/unbilled-hours`.

- [ ] **Step 1: Írd meg a handlert**

```go
// backend/internal/handlers/unbilled_hours_handler.go
package handlers

import (
	"time"

	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

type UnbilledHoursHandler struct {
	permissionService *services.PermissionService
}

func NewUnbilledHoursHandler() *UnbilledHoursHandler {
	return &UnbilledHoursHandler{permissionService: services.NewPermissionService()}
}

// GetUnbilledHours - GET /api/v1/billing/unbilled-hours
// Same access rule as the other invoice endpoints (invoices.read, with the
// admin/super_admin/manager fallback). Read-only.
func (h *UnbilledHoursHandler) GetUnbilledHours(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)
	if err := checkInvoiceAccess(h.permissionService, currentUserID, "invoices.read"); err != nil {
		return err
	}

	result, err := services.LoadUnbilledHours(time.Now())
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading unbilled hours"})
	}
	return c.JSON(result)
}
```

- [ ] **Step 2: Add hozzá az útvonalat**

A `backend/internal/routes/invoice_routes.go`-ban a `SetupInvoiceRoutes` függvény **végére** (a záró `}` elé; olvasd el előbb a függvény végét, és a meglévő `projects` csoport útvonalai után szúrd be):

```go
	// A kiszámlázatlan órák összesítője (projektenként és ügyfelenként)
	unbilledHandler := handlers.NewUnbilledHoursHandler()
	billing := api.Group("/billing")
	billing.Use(middleware.JWTMiddleware())

	// GET /api/v1/billing/unbilled-hours - Számlázatlan és számlázandó órák
	billing.Get("/unbilled-hours", unbilledHandler.GetUnbilledHours)
```

- [ ] **Step 3: gofmt, vet, build, védettség**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -w internal/handlers/unbilled_hours_handler.go internal/routes/invoice_routes.go; gofmt -l internal/handlers/unbilled_hours_handler.go internal/routes/invoice_routes.go; go vet ./internal/handlers/ ./internal/routes/ && go build ./... && echo build-ok'`
Expected: `gofmt -l` üres, `build-ok`. A `git diff backend/internal/routes/invoice_routes.go` csak hozzáadásokat mutasson.

Majd (a hot reload után ~20 mp):
Run: `sleep 20; curl -s -o /dev/null -w "GET /billing/unbilled-hours token nélkül: HTTP %{http_code}\n" http://localhost:8080/api/v1/billing/unbilled-hours`
Expected: `HTTP 401`.

- [ ] **Step 4: Futtasd a teljes backend tesztet**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test -count=1 ./internal/services/ ./internal/handlers/ ./internal/models/ ./internal/middleware/ 2>&1 | awk "{print \$1, \$2}"'`
Expected: mind a négy csomag `ok`.

- [ ] **Step 5: Commit** (csak kérésre)

```bash
git add backend/internal/handlers/unbilled_hours_handler.go backend/internal/routes/invoice_routes.go
git commit -m "feat(billing): add unbilled hours endpoint"
```

---

### Task 4: Frontend – service, oldal és a belépési gomb

**Files:**
- Create: `frontend/src/services/unbilledHoursService.ts`
- Create: `frontend/src/app/dashboard/billing/unbilled/page.tsx`
- Modify: `frontend/src/app/dashboard/billing/page.tsx` (a fejléc gombja)

**Interfaces:**
- Consumes: `GET /billing/unbilled-hours` (Task 3); `apiClient` (`@/lib/api`); `formatHuf` (`@/utils/formatHuf`); `useAuth` (`@/hooks/auth/use-auth`); `hasPermission` (`@/utils/permissions`); `Button`, `EmptyState`, `LoadingState`, `ErrorState` (meglévő UI).
- Produces: `UnbilledHoursService.get()`, és az oldal a `/dashboard/billing/unbilled` címen.

**Előkészület:** olvasd el a `frontend/src/components/ui/button.tsx`, `EmptyState.tsx`, `LoadingState.tsx` és `ErrorState.tsx` fájlokat, és a `frontend/src/app/dashboard/profitability/page.tsx` fejléc-részét, hogy a tulajdonságnevek egyezzenek (Button: `variant`, `size`, `icon`; EmptyState: `icon`, `title`, `description`, `action`). Ha a lent használt valamelyik tulajdonság-érték nem létezik, igazítsd minimálisan, és jelezd.

- [ ] **Step 1: Írd meg a service-t**

```ts
// frontend/src/services/unbilledHoursService.ts
import { apiClient } from '@/lib/api';

export interface UnbilledProjectRow {
    project_id: number;
    project_name: string;
    pricing_type: string;
    hourly_based: boolean;
    hourly_rate: number | null;
    logged_hours: number;
    billable_hours: number;
    billable_amount: number | null;
    in_progress_hours: number;
    oldest_billable_date: string | null;
    missing_rate: boolean;
}

export interface UnbilledClientGroup {
    client_id: number | null;
    client_name: string;
    unassigned: boolean;
    billable_hours: number;
    billable_amount: number;
    in_progress_hours: number;
    projects: UnbilledProjectRow[];
}

export interface UnbilledHours {
    as_of: string;
    cutoff_date: string;
    totals: {
        billable_hours: number;
        billable_amount: number;
        in_progress_hours: number;
    };
    clients: UnbilledClientGroup[];
    fully_invoiced_projects: number;
    warnings: string[];
}

export const UnbilledHoursService = {
    async get(): Promise<UnbilledHours> {
        return apiClient.get('/billing/unbilled-hours');
    },
};
```

- [ ] **Step 2: Írd meg az oldalt**

```tsx
// frontend/src/app/dashboard/billing/unbilled/page.tsx
'use client';

import React from 'react';
import { useRouter } from 'next/navigation';
import { useAuth } from '@/hooks/auth/use-auth';
import { hasPermission } from '@/utils/permissions';
import { UnbilledHoursService, UnbilledHours } from '@/services/unbilledHoursService';
import { Button } from '@/components/ui/button';
import EmptyState from '@/components/ui/EmptyState';
import LoadingState from '@/components/ui/LoadingState';
import ErrorState from '@/components/ui/ErrorState';
import { formatHuf } from '@/utils/formatHuf';
import { ArrowLeft, Receipt } from 'lucide-react';

const errorMessage = (err: unknown) => (err instanceof Error ? err.message : String(err));

const formatHours = (v: number) => `${v.toFixed(1).replace('.', ',')} ó`;

const pricingLabel: Record<string, string> = { fixed: 'Fix áras', hobby: 'Hobbi' };

export default function UnbilledHoursPage() {
    const router = useRouter();
    const { user } = useAuth();
    const canInvoice = hasPermission(user, 'invoices.create');

    const [data, setData] = React.useState<UnbilledHours | null>(null);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);

    const load = React.useCallback(async () => {
        try {
            setLoading(true);
            setError(null);
            setData(await UnbilledHoursService.get());
        } catch (err: unknown) {
            setError(errorMessage(err));
        } finally {
            setLoading(false);
        }
    }, []);

    React.useEffect(() => {
        load();
    }, [load]);

    return (
        <div className="p-6 max-w-6xl space-y-6">
            <div>
                <button
                    onClick={() => router.push('/dashboard/billing')}
                    className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground mb-3"
                >
                    <ArrowLeft size={14} /> Vissza a számlázáshoz
                </button>
                <h1 className="text-2xl font-bold text-foreground mb-1">Számlázatlan órák</h1>
                <p className="text-sm text-muted-foreground">
                    Számlázandó: a kész feladatok és az előző hónapok még ki nem számlázott órái. Folyamatban: az aktuális hónap órái
                    nyitott feladatokon.
                </p>
            </div>

            {loading && <LoadingState message="Számlázatlan órák betöltése..." />}
            {!loading && error && <ErrorState error={error} onRetry={load} />}

            {!loading && !error && data && (
                <>
                    <div className="grid gap-4 sm:grid-cols-3">
                        <div className="bg-card border border-border rounded-lg p-4">
                            <p className="text-xs text-muted-foreground">Számlázandó óra</p>
                            <p className="text-2xl font-semibold text-foreground">{formatHours(data.totals.billable_hours)}</p>
                        </div>
                        <div className="bg-card border border-border rounded-lg p-4">
                            <p className="text-xs text-muted-foreground">Becsült összeg (nettó)</p>
                            <p className="text-2xl font-semibold text-foreground">{formatHuf(data.totals.billable_amount)}</p>
                        </div>
                        <div className="bg-card border border-border rounded-lg p-4">
                            <p className="text-xs text-muted-foreground">Folyamatban</p>
                            <p className="text-2xl font-semibold text-foreground">{formatHours(data.totals.in_progress_hours)}</p>
                        </div>
                    </div>

                    {data.warnings.map((w, i) => (
                        <div key={`${i}-${w}`} className="bg-muted border border-border text-foreground px-3 py-2 rounded text-sm">
                            {w}
                        </div>
                    ))}

                    {data.clients.length === 0 ? (
                        <EmptyState
                            icon="files"
                            title="Nincs számlázandó óra"
                            description="Minden rögzített óra ki van számlázva, vagy még nincs óra rögzítve."
                        />
                    ) : (
                        data.clients.map((group) => (
                            <section key={group.client_id ?? 'unassigned'}>
                                <div className="flex items-baseline justify-between mb-2">
                                    <h2 className="text-lg font-semibold text-foreground">{group.client_name}</h2>
                                    <p className="text-sm text-muted-foreground">
                                        Számlázandó: {formatHours(group.billable_hours)} · {formatHuf(group.billable_amount)}
                                    </p>
                                </div>
                                <div className="overflow-x-auto bg-card border border-border rounded-lg">
                                    <table className="w-full text-sm">
                                        <thead className="text-left text-muted-foreground border-b border-border">
                                            <tr>
                                                <th scope="col" className="p-3">Projekt</th>
                                                <th scope="col" className="p-3 text-right">Számlázandó</th>
                                                <th scope="col" className="p-3 text-right">Becsült összeg</th>
                                                <th scope="col" className="p-3 text-right">Folyamatban</th>
                                                <th scope="col" className="p-3 text-right">Legrégebbi</th>
                                                <th scope="col" className="p-3"></th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {group.projects.map((p) => (
                                                <tr key={p.project_id} className="border-b border-border last:border-0 align-middle">
                                                    <td className="p-3 text-foreground">
                                                        {p.project_name}
                                                        {!p.hourly_based && (
                                                            <span className="ml-2 text-xs bg-muted text-muted-foreground rounded-full px-2 py-0.5">
                                                                {pricingLabel[p.pricing_type] ?? p.pricing_type} · nem óra alapú
                                                            </span>
                                                        )}
                                                    </td>
                                                    {p.hourly_based ? (
                                                        <>
                                                            <td className="p-3 text-right">{formatHours(p.billable_hours)}</td>
                                                            <td className="p-3 text-right">
                                                                {p.billable_amount !== null ? (
                                                                    formatHuf(p.billable_amount)
                                                                ) : p.missing_rate ? (
                                                                    <span className="text-warning" title="A projektnek nincs óradíja">
                                                                        nincs óradíj
                                                                    </span>
                                                                ) : (
                                                                    '—'
                                                                )}
                                                            </td>
                                                            <td className="p-3 text-right text-muted-foreground">
                                                                {formatHours(p.in_progress_hours)}
                                                            </td>
                                                            <td className="p-3 text-right text-muted-foreground">
                                                                {p.oldest_billable_date ?? '—'}
                                                            </td>
                                                        </>
                                                    ) : (
                                                        <>
                                                            <td className="p-3 text-right text-muted-foreground">—</td>
                                                            <td className="p-3 text-right text-muted-foreground">—</td>
                                                            <td className="p-3 text-right text-muted-foreground">—</td>
                                                            <td className="p-3 text-right text-muted-foreground" title="Rögzített órák összesen">
                                                                {formatHours(p.logged_hours)} rögzítve
                                                            </td>
                                                        </>
                                                    )}
                                                    <td className="p-3 text-right">
                                                        {canInvoice && p.hourly_based && p.billable_hours > 0 && (
                                                            <Button
                                                                variant="secondary"
                                                                size="sm"
                                                                icon={Receipt}
                                                                onClick={() => router.push(`/dashboard/board/${p.project_id}/invoice`)}
                                                            >
                                                                Számlázás
                                                            </Button>
                                                        )}
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            </section>
                        ))
                    )}

                    <p className="text-xs text-muted-foreground">
                        {data.fully_invoiced_projects > 0 &&
                            `${data.fully_invoiced_projects} óradíjas projekt teljesen ki van számlázva, ezek nem szerepelnek a listában. `}
                        A „kiszámlázott" állapot a kiállított óradíjas számlák időszaka alapján dől el; a határ az aktuális hónap első
                        napja ({data.cutoff_date}). Az összegek nettók, az óradíj a projektből jön.
                    </p>
                </>
            )}
        </div>
    );
}
```

- [ ] **Step 3: Add hozzá a belépési gombot a Számlázás oldalhoz**

Olvasd el előbb a `frontend/src/app/dashboard/billing/page.tsx` fejlécét (a `return (` utáni első blokk, ~355-360. sor). A jelenlegi:

```tsx
            <div>
                <h1 className="text-2xl font-bold text-foreground mb-1">Számlázás</h1>
                <p className="text-sm text-muted-foreground">Minden projekt számlái egy helyen, projektenkénti szűréssel.</p>
            </div>
```

Cseréld erre (a `router` már létezik az oldalon, a `Button` és a `Receipt` ikon is importálva van; ha az `Receipt` nem importált, add hozzá a meglévő `lucide-react` importhoz — a fájl importjában már szerepel):

```tsx
            <div className="flex items-start justify-between gap-4">
                <div>
                    <h1 className="text-2xl font-bold text-foreground mb-1">Számlázás</h1>
                    <p className="text-sm text-muted-foreground">Minden projekt számlái egy helyen, projektenkénti szűréssel.</p>
                </div>
                <Button variant="secondary" icon={Receipt} onClick={() => router.push('/dashboard/billing/unbilled')}>
                    Számlázatlan órák
                </Button>
            </div>
```

A fájlban semmi más ne változzon (`git diff` csak ez a blokk legyen).

- [ ] **Step 4: Típusellenőrzés, lint, futásidejű fordulás**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cd /app && npx tsc --noEmit -p . 2>&1 | grep -E "unbilled|billing/page|unbilledHours" || echo "tsc: nincs hiba az érintett fájlokban"; echo "tsc összes régi hiba: $(npx tsc --noEmit -p . 2>&1 | grep -c "error TS")"; npx eslint src/services/unbilledHoursService.ts src/app/dashboard/billing/unbilled/page.tsx src/app/dashboard/billing/page.tsx 2>&1 | tail -12; echo eslint-done'`
Expected: `tsc: nincs hiba az érintett fájlokban`, az összes régi hiba `6` (más fájlokban), az eslint a `billing/page.tsx` **korábbi** figyelmeztetéseit kivéve üres. Ha az eslint a `billing/page.tsx`-ben régi hibát jelez, mutasd meg a pontos sort, és ellenőrizd `git stash`-sel **nélkül** (tilos), inkább `git diff`-fel, hogy a hiba nem a te sorodból jön; jelöld pre-existingnek.

Run: `curl -s -o /dev/null -w "oldal: HTTP %{http_code}\n" http://localhost:3010/dashboard/billing/unbilled; docker compose -f .docker/docker-compose.yml logs --tail 30 frontend 2>&1 | grep -iE "error|failed to compile" | tail -5 || true`
Expected: `HTTP 200` (vagy átirányítás a belépésre), fordítási hiba nélkül.

- [ ] **Step 5: Commit** (csak kérésre)

```bash
git add frontend/src/services/unbilledHoursService.ts frontend/src/app/dashboard/billing/unbilled/page.tsx frontend/src/app/dashboard/billing/page.tsx
git commit -m "feat(billing): add unbilled hours page"
```

---

### Task 5: Végellenőrzés

- [ ] **Step 1: Teljes backend-ellenőrzés konténerben**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -l internal/services/unbilled_hours*.go internal/handlers/unbilled_hours_handler.go internal/routes/invoice_routes.go; go vet ./internal/... ; echo "vet exit: $?"; go build ./... && echo build-ok; go test -count=1 ./internal/models/ ./internal/services/ ./internal/handlers/ ./internal/middleware/ 2>&1 | awk "{print \$1, \$2}"'`
Expected: `gofmt -l` üres, `vet exit: 0`, `build-ok`, mind a négy csomag `ok`. (A `gofmt -l internal/models/project.go` régi listázása ide nem tartozik.)

- [ ] **Step 2: Számítsd ki kézzel az oldal alappéldáját**

A dev adatbázis tényleges adatával: Golfrange online, 11 000 Ft/óra, egy 5 órás bejegyzés 2026-09-27-én egy kész feladaton, nincs számla → számlázandó 5 óra × 11 000 = **55 000 Ft**, folyamatban 0. Az aktuális hónap 1. napja előtti dátum miatt akkor is számlázandó lenne, ha a feladat nem volna kész. Írd le a számítást a jelentésben, és vesd össze a Task 2 3. lépésének kimenetével.

- [ ] **Step 3: Végpont és védettség**

Run: `curl -s -o /dev/null -w "GET /billing/unbilled-hours token nélkül: HTTP %{http_code}\n" http://localhost:8080/api/v1/billing/unbilled-hours`
Expected: `HTTP 401`.

- [ ] **Step 4: Böngészős ellenőrzés (ha van belépés)**

Jelentkezz be egy `admin` / `manager` felhasználóval a `http://localhost:3010`-en, a **Számlázás** oldalon kattints a **Számlázatlan órák** gombra. Ellenőrizd: az összesítő három kártyája; az ügyfélcsoport és a projektsor számai; a „Számlázás" gomb a projekt számlakiállító oldalára visz; üres állapotban az `EmptyState`. Ha nincs belépési adat, ezt **ne állítsd ellenőrzöttnek**: írd le, hogy a vizuális és interaktív rész nem volt kipróbálva.

---

## Self-Review

**Követelmény-lefedettség (a brainstorming döntései):**
- „Ami el van végezve, azt számlázni kell, és az elmúlt hónapban keletkezettet is" → a számlázandó kategória: kész feladat **vagy** az aktuális hónap 1. napja előtti dátum (Task 1) ✔
- Kiszámlázott = a meglévő időszak-alapú szabály (`InvoicedPeriodsByProject`, `IsDateInvoiced`) → Task 1–2 ✔
- C: minden projekt szerepel, a fix áras/hobbi értékek nélkül, „nem óra alapú" jelzéssel → Task 1 (`HourlyBased`), Task 4 (jelzés) ✔
- A: egy kijelölt számlázandó ügyfél projektenként, ennek hiányában az egyetlen ügyfél, egyébként külön csoport figyelmeztetéssel → Task 1 ✔
- A: külön oldal a Számlázás alatt, gombbal → Task 4 ✔
- Összeg = óra × óradíj, hiányzó óradíj jelzése → Task 1 ✔
- Nincs adatmódosítás, nincs migráció → ✔
- A meglévő jogosultsági szabály (`checkInvoiceAccess`) újrahasznosítva → Task 3 ✔

**Eltérések / megjegyzések a felhasználónak:**
1. A teljesen kiszámlázott óradíjas projekt **kimarad** a listából, és csak számlálóban jelenik meg (a „minden projekt szerepel" kérés a fix áras és hobbi projektekre, valamint a számlázatlan órával rendelkező óradíjas projektekre vonatkozik; a már rendezett projekt zaj lenne).
2. A „kiszámlázott" időszak-alapú (nem bejegyzésenkénti), ezért egy részben átfedő számla minden átfedő nap óráját kiszámláznak tekinti.
3. A „kész" az `is_done` oszlop-jelzőn múlik; egy kézzel létrehozott, nem jelölt „Kész" oszlop nem számít késznek.

**Placeholder-ellenőrzés:** nincs TBD/TODO; minden kódlépésben teljes kód van. A Task 4 3. lépése a `billing/page.tsx` fejlécének cseréjét pontos előtte/utána kóddal adja; az implementernek előbb el kell olvasnia a fájlt, mert a sorszámok eltolódhattak.

**Típus-konzisztencia:** a Go JSON-címkék (`as_of`, `cutoff_date`, `totals.{billable_hours,billable_amount,in_progress_hours}`, `clients[].{client_id,client_name,unassigned,billable_hours,billable_amount,in_progress_hours,projects[]}`, `projects[].{project_id,project_name,pricing_type,hourly_based,hourly_rate,logged_hours,billable_hours,billable_amount,in_progress_hours,oldest_billable_date,missing_rate}`, `fully_invoiced_projects`, `warnings`) megegyeznek a TS `UnbilledHours` / `UnbilledClientGroup` / `UnbilledProjectRow` mezőivel. A függvénynevek (`BuildUnbilledHours`, `LoadUnbilledHours`, `unbilledCutoff`, `NewUnbilledHoursHandler`, `GetUnbilledHours`) minden taskban azonosak. A tesztek az `approx` segédet a meglévő `profitability_calc_test.go`-ból veszik át; a `d`, `rate`, `cid` segédnevek ütközése esetén a Task 1 5. lépése átnevezést ír elő.

**Nem ellenőrzött feltevések:**
- A valós Billingo-számlák `period_start`/`period_end` értékei a dev adatbázisban nem tesztelhetők (nincs számla), így a „részben átfedő számla" viselkedés csak a tiszta függvény tesztjeivel igazolt.
- A `Button` `size="sm"`, az `EmptyState` `icon="files"` és a `text-warning` osztály a meglévő kódban használatos, de a Task 4 előkészülete ellenőrzi.
- Az archivált feladatok órái beleszámítanak (a számla is így számol); ezt valós adaton nem néztük.
- A vizuális megjelenés és az interakció böngészőben nem ellenőrizhető bejelentkezés nélkül.
