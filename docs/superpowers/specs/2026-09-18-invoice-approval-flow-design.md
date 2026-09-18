# Számla-jóváhagyási flow — Design

**Dátum:** 2026-09-18
**Kapcsolódó specek:** `2026-09-17-billingo-invoicing-design.md`, `2026-09-17-gmail-integration-design.md`

## Cél

Az automatikus havi számlázás jelenleg így működik: minden hónap 1-jén, a
projekten beállított `auto_invoice_enabled` kapcsoló hatására a rendszer
**azonnal létrehozza** a számlát Billingóban, e-mail nélkül
(`services.RunAutoInvoicing`).

Ez a design egy jóváhagyási lépést épít be az értesítés és a tényleges
számla-generálás közé:

1. A hónap elején (automatikusan vagy kézzel) egy **értesítő e-mail** megy ki
   az ügyfélnek a tervezett számláról.
2. Egy csapattag **belül, az alkalmazásban** jóváhagyja az értesítőt — ekkor,
   és csak ekkor, jön létre ténylegesen a számla Billingóban.
3. A jóváhagyás után az ügyfél automatikusan kap egy második e-mailt, a
   kiállított számla PDF-jével csatolva.

A meglévő kézi "Számla kiállítása" gomb (jóváhagyási flow nélkül, közvetlen
számla-létrehozás) **megmarad, változatlanul** — a jóváhagyási flow egy
alternatív, nem kizárólagos út a számla létrehozásához.

## Nem cél

- Nincs ügyfél-oldali (külső, bejelentkezés nélküli) jóváhagyás — a
  jóváhagyás mindig egy belső, `invoices.create` joggal rendelkező
  felhasználó akciója.
- Nincs emlékeztető/lejárati logika a jóváhagyásra váró értesítőkhöz.
- A meglévő kézi számla-létrehozási útvonalon (nem jóváhagyáson keresztül
  létrejött számlák) nem megy ki automatikus PDF-es e-mail.

## 1. Adatmodell

`InvoiceNotice` (`backend/internal/models/invoice_notice.go`) négy új mezőt
kap:

```go
type InvoiceNotice struct {
    ID             uint       `json:"id" gorm:"primaryKey"`
    ProjectID      uint       `json:"project_id" gorm:"not null"`
    ClientID       uint       `json:"client_id" gorm:"not null"`
    PeriodStart    *time.Time `json:"period_start"`
    PeriodEnd      *time.Time `json:"period_end"`
    GmailMessageID string     `json:"gmail_message_id" gorm:"size:100;not null"`
    SentBy         uint       `json:"sent_by" gorm:"not null"`
    SentAt         time.Time  `json:"sent_at"`
    Status         string     `json:"status" gorm:"size:20;not null;default:'pending'"` // "pending" | "approved"
    InvoiceID      *uint      `json:"invoice_id"`
    ApprovedBy     *uint      `json:"approved_by"`
    ApprovedAt     *time.Time `json:"approved_at"`
}
```

Minden helyen, ahol új `InvoiceNotice` sor jön létre, a kódnak explicit be
kell állítania `Status: "pending"`-et (a GORM zero-value string felülírná a
DB oszlop default értékét, ha nincs explicit kiadva).

A `Project.auto_invoice_enabled` / `auto_invoice_client_id` mezők
változatlanok maradnak — csak a **jelentésük** változik: mostantól azt
vezérlik, hogy a hónap elején automatikusan kimenjen-e az értesítő e-mail,
nem azt, hogy létrejöjjön-e a számla.

### Migráció

Új fájlpár: `backend/migrations/000025_add_invoice_notice_approval.up.sql` /
`.down.sql`.

```sql
-- up
ALTER TABLE invoice_notices
    ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'pending',
    ADD COLUMN invoice_id INTEGER REFERENCES invoices(id) ON DELETE SET NULL,
    ADD COLUMN approved_by INTEGER REFERENCES users(id),
    ADD COLUMN approved_at TIMESTAMP;

CREATE INDEX idx_invoice_notices_status ON invoice_notices (status);
```

```sql
-- down
DROP INDEX IF EXISTS idx_invoice_notices_status;
ALTER TABLE invoice_notices
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS invoice_id,
    DROP COLUMN IF EXISTS approved_by,
    DROP COLUMN IF EXISTS approved_at;
```

## 2. Backend flow

### 2.1 Megosztott számla-létrehozási logika

A `CreateInvoice` handler (`backend/internal/handlers/invoice_handler.go`)
törzsét — a kliens/összeg számítást, a Billingo hívást, az
`invoices`+`invoice_items` mentést és a hibakezelést (kb. a jelenlegi 149.
sortól a válasz összeállításáig) — kiemeljük egy csomag-szintű
függvénybe:

```go
// createInvoiceForProject runs the shared invoice-creation flow: pricing
// calculation, Billingo call, and persistence. Used both by the direct
// "create invoice" HTTP handler and by the notice-approval handler, so the
// two paths never diverge.
func createInvoiceForProject(
    billingoService *services.BillingoService,
    project models.Project,
    client models.Client,
    req models.InvoiceCreateRequest,
    createdBy uint,
) (invoice *models.Invoice, items []models.InvoiceItem, httpStatus int, message string)
```

Sikeres híváskor `httpStatus == 0` és `message == ""`; hiba esetén a hívó a
visszaadott `httpStatus`/`message` párost küldi vissza a klienshez
változatlanul (ugyanazok az üzenetek, mint ma).

A `CreateInvoice` HTTP handler ezután: beolvassa a projektet/klienst/body-t,
ellenőrzi a `ProjectClient` kapcsolatot (mint ma), meghívja
`createInvoiceForProject`-et, és a visszatérési érték alapján válaszol.

### 2.2 Értesítő-küldés kiemelése

Az e-mail összeállítás/küldés + `InvoiceNotice` mentés törzsét (jelenleg a
`SendInvoiceNotice` handlerben, kb. 86-114. sor) kiemeljük egy service
függvénybe, mert az ütemezőnek is szüksége lesz rá:

```go
// services/invoice_notice.go
func SendInvoiceNoticeEmail(
    project models.Project,
    client models.Client,
    account models.GmailAccount,
    periodStart, periodEnd *time.Time,
    sentBy uint,
) (*models.InvoiceNotice, error)
```

Ez építi fel a tárgyat/törzset (ugyanaz a szöveg, mint ma), küldi el
`services.BuildRawMessage` + `NewRealGmailAPI().SendMessage`-dzsel, majd
létrehozza a `Status: "pending"` `InvoiceNotice` sort.

A `SendInvoiceNotice` HTTP handler ezután csak a validációt (client_id,
projekt-kliens kapcsolat, Gmail-fiók megléte) végzi, majd meghívja ezt a
függvényt.

### 2.3 Ütemező

`services/auto_invoice.go` — `RunAutoInvoicing` átnevezve
`RunAutoInvoiceNotices`-re (a `scheduler.go`-beli hívás frissül). Minden
jogosult projektnél (hourly, `auto_invoice_enabled`, van
`auto_invoice_client_id` — a szűrés változatlan):

1. Kihagyja, ha már létezik `invoice_notice` (bármilyen `status`) VAGY
   `invoice` (`status IN ('created','pending')`) erre a
   projekt/kliens/időszak hármasra.
2. Összesíti az órákat (`SumLoggedHours`); 0 óránál kihagyja (mint ma).
3. Betölti a projekt `CreatedBy` felhasználóját és annak `GmailAccount`
   sorát; ha nincs csatlakoztatva Gmail-fiókja, naplózza (`log.Printf`) és
   kihagyja ezt a projektet (nem állítja le a többi projekt feldolgozását).
4. Meghívja `services.SendInvoiceNoticeEmail(...)`-t a projekt
   `CreatedBy`-jának Gmail-fiókjával, `sentBy = project.CreatedBy`.

Számla ekkor **nem** jön létre. A `recordFailedAutoInvoice` és a közvetlen
Billingo-hívó kód (jelenlegi `autoInvoiceProject`) törlődik — a jóváhagyás
felelős a tényleges számlázásért és annak hibakezeléséért.

### 2.4 Új végpont: jóváhagyás

`POST /api/v1/projects/:id/invoice-notices/:noticeId/approve`
(`InvoiceNoticeHandler.ApproveInvoiceNotice`, jog: `invoices.create`)

1. Betölti az értesítőt (`id = noticeId AND project_id = id`); 404, ha
   nincs.
2. 409 `"Ez az értesítő már jóvá lett hagyva"`, ha `status != "pending"`.
3. Betölti a projektet és a klienst (mint a `CreateInvoice`-ban).
4. Összeállít egy `models.InvoiceCreateRequest`-et az értesítő adataiból:
   `ClientID`, `PeriodStart`/`PeriodEnd` (`"2006-01-02"` formátumra
   formázva, ha az értesítőn nem `nil`), többi mező üresen (a projekt
   alapértékei érvényesülnek, az órák/összeg **újraszámolódnak** a
   jóváhagyás pillanatában, nem az értesítés-küldéskori állapot szerint).
5. Meghívja a megosztott `createInvoiceForProject(...)`-et.
6. Hiba esetén visszaadja ugyanazt a `httpStatus`/`message` párost; az
   értesítő `pending` marad, újra próbálható.
7. Siker esetén frissíti az értesítőt: `status="approved"`,
   `invoice_id=invoice.ID`, `approved_by=currentUserID`,
   `approved_at=now`.
8. Megpróbálja elküldeni a "számla elkészült" e-mailt PDF-fel:
   - betölti a jóváhagyó felhasználó `GmailAccount` sorát; ha nincs, `email_sent: false` a válaszban, figyelmeztetés naplózva, **nem hibáztatja le a kérést**.
   - letölti a PDF-et (`billingoService.DownloadInvoicePDF`, meglévő).
   - `services.SendInvoiceReadyEmail(account, client, project, invoice, pdfBytes)` — új service függvény, ami `BuildRawMessage`-dzsel csatolja a PDF-et (`MessageAttachment{Filename: invoice.BillingoInvoiceNumber + ".pdf", ContentType: "application/pdf", Data: pdfBytes}`) és elküldi.
   - bármilyen hiba itt csak `email_sent: false`-t és egy `message` figyelmeztetést eredményez, a válasz `success: true` marad (a számla már ténylegesen létrejött).
9. Válasz: `{success: true, invoice: InvoiceResponse, notice: InvoiceNotice, email_sent: bool, message?: string}`.

### 2.5 Globális értesítő-lista végpont

`GET /api/v1/invoice-notices?status=pending` (`InvoiceNoticeHandler.ListAllInvoiceNotices`, jog: `invoices.read`) — az `InvoiceHandler.ListAllInvoices` mintáját követi: projekt/kliens névvel JOIN-olt lista, opcionális `?status=` szűréssel. Ez adja a Billing oldal "Jóváhagyásra váró értesítők" szekciójának adatait.

## 3. Frontend

### 3.1 `invoicesService.ts`

- `InvoiceNotice` interfész bővítése: `status: 'pending' | 'approved'`, `invoice_id: number | null`, `approved_by: number | null`, `approved_at: string | null`, valamint (csak a globális listánál) `project_name?: string`, `client_name?: string`.
- Új `InvoiceNoticesService.approve(projectId: number, noticeId: number)` → `POST /projects/:id/invoice-notices/:noticeId/approve`.
- Új `InvoiceNoticesService.listAll(status?: string)` → `GET /invoice-notices`, opcionális `{status}` query paraméterrel.

### 3.2 Billing oldal (`/dashboard/billing`)

Új "Jóváhagyásra váró értesítők" szekció (a "Számlák" táblázat fölött):
tábla Projekt / Ügyfél / Időszak / Elküldve oszlopokkal, soronként egy
"Jóváhagyás és számla kiállítása" gombbal. Kattintásra:

1. Hívja `InvoiceNoticesService.approve(projectId, noticeId)`-t.
2. Sikeres válasz után eltávolítja a sort a pending listából, frissíti a
   számla-táblázatot (`InvoicesService.getAllInvoices`).
3. Ha `email_sent === false`, egy figyelmeztető sávot jelenít meg:
   "A számla elkészült, de a PDF-es e-mail küldése sikertelen — kérlek
   küldd el manuálisan."
4. Hiba esetén (`success: false`) a hibaüzenetet jeleníti meg, a sor a
   listában marad.

Üres állapot: "Nincs jóváhagyásra váró értesítő."

### 3.3 Projekt számla-oldal (`InvoicePreviewPage`)

A már meglévő, kiválasztott ügyfél/időszak szerinti értesítő-lekérdezés
(116-148. sor körüli `useEffect`) kiegészül a `status`/`id` mezők
eltárolásával. Ha a legutóbbi, ehhez az ügyfél/időszak párhoz tartozó
értesítő `status === 'pending'`, a jelenlegi "Elküldve: ..." szöveg mellett
megjelenik egy "Jóváhagyás és számla kiállítása" gomb, ami ugyanazt az
`InvoiceNoticesService.approve(...)`-t hívja, majd sikeres válasz után
átirányít a projekt oldalra (mint a meglévő kézi `handleSubmit` teszi).

### 3.4 Szövegfrissítés

Két helyen (`InvoicePreviewPage` checkbox leírása, `AutomationRow` label a
billing oldalon) frissül a magyarázó szöveg, hogy jelezze: a kapcsoló
mostantól az **értesítő automatikus kiküldését** vezérli, nem a számla
azonnali létrehozását. Pl.: "Minden hónap 1-jén automatikusan e-mailt küld
az ügyfélnek az előző havi tervezett számláról, jóváhagyásra várva."

## 4. Hibakezelés és szélsőesetek

- **Nincs Gmail-fiók az automatikus küldéshez** (projekt létrehozójának):
  a projekt kimarad az adott futásból, naplózva; a kapcsoló bekapcsolva
  marad, a következő órás ütemező-futás újra megpróbálja (ahogy ma is az
  idempotens újrapróbálkozás működik).
- **Nincs Gmail-fiók a jóváhagyónak**: a számla ettől még létrejön
  Billingóban; a válasz `email_sent: false`-t jelez, a felhasználó
  manuálisan küldheti tovább a PDF-et (a meglévő "Számla PDF megtekintése"
  funkcióval).
- **Kétszeri jóváhagyás** (race / dupla kattintás): a második hívás 409-et
  kap, mert az első már `status="approved"`-re állította az értesítőt.
- **Manuális számla-létrehozás egy már elküldött, pending értesítő
  mellett**: megengedett (a felhasználó explicit kérése), a `pending`
  értesítő ettől nem változik — ha valaki utólag rákattint a
  "Jóváhagyás"-ra, a `createInvoiceForProject` a jóváhagyás pillanatában
  aktuális órákkal újra lefut, ami duplikált számlázáshoz vezethet óradíjas
  projekteknél (ugyanaz a hiányzó once-only szabály, ami ma is fennáll a
  kézi és automatikus óradíjas számlázás között — nem ennek a designnak a
  hatóköre).
- **Jóváhagyás hibázik a Billingo hívásnál**: az értesítő `pending` marad,
  a felhasználó újra megnyomhatja a gombot.

## 5. Érintett fájlok (áttekintés)

**Backend:**
- `backend/internal/models/invoice_notice.go` — új mezők
- `backend/migrations/000025_add_invoice_notice_approval.{up,down}.sql` — új
- `backend/internal/handlers/invoice_handler.go` — `createInvoiceForProject` kiemelése
- `backend/internal/handlers/invoice_notice_handler.go` — `ApproveInvoiceNotice`, `ListAllInvoiceNotices`, `SendInvoiceNotice` egyszerűsítése
- `backend/internal/services/invoice_notice.go` — új (`SendInvoiceNoticeEmail`, `SendInvoiceReadyEmail`)
- `backend/internal/services/auto_invoice.go` — `RunAutoInvoiceNotices`-re átalakítva
- `backend/internal/services/scheduler.go` — függvényhívás frissítése
- `backend/internal/routes/invoice_routes.go` — új route-ok

**Frontend:**
- `frontend/src/services/invoicesService.ts` — `InvoiceNotice` bővítés, `approve`/`listAll`
- `frontend/src/app/dashboard/billing/page.tsx` — pending értesítők szekció, szöveg
- `frontend/src/app/dashboard/board/[projectId]/invoice/page.tsx` — jóváhagyás gomb, szöveg
