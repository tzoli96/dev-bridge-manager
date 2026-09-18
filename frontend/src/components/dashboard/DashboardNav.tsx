'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { LayoutDashboard, KanbanSquare, Users, Building2, ShieldCheck, Mail, Receipt } from 'lucide-react'
import { User } from '@/types/user'
import { hasAnyPermission } from '@/utils/permissions'
import { EmailsService } from '@/services/emailsService'

interface DashboardNavProps {
    user: User | null
}

const UNREAD_POLL_INTERVAL_MS = 30_000

export default function DashboardNav({ user }: DashboardNavProps) {
    const pathname = usePathname()
    const [unreadCount, setUnreadCount] = useState(0)
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

    const links = [
        { href: '/dashboard', label: 'Dashboard', icon: LayoutDashboard, show: true },
        { href: '/dashboard/board', label: 'Projects', icon: KanbanSquare, show: true },
        {
            href: '/dashboard/clients',
            label: 'Clients',
            icon: Building2,
            show: hasAnyPermission(user, ['clients.list', 'clients.read']),
        },
        {
            href: '/dashboard/emails',
            label: 'E-mailek',
            icon: Mail,
            show: canManageGmail,
            badge: unreadCount > 0 ? unreadCount : undefined,
        },
        {
            href: '/dashboard/billing',
            label: 'Számlázás',
            icon: Receipt,
            show: hasAnyPermission(user, ['invoices.read']),
        },
        {
            href: '/users',
            label: 'Team Members',
            icon: Users,
            show: hasAnyPermission(user, ['users.list', 'users.read']),
        },
        {
            href: '/dashboard/admin',
            label: 'Administration',
            icon: ShieldCheck,
            show: hasAnyPermission(user, ['system.settings', 'roles.list']),
        },
    ]

    return (
        <div className="bg-card border-b border-border">
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                <nav className="flex gap-1.5 py-3">
                    {links.filter(link => link.show).map(link => {
                        const isActive = pathname === link.href
                        const Icon = link.icon
                        return (
                            <Link
                                key={link.href}
                                href={link.href}
                                className={`flex items-center gap-2 px-3 py-2 rounded-lg font-medium text-sm transition-colors ${
                                    isActive
                                        ? 'bg-primary/10 text-primary'
                                        : 'text-muted-foreground hover:text-foreground hover:bg-muted'
                                }`}
                            >
                                <Icon size={16} />
                                {link.label}
                                {'badge' in link && link.badge !== undefined && (
                                    <span className="flex items-center justify-center min-w-[18px] h-[18px] px-1 rounded-full bg-red-500 text-white text-[10px] font-semibold leading-none">
                                        {link.badge > 99 ? '99+' : link.badge}
                                    </span>
                                )}
                            </Link>
                        )
                    })}
                </nav>
            </div>
        </div>
    )
}
