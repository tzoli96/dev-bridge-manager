'use client'

import { useState, useEffect } from 'react'
import { User } from '@/types/user'
import { UsersService } from '@/services/usersService'

export interface UseUsersReturn {
    users: User[]
    loading: boolean
    error: string | null
    refetch: () => Promise<void>
    debugInfo?: any
}

export function useUsers(): UseUsersReturn {
    const [users, setUsers] = useState<User[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)
    const [debugInfo, setDebugInfo] = useState<any>(null)

    const fetchUsers = async () => {
        try {
            setLoading(true)
            setError(null)

            console.log('🔍 [useUsers] Starting to fetch users...')
            console.log('🔍 [useUsers] Auth token:', localStorage.getItem('auth_token') ? 'Present' : 'Missing')

            const data = await UsersService.getAllUsers()

            console.log('🔍 [useUsers] Raw response:', data)
            console.log('🔍 [useUsers] Users array:', data)
            console.log('🔍 [useUsers] Users count:', data?.length || 0)

            setDebugInfo({
                responseType: typeof data,
                isArray: Array.isArray(data),
                length: data?.length || 0,
                firstUser: data?.[0] || null,
                hasAuthToken: !!localStorage.getItem('auth_token'),
                timestamp: new Date().toISOString()
            })

            setUsers(data || [])
        } catch (err) {
            console.error('🔍 [useUsers] Error details:', err)

            // Debug info hiba esetén
            setDebugInfo({
                error: err instanceof Error ? err.message : 'Unknown error',
                errorType: typeof err,
                hasAuthToken: !!localStorage.getItem('auth_token'),
                timestamp: new Date().toISOString()
            })

            setError(err instanceof Error ? err.message : 'Unknown error occurred')
        } finally {
            setLoading(false)
        }
    }

    useEffect(() => {
        fetchUsers()
    }, [])

    return {
        users,
        loading,
        error,
        refetch: fetchUsers,
        debugInfo: process.env.NODE_ENV === 'development' ? debugInfo : undefined
    }
}