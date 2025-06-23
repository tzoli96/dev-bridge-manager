import { User } from '@/types/user'
import { Project } from '@/services/projectsService'
import { hasAnyPermission } from '@/utils/permissions'

interface StatsCardsProps {
    user: User | null
    users: User[] | undefined
    projects: Project[] | undefined
}

export default function StatsCards({ user, users, projects }: StatsCardsProps) {
    const cards = [
        {
            title: "Your Profile",
            description: `${user?.role?.display_name} access level`,
            bgColor: "bg-blue-50",
            textColor: "text-blue-900",
            descColor: "text-blue-700",
            show: true
        },
        {
            title: "Team Management",
            description: `Managing ${users?.length || 0} team members`,
            bgColor: "bg-green-50",
            textColor: "text-green-900",
            descColor: "text-green-700",
            show: hasAnyPermission(user, ['users.list', 'users.read'])
        },
        {
            title: "Projects",
            description: `${projects?.length || 0} active projects`,
            bgColor: "bg-indigo-50",
            textColor: "text-indigo-900",
            descColor: "text-indigo-700",
            show: true
        },
        {
            title: "Administration",
            description: "System settings and role management",
            bgColor: "bg-purple-50",
            textColor: "text-purple-900",
            descColor: "text-purple-700",
            show: hasAnyPermission(user, ['system.settings', 'roles.list'])
        }
    ]

    return (
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            {cards.filter(card => card.show).map(card => (
                <div key={card.title} className={`${card.bgColor} rounded-lg p-4`}>
                    <h3 className={`font-medium ${card.textColor}`}>{card.title}</h3>
                    <p className={`text-sm mt-1 ${card.descColor}`}>{card.description}</p>
                </div>
            ))}
        </div>
    )
}