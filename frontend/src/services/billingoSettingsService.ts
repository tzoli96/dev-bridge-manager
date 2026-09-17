// frontend/src/services/billingoSettingsService.ts
import { apiClient } from '@/lib/api'

export interface BillingoSettings {
    api_key_masked: string
    block_id: string
    updated_by: number
    updated_at: string
}

export interface BillingoSettingsUpdateRequest {
    api_key: string
    block_id?: string
}

export interface BillingoSettingsApiResponse {
    success: boolean
    message: string
    api_key_masked?: string
    block_id?: string
    updated_by?: number
    updated_at?: string
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
}
