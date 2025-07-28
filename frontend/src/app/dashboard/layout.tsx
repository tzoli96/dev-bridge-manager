'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { useAuth } from '@/contexts/AuthContext'
import { ModalProvider } from '@/components/dashboard/DashboardModals'
import DashboardHeader from '@/components/dashboard/DashboardHeader'
import DashboardNav from '@/components/dashboard/DashboardNav'
import DashboardModals from '@/components/dashboard/DashboardModals'
import LoadingSpinner from '@/components/ui/LoadingSpinner'

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const { user, isAuthenticated, logout, loading } = useAuth()
  const router = useRouter()

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
    <ModalProvider>
      <div className="min-h-screen bg-gray-50">
        <DashboardHeader user={user} onLogout={logout} />
        <DashboardNav user={user} />
        <main className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          <div className="transition-opacity duration-300 ease-in-out animate-in fade-in-50 duration-300">
            {children}
          </div>
        </main>
        <DashboardModals />
      </div>
    </ModalProvider>
  )
}