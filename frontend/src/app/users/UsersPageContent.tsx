'use client'

import { PermissionGuard } from '@/components/auth/PermissionGuard'
import { usePermissions } from '@/hooks/usePermissions'
import UsersList from '@/components/UserList'

export default function UsersPageContent() {
    const { hasPermission } = usePermissions()

    return (
        <>
            {/* Header with Create Button */}
            <div className="flex justify-between items-center mb-6">
                <div>
                    <h1 className="text-2xl font-bold text-gray-900">Team Members</h1>
                    <p className="text-gray-600">Manage your team members and their roles</p>
                </div>

                <PermissionGuard permission="users.create">
                    <button className="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors">
                        Add New User
                    </button>
                </PermissionGuard>
            </div>

            {/* Users List with permission-aware actions */}
            <UsersList />

            {/* Admin only section */}
            <PermissionGuard permission="users.delete">
                <div className="mt-8 bg-red-50 border border-red-200 rounded-lg p-4">
                    <h3 className="text-sm font-medium text-red-800">Admin Actions</h3>
                    <p className="text-sm text-red-600 mt-1">
                        You have user management permissions. Handle with care.
                    </p>
                </div>
            </PermissionGuard>
        </>
    )
}