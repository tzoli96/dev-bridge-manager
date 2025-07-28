'use client'

import { useAuth } from '@/contexts/AuthContext'
import DashboardTab from '@/components/dashboard/tabs/DashboardTab'

export default function DashboardPage() {
    const { user } = useAuth()

    return <DashboardTab user={user} />
}