// frontend/src/services/invoicesService.ts
import { apiClient } from '@/lib/api'

export interface Invoice {
    id: number
    project_id: number
    client_id: number
    client_name: string
    billingo_invoice_id: string
    billingo_invoice_number: string
    pricing_type: 'hourly' | 'fixed'
    period_start: string | null
    period_end: string | null
    amount: number
    status: 'created' | 'failed'
    error_message: string
    created_by: number
    created_by_name: string
    created_at: string
}

export interface InvoiceCreateRequest {
    client_id: number
    period_start?: string
    period_end?: string
}

export interface InvoicesResponse {
    success: boolean
    message: string
    invoice?: Invoice
    invoices?: Invoice[]
    count?: number
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
}
