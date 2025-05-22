import { apiClient } from '@/lib/api'
import { User, UsersResponse } from '@/types/user'

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
}