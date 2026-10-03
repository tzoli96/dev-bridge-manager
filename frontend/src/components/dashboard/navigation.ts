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
