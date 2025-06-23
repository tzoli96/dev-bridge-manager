'use client'

import { User } from '@/types/user'

interface DashboardHeaderProps {
    user: User | null
    onLogout: () => void
}

export default function DashboardHeader({ user, onLogout }: DashboardHeaderProps) {
    return (
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
                        onClick={onLogout}
                        className="bg-red-600 text-white px-4 py-2 rounded-md hover:bg-red-700 transition-colors"
                    >
                        Logout
                    </button>
                </div>
            </div>
        </div>
    )
}