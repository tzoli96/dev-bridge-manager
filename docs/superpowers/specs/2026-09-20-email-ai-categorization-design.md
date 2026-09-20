# AI E-mail Kategorizálás — Design

## Cél

A Beérkezett mappába érkező e-mailek automatikus kategorizálása, hogy a
felhasználó gyorsan átlássa és szűrhesse a levelezését.

## Kategóriák

Rögzített, 5 elemű halmaz (nem bővíthető UI-ból):

| Kulcs (backend/AI)  | Magyar címke (UI)     |
|---------------------|------------------------|
| `ugyfel`             | Ügyfél                |
| `szamla`             | Számla / pénzügy      |
| `marketing`          | Hirdetés / marketing  |
| `rendszeruzenet`     | Rendszerüzenet        |
| `egyeb`              | Egyéb                 |

## Hatókör

- Csak a **Beérkezett (inbox)** mappa leveleire fut le. A Küldött mappa
  nem kap kategóriát.
- Csak **új** leveleken fut le, a szinkronizáció pillanatában (mind
  `backfillAccount`, mind a folyamatos történet-alapú szinkron ugyanazon
  a ponton megy át: `applyAddedMessages`). Meglévő, már betöltött
  levelekre **nem** futtatjuk vissza visszamenőleg — ez explicit döntés,
  nem YAGNI-vágás: a visszamenőleges kategorizálás külön feature lenne
  (batch-elés, rate limit kezelés más karakterű), ha később kell.
- Ha az AI hívás hibázik vagy time-outol, a levél mentése **nem** áll
  meg emiatt — a `category` mező `NULL` marad, ugyanaz a "logol és megy
  tovább" minta, mint a `GetMessageMetadata` hívásnál ma is
  (`gmail_sync.go:151`).

## Architektúra

```
Gmail API ──> applyAddedMessages (gmail_sync.go)
                    │
                    ├─ meta (subject, snippet, from) érkezik, mint ma
                    │
                    ├─ [ÚJ] ha meta.Folder == "inbox":
                    │     CategorizeEmail(subject, snippet, from) ──HTTP──> ai service
                    │                                                        │
                    │     (hiba/timeout esetén category = nil)      /categorize-email
                    │
                    └─ db.Create(&models.Email{..., Category: category})
```

Az AI service (`ai/app/main.py`, FastAPI + pydantic-ai + Gemini) kap egy
új endpointot. A backend egy sima szinkron HTTP híváson keresztül
kommunikál vele a docker hálózaton belül (`http://ai:8000`), ugyanúgy,
ahogy a `billingo_service.go` is HTTP-n hívja a Billingo API-t —
meglévő mintát követünk, nem vezetünk be új infrastruktúra-komponenst
(queue, worker) egyetlen szinkron hívásért.

## Komponensek

### 1. AI service — `POST /categorize-email`

**Kérés:**
```json
{ "subject": "...", "snippet": "...", "from_address": "...", "from_name": "..." }
```

**Válasz:**
```json
{ "category": "ugyfel" }
```

pydantic-ai `Agent` `output_type`-ja egy Pydantic modell, amelynek
`category` mezője `Literal["ugyfel", "szamla", "marketing",
"rendszeruzenet", "egyeb"]` — a modell strukturálisan nem tud érvénytelen
értéket visszaadni. Az agent a meglévő `GEMINI_MODEL` env varral
konfigurált Gemini modellt használja, ugyanúgy, mint a jövőbeli többi AI
funkció is fogja (nincs külön config ehhez a feature-höz).

Hibakezelés az AI service oldalán: ha a Gemini hívás hibázik, a végpont
`500`-at ad vissza (nincs speciális retry/fallback logika az AI service
szintjén — a hívó, azaz a backend dönt, hogyan reagál).

### 2. Backend — `services/email_categorization.go` (új fájl)

```go
type EmailCategorizationService struct {
    httpClient *http.Client
    baseURL    string // AI_SERVICE_URL env var, default "http://ai:8000"
}

func (s *EmailCategorizationService) Categorize(ctx context.Context, subject, snippet, fromAddress, fromName string) (*string, error)
```

- Timeout: 10s (a `billingo_service.go` 15s-os klienséhez hasonló
  nagyságrend, de rövidebb, mert ez egy szinkron szinkronizációs
  útvonalba ékelődik be és nem szabad hosszan blokkolnia a Gmail
  szinkront).
- Hiba vagy nem 200-as válasz esetén `nil, err`-t ad vissza — a hívó
  (`applyAddedMessages`) ezt logolja és `NULL` kategóriával menti a
  levelet, nem állítja le a szinkront.
- `AI_SERVICE_URL` env var, docker-compose-ban a backend service
  environment blokkjába kerül (`AI_SERVICE_URL: http://ai:8000`),
  ugyanígy ahogy a `GEMINI_MODEL` is env varként van átadva az `ai`
  service-nek.

### 3. `gmail_sync.go` — `applyAddedMessages` módosítása

A meglévő ciklusban, közvetlenül a `meta` sikeres lekérése után, mielőtt
a `db.Create`-et meghívjuk:

```go
var category *string
if meta.Folder == "inbox" {
    cat, err := categorizationService.Categorize(ctx, meta.Subject, meta.Snippet, meta.FromAddress, meta.FromName)
    if err != nil {
        log.Printf("gmail sync: failed to categorize message %s: %v", meta.GmailMessageID, err)
    } else {
        category = cat
    }
}

db.Create(&models.Email{
    ...,
    Category: category,
})
```

`categorizationService` a jelenlegi `SyncAccount`/`backfillAccount`
hívási lánc mentén jut el `applyAddedMessages`-be (paraméterként, nem
globális singleton — így a teszt is tud mockot injektálni, ugyanúgy,
ahogy a `GmailAPI` interfész is ma).

### 4. Adatmodell

**Migráció** `000029_add_email_category.up.sql`:
```sql
ALTER TABLE emails ADD COLUMN category VARCHAR(20);
```
(nullable, nincs default — a régi sorok `NULL`-ok maradnak, ez a
szándékolt "nincs visszamenőleges kategorizálás" viselkedés.)

**`models.Email`** (`backend/internal/models/email.go`):
```go
Category *string `json:"category" gorm:"size:20"`
```

**`models.EmailListItem`**:
```go
Category *string `json:"category"`
```

`EmailHandler.ListEmails` (`email_handler.go:37`) kap egy opcionális
`?category=` query paramot:
```go
category := c.Query("category")
if category != "" {
    db = db.Where("category = ?", category)
}
```
A queryt csak validáljuk a fenti 5 kulcs egyikeként (400, ha más), a
`folder`/`page` melletti meglévő mintát követve.

### 5. Frontend

**`emailsService.ts`:**
- `EmailListItem` kap egy `category: string | null` mezőt.
- `list(folder, page, category?)` — a `category` opcionális extra query
  param, ugyanúgy, mint a `folder`/`page` ma.

**`emails/page.tsx`:**
- Kis színes badge a tárgy sora mellett, hasonló mintát követve, mint az
  `avatarColor()` — de itt fix szín-hozzárendelés kategóriánként (nem
  hash-alapú), mert csak 5 fix érték van:
  ```ts
  const CATEGORY_STYLES: Record<string, { label: string; className: string }> = {
      ugyfel: { label: 'Ügyfél', className: 'bg-blue-500/10 text-blue-600' },
      szamla: { label: 'Számla', className: 'bg-emerald-500/10 text-emerald-600' },
      marketing: { label: 'Marketing', className: 'bg-amber-500/10 text-amber-600' },
      rendszeruzenet: { label: 'Rendszer', className: 'bg-slate-500/10 text-slate-600' },
      egyeb: { label: 'Egyéb', className: 'bg-muted text-muted-foreground' },
  }
  ```
  `NULL` kategóriájú (pl. régi vagy sikertelen AI-hívású) levélnél nem
  jelenik meg badge.
- Szűrő pill-sor a Beérkezett lista fölött (csak `folder === 'inbox'`
  esetén jelenik meg — a Küldött mappának nincs kategóriája): "Mind" +
  az 5 kategória, kattintásra újratölti a listát a kiválasztott
  `category`-vel. Ugyanaz a fetch-újraindítási minta, mint a
  `folder`-váltásnál ma (`useEffect` a `[status?.connected, folder,
  category]` dependency-vel).

## Tesztelés

- **Backend:** unit teszt `email_categorization_test.go`-ban
  `httptest.Server`-rel mockolt AI service válaszra (siker + hiba +
  timeout eset), és hogy a `Categorize` hiba esetén `nil, err`-t ad
  vissza, nem panicol.
- **`gmail_sync_helpers_test.go` / meglévő sync teszt:** hogy sent
  mappájú levélnél a kategorizáló szolgáltatás **nem** hívódik meg, és
  hogy AI-hiba esetén a levél mégis létrejön `category = NULL`-lal (a
  szinkron nem szakad meg).
- **AI service:** pytest a `/categorize-email` endpointra, ami
  ellenőrzi, hogy a válasz `category` mezője mindig az 5 engedélyezett
  érték egyike (Gemini hívás mockolva/`TestModel`-lel, a pydantic-ai
  saját teszt-modeljével, hogy ne hívjunk éles Geminit a teszt során).
- Build/lint ellenőrzés a dokumentált konténerekben: `go build ./...`,
  `go vet ./...`, backend `go test ./...`, frontend `tsc --noEmit`, AI
  service `pytest`.

## Nem célja ennek a feature-nek

- Visszamenőleges kategorizálás meglévő leveleken.
- Kategória kézi felülbírálása a felhasználó által (ha ez felmerül,
  külön feature/spec).
- A Küldött mappa kategorizálása.
