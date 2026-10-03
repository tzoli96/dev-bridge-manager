# Dashboard keret (navigáció és header) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A dashboard keretének (felső menü és header) átalakítása: csoportosított, magyar feliratú menü, ami nagy képernyőn vízszintes sor (szükség esetén csoportok mentén két sorba törve), kis képernyőn pedig egy menü gombról nyíló bal oldali fiók; az aktív menüpont helyes jelölése almenü-oldalakon is.

**Architecture:** Három kis, önálló egység: egy függőségmentes aktív-menüpont logika (`navActive.ts`, külön ellenőrizhető a Next.js nélkül), a menü adatmodellje a jogosultsági szabályokkal (`navigation.ts`), és a megjelenítés (`DashboardNav.tsx` + `MobileNavDrawer.tsx`). A fiók a projekt meglévő Radix alapú `dialog` primitívjeire épül, új függőség nélkül. A főoldal tartalmát ez a terv **nem** érinti (az külön terv).

**Tech Stack:** Next.js (App Router) + TypeScript + Tailwind v4 + `radix-ui` + `lucide-react` (frontend konténer).

**Spec:** nincs külön spec-fájl; a követelményeket a brainstorming döntései rögzítik (lent, „Döntések").

## Global Constraints

- **Csak frontend.** Backend, migráció, jog és adat nem változik. A jogosultsági szabályok **pontosan ugyanazok**, mint ma: aki ma nem lát egy menüpontot, azt továbbra sem látja; az üres csoport nem jelenik meg.
- A menüpontok útvonalai változatlanok. Csak a sorrend (csoportok), a feliratok (magyar) és a megjelenés változik.
- **Új függőség tilos.** A fiókhoz a meglévő `radix-ui` Dialog primitívjeit használjuk (`@/components/ui/dialog` részei + `DialogPrimitive.Content`).
- A frontendnek **nincs tesztfuttatója**. Az ellenőrzés: `tsc`, `eslint` a konténerben, az oldalak fordulása, és a tiszta logika (`navActive.ts`) ideiglenes, a konténerben futó Node-szkripttel (`node --experimental-strip-types`), amit a repón **kívül** (`/tmp`) hozunk létre és törlünk.
- Minden ellenőrzés a projekt konténereiben fut, a repo gyökeréből: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c '…'`. Host-oldali futtatás tilos.
- **Commit, merge és push csak a felhasználó kifejezett kérésére.** A terv commit-lépései addig nem futnak le. `git add` csak a felsorolt fájlokra.
- A working tree-ben **idegen, nem commitolt munka** van: a Jira-integráció (`activity_digest_handler.go`, `jira_*`, `task_comment_handler.go`, `models/kanban.go`, `services/jira_*`, `migrations/000040_*`, `docs/.../marketing-contacts.md`) és a számlázatlan órák funkció (`unbilled_hours*.go`, `billing/unbilled/`, `unbilledHoursService.ts`, a `billing/page.tsx` és az `invoice_routes.go` módosítása, a hozzá tartozó terv). Ezekhez nem nyúlunk, és nem kerülhetnek a commitba.
- A 6 régi `tsc` hiba más fájlokban (pl. `EditProfileModal`, `UserCard`, `task-card`, `time-tracker`, `lib/api.ts`) és a `billing/page.tsx` 8 régi `no-explicit-any` hibája nem ennek a tervnek a része.
- Munkabranch: `feat/dashboard-frame`. Az ideiglenes fájlok neve `zz_tmp_*` vagy `/tmp/zz_*`; jelentés előtt törlendők.

## Döntések (Rulings a tervben; a felhasználónak jelezni)

1. **Töréspont: `xl` (1280 px).** Ez alatt a fiók, felette a vízszintes sor.
2. **A vízszintes sor két sorba törhet, csoportok mentén.** A 10 magyar feliratú menüpont (super adminnak) a becslésem szerint nem fér el egy sorban a jelenlegi `max-w-7xl` konténerben (1216 px használható), ezért a csoportok egységként törnek a második sorba. Ez nem függ a pontos betűszélességtől. A becslés nincs böngészőben mérve.
3. **A negyedik csoport neve „Rendszer",** nem „Adminisztráció", mert egy csoport és egy menüpont ugyanazzal a névvel félreérthető. A csoportok: **Munka** (Irányítópult, Projektek, Ügyfelek), **Pénzügy** (Számlázás, Jövedelmezőség), **Kommunikáció** (E-mailek, Marketing lista), **Rendszer** (Csapat, Adminisztráció, Álláskeresés).
4. **Az aktív menüpont útvonal-előtag alapú** (nem pontos egyezés): a `/dashboard/board/13/invoice` oldalon a „Projektek" az aktív. Kivétel: az „Irányítópult" csak pontos egyezésnél, az „Adminisztráció" pedig nem aktív a `/dashboard/admin/job-search` alatt (az az „Álláskeresés").
5. **Csoportcímkék csak a fiókban látszanak,** a vízszintes sorban csak a térköz választja el a csoportokat (elválasztó vonal sortörésnél a sor elejére kerülne).
6. **Magyar feliratok** a menüben és a headerben („Kijelentkezés", „Projektkezelés"). A főoldal szövegei a második tervben jönnek.
7. **Az olvasatlan e-mail számláló** a vízszintes soron, a fiókban és a fiók gombján is látszik (a gombon pont jelzés, nem szám).

## File Structure

| Fájl | Felelősség |
|---|---|
| `frontend/src/components/dashboard/navActive.ts` | Függőségmentes: `isNavActive`, `activeNavItem` |
| `frontend/src/components/dashboard/navigation.ts` | Menü adatmodell és jogosultsági szabályok: `buildNavGroups` |
| `frontend/src/components/dashboard/NavBadge.tsx` | Az olvasatlan számláló jelvénye (közös) |
| `frontend/src/components/dashboard/MobileNavDrawer.tsx` | A bal oldali fiók |
| `frontend/src/components/dashboard/DashboardNav.tsx` | **Újraírva:** vízszintes sor + mobil sáv + fiók bekötése |
| `frontend/src/components/dashboard/DashboardHeader.tsx` | **Módosul:** magyar feliratok, hozzáférhető kijelentkezés gomb |

---

### Task 1: Az aktív menüpont logikája (függőségmentes)

**Files:**
- Create: `frontend/src/components/dashboard/navActive.ts`

**Interfaces:**
- Produces:
  - `interface NavMatch { href: string; exact?: boolean; excludes?: string[] }`
  - `function isNavActive(pathname: string, item: NavMatch): boolean`
  - `function activeNavItem<T extends NavMatch>(items: T[], pathname: string): T | null` (a leghosszabb illeszkedő `href` nyer)

- [ ] **Step 1: Írd meg az ellenőrző szkriptet (a repón kívül), és ellenőrizd, hogy elbukik**

Run (a fájl még nem létezik, ezért importhibával bukik):
```bash
docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cat > /tmp/zz_navcheck.mts <<EOF
import { isNavActive, activeNavItem } from "/app/src/components/dashboard/navActive.ts";

let failed = 0;
const eq = (actual: unknown, expected: unknown, name: string) => {
  if (actual !== expected) { failed++; console.log("FAIL", name, "got", actual, "want", expected); }
};

const dash = { href: "/dashboard", exact: true };
const board = { href: "/dashboard/board" };
const billing = { href: "/dashboard/billing" };
const admin = { href: "/dashboard/admin", excludes: ["/dashboard/admin/job-search"] };
const jobs = { href: "/dashboard/admin/job-search" };
const users = { href: "/users" };

eq(isNavActive("/dashboard", dash), true, "dashboard exact");
eq(isNavActive("/dashboard/board", dash), false, "dashboard exact does not match children");
eq(isNavActive("/dashboard/board", board), true, "board self");
eq(isNavActive("/dashboard/board/13", board), true, "board child");
eq(isNavActive("/dashboard/board/13/invoice", board), true, "board grandchild");
eq(isNavActive("/dashboard/boardx", board), false, "prefix needs a slash boundary");
eq(isNavActive("/dashboard/board/", board), true, "trailing slash");
eq(isNavActive("/dashboard/billing/unbilled", billing), true, "billing child");
eq(isNavActive("/dashboard/admin", admin), true, "admin self");
eq(isNavActive("/dashboard/admin/roles", admin), true, "admin child");
eq(isNavActive("/dashboard/admin/job-search", admin), false, "admin excludes job-search");
eq(isNavActive("/dashboard/admin/job-search/5", admin), false, "admin excludes job-search child");
eq(isNavActive("/dashboard/admin/job-search", jobs), true, "job-search self");
eq(isNavActive("/users", users), true, "users self");
eq(isNavActive("/users/5", users), true, "users child");
eq(isNavActive("/usersx", users), false, "users boundary");
eq(isNavActive("", dash), false, "empty pathname");

const items = [dash, board, admin, jobs, users];
eq(activeNavItem(items, "/dashboard/admin/job-search")?.href, "/dashboard/admin/job-search", "single active item for job-search");
eq(activeNavItem(items, "/dashboard/board/13")?.href, "/dashboard/board", "board child resolves to board");
eq(activeNavItem(items, "/dashboard")?.href, "/dashboard", "root resolves to dashboard");
eq(activeNavItem(items, "/somewhere/else"), null, "unknown path has no active item");
eq(activeNavItem([board, { href: "/dashboard/board/special" }], "/dashboard/board/special/1")?.href, "/dashboard/board/special", "longest href wins");

console.log(failed ? "RESULT FAIL " + failed : "RESULT OK");
EOF
node --no-warnings --experimental-strip-types /tmp/zz_navcheck.mts 2>&1 | grep -E "RESULT|FAIL|Cannot find|ERR_" | head -5'
```
Expected: `Cannot find module` / `ERR_MODULE_NOT_FOUND` (a fájl még nincs).

- [ ] **Step 2: Írd meg a függőségmentes logikát**

```ts
// frontend/src/components/dashboard/navActive.ts
// Dependency-free on purpose: it can be checked with plain Node, without the
// Next.js toolchain, and it keeps the "which menu item is active" rule in one place.

export interface NavMatch {
    href: string
    /** Match only the exact path (e.g. the dashboard root). */
    exact?: boolean
    /** Sub-paths of href that belong to a different menu item. */
    excludes?: string[]
}

const trimTrailingSlash = (path: string) => (path.length > 1 && path.endsWith('/') ? path.slice(0, -1) : path)

const isPathOrChild = (path: string, base: string) => path === base || path.startsWith(base + '/')

export function isNavActive(pathname: string, item: NavMatch): boolean {
    const path = trimTrailingSlash(pathname)
    if (item.exact) return path === item.href
    if (!isPathOrChild(path, item.href)) return false
    return !(item.excludes ?? []).some((excluded) => isPathOrChild(path, excluded))
}

/** The active item; when several match, the one with the longest href wins. */
export function activeNavItem<T extends NavMatch>(items: T[], pathname: string): T | null {
    let best: T | null = null
    for (const item of items) {
        if (!isNavActive(pathname, item)) continue
        if (best === null || item.href.length > best.href.length) best = item
    }
    return best
}
```

- [ ] **Step 3: Futtasd az ellenőrzést, és ellenőrizd, hogy átmegy**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'node --no-warnings --experimental-strip-types /tmp/zz_navcheck.mts 2>&1 | grep -E "RESULT|FAIL|Cannot find|ERR_" | head -8'`
Expected: `RESULT OK`, `FAIL` sor nélkül. Ha egy eset bukik, **ne** a szkriptet lazítsd: állj meg, és jelentsd a pontos esetet és az eredményt.

- [ ] **Step 4: Töröld az ideiglenes szkriptet, típusellenőrzés és lint**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cd /app && rm -f /tmp/zz_navcheck.mts && ls /tmp/zz_navcheck.mts 2>&1 | head -1; npx tsc --noEmit -p . 2>&1 | grep -E "navActive" || echo "tsc: nincs hiba az érintett fájlban"; npx eslint src/components/dashboard/navActive.ts 2>&1 | tail -4; echo eslint-done'`
Expected: `No such file` (a szkript törölve), `tsc: nincs hiba az érintett fájlban`, üres eslint-kimenet.

- [ ] **Step 5: Commit** (csak kérésre)

```bash
git add frontend/src/components/dashboard/navActive.ts
git commit -m "feat(dashboard): add pure active-nav-item logic"
```

---

### Task 2: A menü adatmodellje és a közös számláló-jelvény

**Files:**
- Create: `frontend/src/components/dashboard/navigation.ts`
- Create: `frontend/src/components/dashboard/NavBadge.tsx`

**Interfaces:**
- Consumes: `NavMatch` (Task 1); `hasAnyPermission`, `isSuperAdmin` (`@/utils/permissions`); `User` (`@/types/user`); lucide ikonok.
- Produces:
  - `interface NavItem extends NavMatch { label: string; icon: LucideIcon; badge?: number }`
  - `interface NavGroup { id: string; label: string; items: NavItem[] }`
  - `function buildNavGroups(user: User | null, unreadCount: number): NavGroup[]` (csak a látható elemekkel; az üres csoport kimarad)
  - `default function NavBadge({ count }: { count: number })`

- [ ] **Step 1: Írd meg a menü adatmodelljét**

A jogosultsági feltételek **szó szerint** a mai `DashboardNav.tsx`-ből valók.

```ts
// frontend/src/components/dashboard/navigation.ts
import type { LucideIcon } from 'lucide-react'
import {
    LayoutDashboard,
    KanbanSquare,
    Users,
    Building2,
    ShieldCheck,
    Mail,
    Receipt,
    Briefcase,
    Contact,
    TrendingUp,
} from 'lucide-react'
import type { User } from '@/types/user'
import { hasAnyPermission, isSuperAdmin } from '@/utils/permissions'
import type { NavMatch } from './navActive'

export interface NavItem extends NavMatch {
    label: string
    icon: LucideIcon
    badge?: number
}

export interface NavGroup {
    id: string
    label: string
    items: NavItem[]
}

const when = (show: boolean, item: NavItem): NavItem[] => (show ? [item] : [])

/**
 * The menu, grouped. Visibility rules are exactly the ones the flat menu had;
 * a group with no visible item is dropped.
 */
export function buildNavGroups(user: User | null, unreadCount: number): NavGroup[] {
    const canManageGmail = hasAnyPermission(user, ['gmail.manage'])

    const groups: NavGroup[] = [
        {
            id: 'work',
            label: 'Munka',
            items: [
                ...when(true, { href: '/dashboard', exact: true, label: 'Irányítópult', icon: LayoutDashboard }),
                ...when(true, { href: '/dashboard/board', label: 'Projektek', icon: KanbanSquare }),
                ...when(hasAnyPermission(user, ['clients.list', 'clients.read']), {
                    href: '/dashboard/clients',
                    label: 'Ügyfelek',
                    icon: Building2,
                }),
            ],
        },
        {
            id: 'finance',
            label: 'Pénzügy',
            items: [
                ...when(hasAnyPermission(user, ['invoices.read']), {
                    href: '/dashboard/billing',
                    label: 'Számlázás',
                    icon: Receipt,
                }),
                ...when(hasAnyPermission(user, ['profitability.read']), {
                    href: '/dashboard/profitability',
                    label: 'Jövedelmezőség',
                    icon: TrendingUp,
                }),
            ],
        },
        {
            id: 'communication',
            label: 'Kommunikáció',
            items: [
                ...when(canManageGmail, {
                    href: '/dashboard/emails',
                    label: 'E-mailek',
                    icon: Mail,
                    badge: unreadCount > 0 ? unreadCount : undefined,
                }),
                ...when(true, { href: '/dashboard/marketing-contacts', label: 'Marketing lista', icon: Contact }),
            ],
        },
        {
            id: 'system',
            label: 'Rendszer',
            items: [
                ...when(hasAnyPermission(user, ['users.list', 'users.read']), {
                    href: '/users',
                    label: 'Csapat',
                    icon: Users,
                }),
                ...when(hasAnyPermission(user, ['system.settings', 'roles.list']), {
                    href: '/dashboard/admin',
                    excludes: ['/dashboard/admin/job-search'],
                    label: 'Adminisztráció',
                    icon: ShieldCheck,
                }),
                ...when(isSuperAdmin(user), {
                    href: '/dashboard/admin/job-search',
                    label: 'Álláskeresés',
                    icon: Briefcase,
                }),
            ],
        },
    ]

    return groups.filter((group) => group.items.length > 0)
}
```

- [ ] **Step 2: Írd meg a közös jelvényt**

A kinézet szó szerint a mai jelvényé.

```tsx
// frontend/src/components/dashboard/NavBadge.tsx
interface NavBadgeProps {
    count: number
}

export default function NavBadge({ count }: NavBadgeProps) {
    return (
        <span className="flex items-center justify-center min-w-[18px] h-[18px] px-1 rounded-full bg-red-500 text-white text-[10px] font-semibold leading-none">
            {count > 99 ? '99+' : count}
        </span>
    )
}
```

- [ ] **Step 3: Típusellenőrzés és lint**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cd /app && npx tsc --noEmit -p . 2>&1 | grep -E "navigation|NavBadge" || echo "tsc: nincs hiba az érintett fájlokban"; npx eslint src/components/dashboard/navigation.ts src/components/dashboard/NavBadge.tsx 2>&1 | tail -6; echo eslint-done'`
Expected: `tsc: nincs hiba az érintett fájlokban`, üres eslint-kimenet.

- [ ] **Step 4: Commit** (csak kérésre)

```bash
git add frontend/src/components/dashboard/navigation.ts frontend/src/components/dashboard/NavBadge.tsx
git commit -m "feat(dashboard): add grouped navigation model"
```

---

### Task 3: A bal oldali fiók

**Files:**
- Create: `frontend/src/components/dashboard/MobileNavDrawer.tsx`

**Interfaces:**
- Consumes: `NavGroup` (Task 2); `isNavActive` (Task 1); `NavBadge` (Task 2); `DialogOverlay`, `DialogPortal`, `DialogTitle` (`@/components/ui/dialog`); `Dialog as DialogPrimitive` (`radix-ui`).
- Produces: `default function MobileNavDrawer(props: { open: boolean; onOpenChange: (open: boolean) => void; groups: NavGroup[]; pathname: string })`.

**Megjegyzés a meglévő `dialog.tsx`-ről:** a `DialogContent` középre igazított felugró ablak, ezért nem használható fiókként. A `DialogOverlay`-t és a `DialogPortal`-t újrahasznosítjuk, a tartalmat pedig közvetlenül a `DialogPrimitive.Content`-tel írjuk. A `dialog.tsx` `data-open:` / `data-closed:` osztályai működnek: a projekt a `shadcn/tailwind.css`-t importálja (`globals.css`), ami ezeket a variánsokat `[data-state="open"]` / `[data-state="closed"]` szelektorra képezi le (a Radix `data-state` attribútumot állít be). A fiók a közvetlen `data-[state=open]:` / `data-[state=closed]:` változatokat használja, ami ugyanezt jelenti, és nem függ a projekt egyedi variánsaitól.

- [ ] **Step 1: Írd meg a fiókot**

```tsx
// frontend/src/components/dashboard/MobileNavDrawer.tsx
'use client'

import Link from 'next/link'
import { Dialog as DialogPrimitive } from 'radix-ui'
import { X } from 'lucide-react'
import { DialogOverlay, DialogPortal, DialogTitle } from '@/components/ui/dialog'
import NavBadge from '@/components/dashboard/NavBadge'
import { isNavActive } from '@/components/dashboard/navActive'
import type { NavGroup } from '@/components/dashboard/navigation'

interface MobileNavDrawerProps {
    open: boolean
    onOpenChange: (open: boolean) => void
    groups: NavGroup[]
    pathname: string
}

export default function MobileNavDrawer({ open, onOpenChange, groups, pathname }: MobileNavDrawerProps) {
    return (
        <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
            <DialogPortal>
                <DialogOverlay />
                <DialogPrimitive.Content
                    aria-describedby={undefined}
                    className="fixed inset-y-0 left-0 z-50 flex w-72 max-w-[85vw] flex-col gap-4 overflow-y-auto border-r border-border bg-card p-4 shadow-xl outline-none duration-200 xl:hidden data-[state=open]:animate-in data-[state=open]:slide-in-from-left data-[state=closed]:animate-out data-[state=closed]:slide-out-to-left"
                >
                    <div className="flex items-center justify-between">
                        <DialogTitle className="text-base font-semibold text-foreground">Menü</DialogTitle>
                        <DialogPrimitive.Close
                            aria-label="Menü bezárása"
                            className="flex h-8 w-8 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                        >
                            <X size={18} />
                        </DialogPrimitive.Close>
                    </div>

                    <nav aria-label="Fő navigáció" className="flex flex-col gap-5">
                        {groups.map((group) => (
                            <div key={group.id}>
                                <p className="px-3 pb-1 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                    {group.label}
                                </p>
                                <ul className="flex flex-col gap-0.5">
                                    {group.items.map((item) => {
                                        const isActive = isNavActive(pathname, item)
                                        const Icon = item.icon
                                        return (
                                            <li key={item.href}>
                                                <Link
                                                    href={item.href}
                                                    onClick={() => onOpenChange(false)}
                                                    aria-current={isActive ? 'page' : undefined}
                                                    className={`flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors ${
                                                        isActive
                                                            ? 'bg-primary/10 text-primary'
                                                            : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                                                    }`}
                                                >
                                                    <Icon size={18} />
                                                    <span className="flex-1">{item.label}</span>
                                                    {item.badge !== undefined && <NavBadge count={item.badge} />}
                                                </Link>
                                            </li>
                                        )
                                    })}
                                </ul>
                            </div>
                        ))}
                    </nav>
                </DialogPrimitive.Content>
            </DialogPortal>
        </DialogPrimitive.Root>
    )
}
```

- [ ] **Step 2: Típusellenőrzés és lint**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cd /app && npx tsc --noEmit -p . 2>&1 | grep -E "MobileNavDrawer" || echo "tsc: nincs hiba az érintett fájlban"; npx eslint src/components/dashboard/MobileNavDrawer.tsx 2>&1 | tail -6; echo eslint-done'`
Expected: `tsc: nincs hiba az érintett fájlban`, üres eslint-kimenet. Ha a `radix-ui` `Dialog` exportja vagy a `DialogOverlay`/`DialogPortal` import más néven létezik, nézd meg a `frontend/src/components/ui/dialog.tsx` elejét, igazítsd minimálisan, és jelezd.

- [ ] **Step 3: Commit** (csak kérésre)

```bash
git add frontend/src/components/dashboard/MobileNavDrawer.tsx
git commit -m "feat(dashboard): add mobile navigation drawer"
```

---

### Task 4: A navigáció újraírása (vízszintes sor, mobil sáv, fiók)

**Files:**
- Modify (újraírás): `frontend/src/components/dashboard/DashboardNav.tsx`

**Interfaces:**
- Consumes: `buildNavGroups` (Task 2); `isNavActive`, `activeNavItem` (Task 1); `NavBadge` (Task 2); `MobileNavDrawer` (Task 3); `EmailsService.getUnreadCount()`; `hasAnyPermission`.
- Produces: `default function DashboardNav({ user }: { user: User | null })` (ugyanaz a nyilvános felület, mint ma, a `layout.tsx` változatlanul használja).

**Előkészület:** olvasd el a jelenlegi `DashboardNav.tsx`-t. Az olvasatlan számláló lekérdezése (`EmailsService.getUnreadCount()`, 30 másodperces polling, `canManageGmail` őr, `cancelled` jelző) **szó szerint megmarad**, csak a megjelenítés változik.

- [ ] **Step 1: Írd át a komponenst**

```tsx
// frontend/src/components/dashboard/DashboardNav.tsx
'use client'

import { useEffect, useMemo, useState } from 'react'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { Menu } from 'lucide-react'
import { User } from '@/types/user'
import { hasAnyPermission } from '@/utils/permissions'
import { EmailsService } from '@/services/emailsService'
import { activeNavItem, isNavActive } from '@/components/dashboard/navActive'
import { buildNavGroups } from '@/components/dashboard/navigation'
import NavBadge from '@/components/dashboard/NavBadge'
import MobileNavDrawer from '@/components/dashboard/MobileNavDrawer'

interface DashboardNavProps {
    user: User | null
}

const UNREAD_POLL_INTERVAL_MS = 30_000

export default function DashboardNav({ user }: DashboardNavProps) {
    const pathname = usePathname() ?? ''
    const [unreadCount, setUnreadCount] = useState(0)
    const [drawerOpen, setDrawerOpen] = useState(false)
    const canManageGmail = hasAnyPermission(user, ['gmail.manage'])

    useEffect(() => {
        if (!canManageGmail) return

        let cancelled = false
        const fetchUnreadCount = () => {
            EmailsService.getUnreadCount()
                .then(count => { if (!cancelled) setUnreadCount(count) })
                .catch(() => {})
        }

        fetchUnreadCount()
        const interval = setInterval(fetchUnreadCount, UNREAD_POLL_INTERVAL_MS)
        return () => { cancelled = true; clearInterval(interval) }
    }, [canManageGmail])

    // Close the drawer whenever the route changes.
    useEffect(() => {
        setDrawerOpen(false)
    }, [pathname])

    const groups = useMemo(() => buildNavGroups(user, unreadCount), [user, unreadCount])
    const currentLabel = activeNavItem(groups.flatMap((group) => group.items), pathname)?.label ?? 'Menü'

    return (
        <div className="bg-card border-b border-border">
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                {/* Wide screens: the groups are units, so a second row starts at a group boundary. */}
                <nav aria-label="Fő navigáció" className="hidden xl:flex flex-wrap items-center gap-x-5 gap-y-1 py-3">
                    {groups.map((group) => (
                        <div key={group.id} className="flex items-center gap-1.5">
                            {group.items.map((item) => {
                                const isActive = isNavActive(pathname, item)
                                const Icon = item.icon
                                return (
                                    <Link
                                        key={item.href}
                                        href={item.href}
                                        aria-current={isActive ? 'page' : undefined}
                                        className={`flex items-center gap-2 px-3 py-2 rounded-lg font-medium text-sm transition-colors ${
                                            isActive
                                                ? 'bg-primary/10 text-primary'
                                                : 'text-muted-foreground hover:text-foreground hover:bg-muted'
                                        }`}
                                    >
                                        <Icon size={16} />
                                        {item.label}
                                        {item.badge !== undefined && <NavBadge count={item.badge} />}
                                    </Link>
                                )
                            })}
                        </div>
                    ))}
                </nav>

                {/* Narrow screens: a slim bar with the menu button and the current page. */}
                <div className="flex items-center gap-3 py-2 xl:hidden">
                    <button
                        type="button"
                        onClick={() => setDrawerOpen(true)}
                        aria-label="Menü megnyitása"
                        aria-expanded={drawerOpen}
                        className="relative flex h-9 w-9 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                    >
                        <Menu size={20} />
                        {unreadCount > 0 && canManageGmail && (
                            <span
                                aria-hidden="true"
                                className="absolute right-1.5 top-1.5 h-2 w-2 rounded-full bg-red-500"
                            />
                        )}
                    </button>
                    <span className="truncate text-sm font-medium text-foreground">{currentLabel}</span>
                </div>
            </div>

            <MobileNavDrawer open={drawerOpen} onOpenChange={setDrawerOpen} groups={groups} pathname={pathname} />
        </div>
    )
}
```

- [ ] **Step 2: Típusellenőrzés és lint**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cd /app && npx tsc --noEmit -p . 2>&1 | grep -E "DashboardNav|MobileNavDrawer|navigation|navActive|NavBadge" || echo "tsc: nincs hiba az érintett fájlokban"; echo "tsc összes régi hiba: $(npx tsc --noEmit -p . 2>&1 | grep -c "error TS")"; npx eslint src/components/dashboard/DashboardNav.tsx src/components/dashboard/MobileNavDrawer.tsx src/components/dashboard/navigation.ts src/components/dashboard/navActive.ts src/components/dashboard/NavBadge.tsx 2>&1 | tail -8; echo eslint-done'`
Expected: `tsc: nincs hiba az érintett fájlokban`, az összes régi hiba száma `6`, üres eslint-kimenet. Ha az eslint a `useEffect` + `setDrawerOpen` miatt `react-hooks` szabályt jelez, **ne** némítsd el a szabályt: jelezd a pontos sort, és javasolj egy ekvivalens megoldást (pl. a fiók zárása a `Link` `onClick`-ben, amit a fiók már úgyis megtesz).

- [ ] **Step 3: Ellenőrizd, hogy semmi más nem hivatkozik a régi belső szerkezetre**

Run: `grep -rn "DashboardNav" frontend/src | grep -v "components/dashboard/DashboardNav.tsx"`
Expected: csak a `app/dashboard/layout.tsx` importja és használata (`<DashboardNav user={user} />`); a nyilvános felület nem változott, így a `layout.tsx`-hez nem kell nyúlni.

- [ ] **Step 4: Commit** (csak kérésre)

```bash
git add frontend/src/components/dashboard/DashboardNav.tsx
git commit -m "feat(dashboard): grouped navigation with a mobile drawer"
```

---

### Task 5: A header magyarosítása és a kijelentkezés gomb hozzáférhetősége

**Files:**
- Modify: `frontend/src/components/dashboard/DashboardHeader.tsx`

**Interfaces:** nincs változás a felületben (`DashboardHeader({ user, onLogout })`).

- [ ] **Step 1: Végezd el a három szövegcserét**

Olvasd el előbb a fájlt, majd **csak** ezeket változtasd:

1. A címsor alatti leírás: `Project management` → `Projektkezelés`.
2. A kijelentkezés gomb látható szövege: `Logout` → `Kijelentkezés`.
3. A kijelentkezés gombhoz add hozzá az `aria-label="Kijelentkezés"` tulajdonságot (kis képernyőn csak az ikon látszik, annak nincs hozzáférhető neve). A gomb `onClick` és osztályai ne változzanak.

- [ ] **Step 2: Ellenőrizd a diffet**

Run: `git diff frontend/src/components/dashboard/DashboardHeader.tsx | grep -E '^[+-][^+-]'`
Expected: pontosan három eltérő sor (a leírás, a gomb szövege, az `aria-label` hozzáadása).

- [ ] **Step 3: Típusellenőrzés és lint**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cd /app && npx tsc --noEmit -p . 2>&1 | grep -E "DashboardHeader" || echo "tsc: nincs hiba az érintett fájlban"; npx eslint src/components/dashboard/DashboardHeader.tsx 2>&1 | tail -6; echo eslint-done'`
Expected: `tsc: nincs hiba az érintett fájlban`, üres eslint-kimenet.

- [ ] **Step 4: Commit** (csak kérésre)

```bash
git add frontend/src/components/dashboard/DashboardHeader.tsx
git commit -m "feat(dashboard): Hungarian header labels and accessible logout"
```

---

### Task 6: Végellenőrzés

- [ ] **Step 1: Teljes frontend-ellenőrzés konténerben**

Run: `docker compose -f .docker/docker-compose.yml exec -T frontend sh -c 'cd /app && npx tsc --noEmit -p . 2>&1 | grep -E "dashboard/|DashboardNav|DashboardHeader|navigation|navActive|MobileNavDrawer|NavBadge" || echo "tsc: nincs hiba az érintett fájlokban"; echo "tsc összes régi hiba: $(npx tsc --noEmit -p . 2>&1 | grep -c "error TS")"; npx eslint src/components/dashboard/DashboardNav.tsx src/components/dashboard/DashboardHeader.tsx src/components/dashboard/MobileNavDrawer.tsx src/components/dashboard/navigation.ts src/components/dashboard/navActive.ts src/components/dashboard/NavBadge.tsx 2>&1 | tail -8; echo eslint-done'`
Expected: `tsc: nincs hiba az érintett fájlokban`, az összes régi hiba `6`, üres eslint-kimenet.

- [ ] **Step 2: Futtasd újra az aktív-menüpont ellenőrzést**

Hozd létre újra a Task 1 1. lépésének szkriptjét (`/tmp/zz_navcheck.mts`), futtasd, majd töröld.
Expected: `RESULT OK`; a szkript törölve (`ls /tmp/zz_navcheck.mts` → `No such file`).

- [ ] **Step 3: Az oldalak fordulása**

Run: `for p in /dashboard /dashboard/board /dashboard/billing /dashboard/profitability /dashboard/emails /dashboard/clients; do curl -s -o /dev/null -m 20 -w "$p: HTTP %{http_code}\n" http://localhost:3010$p; done; docker compose -f .docker/docker-compose.yml logs --tail 40 frontend 2>&1 | grep -iE "error|failed to compile|Module not found" | tail -5 || true`
Expected: mind `HTTP 200` (vagy átirányítás a belépésre), fordítási hiba nélkül. A 200 csak azt jelzi, hogy az oldalak kiszolgálódnak és fordulnak; azt nem, hogy a menü helyesen jelenik meg.

- [ ] **Step 4: A jogosultsági szabályok megőrzése (olvasással)**

Hasonlítsd össze a `navigation.ts` `show` feltételeit a korábbi, lapos `DashboardNav.tsx`-ével (`git diff frontend/src/components/dashboard/DashboardNav.tsx` mutatja a régit). Minden menüponthoz írd le a feltételt a jelentésben: Irányítópult és Projektek mindenkinek; Ügyfelek `clients.list` vagy `clients.read`; Számlázás `invoices.read`; Jövedelmezőség `profitability.read`; E-mailek `gmail.manage`; Marketing lista mindenkinek; Csapat `users.list` vagy `users.read`; Adminisztráció `system.settings` vagy `roles.list`; Álláskeresés super admin. Ha bármelyik eltér a réginél, az **hiba**, jelezd.

- [ ] **Step 5: Böngészős ellenőrzés (ha van belépés)**

Jelentkezz be a `http://localhost:3010`-en, és próbáld ki: (a) 1280 px felett a csoportosított sor látszik, szükség esetén két sorban; (b) 1280 px alatt egy sáv jelenik meg a menü gombbal és az aktuális oldal nevével, a gomb a bal oldali fiókot nyitja; (c) a fiók bezárul navigáláskor, az Esc billentyűre és a háttérre kattintva; (d) a `/dashboard/board/<id>` oldalon a „Projektek" az aktív; (e) az olvasatlan e-mail számláló a soron és a fiókban, a gombon pedig pont; (f) a header feliratai magyarul. Ha nincs belépési adat, ezt **ne állítsd ellenőrzöttnek**: írd le, hogy a vizuális és interaktív rész nem volt kipróbálva.

---

## Self-Review

**Követelmény-lefedettség (a brainstorming döntései):**
- A: felső sor nagy képernyőn, kis képernyőn lenyíló fiók → Task 3–4 ✔
- Csoportok és sorrend, üres csoport nem látszik, jogosultság változatlan → Task 2 (és Task 6/4. lépés ellenőrzi) ✔
- Magyar feliratok a menüben és a headerben → Task 2, 5 ✔
- Az aktív menüpont javítása almenü-oldalakon, `Csapat`/`Álláskeresés` külön ágon → Task 1 (ellenőrző szkripttel) ✔
- Fiók bezárul navigáláskor, Esc-re, háttérre kattintva → Task 3 (Radix Dialog Esc/háttér) + Task 4 (útvonalváltás) ✔
- Az olvasatlan számláló a soron és a fiókban, a gombon pont → Task 2–4 ✔
- Új függőség nincs → ✔
- Nem része: a főoldal tartalma (külön terv).

**Eltérések az eddig mondottaktól (a felhasználónak jelezni):**
1. A vízszintes sor **két sorba törhet** (csoportok mentén), mert a 10 magyar feliratú menüpont becslésem szerint nem fér el egy sorban az 1216 px használható szélességen (Döntés 2). A becslés nincs mérve.
2. A negyedik csoport neve **„Rendszer"**, nem „Adminisztráció" (Döntés 3).
3. A csoportcímkék csak a fiókban látszanak (Döntés 5).

**Placeholder-ellenőrzés:** nincs TBD/TODO; minden kódlépésben teljes kód van. A Task 5 szövegcseréi a fájl elolvasása után végzendők, mert a header pontos sorai változhattak.

**Típus-konzisztencia:** `NavMatch` (Task 1) → `NavItem extends NavMatch` (Task 2) → `NavGroup` (Task 2) → a `MobileNavDrawer` és a `DashboardNav` ugyanezeket használja. A függvénynevek (`isNavActive`, `activeNavItem`, `buildNavGroups`) minden taskban azonosak. A `NavBadge` `count` tulajdonsága minden használatban szám.

**Nem ellenőrzött feltevések:**
- A `radix-ui` `Dialog` exportja és a `DialogOverlay`/`DialogPortal`/`DialogTitle` nevek a `dialog.tsx` olvasása alapján helyesek; a Task 3 2. lépése ellenőrzi a fordítással.
- A `data-[state=open]:slide-in-from-left` osztályok a `tw-animate-css` csomagból érkeznek (a projekt importálja); hogy valóban animálnak-e, csak böngészőben látszik.
- A két soros törés tényleges megjelenése, a fiók animációja, a töréspont (1280 px) és az Esc/háttér bezárás böngészőben nem ellenőrizhető bejelentkezés nélkül.
