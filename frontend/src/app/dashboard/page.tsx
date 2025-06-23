'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { useAuth } from '@/contexts/AuthContext'
import DashboardHeader from '@/components/dashboard/DashboardHeader'
import DashboardTabs from '@/components/dashboard/DashboardTabs'
import DashboardContent from '@/components/dashboard/DashboardContent'
import DashboardModals from '@/components/dashboard/DashboardModals'
import LoadingSpinner from '@/components/ui/LoadingSpinner'

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
        return <LoadingSpinner />
    }

    if (!isAuthenticated) {
        return null
    }

    return (
        <div className="min-h-screen bg-gray-50">
            <DashboardHeader user={user} onLogout={logout} />
            <DashboardTabs
                activeTab={activeTab}
                onTabChange={setActiveTab}
                user={user}
            />
            <DashboardContent activeTab={activeTab} user={user} />
            <DashboardModals />
        </div>
    )
}