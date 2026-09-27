// frontend/src/services/invoiceReconciliationService.ts
import { apiClient } from '@/lib/api'

export interface InvoiceReconciliationFlag {
    id: number
    project_id: number
    invoice_id: number | null
    type: 'discrepancy' | 'stale_hours'
    details: string
    status: 'pending' | 'dismissed'
    created_at: string
    dismissed_by: number | null
    dismissed_at: string | null
}

export interface InvoiceReconciliationFlagWithNames extends InvoiceReconciliationFlag {
    project_name: string
}

interface ListApiResponse {
    success: boolean
    message?: string
    flags?: InvoiceReconciliationFlagWithNames[]
}

interface ActionApiResponse {
    success: boolean
    message?: string
    flag?: InvoiceReconciliationFlag
}

export const InvoiceReconciliationService = {
    async list(status?: 'pending' | 'dismissed'): Promise<InvoiceReconciliationFlagWithNames[]> {
        const response = await apiClient.get<ListApiResponse>(
            '/admin/invoice-reconciliation-flags',
            status ? { status } : undefined
        )
        if (response.success) return response.flags || []
        throw new Error(response.message || 'Failed to fetch invoice reconciliation flags')
    },

    async dismiss(id: number): Promise<void> {
        const response = await apiClient.post<ActionApiResponse>(`/admin/invoice-reconciliation-flags/${id}/dismiss`, {})
        if (!response.success) throw new Error(response.message || 'Failed to dismiss flag')
    },
}
