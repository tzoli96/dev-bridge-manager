'use client'

import { User } from '@/types/user'
import { useModals } from '@/components/dashboard/DashboardModals'
import { hasPermission } from '@/utils/permissions'

interface ProfileTabProps {
    user: User | null
}

export default function ProfileTab({ user }: ProfileTabProps) {
    const { setShowEditProfile, setShowChangePassword } = useModals()

    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-lg font-semibold">Your Profile</h2>
                <p className="text-gray-600">View and manage your account information</p>
            </div>

            <div className="bg-white rounded-lg shadow p-6">
                <h3 className="text-lg font-semibold mb-4">Account Information</h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <div>
                        <label className="block text-sm font-medium text-gray-700">Name</label>
                        <p className="mt-1 text-gray-900">{user?.name}</p>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700">Email</label>
                        <p className="mt-1 text-gray-900">{user?.email}</p>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700">Position</label>
                        <p className="mt-1 text-gray-900">{user?.position || 'Not specified'}</p>
                    </div>
                    <div>
                        <label className="block text-sm font-medium text-gray-700">Role</label>
                        <p className="mt-1 text-gray-900">{user?.role?.display_name}</p>
                    </div>
                </div>

                {hasPermission(user, 'profile.update') && (
                    <div className="mt-6 space-x-3">
                        <button
                            onClick={() => setShowEditProfile(true)}
                            className="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors"
                        >
                            Edit Profile
                        </button>
                        <button
                            onClick={() => setShowChangePassword(true)}
                            className="bg-red-600 text-white px-4 py-2 rounded-md hover:bg-red-700 transition-colors"
                        >
                            Change Password
                        </button>
                    </div>
                )}
            </div>

            {/* Debug info csak fejlesztési módban */}
            {process.env.NODE_ENV === 'development' && user?.permissions && (
                <div className="bg-white rounded-lg shadow p-6">
                    <h3 className="text-lg font-semibold mb-4">Your Permissions (Debug)</h3>
                    <div className="flex flex-wrap gap-2">
                        {user.permissions.map((permission: string) => (
                            <span
                                key={permission}
                                className="text-xs bg-green-100 text-green-800 px-2 py-1 rounded"
                            >
                                {permission}
                            </span>
                        ))}
                    </div>
                </div>
            )}
        </div>
    )
}