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
            bgColor: "bg-gray-100 hover:bg-gray-200",
            action: () => {/* Navigate to profile tab */},
            show: true
        },
        {
            title: "Manage Team",
            description: `View ${users?.length || 0} team members`,
            bgColor: "bg-gray-100 hover:bg-gray-200",
            action: () => {/* Navigate to users tab */},
            show: hasAnyPermission(user, ['users.list', 'users.read'])
        },
        {
            title: "View Projects",
            description: `${projects?.length || 0} projects available`,
            bgColor: "bg-gray-100 hover:bg-gray-200",
            action: () => {/* Navigate to projects tab */},
            show: true
        },
        {
            title: "Add Project",
            description: "Create new project",
            bgColor: "bg-indigo-100 hover:bg-indigo-200",
            textColor: "text-indigo-800",
            descColor: "text-indigo-600",
            action: () => setShowCreateProject(true),
            show: isAdmin(user)
        },
        {
            title: "Add User",
            description: "Create new team member",
            bgColor: "bg-blue-100 hover:bg-blue-200",
            textColor: "text-blue-800",
            descColor: "text-blue-600",
            action: () => setShowCreateUser(true),
            show: hasPermission(user, 'users.create')
        },
        {
            title: "Admin Panel",
            description: "System settings",
            bgColor: "bg-purple-100 hover:bg-purple-200",
            textColor: "text-purple-800",
            descColor: "text-purple-600",
            action: () => {/* Navigate to admin tab */},
            show: hasAnyPermission(user, ['system.settings'])
        }
    ]

    return (
        <div className="bg-white rounded-lg shadow p-6">
            <h3 className="text-lg font-semibold mb-4">Quick Actions</h3>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                {actions.filter(action => action.show).map(action => (
                    <button
                        key={action.title}
                        onClick={action.action}
                        className={`${action.bgColor} p-4 rounded-lg text-left transition-colors`}
                    >
                        <div className={`font-medium ${action.textColor || ''}`}>
                            {action.title}
                        </div>
                        <div className={`text-sm ${action.descColor || 'text-gray-600'}`}>
                            {action.description}
                        </div>
                    </button>
                ))}
            </div>
        </div>
    )
}