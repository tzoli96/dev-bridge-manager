import { User } from '@/types/user'
import { Project } from '@/services/projectsService'
import { useModals } from '@/components/dashboard/DashboardModals'
import { hasPermission, hasAnyPermission, isAdmin } from '@/utils/permissions'

interface QuickActionsGridProps {
    user: User | null
    users: User[] | undefined
    projects: Project[] | undefined
}

export default function QuickActionsGrid({ user, users, projects }: QuickActionsGridProps) {
    const { setShowCreateUser, setShowCreateProject } = useModals()

    const actions = [
        {
            title: "View Profile",
            description: "Update your information",
            bgColor: "bg-muted hover:bg-muted",
            action: () => {/* Navigate to profile tab */},
            show: true
        },
        {
            title: "Manage Team",
            description: `View ${users?.length || 0} team members`,
            bgColor: "bg-muted hover:bg-muted",
            action: () => {/* Navigate to users tab */},
            show: hasAnyPermission(user, ['users.list', 'users.read'])
        },
        {
            title: "View Projects",
            description: `${projects?.length || 0} projects available`,
            bgColor: "bg-muted hover:bg-muted",
            action: () => {/* Navigate to projects tab */},
            show: true
        },
        {
            title: "Add Project",
            description: "Create new project",
            bgColor: "bg-primary/10 hover:bg-primary/20",
            textColor: "text-primary",
            descColor: "text-primary",
            action: () => setShowCreateProject(true),
            show: isAdmin(user)
        },
        {
            title: "Add User",
            description: "Create new team member",
            bgColor: "bg-primary/10 hover:bg-primary/20",
            textColor: "text-primary",
            descColor: "text-primary",
            action: () => setShowCreateUser(true),
            show: hasPermission(user, 'users.create')
        },
        {
            title: "Admin Panel",
            description: "System settings",
            bgColor: "bg-primary/10 hover:bg-primary/20",
            textColor: "text-primary",
            descColor: "text-primary",
            action: () => {/* Navigate to admin tab */},
            show: hasAnyPermission(user, ['system.settings'])
        }
    ]

    return (
        <div className="bg-card rounded-xl shadow-sm border border-border p-6">
            <h3 className="text-base font-semibold text-foreground mb-4">Quick Actions</h3>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
                {actions.filter(action => action.show).map(action => (
                    <button
                        key={action.title}
                        onClick={action.action}
                        className={`${action.bgColor} p-4 rounded-xl text-left transition-all hover:shadow-sm active:scale-[0.98]`}
                    >
                        <div className={`font-medium text-sm ${action.textColor || 'text-foreground'}`}>
                            {action.title}
                        </div>
                        <div className={`text-xs mt-0.5 ${action.descColor || 'text-muted-foreground'}`}>
                            {action.description}
                        </div>
                    </button>
                ))}
            </div>
        </div>
    )
}