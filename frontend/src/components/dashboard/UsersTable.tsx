import { User } from '@/types/user'
import { useModals } from '@/components/dashboard/DashboardModals'
import { hasPermission, hasAnyPermission } from '@/utils/permissions'

interface UsersTableProps {
    users: User[]
    currentUser: User | null
    onDeleteUser: (userId: number) => void
    deleting: number | null
}

export default function UsersTable({ users, currentUser, onDeleteUser, deleting }: UsersTableProps) {
    const { setShowEditUser, setSelectedUser } = useModals()

    const handleEditUser = (user: User) => {
        setSelectedUser(user)
        setShowEditUser(true)
    }

    return (
        <div className="bg-card shadow-sm border border-border rounded-xl overflow-hidden">
            <div className="bg-muted px-6 py-3 border-b border-border">
                <p className="text-sm text-muted-foreground">
                    Showing {users.length} user{users.length !== 1 ? 's' : ''}
                </p>
            </div>

            <table className="min-w-full divide-y divide-border">
                <thead className="bg-muted">
                <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">
                        User
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">
                        Role
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">
                        Position
                    </th>
                    {hasAnyPermission(currentUser, ['users.update', 'users.delete']) && (
                        <th className="px-6 py-3 text-left text-xs font-medium text-muted-foreground uppercase tracking-wider">
                            Actions
                        </th>
                    )}
                </tr>
                </thead>
                <tbody className="bg-card divide-y divide-border">
                {users.map((user) => (
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
                                    {user.role?.display_name || 'Unknown Role'}
                                </span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-foreground">
                            {user.position || '-'}
                        </td>
                        {hasAnyPermission(currentUser, ['users.update', 'users.delete']) && (
                            <td className="px-6 py-4 whitespace-nowrap text-sm font-medium space-x-2">
                                {hasPermission(currentUser, 'users.update') && (
                                    <button
                                        onClick={() => handleEditUser(user)}
                                        className="text-primary hover:text-primary transition-colors"
                                    >
                                        Edit
                                    </button>
                                )}
                                {hasPermission(currentUser, 'users.delete') && user.id !== currentUser?.id && (
                                    <button
                                        onClick={() => onDeleteUser(user.id)}
                                        disabled={deleting === user.id}
                                        className="text-destructive hover:text-destructive disabled:opacity-50 transition-colors"
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
    )
}
