// frontend/src/services/billingoSettingsService.ts
import { apiClient } from '@/lib/api'

export interface BillingoSettings {
    api_key_masked: string
    block_id: string
    default_unit: string
    default_unit_price_type: 'net' | 'gross'
    updated_by: number
    updated_at: string
}

export interface BillingoSettingsUpdateRequest {
    api_key: string
    block_id?: string
    default_unit?: string
    default_unit_price_type?: 'net' | 'gross'
}

export interface BillingoSettingsApiResponse {
    success: boolean
    message: string
    api_key_masked?: string
    block_id?: string
    default_unit?: string
    default_unit_price_type?: 'net' | 'gross'
    updated_by?: number
    updated_at?: string
}

export interface BillingoDocumentBlock {
    id: number
    name: string
    prefix: string
    type: string
}

interface BillingoBlocksApiResponse {
    success: boolean
    message?: string
    data?: BillingoDocumentBlock[]
}

export class BillingoSettingsService {
    private static baseUrl = '/admin/billingo-settings'

    static async getSettings(): Promise<BillingoSettings> {
        try {
            const response = await apiClient.get<BillingoSettingsApiResponse>(this.baseUrl)

            if (response.success) {
                return {
                    api_key_masked: response.api_key_masked || '',
                    block_id: response.block_id || '',
                    default_unit: response.default_unit || 'db',
                    default_unit_price_type: response.default_unit_price_type || 'net',
                    updated_by: response.updated_by || 0,
                    updated_at: response.updated_at || ''
                }
            }

            throw new Error(response.message || 'Failed to fetch Billingo settings')
        } catch (error: any) {
            console.error('Error fetching Billingo settings:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch Billingo settings')
        }
    }

    static async updateSettings(data: BillingoSettingsUpdateRequest): Promise<BillingoSettings> {
        try {
            const response = await apiClient.put<BillingoSettingsApiResponse>(this.baseUrl, data)

            if (response.success) {
                return {
                    api_key_masked: response.api_key_masked || '',
                    block_id: response.block_id || '',
                    default_unit: response.default_unit || 'db',
                    default_unit_price_type: response.default_unit_price_type || 'net',
                    updated_by: response.updated_by || 0,
                    updated_at: response.updated_at || ''
                }
            }

            throw new Error(response.message || 'Failed to update Billingo settings')
        } catch (error: any) {
            console.error('Error updating Billingo settings:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to update Billingo settings')
        }
    }

    static async getBlocks(): Promise<BillingoDocumentBlock[]> {
        try {
            const response = await apiClient.get<BillingoBlocksApiResponse>(`${this.baseUrl}/blocks`)

            if (response.success) {
                return response.data || []
            }

            throw new Error(response.message || 'Failed to fetch Billingo invoice blocks')
        } catch (error: any) {
            console.error('Error fetching Billingo invoice blocks:', error)
            throw new Error(error.response?.data?.message || error.message || 'Failed to fetch Billingo invoice blocks')
        }
    }
}
