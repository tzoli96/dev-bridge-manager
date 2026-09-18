import { apiClient } from '@/lib/api'

export interface Client {
    id: number
    type: 'company' | 'individual'
    name: string
    tax_number: string
    eu_vat_number: string
    company_reg_number: string
    billing_zip: string
    billing_city: string
    billing_address: string
    bank_account_number: string
    email: string
    phone: string
    notes: string
    billingo_unit: string
    billingo_unit_price_type: '' | 'net' | 'gross'
    is_active: boolean
    created_by: number
    created_by_name: string
    created_at: string
    updated_at: string
}

export interface ClientCreateRequest {
    type?: 'company' | 'individual'
    name: string
    tax_number?: string
    eu_vat_number?: string
    company_reg_number?: string
    billing_zip?: string
    billing_city?: string
    billing_address?: string
    bank_account_number?: string
    email?: string
    phone?: string
    notes?: string
    billingo_unit?: string
    billingo_unit_price_type?: '' | 'net' | 'gross'
}

export interface ClientUpdateRequest {
    type?: 'company' | 'individual'
    name?: string
    tax_number?: string
    eu_vat_number?: string
    company_reg_number?: string
    billing_zip?: string
    billing_city?: string
    billing_address?: string
    bank_account_number?: string
    email?: string
    phone?: string
    notes?: string
    billingo_unit?: string
    billingo_unit_price_type?: '' | 'net' | 'gross'
    is_active?: boolean
}

export interface ClientsResponse {
    success: boolean
    message: string
    clients?: Client[]
    client?: Client
    count?: number
}

export class ClientsService {
    private static baseUrl = '/clients'

    static async getAllClients(): Promise<Client[]> {
        try {
            const response = await apiClient.get<ClientsResponse>(this.baseUrl)

            if (response.success) {
                return response.clients || []
            }

            throw new Error(response.message || 'Failed to fetch clients')
        } catch (error: any) {
            console.error('Error fetching clients:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch clients')
        }
    }

    static async getClient(id: number): Promise<Client> {
        try {
            const response = await apiClient.get<ClientsResponse>(`${this.baseUrl}/${id}`)

            if (response.success && response.client) {
                return response.client
            }

            throw new Error(response.message || 'Failed to fetch client')
        } catch (error: any) {
            console.error('Error fetching client:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch client')
        }
    }

    static async createClient(clientData: ClientCreateRequest): Promise<Client> {
        try {
            const response = await apiClient.post<ClientsResponse>(this.baseUrl, clientData)

            if (response.success && response.client) {
                return response.client
            }

            throw new Error(response.message || 'Failed to create client')
        } catch (error: any) {
            console.error('Error creating client:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to create client')
        }
    }

    static async updateClient(id: number, clientData: ClientUpdateRequest): Promise<Client> {
        try {
            const response = await apiClient.put<ClientsResponse>(`${this.baseUrl}/${id}`, clientData)

            if (response.success && response.client) {
                return response.client
            }

            throw new Error(response.message || 'Failed to update client')
        } catch (error: any) {
            console.error('Error updating client:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to update client')
        }
    }

    static async deleteClient(id: number): Promise<void> {
        try {
            const response = await apiClient.delete<ClientsResponse>(`${this.baseUrl}/${id}`)

            if (!response.success) {
                throw new Error(response.message || 'Failed to delete client')
            }
        } catch (error: any) {
            console.error('Error deleting client:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to delete client')
        }
    }
}
