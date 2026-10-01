# Jövedelmezőség – 2. szakasz: bevétel-előrejelzés Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ügyfelenkénti 3–6 hónapos bevétel-előrejelzés, a szerződés lejáratától függő bevétel külön sávban, az Előrejelzés fülön diagrammal és táblázatokkal.

**Architecture:** Az 1. szakasz mintáját követi: tiszta Go függvény (`BuildForecast`) + vékony SQL-betöltő (`LoadForecast`) + egy új végpont a meglévő `/api/v1/profitability` csoportban (a `profitability.read` jog már védi). A felület a meglévő oldalt két fülre osztja (`Áttekintés`, `Előrejelzés`); az előrejelzés önálló komponens, kézzel rajzolt SVG diagrammal (nincs új függőség).

**Tech Stack:** Go + Fiber + GORM (backend konténer), Next.js + TypeScript (frontend konténer), PostgreSQL. Az 1. szakasz kódja (`profitability_calc.go`, `profitability.go`, `profitability_handler.go`) már a `main`-ben van.

**Spec:** `docs/superpowers/specs/2026-10-01-profitability-design.md` (ez a terv a **2. szállítási szakaszt** valósítja meg; a 3–4. szakasz külön tervet kap.)

## Global Constraints

- **Nincs beépített adó- vagy járulékszabály, nincs alapértelmezett kulcs.** (Ebben a szakaszban adó nincs.)
- Alapadat: az **utolsó 3 teljes hónap** számlái (`status = 'created'` és `billingo_invoice_id <> ''`, a hónap `COALESCE(period_end, created_at)`), ugyanaz a definíció, mint az 1. szakaszban (`loadInvoiceMonths`). Az `invoices.amount` nettónak számít (az 1. szakasz Ruling 1-je).
- Az előrejelzés **ügyfélenként** a projekt–ügyfél párok átlagából épül, az átlag osztója mindig az alapidőszak hossza (3), nem az adatos hónapok száma.
- Jogosultság: a végpont a meglévő `/profitability` csoportban van (`profitability.read`); új jog nem kell, migráció nem kell.
- Minden ellenőrzés (go test, gofmt, go vet, tsc, eslint, psql) **a projekt konténereiben** fut, a repo gyökeréből: `docker compose -f .docker/docker-compose.yml exec -T backend|frontend|postgres sh -c '…'`. Host-oldali futtatás tilos.
- Adatbázis-ellenőrzés csak a lokális dev adatbázison, **visszagörgetett tranzakcióban**, ideiglenes tesztadattal. Valós vagy megosztott adat nem érintett.
- **Commit, merge és push csak a felhasználó kifejezett kérésére.** A terv commit-lépései addig nem futnak le. `git add` csak a felsorolt fájlokra.
- A working tree-ben **idegen, nem commitolt munka** van (Jira: `activity_digest_handler.go`, `jira_*`, `task_comment_handler.go`, `models/kanban.go`, `services/jira_*`, `migrations/000040_*`, `docs/.../marketing-contacts.md`). Ezekhez nem nyúlunk.
- Új függőség tilos (nincs chart-könyvtár): a diagram inline SVG. A diagram kódja előtt töltsd be a `dataviz` skillt, és tartsd be az elveit (színen kívüli jelölés, olvasható tengelyek, adat-táblázat alternatíva).
- Munkabranch: `feat/profitability-forecast` (a `main`-ről).
- A `gofmt -w` minden új Go fájlon kötelező, majd `gofmt -l` üres.

## Döntések (Rulings a tervben; a felhasználónak jelezni)

1. **A fix áras számlák nem részei az előrejelzésnek.** Fix áras projektre egyszeri számla készül (a kódban egyszeri-index védi), ezért a 3 hónapra visszanézett átlag „kivetítése" hamis rendszeres bevételt adna. A kizárt összeg figyelmeztetésben jelenik meg. *Ha téves: a `Fixed` kizárása egy `if` a `BuildForecast`-ban.*
2. **A horizont az aktuális hónappal kezdődik.** Az aktuális hónap számlája még nincs kiállítva (a havi számla a következő hó elején készül), tehát bevételként még előttünk van.
3. **Szerződés vége:** egy projekt bevétele az `contract_end_date` hónapjáig „biztos", az utána következő hónapok a „megújítástól függő" sávba kerülnek. Ha a lejárat a horizont előtt volt, minden hónap függő. A lejáratot **projektenként** alkalmazzuk, egy ügyfél soránál a legkorábbi lejárat látszik.
4. **Alapértelmezett horizont 6 hónap**, megengedett 3–6.
5. **Kevés adat:** ha az alapidőszak 3 hónapjából kevesebbben volt rendszeres bevétel, az eredmény `low_data` jelzést és figyelmeztetést kap.
6. **Az oldal két fülre oszlik** (`Áttekintés`, `Előrejelzés`); a spec többi füle (Forgatókönyvek, Beállítások) a 3–4. szakaszban jön. A meglévő „Beállítások" gomb az Áttekintés fülön marad.

## File Structure

| Fájl | Felelősség |
|---|---|
| `backend/internal/services/profitability_forecast.go` | Tiszta számítás: `FutureMonths`, `BuildForecast`, típusok |
| `backend/internal/services/profitability_forecast_test.go` | A fentiek tesztjei |
| `backend/internal/services/profitability.go` | **Módosul:** `LoadForecast` + két betöltő segéd |
| `backend/internal/handlers/profitability_handler.go` | **Módosul:** `parseForecastMonths`, `GetForecast` |
| `backend/internal/handlers/profitability_handler_test.go` | **Módosul:** `TestParseForecastMonths` |
| `backend/internal/routes/profitability_routes.go` | **Módosul:** 1 útvonal |
| `frontend/src/utils/formatHuf.ts` | Közös Ft-formázó (az oldalról kiemelve) |
| `frontend/src/services/profitabilityService.ts` | **Módosul:** előrejelzés típusok és hívás |
| `frontend/src/components/profitability/ForecastTab.tsx` | Előrejelzés fül: választó, diagram, táblázatok |
| `frontend/src/app/dashboard/profitability/page.tsx` | **Módosul:** fülek, `formatHuf` import |

---

### Task 1: Tiszta előrejelzés (TDD)

**Files:**
- Create: `backend/internal/services/profitability_forecast.go`
- Test: `backend/internal/services/profitability_forecast_test.go`

**Interfaces:**
- Consumes: semmi az 1. szakasz függvényeiből (önálló típusok). A tesztek használják a `profitability_calc_test.go`-ban már definiált `approx(a, b float64) bool` segédet — **ne definiáld újra**.
- Produces:
  - `const forecastBaselineMonths = 3`
  - `func FutureMonths(now time.Time, n int) []string`
  - `func BuildForecast(in ForecastInput) Forecast`
  - típusok: `ForecastInvoice`, `ForecastInput`, `ForecastClient`, `Forecast`

- [ ] **Step 1: Írd meg a bukó teszteket**

```go
// backend/internal/services/profitability_forecast_test.go
package services

import (
	"testing"
	"time"
)

var (
	fcBaseline = []string{"2026-07", "2026-08", "2026-09"}
	fcHorizon  = []string{"2026-10", "2026-11", "2026-12"}
)

func sameFloats(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !approx(a[i], b[i]) {
			return false
		}
	}
	return true
}

// recurring returns one invoice per baseline month for a project/client.
func recurring(project, client uint, amount float64) []ForecastInvoice {
	out := make([]ForecastInvoice, 0, len(fcBaseline))
	for _, m := range fcBaseline {
		out = append(out, ForecastInvoice{ProjectID: project, ClientID: client, Month: m, Amount: amount})
	}
	return out
}

func TestFutureMonthsStartsAtCurrentMonthAndCrossesYear(t *testing.T) {
	now := time.Date(2026, 11, 30, 12, 0, 0, 0, time.UTC)
	got := FutureMonths(now, 3)
	want := []string{"2026-11", "2026-12", "2027-01"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestFutureMonthsZeroIsEmptyNotNil(t *testing.T) {
	if got := FutureMonths(time.Now(), 0); got == nil || len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestBuildForecastUsesBaselineAverage(t *testing.T) {
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices:    recurring(7, 1, 300000),
		ClientNames: map[uint]string{1: "PIXEL"},
	})
	if len(out.Clients) != 1 {
		t.Fatalf("clients = %d", len(out.Clients))
	}
	c := out.Clients[0]
	if c.Name != "PIXEL" || !approx(c.MonthlyAverage, 300000) || c.MonthsWithData != 3 {
		t.Fatalf("client = %+v", c)
	}
	if !sameFloats(out.Committed, []float64{300000, 300000, 300000}) || !sameFloats(out.Dependent, []float64{0, 0, 0}) {
		t.Fatalf("totals committed=%v dependent=%v", out.Committed, out.Dependent)
	}
	if out.LowData || len(out.Warnings) != 0 {
		t.Fatalf("unexpected low data / warnings: %v", out.Warnings)
	}
}

func TestBuildForecastDividesByBaselineLengthNotByActiveMonths(t *testing.T) {
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices: []ForecastInvoice{{ProjectID: 7, ClientID: 1, Month: "2026-09", Amount: 90000}},
	})
	if !approx(out.Clients[0].MonthlyAverage, 30000) {
		t.Fatalf("avg = %v, want 30000", out.Clients[0].MonthlyAverage)
	}
}

func TestBuildForecastSplitsAfterContractEndMonth(t *testing.T) {
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices:         recurring(7, 1, 300000),
		ContractEndMonth: map[uint]string{7: "2026-11"},
	})
	// End month itself is still committed; only months after it depend on renewal.
	if !sameFloats(out.Committed, []float64{300000, 300000, 0}) || !sameFloats(out.Dependent, []float64{0, 0, 300000}) {
		t.Fatalf("committed=%v dependent=%v", out.Committed, out.Dependent)
	}
	c := out.Clients[0]
	if c.ContractEndMonth == nil || *c.ContractEndMonth != "2026-11" {
		t.Fatalf("contract end = %v", c.ContractEndMonth)
	}
}

func TestBuildForecastContractEndedBeforeHorizonIsAllDependent(t *testing.T) {
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices:         recurring(7, 1, 300000),
		ContractEndMonth: map[uint]string{7: "2026-08"},
	})
	if !sameFloats(out.Committed, []float64{0, 0, 0}) || !sameFloats(out.Dependent, []float64{300000, 300000, 300000}) {
		t.Fatalf("committed=%v dependent=%v", out.Committed, out.Dependent)
	}
}

func TestBuildForecastAppliesContractEndPerProject(t *testing.T) {
	invoices := append(recurring(7, 1, 300000), recurring(8, 1, 150000)...)
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices:         invoices,
		ContractEndMonth: map[uint]string{7: "2026-10"}, // project 8 has no end
	})
	if len(out.Clients) != 1 {
		t.Fatalf("clients = %d", len(out.Clients))
	}
	c := out.Clients[0]
	if !approx(c.MonthlyAverage, 450000) {
		t.Fatalf("avg = %v", c.MonthlyAverage)
	}
	if !sameFloats(c.Committed, []float64{450000, 150000, 150000}) || !sameFloats(c.Dependent, []float64{0, 300000, 300000}) {
		t.Fatalf("committed=%v dependent=%v", c.Committed, c.Dependent)
	}
	if c.ContractEndMonth == nil || *c.ContractEndMonth != "2026-10" {
		t.Fatalf("contract end = %v", c.ContractEndMonth)
	}
}

func TestBuildForecastExcludesFixedPriceInvoicesAndWarns(t *testing.T) {
	invoices := append(recurring(7, 1, 100000),
		ForecastInvoice{ProjectID: 9, ClientID: 2, Month: "2026-09", Amount: 600000, Fixed: true})
	out := BuildForecast(ForecastInput{BaselineMonths: fcBaseline, Months: fcHorizon, Invoices: invoices})
	if len(out.Clients) != 1 || out.Clients[0].ClientID != 1 {
		t.Fatalf("clients = %+v", out.Clients)
	}
	if !approx(out.ExcludedFixedRevenue, 600000) {
		t.Fatalf("excluded = %v", out.ExcludedFixedRevenue)
	}
	if len(out.Warnings) != 1 {
		t.Fatalf("warnings = %v", out.Warnings)
	}
}

func TestBuildForecastFlagsLowData(t *testing.T) {
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices: []ForecastInvoice{
			{ProjectID: 7, ClientID: 1, Month: "2026-08", Amount: 100000},
			{ProjectID: 7, ClientID: 1, Month: "2026-09", Amount: 100000},
		},
	})
	if !out.LowData || out.HistoryMonths != 2 || len(out.Warnings) != 1 {
		t.Fatalf("lowData=%v history=%d warnings=%v", out.LowData, out.HistoryMonths, out.Warnings)
	}
}

func TestBuildForecastIgnoresInvoicesOutsideBaseline(t *testing.T) {
	out := BuildForecast(ForecastInput{
		BaselineMonths: fcBaseline, Months: fcHorizon,
		Invoices: []ForecastInvoice{{ProjectID: 7, ClientID: 1, Month: "2026-05", Amount: 999999}},
	})
	if len(out.Clients) != 0 {
		t.Fatalf("clients = %+v", out.Clients)
	}
}

func TestBuildForecastEmptyInputHasNonNilSlices(t *testing.T) {
	out := BuildForecast(ForecastInput{BaselineMonths: fcBaseline, Months: fcHorizon})
	if out.Clients == nil || out.Warnings == nil || out.Committed == nil || out.Dependent == nil {
		t.Fatalf("nil slice in %+v", out)
	}
	if len(out.Committed) != 3 || !out.LowData {
		t.Fatalf("committed=%v lowData=%v", out.Committed, out.LowData)
	}
}

func TestBuildForecastSortsClientsByAverageThenID(t *testing.T) {
	invoices := append(recurring(7, 2, 100000), recurring(8, 1, 100000)...)
	invoices = append(invoices, recurring(9, 3, 500000)...)
	out := BuildForecast(ForecastInput{BaselineMonths: fcBaseline, Months: fcHorizon, Invoices: invoices})
	got := []uint{out.Clients[0].ClientID, out.Clients[1].ClientID, out.Clients[2].ClientID}
	want := []uint{3, 1, 2}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestBuildForecastUnknownClientNameFallsBackToID(t *testing.T) {
	out := BuildForecast(ForecastInput{BaselineMonths: fcBaseline, Months: fcHorizon, Invoices: recurring(7, 42, 1000)})
	if out.Clients[0].Name != "#42" {
		t.Fatalf("name = %q", out.Clients[0].Name)
	}
}
```

- [ ] **Step 2: Futtasd, és ellenőrizd, hogy elbukik**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/services/ -run "FutureMonths|BuildForecast" 2>&1 | tail -10'`
Expected: FAIL (`undefined: FutureMonths`, `undefined: BuildForecast`, `undefined: ForecastInvoice` …).

- [ ] **Step 3: Írd meg a minimális implementációt**

```go
// backend/internal/services/profitability_forecast.go
package services

import (
	"fmt"
	"sort"
	"time"
)

// forecastBaselineMonths is the look-back window the run-rate is averaged
// over; it is also the minimum history before the forecast stops being
// flagged "low data".
const forecastBaselineMonths = 3

// ForecastInvoice is one invoice-month bucket. Fixed marks one-off
// fixed-price invoices, which are not a run-rate and are excluded.
type ForecastInvoice struct {
	ProjectID uint
	ClientID  uint
	Month     string // "YYYY-MM"
	Amount    float64
	Fixed     bool
}

type ForecastInput struct {
	BaselineMonths   []string          // the complete past months averaged over
	Months           []string          // the forecast horizon, "YYYY-MM"
	Invoices         []ForecastInvoice // may include months outside the baseline; ignored
	ContractEndMonth map[uint]string   // project id -> "YYYY-MM" of contract_end_date
	ClientNames      map[uint]string
}

type ForecastClient struct {
	ClientID         uint      `json:"client_id"`
	Name             string    `json:"name"`
	MonthlyAverage   float64   `json:"monthly_average"`
	ContractEndMonth *string   `json:"contract_end_month"`
	MonthsWithData   int       `json:"months_with_data"`
	Committed        []float64 `json:"committed"`
	Dependent        []float64 `json:"dependent"`
}

type Forecast struct {
	BaselineMonths       []string         `json:"baseline_months"`
	Months               []string         `json:"months"`
	Committed            []float64        `json:"committed"`
	Dependent            []float64        `json:"dependent"`
	Clients              []ForecastClient `json:"clients"`
	HistoryMonths        int              `json:"history_months"`
	LowData              bool             `json:"low_data"`
	ExcludedFixedRevenue float64          `json:"excluded_fixed_revenue"`
	Warnings             []string         `json:"warnings"`
}

// FutureMonths returns n "YYYY-MM" labels starting with now's month. The
// current month is included because its invoice is typically issued at the
// start of the next one, so that revenue is still ahead of us.
func FutureMonths(now time.Time, n int) []string {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	months := make([]string, 0, n)
	for i := 0; i < n; i++ {
		months = append(months, start.AddDate(0, i, 0).Format("2006-01"))
	}
	return months
}

// BuildForecast projects each (project, client) pair's baseline average over
// the horizon. A pair's revenue is "committed" up to and including its
// project's contract end month and "dependent" on renewal afterwards.
// Pairs are processed in sorted order so float sums are deterministic.
func BuildForecast(in ForecastInput) Forecast {
	baseline := make(map[string]bool, len(in.BaselineMonths))
	for _, m := range in.BaselineMonths {
		baseline[m] = true
	}

	type pair struct{ project, client uint }
	sums := map[pair]float64{}
	clientMonths := map[uint]map[string]bool{}
	historyMonths := map[string]bool{}
	var excludedFixed float64

	for _, inv := range in.Invoices {
		if !baseline[inv.Month] {
			continue
		}
		if inv.Fixed {
			excludedFixed += inv.Amount
			continue
		}
		sums[pair{inv.ProjectID, inv.ClientID}] += inv.Amount
		if clientMonths[inv.ClientID] == nil {
			clientMonths[inv.ClientID] = map[string]bool{}
		}
		clientMonths[inv.ClientID][inv.Month] = true
		historyMonths[inv.Month] = true
	}

	pairs := make([]pair, 0, len(sums))
	for p := range sums {
		pairs = append(pairs, p)
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].client != pairs[j].client {
			return pairs[i].client < pairs[j].client
		}
		return pairs[i].project < pairs[j].project
	})

	months := in.Months
	if months == nil {
		months = []string{}
	}
	baselineMonths := in.BaselineMonths
	if baselineMonths == nil {
		baselineMonths = []string{}
	}
	horizon := len(months)

	out := Forecast{
		BaselineMonths:       baselineMonths,
		Months:               months,
		Committed:            make([]float64, horizon),
		Dependent:            make([]float64, horizon),
		Clients:              []ForecastClient{},
		HistoryMonths:        len(historyMonths),
		ExcludedFixedRevenue: excludedFixed,
		Warnings:             []string{},
	}

	rows := map[uint]*ForecastClient{}
	// sums is empty when the baseline is empty, so the division below never
	// runs with a zero denominator.
	denominator := float64(len(in.BaselineMonths))
	for _, p := range pairs {
		avg := sums[p] / denominator

		row, ok := rows[p.client]
		if !ok {
			name := in.ClientNames[p.client]
			if name == "" {
				name = fmt.Sprintf("#%d", p.client)
			}
			row = &ForecastClient{
				ClientID:  p.client,
				Name:      name,
				Committed: make([]float64, horizon),
				Dependent: make([]float64, horizon),
			}
			rows[p.client] = row
		}
		row.MonthlyAverage += avg

		endMonth, hasEnd := in.ContractEndMonth[p.project]
		if hasEnd && (row.ContractEndMonth == nil || endMonth < *row.ContractEndMonth) {
			end := endMonth
			row.ContractEndMonth = &end
		}

		for i, m := range months {
			if hasEnd && m > endMonth { // "YYYY-MM" compares correctly as a string
				row.Dependent[i] += avg
				out.Dependent[i] += avg
			} else {
				row.Committed[i] += avg
				out.Committed[i] += avg
			}
		}
	}

	for id, row := range rows {
		row.MonthsWithData = len(clientMonths[id])
		out.Clients = append(out.Clients, *row)
	}
	sort.Slice(out.Clients, func(i, j int) bool {
		if out.Clients[i].MonthlyAverage != out.Clients[j].MonthlyAverage {
			return out.Clients[i].MonthlyAverage > out.Clients[j].MonthlyAverage
		}
		return out.Clients[i].ClientID < out.Clients[j].ClientID
	})

	out.LowData = out.HistoryMonths < forecastBaselineMonths
	if out.LowData {
		out.Warnings = append(out.Warnings, fmt.Sprintf(
			"Csak %d hónapnyi rendszeres számlázási előzmény van, az előrejelzés tájékoztató jellegű", out.HistoryMonths))
	}
	if excludedFixed > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf(
			"%.0f Ft fix áras bevétel nem része az előrejelzésnek (egyszeri számla, nem rendszeres)", excludedFixed))
	}
	return out
}
```

- [ ] **Step 4: Futtasd, és ellenőrizd, hogy átmegy**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -w internal/services/profitability_forecast.go internal/services/profitability_forecast_test.go; gofmt -l internal/services/profitability_forecast*.go; go vet ./internal/services/ && go test ./internal/services/ -run "FutureMonths|BuildForecast" -v 2>&1 | grep -E "^(--- |PASS|FAIL|ok)"'`
Expected: a `gofmt -l` nem listáz fájlt; minden teszt `--- PASS`, a végén `ok`.

- [ ] **Step 5: Futtasd az 1. szakasz tesztjeit is (nem romlott-e el semmi)**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/services/ 2>&1 | tail -3'`
Expected: `ok`.

- [ ] **Step 6: Commit** (csak kérésre)

```bash
git add backend/internal/services/profitability_forecast.go backend/internal/services/profitability_forecast_test.go
git commit -m "feat(profitability): add pure revenue forecast"
```

---

### Task 2: SQL-betöltő

**Files:**
- Modify: `backend/internal/services/profitability.go` (új `LoadForecast`, `loadForecastInvoices`, `loadContractEndMonths` a fájl végére; az import-blokk változatlan marad, mert `time`, `database`, `gorm` már importálva van)

**Interfaces:**
- Consumes: `FutureMonths`, `BuildForecast`, `ForecastInvoice`, `ForecastInput`, `Forecast`, `forecastBaselineMonths` (Task 1); `MonthWindow`, `loadNames` (1. szakasz, ugyanebben a fájlban/csomagban).
- Produces: `func LoadForecast(months int) (Forecast, error)`.

- [ ] **Step 1: Fűzd hozzá a betöltőt a fájl végére**

```go
// LoadForecast builds the revenue forecast: the last forecastBaselineMonths
// complete months of invoices are averaged and projected over `months`
// months starting with the current one.
func LoadForecast(months int) (Forecast, error) {
	db := database.GetDB()
	now := time.Now()
	baseline, from, to := MonthWindow(now, forecastBaselineMonths)

	invoices, err := loadForecastInvoices(db, from, to)
	if err != nil {
		return Forecast{}, err
	}
	endMonths, err := loadContractEndMonths(db)
	if err != nil {
		return Forecast{}, err
	}
	clientNames, err := loadNames(db, "clients")
	if err != nil {
		return Forecast{}, err
	}

	return BuildForecast(ForecastInput{
		BaselineMonths:   baseline,
		Months:           FutureMonths(now, months),
		Invoices:         invoices,
		ContractEndMonth: endMonths,
		ClientNames:      clientNames,
	}), nil
}

// Same "real invoice" definition and month bucketing as loadInvoiceMonths;
// additionally flags fixed-price invoices (NULL pricing_type counts as not
// fixed), which the forecast excludes from the run-rate.
func loadForecastInvoices(db *gorm.DB, from, to time.Time) ([]ForecastInvoice, error) {
	var rows []ForecastInvoice
	err := db.Table("invoices").
		Select("project_id, client_id, to_char(COALESCE(period_end, created_at), 'YYYY-MM') AS month, "+
			"(COALESCE(pricing_type, '') = 'fixed') AS fixed, SUM(amount) AS amount").
		Where("status = ? AND billingo_invoice_id <> ? AND COALESCE(period_end, created_at) >= ? AND COALESCE(period_end, created_at) < ?",
			"created", "", from, to).
		Group("project_id, client_id, to_char(COALESCE(period_end, created_at), 'YYYY-MM'), (COALESCE(pricing_type, '') = 'fixed')").
		Scan(&rows).Error
	return rows, err
}

func loadContractEndMonths(db *gorm.DB) (map[uint]string, error) {
	var rows []struct {
		ID    uint
		Month string
	}
	err := db.Table("projects").
		Select("id, to_char(contract_end_date, 'YYYY-MM') AS month").
		Where("contract_end_date IS NOT NULL").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	months := make(map[uint]string, len(rows))
	for _, r := range rows {
		months[r.ID] = r.Month
	}
	return months, nil
}
```

- [ ] **Step 2: Fordítás, gofmt, vet**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -l internal/services/profitability.go; go vet ./internal/services/ && go build ./... && echo build-ok'`
Expected: `gofmt -l` nem listáz fájlt, `build-ok`.

- [ ] **Step 3: Ellenőrizd mindkét SQL-t ideiglenes adattal, visszagörgetett tranzakcióban**

Feltétel: van legalább egy `users` sor. Az `invoices` kötelező oszlopai az 1. szakasz ellenőrzése alapján: `pricing_type`, `created_by` (a `clients.created_by` is kötelező).

Run:
```bash
cat <<'SQL' | docker compose -f .docker/docker-compose.yml exec -T postgres psql -U devbridge_user -d devbridge -v ON_ERROR_STOP=1 2>&1 | grep -vE "^(INSERT|BEGIN|ROLLBACK)"
BEGIN;
INSERT INTO clients (name, created_by) SELECT '__tmp_fc_client', id FROM users ORDER BY id LIMIT 1;
INSERT INTO projects (name, created_by, contract_end_date)
  SELECT '__tmp_fc_project', id, '2026-11-20' FROM users ORDER BY id LIMIT 1;
-- recurring hourly invoice, September period
INSERT INTO invoices (project_id, client_id, status, billingo_invoice_id, amount, period_end, pricing_type, created_by)
  SELECT p.id, c.id, 'created', 'B-1', 100000, '2026-09-30', 'hourly', p.created_by
  FROM projects p, clients c WHERE p.name='__tmp_fc_project' AND c.name='__tmp_fc_client';
-- one-off fixed invoice, September period (must come back with fixed = true)
INSERT INTO invoices (project_id, client_id, status, billingo_invoice_id, amount, period_end, pricing_type, created_by)
  SELECT p.id, c.id, 'created', 'B-2', 600000, '2026-09-30', 'fixed', p.created_by
  FROM projects p, clients c WHERE p.name='__tmp_fc_project' AND c.name='__tmp_fc_client';
-- failed invoice (must be excluded)
INSERT INTO invoices (project_id, client_id, status, billingo_invoice_id, amount, period_end, pricing_type, created_by)
  SELECT p.id, c.id, 'failed', '', 999999, '2026-09-30', 'hourly', p.created_by
  FROM projects p, clients c WHERE p.name='__tmp_fc_project' AND c.name='__tmp_fc_client';

SELECT 'invoices (varhato: 100000 fixed=f ES 600000 fixed=t, a failed nem)' AS ellenorzes,
       project_id, client_id, to_char(COALESCE(period_end, created_at), 'YYYY-MM') AS month,
       (COALESCE(pricing_type, '') = 'fixed') AS fixed, SUM(amount) AS amount
FROM invoices
WHERE status = 'created' AND billingo_invoice_id <> ''
  AND COALESCE(period_end, created_at) >= '2026-07-01' AND COALESCE(period_end, created_at) < '2026-10-01'
  AND project_id = (SELECT id FROM projects WHERE name='__tmp_fc_project')
GROUP BY project_id, client_id, to_char(COALESCE(period_end, created_at), 'YYYY-MM'), (COALESCE(pricing_type, '') = 'fixed')
ORDER BY fixed;

SELECT 'contract end (varhato 2026-11)' AS ellenorzes, id, to_char(contract_end_date, 'YYYY-MM') AS month
FROM projects WHERE contract_end_date IS NOT NULL AND name='__tmp_fc_project';
ROLLBACK;
SQL
echo "--- cleanup (varhato 0 0) ---"; docker compose -f .docker/docker-compose.yml exec -T postgres psql -U devbridge_user -d devbridge -tc "SELECT (SELECT count(*) FROM clients WHERE name LIKE '__tmp_fc%'), (SELECT count(*) FROM projects WHERE name LIKE '__tmp_fc%');"
```
Expected: az első lekérdezés két sort ad (`fixed = f`, `amount = 100000` és `fixed = t`, `amount = 600000`), a `failed` nem szerepel; a második `2026-11`; a cleanup `0 | 0`. Ha az INSERT-ek valamely kötelező oszlop hiányára hibáznak, csak az oszloplistát egészítsd ki, a vizsgált logikát ne változtasd.

- [ ] **Step 4: Futtasd a valódi betöltőt a dev adatbázison egy ideiglenes, csak olvasó teszttel, majd töröld**

Run:
```bash
cat > backend/internal/services/zz_tmp_forecast_test.go <<'EOF'
package services

import (
	"testing"

	"dev-bridge-manager/internal/database"
)

// TEMPORARY, read-only: runs the real loader against the dev DB. Deleted right after.
func TestTmpLoadForecastAgainstDevDB(t *testing.T) {
	database.Connect()
	f, err := LoadForecast(6)
	if err != nil {
		t.Fatalf("LoadForecast: %v", err)
	}
	t.Logf("baseline=%v horizon=%v clients=%d lowData=%v history=%d warnings=%v",
		f.BaselineMonths, f.Months, len(f.Clients), f.LowData, f.HistoryMonths, f.Warnings)
}
EOF
docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/services/ -run TestTmpLoadForecastAgainstDevDB -v 2>&1 | grep -E "baseline=|FAIL|PASS|panic|error"'
rm -f backend/internal/services/zz_tmp_forecast_test.go; ls backend/internal/services | grep -c zz_tmp
```
Expected: `PASS`, egy `baseline=[…3 hónap…] horizon=[…6 hónap, az aktuálissal kezdve…]` sor, hiba nélkül; az utolsó `ls … | grep -c` kimenete `0` (az ideiglenes fájl törölve).

- [ ] **Step 5: Commit** (csak kérésre)

```bash
git add backend/internal/services/profitability.go
git commit -m "feat(profitability): add forecast SQL loader"
```

---

### Task 3: Végpont (TDD a validációra)

**Files:**
- Modify: `backend/internal/handlers/profitability_handler.go` (két új függvény a fájl végére)
- Modify: `backend/internal/handlers/profitability_handler_test.go` (új teszt a fájl végére)
- Modify: `backend/internal/routes/profitability_routes.go` (1 útvonal)

**Interfaces:**
- Consumes: `services.LoadForecast` (Task 2); az 1. szakasz `ProfitabilityHandler` típusa és importjai (`strconv`, `services`, `fiber` már megvannak).
- Produces: `GET /api/v1/profitability/forecast?months=` ; `func parseForecastMonths(raw string) (int, bool)`.

- [ ] **Step 1: Írd meg a bukó tesztet (a teszt-fájl végére fűzd)**

```go
func TestParseForecastMonths(t *testing.T) {
	cases := []struct {
		in     string
		want   int
		wantOK bool
	}{
		{"", 6, true},
		{"3", 3, true},
		{"6", 6, true},
		{"2", 0, false},
		{"7", 0, false},
		{"0", 0, false},
		{"abc", 0, false},
	}
	for _, tc := range cases {
		got, ok := parseForecastMonths(tc.in)
		if ok != tc.wantOK || (ok && got != tc.want) {
			t.Fatalf("parseForecastMonths(%q) = %d,%v want %d,%v", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
}
```

- [ ] **Step 2: Futtasd, és ellenőrizd, hogy elbukik**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/handlers/ -run "ParseForecastMonths" 2>&1 | tail -6'`
Expected: FAIL (`undefined: parseForecastMonths`).

- [ ] **Step 3: Írd meg a handlert (a handler-fájl végére fűzd)**

```go
// parseForecastMonths returns the requested horizon; empty means the default
// of 6 months, anything outside 3..6 is rejected.
func parseForecastMonths(raw string) (int, bool) {
	if raw == "" {
		return 6, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 3 || n > 6 {
		return 0, false
	}
	return n, true
}

// GetForecast - GET /api/v1/profitability/forecast?months=
func (h *ProfitabilityHandler) GetForecast(c *fiber.Ctx) error {
	months, ok := parseForecastMonths(c.Query("months"))
	if !ok {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "months must be between 3 and 6"})
	}
	forecast, err := services.LoadForecast(months)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading profitability forecast"})
	}
	return c.JSON(forecast)
}
```

- [ ] **Step 4: Add hozzá az útvonalat**

A `backend/internal/routes/profitability_routes.go`-ban a `g.Get("/overview", h.GetOverview)` sor **után**:

```go
	// GET /api/v1/profitability/forecast - 3–6 hónapos bevétel-előrejelzés (biztos és megújítástól függő sáv)
	g.Get("/forecast", h.GetForecast)
```

- [ ] **Step 5: Teszt, gofmt, vet, és az útvonal védettsége**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -l internal/handlers/profitability_handler.go internal/handlers/profitability_handler_test.go internal/routes/profitability_routes.go; go vet ./internal/handlers/ ./internal/routes/ && go test ./internal/handlers/ -run "ParseForecastMonths|ParseOverviewMonths|ValidateProfitSettings|ValidateMeetingAllowance" -v 2>&1 | grep -E "^(--- |PASS|FAIL|ok)"'`
Expected: `gofmt -l` üres, minden teszt `--- PASS`, `ok`.

Majd (a hot reload után ~15 mp):
Run: `sleep 15; curl -s -o /dev/null -w "GET /profitability/forecast token nélkül: HTTP %{http_code}\n" http://localhost:8080/api/v1/profitability/forecast`
Expected: `HTTP 401`.

- [ ] **Step 6: Commit** (csak kérésre)

```bash
git add backend/internal/handlers/profitability_handler.go backend/internal/handlers/profitability_handler_test.go backend/internal/routes/profitability_routes.go
git commit -m "feat(profitability): add forecast endpoint"
```

---

### Task 4: Frontend – fülek, előrejelzés fül, diagram

**Files:**
- Create: `frontend/src/utils/formatHuf.ts`
- Modify: `frontend/src/services/profitabilityService.ts`
- Create: `frontend/src/components/profitability/ForecastTab.tsx`
- Modify: `frontend/src/app/dashboard/profitability/page.tsx`

**Interfaces:**
- Consumes: `GET /profitability/forecast?months=` (Task 3); a meglévő UI-komponensek (`Tabs`, `EmptyState`, `LoadingState`, `ErrorState`); a `dataviz` skill.
- Produces: `formatHuf`, `profitabilityService.forecast(months)`, `ForecastTab` (alapértelmezett export, prop nélkül).

**Előkészület:** a diagram kódja előtt töltsd be a `dataviz` skillt (`Skill` eszköz), és vesd össze vele az alábbi diagramot: a jelölés ne csak színnel különbözzön (a „függő" sáv vonalkázott), legyen olvasható tengely és szöveges/táblázatos alternatíva. Ha a skill ennél szigorúbbat ír, kövesd azt, és jelezd a jelentésben.

- [ ] **Step 1: Emeld ki a Ft-formázót**

```ts
// frontend/src/utils/formatHuf.ts
export const formatHuf = (value: number | null) =>
    value === null ? '—' : `${Math.round(value).toLocaleString('hu-HU')} Ft`;
```

A `page.tsx`-ben töröld a helyi `const formatHuf = …` definíciót (két sor), és add hozzá az importot: `import { formatHuf } from '@/utils/formatHuf';`. A `formatHours` és `formatRatio` marad a helyén.

- [ ] **Step 2: Bővítsd a service-t**

A `frontend/src/services/profitabilityService.ts` típusai közé (a `ProfitabilityOverview` után):

```ts
export interface ForecastClient {
    client_id: number;
    name: string;
    monthly_average: number;
    contract_end_month: string | null;
    months_with_data: number;
    committed: number[];
    dependent: number[];
}

export interface ProfitabilityForecast {
    baseline_months: string[];
    months: string[];
    committed: number[];
    dependent: number[];
    clients: ForecastClient[];
    history_months: number;
    low_data: boolean;
    excluded_fixed_revenue: number;
    warnings: string[];
}
```

és a `profitabilityService` objektumba (az `overview` után):

```ts
    async forecast(months: number): Promise<ProfitabilityForecast> {
        return apiClient.get(`/profitability/forecast?months=${months}`);
    },
```

- [ ] **Step 3: Írd meg az Előrejelzés fület**

```tsx
// frontend/src/components/profitability/ForecastTab.tsx
'use client';

import React from 'react';
import { profitabilityService, ProfitabilityForecast } from '@/services/profitabilityService';
import EmptyState from '@/components/ui/EmptyState';
import LoadingState from '@/components/ui/LoadingState';
import ErrorState from '@/components/ui/ErrorState';
import { formatHuf } from '@/utils/formatHuf';

const errorMessage = (err: unknown) => (err instanceof Error ? err.message : String(err));

// Rounds a maximum up to 1, 2, 5 or 10 x a power of ten so the axis ticks are readable.
function niceMax(value: number): number {
    if (value <= 0) return 1;
    const exp = Math.pow(10, Math.floor(Math.log10(value)));
    const f = value / exp;
    const nice = f <= 1 ? 1 : f <= 2 ? 2 : f <= 5 ? 5 : 10;
    return nice * exp;
}

function compactHuf(value: number): string {
    if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1).replace(/\.0$/, '')} M Ft`;
    if (value >= 1_000) return `${Math.round(value / 1_000)} e Ft`;
    return `${Math.round(value)} Ft`;
}

interface ForecastChartProps {
    months: string[];
    committed: number[];
    dependent: number[];
}

// Stacked bars per month: solid = committed, hatched = depends on renewal.
// The hatch (not only the colour) separates the two series; the table below
// the chart repeats every value as text.
function ForecastChart({ months, committed, dependent }: ForecastChartProps) {
    const W = 640;
    const H = 260;
    const margin = { top: 12, right: 12, bottom: 28, left: 64 };
    const plotW = W - margin.left - margin.right;
    const plotH = H - margin.top - margin.bottom;

    const totals = months.map((_, i) => (committed[i] ?? 0) + (dependent[i] ?? 0));
    const max = niceMax(Math.max(0, ...totals));
    const y = (v: number) => margin.top + plotH - (v / max) * plotH;
    const band = plotW / Math.max(months.length, 1);
    const barW = Math.min(56, band * 0.6);
    const ticks = [0, 0.5, 1].map((f) => f * max);

    return (
        <svg
            viewBox={`0 0 ${W} ${H}`}
            role="img"
            aria-label={`Bevétel-előrejelzés ${months[0] ?? ''} és ${months[months.length - 1] ?? ''} között, havonta`}
            className="w-full h-auto"
        >
            <defs>
                <pattern id="dependent-hatch" patternUnits="userSpaceOnUse" width="6" height="6" patternTransform="rotate(45)">
                    <rect width="6" height="6" className="fill-primary/15" />
                    <line x1="0" y1="0" x2="0" y2="6" strokeWidth="2" className="stroke-primary" />
                </pattern>
            </defs>

            {ticks.map((t) => (
                <g key={t}>
                    <line x1={margin.left} x2={W - margin.right} y1={y(t)} y2={y(t)} className="stroke-border" strokeWidth="1" />
                    <text x={margin.left - 8} y={y(t) + 3} textAnchor="end" className="fill-muted-foreground" fontSize="10">
                        {compactHuf(t)}
                    </text>
                </g>
            ))}

            {months.map((m, i) => {
                const c = committed[i] ?? 0;
                const d = dependent[i] ?? 0;
                const x = margin.left + band * i + (band - barW) / 2;
                return (
                    <g key={m}>
                        <rect x={x} y={y(c)} width={barW} height={Math.max(y(0) - y(c), 0)} className="fill-primary" />
                        <rect
                            x={x}
                            y={y(c + d)}
                            width={barW}
                            height={Math.max(y(c) - y(c + d), 0)}
                            fill="url(#dependent-hatch)"
                            className="stroke-primary"
                            strokeWidth="1"
                        />
                        <text x={x + barW / 2} y={H - 8} textAnchor="middle" className="fill-muted-foreground" fontSize="10">
                            {m}
                        </text>
                    </g>
                );
            })}
        </svg>
    );
}

export default function ForecastTab() {
    const [months, setMonths] = React.useState(6);
    const [data, setData] = React.useState<ProfitabilityForecast | null>(null);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);

    const load = React.useCallback(async () => {
        try {
            setLoading(true);
            setError(null);
            setData(await profitabilityService.forecast(months));
        } catch (err: unknown) {
            setError(errorMessage(err));
        } finally {
            setLoading(false);
        }
    }, [months]);

    React.useEffect(() => {
        load();
    }, [load]);

    return (
        <div>
            <div className="flex items-center justify-between mb-4">
                <p className="text-xs text-muted-foreground max-w-2xl">
                    Az előrejelzés az utolsó {data?.baseline_months.length ?? 3} teljes hónap rendszeres
                    (nem fix áras) számláinak ügyfélenkénti átlagát vetíti előre, az aktuális hónappal kezdve. A
                    szerződés lejárata utáni hónapok bevétele a „megújítástól függő” sávba kerül.
                </p>
                <select
                    value={months}
                    onChange={(e) => setMonths(Number(e.target.value))}
                    className="border border-border rounded-md bg-background text-sm px-2 py-1.5"
                >
                    {[3, 6].map((m) => (
                        <option key={m} value={m}>
                            Következő {m} hónap
                        </option>
                    ))}
                </select>
            </div>

            {loading && <LoadingState message="Előrejelzés betöltése..." />}
            {!loading && error && <ErrorState error={error} onRetry={load} />}

            {!loading && !error && data && (
                <>
                    {data.warnings.map((w) => (
                        <div key={w} className="bg-muted border border-border text-foreground px-3 py-2 rounded text-sm mb-4">
                            {w}
                        </div>
                    ))}

                    {data.clients.length === 0 ? (
                        <EmptyState
                            icon="files"
                            title="Nincs rendszeres bevétel az alapidőszakban"
                            description="Az előrejelzéshez kiállított, nem fix áras számla kell az utolsó 3 teljes hónapból."
                        />
                    ) : (
                        <>
                            <div className="bg-card border border-border rounded-lg p-4 mb-4">
                                <ForecastChart months={data.months} committed={data.committed} dependent={data.dependent} />
                                <div className="flex items-center gap-4 mt-2 text-xs text-muted-foreground">
                                    <span className="inline-flex items-center gap-1.5">
                                        <span className="inline-block w-3 h-3 bg-primary" /> Biztos
                                    </span>
                                    <span className="inline-flex items-center gap-1.5">
                                        <svg width="12" height="12" aria-hidden="true">
                                            <rect width="12" height="12" className="fill-primary/15 stroke-primary" strokeWidth="1" />
                                            <line x1="0" y1="12" x2="12" y2="0" className="stroke-primary" strokeWidth="1.5" />
                                        </svg>
                                        Megújítástól függő
                                    </span>
                                </div>
                            </div>

                            <section className="mb-6">
                                <h2 className="text-lg font-semibold text-foreground mb-3">Havi összesítő</h2>
                                <div className="overflow-x-auto bg-card border border-border rounded-lg">
                                    <table className="w-full text-sm">
                                        <thead className="text-left text-muted-foreground border-b border-border">
                                            <tr>
                                                <th className="p-3">Hónap</th>
                                                <th className="p-3 text-right">Biztos</th>
                                                <th className="p-3 text-right">Megújítástól függő</th>
                                                <th className="p-3 text-right">Összesen</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {data.months.map((m, i) => (
                                                <tr key={m} className="border-b border-border last:border-0">
                                                    <td className="p-3 text-foreground">{m}</td>
                                                    <td className="p-3 text-right">{formatHuf(data.committed[i])}</td>
                                                    <td className="p-3 text-right">{formatHuf(data.dependent[i])}</td>
                                                    <td className="p-3 text-right font-medium">
                                                        {formatHuf(data.committed[i] + data.dependent[i])}
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            </section>

                            <section>
                                <h2 className="text-lg font-semibold text-foreground mb-3">Ügyfelek</h2>
                                <div className="overflow-x-auto bg-card border border-border rounded-lg">
                                    <table className="w-full text-sm">
                                        <thead className="text-left text-muted-foreground border-b border-border">
                                            <tr>
                                                <th className="p-3">Ügyfél</th>
                                                <th className="p-3 text-right">Havi átlag</th>
                                                <th className="p-3 text-right">Szerződés vége</th>
                                                <th className="p-3 text-right">Biztos ({data.months.length} hó)</th>
                                                <th className="p-3 text-right">Megújítástól függő</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            {data.clients.map((c) => (
                                                <tr key={c.client_id} className="border-b border-border last:border-0">
                                                    <td className="p-3 text-foreground">
                                                        {c.name}
                                                        {c.months_with_data < data.baseline_months.length && (
                                                            <span
                                                                className="ml-2 text-xs bg-muted text-muted-foreground rounded-full px-2 py-0.5"
                                                                title={`Csak ${c.months_with_data} hónapban volt számlázva az alapidőszakban`}
                                                            >
                                                                kevés adat
                                                            </span>
                                                        )}
                                                    </td>
                                                    <td className="p-3 text-right">{formatHuf(c.monthly_average)}</td>
                                                    <td className="p-3 text-right">{c.contract_end_month ?? '—'}</td>
                                                    <td className="p-3 text-right">
                                                        {formatHuf(c.committed.reduce((a, b) => a + b, 0))}
                                                    </td>
                                                    <td className="p-3 text-right">
                                                        {formatHuf(c.dependent.reduce((a, b) => a + b, 0))}
                                                    </td>
                                                </tr>
                                            ))}
                                        </tbody>
                                    </table>
                                </div>
                            </section>
                        </>
                    )}
                </>
            )}
        </div>
    );
}
```

- [ ] **Step 4: Építsd be a füleket a `page.tsx`-be**

Olvasd el előbb a fájlt, majd pontosan ezt a négy változtatást végezd el (a modalokhoz és a betöltő logikához nem nyúlsz):

1. Importok: `import { Tabs } from '@/components/ui/tabs';` és `import ForecastTab from '@/components/profitability/ForecastTab';`.
2. Állapot, a többi `useState` mellé:
   ```tsx
   const [activeTab, setActiveTab] = React.useState<'overview' | 'forecast'>('overview');
   ```
3. A fejléc (`<div className="flex items-center justify-between mb-6">` blokk) után, a hibadoboz (`{actionError && !showSettings && …}`) **elé** szúrd be:
   ```tsx
   <Tabs
       className="mb-6"
       tabs={[
           { id: 'overview', label: 'Áttekintés' },
           { id: 'forecast', label: 'Előrejelzés' },
       ]}
       activeTab={activeTab}
       onChange={(id) => setActiveTab(id as 'overview' | 'forecast')}
   />
   ```
4. Az **Áttekintés tartalmát** (a hibadoboztól a `{!loading && !error && data && (…)}` blokk végéig — vagyis minden, ami a `<Tabs>` és az első `<Modal>` között van) csomagold be: `{activeTab === 'overview' && (<> … </>)}`. Ugyanide jön az `{activeTab === 'forecast' && <ForecastTab />}`. A fejlécben a hónap-választó `<select>` és a „Beállítások" gomb csak az Áttekintés fülön látszódjon: a `<div className="flex items-center gap-2">`, ami őket tartalmazza, kapjon `activeTab === 'overview' &&` feltételt. A két `<Modal>` a feltételes blokkokon kívül marad.

- [ ] **Step 5: Típusellenőrzés, lint, futásidejű fordulás**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cd /app && npx tsc --noEmit -p . 2>&1 | grep -E "profitability|ForecastTab|formatHuf" || echo "tsc: nincs hiba az érintett fájlokban"; npx eslint src/utils/formatHuf.ts src/services/profitabilityService.ts src/components/profitability/ForecastTab.tsx src/app/dashboard/profitability/page.tsx 2>&1 | tail -12; echo eslint-done'`
Expected: `tsc: nincs hiba az érintett fájlokban`, az eslint kimenete üres a `eslint-done` előtt. (A projekt 6 régi `tsc` hibája más fájlokban van; ne nyúlj hozzájuk.)

Run: `curl -s -o /dev/null -w "oldal: HTTP %{http_code}\n" http://localhost:3010/dashboard/profitability; docker compose -f .docker/docker-compose.yml logs --tail 20 frontend 2>&1 | grep -iE "error|failed to compile" | tail -5 || true`
Expected: `HTTP 200` (vagy átirányítás a belépésre), fordítási hiba nélkül.

- [ ] **Step 6: Commit** (csak kérésre)

```bash
git add frontend/src/utils/formatHuf.ts frontend/src/services/profitabilityService.ts frontend/src/components/profitability/ForecastTab.tsx frontend/src/app/dashboard/profitability/page.tsx
git commit -m "feat(profitability): add forecast tab with stacked chart"
```

---

### Task 5: Végellenőrzés és dokumentáció

- [ ] **Step 1: Teljes backend-ellenőrzés konténerben**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -l internal/services/profitability*.go internal/handlers/profitability_handler*.go internal/routes/profitability_routes.go; go vet ./internal/... 2>&1 | tail -3; go build ./... && echo build-ok; go test ./internal/services/ ./internal/handlers/ ./internal/middleware/ 2>&1 | tail -5'`
Expected: `gofmt -l` üres, `build-ok`, mindhárom csomag `ok`. Ha egy **régi** teszt bukik, külön jelöld pre-existing-ként.

- [ ] **Step 2: Számítsd ki kézzel egy példát, és vesd össze**

Vedd a Task 1 `TestBuildForecastAppliesContractEndPerProject` esetét: projekt 7 (300 000/hó, vége 2026-10) + projekt 8 (150 000/hó, nincs vége), ügyfél 1. Kézzel: okt = 450 000 biztos; nov, dec = 150 000 biztos + 300 000 függő. Egyezik a teszt elvárásával; ha nem, a hiba a számításban van.

- [ ] **Step 3: Böngészős ellenőrzés (ha van belépés)**

Jelentkezz be egy `admin` / `super_admin` felhasználóval a `http://localhost:3010`-en, nyisd meg a **Jövedelmezőség** → **Előrejelzés** fület. Ellenőrizd: a fülváltás működik; üres adatnál az `EmptyState` jelenik meg; a 3/6 hónap választó újratölt; a legenda és a vonalkázott sáv látszik; sötét és világos módban is olvasható. Ha nincs belépési adat, ezt **ne állítsd ellenőrzöttnek**: írd le, hogy a vizuális rész nem volt kipróbálva.

- [ ] **Step 4: Frissítsd a specet**

A `docs/superpowers/specs/2026-10-01-profitability-design.md` „Nyitott pontok" részéhez fűzd hozzá:

```markdown
- **2. szakasz döntései:** a fix áras számlák kimaradnak az előrejelzésből (egyszeri számla, figyelmeztetéssel); a horizont az aktuális hónappal kezdődik; a szerződés vége projektenként érvényesül (a lejárat hónapja még „biztos"); alapértelmezett horizont 6 hónap (3–6); az oldal két fülre oszlik, a Forgatókönyvek és a Beállítások fül a 3–4. szakaszban jön.
- **Előrejelzés korlátja:** az átlag az utolsó 3 hónapra támaszkodik, ezért új vagy megszűnő ügyfelet lassan követ, és pipeline nincs (új bevétel csak forgatókönyvben szerepelhet).
```

- [ ] **Step 5: Commit** (csak kérésre)

```bash
git add docs/superpowers/specs/2026-10-01-profitability-design.md
git commit -m "docs(profitability): record stage 2 forecast decisions"
```

---

## Self-Review

**Spec-lefedettség (csak a 2. szakasz):**
- „Előrejelzés (alap): az utolsó 3 teljes hónap ügyfélenkénti átlaga" → Task 1 (`BuildForecast`, osztó = 3) ✔
- „contract_end_date a horizonton belül → külön »megújítástól függő« sáv" → Task 1 + 2 (`ContractEndMonth`, `loadContractEndMonths`) ✔
- „Kevés adat: 3 hónapnál rövidebb előzménynél figyelmeztetés" → Task 1 (`LowData`, `HistoryMonths`, figyelmeztetés) ✔
- `GET /forecast?months=3..6` + `profitability.read` → Task 3 ✔
- „Előrejelzés – 3–6 hónapos diagram, a megújítástól függő sáv külön színnel (`dataviz` skill)" → Task 4 (SVG, vonalkázás + szín, `dataviz` előkészület) ✔
- „Pipeline nincs" → nem épül be; a korlát a specbe kerül (Task 5) ✔
- Nem ennek a tervnek a része: forgatókönyvek, paraméterkészletek, összehasonlítás, sablonok, Beállítások fül (3–4. szakasz).

**Eltérések a spec szövegétől (a felhasználónak jelezni):**
1. A **fix áras számlák kimaradnak** (a spec ezt nem mondta ki; egyszeri számla kivetítése hamis lenne).
2. A horizont az **aktuális hónappal kezdődik** (a spec „következő hónapok"-at írt, de az aktuális hónap számlája még nincs kiállítva).
3. A **diagram kézzel rajzolt SVG**, nem könyvtár, mert a rendszerben nincs chart-függőség, és új függőséget nem vezetünk be indoklás nélkül.
4. Az **alapértelmezett horizont 6 hónap** (a spec csak a 3–6 tartományt rögzítette).

**Placeholder-ellenőrzés:** nincs TBD/TODO; minden kódlépésben teljes kód van. A Task 4 4. lépése a `page.tsx` szerkezeti átalakítását pontos utasításként adja (nem szó szerinti kódként), mert a fájl az 1. szakasz utáni UX-javítások miatt változott; ott az implementernek előbb el kell olvasnia a fájlt.

**Típus-konzisztencia:** a Go JSON-címkék (`baseline_months`, `months`, `committed`, `dependent`, `clients[].client_id|name|monthly_average|contract_end_month|months_with_data|committed|dependent`, `history_months`, `low_data`, `excluded_fixed_revenue`, `warnings`) megegyeznek a TS `ProfitabilityForecast` / `ForecastClient` mezőivel. A függvénynevek (`FutureMonths`, `BuildForecast`, `LoadForecast`, `parseForecastMonths`, `loadForecastInvoices`, `loadContractEndMonths`) minden taskban azonosak. Az `approx` segédet a Task 1 a meglévő `profitability_calc_test.go`-ból veszi át (nem definiálja újra).

**Nem ellenőrzött feltevések:**
- A `fill-primary/15`, `stroke-primary` Tailwind-osztályok az SVG-ben a projekt Tailwind-verziójával működnek-e, azt csak böngészőben lehet látni (Task 5 / 3. lépés).
- A `invoices.pricing_type` valós értékei (`hourly` / `fixed`) a kódból és a korábbi ellenőrzésből ismertek, de a dev adatbázisban nincs számla, ezért a szűrés valós adaton nem volt kipróbálva.
