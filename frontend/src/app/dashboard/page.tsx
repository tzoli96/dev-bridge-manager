'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { useAuth } from '../../contexts/AuthContext'

export default function DashboardPage() {
    const { user, isAuthenticated, logout, loading } = useAuth()
    const router = useRouter()
    const [activeTab, setActiveTab] = useState('dashboard')

    useEffect(() => {
        if (!loading && !isAuthenticated) {
            router.push('/auth')
        }
    }, [isAuthenticated, loading, router])

    if (loading) {
        return (
            <div className="min-h-screen bg-gray-50 flex items-center justify-center">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
                <span className="ml-2 text-gray-600">Loading...</span>
            </div>
        )
    }

    if (!isAuthenticated) {
        return null
    }

    // Helper function to check if user has permission
    const hasPermission = (permission: string) => {
        return user?.permissions?.includes(permission) || false
    }

    // Helper function to check if user has any of the permissions
    const hasAnyPermission = (permissions: string[]) => {
        return permissions.some(permission => hasPermission(permission))
    }

    return (
        <div className="min-h-screen bg-gray-50">
            {/* Header */}
            <div className="bg-white shadow">
                <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                    <div className="flex justify-between items-center py-6">
                        <div>
                            <h1 className="text-2xl font-bold text-gray-900">Dev Bridge Manager</h1>
                            <p className="text-gray-600">
                                Welcome back, {user?.name}!
                                <span className="ml-2 text-sm bg-blue-100 text-blue-800 px-2 py-1 rounded">
                                    {user?.role?.display_name}
                                </span>
                            </p>
                        </div>
                        <button
                            onClick={logout}
                            className="bg-red-600 text-white px-4 py-2 rounded-md hover:bg-red-700 transition-colors"
                        >
                            Logout
                        </button>
                    </div>
                </div>
            </div>

            {/* Navigation Tabs */}
            <div className="bg-white border-b">
                <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                    <nav className="flex space-x-8">
                        <button
                            onClick={() => setActiveTab('dashboard')}
                            className={`py-4 px-1 border-b-2 font-medium text-sm ${
                                activeTab === 'dashboard'
                                    ? 'border-blue-500 text-blue-600'
                                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                            }`}
                        >
                            Dashboard
                        </button>

                        {/* Users tab - only show if user has permission */}
                        {hasAnyPermission(['users.list', 'users.read']) && (
                            <button
                                onClick={() => setActiveTab('users')}
                                className={`py-4 px-1 border-b-2 font-medium text-sm ${
                                    activeTab === 'users'
                                        ? 'border-blue-500 text-blue-600'
                                        : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                                }`}
                            >
                                Team Members
                            </button>
                        )}

                        {/* Admin tab - only show for admin roles */}
                        {hasAnyPermission(['system.settings', 'roles.list']) && (
                            <button
                                onClick={() => setActiveTab('admin')}
                                className={`py-4 px-1 border-b-2 font-medium text-sm ${
                                    activeTab === 'admin'
                                        ? 'border-blue-500 text-blue-600'
                                        : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                                }`}
                            >
                                Administration
                            </button>
                        )}

                        <button
                            onClick={() => setActiveTab('profile')}
                            className={`py-4 px-1 border-b-2 font-medium text-sm ${
                                activeTab === 'profile'
                                    ? 'border-blue-500 text-blue-600'
                                    : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                            }`}
                        >
                            Profile
                        </button>
                    </nav>
                </div>
            </div>

            {/* Main Content */}
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                {/* Dashboard Tab */}
                {activeTab === 'dashboard' && (
                    <div className="space-y-6">
                        <div className="bg-white rounded-lg shadow p-6">
                            <h2 className="text-lg font-semibold mb-4">Welcome to your Dashboard</h2>
                            <p className="text-gray-600 mb-4">
                                Here's a quick overview of your account and available features.
                            </p>

                            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                                {/* Profile Card */}
                                <div className="bg-blue-50 rounded-lg p-4">
                                    <h3 className="font-medium text-blue-900">Your Profile</h3>
                                    <p className="text-blue-700 text-sm mt-1">
                                        {user?.role?.display_name} access level
                                    </p>
                                </div>

                                {/* Users Card - conditional */}
                                {hasAnyPermission(['users.list', 'users.read']) && (
                                    <div className="bg-green-50 rounded-lg p-4">
                                        <h3 className="font-medium text-green-900">Team Management</h3>
                                        <p className="text-green-700 text-sm mt-1">
                                            You can view and manage team members
                                        </p>
                                    </div>
                                )}

                                {/* Admin Card - conditional */}
                                {hasAnyPermission(['system.settings', 'roles.list']) && (
                                    <div className="bg-purple-50 rounded-lg p-4">
                                        <h3 className="font-medium text-purple-900">Administration</h3>
                                        <p className="text-purple-700 text-sm mt-1">
                                            System settings and role management
                                        </p>
                                    </div>
                                )}
                            </div>
                        </div>

                        {/* Quick Actions */}
                        <div className="bg-white rounded-lg shadow p-6">
                            <h3 className="text-lg font-semibold mb-4">Quick Actions</h3>
                            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                                <button
                                    onClick={() => setActiveTab('profile')}
                                    className="bg-gray-100 hover:bg-gray-200 p-4 rounded-lg text-left transition-colors"
                                >
                                    <div className="font-medium">View Profile</div>
                                    <div className="text-sm text-gray-600">Update your information</div>
                                </button>

                                {hasAnyPermission(['users.list', 'users.read']) && (
                                    <button
                                        onClick={() => setActiveTab('users')}
                                        className="bg-gray-100 hover:bg-gray-200 p-4 rounded-lg text-left transition-colors"
                                    >
                                        <div className="font-medium">Manage Team</div>
                                        <div className="text-sm text-gray-600">View team members</div>
                                    </button>
                                )}

                                {hasPermission('users.create') && (
                                    <button className="bg-blue-100 hover:bg-blue-200 p-4 rounded-lg text-left transition-colors">
                                        <div className="font-medium text-blue-800">Add User</div>
                                        <div className="text-sm text-blue-600">Create new team member</div>
                                    </button>
                                )}

                                {hasAnyPermission(['system.settings']) && (
                                    <button
                                        onClick={() => setActiveTab('admin')}
                                        className="bg-purple-100 hover:bg-purple-200 p-4 rounded-lg text-left transition-colors"
                                    >
                                        <div className="font-medium text-purple-800">Admin Panel</div>
                                        <div className="text-sm text-purple-600">System settings</div>
                                    </button>
                                )}
                            </div>
                        </div>
                    </div>
                )}

                {/* Users Tab */}
                {activeTab === 'users' && hasAnyPermission(['users.list', 'users.read']) && (
                    <div className="space-y-6">
                        <div className="flex justify-between items-center">
                            <div>
                                <h2 className="text-lg font-semibold">Team Members</h2>
                                <p className="text-gray-600">Manage your team members and their roles</p>
                            </div>
                            {hasPermission('users.create') && (
                                <button className="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors">
                                    Add New User
                                </button>
                            )}
                        </div>

                        {/* Users List Placeholder */}
                        <div className="bg-white rounded-lg shadow">
                            <div className="p-6">
                                <div className="text-center py-8">
                                    <div className="text-gray-500 mb-4">
                                        <svg className="mx-auto h-12 w-12" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197m13.5-9a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0z" />
                                        </svg>
                                    </div>
                                    <h3 className="text-lg font-medium text-gray-900 mb-2">Users List Coming Soon</h3>
                                    <p className="text-gray-500">
                                        The users list component will be integrated here.
                                    </p>
                                    <button
                                        onClick={() => router.push('/users')}
                                        className="mt-4 bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors"
                                    >
                                        Go to Users Page
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                )}

                {/* Admin Tab */}
                {activeTab === 'admin' && hasAnyPermission(['system.settings', 'roles.list']) && (
                    <div className="space-y-6">
                        <div>
                            <h2 className="text-lg font-semibold">Administration</h2>
                            <p className="text-gray-600">System settings and administrative tools</p>
                        </div>

                        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                            {hasPermission('system.settings') && (
                                <div className="bg-white rounded-lg shadow p-6">
                                    <h3 className="font-medium mb-2">System Settings</h3>
                                    <p className="text-gray-600 text-sm mb-4">Configure system-wide settings</p>
                                    <button className="bg-purple-600 text-white px-4 py-2 rounded-md hover:bg-purple-700 transition-colors">
                                        Open Settings
                                    </button>
                                </div>
                            )}

                            {hasPermission('roles.list') && (
                                <div className="bg-white rounded-lg shadow p-6">
                                    <h3 className="font-medium mb-2">Role Management</h3>
                                    <p className="text-gray-600 text-sm mb-4">Manage user roles and permissions</p>
                                    <button className="bg-purple-600 text-white px-4 py-2 rounded-md hover:bg-purple-700 transition-colors">
                                        Manage Roles
                                    </button>
                                </div>
                            )}

                            {hasPermission('system.logs') && (
                                <div className="bg-white rounded-lg shadow p-6">
                                    <h3 className="font-medium mb-2">System Logs</h3>
                                    <p className="text-gray-600 text-sm mb-4">View system activity and logs</p>
                                    <button className="bg-purple-600 text-white px-4 py-2 rounded-md hover:bg-purple-700 transition-colors">
                                        View Logs
                                    </button>
                                </div>
                            )}

                            <div className="bg-white rounded-lg shadow p-6">
                                <h3 className="font-medium mb-2">Statistics</h3>
                                <p className="text-gray-600 text-sm mb-4">System usage and performance metrics</p>
                                <button className="bg-purple-600 text-white px-4 py-2 rounded-md hover:bg-purple-700 transition-colors">
                                    View Stats
                                </button>
                            </div>
                        </div>
                    </div>
                )}

                {/* Profile Tab */}
                {activeTab === 'profile' && (
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

                            {hasPermission('profile.update') && (
                                <div className="mt-6">
                                    <button className="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors">
                                        Edit Profile
                                    </button>
                                </div>
                            )}
                        </div>

                        {/* Permissions - Development mode only */}
                        {process.env.NODE_ENV === 'development' && user?.permissions && (
                            <div className="bg-white rounded-lg shadow p-6">
                                <h3 className="text-lg font-semibold mb-4">Your Permissions (Debug)</h3>
                                <div className="flex flex-wrap gap-2">
                                    {user.permissions.map((permission: string) => (
                                        <span key={permission} className="text-xs bg-green-100 text-green-800 px-2 py-1 rounded">
                                            {permission}
                                        </span>
                                    ))}
                                </div>
                            </div>
                        )}
                    </div>
                )}
            </div>
        </div>
    )
}