'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useAuth } from '../../contexts/AuthContext'

export default function DashboardPage() {
    const { user, isAuthenticated, logout, loading } = useAuth()
    const router = useRouter()

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
        return null // Redirect happening
    }

    return (
        <div className="min-h-screen bg-gray-50">
            {/* Header */}
            <div className="bg-white shadow">
                <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                    <div className="flex justify-between items-center py-6">
                        <div>
                            <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
                            <p className="text-gray-600">Welcome back, {user?.name}!</p>
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

            {/* Main Content */}
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
                {/* User Info Card */}
                <div className="bg-white rounded-lg shadow p-6 mb-8">
                    <h2 className="text-lg font-semibold mb-4">Your Profile</h2>
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
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
                            <label className="block text-sm font-medium text-gray-700">User ID</label>
                            <p className="mt-1 text-gray-900">#{user?.id}</p>
                        </div>
                    </div>
                </div>

                {/* Quick Actions */}
                <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                    <div className="bg-white rounded-lg shadow p-6">
                        <h3 className="text-lg font-semibold mb-2">Projects</h3>
                        <p className="text-gray-600 mb-4">Manage your projects</p>
                        <button className="bg-blue-600 text-white px-4 py-2 rounded-md hover:bg-blue-700 transition-colors">
                            View Projects
                        </button>
                    </div>

                    <div className="bg-white rounded-lg shadow p-6">
                        <h3 className="text-lg font-semibold mb-2">Team Members</h3>
                        <p className="text-gray-600 mb-4">View team members</p>
                        <button
                            onClick={() => router.push('/users')}
                            className="bg-green-600 text-white px-4 py-2 rounded-md hover:bg-green-700 transition-colors"
                        >
                            View Users
                        </button>
                    </div>

                    <div className="bg-white rounded-lg shadow p-6">
                        <h3 className="text-lg font-semibold mb-2">Settings</h3>
                        <p className="text-gray-600 mb-4">Account settings</p>
                        <button className="bg-gray-600 text-white px-4 py-2 rounded-md hover:bg-gray-700 transition-colors">
                            Settings
                        </button>
                    </div>
                </div>
            </div>
        </div>
    )
}