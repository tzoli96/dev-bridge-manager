// frontend/src/services/invoicesService.ts
import { apiClient } from '@/lib/api'

export interface Invoice {
    id: number
    project_id: number
    project_name?: string
    client_id: number
    client_name: string
    billingo_invoice_id: string
    billingo_invoice_number: string
    pricing_type: 'hourly' | 'fixed'
    period_start: string | null
    period_end: string | null
    item_name: string
    due_date: string | null
    amount: number
    status: 'created' | 'failed'
    payment_status: 'outstanding' | 'paid' | 'partially_paid' | 'expired' | 'none' | ''
    paid_date: string | null
    error_message: string
    created_by: number
    created_by_name: string
    created_at: string
    items?: InvoiceItem[]
}

export interface InvoiceItem {
    id: number
    invoice_id: number
    name: string
    quantity: number
    unit: string
    unit_price: number
    unit_price_type: string
    line_total: number
    is_base: boolean
}

export interface InvoiceExtraItemInput {
    name: string
    quantity: number
    unit: string
    unit_price: number
}

export interface InvoiceCreateRequest {
    client_id: number
    period_start?: string
    period_end?: string
    item_name?: string
    due_date?: string
    base_unit_price?: number
    extra_items?: InvoiceExtraItemInput[]
}

export interface InvoicesResponse {
    success: boolean
    message: string
    invoice?: Invoice
    invoices?: Invoice[]
    count?: number
}

export interface InvoiceLineItem {
    task_id: string
    task_title: string
    board_id: number
    date: string
    hours: number
    user_name: string
}

export interface InvoiceBreakdownResponse {
    success: boolean
    message: string
    items?: InvoiceLineItem[]
}

export interface MonthlyRevenue {
    month: string
    amount: number
}

export interface YearlyRevenue {
    year: string
    amount: number
}

export interface RevenueAnalytics {
    monthly: MonthlyRevenue[]
    yearly: YearlyRevenue[]
    total: number
}

interface RevenueAnalyticsResponse {
    success: boolean
    message: string
    monthly?: MonthlyRevenue[]
    yearly?: YearlyRevenue[]
    total?: number
}

export interface InvoiceNotice {
    id: number
    project_id: number
    client_id: number
    period_start: string | null
    period_end: string | null
    gmail_message_id: string
    sent_by: number
    sent_at: string
    status: 'pending' | 'approved'
    invoice_id: number | null
    approved_by: number | null
    approved_at: string | null
}

export interface InvoiceNoticeWithNames extends InvoiceNotice {
    project_name: string
    client_name: string
    billingo_invoice_number?: string
}

export const InvoiceNoticesService = {
    async send(projectId: number, payload: { client_id: number; period_start?: string; period_end?: string }) {
        return apiClient.post<{ success: boolean; message?: string; notice?: InvoiceNotice }>(
            `/projects/${projectId}/invoice-notice`,
            payload
        )
    },

    async list(projectId: number, periodStart?: string, periodEnd?: string) {
        return apiClient.get<{ success: boolean; notices?: InvoiceNotice[] }>(
            `/projects/${projectId}/invoice-notices`,
            { period_start: periodStart, period_end: periodEnd }
        )
    },

    async approve(projectId: number, noticeId: number) {
        return apiClient.post<{ success: boolean; message?: string; invoice?: Invoice; notice?: InvoiceNotice; email_sent: boolean }>(
            `/projects/${projectId}/invoice-notices/${noticeId}/approve`,
            {}
        )
    },

    async listAll(status?: 'pending' | 'approved') {
        return apiClient.get<{ success: boolean; notices?: InvoiceNoticeWithNames[] }>(
            '/invoice-notices',
            status ? { status } : undefined
        )
    },
}

export class InvoicesService {
    private static baseUrl = '/projects'

    static async getProjectInvoices(projectId: number): Promise<Invoice[]> {
        try {
            const response = await apiClient.get<InvoicesResponse>(`${this.baseUrl}/${projectId}/invoices`)

            if (response.success) {
                return response.invoices || []
            }

            throw new Error(response.message || 'Failed to fetch invoices')
        } catch (error: any) {
            console.error('Error fetching invoices:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch invoices')
        }
    }

    static async getAllInvoices(projectId?: number): Promise<Invoice[]> {
        try {
            const response = await apiClient.get<InvoicesResponse>('/invoices', projectId ? { project_id: projectId } : undefined)

            if (response.success) {
                return response.invoices || []
            }

            throw new Error(response.message || 'Failed to fetch invoices')
        } catch (error: any) {
            console.error('Error fetching invoices:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch invoices')
        }
    }

    static async createInvoice(projectId: number, data: InvoiceCreateRequest): Promise<Invoice> {
        try {
            const response = await apiClient.post<InvoicesResponse>(`${this.baseUrl}/${projectId}/invoices`, data)

            if (response.success && response.invoice) {
                return response.invoice
            }

            throw new Error(response.message || 'Failed to create invoice')
        } catch (error: any) {
            console.error('Error creating invoice:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to create invoice')
        }
    }

    static async getInvoiceBreakdown(projectId: number, invoiceId: number): Promise<InvoiceLineItem[]> {
        try {
            const response = await apiClient.get<InvoiceBreakdownResponse>(`${this.baseUrl}/${projectId}/invoices/${invoiceId}/breakdown`)

            if (response.success) {
                return response.items || []
            }

            throw new Error(response.message || 'Failed to fetch invoice breakdown')
        } catch (error: any) {
            console.error('Error fetching invoice breakdown:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch invoice breakdown')
        }
    }

    static async downloadInvoicePdf(projectId: number, invoiceId: number): Promise<Blob> {
        return apiClient.getBlob(`${this.baseUrl}/${projectId}/invoices/${invoiceId}/pdf`)
    }

    static async getProjectRevenueAnalytics(projectId: number): Promise<RevenueAnalytics> {
        try {
            const response = await apiClient.get<RevenueAnalyticsResponse>(`${this.baseUrl}/${projectId}/invoices/analytics`)

            if (response.success) {
                return { monthly: response.monthly || [], yearly: response.yearly || [], total: response.total || 0 }
            }

            throw new Error(response.message || 'Failed to fetch revenue analytics')
        } catch (error: any) {
            console.error('Error fetching project revenue analytics:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch revenue analytics')
        }
    }

    static async getClientRevenueAnalytics(clientId: number): Promise<RevenueAnalytics> {
        try {
            const response = await apiClient.get<RevenueAnalyticsResponse>(`/clients/${clientId}/invoices/analytics`)

            if (response.success) {
                return { monthly: response.monthly || [], yearly: response.yearly || [], total: response.total || 0 }
            }

            throw new Error(response.message || 'Failed to fetch revenue analytics')
        } catch (error: any) {
            console.error('Error fetching client revenue analytics:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch revenue analytics')
        }
    }

    static async sendInvoiceEmail(projectId: number, invoiceId: number): Promise<void> {
        try {
            const response = await apiClient.post<{ success: boolean; message: string }>(
                `${this.baseUrl}/${projectId}/invoices/${invoiceId}/send-email`,
                {}
            )
            if (!response.success) {
                throw new Error(response.message || 'Failed to send invoice e-mail')
            }
        } catch (error: any) {
            console.error('Error sending invoice e-mail:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to send invoice e-mail')
        }
    }

    static async refreshPaymentStatuses(projectId: number): Promise<void> {
        try {
            const response = await apiClient.post<{ success: boolean; message: string }>(
                `${this.baseUrl}/${projectId}/invoices/refresh-payment-status`,
                {}
            )
            if (!response.success) {
                throw new Error(response.message || 'Failed to refresh payment statuses')
            }
        } catch (error: any) {
            console.error('Error refreshing payment statuses:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to refresh payment statuses')
        }
    }
}
