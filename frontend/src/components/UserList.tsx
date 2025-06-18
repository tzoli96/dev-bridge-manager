import { useUsers } from '@/hooks/useUser'
import { PermissionGuard } from '@/components/auth/PermissionGuard'
import { usePermissions } from '@/hooks/usePermissions'
import { useState } from 'react'
import { UsersService } from '@/services/usersService'

export default function UsersList() {
    const { users, loading, error, refetch, debugInfo } = useUsers()
    const { hasPermission, user: currentUser } = usePermissions()
    const [deleting, setDeleting] = useState<number | null>(null)

    const handleDelete = async (userId: number) => {
        if (!confirm('Are you sure you want to delete this user?')) {
            return
        }

        try {
            setDeleting(userId)
            await UsersService.deleteUser(userId)
            await refetch() // Frissítsük a listát
        } catch (error: any) {
            alert(`Failed to delete user: ${error.message}`)
        } finally {
            setDeleting(null)
        }
    }

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
            <div className="space-y-4">
                <div className="bg-red-50 border border-red-200 text-red-600 px-4 py-3 rounded">
                    <div className="font-medium">Error loading users:</div>
                    <div>{error}</div>
                </div>
                <button
                    onClick={refetch}
                    className="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors"
                >
                    Try Again
                </button>

                {/* Debug info development környezetben */}
                {process.env.NODE_ENV === 'development' && (
                    <div className="bg-yellow-50 border border-yellow-200 text-yellow-800 px-4 py-3 rounded text-xs space-y-2">
                        <div className="font-medium">Debug Info:</div>
                        <div>Current user permissions: {currentUser?.permissions?.join(', ') || 'None'}</div>
                        <div>Required permissions: users.list OR users.read</div>
                        <div>Has users.list: {hasPermission('users.list') ? 'Yes' : 'No'}</div>
                        <div>Has users.read: {hasPermission('users.read') ? 'Yes' : 'No'}</div>
                        {debugInfo && (
                            <div className="mt-2 p-2 bg-yellow-100 rounded">
                                <div className="font-medium">API Debug:</div>
                                <pre className="text-xs mt-1 whitespace-pre-wrap">
                                    {JSON.stringify(debugInfo, null, 2)}
                                </pre>
                            </div>
                        )}
                    </div>
                )}
            </div>
        )
    }

    if (!users || users.length === 0) {
        return (
            <div className="space-y-4">
                <div className="bg-white shadow rounded-lg p-8 text-center">
                    <div className="text-gray-500 mb-4">
                        <svg className="mx-auto h-12 w-12" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197m13.5-9a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0z" />
                        </svg>
                    </div>
                    <h3 className="text-lg font-medium text-gray-900 mb-2">No users found</h3>
                    <p className="text-gray-500">There are no users to display.</p>
                    <button
                        onClick={refetch}
                        className="mt-4 bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors"
                    >
                        Refresh
                    </button>
                </div>

                {/* Debug info üres eredmény esetén */}
                {process.env.NODE_ENV === 'development' && (
                    <div className="bg-yellow-50 border border-yellow-200 text-yellow-800 px-4 py-3 rounded text-xs space-y-2">
                        <div className="font-medium">Empty Result Debug Info:</div>
                        <div>Current user permissions: {currentUser?.permissions?.join(', ') || 'None'}</div>
                        <div>Required permissions: users.list OR users.read</div>
                        <div>Has users.list: {hasPermission('users.list') ? 'Yes' : 'No'}</div>
                        <div>Has users.read: {hasPermission('users.read') ? 'Yes' : 'No'}</div>
                        {debugInfo && (
                            <div className="mt-2 p-2 bg-yellow-100 rounded">
                                <div className="font-medium">API Response Debug:</div>
                                <pre className="text-xs mt-1 whitespace-pre-wrap">
                                    {JSON.stringify(debugInfo, null, 2)}
                                </pre>
                            </div>
                        )}
                        <div className="mt-2 p-2 bg-red-100 rounded text-red-800">
                            <div className="font-medium">Possible causes:</div>
                            <ul className="text-xs mt-1 space-y-1">
                                <li>• No users in database</li>
                                <li>• Permission issue (users.list or users.read missing)</li>
                                <li>• Wrong API endpoint being called</li>
                                <li>• Auth token invalid or missing</li>
                                <li>• Backend returning empty array due to filters</li>
                            </ul>
                        </div>
                    </div>
                )}
            </div>
        )
    }

    return (
        <div className="bg-white shadow rounded-lg overflow-hidden">
            {/* Users count info */}
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
                    <PermissionGuard permissions={['users.update', 'users.delete']}>
                        <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
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
                                    <button className="text-blue-600 hover:text-blue-900 transition-colors">
                                        Edit
                                    </button>
                                </PermissionGuard>

                                <PermissionGuard permission="users.delete">
                                    {user.id !== currentUser?.id && (
                                        <button
                                            onClick={() => handleDelete(user.id)}
                                            disabled={deleting === user.id}
                                            className="text-red-600 hover:text-red-900 disabled:opacity-50 transition-colors"
                                        >
                                            {deleting === user.id ? 'Deleting...' : 'Delete'}
                                        </button>
                                    )}
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