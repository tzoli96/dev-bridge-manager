'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { LayoutDashboard, KanbanSquare, Users, Building2 } from 'lucide-react'
import { User } from '@/types/user'
import { hasAnyPermission } from '@/utils/permissions'

interface DashboardNavProps {
    user: User | null
}

export default function DashboardNav({ user }: DashboardNavProps) {
    const pathname = usePathname()

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
            href: '/users',
            label: 'Team Members',
            icon: Users,
            show: hasAnyPermission(user, ['users.list', 'users.read']),
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
                            </Link>
                        )
                    })}
                </nav>
            </div>
        </div>
    )
}
