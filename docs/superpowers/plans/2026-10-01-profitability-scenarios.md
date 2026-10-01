# Jövedelmezőség – 3. szakasz: paraméterkészletek és forgatókönyvek Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Mentett „mi lenne, ha” forgatókönyvek: ügyfélenkénti ár-, óra- és kiesés-módosítás, új ügyfél, saját kapacitás és költség-paraméterkészlet mellett élő nettó nyereség az alaphelyzethez képest.

**Architecture:** Az 1–2. szakasz mintáját követi. Egy tiszta Go motor (`BuildScenario`) a havi alapadatból (az előrejelzés ügyfélátlagai + az áttekintés órái) és a mentett paraméterekből számol; egy vékony betöltő adja az alapadatot. Két új tábla (JSONB listákkal), CRUD-végpontok és egy mentés nélküli `compute` végpont a meglévő `/api/v1/profitability` csoportban. A felület a harmadik fül (`Forgatókönyvek`), az élő eredményt a szerver számolja.

**Tech Stack:** Go + Fiber + GORM + golang-migrate (backend konténer), Next.js + TypeScript (frontend konténer), PostgreSQL.

**Spec:** `docs/superpowers/specs/2026-10-01-profitability-design.md` (ez a terv a **3. szállítási szakaszt** valósítja meg; az összehasonlító nézet, a sablonok és a Beállítások fül a 4. szakaszban jön.)

## Global Constraints

- **Nincs beépített adó- vagy járulékszabály, nincs alapértelmezett kulcs.** A százalékos tételek szerkezete fix (`label`, `percent`, `base`), az értékeket a felhasználó adja. A számítás eredménye nem adótanács.
- Az alapadat az **utolsó 3 teljes hónap** (az 1–2. szakasz „valódi számla” definíciója): a bevétel a nem fix áras számlák ügyfélátlaga (`ForecastClient.MonthlyAverage`), az órák az áttekintés ügyfélsorából (`LoggedHours`, `EmailHours`, `MeetingHours`) havi átlagra osztva. Az `invoices.amount` nettó (az 1. szakasz Ruling 1-je).
- Jogosultság: a meglévő `profitability.read` (olvasás, `compute`) és `profitability.manage` (írás). Új jog nem kell.
- `profit_scenarios.parameter_set_id` → `ON DELETE RESTRICT`; a paraméterkészlet nem törölhető, amíg forgatókönyv hivatkozik rá (a végpont `409`-et ad az érintett darabszámmal).
- Minden ellenőrzés (go test, gofmt, go vet, tsc, eslint, psql) **a projekt konténereiben** fut, a repo gyökeréből: `docker compose -f .docker/docker-compose.yml exec -T backend|frontend|postgres sh -c '…'`. Host-oldali futtatás tilos.
- Adatbázis-ellenőrzés csak a lokális dev adatbázison, **visszagörgetett tranzakcióban**, ideiglenes tesztadattal. Valós vagy megosztott adat nem érintett.
- **Commit, merge és push csak a felhasználó kifejezett kérésére.** A terv commit-lépései addig nem futnak le. `git add` csak a felsorolt fájlokra.
- A working tree-ben **idegen, nem commitolt munka** van (Jira: `activity_digest_handler.go`, `jira_*`, `task_comment_handler.go`, `models/kanban.go`, `services/jira_*`, `migrations/000040_*`, `docs/.../marketing-contacts.md`). Ezekhez nem nyúlunk.
- Új függőség tilos. A `gofmt -w` minden új vagy módosított Go fájlon kötelező, majd `gofmt -l` üres.
- Munkabranch: `feat/profitability-scenarios` (a `main`-ről).
- A migráció az alkalmazás indulásakor fut le (a Go fájl-módosítás újraindítja a backendet). **Kézzel ne alkalmazd**; a Task 6 ellenőrzi, hogy a `schema_migrations` 45-ös, nem dirty.

## Döntések (Rulings a tervben; a felhasználónak jelezni)

1. **Az alaphelyzet „lapos”:** minden horizont-hónap ugyanazt a havi eredményt kapja. A szerződés lejárata (megújítás-függő sáv) a forgatókönyvben **nem** módosítja a bevételt; a horizont csak hónapcímkét és szorzót ad. (Az 1. szakasz előrejelzése külön marad.)
2. **Az alapban csak rendszeres bevételű ügyfél van** (ugyanaz a szűrés, mint az előrejelzésé: fix áras számla nem része). Fix áras ügyfelet `new_clients`-ként lehet modellezni.
3. **Az óraváltozás a naplózott órákra hat**, a levelezés és megbeszélés fix marad. Új óradíj: `bevétel = díj × (naplózott + változás)`. Új fix ár: `bevétel = ár`. Ár nélküli óraváltozás: a bevétel arányosan skálázódik a jelenlegi névleges díjon (`bevétel × új naplózott / régi naplózott`). Kiesés: bevétel és óra is 0. A naplózott óra nem lehet negatív (0-ra vágjuk).
4. **Százalékos tételek:** a `revenue` alapú tétel a bevétel %-a. Az `after_costs` alapú tétel az „eredmény a költségek után” (`bevétel − fix költségek − bevétel-alapú tételek`) %-a; több ilyen tétel **nem halmozódik** (mind ugyanazon az eredményen számol), és **negatív eredményen 0** (nincs negatív adó). Ez modellezési választás, nem adószabály.
5. **A „kapacitás” kötelező és pozitív** (0 < kapacitás ≤ 744 óra/hó). A fő mérőszám a nettó nyereség kapacitásórára vetítve.
6. **Új ügyfél `monthly_hours`-a összes óra** (nem csak naplózott).
7. **Nincs szerver-oldali cache:** minden `compute` hívás újratölti az alapadatot (néhány lekérdezés). A felület 400 ms-ot vár gépelés után (debounce) és a régi válaszokat eldobja. Ha ez lassúnak bizonyul, a pillanatképes megközelítés a betöltő mögé tehető.
8. **A paraméterkészletek kezelése** a Forgatókönyvek fülről nyíló ablakban van; a spec szerinti Beállítások fül a 4. szakaszban jön.
9. **A `JSONB` listák egy általános `models.JSONList[T]` típuson** keresztül mennek (a repo más JSONB-oszlopai sima `string`-ek; itt a tipizált lista kevesebb kézi kódolást jelent). A típus külön teszttel védett.

## File Structure

| Fájl | Felelősség |
|---|---|
| `backend/migrations/000045_add_profit_scenarios.{up,down}.sql` | `profit_parameter_sets`, `profit_scenarios` |
| `backend/internal/models/json_list.go` (+ `_test.go`) | `JSONList[T]`: jsonb ↔ típusos lista |
| `backend/internal/models/profit_scenario.go` | Modellek és kérés-típusok |
| `backend/internal/services/profitability_scenario.go` (+ `_test.go`) | Tiszta motor: `BuildScenario` |
| `backend/internal/services/profitability_scenario_load.go` | `LoadScenarioBase`, `ComputeScenario` |
| `backend/internal/handlers/profitability_scenario_handler.go` (+ `_test.go`) | Validáció és CRUD + compute |
| `backend/internal/routes/profitability_routes.go` | **Módosul:** 9 útvonal |
| `frontend/src/services/profitabilityService.ts` | **Módosul:** típusok és hívások |
| `frontend/src/components/profitability/ParameterSetsModal.tsx` | Paraméterkészlet-kezelő ablak |
| `frontend/src/components/profitability/ScenarioResultPanel.tsx` | Alap vs forgatókönyv táblázat, „mivel számolt” |
| `frontend/src/components/profitability/ScenarioEditor.tsx` | Szerkesztő: piszkozat, élő számolás, mentés |
| `frontend/src/components/profitability/ScenariosTab.tsx` | Lista, kiválasztás, ablakok |
| `frontend/src/app/dashboard/profitability/page.tsx` | **Módosul:** harmadik fül |

---

### Task 1: Migráció

**Files:**
- Create: `backend/migrations/000045_add_profit_scenarios.up.sql`
- Create: `backend/migrations/000045_add_profit_scenarios.down.sql`

**Interfaces:**
- Produces: tábla `profit_parameter_sets(id, name, percent_items JSONB, fixed_monthly_costs JSONB, created_by, created_at, updated_at)` és `profit_scenarios(id, name, horizon_months, parameter_set_id, capacity_hours_per_month, client_adjustments JSONB, new_clients JSONB, created_by, created_at, updated_at)`.

- [ ] **Step 1: Írd meg az up migrációt**

```sql
CREATE TABLE profit_parameter_sets (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    percent_items JSONB NOT NULL DEFAULT '[]',
    fixed_monthly_costs JSONB NOT NULL DEFAULT '[]',
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE profit_scenarios (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    horizon_months INTEGER NOT NULL CHECK (horizon_months BETWEEN 3 AND 6),
    parameter_set_id INTEGER NOT NULL REFERENCES profit_parameter_sets(id) ON DELETE RESTRICT,
    capacity_hours_per_month NUMERIC(6,2) NOT NULL,
    client_adjustments JSONB NOT NULL DEFAULT '[]',
    new_clients JSONB NOT NULL DEFAULT '[]',
    created_by INTEGER NOT NULL REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_profit_scenarios_parameter_set ON profit_scenarios (parameter_set_id);
```

- [ ] **Step 2: Írd meg a down migrációt**

```sql
DROP INDEX IF EXISTS idx_profit_scenarios_parameter_set;
DROP TABLE IF EXISTS profit_scenarios;
DROP TABLE IF EXISTS profit_parameter_sets;
```

- [ ] **Step 3: Próbáld ki visszagörgetett tranzakcióban: up, kényszerek, down, újra up**

Run:
```bash
{ echo "BEGIN;"; cat backend/migrations/000045_add_profit_scenarios.up.sql
echo "INSERT INTO profit_parameter_sets (name, created_by) SELECT '__tmp_set', id FROM users ORDER BY id LIMIT 1;"
echo "INSERT INTO profit_scenarios (name, horizon_months, parameter_set_id, capacity_hours_per_month, created_by) SELECT '__tmp_sc', 6, s.id, 120, s.created_by FROM profit_parameter_sets s WHERE s.name='__tmp_set';"
echo "SELECT 'insert_ok' AS lepes, count(*) FROM profit_scenarios;"
echo "SAVEPOINT a; DELETE FROM profit_parameter_sets WHERE name='__tmp_set';"
echo "ROLLBACK TO SAVEPOINT a;"
echo "SAVEPOINT b; INSERT INTO profit_scenarios (name, horizon_months, parameter_set_id, capacity_hours_per_month, created_by) SELECT 'bad', 7, s.id, 120, s.created_by FROM profit_parameter_sets s WHERE s.name='__tmp_set';"
echo "ROLLBACK TO SAVEPOINT b;"
cat backend/migrations/000045_add_profit_scenarios.down.sql
echo "SELECT to_regclass('profit_scenarios') IS NULL AS down_ok;"
cat backend/migrations/000045_add_profit_scenarios.up.sql
echo "SELECT to_regclass('profit_scenarios') IS NOT NULL AS up_again_ok;"
echo "ROLLBACK;"; } | docker compose -f .docker/docker-compose.yml exec -T postgres psql -U devbridge_user -d devbridge 2>&1 | grep -vE "^(INSERT|DELETE|DROP|CREATE|BEGIN|ROLLBACK|SAVEPOINT)"
```
Expected: `insert_ok | 1`; **két** `ERROR` sor (a `DELETE` a `profit_scenarios_parameter_set_id_fkey` megsértése miatt, és a `horizon_months = 7` a CHECK miatt) — ezek a várt kényszer-ellenőrzések; `down_ok = t`; `up_again_ok = t`. Más hiba ne legyen. A `psql` itt szándékosan nem áll meg az első hibán (nincs `ON_ERROR_STOP`), mert a SAVEPOINT-ok visszaállítják a tranzakciót.

- [ ] **Step 4: Ellenőrizd, hogy nem maradt semmi**

Run: `docker compose -f .docker/docker-compose.yml exec -T postgres psql -U devbridge_user -d devbridge -tc "SELECT to_regclass('profit_scenarios'), (SELECT count(*) FROM schema_migrations WHERE version = 45);"`
Expected: a `to_regclass` üres (a tábla még nem létezik valósan), a verziószám-darab `0`. (A migráció a Task 6-ban, a backend újraindulásakor alkalmazódik.)

- [ ] **Step 5: Commit** (csak kérésre)

```bash
git add backend/migrations/000045_add_profit_scenarios.up.sql backend/migrations/000045_add_profit_scenarios.down.sql
git commit -m "feat(profitability): add parameter sets and scenarios tables"
```

---

### Task 2: `JSONList` és a modellek (TDD a `JSONList`-re)

**Files:**
- Create: `backend/internal/models/json_list.go`
- Test: `backend/internal/models/json_list_test.go`
- Create: `backend/internal/models/profit_scenario.go`

**Interfaces:**
- Produces:
  - `models.JSONList[T any] []T` (`Value`, `Scan`, `MarshalJSON`)
  - `models.ProfitPercentItem{Label, Percent, Base}`, `models.ProfitFixedCost{Label, Amount}`
  - `models.ProfitParameterSet`, `models.ProfitParameterSetRequest`
  - `models.ProfitScenarioAdjustment{ClientID, NewHourlyRate *float64, NewFixedPrice *float64, HoursDelta, Drops}`, `models.ProfitScenarioNewClient{Name, MonthlyRevenue, MonthlyHours}`
  - `models.ProfitScenario`, `models.ProfitScenarioRequest`

- [ ] **Step 1: Írd meg a bukó tesztet**

```go
// backend/internal/models/json_list_test.go
package models

import (
	"encoding/json"
	"testing"
)

type jlItem struct {
	Label string  `json:"label"`
	N     float64 `json:"n"`
}

func TestJSONListValueNilIsEmptyArray(t *testing.T) {
	var l JSONList[jlItem]
	v, err := l.Value()
	if err != nil || v != "[]" {
		t.Fatalf("got %v, %v", v, err)
	}
}

func TestJSONListValueMarshalsItems(t *testing.T) {
	v, err := JSONList[jlItem]{{Label: "a", N: 1.5}}.Value()
	if err != nil || v != `[{"label":"a","n":1.5}]` {
		t.Fatalf("got %v, %v", v, err)
	}
}

func TestJSONListScanAcceptsBytesAndString(t *testing.T) {
	for _, src := range []any{[]byte(`[{"label":"x","n":2}]`), `[{"label":"x","n":2}]`} {
		var l JSONList[jlItem]
		if err := l.Scan(src); err != nil {
			t.Fatalf("scan %T: %v", src, err)
		}
		if len(l) != 1 || l[0].Label != "x" || l[0].N != 2 {
			t.Fatalf("scan %T: got %+v", src, l)
		}
	}
}

func TestJSONListScanNilIsEmptyNotNil(t *testing.T) {
	l := JSONList[jlItem]{{Label: "stale"}}
	if err := l.Scan(nil); err != nil {
		t.Fatal(err)
	}
	if l == nil || len(l) != 0 {
		t.Fatalf("got %#v", l)
	}
}

func TestJSONListScanRejectsBadInput(t *testing.T) {
	var l JSONList[jlItem]
	if err := l.Scan(`{"not":"a list"}`); err == nil {
		t.Fatal("expected error for a JSON object")
	}
	if err := l.Scan(42); err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestJSONListMarshalJSONNilIsEmptyArray(t *testing.T) {
	b, err := json.Marshal(struct {
		L JSONList[jlItem] `json:"l"`
	}{})
	if err != nil || string(b) != `{"l":[]}` {
		t.Fatalf("got %s, %v", b, err)
	}
}

func TestJSONListRoundTrip(t *testing.T) {
	in := JSONList[jlItem]{{Label: "a", N: 1}, {Label: "b", N: 2}}
	v, err := in.Value()
	if err != nil {
		t.Fatal(err)
	}
	var out JSONList[jlItem]
	if err := out.Scan(v); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[1].Label != "b" || out[1].N != 2 {
		t.Fatalf("got %+v", out)
	}
}
```

- [ ] **Step 2: Futtasd, és ellenőrizd, hogy elbukik**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/models/ -run JSONList 2>&1 | tail -8'`
Expected: FAIL (`undefined: JSONList`).

- [ ] **Step 3: Írd meg a `JSONList`-et**

```go
// backend/internal/models/json_list.go
package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONList stores a slice in a Postgres jsonb column. A nil list is written
// as [] and read back as an empty, non-nil list, and always serialises as
// [] (never null), so API consumers can iterate without a nil check.
type JSONList[T any] []T

// Value implements driver.Valuer. The JSON is handed over as text, which the
// jsonb column type accepts.
func (l JSONList[T]) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]T(l))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner.
func (l *JSONList[T]) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case nil:
		*l = JSONList[T]{}
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("JSONList: unsupported scan type %T", src)
	}
	out := []T{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Errorf("JSONList: %w", err)
	}
	*l = out
	return nil
}

// MarshalJSON implements json.Marshaler. Marshalling the underlying slice
// type avoids recursing into this method.
func (l JSONList[T]) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]T(l))
}
```

- [ ] **Step 4: Írd meg a modelleket**

```go
// backend/internal/models/profit_scenario.go
package models

import "time"

// ProfitPercentItem is a percentage line of a parameter set. Base is
// "revenue" (percent of revenue) or "after_costs" (percent of the result
// after fixed costs and revenue-based items). No rates are built in; the
// user supplies every value.
type ProfitPercentItem struct {
	Label   string  `json:"label"`
	Percent float64 `json:"percent"`
	Base    string  `json:"base"`
}

type ProfitFixedCost struct {
	Label  string  `json:"label"`
	Amount float64 `json:"amount"`
}

type ProfitParameterSet struct {
	ID                uint                        `json:"id" gorm:"primaryKey"`
	Name              string                      `json:"name" gorm:"not null"`
	PercentItems      JSONList[ProfitPercentItem] `json:"percent_items" gorm:"type:jsonb;not null"`
	FixedMonthlyCosts JSONList[ProfitFixedCost]   `json:"fixed_monthly_costs" gorm:"type:jsonb;not null"`
	CreatedBy         uint                        `json:"created_by" gorm:"not null"`
	CreatedAt         time.Time                   `json:"created_at"`
	UpdatedAt         time.Time                   `json:"updated_at"`
}

func (ProfitParameterSet) TableName() string { return "profit_parameter_sets" }

type ProfitParameterSetRequest struct {
	Name              string              `json:"name"`
	PercentItems      []ProfitPercentItem `json:"percent_items"`
	FixedMonthlyCosts []ProfitFixedCost   `json:"fixed_monthly_costs"`
}

// ProfitScenarioAdjustment changes one existing client. At most one of
// NewHourlyRate / NewFixedPrice may be set; Drops removes the client.
type ProfitScenarioAdjustment struct {
	ClientID      uint     `json:"client_id"`
	NewHourlyRate *float64 `json:"new_hourly_rate"`
	NewFixedPrice *float64 `json:"new_fixed_price"`
	HoursDelta    float64  `json:"hours_delta"`
	Drops         bool     `json:"drops"`
}

// ProfitScenarioNewClient is a hypothetical client; MonthlyHours is total hours.
type ProfitScenarioNewClient struct {
	Name           string  `json:"name"`
	MonthlyRevenue float64 `json:"monthly_revenue"`
	MonthlyHours   float64 `json:"monthly_hours"`
}

type ProfitScenario struct {
	ID                    uint                                `json:"id" gorm:"primaryKey"`
	Name                  string                              `json:"name" gorm:"not null"`
	HorizonMonths         int                                 `json:"horizon_months" gorm:"not null"`
	ParameterSetID        uint                                `json:"parameter_set_id" gorm:"not null;index"`
	CapacityHoursPerMonth float64                             `json:"capacity_hours_per_month" gorm:"not null"`
	ClientAdjustments     JSONList[ProfitScenarioAdjustment] `json:"client_adjustments" gorm:"type:jsonb;not null"`
	NewClients            JSONList[ProfitScenarioNewClient]  `json:"new_clients" gorm:"type:jsonb;not null"`
	CreatedBy             uint                                `json:"created_by" gorm:"not null"`
	CreatedAt             time.Time                           `json:"created_at"`
	UpdatedAt             time.Time                           `json:"updated_at"`
}

func (ProfitScenario) TableName() string { return "profit_scenarios" }

type ProfitScenarioRequest struct {
	Name                  string                     `json:"name"`
	HorizonMonths         int                        `json:"horizon_months"`
	ParameterSetID        uint                       `json:"parameter_set_id"`
	CapacityHoursPerMonth float64                    `json:"capacity_hours_per_month"`
	ClientAdjustments     []ProfitScenarioAdjustment `json:"client_adjustments"`
	NewClients            []ProfitScenarioNewClient  `json:"new_clients"`
}
```

- [ ] **Step 5: gofmt, vet, teszt**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -w internal/models/json_list.go internal/models/json_list_test.go internal/models/profit_scenario.go; gofmt -l internal/models/json_list*.go internal/models/profit_scenario.go; go vet ./internal/models/ && go test ./internal/models/ -run JSONList -v 2>&1 | grep -E "^(--- |PASS|FAIL|ok)"'`
Expected: `gofmt -l` üres; minden teszt `--- PASS`, `ok`.

- [ ] **Step 6: Commit** (csak kérésre)

```bash
git add backend/internal/models/json_list.go backend/internal/models/json_list_test.go backend/internal/models/profit_scenario.go
git commit -m "feat(profitability): add JSONList and scenario models"
```

---

### Task 3: A forgatókönyv-motor (TDD)

**Files:**
- Create: `backend/internal/services/profitability_scenario.go`
- Test: `backend/internal/services/profitability_scenario_test.go`

**Interfaces:**
- Consumes: `models.ProfitParameterSet`, `models.ProfitScenarioAdjustment`, `models.ProfitScenarioNewClient` (Task 2). A tesztek használják a `profitability_calc_test.go`-ban definiált `approx(a, b float64) bool` segédet — **ne definiáld újra**.
- Produces:
  - `const PercentBaseRevenue = "revenue"`, `const PercentBaseAfterCosts = "after_costs"`
  - `ScenarioClientBase`, `ScenarioBase`, `ScenarioInput`, `ScenarioItemAmount`, `ScenarioMonth`, `ScenarioClientResult`, `ScenarioTotals`, `ScenarioBasis`, `ScenarioResult`
  - `func BuildScenario(base ScenarioBase, in ScenarioInput) ScenarioResult`

- [ ] **Step 1: Írd meg a bukó teszteket**

```go
// backend/internal/services/profitability_scenario_test.go
package services

import (
	"testing"

	"dev-bridge-manager/internal/models"
)

func f64(v float64) *float64 { return &v }

// Two clients: c1 300000/month, 20 logged + 4 overhead hours; c2 100000/month,
// 10 logged + 2 overhead hours. Baseline: revenue 400000, hours 36.
func scBase() ScenarioBase {
	return ScenarioBase{
		BaselineMonths: []string{"2026-07", "2026-08", "2026-09"},
		Clients: []ScenarioClientBase{
			{ClientID: 1, Name: "PIXEL", MonthlyRevenue: 300000, LoggedHours: 20, OverheadHours: 4},
			{ClientID: 2, Name: "ACME", MonthlyRevenue: 100000, LoggedHours: 10, OverheadHours: 2},
		},
	}
}

// 20000 fixed cost, 10% of revenue, 15% of the result after costs.
func scParams() models.ProfitParameterSet {
	return models.ProfitParameterSet{
		Name:              "Alap",
		FixedMonthlyCosts: models.JSONList[models.ProfitFixedCost]{{Label: "Eszközök", Amount: 20000}},
		PercentItems: models.JSONList[models.ProfitPercentItem]{
			{Label: "Járulék", Percent: 10, Base: PercentBaseRevenue},
			{Label: "Adó", Percent: 15, Base: PercentBaseAfterCosts},
		},
	}
}

func scInput(adj ...models.ProfitScenarioAdjustment) ScenarioInput {
	return ScenarioInput{
		Months:        []string{"2026-10", "2026-11", "2026-12"},
		CapacityHours: 60,
		Parameters:    scParams(),
		Adjustments:   adj,
	}
}

func TestBuildScenarioBaselineNumbers(t *testing.T) {
	m := BuildScenario(scBase(), scInput()).Baseline
	if !approx(m.Revenue, 400000) || !approx(m.FixedCosts, 20000) || !approx(m.ResultAfterCosts, 340000) || !approx(m.NetProfit, 289000) {
		t.Fatalf("baseline = %+v", m)
	}
	if len(m.RevenueItems) != 1 || !approx(m.RevenueItems[0].Amount, 40000) {
		t.Fatalf("revenue items = %+v", m.RevenueItems)
	}
	if len(m.AfterCostsItems) != 1 || !approx(m.AfterCostsItems[0].Amount, 51000) {
		t.Fatalf("after-costs items = %+v", m.AfterCostsItems)
	}
	if !approx(m.RequiredHours, 36) || m.Utilization == nil || !approx(*m.Utilization, 60) || m.Overloaded {
		t.Fatalf("hours/utilization = %v %v %v", m.RequiredHours, m.Utilization, m.Overloaded)
	}
	if m.NetProfitPerCapacityHour == nil || !approx(*m.NetProfitPerCapacityHour, 289000.0/60.0) {
		t.Fatalf("per capacity hour = %v", m.NetProfitPerCapacityHour)
	}
}

func TestBuildScenarioWithoutChangesEqualsBaseline(t *testing.T) {
	out := BuildScenario(scBase(), scInput())
	if !approx(out.Monthly.NetProfit, out.Baseline.NetProfit) || !approx(out.Monthly.Revenue, out.Baseline.Revenue) {
		t.Fatalf("monthly %+v != baseline %+v", out.Monthly, out.Baseline)
	}
}

func TestBuildScenarioNewHourlyRate(t *testing.T) {
	out := BuildScenario(scBase(), scInput(models.ProfitScenarioAdjustment{ClientID: 1, NewHourlyRate: f64(18000)}))
	m := out.Monthly
	// c1: 18000 x 20 = 360000; total 460000; 46000 + 59100 on top of 20000 fixed.
	if !approx(m.Revenue, 460000) || !approx(m.ResultAfterCosts, 394000) || !approx(m.NetProfit, 334900) || !approx(m.RequiredHours, 36) {
		t.Fatalf("monthly = %+v", m)
	}
}

func TestBuildScenarioDropClient(t *testing.T) {
	out := BuildScenario(scBase(), scInput(models.ProfitScenarioAdjustment{ClientID: 2, Drops: true}))
	m := out.Monthly
	if !approx(m.Revenue, 300000) || !approx(m.NetProfit, 212500) || !approx(m.RequiredHours, 24) {
		t.Fatalf("monthly = %+v", m)
	}
	if !out.Clients[1].Dropped || !approx(out.Clients[1].Revenue, 0) || !approx(out.Clients[1].Hours, 0) {
		t.Fatalf("client row = %+v", out.Clients[1])
	}
}

func TestBuildScenarioHoursDeltaScalesRevenueAtCurrentRate(t *testing.T) {
	out := BuildScenario(scBase(), scInput(models.ProfitScenarioAdjustment{ClientID: 1, HoursDelta: 10}))
	m := out.Monthly
	// c1 logged 20 -> 30 at 15000/h = 450000; hours 30+4 = 34 (+12 for c2).
	if !approx(m.Revenue, 550000) || !approx(m.NetProfit, 403750) || !approx(m.RequiredHours, 46) {
		t.Fatalf("monthly = %+v", m)
	}
}

func TestBuildScenarioNewFixedPrice(t *testing.T) {
	out := BuildScenario(scBase(), scInput(models.ProfitScenarioAdjustment{ClientID: 2, NewFixedPrice: f64(150000)}))
	m := out.Monthly
	if !approx(m.Revenue, 450000) || !approx(m.RequiredHours, 36) {
		t.Fatalf("monthly = %+v", m)
	}
}

func TestBuildScenarioNegativeLoggedHoursClampToZero(t *testing.T) {
	out := BuildScenario(scBase(), scInput(models.ProfitScenarioAdjustment{ClientID: 1, NewHourlyRate: f64(18000), HoursDelta: -30}))
	row := out.Clients[0]
	if !approx(row.Revenue, 0) || !approx(row.Hours, 4) {
		t.Fatalf("client row = %+v", row)
	}
}

func TestBuildScenarioNewClientIsMarkedAndOverloadsCapacity(t *testing.T) {
	in := scInput()
	in.CapacityHours = 50
	in.NewClients = []models.ProfitScenarioNewClient{{Name: "Új Kft", MonthlyRevenue: 100000, MonthlyHours: 20}}
	out := BuildScenario(scBase(), in)
	if !approx(out.Monthly.Revenue, 500000) || !approx(out.Monthly.RequiredHours, 56) || !out.Monthly.Overloaded {
		t.Fatalf("monthly = %+v", out.Monthly)
	}
	last := out.Clients[len(out.Clients)-1]
	if !last.IsNew || last.Name != "Új Kft" || last.ClientID != 0 {
		t.Fatalf("new client row = %+v", last)
	}
	if out.Baseline.Overloaded {
		t.Fatalf("baseline (36h of 50) must not be overloaded: %+v", out.Baseline)
	}
}

func TestBuildScenarioNegativeResultHasNoAfterCostsAmount(t *testing.T) {
	base := ScenarioBase{Clients: []ScenarioClientBase{{ClientID: 1, Name: "A", MonthlyRevenue: 10000, LoggedHours: 10}}}
	in := scInput()
	in.CapacityHours = 100
	out := BuildScenario(base, in)
	m := out.Monthly
	// 10000 - 20000 fixed - 1000 (10%) = -11000; the 15% item must not go negative.
	if !approx(m.ResultAfterCosts, -11000) || !approx(m.AfterCostsItems[0].Amount, 0) || !approx(m.NetProfit, -11000) {
		t.Fatalf("monthly = %+v", m)
	}
}

func TestBuildScenarioEmptyParametersNetEqualsRevenue(t *testing.T) {
	in := scInput()
	in.Parameters = models.ProfitParameterSet{Name: "Üres"}
	m := BuildScenario(scBase(), in).Monthly
	if !approx(m.NetProfit, m.Revenue) || len(m.RevenueItems) != 0 || len(m.AfterCostsItems) != 0 {
		t.Fatalf("monthly = %+v", m)
	}
}

func TestBuildScenarioZeroCapacityHasNoPerHourMetrics(t *testing.T) {
	in := scInput()
	in.CapacityHours = 0
	m := BuildScenario(scBase(), in).Monthly
	if m.Utilization != nil || m.NetProfitPerCapacityHour != nil || m.Overloaded {
		t.Fatalf("monthly = %+v", m)
	}
}

func TestBuildScenarioUnknownClientAdjustmentIsSkippedWithWarning(t *testing.T) {
	out := BuildScenario(scBase(), scInput(models.ProfitScenarioAdjustment{ClientID: 99, Drops: true}))
	if !approx(out.Monthly.NetProfit, out.Baseline.NetProfit) {
		t.Fatalf("unknown client changed the result: %+v", out.Monthly)
	}
	if len(out.Warnings) != 1 {
		t.Fatalf("warnings = %v", out.Warnings)
	}
}

func TestBuildScenarioHorizonTotalsMultiplyByMonths(t *testing.T) {
	out := BuildScenario(scBase(), scInput())
	if !approx(out.Horizon.NetProfit, 289000*3) || !approx(out.Horizon.Revenue, 400000*3) || !approx(out.BaselineHorizon.NetProfit, 289000*3) {
		t.Fatalf("horizon = %+v baseline = %+v", out.Horizon, out.BaselineHorizon)
	}
}

func TestBuildScenarioBasisReportsWhatWasUsed(t *testing.T) {
	out := BuildScenario(scBase(), scInput())
	b := out.Basis
	if b.ParameterSetName != "Alap" || len(b.PercentItems) != 2 || len(b.FixedMonthlyCosts) != 1 || !approx(b.CapacityHours, 60) || b.HorizonMonths != 3 || len(b.BaselineMonths) != 3 {
		t.Fatalf("basis = %+v", b)
	}
}

func TestBuildScenarioPassesThroughBaseWarningsAndLowData(t *testing.T) {
	base := scBase()
	base.LowData = true
	base.Warnings = []string{"kevés adat"}
	out := BuildScenario(base, scInput())
	if !out.LowData || len(out.Warnings) != 1 || out.Warnings[0] != "kevés adat" {
		t.Fatalf("lowData=%v warnings=%v", out.LowData, out.Warnings)
	}
}

func TestBuildScenarioEmptyBaseHasNonNilSlices(t *testing.T) {
	out := BuildScenario(ScenarioBase{}, ScenarioInput{Months: []string{"2026-10"}})
	if out.Clients == nil || out.Warnings == nil || out.Monthly.RevenueItems == nil || out.Monthly.AfterCostsItems == nil || out.Basis.PercentItems == nil || out.Basis.FixedMonthlyCosts == nil || out.Basis.BaselineMonths == nil {
		t.Fatalf("nil slice in %+v", out)
	}
}
```

- [ ] **Step 2: Futtasd, és ellenőrizd, hogy elbukik**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/services/ -run BuildScenario 2>&1 | tail -8'`
Expected: FAIL (`undefined: BuildScenario`, `undefined: ScenarioBase` …).

- [ ] **Step 3: Írd meg a motort**

```go
// backend/internal/services/profitability_scenario.go
package services

import (
	"fmt"
	"math"

	"dev-bridge-manager/internal/models"
)

const (
	PercentBaseRevenue    = "revenue"
	PercentBaseAfterCosts = "after_costs"
)

// ScenarioClientBase is one client's monthly run-rate: revenue from the
// forecast baseline, hours from the overview (logged hours, plus the
// estimated e-mail and meeting hours as overhead).
type ScenarioClientBase struct {
	ClientID       uint
	Name           string
	MonthlyRevenue float64
	LoggedHours    float64
	OverheadHours  float64
}

type ScenarioBase struct {
	BaselineMonths []string
	Clients        []ScenarioClientBase
	LowData        bool
	Warnings       []string
}

type ScenarioInput struct {
	Months        []string // horizon labels; the monthly result is flat across them
	CapacityHours float64
	Parameters    models.ProfitParameterSet
	Adjustments   []models.ProfitScenarioAdjustment
	NewClients    []models.ProfitScenarioNewClient
}

type ScenarioItemAmount struct {
	Label   string  `json:"label"`
	Percent float64 `json:"percent"`
	Base    string  `json:"base"`
	Amount  float64 `json:"amount"`
}

type ScenarioMonth struct {
	Revenue                  float64              `json:"revenue"`
	FixedCosts               float64              `json:"fixed_costs"`
	RevenueItems             []ScenarioItemAmount `json:"revenue_items"`
	ResultAfterCosts         float64              `json:"result_after_costs"`
	AfterCostsItems          []ScenarioItemAmount `json:"after_costs_items"`
	NetProfit                float64              `json:"net_profit"`
	RequiredHours            float64              `json:"required_hours"`
	CapacityHours            float64              `json:"capacity_hours"`
	Utilization              *float64             `json:"utilization"`
	Overloaded               bool                 `json:"overloaded"`
	NetProfitPerCapacityHour *float64             `json:"net_profit_per_capacity_hour"`
}

type ScenarioClientResult struct {
	ClientID    uint    `json:"client_id"`
	Name        string  `json:"name"`
	IsNew       bool    `json:"is_new"`
	Dropped     bool    `json:"dropped"`
	BaseRevenue float64 `json:"base_revenue"`
	BaseHours   float64 `json:"base_hours"`
	Revenue     float64 `json:"revenue"`
	Hours       float64 `json:"hours"`
}

type ScenarioTotals struct {
	Revenue   float64 `json:"revenue"`
	NetProfit float64 `json:"net_profit"`
}

// ScenarioBasis lists exactly what the numbers were computed with, so a
// result can be checked by hand.
type ScenarioBasis struct {
	ParameterSetName  string                     `json:"parameter_set_name"`
	PercentItems      []models.ProfitPercentItem `json:"percent_items"`
	FixedMonthlyCosts []models.ProfitFixedCost   `json:"fixed_monthly_costs"`
	CapacityHours     float64                    `json:"capacity_hours"`
	BaselineMonths    []string                   `json:"baseline_months"`
	HorizonMonths     int                        `json:"horizon_months"`
}

type ScenarioResult struct {
	Months          []string               `json:"months"`
	Monthly         ScenarioMonth          `json:"monthly"`
	Baseline        ScenarioMonth          `json:"baseline"`
	Horizon         ScenarioTotals         `json:"horizon"`
	BaselineHorizon ScenarioTotals         `json:"baseline_horizon"`
	Clients         []ScenarioClientResult `json:"clients"`
	LowData         bool                   `json:"low_data"`
	Warnings        []string               `json:"warnings"`
	Basis           ScenarioBasis          `json:"basis"`
}

// BuildScenario applies the scenario to the baseline run-rate and prices it
// with the parameter set. Clients are processed in base order so the float
// sums are deterministic.
func BuildScenario(base ScenarioBase, in ScenarioInput) ScenarioResult {
	adjustments := make(map[uint]models.ProfitScenarioAdjustment, len(in.Adjustments))
	for _, a := range in.Adjustments {
		adjustments[a.ClientID] = a
	}
	known := make(map[uint]bool, len(base.Clients))

	clients := make([]ScenarioClientResult, 0, len(base.Clients)+len(in.NewClients))
	var revenue, hours, baseRevenue, baseHours float64

	for _, c := range base.Clients {
		known[c.ClientID] = true
		bRevenue := c.MonthlyRevenue
		bHours := c.LoggedHours + c.OverheadHours
		baseRevenue += bRevenue
		baseHours += bHours

		row := ScenarioClientResult{
			ClientID: c.ClientID, Name: c.Name,
			BaseRevenue: bRevenue, BaseHours: bHours,
			Revenue: bRevenue, Hours: bHours,
		}
		if a, ok := adjustments[c.ClientID]; ok {
			if a.Drops {
				row.Revenue, row.Hours, row.Dropped = 0, 0, true
			} else {
				logged := math.Max(c.LoggedHours+a.HoursDelta, 0)
				switch {
				case a.NewHourlyRate != nil:
					row.Revenue = *a.NewHourlyRate * logged
				case a.NewFixedPrice != nil:
					row.Revenue = *a.NewFixedPrice
				case c.LoggedHours > 0:
					row.Revenue = c.MonthlyRevenue * logged / c.LoggedHours
				}
				row.Hours = logged + c.OverheadHours
			}
		}
		revenue += row.Revenue
		hours += row.Hours
		clients = append(clients, row)
	}

	warnings := append([]string{}, base.Warnings...)
	for _, a := range in.Adjustments {
		if !known[a.ClientID] {
			warnings = append(warnings, fmt.Sprintf(
				"A(z) #%d ügyfélre vonatkozó módosítás kimaradt: nincs rendszeres bevétele az alapidőszakban", a.ClientID))
		}
	}

	for _, n := range in.NewClients {
		clients = append(clients, ScenarioClientResult{
			Name: n.Name, IsNew: true, Revenue: n.MonthlyRevenue, Hours: n.MonthlyHours,
		})
		revenue += n.MonthlyRevenue
		hours += n.MonthlyHours
	}

	monthly := buildScenarioMonth(revenue, hours, in.Parameters, in.CapacityHours)
	baseline := buildScenarioMonth(baseRevenue, baseHours, in.Parameters, in.CapacityHours)

	horizon := float64(len(in.Months))
	months := in.Months
	if months == nil {
		months = []string{}
	}
	baselineMonths := base.BaselineMonths
	if baselineMonths == nil {
		baselineMonths = []string{}
	}
	percentItems := []models.ProfitPercentItem(in.Parameters.PercentItems)
	if percentItems == nil {
		percentItems = []models.ProfitPercentItem{}
	}
	fixedCosts := []models.ProfitFixedCost(in.Parameters.FixedMonthlyCosts)
	if fixedCosts == nil {
		fixedCosts = []models.ProfitFixedCost{}
	}

	return ScenarioResult{
		Months:          months,
		Monthly:         monthly,
		Baseline:        baseline,
		Horizon:         ScenarioTotals{Revenue: monthly.Revenue * horizon, NetProfit: monthly.NetProfit * horizon},
		BaselineHorizon: ScenarioTotals{Revenue: baseline.Revenue * horizon, NetProfit: baseline.NetProfit * horizon},
		Clients:         clients,
		LowData:         base.LowData,
		Warnings:        warnings,
		Basis: ScenarioBasis{
			ParameterSetName:  in.Parameters.Name,
			PercentItems:      percentItems,
			FixedMonthlyCosts: fixedCosts,
			CapacityHours:     in.CapacityHours,
			BaselineMonths:    baselineMonths,
			HorizonMonths:     len(in.Months),
		},
	}
}

// buildScenarioMonth prices one month: revenue-based items come off first,
// the remainder after fixed costs is the base of the after_costs items (which
// do not compound and never go negative), the rest is net profit.
func buildScenarioMonth(revenue, hours float64, p models.ProfitParameterSet, capacity float64) ScenarioMonth {
	m := ScenarioMonth{
		Revenue:         revenue,
		RequiredHours:   hours,
		CapacityHours:   capacity,
		RevenueItems:    []ScenarioItemAmount{},
		AfterCostsItems: []ScenarioItemAmount{},
	}
	for _, c := range p.FixedMonthlyCosts {
		m.FixedCosts += c.Amount
	}

	var revenueItemsTotal float64
	for _, it := range p.PercentItems {
		if it.Base != PercentBaseRevenue {
			continue
		}
		amount := revenue * it.Percent / 100
		revenueItemsTotal += amount
		m.RevenueItems = append(m.RevenueItems, ScenarioItemAmount{Label: it.Label, Percent: it.Percent, Base: it.Base, Amount: amount})
	}

	m.ResultAfterCosts = revenue - m.FixedCosts - revenueItemsTotal

	afterBase := math.Max(m.ResultAfterCosts, 0)
	var afterTotal float64
	for _, it := range p.PercentItems {
		if it.Base != PercentBaseAfterCosts {
			continue
		}
		amount := afterBase * it.Percent / 100
		afterTotal += amount
		m.AfterCostsItems = append(m.AfterCostsItems, ScenarioItemAmount{Label: it.Label, Percent: it.Percent, Base: it.Base, Amount: amount})
	}

	m.NetProfit = m.ResultAfterCosts - afterTotal

	if capacity > 0 {
		utilization := hours / capacity * 100
		m.Utilization = &utilization
		m.Overloaded = hours > capacity+1e-9
		perHour := m.NetProfit / capacity
		m.NetProfitPerCapacityHour = &perHour
	}
	return m
}
```

- [ ] **Step 4: Futtasd, és ellenőrizd, hogy átmegy**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -w internal/services/profitability_scenario.go internal/services/profitability_scenario_test.go; gofmt -l internal/services/profitability_scenario*.go; go vet ./internal/services/ && go test ./internal/services/ -run BuildScenario -v 2>&1 | grep -E "^(--- |PASS|FAIL|ok)"'`
Expected: `gofmt -l` üres; minden teszt `--- PASS`, `ok`.

- [ ] **Step 5: Futtasd a csomag összes tesztjét (nem romlott-e el semmi)**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test -count=1 ./internal/services/ 2>&1 | tail -3'`
Expected: `ok`.

- [ ] **Step 6: Commit** (csak kérésre)

```bash
git add backend/internal/services/profitability_scenario.go backend/internal/services/profitability_scenario_test.go
git commit -m "feat(profitability): add pure scenario engine"
```

---

### Task 4: Alapadat-betöltő és számítás-összeállítás

**Files:**
- Create: `backend/internal/services/profitability_scenario_load.go`

**Interfaces:**
- Consumes: `LoadForecast`, `LoadOverview`, `forecastBaselineMonths`, `FutureMonths`, `RateRow`, `BuildScenario`, `ScenarioBase`, `ScenarioInput`, `ScenarioClientBase`, `ScenarioResult` (1–3. szakasz / Task 3).
- Produces: `func LoadScenarioBase() (ScenarioBase, error)` és `func ComputeScenario(in ScenarioInput, horizonMonths int) (ScenarioResult, error)`.

- [ ] **Step 1: Írd meg a betöltőt**

```go
// backend/internal/services/profitability_scenario_load.go
package services

import "time"

// LoadScenarioBase builds the baseline run-rate the scenarios modify. Revenue
// per client is the forecast's monthly average (recurring invoices only);
// hours come from the overview over the same last-3-complete-months window,
// averaged per month. Clients without recurring revenue are not part of the
// base (a fixed-price client can be modelled as a new client).
func LoadScenarioBase() (ScenarioBase, error) {
	forecast, err := LoadForecast(forecastBaselineMonths)
	if err != nil {
		return ScenarioBase{}, err
	}
	overview, err := LoadOverview(forecastBaselineMonths)
	if err != nil {
		return ScenarioBase{}, err
	}

	rows := make(map[uint]RateRow, len(overview.Clients))
	for _, r := range overview.Clients {
		rows[r.ID] = r
	}

	months := float64(forecastBaselineMonths)
	clients := make([]ScenarioClientBase, 0, len(forecast.Clients))
	for _, c := range forecast.Clients {
		row := rows[c.ClientID] // zero value when the client has no hours/mail in the window
		clients = append(clients, ScenarioClientBase{
			ClientID:       c.ClientID,
			Name:           c.Name,
			MonthlyRevenue: c.MonthlyAverage,
			LoggedHours:    row.LoggedHours / months,
			OverheadHours:  (row.EmailHours + row.MeetingHours) / months,
		})
	}

	return ScenarioBase{
		BaselineMonths: forecast.BaselineMonths,
		Clients:        clients,
		LowData:        forecast.LowData,
		Warnings:       forecast.Warnings,
	}, nil
}

// ComputeScenario loads the baseline and prices the scenario over
// horizonMonths months starting with the current one. Every call reloads the
// baseline (no cache).
func ComputeScenario(in ScenarioInput, horizonMonths int) (ScenarioResult, error) {
	base, err := LoadScenarioBase()
	if err != nil {
		return ScenarioResult{}, err
	}
	in.Months = FutureMonths(time.Now(), horizonMonths)
	return BuildScenario(base, in), nil
}
```

- [ ] **Step 2: gofmt, vet, build**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -w internal/services/profitability_scenario_load.go; gofmt -l internal/services/profitability_scenario_load.go; go vet ./internal/services/ && go build ./... && echo build-ok'`
Expected: `gofmt -l` üres, `build-ok`.

- [ ] **Step 3: Futtasd a valódi betöltőt a dev adatbázison egy ideiglenes, csak olvasó teszttel, majd töröld**

Run:
```bash
cat > backend/internal/services/zz_tmp_scenario_test.go <<'EOF'
package services

import (
	"testing"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
)

// TEMPORARY, read-only: runs the real loader against the dev DB. Deleted right after.
func TestTmpComputeScenarioAgainstDevDB(t *testing.T) {
	database.Connect()
	res, err := ComputeScenario(ScenarioInput{
		CapacityHours: 120,
		Parameters:    models.ProfitParameterSet{Name: "tmp"},
	}, 6)
	if err != nil {
		t.Fatalf("ComputeScenario: %v", err)
	}
	t.Logf("months=%v clients=%d lowData=%v warnings=%v baselineNet=%.0f", res.Months, len(res.Clients), res.LowData, res.Warnings, res.Baseline.NetProfit)
}
EOF
docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/services/ -run TestTmpComputeScenarioAgainstDevDB -v 2>&1 | grep -E "months=|FAIL|PASS|panic|error"'
rm -f backend/internal/services/zz_tmp_scenario_test.go; ls backend/internal/services | grep -c zz_tmp
```
Expected: `PASS`, egy `months=[…6 hónap, az aktuálissal kezdve…] clients=0 lowData=true …` sor (a dev adatbázisban nincs rendszeres számla), hiba nélkül; az utolsó sor `0` (az ideiglenes fájl törölve). Ez azt igazolja, hogy a két betöltő együtt hiba nélkül fut; a nem üres utat az egységtesztek fedik.

- [ ] **Step 4: Commit** (csak kérésre)

```bash
git add backend/internal/services/profitability_scenario_load.go
git commit -m "feat(profitability): add scenario base loader"
```

---

### Task 5: Paraméterkészlet-végpontok (TDD a validációra)

**Files:**
- Create: `backend/internal/handlers/profitability_scenario_handler.go`
- Test: `backend/internal/handlers/profitability_scenario_handler_test.go`
- Modify: `backend/internal/routes/profitability_routes.go`

**Interfaces:**
- Consumes: `models.ProfitParameterSet`, `models.ProfitParameterSetRequest`, `models.JSONList`, `models.ProfitScenario` (Task 2); `services.PercentBaseRevenue`, `services.PercentBaseAfterCosts` (Task 3); a stage 1 `ProfitabilityHandler` típusa, `currentUserID(c *fiber.Ctx) uint` (a `handlers` csomagban már létezik).
- Produces:
  - `func validateParameterSetRequest(req *models.ProfitParameterSetRequest) string`
  - `GET /profitability/parameter-sets`, `POST /profitability/parameter-sets`, `PUT /profitability/parameter-sets/:id`, `DELETE /profitability/parameter-sets/:id` (`409`, ha forgatókönyv hivatkozik rá)

- [ ] **Step 1: Írd meg a bukó teszteket**

```go
// backend/internal/handlers/profitability_scenario_handler_test.go
package handlers

import (
	"strings"
	"testing"

	"dev-bridge-manager/internal/models"
)

func validSet() models.ProfitParameterSetRequest {
	return models.ProfitParameterSetRequest{
		Name: "Alap",
		PercentItems: []models.ProfitPercentItem{
			{Label: "Járulék", Percent: 10, Base: "revenue"},
			{Label: "Adó", Percent: 15, Base: "after_costs"},
		},
		FixedMonthlyCosts: []models.ProfitFixedCost{{Label: "Eszközök", Amount: 20000}},
	}
}

func TestValidateParameterSetRequestAcceptsValid(t *testing.T) {
	req := validSet()
	if msg := validateParameterSetRequest(&req); msg != "" {
		t.Fatalf("valid set rejected: %s", msg)
	}
}

func TestValidateParameterSetRequestAcceptsEmptyLists(t *testing.T) {
	req := models.ProfitParameterSetRequest{Name: "Üres"}
	if msg := validateParameterSetRequest(&req); msg != "" {
		t.Fatalf("empty lists rejected: %s", msg)
	}
}

func TestValidateParameterSetRequestTrims(t *testing.T) {
	req := validSet()
	req.Name = "  Alap  "
	req.PercentItems[0].Label = "  Járulék "
	req.FixedMonthlyCosts[0].Label = " Eszközök  "
	if msg := validateParameterSetRequest(&req); msg != "" {
		t.Fatal(msg)
	}
	if req.Name != "Alap" || req.PercentItems[0].Label != "Járulék" || req.FixedMonthlyCosts[0].Label != "Eszközök" {
		t.Fatalf("not trimmed: %+v", req)
	}
}

func TestValidateParameterSetRequestRejects(t *testing.T) {
	cases := map[string]func(r *models.ProfitParameterSetRequest){
		"blank name":         func(r *models.ProfitParameterSetRequest) { r.Name = "   " },
		"name too long":      func(r *models.ProfitParameterSetRequest) { r.Name = strings.Repeat("a", 256) },
		"blank percent label": func(r *models.ProfitParameterSetRequest) { r.PercentItems[0].Label = " " },
		"label too long":     func(r *models.ProfitParameterSetRequest) { r.PercentItems[0].Label = strings.Repeat("a", 101) },
		"negative percent":   func(r *models.ProfitParameterSetRequest) { r.PercentItems[0].Percent = -1 },
		"percent above 100":  func(r *models.ProfitParameterSetRequest) { r.PercentItems[0].Percent = 100.5 },
		"unknown base":       func(r *models.ProfitParameterSetRequest) { r.PercentItems[0].Base = "profit" },
		"blank cost label":   func(r *models.ProfitParameterSetRequest) { r.FixedMonthlyCosts[0].Label = "" },
		"negative cost":      func(r *models.ProfitParameterSetRequest) { r.FixedMonthlyCosts[0].Amount = -1 },
		"huge cost":          func(r *models.ProfitParameterSetRequest) { r.FixedMonthlyCosts[0].Amount = 2_000_000_000 },
		"too many percents": func(r *models.ProfitParameterSetRequest) {
			r.PercentItems = make([]models.ProfitPercentItem, 21)
			for i := range r.PercentItems {
				r.PercentItems[i] = models.ProfitPercentItem{Label: "x", Base: "revenue"}
			}
		},
		"too many costs": func(r *models.ProfitParameterSetRequest) {
			r.FixedMonthlyCosts = make([]models.ProfitFixedCost, 21)
			for i := range r.FixedMonthlyCosts {
				r.FixedMonthlyCosts[i] = models.ProfitFixedCost{Label: "x"}
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := validSet()
			mutate(&req)
			if msg := validateParameterSetRequest(&req); msg == "" {
				t.Fatalf("accepted: %+v", req)
			}
		})
	}
}

func TestValidateParameterSetRequestAcceptsBoundaries(t *testing.T) {
	req := validSet()
	req.PercentItems[0].Percent = 0
	req.PercentItems[1].Percent = 100
	req.FixedMonthlyCosts[0].Amount = 1_000_000_000
	if msg := validateParameterSetRequest(&req); msg != "" {
		t.Fatalf("boundaries rejected: %s", msg)
	}
}
```

- [ ] **Step 2: Futtasd, és ellenőrizd, hogy elbukik**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/handlers/ -run ValidateParameterSet 2>&1 | tail -6'`
Expected: FAIL (`undefined: validateParameterSetRequest`).

- [ ] **Step 3: Írd meg a handlert**

```go
// backend/internal/handlers/profitability_scenario_handler.go
package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"

	"github.com/gofiber/fiber/v2"
)

const maxScenarioMoney = 1_000_000_000.0

// validateParameterSetRequest trims the request in place and returns a
// user-facing message for the first problem found, or "" when it is valid.
func validateParameterSetRequest(req *models.ProfitParameterSetRequest) string {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return "Name is required"
	}
	if utf8.RuneCountInString(req.Name) > 255 {
		return "Name is too long"
	}
	if len(req.PercentItems) > 20 || len(req.FixedMonthlyCosts) > 20 {
		return "At most 20 items per list"
	}
	for i := range req.PercentItems {
		it := &req.PercentItems[i]
		it.Label = strings.TrimSpace(it.Label)
		if it.Label == "" || utf8.RuneCountInString(it.Label) > 100 {
			return "Each percent item needs a label (max 100 characters)"
		}
		if it.Percent < 0 || it.Percent > 100 {
			return "Percent must be between 0 and 100"
		}
		if it.Base != services.PercentBaseRevenue && it.Base != services.PercentBaseAfterCosts {
			return "Base must be revenue or after_costs"
		}
	}
	for i := range req.FixedMonthlyCosts {
		c := &req.FixedMonthlyCosts[i]
		c.Label = strings.TrimSpace(c.Label)
		if c.Label == "" || utf8.RuneCountInString(c.Label) > 100 {
			return "Each fixed cost needs a label (max 100 characters)"
		}
		if c.Amount < 0 || c.Amount > maxScenarioMoney {
			return "Fixed cost must be between 0 and 1000000000"
		}
	}
	return ""
}

// GetParameterSets - GET /api/v1/profitability/parameter-sets
func (h *ProfitabilityHandler) GetParameterSets(c *fiber.Ctx) error {
	var sets []models.ProfitParameterSet
	if err := database.GetDB().Order("name ASC, id ASC").Find(&sets).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading parameter sets"})
	}
	return c.JSON(sets)
}

// CreateParameterSet - POST /api/v1/profitability/parameter-sets
func (h *ProfitabilityHandler) CreateParameterSet(c *fiber.Ctx) error {
	var req models.ProfitParameterSetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateParameterSetRequest(&req); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}

	set := models.ProfitParameterSet{
		Name:              req.Name,
		PercentItems:      models.JSONList[models.ProfitPercentItem](req.PercentItems),
		FixedMonthlyCosts: models.JSONList[models.ProfitFixedCost](req.FixedMonthlyCosts),
		CreatedBy:         currentUserID(c),
	}
	if err := database.GetDB().Create(&set).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating parameter set"})
	}
	return c.Status(201).JSON(set)
}

// UpdateParameterSet - PUT /api/v1/profitability/parameter-sets/:id
func (h *ProfitabilityHandler) UpdateParameterSet(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid parameter set ID"})
	}
	var set models.ProfitParameterSet
	if err := database.GetDB().First(&set, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Parameter set not found"})
	}

	var req models.ProfitParameterSetRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateParameterSetRequest(&req); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}

	set.Name = req.Name
	set.PercentItems = models.JSONList[models.ProfitPercentItem](req.PercentItems)
	set.FixedMonthlyCosts = models.JSONList[models.ProfitFixedCost](req.FixedMonthlyCosts)
	if err := database.GetDB().Save(&set).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating parameter set"})
	}
	return c.JSON(set)
}

// DeleteParameterSet - DELETE /api/v1/profitability/parameter-sets/:id
// Refuses with 409 while scenarios still reference the set (the database
// foreign key is ON DELETE RESTRICT as well).
func (h *ProfitabilityHandler) DeleteParameterSet(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid parameter set ID"})
	}

	var used int64
	if err := database.GetDB().Model(&models.ProfitScenario{}).Where("parameter_set_id = ?", id).Count(&used).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting parameter set"})
	}
	if used > 0 {
		return c.Status(409).JSON(fiber.Map{"success": false, "message": fmt.Sprintf("Parameter set is used by %d scenario(s)", used)})
	}

	result := database.GetDB().Delete(&models.ProfitParameterSet{}, id)
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting parameter set"})
	}
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Parameter set not found"})
	}
	return c.JSON(fiber.Map{"success": true, "message": "Parameter set deleted successfully"})
}
```

- [ ] **Step 4: Add hozzá az útvonalakat**

A `backend/internal/routes/profitability_routes.go`-ban a `/forecast` útvonal **után** (ugyanabban a csoportban, így a JWT és a `profitability.read` már előttük van):

```go
	// GET /api/v1/profitability/parameter-sets - Mentett költség-paraméterkészletek
	g.Get("/parameter-sets", h.GetParameterSets)

	// POST /api/v1/profitability/parameter-sets - Új paraméterkészlet
	g.Post("/parameter-sets", middleware.RequirePermission("profitability.manage"), h.CreateParameterSet)

	// PUT /api/v1/profitability/parameter-sets/:id - Paraméterkészlet módosítása
	g.Put("/parameter-sets/:id", middleware.RequirePermission("profitability.manage"), h.UpdateParameterSet)

	// DELETE /api/v1/profitability/parameter-sets/:id - Törlés (409, ha forgatókönyv használja)
	g.Delete("/parameter-sets/:id", middleware.RequirePermission("profitability.manage"), h.DeleteParameterSet)
```

- [ ] **Step 5: Teszt, gofmt, vet, védettség**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -w internal/handlers/profitability_scenario_handler.go internal/handlers/profitability_scenario_handler_test.go; gofmt -l internal/handlers/profitability_scenario_handler*.go internal/routes/profitability_routes.go; go vet ./internal/handlers/ ./internal/routes/ && go test -count=1 ./internal/handlers/ 2>&1 | tail -4'`
Expected: `gofmt -l` üres, `ok`.

Majd (a hot reload után ~15 mp; a **migráció még nincs alkalmazva**, de a védettség ettől független):
Run: `sleep 15; for m in GET POST; do curl -s -o /dev/null -X $m -w "$m /profitability/parameter-sets token nélkül: HTTP %{http_code}\n" http://localhost:8080/api/v1/profitability/parameter-sets; done`
Expected: mindkettő `HTTP 401`.

- [ ] **Step 6: Commit** (csak kérésre)

```bash
git add backend/internal/handlers/profitability_scenario_handler.go backend/internal/handlers/profitability_scenario_handler_test.go backend/internal/routes/profitability_routes.go
git commit -m "feat(profitability): add parameter set endpoints"
```

---

### Task 6: Forgatókönyv-végpontok és `compute` (TDD a validációra), a migráció alkalmazása

**Files:**
- Modify: `backend/internal/handlers/profitability_scenario_handler.go` (új függvények a fájl végére; az import-blokk már tartalmaz mindent, kivéve ha a fordító mást kér)
- Modify: `backend/internal/handlers/profitability_scenario_handler_test.go` (új tesztek a fájl végére)
- Modify: `backend/internal/routes/profitability_routes.go`

**Interfaces:**
- Consumes: `services.ComputeScenario`, `services.ScenarioInput` (Task 3–4); `models.ProfitScenario`, `models.ProfitScenarioRequest`, `models.JSONList` (Task 2).
- Produces:
  - `func validateScenarioRequest(req *models.ProfitScenarioRequest, requireName bool) string`
  - `GET /profitability/scenarios`, `POST /profitability/scenarios`, `PUT /profitability/scenarios/:id`, `DELETE /profitability/scenarios/:id`, `POST /profitability/scenarios/compute`

- [ ] **Step 1: Írd meg a bukó teszteket (a teszt-fájl végére fűzd)**

```go
func validScenario() models.ProfitScenarioRequest {
	rate := 18000.0
	return models.ProfitScenarioRequest{
		Name: "Áremelés", HorizonMonths: 6, ParameterSetID: 1, CapacityHoursPerMonth: 120,
		ClientAdjustments: []models.ProfitScenarioAdjustment{
			{ClientID: 1, NewHourlyRate: &rate, HoursDelta: 5},
			{ClientID: 2, Drops: true},
		},
		NewClients: []models.ProfitScenarioNewClient{{Name: "Új Kft", MonthlyRevenue: 100000, MonthlyHours: 20}},
	}
}

func TestValidateScenarioRequestAcceptsValid(t *testing.T) {
	req := validScenario()
	if msg := validateScenarioRequest(&req, true); msg != "" {
		t.Fatalf("valid scenario rejected: %s", msg)
	}
}

func TestValidateScenarioRequestNameOnlyRequiredWhenAsked(t *testing.T) {
	req := validScenario()
	req.Name = ""
	if msg := validateScenarioRequest(&req, true); msg == "" {
		t.Fatal("blank name accepted for save")
	}
	if msg := validateScenarioRequest(&req, false); msg != "" {
		t.Fatalf("blank name rejected for compute: %s", msg)
	}
}

func TestValidateScenarioRequestTrims(t *testing.T) {
	req := validScenario()
	req.Name = "  Áremelés  "
	req.NewClients[0].Name = " Új Kft "
	if msg := validateScenarioRequest(&req, true); msg != "" {
		t.Fatal(msg)
	}
	if req.Name != "Áremelés" || req.NewClients[0].Name != "Új Kft" {
		t.Fatalf("not trimmed: %+v", req)
	}
}

func TestValidateScenarioRequestRejects(t *testing.T) {
	big := 2_000_000_000.0
	neg := -1.0
	rate := 100.0
	cases := map[string]func(r *models.ProfitScenarioRequest){
		"name too long":        func(r *models.ProfitScenarioRequest) { r.Name = strings.Repeat("a", 256) },
		"horizon too short":    func(r *models.ProfitScenarioRequest) { r.HorizonMonths = 2 },
		"horizon too long":     func(r *models.ProfitScenarioRequest) { r.HorizonMonths = 7 },
		"no parameter set":     func(r *models.ProfitScenarioRequest) { r.ParameterSetID = 0 },
		"zero capacity":        func(r *models.ProfitScenarioRequest) { r.CapacityHoursPerMonth = 0 },
		"negative capacity":    func(r *models.ProfitScenarioRequest) { r.CapacityHoursPerMonth = -5 },
		"capacity above month": func(r *models.ProfitScenarioRequest) { r.CapacityHoursPerMonth = 745 },
		"adjustment no client": func(r *models.ProfitScenarioRequest) { r.ClientAdjustments[0].ClientID = 0 },
		"duplicate client":     func(r *models.ProfitScenarioRequest) { r.ClientAdjustments[1].ClientID = r.ClientAdjustments[0].ClientID },
		"rate and fixed both": func(r *models.ProfitScenarioRequest) {
			r.ClientAdjustments[0].NewFixedPrice = &rate
		},
		"negative rate":      func(r *models.ProfitScenarioRequest) { r.ClientAdjustments[0].NewHourlyRate = &neg },
		"huge fixed price": func(r *models.ProfitScenarioRequest) {
			r.ClientAdjustments[0].NewHourlyRate = nil
			r.ClientAdjustments[0].NewFixedPrice = &big
		},
		"hours delta too low":  func(r *models.ProfitScenarioRequest) { r.ClientAdjustments[0].HoursDelta = -745 },
		"hours delta too high": func(r *models.ProfitScenarioRequest) { r.ClientAdjustments[0].HoursDelta = 745 },
		"new client no name":   func(r *models.ProfitScenarioRequest) { r.NewClients[0].Name = "  " },
		"new client negative revenue": func(r *models.ProfitScenarioRequest) { r.NewClients[0].MonthlyRevenue = -1 },
		"new client hours too high":   func(r *models.ProfitScenarioRequest) { r.NewClients[0].MonthlyHours = 745 },
		"too many adjustments": func(r *models.ProfitScenarioRequest) {
			r.ClientAdjustments = make([]models.ProfitScenarioAdjustment, 201)
			for i := range r.ClientAdjustments {
				r.ClientAdjustments[i] = models.ProfitScenarioAdjustment{ClientID: uint(i + 1)}
			}
		},
		"too many new clients": func(r *models.ProfitScenarioRequest) {
			r.NewClients = make([]models.ProfitScenarioNewClient, 51)
			for i := range r.NewClients {
				r.NewClients[i] = models.ProfitScenarioNewClient{Name: "x"}
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := validScenario()
			mutate(&req)
			if msg := validateScenarioRequest(&req, true); msg == "" {
				t.Fatalf("accepted: %+v", req)
			}
		})
	}
}

func TestValidateScenarioRequestAcceptsBoundaries(t *testing.T) {
	req := validScenario()
	req.HorizonMonths = 3
	req.CapacityHoursPerMonth = 744
	req.ClientAdjustments[0].HoursDelta = -744
	req.NewClients[0].MonthlyHours = 744
	if msg := validateScenarioRequest(&req, true); msg != "" {
		t.Fatalf("boundaries rejected: %s", msg)
	}
}
```

- [ ] **Step 2: Futtasd, és ellenőrizd, hogy elbukik**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/handlers/ -run ValidateScenarioRequest 2>&1 | tail -6'`
Expected: FAIL (`undefined: validateScenarioRequest`).

- [ ] **Step 3: Írd meg a handlereket (a handler-fájl végére fűzd)**

```go
// validateScenarioRequest trims the request in place and returns a
// user-facing message for the first problem found, or "" when it is valid.
// The name is only required when the scenario is saved, not for a live compute.
func validateScenarioRequest(req *models.ProfitScenarioRequest, requireName bool) string {
	req.Name = strings.TrimSpace(req.Name)
	if requireName && req.Name == "" {
		return "Name is required"
	}
	if utf8.RuneCountInString(req.Name) > 255 {
		return "Name is too long"
	}
	if req.HorizonMonths < 3 || req.HorizonMonths > 6 {
		return "Horizon must be between 3 and 6 months"
	}
	if req.ParameterSetID == 0 {
		return "Parameter set is required"
	}
	if req.CapacityHoursPerMonth <= 0 || req.CapacityHoursPerMonth > 744 {
		return "Capacity must be greater than 0 and at most 744 hours per month"
	}
	if len(req.ClientAdjustments) > 200 {
		return "At most 200 client adjustments"
	}
	if len(req.NewClients) > 50 {
		return "At most 50 new clients"
	}

	seen := make(map[uint]bool, len(req.ClientAdjustments))
	for _, a := range req.ClientAdjustments {
		if a.ClientID == 0 {
			return "Each adjustment needs a client"
		}
		if seen[a.ClientID] {
			return "A client can only be adjusted once"
		}
		seen[a.ClientID] = true
		if a.NewHourlyRate != nil && a.NewFixedPrice != nil {
			return "Set either a new hourly rate or a new fixed price, not both"
		}
		if a.NewHourlyRate != nil && (*a.NewHourlyRate < 0 || *a.NewHourlyRate > maxScenarioMoney) {
			return "Hourly rate must be between 0 and 1000000000"
		}
		if a.NewFixedPrice != nil && (*a.NewFixedPrice < 0 || *a.NewFixedPrice > maxScenarioMoney) {
			return "Fixed price must be between 0 and 1000000000"
		}
		if a.HoursDelta < -744 || a.HoursDelta > 744 {
			return "Hours change must be between -744 and 744"
		}
	}

	for i := range req.NewClients {
		n := &req.NewClients[i]
		n.Name = strings.TrimSpace(n.Name)
		if n.Name == "" || utf8.RuneCountInString(n.Name) > 255 {
			return "Each new client needs a name (max 255 characters)"
		}
		if n.MonthlyRevenue < 0 || n.MonthlyRevenue > maxScenarioMoney {
			return "New client revenue must be between 0 and 1000000000"
		}
		if n.MonthlyHours < 0 || n.MonthlyHours > 744 {
			return "New client hours must be between 0 and 744"
		}
	}
	return ""
}

// parameterSetExists reports whether the referenced parameter set is there.
func parameterSetExists(id uint) bool {
	var count int64
	database.GetDB().Model(&models.ProfitParameterSet{}).Where("id = ?", id).Count(&count)
	return count > 0
}

// GetScenarios - GET /api/v1/profitability/scenarios
func (h *ProfitabilityHandler) GetScenarios(c *fiber.Ctx) error {
	var scenarios []models.ProfitScenario
	if err := database.GetDB().Order("created_at DESC, id DESC").Find(&scenarios).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading scenarios"})
	}
	return c.JSON(scenarios)
}

// CreateScenario - POST /api/v1/profitability/scenarios
func (h *ProfitabilityHandler) CreateScenario(c *fiber.Ctx) error {
	var req models.ProfitScenarioRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateScenarioRequest(&req, true); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}
	if !parameterSetExists(req.ParameterSetID) {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Parameter set not found"})
	}

	scenario := models.ProfitScenario{
		Name:                  req.Name,
		HorizonMonths:         req.HorizonMonths,
		ParameterSetID:        req.ParameterSetID,
		CapacityHoursPerMonth: req.CapacityHoursPerMonth,
		ClientAdjustments:     models.JSONList[models.ProfitScenarioAdjustment](req.ClientAdjustments),
		NewClients:            models.JSONList[models.ProfitScenarioNewClient](req.NewClients),
		CreatedBy:             currentUserID(c),
	}
	if err := database.GetDB().Create(&scenario).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating scenario"})
	}
	return c.Status(201).JSON(scenario)
}

// UpdateScenario - PUT /api/v1/profitability/scenarios/:id
func (h *ProfitabilityHandler) UpdateScenario(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid scenario ID"})
	}
	var scenario models.ProfitScenario
	if err := database.GetDB().First(&scenario, id).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Scenario not found"})
	}

	var req models.ProfitScenarioRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateScenarioRequest(&req, true); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}
	if !parameterSetExists(req.ParameterSetID) {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Parameter set not found"})
	}

	scenario.Name = req.Name
	scenario.HorizonMonths = req.HorizonMonths
	scenario.ParameterSetID = req.ParameterSetID
	scenario.CapacityHoursPerMonth = req.CapacityHoursPerMonth
	scenario.ClientAdjustments = models.JSONList[models.ProfitScenarioAdjustment](req.ClientAdjustments)
	scenario.NewClients = models.JSONList[models.ProfitScenarioNewClient](req.NewClients)
	if err := database.GetDB().Save(&scenario).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating scenario"})
	}
	return c.JSON(scenario)
}

// DeleteScenario - DELETE /api/v1/profitability/scenarios/:id
func (h *ProfitabilityHandler) DeleteScenario(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid scenario ID"})
	}
	result := database.GetDB().Delete(&models.ProfitScenario{}, id)
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting scenario"})
	}
	if result.RowsAffected == 0 {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Scenario not found"})
	}
	return c.JSON(fiber.Map{"success": true, "message": "Scenario deleted successfully"})
}

// ComputeScenario - POST /api/v1/profitability/scenarios/compute
// Prices an unsaved scenario draft (nothing is written).
func (h *ProfitabilityHandler) ComputeScenario(c *fiber.Ctx) error {
	var req models.ProfitScenarioRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if msg := validateScenarioRequest(&req, false); msg != "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": msg})
	}

	var set models.ProfitParameterSet
	if err := database.GetDB().First(&set, req.ParameterSetID).Error; err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Parameter set not found"})
	}

	result, err := services.ComputeScenario(services.ScenarioInput{
		CapacityHours: req.CapacityHoursPerMonth,
		Parameters:    set,
		Adjustments:   req.ClientAdjustments,
		NewClients:    req.NewClients,
	}, req.HorizonMonths)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error computing scenario"})
	}
	return c.JSON(result)
}
```

- [ ] **Step 4: Add hozzá az útvonalakat**

A `backend/internal/routes/profitability_routes.go`-ban a `DELETE /parameter-sets/:id` útvonal **után**:

```go
	// GET /api/v1/profitability/scenarios - Mentett forgatókönyvek
	g.Get("/scenarios", h.GetScenarios)

	// POST /api/v1/profitability/scenarios/compute - Mentés nélküli számolás (élő eredmény)
	g.Post("/scenarios/compute", h.ComputeScenario)

	// POST /api/v1/profitability/scenarios - Új forgatókönyv
	g.Post("/scenarios", middleware.RequirePermission("profitability.manage"), h.CreateScenario)

	// PUT /api/v1/profitability/scenarios/:id - Forgatókönyv módosítása
	g.Put("/scenarios/:id", middleware.RequirePermission("profitability.manage"), h.UpdateScenario)

	// DELETE /api/v1/profitability/scenarios/:id - Forgatókönyv törlése
	g.Delete("/scenarios/:id", middleware.RequirePermission("profitability.manage"), h.DeleteScenario)
```

- [ ] **Step 5: Teszt, gofmt, vet, build**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -w internal/handlers/profitability_scenario_handler.go internal/handlers/profitability_scenario_handler_test.go; gofmt -l internal/handlers/profitability_scenario_handler*.go internal/routes/profitability_routes.go; go vet ./internal/handlers/ ./internal/routes/ && go build ./... && echo build-ok; go test -count=1 ./internal/handlers/ ./internal/services/ ./internal/models/ ./internal/middleware/ 2>&1 | tail -6'`
Expected: `gofmt -l` üres, `build-ok`, mind a négy csomag `ok`.

- [ ] **Step 6: A migráció alkalmazása és a védettség ellenőrzése**

A Go fájlok módosítása újraindította a backendet, ami induláskor lefuttatja a függő migrációkat. **Ne alkalmazd kézzel.**

Run: `sleep 20; docker compose -f .docker/docker-compose.yml exec -T postgres psql -U devbridge_user -d devbridge -tc "SELECT version, dirty FROM schema_migrations;" -tc "SELECT to_regclass('profit_scenarios'), to_regclass('profit_parameter_sets');"`
Expected: `45 | f`, mindkét tábla létezik. Ha még `44`: várj 20 mp-et és ismételd; ha tartósan 44 vagy `dirty = t`, **állj meg**, nézd meg `docker compose -f .docker/docker-compose.yml logs --tail 40 backend` kimenetét, és jelezd a hibát; ne javítsd kézzel az adatbázist.

Run: `for p in scenarios parameter-sets; do curl -s -o /dev/null -w "GET /profitability/$p token nélkül: HTTP %{http_code}\n" http://localhost:8080/api/v1/profitability/$p; done; curl -s -o /dev/null -X POST -w "POST /scenarios/compute token nélkül: HTTP %{http_code}\n" http://localhost:8080/api/v1/profitability/scenarios/compute`
Expected: mind a három `HTTP 401`.

- [ ] **Step 7: Igazold a JSONB oda-vissza utat és a külső kulcsot valódi GORM-mal, visszagörgetett tranzakcióban**

Run:
```bash
cat > backend/internal/handlers/zz_tmp_scenario_db_test.go <<'EOF'
package handlers

import (
	"testing"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
)

// TEMPORARY: GORM round trip of the JSONB lists inside a rolled-back transaction. Deleted right after.
func TestTmpScenarioJSONBRoundTrip(t *testing.T) {
	database.Connect()
	tx := database.GetDB().Begin()
	defer tx.Rollback()

	var uid uint
	if err := tx.Raw("SELECT id FROM users ORDER BY id LIMIT 1").Scan(&uid).Error; err != nil || uid == 0 {
		t.Fatalf("no user: %v", err)
	}

	rate := 18000.0
	set := models.ProfitParameterSet{
		Name:              "__tmp_set",
		PercentItems:      models.JSONList[models.ProfitPercentItem]{{Label: "Járulék", Percent: 10, Base: "revenue"}},
		FixedMonthlyCosts: models.JSONList[models.ProfitFixedCost]{{Label: "Eszközök", Amount: 20000}},
		CreatedBy:         uid,
	}
	if err := tx.Create(&set).Error; err != nil {
		t.Fatalf("create set: %v", err)
	}
	sc := models.ProfitScenario{
		Name: "__tmp_sc", HorizonMonths: 6, ParameterSetID: set.ID, CapacityHoursPerMonth: 120,
		ClientAdjustments: models.JSONList[models.ProfitScenarioAdjustment]{{ClientID: 1, NewHourlyRate: &rate, HoursDelta: 5}},
		CreatedBy:         uid,
	}
	if err := tx.Create(&sc).Error; err != nil {
		t.Fatalf("create scenario: %v", err)
	}

	var gotSet models.ProfitParameterSet
	var gotSc models.ProfitScenario
	if err := tx.First(&gotSet, set.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.First(&gotSc, sc.ID).Error; err != nil {
		t.Fatal(err)
	}
	if len(gotSet.PercentItems) != 1 || gotSet.PercentItems[0].Label != "Járulék" || gotSet.FixedMonthlyCosts[0].Amount != 20000 {
		t.Fatalf("set round trip: %+v", gotSet)
	}
	if len(gotSc.ClientAdjustments) != 1 || gotSc.ClientAdjustments[0].NewHourlyRate == nil || *gotSc.ClientAdjustments[0].NewHourlyRate != 18000 {
		t.Fatalf("scenario round trip: %+v", gotSc.ClientAdjustments)
	}
	if gotSc.NewClients == nil || len(gotSc.NewClients) != 0 {
		t.Fatalf("nil list must read back empty non-nil: %#v", gotSc.NewClients)
	}

	if err := tx.Delete(&models.ProfitParameterSet{}, set.ID).Error; err == nil {
		t.Fatal("deleting a referenced parameter set must fail (ON DELETE RESTRICT)")
	} else {
		t.Logf("restrict ok: %v", err)
	}
}
EOF
docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/handlers/ -run TestTmpScenarioJSONBRoundTrip -v 2>&1 | grep -E "restrict ok|FAIL|PASS|panic|Error"'
rm -f backend/internal/handlers/zz_tmp_scenario_db_test.go; ls backend/internal/handlers | grep -c zz_tmp
docker compose -f .docker/docker-compose.yml exec -T postgres psql -U devbridge_user -d devbridge -tc "SELECT (SELECT count(*) FROM profit_parameter_sets), (SELECT count(*) FROM profit_scenarios);"
```
Expected: `restrict ok: …` (FK-hiba), `PASS`; az `ls … | grep -c` kimenete `0` (az ideiglenes fájl törölve); a két darabszám `0 | 0` (a tranzakció visszagörgetve, nem maradt adat). Ha a `Create` a `jsonb` oszlopnál típushibát ad (pl. `cannot use … as jsonb`), **állj meg** és jelezd a pontos hibát; a javítás a `JSONList.Value` visszatérési típusa lenne, ne találgass.

- [ ] **Step 8: Commit** (csak kérésre)

```bash
git add backend/internal/handlers/profitability_scenario_handler.go backend/internal/handlers/profitability_scenario_handler_test.go backend/internal/routes/profitability_routes.go
git commit -m "feat(profitability): add scenario endpoints and live compute"
```

---

### Task 7: Frontend – service és típusok

**Files:**
- Modify: `frontend/src/services/profitabilityService.ts`

**Interfaces:**
- Consumes: a Task 5–6 végpontjai.
- Produces: típusok `PercentItem`, `FixedCost`, `ParameterSet`, `ParameterSetInput`, `ScenarioAdjustment`, `ScenarioNewClient`, `ScenarioInput`, `Scenario`, `ScenarioItemAmount`, `ScenarioMonth`, `ScenarioClientResult`, `ScenarioResult`; metódusok `listParameterSets`, `createParameterSet`, `updateParameterSet`, `deleteParameterSet`, `listScenarios`, `createScenario`, `updateScenario`, `deleteScenario`, `computeScenario`.

- [ ] **Step 1: Nézd meg, hogyan adja vissza az `apiClient` a hibaüzenetet**

Run: `sed -n 40,80p frontend/src/lib/api.ts`
Cél: megállapítani, hogy a szerver `{"message": …}` törzse (pl. a `409` „Parameter set is used by 2 scenario(s)”) eljut-e az `err.message`-be. A későbbi UI-lépések erre építenek; ha az `apiClient` ezt nem teszi meg, jegyezd fel a jelentésben (ne módosítsd a `lib/api.ts`-t, az nem ennek a tervnek a része).

- [ ] **Step 2: Add hozzá a típusokat**

A `profitabilityService.ts` típusai közé (a `ProfitabilityForecast` után):

```ts
export type PercentBase = 'revenue' | 'after_costs';

export interface PercentItem {
    label: string;
    percent: number;
    base: PercentBase;
}

export interface FixedCost {
    label: string;
    amount: number;
}

export interface ParameterSet {
    id: number;
    name: string;
    percent_items: PercentItem[];
    fixed_monthly_costs: FixedCost[];
    created_by: number;
    created_at: string;
    updated_at: string;
}

export interface ParameterSetInput {
    name: string;
    percent_items: PercentItem[];
    fixed_monthly_costs: FixedCost[];
}

export interface ScenarioAdjustment {
    client_id: number;
    new_hourly_rate: number | null;
    new_fixed_price: number | null;
    hours_delta: number;
    drops: boolean;
}

export interface ScenarioNewClient {
    name: string;
    monthly_revenue: number;
    monthly_hours: number;
}

export interface ScenarioInput {
    name: string;
    horizon_months: number;
    parameter_set_id: number;
    capacity_hours_per_month: number;
    client_adjustments: ScenarioAdjustment[];
    new_clients: ScenarioNewClient[];
}

export interface Scenario extends ScenarioInput {
    id: number;
    created_by: number;
    created_at: string;
    updated_at: string;
}

export interface ScenarioItemAmount {
    label: string;
    percent: number;
    base: PercentBase;
    amount: number;
}

export interface ScenarioMonth {
    revenue: number;
    fixed_costs: number;
    revenue_items: ScenarioItemAmount[];
    result_after_costs: number;
    after_costs_items: ScenarioItemAmount[];
    net_profit: number;
    required_hours: number;
    capacity_hours: number;
    utilization: number | null;
    overloaded: boolean;
    net_profit_per_capacity_hour: number | null;
}

export interface ScenarioClientResult {
    client_id: number;
    name: string;
    is_new: boolean;
    dropped: boolean;
    base_revenue: number;
    base_hours: number;
    revenue: number;
    hours: number;
}

export interface ScenarioResult {
    months: string[];
    monthly: ScenarioMonth;
    baseline: ScenarioMonth;
    horizon: { revenue: number; net_profit: number };
    baseline_horizon: { revenue: number; net_profit: number };
    clients: ScenarioClientResult[];
    low_data: boolean;
    warnings: string[];
    basis: {
        parameter_set_name: string;
        percent_items: PercentItem[];
        fixed_monthly_costs: FixedCost[];
        capacity_hours: number;
        baseline_months: string[];
        horizon_months: number;
    };
}
```

- [ ] **Step 3: Add hozzá a hívásokat**

A `profitabilityService` objektumba (a `forecast` után):

```ts
    async listParameterSets(): Promise<ParameterSet[]> {
        return apiClient.get('/profitability/parameter-sets');
    },

    async createParameterSet(data: ParameterSetInput): Promise<ParameterSet> {
        return apiClient.post('/profitability/parameter-sets', data);
    },

    async updateParameterSet(id: number, data: ParameterSetInput): Promise<ParameterSet> {
        return apiClient.put(`/profitability/parameter-sets/${id}`, data);
    },

    async deleteParameterSet(id: number): Promise<void> {
        return apiClient.delete(`/profitability/parameter-sets/${id}`);
    },

    async listScenarios(): Promise<Scenario[]> {
        return apiClient.get('/profitability/scenarios');
    },

    async createScenario(data: ScenarioInput): Promise<Scenario> {
        return apiClient.post('/profitability/scenarios', data);
    },

    async updateScenario(id: number, data: ScenarioInput): Promise<Scenario> {
        return apiClient.put(`/profitability/scenarios/${id}`, data);
    },

    async deleteScenario(id: number): Promise<void> {
        return apiClient.delete(`/profitability/scenarios/${id}`);
    },

    async computeScenario(data: ScenarioInput): Promise<ScenarioResult> {
        return apiClient.post('/profitability/scenarios/compute', data);
    },
```

- [ ] **Step 4: Típusellenőrzés és lint**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cd /app && npx tsc --noEmit -p . 2>&1 | grep -E "profitabilityService" || echo "tsc: nincs hiba az érintett fájlban"; npx eslint src/services/profitabilityService.ts 2>&1 | tail -6; echo eslint-done'`
Expected: `tsc: nincs hiba az érintett fájlban`, üres eslint-kimenet.

- [ ] **Step 5: Commit** (csak kérésre)

```bash
git add frontend/src/services/profitabilityService.ts
git commit -m "feat(profitability): add scenario service and types"
```

---

### Task 8: Frontend – paraméterkészlet-kezelő ablak

**Files:**
- Create: `frontend/src/components/profitability/ParameterSetsModal.tsx`

**Interfaces:**
- Consumes: `profitabilityService` hívások és típusok (Task 7); `Modal`, `Input`, `Select`, `Button`, `LoadingState` (meglévő UI).
- Produces: `export default function ParameterSetsModal(props: { isOpen: boolean; onClose: () => void; canManage: boolean; onChanged: () => void })`. Az `onChanged` minden sikeres mentés/törlés után meghívódik, hogy a szülő újratöltse a készlet-listát.

- [ ] **Step 1: Nézd meg a használt UI-komponensek tényleges tulajdonságait**

Run: `sed -n 1,60p frontend/src/components/ui/modal.tsx | head -60; sed -n 1,50p frontend/src/components/ui/button.tsx | head -50`
Cél: a `Modal` (`isOpen`, `onClose`, `title`, `size`), a `Button` (`variant`, `size`, `icon`, `loading`, `disabled`, `onClick`) és az `Input`/`Select` (`label`, `value`, `onChange(value: string)`) tulajdonságnevei egyezzenek az alábbi kóddal. Ha eltérés van, igazítsd minimálisan és jelezd a jelentésben.

- [ ] **Step 2: Írd meg a komponenst**

```tsx
// frontend/src/components/profitability/ParameterSetsModal.tsx
'use client';

import React from 'react';
import {
    profitabilityService,
    ParameterSet,
    ParameterSetInput,
    PercentBase,
} from '@/services/profitabilityService';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Modal } from '@/components/ui/modal';
import LoadingState from '@/components/ui/LoadingState';
import { formatHuf } from '@/utils/formatHuf';
import { Plus, Pencil, Trash2 } from 'lucide-react';

const errorMessage = (err: unknown) => (err instanceof Error ? err.message : String(err));

const BASE_OPTIONS = [
    { value: 'revenue', label: 'A bevétel %-a' },
    { value: 'after_costs', label: 'A költségek utáni eredmény %-a' },
];

interface PercentDraft {
    label: string;
    percent: string;
    base: PercentBase;
}

interface CostDraft {
    label: string;
    amount: string;
}

interface SetDraft {
    id: number | null;
    name: string;
    percentItems: PercentDraft[];
    costs: CostDraft[];
}

const emptyDraft: SetDraft = { id: null, name: '', percentItems: [], costs: [] };

// An empty or invalid number field counts as 0; the server validates the range.
const toNumber = (s: string) => {
    const n = Number(s.trim().replace(',', '.'));
    return Number.isFinite(n) ? n : 0;
};

const draftFromSet = (set: ParameterSet): SetDraft => ({
    id: set.id,
    name: set.name,
    percentItems: set.percent_items.map((p) => ({ label: p.label, percent: String(p.percent), base: p.base })),
    costs: set.fixed_monthly_costs.map((c) => ({ label: c.label, amount: String(c.amount) })),
});

const inputFromDraft = (d: SetDraft): ParameterSetInput => ({
    name: d.name,
    percent_items: d.percentItems.map((p) => ({ label: p.label, percent: toNumber(p.percent), base: p.base })),
    fixed_monthly_costs: d.costs.map((c) => ({ label: c.label, amount: toNumber(c.amount) })),
});

interface ParameterSetsModalProps {
    isOpen: boolean;
    onClose: () => void;
    canManage: boolean;
    onChanged: () => void;
}

export default function ParameterSetsModal({ isOpen, onClose, canManage, onChanged }: ParameterSetsModalProps) {
    const [sets, setSets] = React.useState<ParameterSet[]>([]);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);
    const [draft, setDraft] = React.useState<SetDraft | null>(null);
    const [isSaving, setIsSaving] = React.useState(false);

    const load = React.useCallback(async () => {
        try {
            setLoading(true);
            setError(null);
            setSets(await profitabilityService.listParameterSets());
        } catch (err: unknown) {
            setError(errorMessage(err));
        } finally {
            setLoading(false);
        }
    }, []);

    React.useEffect(() => {
        if (isOpen) {
            setDraft(null);
            load();
        }
    }, [isOpen, load]);

    const save = async () => {
        if (!draft) return;
        setIsSaving(true);
        try {
            const input = inputFromDraft(draft);
            if (draft.id === null) {
                await profitabilityService.createParameterSet(input);
            } else {
                await profitabilityService.updateParameterSet(draft.id, input);
            }
            setError(null);
            setDraft(null);
            await load();
            onChanged();
        } catch (err: unknown) {
            setError(errorMessage(err));
        } finally {
            setIsSaving(false);
        }
    };

    const remove = async (set: ParameterSet) => {
        if (!window.confirm(`Biztosan törlöd a(z) "${set.name}" paraméterkészletet?`)) return;
        try {
            await profitabilityService.deleteParameterSet(set.id);
            setError(null);
            await load();
            onChanged();
        } catch (err: unknown) {
            setError(errorMessage(err));
        }
    };

    const patchPercent = (i: number, patch: Partial<PercentDraft>) =>
        setDraft((d) => (d ? { ...d, percentItems: d.percentItems.map((p, idx) => (idx === i ? { ...p, ...patch } : p)) } : d));
    const patchCost = (i: number, patch: Partial<CostDraft>) =>
        setDraft((d) => (d ? { ...d, costs: d.costs.map((c, idx) => (idx === i ? { ...c, ...patch } : c)) } : d));

    return (
        <Modal isOpen={isOpen} onClose={onClose} title="Paraméterkészletek" size="lg">
            <div className="space-y-4 mt-2">
                {error && (
                    <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">
                        {error}
                    </div>
                )}

                {draft === null && (
                    <>
                        <p className="text-xs text-muted-foreground">
                            A készlet a forgatókönyv költségeit adja meg: fix havi költségeket és százalékos tételeket.
                            A rendszer nem tartalmaz adókulcsot vagy járulékot: minden értéket te adsz meg, és az eredmény
                            nem adótanács.
                        </p>

                        {loading && <LoadingState message="Készletek betöltése..." />}

                        {!loading && sets.length === 0 && (
                            <p className="text-sm text-muted-foreground">Még nincs paraméterkészlet.</p>
                        )}

                        {!loading &&
                            sets.map((set) => (
                                <div key={set.id} className="bg-card border border-border rounded-lg p-3">
                                    <div className="flex items-center justify-between gap-2">
                                        <h3 className="font-medium text-foreground">{set.name}</h3>
                                        {canManage && (
                                            <div className="flex items-center gap-1">
                                                <Button variant="ghost" size="icon-sm" icon={Pencil} onClick={() => setDraft(draftFromSet(set))} />
                                                <Button variant="ghost" size="icon-sm" icon={Trash2} onClick={() => remove(set)} />
                                            </div>
                                        )}
                                    </div>
                                    <ul className="mt-2 text-xs text-muted-foreground space-y-0.5">
                                        {set.fixed_monthly_costs.map((c, i) => (
                                            <li key={`c${i}`}>
                                                {c.label}: {formatHuf(c.amount)} / hó
                                            </li>
                                        ))}
                                        {set.percent_items.map((p, i) => (
                                            <li key={`p${i}`}>
                                                {p.label}: {p.percent}% ({p.base === 'revenue' ? 'a bevételből' : 'a költségek utáni eredményből'})
                                            </li>
                                        ))}
                                        {set.fixed_monthly_costs.length === 0 && set.percent_items.length === 0 && <li>Nincs tétel.</li>}
                                    </ul>
                                </div>
                            ))}

                        {canManage && (
                            <div className="flex justify-end">
                                <Button icon={Plus} onClick={() => setDraft({ ...emptyDraft })}>
                                    Új készlet
                                </Button>
                            </div>
                        )}
                    </>
                )}

                {draft !== null && (
                    <div className="space-y-4">
                        <Input label="Név" value={draft.name} onChange={(v) => setDraft((d) => (d ? { ...d, name: v } : d))} />

                        <section>
                            <h3 className="text-sm font-medium text-foreground mb-2">Fix havi költségek</h3>
                            <div className="space-y-2">
                                {draft.costs.map((c, i) => (
                                    <div key={i} className="flex items-end gap-2">
                                        <div className="flex-1">
                                            <Input label="Megnevezés" value={c.label} onChange={(v) => patchCost(i, { label: v })} />
                                        </div>
                                        <div className="w-40">
                                            <Input label="Összeg (Ft / hó)" type="number" value={c.amount} onChange={(v) => patchCost(i, { amount: v })} />
                                        </div>
                                        <Button
                                            variant="ghost"
                                            size="icon-sm"
                                            icon={Trash2}
                                            onClick={() => setDraft((d) => (d ? { ...d, costs: d.costs.filter((_, idx) => idx !== i) } : d))}
                                        />
                                    </div>
                                ))}
                            </div>
                            <div className="mt-2">
                                <Button
                                    variant="secondary"
                                    size="sm"
                                    icon={Plus}
                                    onClick={() => setDraft((d) => (d ? { ...d, costs: [...d.costs, { label: '', amount: '0' }] } : d))}
                                >
                                    Költség
                                </Button>
                            </div>
                        </section>

                        <section>
                            <h3 className="text-sm font-medium text-foreground mb-2">Százalékos tételek</h3>
                            <div className="space-y-2">
                                {draft.percentItems.map((p, i) => (
                                    <div key={i} className="flex items-end gap-2">
                                        <div className="flex-1">
                                            <Input label="Megnevezés" value={p.label} onChange={(v) => patchPercent(i, { label: v })} />
                                        </div>
                                        <div className="w-24">
                                            <Input label="%" type="number" value={p.percent} onChange={(v) => patchPercent(i, { percent: v })} />
                                        </div>
                                        <div className="w-56">
                                            <Select
                                                label="Alap"
                                                value={p.base}
                                                options={BASE_OPTIONS}
                                                onChange={(v) => patchPercent(i, { base: v as PercentBase })}
                                            />
                                        </div>
                                        <Button
                                            variant="ghost"
                                            size="icon-sm"
                                            icon={Trash2}
                                            onClick={() => setDraft((d) => (d ? { ...d, percentItems: d.percentItems.filter((_, idx) => idx !== i) } : d))}
                                        />
                                    </div>
                                ))}
                            </div>
                            <div className="mt-2">
                                <Button
                                    variant="secondary"
                                    size="sm"
                                    icon={Plus}
                                    onClick={() =>
                                        setDraft((d) =>
                                            d ? { ...d, percentItems: [...d.percentItems, { label: '', percent: '0', base: 'revenue' }] } : d
                                        )
                                    }
                                >
                                    Százalékos tétel
                                </Button>
                            </div>
                        </section>

                        <div className="flex justify-end gap-2 pt-2">
                            <Button variant="secondary" onClick={() => setDraft(null)}>
                                Mégse
                            </Button>
                            <Button onClick={save} loading={isSaving} disabled={!draft.name.trim()}>
                                Mentés
                            </Button>
                        </div>
                    </div>
                )}
            </div>
        </Modal>
    );
}
```

- [ ] **Step 3: Típusellenőrzés és lint**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cd /app && npx tsc --noEmit -p . 2>&1 | grep -E "ParameterSetsModal|profitabilityService" || echo "tsc: nincs hiba az érintett fájlokban"; npx eslint src/components/profitability/ParameterSetsModal.tsx 2>&1 | tail -8; echo eslint-done'`
Expected: `tsc: nincs hiba az érintett fájlokban`, üres eslint-kimenet. Ha a `Button` `size="sm"` vagy a `Modal` `size="lg"` nem létező érték, használj egy meglévőt a Step 1 alapján, és jelezd.

- [ ] **Step 4: Commit** (csak kérésre)

```bash
git add frontend/src/components/profitability/ParameterSetsModal.tsx
git commit -m "feat(profitability): add parameter sets modal"
```

---

### Task 9: Frontend – az eredmény-panel

**Files:**
- Create: `frontend/src/components/profitability/ScenarioResultPanel.tsx`

**Interfaces:**
- Consumes: `ScenarioResult`, `ScenarioMonth`, `ScenarioItemAmount` (Task 7); `formatHuf`.
- Produces: `export default function ScenarioResultPanel({ result }: { result: ScenarioResult })`.

- [ ] **Step 1: Írd meg a komponenst**

```tsx
// frontend/src/components/profitability/ScenarioResultPanel.tsx
'use client';

import React from 'react';
import { ScenarioResult, ScenarioMonth } from '@/services/profitabilityService';
import { formatHuf } from '@/utils/formatHuf';

const formatHours = (v: number) => `${v.toFixed(1).replace('.', ',')} ó`;
const formatPercent = (v: number | null) => (v === null ? '—' : `${Math.round(v)}%`);

interface Row {
    label: string;
    base: string;
    scenario: string;
    delta: string | null;
    // positive = better for the owner, used only for the colour of the delta
    better: boolean | null;
    emphasis?: boolean;
}

function money(label: string, base: number, scenario: number, opts: { costLike?: boolean; emphasis?: boolean } = {}): Row {
    const diff = scenario - base;
    const rounded = Math.round(diff);
    return {
        label,
        base: formatHuf(base),
        scenario: formatHuf(scenario),
        delta: rounded === 0 ? '—' : `${rounded > 0 ? '+' : ''}${rounded.toLocaleString('hu-HU')} Ft`,
        better: rounded === 0 ? null : opts.costLike ? diff < 0 : diff > 0,
        emphasis: opts.emphasis,
    };
}

function buildRows(base: ScenarioMonth, scenario: ScenarioMonth): Row[] {
    const rows: Row[] = [money('Bevétel', base.revenue, scenario.revenue)];
    rows.push(money('Fix költségek', base.fixed_costs, scenario.fixed_costs, { costLike: true }));
    // Both months are priced with the same parameter set, so the items line up by index.
    scenario.revenue_items.forEach((item, i) =>
        rows.push(money(`${item.label} (${item.percent}% a bevételből)`, base.revenue_items[i]?.amount ?? 0, item.amount, { costLike: true }))
    );
    rows.push(money('Eredmény a költségek után', base.result_after_costs, scenario.result_after_costs));
    scenario.after_costs_items.forEach((item, i) =>
        rows.push(
            money(`${item.label} (${item.percent}% az eredményből)`, base.after_costs_items[i]?.amount ?? 0, item.amount, { costLike: true })
        )
    );
    rows.push(money('Nettó nyereség / hó', base.net_profit, scenario.net_profit, { emphasis: true }));

    const hoursDiff = scenario.required_hours - base.required_hours;
    rows.push({
        label: 'Szükséges órák / hó',
        base: formatHours(base.required_hours),
        scenario: formatHours(scenario.required_hours),
        delta: Math.abs(hoursDiff) < 0.05 ? '—' : `${hoursDiff > 0 ? '+' : ''}${hoursDiff.toFixed(1).replace('.', ',')} ó`,
        better: null,
    });
    rows.push({
        label: `Kihasználtság (kapacitás: ${formatHours(scenario.capacity_hours)})`,
        base: formatPercent(base.utilization),
        scenario: formatPercent(scenario.utilization),
        delta: null,
        better: null,
    });
    const basePerHour = base.net_profit_per_capacity_hour;
    const perHour = scenario.net_profit_per_capacity_hour;
    if (basePerHour !== null && perHour !== null) {
        rows.push(money('Nettó nyereség / kapacitásóra', basePerHour, perHour, { emphasis: true }));
    }
    return rows;
}

export default function ScenarioResultPanel({ result }: { result: ScenarioResult }) {
    const rows = buildRows(result.baseline, result.monthly);
    const horizonDelta = result.horizon.net_profit - result.baseline_horizon.net_profit;

    return (
        <div>
            {result.monthly.overloaded && (
                <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm mb-3">
                    A forgatókönyv {formatHours(result.monthly.required_hours)} havi munkát igényel, ami meghaladja a megadott{' '}
                    {formatHours(result.monthly.capacity_hours)} kapacitást.
                </div>
            )}

            {result.warnings.map((w) => (
                <div key={w} className="bg-muted border border-border text-foreground px-3 py-2 rounded text-sm mb-3">
                    {w}
                </div>
            ))}

            <div className="overflow-x-auto bg-card border border-border rounded-lg">
                <table className="w-full text-sm">
                    <thead className="text-left text-muted-foreground border-b border-border">
                        <tr>
                            <th className="p-3">Havi eredmény</th>
                            <th className="p-3 text-right">Alaphelyzet</th>
                            <th className="p-3 text-right">Forgatókönyv</th>
                            <th className="p-3 text-right">Változás</th>
                        </tr>
                    </thead>
                    <tbody>
                        {rows.map((row) => (
                            <tr key={row.label} className="border-b border-border last:border-0">
                                <td className={`p-3 ${row.emphasis ? 'font-medium text-foreground' : 'text-foreground'}`}>{row.label}</td>
                                <td className="p-3 text-right">{row.base}</td>
                                <td className={`p-3 text-right ${row.emphasis ? 'font-medium' : ''}`}>{row.scenario}</td>
                                <td
                                    className={`p-3 text-right ${
                                        row.better === null ? 'text-muted-foreground' : row.better ? 'text-success' : 'text-destructive'
                                    }`}
                                >
                                    {row.delta ?? '—'}
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            </div>

            <p className="text-sm text-foreground mt-3">
                {result.months.length} hónap alatt a nettó nyereség{' '}
                <span className="font-medium">{formatHuf(result.horizon.net_profit)}</span> (alaphelyzet:{' '}
                {formatHuf(result.baseline_horizon.net_profit)}, különbség:{' '}
                <span className={horizonDelta >= 0 ? 'text-success' : 'text-destructive'}>
                    {horizonDelta >= 0 ? '+' : ''}
                    {Math.round(horizonDelta).toLocaleString('hu-HU')} Ft
                </span>
                ).
            </p>

            <details className="mt-3 text-xs text-muted-foreground">
                <summary className="cursor-pointer">Mivel számoltunk?</summary>
                <ul className="mt-2 space-y-1">
                    <li>
                        Alap: az utolsó {result.basis.baseline_months.length} teljes hónap ({result.basis.baseline_months.join(', ')})
                        rendszeres (nem fix áras) számláinak és óráinak havi átlaga, nettó összegekkel.
                    </li>
                    <li>Paraméterkészlet: {result.basis.parameter_set_name}</li>
                    {result.basis.fixed_monthly_costs.map((c, i) => (
                        <li key={`c${i}`}>
                            {c.label}: {formatHuf(c.amount)} / hó
                        </li>
                    ))}
                    {result.basis.percent_items.map((p, i) => (
                        <li key={`p${i}`}>
                            {p.label}: {p.percent}% ({p.base === 'revenue' ? 'a bevételből' : 'a költségek utáni eredményből'})
                        </li>
                    ))}
                    <li>
                        Kapacitás: {formatHours(result.basis.capacity_hours)} / hó. A havi eredmény minden vizsgált hónapra azonos; a
                        szerződések lejárata nincs figyelembe véve. A százalékos tételek az eredményen nem halmozódnak, és
                        negatív eredményre nem számolódik tétel.
                    </li>
                    {result.low_data && <li>Kevés a rendszeres számlázási előzmény, a számok tájékoztató jellegűek.</li>}
                </ul>
            </details>
        </div>
    );
}
```

- [ ] **Step 2: Ellenőrizd a használt Tailwind színosztályokat**

Run: `grep -n "success\|destructive" frontend/src/app/globals.css | head -8`
Cél: a `text-success` osztály létezik-e a témában (`--color-success`). Ha nincs, cseréld a `text-success` előfordulásait egy létezőre (pl. `text-primary`), és jelezd a jelentésben.

- [ ] **Step 3: Típusellenőrzés és lint**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cd /app && npx tsc --noEmit -p . 2>&1 | grep -E "ScenarioResultPanel|profitabilityService" || echo "tsc: nincs hiba az érintett fájlokban"; npx eslint src/components/profitability/ScenarioResultPanel.tsx 2>&1 | tail -8; echo eslint-done'`
Expected: `tsc: nincs hiba az érintett fájlokban`, üres eslint-kimenet.

- [ ] **Step 4: Commit** (csak kérésre)

```bash
git add frontend/src/components/profitability/ScenarioResultPanel.tsx
git commit -m "feat(profitability): add scenario result panel"
```

---

### Task 10: Frontend – szerkesztő, fül és az oldalba építés

**Files:**
- Create: `frontend/src/components/profitability/ScenarioEditor.tsx`
- Create: `frontend/src/components/profitability/ScenariosTab.tsx`
- Modify: `frontend/src/app/dashboard/profitability/page.tsx`

**Interfaces:**
- Consumes: Task 7 szolgáltatás és típusok, Task 8 `ParameterSetsModal`, Task 9 `ScenarioResultPanel`; a stage 1 `profitabilityService.getSettings()` (alap kapacitás).
- Produces: `ScenarioEditor` (props: `scenario: Scenario | null`, `parameterSets: ParameterSet[]`, `defaultCapacity: number`, `canManage: boolean`, `onSaved(s: Scenario): void`, `onDeleted(id: number): void`, `onOpenParameterSets(): void`), `ScenariosTab` (props: `canManage: boolean`).

- [ ] **Step 1: Írd meg a szerkesztőt**

```tsx
// frontend/src/components/profitability/ScenarioEditor.tsx
'use client';

import React from 'react';
import {
    profitabilityService,
    ParameterSet,
    Scenario,
    ScenarioAdjustment,
    ScenarioInput,
    ScenarioNewClient,
    ScenarioResult,
} from '@/services/profitabilityService';
import ScenarioResultPanel from '@/components/profitability/ScenarioResultPanel';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { formatHuf } from '@/utils/formatHuf';
import { Plus, Trash2, Settings2 } from 'lucide-react';

const errorMessage = (err: unknown) => (err instanceof Error ? err.message : String(err));

interface AdjustmentDraft {
    rate: string;
    fixed: string;
    delta: string;
    drops: boolean;
}

interface NewClientDraft {
    key: number;
    name: string;
    revenue: string;
    hours: string;
}

interface Draft {
    name: string;
    horizon: string;
    capacity: string;
    parameterSetId: string;
    adjustments: Record<number, AdjustmentDraft>;
    newClients: NewClientDraft[];
}

const emptyAdjustment: AdjustmentDraft = { rate: '', fixed: '', delta: '', drops: false };

// null = empty or not a number.
const parseNum = (s: string): number | null => {
    const t = s.trim().replace(',', '.');
    if (t === '') return null;
    const n = Number(t);
    return Number.isFinite(n) ? n : null;
};

// Builds the request body, or null while the draft cannot be priced yet
// (no parameter set / capacity). Half-filled rows are left out so that live
// typing does not trigger validation errors.
function toInput(draft: Draft): ScenarioInput | null {
    const capacity = parseNum(draft.capacity);
    const setId = Number(draft.parameterSetId);
    const horizon = Number(draft.horizon);
    if (capacity === null || capacity <= 0 || !Number.isFinite(setId) || setId <= 0) return null;

    const adjustments: ScenarioAdjustment[] = [];
    for (const [clientId, a] of Object.entries(draft.adjustments)) {
        const rate = parseNum(a.rate);
        const fixed = parseNum(a.fixed);
        const delta = parseNum(a.delta) ?? 0;
        if (!a.drops && rate === null && fixed === null && delta === 0) continue;
        adjustments.push({
            client_id: Number(clientId),
            new_hourly_rate: rate,
            new_fixed_price: fixed,
            hours_delta: delta,
            drops: a.drops,
        });
    }

    const newClients: ScenarioNewClient[] = draft.newClients
        .filter((n) => n.name.trim() !== '')
        .map((n) => ({
            name: n.name.trim(),
            monthly_revenue: parseNum(n.revenue) ?? 0,
            monthly_hours: parseNum(n.hours) ?? 0,
        }));

    return {
        name: draft.name.trim(),
        horizon_months: horizon,
        parameter_set_id: setId,
        capacity_hours_per_month: capacity,
        client_adjustments: adjustments,
        new_clients: newClients,
    };
}

function draftFromScenario(s: Scenario): Draft {
    const adjustments: Record<number, AdjustmentDraft> = {};
    for (const a of s.client_adjustments) {
        adjustments[a.client_id] = {
            rate: a.new_hourly_rate === null ? '' : String(a.new_hourly_rate),
            fixed: a.new_fixed_price === null ? '' : String(a.new_fixed_price),
            delta: a.hours_delta === 0 ? '' : String(a.hours_delta),
            drops: a.drops,
        };
    }
    return {
        name: s.name,
        horizon: String(s.horizon_months),
        capacity: String(s.capacity_hours_per_month),
        parameterSetId: String(s.parameter_set_id),
        adjustments,
        newClients: s.new_clients.map((n, i) => ({
            key: i,
            name: n.name,
            revenue: String(n.monthly_revenue),
            hours: String(n.monthly_hours),
        })),
    };
}

interface ScenarioEditorProps {
    scenario: Scenario | null;
    parameterSets: ParameterSet[];
    defaultCapacity: number;
    canManage: boolean;
    onSaved: (scenario: Scenario) => void;
    onDeleted: (id: number) => void;
    onOpenParameterSets: () => void;
}

export default function ScenarioEditor({
    scenario,
    parameterSets,
    defaultCapacity,
    canManage,
    onSaved,
    onDeleted,
    onOpenParameterSets,
}: ScenarioEditorProps) {
    const [draft, setDraft] = React.useState<Draft>(() =>
        scenario
            ? draftFromScenario(scenario)
            : {
                  name: '',
                  horizon: '6',
                  capacity: String(defaultCapacity),
                  parameterSetId: parameterSets[0] ? String(parameterSets[0].id) : '',
                  adjustments: {},
                  newClients: [],
              }
    );
    const [result, setResult] = React.useState<ScenarioResult | null>(null);
    const [computing, setComputing] = React.useState(false);
    const [error, setError] = React.useState<string | null>(null);
    const [isSaving, setIsSaving] = React.useState(false);
    const requestId = React.useRef(0);
    const nextKey = React.useRef(1000);

    // If the parameter sets load after the editor opened (or the chosen one was
    // deleted), fall back to the first available one.
    React.useEffect(() => {
        if (parameterSets.length === 0) return;
        const exists = parameterSets.some((p) => String(p.id) === draft.parameterSetId);
        if (!exists && !scenario) {
            setDraft((d) => ({ ...d, parameterSetId: String(parameterSets[0].id) }));
        }
    }, [parameterSets, draft.parameterSetId, scenario]);

    // Live pricing: wait 400 ms after the last change and drop stale answers.
    React.useEffect(() => {
        const input = toInput(draft);
        if (!input) {
            setResult(null);
            return;
        }
        const id = ++requestId.current;
        const timer = setTimeout(async () => {
            try {
                setComputing(true);
                const r = await profitabilityService.computeScenario(input);
                if (id === requestId.current) {
                    setResult(r);
                    setError(null);
                }
            } catch (err: unknown) {
                if (id === requestId.current) setError(errorMessage(err));
            } finally {
                if (id === requestId.current) setComputing(false);
            }
        }, 400);
        return () => clearTimeout(timer);
    }, [draft]);

    const patchAdjustment = (clientId: number, patch: Partial<AdjustmentDraft>) =>
        setDraft((d) => ({
            ...d,
            adjustments: { ...d.adjustments, [clientId]: { ...(d.adjustments[clientId] ?? emptyAdjustment), ...patch } },
        }));

    const patchNewClient = (key: number, patch: Partial<NewClientDraft>) =>
        setDraft((d) => ({ ...d, newClients: d.newClients.map((n) => (n.key === key ? { ...n, ...patch } : n)) }));

    const save = async () => {
        const input = toInput(draft);
        if (!input || !input.name) {
            setError('A mentéshez adj nevet, válassz paraméterkészletet és adj meg kapacitást.');
            return;
        }
        setIsSaving(true);
        try {
            const saved = scenario
                ? await profitabilityService.updateScenario(scenario.id, input)
                : await profitabilityService.createScenario(input);
            setError(null);
            onSaved(saved);
        } catch (err: unknown) {
            setError(errorMessage(err));
        } finally {
            setIsSaving(false);
        }
    };

    const remove = async () => {
        if (!scenario) return;
        if (!window.confirm(`Biztosan törlöd a(z) "${scenario.name}" forgatókönyvet?`)) return;
        try {
            await profitabilityService.deleteScenario(scenario.id);
            onDeleted(scenario.id);
        } catch (err: unknown) {
            setError(errorMessage(err));
        }
    };

    const clientRows = result ? result.clients.filter((c) => !c.is_new) : [];

    return (
        <div className="space-y-5">
            {error && (
                <div className="bg-destructive/10 border border-destructive/20 text-destructive px-3 py-2 rounded text-sm">{error}</div>
            )}

            <div className="grid gap-3 sm:grid-cols-2">
                <Input label="Név" value={draft.name} onChange={(v) => setDraft((d) => ({ ...d, name: v }))} />
                <Select
                    label="Időtáv"
                    value={draft.horizon}
                    options={[3, 4, 5, 6].map((m) => ({ value: String(m), label: `${m} hónap` }))}
                    onChange={(v) => setDraft((d) => ({ ...d, horizon: v }))}
                />
                <Input
                    label="Kapacitás (óra / hó)"
                    type="number"
                    value={draft.capacity}
                    onChange={(v) => setDraft((d) => ({ ...d, capacity: v }))}
                />
                <div className="flex items-end gap-2">
                    <div className="flex-1">
                        <Select
                            label="Paraméterkészlet"
                            value={draft.parameterSetId}
                            options={parameterSets.map((p) => ({ value: String(p.id), label: p.name }))}
                            placeholder="Válassz készletet"
                            onChange={(v) => setDraft((d) => ({ ...d, parameterSetId: v }))}
                        />
                    </div>
                    <Button variant="secondary" icon={Settings2} onClick={onOpenParameterSets}>
                        Készletek
                    </Button>
                </div>
            </div>

            {parameterSets.length === 0 && (
                <p className="text-sm text-muted-foreground">
                    Az eredményhez előbb hozz létre egy paraméterkészletet (az akár üres is lehet).
                </p>
            )}

            <section>
                <h3 className="text-sm font-medium text-foreground mb-2">Ügyfelek módosítása</h3>
                {!result && <p className="text-sm text-muted-foreground">Az ügyfelek az első számolás után jelennek meg.</p>}
                {result && clientRows.length === 0 && (
                    <p className="text-sm text-muted-foreground">Nincs rendszeres bevételű ügyfél az alapidőszakban.</p>
                )}
                {clientRows.length > 0 && (
                    <div className="overflow-x-auto bg-card border border-border rounded-lg">
                        <table className="w-full text-sm">
                            <thead className="text-left text-muted-foreground border-b border-border">
                                <tr>
                                    <th className="p-3">Ügyfél</th>
                                    <th className="p-3 text-right">Havi bevétel</th>
                                    <th className="p-3 text-right">Havi óra</th>
                                    <th className="p-3">Új óradíj (Ft)</th>
                                    <th className="p-3">Új fix ár (Ft)</th>
                                    <th className="p-3">Óraváltozás</th>
                                    <th className="p-3">Kiesik</th>
                                    <th className="p-3 text-right">Új bevétel</th>
                                </tr>
                            </thead>
                            <tbody>
                                {clientRows.map((c) => {
                                    const a = draft.adjustments[c.client_id] ?? emptyAdjustment;
                                    return (
                                        <tr key={c.client_id} className="border-b border-border last:border-0 align-middle">
                                            <td className="p-3 text-foreground">{c.name}</td>
                                            <td className="p-3 text-right">{formatHuf(c.base_revenue)}</td>
                                            <td className="p-3 text-right">{c.base_hours.toFixed(1).replace('.', ',')} ó</td>
                                            <td className="p-2 w-32">
                                                <Input type="number" value={a.rate} onChange={(v) => patchAdjustment(c.client_id, { rate: v })} />
                                            </td>
                                            <td className="p-2 w-32">
                                                <Input type="number" value={a.fixed} onChange={(v) => patchAdjustment(c.client_id, { fixed: v })} />
                                            </td>
                                            <td className="p-2 w-24">
                                                <Input type="number" value={a.delta} onChange={(v) => patchAdjustment(c.client_id, { delta: v })} />
                                            </td>
                                            <td className="p-3">
                                                <input
                                                    type="checkbox"
                                                    checked={a.drops}
                                                    aria-label={`${c.name} kiesik`}
                                                    onChange={(e) => patchAdjustment(c.client_id, { drops: e.target.checked })}
                                                />
                                            </td>
                                            <td className="p-3 text-right font-medium">{c.dropped ? '—' : formatHuf(c.revenue)}</td>
                                        </tr>
                                    );
                                })}
                            </tbody>
                        </table>
                    </div>
                )}
                <p className="text-xs text-muted-foreground mt-2">
                    Új óradíj és új fix ár egyszerre nem adható meg. Az óraváltozás a naplózott órákra vonatkozik; ár nélkül a bevétel a
                    jelenlegi óradíjon arányosan változik.
                </p>
            </section>

            <section>
                <h3 className="text-sm font-medium text-foreground mb-2">Új ügyfelek</h3>
                <div className="space-y-2">
                    {draft.newClients.map((n) => (
                        <div key={n.key} className="flex items-end gap-2">
                            <div className="flex-1">
                                <Input label="Név" value={n.name} onChange={(v) => patchNewClient(n.key, { name: v })} />
                            </div>
                            <div className="w-40">
                                <Input label="Havi bevétel (Ft)" type="number" value={n.revenue} onChange={(v) => patchNewClient(n.key, { revenue: v })} />
                            </div>
                            <div className="w-32">
                                <Input label="Havi óra" type="number" value={n.hours} onChange={(v) => patchNewClient(n.key, { hours: v })} />
                            </div>
                            <Button
                                variant="ghost"
                                size="icon-sm"
                                icon={Trash2}
                                onClick={() => setDraft((d) => ({ ...d, newClients: d.newClients.filter((x) => x.key !== n.key) }))}
                            />
                        </div>
                    ))}
                </div>
                <div className="mt-2">
                    <Button
                        variant="secondary"
                        icon={Plus}
                        onClick={() =>
                            setDraft((d) => ({
                                ...d,
                                newClients: [...d.newClients, { key: nextKey.current++, name: '', revenue: '0', hours: '0' }],
                            }))
                        }
                    >
                        Új ügyfél
                    </Button>
                </div>
            </section>

            <section>
                <h3 className="text-sm font-medium text-foreground mb-2">
                    Eredmény {computing && <span className="text-xs font-normal text-muted-foreground">(számolás...)</span>}
                </h3>
                {result ? (
                    <ScenarioResultPanel result={result} />
                ) : (
                    <p className="text-sm text-muted-foreground">Válassz paraméterkészletet és adj meg kapacitást az eredményhez.</p>
                )}
            </section>

            {canManage && (
                <div className="flex justify-end gap-2">
                    {scenario && (
                        <Button variant="secondary" icon={Trash2} onClick={remove}>
                            Törlés
                        </Button>
                    )}
                    <Button onClick={save} loading={isSaving} disabled={!draft.name.trim()}>
                        Mentés
                    </Button>
                </div>
            )}
        </div>
    );
}
```

- [ ] **Step 2: Írd meg a fület**

```tsx
// frontend/src/components/profitability/ScenariosTab.tsx
'use client';

import React from 'react';
import { profitabilityService, ParameterSet, Scenario } from '@/services/profitabilityService';
import ScenarioEditor from '@/components/profitability/ScenarioEditor';
import ParameterSetsModal from '@/components/profitability/ParameterSetsModal';
import { Button } from '@/components/ui/button';
import LoadingState from '@/components/ui/LoadingState';
import ErrorState from '@/components/ui/ErrorState';
import { Plus } from 'lucide-react';

const errorMessage = (err: unknown) => (err instanceof Error ? err.message : String(err));

// null = a new, unsaved scenario; undefined = nothing selected.
type Selection = Scenario | null | undefined;

export default function ScenariosTab({ canManage }: { canManage: boolean }) {
    const [scenarios, setScenarios] = React.useState<Scenario[]>([]);
    const [parameterSets, setParameterSets] = React.useState<ParameterSet[]>([]);
    const [defaultCapacity, setDefaultCapacity] = React.useState(120);
    const [loading, setLoading] = React.useState(true);
    const [error, setError] = React.useState<string | null>(null);
    const [selected, setSelected] = React.useState<Selection>(undefined);
    const [showSets, setShowSets] = React.useState(false);

    const load = React.useCallback(async () => {
        try {
            setLoading(true);
            setError(null);
            const [list, sets, settings] = await Promise.all([
                profitabilityService.listScenarios(),
                profitabilityService.listParameterSets(),
                profitabilityService.getSettings(),
            ]);
            setScenarios(list);
            setParameterSets(sets);
            setDefaultCapacity(settings.default_capacity_hours_per_month);
        } catch (err: unknown) {
            setError(errorMessage(err));
        } finally {
            setLoading(false);
        }
    }, []);

    React.useEffect(() => {
        load();
    }, [load]);

    const reloadParameterSets = React.useCallback(async () => {
        try {
            setParameterSets(await profitabilityService.listParameterSets());
        } catch (err: unknown) {
            setError(errorMessage(err));
        }
    }, []);

    if (loading) return <LoadingState message="Forgatókönyvek betöltése..." />;
    if (error) return <ErrorState error={error} onRetry={load} />;

    return (
        <div className="grid gap-6 lg:grid-cols-[16rem_1fr]">
            <aside>
                <div className="flex items-center justify-between mb-3">
                    <h2 className="text-lg font-semibold text-foreground">Mentett</h2>
                    {canManage && (
                        <Button variant="secondary" size="sm" icon={Plus} onClick={() => setSelected(null)}>
                            Új
                        </Button>
                    )}
                </div>
                {scenarios.length === 0 && <p className="text-sm text-muted-foreground">Még nincs mentett forgatókönyv.</p>}
                <ul className="space-y-1">
                    {scenarios.map((s) => (
                        <li key={s.id}>
                            <button
                                onClick={() => setSelected(s)}
                                className={`w-full text-left px-3 py-2 rounded-md text-sm border ${
                                    selected && selected.id === s.id
                                        ? 'border-primary bg-primary/5 text-foreground'
                                        : 'border-border text-muted-foreground hover:text-foreground'
                                }`}
                            >
                                {s.name}
                                <span className="block text-xs text-muted-foreground">{s.horizon_months} hónap</span>
                            </button>
                        </li>
                    ))}
                </ul>
            </aside>

            <section>
                {selected === undefined ? (
                    <p className="text-sm text-muted-foreground">
                        Válassz egy mentett forgatókönyvet, vagy indíts újat. A számolás élőben frissül, mentés nélkül is.
                    </p>
                ) : (
                    <ScenarioEditor
                        key={selected ? selected.id : 'new'}
                        scenario={selected}
                        parameterSets={parameterSets}
                        defaultCapacity={defaultCapacity}
                        canManage={canManage}
                        onSaved={(saved) => {
                            setScenarios((prev) => (prev.some((p) => p.id === saved.id) ? prev.map((p) => (p.id === saved.id ? saved : p)) : [saved, ...prev]));
                            setSelected(saved);
                        }}
                        onDeleted={(id) => {
                            setScenarios((prev) => prev.filter((p) => p.id !== id));
                            setSelected(undefined);
                        }}
                        onOpenParameterSets={() => setShowSets(true)}
                    />
                )}
            </section>

            <ParameterSetsModal
                isOpen={showSets}
                onClose={() => setShowSets(false)}
                canManage={canManage}
                onChanged={reloadParameterSets}
            />
        </div>
    );
}
```

- [ ] **Step 3: Építsd be a `page.tsx`-be**

Olvasd el előbb a fájlt. Három változtatás:
1. Import: `import ScenariosTab from '@/components/profitability/ScenariosTab';`
2. Az `activeTab` típusa legyen `'overview' | 'forecast' | 'scenarios'` (az `useState` generikusa **és** az `onChange` cast), és a `<Tabs tabs=…>` listába kerüljön be a harmadik: `{ id: 'scenarios', label: 'Forgatókönyvek' }`.
3. A `{activeTab === 'forecast' && <ForecastTab />}` sor mellé: `{activeTab === 'scenarios' && <ScenariosTab canManage={canManage} />}`.

Az Áttekintés fül viselkedéséhez, a két `<Modal>`-hoz és a meglévő fejléc-feltételekhez (`activeTab === 'overview' &&`) ne nyúlj.

- [ ] **Step 4: Típusellenőrzés, lint, futásidejű fordulás**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cd /app && npx tsc --noEmit -p . 2>&1 | grep -E "profitability|Scenario|ParameterSets|formatHuf" || echo "tsc: nincs hiba az érintett fájlokban"; echo "tsc összes régi hiba: $(npx tsc --noEmit -p . 2>&1 | grep -c "error TS")"; npx eslint src/components/profitability/ScenarioEditor.tsx src/components/profitability/ScenariosTab.tsx src/components/profitability/ScenarioResultPanel.tsx src/components/profitability/ParameterSetsModal.tsx src/app/dashboard/profitability/page.tsx 2>&1 | tail -12; echo eslint-done'`
Expected: `tsc: nincs hiba az érintett fájlokban`, az összes régi hiba száma `6` (más fájlokban, ne nyúlj hozzájuk), üres eslint-kimenet. A `react-hooks/exhaustive-deps` figyelmeztetést a `ScenarioEditor` effektjeire ne némítsd el: ha jelez, jelezd a jelentésben a pontos sorral, és igazítsd a függőséglistát úgy, hogy a viselkedés ne változzon.

Run: `curl -s -o /dev/null -w "oldal: HTTP %{http_code}\n" http://localhost:3010/dashboard/profitability; docker compose -f .docker/docker-compose.yml logs --tail 30 frontend 2>&1 | grep -iE "error|failed to compile" | tail -5 || true`
Expected: `HTTP 200` (vagy átirányítás a belépésre), fordítási hiba nélkül.

- [ ] **Step 5: Commit** (csak kérésre)

```bash
git add frontend/src/components/profitability/ScenarioEditor.tsx frontend/src/components/profitability/ScenariosTab.tsx frontend/src/app/dashboard/profitability/page.tsx
git commit -m "feat(profitability): add scenarios tab with live pricing"
```

---

### Task 11: Végellenőrzés és dokumentáció

- [ ] **Step 1: Teljes backend-ellenőrzés konténerben**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -l internal/models/json_list*.go internal/models/profit_scenario.go internal/services/profitability_scenario*.go internal/handlers/profitability_scenario_handler*.go internal/routes/profitability_routes.go; go vet ./internal/... ; echo "vet exit: $?"; go build ./... && echo build-ok; go test -count=1 ./internal/models/ ./internal/services/ ./internal/handlers/ ./internal/middleware/ 2>&1 | grep -E "^(ok|FAIL|---|panic)"'`
Expected: `gofmt -l` üres, `vet exit: 0`, `build-ok`, mind a négy csomag `ok`. Ha egy **régi** teszt bukik, jelöld pre-existing-ként.

- [ ] **Step 2: Számítsd ki kézzel egy példát, és vesd össze a motor tesztjével**

Vedd a `TestBuildScenarioNewHourlyRate` esetét: ügyfél 1 (300 000 Ft/hó, 20 naplózott + 4 egyéb óra) új óradíja 18 000 Ft, ügyfél 2 (100 000 Ft/hó) változatlan; fix költség 20 000, 10% a bevételből, 15% a költségek utáni eredményből.
Kézzel: ügyfél 1 = 18 000 × 20 = 360 000; összes bevétel 460 000; bevétel-alapú tétel 46 000; eredmény a költségek után = 460 000 − 20 000 − 46 000 = 394 000; 15% = 59 100; nettó nyereség = 334 900. Az alaphelyzet nettó nyeresége 289 000, tehát +45 900 Ft/hó. Írd le a számítást a jelentésben; ha nem egyezik a teszttel, a hiba a számításban van.

- [ ] **Step 3: Végpont-ellenőrzés a futó backenden (token nélkül)**

Run: `for p in forecast scenarios parameter-sets; do curl -s -o /dev/null -w "GET /profitability/$p: HTTP %{http_code}\n" http://localhost:8080/api/v1/profitability/$p; done; curl -s -o /dev/null -X POST -w "POST /scenarios/compute: HTTP %{http_code}\n" http://localhost:8080/api/v1/profitability/scenarios/compute; docker compose -f .docker/docker-compose.yml exec -T postgres psql -U devbridge_user -d devbridge -tc "SELECT version, dirty FROM schema_migrations;"`
Expected: négyszer `HTTP 401`, a migrációs verzió `45 | f`.

- [ ] **Step 4: Böngészős ellenőrzés (ha van belépés)**

Jelentkezz be egy `admin` / `super_admin` felhasználóval (az új jogok miatt újra be kell lépni, ha korábban nem tetted) a `http://localhost:3010`-en, nyisd meg a **Jövedelmezőség** → **Forgatókönyvek** fület. Próbáld ki: új készlet létrehozása és törlése (a hivatkozott készlet törlésénél a `409` üzenet jelenik meg); új forgatókönyv mentése; az élő eredmény frissül gépelés után; túlterhelés esetén a figyelmeztetés látszik; a mentett forgatókönyv újra megnyitható. Ha nincs belépési adat, ezt **ne állítsd ellenőrzöttnek**: írd le, hogy a vizuális és interaktív rész nem volt kipróbálva. A dev adatbázisban nincs rendszeres számla, ezért az ügyféltábla üres lesz, amíg nincs ilyen adat; ezt is jegyezd fel.

- [ ] **Step 5: Frissítsd a specet**

A `docs/superpowers/specs/2026-10-01-profitability-design.md` „Nyitott pontok” részének végére fűzd:

```markdown
- **3. szakasz döntései:** a forgatókönyv alapja az előrejelzés ügyfélátlaga (nem fix áras számlák) és az áttekintés havi órái; a havi eredmény minden hónapra azonos, a szerződések lejárata nem hat rá; az óraváltozás a naplózott órákra vonatkozik, ár nélkül a bevétel arányosan skálázódik; az `after_costs` tételek nem halmozódnak és negatív eredményre 0-t adnak; a kapacitás kötelező és pozitív; nincs szerver-oldali cache (a felület 400 ms-ot vár); a paraméterkészletek kezelése a Forgatókönyvek fülről nyíló ablakban van, a Beállítások fül és a sablonok a 4. szakaszban jönnek.
- **Forgatókönyv korlátai:** fix áras ügyfél csak új ügyfélként modellezhető; a levelezési és megbeszélési órák nem módosíthatók ügyfélenként; az eredmény nem adótanács, és a rendszer nem tartalmaz adószabályt.
```

- [ ] **Step 6: Commit** (csak kérésre)

```bash
git add docs/superpowers/specs/2026-10-01-profitability-design.md
git commit -m "docs(profitability): record stage 3 scenario decisions"
```

---

## Self-Review

**Spec-lefedettség (csak a 3. szakasz):**
- `profit_parameter_sets` (`name`, `percent_items` `[{label, percent, base}]`, `fixed_monthly_costs`) → Task 1–2 ✔
- `profit_scenarios` (`horizon_months` CHECK 3–6, `parameter_set_id` `ON DELETE RESTRICT`, `capacity_hours_per_month`, `client_adjustments`, `new_clients`, `created_by`, időbélyegek) → Task 1–2 ✔
- Hivatkozási szabályok: a készlet nem törölhető, amíg forgatókönyv hivatkozik rá (`409` + FK) → Task 5 ✔; a forgatókönyv hivatkozik a készletre (nem másol) → Task 2/6 ✔; törölt ügyfélre mutató módosítás kihagyva figyelmeztetéssel → Task 3 (`known`, figyelmeztetés) ✔
- Motor: `Compute(alapadat, forgatókönyv, paraméterkészlet, beállítások)` → `BuildScenario`; havi bevétel, szükséges órák és kapacitás-túllépés, százalékos tételek a `base` szerint, fix költségek, nettó nyereség, a kapacitásórára vetített nettó nyereség → Task 3 ✔
- „Mivel számolt” blokk → `Basis` (Task 3) és a panel (Task 9) ✔
- Végpontok: `GET/POST/PUT/DELETE /parameter-sets`, `GET/POST/PUT/DELETE /scenarios`, `POST /scenarios/compute` → Task 5–6 ✔
- Felület: Forgatókönyvek fül, szerkesztő ügyfélenkénti mezőkkel és kapacitással, élő eredménnyel → Task 8–10 ✔
- Tesztek: áremelés, kiesés, új ügyfél, kapacitás-túllépés, mindkét `base`, üres készlet, 0 óra, törölt ügyfélre mutató bejegyzés → Task 3 ✔; „megújítás a horizont szélén” a 2. szakaszban lefedett; itt a lapos modell miatt nem releváns (Döntés 1).
- **Nem része (a 4. szakaszra marad):** 2–3 forgatókönyv egymás melletti összehasonlítása, a sablonok, a Beállítások fül.
- **A spec „GET /clients/:id/meeting-allowance” és hasonló pontjaihoz nem nyúlunk.**

**Eltérések a spec szövegétől (a felhasználónak jelezni):**
1. A spec szerint az alapadat egyszer töltődik be, és a csúszkák csak a motort hívják. Itt minden `compute` hívás újratölti az alapot (nincs cache), a kliens pedig 400 ms-ot vár (Döntés 7).
2. A forgatókönyv-eredmény **lapos** a horizonton (Döntés 1); a spec „megújítás a horizont szélén” esete itt nem módosítja a bevételt.
3. A paraméterkészlet-kezelő ablak a Forgatókönyvek fülön van, nem a Beállítások fülön (Döntés 8).
4. A JSONB-oszlopokhoz új, általános `JSONList[T]` típus készül (Döntés 9).

**Placeholder-ellenőrzés:** nincs TBD/TODO; minden kódlépésben teljes kód van. A Task 10 3. lépése a `page.tsx` három pontos módosítását utasításként adja (nem szó szerinti kódként), mert a fájl az előző szakaszok óta többször változott; ott az implementernek előbb el kell olvasnia a fájlt.

**Típus-konzisztencia:** a Go JSON-címkék (`ScenarioResult`: `months`, `monthly`, `baseline`, `horizon`, `baseline_horizon`, `clients`, `low_data`, `warnings`, `basis`; `ScenarioMonth`: `revenue`, `fixed_costs`, `revenue_items`, `result_after_costs`, `after_costs_items`, `net_profit`, `required_hours`, `capacity_hours`, `utilization`, `overloaded`, `net_profit_per_capacity_hour`; `ScenarioClientResult`: `client_id`, `name`, `is_new`, `dropped`, `base_revenue`, `base_hours`, `revenue`, `hours`; `ScenarioBasis`: `parameter_set_name`, `percent_items`, `fixed_monthly_costs`, `capacity_hours`, `baseline_months`, `horizon_months`) megegyeznek a TS `ScenarioResult` / `ScenarioMonth` / `ScenarioClientResult` mezőivel. A kérés-típusok (`ProfitScenarioRequest`, `ProfitParameterSetRequest`) JSON-címkéi megegyeznek a TS `ScenarioInput` / `ParameterSetInput` mezőivel. A függvénynevek (`BuildScenario`, `LoadScenarioBase`, `ComputeScenario`, `validateParameterSetRequest`, `validateScenarioRequest`, `parameterSetExists`) minden taskban azonosak. A tesztek az `approx` segédet a meglévő `profitability_calc_test.go`-ból veszik át (nem definiálják újra); az `f64` segéd a `profitability_scenario_test.go`-ban új, ütközés esetén (a csomagban már van ilyen nevű függvény) nevezd át `ptrF64`-re az egész fájlban.

**Nem ellenőrzött feltevések:**
- A `Button` `size="sm"`, a `Modal` `size="lg"`, a `text-success` osztály és az `Input` szám-tulajdonságai a Task 8–10 első lépéseiben ellenőrizendők; a terv ezt kifejezetten előírja.
- A `JSONList.Value` szövegként (`string`) adja át a JSON-t a `jsonb` oszlopnak; ezt a Task 6 7. lépése valódi GORM-mal, visszagörgetett tranzakcióban igazolja. Ha ott típushibát ad, a terv szerint meg kell állni.
- A dev adatbázisban nincs rendszeres számla, ezért az alapadat nem üres útja csak egységtesztekkel igazolt, valós adaton nem.
- A `apiClient` hibaüzenet-kezelése (a `409` szövegének eljutása a felületig) a Task 7 1. lépésében ellenőrizendő.
