// frontend/src/services/emailsService.ts
import { apiClient } from '@/lib/api'

export interface EmailAttachment {
    filename: string
    size: number
    attachment_id: string
}

export interface EmailListItem {
    id: number
    folder: 'inbox' | 'sent'
    from_address: string
    from_name: string
    to_addresses: string
    subject: string
    snippet: string
    has_attachments: boolean
    attachments: EmailAttachment[]
    is_read: boolean
    category: string | null
    received_at: string
}

export interface EmailListResponse {
    success: boolean
    message?: string
    emails?: EmailListItem[]
    total?: number
}

export interface EmailDetail {
    success: boolean
    message?: string
    id?: number
    folder?: string
    subject?: string
    from?: string
    to?: string
    body_text?: string
    body_html?: string
    attachments?: EmailAttachment[]
    received_at?: string
}

export interface SendEmailRequest {
    to: string
    subject: string
    body: string
    body_html?: string
    in_reply_to_email_id?: number
    files?: File[]
}

export const EmailsService = {
    async list(folder: 'inbox' | 'sent', page = 1, category?: string): Promise<EmailListResponse> {
        const params: Record<string, string | number> = { folder, page }
        if (category) params.category = category
        return apiClient.get<EmailListResponse>('/emails', params)
    },

    async get(id: number): Promise<EmailDetail> {
        return apiClient.get<EmailDetail>(`/emails/${id}`)
    },

    async getUnreadCount(): Promise<number> {
        const res = await apiClient.get<{ success: boolean; count: number }>('/emails/unread-count')
        return res.count
    },

    getAttachmentUrl(emailId: number, attachmentId: string): string {
        return `/emails/${emailId}/attachments/${attachmentId}`
    },

    async downloadAttachment(emailId: number, attachmentId: string, filename: string): Promise<void> {
        const blob = await apiClient.getBlob(EmailsService.getAttachmentUrl(emailId, attachmentId))
        const url = window.URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = filename
        a.click()
        window.URL.revokeObjectURL(url)
    },

    async send(payload: SendEmailRequest): Promise<{ success: boolean; message?: string; gmail_message_id?: string }> {
        const extraFields: Record<string, string> = {
            to: payload.to,
            subject: payload.subject,
            body: payload.body,
        }
        if (payload.body_html !== undefined) {
            extraFields.body_html = payload.body_html
        }
        if (payload.in_reply_to_email_id !== undefined) {
            extraFields.in_reply_to_email_id = String(payload.in_reply_to_email_id)
        }
        return apiClient.uploadFiles('/emails/send', payload.files || [], extraFields)
    },

    async draftReply(id: number): Promise<{ success: boolean; message?: string; draft?: string }> {
        // The AI service can take a while to generate a full draft, and the
        // backend's own AI-service HTTP client waits up to 30s - give the
        // browser a longer timeout so it doesn't give up before the backend
        // itself would (same reasoning as GmailService.sync's longer timeout).
        return apiClient.post(`/emails/${id}/draft-reply`, undefined, 40000)
    },
}
