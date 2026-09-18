'use client'

import { useAuth } from '@/contexts/AuthContext'
import { ProtectedRoute } from '@/components/auth/ProtectedRoute'
import AdminTab from '@/components/dashboard/tabs/AdminTab'

export default function AdminPage() {
    const { user } = useAuth()

    return (
        <ProtectedRoute permissions={['system.settings', 'roles.list']} requireAll={false}>
            <AdminTab user={user} />
        </ProtectedRoute>
    )
}
