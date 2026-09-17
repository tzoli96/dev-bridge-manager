import { User } from '@/types/user'
import { Project } from '@/services/projectsService'
import { hasAnyPermission } from '@/utils/permissions'
import { UserCircle, Users, FolderKanban, ShieldCheck, LucideIcon } from 'lucide-react'

interface StatsCardsProps {
    user: User | null
    users: User[] | undefined
    projects: Project[] | undefined
}

type Tone = 'primary' | 'success' | 'warning' | 'muted'

const TONE_STYLES: Record<Tone, { badge: string; icon: string }> = {
    primary: { badge: 'bg-primary/10', icon: 'text-primary' },
    success: { badge: 'bg-success/10', icon: 'text-success' },
    warning: { badge: 'bg-warning/10', icon: 'text-warning' },
    muted: { badge: 'bg-muted', icon: 'text-muted-foreground' },
}

export default function StatsCards({ user, users, projects }: StatsCardsProps) {
    const cards: { title: string; description: string; icon: LucideIcon; tone: Tone; show: boolean }[] = [
        {
            title: "Your Profile",
            description: `${user?.role?.display_name} access level`,
            icon: UserCircle,
            tone: "primary",
            show: true
        },
        {
            title: "Team Management",
            description: `Managing ${users?.length || 0} team members`,
            icon: Users,
            tone: "success",
            show: hasAnyPermission(user, ['users.list', 'users.read'])
        },
        {
            title: "Projects",
            description: `${projects?.length || 0} active projects`,
            icon: FolderKanban,
            tone: "warning",
            show: true
        },
        {
            title: "Administration",
            description: "System settings and role management",
            icon: ShieldCheck,
            tone: "muted",
            show: hasAnyPermission(user, ['system.settings', 'roles.list'])
        }
    ]

    return (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {cards.filter(card => card.show).map(card => {
                const Icon = card.icon
                const tone = TONE_STYLES[card.tone]
                return (
                    <div
                        key={card.title}
                        className="flex items-start gap-3 rounded-xl border border-border bg-card p-4 transition-shadow hover:shadow-sm"
                    >
                        <div className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ${tone.badge}`}>
                            <Icon size={18} className={tone.icon} />
                        </div>
                        <div>
                            <h3 className="font-medium text-sm text-foreground">{card.title}</h3>
                            <p className="text-sm mt-0.5 text-muted-foreground">{card.description}</p>
                        </div>
                    </div>
                )
            })}
        </div>
    )
}
