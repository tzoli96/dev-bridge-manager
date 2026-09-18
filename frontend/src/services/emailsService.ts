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
    async list(folder: 'inbox' | 'sent', page = 1): Promise<EmailListResponse> {
        return apiClient.get<EmailListResponse>('/emails', { folder, page })
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
}
