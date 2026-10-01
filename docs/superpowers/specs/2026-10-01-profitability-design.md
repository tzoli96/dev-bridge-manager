# Jövedelmezőség és forgatókönyvek – terv

Dátum: 2026-10-01 · Állapot: jóváhagyásra vár · Hatókör: csak a profitabilitási alrendszer (az AI ügynök külön terv).

## Cél

Megmutatni, hogy a projektek és ügyfelek **valójában** mennyit hoznak óránként (a levelezésre és megbeszélésekre fordított idővel együtt), 3–6 hónapos bevétel-előrejelzést adni, és mentett **forgatókönyvekkel** megválaszolni, hogy kinek érdemes árat emelni, melyik ügyfél éri meg, és mi lenne a **nettó nyereség**, ha valami változna.

## Eldöntött keretek

| Kérdés | Döntés |
|---|---|
| Levelezés/megbeszélés ideje | Becslés a meglévő adatból: levelek száma × beállítható percek, plusz havi megbeszélés-átalány ügyfélenként. |
| "Mi lenne, ha" mélysége | Teljes forgatókönyv-kezelés: mentett, egymás mellett összehasonlítható, több változós. |
| Forgatókönyv változói | Árak és volumen, saját kapacitás, költségek. Az eredmény nettó nyereség. |
| Adó és járulék | Szabadon megadható paraméterek, szerkeszthető sablonokkal. A rendszer **nem tartalmaz adózási szabályt és alapértelmezett kulcsot**. |
| Számítás helye | Igény szerint a backendben (mint a `client_health`), tiszta Go motorral. Nincs pillanatkép-tábla és ütemező. |

## Adatmodell

Meglévő táblákhoz nem nyúlunk. Új migráció(k), a `000043` mintájára; a számozás a következő szabad sorszám.

**`profit_settings`** – egyetlen sor (`id = 1`, `CHECK (id = 1)`).
`minutes_per_inbound_email`, `minutes_per_outbound_email`, `default_capacity_hours_per_month`, `underpriced_ratio_threshold` (az "áremelés-jelölt" küszöb).

**`client_meeting_allowances`** – `client_id` (FK, `ON DELETE CASCADE`, `UNIQUE`), `hours_per_month`.

**`profit_parameter_sets`** – `name`, `percent_items` JSONB `[{label, percent, base}]`, `fixed_monthly_costs` JSONB `[{label, amount}]`.
`base` értéke `revenue` (a bevétel százaléka) vagy `after_costs` (a költségek utáni eredmény százaléka). A szerkezet fix, az értékeket a felhasználó adja.

**`profit_scenarios`** – `name`, `horizon_months` (3–6, `CHECK`), `parameter_set_id` (FK, **`ON DELETE RESTRICT`**), `capacity_hours_per_month`, `client_adjustments` JSONB, `new_clients` JSONB, `created_by`, időbélyegek.
- `client_adjustments`: ügyfélenként `{client_id, new_hourly_rate | new_fixed_price, hours_delta, drops}`.
- `new_clients`: `{name, monthly_revenue, monthly_hours}`.

**Hivatkozási szabályok:**
- Paraméterkészlet nem törölhető, amíg forgatókönyv hivatkozik rá (a felület megmondja, melyik).
- Forgatókönyv a készletre **hivatkozik**, nem másolja; a készlet átírása a hozzá tartozó forgatókönyvek eredményét is módosítja. Az eredmény "mivel számolt" blokkja mindig az aktuális értékeket mutatja.
- Ha egy `client_adjustments` bejegyzés törölt ügyfélre mutat, a motor kihagyja, és figyelmeztetést ad az eredményben (a JSONB-ben nincs FK).

## Számítás

**Havi összesítő (ügyfél × hónap), a meglévő táblákból:**
- **Bevétel:** a számlák `amount`-ja a `period_end` hónapjában (nem a fizetés napján).
- **Naplózott óra:** `task_time_entries` → `tasks` → projekt. Ha egy projekten több ügyfél van, az órák a számlázott összegek arányában oszlanak.
- **Levelezési idő:** az ügyfélhez rendelt e-mailek (`emails.client_id`) száma × a beállított percek, a `folder` szerint (`inbox` / `sent`). Csak az `ugyfel` és `szamla` kategória számít.
- **Megbeszélés:** a havi átalány.

**Valódi óradíj** = bevétel ÷ (naplózott + levelezési + megbeszélési órák).
**Névleges óradíj** = bevétel ÷ naplózott órák. **Arány** = valódi ÷ névleges; a küszöb alatti arány az "áremelés-jelölt".

**Előrejelzés (alap):** az utolsó 3 teljes hónap ügyfélenkénti átlaga. Azok az ügyfelek, akiknek a `contract_end_date`-je a horizonton belül van, a bevételüket külön "megújítástól függő" sávban kapják.

**Forgatókönyv-motor:** tiszta függvény, `Compute(baseData, scenario, parameterSet, settings) → Result`. Havonta számol: bevétel, szükséges órák és kapacitás-túllépés jelzés, százalékos tételek (a `base` szerint), fix költségek, nettó nyereség. Az összehasonlítás fő mérőszáma a nettó nyereség a kapacitásórára vetítve.

**Minden eredményhez** tartozik egy "mivel számolt" blokk: a felhasznált szorzók, paraméterek és a hiányos adatú hónapok.

**Kevés adat:** 3 hónapnál rövidebb előzménynél az eredmény "kevés adat" jelzést kap, és nem állít pontosságot.

## API

Csoport: `/api/v1/profitability`, JWT + jogosultság.

| Végpont | Jog |
|---|---|
| `GET /overview?months=` | read |
| `GET /forecast?months=3..6` | read |
| `GET`, `PUT /settings` | read, manage |
| `GET`, `PUT /clients/:id/meeting-allowance` | read, manage |
| `GET`, `POST`, `PUT`, `DELETE /parameter-sets` | read, manage |
| `GET`, `POST`, `PUT`, `DELETE /scenarios` | read, manage |
| `POST /scenarios/compute` (mentés nélkül) | read |
| `GET /scenarios/compare?ids=` | read |

**Jogok:** új `profitability.read` és `profitability.manage`, a `000011` mintájára felvéve; alapból csak az admin szerepkörök kapják. A projekttagság nem elég.

## Felület

Új menüpont: **Jövedelmezőség** (`/dashboard/profitability`), négy fül:
1. **Áttekintés** – ügyfél- és projekttábla, rendezhető arány szerint, áremelés-jelölt jelvény.
2. **Előrejelzés** – 3–6 hónapos diagram, a megújítástól függő sáv külön színnel (`dataviz` skill).
3. **Forgatókönyvek** – lista, szerkesztő élő eredménnyel, 2–3 kijelölése összehasonlításhoz.
4. **Beállítások** – szorzók, megbeszélés-átalányok, paraméterkészletek és sablonok.

Az alapadat (havi összesítők) egyszer töltődik be; a csúszkák csak a motort hívják ezen (`POST /scenarios/compute`).

## Sablonok

A sablon egy előre létrehozott, **üres értékű** paraméterkészlet-szerkezet (pl. "átalányadózó" névvel és a tételek címkéivel), amit a felhasználó kitölt. Adókulcsot és járulékot nem szállítunk; a számok ellenőrzése a felhasználó és a könyvelője feladata.

## Tesztelés

- **Motor és számítási részek:** tiszta függvények, táblázatos Go tesztek. Esetek: áremelés, kiesés, új ügyfél, kapacitás-túllépés, mindkét `base`, üres készlet, 0 óra, megújítás a horizont szélén, törölt ügyfélre mutató bejegyzés.
- **SQL:** a lokális dev adatbázison, visszagörgetett tranzakcióban, ideiglenes tesztadattal. Valós vagy megosztott adaton nem.
- **Frontend:** `tsc` és eslint a konténerben az új fájlokra, plusz böngészős végigpróbálás Playwrighttal.
- Minden ellenőrzés a projekt konténereiben fut; a riport pontosan rögzíti, mi futott és mi nem.

## Szállítási sorrend

Minden szakasz külön feature branchen, önállóan használható:
1. **Valódi óradíj** – migrációk (jogok, beállítások, átalány), `overview`, Áttekintés fül.
2. **Előrejelzés** – `forecast` és a diagram.
3. **Paraméterkészletek és motor** – CRUD, motor, `compute`.
4. **Összehasonlítás és sablonok.**

Merge a `main`-be és push csak kifejezett kérésre.

## Nyitott pontok

- **Az 1. szakasz előfeltétele:** valós Billingo-számlán ellenőrizni, hogy az `Invoice.amount` nettó-e. A motor nettónak veszi.
- A több ügyfeles projektek óraelosztása a számlázott összegek arányában történik. Ha ez félrevezető eredményt ad, az 1. szakasz után felülvizsgáljuk.
- A küszöb alapértéke (`underpriced_ratio_threshold`) a beállításokban van; kezdőértéket az 1. szakasz során, valós adaton javaslunk.
- A rendszerben jelenleg egy projekt és kevés történet van, ezért az első számok tájékoztató jellegűek.
- **2. szakasz döntései:** a fix áras számlák kimaradnak az előrejelzésből (egyszeri számla, figyelmeztetéssel); a horizont az aktuális hónappal kezdődik; a szerződés vége projektenként érvényesül (a lejárat hónapja még „biztos"); alapértelmezett horizont 6 hónap (3–6); az oldal két fülre oszlik, a Forgatókönyvek és a Beállítások fül a 3–4. szakaszban jön.
- **Előrejelzés korlátja:** az átlag az utolsó 3 hónapra támaszkodik, ezért új vagy megszűnő ügyfelet lassan követ, és pipeline nincs (új bevétel csak forgatókönyvben szerepelhet).

## Kapcsolat az AI ügynökkel

Az `overview` és a `forecast` kimenete strukturált jelzéseket tartalmaz (pl. az arány a küszöb alá esett), amelyeket a későbbi AI ügynök felhasználhat. Az ügynök külön brainstorming-kört kap; ebben a tervben nem szerepel.
