'use client'

import { useUsers } from '@/hooks/useUser'
import { PermissionGuard } from '@/components/auth/PermissionGuard'
import { usePermissions } from '@/hooks/usePermissions'

export default function UsersList() {
    const { users, loading, error } = useUsers()
    const { hasPermission, user: currentUser } = usePermissions()

    if (loading) {
        return (
            <div className="flex justify-center items-center py-8">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
                <span className="ml-2">Loading users...</span>
            </div>
        )
    }

    if (error) {
        return (
            <div className="bg-red-50 border border-red-200 text-red-600 px-4 py-3 rounded">
                Error: {error}
            </div>
        )
    }

    return (
        <div className="bg-white shadow rounded-lg overflow-hidden">
            <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                        User
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                        Role
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                        Position
                    </th>
                    <PermissionGuard permissions={['users.update', 'users.delete']}>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">
                            Actions
                        </th>
                    </PermissionGuard>
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
                                    {user.role?.display_name || 'No Role'}
                                </span>
                        </td>
                        <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                            {user.position || '-'}
                        </td>
                        <PermissionGuard permissions={['users.update', 'users.delete']}>
                            <td className="px-6 py-4 whitespace-nowrap text-sm font-medium space-x-2">
                                <PermissionGuard permission="users.update">
                                    <button className="text-blue-600 hover:text-blue-900">
                                        Edit
                                    </button>
                                </PermissionGuard>

                                <PermissionGuard permission="users.delete">
                                    <>
                                        {user.id !== currentUser?.id && (
                                            <button className="text-red-600 hover:text-red-900">
                                                Delete
                                            </button>
                                        )}
                                    </>
                                </PermissionGuard>
                            </td>
                        </PermissionGuard>
                    </tr>
                ))}
                </tbody>
            </table>
        </div>
    )
}