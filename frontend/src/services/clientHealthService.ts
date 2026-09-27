// frontend/src/services/clientHealthService.ts
import { apiClient } from '@/lib/api'

export interface ClientHealth {
    client_id: number
    client_name: string
    status: 'green' | 'yellow' | 'red'
    has_stalled_task: boolean
    has_overdue_invoice: boolean
}

interface ListApiResponse {
    success: boolean
    message?: string
    clients?: ClientHealth[]
}

export const ClientHealthService = {
    async list(): Promise<ClientHealth[]> {
        const response = await apiClient.get<ListApiResponse>('/admin/client-health')
        if (response.success) return response.clients || []
        throw new Error(response.message || 'Failed to fetch client health')
    },
}
