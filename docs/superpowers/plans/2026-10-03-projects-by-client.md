# Projektlista ügyfél szerint csoportosítva Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A `/dashboard/board` projektlistája ügyfél szerint csoportosított, lenyitható blokkokban jelenjen meg (egy ügyfélnek több projektje lehet; a több ügyfeles projektek külön csoportba kerülnek), magyar feliratokkal.

**Architecture:** A `GET /projects` válasza egy plusz, kötegelt lekérdezéssel kitölti a már meglévő `clients` mezőt (additív, `omitempty`). A csoportosítás egy tiszta, függőségmentes TypeScript függvény (`groupProjectsByClient`), amit egy ideiglenes Node-szkript ellenőriz. A felület a meglévő táblázatot csoportonként ismétli egy valódi gombbal nyitható/csukható blokkban; az állapot ügyfél-azonosítóra kulcsolva `localStorage`-ban marad meg.

**Tech Stack:** Go + Fiber + GORM (backend konténer), Next.js + TypeScript + Tailwind v4 + `lucide-react` (frontend konténer).

**Spec:** nincs külön spec-fájl; a követelményeket a brainstorming döntései rögzítik (lent).

## Döntések (a felhasználóval egyeztetve)

1. **Csoportosítás:** pontosan 1 hozzárendelt ügyfél → az ügyfél csoportja; 2+ ügyfél → egy közös **„Több ügyfél"** csoport (a sor mutatja az ügyfelek neveit); nincs ügyfél → **„Ügyfél nélkül"** csoport. Az ügyfél-csoportok ábécé szerint (magyar rendezés), utánuk „Több ügyfél", legvégül „Ügyfél nélkül". A csoporton belül a projektek a kapott sorrendben maradnak.
2. **Lenyitás:** alapból minden nyitva; az állapot megmarad `localStorage`-ban, **az ügyfél azonosítójára kulcsolva** (átnevezéskor ne vesszen el); felül „Mind lenyit / Mind becsuk" gomb.
3. **Összesítő:** az **egyedi projektek** számát mutatja (nem a csoportok összegét).
4. **Magyarosítás:** csak ennek az oldalnak a feliratai. A státusz értékei (active, completed…) a backend értékei, változatlanok.

## Global Constraints

- **Adatot nem módosít.** Csak megjelenítés és egy olvasó lekérdezés. Új végpont, migráció, jog nincs.
- A `GET /projects` válasza **additív** módon bővül (`clients`, `omitempty`); a többi fogyasztó (Számlázás oldal, főoldal `useProjects`, e-mail → feladat ablak) nem sérülhet. A meglévő mezők és a rendezés (`created_at DESC`) változatlanok.
- A szerkesztés (modal), a megnyitás, az „Új projekt" gomb és a jogosultságok (`isAdmin`) **pontosan úgy működnek, mint ma**.
- A frontendnek **nincs tesztfuttatója**: ellenőrzés `tsc`, `eslint`, az oldal fordulása, és a tiszta logika ideiglenes Node-szkripttel (`node --no-warnings --experimental-strip-types`), amit `/tmp`-ben hozunk létre a **frontend konténerben**, és törlünk. Backend oldalon egy ideiglenes, csak olvasó Go teszt a valódi handlerrel a dev adatbázison (`backend/internal/handlers/zz_tmp_*_test.go`), amit törlünk.
- Minden ellenőrzés a projekt konténereiben fut, a repo gyökeréből: `docker compose -f .docker/docker-compose.yml exec -T backend|frontend sh -c '…'`. Host-oldali futtatás tilos. A dev adatbázison **írás nincs**.
- **Commit, merge és push csak a felhasználó kifejezett kérésére.** Az alügynökök nem commitolnak (`git add`, `commit`, `stash`, `reset`, `checkout`, `switch` tilos).
- A working tree-ben **idegen, nem commitolt munka** van (Jira: `activity_digest_handler.go`, `jira_*`, `task_comment_handler.go`, `models/kanban.go`, `services/jira_*`, `migrations/000040_*`, `docs/.../marketing-contacts.md`). Ezekhez nem nyúlunk.
- Régi, nem ide tartozó hibák: a `tsc` 6 hibája más fájlokban, és a `useProjects.ts` `err: any` (nem érintjük). Új `any` nem kerülhet be. Új függőség tilos. `gofmt -w` az érintett Go fájlokon, majd `gofmt -l` üres. A kódstílus a frontend `app/dashboard` mappájában: 4 szóköz, **pontosvessző nélkül**, magyar felirat.
- Munkabranch: `feat/projects-by-client`.

## File Structure

| Fájl | Felelősség |
|---|---|
| `backend/internal/handlers/project_handler.go` | **Módosul:** `GetAllProjects` kitölti a `clients` mezőt (kötegelt lekérdezés) |
| `frontend/src/utils/groupProjects.ts` | Tiszta csoportosítás: `groupProjectsByClient` |
| `frontend/src/app/dashboard/board/page.tsx` | **Újraírva:** csoportos, lenyitható lista, magyar feliratok |

---

### Task 1: A projektlista adja vissza az ügyfeleket

**Files:**
- Modify: `backend/internal/handlers/project_handler.go` (`GetAllProjects`)

**Interfaces:** a válasz `projects[].clients` mezője `[]models.ProjectClientResponse` (a meglévő típus: `id, project_id, client_id, client_name, client_type, assigned_at, assigned_by, assigned_by_name`). A mező hiányzik (`omitempty`), ha a projektnek nincs ügyfele.

- [ ] **Step 1: Olvasd el** a `GetAllProjects` függvényt (`project_handler.go`, ~73. sor) és a `GetProjectClients` lekérdezését (`project_client_handler.go`, ~57. sor), amelynek `Select`/`Joins` mintáját követed.

- [ ] **Step 2: Töltsd be az ügyfeleket kötegelve.** A projektek `Scan`-je után, a `response` összeállítása **előtt**, egyetlen lekérdezéssel az összes projekt hozzárendelését, majd építsd fel a `map[uint][]models.ProjectClientResponse` térképet (kulcs: `ProjectID`), és tedd a `ProjectResponse{... Clients: clientsByProject[project.ID]}` mezőbe:

```go
	// Egyetlen lekérdezés az összes projekt ügyfeleihez (nem projektenként), hogy
	// a lista ügyfél szerint csoportosítható legyen.
	projectIDs := make([]uint, 0, len(projects))
	for _, project := range projects {
		projectIDs = append(projectIDs, project.ID)
	}
	clientsByProject := make(map[uint][]models.ProjectClientResponse, len(projects))
	if len(projectIDs) > 0 {
		var links []models.ProjectClientResponse
		err = database.GetDB().Table("project_clients").
			Select(`project_clients.id, project_clients.project_id, project_clients.client_id,
				clients.name as client_name, clients.type as client_type,
				project_clients.assigned_at, project_clients.assigned_by,
				users.name as assigned_by_name`).
			Joins("LEFT JOIN clients ON project_clients.client_id = clients.id").
			Joins("LEFT JOIN users ON project_clients.assigned_by = users.id").
			Where("project_clients.project_id IN ?", projectIDs).
			Order("clients.name ASC, project_clients.id ASC").
			Scan(&links).Error
		if err != nil {
			return c.Status(500).JSON(models.ProjectListResponse{
				Success: false,
				Message: "Error fetching project clients",
			})
		}
		for _, link := range links {
			clientsByProject[link.ProjectID] = append(clientsByProject[link.ProjectID], link)
		}
	}
```

A `ProjectResponse{...}` literálba add hozzá: `Clients: clientsByProject[project.ID],`. A függvény többi részéhez (a `Scan`, a mezők, a `ProjectListResponse`) ne nyúlj. Ha a `err` változó már deklarált a függvényben, használd az `=`-t (a fenti kód erre van írva); ha nem, a fordító jelezni fogja, és `:=`-re kell cserélni.

- [ ] **Step 3: gofmt, vet, build**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -w internal/handlers/project_handler.go; gofmt -l internal/handlers/project_handler.go; go vet ./internal/handlers/ && go build ./... && echo build-ok'`
Expected: `gofmt -l` üres, `build-ok`. `git diff backend/internal/handlers/project_handler.go` csak hozzáadott sorokat és a `Clients:` mezőt mutassa.

- [ ] **Step 4: A valódi handler ellenőrzése a dev adatbázison (csak olvas)**

Run:
```bash
cat > backend/internal/handlers/zz_tmp_projects_clients_test.go <<'EOF'
package handlers

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"github.com/gofiber/fiber/v2"
)

// TEMPORARY, read-only: runs the real GetAllProjects against the dev DB. Deleted right after.
func TestTmpGetAllProjectsIncludesClients(t *testing.T) {
	database.Connect()
	app := fiber.New()
	app.Get("/projects", NewProjectHandler().GetAllProjects)
	resp, err := app.Test(httptest.NewRequest("GET", "/projects", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	var out models.ProjectListResponse
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	t.Logf("status=%d success=%v count=%d", resp.StatusCode, out.Success, len(out.Projects))
	for _, p := range out.Projects {
		names := []string{}
		for _, c := range p.Clients {
			names = append(names, c.ClientName)
		}
		t.Logf("PROJECT id=%d name=%q clients=%v", p.ID, p.Name, names)
	}
}
EOF
docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && go test ./internal/handlers/ -run TestTmpGetAllProjectsIncludesClients -v 2>&1 | grep -E "status=|PROJECT|FAIL|PASS|panic"'
rm -f backend/internal/handlers/zz_tmp_projects_clients_test.go; ls backend/internal/handlers | grep -c zz_tmp
```
Expected: `status=200 success=true`; a dev adatban a két projekt (Golfrange online, Mentorfy) mindegyike `clients=[PIXEL STÚDIÓ Kft]`; az utolsó sor `0` (az ideiglenes fájl törölve). Ha az adat időközben változott, az elvárást a tényleges `project_clients` sorokból számold újra (psql olvasás), és írd le.

- [ ] **Step 5: A teljes backend teszt**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && out=""; for p in models services handlers middleware; do r=$(go test -count=1 ./internal/$p/ 2>&1 | tail -1 | cut -d" " -f1); out="$out $p=$r"; done; echo "EREDMÉNY:$out"'`
Expected: `models=ok services=ok handlers=ok middleware=ok`.

- [ ] **Step 6: Commit** (csak kérésre)

```bash
git add backend/internal/handlers/project_handler.go
git commit -m "feat(projects): include client assignments in the project list"
```

---

### Task 2: A csoportosító függvény (tiszta, ellenőrzött)

**Files:**
- Create: `frontend/src/utils/groupProjects.ts`

**Interfaces:**
- Consumes: a `Project` és `ProjectClient` típusok (`@/services/projectsService`) — **csak típusként** (`import type`), hogy a modul Node-ban, a Next.js nélkül futtatható legyen.
- Produces:
  - `type ProjectGroupKind = 'client' | 'multiple' | 'none'`
  - `interface ProjectGroup { key: string; kind: ProjectGroupKind; clientId: number | null; label: string; projects: Project[] }` (`key`: `client:<id>`, `multiple`, `none`)
  - `function groupProjectsByClient(projects: Project[]): ProjectGroup[]`
  - `function projectClientNames(project: Project): string` (a „Több ügyfél" sorához: a nevek ábécé szerint, vesszővel)
  - `const MULTIPLE_LABEL = 'Több ügyfél'`, `const NONE_LABEL = 'Ügyfél nélkül'`

- [ ] **Step 1: Írd meg az ellenőrző szkriptet (a konténerben, a repón kívül), és ellenőrizd, hogy elbukik**

Run (a modul még nem létezik, ezért importhibával bukik):
```bash
docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cat > /tmp/zz_group.mts <<EOF
import { groupProjectsByClient, projectClientNames } from "/app/src/utils/groupProjects.ts";

let failed = 0;
const eq = (actual: unknown, expected: unknown, name: string) => {
  const a = JSON.stringify(actual), e = JSON.stringify(expected);
  if (a !== e) { failed++; console.log("FAIL", name, "\n  got ", a, "\n  want", e); }
};
const link = (id: number, name: string) => ({ id, project_id: 0, client_id: id, client_name: name, client_type: "company", assigned_at: "", assigned_by: 0, assigned_by_name: "" });
const p = (id: number, name: string, clients?: ReturnType<typeof link>[]) => ({ id, name, clients }) as any;

// 1. one client each, two projects of the same client share a group, input order kept inside
let g = groupProjectsByClient([p(1, "A1", [link(11, "PIXEL")]), p(2, "B1", [link(12, "Acme")]), p(3, "A2", [link(11, "PIXEL")])]);
eq(g.map((x) => x.label), ["Acme", "PIXEL"], "client groups sorted by name");
eq(g[1].projects.map((x: any) => x.id), [1, 3], "projects of one client keep input order");
eq(g[0].kind, "client", "kind client");
eq(g[0].key, "client:12", "key by client id");
eq(g[0].clientId, 12, "clientId");

// 2. multiple clients -> one shared "Több ügyfél" group after the client groups
g = groupProjectsByClient([p(1, "Shared", [link(11, "PIXEL"), link(12, "Acme")]), p(2, "Solo", [link(11, "PIXEL")])]);
eq(g.map((x) => x.key), ["client:11", "multiple"], "multiple group after clients");
eq(g[1].label, "Több ügyfél", "multiple label");
eq(g[1].clientId, null, "multiple has no clientId");
eq(g[1].projects.map((x: any) => x.id), [1], "shared project only in the multiple group");

// 3. no clients (undefined, empty array) -> "Ügyfél nélkül" last
g = groupProjectsByClient([p(1, "NoField"), p(2, "Empty", []), p(3, "Solo", [link(11, "PIXEL")]), p(4, "Shared", [link(11, "PIXEL"), link(12, "Acme")])]);
eq(g.map((x) => x.key), ["client:11", "multiple", "none"], "order: clients, multiple, none");
eq(g[2].label, "Ügyfél nélkül", "none label");
eq(g[2].projects.map((x: any) => x.id), [1, 2], "none holds undefined and empty");

// 4. every project appears in EXACTLY one group (no double counting)
const all = [p(1, "a", [link(11, "X")]), p(2, "b", [link(11, "X"), link(12, "Y")]), p(3, "c"), p(4, "d", [link(12, "Y")])];
g = groupProjectsByClient(all);
const ids = g.flatMap((x) => x.projects.map((q: any) => q.id)).sort();
eq(ids, [1, 2, 3, 4], "each project once");

// 5. two DIFFERENT clients with the same name stay separate groups (key by id), stable order by id
g = groupProjectsByClient([p(1, "a", [link(21, "Kft")]), p(2, "b", [link(20, "Kft")])]);
eq(g.map((x) => x.key), ["client:20", "client:21"], "same name: separate groups ordered by id");

// 6. duplicate link rows of the same client count as ONE client (not multiple)
g = groupProjectsByClient([p(1, "dup", [link(11, "PIXEL"), link(11, "PIXEL")])]);
eq(g.map((x) => x.key), ["client:11"], "duplicate link of one client is not 'multiple'");

// 7. Hungarian collation: Á sorts with A, not after Z
g = groupProjectsByClient([p(1, "z", [link(1, "Zebra")]), p(2, "a", [link(2, "Árvíz")]), p(3, "b", [link(3, "Béta")])]);
eq(g.map((x) => x.label), ["Árvíz", "Béta", "Zebra"], "hu collation");

// 8. empty input
eq(groupProjectsByClient([]), [], "empty input");

// 9. names for the multiple-client row
eq(projectClientNames(p(1, "x", [link(12, "Béta"), link(11, "Alfa"), link(11, "Alfa")])), "Alfa, Béta", "names sorted, deduplicated");
eq(projectClientNames(p(1, "x")), "", "no clients -> empty string");

console.log(failed ? "RESULT FAIL " + failed : "RESULT OK");
EOF
node --no-warnings --experimental-strip-types /tmp/zz_group.mts 2>&1 | grep -E "RESULT|FAIL|Cannot find|ERR_" | head -5'
```
Expected: `ERR_MODULE_NOT_FOUND` / `Cannot find module` (a fájl még nincs).

- [ ] **Step 2: Írd meg a tiszta modult**

```ts
// frontend/src/utils/groupProjects.ts
// Dependency-free on purpose (type-only imports): the grouping rule can be
// checked with plain Node, without the Next.js toolchain.
import type { Project, ProjectClient } from '@/services/projectsService'

export const MULTIPLE_LABEL = 'Több ügyfél'
export const NONE_LABEL = 'Ügyfél nélkül'

export type ProjectGroupKind = 'client' | 'multiple' | 'none'

export interface ProjectGroup {
    /** Stable key: client:<id>, multiple or none. Used for the persisted open/closed state. */
    key: string
    kind: ProjectGroupKind
    clientId: number | null
    label: string
    projects: Project[]
}

const collator = new Intl.Collator('hu')

// A project may carry duplicate link rows for the same client: count each client once.
function distinctClients(project: Project): ProjectClient[] {
    const byId = new Map<number, ProjectClient>()
    for (const link of project.clients ?? []) {
        if (!byId.has(link.client_id)) byId.set(link.client_id, link)
    }
    return [...byId.values()]
}

/** The client names of a project, alphabetical, comma separated ("" when none). */
export function projectClientNames(project: Project): string {
    return distinctClients(project)
        .map((link) => link.client_name)
        .sort((a, b) => collator.compare(a, b))
        .join(', ')
}

/**
 * Groups projects by client. One client -> that client's group; two or more
 * -> a shared "Több ügyfél" group; none -> "Ügyfél nélkül". Every project is
 * in exactly one group. Client groups are alphabetical (ties by id), then the
 * shared group, then the one without clients. Projects keep their input order.
 */
export function groupProjectsByClient(projects: Project[]): ProjectGroup[] {
    const byClient = new Map<number, ProjectGroup>()
    const multiple: Project[] = []
    const none: Project[] = []

    for (const project of projects) {
        const clients = distinctClients(project)
        if (clients.length === 0) {
            none.push(project)
        } else if (clients.length > 1) {
            multiple.push(project)
        } else {
            const only = clients[0]
            let group = byClient.get(only.client_id)
            if (!group) {
                group = {
                    key: `client:${only.client_id}`,
                    kind: 'client',
                    clientId: only.client_id,
                    label: only.client_name,
                    projects: [],
                }
                byClient.set(only.client_id, group)
            }
            group.projects.push(project)
        }
    }

    const groups = [...byClient.values()].sort(
        (a, b) => collator.compare(a.label, b.label) || (a.clientId ?? 0) - (b.clientId ?? 0)
    )
    if (multiple.length > 0) {
        groups.push({ key: 'multiple', kind: 'multiple', clientId: null, label: MULTIPLE_LABEL, projects: multiple })
    }
    if (none.length > 0) {
        groups.push({ key: 'none', kind: 'none', clientId: null, label: NONE_LABEL, projects: none })
    }
    return groups
}
```

- [ ] **Step 3: Futtasd az ellenőrzést, és ellenőrizd, hogy átmegy**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'node --no-warnings --experimental-strip-types /tmp/zz_group.mts 2>&1 | grep -E "RESULT|FAIL|Cannot find|ERR_|got |want" | head -12'`
Expected: `RESULT OK`, `FAIL` sor nélkül. Ha egy eset bukik, **ne** a szkriptet vagy az implementációt hangold ösztönből: állj meg, és jelentsd a pontos esetet és az eredményt. (A `Intl.Collator('hu')` a konténer Node-jának ICU-támogatásától függ: ha a 7. eset a rendezés miatt bukik, jelentsd, hogy a Node ICU nélkül fut, és hogy a böngésző viselkedése ettől független.)

- [ ] **Step 4: Töröld az ideiglenes szkriptet, típusellenőrzés és lint**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cd /app && rm -f /tmp/zz_group.mts; ls /tmp/zz_group.mts 2>&1 | head -1; echo "tsc az érintett fájlban: $(npx tsc --noEmit -p . 2>&1 | grep -c groupProjects)"; echo "tsc összes régi hiba: $(npx tsc --noEmit -p . 2>&1 | grep -c "error TS")"; npx eslint src/utils/groupProjects.ts > /tmp/zz_e.out 2>&1; echo "eslint exit: $? sorok: $(wc -l < /tmp/zz_e.out)"; head -4 /tmp/zz_e.out; rm -f /tmp/zz_e.out'`
Expected: `No such file` (a szkript törölve), `tsc az érintett fájlban: 0`, összes régi hiba `6`, `eslint exit: 0 sorok: 0`.

- [ ] **Step 5: Commit** (csak kérésre)

```bash
git add frontend/src/utils/groupProjects.ts
git commit -m "feat(projects): add pure group-projects-by-client logic"
```

---

### Task 3: Az oldal újraírása (lenyitható csoportok, magyar feliratok)

**Files:**
- Modify (újraírás): `frontend/src/app/dashboard/board/page.tsx`

**Interfaces:**
- Consumes: `useProjects()` (`{ projects, loading, error, refetch }`), `useModals()` (`setShowCreateProject`, `setShowEditProject`, `setSelectedProject`, `setOnProjectUpdated`), `useAuth()`, `isAdmin`, `groupProjectsByClient`, `projectClientNames`, `MULTIPLE_LABEL` (Task 2), `Button`, `LoadingState`, `ErrorState`, `EmptyState`, lucide `ChevronDown`, `ChevronRight`.
- Produces: ugyanaz az alapértelmezett export (`BoardPage`), ugyanazon az útvonalon.

**Előkészület:** olvasd el a jelenlegi `page.tsx`-t. **Szó szerint megmarad:** az `onProjectUpdated` bekötés (`useEffect` + `setOnProjectUpdated(() => refetch)`), a `statusStyles`, a `formatPricing` logika, a jogosultsági feltételek (`isAdmin(user)` a „Szerkesztés" és az „Új projekt" gombon), a szerkesztés (`setSelectedProject(project); setShowEditProject(true)`) és a megnyitás (`router.push(`/dashboard/board/${project.id}`)`). Csak a megjelenés és a feliratok változnak.

- [ ] **Step 1: Írd át az oldalt**

A `formatPricing` szövegei (`HUF/hr`, `HUF fixed`, `Hobbi projekt`) maradjanak, ahogy vannak (a `Hobbi projekt` már magyar; a másik kettő pénzügyi jelölés, nem ennek a feladatnak a része).

```tsx
'use client'

import { useCallback, useEffect, useMemo, useState } from 'react'
import { useRouter } from 'next/navigation'
import { ChevronDown, ChevronRight } from 'lucide-react'
import { useProjects } from '@/hooks/useProjects'
import { useModals } from '@/components/dashboard/DashboardModals'
import { useAuth } from '@/contexts/AuthContext'
import { isAdmin } from '@/utils/permissions'
import { groupProjectsByClient, projectClientNames } from '@/utils/groupProjects'
import { Button } from '@/components/ui/button'
import LoadingState from '@/components/ui/LoadingState'
import ErrorState from '@/components/ui/ErrorState'
import EmptyState from '@/components/ui/EmptyState'

const statusStyles: Record<string, string> = {
    active: 'bg-success/10 text-success',
    completed: 'bg-primary/10 text-primary',
    'on-hold': 'bg-warning/10 text-warning',
    cancelled: 'bg-muted text-muted-foreground',
}

function formatPricing(project: { pricing_type?: 'hourly' | 'fixed' | 'hobby' | ''; hourly_rate?: number | null; fixed_price?: number | null }): string {
    if (project.pricing_type === 'hourly' && project.hourly_rate) {
        return `${project.hourly_rate.toLocaleString()} HUF/hr`
    }
    if (project.pricing_type === 'fixed' && project.fixed_price) {
        return `${project.fixed_price.toLocaleString()} HUF fixed`
    }
    if (project.pricing_type === 'hobby') {
        return 'Hobbi projekt'
    }
    return '-'
}

const COLLAPSED_STORAGE_KEY = 'board-projects-collapsed-groups'

// The set of collapsed group keys is persisted; anything not listed is open.
function readCollapsed(): string[] {
    try {
        const raw = window.localStorage.getItem(COLLAPSED_STORAGE_KEY)
        const parsed: unknown = raw ? JSON.parse(raw) : []
        return Array.isArray(parsed) ? parsed.filter((item): item is string => typeof item === 'string') : []
    } catch {
        return []
    }
}

export default function BoardPage() {
    const router = useRouter()
    const { user } = useAuth()
    const { projects, loading, error, refetch } = useProjects()
    const { setShowCreateProject, setShowEditProject, setSelectedProject, setOnProjectUpdated } = useModals()
    const [collapsed, setCollapsed] = useState<string[]>([])
    const [collapsedLoaded, setCollapsedLoaded] = useState(false)

    useEffect(() => {
        setOnProjectUpdated(() => refetch)
        return () => setOnProjectUpdated(undefined)
    }, [setOnProjectUpdated, refetch])

    // localStorage is only available in the browser: read it after mount.
    useEffect(() => {
        setCollapsed(readCollapsed())
        setCollapsedLoaded(true)
    }, [])

    const groups = useMemo(() => groupProjectsByClient(projects ?? []), [projects])

    const persist = useCallback((next: string[]) => {
        setCollapsed(next)
        try {
            window.localStorage.setItem(COLLAPSED_STORAGE_KEY, JSON.stringify(next))
        } catch {
            // Storage unavailable (private mode / quota): the state still works for this visit.
        }
    }, [])

    const toggleGroup = (key: string) => {
        persist(collapsed.includes(key) ? collapsed.filter((k) => k !== key) : [...collapsed, key])
    }

    if (loading) return <LoadingState message="Projektek betöltése..." />
    if (error) return <ErrorState error={error} onRetry={refetch} />

    const projectCount = projects?.length ?? 0
    const allCollapsed = groups.length > 0 && groups.every((group) => collapsed.includes(group.key))

    return (
        <div className="space-y-6">
            <div className="flex justify-between items-center">
                <div>
                    <h1 className="text-2xl font-bold text-foreground">Projektek</h1>
                    <p className="text-muted-foreground text-sm">Válassz egy projektet a boardjainak megnyitásához</p>
                </div>
                {isAdmin(user) && (
                    <Button onClick={() => setShowCreateProject(true)}>
                        Új projekt
                    </Button>
                )}
            </div>

            {projectCount === 0 ? (
                <EmptyState
                    icon="projects"
                    title="Nincs projekt"
                    description="Nincs megjeleníthető projekt."
                    action={isAdmin(user) ? {
                        label: "Első projekt létrehozása",
                        onClick: () => setShowCreateProject(true)
                    } : undefined}
                />
            ) : (
                <div className="space-y-4">
                    <div className="flex items-center justify-between">
                        <p className="text-sm text-muted-foreground">
                            {projectCount} projekt, {groups.length} csoport
                        </p>
                        <Button
                            variant="secondary"
                            size="sm"
                            onClick={() => persist(allCollapsed ? [] : groups.map((group) => group.key))}
                        >
                            {allCollapsed ? 'Mind lenyit' : 'Mind becsuk'}
                        </Button>
                    </div>

                    {groups.map((group) => {
                        const isOpen = !collapsedLoaded || !collapsed.includes(group.key)
                        const panelId = `projects-${group.key.replace(/[^a-z0-9]+/gi, '-')}`
                        const Chevron = isOpen ? ChevronDown : ChevronRight
                        return (
                            <section key={group.key} className="bg-card shadow-sm border border-border rounded-xl overflow-hidden">
                                <button
                                    type="button"
                                    onClick={() => toggleGroup(group.key)}
                                    aria-expanded={isOpen}
                                    aria-controls={panelId}
                                    className="flex w-full items-center gap-3 bg-muted px-6 py-3 text-left transition-colors hover:bg-muted/70"
                                >
                                    <Chevron size={16} className="shrink-0 text-muted-foreground" />
                                    <span className="font-medium text-foreground">{group.label}</span>
                                    <span className="text-sm text-muted-foreground">
                                        {group.projects.length} projekt
                                    </span>
                                </button>

                                {isOpen && (
                                    <div id={panelId} className="overflow-x-auto">
                                        <table className="min-w-full divide-y divide-border">
                                            <thead className="bg-muted">
                                                <tr>
                                                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Projekt</th>
                                                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Árazás</th>
                                                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Állapot</th>
                                                    <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Létrehozta</th>
                                                    <th scope="col" className="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase tracking-wider">Műveletek</th>
                                                </tr>
                                            </thead>
                                            <tbody className="bg-card divide-y divide-border">
                                                {group.projects.map(project => (
                                                    <tr key={project.id} className="hover:bg-muted/50">
                                                        <td className="px-6 py-4">
                                                            <div className="text-sm font-medium text-foreground">{project.name}</div>
                                                            {group.kind === 'multiple' && (
                                                                <div className="text-xs text-muted-foreground">{projectClientNames(project)}</div>
                                                            )}
                                                            <div className="text-sm text-muted-foreground line-clamp-1">{project.description}</div>
                                                        </td>
                                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-foreground">
                                                            {formatPricing(project)}
                                                        </td>
                                                        <td className="px-6 py-4 whitespace-nowrap">
                                                            <span className={`inline-flex px-2 py-1 text-xs font-semibold rounded-full ${statusStyles[project.status] || statusStyles.active}`}>
                                                                {project.status}
                                                            </span>
                                                        </td>
                                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-muted-foreground">
                                                            {project.created_by_name || '-'}
                                                        </td>
                                                        <td className="px-6 py-4 whitespace-nowrap text-right text-sm space-x-4">
                                                            {isAdmin(user) && (
                                                                <button
                                                                    onClick={() => {
                                                                        setSelectedProject(project)
                                                                        setShowEditProject(true)
                                                                    }}
                                                                    className="text-muted-foreground hover:text-foreground font-medium"
                                                                >
                                                                    Szerkesztés
                                                                </button>
                                                            )}
                                                            <button
                                                                onClick={() => router.push(`/dashboard/board/${project.id}`)}
                                                                className="text-primary hover:text-primary/80 font-medium"
                                                            >
                                                                Megnyitás
                                                            </button>
                                                        </td>
                                                    </tr>
                                                ))}
                                            </tbody>
                                        </table>
                                    </div>
                                )}
                            </section>
                        )
                    })}
                </div>
            )}
        </div>
    )
}
```

**Megjegyzések az implementernek:**
- A `groups.length > 0` őr az `allCollapsed`-ban: ha a lista üres, a gomb úgyis rejtve van.
- Az első rendereléskor (a `localStorage` olvasása előtt) minden csoport nyitva látszik (`!collapsedLoaded`), hogy ne villanjon be egy üres állapot; a betöltés után az elmentett állapot érvényesül.
- A `Button` `variant="secondary"` és `size="sm"` a meglévő komponensben létezik (a profitabilitási oldalak használják); ha a fordító mást jelez, igazítsd minimálisan, és jelezd.
- Ha az eslint `react-hooks` szabály a `setCollapsed` hívás miatt panaszkodik egy effektben, **ne** némítsd el: jelezd a pontos üzenetet, és használj a viselkedést nem változtató megoldást (pl. lusta kezdeti állapot nem lehet, mert SSR; az effekt a helyes minta).

- [ ] **Step 2: Típusellenőrzés, lint, fordulás**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cd /app && echo "tsc az érintett fájlokban: $(npx tsc --noEmit -p . 2>&1 | grep -cE "board/page|groupProjects")"; echo "tsc összes régi hiba: $(npx tsc --noEmit -p . 2>&1 | grep -c "error TS")"; npx eslint src/app/dashboard/board/page.tsx src/utils/groupProjects.ts > /tmp/zz_e.out 2>&1; echo "eslint exit: $? sorok: $(wc -l < /tmp/zz_e.out)"; head -8 /tmp/zz_e.out; rm -f /tmp/zz_e.out'`
Expected: `tsc az érintett fájlokban: 0`, összes régi hiba `6`, `eslint exit: 0 sorok: 0`.

Run: `curl -s -o /dev/null -m 20 -w "oldal: HTTP %{http_code}\n" http://localhost:3010/dashboard/board; docker compose -f .docker/docker-compose.yml logs --tail 40 frontend 2>&1 | grep -iE "error|failed to compile|Module not found" | tail -5 || true`
Expected: `HTTP 200` (vagy átirányítás a belépésre), fordítási hiba nélkül. A 200 csak azt jelzi, hogy az oldal fordul és kiszolgálódik, azt nem, hogy a lista helyesen jelenik meg.

- [ ] **Step 3: Ellenőrizd, hogy a többi `useProjects` / `getAllProjects` fogyasztót nem érinti**

Run: `git diff --stat HEAD -- frontend/src | cat; grep -rn "getAllProjects\|useProjects()" frontend/src | grep -v "hooks/useProjects.ts\|projectsService.ts\|app/dashboard/board/page.tsx" | cut -c1-120`
Expected: a módosított frontend fájlok csak a `board/page.tsx` és a `groupProjects.ts`; a többi fogyasztó (Számlázás oldal, főoldal, e-mail → feladat ablak) változatlan.

- [ ] **Step 4: Commit** (csak kérésre)

```bash
git add frontend/src/app/dashboard/board/page.tsx
git commit -m "feat(projects): group the project list by client in collapsible blocks"
```

---

### Task 4: Végellenőrzés

- [ ] **Step 1: Backend és frontend együtt**

Run: `docker compose -f .docker/docker-compose.yml exec -T backend sh -c 'cd /app && gofmt -l internal/handlers/project_handler.go; go vet ./internal/... >/dev/null 2>&1; echo "vet exit: $?"; go build ./... >/dev/null 2>&1; echo "build exit: $?"; out=""; for p in models services handlers middleware; do r=$(go test -count=1 ./internal/$p/ 2>&1 | tail -1 | cut -d" " -f1); out="$out $p=$r"; done; echo "EREDMÉNY:$out"'`
Expected: `gofmt -l` üres, `vet exit: 0`, `build exit: 0`, mind a négy csomag `ok`.

Run a frontend ellenőrzést a Task 3 2. lépése szerint (az összes régi `tsc` hiba `6`).

- [ ] **Step 2: A végpont a futó backenden**

Run: `sleep 8; curl -s -o /dev/null -m 10 -w "GET /projects token nélkül (várt 401): HTTP %{http_code}\n" http://localhost:8080/api/v1/projects/`
Expected: `HTTP 401` (a végpont védett, és a backend újraindult és fut).

- [ ] **Step 3: Kézi számítás a dev adattal**

A dev adatbázisban mindkét projekt (Golfrange online, Mentorfy) egyetlen ügyfélhez, a PIXEL STÚDIÓ Kft-hez tartozik. A várt oldal: **egy** csoport („PIXEL STÚDIÓ Kft", 2 projekt), „Több ügyfél" és „Ügyfél nélkül" csoport nincs, az összesítő „2 projekt, 1 csoport". Olvasd ki a tényleges `project_clients` sorokat (psql, csak olvasás), írd le a várt eredményt, és vesd össze a Task 1 4. lépésének kimenetével.

- [ ] **Step 4: Böngészős ellenőrzés (ha van belépés)**

Jelentkezz be, és nyisd meg a `http://localhost:3010/dashboard/board` oldalt. Próbáld ki: a csoport fejléce nyit/csuk; a „Mind becsuk / Mind lenyit" működik; frissítés után a csukott állapot megmarad; a „Szerkesztés" a modalt nyitja, a „Megnyitás" a projekt oldalára visz; új projekt hozzárendelés után az ügyfél alatt jelenik meg. Ha nincs belépési adat, ezt **ne állítsd ellenőrzöttnek**: írd le, hogy a vizuális és interaktív rész nem volt kipróbálva.

---

## Self-Review

**Követelmény-lefedettség:**
- Lenyitható menü, ügyfél szerint csoportosítva, egy ügyfélnek több projekt → Task 2–3 ✔
- C: több ügyfeles projektek külön csoportban, a sor mutatja a neveket; ügyfél nélküli külön csoport a végén → Task 2–3 ✔
- A: alapból nyitva, állapot megmarad (ügyfél-azonosítóra kulcsolva), „Mind lenyit/becsuk" → Task 3 ✔
- Az összesítő az egyedi projektek számát mutatja → Task 3 (`projectCount = projects.length`; a csoportok darabszáma külön szerepel) ✔
- A backend `clients` mező kitöltése egy kötegelt lekérdezéssel → Task 1 ✔
- A meglévő működés (szerkesztés, megnyitás, jogosultság) változatlan → Task 3 (szó szerint megmarad) ✔

**Eltérések / megjegyzések a felhasználónak:**
1. Az összesítő most „N projekt, M csoport" formájú (az egyedi projektek száma és a csoportok száma), nem a régi „N projects".
2. Az `Edit`/`Open` gombok **Szerkesztés/Megnyitás** lettek.
3. A `HUF/hr`, `HUF fixed` árazási szövegek változatlanok (nem ennek a feladatnak a része).

**Placeholder-ellenőrzés:** nincs TBD/TODO; minden kódlépésben teljes kód van. A Task 1 2. lépése pontos beillesztendő kódot ad, de az implementernek előbb el kell olvasnia a `GetAllProjects` jelenlegi szerkezetét (az `err` változó deklarációja miatt).

**Típus-konzisztencia:** `ProjectGroup { key, kind, clientId, label, projects }`, `groupProjectsByClient`, `projectClientNames`, `MULTIPLE_LABEL`, `NONE_LABEL` minden taskban azonosak. A Go `ProjectClientResponse` JSON-címkéi megegyeznek a TS `ProjectClient` mezőivel (a meglévő típus).

**Nem ellenőrzött feltevések:**
- Hogy a `Intl.Collator('hu')` a konténer Node-jában ICU-val fut (a Task 2 3. lépése jelzi, ha nem).
- Hogy a `Button` `variant="secondary"` / `size="sm"` létezik (a Task 3 jegyzete szerint ellenőrizendő).
- A vizuális megjelenés, a nyitás/csukás, a megmaradó állapot és az üres/egy csoportos viselkedés böngészőben nem ellenőrizhető bejelentkezés nélkül.
