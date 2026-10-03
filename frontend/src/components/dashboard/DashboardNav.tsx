'use client'

import { useEffect, useMemo, useRef, useState } from 'react'
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
    const menuButtonRef = useRef<HTMLButtonElement>(null)
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
                        ref={menuButtonRef}
                        type="button"
                        onClick={() => setDrawerOpen(true)}
                        aria-label={`Menü megnyitása${unreadCount > 0 && canManageGmail ? `, ${unreadCount} olvasatlan e-mail` : ''}`}
                        aria-haspopup="dialog"
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

            <MobileNavDrawer
                open={drawerOpen}
                onOpenChange={setDrawerOpen}
                groups={groups}
                pathname={pathname}
                returnFocusRef={menuButtonRef}
            />
        </div>
    )
}
