# Felhasználói Profil / Perszóna — Design

## Cél

Egy megosztott, központi "ki vagyok én" profil (szakmai háttér,
szakterület, kommunikációs stílus, írásminták), amit **bármelyik** AI
funkció felhasználhat, amikor a felhasználó nevében fogalmaz meg
szöveget (pl. email-válasz javaslat, később állásjelentkezés-generátor
vagy más agent). A profil egyszer kerül karbantartásra egy admin
felületen, és minden jövőbeli AI funkció ugyanazt a forrást olvassa —
nincs duplikált "ki vagyok én" logika funkciónként.

Ez a spec csak magát a profil-alrendszert (tárolás, admin CRUD, belső
lekérdező végpont) és az első fogyasztóját (email-válasz-javaslat)
fedi le. Jövőbeli agentek ugyanezt a belső végpontot fogják használni,
saját spec-jük szerint.

## Hatókör és nem-célok

- **Egyetlen, globális profil** létezik (singleton, `id = 1`), nem
  per-felhasználó. A rendszer több `User`/`Role`-t ismer, de ez a
  profil kifejezetten a super-admin (a rendszer üzemeltetőjének)
  személyes adata — ahogy a `BillingoSettings` is egyetlen globális
  konfiguráció, nem per-user.
- **Csak `super_admin` szerepkör** olvashatja/szerkesztheti admin
  felületen keresztül — explicit, szigorúbb, mint a repóban megszokott
  "admin vagy super_admin" fallback minta (pl.
  `billingo_settings_handler.go:33`), mert ez a profil személyes
  háttér- és stílusadatot tartalmaz, amit a felhasználó kifejezetten
  csak magának akar fenntartani.
- **Nem cél** ebben a körben: konkrét fogyasztó agentek (állásjelentkezés-
  generátor stb.) megépítése — csak az email-válasz-javaslat épül meg
  első fogyasztóként, hogy a belső végpont bizonyítottan működjön.
- **Nem cél:** automatikus küldés bármilyen csatornán. Minden AI által
  generált szöveg csak javaslat — a tényleges elküldés mindig emberi
  jóváhagyással történik.
- **Nem cél:** a profil verziózása/története. Egy `UpdatedAt`/`UpdatedByID`
  mező elég a "ki és mikor módosította utoljára" nyomon követésére,
  nincs teljes audit log vagy visszaállítás korábbi verzióra.

## Architektúra

```
Admin UI (super_admin) ──PUT/GET──> Go backend: ProfileHandler
                                          │
                                          ├─ Profile + ProfileSample (GORM, singleton id=1)
                                          │
                                          └─ GET /api/v1/internal/profile-context
                                                  │
                                                  ▼
                              AI service (ai/app) — draft-reply endpoint
                                          │
                                          ▼
                              Gemini (pydantic-ai Agent), few-shot promptban
```

A profil a Go backendben él (ahol a `User`/`Role` jogosultságkezelés
már megvan), nem az AI service-ben — így a super-admin-ellenőrzés nem
duplikálódik egy második szolgáltatásban. Az AI service "buta"
végrehajtó marad, ugyanúgy, mint a `/categorize-email` végpontnál:
kap egy már összeállított szöveges kontextust, és nem tud semmit a
jogosultságokról.

## Komponensek

### 1. Adatmodell — `backend/internal/models/profile.go` (új fájl)

```go
type Profile struct {
    ID          uint            `json:"id" gorm:"primaryKey"`
    Background  string          `json:"background" gorm:"type:text"`
    Expertise   string          `json:"expertise" gorm:"type:text"`
    ToneRules   string          `json:"tone_rules" gorm:"type:text"`
    Samples     []ProfileSample `json:"samples" gorm:"foreignKey:ProfileID"`
    UpdatedByID uint            `json:"updated_by_id"`
    UpdatedAt   time.Time       `json:"updated_at"`
}

func (Profile) TableName() string { return "profiles" }

type ProfileSample struct {
    ID        uint   `json:"id" gorm:"primaryKey"`
    ProfileID uint   `json:"profile_id" gorm:"not null;index"`
    Label     string `json:"label" gorm:"size:100"`
    Content   string `json:"content" gorm:"type:text"`
}

func (ProfileSample) TableName() string { return "profile_samples" }
```

Singleton konvenció: mindig `id = 1` sor (a `BillingoSettings` mintáját
követve — `database.GetDB().First(&profile, 1)`). Migráció seedeli az
üres `id=1` sort, hogy a `GET` sose fusson `record not found`-ba.

**Migráció** `NNNNNN_add_profile.up.sql` (a következő szabad sorszámon —
lásd "Migrációs sorszám" megjegyzés lent):
```sql
CREATE TABLE profiles (
    id BIGSERIAL PRIMARY KEY,
    background TEXT NOT NULL DEFAULT '',
    expertise TEXT NOT NULL DEFAULT '',
    tone_rules TEXT NOT NULL DEFAULT '',
    updated_by_id BIGINT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE profile_samples (
    id BIGSERIAL PRIMARY KEY,
    profile_id BIGINT NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
    label VARCHAR(100) NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT ''
);

INSERT INTO profiles (id, background, expertise, tone_rules) VALUES (1, '', '', '');
```

> **Migrációs sorszám:** a repóban jelenleg van egy folyamatban lévő,
> nem ehhez a feature-höz tartozó sorszám-ütközés (`000027`/`000028`
> még nincs commitolva, `000029` már commitolva). Ennek a feature-nek a
> migrációja a soron következő, ténylegesen szabad sorszámot kapja
> commitoláskor — ez a plan-írás/implementáció idején tisztázandó, nem
> ez a spec dönt róla.

### 2. Backend — admin CRUD végpontok

Új `backend/internal/handlers/profile_handler.go`, a meglévő
`/api/v1/admin/*` route-csoport mintáját követve (lásd
`billingo_settings_routes.go`: `admin := api.Group("/admin");
admin.Use(middleware.JWTMiddleware())`):

- `GET /api/v1/admin/profile` — teljes profil visszaadása (mezők +
  minták listája), admin UI tölti be szerkesztéshez.
- `PUT /api/v1/admin/profile` — `background`, `expertise`, `tone_rules`
  és a teljes `samples` lista frissítése egyben (a kliens mindig a
  teljes listát küldi, a backend törli a régi `ProfileSample` sorokat
  és beszúrja az újakat egy tranzakcióban — nincs külön CRUD minden
  egyes mintára, ahogy a Task 7 tervezésekor is a legegyszerűbb működő
  megoldást választottuk).

Jogosultság-ellenőrzés (`checkProfileAccess`), szigorúan csak
`super_admin`:
```go
func (h *ProfileHandler) checkProfileAccess(userID uint) error {
    user, err := h.permissionService.GetUserWithPermissions(userID)
    if err != nil {
        return fiber.NewError(500, "Error checking permissions")
    }
    if user.Role.Name != "super_admin" {
        return fiber.NewError(403, "Insufficient permissions")
    }
    return nil
}
```

Route regisztráció a meglévő admin-route csoport mintáját követve
(`routes/profile_routes.go`, az `/admin` csoportba regisztrálva,
`middleware.JWTMiddleware()`-rel védve, ahogy a többi admin végpont
is).

### 3. Backend — belső kontextus-végpont

`GET /api/v1/internal/profile-context` — **nem** admin-jogosultsághoz
kötött (nem ember hívja, hanem az AI service, docker belső hálózaton
keresztül, ugyanúgy ahogy a kategorizáló hívás megy backend → ai
irányban). Válasz:
```json
{ "context": "Háttér: ...\n\nSzakterület: ...\n\nKommunikációs stílus: ...\n\nÍrásminták:\n- [Címke]: szöveg\n- [Címke]: szöveg" }
```

A backend állítja össze ezt az egy, promptba illeszthető szöveg-blokkot
egy fix sablon szerint (`services/profile_context.go`, új fájl) — így
minden fogyasztó agent ugyanazt a formázást kapja, nem kell nekik
külön-külön összerakniuk a mezőkből. Üres profil esetén (`background`,
`expertise`, `tone_rules` mind üres string és nincs minta) a `context`
mező üres string — a hívó agent ilyenkor a profil-blokk nélkül, alap
promptból dolgozik, nem hibázik el emiatt.

### 4. AI service — `POST /draft-reply` (első fogyasztó)

Az `ai/app`-ban, a `/categorize-email` mintáját követve, új pydantic-ai
Agent:

**Kérés:**
```json
{ "email_content": "...", "profile_context": "..." }
```
(`profile_context` a backend `/internal/profile-context` válaszából jön
— a backend adja tovább, az AI service nem hívja vissza a backendet
saját magától, hogy ne legyen kör-irányú szolgáltatás-függőség egy
egyszerű szöveg-átadásért.)

**Válasz:**
```json
{ "draft": "..." }
```

Rendszerüzenet váza:
```
Az alábbi profil alapján fogalmazz email-választ a felhasználó
nevében. Vedd figyelembe a hátterét, szakterületét és kommunikációs
stílusát, az írásmintákat pedig hangnem-referenciaként használd.

{profile_context}
```

Ha `profile_context` üres, a rendszerüzenet ezt a blokkot kihagyja, és
egy semleges, udvarias alapstílusban fogalmaz (nincs hibaágazat emiatt).

### 5. Backend — email-válasz-javaslat integrálása

Új `POST /api/v1/emails/:id/draft-reply` végpont (meglévő email-kezelő
mintáit követve): lekéri a levél tartalmát, lekéri a
`/internal/profile-context`-et, meghívja az AI service `/draft-reply`
végpontját, visszaadja a `draft` szöveget. **Nem ment semmit, nem küld
semmit** — a válasz csak a frontendnek szól megjelenítésre.

### 6. Frontend — admin felület

Új, csak `super_admin`-nak látható menüpont (a meglévő szerepkör-alapú
menü-szűrés mintáját követve). Form:
- 3 `<textarea>` mező: Háttér, Szakterület, Kommunikációs stílus
- Dinamikus lista az írásmintákhoz (Label input + Content textarea
  páronként, "+ Új minta" gomb, "Törlés" minden sor mellett — puha
  figyelmeztetés 8 minta fölött, nem kemény limit)
- "Mentés" gomb → `PUT /api/v1/admin/profile`

### 7. Frontend — email-válasz-javaslat UI

Az Emails oldalon (Task 7 UI-jára építve), egy levél megnyitásakor egy
"AI válasz-javaslat" gomb, ami meghívja a `/draft-reply` végpontot, és
a kapott szöveget egy szerkeszthető textarea-ba tölti be — a
felhasználó innen kézzel másolja/küldi tovább a saját email-kliensén
keresztül (ennek a körnek nem célja a tényleges email-küldés
integrálása, csak a fogalmazás-segítség).

## Tesztelés

- **Backend:** `profile_handler_test.go` — CRUD happy path, 403 nem
  `super_admin` szerepkörre (beleértve egy `admin` role tesztesetet is,
  hogy explicit bizonyítsuk: itt NEM elég az `admin`, csak a
  `super_admin`), singleton viselkedés (mindig `id=1` frissül, nem jön
  létre második sor).
- **`profile_context_test.go`:** helyes szöveg-összeállítás kitöltött
  és üres profil esetén is.
- **AI service:** pytest a `/draft-reply` végpontra, `TestModel`-lel
  mockolt Gemini-hívással (nem éles API-hívás a teszt során), üres és
  kitöltött `profile_context` esettel is.
- Build/lint a dokumentált konténerekben: `go build ./...`,
  `go vet ./...`, `go test ./...`, `tsc --noEmit`, AI service `pytest`.

## Biztonsági megjegyzés

A profil személyes háttéradatot tartalmaz — a belső
`/internal/profile-context` végpont docker belső hálózaton kívülről
nem elérhető (ugyanaz a hálózati izolációs garancia, mint amit a final
review már megerősített az AI service portjára — 8000-es port csak a
compose belső hálózaton). Külső providerhez (Gemini) a profil-adat
ugyanúgy kimegy, mint bármely más prompt-tartalom ma is — ez nem új
kockázat, hanem a meglévő AI-integráció már elfogadott
adatkezelési modellje.
