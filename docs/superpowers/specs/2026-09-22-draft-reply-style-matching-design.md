# AI válasz-javaslat stílus-illesztés — Design

## Cél

Az email "AI válasz-javaslat" funkció (`ai/app/draft_reply.py`,
`backend/internal/services/draft_reply.go`) jelenleg kizárólag a
statikus, kézzel karbantartott profilra (`BuildProfileContext`)
támaszkodik ahhoz, hogy a felhasználó stílusában fogalmazzon. Ez a
minőséget teljesen a profil kézi karbantartásától teszi függővé, és
soha nem tanul a ténylegesen elküldött válaszokból.

Ez a spec két, egymást kiegészítő javítást fed le (a
`2026-09-22-user-profile-persona-design.md` utáni elemzés B és D
opciója, ld. a kapcsolódó elemző dokumentumot:
https://claude.ai/artifact/QwNRrpnjPUUsDF7suSRmV6):

1. **Retrieval-alapú dinamikus példák (RAG-lite)** — a draft-generálás
   automatikusan felhasznál 0-2 korábbi, releváns, ténylegesen
   elküldött választ konkrét stíluspéldaként, a már szinkronizált
   `emails` táblából.
2. **Könnyű visszacsatolási mérés** — küldéskor megmérjük és
   naplózzuk, mennyire változtatta meg a felhasználó az AI-javaslatot,
   mint jövőbeli minőségi jelzőszám. Ebben a körben **nincs** admin
   jóváhagyó felület a naplózott adatokhoz — csak mérés és tárolás.

## Hatókör és nem-célok

- **Nem cél** ebben a körben: super_admin jóváhagyó/review UI a
  visszacsatolási naplóhoz, vagy automatikus `ProfileSample`
  létrehozás a naplóból. Ez egy jövőbeli, külön spec/feladat (a
  korábbi elemzésben "+2-3 napos" bővítésként szerepelt).
- **Nem cél:** szemantikus (embedding-alapú) keresés vagy vektor-
  adatbázis bevezetése. A jelenlegi email-mennyiségnél (34 elküldött
  email, 25 thread — mérve 2026-09-22-én) a lexikai (szóátfedés)
  hasonlóság elegendő, és nem igényel új infrastruktúrát vagy
  függőséget.
- **Nem cél:** modell finomhangolás (fine-tuning) — a korpusz mérete
  ehhez két nagyságrenddel túl kicsi (ld. az elemző dokumentum C
  opciója).
- **Nem cél:** a `Profile`/`ProfileSample` admin felület vagy
  `BuildProfileContext` mezőinek (háttér, szakterület, hangnem-
  szabályok) átalakítása. Ezek változatlanul a promptba kerülnek, a
  retrieval-alapú példák *mellett*, nem helyette.
- Az AI továbbra sem küld semmit automatikusan — a draft csak
  javaslat, a tényleges elküldés mindig emberi jóváhagyással történik
  (változatlan globális megkötés, ld. a profil-spec).

## Architektúra

```
DraftReply handler (email_handler.go)
     │
     ├─ (meglévő) BuildProfileContext(profile)  → statikus profil-szöveg
     │
     ├─ (új) FindSimilarReplies(email, content)  → 0-2 korábbi sent email törzse
     │         │
     │         ├─ szűrés: emails tábla, folder='sent',
     │         │   azonos thread_id VAGY azonos címzett cím/domain
     │         ├─ rangsorolás: thread-egyezés > Jaccard(subject+snippet)
     │         └─ top-2 törzsének élő lekérése (GmailAPI.GetFullMessage)
     │
     └─ DraftReplyService.DraftReply(content, profileContext, similarReplies)
              │
              ▼
     AI service: POST /draft-reply { email_content, profile_context, similar_replies }
              │
              ▼
     Gemini (pydantic-ai Agent) — promptban: profil + korábbi válaszok + bejövő email


SendEmail handler (email_handler.go)
     │
     ├─ (meglévő) email küldése Gmailen, helyi tükrözés az emails táblába
     │         └─ (javítás) Snippet mostantól a body-ból generálva, nem üresen
     │
     └─ (új) ha a compose AI-javaslatból indult (ai_draft_text mező jött):
              Jaccard(ai_draft_text, sent body) → draft_feedback sor naplózása
```

A rangsoroláshoz **nem** kérünk le élőben minden jelöltet — a
`subject`+`snippet` már helyben, a Postgres `emails` táblájában van.
Csak a végül kiválasztott legfeljebb 2 jelölt teljes törzsét kérjük le
a Gmail API-n keresztül (ugyanaz a hívás, amit a `DraftReply` handler
már ma is használ a bejövő email törzséhez).

## Előfeltétel-javítás: a küldött emailek snippet-je jelenleg üres

A `SendEmail` handler a sikeres küldés után helyben létrehoz egy
`models.Email{Folder: "sent", ...}` sort, hogy a Küldött mappa azonnal
frissüljön (`email_handler.go:334-347`), de `Snippet: ""`-t ír bele. A
Gmail-szinkron (`gmail_sync.go`) csak az **ismeretlen**
`gmail_message_id`-ket dolgozza fel — mivel ez a sor már ismert, a
snippet soha nem töltődik ki utólag.

Mivel a B opció rangsorolása pont a saját korábbi válaszainkra épül,
ez a hiányzó snippet közvetlenül lerontaná a legfontosabb jelölt-
forrás minőségét. Javítás: küldéskor a már rendelkezésre álló `body`
(plain text) alapján generálunk egy snippetet (Gmail-konvenció szerint
kb. az első 200 karakter, szóhatáron vágva), és ezt írjuk a helyi
tükör-sorba.

## 1. rész: Retrieval-alapú dinamikus példák (RAG-lite)

### Jelölt-szűrés

Egy `sent` mappabeli email csak akkor jelölt, ha:

- **azonos `thread_id`**, mint a megválaszolandó (bejövő) emailé, VAGY
- a jelölt `to_addresses` mezője tartalmazza a bejövő email
  feladójának **email-címét vagy domainjét** (a domain kinyerése a
  stdlib `net/mail.ParseAddress`-szel).

A lekérdezés a legutóbbi 200 `sent` sorra korlátozódik
(`ORDER BY received_at DESC LIMIT 200`), hogy a jelölt-halmaz mérete
ne nőjön korlátlanul a rangsorolás előtt.

### Rangsorolás

- **Azonos thread** → legmagasabb pontszám (a legerősebb jel: ugyanaz
  a beszélgetés folytatása).
- **Cím/domain-egyezés, más thread** → szó-alapú Jaccard-hasonlóság a
  jelölt `subject + " " + snippet` és a bejövő email tárgya + törzse
  (a handlerben már meglévő, 4000 karakterre vágott `content`) között.
  Tokenizálás: kisbetűsítés, nem alfanumerikus karaktereken vágás,
  3 karakternél rövidebb tokenek eldobása.
- Ha nincs thread-egyezés **és** a Jaccard-pontszám 0 (egyetlen közös,
  releváns szó sincs), a jelölt kiesik — nem kerül be relevancia
  nélküli példa.
- A megmaradt jelöltek közül a top-2 törzsét kérjük le élőben
  (`GmailAPI.GetFullMessage`); ha egy adott lekérés hibázik, azt a
  jelöltet kihagyjuk, a draft-generálás nem bukik el emiatt.
- Ha a szűrés után 0 jelölt marad, `similar_replies` üresen megy
  tovább — ugyanaz, mint a mai viselkedés (csak statikus profil).

### Interfész-változások

- `ai/app/draft_reply.py`: `DraftReplyRequest` új mezője
  `similar_replies: list[str] = []`. Ha nem üres, a prompt kap egy új
  szakaszt a statikus profil-kontextus után:

  ```
  Az alábbi, korábban általad írt, hasonló témájú/címzettnek szóló
  válaszok stílusát is vedd figyelembe:

  ---
  {similar_replies[0]}
  ---
  {similar_replies[1]}
  ```

  Az ügynök egyéb instrukciói (magyar nyelv, hossz, "ez csak
  javaslat") változatlanok.

- `backend/internal/services/draft_reply.go`: a `DraftReplier`
  interfész és `DraftReplyService.DraftReply` egy új
  `similarReplies []string` paramétert kap; a belső
  `draftReplyRequest` struct kiegészül
  `SimilarReplies []string \`json:"similar_replies"\`` mezővel.

- Új fájl: `backend/internal/services/similar_replies.go` —
  `FindSimilarReplies(ctx, db, email *models.Email, content string, gmailAPI GmailAPI) []string`
  végzi a szűrést/rangsorolást/lekérést. A pontozó logika (thread-
  egyezés, Jaccard, domain-kinyerés) külön, tiszta függvényekbe kerül,
  hogy DB nélkül tesztelhető legyen.

- `email_handler.go`'s `DraftReply` a `draftReplier.DraftReply` hívás
  előtt meghívja `FindSimilarReplies`-t, és az eredményt átadja.

## 2. rész: Könnyű visszacsatolási mérés

### Adatmodell

Új migráció `000031_add_draft_feedback`:

```sql
-- up
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

`models.DraftFeedback{ID uint, EmailID uint, AIDraftText string, SentText string, Similarity float64, CreatedAt time.Time}`,
tábla: `draft_feedback`. `EmailID` a most elküldött (helyben tükrözött)
`emails` sorra mutat, nem az eredeti bejövő emailre — így a jövőbeli
review-funkció egy helyről éri el a küldött szöveget is.

### Hasonlósági metrika

Ugyanaz a szó-alapú Jaccard-függvény, amit az 1. rész is használ
(konzisztencia, nincs új eszköz/függőség):
`Similarity = |közös szavak| / |unió|` az AI-draft és a ténylegesen
elküldött szöveg (plain text `body`) között. `1.0` = változatlanul
elküldve, `0.0` = teljesen átírva.

### Mérési folyamat

- **Frontend** (`frontend/src/app/dashboard/emails/page.tsx`): a
  jelenlegi `pendingDraftText` a szerkesztőbe töltés után törlődik.
  Új state, `aiDraftOriginalText`, amit `handleDraftReply` állít be a
  kapott draft szövegére, és amit a compose-munkamenet végéig (bezárás
  vagy a válaszcélváltása) megőrzünk. `handleSend` ezt elküldi egy új,
  opcionális `ai_draft_text` mezőben a `/emails/send` multipart
  kérésben, ha van ilyen.
- **Backend** (`SendEmail` handler): a helyi `localEmail` sor sikeres
  létrehozása után (a `database.GetDB().Create(&localEmail)` hívás
  már kitöltötte `localEmail.ID`-t), ha a kérés tartalmazott nem üres
  `ai_draft_text`-et, kiszámolja a Jaccard-hasonlóságot az AI-draft és
  a ténylegesen elküldött `body` között, és beszúr egy
  `draft_feedback` sort `EmailID: localEmail.ID`-vel. Hiba esetén csak
  logol (`log.Printf`), a küldés sikerét nem befolyásolja — ugyanaz a
  hibatűrési minta, mint a helyi email-tükrözésnél.
- Ha a compose nem AI-javaslatból indult (nincs `ai_draft_text`), nem
  keletkezik napló sor.

## Tesztelés

A projektben nincs DB-alapú/integrációs teszt-infrastruktúra, és a
frontendnek jelenleg egyetlen `*.test.tsx` fájlja sincs — egyiket sem
vezetjük be. Tiszta függvények kapnak unit tesztet, DB-t érintő
handlerek nem (ugyanaz a konvenció, mint `draft_reply_test.go` és
`email_handler_test.go` esetén).

- **Jaccard-hasonlóság + tokenizálás** (közös az 1. és 2. résznek):
  táblázatos teszt — azonos szöveg → 1.0, nincs közös szó → 0.0,
  részleges átfedés, üres bemenet.
- **Domain-kinyerés** (`net/mail`-alapú helper): unit teszt tipikus és
  hibás címekre.
- **Jelölt-pontozás** (thread-egyezés vs. domain+Jaccard, kizárási
  küszöb): tiszta függvényként kiemelve a DB-lekérdezésből.
- **`FindSimilarReplies`** DB-lekérdező része: nem kap tesztet (követi
  a meglévő Fiber handler konvenciót).
- **`draft_reply.go`**: a meglévő HTTP-kliens teszt bővül a
  `similar_replies` mező request-be kerülésének ellenőrzésével.
- **`ai/app/draft_reply.py`**: a meglévő `test_draft_reply.py` mintája
  szerint új eset — ha `similar_replies` nem üres, a prompt tartalmazza
  a szakaszt; ha üres/hiányzik, nem (`TestModel`-lel, ahogy a meglévő
  tesztek).
- **Snippet-generálás küldéskor** (`generateSnippet(body string) string`):
  unit teszt (hosszú szöveg vágása szóhatáron, üres body).
- **Visszacsatolás-naplózás** hasonlóság-számítása: unit teszt (ugyanaz
  a Jaccard-függvény, más bemenetekkel).

## Globális megkötések (a plan minden feladatára érvényes)

- Nincs GORM `AutoMigrate` — minden séma-változás `golang-migrate`
  fájlokon keresztül (`backend/migrations/`).
- A `000031` migrációs szám lefoglalása előtt ellenőrizni kell
  (`ls backend/migrations | sort -V | tail -5`), hogy még szabad-e.
- Az AI service minden pydantic-ai `Agent`-je saját `GEMINI_MODEL`
  env-változót olvas (nincs kereszt-import más agent modulból) — ugyanaz
  az indoklás, mint a `categorize_email.py`/`draft_reply.py` meglévő
  kommentjében.
- Minden Go backend parancsot `docker exec devbridge_backend ...`,
  minden AI-service Python parancsot `docker exec devbridge_ai ...`,
  minden frontend parancsot `docker exec devbridge_frontend ...`
  konténerben futtatunk, sosem a hoston.
- Az AI soha nem küld semmit automatikusan — a draft csak javaslat,
  a `draft_feedback` napló is csak mérés/utólagos elemzés célját
  szolgálja, nem indít semmilyen automatikus akciót.
