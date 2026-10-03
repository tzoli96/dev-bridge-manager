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
