# Álláskeresés + AI-matching — Design

## Cél

Automatikusan figyelje a profession.hu és a nofluffjobs.com álláshirdetéseit,
pontozza őket a felhasználó (kizárólag a super_admin szerepkörű üzemeltető)
CV-je és skilljei alapján, és tegye lehetővé, hogy a legjobban illeszkedő
hirdetésekre AI-generált kísérőlevél-tervezettel gyorsan lehessen
jelentkezni. Ez egy tisztán személyes, egyfelhasználós funkció — nem az
ügyfélkezelő rendszer más felhasználóinak szól.

## Hatókör és nem-célok

- **Csak profession.hu és nofluffjobs.com** kerül automatikusan
  scrapelésre ebben a körben. A LinkedIn kifejezetten kizárt: a
  felhasználói szerződése tiltja az automatizált scrapelést/botokat, és
  agresszív bot-detektálása van (CAPTCHA, IP-tiltás, fiók-felfüggesztés
  kockázata). LinkedIn-integráció egy **külön, jövőbeli spec** tárgya
  lesz, ha egyáltalán megvalósul.
- **Nincs teljesen automatizált jelentkezés-beküldés** (nincs headless
  böngésző / form-kitöltő automatizálás egyik site-on sem). A rendszer
  AI-jal elkészíti a kísérőlevél-tervezetet, de a tényleges beküldés
  vagy egy általunk küldött emailen keresztül, vagy a felhasználó saját
  kattintásával történik a hirdetés oldalán.
- **Nincs strukturált szűrőmező** (külön location/remote/fizetés mező) a
  profilban — a CV, a skillek és a szabad szöveges preferenciák mind
  szabad szövegként kerülnek az AI-matching promptjába, ugyanúgy, ahogy a
  meglévő `Profile.Background`/`Expertise` is szabad szöveg.
- **Nincs kulcsszavas keresés** a scraping oldalán — mindkét scraper a
  site saját IT/szoftverfejlesztés kategória-listázóját járja végig, a
  tényleges illeszkedést kizárólag az AI-matching lépés dönti el a teljes
  CV alapján.
- A funkció kizárólag `super_admin` szerepkörhöz kötött, ugyanúgy, ahogy a
  meglévő `Profile` (AI-perszóna) oldal — nincs admin-fallback.

## Architektúra

```
[Scheduler (time.Ticker) / "Keresés most" gomb]
        │
        ▼
[Go: JobScrapingService.RunScrape(ctx)]
        │  minden regisztrált JobScraper-re (ProfessionHuScraper,
        │  NoFluffJobsScraper) egymástól függetlenül:
        │    - robots.txt betartása
        │    - IT/szoftverfejlesztés kategória-lista lapozott bejárása
        │    - lapozás megáll N egymást követő már ismert URL-nél
        ▼
[Postgres: job_listings]  (INSERT ... ON CONFLICT (site, external_url) DO NOTHING)
        │
        ▼ (job_matches nélküli sorokra)
[Go → AI service: POST /job-match]
        ▼
[Python: job_match.py — pydantic_ai agent]
        │  CV + skillek + preferenciák vs. hirdetés → score (0-100) + indoklás
        ▼
[Postgres: job_matches]  (score, reasoning, status)
        │
        ▼
[Frontend: /dashboard/admin/job-search — rangsorolt lista]
        │  kiválasztott hirdetésre "Jelentkezés tervezete" gomb
        ▼
[Go → AI service: POST /job-application-draft]
        ▼
[Python: job_application_draft.py — pydantic_ai agent]
        │  kész, szerkeszthető kísérőlevél-szöveg
        ▼
[Frontend modal: szerkeszthető szöveg]
        ├─ "Küldés emailben" → meglévő Gmail-küldés integráció → job_matches.status = applied
        └─ "Hirdetés megnyitása" (external_url) + "Megjelölés jelentkezettként" gomb
```

Ugyanaz a 3-rétegű minta, amit a draft-reply és a task-breakdown
feature-öknél is használtunk: Python AI agent (statikus `instructions` +
per-kérés prompt-építés) + Go service (HTTP kliens az `AI_SERVICE_URL`
felé, interfész-seam a teszteléshez) + handler + route + frontend service
+ UI.

## Adatmodell

Új migráció `000032_add_job_search` (a `000031_add_draft_feedback` a
jelenleg legutóbbi — ellenőrizni kell ismét `ls backend/migrations | sort
-V | tail -5`-tel a plan végrehajtásakor, hátha közben új migráció került
be):

```sql
-- up
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

`job_search_profiles` szándékosan singleton (`id=1`), ugyanúgy ahogy a
`profiles` tábla — a `INSERT ... VALUES (1, 1)` a migrációban hozza létre
az egyetlen sort (`updated_by=1` a rendszer első felhasználójára mutat,
ugyanaz a minta, amit a `profiles` tábla `000030` migrációja is használt).

Go modellek (`backend/internal/models/job_search.go`):
```go
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

`status` érvényes értékei: `"new"`, `"reviewed"`, `"dismissed"`,
`"applied"` — Go oldalon `const`-ok, nem külön enum-tábla (YAGNI, a
`draft_feedback`/`emails` táblák is sima string-mezőket használnak
státuszra/kategóriára).

## Scraping komponens (Go)

Új csomag: `backend/internal/services/jobscraper/` (a Go konvenció miatt
külön alcsomag, mert több fájlból áll: interfész + 2 implementáció + közös
robots.txt/HTTP segédkód — a `services` csomag már ma is nagy, ezt nem
duzzasztjuk tovább).

```go
package jobscraper

type ScrapedJob struct {
    ExternalURL string
    Title       string
    Company     string
    Location    string
    Description string
    PostedAt    *time.Time
}

type Scraper interface {
    Site() string
    Scrape(ctx context.Context) ([]ScrapedJob, error)
}
```

- `ProfessionHuScraper` és `NoFluffJobsScraper` mindketten a `goquery`
  library-t használják a HTML-parsoláshoz (**új, direkt függőség** — a
  backendben jelenleg semmilyen HTML-parser nincs, kézzel regex-szel
  parsolni HTML-t törékeny és rossz gyakorlat, erre nincs kiváltó meglévő
  mechanizmus).
- A HTML-letöltés és a DOM-ból való kinyerés (`ScrapedJob` mezőnkénti
  CSS-szelektorok) két külön, tiszta függvénybe kerül site-onként:
  `fetchListingPages(ctx) ([]string, error)` (nyers HTML lapok) és
  `parseListingPage(html string) ([]ScrapedJob, error)` (tiszta
  függvény — ez teszi lehetővé a hálózat nélküli, fixture-alapú tesztet).
- Lapozás: legfeljebb 10 oldal per futás, és megáll, ha 1 oldalon belül
  minden hirdetés `external_url`-je már szerepel a `job_listings`
  táblában (a `RunScrape` egy `map[string]bool` már-ismert-URL halmazt ad
  át a scraper-eknek induláskor, egy `SELECT external_url FROM
  job_listings WHERE site = ?` lekérdezésből).
- **robots.txt**: `backend/internal/services/jobscraper/robots.go` —
  letölti a domain `/robots.txt`-jét, egyszerű soronkénti
  `User-agent: *` / `Disallow:` prefix-egyezést csinál (nincs szükség
  külön robots-parser library-ra, ez a szabály-forma triviálisan
  kézzel is parsolható). Ha egy útvonal tiltott, a scraper kihagyja.
- User-Agent: egy explicit, azonosítható string (pl.
  `"DevBridgeManager-JobSearch/1.0 (personal use, contact: <email>)"`),
  kérések között 1.5 mp késleltetés (`time.Sleep`) — nem próbálunk
  bot-detektálást megkerülni, csak udvariasan, azonosíthatóan járunk el.
- `RunScrape(ctx)`: sorban végigmegy a regisztrált scraper-eken; ha
  az egyik `Scrape` hibázik (pl. megváltozott HTML-szerkezet), a hiba
  logolásra kerül (`log.Printf`), a másik scraper attól még lefut —
  ugyanaz a hibatűrési minta, mint a `RunGmailSync`-ban az accountonkénti
  hibakezelés.
- Minden site-ból kapott új `ScrapedJob`-ot `INSERT ... ON CONFLICT (site,
  external_url) DO NOTHING`-gal ment be a `job_listings` táblába.
- Scheduler: `backend/internal/services/job_scraping_scheduler.go` —
  pontosan a `StartGmailSyncScheduler` mintája (`time.Ticker`, nincs cron
  függőség), napi 1 ciklus (`24 * time.Hour`), induláskor egyszer lefut.
  `main.go`-ban: `go services.StartJobScrapingScheduler()`.
- Pontozás: `RunScrape` a scrapelés után lekérdezi a `job_matches`
  nélküli `job_listings` sorokat (`LEFT JOIN`), és mindegyikre sorban
  meghívja a `JobMatchService`-t (lásd lent), elmenti az eredményt. Nincs
  queue: a napi hirdetésmennyiség (becsülve tucat-száz db) simán elfér
  szinkron, egymás utáni AI-hívásokban egy háttérfolyamatban.

## AI-matching (`ai/app/job_match.py`)

```python
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
```

- `job_match_agent` — `pydantic_ai.Agent`, saját `GEMINI_MODEL` env-olvasás
  (ugyanaz az indoklás, mint a `draft_reply.py`/`categorize_email.py`
  meglévő kommentjében: elkerüli a kereszt-importot).
- `instructions`: értékelje 0-100 skálán, mennyire illik a jelölt CV-je és
  skilljei az adott hirdetéshez, vegye figyelembe a preferenciákat is (ha
  meg vannak adva), adjon 1-2 mondatos magyar indoklást. A score legyen
  következetes: 0-20 egyáltalán nem releváns, 40-60 részleges egyezés,
  80-100 erős egyezés.
- Végpont `ai/app/main.py`-ban: `POST /job-match`, ugyanaz a minta, mint a
  meglévő `/draft-reply`/`/task-breakdown` végpontoknál.

## Jelentkezés-tervezet (`ai/app/job_application_draft.py`)

```python
class JobApplicationDraftRequest(BaseModel):
    cv_text: str
    skills: str
    job_title: str
    company: str
    job_description: str
    instruction: str = ""

class JobApplicationDraftResult(BaseModel):
    draft: str
```

- Külön agent (nem a `draft_reply_agent` bővítése), mert ez más célú
  szöveg: motivációs/kísérőlevél egy állásra, nem üzleti email-válasz.
- `instructions`: írjon udvarias, magyar nyelvű, kész (nem placeholder)
  kísérőlevél-tervezetet a megadott CV/skillek és a hirdetés alapján; ha
  `instruction` mező ki van töltve, azt vegye figyelembe konkrét
  hangsúlyként (pl. "emeld ki a Go tapasztalatot").
- Végpont: `POST /job-application-draft`.

## Go service-réteg

`backend/internal/services/job_match.go` és
`backend/internal/services/job_application_draft.go` — mindkettő
pontosan a `DraftReplyService` mintáját követi: interfész-seam
(`JobMatcher`, `JobApplicationDrafter`), `AI_SERVICE_URL`-t olvasó
konstruktor, JSON marshal/HTTP POST/unmarshal, hosszabb (30s) timeout,
mert generatív AI-hívásokról van szó.

## Admin API (`super_admin`-only)

Új fájlok: `backend/internal/handlers/job_search_handler.go`,
`backend/internal/routes/job_search_routes.go`. A route-regisztráció
pontosan a `profile_routes.go` mintáját követi: `/admin` csoport,
`middleware.JWTMiddleware()`, majd **route-onként** (nem
`admin.Use()`-zal, hogy ne szivárogjon más `/admin/*` route-okra)
`middleware.RequireRole("super_admin")`.

- `GET /admin/job-search/profile` — a `job_search_profiles` id=1 sor
- `PUT /admin/job-search/profile` — CV/skillek/preferenciák mentése
  (ugyanaz a "teljes felülírás" minta, mint `UpdateProfile`)
- `POST /admin/job-search/scan-now` — szinkron meghívja `RunScrape`-et,
  visszaadja, hány új hirdetés/match született (a válasz megvárja a
  futást — ha ez a gyakorlatban túl lassúnak bizonyul a HTTP-timeouthoz
  képest, ez egy implementációs döntés, amit a plan végrehajtásakor kell
  finomítani, pl. `context.WithTimeout`-tal)
- `GET /admin/job-search/matches?status=` — rangsorolt lista (`ORDER BY
  score DESC`), opcionális `status` szűréssel, `Preload("JobListing")`
- `PATCH /admin/job-search/matches/:id/status` — `reviewed`/`dismissed`
  beállítása
- `POST /admin/job-search/matches/:id/draft-application` — meghívja a
  `JobApplicationDrafter`-t, **nem ment semmit**, csak visszaadja a
  szöveget szerkesztésre (ugyanaz a "csak generál, nem perzisztál" minta,
  mint a `BreakdownEmailIntoTasks`-nál)
- `POST /admin/job-search/matches/:id/mark-applied` — `status='applied'`,
  `applied_at=now()`, `application_text` elmentése a kérésből kapott
  (esetlegesen a felhasználó által szerkesztett) szöveggel — ezt hívja
  mind az "emailben küldve", mind a "manuálisan jelentkeztem a site-on"
  ág, miután a tényleges küldés/beküldés megtörtént

Az "emailben küldés" maga **nem új endpoint** — a frontend a meglévő
`/emails/send` végpontot hívja (cél email-cím + a szerkesztett
kísérőlevél-szöveg body-ként), majd sikeres küldés után hívja a
`mark-applied` endpointot. Nincs szükség arra, hogy a job-search backend
tudjon Gmailt küldeni — ez a meglévő email-küldés flow tiszta
újrafelhasználása.

## Frontend

- `AdminTab.tsx` `adminCards` tömbje kap egy új elemet:
  `{ title: "Álláskeresés", requireSuperAdmin: true, action: () =>
  router.push('/dashboard/admin/job-search') }` — ugyanaz a
  `requireSuperAdmin` minta, mint az "AI Profil / Perszóna" kártyánál,
  csak modal helyett külön oldalra navigál (a felület mérete miatt: CV-
  szerkesztő + lista + jelentkezés-modal nem fér el egy modalban).
- Új oldal: `frontend/src/app/dashboard/admin/job-search/page.tsx`
  - CV/skillek/preferenciák szerkesztő szekció (3 textarea + Mentés gomb,
    ugyanaz a state/hívás-minta, mint a `ProfileModal`-ban)
  - "Keresés most" gomb (`scan-now` hívás, loading állapot, majd a lista
    frissítése)
  - Rangsorolt lista: cím, cég, helyszín, score-badge (színkódolt: pl.
    zöld ≥70, sárga 40-69, szürke <40), rövid `reasoning`, státusz-badge,
    "Hirdetés megnyitása" link, "Jelentkezés" gomb, "Elutasítás" gomb
  - Jelentkezés modal: megnyitáskor lekéri a `draft-application`
    végpontot, szerkeszthető `<textarea>`-ba tölti (ugyanaz a minta, mint
    a `CreateTaskFromEmailModal`-ban), két gomb: "Küldés emailben" (email
    cím input + `EmailsService.send`, majd `mark-applied`) és "Hirdetés
    megnyitása" (`window.open(external_url)`) + külön "Megjelölés
    jelentkezettként" gomb a `mark-applied`-hoz
- Új service: `frontend/src/services/jobSearchService.ts` — a meglévő
  `apiClient` mintát követi (40000ms timeout az AI-hívásoknál, mint a
  `draftReply`/`breakdownIntoTasks`-nál).

## Tesztelés

A projektben nincs DB-alapú/integrációs teszt-infrastruktúra — ezt itt
sem vezetjük be. Tiszta függvények kapnak unit tesztet, DB-t érintő
handlerek nem (ugyanaz a konvenció, mint a `draft_reply_test.go` és
`email_handler_test.go` esetén).

- **`parseListingPage`** mindkét scraper-nél: mentett HTML-fixture
  fájlokkal (`testdata/professionhu_sample.html`,
  `testdata/nofluffjobs_sample.html`) — assert a kinyert `ScrapedJob`
  mezőkre. Ha egy site megváltoztatja a struktúráját, csak ez a teszt
  bukik, azonnal látszik melyik scraper romlott el.
- **robots.txt parser**: unit teszt (tiltott/engedélyezett útvonalak,
  hiányzó `robots.txt`).
- **`job_match.py`/`job_application_draft.py`**: `pytest` +
  `TestModel`, a `test_draft_reply.py` mintája szerint — prompt-építés
  (CV/skillek/preferenciák szerepelnek-e a promptban) és a végpontok
  sikeres válasza mockolt AI-kimenettel.
- **`job_match.go`/`job_application_draft.go` Go HTTP-kliensek**: a
  `draft_reply_test.go` mintájával megegyező request/response
  szerializációs teszt.
- **Frontend**: `tsc --noEmit`, manuális böngészős ellenőrzés kérve a
  felhasználótól (nincs böngésző-automatizálási eszköz).

## Globális megkötések (a plan minden feladatára érvényes)

- Nincs GORM `AutoMigrate` — minden séma-változás `golang-migrate`
  fájlokon keresztül (`backend/migrations/`); a `000032`-es szám
  lefoglalása előtt ellenőrizni kell (`ls backend/migrations | sort -V |
  tail -5`), hogy még szabad-e.
- A `goquery` egy új, direkt Go-függőség (`go get
  github.com/PuerkitoBio/goquery`) — ez tudatosan bevezetett kivétel a
  "ne vezess be új függőséget" szabály alól, mert HTML-scrapinghez nincs
  meglévő mechanizmus a kódbázisban, és a kézi regex-parsolás
  objektíven rosszabb/törékenyebb megoldás lenne.
- A robots.txt-parsoláshoz **nem** vezetünk be külön library-t — a
  szabályforma elég egyszerű a kézi implementációhoz.
- Az AI service minden pydantic-ai `Agent`-je saját `GEMINI_MODEL`
  env-változót olvas (nincs kereszt-import más agent modulból) — ugyanaz
  az indoklás, mint a `categorize_email.py`/`draft_reply.py` meglévő
  kommentjében.
- Minden Go backend parancsot `docker exec devbridge_backend ...`,
  minden AI-service Python parancsot `docker exec devbridge_ai ...`,
  minden frontend parancsot `docker exec devbridge_frontend ...`
  konténerben futtatunk, sosem a hoston.
- A `/admin/job-search/*` route-ok kizárólag `super_admin`-nak
  érhetők el, route-onként (nem `admin.Use()`-zal) — ugyanaz a minta és
  ugyanaz az indoklás, mint a `profile_routes.go`-ban: ne szivárogjon a
  jogosultság más `/admin/*` route-okra.
- Nincs teljesen automatizált jelentkezés-beküldés (headless böngésző,
  form-automatizálás) egyik site-on sem, és a LinkedIn semmilyen
  formában nem kerül automatizált scrapelésre/jelentkezésre ebben a
  körben.
- A scraper-ek betartják a `robots.txt`-t, azonosítható User-Agent-tel és
  kérések közti késleltetéssel futnak — nem próbálunk semmilyen
  bot-detektálást megkerülni.
