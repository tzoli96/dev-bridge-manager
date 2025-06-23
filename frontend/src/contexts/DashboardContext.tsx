'use client'

import { createContext, useContext, ReactNode } from 'react'
import { useTabNavigation } from '@/hooks/useTabNavigation'
import { User } from '@/types/user'

interface DashboardContextType {
    activeTab: string
    navigateToTab: (tab: string) => void
    user: User | null
}

const DashboardContext = createContext<DashboardContextType | null>(null)

export const useDashboard = () => {
    const context = useContext(DashboardContext)
    if (!context) {
        throw new Error('useDashboard must be used within DashboardProvider')
    }
    return context
}

interface DashboardProviderProps {
    children: ReactNode
    user: User | null
}

export function DashboardProvider({ children, user }: DashboardProviderProps) {
    const { activeTab, navigateToTab } = useTabNavigation()

    return (
        <DashboardContext.Provider value={{
            activeTab,
            navigateToTab,
            user
        }}>
            {children}
        </DashboardContext.Provider>
    )
}
