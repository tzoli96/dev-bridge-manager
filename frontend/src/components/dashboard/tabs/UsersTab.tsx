'use client'

import { User } from '@/types/user'
import { useUsers } from '@/hooks/useUser'
import { useModals } from '@/components/dashboard/DashboardModals'
import { UsersService } from '@/services/usersService'
import { hasPermission, hasAnyPermission } from '@/utils/permissions'
import { useState } from 'react'
import UsersTable from '@/components/dashboard/UsersTable'
import EmptyState from '@/components/ui/EmptyState'
import ErrorState from '@/components/ui/ErrorState'
import LoadingState from '@/components/ui/LoadingState'

interface UsersTabProps {
    user: User | null
}

export default function UsersTab({ user }: UsersTabProps) {
    const { users, loading, error, refetch } = useUsers()
    const { setShowCreateUser } = useModals()
    const [deleting, setDeleting] = useState<number | null>(null)

    const handleDeleteUser = async (userId: number) => {
        if (!confirm('Are you sure you want to delete this user?')) return

        try {
            setDeleting(userId)
            await UsersService.deleteUser(userId)
            await refetch()
        } catch (error: any) {
            alert(`Failed to delete user: ${error.message}`)
        } finally {
            setDeleting(null)
        }
    }

    if (loading) return <LoadingState message="Loading users..." />
    if (error) return <ErrorState error={error} onRetry={refetch} />

    return (
        <div className="space-y-6">
            <div className="flex justify-between items-center">
                <div>
                    <h2 className="text-lg font-semibold">Team Members</h2>
                    <p className="text-gray-600">Manage your team members and their roles</p>
                </div>
                {hasPermission(user, 'users.create') && (
                    <button
                        onClick={() => setShowCreateUser(true)}
                        className="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors"
                    >
                        Add New User
                    </button>
                )}
            </div>

            {!users || users.length === 0 ? (
                <EmptyState
                    icon="users"
                    title="No users found"
                    description="There are no users to display."
                    action={hasPermission(user, 'users.create') ? {
                        label: "Add First User",
                        onClick: () => setShowCreateUser(true)
                    } : undefined}
                />
            ) : (
                <UsersTable
                    users={users}
                    currentUser={user}
                    onDeleteUser={handleDeleteUser}
                    deleting={deleting}
                />
            )}
        </div>
    )
}
