'use client'

import { User } from '@/types/user'
import { useUsers } from '@/hooks/useUser'
import { useProjects } from '@/hooks/useProjects'
import { hasPermission, hasAnyPermission } from '@/utils/permissions'

interface DashboardTabsProps {
    activeTab: string
    onTabChange: (tab: string) => void
    user: User | null
}

export default function DashboardTabs({ activeTab, onTabChange, user }: DashboardTabsProps) {
    const { users } = useUsers()
    const { projects } = useProjects()

    const tabs = [
        { id: 'dashboard', label: 'Dashboard', show: true },
        {
            id: 'users',
            label: 'Team Members',
            show: hasAnyPermission(user, ['users.list', 'users.read']),
            badge: users?.length
        },
        {
            id: 'projects',
            label: 'Projects',
            show: true,
            badge: projects?.length
        },
        {
            id: 'admin',
            label: 'Administration',
            show: hasAnyPermission(user, ['system.settings', 'roles.list'])
        },
        { id: 'profile', label: 'Profile', show: true }
    ]

    return (
        <div className="bg-card border-b">
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                <nav className="flex space-x-8">
                    {tabs.filter(tab => tab.show).map(tab => (
                        <button
                            key={tab.id}
                            onClick={() => onTabChange(tab.id)}
                            className={`py-4 px-1 border-b-2 font-medium text-sm flex items-center ${
                                activeTab === tab.id
                                    ? 'border-primary text-primary'
                                    : 'border-transparent text-muted-foreground hover:text-foreground hover:border-border'
                            }`}
                        >
                            {tab.label}
                            {tab.badge && tab.badge > 0 && (
                                <span className={`ml-1 px-2 py-0.5 rounded-full text-xs ${
                                    tab.id === 'users'
                                        ? 'bg-primary/10 text-primary'
                                        : 'bg-success/10 text-success'
                                }`}>
                                    {tab.badge}
                                </span>
                            )}
                        </button>
                    ))}
                </nav>
            </div>
        </div>
    )
}
