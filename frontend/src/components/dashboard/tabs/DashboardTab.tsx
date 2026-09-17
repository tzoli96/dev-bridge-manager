'use client'

import { User } from '@/types/user'
import { useUsers } from '@/hooks/useUser'
import { useProjects } from '@/hooks/useProjects'
import { useModals } from '@/components/dashboard/DashboardModals'
import { hasPermission, hasAnyPermission, isAdmin } from '@/utils/permissions'
import QuickActionsGrid from '@/components/dashboard/QuickActionsGrid'
import StatsCards from '@/components/dashboard/StatsCards'

interface DashboardTabProps {
    user: User | null
}

export default function DashboardTab({ user }: DashboardTabProps) {
    const { users } = useUsers()
    const { projects } = useProjects()

    return (
        <div className="space-y-6">
            <div className="bg-card rounded-xl shadow-sm border border-border p-6">
                <h2 className="text-lg font-semibold text-foreground mb-1">
                    Welcome back{user?.name ? `, ${user.name.split(' ')[0]}` : ''}
                </h2>
                <p className="text-muted-foreground text-sm mb-5">
                    Here&apos;s a quick overview of your account and available features.
                </p>
                <StatsCards user={user} users={users} projects={projects} />
            </div>
            <QuickActionsGrid user={user} users={users} projects={projects} />
        </div>
    )
}