import { useState, useCallback } from 'react'

export const useTabNavigation = (defaultTab: string = 'dashboard') => {
    const [activeTab, setActiveTab] = useState(defaultTab)

    const navigateToTab = useCallback((tab: string) => {
        setActiveTab(tab)
    }, [])

    return {
        activeTab,
        navigateToTab
    }
}