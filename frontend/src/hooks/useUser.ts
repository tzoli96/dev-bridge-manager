'use client'

import { useState, useEffect } from 'react'
import { User } from '@/types/user'
import { UsersService } from '@/services/usersService'

export interface UseUsersReturn {
    users: User[]
    loading: boolean
    error: string | null
    refetch: () => Promise<void>
}

export function useUsers(): UseUsersReturn {
    const [users, setUsers] = useState<User[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)

    const fetchUsers = async () => {
        try {
            setLoading(true)
            setError(null)
            const data = await UsersService.getAllUsers()
            setUsers(data)
        } catch (err) {
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
        refetch: fetchUsers
    }
}