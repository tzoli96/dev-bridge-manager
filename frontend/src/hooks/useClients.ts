import { useState, useEffect, useCallback } from 'react'
import { ClientsService, Client } from '@/services/clientsService'

export const useClients = () => {
    const [clients, setClients] = useState<Client[]>([])
    const [loading, setLoading] = useState(true)
    const [error, setError] = useState<string | null>(null)

    const fetchClients = useCallback(async () => {
        try {
            setLoading(true)
            setError(null)
            const clientsData = await ClientsService.getAllClients()
            setClients(clientsData)
        } catch (err: any) {
            setError(err.message)
            setClients([])
        } finally {
            setLoading(false)
        }
    }, [])

    useEffect(() => {
        fetchClients()
    }, [fetchClients])

    const refetch = useCallback(() => {
        return fetchClients()
    }, [fetchClients])

    return {
        clients,
        loading,
        error,
        refetch
    }
}
