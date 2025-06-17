import { apiClient } from '@/lib/api'
import { User, UsersResponse } from '@/types/user'

export interface UpdateProfileRequest {
    name?: string
    email?: string
    position?: string
}

export interface UpdateProfileResponse {
    success: boolean
    message: string
    user?: User
}

export interface ChangePasswordRequest {
    current_password: string
    new_password: string
    confirm_password: string
}

export interface ChangePasswordResponse {
    success: boolean
    message: string
}

export class UsersService {
    private static readonly ENDPOINT = '/users'

    static async getAllUsers(): Promise<User[]> {
        try {
            const response = await apiClient.get<UsersResponse>(this.ENDPOINT)
            return response.data
        } catch (error) {
            console.error('Failed to fetch users:', error)
            throw new Error('Failed to load users')
        }
    }

    static async getUserById(id: number): Promise<User> {
        try {
            const response = await apiClient.get<{ success: boolean; data: User }>(`${this.ENDPOINT}/${id}`)
            return response.data
        } catch (error) {
            console.error(`Failed to fetch user ${id}:`, error)
            throw new Error(`Failed to load user with ID ${id}`)
        }
    }

    static async updateProfile(data: UpdateProfileRequest): Promise<User> {
        try {
            const response = await apiClient.put<UpdateProfileResponse>(`${this.ENDPOINT}/profile/me`, data)

            if (!response.success || !response.user) {
                throw new Error(response.message || 'Failed to update profile')
            }

            return response.user
        } catch (error) {
            console.error('Failed to update profile:', error)
            if (error instanceof Error) {
                throw error
            }
            throw new Error('Failed to update profile')
        }
    }

    static async changePassword(data: ChangePasswordRequest): Promise<string> {
        try {
            const response = await apiClient.put<ChangePasswordResponse>(`${this.ENDPOINT}/profile/password`, data)

            if (!response.success) {
                throw new Error(response.message || 'Failed to change password')
            }

            return response.message
        } catch (error) {
            console.error('Failed to change password:', error)
            if (error instanceof Error) {
                throw error
            }
            throw new Error('Failed to change password')
        }
    }
}