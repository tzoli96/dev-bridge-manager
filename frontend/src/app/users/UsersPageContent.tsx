'use client'

import { useState } from 'react'
import { PermissionGuard } from '@/components/auth/PermissionGuard'
import { usePermissions } from '@/hooks/usePermissions'
import { useUsers } from '@/hooks/useUser'
import { UsersService } from '@/services/usersService'
import { User } from '@/types/user'
import LoadingState from '@/components/ui/LoadingState'
import ErrorState from '@/components/ui/ErrorState'
import EmptyState from '@/components/ui/EmptyState'
import CreateUserModal from '@/components/CreateUserModal'
import EditUserModal from '@/components/EditUserModal'

export default function UsersPageContent() {
    const { hasPermission, user: currentUser } = usePermissions()
    const { users, loading, error, refetch } = useUsers()
    const [showCreate, setShowCreate] = useState(false)
    const [showEdit, setShowEdit] = useState(false)
    const [selectedUser, setSelectedUser] = useState<User | null>(null)
    const [deleting, setDeleting] = useState<number | null>(null)

    const canCreate = hasPermission('users.create')
    const canUpdate = hasPermission('users.update')
    const canDelete = hasPermission('users.delete')

    const handleEdit = (user: User) => {
        setSelectedUser(user)
        setShowEdit(true)
    }

    const handleDelete = async (user: User) => {
        if (!confirm(`Are you sure you want to delete "${user.name}"?`)) return

        try {
            setDeleting(user.id)
            await UsersService.deleteUser(user.id)
            await refetch()
        } catch (err: any) {
            alert(`Failed to delete user: ${err.message}`)
        } finally {
            setDeleting(null)
        }
    }

    if (loading) return <LoadingState message="Loading users..." />
    if (error) return <ErrorState error={error} onRetry={refetch} />

    return (
        <>
            <div className="flex justify-between items-center mb-6">
                <div>
                    <h1 className="text-2xl font-bold text-foreground">Team Members</h1>
                    <p className="text-muted-foreground text-sm">Manage your team members and their roles</p>
                </div>
                {canCreate && (
                    <button
                        onClick={() => setShowCreate(true)}
                        className="bg-primary text-white px-4 py-2 rounded-lg font-medium text-sm shadow-sm hover:bg-primary/90 hover:shadow-sm active:scale-[0.98] transition-all"
                    >
                        New User
                    </button>
                )}
            </div>

            {!users || users.length === 0 ? (
                <EmptyState
                    icon="users"
                    title="No users yet"
                    description="No team members have been added to the system yet."
                    action={canCreate ? {
                        label: "Create First User",
                        onClick: () => setShowCreate(true)
                    } : undefined}
                />
            ) : (
                <div className="bg-card shadow-sm border border-border rounded-xl overflow-hidden">
                    <div className="bg-muted px-6 py-3 border-b">
                        <p className="text-sm text-muted-foreground">
                            {users.length} user{users.length !== 1 ? 's' : ''}
                        </p>
                    </div>

                    <div className="overflow-x-auto">
                        <table className="min-w-full divide-y divide-border">
                            <thead className="bg-muted">
                                <tr>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">User</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Role</th>
                                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">Position</th>
                                    {(canUpdate || canDelete) && (
                                        <th className="px-6 py-3 text-right text-xs font-medium text-muted-foreground uppercase tracking-wider">Actions</th>
                                    )}
                                </tr>
                            </thead>
                            <tbody className="bg-card divide-y divide-border">
                                {users.map(user => (
                                    <tr key={user.id} className="hover:bg-muted">
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <div className="flex items-center">
                                                <div className="flex-shrink-0 h-10 w-10">
                                                    <div className="h-10 w-10 rounded-full bg-primary flex items-center justify-center">
                                                        <span className="text-white font-medium">
                                                            {user.name.charAt(0).toUpperCase()}
                                                        </span>
                                                    </div>
                                                </div>
                                                <div className="ml-4">
                                                    <div className="text-sm font-medium text-foreground">
                                                        {user.name}
                                                        {user.id === currentUser?.id && (
                                                            <span className="ml-2 text-xs bg-primary/10 text-primary px-2 py-1 rounded">
                                                                You
                                                            </span>
                                                        )}
                                                    </div>
                                                    <div className="text-sm text-muted-foreground">{user.email}</div>
                                                </div>
                                            </div>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap">
                                            <span className="inline-flex px-2 py-1 text-xs font-semibold rounded-full bg-success/10 text-success">
                                                {user.role?.display_name || 'No Role'}
                                            </span>
                                        </td>
                                        <td className="px-6 py-4 whitespace-nowrap text-sm text-foreground">
                                            {user.position || '-'}
                                        </td>
                                        {(canUpdate || canDelete) && (
                                            <td className="px-6 py-4 whitespace-nowrap text-right text-sm space-x-3">
                                                {canUpdate && (
                                                    <button
                                                        onClick={() => handleEdit(user)}
                                                        className="text-primary hover:text-primary font-medium"
                                                    >
                                                        Edit
                                                    </button>
                                                )}
                                                {canDelete && user.id !== currentUser?.id && (
                                                    <button
                                                        onClick={() => handleDelete(user)}
                                                        disabled={deleting === user.id}
                                                        className="text-destructive hover:text-destructive font-medium disabled:opacity-50"
                                                    >
                                                        {deleting === user.id ? 'Deleting...' : 'Delete'}
                                                    </button>
                                                )}
                                            </td>
                                        )}
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                </div>
            )}

            <PermissionGuard permission="users.delete">
                <div className="mt-8 bg-destructive/10 border border-destructive/20 rounded-xl p-4">
                    <h3 className="text-sm font-medium text-destructive">Admin Actions</h3>
                    <p className="text-sm text-destructive mt-1">
                        You have user management permissions. Handle with care.
                    </p>
                </div>
            </PermissionGuard>

            <CreateUserModal
                isOpen={showCreate}
                onClose={() => setShowCreate(false)}
                onSuccess={() => { setShowCreate(false); refetch() }}
            />
            <EditUserModal
                isOpen={showEdit}
                user={selectedUser}
                onClose={() => { setShowEdit(false); setSelectedUser(null) }}
                onSuccess={() => { setShowEdit(false); setSelectedUser(null); refetch() }}
            />
        </>
    )
}
