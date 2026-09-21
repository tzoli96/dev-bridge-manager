// frontend/src/services/profileService.ts
import { apiClient } from '@/lib/api'

export interface ProfileSample {
    id?: number
    label: string
    content: string
}

export interface ProfileData {
    background: string
    expertise: string
    tone_rules: string
    samples: ProfileSample[]
    updated_by: number
    updated_at: string
}

interface ProfileApiResponse {
    success: boolean
    message?: string
    profile?: {
        id: number
        background: string
        expertise: string
        tone_rules: string
        samples: ProfileSample[] | null
        updated_by: number
        updated_at: string
    }
}

export interface ProfileUpdateRequest {
    background: string
    expertise: string
    tone_rules: string
    samples: ProfileSample[]
}

export class ProfileService {
    private static baseUrl = '/admin/profile'

    static async getProfile(): Promise<ProfileData> {
        const response = await apiClient.get<ProfileApiResponse>(this.baseUrl)
        if (response.success && response.profile) {
            return {
                background: response.profile.background,
                expertise: response.profile.expertise,
                tone_rules: response.profile.tone_rules,
                samples: response.profile.samples || [],
                updated_by: response.profile.updated_by,
                updated_at: response.profile.updated_at,
            }
        }
        throw new Error(response.message || 'Failed to fetch profile')
    }

    static async updateProfile(data: ProfileUpdateRequest): Promise<ProfileData> {
        const response = await apiClient.put<ProfileApiResponse>(this.baseUrl, data)
        if (response.success && response.profile) {
            return {
                background: response.profile.background,
                expertise: response.profile.expertise,
                tone_rules: response.profile.tone_rules,
                samples: response.profile.samples || [],
                updated_by: response.profile.updated_by,
                updated_at: response.profile.updated_at,
            }
        }
        throw new Error(response.message || 'Failed to update profile')
    }
}
