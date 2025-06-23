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
        <div className="bg-white shadow rounded-lg overflow-hidden">
            <div className="bg-gray-50 px-6 py-3 border-b">
                <p className="text-sm text-gray-600">
                    Showing {users.length} user{users.length !== 1 ? 's' : ''}
                </p>
            </div>

            <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                        User
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                        Role
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                        Position
                    </th>
                    {hasAnyPermission(currentUser, ['users.update', 'users.delete']) && (
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                            Actions
                        </th>
                    )}
                </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                {users.map((user) => (
                    <tr key={user.id} className="hover:bg-gray-50">
                        <td className="px-6 py-4 whitespace-nowrap">
                            <div className="flex items-center">
                                <div className="flex-shrink-0 h-10 w-10">
                                    <div className="h-10 w-10 rounded-full bg-blue-500 flex items-center justify-center">
                                            <span className="text-white font-medium">
                                                {user.name.charAt(0).toUpperCase()}
                                            </span>
                                    </div>
                                </div>
                                <div className="ml-4">
                                    <div className="text-sm font-medium text-gray-900">
                                        {user.name}
                                        {user.id === currentUser?.id && (
                                            <span className="ml-2 text-xs bg-blue-100 text-blue-800 px-2 py-1 rounded">
                                                    You
                                                </span>
                                        )}
                                    </div>
                                    <div className="text-sm text-gray-500">{user.email}</div>
                                </div>
                            </div>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap">
                                <span className="inline-flex px-2 py-1 text-xs font-semibold rounded-full bg-green-100 text-green-800">
                                    {user.role?.display_name || 'Unknown Role'}
                                </span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                            {user.position || '-'}
                        </td>
                        {hasAnyPermission(currentUser, ['users.update', 'users.delete']) && (
                            <td className="px-6 py-4 whitespace-nowrap text-sm font-medium space-x-2">
                                {hasPermission(currentUser, 'users.update') && (
                                    <button
                                        onClick={() => handleEditUser(user)}
                                        className="text-blue-600 hover:text-blue-900 transition-colors"
                                    >
                                        Edit
                                    </button>
                                )}
                                {hasPermission(currentUser, 'users.delete') && user.id !== currentUser?.id && (
                                    <button
                                        onClick={() => onDeleteUser(user.id)}
                                        disabled={deleting === user.id}
                                        className="text-red-600 hover:text-red-900 disabled:opacity-50 transition-colors"
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
