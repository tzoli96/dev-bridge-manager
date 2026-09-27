// frontend/src/services/clientStatusEmailService.ts
import { apiClient } from '@/lib/api'

export interface ClientStatusEmail {
    id: number
    client_id: number
    period_start: string
    period_end: string
    subject: string
    body: string
    status: 'pending' | 'sent' | 'dismissed'
    created_at: string
    sent_by: number | null
    sent_at: string | null
    gmail_message_id: string
    dismissed_by: number | null
    dismissed_at: string | null
}

export interface ClientStatusEmailWithNames extends ClientStatusEmail {
    client_name: string
}

interface ListApiResponse {
    success: boolean
    message?: string
    emails?: ClientStatusEmailWithNames[]
}

interface ActionApiResponse {
    success: boolean
    message?: string
    email?: ClientStatusEmail
}

export const ClientStatusEmailService = {
    async list(status?: 'pending' | 'sent' | 'dismissed'): Promise<ClientStatusEmailWithNames[]> {
        const response = await apiClient.get<ListApiResponse>(
            '/admin/client-status-emails',
            status ? { status } : undefined
        )
        if (response.success) return response.emails || []
        throw new Error(response.message || 'Failed to fetch client status emails')
    },

    async approve(id: number, subject: string, body: string): Promise<void> {
        const response = await apiClient.post<ActionApiResponse>(`/admin/client-status-emails/${id}/approve`, {
            subject,
            body,
        })
        if (!response.success) throw new Error(response.message || 'Failed to send status email')
    },

    async dismiss(id: number): Promise<void> {
        const response = await apiClient.post<ActionApiResponse>(`/admin/client-status-emails/${id}/dismiss`, {})
        if (!response.success) throw new Error(response.message || 'Failed to dismiss status email')
    },
}
