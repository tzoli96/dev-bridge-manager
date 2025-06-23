'use client'

import { User } from '@/types/user'
import DashboardTab from '@/components/dashboard/tabs/DashboardTab'
import UsersTab from '@/components/dashboard/tabs/UsersTab'
import ProjectsTab from '@/components/dashboard/tabs/ProjectsTab'
import AdminTab from '@/components/dashboard/tabs/AdminTab'
import ProfileTab from '@/components/dashboard/tabs/ProfileTab'
import { hasAnyPermission } from '@/utils/permissions'

interface DashboardContentProps {
    activeTab: string
    user: User | null
}

export default function DashboardContent({ activeTab, user }: DashboardContentProps) {
    return (
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
            {activeTab === 'dashboard' && <DashboardTab user={user} />}

            {activeTab === 'users' && hasAnyPermission(user, ['users.list', 'users.read']) && (
                <UsersTab user={user} />
            )}

            {activeTab === 'projects' && <ProjectsTab user={user} />}

            {activeTab === 'admin' && hasAnyPermission(user, ['system.settings', 'roles.list']) && (
                <AdminTab user={user} />
            )}

            {activeTab === 'profile' && <ProfileTab user={user} />}
        </div>
    )
}
