// frontend/src/services/gmailService.ts
import { apiClient } from '@/lib/api'

export interface GmailStatus {
    success: boolean
    connected: boolean
    email_address?: string
    last_synced_at?: string
    needs_reauth?: boolean
}

export const GmailService = {
    async getAuthURL(): Promise<string> {
        const res = await apiClient.get<{ success: boolean; url: string }>('/gmail/auth-url')
        return res.url
    },

    async getStatus(): Promise<GmailStatus> {
        return apiClient.get<GmailStatus>('/gmail/status')
    },

    async disconnect(): Promise<void> {
        await apiClient.post('/gmail/disconnect')
    },

    async sync(): Promise<{ success: boolean; message?: string; last_synced_at?: string }> {
        // Sync categorizes each new inbox message with a serial AI call
        // (measured ~3.5-4s each), which can exceed the default 10s
        // timeout once a handful of new messages arrive at once.
        return apiClient.post('/gmail/sync', undefined, 120000)
    },
}
